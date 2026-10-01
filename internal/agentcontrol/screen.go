package agentcontrol

import (
	"strconv"
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
	// tmux wrote: trailing spaces, even styled ones, may be absent, and cell()
	// reads past the end as a default space. Columns are measured by grapheme
	// cluster while tmux places cells per code point, so after some authored
	// clusters the rest of the row sits a column off tmux's. A grammar anchors
	// columns only on chrome left of authored text and never requires an exact
	// cell count on a row that can hold authored text.
	cells []cell
}

type presence uint8

const (
	rowAbsent      presence = iota // dropped by its region's byte or row limit
	rowUnparseable                 // holds something tmux's capture writer never writes
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
	for _, region := range observation.Regions {
		for index, text := range region.Rows {
			parsed.rows[region.FirstRow+index] = parseRow(region.Kind, observation.Width, text)
		}
	}
	return parsed
}

// parseRow reads one row as tmux's capture-pane -e writes it: graphemes, tab
// cells (whose last stop is the pane width's last column), SGR, OSC 8
// hyperlinks and the SO/SI charset shifts. Anything else, such as another
// escape or control byte, or an SGR or hyperlink outside tmux's writer, makes
// the row unparseable.
func parseRow(region tmuxclient.RegionKind, width int, text string) row {
	parsed := row{presence: rowParsed, region: region}
	unparseable := row{presence: rowUnparseable, region: region}
	var current style
	for text != "" {
		switch {
		case strings.HasPrefix(text, "\x1b]8;"):
			// tmux writes a hyperlink ST-terminated, its URI's UTF-8 raw and no C0
			// control or DEL inside (it drops or octal-escapes them), so the first
			// ST ends it and a control before it means the writer changed.
			link, after, found := strings.Cut(text[len("\x1b]8;"):], "\x1b\\")
			if !found || strings.ContainsFunc(link, func(r rune) bool { return r < ' ' || r == ansi.DEL }) {
				return unparseable
			}
			text = after
		case strings.HasPrefix(text, "\x1b["):
			// Only an SGR ends at its first m: any other CSI leaves its final byte
			// inside a parameter, which applySGR refuses.
			params, after, found := strings.Cut(text[len("\x1b["):], "m")
			if !found || !applySGR(&current, strings.Split(params, ";")) {
				return unparseable
			}
			text = after
		case text[0] == '\t':
			// tmux 3.7c (not 3.4) stores a tab over blank cells as one tab cell,
			// written as TAB, that spans to the next default tab stop or the last
			// column. Stops stay every 8 columns: neither provider sets them.
			for range min(len(parsed.cells)/8*8+8, width-1) - len(parsed.cells) {
				parsed.cells = append(parsed.cells, cell{text: " ", style: current})
			}
			text = text[1:]
		case text[0] == '\x0e' || text[0] == '\x0f':
			text = text[1:]
		case text[0] < ' ' || text[0] == ansi.DEL:
			return unparseable
		default:
			// Rows are valid UTF-8, so the text starts a printable grapheme.
			grapheme, columns, size, _ := ansi.DecodeSequence(text, ansi.NormalState, nil)
			text = text[size:]
			if columns == 0 {
				// A zero-width grapheme joins the cell before it, as in tmux.
				if last := len(parsed.cells) - 1; last >= 0 {
					for parsed.cells[last].text == "" {
						last--
					}
					parsed.cells[last].text += grapheme
				}
				continue
			}
			parsed.cells = append(parsed.cells, cell{text: grapheme, style: current})
			for range columns - 1 {
				parsed.cells = append(parsed.cells, cell{style: current})
			}
		}
	}
	return parsed
}

// applySGR folds one SGR's parameters into current. It reads what tmux's
// grid_string_cells_code writes: the literal attribute cases below and decimal
// colour numbers. Anything else, including an empty or missing parameter,
// refuses the row: tmux's writer changed.
func applySGR(current *style, params []string) bool {
	for index := 0; index < len(params); index++ {
		switch params[index] {
		case "0":
			*current = style{}
		case "1":
			current.bold = true
		case "2":
			current.dim = true
		case "7":
			current.reverse = true
		case "3", "4", "5", "8", "9", "4:2", "4:3", "4:4", "4:5", "5:3":
			// italic, underline, blink, conceal, strikethrough, the underline
			// styles and overline: no grammar reads them.
		case "39":
			current.fg = color{}
		case "49":
			current.bg = color{}
		case "38", "48", "58":
			// tmux follows a colour code with 5;n or 2;r;g;b, each 0-255.
			var value color
			var channels []string
			switch rest := params[index+1:]; {
			case len(rest) >= 2 && rest[0] == "5":
				value.kind, channels = colorPalette, rest[1:2]
			case len(rest) >= 4 && rest[0] == "2":
				value.kind, channels = colorRGB, rest[1:4]
			default:
				return false
			}
			for _, channel := range channels {
				number, err := strconv.ParseUint(channel, 10, 8)
				if err != nil {
					return false
				}
				value.value = value.value<<8 | uint32(number)
			}
			switch params[index] {
			case "38":
				current.fg = value
			case "48":
				current.bg = value
			case "58":
				// underline colour: no grammar reads it.
			}
			index += 1 + len(channels)
		default:
			code, err := strconv.ParseUint(params[index], 10, 8)
			switch {
			case err != nil:
				return false
			case code >= 30 && code <= 37:
				current.fg = color{kind: colorPalette, value: uint32(code - 30)}
			case code >= 90 && code <= 97:
				current.fg = color{kind: colorPalette, value: uint32(code - 90 + 8)}
			case code >= 40 && code <= 47:
				current.bg = color{kind: colorPalette, value: uint32(code - 40)}
			case code >= 100 && code <= 107:
				current.bg = color{kind: colorPalette, value: uint32(code - 100 + 8)}
			default:
				return false
			}
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
