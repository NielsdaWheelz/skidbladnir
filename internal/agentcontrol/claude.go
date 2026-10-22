package agentcontrol

import (
	"regexp"
	"strconv"
	"strings"
)

// detectClaude reads a claude 2.1.286 screen, and 2.1.284's numbered theme
// picker, by the frozen grammar of docs/terminal-observation-claude.md sections
// 2-3, whose section numbers the comments below cite. It finds the lowest drawn
// row and tries the surfaces in order: the composer, then the request and menu
// families.
// Idle comes only from a complete current ready layout; a request or menu
// never reports activity, because it replaces the spinner, pill and panel.
func detectClaude(parsed screen) reading {
	screen := newClaudeScreen(parsed)
	last := len(screen.lines) - 1
	for last >= screen.floor && screen.lines[last].blank() {
		last--
	}
	if last < screen.floor {
		if screen.floor > 0 {
			// The captured rows are blank: the lowest drawn row lies in the cut.
			return screen.clipped()
		}
		return unknownReading(causeUnrecognized)
	}
	if read, ok := claudeComposer(screen, last); ok {
		return read
	}
	if read, ok := claudeFamily(screen, last); ok {
		return read
	}
	if screen.cut {
		return screen.clipped()
	}
	return unknownReading(causeUnrecognized)
}

// claudeScreen is a screen as the claude grammar reads it. Rows floor..end
// were captured: every read moves up from the lowest drawn row, so a read that
// needs a row above floor meets the cut, and its evidence is clipped.
type claudeScreen struct {
	screen
	lines []claudeLine
	floor int
	cut   bool // some read met the cut
}

// claudeLine is a row with the plain text and first column that every column
// test reads.
type claudeLine struct {
	row
	plain string // the text, NBSP as a space, without trailing spaces
	col   int    // the first drawn column; -1 for a blank or unparseable row
}

func newClaudeLine(row row) claudeLine {
	line := claudeLine{row: row, col: -1}
	if row.presence == rowParsed {
		line.plain = strings.TrimRight(strings.ReplaceAll(cellText(row.cells), "\u00a0", " "), " ")
		if line.plain != "" {
			line.col = len(line.plain) - len(strings.TrimLeft(line.plain, " "))
		}
	}
	return line
}

func newClaudeScreen(parsed screen) *claudeScreen {
	screen := &claudeScreen{screen: parsed, lines: make([]claudeLine, len(parsed.rows)), floor: len(parsed.rows)}
	for screen.floor > 0 && parsed.rows[screen.floor-1].presence != rowAbsent {
		screen.floor--
	}
	for r, row := range parsed.rows {
		screen.lines[r] = newClaudeLine(row)
	}
	return screen
}

// line returns row r to a read moving up the screen. It is false above row 0
// and at the cut, which it records.
func (screen *claudeScreen) line(r int) (claudeLine, bool) {
	if r < screen.floor {
		screen.cut = screen.cut || r >= 0
		return claudeLine{}, false
	}
	return screen.lines[r], true
}

// clippedAt reports a row inside the cut rather than above the pane.
func (screen *claudeScreen) clippedAt(r int) bool {
	return r >= 0 && r < screen.floor
}

// blankAt reports a readable blank row.
func (screen *claudeScreen) blankAt(r int) bool {
	line, ok := screen.line(r)
	return ok && line.blank()
}

// find returns the first row at or above from that matches.
func (screen *claudeScreen) find(from int, match func(claudeLine) bool) (int, bool) {
	for r := from; ; r-- {
		line, ok := screen.line(r)
		if !ok {
			return 0, false
		}
		if match(line) {
			return r, true
		}
	}
}

// clipped is the reading of a screen whose deciding rows were not captured.
func (screen *claudeScreen) clipped() reading {
	read := unknownReading(causeClipped)
	read.rules = []string{"claude.region.clipped"}
	return read
}

// ruleRow is a row of `─` exactly the pane width wide.
func (screen *claudeScreen) ruleRow(line claudeLine) bool {
	if len(line.cells) != screen.width {
		return false
	}
	for _, cell := range line.cells {
		if cell.text != "─" {
			return false
		}
	}
	return true
}

// banner is a full-width rule carrying a right-aligned label: a named,
// coloured or agent session's top rule.
func (screen *claudeScreen) banner(line claudeLine) bool {
	if len(line.cells) != screen.width {
		return false
	}
	head := strings.TrimLeft(line.plain, "─")
	label, found := strings.CutSuffix(head, " ─")
	label, spaced := strings.CutPrefix(label, " ")
	return head != line.plain && found && spaced && label != "" && strings.TrimSpace(label) == label
}

// leftLabel is a full-width rule labelled at its left: history recall.
func (screen *claudeScreen) leftLabel(line claudeLine) bool {
	return len(line.cells) == screen.width && strings.HasPrefix(line.plain, "── ") && strings.HasSuffix(line.plain, "─") && !screen.banner(line)
}

// edge is a full-width row of `▔` that may embed a notification.
func (screen *claudeScreen) edge(line claudeLine) bool {
	return len(line.cells) == screen.width && line.cells[0].text == "▔"
}

// claudeBoxRow holds only rule glyphs.
func claudeBoxRow(line claudeLine) bool {
	return line.plain != "" && strings.Trim(line.plain, "─╌▔ ") == ""
}

// claudeMarker starts an option: a pointer, a number or a checkbox.
var claudeMarker = regexp.MustCompile(`^ *(?:❯|\d+\. |\[[ ✔]\] )`)

// claudeDigits counts the ASCII digits text starts with.
func claudeDigits(text string) int {
	return len(text) - len(strings.TrimLeft(text, "0123456789"))
}

// claudeNotification is a right-aligned notification or one of claude's two
// notices about the tmux it runs in, at column 2: the only rows the composer
// margin and footer tail may hold.
func claudeNotification(line claudeLine) bool {
	return line.col >= 8 || line.col == 2 && (strings.HasPrefix(line.plain, "  tmux detected · ") || strings.HasPrefix(line.plain, "  tmux focus-events off · "))
}

// blockTop is the top row of the logical block at column c ending at row end
// (2.0, blk): the rows from end upward that are drawn, within one
// column of c, and neither an option marker nor a box row. It is end+1 when
// end itself does not belong.
func (screen *claudeScreen) blockTop(c, end int) int {
	top := end + 1
	for {
		line, ok := screen.line(top - 1)
		if !ok || line.presence != rowParsed || line.blank() || line.col < c-1 || line.col > c+1 || claudeMarker.MatchString(line.plain) || claudeBoxRow(line) {
			return top
		}
		top--
	}
}

// block is the text of a hint or anchor block at column c ending at row end
// (2.0): it holds only when its top row is at exactly c, which keeps
// transcript text, drawn from column 2, out of lower columns.
func (screen *claudeScreen) block(c, end int) (text string, top int, ok bool) {
	top = screen.blockTop(c, end)
	if top > end || screen.lines[top].col != c {
		return "", 0, false
	}
	return screen.joined(top, end), top, true
}

// joined is the texts of rows top..end without their indent, joined by one
// space: how a wrapped block reads, since claude wraps at word boundaries. A
// blank row adds an empty text.
func (screen *claudeScreen) joined(top, end int) string {
	texts := make([]string, 0, end-top+1)
	for r := top; r <= end; r++ {
		texts = append(texts, screen.lines[r].plain[max(screen.lines[r].col, 0):])
	}
	return strings.Join(texts, " ")
}

// above is the block at c ending at the first drawn row above row (2.0): any
// column within one of c may top it.
func (screen *claudeScreen) above(c, row int) (text string, top, end int, ok bool) {
	end, found := screen.find(row-1, func(line claudeLine) bool { return !line.blank() })
	if !found {
		return "", 0, 0, false
	}
	top = screen.blockTop(c, end)
	if top > end {
		return "", 0, 0, false
	}
	return screen.joined(top, end), top, end, true
}

// textBlock reports whether a block at and above end, whose top row is at
// exactly c, matches.
func (screen *claudeScreen) textBlock(c, end int, match func(string) bool) bool {
	for r := end; ; r-- {
		line, ok := screen.line(r)
		if !ok {
			return false
		}
		if line.blank() || line.col < c-1 || line.col > c+1 {
			continue
		}
		if text, top, ok := screen.block(c, r); ok {
			if match(text) {
				return true
			}
			r = top
		}
	}
}

type claudeOption struct {
	label    string
	selected bool
}

// claudeOptionStart reads `^ {c}(❯| ) N\. label`.
func claudeOptionStart(plain string, c int) (number int, label string, selected, ok bool) {
	if len(plain) < c || strings.TrimLeft(plain[:c], " ") != "" {
		return 0, "", false, false
	}
	rest := plain[c:]
	if after, found := strings.CutPrefix(rest, "❯ "); found {
		rest, selected = after, true
	} else if after, found := strings.CutPrefix(rest, "  "); found {
		rest = after
	} else {
		return 0, "", false, false
	}
	digits := claudeDigits(rest)
	label, found := strings.CutPrefix(rest[digits:], ". ")
	if digits == 0 || digits > 2 || !found || label == "" {
		return 0, "", false, false
	}
	number, _ = strconv.Atoi(rest[:digits]) // justify-ignore-error: one or two ASCII digits always parse.
	return number, label, selected, true
}

// claudeParseOptions reads a top-down run of option rows at column c: option
// starts numbered from 1, each followed by continuation rows from column c+5,
// with at most one pointer.
func claudeParseOptions(lines []claudeLine, c int) ([]claudeOption, bool) {
	var options []claudeOption
	selected := 0
	for _, line := range lines {
		number, label, pointer, start := claudeOptionStart(line.plain, c)
		switch {
		case start && number == len(options)+1:
			options = append(options, claudeOption{label, pointer})
			if pointer {
				selected++
			}
		case !start && line.col >= c+5 && len(options) > 0:
			options[len(options)-1].label += " " + line.plain[line.col:]
		default:
			return nil, false
		}
	}
	return options, len(options) > 0 && selected <= 1
}

// options reads the numbered options at column c (2.0): past blank
// rows at and above from, the contiguous option rows.
func (screen *claudeScreen) options(c, from int) (options []claudeOption, top int, ok bool) {
	bottom := from
	for screen.blankAt(bottom) {
		bottom--
	}
	top = bottom + 1
	for {
		line, readable := screen.line(top - 1)
		if !readable || line.presence != rowParsed || line.blank() {
			break
		}
		if _, _, _, start := claudeOptionStart(line.plain, c); !start && line.col < c+5 {
			break
		}
		top--
	}
	if top > bottom {
		return nil, 0, false
	}
	options, ok = claudeParseOptions(screen.lines[top:bottom+1], c)
	return options, top, ok
}

// unnumbered reads the unnumbered options at column c (2.0): past
// blank rows at and above from, the contiguous option rows (labels at c+2, at
// most one pointer at c), joined. A wrapped label continues where an
// unselected option starts, so only the whole text is meaningful.
func (screen *claudeScreen) unnumbered(c, from int) (text string, top int, ok bool) {
	bottom := from
	for screen.blankAt(bottom) {
		bottom--
	}
	top, selected := bottom+1, 0
	for {
		line, readable := screen.line(top - 1)
		pointer := readable && line.col == c && strings.HasPrefix(line.plain[c:], "❯ ") && !strings.HasPrefix(line.plain[c:], "❯  ")
		if !readable || !pointer && line.col != c+2 {
			break
		}
		if pointer {
			selected++
		}
		top--
	}
	if top > bottom || selected > 1 {
		return "", 0, false
	}
	return screen.joined(top, bottom), top, true
}

// titled finds a dialog title above row: the first drawn row left of
// column 2 reads exactly title and has a full rule directly above it.
func (screen *claudeScreen) titled(row int, title string) bool {
	found, ok := screen.find(row-1, func(line claudeLine) bool { return !line.blank() && line.col < 2 })
	if !ok || screen.lines[found].plain != title {
		return false
	}
	rule, ok := screen.line(found - 1)
	return ok && screen.ruleRow(rule)
}

// claudeWords reports phrase in text between word boundaries.
func claudeWords(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}
