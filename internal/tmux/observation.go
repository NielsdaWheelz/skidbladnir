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
// lower top rows as clipped, as under the byte limit. CapturePanes reads its
// script from stdin, without that limit, and keeps the same selection.
const (
	bottomRegionRows  = 64
	bottomRegionBytes = 64 << 10
	topRegionRows     = 192
	topRegionBytes    = 16 << 10

	observedMarker      = "SKIDBLADNIR_OBSERVED"
	capturedMarker      = "SKIDBLADNIR_CAPTURED"
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
// selected-pane guard. Errors: ErrTargetChanged, ErrScreenChanged (dimensions
// or alternate screen changed), ErrUnavailable.
func (client Client) ObservePane(ctx context.Context, target PaneTarget) (PaneObservation, error) {
	pane := commandToken(target.PaneID)
	screen, err := client.Output(ctx, "read-pane-screen", "-N", "if-shell", "-F", "-t", target.SessionID, target.condition(),
		"display-message -p -t "+pane+" '"+paneScreenFormat+"'",
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
	rows := newObservedRows(width, height, fields[2] == "1")
	// The capture runs only while the target and the screen read above are
	// unchanged; the refusal distinguishes a changed screen from a stale target.
	condition := andFormatConditions([]string{target.condition(), "#{==:" + paneScreenFormat + "," + formatLiteral(screen) + "}"})
	output := captureOutput{body: &rows}
	command := client.command(ctx, nil, "-N", "if-shell", "-F", "-t", target.SessionID, condition,
		"display-message -p -l '"+observedMarker+"'"+rows.commands(target.PaneID), target.refusal(screenChangedMarker))
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
	return rows.observation()
}

// ScreenTarget is an exact pane screen as an inventory row read it: the
// capture guard's target.
type ScreenTarget struct {
	PaneTarget
	PanePID       int
	Width, Height int
	Alternate     bool
}

// PaneCapture is one CapturePanes block: its observation, or ErrScreenChanged
// when its guard refused, or ErrUnavailable when it could not be read (a
// screen without rows or columns, a row that is not UTF-8).
type PaneCapture struct {
	Observation PaneObservation
	Err         error
}

// CapturePanes captures every target's selected rows in one client between B
// and E records (tmux-exec-budget.md a2). Each target's block runs only while
// its lifetime, session, selected pane, pane pid and screen are the row's, and
// every -t inside a block sits behind that guard: an unguarded failing target
// would end the client and lose every block. Server is B (== E), zero when no
// server runs (captures is then nil). Errors are batch's
// (WorkspaceSourceChanged, WorkspaceLimit, WorkspaceProtocolInvalid, ctx,
// child) and mean no entry is valid.
func (client Client) CapturePanes(ctx context.Context, targets []ScreenTarget) (ServerIdentity, []PaneCapture, error) {
	captures := make([]PaneCapture, len(targets))
	rows := make([]observedRows, len(targets))
	blocks := make([]string, 0, len(targets))
	for index, target := range targets {
		if target.Width < 1 || target.Height < 1 {
			captures[index].Err = ErrUnavailable
			continue
		}
		rows[index] = newObservedRows(target.Width, target.Height, target.Alternate)
		alternate := "0"
		if target.Alternate {
			alternate = "1"
		}
		condition := andFormatConditions([]string{
			target.condition(),
			"#{==:#{pane_pid}," + formatLiteral(strconv.Itoa(target.PanePID)) + "}",
			"#{==:" + paneScreenFormat + "," + formatLiteral(strconv.Itoa(target.Width)+" "+strconv.Itoa(target.Height)+" "+alternate) + "}",
		})
		branch := "display-message -p -l '" + observedMarker + "'" + rows[index].commands(target.PaneID) + " ; display-message -p -l '" + capturedMarker + "'"
		blocks = append(blocks, commandText("if-shell", "-F", "-t", target.SessionID, condition, branch, "display-message -p -l '"+screenChangedMarker+"'"))
	}
	server, transcript, err := client.batch(ctx, strings.Join(blocks, " ; "))
	if err != nil || server == (ServerIdentity{}) {
		return server, nil, err
	}
	for index := range targets {
		if captures[index].Err != nil {
			continue // no block
		}
		marker, rest, found := bytes.Cut(transcript, []byte{'\n'})
		if !found {
			return ServerIdentity{}, nil, WorkspaceProtocolInvalid
		}
		if string(marker) == screenChangedMarker {
			captures[index].Err = ErrScreenChanged
			transcript = rest
			continue
		}
		if string(marker) != observedMarker {
			return ServerIdentity{}, nil, WorkspaceProtocolInvalid
		}
		// A row never contains a newline, so counting newlines frames the
		// block even when a row looks like a marker.
		end := 0
		for range rows[index].expected {
			next := bytes.IndexByte(rest[end:], '\n')
			if next < 0 {
				return ServerIdentity{}, nil, WorkspaceProtocolInvalid
			}
			end += next + 1
		}
		marker, transcript, found = bytes.Cut(rest[end:], []byte{'\n'})
		if !found || string(marker) != capturedMarker {
			return ServerIdentity{}, nil, WorkspaceProtocolInvalid
		}
		if _, err := rows[index].Write(rest[:end]); err != nil {
			captures[index].Err = ErrUnavailable
			continue
		}
		captures[index].Observation, captures[index].Err = rows[index].observation()
	}
	if len(transcript) != 0 {
		return ServerIdentity{}, nil, WorkspaceProtocolInvalid
	}
	return server, captures, nil
}

// observedRows is one screen's row selection: it emits the captures and
// receives their rows in capture order, refusing any byte past the expected
// last row. Each region keeps rows until the next would exceed its byte limit;
// the rest of that region is discarded as it arrives, so memory stays within
// the two limits.
type observedRows struct {
	width, height int
	alternate     bool
	above         int // rows above the bottom region
	topCount      int // leading rows that belong to the top region
	expected      int
	received      int
	row           []byte
	top           observedRegion
	bottom        observedRegion
}

type observedRegion struct {
	remaining int // bytes the region may still keep
	rows      []string
	clipped   bool
}

func newObservedRows(width, height int, alternate bool) observedRows {
	above := max(0, height-bottomRegionRows)
	top := min(topRegionRows, above)
	return observedRows{
		width: width, height: height, alternate: alternate, above: above, topCount: top, expected: top + min(bottomRegionRows, height),
		top: observedRegion{remaining: topRegionBytes}, bottom: observedRegion{remaining: bottomRegionBytes},
	}
}

// commands is the " ; capture-pane …" run of the selected rows. Each row is
// its own capture: one capture carries style state from row to row, so only a
// fresh capture starts a row from the default style. The bottom region is read
// upward so both regions arrive nearest-edge first.
func (output *observedRows) commands(paneID string) string {
	pane := commandToken(paneID)
	var run strings.Builder
	capture := func(row int) {
		line := strconv.Itoa(row)
		run.WriteString(" ; capture-pane -p -e -t " + pane + " -S " + line + " -E " + line)
	}
	for row := range output.topCount {
		capture(row)
	}
	for row := output.height - 1; row >= output.height-(output.expected-output.topCount); row-- {
		capture(row)
	}
	return run.String()
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

// observation assembles the received rows; a short capture is ErrUnavailable.
func (output *observedRows) observation() (PaneObservation, error) {
	if output.received != output.expected {
		return PaneObservation{}, ErrUnavailable
	}
	observation := PaneObservation{Width: output.width, Height: output.height, Alternate: output.alternate}
	if output.topCount > 0 {
		observation.Regions = append(observation.Regions, PaneRegion{Kind: RegionTop, Rows: output.top.rows, Clipped: output.top.clipped || output.topCount < output.above})
	}
	slices.Reverse(output.bottom.rows)
	observation.Regions = append(observation.Regions, PaneRegion{
		Kind: RegionBottom, FirstRow: output.height - len(output.bottom.rows), Rows: output.bottom.rows, Clipped: output.bottom.clipped,
	})
	return observation, nil
}
