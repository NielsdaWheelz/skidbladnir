package agentcontrol

import (
	"math/bits"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

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

	layout := "claude.layout.composer"
	footer := claudeReadFooter(screen, bottom, last, bash)
	if footer.kind == claudeFooterVoid {
		// Anything the grammar cannot account for may be drawn over this
		// surface: a dead process's frame, autocomplete, a focused panel.
		read := unknownReading(causeUnrecognized)
		read.rules = []string{layout}
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
	if ordinary && footer.kind == claudeFooterOrdinary {
		read.usage, read.footer = claudeReadStatusline(screen, bottom, footer.mode)
	}
	var activity string
	// The panel, the waiting row and a work pill are independent of the
	// chevron and the slot, so they decide before any conflict.
	switch {
	case footer.work, kind == claudeSlotWait, footer.classes&(1<<claudeWork) != 0:
		read.activity, activity = sessions.ActivityWorking, "claude.activity.background"
	case corroborator && ready:
		read.activityCause, activity = causeConflict, "claude.activity.conflict"
	case loading && kind == claudeSlotSpinner:
		read.activity, activity = sessions.ActivityWorking, "claude.activity.spinner"
	case loading && kind == claudeSlotRetry:
		read.activity, activity = sessions.ActivityWorking, "claude.activity.retry"
	case interrupt:
		read.activity, activity = sessions.ActivityWorking, "claude.activity.hint"
	case corroborator:
		// Spinner chrome under a chevron whose style says nothing: unknown.
	case kind == claudeSlotClipped:
		read.activityCause = causeClipped
	case complete:
		read.activity, activity = sessions.ActivityIdle, "claude.activity.idle"
	}

	var family string
	switch {
	case waiting:
		read.interaction, family = sessions.InteractionUnknown, "claude.limit.wait"
	case !ordinary:
		read.interaction = sessions.InteractionUnknown
	case footer.viewed:
		read.interaction, family = sessions.InteractionMenu, "claude.menu.subagent"
	case footer.kind == claudeFooterHelp:
		read.interaction, family = sessions.InteractionMenu, "claude.menu.help"
	default:
		read.interaction = sessions.InteractionNone
	}

	var composerRule string
	switch {
	case read.interaction == sessions.InteractionMenu:
		read.composer = composerBlocked
	case read.interaction == sessions.InteractionUnknown:
	case kind == claudeSlotOther && claudeBandLegend.MatchString(screen.lines[slot].plain):
		composerRule = "claude.composer.band" // a band sets no composer
	case bash:
		read.composer = composerDraft
	default:
		read.composer = claudeInput(screen, chevron, bottom)
	}
	if read.composer != composerUnknown {
		composerRule = "claude.composer." + string(read.composer)
	}

	// Notice (2.8): the latest turn's interruption or error marker.
	var notice string
	if read.notice = claudeNotice(screen, top); read.notice != "" {
		notice = "claude.notice." + string(read.notice)
	}

	if read.activityCause == causeClipped {
		read.rules = append(read.rules, "claude.region.clipped")
	}
	read.rules = append(read.rules, layout)
	for _, rule := range [...]string{family, activity, composerRule, notice} {
		if rule != "" {
			read.rules = append(read.rules, rule)
		}
	}
	return read, true
}

// claudeMargin is the first row above the composer's top margin: past at most
// six notification rows above the top rule at top, then blank rows.
func claudeMargin(screen *claudeScreen, top int) int {
	row := top - 1
	for skipped := 0; skipped < 6; skipped++ {
		line, ok := screen.line(row)
		if !ok || !claudeNotification(line) {
			break
		}
		row--
	}
	for screen.blankAt(row) {
		row--
	}
	return row
}

// claudeNotice reads how the latest turn ended (2.8): the interruption row as
// the transcript's last row above the composer, or a failure block as the last
// block above the turn's completion row. Claude draws an API error, a model or
// usage failure and other turn-ending warnings as a `⏺` block whose glyph and
// text share one explicit colour; replies, tool calls and agent notices never
// colour both alike. A later prompt or turn moves an older marker away from the
// composer, so only the latest one counts. No notice is zero.
func claudeNotice(screen *claudeScreen, top int) sessions.Notice {
	row := claudeMargin(screen, top)
	line, ok := screen.line(row)
	if !ok {
		return ""
	}
	if line.col == 2 && strings.HasPrefix(line.plain, "  ⎿  Interrupted") {
		return sessions.NoticeInterrupted
	}
	if line.col == 0 && strings.HasPrefix(line.plain, "✻ ") && claudeCompletion(line.plain[len("✻ "):]) {
		row--
		for screen.blankAt(row) {
			row--
		}
	}
	// The block's head is the column-0 row above its wrapped continuation rows.
	for continued := 0; continued < 12; continued++ {
		if line, ok = screen.line(row); !ok || line.col != 2 || strings.HasPrefix(line.plain, "  ⎿") {
			break
		}
		row--
	}
	if !ok || line.col != 0 || !strings.HasPrefix(line.plain, "⏺ ") {
		return ""
	}
	glyph := line.cells[0].style.fg
	if glyph.kind == colorDefault || slices.ContainsFunc(line.cells[2:], func(cell cell) bool {
		return cell.text != " " && cell.text != "" && cell.style.fg != glyph
	}) {
		return ""
	}
	return sessions.NoticeError
}

// claudeInput reads the ordinary composer's input row and its continuation
// rows: empty when only a drawn cursor or a dim placeholder follows the
// prompt. Placeholders are known by their dim style, never by their words;
// default-style spaces do not change their visible styling.
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
		if !cell.style.dim && (cell.text != " " || cell.style != (style{})) {
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

// claudeSlot walks up from the composer's top rule to the slot row: past the
// top margin and one spinner child block, which begins `⎿` at column 2 within
// 12 rows. A live spinner cannot be passed.
func claudeSlot(screen *claudeScreen, top int) (slot int, kind claudeSlotKind) {
	slot = claudeMargin(screen, top)
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
// wrap-up lead and the default next-line copy (2.3).
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
	work    bool   // a panel row has a running status
	viewed  bool   // a panel row other than main is viewed
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
	void := claudeFooter{mode: -1}
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
			footer.viewed = footer.viewed || agent[1] != "◯"
			footer.work = footer.work || claudeRunningStatus.MatchString(plain)
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
	for _, label := range claudeFooterLabels {
		if claudeInstance(label.text, segment, false) {
			return label.class, true
		}
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
	segments []string
	cut      bool // a trailing ` ·`: a later item was cut away
	blanks   int  // blank cells after the last item, to the row end or the notification suffix
	notified bool // a notification suffix ends the row
}

// claudeModeItem cuts the mode item from the start of text: an optional vim
// prefix, the mode glyph, the mode and an optional cycle hint. Manual mode
// never draws the hint; elsewhere the item is one text wrapping in a box one
// row high, so a narrow row shows only its first line: the hint cut, read like
// a cut segment (earlier tokens whole, the last a character prefix), or
// wrapped away whole. That line keeps its trailing space and the box is as wide as the
// text's widest line, so blank cells, at least two counting the separator's
// own, precede the next `·`.
func claudeModeItem(text string) (mode claudeModeRow, rest string, ok bool) {
	for _, vim := range [...]string{"-- INSERT -- ", "-- VISUAL -- ", "-- VISUAL LINE -- "} {
		if after, found := strings.CutPrefix(text, vim); found {
			text = after
			break
		}
	}
	after, pause := strings.CutPrefix(text, "⏸ ")
	if !pause {
		if after, ok = strings.CutPrefix(text, "⏵⏵ "); !ok {
			return mode, "", false
		}
	}
	text = after
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
		if mode.manual {
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
		mode.cycle = true
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
	mode, items, ok := claudeModeItem(text)
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
	lastExact := true // the mode item itself, when no segment follows
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
	// follow a hint-less item to the row end; a hinted item's box may hold
	// those cells itself, but then its hint proves the slot (P0).
	room := 6
	if len(mode.segments) == 0 && !mode.notified {
		room = 4
	}
	return classes, proven || !mode.cut && lastExact && mode.blanks >= room
}
