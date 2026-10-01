package agentcontrol

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// screen is one PaneObservation parsed once into physical rows: rows[r] is
// pane row r, so len(rows) is the pane height. The regions are contiguous
// unless a byte or row limit clipped one, so a row the observation does not
// hold was dropped, and a grammar that needs it has clipped evidence.
type screen struct {
	width     int
	alternate bool
	rows      []row
}

// row is one physical row, parsed on its own from the default style.
type row struct {
	presence presence
	region   tmuxclient.RegionKind // captured rows only
	// cells holds one cell per display column, from column 0 to the last cell
	// tmux wrote. tmux can drop a row's trailing spaces, styled or not, so a
	// trailing space may be absent.
	cells []cell
}

type presence uint8

const (
	rowAbsent      presence = iota // dropped by its region's byte or row limit
	rowUnparseable                 // holds an escape other than SGR, OSC 8 or SO/SI
	rowParsed
)

// cell is one display column. A wide grapheme fills its first column; the
// next column is a continuation cell with empty text and the same style. A
// cell shifted into the line-drawing set by SO keeps tmux's raw letter: no
// grammar anchors on it, both providers draw with Unicode box glyphs.
type cell struct {
	text  string // one grapheme cluster with its zero-width marks
	style style
}

// style keeps the SGR attributes the grammars read.
type style struct {
	bold, dim, reverse bool
	fg, bg             color
}

// color is an explicit SGR colour. The basic and bright SGR colours are
// palette entries 0-15, so a colour reads the same however tmux encoded it.
type color struct {
	kind  colorKind
	value uint32 // palette index, or 0xRRGGBB
}

type colorKind uint8

const (
	colorDefault colorKind = iota
	colorPalette
	colorRGB
)

// parseScreen parses every captured row once; the rows no region holds stay
// absent.
func parseScreen(observation tmuxclient.PaneObservation) screen {
	parsed := screen{width: observation.Width, alternate: observation.Alternate, rows: make([]row, observation.Height)}
	var decoder ansi.Parser
	// tmux writes at most 12 parameters in one SGR. Given 32 slots, ansi's
	// decoder drops a 32nd parameter and panics on a 33rd: a CSI that long is
	// outside tmux's writer, so it is a defect, not an unparseable row.
	decoder.SetParamsSize(32)
	for _, region := range observation.Regions {
		for index, text := range region.Rows {
			parsed.rows[region.FirstRow+index] = parseRow(region.Kind, text, &decoder)
		}
	}
	return parsed
}

// parseRow decodes one row as tmux capture-pane -e writes it: graphemes, SGR,
// OSC 8 hyperlinks and the SO/SI charset shifts, which take no column. Any
// other escape or control makes the row unparseable.
func parseRow(region tmuxclient.RegionKind, text string, decoder *ansi.Parser) row {
	parsed := row{presence: rowParsed, region: region}
	unparseable := row{presence: rowUnparseable, region: region}
	var current style
	for text != "" {
		if rest, ok := strings.CutPrefix(text, "\x1b]8;"); ok {
			// tmux writes a hyperlink ST-terminated, with its URI's valid UTF-8
			// raw and every control escaped, so the first ST ends it. ansi's
			// decoder would also end it at a 0x9c continuation byte.
			_, after, found := strings.Cut(rest, "\x1b\\")
			if !found {
				return unparseable
			}
			text = after
			continue
		}
		sequence, width, size, state := ansi.DecodeSequence(text, ansi.NormalState, decoder)
		text = text[size:]
		switch {
		case state != ansi.NormalState:
			return unparseable
		case strings.HasPrefix(sequence, "\x1b["):
			if decoder.Command() != 'm' || !applySGR(&current, decoder.Params()) {
				return unparseable
			}
		case sequence == "\x0e" || sequence == "\x0f":
		case sequence[0] < ' ' || sequence[0] == ansi.DEL:
			return unparseable
		case width == 0:
			// A zero-width grapheme joins the cell before it, as in tmux.
			if last := len(parsed.cells) - 1; last >= 0 {
				for parsed.cells[last].text == "" {
					last--
				}
				parsed.cells[last].text += sequence
			}
		default:
			parsed.cells = append(parsed.cells, cell{text: sequence, style: current})
			for range width - 1 {
				parsed.cells = append(parsed.cells, cell{style: current})
			}
		}
	}
	return parsed
}

// applySGR folds one SGR into current. It accepts exactly what tmux's
// grid_string_cells_code writes: 0 then the attributes still set (tmux resets
// only with 0), the attributes 1, 2, 3, 4, 5, 7, 8 and 9, the underline styles
// as 4:N and overline as 5:3, and colours as semicolon parameters (30-37,
// 90-97, 40-47, 100-107, 39, 49, and 38, 48 or 58 with 5;n or 2;r;g;b).
// Anything else is refused, never guessed.
func applySGR(current *style, params ansi.Params) bool {
	if len(params) == 0 {
		return false
	}
	for index := 0; index < len(params); index++ {
		code := params[index].Param(0)
		if params[index].HasMore() {
			if code != 4 && code != 5 {
				return false
			}
			index++
			continue
		}
		switch {
		case code == 0:
			*current = style{}
		case code == 1:
			current.bold = true
		case code == 2:
			current.dim = true
		case code == 7:
			current.reverse = true
		case code == 3, code == 4, code == 5, code == 8, code == 9:
			// italic, underline, blink, conceal and strikethrough: no grammar reads them.
		case code >= 30 && code <= 37:
			current.fg = color{kind: colorPalette, value: uint32(code - 30)}
		case code >= 90 && code <= 97:
			current.fg = color{kind: colorPalette, value: uint32(code - 90 + 8)}
		case code >= 40 && code <= 47:
			current.bg = color{kind: colorPalette, value: uint32(code - 40)}
		case code >= 100 && code <= 107:
			current.bg = color{kind: colorPalette, value: uint32(code - 100 + 8)}
		case code == 39:
			current.fg = color{}
		case code == 49:
			current.bg = color{}
		case code == 38 || code == 48 || code == 58:
			var value color
			switch kind, _, _ := params.Param(index+1, -1); kind {
			case 5:
				palette, _, _ := params.Param(index+2, -1)
				if palette < 0 || palette > 255 {
					return false
				}
				value, index = color{kind: colorPalette, value: uint32(palette)}, index+2
			case 2:
				rgb := uint32(0)
				for offset := 2; offset <= 4; offset++ {
					channel, _, _ := params.Param(index+offset, -1)
					if channel < 0 || channel > 255 {
						return false
					}
					rgb = rgb<<8 | uint32(channel)
				}
				value, index = color{kind: colorRGB, value: rgb}, index+4
			default:
				return false
			}
			switch code {
			case 38:
				current.fg = value
			case 48:
				current.bg = value
			}
		default:
			return false
		}
	}
	return true
}

// blank reports a parsed row with no visible glyph: every cell is a space or
// NBSP, whatever its style, so a background-only padding row is blank. An
// absent or unparseable row is never blank.
func (row row) blank() bool {
	if row.presence != rowParsed {
		return false
	}
	for _, cell := range row.cells {
		switch cell.text {
		case " ", "\u00a0":
		default:
			return false
		}
	}
	return true
}

// text joins the row's graphemes; a continuation column adds nothing.
func (row row) text() string {
	if row.presence != rowParsed {
		panic("text of a row that was not parsed") // justify-defect: grammars read only parsed rows.
	}
	var text strings.Builder
	for _, cell := range row.cells {
		text.WriteString(cell.text)
	}
	return text.String()
}

// cell is the cell at column; a column past the cells tmux emitted is a
// default space.
func (row row) cell(column int) cell {
	if row.presence != rowParsed {
		panic("cell of a row that was not parsed") // justify-defect: grammars read only parsed rows.
	}
	if column < len(row.cells) {
		return row.cells[column]
	}
	return cell{text: " "}
}

// rule names a matched rule by the region of its decisive rows first..last:
// compound when they span both regions.
func (screen screen) rule(id string, first, last int) DiagnosticRule {
	upper, lower := screen.rows[first], screen.rows[last]
	if upper.presence == rowAbsent || lower.presence == rowAbsent {
		panic("rule decided by an absent row") // justify-defect: grammars decide only on captured rows.
	}
	var region DiagnosticRegion
	switch {
	case upper.region != lower.region:
		region = DiagnosticCompound
	case upper.region == tmuxclient.RegionTop:
		region = DiagnosticTop
	case upper.region == tmuxclient.RegionBottom:
		region = DiagnosticBottom
	default:
		panic("unknown screen region") // justify-defect: tmux has exactly two region kinds.
	}
	return DiagnosticRule{ID: id, Region: region}
}
