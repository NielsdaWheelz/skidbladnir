package agentcontrol

import (
	"math/bits"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// detectClaude reads a claude 2.1.286 screen by the frozen grammar of
// research/claude.md sections 2-3. It finds the lowest drawn row and tries
// the surfaces in order: screen-reader startup, the screen-reader input row,
// a screen-reader request, the composer, then the request and menu families.
// Idle comes only from a complete current ready layout; a request or menu
// never reports activity, because it replaces the spinner, pill and panel.
func detectClaude(screen screen) reading {
	s := newClaudeScreen(screen)
	last := -1
	for r := len(s.lines) - 1; r >= s.floor && last < 0; r-- {
		if !s.lines[r].blank() {
			last = r
		}
	}
	if last < 0 {
		if s.floor > 0 {
			// The captured rows are blank: the lowest drawn row lies in the cut.
			return s.clipped()
		}
		return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}
	}
	if read, ok := claudeStartupSR(s, last); ok {
		return read
	}
	if read, ok := claudeSurfaceSR(s, last); ok {
		return read
	}
	if read, ok := claudeRequestSR(s, last); ok {
		return read
	}
	if read, ok := claudeComposer(s, last); ok {
		return read
	}
	if read, ok := claudeFamily(s, last); ok {
		return read
	}
	if s.cut {
		return s.clipped()
	}
	return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}
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

type claudeLine struct {
	parsed bool
	cells  []cell
	plain  string // the text, NBSP as a space, without trailing spaces
	col    int    // the first drawn column; -1 for a blank or unparseable row
}

func newClaudeScreen(screen screen) *claudeScreen {
	s := &claudeScreen{screen: screen, lines: make([]claudeLine, len(screen.rows)), floor: len(screen.rows)}
	for s.floor > 0 && screen.rows[s.floor-1].presence != rowAbsent {
		s.floor--
	}
	for r, row := range screen.rows {
		line := claudeLine{col: -1}
		if row.presence == rowParsed {
			line.parsed, line.cells = true, row.cells
			line.plain = strings.TrimRight(strings.ReplaceAll(row.text(), "\u00a0", " "), " ")
			if line.plain != "" {
				line.col = len(line.plain) - len(strings.TrimLeft(line.plain, " "))
			}
		}
		s.lines[r] = line
	}
	return s
}

// blank is a parsed row with nothing drawn; an unparseable row is never blank.
func (line claudeLine) blank() bool {
	return line.parsed && line.plain == ""
}

// line returns row r to a read moving up the screen. It is false above row 0
// and at the cut, which it records.
func (s *claudeScreen) line(r int) (claudeLine, bool) {
	if r < s.floor {
		s.cut = s.cut || r >= 0
		return claudeLine{}, false
	}
	return s.lines[r], true
}

// clippedAt reports a row inside the cut rather than above the pane.
func (s *claudeScreen) clippedAt(r int) bool {
	return r >= 0 && r < s.floor
}

// blankAt reports a readable blank row.
func (s *claudeScreen) blankAt(r int) bool {
	line, ok := s.line(r)
	return ok && line.blank()
}

// find returns the first row at or above from that matches.
func (s *claudeScreen) find(from int, match func(claudeLine) bool) (int, bool) {
	for r := from; ; r-- {
		line, ok := s.line(r)
		if !ok {
			return 0, false
		}
		if match(line) {
			return r, true
		}
	}
}

// clipped is the reading of a screen whose deciding rows were not captured.
func (s *claudeScreen) clipped() reading {
	return reading{
		activity: sessions.ActivityUnknown, activityCause: causeClipped,
		interaction: sessions.InteractionUnknown, interactionCause: causeClipped,
		notice: sessions.NoticeNone, rules: []DiagnosticRule{s.clippedRule()},
	}
}

// clippedRule names the captured row just below the cut, or the bottom
// region, which always ends at the last row, when it kept no row at all.
func (s *claudeScreen) clippedRule() DiagnosticRule {
	rule := DiagnosticRule{Region: DiagnosticBottom}
	if s.floor < len(s.lines) {
		rule = s.rule("", s.floor, s.floor)
	}
	rule.ID = "claude.region.clipped"
	return rule
}

// backgroundRule names the row whose background work makes the pane working.
func (s *claudeScreen) backgroundRule(row int) DiagnosticRule {
	return s.rule("claude.activity.background", row, row)
}

// composerRule names a composer value that has its own rule.
func (s *claudeScreen) composerRule(value composer, first, last int) DiagnosticRule {
	switch value {
	case composerEmpty:
		return s.rule("claude.composer.empty", first, last)
	case composerDraft:
		return s.rule("claude.composer.draft", first, last)
	case composerBlocked:
		return s.rule("claude.composer.blocked", first, last)
	default:
		panic("claude composer without a rule") // justify-defect: callers name only empty, draft or blocked.
	}
}

// full is a row of exactly the pane width drawn in one glyph: a RULE of `─`.
func (s *claudeScreen) full(line claudeLine, glyph string) bool {
	if len(line.cells) != s.width {
		return false
	}
	for _, cell := range line.cells {
		if cell.text != glyph {
			return false
		}
	}
	return true
}

// banner is a full-width rule carrying a right-aligned label: a named,
// coloured or agent session's top rule.
func (s *claudeScreen) banner(line claudeLine) bool {
	if len(line.cells) != s.width {
		return false
	}
	head := strings.TrimLeft(line.plain, "─")
	label, found := strings.CutSuffix(head, " ─")
	label, spaced := strings.CutPrefix(label, " ")
	return head != line.plain && found && spaced && label != "" && strings.TrimSpace(label) == label
}

// leftLabel is a full-width rule labelled at its left: history recall.
func (s *claudeScreen) leftLabel(line claudeLine) bool {
	return len(line.cells) == s.width && strings.HasPrefix(line.plain, "── ") && strings.HasSuffix(line.plain, "─") && !s.banner(line)
}

// edge is a full-width row of `▔` that may embed a notification.
func (s *claudeScreen) edge(line claudeLine) bool {
	return len(line.cells) == s.width && line.cells[0].text == "▔"
}

// boxRow holds only rule glyphs.
func claudeBoxRow(line claudeLine) bool {
	return line.parsed && line.plain != "" && strings.Trim(line.plain, "─╌▔ ") == ""
}

// marker starts an option: a pointer, a number or a checkbox.
func claudeMarker(line claudeLine) bool {
	text := line.plain[max(line.col, 0):]
	digits := len(text) - len(strings.TrimLeft(text, "0123456789"))
	return strings.HasPrefix(text, "❯") || digits > 0 && strings.HasPrefix(text[digits:], ". ") ||
		strings.HasPrefix(text, "[ ] ") || strings.HasPrefix(text, "[✔] ")
}

// claudeNotification is a right-aligned notification or a tmux notice at
// column 2: the only rows the composer margin and footer tail may hold.
func claudeNotification(line claudeLine) bool {
	return line.col >= 8 || line.col == 2 &&
		(strings.HasPrefix(line.plain, "  tmux detected · ") || strings.HasPrefix(line.plain, "  tmux focus-events off · "))
}

// block is blk(c, end): the rows from end upward that are drawn, within one
// column of c, and neither an option marker nor a box row, their stripped
// texts joined by one space. It holds only when its top row is at exactly c,
// which keeps transcript text, drawn from column 2, out of lower columns.
func (s *claudeScreen) block(c, end int) (text string, top int, ok bool) {
	top = end + 1
	for {
		line, readable := s.line(top - 1)
		if !readable || !line.parsed || line.blank() || line.col < c-1 || line.col > c+1 || claudeMarker(line) || claudeBoxRow(line) {
			break
		}
		top--
	}
	if top > end || s.lines[top].col != c {
		return "", 0, false
	}
	texts := make([]string, 0, end-top+1)
	for r := top; r <= end; r++ {
		texts = append(texts, s.lines[r].plain[s.lines[r].col:])
	}
	return strings.Join(texts, " "), top, true
}

// above is the block at c ending at the first drawn row above row.
func (s *claudeScreen) above(c, row int) (text string, top, end int, ok bool) {
	end, found := s.find(row-1, func(line claudeLine) bool { return !line.blank() })
	if !found {
		return "", 0, 0, false
	}
	text, top, ok = s.block(c, end)
	return text, top, end, ok
}

// claudeBlockText is one texts(c) block.
type claudeBlockText struct {
	text string
	top  int
}

// texts is texts(c) over the rows at and above end: every block whose top row
// is at exactly c.
func (s *claudeScreen) texts(c, end int) []claudeBlockText {
	var blocks []claudeBlockText
	for r := end; ; r-- {
		line, ok := s.line(r)
		if !ok {
			return blocks
		}
		if line.blank() || line.col < c-1 || line.col > c+1 {
			continue
		}
		if text, top, ok := s.block(c, r); ok {
			blocks = append(blocks, claudeBlockText{text, top})
			r = top
		}
	}
}

// claudeTextBlock finds the topmost block of blocks that matches.
func claudeTextBlock(blocks []claudeBlockText, match func(string) bool) (int, bool) {
	for index := len(blocks) - 1; index >= 0; index-- {
		if match(blocks[index].text) {
			return blocks[index].top, true
		}
	}
	return 0, false
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
	digits := len(rest) - len(strings.TrimLeft(rest, "0123456789"))
	label, found := strings.CutPrefix(rest[digits:], ". ")
	if digits == 0 || digits > 2 || !found || label == "" {
		return 0, "", false, false
	}
	for _, digit := range rest[:digits] {
		number = number*10 + int(digit-'0')
	}
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

// options is opt(c, from): past blank rows at and above from, the contiguous
// option rows at column c.
func (s *claudeScreen) options(c, from int) (options []claudeOption, top int, ok bool) {
	bottom := from
	for s.blankAt(bottom) {
		bottom--
	}
	top = bottom + 1
	for {
		line, readable := s.line(top - 1)
		if !readable || !line.parsed || line.blank() {
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
	options, ok = claudeParseOptions(s.lines[top:bottom+1], c)
	return options, top, ok
}

// unnumbered is uopt(c, from): past blank rows at and above from, the
// contiguous unnumbered option rows (labels at c+2, at most one pointer at c),
// joined. A wrapped label continues where an unselected option starts, so
// only the whole text is meaningful.
func (s *claudeScreen) unnumbered(c, from int) (text string, top int, ok bool) {
	bottom := from
	for s.blankAt(bottom) {
		bottom--
	}
	top, selected := bottom+1, 0
	for {
		line, readable := s.line(top - 1)
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
	texts := make([]string, 0, bottom-top+1)
	for r := top; r <= bottom; r++ {
		texts = append(texts, s.lines[r].plain[s.lines[r].col:])
	}
	return strings.Join(texts, " "), top, true
}

// titled finds a dialog title above row: the first drawn row left of
// column 2 reads exactly title and has a full rule directly above it. It
// returns the rule's row.
func (s *claudeScreen) titled(row int, title string) (int, bool) {
	found, ok := s.find(row-1, func(line claudeLine) bool { return !line.blank() && line.col < 2 })
	if !ok || s.lines[found].plain != title {
		return 0, false
	}
	rule, ok := s.line(found - 1)
	return found - 1, ok && s.full(rule, "─")
}

// claudeWords reports phrase in text between word boundaries.
func claudeWords(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

// claudeStartupSR is the screen reader's startup quiet: its banner is the
// lowest row until the first layout mounts.
func claudeStartupSR(s *claudeScreen, last int) (reading, bool) {
	switch s.lines[last].plain {
	case "[Screen Reader Mode: on via flag]", "[Screen Reader Mode: on via env]",
		"[Screen Reader Mode: on via settings]", "[Screen Reader Mode: on]":
		return reading{
			activity: sessions.ActivityStarting, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone,
			rules: []DiagnosticRule{s.rule("claude.activity.starting_sr", last, last)},
		}, true
	}
	return reading{}, false
}

// claudeSurfaceSR reads the screen reader's input row `$`, the notification
// blocks above it and its flattened mode row M′ (research 2.5).
func claudeSurfaceSR(s *claudeScreen, last int) (reading, bool) {
	input := s.lines[last]
	if !input.parsed || input.cells[0].text != "$" || len(input.cells) > 1 && input.cells[1].text != "\u00a0" {
		return reading{}, false
	}
	mode := last - 1
	for {
		line, ok := s.line(mode)
		if !ok {
			break
		}
		if claudeEffortSR(line.plain) {
			mode--
			continue
		}
		// A tmux notice wraps over at most three rows.
		notice := 0
		for rows, joined := 1, line.plain; notice == 0; rows++ {
			if joined == claudeTmuxNotices[0] || joined == claudeTmuxNotices[1] {
				notice = rows
				break
			}
			upper, ok := s.line(mode - rows)
			if !ok || rows == 3 {
				break
			}
			joined = upper.plain + " " + joined
		}
		if notice == 0 {
			break
		}
		mode -= notice
	}
	line, ok := s.line(mode)
	first := mode
	if !ok {
		first++ // only the notification rows were read
	}
	layout := s.rule("claude.layout.sr", first, last)
	unknown := reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone,
		rules: []DiagnosticRule{layout}}
	if !ok && s.clippedAt(mode) {
		unknown = s.clipped()
		unknown.rules = append(unknown.rules, layout)
	}
	parsed, parsedOK := claudeModeSR(line.plain)
	if !ok || !parsedOK {
		return unknown, true
	}
	var classes uint16
	for _, segment := range parsed.segments {
		class, _ := claudeExactSegment(segment)
		classes |= 1 << class
	}
	if classes&(1<<claudeUnlisted) != 0 {
		return unknown, true
	}

	read := reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionNone, notice: sessions.NoticeNone, composer: composerDraft}
	if input.plain == "$" {
		read.composer = composerEmpty
	}
	var activity DiagnosticRule
	switch {
	case classes&(1<<claudeWork) != 0:
		read.activity, activity = sessions.ActivityWorking, s.backgroundRule(mode)
	case classes&(1<<claudeInterrupt) != 0:
		read.activity, activity = sessions.ActivityWorking, s.rule("claude.activity.hint_sr", mode, mode)
	case classes&(1<<claudeUnknownPill) == 0 && read.composer == composerEmpty:
		above, ok := s.line(mode - 1)
		switch {
		case ok && (claudeCompletion(above.plain) || strings.HasPrefix(above.plain, "claude: ")):
			read.activity, activity = sessions.ActivityIdle, s.rule("claude.activity.idle_sr", mode-1, last)
		case s.clippedAt(mode - 1):
			read.activityCause = causeClipped
		}
	}
	if read.activityCause == causeClipped {
		read.rules = append(read.rules, s.clippedRule())
	}
	read.rules = append(read.rules, layout)
	if activity.ID != "" {
		read.rules = append(read.rules, activity)
	}
	read.rules = append(read.rules, s.composerRule(read.composer, last, last))
	return read, true
}

var claudeTmuxNotices = [2]string{
	"tmux detected · scroll with PgUp/PgDn · or add 'set -g mouse on' to ~/.tmux.conf for wheel scroll",
	"tmux focus-events off · add 'set -g focus-events on' to ~/.tmux.conf and reattach for focus tracking",
}

// claudeEffortSR is the screen reader's effort notification row.
func claudeEffortSR(plain string) bool {
	level, found := strings.CutPrefix(plain, "effort: ")
	level, ends := strings.CutSuffix(level, " · /effort")
	level = strings.TrimSuffix(level, " · ultracode")
	switch level {
	case "low", "medium", "high", "xhigh", "max":
		return found && ends
	}
	return false
}

// claudeRequestSR reads a screen-reader request: its request row is the
// lowest row, or the row above a trailing hint or theme preview, and its kind
// comes from anchor rows above it. Claude draws the request row only while a
// request is live, so a quoted anchor can only change the kind.
func claudeRequestSR(s *claudeScreen, last int) (reading, bool) {
	request := last
	if plain := s.lines[last].plain; plain == "Esc to cancel · Tab to amend" || plain == "Enter to confirm · Esc to cancel" ||
		strings.HasPrefix(plain, " Syntax theme: ") {
		request--
	}
	line, ok := s.line(request)
	if !ok || !claudeRequestRowSR(line.plain) {
		return reading{}, false
	}
	setup, permission, question, proceed := -1, -1, -1, false
	for r := request - 1; ; r-- {
		line, ok := s.line(r)
		if !ok {
			break
		}
		title, required := strings.CutPrefix(line.plain, "Permission Required: ")
		for _, anchor := range [...]string{"Accessing workspace:", "Detected a custom API key in your environment",
			"WARNING: Claude Code running in Bypass Permissions mode", "New MCP server found in this project: ",
			"Select login method:", "Let's get started.", "Security notes:", "Settings Error"} {
			if strings.HasPrefix(title, anchor) {
				setup = r
			}
		}
		if required {
			permission = r
		}
		proceed = proceed || strings.HasPrefix(line.plain, "Do you want to ") && strings.HasSuffix(line.plain, "?") && len(line.plain) > len("Do you want to ?")
		// A question's tab row `←  ☐ …`, its header ` ☐ …`, or its `> …?` prompt.
		tabs, tab := strings.CutPrefix(line.plain, "←")
		boxes := strings.TrimLeft(tabs, " ")
		header, spaced := strings.CutPrefix(line.plain, " ")
		if tab && boxes != tabs && (strings.HasPrefix(boxes, "☐") || strings.HasPrefix(boxes, "☒") || strings.HasPrefix(boxes, "✔")) ||
			spaced && claudeCheckbox(header) ||
			strings.HasPrefix(line.plain, "> ") && strings.HasSuffix(line.plain, "?") && len(line.plain) > len("> ?") {
			question = r
		}
	}
	var id string
	var interaction sessions.Interaction
	var first int
	switch {
	case setup >= 0:
		id, interaction, first = "claude.setup.sr", sessions.InteractionSetup, setup
	case permission >= 0 && proceed:
		id, interaction, first = "claude.permission.sr", sessions.InteractionPermission, permission
	case question >= 0:
		id, interaction, first = "claude.question.sr", sessions.InteractionQuestion, question
	case s.cut:
		return s.clipped(), true
	default:
		return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}, true
	}
	return reading{
		activity: sessions.ActivityUnknown, interaction: interaction, notice: sessions.NoticeNone, composer: composerBlocked,
		rules: []DiagnosticRule{s.rule(id, first, last), s.composerRule(composerBlocked, first, last)},
	}, true
}

// claudeCheckbox starts with a question tab's checkbox followed by a space
// and a label.
func claudeCheckbox(text string) bool {
	for _, box := range [...]string{"☐ ", "☒ ", "✔ "} {
		if label, found := strings.CutPrefix(text, box); found && label != "" && label[0] != ' ' {
			return true
		}
	}
	return false
}

// claudeRequestRowSR is the screen reader's live request prompt.
func claudeRequestRowSR(plain string) bool {
	if rest, found := strings.CutPrefix(plain, "Select with numbers [1-"); found {
		digits := len(rest) - len(strings.TrimLeft(rest, "0123456789"))
		return digits > 0 && strings.HasPrefix(rest[digits:], "]")
	}
	if rest, found := strings.CutPrefix(plain, "Enter text for option "); found {
		digits := len(rest) - len(strings.TrimLeft(rest, "0123456789"))
		return digits > 0 && strings.HasPrefix(rest[digits:], " ")
	}
	return plain == "Enter y/n:" || plain == "Press Enter to continue…"
}

// claudeComposer reads the composer surface (research 2.3): the input row
// between a top and a bottom rule, the footer below them and the slot row
// above them. Only the first full rule at or above the lowest row is tried, so
// a dead process's frame above a newer surface never anchors it.
func claudeComposer(s *claudeScreen, last int) (reading, bool) {
	bottom := -1
	for r := last; r > last-30 && bottom < 0; r-- {
		line, ok := s.line(r)
		if !ok {
			return reading{}, false
		}
		if s.full(line, "─") {
			bottom = r
		}
	}
	if bottom < 0 {
		return reading{}, false
	}
	chevron := bottom - 1
	for {
		line, ok := s.line(chevron)
		if !ok {
			return reading{}, false
		}
		if !line.blank() && line.col < 2 {
			break
		}
		if chevron == bottom-41 {
			return reading{}, false // more than 40 continuation rows
		}
		chevron--
	}
	// A chevron is `❯` or bash mode's `!` followed by NBSP. A `❯` with a
	// background or an ordinary space is a user prompt in the transcript.
	prompt := s.lines[chevron]
	if !prompt.parsed {
		return reading{}, false
	}
	head := prompt.cells[0]
	nbsp := len(prompt.cells) > 1 && prompt.cells[1].text == "\u00a0"
	bash := head.text == "!" && nbsp
	if !bash && !(head.text == "❯" && head.style.bg.kind == colorDefault && (nbsp || len(prompt.cells) == 1)) {
		return reading{}, false
	}
	top := chevron - 1
	topRule, ok := s.line(top)
	if !ok {
		return reading{}, false
	}
	ordinary := s.full(topRule, "─") || s.banner(topRule)
	if !ordinary && !s.leftLabel(topRule) {
		return reading{}, false
	}

	layout := s.rule("claude.layout.composer", top, last)
	footer := claudeReadFooter(s, bottom, last, bash)
	if footer.kind == claudeFooterVoid {
		// Anything the grammar cannot account for may be drawn over this
		// surface: a dead process's frame, autocomplete, a focused panel.
		return reading{
			activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone,
			rules: []DiagnosticRule{layout},
		}, true
	}
	slot, kind, margin := claudeSlot(s, top)
	limitFirst, limitLast, limit := claudeLimitWait(s, slot, kind, margin, top, bottom, last)

	// The rule is promptBorder-coloured in every theme, so a rule without a
	// foreground means colour is off and the chevron's style says nothing.
	live := topRule.cells[0].style.fg.kind != colorDefault
	ready := !bash && live && head.style.fg.kind == colorDefault && !head.style.dim && !head.style.reverse
	loading := !bash && (head.style.fg.kind != colorDefault || head.style.dim)
	interrupt := footer.classes&(1<<claudeInterrupt) != 0
	corroborator := kind == claudeSlotSpinner || kind == claudeSlotRetry || interrupt

	read := reading{activity: sessions.ActivityUnknown, notice: sessions.NoticeNone}
	var activity DiagnosticRule
	switch {
	case footer.work >= 0 || kind == claudeSlotWait || footer.classes&(1<<claudeWork) != 0:
		// The panel, the waiting row and a work pill are independent of the
		// chevron and the slot, so they decide before any conflict.
		row := footer.work
		if row < 0 && kind == claudeSlotWait {
			row = slot
		} else if row < 0 {
			row = footer.mode
		}
		read.activity, activity = sessions.ActivityWorking, s.backgroundRule(row)
	case corroborator && ready:
		first, end := chevron, chevron
		if kind == claudeSlotSpinner || kind == claudeSlotRetry {
			first = slot
		}
		if interrupt {
			end = footer.mode
		}
		read.activityCause, activity = causeConflict, s.rule("claude.activity.conflict", first, end)
	case loading && kind == claudeSlotSpinner:
		read.activity, activity = sessions.ActivityWorking, s.rule("claude.activity.spinner", slot, chevron)
	case loading && kind == claudeSlotRetry:
		read.activity, activity = sessions.ActivityWorking, s.rule("claude.activity.retry", slot, chevron)
	case interrupt:
		read.activity, activity = sessions.ActivityWorking, s.rule("claude.activity.hint", chevron, footer.mode)
	case corroborator:
		// Spinner chrome under a chevron whose style says nothing: unknown.
	case kind == claudeSlotClipped:
		read.activityCause = causeClipped
	case ready && ordinary && footer.kind == claudeFooterOrdinary && !limit && !footer.extra && footer.proven &&
		footer.classes&^claudeTolerated == 0 && (kind == claudeSlotNone || kind == claudeSlotOther):
		first := top
		if kind == claudeSlotOther {
			first = slot
		}
		read.activity, activity = sessions.ActivityIdle, s.rule("claude.activity.idle", first, last)
	}

	var family DiagnosticRule
	switch {
	case !ordinary || limit:
		read.interaction = sessions.InteractionUnknown
		if limit {
			family = s.rule("claude.limit.wait", limitFirst, limitLast)
		}
	case footer.viewed >= 0:
		read.interaction, family = sessions.InteractionMenu, s.rule("claude.menu.subagent", footer.viewed, footer.viewed)
	case footer.kind == claudeFooterHelp:
		read.interaction, family = sessions.InteractionMenu, s.rule("claude.menu.help", bottom+1, last)
	default:
		read.interaction = sessions.InteractionNone
	}

	var composerRule DiagnosticRule
	switch {
	case read.interaction == sessions.InteractionMenu:
		read.composer, composerRule = composerBlocked, s.composerRule(composerBlocked, chevron, chevron)
	case read.interaction == sessions.InteractionUnknown:
	case kind == claudeSlotOther && claudeBand(s.lines[slot]):
		composerRule = s.rule("claude.composer.band", slot, slot)
	case bash:
		read.composer, composerRule = composerDraft, s.composerRule(composerDraft, chevron, bottom-1)
	default:
		read.composer = claudeInput(s, prompt, chevron, bottom)
		composerRule = s.composerRule(read.composer, chevron, bottom-1)
	}

	if read.activityCause == causeClipped {
		read.rules = append(read.rules, s.clippedRule())
	}
	read.rules = append(read.rules, layout)
	for _, rule := range [...]DiagnosticRule{family, activity, composerRule} {
		if rule.ID != "" {
			read.rules = append(read.rules, rule)
		}
	}
	return read, true
}

// claudeInput reads the ordinary composer's input row and its continuation
// rows: empty when only a drawn cursor or a dim placeholder follows the
// prompt. Placeholders are known by their dim style, never by their words.
func claudeInput(s *claudeScreen, prompt claudeLine, chevron, bottom int) composer {
	for r := chevron + 1; r < bottom; r++ {
		if !s.lines[r].blank() {
			return composerDraft
		}
	}
	cells := prompt.cells[min(2, len(prompt.cells)):]
	for len(cells) > 0 && cells[len(cells)-1].text == " " && !cells[len(cells)-1].style.reverse && cells[len(cells)-1].style.bg.kind == colorDefault {
		cells = cells[:len(cells)-1]
	}
	if len(cells) == 0 || len(cells) == 1 && cells[0].text == " " && cells[0].style.reverse {
		return composerEmpty
	}
	if cells[0].style.reverse && cells[0].text != " " {
		cells = cells[1:] // the cursor on the placeholder's first letter
	}
	if len(cells) == 0 {
		return composerDraft // the cursor on a typed character
	}
	for _, cell := range cells {
		if !cell.style.dim {
			return composerDraft
		}
	}
	return composerEmpty
}

type claudeSlotKind uint8

const (
	claudeSlotNone       claudeSlotKind = iota // nothing above the composer: the pane's top
	claudeSlotOther                            // a row that is not live spinner chrome
	claudeSlotSpinner                          // SP
	claudeSlotRetry                            // RT
	claudeSlotWait                             // WAIT, the end-of-turn row while launched work runs
	claudeSlotUnreadable                       // an unparseable row
	claudeSlotClipped                          // the walk met the cut
)

var (
	claudeSpinnerShape = regexp.MustCompile(`^[·✢✳✶✻✽*●] \S.*?(?:…|\.\.\.)(?: \(.*)?$`)
	claudeWaitShape    = regexp.MustCompile(`^✻ Waiting for(?: \d+ background agents?)?(?: and)?(?: \d+ dynamic workflows?)? to finish$`)
)

// claudeSlot walks up from the composer's top rule to the slot row S: past at
// most six notification rows in the composer's top margin, blank rows, and
// one spinner child block, which begins `⎿` at column 2 within 12 rows. A
// live spinner cannot be passed. margin is the first passed notification row.
func claudeSlot(s *claudeScreen, top int) (slot int, kind claudeSlotKind, margin int) {
	slot = top - 1
	for skipped := 0; skipped < 6; skipped++ {
		line, ok := s.line(slot)
		if !ok || !claudeNotification(line) {
			break
		}
		slot--
	}
	margin = slot + 1
	for s.blankAt(slot) {
		slot--
	}
	end := func(r int) (int, claudeSlotKind, int) {
		if s.clippedAt(r) {
			return r, claudeSlotClipped, margin
		}
		return r, claudeSlotNone, margin
	}
	line, ok := s.line(slot)
	if !ok {
		return end(slot)
	}
	if line.col >= 5 || line.col == 2 && strings.HasPrefix(line.plain, "  ⎿") {
		for child := slot; child > slot-12; child-- {
			row, ok := s.line(child)
			if !ok {
				if s.clippedAt(child) {
					return end(child)
				}
				break
			}
			if row.col == 2 && strings.HasPrefix(row.plain, "  ⎿") {
				slot = child - 1
				if line, ok = s.line(slot); !ok {
					return end(slot)
				}
				break
			}
			if row.col < 5 {
				break
			}
		}
	}
	// A narrow spinner wraps its details alone onto the row below it.
	if line.col == 0 && strings.HasPrefix(line.plain, "(") && strings.HasSuffix(line.plain, ")") {
		above, ok := s.line(slot - 1)
		switch {
		case !ok && s.clippedAt(slot-1):
			return end(slot - 1)
		case ok && claudeSpinnerShape.MatchString(above.plain) && (strings.HasSuffix(above.plain, "…") || strings.HasSuffix(above.plain, "...")):
			slot, line = slot-1, above
		}
	}
	plain := line.plain
	switch {
	case !line.parsed:
		return slot, claudeSlotUnreadable, margin
	case line.col != 0:
		return slot, claudeSlotOther, margin
	case claudeWaitShape.MatchString(plain) && strings.ContainsAny(plain, "0123456789"):
		return slot, claudeSlotWait, margin
	case strings.HasPrefix(plain, "✻ ") && claudeCompletion(plain[len("✻ "):]):
		return slot, claudeSlotOther, margin
	case claudeSpinnerShape.MatchString(plain):
		return slot, claudeSlotSpinner, margin
	case strings.HasPrefix(plain, "✻ ") && (strings.Contains(plain, " · Retrying in ") || strings.Contains(plain, " · will retry in ") ||
		strings.Contains(plain, " · next try in ") || strings.Contains(plain, "No response from the API after ")):
		return slot, claudeSlotRetry, margin
	default:
		return slot, claudeSlotOther, margin
	}
}

// claudeCompletion is a turn's duration row: one of claude's fixed past-tense
// verbs, then ` for `.
func claudeCompletion(text string) bool {
	verb, _, found := strings.Cut(text, " for ")
	switch verb {
	case "Baked", "Brewed", "Churned", "Cogitated", "Cooked", "Crunched", "Sautéed", "Worked":
		return found
	}
	return false
}

// claudeLimitWait finds the usage-limit wait's default copy where claude
// draws notifications: the slot row's block, the notifications passed in the
// composer's top margin, and the footer.
func claudeLimitWait(s *claudeScreen, slot int, kind claudeSlotKind, margin, top, bottom, last int) (first, end int, found bool) {
	limit := func(text string) bool {
		for _, copy := range [...]string{"Usage limit reached", "Your usage limit has reset", "Continuing automatically ",
			"Continuing shortly · esc to cancel", "Press enter to continue"} {
			if strings.Contains(text, copy) {
				return true
			}
		}
		return false
	}
	if kind != claudeSlotNone && kind != claudeSlotClipped && kind != claudeSlotUnreadable {
		if text, blockTop, ok := s.block(s.lines[slot].col, slot); ok && limit(text) {
			return blockTop, slot, true
		}
	}
	for r := margin; r < top; r++ {
		if limit(s.lines[r].plain) {
			return r, r, true
		}
	}
	for r := bottom + 1; r <= last; r++ {
		if limit(s.lines[r].plain) {
			return r, r, true
		}
	}
	return 0, 0, false
}

// claudeBand is the legend of an optional prompt drawn above the composer
// that takes a single key typed into it (research 2.3, digit band).
func claudeBand(line claudeLine) bool {
	wordPrefix := func(prefix string) bool {
		rest, found := strings.CutPrefix(line.plain, prefix)
		return found && (rest == "" || !strings.ContainsAny(rest[:1], "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_"))
	}
	switch line.col {
	case 2:
		return wordPrefix("  1: Bad") || wordPrefix("  y: Yes") || wordPrefix("  1: Yes, run /web-setup") ||
			wordPrefix("  Enter to send · Esc to clear") || wordPrefix("  Enter to send · Esc to skip") ||
			wordPrefix("  Enter to skip · Esc to clear") || wordPrefix("  Enter to skip · Esc to skip")
	case 0:
		switch line.plain {
		case "(Optional) Press [1] to tell us what went well · /feedback", "(Optional) Press [1] to tell us what went wrong · /feedback",
			"(Optional) Press [1] to tell us more · /feedback", "Send without reviewing (full draft + env, no transcript)? 2 to send · Esc to back",
			"Turn off Claude-drafted feedback? 0 to turn off · Esc to keep":
			return true
		}
		return strings.HasPrefix(line.plain, "1 to review · 2 to send · 0 to dismiss") ||
			strings.HasSuffix(line.plain, " 1 to review & retry · Esc to dismiss")
	}
	return false
}

type claudeFooterKind uint8

const (
	claudeFooterVoid claudeFooterKind = iota
	claudeFooterOrdinary
	claudeFooterBash
	claudeFooterHelp
)

// claudeFooter is what the rows below the composer's bottom rule account for.
type claudeFooter struct {
	kind    claudeFooterKind
	mode    int    // the mode row M of an ordinary footer
	classes uint16 // the classes of M's segments, one bit each
	proven  bool   // M proves that no task pill is hidden
	work    int    // a panel row with a running status, or -1
	viewed  int    // a viewed panel row other than main, or -1
	extra   bool   // a panel row other than main
}

var (
	claudePanelMain     = regexp.MustCompile(`^(?:  |❯ )[◯⏺●] main(?: +↑ \d+ more)?$`)
	claudePanelSummary  = regexp.MustCompile(`^(?:  |❯ )◯ \d+ idle agents?$`)
	claudePanelMore     = regexp.MustCompile(`^  ↓ \d+ more$`)
	claudePanelAgent    = regexp.MustCompile(`^(?:  |❯ )(?:(?:  )*[├└] )?([◯⏺●]) \S`)
	claudePanelWorkflow = regexp.MustCompile(`^(?:  |❯ )([◯⏸]) \S`)
	// A running agent or workflow shows its elapsed time, optional progress,
	// token and queue counts; idle, waiting and paused rows show no time.
	claudeRunningStatus = regexp.MustCompile(`\s(?:\d+/\d+ · )?\d+[dhms](?: ?\d+[dhms])*(?: · [↑↓] \S+ tokens)?(?: · \d+ queued)?$`)
)

// claudeReadFooter accounts for the rows below the bottom rule: an ordinary
// footer (opaque statusline rows, the mode row M, notification rows, then
// optionally one blank row and the agent panel), bash mode's footer, or the
// help grid. Anything else leaves the surface void.
func claudeReadFooter(s *claudeScreen, bottom, last int, bash bool) claudeFooter {
	void := claudeFooter{mode: -1, work: -1, viewed: -1}
	if bottom == last {
		return void
	}
	footer := void
	tail := bottom + 1
	first := s.lines[bottom+1].plain
	switch {
	case bash:
		if first != "  ! for shell mode" {
			return void
		}
		footer.kind, tail = claudeFooterBash, bottom+2
	case strings.HasPrefix(first, "  ! for shell mode "):
		// The help grid replaces the whole footer.
		for r := bottom + 1; r <= last; r++ {
			if strings.Contains(s.lines[r].plain, "/ for commands") {
				footer.kind = claudeFooterHelp
				return footer
			}
		}
		return void
	default:
		for r := last; r > bottom && footer.mode < 0; r-- {
			if mode, ok := claudeModeFull(s.lines[r], s.width); ok {
				footer.kind, footer.mode, tail = claudeFooterOrdinary, r, r+1
				footer.classes, footer.proven = claudeModeEvidence(mode)
			}
		}
		if footer.mode < 0 {
			return void
		}
	}
	r := tail
	for r <= last && claudeNotification(s.lines[r]) {
		r++
	}
	if r > last {
		return footer
	}
	if !s.lines[r].blank() {
		return void
	}
	gaps := 0
	for p := r + 1; p <= last; p++ {
		plain := s.lines[p].plain
		if s.lines[p].blank() {
			// One blank row may stand for the `more` row of a long panel.
			if gaps++; gaps > 1 || p == r+1 {
				return void
			}
			continue
		}
		footer.extra = footer.extra || !claudePanelMain.MatchString(plain)
		switch {
		case claudePanelMain.MatchString(plain), claudePanelSummary.MatchString(plain), claudePanelMore.MatchString(plain):
		case claudePanelAgent.MatchString(plain):
			glyph := claudePanelAgent.FindStringSubmatch(plain)[1]
			if glyph != "◯" && footer.viewed < 0 {
				footer.viewed = p
			}
			if footer.work < 0 && claudeRunningStatus.MatchString(plain) {
				footer.work = p
			}
		case claudePanelWorkflow.MatchString(plain):
			if footer.work < 0 && claudePanelWorkflow.FindStringSubmatch(plain)[1] != "⏸" && claudeRunningStatus.MatchString(plain) {
				footer.work = p
			}
		default:
			return void
		}
	}
	return footer
}

// claudeSegmentClass is a footer segment's class (research 2.4).
type claudeSegmentClass uint8

const (
	claudeUnlisted    claudeSegmentClass = iota // blocks idle
	claudeInterrupt                             // corroborates work
	claudeTail                                  // another hint item member
	claudeAgents                                // other sessions' agents: proves nothing
	claudeWork                                  // local agents running
	claudeProcess                               // a surviving task count: not work
	claudeUnknownPill                           // blocks idle, no work claim
	claudeIndicator                             // the footer indicator
	claudeOther                                 // PR status
)

// claudeTolerated are the segment classes idle allows.
const claudeTolerated = 1<<claudeTail | 1<<claudeAgents | 1<<claudeProcess | 1<<claudeIndicator | 1<<claudeOther

// claudeFooterLabels is the closed footer vocabulary: <chord> is one key
// token and N a count. hint marks the members of the hint item Hs, which
// truncates with `…`; every other label is a box, cut bare.
var claudeFooterLabels = [...]struct {
	text  string
	class claudeSegmentClass
	hint  bool
}{
	{"<chord> to interrupt", claudeInterrupt, true},
	{"<chord> to return to team lead", claudeTail, true},
	{"<chord> to hide tasks", claudeTail, true},
	{"<chord> to show tasks", claudeTail, true},
	{"/tasks to see subagents", claudeTail, true},
	{"? for shortcuts", claudeTail, true},
	{"← for agents", claudeAgents, true},
	{"← N agent", claudeAgents, true},
	{"← N agents", claudeAgents, true},
	{"← N done", claudeAgents, true},
	{"N feedback draft", claudeTail, true},
	{"N feedback drafts", claudeTail, true},
	{"keep holding…", claudeTail, true},
	{"ctrl+c to copy", claudeTail, true},
	{"option+click to native select", claudeTail, true},
	{"shift+click to native select", claudeTail, true},
	{"set macOptionClickForcesSelection in VS Code settings", claudeTail, true},
	{"hold <chord> to speak", claudeTail, true},
	{"↓ to manage", claudeTail, true},
	{"<chord> to view tasks", claudeTail, true},
	{"<chord> to view artifacts", claudeTail, true},
	{"N local agent", claudeWork, false},
	{"N local agents", claudeWork, false},
	{"N shell", claudeProcess, false},
	{"N shells", claudeProcess, false},
	{"N shell, N monitor", claudeProcess, false},
	{"N shell, N monitors", claudeProcess, false},
	{"N shells, N monitor", claudeProcess, false},
	{"N shells, N monitors", claudeProcess, false},
	{"N monitor", claudeProcess, false},
	{"N monitors", claudeProcess, false},
	{"N Artifact comment monitor", claudeProcess, false},
	{"N Artifact comment monitors", claudeProcess, false},
	{"dreaming", claudeProcess, false},
	{"auto-mode scan", claudeProcess, false},
	{"memory import", claudeProcess, false},
	{"memory import N/N", claudeProcess, false},
	{"N MCP task", claudeUnknownPill, false},
	{"N MCP tasks", claudeUnknownPill, false},
	{"N team", claudeUnknownPill, false},
	{"N teams", claudeUnknownPill, false},
	{"◇ N cloud session", claudeUnknownPill, false},
	{"◇ N cloud sessions", claudeUnknownPill, false},
	{"◇ N remote dynamic workflow", claudeUnknownPill, false},
	{"◇ N remote dynamic workflows", claudeUnknownPill, false},
	{"◆ ultraplan ready", claudeUnknownPill, false},
	{"◇ ultraplan", claudeUnknownPill, false},
	{"◇ ultraplan needs your input", claudeUnknownPill, false},
	{"↓ to view", claudeUnknownPill, false},
	{"N background task", claudeUnknownPill, false},
	{"N background tasks", claudeUnknownPill, false},
	{"↳ N background", claudeUnknownPill, false},
	{"↳ N background (<chord> to manage)", claudeUnknownPill, false},
	{"PR #N", claudeOther, false},
	{"MR !N", claudeOther, false},
	{"gh auth login for PR status", claudeOther, false},
	{"install gh for PR status", claudeOther, false},
}

// claudeInstance reports whether segment is an instance of label, or with
// prefix a cut instance: earlier tokens whole, the last a character prefix.
func claudeInstance(label, segment string, prefix bool) bool {
	labels, tokens := strings.Split(label, " "), strings.Split(segment, " ")
	if len(tokens) > len(labels) || !prefix && len(tokens) < len(labels) {
		return false
	}
	for index, token := range tokens {
		if !claudeTokenInstance(labels[index], token, prefix && index == len(tokens)-1) {
			return false
		}
	}
	return true
}

// claudeTokenInstance matches one token: a trailing <chord> stands for any
// key, and N for a count (digits, or 99+). Labels hold no other capital N.
func claudeTokenInstance(label, token string, prefix bool) bool {
	if head, chord := strings.CutSuffix(label, "<chord>"); chord {
		key, found := strings.CutPrefix(token, head)
		if !found {
			return prefix && strings.HasPrefix(head, token)
		}
		return prefix || key != ""
	}
	for label != "" {
		if token == "" {
			return prefix
		}
		if label[0] == 'N' {
			digits := len(token) - len(strings.TrimLeft(token, "0123456789"))
			if digits == 0 {
				return false
			}
			token, label = strings.TrimPrefix(token[digits:], "+"), label[1:]
			continue
		}
		if label[0] != token[0] {
			return false
		}
		label, token = label[1:], token[1:]
	}
	return token == ""
}

// claudeExactSegment classifies a segment that is an exact label instance.
func claudeExactSegment(segment string) (claudeSegmentClass, bool) {
	for _, label := range claudeFooterLabels {
		if claudeInstance(label.text, segment, false) {
			return label.class, true
		}
	}
	return claudeUnlisted, false
}

// claudeIndicatorShape is `◆ ` and one to six cells: claude cuts the footer
// indicator itself.
func claudeIndicatorShape(segment string) bool {
	text, found := strings.CutPrefix(segment, "◆ ")
	return found && text != "" && ansi.StringWidth(text) <= 6
}

// claudeHintCut strips a hint item's truncation: the `…`, trailing spaces and
// one trailing separator dot.
func claudeHintCut(segment string) (string, bool) {
	text, found := strings.CutSuffix(segment, "…")
	return strings.TrimRight(strings.TrimSuffix(strings.TrimRight(text, " "), "·"), " "), found
}

// claudeSegment classifies one segment of the fullscreen or classic mode row.
// Only the last segment can be cut: Hs truncates with `…`, a box is cut bare.
// A cut segment takes the one class it could have been cut from, or none.
func claudeSegment(segment string, last bool) (class claudeSegmentClass, exact bool) {
	if class, exact := claudeExactSegment(segment); exact {
		return class, true
	}
	if !last {
		if claudeIndicatorShape(segment) {
			return claudeIndicator, false
		}
		return claudeUnlisted, false
	}
	var classes uint16
	if text, truncated := claudeHintCut(segment); truncated {
		if text == "" {
			classes |= 1 << claudeTail
		}
		for _, label := range claudeFooterLabels {
			if label.hint && text != "" && claudeInstance(label.text, text, true) {
				classes |= 1 << label.class
			}
		}
	} else {
		for _, label := range claudeFooterLabels {
			if !label.hint && claudeInstance(label.text, segment, true) {
				classes |= 1 << label.class
			}
		}
	}
	if claudeIndicatorShape(segment) {
		classes |= 1 << claudeIndicator
	}
	if classes == 0 || classes&(classes-1) != 0 {
		return claudeUnlisted, false
	}
	return claudeSegmentClass(bits.TrailingZeros16(classes)), false
}

// claudeModeRow is a parsed mode row.
type claudeModeRow struct {
	manual   bool
	cycle    bool // the mode item ends with a complete ` (<chord> to cycle)`
	segments []string
	cut      bool // a trailing ` ·`: a later item was cut away
	blanks   int  // blank cells after the last item, to the row end or the notification suffix
}

// claudeModeItem cuts the mode item from the start of text: an optional vim
// prefix, the mode glyph (not in the screen reader), the mode and an optional
// complete cycle hint.
func claudeModeItem(text string, glyph bool) (rest string, manual, cycle, ok bool) {
	for _, vim := range [...]string{"-- INSERT -- ", "-- VISUAL -- ", "-- VISUAL LINE -- "} {
		if after, found := strings.CutPrefix(text, vim); found {
			text = after
			break
		}
	}
	if glyph {
		after, pause := strings.CutPrefix(text, "⏸ ")
		if !pause {
			if after, ok = strings.CutPrefix(text, "⏵⏵ "); !ok {
				return "", false, false, false
			}
		}
		text = after
	}
	for _, mode := range [...]string{"manual mode", "plan mode", "accept edits", "auto mode", "bypass permissions", "don't ask"} {
		rest, found := strings.CutPrefix(text, mode+" on")
		if !found {
			continue
		}
		if hint, found := strings.CutPrefix(rest, " ("); found {
			chord, after, spaced := strings.Cut(hint, " ")
			rest, cycle = strings.CutPrefix(after, "to cycle)")
			if !spaced || chord == "" || !cycle {
				return "", false, false, false
			}
		}
		return rest, mode == "manual mode", cycle, true
	}
	return "", false, false, false
}

// claudeModeFull parses the fullscreen and classic mode row: two cells of
// padding, the mode item, ` · `-separated segments, an optional trailing
// ` ·`, and a right-aligned notification suffix after three or more spaces.
func claudeModeFull(line claudeLine, width int) (claudeModeRow, bool) {
	var mode claudeModeRow
	text, padded := strings.CutPrefix(line.plain, "  ")
	if !padded {
		return mode, false
	}
	items, manual, cycle, ok := claudeModeItem(text, true)
	if !ok {
		return mode, false
	}
	mode.manual, mode.cycle = manual, cycle
	items, suffix, notified := strings.Cut(items, "   ")
	if notified {
		mode.blanks = 3 + len(suffix) - len(strings.TrimLeft(suffix, " "))
	} else {
		extent := len(line.cells)
		for extent > 0 && (line.cells[extent-1].text == " " || line.cells[extent-1].text == "\u00a0") {
			extent--
		}
		mode.blanks = width - extent
	}
	items, mode.cut = strings.CutSuffix(items, " ·")
	if items != "" {
		body, separated := strings.CutPrefix(items, " · ")
		if !separated {
			return mode, false
		}
		mode.segments = strings.Split(body, " · ")
	}
	return mode, true
}

// claudeModeEvidence classifies M's segments and decides whether M proves the
// pill slot: a visible task pill is never hidden behind what M shows.
func claudeModeEvidence(mode claudeModeRow) (classes uint16, proven bool) {
	// P0: claude draws the cycle hint in a non-default mode only without a pill.
	proven = !mode.manual && mode.cycle
	lastExact := true // the mode item itself, when no segment follows
	for index, segment := range mode.segments {
		last := index == len(mode.segments)-1
		class, exact := claudeSegment(segment, last)
		classes |= 1 << class
		// P1: a complete hint member means Hs was laid out, which happens only
		// after every box before it has its full width.
		if exact && class == claudeTail {
			proven = true
		}
		if last {
			lastExact = exact && !strings.HasSuffix(segment, "…")
			// P1 too: a cut hint item is Hs, laid out last.
			if text, truncated := claudeHintCut(segment); truncated {
				proven = proven || text == ""
				for _, label := range claudeFooterLabels {
					proven = proven || label.hint && claudeInstance(label.text, text, true)
				}
			}
		}
	}
	// P2: a complete label with room after it left no box unshown.
	return classes, proven || !mode.cut && lastExact && mode.blanks >= 6
}

// claudeModeSR parses the screen reader's mode row: its boxes join with one
// space, so their separators read `  ·  `; it never cuts an item.
func claudeModeSR(plain string) (claudeModeRow, bool) {
	var mode claudeModeRow
	parts := strings.Split(plain, "·")
	text := parts[0]
	for _, part := range parts[1:] {
		if trimmed, after := strings.TrimRight(text, " "), strings.TrimLeft(part, " "); trimmed != text && after != part {
			text = trimmed + " · " + after
		} else {
			text += "·" + part
		}
	}
	rest, manual, cycle, ok := claudeModeItem(text, false)
	if !ok {
		return mode, false
	}
	mode.manual, mode.cycle = manual, cycle
	if rest != "" {
		body, separated := strings.CutPrefix(rest, " · ")
		if !separated {
			return mode, false
		}
		mode.segments = strings.Split(body, " · ")
	}
	return mode, true
}

// claudeFamily reads the request and menu families (research 2.6), which
// replace the composer. Their hint block or options end at L′, the lowest row
// once at most two trailing right-aligned rows are passed; every column is
// exact, so quoted transcript text cannot reach them.
func claudeFamily(s *claudeScreen, last int) (reading, bool) {
	hint := last
	for skipped := 0; skipped < 2 && s.lines[hint].col >= 8; skipped++ {
		hint--
		for s.blankAt(hint) {
			hint--
		}
		if _, ok := s.line(hint); !ok {
			return reading{}, false
		}
	}
	id, interaction, first, ok := claudeViewer(s, hint)
	if !ok {
		id, interaction, first, ok = claudeMenu(s, hint)
	}
	if !ok {
		id, interaction, first, ok = claudePermission(s, hint)
	}
	if !ok {
		id, interaction, first, ok = claudeQuestion(s, hint)
	}
	if !ok {
		id, interaction, first, ok = claudeElicitation(s, hint)
	}
	if !ok {
		id, interaction, first, ok = claudePlan(s, hint)
	}
	if !ok {
		id, interaction, first, ok = claudeSetup(s, hint)
	}
	if !ok {
		return reading{}, false
	}
	return reading{
		activity: sessions.ActivityUnknown, interaction: interaction, notice: sessions.NoticeNone, composer: composerBlocked,
		rules: []DiagnosticRule{s.rule(id, first, hint), s.composerRule(composerBlocked, first, hint)},
	}, true
}

var claudeAgentsViewCounts = regexp.MustCompile(`\d+ awaiting input · \d+ working · \d+ completed`)

// claudeViewer reads the viewer footers: the transcript viewer, with a dialog
// waiting behind it or not, and the agents view.
func claudeViewer(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	text, top, ok := s.block(2, hint)
	switch {
	case !ok:
	case strings.HasPrefix(text, "dialog waiting · Showing detailed transcript · "):
		return "claude.input.dialog_waiting", sessions.InteractionInput, top, true
	case strings.HasPrefix(text, "Showing detailed transcript · "):
		return "claude.menu.transcript", sessions.InteractionMenu, top, true
	case text == "enter to return" || strings.HasPrefix(text, "enter to return · ") ||
		strings.Contains(text, " · enter to return · ") || strings.HasSuffix(text, " · enter to return"):
		if row, found := s.find(top-1, func(line claudeLine) bool { return claudeAgentsViewCounts.MatchString(line.plain) }); found {
			return "claude.menu.agents_view", sessions.InteractionMenu, row, true
		}
	}
	return "", "", 0, false
}

// claudeMenu reads the model, settings, theme, resume and side-question menus.
func claudeMenu(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, _, ok := s.block(3, hint); ok {
		switch {
		case strings.HasPrefix(text, "Enter to set as default · "):
			if row, found := s.find(hint-1, s.edge); found {
				return "claude.menu.model", sessions.InteractionMenu, row, true
			}
		case strings.HasPrefix(text, "Type to filter · ") || strings.HasPrefix(text, "Enter/Space to change · "):
			if row, found := s.find(hint-1, func(line claudeLine) bool {
				fields := strings.Fields(line.plain)
				return strings.HasPrefix(line.plain, "   Settings ") && len(fields) >= 4 && fields[1] == "Status" && fields[2] == "Config" && fields[3] == "Usage"
			}); found {
				return "claude.menu.settings", sessions.InteractionMenu, row, true
			}
		case text == "Enter to select · Esc to cancel":
			if row, found := s.find(hint-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    Syntax theme: ") }); found {
				return "claude.menu.theme", sessions.InteractionMenu, row, true
			}
		}
	}
	if text, top, ok := s.block(5, hint); ok && strings.Contains(text, "to show all projects") && strings.HasSuffix(text, "Esc to cancel") {
		return "claude.menu.resume", sessions.InteractionMenu, top, true
	}
	if text, top, ok := s.block(4, hint); ok && strings.HasSuffix(text, "Esc to close") {
		_, question := s.find(top-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    /btw ") })
		if row, found := s.find(top-1, s.edge); found && question {
			return "claude.menu.btw", sessions.InteractionMenu, row, true
		}
	}
	return "", "", 0, false
}

// claudeChord cuts `<chord><phrase>` from the start of text.
func claudeChord(text, phrase string) (string, bool) {
	chord, rest, found := strings.Cut(text, " ")
	if !found || chord == "" {
		return "", false
	}
	return strings.CutPrefix(" "+rest, phrase)
}

// claudeCancelHint is a permission hint `<chord> to cancel`, optionally
// ` · <chord> to amend`, and for a workflow ` <chord> to edit script in $EDITOR`.
func claudeCancelHint(text string, workflow bool) bool {
	rest, ok := claudeChord(text, " to cancel")
	if !ok {
		return false
	}
	if after, found := strings.CutPrefix(rest, " · "); found {
		if rest, ok = claudeChord(after, " to amend"); !ok {
			return false
		}
	}
	if after, found := strings.CutPrefix(rest, " "); workflow && found {
		if rest, ok = claudeChord(after, " to edit script in $EDITOR"); !ok {
			return false
		}
	}
	return rest == ""
}

// claudePermission reads the tool permission dialog (options and hint at
// column 1) and the dynamic workflow permission (column 2, titled at column 1).
func claudePermission(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, top, ok := s.block(1, hint); ok && claudeCancelHint(text, false) {
		options, optionsTop, ok := s.options(1, top-1)
		if ok && strings.HasPrefix(options[0].label, "Yes") {
			if _, questionTop, end, ok := s.above(1, optionsTop); ok {
				for r := end; r >= questionTop; r-- {
					if strings.HasPrefix(s.lines[r].plain[s.lines[r].col:], "Do you want to ") {
						texts := make([]string, 0, end-r+1)
						for row := r; row <= end; row++ {
							texts = append(texts, s.lines[row].plain[s.lines[row].col:])
						}
						if strings.HasSuffix(strings.Join(texts, " "), "?") {
							return "claude.permission.dialog", sessions.InteractionPermission, r, true
						}
						break
					}
				}
			}
		}
	}
	if text, top, ok := s.block(2, hint); ok && claudeCancelHint(text, true) {
		options, optionsTop, ok := s.options(2, top-1)
		if ok && options[0].label == "Yes, run it" && options[len(options)-1].label == "No" {
			if rule, found := s.titled(optionsTop, " Run a dynamic workflow?"); found {
				return "claude.permission.workflow", sessions.InteractionPermission, rule, true
			}
		}
	}
	return "", "", 0, false
}

// claudeQuestion reads the question form, the question with option previews,
// and the submit review.
func claudeQuestion(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, top, ok := s.block(0, hint); ok && strings.HasPrefix(text, "Enter to select · ") && s.blankAt(top-1) {
		chat, chatOK := s.line(top - 2)
		rule, ruleOK := s.line(top - 3)
		if chatOK && ruleOK && s.full(rule, "─") {
			switch {
			case strings.Contains(text, " · n to add notes · ") && strings.HasSuffix(text, "Esc to cancel") &&
				(chat.plain == "❯ Chat about this" || chat.plain == "  Chat about this"):
				if first, ok := claudePreview(s, top-3, strings.HasPrefix(chat.plain, "❯")); ok {
					return "claude.question.preview", sessions.InteractionQuestion, first, true
				}
			case strings.Contains(text, " to navigate") && strings.HasSuffix(text, "to cancel") && !strings.Contains(text, "n to add notes"):
				number, label, pointer, start := claudeOptionStart(chat.plain, 0)
				options, optionsTop, ok := s.options(0, top-4)
				if start && label == "Chat about this" && number > 0 && ok {
					for _, option := range options {
						pointer = pointer || option.selected
					}
					if pointer {
						return "claude.question.form", sessions.InteractionQuestion, optionsTop, true
					}
				}
			}
		}
	}
	options, _, ok := s.options(0, hint)
	if ok && len(options) == 2 && options[0].label == "Submit answers" && options[1].label == "Cancel" {
		blocks := s.texts(0, hint)
		_, ready := claudeTextBlock(blocks, func(text string) bool { return strings.Contains(text, "Ready to submit your answers?") })
		if review, found := claudeTextBlock(blocks, func(text string) bool { return strings.Contains(text, "Review your answers") }); found && ready {
			return "claude.question.review", sessions.InteractionQuestion, review, true
		}
	}
	return "", "", 0, false
}

// claudePreview reads a question's side-by-side options and preview box above
// its rule: the cells left of the box corner are the options. It returns the
// corner's row, and holds when a pointer marks an option or the chat row.
func claudePreview(s *claudeScreen, rule int, chatPointer bool) (int, bool) {
	corner, column := -1, -1
	for r := rule - 1; corner < 0; r-- {
		line, ok := s.line(r)
		if !ok {
			return 0, false
		}
		for index, cell := range line.cells {
			if cell.text == "┌" {
				corner, column = r, index
				break
			}
		}
	}
	var lefts []claudeLine
	for r := corner; r < rule; r++ {
		cells := s.lines[r].cells[:min(column, len(s.lines[r].cells))]
		var text strings.Builder
		for _, cell := range cells {
			text.WriteString(cell.text)
		}
		left := claudeLine{parsed: true, cells: cells, col: -1}
		left.plain = strings.TrimRight(strings.ReplaceAll(text.String(), "\u00a0", " "), " ")
		if left.plain != "" {
			left.col = len(left.plain) - len(strings.TrimLeft(left.plain, " "))
		}
		if _, _, _, start := claudeOptionStart(left.plain, 0); !start && (left.blank() || left.col < 5 || len(lefts) == 0) {
			break
		}
		lefts = append(lefts, left)
	}
	options, ok := claudeParseOptions(lefts, 0)
	if !ok {
		return 0, false
	}
	for _, option := range options {
		chatPointer = chatPointer || option.selected
	}
	return corner, chatPointer
}

// claudeElicitation reads an mcp server's elicitation form.
func claudeElicitation(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, _, ok := s.block(2, hint); ok && strings.HasPrefix(text, "Esc to cancel · ") {
		if row, found := s.find(hint-1, func(line claudeLine) bool {
			return strings.HasPrefix(line.plain, "  MCP server “") &&
				(strings.HasSuffix(line.plain, "” requests your input") || strings.HasSuffix(line.plain, "” wants to open a URL"))
		}); found {
			return "claude.input.elicitation", sessions.InteractionInput, row, true
		}
	}
	return "", "", 0, false
}

// claudePlan reads plan approval and the exit- and enter-plan decisions.
func claudePlan(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	end := hint
	if text, top, ok := s.block(3, hint); ok {
		chord, rest, found := strings.Cut(text, " ")
		if found && chord != "" && strings.HasPrefix(rest, "to edit in ") {
			end = top - 1
		}
	}
	if options, optionsTop, ok := s.options(3, end); ok && strings.HasPrefix(options[0].label, "Yes") {
		change := false
		for _, option := range options {
			change = change || strings.HasPrefix(option.label, "Tell Claude what to change")
		}
		if text, top, _, ok := s.above(3, optionsTop); ok && change && strings.HasPrefix(text, "Claude has written up a plan and is ready to execute.") {
			return "claude.confirmation.plan", sessions.InteractionConfirmation, top, true
		}
	}
	if options, optionsTop, ok := s.options(4, hint); ok && strings.HasPrefix(options[0].label, "Yes, and switch to ") && options[len(options)-1].label == "No" {
		if text, top, _, ok := s.above(4, optionsTop); ok && text == "Claude wants to exit plan mode" {
			return "claude.confirmation.exit_plan", sessions.InteractionConfirmation, top, true
		}
	}
	// Entering plan mode draws no hint; its confirm label names the mode.
	if options, optionsTop, ok := s.unnumbered(2, hint); ok && strings.HasSuffix(options, "No, start implementing now") {
		if rule, found := s.titled(optionsTop, " Enter plan mode?"); found {
			return "claude.confirmation.enter_plan", sessions.InteractionConfirmation, rule, true
		}
	}
	return "", "", 0, false
}

// claudeSetup reads the dialogs before and around the first prompt. Trust is
// drawn at column 1; claude's shared dialog draws its options and hint at
// column 2 under a full rule, and its variants differ by their anchors.
func claudeSetup(s *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	const confirm = "Enter to confirm · Esc to cancel"
	ruleAbove := func(row int) (int, bool) {
		return s.find(row-1, func(line claudeLine) bool { return s.full(line, "─") })
	}
	hint1, top1, ok1 := s.block(1, hint)
	hint2, top2, ok2 := s.block(2, hint)
	if ok1 && hint1 == confirm {
		options, optionsTop, ok := s.unnumbered(1, top1-1)
		_, workspace := claudeTextBlock(s.texts(1, hint), func(text string) bool { return strings.Contains(text, "Accessing workspace:") })
		if ok && workspace && claudeWords(options, "No, exit") && claudeWords(options, "Yes, I trust this folder") {
			if rule, found := ruleAbove(optionsTop); found {
				return "claude.setup.trust", sessions.InteractionSetup, rule, true
			}
		}
	}
	if ok2 && hint2 == confirm {
		blocks := s.texts(2, hint)
		contains := func(copy string) bool {
			_, found := claudeTextBlock(blocks, func(text string) bool { return strings.Contains(text, copy) })
			return found
		}
		starts := func(copy string) bool {
			_, found := claudeTextBlock(blocks, func(text string) bool { return strings.HasPrefix(text, copy) })
			return found
		}
		options, optionsTop, unnumbered := s.unnumbered(2, top2-1)
		numbered, numberedTop, ok := s.options(2, top2-1)
		labels := map[string]bool{}
		for _, option := range numbered {
			labels[option.label] = true
		}
		var id string
		switch {
		case unnumbered && contains("Detected a custom API key in your environment") && contains("Do you want to use this API key?"):
			id = "claude.setup.apikey"
		case unnumbered && starts("WARNING: Claude Code running in Bypass Permissions mode") &&
			claudeWords(options, "No, exit") && claudeWords(options, "Yes, I accept"):
			id = "claude.setup.bypass"
		case unnumbered && starts("New MCP server found in this project: ") &&
			claudeWords(options, "Use this MCP server") && claudeWords(options, "Continue without using this MCP server"):
			id = "claude.setup.mcp_server"
		case ok && starts("Settings Error") && labels["Exit and fix manually"] && labels["Continue without these settings"]:
			id, optionsTop = "claude.setup.settings_error", numberedTop
		}
		if id != "" {
			if rule, found := ruleAbove(optionsTop); found {
				return id, sessions.InteractionSetup, rule, true
			}
		}
	}
	if rest, ok := claudeChord(hint1, " to select"); ok1 && ok && claudeServerApproval(s, top1, rest) {
		blocks := s.texts(2, hint)
		_, found := claudeTextBlock(blocks, func(text string) bool {
			digits := len(text) - len(strings.TrimLeft(text, "0123456789"))
			return digits > 0 && strings.HasPrefix(text[digits:], " new MCP servers found in this project")
		})
		if rule, ruled := ruleAbove(top1); found && ruled {
			return "claude.setup.mcp_servers", sessions.InteractionSetup, rule, true
		}
	}
	if ok2 && strings.HasPrefix(hint2, "Syntax theme: ") {
		// The theme choices sit at column 1 above the syntax preview.
		bottom, found := s.find(top2-1, claudeThemeOption)
		top, selected := bottom, 0
		for found {
			if line := s.lines[top]; strings.HasPrefix(line.plain, " ❯ ") {
				selected++
			}
			line, ok := s.line(top - 1)
			if !ok || !claudeThemeOption(line) {
				break
			}
			top--
		}
		_, started := claudeTextBlock(s.texts(1, hint), func(text string) bool { return strings.Contains(text, "Let's get started.") })
		if found && started && selected == 1 {
			texts := make([]string, 0, bottom-top+1)
			for r := top; r <= bottom; r++ {
				texts = append(texts, s.lines[r].plain[s.lines[r].col:])
			}
			if claudeWords(strings.Join(texts, " "), "Dark mode (ANSI colors only)") {
				return "claude.setup.theme", sessions.InteractionSetup, top, true
			}
		}
	}
	if options, optionsTop, ok := s.options(1, hint); ok && strings.HasPrefix(options[0].label, "Claude account with subscription") {
		if row, found := claudeTextBlock(s.texts(1, hint), func(text string) bool { return strings.Contains(text, "Select login method:") }); found {
			return "claude.setup.login", sessions.InteractionSetup, min(row, optionsTop), true
		}
	}
	if ok1 && hint1 == "Press Enter to continue…" {
		if row, found := claudeTextBlock(s.texts(1, hint), func(text string) bool { return strings.Contains(text, "Security notes:") }); found {
			return "claude.setup.security", sessions.InteractionSetup, row, true
		}
	}
	return "", "", 0, false
}

// claudeServerApproval reads the multi-server approval below its title: the
// rest of a `<chord> to select · <chord> to reject all` hint at column 1, the
// submit row above it and the checkbox rows above that, one pointer at most.
func claudeServerApproval(s *claudeScreen, hint int, rest string) bool {
	after, separated := strings.CutPrefix(rest, " · ")
	if rest, ok := claudeChord(after, " to reject all"); !separated || !ok || rest != "" {
		return false
	}
	submit, ok := s.line(hint - 1)
	if !ok || submit.plain != "       Enable selected" && submit.plain != "  ❯    Enable selected" {
		return false
	}
	pointers, boxes := strings.Count(submit.plain, "❯"), 0
	for r := hint - 2; ; r-- {
		line, ok := s.line(r)
		if !ok {
			break
		}
		label, pointer := strings.CutPrefix(line.plain, "  ❯ ")
		if !pointer {
			if label, ok = strings.CutPrefix(line.plain, "    "); !ok {
				break
			}
		}
		name, boxed := strings.CutPrefix(label, "[✔] ")
		if !boxed {
			name, boxed = strings.CutPrefix(label, "[ ] ")
		}
		if !boxed || name == "" || name[0] == ' ' {
			break
		}
		if pointer {
			pointers++
		}
		boxes++
	}
	return boxes > 0 && pointers <= 1
}

// claudeThemeOption is a theme choice at column 1: `^ (❯ |    )\S`.
func claudeThemeOption(line claudeLine) bool {
	return line.col == 1 && strings.HasPrefix(line.plain, " ❯ ") && !strings.HasPrefix(line.plain, " ❯  ") || line.col == 5
}
