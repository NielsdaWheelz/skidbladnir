package agentcontrol

import (
	"regexp"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// codexElapsed is the status row's elapsed time (fmt_elapsed_compact).
const codexElapsed = `\d+s|\d+m \d\ds|\d+h \d\dm \d\ds`

var (
	codexRed     = color{kind: colorPalette, value: 1}
	codexMagenta = color{kind: colorPalette, value: 5}
	codexCyan    = color{kind: colorPalette, value: 6}

	codexDuration = regexp.MustCompile(`^(` + codexElapsed + `)`)
	// codexCutGroup is what the status row's truncation, which cuts after any
	// cell, can leave of its paren group before the `…`: a proper prefix of
	// `D • K to interrupt)` or of `D)`, that is part of D, or D followed by
	// nothing, a space, `)` or ` •` and the rest.
	codexCutGroup = regexp.MustCompile(`^(\d*|\d+m( \d{0,2})?|\d+h( \d{0,2}| \d\dm( \d{0,2})?)?|(` + codexElapsed + `)( |\)| •.*)?)$`)
	// codexClock starts a completion separator without its `Worked for` part:
	// an optional date, then the 12- or 24-hour clock time.
	codexClock = regexp.MustCompile(`^((Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{1,2}(, \d{4})? at )?\d{1,2}:\d\d`)
	// codexImageLabels is a row of attached-image labels.
	codexImageLabels = regexp.MustCompile(`^\[Image #\d+\]( \[Image #\d+\])*$`)
	// codexQuestionCount is the collapsed async questions' bold count.
	codexQuestionCount = regexp.MustCompile(`^\d+ questions?$`)
	// codexOptionCount is the question footers' keyless option count.
	codexOptionCount = regexp.MustCompile(`^option \d+/\d+\b`)
	// codexOption is a numbered form option read from column 4; its label ends
	// at the double space before a description.
	codexOption = regexp.MustCompile(`^\d+\. (\S+(?: \S+)*)`)
	// codexResumeRule is the resume picker's rule with its progress label.
	codexResumeRule = regexp.MustCompile(`^─+ (\d+ ?/ ?\d+…? · )?\d+% ─$`)
)

// detectCodex reads the codex 0.159.2 screen grammar of
// docs/terminal-observation-codex.md, sections 2-4. E (end) is the last
// non-blank row; every surface is anchored on it and tried in the grammar's
// order, and the first that matches decides. Rows are pane rows, so a read
// above the bottom region continues into the top region by index. A row that
// was not parsed stops a read. A dropped row is missing evidence: a rule that
// needs it falls through or, where the grammar says so, reads clipped. An
// unparseable row is present but unrecognized: it matches no rule and leaves
// what it would decide unknown, never clipped.
func detectCodex(parsed screen) reading {
	end := len(parsed.rows) - 1
	for end >= 0 && parsed.rows[end].blank() {
		end--
	}
	switch {
	case end < 0:
		return unknownReading(causeUnrecognized)
	case parsed.rows[end].presence == rowAbsent:
		// The screen continues into a dropped row (2.0 step 1).
		return unknownReading(causeClipped)
	case parsed.rows[end].presence == rowUnparseable:
		// Every surface anchors on E, which no rule recognizes.
		return unknownReading(causeUnrecognized)
	}
	// 2.0 step 4: the surfaces in the grammar's order; the first match wins.
	for _, surface := range [...]func(screen, int) (reading, bool){codexApproval, codexForm, codexAsyncEditor,
		codexResume, codexPager, codexWarnings, codexLogin, codexPicker, codexComposer} {
		if read, ok := surface(parsed, end); ok {
			return read
		}
	}
	return unknownReading(causeUnrecognized)
}

// codexOverlay is the reading of a surface that hides the composer: activity
// starts unknown and nothing is carried forward (2.0 step 5).
func codexOverlay(interaction sessions.Interaction, rules ...DiagnosticRule) reading {
	return reading{activity: sessions.ActivityUnknown, interaction: interaction, composer: composerBlocked, rules: rules}
}

// codexApproval is rule 1, the `Press` footer (2.3). It word-wraps upward
// from E, so its first row is the nearest one starting `Press `.
func codexApproval(screen screen, end int) (reading, bool) {
	if !codexIndented(screen.rows[end]) {
		return reading{}, false
	}
	first := end
	for !strings.HasPrefix(cellText(codexCells(screen.rows[first], 2)), "Press ") {
		first--
		if first < 0 || !codexIndented(screen.rows[first]) || screen.rows[first].blank() {
			return reading{}, false
		}
	}
	// Join the wrapped rows with the space the wrap removed. It is bold only
	// inside a key, a chord broken at its own space.
	var cells []cell
	for index := first; index <= end; index++ {
		next := codexCells(screen.rows[index], 2)
		if len(cells) > 0 {
			cells = append(cells, cell{text: " ", style: style{bold: cells[len(cells)-1].style.bold && next[0].style.bold}})
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

// codexForm is rules 2 and 3, the ` | ` footers of the legacy
// request_user_input view and the mcp form (2.4). Both lead their first row
// with the option count when the options do not fit, and each can lose hints
// to the footer height or the right edge. The legacy view is told first, by a
// hint only it renders or by a footer of the count alone; the mcp form then by
// a submit hint, or by the count and a hint only it renders when the count,
// prefixed without rewrapping, clipped the submit hint.
func codexForm(screen screen, end int) (reading, bool) {
	first, counted, labels := codexHints(screen, end, " | ")
	if first < 0 {
		return reading{}, false
	}
	has := func(members ...string) bool {
		return slices.ContainsFunc(labels, func(label string) bool { return slices.Contains(members, label) })
	}
	// The `…` members are notes hints wider than the footer, cut at a word
	// boundary after their distinguishing words; the mcp form never adds `…`.
	// A lone count is a one-row footer that could not also hold the next hint;
	// the mcp form draws its count on its submit row.
	legacy := has(" to add notes", " or esc to clear notes", " to navigate questions", " change question", " to interrupt",
		" to add…", " or esc…", " or esc to…", " or esc to clear…") || counted && len(labels) == 0
	mcp := has(" to submit", " to submit all", " to submit answer") ||
		counted && has(" to cancel", " to navigate fields", " change field")
	fields := has(" to submit all", " to submit answer", " to navigate fields", " change field")
	switch {
	case legacy:
		return codexOverlay(sessions.InteractionQuestion, screen.rule("codex.question.legacy", first, end)), true
	case mcp && !fields && codexApprovalActions(screen, first-1):
		return codexOverlay(sessions.InteractionPermission, screen.rule("codex.permission.mcp_approval", first, end)), true
	case mcp:
		return codexOverlay(sessions.InteractionInput, screen.rule("codex.input.mcp_form", first, end)), true
	default:
		return reading{}, false
	}
}

// codexApprovalActions reports an mcp form in approval-action mode: the
// option block above the footer holds numbered options, all approval actions.
// A row that was not parsed could hold another option, so it fails the test.
func codexApprovalActions(screen screen, start int) bool {
	index := start
	for index >= 0 && screen.rows[index].blank() {
		index--
	}
	options := 0
	for ; index >= 0 && !screen.rows[index].blank(); index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			return false
		}
		marker := row.cell(2).text + row.cell(3).text
		match := codexOption.FindStringSubmatch(cellText(codexCells(row, 4)))
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
// replaces the composer and status line; the status row stays above it.
func codexAsyncEditor(screen screen, end int) (reading, bool) {
	first, _, labels := codexHints(screen, end, "   ")
	if first < 0 || !slices.Contains(labels, " submit") || !slices.Contains(labels, " skip") {
		return reading{}, false
	}
	read := codexOverlay(sessions.InteractionQuestion, screen.rule("codex.question.async_editor", first, end))
	switch scan := codexScan(screen, first-1); {
	case scan.status >= 0:
		read.activity = sessions.ActivityWorking
		read.rules = append(read.rules, screen.rule("codex.activity.status_row", scan.status, scan.status))
	case scan.ending == codexUnproven:
		read.activityCause = causeClipped
	}
	return read, true
}

// codexHints reads a question footer (2.4, 2.5): the run of hint rows ending
// at E, each indented and starting at column 2 with a key or with the option
// count, which can lead a wrapped row. The selected option and the notes row
// start there with a bold `›`, which no key label is, and can sit right above
// the footer. It returns the first row, whether that row leads with the count,
// and the labels of the keyed members joined by separator; first is -1 when
// there is no hint row or a member is neither keyed nor the count.
func codexHints(screen screen, end int, separator string) (first int, counted bool, labels []string) {
	hintRow := func(row row) bool {
		cells := codexCells(row, 2)
		return codexIndented(row) && len(cells) > 0 && cells[0].text != " " && cells[0].text != "›" &&
			(codexKey(cells) > 0 || codexOptionTip(cells) > 0)
	}
	first = end + 1
	for first > 0 && hintRow(screen.rows[first-1]) {
		first--
	}
	if first > end {
		return -1, false, nil
	}
	for index := first; index <= end; index++ {
		for _, member := range codexSplit(codexCells(screen.rows[index], 2), separator) {
			switch key := codexKey(member); {
			case key > 0:
				labels = append(labels, cellText(member[key:]))
			case len(member) > 0 && codexOptionTip(member) == len(member):
				// The count, shown when the options do not fit.
			default:
				return -1, false, nil
			}
		}
	}
	return first, codexOptionTip(codexCells(screen.rows[first], 2)) > 0, labels
}

// codexOptionTip is the width of the dim `option N/M` count leading cells, or
// 0.
func codexOptionTip(cells []cell) int {
	tip := len(codexOptionCount.FindString(cellText(cells))) // ASCII: one cell per byte
	if tip == 0 || !codexDim(cells[:tip]) {
		return 0
	}
	return tip
}

// codexResume is rule 5, the resume picker (2.7): a dim rule with its progress
// label over one or two footer rows whose first hint resumes.
func codexResume(screen screen, end int) (reading, bool) {
	ruleRow := end - 1
	if !codexResumeRuleRow(codexRow(screen, ruleRow)) {
		ruleRow = end - 2
		if !codexResumeRuleRow(codexRow(screen, ruleRow)) {
			return reading{}, false
		}
	}
	footer := screen.rows[ruleRow+1]
	if footer.presence != rowParsed || footer.cell(0).text != " " {
		return reading{}, false
	}
	first := codexSplit(codexCells(footer, 1), "   ")[0]
	key := codexKey(first)
	switch label := cellText(first[key:]); {
	case key > 0 && (label == " resume" || label == " fork" || label == " restore"):
		return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.resume", ruleRow, end)), true
	default:
		return reading{}, false
	}
}

func codexResumeRuleRow(row row) bool {
	return row.presence == rowParsed && row.cell(0).text == "─" && row.cell(0).style.dim &&
		codexResumeRule.MatchString(cellText(codexCells(row, 0)))
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
	// The dim `/ <title>` header over a `/ / /` fill; an unparseable header
	// has no cells, so it is no title.
	title, isPager := strings.CutPrefix(cellText(codexCells(header, 0)), "/ ")
	if !isPager || !header.cell(0).style.dim {
		return codexOverlay(sessions.InteractionUnknown), true
	}
	switch strings.TrimRight(title, "/ ") {
	case "E X E C", "P A T C H", "P E R M I S S I O N S", "E L I C I T A T I O N", "U S E R  V E R I F I C A T I O N":
		return codexOverlay(sessions.InteractionPermission, screen.rule("codex.permission.details_pager", 0, end)), true
	case "What we detected":
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
	for _, member := range codexSplit(codexCells(screen.rows[end], 2), " · ") {
		key := codexKey(member)
		if key == 0 {
			break
		}
		labels = append(labels, cellText(member[key:]))
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
	titleRow, title := selected, ""
	for index := selected - 1; index >= 0; index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			// The row could have decided setup or confirmation, so this partial
			// match is final, as rule 6's is (2.0 step 4). Only a dropped row is
			// clipped.
			read := unknownReading(causeUnrecognized)
			if row.presence == rowAbsent {
				read.interactionCause = causeClipped
			}
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
			titleRow, title = index, cellText(codexCells(row, 2))
			break
		}
	}
	titled := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(title, prefix) })
	}
	if !footered {
		if titled("Background server has incompatible feature settings", "Cannot use the background server") {
			return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.daemon_recovery", titleRow, end)), true
		}
		// Precaution, safety buffering, usage progress and reserve fall
		// through; the composer rule rejects their selected row.
		return reading{}, false
	}
	switch {
	case titled("Implement this plan?"):
		return codexOverlay(sessions.InteractionConfirmation, screen.rule("codex.confirmation.plan", titleRow, end)), true
	case titled("Approaching rate limits", "Resume paused goal?", "Usage limit reached", "You've reached your workspace credit limit"):
		return codexOverlay(sessions.InteractionConfirmation, screen.rule("codex.confirmation.provider_picker", titleRow, end)), true
	case titled("Folder access"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.trust", titleRow, end)), true
	case titled("Update available"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.update", titleRow, end)), true
	case titled("Codex just got an upgrade"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.migration", titleRow, end)), true
	case titled("Hooks need review"):
		return codexOverlay(sessions.InteractionSetup, screen.rule("codex.setup.hooks_review", titleRow, end)), true
	default:
		return codexOverlay(sessions.InteractionMenu, screen.rule("codex.menu.picker", titleRow, end)), true
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
	// the blank row above it, is the composer's bottom padding; C is the first
	// column-0 row above B over blank and indented rows. The last rule, so a
	// dropped row this anchoring needs leaves the screen clipped; an
	// unparseable one is no B or C, so the screen is unrecognized.
	footerFirst := end
	for footerFirst > end-2 && footerFirst > 0 && screen.rows[footerFirst-1].presence == rowParsed && !screen.rows[footerFirst-1].blank() {
		footerFirst--
	}
	if footerFirst == 0 {
		return reading{}, false
	}
	padding := footerFirst - 1
	switch {
	case screen.rows[padding].presence == rowAbsent:
		return unknownReading(causeClipped), true
	case !screen.rows[padding].blank():
		return reading{}, false
	}
	inputRow := padding - 1
	for inputRow >= 0 && (screen.rows[inputRow].blank() || codexIndented(screen.rows[inputRow])) {
		inputRow--
	}
	switch {
	case inputRow < 0 || screen.rows[inputRow].presence == rowUnparseable:
		return reading{}, false
	case screen.rows[inputRow].presence == rowAbsent:
		return unknownReading(causeClipped), true
	}
	input := screen.rows[inputRow]
	if input.cell(0).text == " " {
		return reading{}, false
	}
	// C is row-local: its glyph is enabled, disabled or dimmed, and no later
	// cell is bold unless also reverse (history-search highlights).
	glyph := input.cell(0)
	allDim, typed := true, false
	for _, row := range screen.rows[inputRow:padding] {
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
	statusLine, hint, state := footerFirst, end, codexReadRunState(screen.rows[footerFirst])
	if footerFirst == end {
		if state == codexNoRunState {
			statusLine = -1
		} else {
			hint = -1
		}
	}
	// The hint-row overrides (3 step 1): the disconnect hint `K quit` and the
	// external editor's constant, each optionally followed by right context.
	disconnected, editor := false, false
	if hint >= 0 && codexIndented(screen.rows[hint]) {
		cells := codexCells(screen.rows[hint], 2)
		key := codexKey(cells)
		quit := codexToken(cells, key, " quit")
		disconnected = key > 0 && quit >= 0 &&
			!slices.ContainsFunc(cells[key:quit], func(cell cell) bool { return cell.style.dim || cell.style.bold })
		editor = cellText(cells[:key]) == "Save and close external editor to continue." && (key == len(cells) || cells[key].text == " ")
	}

	// The band: the top padding C-1, C, its continuation rows and B; remote
	// image rows add themselves and one more padding row above them. The
	// transcript scan starts above the band.
	start := inputRow - 2
	for codexImageRow(codexRow(screen, start)) {
		start--
	}
	images := start < inputRow-2
	if images {
		start-- // the padding row above the image rows
	}
	// The placeholder is the dim run at column 2; the band is clean when it and
	// the glyph are its only glyphs.
	run, after := "", 2
	if input.cell(2).text != " " {
		for after < len(input.cells) && input.cells[after].style.dim {
			after++
		}
		run = strings.TrimRight(cellText(input.cells[2:after]), " ")
	}
	clean := run != "" && input.cell(1).text == " " && len(codexCells(input, after)) == 0 && !images &&
		codexRow(screen, inputRow-1).blank() && !slices.ContainsFunc(screen.rows[inputRow+1:padding], func(row row) bool { return !row.blank() })
	showing := func(placeholder string) bool {
		// A narrow pane clips the placeholder at its right edge.
		return clean && (run == placeholder || len(run) >= 5 && len(run) < len(placeholder) &&
			strings.HasPrefix(placeholder, run) && 2+len(run)-1 >= screen.width-4)
	}
	main, side := showing("Ask Codex to do anything"), showing("Ask a follow-up question")

	var read reading
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
		read.rules = append(read.rules, screen.rule("codex.scope.side", inputRow, inputRow))
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
		read.rules = append(read.rules, screen.rule("codex.menu.transcript_footer", inputRow, end))
	case scan.information:
		read.interaction = sessions.InteractionUnknown
	default:
		read.interaction = sessions.InteractionNone
		read.rules = append(read.rules, screen.rule("codex.interaction.none", inputRow, end))
	}

	// Activity (3): the hint-row override, the status row, then the run-state
	// word; Ready is idle only under every qualification of step 4.
	if scan.status >= 0 {
		read.rules = append(read.rules, screen.rule("codex.activity.status_row", scan.status, scan.status))
	}
	read.activity = sessions.ActivityUnknown
	readyFirst := statusLine
	switch {
	case disconnected:
		read.rules = append(read.rules, screen.rule("codex.activity.disconnected", hint, hint))
		if scan.status >= 0 || state != codexNoRunState {
			read.activityCause = causeConflict
		}
	case scan.status >= 0 && state == codexReady:
		read.activityCause = causeConflict
	case scan.status >= 0 || state == codexWorking:
		read.activity = sessions.ActivityWorking
	case state == codexStarting:
		read.activity = sessions.ActivityStarting
	case state == codexReady:
		switch {
		case !enabled && !dimmed || !main || editor:
			// Ready without idle's composer qualifications stays unknown (3 step 4).
		case codexGoalActive(screen.rows[statusLine]):
			read.activityCause = causeConflict
			read.rules = append(read.rules, screen.rule("codex.activity.goal_active", statusLine, statusLine))
		case scan.ending == codexOpen:
			read.activityCause = causeConflict
			read.rules = append(read.rules, screen.rule("codex.activity.prompt_pending", scan.stop, statusLine))
		case scan.ending == codexUnproven:
			read.activityCause = causeClipped
		case scan.ending == codexUnreadable:
			// An unparseable row hides the transcript's end: unknown, not clipped.
		case scan.ending == codexSettled:
			read.activity, readyFirst = sessions.ActivityIdle, scan.stop
		default:
			panic("unknown codex transcript ending") // justify-defect: the scan ends settled, open, unproven or unreadable.
		}
	case scan.ending == codexUnproven:
		read.activityCause = causeClipped
	}
	switch state {
	case codexNoRunState:
	case codexStarting:
		read.rules = append(read.rules, screen.rule("codex.run_state.starting", statusLine, statusLine))
	case codexReady:
		read.rules = append(read.rules, screen.rule("codex.run_state.ready", readyFirst, statusLine))
	case codexWorking:
		read.rules = append(read.rules, screen.rule("codex.run_state.working", statusLine, statusLine))
	default:
		panic("unknown codex run state") // justify-defect: codexReadRunState returns only the closed states.
	}
	return read, true
}

// codexRunState is SL's run-state word (3). Working, Thinking and Waiting all
// mean a running turn: SL refreshes lazily, so they lag one another.
type codexRunState uint8

const (
	codexNoRunState codexRunState = iota // no SL, or its first item is not a run-state word
	codexStarting
	codexReady
	codexWorking
)

// codexReadRunState reads SL's first item: items are ` · `-separated, and a
// right-aligned indicator follows a wider gap.
func codexReadRunState(row row) codexRunState {
	if !codexIndented(row) {
		return codexNoRunState
	}
	word, _, _ := strings.Cut(cellText(codexCells(row, 2)), " · ")
	word, _, _ = strings.Cut(word, "  ")
	switch word {
	case "Starting":
		return codexStarting
	case "Ready":
		return codexReady
	case "Working", "Thinking", "Waiting":
		return codexWorking
	default:
		return codexNoRunState
	}
}

// codexGoalActive finds the magenta `Pursuing goal` or `Pursuing goal (…)`
// indicator on SL, wherever IDE context or truncated items leave it; other
// goal states are not the indicator.
func codexGoalActive(row row) bool {
	for column := range row.cells {
		if next := codexToken(row.cells, column, "Pursuing goal"); next >= 0 &&
			!slices.ContainsFunc(row.cells[column:next], func(cell cell) bool { return cell.style.fg != codexMagenta }) {
			return true
		}
	}
	return false
}

// codexEnding is how the transcript ends, as the upward scan finds it.
type codexEnding uint8

const (
	codexSettled    codexEnding = iota // a terminator ends it
	codexOpen                          // a prompt ends it: a submitted turn has not started
	codexUnproven                      // no stop before pane row 0 or a dropped row
	codexUnreadable                    // an unparseable row, which could be the stop
)

// codexTranscript is what the upward transcript scan found: how the
// transcript ends and where, and the bottom-pane elements it passed over on
// the way. Rows are -1 when absent.
type codexTranscript struct {
	ending      codexEnding
	stop        int
	status      int // the status row nearest the band
	otherThread int // `! Approval needed in …`
	questions   int // collapsed async questions
	banner      int // an inline banner's `Press a number to choose` hint
	information bool
}

// codexScan is the transcript scan (2.2): from start upward, the first stop
// (a prompt or a terminator) classifies the transcript's end. Every other row
// is passed over, never skipped by its text. A row that was not parsed could
// be the stop, so it ends the scan: unproven when dropped, unreadable when
// unparseable.
func codexScan(screen screen, start int) codexTranscript {
	scan := codexTranscript{ending: codexUnproven, stop: -1, status: -1, otherThread: -1, questions: -1, banner: -1}
	for index := start; index >= 0; index-- {
		row := screen.rows[index]
		if row.presence != rowParsed {
			if row.presence == rowUnparseable {
				scan.ending = codexUnreadable
			}
			return scan
		}
		switch {
		case codexPrompt(row):
			scan.ending, scan.stop = codexOpen, index
			return scan
		case codexTerminator(row):
			scan.ending, scan.stop = codexSettled, index
			return scan
		}
		if scan.status < 0 && codexStatusRow(row) {
			scan.status = index
		}
		if !codexIndented(row) {
			continue
		}
		label := codexCells(row, 2)
		text := cellText(label)
		switch {
		case strings.HasPrefix(text, "! Approval needed") && label[0].style.bold && label[0].style.fg == codexRed:
			scan.otherThread = index
		case strings.HasPrefix(text, "? ") && label[0].style.dim &&
			codexQuestionCount.MatchString(cellText(label[2:2+codexKey(label[2:])])):
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
	text := cellText(cells)
	name := codexSpells(cells, 3, "OpenAI Codex")
	return codexDim(row.cells) && (strings.HasPrefix(text, "Worked for ") || codexClock.MatchString(text)) ||
		strings.HasPrefix(text, ">_ ") && name >= 0 && !slices.ContainsFunc(cells[3:name], func(cell cell) bool { return !cell.style.bold })
}

// codexImageRow is a row of `[Image #N]` labels at column 2, coloured and not
// dim: an image-only prompt in the transcript, or remote images in the band.
func codexImageRow(row row) bool {
	cells := codexCells(row, 2)
	return codexIndented(row) && codexImageLabels.MatchString(cellText(cells)) &&
		!slices.ContainsFunc(cells, func(cell cell) bool {
			return cell.text != " " && (cell.style.fg.kind == colorDefault || cell.style.dim)
		})
}

// codexStatusRow is the status row (2.2): a header from column 0 (with or
// without the activity glyph), a space, then the dim elapsed group in full
// `(D • K to interrupt)`, hintless `(D)` or cut before a final `…`,
// optionally followed by dim ` · ` details.
func codexStatusRow(row row) bool {
	cells := codexCells(row, 0)
	// History cells, the hook row and the reduced-motion static bullet start
	// with a dim `• `; the status glyph never does.
	if len(cells) == 0 || cells[0].text == " " || cells[0].text == "•" && cells[0].style.dim && row.cell(1).text == " " {
		return false
	}
	last := len(cells) - 1
	tail := func(from int) bool { return from > last || cells[from].text == " " && codexDim(cells[from:]) }
	for open := 2; open <= last; open++ {
		if cells[open].text != "(" || !cells[open].style.dim || cells[open-1].text != " " {
			continue
		}
		after := open + 1 + len(codexDuration.FindString(cellText(cells[open+1:]))) // D is ASCII: one cell per byte
		// Cut: dim through ` • `, then the key's bold run, then dim; the `…`
		// takes the style of whatever it follows.
		if cells[last].text == "…" && codexCutGroup.MatchString(cellText(cells[open+1:last])) {
			key := codexSpells(cells, after, " • ")
			if key < 0 {
				key = last
			}
			if codexDim(cells[open:key]) && codexDim(cells[key+codexKey(cells[key:last]):last]) {
				return true
			}
		}
		if after == open+1 || !codexDim(cells[open:after]) {
			continue
		}
		if closed := codexSpells(cells, after, ")"); closed >= 0 && cells[after].style.dim && tail(closed) {
			return true
		}
		if bullet := codexSpells(cells, after, " • "); bullet >= 0 && codexDim(cells[after:bullet]) {
			key := bullet + codexKey(cells[bullet:])
			if end := codexSpells(cells, key, " to interrupt)"); key > bullet && end >= 0 && codexDim(cells[key:end]) && tail(end) {
				return true
			}
		}
	}
	return false
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

// codexSpells reports the column after token when cells from at spell it, one
// rune per cell, or -1.
func codexSpells(cells []cell, at int, token string) int {
	for _, char := range token {
		if at >= len(cells) || cells[at].text != string(char) {
			return -1
		}
		at++
	}
	return at
}

// codexToken is codexSpells for a whole word: the cells end after token or
// continue with a space.
func codexToken(cells []cell, at int, token string) int {
	next := codexSpells(cells, at, token)
	if next < 0 || next < len(cells) && cells[next].text != " " {
		return -1
	}
	return next
}

// codexSplit splits cells into members at separator.
func codexSplit(cells []cell, separator string) [][]cell {
	var members [][]cell
	from := 0
	for column := 0; column < len(cells); column++ {
		if next := codexSpells(cells, column, separator); next >= 0 {
			members = append(members, cells[from:column])
			from, column = next, next-1
		}
	}
	return append(members, cells[from:])
}

// codexRun is a maximal span of cells that agree on bold: a key label, or the
// text between key labels. Like codexDim, dim and plain read only glyphs.
type codexRun struct {
	text  string
	bold  bool
	dim   bool // every glyph is dim
	plain bool // no glyph is dim
}

func codexRuns(cells []cell) []codexRun {
	var runs []codexRun
	for _, cell := range cells {
		if len(runs) == 0 || runs[len(runs)-1].bold != cell.style.bold {
			runs = append(runs, codexRun{bold: cell.style.bold, dim: true, plain: true})
		}
		run := &runs[len(runs)-1]
		run.text += cell.text
		if cell.text != " " {
			run.dim, run.plain = run.dim && cell.style.dim, run.plain && !cell.style.dim
		}
	}
	return runs
}
