package tmux

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrScreenChanged means the pane's dimensions or alternate screen changed
// between reading them and capturing its rows; the sample is invalid.
var ErrScreenChanged = errors.New("terminal screen changed during observation")

// The bottom region holds the current composer, footer and request controls;
// the top region is the rows above it, read from row 0, so the two are
// contiguous and a tall pane's top-anchored modal or short transcript is
// captured whole. Rows are selected before bytes: a byte limit drops the rows
// farthest from the region's anchored edge. Each row is its own capture-pane
// command and tmux refuses a client command over its 16 KiB message, so the
// top region requests at most topRegionRows rows; a taller pane loses its
// lower top rows as clipped, as under the byte limit.
const (
	bottomRegionRows  = 64
	bottomRegionBytes = 64 << 10
	topRegionRows     = 192
	topRegionBytes    = 16 << 10

	observedMarker      = "SKIDBLADNIR_OBSERVED"
	screenChangedMarker = "SKIDBLADNIR_SCREEN_CHANGED"
	// paneScreenFormat is read once and then required unchanged by the capture.
	paneScreenFormat = "#{pane_width} #{pane_height} #{alternate_on}"
)

type RegionKind string

const (
	RegionTop    RegionKind = "top"
	RegionBottom RegionKind = "bottom"
)

// PaneRegion is a contiguous run of physical visible rows: Rows covers rows
// FirstRow through FirstRow+len(Rows)-1, zero-based. Every row is
// self-contained: parsed from the default terminal state, its escape sequences
// (SGR, OSC 8 hyperlinks carrying their URI, SO/SI charset shifts) reproduce its
// styling, and no state carries to the next row. Rows are complete UTF-8,
// contain no newline, and are never joined. A clipped region may hold no rows
// when its anchored row alone exceeds the byte limit; the bottom region then
// has FirstRow == Height.
type PaneRegion struct {
	Kind     RegionKind
	FirstRow int
	Rows     []string
	Clipped  bool // rows of the region were dropped at its byte or row limit
}

// PaneObservation is one bounded sample of the visible screen: the bottom rows
// and, for a pane taller than the bottom region, the rows above them. Regions
// are in screen order (top first) and contiguous unless a limit clipped one.
// It is not an atomic snapshot of the program drawing it, and it never
// includes scrollback.
type PaneObservation struct {
	Width     int
	Height    int
	Alternate bool
	Regions   []PaneRegion
}

// ObservePane samples the exact target's visible rows under its lifetime and
// selected-pane guard. Errors: ErrInputInvalid, ErrTargetChanged,
// ErrScreenChanged (dimensions or alternate screen changed), ErrUnavailable.
func (client Client) ObservePane(ctx context.Context, target PaneTarget) (PaneObservation, error) {
	if !target.valid() {
		return PaneObservation{}, ErrInputInvalid
	}
	screen, err := client.Output(ctx, "read-pane-screen", "-N", "if-shell", "-F", "-t", target.SessionID, target.condition(),
		"display-message -p -t '"+target.PaneID+"' '"+paneScreenFormat+"'",
		"display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		return PaneObservation{}, ErrUnavailable
	}
	if screen == identityMismatchMarker {
		return PaneObservation{}, ErrTargetChanged
	}
	fields := strings.Split(screen, " ")
	if len(fields) != 3 {
		return PaneObservation{}, ErrUnavailable
	}
	width, widthErr := strconv.Atoi(fields[0])
	height, heightErr := strconv.Atoi(fields[1])
	if widthErr != nil || heightErr != nil || width < 1 || height < 1 {
		return PaneObservation{}, ErrUnavailable
	}
	if fields[2] != "0" && fields[2] != "1" {
		return PaneObservation{}, ErrUnavailable
	}
	alternate := fields[2] == "1"

	above := max(0, height-bottomRegionRows)
	top := min(topRegionRows, above)
	bottom := min(bottomRegionRows, height)
	// Each row is its own capture: one capture carries style state from row to
	// row, so only a fresh capture starts a row from the default style. The
	// bottom region is read upward so both regions arrive nearest-edge first.
	requested := make([]int, 0, top+bottom)
	for row := range top {
		requested = append(requested, row)
	}
	for row := height - 1; row >= height-bottom; row-- {
		requested = append(requested, row)
	}
	var branch strings.Builder
	branch.WriteString("display-message -p -l '" + observedMarker + "'")
	for _, row := range requested {
		line := strconv.Itoa(row)
		branch.WriteString(" ; capture-pane -p -e -t '" + target.PaneID + "' -S " + line + " -E " + line)
	}
	// The capture runs only while the target and the screen read above are
	// unchanged; the refusal distinguishes a changed screen from a stale target.
	condition := andFormatConditions([]string{target.condition(), "#{==:" + paneScreenFormat + "," + formatLiteral(screen) + "}"})
	rows := observedRows{topCount: top, expected: len(requested), top: observedRegion{remaining: topRegionBytes}, bottom: observedRegion{remaining: bottomRegionBytes}}
	output := captureOutput{body: &rows}
	command := client.command(ctx, nil, "-N", "if-shell", "-F", "-t", target.SessionID, condition, branch.String(), target.refusal(screenChangedMarker))
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return PaneObservation{}, ErrUnavailable
	}
	switch output.header {
	case observedMarker:
	case screenChangedMarker:
		return PaneObservation{}, ErrScreenChanged
	case identityMismatchMarker:
		return PaneObservation{}, ErrTargetChanged
	default:
		return PaneObservation{}, ErrUnavailable
	}
	if rows.received != rows.expected {
		return PaneObservation{}, ErrUnavailable
	}
	observation := PaneObservation{Width: width, Height: height, Alternate: alternate}
	if top > 0 {
		observation.Regions = append(observation.Regions, PaneRegion{Kind: RegionTop, Rows: rows.top.rows, Clipped: rows.top.clipped || top < above})
	}
	slices.Reverse(rows.bottom.rows)
	observation.Regions = append(observation.Regions, PaneRegion{
		Kind: RegionBottom, FirstRow: height - len(rows.bottom.rows), Rows: rows.bottom.rows, Clipped: rows.bottom.clipped,
	})
	return observation, nil
}

// observedRows receives the rows in capture order and refuses any byte past the
// expected last row. Each region keeps rows until the next would exceed its
// byte limit; the rest of that region is discarded as it arrives, so memory
// stays within the two limits.
type observedRows struct {
	topCount int // leading rows that belong to the top region
	expected int
	received int
	row      []byte
	top      observedRegion
	bottom   observedRegion
}

type observedRegion struct {
	remaining int // bytes the region may still keep
	rows      []string
	clipped   bool
}

func (output *observedRows) Write(contents []byte) (int, error) {
	count := len(contents)
	for len(contents) > 0 {
		if output.received == output.expected {
			return 0, errors.New("unexpected observation row")
		}
		region := &output.bottom
		if output.received < output.topCount {
			region = &output.top
		}
		line, rest, complete := bytes.Cut(contents, []byte{'\n'})
		if !region.clipped && len(output.row)+len(line) <= region.remaining {
			output.row = append(output.row, line...)
		} else {
			region.clipped = true
		}
		if complete {
			if !region.clipped {
				if !utf8.Valid(output.row) {
					return 0, errors.New("observation row is not UTF-8")
				}
				region.rows = append(region.rows, string(output.row))
				region.remaining -= len(output.row)
			}
			output.row = output.row[:0]
			output.received++
		}
		contents = rest
	}
	return count, nil
}
