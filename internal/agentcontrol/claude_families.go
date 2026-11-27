package agentcontrol

import (
	"regexp"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

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
	for _, family := range [...]func(*claudeScreen, int) (string, api.Interaction, bool){
		claudeViewer, claudeMenu, claudePermission, claudeQuestion, claudeElicitation, claudePlan, claudeSetup,
	} {
		if id, interaction, ok := family(screen, hint); ok {
			return reading{
				activity: api.ActivityUnknown, interaction: interaction, composer: composerBlocked,
				rules: []string{id, "claude.composer.blocked"},
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
func claudeViewer(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	text, top, ok := screen.block(2, hint)
	switch {
	case !ok:
	case strings.HasPrefix(text, "dialog waiting · Showing detailed transcript · "):
		return "claude.input.dialog_waiting", api.InteractionInput, true
	case strings.HasPrefix(text, "Showing detailed transcript · "):
		return "claude.menu.transcript", api.InteractionMenu, true
	case claudeAgentsViewHint.MatchString(text):
		if _, found := screen.find(top-1, func(line claudeLine) bool { return claudeAgentsViewCounts.MatchString(line.plain) }); found {
			return "claude.menu.agents_view", api.InteractionMenu, true
		}
	}
	return "", "", false
}

// claudeSettingsTabs is the settings menu's tab row.
var claudeSettingsTabs = regexp.MustCompile(`^   Settings +Status +Config +Usage`)

// The 2.1.332 menus share a ruled header and an inset footer. Tab padding
// depends on selection, and the resume footer changes when sessions exist.
var claudeMenus = [...]struct{ title, hint, id string }{
	{"Select model", "s to use this session only · Enter to save · Esc to cancel", "claude.menu.model"},
	{"Help General Commands Custom commands", "←/→ to switch · Esc to close", "claude.menu.help"},
	{"Settings Status Config Usage Stats", "←/→ to switch · Esc to close", "claude.menu.settings"},
	{"Settings Status Config Usage Stats", "↓ to navigate · ↑ to switch tabs · Esc to go back", "claude.menu.settings"},
	{"Settings Status Config Usage Stats", "↓ to navigate · ←/→ to switch · Esc to close", "claude.menu.settings"},
	{"Theme", "Enter to select · Esc to cancel", "claude.menu.theme"},
	{"Resume session", "Ctrl+A to show all projects · Ctrl+B to only show current branch · Esc to close", "claude.menu.resume"},
	{"Resume session", "Ctrl+A to show all projects · Ctrl+B to only show current branch · Space to preview · Ctrl+R to rename · Type to search · Enter to resume · Esc to close", "claude.menu.resume"},
	{"Permissions Recently blocked Allow Ask Deny Auto mode Workspace", "↓ to navigate · ←/→ to switch · / to search · Esc to close", "claude.menu.permissions"},
	{"Background", "Esc to close", "claude.menu.tasks"},
	{"Effort", "←/→ to adjust · s to use this session only · Enter to save · Esc to cancel", "claude.menu.effort"},
	{"Memory", "↑/↓ to navigate · Enter to select · Esc to close", "claude.menu.memory"},
	{"Rewind", "Enter to continue · Esc to cancel", "claude.menu.rewind"},
	{"/btw", "c to copy · f to fork · Esc to close", "claude.menu.btw"},
}

// 2.1.332 draws the dialog title directly below its divider, in bold at
// the dialog's inset. Stop at the first row left of that inset so a
// title in the transcript or behind another dialog cannot supply the header.
func claudeDialogTitle(screen *claudeScreen, above, inset int, allowEdge bool) string {
	r, ok := screen.find(above-1, func(line claudeLine) bool {
		return line.presence != rowParsed || !line.blank() && line.col < inset
	})
	if !ok || r+1 >= above {
		return ""
	}
	divider := screen.lines[r]
	if !screen.ruleRow(divider) && !(allowEdge && screen.edge(divider) && strings.Trim(divider.plain, "▔") == "") {
		return ""
	}
	title := screen.lines[r+1]
	if title.col != inset || !title.cell(inset).style.bold {
		return ""
	}
	return strings.Join(strings.Fields(title.plain), " ")
}

// claudeMenu reads the model, settings, theme, resume and side-question menus.
func claudeMenu(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	if text, top, ok := screen.block(2, hint); ok && (strings.HasSuffix(text, "Esc to cancel") || strings.HasSuffix(text, "Esc to close") || strings.HasSuffix(text, "Esc to go back")) {
		title := claudeDialogTitle(screen, top, 2, true)
		if strings.HasPrefix(title, "/btw ") {
			title = "/btw"
		}
		for _, menu := range claudeMenus {
			if title == menu.title && text == menu.hint {
				return menu.id, api.InteractionMenu, true
			}
		}
	}
	if text, _, ok := screen.block(3, hint); ok {
		switch {
		case strings.HasPrefix(text, "Enter to set as default · "):
			if _, found := screen.find(hint-1, screen.edge); found {
				return "claude.menu.model", api.InteractionMenu, true
			}
		case strings.HasPrefix(text, "Type to filter · ") || strings.HasPrefix(text, "Enter/Space to change · "):
			if _, found := screen.find(hint-1, func(line claudeLine) bool { return claudeSettingsTabs.MatchString(line.plain) }); found {
				return "claude.menu.settings", api.InteractionMenu, true
			}
		case text == "Enter to select · Esc to cancel":
			if _, found := screen.find(hint-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    Syntax theme: ") }); found {
				return "claude.menu.theme", api.InteractionMenu, true
			}
		}
	}
	if text, _, ok := screen.block(5, hint); ok && strings.Contains(text, "to show all projects") && strings.HasSuffix(text, "Esc to cancel") {
		return "claude.menu.resume", api.InteractionMenu, true
	}
	if text, top, ok := screen.block(4, hint); ok && strings.HasSuffix(text, "Esc to close") {
		_, question := screen.find(top-1, func(line claudeLine) bool { return strings.HasPrefix(line.plain, "    /btw ") })
		if _, found := screen.find(top-1, screen.edge); found && question {
			return "claude.menu.btw", api.InteractionMenu, true
		}
	}
	return "", "", false
}

// The tool and workflow permission hints; a <chord> is any one token.
var (
	claudePermissionHint = regexp.MustCompile(`^\S+ to cancel(?: · \S+ to amend)?$`)
	claudeWorkflowHint   = regexp.MustCompile(`^\S+ to cancel(?: · \S+ to amend)?(?: \S+ to edit script in \$EDITOR)?$`)
)

// claudePermission reads the tool permission dialog (options and hint at
// column 1) and the dynamic workflow permission (column 2, titled at column 1).
func claudePermission(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	if text, top, ok := screen.block(1, hint); ok && claudePermissionHint.MatchString(text) {
		options, optionsTop, ok := screen.options(1, top-1)
		if ok && strings.HasPrefix(options[0].label, "Yes") {
			if _, questionTop, end, ok := screen.above(1, optionsTop); ok {
				for r := end; r >= questionTop; r-- {
					if strings.HasPrefix(screen.lines[r].plain[screen.lines[r].col:], "Do you want to ") {
						if strings.HasSuffix(screen.joined(r, end), "?") {
							return "claude.permission.dialog", api.InteractionPermission, true
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
			if screen.titled(optionsTop, " Run a dynamic workflow?") {
				return "claude.permission.workflow", api.InteractionPermission, true
			}
		}
	}
	return "", "", false
}

// claudeQuestion reads the question form, the question with option previews,
// and the submit review.
func claudeQuestion(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	if text, top, ok := screen.block(0, hint); ok && strings.HasPrefix(text, "Enter to select · ") && screen.blankAt(top-1) {
		chat, chatOK := screen.line(top - 2)
		rule, ruleOK := screen.line(top - 3)
		if chatOK && ruleOK && screen.ruleRow(rule) {
			switch {
			case strings.Contains(text, " · n to add notes · ") && strings.HasSuffix(text, "Esc to cancel") &&
				(chat.plain == "❯ Chat about this" || chat.plain == "  Chat about this"):
				if claudePreview(screen, top-3, strings.HasPrefix(chat.plain, "❯")) {
					return "claude.question.preview", api.InteractionQuestion, true
				}
			case strings.Contains(text, " to navigate") && strings.HasSuffix(text, "to cancel") && !strings.Contains(text, "n to add notes"):
				number, label, pointer, start := claudeOptionStart(chat.plain, 0)
				options, _, ok := screen.options(0, top-4)
				if start && label == "Chat about this" && number > 0 && ok {
					for _, option := range options {
						pointer = pointer || option.selected
					}
					if pointer {
						return "claude.question.form", api.InteractionQuestion, true
					}
				}
			}
		}
	}
	options, top, ok := screen.options(0, hint)
	if ok && len(options) == 2 && options[0].label == "Submit answers" && options[1].label == "Cancel" {
		// 2.1.332 omits the confirmation sentence. The submit tab and
		// review heading frame the indented answers above these choices.
		if options[0].selected || options[1].selected {
			r, found := screen.find(top-1, func(line claudeLine) bool { return line.presence != rowParsed || line.col == 0 })
			if found && screen.lines[r].plain == "Review your answers" && screen.lines[r].cell(0).style.bold {
				tabs, ok := screen.line(r - 2)
				rule, ruled := screen.line(r - 3)
				if ok && strings.HasPrefix(tabs.plain, "←  ☒ ") && strings.HasSuffix(tabs.plain, " ✔ Submit  →") &&
					screen.blankAt(r-1) && ruled && screen.ruleRow(rule) {
					return "claude.question.review", api.InteractionQuestion, true
				}
			}
		}
		if screen.textBlock(0, hint, func(text string) bool { return strings.Contains(text, "Ready to submit your answers?") }) &&
			screen.textBlock(0, hint, func(text string) bool { return strings.Contains(text, "Review your answers") }) {
			return "claude.question.review", api.InteractionQuestion, true
		}
	}
	return "", "", false
}

// claudePreview reads a question's side-by-side options and preview box above
// its rule. The box's corner `┌` is on the row of option 1, the block's first
// row; a lower `┌` belongs to a box the preview's own markdown draws, so the
// corner is the lowest one whose cells to its left start option 1. The cells
// left of the corner's column, from its row down, are the options. It holds
// when a pointer marks an option or the chat row.
func claudePreview(screen *claudeScreen, rule int, chatPointer bool) bool {
	left := func(r, column int) claudeLine {
		cut := screen.lines[r].row
		cut.cells = cut.cells[:min(column, len(cut.cells))]
		return newClaudeLine(cut)
	}
	for corner := rule - 1; ; corner-- {
		line, ok := screen.line(corner)
		if !ok {
			return false
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
			return false
		}
		for _, option := range options {
			chatPointer = chatPointer || option.selected
		}
		return chatPointer
	}
}

// claudeElicitationTitle is an mcp server's elicitation title: the server
// name, then the rest of its block.
var claudeElicitationTitle = regexp.MustCompile(`^MCP server “.*?” (.*)$`)

// claudeElicitation reads an mcp server's elicitation form. Its title starts a
// block at column 2: classic wraps it, while fullscreen keeps it on one row,
// cutting the server name and then the title's ending to a prefix and `…`. A
// task's elicitation appends ` (task <id>)` to the ending, which fullscreen
// draws only when it fits uncut, so a space and more text may follow it.
func claudeElicitation(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
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
		if screen.textBlock(2, hint, title) {
			return "claude.input.elicitation", api.InteractionInput, true
		}
	}
	return "", "", false
}

// claudePlanEditHint is plan approval's optional editor hint.
var claudePlanEditHint = regexp.MustCompile(`^\S+ to edit in `)

// claudePlan reads plan approval and the exit- and enter-plan decisions.
func claudePlan(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	end := hint
	if text, top, ok := screen.block(3, hint); ok && claudePlanEditHint.MatchString(text) {
		end = top - 1
	}
	if options, optionsTop, ok := screen.options(3, end); ok && strings.HasPrefix(options[0].label, "Yes") {
		change := false
		for _, option := range options {
			change = change || strings.HasPrefix(option.label, "Tell Claude what to change")
		}
		if text, _, _, ok := screen.above(3, optionsTop); ok && change && strings.HasPrefix(text, "Claude has written up a plan and is ready to execute.") {
			return "claude.confirmation.plan", api.InteractionConfirmation, true
		}
	}
	if options, optionsTop, ok := screen.options(4, hint); ok && strings.HasPrefix(options[0].label, "Yes, and switch to ") && options[len(options)-1].label == "No" {
		if text, _, _, ok := screen.above(4, optionsTop); ok && text == "Claude wants to exit plan mode" {
			return "claude.confirmation.exit_plan", api.InteractionConfirmation, true
		}
	}
	// 2.1.332 insets the exit form inside its own rule and aligns the
	// two choices with the title. Keep the form bound to that heading.
	if options, top, ok := screen.options(3, hint); ok && len(options) == 2 &&
		strings.HasPrefix(options[0].label, "Yes, and switch to ") && options[1].label == "No" &&
		(options[0].selected || options[1].selected) {
		if text, body, _, ok := screen.above(3, top); ok && text == "Claude wants to exit plan mode" {
			title := body - 2
			line, ok := screen.line(title)
			rule, ruled := screen.line(title - 1)
			outer, framed := screen.line(title - 3)
			if ok && line.plain == "   Exit plan mode?" && line.cell(3).style.bold &&
				screen.blankAt(title+1) && screen.blankAt(title-2) && framed && screen.edge(outer) &&
				ruled && rule.col == 2 && len(rule.cells) == screen.width-2 && strings.Trim(rule.plain[2:], "─") == "" {
				return "claude.confirmation.exit_plan", api.InteractionConfirmation, true
			}
		}
	}
	// Entering plan mode draws no hint; its confirm label names the mode.
	if options, optionsTop, ok := screen.unnumbered(2, hint); ok && strings.HasSuffix(options, "No, start implementing now") {
		if screen.titled(optionsTop, " Enter plan mode?") {
			return "claude.confirmation.enter_plan", api.InteractionConfirmation, true
		}
	}
	if options, top, ok := screen.unnumbered(1, hint); ok && strings.Contains(options, "❯ ") &&
		strings.Replace(options, "❯ ", "", 1) == "Yes, and switch to plan mode (research and propose changes without making them) for this session No, start implementing now" &&
		claudeDialogTitle(screen, top, 1, false) == "Enter plan mode?" {
		return "claude.confirmation.enter_plan", api.InteractionConfirmation, true
	}
	return "", "", false
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
func claudeSetup(screen *claudeScreen, hint int) (string, api.Interaction, bool) {
	const confirm = "Enter to confirm · Esc to cancel"
	if options, top, ok := screen.unnumbered(2, hint); ok && strings.Contains(options, "❯ ") &&
		strings.Replace(options, "❯ ", "", 1) == "Yes, set auto mode as my default permission mode No, keep manual mode" &&
		claudeDialogTitle(screen, top, 2, false) == "Make auto mode your default permission mode?" {
		return "claude.setup.auto_default", api.InteractionSetup, true
	}
	hint1, top1, ok1 := screen.block(1, hint)
	hint2, top2, ok2 := screen.block(2, hint)
	if ok1 && hint1 == confirm {
		options, optionsTop, ok := screen.unnumbered(1, top1-1)
		workspace := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Accessing workspace:") })
		if ok && workspace && claudeWords(options, "No, exit") && claudeWords(options, "Yes, I trust this folder") {
			if _, found := screen.find(optionsTop-1, screen.ruleRow); found {
				return "claude.setup.trust", api.InteractionSetup, true
			}
		}
	}
	if ok2 && (hint2 == confirm || hint2 == "Enter to confirm · Esc to reject") {
		apiKey := screen.textBlock(2, hint, func(text string) bool { return strings.Contains(text, "Detected a custom API key in your environment") })
		useKey := screen.textBlock(2, hint, func(text string) bool { return strings.Contains(text, "Do you want to use this API key?") })
		bypass := screen.textBlock(2, hint, func(text string) bool {
			return strings.HasPrefix(text, "WARNING: Claude Code running in Bypass Permissions mode")
		})
		server := screen.textBlock(2, hint, func(text string) bool { return strings.HasPrefix(text, "New MCP server found in this project: ") })
		settings := screen.textBlock(2, hint, func(text string) bool { return strings.HasPrefix(text, "Settings Error") })
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
		if id != "" && (hint2 == confirm || id == "claude.setup.mcp_server") {
			if _, found := screen.find(optionsTop-1, screen.ruleRow); found {
				return id, api.InteractionSetup, true
			}
		}
	}
	if ok1 && claudeServerHint.MatchString(hint1) && claudeServerApproval(screen, top1) {
		if _, ruled := screen.find(top1-1, screen.ruleRow); ruled && screen.textBlock(2, hint, claudeServersTitle.MatchString) {
			return "claude.setup.mcp_servers", api.InteractionSetup, true
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
			started := screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Let's get started.") })
			if started && oneForm && selected == 1 && claudeWords(screen.joined(top, bottom), "Dark mode (ANSI colors only)") {
				return "claude.setup.theme", api.InteractionSetup, true
			}
		}
	}
	if options, _, ok := screen.options(1, hint); ok && strings.HasPrefix(options[0].label, "Claude account with subscription") {
		if screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Select login method:") }) {
			return "claude.setup.login", api.InteractionSetup, true
		}
	}
	if ok1 && hint1 == "Press Enter to continue…" {
		if screen.textBlock(1, hint, func(text string) bool { return strings.Contains(text, "Security notes:") }) {
			return "claude.setup.security", api.InteractionSetup, true
		}
	}
	return "", "", false
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
