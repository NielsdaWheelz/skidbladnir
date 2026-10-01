package agentcontrol

import (
	"math/bits"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// detectClaude reads a claude 2.1.286 screen, and 2.1.284's numbered theme
// picker, by the frozen grammar of docs/terminal-observation-claude.md sections
// 2-3, whose section numbers the comments below cite. It finds the lowest drawn
// row and tries the surfaces in order: screen-reader startup, the screen-reader
// input row, a screen-reader request, the composer, then the request and menu
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
	if read, ok := claudeStartupSR(screen, last); ok {
		return read
	}
	if read, ok := claudeSurfaceSR(screen, last); ok {
		return read
	}
	if read, ok := claudeRequestSR(screen, last); ok {
		return read
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
	read.rules = []DiagnosticRule{screen.clippedRule()}
	return read
}

// clippedRule names the captured row just below the cut, or the bottom
// region, which always ends at the last row, when it kept no row at all.
func (screen *claudeScreen) clippedRule() DiagnosticRule {
	if screen.floor == len(screen.lines) {
		return DiagnosticRule{ID: "claude.region.clipped", Region: DiagnosticBottom}
	}
	return screen.rule("claude.region.clipped", screen.floor, screen.floor)
}

// backgroundRule names the row whose background work makes the pane working.
func (screen *claudeScreen) backgroundRule(row int) DiagnosticRule {
	return screen.rule("claude.activity.background", row, row)
}

// composerRule names a composer value that has its own rule.
func (screen *claudeScreen) composerRule(value composer, first, last int) DiagnosticRule {
	switch value {
	case composerEmpty:
		return screen.rule("claude.composer.empty", first, last)
	case composerDraft:
		return screen.rule("claude.composer.draft", first, last)
	case composerBlocked:
		return screen.rule("claude.composer.blocked", first, last)
	default:
		panic("claude composer without a rule") // justify-defect: callers name only empty, draft or blocked.
	}
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

// claudeNotification is a right-aligned notification or a tmux notice at
// column 2: the only rows the composer margin and footer tail may hold.
func claudeNotification(line claudeLine) bool {
	return line.col >= 8 || line.col == 2 && slices.ContainsFunc(claudeTmuxNotices, func(notice string) bool {
		head, _, _ := strings.Cut(notice, " · ")
		return strings.HasPrefix(line.plain, "  "+head+" · ")
	})
}

// claudeTmuxNotices are claude's two notices about the tmux it runs in.
var claudeTmuxNotices = []string{
	"tmux detected · scroll with PgUp/PgDn · or add 'set -g mouse on' to ~/.tmux.conf for wheel scroll",
	"tmux focus-events off · add 'set -g focus-events on' to ~/.tmux.conf and reattach for focus tracking",
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

// textBlock finds the topmost of the blocks whose top row is at exactly c, at
// and above end, whose text matches.
func (screen *claudeScreen) textBlock(c, end int, match func(string) bool) (int, bool) {
	found, matched := 0, false
	for r := end; ; r-- {
		line, ok := screen.line(r)
		if !ok {
			return found, matched
		}
		if line.blank() || line.col < c-1 || line.col > c+1 {
			continue
		}
		if text, top, ok := screen.block(c, r); ok {
			if match(text) {
				found, matched = top, true
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
// column 2 reads exactly title and has a full rule directly above it. It
// returns the rule's row.
func (screen *claudeScreen) titled(row int, title string) (int, bool) {
	found, ok := screen.find(row-1, func(line claudeLine) bool { return !line.blank() && line.col < 2 })
	if !ok || screen.lines[found].plain != title {
		return 0, false
	}
	rule, ok := screen.line(found - 1)
	return found - 1, ok && screen.ruleRow(rule)
}

// claudeWords reports phrase in text between word boundaries.
func claudeWords(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

// claudeStartupSR is the screen reader's startup quiet: its banner is the
// lowest row until the first layout mounts.
func claudeStartupSR(screen *claudeScreen, last int) (reading, bool) {
	switch screen.lines[last].plain {
	case "[Screen Reader Mode: on via flag]", "[Screen Reader Mode: on via env]",
		"[Screen Reader Mode: on via settings]", "[Screen Reader Mode: on]":
		return reading{
			activity: sessions.ActivityStarting, interaction: sessions.InteractionUnknown,
			rules: []DiagnosticRule{screen.rule("claude.activity.starting_sr", last, last)},
		}, true
	}
	return reading{}, false
}

// claudeEffortSR is the screen reader's effort notification (2.5).
var claudeEffortSR = regexp.MustCompile(`^effort: (?:low|medium|high|xhigh|max)(?: · ultracode)? · /effort$`)

// claudeSurfaceSR reads the screen reader's input row `$`, the notification
// blocks above it, its flattened mode row and the pinned column above that
// (2.5).
func claudeSurfaceSR(screen *claudeScreen, last int) (reading, bool) {
	input := screen.lines[last]
	if input.presence != rowParsed || input.cells[0].text != "$" || len(input.cells) > 1 && input.cells[1].text != "\u00a0" {
		return reading{}, false
	}
	mode := last - 1
	for {
		line, ok := screen.line(mode)
		if !ok {
			break
		}
		if claudeEffortSR.MatchString(line.plain) {
			mode--
			continue
		}
		// A tmux notice wraps over at most three rows.
		notice, joined := 0, line.plain
		for rows := 1; ; rows++ {
			if slices.Contains(claudeTmuxNotices, joined) {
				notice = rows
				break
			}
			if rows == 3 {
				break
			}
			upper, ok := screen.line(mode - rows)
			if !ok {
				// A notice's tail under the cut: its head, and the mode row
				// above it, lie in the cut.
				tail := " " + joined
				if screen.clippedAt(mode-rows) && slices.ContainsFunc(claudeTmuxNotices, func(notice string) bool { return strings.HasSuffix(notice, tail) }) {
					notice = rows
				}
				break
			}
			joined = upper.plain + " " + joined
		}
		if notice == 0 {
			break
		}
		mode -= notice
	}
	line, ok := screen.line(mode)
	first := mode
	if !ok {
		first++ // only the notification rows were read
	}
	layout := screen.rule("claude.layout.sr", first, last)
	unknown := unknownReading(causeUnrecognized)
	if !ok && screen.clippedAt(mode) {
		unknown = screen.clipped()
	}
	unknown.rules = append(unknown.rules, layout)
	segments, parsed := claudeModeSR(line.plain)
	if !ok || !parsed {
		return unknown, true
	}
	var classes uint16
	for _, segment := range segments {
		classes |= 1 << claudeExactSegment(segment)
	}
	if classes&(1<<claudeUnlisted) != 0 {
		return unknown, true
	}

	read := reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionNone, composer: composerDraft}
	if input.plain == "$" {
		read.composer = composerEmpty
	}
	// Claude draws its pinned notices, then the usage-limit wait, in one column
	// above the mode row and any statusline rows. Each item starts `⚠ ` and
	// wraps below its start, so the rows from the lowest such start down to
	// the mode row hold any wait. The 16-row bound keeps a quoted `⚠ ` row in
	// the transcript out of the read.
	lead, bound := mode-1, max(mode-16, 0)
	for lead >= bound && !screen.clippedAt(lead) && !strings.HasPrefix(screen.lines[lead].plain, "⚠ ") {
		lead--
	}
	var limit DiagnosticRule
	switch {
	case lead < bound:
		// No pinned column.
	case screen.clippedAt(lead):
		// The column's start, and so a wait, may lie in the cut.
		read.interaction, read.interactionCause, read.composer = sessions.InteractionUnknown, causeClipped, composerUnknown
	case claudeLimitCopy.MatchString(screen.joined(lead, mode-1)):
		read.interaction, read.composer, limit = sessions.InteractionUnknown, composerUnknown, screen.rule("claude.limit.wait", lead, mode-1)
	}
	var activity DiagnosticRule
	switch {
	case classes&(1<<claudeWork) != 0:
		read.activity, activity = sessions.ActivityWorking, screen.backgroundRule(mode)
	case classes&(1<<claudeInterrupt) != 0:
		read.activity, activity = sessions.ActivityWorking, screen.rule("claude.activity.hint_sr", mode, mode)
	case classes&(1<<claudeUnknownPill) == 0 && read.composer == composerEmpty:
		if above, ok := screen.line(mode - 1); ok && (claudeCompletion(above.plain) || strings.HasPrefix(above.plain, "claude: ")) {
			read.activity, activity = sessions.ActivityIdle, screen.rule("claude.activity.idle_sr", mode-1, last)
		}
	}
	if read.interactionCause == causeClipped {
		read.rules = append(read.rules, screen.clippedRule())
	}
	for _, rule := range [...]DiagnosticRule{layout, limit, activity} {
		if rule.ID != "" {
			read.rules = append(read.rules, rule)
		}
	}
	if read.composer != composerUnknown {
		read.rules = append(read.rules, screen.composerRule(read.composer, last, last))
	}
	return read, true
}

// The screen reader's live request row and the anchors of a setup, a
// permission and a question (its tab row, its header, or its `> …?` prompt)
// (2.5).
var (
	claudeRequestRowSR = regexp.MustCompile(`^(?:Select with numbers \[1-\d+\]|Enter text for option \d+ |Enter y/n:$|Press Enter to continue…$)`)
	claudeSetupSR      = regexp.MustCompile(`^(?:Permission Required: )?(?:Accessing workspace:|Detected a custom API key in your environment|` +
		`WARNING: Claude Code running in Bypass Permissions mode|New MCP server found in this project: |Select login method:|` +
		`Let's get started\.|Security notes:|Settings Error)`)
	claudeProceedSR  = regexp.MustCompile(`^Do you want to .+\?$`)
	claudeQuestionSR = regexp.MustCompile(`^(?:←\s+[☐☒✔]| [☐☒✔] \S|> .+\?$)`)
)

// claudeRequestSR reads a screen-reader request: its request row is the
// lowest row, or the row above a trailing hint or theme preview, and its kind
// comes from anchor rows above it. Claude draws the request row only while a
// request is live, so a quoted anchor can only change the kind.
func claudeRequestSR(screen *claudeScreen, last int) (reading, bool) {
	request := last
	if plain := screen.lines[last].plain; plain == "Esc to cancel · Tab to amend" || plain == "Enter to confirm · Esc to cancel" ||
		strings.HasPrefix(plain, " Syntax theme: ") {
		request--
	}
	line, ok := screen.line(request)
	if !ok || !claudeRequestRowSR.MatchString(line.plain) {
		return reading{}, false
	}
	setup, permission, question, proceed := -1, -1, -1, false
	r := request - 1
	for ; ; r-- {
		line, ok := screen.line(r)
		if !ok {
			break
		}
		if claudeSetupSR.MatchString(line.plain) {
			setup = r
		}
		if strings.HasPrefix(line.plain, "Permission Required: ") {
			permission = r
		}
		proceed = proceed || claudeProceedSR.MatchString(line.plain)
		if claudeQuestionSR.MatchString(line.plain) {
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
	case screen.clippedAt(r):
		return screen.clipped(), true
	default:
		return unknownReading(causeUnrecognized), true
	}
	return reading{
		activity: sessions.ActivityUnknown, interaction: interaction, composer: composerBlocked,
		rules: []DiagnosticRule{screen.rule(id, first, last), screen.composerRule(composerBlocked, first, last)},
	}, true
}

// claudeComposer reads the composer surface (2.3): the input row
// between a top and a bottom rule, the footer below them and the slot row
// above them. Only the first full rule at or above the lowest row is tried, so
// a dead process's frame above a newer surface never anchors it.
func claudeComposer(screen *claudeScreen, last int) (reading, bool) {
	bottom := last
	for ; bottom > last-30; bottom-- {
		line, ok := screen.line(bottom)
		if !ok {
			return reading{}, false
		}
		if screen.ruleRow(line) {
			break
		}
	}
	if bottom == last-30 {
		return reading{}, false
	}
	chevron := bottom - 1
	for {
		line, ok := screen.line(chevron)
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
	prompt := screen.lines[chevron]
	if prompt.presence != rowParsed {
		return reading{}, false
	}
	head := prompt.cells[0]
	nbsp := len(prompt.cells) > 1 && prompt.cells[1].text == "\u00a0"
	bash := head.text == "!" && nbsp
	if !bash && !(head.text == "❯" && head.style.bg.kind == colorDefault && (nbsp || len(prompt.cells) == 1)) {
		return reading{}, false
	}
	top := chevron - 1
	topRule, ok := screen.line(top)
	if !ok {
		return reading{}, false
	}
	ordinary := screen.ruleRow(topRule) || screen.banner(topRule)
	if !ordinary && !screen.leftLabel(topRule) {
		return reading{}, false
	}

	layout := screen.rule("claude.layout.composer", top, last)
	footer := claudeReadFooter(screen, bottom, last, bash)
	if footer.kind == claudeFooterVoid {
		// Anything the grammar cannot account for may be drawn over this
		// surface: a dead process's frame, autocomplete, a focused panel.
		read := unknownReading(causeUnrecognized)
		read.rules = []DiagnosticRule{layout}
		return read, true
	}
	slot, kind := claudeSlot(screen, top)
	// Claude draws the usage-limit wait only in its pinned column, between the
	// bottom rule and the statusline, which the footer reads as opaque rows. Its
	// items wrap, so the rows below the bottom rule read as one text.
	waiting := claudeLimitCopy.MatchString(screen.joined(bottom+1, last))

	// The rule is promptBorder-coloured in every theme, so a rule without a
	// foreground means colour is off and the chevron's style says nothing.
	live := topRule.cells[0].style.fg.kind != colorDefault
	ready := !bash && live && head.style.fg.kind == colorDefault && !head.style.dim && !head.style.reverse
	loading := !bash && (head.style.fg.kind != colorDefault || head.style.dim)
	interrupt := footer.classes&(1<<claudeInterrupt) != 0
	corroborator := kind == claudeSlotSpinner || kind == claudeSlotRetry || interrupt
	// Idle needs the complete ready layout, every region that could show work
	// or a request read and showing none, and no usage-limit wait.
	complete := ready && ordinary && footer.kind == claudeFooterOrdinary && !footer.extra && footer.proven &&
		footer.classes&^claudeTolerated == 0 && (kind == claudeSlotNone || kind == claudeSlotOther) && !waiting

	read := reading{activity: sessions.ActivityUnknown}
	var activity DiagnosticRule
	// The panel, the waiting row and a work pill are independent of the
	// chevron and the slot, so they decide before any conflict.
	switch {
	case footer.work >= 0:
		read.activity, activity = sessions.ActivityWorking, screen.backgroundRule(footer.work)
	case kind == claudeSlotWait:
		read.activity, activity = sessions.ActivityWorking, screen.backgroundRule(slot)
	case footer.classes&(1<<claudeWork) != 0:
		read.activity, activity = sessions.ActivityWorking, screen.backgroundRule(footer.mode)
	case corroborator && ready:
		first, end := chevron, chevron
		if kind == claudeSlotSpinner || kind == claudeSlotRetry {
			first = slot
		}
		if interrupt {
			end = footer.mode
		}
		read.activityCause, activity = causeConflict, screen.rule("claude.activity.conflict", first, end)
	case loading && kind == claudeSlotSpinner:
		read.activity, activity = sessions.ActivityWorking, screen.rule("claude.activity.spinner", slot, chevron)
	case loading && kind == claudeSlotRetry:
		read.activity, activity = sessions.ActivityWorking, screen.rule("claude.activity.retry", slot, chevron)
	case interrupt:
		read.activity, activity = sessions.ActivityWorking, screen.rule("claude.activity.hint", chevron, footer.mode)
	case corroborator:
		// Spinner chrome under a chevron whose style says nothing: unknown.
	case kind == claudeSlotClipped:
		read.activityCause = causeClipped
	case complete:
		first := top
		if kind == claudeSlotOther {
			first = slot
		}
		read.activity, activity = sessions.ActivityIdle, screen.rule("claude.activity.idle", first, last)
	}

	var family DiagnosticRule
	switch {
	case waiting:
		read.interaction, family = sessions.InteractionUnknown, screen.rule("claude.limit.wait", bottom+1, last)
	case !ordinary:
		read.interaction = sessions.InteractionUnknown
	case footer.viewed >= 0:
		read.interaction, family = sessions.InteractionMenu, screen.rule("claude.menu.subagent", footer.viewed, footer.viewed)
	case footer.kind == claudeFooterHelp:
		read.interaction, family = sessions.InteractionMenu, screen.rule("claude.menu.help", bottom+1, last)
	default:
		read.interaction = sessions.InteractionNone
	}

	var composerRule DiagnosticRule
	switch {
	case read.interaction == sessions.InteractionMenu:
		read.composer, composerRule = composerBlocked, screen.composerRule(composerBlocked, chevron, chevron)
	case read.interaction == sessions.InteractionUnknown:
	case kind == claudeSlotOther && claudeBandLegend.MatchString(screen.lines[slot].plain):
		composerRule = screen.rule("claude.composer.band", slot, slot)
	case bash:
		read.composer, composerRule = composerDraft, screen.composerRule(composerDraft, chevron, bottom-1)
	default:
		read.composer = claudeInput(screen, chevron, bottom)
		composerRule = screen.composerRule(read.composer, chevron, bottom-1)
	}

	if read.activityCause == causeClipped {
		read.rules = append(read.rules, screen.clippedRule())
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
func claudeInput(screen *claudeScreen, chevron, bottom int) composer {
	for r := chevron + 1; r < bottom; r++ {
		if !screen.lines[r].blank() {
			return composerDraft
		}
	}
	cells := screen.lines[chevron].cells
	cells = cells[min(2, len(cells)):]
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
	claudeSlotSpinner                          // a spinner row
	claudeSlotRetry                            // a spinner row counting down to a retry
	claudeSlotWait                             // the end-of-turn row while work the turn launched runs
	claudeSlotUnreadable                       // an unparseable row, which may be spinner chrome
	claudeSlotClipped                          // the walk met the cut
)

var (
	claudeSpinnerShape = regexp.MustCompile(`^[·✢✳✶✻✽*●] \S.*?(?:…|\.\.\.)(?: \(.*)?$`)
	claudeRetryShape   = regexp.MustCompile(`^✻ .*(?: · Retrying in | · will retry in | · next try in |No response from the API after )`)
	claudeWaitShape    = regexp.MustCompile(`^✻ Waiting for(?: \d+ background agents?)?(?: and)?(?: \d+ dynamic workflows?)? to finish$`)
)

// claudeSlot walks up from the composer's top rule to the slot row: past at
// most six notification rows in the composer's top margin, blank rows, and
// one spinner child block, which begins `⎿` at column 2 within 12 rows. A
// live spinner cannot be passed.
func claudeSlot(screen *claudeScreen, top int) (slot int, kind claudeSlotKind) {
	slot = top - 1
	for skipped := 0; skipped < 6; skipped++ {
		line, ok := screen.line(slot)
		if !ok || !claudeNotification(line) {
			break
		}
		slot--
	}
	for screen.blankAt(slot) {
		slot--
	}
	if line, ok := screen.line(slot); ok && (line.col >= 5 || line.col == 2 && strings.HasPrefix(line.plain, "  ⎿")) {
		for child := slot; child > slot-12; child-- {
			row, ok := screen.line(child)
			if !ok {
				if screen.clippedAt(child) {
					return child, claudeSlotClipped
				}
				break
			}
			if row.col == 2 && strings.HasPrefix(row.plain, "  ⎿") {
				slot = child - 1
				break
			}
			if row.col < 5 {
				break
			}
		}
	}
	line, ok := screen.line(slot)
	if !ok {
		if screen.clippedAt(slot) {
			return slot, claudeSlotClipped
		}
		return slot, claudeSlotNone
	}
	// A narrow spinner wraps its details alone onto the row below it.
	if line.col == 0 && strings.HasPrefix(line.plain, "(") && strings.HasSuffix(line.plain, ")") {
		above, ok := screen.line(slot - 1)
		switch {
		case !ok && screen.clippedAt(slot-1):
			return slot - 1, claudeSlotClipped
		case ok && claudeSpinnerShape.MatchString(above.plain) && (strings.HasSuffix(above.plain, "…") || strings.HasSuffix(above.plain, "...")):
			slot, line = slot-1, above
		}
	}
	plain := line.plain
	switch {
	case line.presence != rowParsed:
		return slot, claudeSlotUnreadable
	case line.col != 0:
		return slot, claudeSlotOther
	case claudeWaitShape.MatchString(plain) && strings.ContainsAny(plain, "0123456789"):
		return slot, claudeSlotWait
	case strings.HasPrefix(plain, "✻ ") && claudeCompletion(plain[len("✻ "):]):
		return slot, claudeSlotOther
	case claudeSpinnerShape.MatchString(plain):
		return slot, claudeSlotSpinner
	case claudeRetryShape.MatchString(plain):
		return slot, claudeSlotRetry
	default:
		return slot, claudeSlotOther
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

// claudeLimitCopy is the usage-limit wait's fixed status copy, every default
// wrap-up lead and the default next-line copy (2.3, 2.5).
var claudeLimitCopy = regexp.MustCompile(`Usage limit reached|Your usage limit has reset|Continuing automatically |Continuing shortly · esc to cancel|Press enter to continue`)

// claudeBandLegend is the legend of an optional prompt drawn above the
// composer that takes a single key typed into it (2.3, digit band):
// the column-2 legends, then the column-0 ones.
var claudeBandLegend = regexp.MustCompile(`^(?:` +
	`  (?:1: Bad|y: Yes|1: Yes, run /web-setup|Enter to (?:send|skip) · Esc to (?:clear|skip))\b|` +
	`\(Optional\) Press \[1\] to tell us (?:what went well|what went wrong|more) · /feedback$|` +
	`1 to review · 2 to send · 0 to dismiss|` +
	`Send without reviewing \(full draft \+ env, no transcript\)\? 2 to send · Esc to back$|` +
	`\S.* 1 to review & retry · Esc to dismiss$|` +
	`Turn off Claude-drafted feedback\? 0 to turn off · Esc to keep$)`)

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
	mode    int    // the mode row of an ordinary footer
	classes uint16 // the classes of the mode row's segments, one bit each
	proven  bool   // the mode row proves that no task pill is hidden
	work    int    // a panel row with a running status, or -1
	viewed  int    // a viewed panel row other than main, or -1
	extra   bool   // a panel row other than main
}

var (
	claudePanelMain    = regexp.MustCompile(`^(?:  |❯ )[◯⏺●] main(?: +↑ \d+ more)?$`)
	claudePanelSummary = regexp.MustCompile(`^(?:  |❯ )◯ \d+ idle agents?$`)
	claudePanelMore    = regexp.MustCompile(`^  ↓ \d+ more$`)
	claudePanelAgent   = regexp.MustCompile(`^(?:  |❯ )(?:(?:  )*[├└] )?([◯⏺●]) \S`)
	claudePanelPaused  = regexp.MustCompile(`^(?:  |❯ )⏸ \S`)
	// A running agent or workflow shows its elapsed time, optional progress,
	// token and queue counts; idle, waiting and paused rows show no time.
	claudeRunningStatus = regexp.MustCompile(`\s(?:\d+/\d+ · )?\d+[dhms](?: ?\d+[dhms])*(?: · [↑↓] \S+ tokens)?(?: · \d+ queued)?$`)
)

// claudeReadFooter accounts for the rows below the bottom rule: an ordinary
// footer (opaque statusline rows, the mode row, notification rows, then
// optionally one blank row and the agent panel), bash mode's footer, or the
// help grid. Anything else leaves the surface void.
func claudeReadFooter(screen *claudeScreen, bottom, last int, bash bool) claudeFooter {
	void := claudeFooter{mode: -1, work: -1, viewed: -1}
	if bottom == last {
		return void
	}
	footer := void
	tail := bottom + 1
	first := screen.lines[bottom+1].plain
	switch {
	case bash:
		if first != "  ! for shell mode" {
			return void
		}
		footer.kind, tail = claudeFooterBash, bottom+2
	case strings.HasPrefix(first, "  ! for shell mode "):
		// The help grid replaces the whole footer.
		for r := bottom + 1; r <= last; r++ {
			if strings.Contains(screen.lines[r].plain, "/ for commands") {
				footer.kind = claudeFooterHelp
				return footer
			}
		}
		return void
	default:
		for r := last; r > bottom && footer.mode < 0; r-- {
			if mode, ok := claudeModeFull(screen.lines[r], screen.width); ok {
				footer.kind, footer.mode, tail = claudeFooterOrdinary, r, r+1
				footer.classes, footer.proven = claudeModeEvidence(mode)
			}
		}
		if footer.mode < 0 {
			return void
		}
	}
	r := tail
	for r <= last && claudeNotification(screen.lines[r]) {
		r++
	}
	if r > last {
		return footer
	}
	if !screen.lines[r].blank() {
		return void
	}
	gaps := 0
	for p := r + 1; p <= last; p++ {
		plain := screen.lines[p].plain
		if screen.lines[p].blank() {
			// One blank row may stand for the `more` row of a long panel.
			if gaps++; gaps > 1 || p == r+1 {
				return void
			}
			continue
		}
		if claudePanelMain.MatchString(plain) {
			continue
		}
		footer.extra = true
		agent := claudePanelAgent.FindStringSubmatch(plain)
		switch {
		case claudePanelSummary.MatchString(plain), claudePanelMore.MatchString(plain):
		case agent != nil:
			// An agent row, or a workflow row, which shares the agent's `◯`.
			if agent[1] != "◯" && footer.viewed < 0 {
				footer.viewed = p
			}
			if footer.work < 0 && claudeRunningStatus.MatchString(plain) {
				footer.work = p
			}
		case claudePanelPaused.MatchString(plain):
			// A workflow paused on a rate limit: no work claim.
		default:
			return void
		}
	}
	return footer
}

// claudeSegmentClass is a footer segment's class (2.4).
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

// claudeFooterLabels is the closed footer vocabulary: a <chord> token is any
// key and <n> a count. The mode row ends with the hint item, whose members are
// the interrupt, tail and agents labels and which truncates with `…`; every
// item before it is a box, cut bare, whose labels are those of every class but
// interrupt and tail. The agents labels are both, because claude draws the
// same copy in the bg-detach box.
var claudeFooterLabels = [...]struct {
	text  string
	class claudeSegmentClass
}{
	{"<chord> to interrupt", claudeInterrupt},
	{"<chord> to return to team lead", claudeTail},
	{"<chord> to hide tasks", claudeTail},
	{"<chord> to show tasks", claudeTail},
	{"/tasks to see subagents", claudeTail},
	{"? for shortcuts", claudeTail},
	{"← for agents", claudeAgents},
	{"← <n> agent", claudeAgents},
	{"← <n> agents", claudeAgents},
	{"← <n> done", claudeAgents},
	{"<n> feedback draft", claudeTail},
	{"<n> feedback drafts", claudeTail},
	{"keep holding…", claudeTail},
	{"ctrl+c to copy", claudeTail},
	{"option+click to native select", claudeTail},
	{"shift+click to native select", claudeTail},
	{"set macOptionClickForcesSelection in VS Code settings", claudeTail},
	{"hold <chord> to speak", claudeTail},
	{"↓ to manage", claudeTail},
	{"<chord> to view tasks", claudeTail},
	{"<chord> to view artifacts", claudeTail},
	{"<n> local agent", claudeWork},
	{"<n> local agents", claudeWork},
	{"<n> shell", claudeProcess},
	{"<n> shells", claudeProcess},
	{"<n> shell, <n> monitor", claudeProcess},
	{"<n> shell, <n> monitors", claudeProcess},
	{"<n> shells, <n> monitor", claudeProcess},
	{"<n> shells, <n> monitors", claudeProcess},
	{"<n> monitor", claudeProcess},
	{"<n> monitors", claudeProcess},
	{"<n> Artifact comment monitor", claudeProcess},
	{"<n> Artifact comment monitors", claudeProcess},
	{"dreaming", claudeProcess},
	{"auto-mode scan", claudeProcess},
	{"memory import", claudeProcess},
	{"memory import <n>/<n>", claudeProcess},
	{"<n> MCP task", claudeUnknownPill},
	{"<n> MCP tasks", claudeUnknownPill},
	{"<n> team", claudeUnknownPill},
	{"<n> teams", claudeUnknownPill},
	{"◇ <n> cloud session", claudeUnknownPill},
	{"◇ <n> cloud sessions", claudeUnknownPill},
	{"◇ <n> remote dynamic workflow", claudeUnknownPill},
	{"◇ <n> remote dynamic workflows", claudeUnknownPill},
	{"◆ ultraplan ready", claudeUnknownPill},
	{"◇ ultraplan", claudeUnknownPill},
	{"◇ ultraplan needs your input", claudeUnknownPill},
	{"↓ to view", claudeUnknownPill},
	{"<n> background task", claudeUnknownPill},
	{"<n> background tasks", claudeUnknownPill},
	{"↳ <n> background", claudeUnknownPill},
	{"↳ <n> background (<chord> to manage)", claudeUnknownPill},
	{"PR #<n>", claudeOther},
	{"MR !<n>", claudeOther},
	{"gh auth login for PR status", claudeOther},
	{"install gh for PR status", claudeOther},
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
// key, and <n> for a count (digits, or 99+).
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
		if rest, count := strings.CutPrefix(label, "<n>"); count {
			digits := claudeDigits(token)
			if digits == 0 {
				return false
			}
			token, label = strings.TrimPrefix(token[digits:], "+"), rest
			continue
		}
		if label[0] != token[0] {
			return false
		}
		label, token = label[1:], token[1:]
	}
	return token == ""
}

// claudeExactSegment is the class of the label segment is an exact instance
// of, or unlisted.
func claudeExactSegment(segment string) claudeSegmentClass {
	for _, label := range claudeFooterLabels {
		if claudeInstance(label.text, segment, false) {
			return label.class
		}
	}
	return claudeUnlisted
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

// claudeCutHintClasses is what a hint item cut to text could have been: the
// tail when nothing of it is left, else the classes of the hint item members
// text is a character prefix of.
func claudeCutHintClasses(text string) uint16 {
	if text == "" {
		return 1 << claudeTail
	}
	var classes uint16
	for _, label := range claudeFooterLabels {
		member := label.class == claudeInterrupt || label.class == claudeTail || label.class == claudeAgents
		if member && claudeInstance(label.text, text, true) {
			classes |= 1 << label.class
		}
	}
	return classes
}

// claudeSegment classifies one segment of the fullscreen or classic mode row.
// Only the last segment can be cut: the hint item truncates with `…`, a box is
// cut bare. A cut segment takes the one class it could have been cut from, or
// none.
func claudeSegment(segment string, last bool) (class claudeSegmentClass, exact bool) {
	if class := claudeExactSegment(segment); class != claudeUnlisted {
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
		classes = claudeCutHintClasses(text)
	} else {
		for _, label := range claudeFooterLabels {
			box := label.class != claudeInterrupt && label.class != claudeTail
			if box && claudeInstance(label.text, segment, true) {
				classes |= 1 << label.class
			}
		}
	}
	if claudeIndicatorShape(segment) {
		classes |= 1 << claudeIndicator
	}
	if bits.OnesCount16(classes) != 1 {
		return claudeUnlisted, false
	}
	return claudeSegmentClass(bits.TrailingZeros16(classes)), false
}

// claudeModeRow is a parsed mode row.
type claudeModeRow struct {
	manual   bool
	cycle    bool // the mode item draws its cycle hint, whole or cut
	wrapped  bool // the mode item shows only the first line of its wrapped text
	segments []string
	cut      bool // a trailing ` ·`: a later item was cut away
	blanks   int  // blank cells after the last item, to the row end or the notification suffix
	notified bool // a notification suffix ends the row
}

// claudeModeItem cuts the mode item from the start of text: an optional vim
// prefix, the mode glyph (not in the screen reader), the mode and an optional
// cycle hint. The screen reader never cuts the hint; elsewhere the item is one
// text wrapping in a box one row high, so a narrow row shows only its first
// line: the hint cut after a word, or wrapped away whole. That line keeps its
// trailing space and the box is as wide as the text's widest line, so blank
// cells, at least two counting the separator's own, precede the next `·`.
func claudeModeItem(text string, screenReader bool) (mode claudeModeRow, rest string, ok bool) {
	for _, vim := range [...]string{"-- INSERT -- ", "-- VISUAL -- ", "-- VISUAL LINE -- "} {
		if after, found := strings.CutPrefix(text, vim); found {
			text = after
			break
		}
	}
	if !screenReader {
		after, pause := strings.CutPrefix(text, "⏸ ")
		if !pause {
			if after, ok = strings.CutPrefix(text, "⏵⏵ "); !ok {
				return mode, "", false
			}
		}
		text = after
	}
	for _, name := range [...]string{"manual mode", "plan mode", "accept edits", "auto mode", "bypass permissions", "don't ask"} {
		after, found := strings.CutPrefix(text, name+" on")
		if !found {
			continue
		}
		mode.manual = name == "manual mode"
		hint, hinted := strings.CutPrefix(after, " (")
		if hinted {
			chord, tail, spaced := strings.Cut(hint, " ")
			if rest, complete := strings.CutPrefix(tail, "to cycle)"); spaced && chord != "" && complete {
				mode.cycle = true
				return mode, rest, true
			}
		}
		// With no hint showing, it wrapped away whole when blank cells and
		// then the separator follow `on`. `on` before the separator or a
		// notification suffix has no hint; `on` at the row end is left to P2.
		if !hinted && !(strings.HasPrefix(after, "  ") && strings.HasPrefix(strings.TrimLeft(after, " "), "·")) {
			return mode, after, true
		}
		if screenReader {
			return claudeModeRow{}, "", false
		}
		first, rest := after, ""
		if blanks := strings.Index(after, "  "); blanks >= 0 {
			first, rest = after[:blanks], after[blanks:]
		}
		if hinted && !claudeInstance("<chord> to cycle)", strings.TrimPrefix(first, " ("), true) {
			return claudeModeRow{}, "", false
		}
		if separator := strings.TrimLeft(rest, " "); strings.HasPrefix(separator, "·") {
			rest = " " + separator
		}
		mode.cycle, mode.wrapped = true, true
		return mode, rest, true
	}
	return claudeModeRow{}, "", false
}

// claudeModeFull parses the fullscreen and classic mode row: two cells of
// padding, the mode item, ` · `-separated segments, an optional trailing
// ` ·`, and a right-aligned notification suffix after three or more spaces.
func claudeModeFull(line claudeLine, width int) (claudeModeRow, bool) {
	text, padded := strings.CutPrefix(line.plain, "  ")
	if !padded {
		return claudeModeRow{}, false
	}
	mode, items, ok := claudeModeItem(text, false)
	if !ok {
		return mode, false
	}
	items, suffix, notified := strings.Cut(items, "   ")
	mode.notified = notified
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

// claudeModeEvidence classifies the mode row's segments and decides whether
// the row proves the pill slot (2.4, P0-P2): a visible task pill is
// never hidden behind what the row shows.
func claudeModeEvidence(mode claudeModeRow) (classes uint16, proven bool) {
	// P0: claude draws the cycle hint in a non-default mode only without a
	// pill, so any of the hint proves the slot.
	proven = !mode.manual && mode.cycle
	lastExact := !mode.wrapped // the mode item itself, when no segment follows
	for index, segment := range mode.segments {
		last := index == len(mode.segments)-1
		class, exact := claudeSegment(segment, last)
		classes |= 1 << class
		// P1: a complete tail label means the hint item was laid out, which
		// happens only after every box before it has its full width. A
		// complete agents label may be the bg-detach box instead, and an
		// interrupt label already means work.
		if exact && class == claudeTail {
			proven = true
		}
		if last {
			lastExact = exact && !strings.HasSuffix(segment, "…")
			// P1 too: a cut hint item is the hint item, laid out last.
			if text, truncated := claudeHintCut(segment); truncated {
				proven = proven || claudeCutHintClasses(text) != 0
			}
		}
	}
	// P2: a complete label with room after it left no box unshown. The mode
	// item's separator is a box of its own whose `·` shows once 4 cells
	// follow the item to the row end, and in a non-default mode an item that
	// shows no more than `on` there may be its hint wrapped away (P0).
	room := 6
	if len(mode.segments) == 0 && !mode.manual && !mode.notified {
		room = 4
	}
	return classes, proven || !mode.cut && lastExact && mode.blanks >= room
}

// claudeSeparatorSR is a separator of the screen reader's flattened boxes.
var claudeSeparatorSR = regexp.MustCompile(`\s+·\s+`)

// claudeModeSR parses the screen reader's mode row into its segments: its
// boxes join with one space, so their separators read `  ·  `; it never cuts
// an item.
func claudeModeSR(plain string) ([]string, bool) {
	_, rest, ok := claudeModeItem(claudeSeparatorSR.ReplaceAllString(plain, " · "), true)
	if !ok {
		return nil, false
	}
	if rest == "" {
		return nil, true
	}
	body, separated := strings.CutPrefix(rest, " · ")
	if !separated {
		return nil, false
	}
	return strings.Split(body, " · "), true
}

// claudeFamily reads the request and menu families (2.6), which
// replace the composer, in their table order. Their hint block or options end
// at the hint row, the lowest row once at most two trailing right-aligned rows
// are passed; every column is exact, so quoted transcript text cannot reach
// them.
func claudeFamily(screen *claudeScreen, last int) (reading, bool) {
	hint := last
	for skipped := 0; skipped < 2 && screen.lines[hint].col >= 8; skipped++ {
		hint--
		for screen.blankAt(hint) {
			hint--
		}
		if _, ok := screen.line(hint); !ok {
			return reading{}, false
		}
	}
	for _, family := range [...]func(*claudeScreen, int) (string, sessions.Interaction, int, bool){
		claudeViewer, claudeMenu, claudePermission, claudeQuestion, claudeElicitation, claudePlan, claudeSetup,
	} {
		if id, interaction, first, ok := family(screen, hint); ok {
			return reading{
				activity: sessions.ActivityUnknown, interaction: interaction, composer: composerBlocked,
				rules: []DiagnosticRule{screen.rule(id, first, hint), screen.composerRule(composerBlocked, first, hint)},
			}, true
		}
	}
	return reading{}, false
}

// The agents view's hint and its counts row (2.6).
var (
	claudeAgentsViewHint   = regexp.MustCompile(`^(?:.* · )?enter to return(?: · |$)`)
	claudeAgentsViewCounts = regexp.MustCompile(`\d+ awaiting input · \d+ working · \d+ completed`)
)

// claudeViewer reads the viewer footers: the transcript viewer, with a dialog
// waiting behind it or not, and the agents view.
func claudeViewer(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	text, top, ok := screen.block(2, hint)
	switch {
	case !ok:
	case strings.HasPrefix(text, "dialog waiting · Showing detailed transcript · "):
		return "claude.input.dialog_waiting", sessions.InteractionInput, top, true
	case strings.HasPrefix(text, "Showing detailed transcript · "):
		return "claude.menu.transcript", sessions.InteractionMenu, top, true
	case claudeAgentsViewHint.MatchString(text):
		if row, found := screen.find(top-1, func(line claudeLine) bool { return claudeAgentsViewCounts.MatchString(line.plain) }); found {
			return "claude.menu.agents_view", sessions.InteractionMenu, row, true
		}
	}
	return "", "", 0, false
}

// claudeSettingsTabs is the settings menu's tab row.
var claudeSettingsTabs = regexp.MustCompile(`^   Settings +Status +Config +Usage`)

// claudeMenu reads the model, settings, theme, resume and side-question menus.
func claudeMenu(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, _, ok := screen.block(3, hint); ok {
		switch {
		case strings.HasPrefix(text, "Enter to set as default · "):
			if row, found := screen.find(hint-1, screen.edge); found {
				return "claude.menu.model", sessions.InteractionMenu, row, true
			}
		case strings.HasPrefix(text, "Type to filter · ") || strings.HasPrefix(text, "Enter/Space to change · "):
			if row, found := screen.find(hint-1, func(line claudeLine) bool { return claudeSettingsTabs.MatchString(line.plain) }); found {
				return "claude.menu.settings", sessions.InteractionMenu, row, true
			}
		case text == "Enter to select · Esc to cancel":
			if row, found := screen.find(hint-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    Syntax theme: ") }); found {
				return "claude.menu.theme", sessions.InteractionMenu, row, true
			}
		}
	}
	if text, top, ok := screen.block(5, hint); ok && strings.Contains(text, "to show all projects") && strings.HasSuffix(text, "Esc to cancel") {
		return "claude.menu.resume", sessions.InteractionMenu, top, true
	}
	if text, top, ok := screen.block(4, hint); ok && strings.HasSuffix(text, "Esc to close") {
		_, question := screen.find(top-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    /btw ") })
		if row, found := screen.find(top-1, screen.edge); found && question {
			return "claude.menu.btw", sessions.InteractionMenu, row, true
		}
	}
	return "", "", 0, false
}

// The tool and workflow permission hints; a <chord> is any one token.
var (
	claudePermissionHint = regexp.MustCompile(`^\S+ to cancel(?: · \S+ to amend)?$`)
	claudeWorkflowHint   = regexp.MustCompile(`^\S+ to cancel(?: · \S+ to amend)?(?: \S+ to edit script in \$EDITOR)?$`)
)

// claudePermission reads the tool permission dialog (options and hint at
// column 1) and the dynamic workflow permission (column 2, titled at column 1).
func claudePermission(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, top, ok := screen.block(1, hint); ok && claudePermissionHint.MatchString(text) {
		options, optionsTop, ok := screen.options(1, top-1)
		if ok && strings.HasPrefix(options[0].label, "Yes") {
			if _, questionTop, end, ok := screen.above(1, optionsTop); ok {
				for r := end; r >= questionTop; r-- {
					if strings.HasPrefix(screen.lines[r].plain[screen.lines[r].col:], "Do you want to ") {
						if strings.HasSuffix(screen.joined(r, end), "?") {
							return "claude.permission.dialog", sessions.InteractionPermission, r, true
						}
						break
					}
				}
			}
		}
	}
	if text, top, ok := screen.block(2, hint); ok && claudeWorkflowHint.MatchString(text) {
		options, optionsTop, ok := screen.options(2, top-1)
		if ok && options[0].label == "Yes, run it" && options[len(options)-1].label == "No" {
			if rule, found := screen.titled(optionsTop, " Run a dynamic workflow?"); found {
				return "claude.permission.workflow", sessions.InteractionPermission, rule, true
			}
		}
	}
	return "", "", 0, false
}

// claudeQuestion reads the question form, the question with option previews,
// and the submit review.
func claudeQuestion(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, top, ok := screen.block(0, hint); ok && strings.HasPrefix(text, "Enter to select · ") && screen.blankAt(top-1) {
		chat, chatOK := screen.line(top - 2)
		rule, ruleOK := screen.line(top - 3)
		if chatOK && ruleOK && screen.ruleRow(rule) {
			switch {
			case strings.Contains(text, " · n to add notes · ") && strings.HasSuffix(text, "Esc to cancel") &&
				(chat.plain == "❯ Chat about this" || chat.plain == "  Chat about this"):
				if first, ok := claudePreview(screen, top-3, strings.HasPrefix(chat.plain, "❯")); ok {
					return "claude.question.preview", sessions.InteractionQuestion, first, true
				}
			case strings.Contains(text, " to navigate") && strings.HasSuffix(text, "to cancel") && !strings.Contains(text, "n to add notes"):
				number, label, pointer, start := claudeOptionStart(chat.plain, 0)
				options, optionsTop, ok := screen.options(0, top-4)
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
	options, _, ok := screen.options(0, hint)
	if ok && len(options) == 2 && options[0].label == "Submit answers" && options[1].label == "Cancel" {
		_, ready := screen.textBlock(0, hint, func(text string) bool { return strings.Contains(text, "Ready to submit your answers?") })
		if review, found := screen.textBlock(0, hint, func(text string) bool { return strings.Contains(text, "Review your answers") }); found && ready {
			return "claude.question.review", sessions.InteractionQuestion, review, true
		}
	}
	return "", "", 0, false
}

// claudePreview reads a question's side-by-side options and preview box above
// its rule. The box's corner `┌` is on the row of option 1, the block's first
// row; a lower `┌` belongs to a box the preview's own markdown draws, so the
// corner is the lowest one whose cells to its left start option 1. The cells
// left of the corner's column, from its row down, are the options. It returns
// the corner's row, and holds when a pointer marks an option or the chat row.
func claudePreview(screen *claudeScreen, rule int, chatPointer bool) (int, bool) {
	left := func(r, column int) claudeLine {
		cut := screen.lines[r].row
		cut.cells = cut.cells[:min(column, len(cut.cells))]
		return newClaudeLine(cut)
	}
	for corner := rule - 1; ; corner-- {
		line, ok := screen.line(corner)
		if !ok {
			return 0, false
		}
		column := slices.IndexFunc(line.cells, func(cell cell) bool { return cell.text == "┌" })
		if column < 0 {
			continue
		}
		if number, _, _, start := claudeOptionStart(left(corner, column).plain, 0); !start || number != 1 {
			continue
		}
		var lefts []claudeLine
		for r := corner; r < rule; r++ {
			cut := left(r, column)
			if _, _, _, start := claudeOptionStart(cut.plain, 0); !start && (cut.blank() || cut.col < 5) {
				break
			}
			lefts = append(lefts, cut)
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
}

// claudeElicitationTitle is an mcp server's elicitation title: the server
// name, then the rest of its block.
var claudeElicitationTitle = regexp.MustCompile(`^MCP server “.*?” (.*)$`)

// claudeElicitation reads an mcp server's elicitation form. Its title starts a
// block at column 2: classic wraps it, while fullscreen keeps it on one row,
// cutting the server name and then the title's ending to a prefix and `…`.
func claudeElicitation(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	if text, _, ok := screen.block(2, hint); ok && strings.HasPrefix(text, "Esc to cancel · ") {
		title := func(text string) bool {
			match := claudeElicitationTitle.FindStringSubmatch(text)
			if match == nil {
				return false
			}
			rest, cut := strings.CutSuffix(match[1], "…")
			return slices.ContainsFunc([]string{"requests your input", "wants to open a URL"}, func(ending string) bool {
				return strings.HasPrefix(match[1]+" ", ending+" ") || cut && strings.HasPrefix(ending, rest)
			})
		}
		if row, found := screen.textBlock(2, hint, title); found {
			return "claude.input.elicitation", sessions.InteractionInput, row, true
		}
	}
	return "", "", 0, false
}

// claudePlanEditHint is plan approval's optional editor hint.
var claudePlanEditHint = regexp.MustCompile(`^\S+ to edit in `)

// claudePlan reads plan approval and the exit- and enter-plan decisions.
func claudePlan(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	end := hint
	if text, top, ok := screen.block(3, hint); ok && claudePlanEditHint.MatchString(text) {
		end = top - 1
	}
	if options, optionsTop, ok := screen.options(3, end); ok && strings.HasPrefix(options[0].label, "Yes") {
		change := false
		for _, option := range options {
			change = change || strings.HasPrefix(option.label, "Tell Claude what to change")
		}
		if text, top, _, ok := screen.above(3, optionsTop); ok && change && strings.HasPrefix(text, "Claude has written up a plan and is ready to execute.") {
			return "claude.confirmation.plan", sessions.InteractionConfirmation, top, true
		}
	}
	if options, optionsTop, ok := screen.options(4, hint); ok && strings.HasPrefix(options[0].label, "Yes, and switch to ") && options[len(options)-1].label == "No" {
		if text, top, _, ok := screen.above(4, optionsTop); ok && text == "Claude wants to exit plan mode" {
			return "claude.confirmation.exit_plan", sessions.InteractionConfirmation, top, true
		}
	}
	// Entering plan mode draws no hint; its confirm label names the mode.
	if options, optionsTop, ok := screen.unnumbered(2, hint); ok && strings.HasSuffix(options, "No, start implementing now") {
		if rule, found := screen.titled(optionsTop, " Enter plan mode?"); found {
			return "claude.confirmation.enter_plan", sessions.InteractionConfirmation, rule, true
		}
	}
	return "", "", 0, false
}

// The multi-server approval's hint and title, and an unnumbered theme choice.
var (
	claudeServerHint   = regexp.MustCompile(`^\S+ to select · \S+ to reject all$`)
	claudeServersTitle = regexp.MustCompile(`^\d+ new MCP servers found in this project`)
	claudeThemeOption  = regexp.MustCompile(`^ (?:❯ |    )\S`)
)

// claudeSetup reads the dialogs before and around the first prompt. Trust is
// drawn at column 1; claude's shared dialog draws its options and hint at
// column 2 under a full rule, and its variants differ by their anchors.
func claudeSetup(screen *claudeScreen, hint int) (string, sessions.Interaction, int, bool) {
	const confirm = "Enter to confirm · Esc to cancel"
	hint1, top1, ok1 := screen.block(1, hint)
	hint2, top2, ok2 := screen.block(2, hint)
	if ok1 && hint1 == confirm {
		options, optionsTop, ok := screen.unnumbered(1, top1-1)
		_, workspace := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Accessing workspace:") })
		if ok && workspace && claudeWords(options, "No, exit") && claudeWords(options, "Yes, I trust this folder") {
			if rule, found := screen.find(optionsTop-1, screen.ruleRow); found {
				return "claude.setup.trust", sessions.InteractionSetup, rule, true
			}
		}
	}
	if ok2 && hint2 == confirm {
		_, apiKey := screen.textBlock(2, hint, func(text string) bool { return strings.Contains(text, "Detected a custom API key in your environment") })
		_, useKey := screen.textBlock(2, hint, func(text string) bool { return strings.Contains(text, "Do you want to use this API key?") })
		_, bypass := screen.textBlock(2, hint, func(text string) bool {
			return strings.HasPrefix(text, "WARNING: Claude Code running in Bypass Permissions mode")
		})
		_, server := screen.textBlock(2, hint, func(text string) bool { return strings.HasPrefix(text, "New MCP server found in this project: ") })
		_, settings := screen.textBlock(2, hint, func(text string) bool { return strings.HasPrefix(text, "Settings Error") })
		options, optionsTop, unnumbered := screen.unnumbered(2, top2-1)
		numbered, numberedTop, ok := screen.options(2, top2-1)
		labels := map[string]bool{}
		for _, option := range numbered {
			labels[option.label] = true
		}
		var id string
		switch {
		case unnumbered && apiKey && useKey:
			id = "claude.setup.apikey"
		case unnumbered && bypass && claudeWords(options, "No, exit") && claudeWords(options, "Yes, I accept"):
			id = "claude.setup.bypass"
		case unnumbered && server && claudeWords(options, "Use this MCP server") && claudeWords(options, "Continue without using this MCP server"):
			id = "claude.setup.mcp_server"
		case ok && settings && labels["Exit and fix manually"] && labels["Continue without these settings"]:
			id, optionsTop = "claude.setup.settings_error", numberedTop
		}
		if id != "" {
			if rule, found := screen.find(optionsTop-1, screen.ruleRow); found {
				return id, sessions.InteractionSetup, rule, true
			}
		}
	}
	if ok1 && claudeServerHint.MatchString(hint1) && claudeServerApproval(screen, top1) {
		_, found := screen.textBlock(2, hint, claudeServersTitle.MatchString)
		if rule, ruled := screen.find(top1-1, screen.ruleRow); found && ruled {
			return "claude.setup.mcp_servers", sessions.InteractionSetup, rule, true
		}
	}
	if ok2 && strings.HasPrefix(hint2, "Syntax theme: ") {
		// The theme choices sit at column 1 above the syntax preview: unnumbered
		// (2.1.286), or options numbered from 1 whose labels wrap at column 6
		// (2.1.284). The run takes rows of either form and any row at column 6 or
		// right of it, where only a numbered label continues (claudeParseOptions'
		// c+5); claude draws one form per version, so a run mixing them is not
		// the picker.
		numberedRow := func(line claudeLine) bool {
			_, _, _, start := claudeOptionStart(line.plain, 1)
			return start
		}
		theme := func(line claudeLine) bool { return numberedRow(line) || claudeThemeOption.MatchString(line.plain) }
		if bottom, found := screen.find(top2-1, theme); found {
			top, selected := bottom, 0
			for {
				if strings.HasPrefix(screen.lines[top].plain, " ❯ ") {
					selected++
				}
				line, ok := screen.line(top - 1)
				if !ok || !theme(line) && line.col < 6 {
					break
				}
				top--
			}
			choices := screen.lines[top : bottom+1]
			_, fromOne := claudeParseOptions(choices, 1)
			oneForm := fromOne || !slices.ContainsFunc(choices, func(line claudeLine) bool { return numberedRow(line) || line.col >= 6 })
			_, started := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Let's get started.") })
			if started && oneForm && selected == 1 && claudeWords(screen.joined(top, bottom), "Dark mode (ANSI colors only)") {
				return "claude.setup.theme", sessions.InteractionSetup, top, true
			}
		}
	}
	if options, optionsTop, ok := screen.options(1, hint); ok && strings.HasPrefix(options[0].label, "Claude account with subscription") {
		if row, found := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Select login method:") }); found {
			return "claude.setup.login", sessions.InteractionSetup, min(row, optionsTop), true
		}
	}
	if ok1 && hint1 == "Press Enter to continue…" {
		if row, found := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Security notes:") }); found {
			return "claude.setup.security", sessions.InteractionSetup, row, true
		}
	}
	return "", "", 0, false
}

// claudeServerOption is a server's checkbox row.
var claudeServerOption = regexp.MustCompile(`^(?:  ❯ |    )\[[ ✔]\] \S`)

// claudeServerApproval reads the multi-server approval above its hint at row
// hint: the submit row and the checkbox rows above it, one pointer at most.
func claudeServerApproval(screen *claudeScreen, hint int) bool {
	submit, ok := screen.line(hint - 1)
	if !ok || submit.plain != "       Enable selected" && submit.plain != "  ❯    Enable selected" {
		return false
	}
	pointers, boxes := strings.Count(submit.plain, "❯"), 0
	for r := hint - 2; ; r-- {
		line, ok := screen.line(r)
		if !ok || !claudeServerOption.MatchString(line.plain) {
			break
		}
		if strings.HasPrefix(line.plain, "  ❯ ") {
			pointers++
		}
		boxes++
	}
	return boxes > 0 && pointers <= 1
}
