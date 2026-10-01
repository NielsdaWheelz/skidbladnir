package agentcontrol

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// The codex grammar is research/codex.md revision 4, sections 2-4, for codex
// 0.159.2. E (end) is the last non-blank row; every surface is anchored on it
// and tried in the grammar's order, and the first that matches decides. Rows
// are pane rows, so a read above the bottom region continues into the top
// region by index, and a row the capture dropped stops it: an absent required
// row is clipped evidence. Codex draws no current notice structure, so the
// notice is always none (2.9). Rules are listed in byte order of their ids.

const (
	codexMainPlaceholder = "Ask Codex to do anything"
	codexSidePlaceholder = "Ask a follow-up question"
)

var (
	codexRed     = color{kind: colorPalette, value: 1}
	codexMagenta = color{kind: colorPalette, value: 5}
	codexCyan    = color{kind: colorPalette, value: 6}

	// codexDuration is the status row's elapsed time (fmt_elapsed_compact).
	codexDuration = regexp.MustCompile(`^(\d+s|\d+m \d\ds|\d+h \d\dm \d\ds)`)
	// codexClock starts a completion separator without its `Worked for` part:
	// an optional date, then the 12- or 24-hour clock time.
	codexClock = regexp.MustCompile(`^((Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{1,2}(, \d{4})? at )?\d{1,2}:\d\d`)
	// codexImageLabels is a row of attached-image labels.
	codexImageLabels = regexp.MustCompile(`^\[Image #\d+\]( \[Image #\d+\])*$`)
	// codexQuestionCount is the collapsed async questions' bold count.
	codexQuestionCount = regexp.MustCompile(`^\d+ questions?$`)
	// codexOption is a numbered form option read from column 4; its label ends
	// at the double space before a description.
	codexOption = regexp.MustCompile(`^\d+\. (\S+(?: \S+)*)`)
	// codexResumeRule is the resume picker's rule with its progress label.
	codexResumeRule = regexp.MustCompile(`^─+ (\d+ ?/ ?\d+…? · )?\d+% ─$`)
)

func detectCodex(screen screen) reading {
	end := len(screen.rows) - 1
	for end >= 0 && screen.rows[end].blank() {
		end--
	}
	switch {
	case end < 0 || screen.rows[end].presence == rowUnparseable:
		return codexUnknown()
	case screen.rows[end].presence == rowAbsent:
		// The screen continues into rows the capture dropped (2.0 step 1).
		read := codexUnknown()
		read.activityCause, read.interactionCause = causeClipped, causeClipped
		return read
	}
	// 2.0 step 4: the surfaces in the grammar's order; the first match wins.
	read, ok := codexApproval(screen, end)
	if !ok {
		read, ok = codexForm(screen, end)
	}
	if !ok {
		read, ok = codexAsyncEditor(screen, end)
	}
	if !ok {
		read, ok = codexResume(screen, end)
	}
	if !ok {
		read, ok = codexPager(screen, end)
	}
	if !ok {
		read, ok = codexWarnings(screen, end)
	}
	if !ok {
		read, ok = codexLogin(screen, end)
	}
	if !ok {
		read, ok = codexPicker(screen, end)
	}
	if !ok {
		read, ok = codexComposer(screen, end)
	}
	if !ok {
		return codexUnknown()
	}
	slices.SortFunc(read.rules, func(a, b DiagnosticRule) int { return strings.Compare(a.ID, b.ID) })
	return read
}

func codexUnknown() reading {
	return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}
}

// codexOverlay is a surface that replaces the composer, status row and status
// line: activity is unknown and nothing is carried forward (2.0 step 5).
func codexOverlay(interaction sessions.Interaction, rules ...DiagnosticRule) reading {
	return reading{activity: sessions.ActivityUnknown, interaction: interaction, notice: sessions.NoticeNone,
		composer: composerBlocked, rules: rules}
}

// codexApproval is rule 1, the `Press` footer (2.3). It word-wraps upward
// from E, so its first row is the nearest one starting `Press `.
func codexApproval(screen screen, end int) (reading, bool) {
	if !codexIndented(screen.rows[end]) {
		return reading{}, false
	}
	first := end
	for !strings.HasPrefix(codexText(codexCells(screen.rows[first], 2)), "Press ") {
		first--
		if first < 0 || !codexIndented(screen.rows[first]) || screen.rows[first].blank() {
			return reading{}, false
		}
	}
	// Join the wrapped rows; the space at a break belongs to the text side.
	var cells []cell
	for index := first; index <= end; index++ {
		next := codexCells(screen.rows[index], 2)
		if len(cells) > 0 {
			style := cells[len(cells)-1].style
			if style.bold && !next[0].style.bold {
				style = next[0].style
			}
			cells = append(cells, cell{text: " ", style: style})
		}
		cells = append(cells, next...)
	}
	// Press K <label>[ or K <label>]*: keys are bold (user verification dims
	// them too); the text is all dim, or all plain in the legacy confirm.
	runs := codexRuns(cells)
	if len(runs) < 3 || len(runs)%2 == 0 || runs[0].text != "Press " {
		return reading{}, false
	}
	dim, plain := runs[0].dim, runs[0].plain
	var labels []string
	for index := 2; index < len(runs); index += 2 {
		label, joined := runs[index].text, true
		if index+1 < len(runs) {
			label, joined = strings.CutSuffix(label, " or ")
		}
		label, spaced := strings.CutPrefix(label, " ")
		if !joined || !spaced {
			return reading{}, false
		}
		labels = append(labels, label)
		dim, plain = dim && runs[index].dim, plain && runs[index].plain
	}
	switch last := labels[len(labels)-1]; {
	case dim && (last == "to cancel" || last == "to open thread"):
		selected, found := codexOptionScan(screen, first-1)
		if !found {
			return reading{}, false
		}
		return codexOverlay(sessions.InteractionPermission, screen.rule("codex.permission.overlay", selected, end)), true
	case plain && slices.Equal(labels, []string{"to confirm", "to go back"}):
		return codexOverlay(sessions.InteractionQuestion, screen.rule("codex.question.legacy_confirm", first, end)), true
	default:
		// `to confirm` alone (cancel unbound), reserve's `to continue working`
		// and login's `to continue` fall through.
		return reading{}, false
	}
}

// codexForm is rules 2 and 3, the ` | ` footers of the mcp form and the legacy
// request_user_input view (2.4). They pack whole hints per row, so the footer
// is the run of hint rows ending at E, and every member starts with a key.
func codexForm(screen screen, end int) (reading, bool) {
	first := codexHintRows(screen, end)
	if first < 0 {
		return reading{}, false
	}
	var keys, labels []string
	for index := first; index <= end; index++ {
		for _, member := range codexSplit(codexCells(screen.rows[index], 2), " | ", true) {
			key := codexKey(member)
			if key == 0 {
				return reading{}, false
			}
			keys, labels = append(keys, codexText(member[:key])), append(labels, codexText(member[key:]))
		}
	}
	last := len(labels) - 1
	submit := slices.IndexFunc(labels, func(label string) bool {
		return label == " to submit" || label == " to submit all" || label == " to submit answer"
	})
	switch {
	case keys[last] == "esc" && labels[last] == " to cancel" && submit >= 0:
		if labels[submit] == " to submit" && codexApprovalActions(screen, first-1) {
			return codexOverlay(sessions.InteractionPermission, screen.rule("codex.permission.mcp_approval", first, end)), true
		}
		return codexOverlay(sessions.InteractionInput, screen.rule("codex.input.mcp_form", first, end)), true
	case slices.ContainsFunc(labels, func(label string) bool { return label == " to submit answer" || label == " to submit all" }) &&
		!slices.Contains(labels, " to cancel"):
		return codexOverlay(sessions.InteractionQuestion, screen.rule("codex.question.legacy", first, end)), true
	default:
		return reading{}, false
	}
}

// codexApprovalActions reports an mcp form in approval-action mode: the
// option block above the footer holds numbered options, all approval actions.
func codexApprovalActions(screen screen, start int) bool {
	index := start
	for index >= 0 && screen.rows[index].blank() {
		index--
	}
	options := 0
	for ; index >= 0 && screen.rows[index].presence == rowParsed && !screen.rows[index].blank(); index-- {
		row := screen.rows[index]
		marker := row.cell(2).text + row.cell(3).text
		match := codexOption.FindStringSubmatch(codexText(codexCells(row, 4)))
		if !codexIndented(row) || marker != "› " && marker != "  " || match == nil {
			continue
		}
		switch match[1] {
		case "Allow", "Allow for this session", "Always allow", "Deny", "Cancel":
			options++
		default:
			return false
		}
	}
	return options > 0
}

// codexAsyncEditor is rule 4, the expanded async questions editor (2.5). It
// replaces the composer and status line; the status row stays above it. Its
// hints are separated by three spaces and may include a keyless option count.
func codexAsyncEditor(screen screen, end int) (reading, bool) {
	first := codexHintRows(screen, end)
	if first < 0 {
		return reading{}, false
	}
	var labels []string
	for index := first; index <= end; index++ {
		for _, member := range codexSplit(codexCells(screen.rows[index], 2), "   ", false) {
			if key := codexKey(member); key > 0 {
				labels = append(labels, codexText(member[key:]))
			}
		}
	}
	if !slices.Contains(labels, " submit") || !slices.Contains(labels, " skip") {
		return reading{}, false
	}
	read := codexOverlay(sessions.InteractionQuestion, screen.rule("codex.question.async_editor", first, end))
	switch scan := codexScan(screen, end-1); {
	case scan.status >= 0:
		read.activity = sessions.ActivityWorking
		read.rules = append(read.rules, screen.rule("codex.activity.status_row", scan.status, scan.status))
	case scan.end == codexUnproven:
		read.activityCause = causeClipped
	}
	return read, true
}

// codexResume is rule 5, the resume picker (2.7): a dim rule with its progress
// label over one or two footer rows whose first hint resumes.
func codexResume(screen screen, end int) (reading, bool) {
	rule := end - 1
	if !codexResumeRuleRow(codexRow(screen, rule)) {
		rule = end - 2
		if !codexResumeRuleRow(codexRow(screen, rule)) {
			return reading{}, false
		}
	}
	footer := screen.rows[rule+1]
	if footer.presence != rowParsed || footer.cell(0).text != " " {
		return reading{}, false
	}
	first := codexSplit(codexCells(footer, 1), "   ", false)[0]
	key := codexKey(first)
	switch label := codexText(first[key:]); {
	case key > 0 && (label == " resume" || label == " fork" || label == " restore"):
		return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.resume", rule, end)), true
	default:
		return reading{}, false
	}
}

func codexResumeRuleRow(row row) bool {
	return row.presence == rowParsed && row.cell(0).text == "─" && row.cell(0).style.dim &&
		codexResumeRule.MatchString(codexText(codexCells(row, 0)))
}

// codexPager is rule 6, the fullscreen pager (2.7). Its header is pane row 0,
// which a clipped bottom region drops while the top region always keeps it.
func codexPager(screen screen, end int) (reading, bool) {
	if !codexPagerHint(screen.rows[end], " close") || !codexPagerHint(codexRow(screen, end-1), " to scroll") {
		return reading{}, false
	}
	header := screen.rows[0]
	if header.presence == rowAbsent {
		read := codexOverlay(sessions.InteractionUnknown)
		read.interactionCause = causeClipped
		return read, true
	}
	// The dim `/ <title>` header over a `/ / /` fill.
	title, isPager := "", false
	if header.presence == rowParsed && header.cell(0).style.dim {
		title, isPager = strings.CutPrefix(codexText(codexCells(header, 0)), "/ ")
	}
	switch title = strings.TrimRight(title, "/ "); {
	case !isPager:
		return codexOverlay(sessions.InteractionUnknown), true
	case title == "E X E C" || title == "P A T C H" || title == "P E R M I S S I O N S" ||
		title == "E L I C I T A T I O N" || title == "U S E R  V E R I F I C A T I O N":
		return codexOverlay(sessions.InteractionPermission, screen.rule("codex.permission.details_pager", 0, end)), true
	case title == "What we detected":
		return codexOverlay(sessions.InteractionConfirmation, screen.rule("codex.confirmation.details_pager", 0, end)), true
	default:
		return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.pager", 0, end)), true
	}
}

// codexPagerHint is a pager footer row: a space, a key at column 1, then a dim
// label.
func codexPagerHint(row row, label string) bool {
	if row.presence != rowParsed || row.cell(0).text != " " {
		return false
	}
	runs := codexRuns(codexCells(row, 1))
	return len(runs) >= 2 && runs[0].bold && runs[0].plain && runs[1].dim &&
		(runs[1].text == label || strings.HasPrefix(runs[1].text, label+" · "))
}

// codexWarnings is rule 7, the warnings view (2.7).
func codexWarnings(screen screen, end int) (reading, bool) {
	if !codexIndented(screen.rows[end]) {
		return reading{}, false
	}
	var labels []string
	for _, member := range codexSplit(codexCells(screen.rows[end], 2), " · ", false) {
		key := codexKey(member)
		if key == 0 {
			break
		}
		labels = append(labels, codexText(member[key:]))
	}
	if len(labels) < 2 || labels[0] != " keep & next" || labels[1] != " dismiss & close" {
		return reading{}, false
	}
	return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.warnings", end, end)), true
}

// codexLogin is rule 8, the first login screen (2.8): `Press K to continue`
// under options whose selected row is a dim cyan `> N. ` at column 0.
func codexLogin(screen screen, end int) (reading, bool) {
	runs := codexRuns(codexCells(screen.rows[end], 2))
	if !codexIndented(screen.rows[end]) || len(runs) != 3 || !runs[0].dim || runs[0].text != "Press " ||
		!runs[1].bold || !runs[1].plain || !runs[2].dim || runs[2].text != " to continue" {
		return reading{}, false
	}
	selected := end - 1
	for selected >= 0 && (screen.rows[selected].blank() || codexIndented(screen.rows[selected])) {
		selected--
	}
	marker := codexRow(screen, selected)
	if marker.presence != rowParsed || marker.cell(0).text != ">" || !marker.cell(0).style.dim ||
		marker.cell(0).style.fg != codexCyan || marker.cell(1).text != " " {
		return reading{}, false
	}
	return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.login", selected, end)), true
}

// codexPicker is rule 9 (2.6): an option scan from E finds the selected row,
// and the nearest bold title above it decides the value.
func codexPicker(screen screen, end int) (reading, bool) {
	selected, found := codexOptionScan(screen, end)
	if !found {
		return reading{}, false
	}
	footered := codexKeyHintFooter(screen.rows[end])
	// Title lookup: a column-0 row or pane row 0 ends it without a title.
	title, text := selected, ""
	for index := selected - 1; index >= 0; index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			// The dropped row could have decided setup or confirmation: the one
			// partial match that is final.
			read := codexUnknown()
			read.interactionCause = causeClipped
			if footered {
				read.composer = composerBlocked
			}
			return read, true
		}
		if row.blank() || codexScrollIndicator(row) {
			continue
		}
		if row.cell(0).text != " " {
			break
		}
		if head := row.cell(2); codexIndented(row) && head.text != " " && head.style.bold && !head.style.dim {
			title, text = index, codexText(codexCells(row, 2))
			break
		}
	}
	titled := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(text, prefix) })
	}
	if !footered {
		if titled("Background server has incompatible feature settings", "Cannot use the background server") {
			return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.daemon_recovery", title, end)), true
		}
		// Precaution, safety buffering, usage progress and reserve fall
		// through; the composer rule rejects their selected row.
		return reading{}, false
	}
	switch {
	case titled("Implement this plan?"):
		return codexOverlay(sessions.InteractionConfirmation, screen.rule("codex.confirmation.plan", title, end)), true
	case titled("Approaching rate limits", "Resume paused goal?", "Usage limit reached", "You've reached your workspace credit limit"):
		return codexOverlay(sessions.InteractionConfirmation, screen.rule("codex.confirmation.provider_picker", title, end)), true
	case titled("Folder access"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.trust", title, end)), true
	case titled("Update available"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.update", title, end)), true
	case titled("Codex just got an upgrade"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.migration", title, end)), true
	case titled("Hooks need review"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.hooks_review", title, end)), true
	default:
		return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.picker", title, end)), true
	}
}

// codexKeyHintFooter is a picker footer: a key at column 2, a dim label
// starting with a space, then only keys and dim text. Composer hint rows fail
// because their labels are plain or coloured, never dim.
func codexKeyHintFooter(row row) bool {
	if !codexIndented(row) {
		return false
	}
	runs := codexRuns(codexCells(row, 2))
	if len(runs) < 2 || !runs[0].bold || !strings.HasPrefix(runs[1].text, " ") {
		return false
	}
	return !slices.ContainsFunc(runs, func(run codexRun) bool { return run.bold && !run.plain || !run.bold && !run.dim })
}

// codexOptionScan walks up from start over blank, indented and scroll
// indicator rows; the first other row must be a picker's selected row.
func codexOptionScan(screen screen, start int) (int, bool) {
	for index := start; index >= 0; index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			return 0, false
		}
		if row.blank() || codexIndented(row) || codexScrollIndicator(row) {
			continue
		}
		// The glyph and column 2 agree on reverse: selection is bold+reverse
		// over the row, or bold on a highlight colour when the terminal
		// background is known.
		glyph, label := row.cell(0), row.cell(2)
		return index, glyph.text == "›" && glyph.style.bold && !glyph.style.dim && label.style.bold && glyph.style.reverse == label.style.reverse
	}
	return 0, false
}

func codexScrollIndicator(row row) bool {
	return (row.cell(0).text == "↑" || row.cell(0).text == "↓") && len(codexCells(row, 1)) == 0
}

// codexComposer is rule 10, the composer surface (2.1, 3, 4).
func codexComposer(screen screen, end int) (reading, bool) {
	// F, the run of non-blank rows ending at E, holds at most three rows; B,
	// the blank row above it, is the composer's bottom padding.
	footer := end
	for footer > end-2 && footer > 0 && screen.rows[footer-1].presence == rowParsed && !screen.rows[footer-1].blank() {
		footer--
	}
	if footer == 0 || !screen.rows[footer-1].blank() {
		return reading{}, false
	}
	bottom := footer - 1
	composer := bottom - 1
	for composer >= 0 && (screen.rows[composer].blank() || codexIndented(screen.rows[composer])) {
		composer--
	}
	input := codexRow(screen, composer)
	if input.presence != rowParsed || input.cell(0).text == " " {
		return reading{}, false
	}
	// C is row-local: its glyph is enabled, disabled or dimmed, and no later
	// cell is bold unless also reverse (history-search highlights).
	glyph := input.cell(0)
	continuation := screen.rows[composer+1 : bottom]
	allDim, typed := true, false
	for _, row := range append([]row{input}, continuation...) {
		for column, cell := range row.cells {
			if column > 0 && cell.text != " " && !cell.style.dim {
				allDim, typed = false, typed || column >= 2
			}
		}
	}
	marked := glyph.text == "›" || glyph.text == "»" || glyph.text == "!"
	enabled := marked && glyph.style.bold && !glyph.style.dim
	disabled := glyph.text == "›" && glyph.style.dim && !glyph.style.bold
	dimmed := marked && glyph.style.bold && glyph.style.dim && allDim
	if glyph.style.reverse || !enabled && !disabled && !dimmed ||
		slices.ContainsFunc(input.cells[1:], func(cell cell) bool { return cell.style.bold && !cell.style.reverse }) {
		return reading{}, false
	}

	// SL and the hint row: two or three footer rows are SL then the hint row;
	// one row is SL only when it starts with a run-state word.
	statusLine, hint := -1, -1
	switch {
	case footer < end:
		statusLine, hint = footer, end
	case codexRunState(screen.rows[end]) != "":
		statusLine = end
	default:
		hint = end
	}
	word := ""
	if statusLine >= 0 {
		word = codexRunState(screen.rows[statusLine])
	}
	// The hint-row overrides (3 step 1): the disconnect hint `K quit` and the
	// external editor's constant, each optionally followed by right context.
	disconnected, editor := false, false
	if hint >= 0 && codexIndented(screen.rows[hint]) {
		cells := codexCells(screen.rows[hint], 2)
		key := codexKey(cells)
		disconnected = key > 0 && codexToken(cells, key, " quit") &&
			!slices.ContainsFunc(cells[key:key+5], func(cell cell) bool { return cell.style.dim || cell.style.bold })
		editor = codexText(cells[:key]) == "Save and close external editor to continue." && codexToken(cells, key, "")
	}

	// The band: the top padding C-1, C, its continuation rows and B; remote
	// image rows add themselves and one more padding row above them.
	image := composer - 2
	for codexImageRow(codexRow(screen, image)) {
		image--
	}
	images := image < composer-2
	start := composer - 2
	if images {
		start = image - 1
	}
	// The placeholder is the dim run at column 2; the band is clean when it and
	// the glyph are its only glyphs.
	run, after := "", 2
	if input.cell(2).text != " " {
		for after < len(input.cells) && input.cells[after].style.dim {
			after++
		}
		run = strings.TrimRight(codexText(input.cells[2:after]), " ")
	}
	clean := run != "" && input.cell(1).text == " " && len(codexCells(input, after)) == 0 && !images &&
		codexRow(screen, composer-1).blank() && !slices.ContainsFunc(continuation, func(row row) bool { return !row.blank() })
	showing := func(placeholder string) bool {
		// A narrow pane clips the placeholder at its right edge.
		return clean && (run == placeholder || len(run) >= 5 && len(run) < len(placeholder) &&
			strings.HasPrefix(placeholder, run) && 2+len(run)-1 >= screen.width-4)
	}
	main, side := showing(codexMainPlaceholder), showing(codexSidePlaceholder)

	read := reading{notice: sessions.NoticeNone}
	// Composer (4), first match wins.
	switch {
	case disabled || dimmed || side || editor:
		read.composer = composerBlocked
	case images || glyph.text == "!":
		read.composer = composerDraft
	case run != "" && !clean:
		read.composer = composerUnknown // sparkle or ignition painted the band
	case typed:
		read.composer = composerDraft
	case enabled && main:
		read.composer = composerEmpty
	default:
		read.composer = composerUnknown
	}
	if side {
		read.rules = append(read.rules, screen.rule("codex.scope.side", composer, composer))
	}
	if editor {
		read.rules = append(read.rules, screen.rule("codex.composer.external_editor", hint, hint))
	}

	// Interaction (2.1): the elements between the transcript's stop and the band.
	scan := codexScan(screen, start)
	switch {
	case scan.otherThread >= 0:
		read.interaction = sessions.InteractionPermission
		read.rules = append(read.rules, screen.rule("codex.permission.other_thread", scan.otherThread, scan.otherThread))
	case scan.questions >= 0:
		read.interaction = sessions.InteractionQuestion
		read.rules = append(read.rules, screen.rule("codex.question.async_collapsed", scan.questions, scan.questions))
	case scan.banner >= 0:
		read.interaction = sessions.InteractionConfirmation
		read.rules = append(read.rules, screen.rule("codex.confirmation.inline_banner", scan.banner, scan.banner))
	case dimmed:
		read.interaction = sessions.InteractionMenu
		read.rules = append(read.rules, screen.rule("codex.menu.transcript_footer", composer, end))
	case scan.information:
		read.interaction = sessions.InteractionUnknown
	default:
		read.interaction = sessions.InteractionNone
		read.rules = append(read.rules, screen.rule("codex.interaction.none", composer, end))
	}

	// Activity (3): the hint-row override, the status row, then the run-state
	// word; Ready is idle only under every qualification of step 4.
	if scan.status >= 0 {
		read.rules = append(read.rules, screen.rule("codex.activity.status_row", scan.status, scan.status))
	}
	read.activity = sessions.ActivityUnknown
	ready := statusLine
	switch {
	case disconnected:
		read.rules = append(read.rules, screen.rule("codex.activity.disconnected", hint, hint))
		if scan.status >= 0 || word != "" {
			read.activityCause = causeConflict
		}
	case scan.status >= 0 && word == "Ready":
		read.activityCause = causeConflict
	case scan.status >= 0 || word == "Working" || word == "Thinking" || word == "Waiting":
		read.activity = sessions.ActivityWorking
	case word == "Starting":
		read.activity = sessions.ActivityStarting
	case word == "Ready":
		switch {
		case !enabled && !dimmed || !main || editor:
		case codexGoalActive(screen.rows[statusLine]):
			read.activityCause = causeConflict
			read.rules = append(read.rules, screen.rule("codex.activity.goal_active", statusLine, statusLine))
		case scan.end == codexOpen:
			read.activityCause = causeConflict
			read.rules = append(read.rules, screen.rule("codex.activity.prompt_pending", scan.stop, statusLine))
		case scan.end == codexUnproven:
			read.activityCause = causeClipped
		default:
			read.activity, ready = sessions.ActivityIdle, scan.stop
		}
	case scan.end == codexUnproven:
		read.activityCause = causeClipped
	}
	switch word {
	case "Ready":
		read.rules = append(read.rules, screen.rule("codex.run_state.ready", ready, statusLine))
	case "Working", "Thinking", "Waiting":
		read.rules = append(read.rules, screen.rule("codex.run_state.working", statusLine, statusLine))
	case "Starting":
		read.rules = append(read.rules, screen.rule("codex.run_state.starting", statusLine, statusLine))
	}
	return read, true
}

// codexRunState is SL's first item when it is a run-state word (3): items are
// ` · `-separated, and a right-aligned indicator follows a wider gap.
func codexRunState(row row) string {
	if !codexIndented(row) {
		return ""
	}
	word, _, _ := strings.Cut(codexText(codexCells(row, 2)), " · ")
	word, _, _ = strings.Cut(word, "  ")
	switch word {
	case "Starting", "Ready", "Working", "Thinking", "Waiting":
		return word
	default:
		return ""
	}
}

// codexGoalActive finds the magenta `Pursuing goal` or `Pursuing goal (…)`
// indicator on SL, wherever IDE context or truncated items leave it; other
// goal states are not the indicator.
func codexGoalActive(row row) bool {
	for column := range row.cells {
		if codexToken(row.cells, column, "Pursuing goal") &&
			!slices.ContainsFunc(row.cells[column:column+len("Pursuing goal")], func(cell cell) bool { return cell.style.fg != codexMagenta }) {
			return true
		}
	}
	return false
}

type codexEnd uint8

const (
	codexSettled  codexEnd = iota // a terminator ends the transcript
	codexOpen                     // a prompt ends it: a submitted turn has not started
	codexUnproven                 // no stop before pane row 0 or a dropped row
)

// codexTranscript is what the upward transcript scan found: how the
// transcript ends and where, and the bottom-pane elements it passed over on
// the way. Rows are -1 when absent.
type codexTranscript struct {
	end         codexEnd
	stop        int
	status      int // the status row nearest the band
	otherThread int // `! Approval needed in …`
	questions   int // collapsed async questions
	banner      int // an inline banner's `Press a number to choose` hint
	information bool
}

// codexScan is the transcript scan (2.2): from start upward, the first stop
// (a prompt or a terminator) classifies the transcript's end. Every other row
// is passed over, never skipped by its text. An unparseable row stops it
// unproven like a dropped one: it could hide the stop.
func codexScan(screen screen, start int) codexTranscript {
	scan := codexTranscript{end: codexUnproven, stop: -1, status: -1, otherThread: -1, questions: -1, banner: -1}
	for index := start; index >= 0; index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			return scan
		}
		switch {
		case codexPrompt(row):
			scan.end, scan.stop = codexOpen, index
			return scan
		case codexTerminator(row):
			scan.end, scan.stop = codexSettled, index
			return scan
		}
		if scan.status < 0 && codexStatusRow(row) {
			scan.status = index
		}
		if !codexIndented(row) {
			continue
		}
		label := codexCells(row, 2)
		text := codexText(label)
		switch {
		case codexToken(label, 0, "!") && label[0].style.bold && label[0].style.fg == codexRed && strings.HasPrefix(text, "! Approval needed"):
			scan.otherThread = index
		case codexToken(label, 0, "?") && label[0].style.dim && len(label) > 2 &&
			codexQuestionCount.MatchString(codexText(label[2:2+codexKey(label[2:])])):
			scan.questions = index
		case codexDim(label) && strings.HasPrefix(text, "Press a number to choose"):
			if _, found := codexOptionScan(screen, index-1); found {
				scan.banner = index
			}
		case codexDim(label) && strings.HasPrefix(text, "esc to dismiss"):
			scan.information = true
		}
	}
	return scan
}

// codexPrompt is a prompt stop: a user row (`› ` bold+dim, or red+bold when
// spoken; the sticky prompt header renders the same), or the image-label row
// of an image-only prompt.
func codexPrompt(row row) bool {
	glyph := row.cell(0)
	return glyph.text == "›" && glyph.style.bold && !glyph.style.reverse && (glyph.style.dim || glyph.style.fg == codexRed) &&
		row.cell(1).text == " " || codexImageRow(row)
}

// codexTerminator is a terminator stop: a completion separator, a notice
// cell (`■`: interruption, goal budget, error) or the session header.
func codexTerminator(row row) bool {
	if row.cell(0).text == "■" {
		return true
	}
	cells := codexCells(row, 2)
	if !codexIndented(row) || len(cells) == 0 || cells[0].text == " " {
		return false
	}
	text := codexText(cells)
	return codexDim(row.cells) && (strings.HasPrefix(text, "Worked for ") || codexClock.MatchString(text)) ||
		strings.HasPrefix(text, ">_ ") && codexSpells(cells, 3, "OpenAI Codex") &&
			!slices.ContainsFunc(cells[3:3+len("OpenAI Codex")], func(cell cell) bool { return !cell.style.bold })
}

// codexImageRow is a row of `[Image #N]` labels at column 2, coloured and not
// dim: an image-only prompt in the transcript, or remote images in the band.
func codexImageRow(row row) bool {
	cells := codexCells(row, 2)
	return codexIndented(row) && codexImageLabels.MatchString(codexText(cells)) &&
		!slices.ContainsFunc(cells, func(cell cell) bool {
			return cell.text != " " && (cell.style.fg.kind == colorDefault || cell.style.dim)
		})
}

// codexStatusRow is the status row (2.2): a header from column 0 (with or
// without the activity glyph), a space, then the dim elapsed group in full
// `(D • K to interrupt)`, hintless `(D)` or cut by `…`, optionally followed by
// dim ` · ` details.
func codexStatusRow(row row) bool {
	cells := codexCells(row, 0)
	if len(cells) == 0 || cells[0].text == " " {
		return false
	}
	last := len(cells) - 1
	tail := func(from int) bool { return from > last || cells[from].text == " " && codexDim(cells[from:]) }
	for open := 2; open <= last; open++ {
		if cells[open].text != "(" || !cells[open].style.dim || cells[open-1].text != " " {
			continue
		}
		// D is ASCII, so its byte length is its cell count.
		after := open + 1 + len(codexDuration.FindString(codexText(cells[open+1:])))
		if after == open+1 || !codexDim(cells[open:after]) {
			continue
		}
		switch {
		case after == last && cells[after].text == "…", codexSpells(cells, after, " •") && cells[last].text == "…":
			return true
		case codexSpells(cells, after, ")") && cells[after].style.dim && tail(after+1):
			return true
		case codexSpells(cells, after, " • ") && codexDim(cells[after:after+3]):
			key := after + 3 + codexKey(cells[after+3:])
			if key > after+3 && codexSpells(cells, key, " to interrupt)") && codexDim(cells[key:key+14]) && tail(key+14) {
				return true
			}
		}
	}
	return false
}

// codexHintRows returns the first of the hint rows ending at E (each an
// indented row with a key at column 2), or -1 when E is not one.
func codexHintRows(screen screen, end int) int {
	hintRow := func(row row) bool {
		return codexIndented(row) && row.cell(2).text != " " && row.cell(2).style.bold && !row.cell(2).style.dim
	}
	if !hintRow(screen.rows[end]) {
		return -1
	}
	first := end
	for first > 0 && hintRow(screen.rows[first-1]) {
		first--
	}
	return first
}

// codexIndented is a parsed row with spaces at columns 0 and 1.
func codexIndented(row row) bool {
	return row.presence == rowParsed && row.cell(0).text == " " && row.cell(1).text == " "
}

// codexRow is pane row index; a row above the pane reads as absent.
func codexRow(screen screen, index int) row {
	if index < 0 {
		return row{}
	}
	return screen.rows[index]
}

// codexCells is a parsed row's cells from column from to its last glyph.
func codexCells(row row, from int) []cell {
	last := len(row.cells)
	for last > from && row.cells[last-1].text == " " {
		last--
	}
	if from >= last {
		return nil
	}
	return row.cells[from:last]
}

func codexText(cells []cell) string {
	var text strings.Builder
	for _, cell := range cells {
		text.WriteString(cell.text)
	}
	return text.String()
}

// codexDim reports that every glyph is dim; spaces may carry any style.
func codexDim(cells []cell) bool {
	return !slices.ContainsFunc(cells, func(cell cell) bool { return cell.text != " " && !cell.style.dim })
}

// codexKey is the length of the key label leading cells: bold, not dim.
func codexKey(cells []cell) int {
	key := 0
	for key < len(cells) && cells[key].style.bold && !cells[key].style.dim {
		key++
	}
	return key
}

// codexSpells reports that cells from at spell token, one rune per cell.
func codexSpells(cells []cell, at int, token string) bool {
	for _, char := range token {
		if at >= len(cells) || cells[at].text != string(char) {
			return false
		}
		at++
	}
	return true
}

// codexToken reports that cells from at spell token as a whole word: the
// cells end after it or continue with a space.
func codexToken(cells []cell, at int, token string) bool {
	next := at + utf8.RuneCountInString(token)
	return codexSpells(cells, at, token) && (next >= len(cells) || cells[next].text == " ")
}

// codexSplit splits cells into members at separator; with dim set only an
// all-dim separator splits.
func codexSplit(cells []cell, separator string, dim bool) [][]cell {
	width := utf8.RuneCountInString(separator)
	var members [][]cell
	from := 0
	for column := 0; column+width <= len(cells); column++ {
		if codexSpells(cells, column, separator) && (!dim || !slices.ContainsFunc(cells[column:column+width], func(cell cell) bool { return !cell.style.dim })) {
			members = append(members, cells[from:column])
			from = column + width
			column = from - 1
		}
	}
	return append(members, cells[from:])
}

// codexRun is a maximal span of cells that agree on bold: a key label, or the
// text between key labels.
type codexRun struct {
	text  string
	bold  bool
	dim   bool // every cell is dim
	plain bool // no cell is dim
}

func codexRuns(cells []cell) []codexRun {
	var runs []codexRun
	for _, cell := range cells {
		if len(runs) == 0 || runs[len(runs)-1].bold != cell.style.bold {
			runs = append(runs, codexRun{bold: cell.style.bold, dim: true, plain: true})
		}
		run := &runs[len(runs)-1]
		run.text += cell.text
		run.dim, run.plain = run.dim && cell.style.dim, run.plain && !cell.style.dim
	}
	return runs
}
