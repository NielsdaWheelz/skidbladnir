package agentcontrol

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// screen is one PaneObservation parsed once into physical rows: rows[r] is
// pane row r. The regions are contiguous unless a byte or row limit clipped
// one, so a row the observation does not hold was dropped, and a grammar that
// needs it has clipped evidence.
type screen struct {
	width, height int
	alternate     bool
	rows          []row
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
// next column is a continuation cell with empty text and the same style.
type cell struct {
	text  string // one grapheme cluster with its zero-width marks
	style style
}

// style keeps the SGR attributes the grammars read. Blink, conceal,
// strikethrough, overline and underline colour are accepted and dropped.
type style struct {
	bold, dim, italic, underline, reverse bool
	fg, bg                                color
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
	parsed := screen{width: observation.Width, height: observation.Height, alternate: observation.Alternate, rows: make([]row, observation.Height)}
	var decoder ansi.Parser
	decoder.SetParamsSize(32) // more than any SGR tmux writes
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
		if strings.HasPrefix(text, "\x1b]") {
			// tmux keeps valid UTF-8 raw inside a hyperlink's URI and escapes
			// every control in it, so only ST or BEL ends it. ansi's decoder
			// would also end it at a 0x9c continuation byte.
			end := strings.IndexAny(text[2:], "\x07\x1b") + 2
			if !strings.HasPrefix(text, "\x1b]8;") || end < 2 {
				return unparseable
			}
			if text[end] == '\x1b' {
				if !strings.HasPrefix(text[end:], "\x1b\\") {
					return unparseable
				}
				end++
			}
			text = text[end+1:]
			continue
		}
		sequence, width, size, state := ansi.DecodeSequence(text, ansi.NormalState, decoder)
		text = text[size:]
		switch {
		case state != ansi.NormalState:
			return unparseable
		case sequence[0] == ansi.ESC:
			if len(sequence) < 3 || sequence[1] != '[' || decoder.Command() != 'm' || !applySGR(&current, decoder.Params()) {
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

// applySGR folds one SGR into current. It accepts the codes tmux writes
// (grid_string_cells_code: attributes, 4:N underline styles, overline written
// as 5:3, palette and rgb colours as semicolon parameters) and their resets;
// anything else is refused, never guessed.
func applySGR(current *style, params ansi.Params) bool {
	if len(params) == 0 {
		*current = style{}
		return true
	}
	for index := 0; index < len(params); index++ {
		code := params[index].Param(0)
		if params[index].HasMore() {
			last := index
			for params[last].HasMore() {
				last++
				if last == len(params) {
					return false
				}
			}
			switch code {
			case 4:
				current.underline = params[index+1].Param(0) != 0
			case 5, 58: // overline as tmux writes it; underline colour
			default:
				return false
			}
			index = last
			continue
		}
		switch {
		case code == 0:
			*current = style{}
		case code == 1:
			current.bold = true
		case code == 2:
			current.dim = true
		case code == 3:
			current.italic = true
		case code == 4:
			current.underline = true
		case code == 7:
			current.reverse = true
		case code == 22:
			current.bold, current.dim = false, false
		case code == 23:
			current.italic = false
		case code == 24:
			current.underline = false
		case code == 27:
			current.reverse = false
		case code == 5, code == 8, code == 9, code == 25, code == 28, code == 29, code == 53, code == 55, code == 59:
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
		case " ", " ", "":
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
	upper, lower := screen.rows[first].region, screen.rows[last].region
	if upper == "" || lower == "" {
		panic("rule decided by an absent row") // justify-defect: grammars decide only on captured rows.
	}
	region := DiagnosticCompound
	switch {
	case upper != lower:
	case upper == tmuxclient.RegionTop:
		region = DiagnosticTop
	case upper == tmuxclient.RegionBottom:
		region = DiagnosticBottom
	default:
		panic("unknown screen region") // justify-defect: tmux has exactly two region kinds.
	}
	return DiagnosticRule{ID: id, Region: region}
}
