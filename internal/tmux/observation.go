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

// ErrObservationChanged means the pane's dimensions or screen changed between
// reading them and capturing its rows; the sample is invalid.
var ErrObservationChanged = errors.New("terminal pane changed during observation")

// The bottom region holds the current composer, footer and request controls;
// the top region holds modals that providers anchor to the top of tall panes.
// Rows are selected before bytes: a byte limit drops the rows farthest from
// the region's anchored edge.
const (
	bottomRegionRows  = 64
	bottomRegionBytes = 64 << 10
	topRegionRows     = 24
	topRegionBytes    = 16 << 10

	observedMarker           = "SKIDBLADNIR_OBSERVED"
	observationChangedMarker = "SKIDBLADNIR_OBSERVATION_CHANGED"
)

type RegionKind string

const (
	RegionTop    RegionKind = "top"
	RegionBottom RegionKind = "bottom"
)

// PaneRegion is a contiguous run of physical visible rows. Every row is
// self-contained: read from the default style, its SGR sequences reproduce its
// styling. Rows are complete UTF-8, contain no newline, and are never joined.
type PaneRegion struct {
	Kind     RegionKind
	FirstRow int // zero-based physical row of Rows[0]
	Rows     []string
	Clipped  bool // requested rows were dropped at the region's byte limit
}

// PaneObservation is one bounded sample of the visible screen: the bottom rows
// and, for a pane taller than the bottom region, the non-overlapping top rows.
// Regions are in screen order (top first). It is not an atomic snapshot of the
// program drawing it, and it never includes scrollback.
type PaneObservation struct {
	Width     int
	Height    int
	Alternate bool
	Regions   []PaneRegion
}

// ObservePane samples the exact target's visible rows under its lifetime and
// selected-pane guard. Errors: ErrInputInvalid, ErrTargetChanged,
// ErrObservationChanged (dimensions or alternate screen changed), ErrUnavailable.
func (client Client) ObservePane(ctx context.Context, target PaneTarget) (PaneObservation, error) {
	if !target.valid() {
		return PaneObservation{}, ErrInputInvalid
	}
	screen, err := client.Output(ctx, "read-pane-screen", "-N", "if-shell", "-F", "-t", target.SessionID, target.condition(),
		"display-message -p -t '"+target.PaneID+"' '#{pane_width} #{pane_height} #{alternate_on}'",
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
	if widthErr != nil || heightErr != nil || width < 1 || height < 1 || fields[2] != "0" && fields[2] != "1" {
		return PaneObservation{}, ErrUnavailable
	}

	top := 0
	if height > bottomRegionRows {
		top = min(topRegionRows, height-bottomRegionRows)
	}
	bottom := min(bottomRegionRows, height)
	// Each row is its own capture: one capture carries style state from row to
	// row, so only a fresh capture starts a row from the default style. The
	// bottom region is read upward so both regions arrive nearest-edge first.
	rows := make([]int, 0, top+bottom)
	for row := range top {
		rows = append(rows, row)
	}
	for row := height - 1; row >= height-bottom; row-- {
		rows = append(rows, row)
	}
	var branch strings.Builder
	branch.WriteString("display-message -p -l '" + observedMarker + "'")
	for _, row := range rows {
		line := strconv.Itoa(row)
		branch.WriteString(" ; capture-pane -p -e -t '" + target.PaneID + "' -S " + line + " -E " + line)
	}
	// The capture runs only while the target and the screen read above are
	// unchanged; the refusal distinguishes a changed screen from a stale target.
	condition := andFormatConditions([]string{target.condition(), "#{==:#{pane_width}," + strconv.Itoa(width) + "}",
		"#{==:#{pane_height}," + strconv.Itoa(height) + "}", "#{==:#{alternate_on}," + fields[2] + "}"})
	refusal := "if-shell -F -t '" + target.SessionID + "' '" + target.condition() + "' 'display-message -p -l " +
		observationChangedMarker + "' 'display-message -p -l " + identityMismatchMarker + "'"
	regions := observedRows{topRows: top, expected: len(rows), top: observedRegion{limit: topRegionBytes}, bottom: observedRegion{limit: bottomRegionBytes}}
	output := captureOutput{body: &regions}
	command := client.command(ctx, nil, "-N", "if-shell", "-F", "-t", target.SessionID, condition, branch.String(), refusal)
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return PaneObservation{}, ErrUnavailable
	}
	switch output.header {
	case observedMarker:
	case observationChangedMarker:
		return PaneObservation{}, ErrObservationChanged
	case identityMismatchMarker:
		return PaneObservation{}, ErrTargetChanged
	default:
		return PaneObservation{}, ErrUnavailable
	}
	if regions.received != regions.expected || len(regions.row) != 0 {
		return PaneObservation{}, ErrUnavailable
	}
	observation := PaneObservation{Width: width, Height: height, Alternate: fields[2] == "1"}
	if top > 0 {
		observation.Regions = append(observation.Regions, PaneRegion{Kind: RegionTop, Rows: regions.top.rows, Clipped: regions.top.clipped})
	}
	slices.Reverse(regions.bottom.rows)
	observation.Regions = append(observation.Regions, PaneRegion{
		Kind: RegionBottom, FirstRow: height - len(regions.bottom.rows), Rows: regions.bottom.rows, Clipped: regions.bottom.clipped,
	})
	return observation, nil
}

// observedRows receives the rows in capture order. Each region keeps rows
// until the next would exceed its byte limit; the rest of that region is
// discarded as it arrives, so memory stays within the two limits.
type observedRows struct {
	topRows  int // leading rows that belong to the top region
	expected int
	received int
	row      []byte
	top      observedRegion
	bottom   observedRegion
}

type observedRegion struct {
	limit   int
	size    int
	rows    []string
	clipped bool
}

func (output *observedRows) Write(contents []byte) (int, error) {
	count := len(contents)
	for len(contents) > 0 {
		if output.received == output.expected {
			return 0, errors.New("unexpected observation row")
		}
		region := &output.bottom
		if output.received < output.topRows {
			region = &output.top
		}
		line, rest, complete := bytes.Cut(contents, []byte{'\n'})
		if !region.clipped && region.size+len(output.row)+len(line) <= region.limit {
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
				region.size += len(output.row)
			}
			output.row = output.row[:0]
			output.received++
		}
		contents = rest
	}
	return count, nil
}
