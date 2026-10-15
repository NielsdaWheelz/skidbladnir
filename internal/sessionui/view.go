package sessionui

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/charmbracelet/x/ansi"
)

// Layout uses plain sanitized text; finished fragments receive design-language
// accents. Labels remain complete without color.
var (
	plain    = ansi.Style{}
	bold     = ansi.Style{}.Bold()
	faint    = ansi.Style{}.Faint()
	wordmark = ansi.Style{}.Reverse(true)
	here     = ansi.Style{}.ForegroundColor(ansi.BrightYellow)
	frost    = ansi.Style{}.ForegroundColor(color.RGBA{R: 0x78, G: 0xA9, B: 0xC6, A: 0xff})
	moss     = ansi.Style{}.ForegroundColor(color.RGBA{R: 0x76, G: 0xB0, B: 0x82, A: 0xff})
	muted    = ansi.Style{}.ForegroundColor(color.RGBA{R: 0xAA, G: 0xA6, B: 0x9D, A: 0xff})
	ember    = ansi.Style{}.ForegroundColor(ansi.BrightRed)
	danger   = ansi.Style{}.Bold().ForegroundColor(ansi.BrightRed)
)

type hint struct{ key, label string }

func (m *model) View() tea.View {
	var content string
	if m.width < 80 || m.height < 24 {
		// the outcome still shows, so a refused ctrl-c says why.
		lines := append(wrapped("80 × 24 minimum; ctrl-c quits", m.width), m.outcomeLines(m.width)...)
		content = strings.Join(lines[:min(len(lines), max(1, m.height))], "\n")
	} else {
		footer := m.footerLines()
		lines := []string{m.header(), ""}
		lines = append(lines, m.bodyLines(m.height-len(lines)-len(footer))...)
		for len(lines) < m.height-len(footer) {
			lines = append(lines, "")
		}
		lines = append(lines, footer...)
		for index, line := range lines {
			lines[index] = " " + line
		}
		content = strings.Join(lines, "\n")
	}
	if os.Getenv("NO_COLOR") != "" {
		content = ansi.Strip(content)
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.ReportFocus = true
	return view
}

// header is the wordmark, then the strip of views in stepping order.
func (m *model) header() string {
	room := m.width - 2 - 7
	tabs, current := []tab{}, 0
	for index, view := range m.viewOptions() {
		switch view.kind {
		case viewNeedsInput:
			tabs = append(tabs, tab{fixed: "needs input"})
		case viewAll:
			tabs = append(tabs, tab{fixed: "all"})
		case viewGroup:
			if view.label.IsUnassigned() {
				tabs = append(tabs, tab{fixed: "unassigned"})
			} else {
				tabs = append(tabs, tab{label: attention.SingleLine(view.label.String())})
			}
		default:
			panic("invalid selected view") // justify-defect: viewOptions emits only the closed view kinds.
		}
		if view == m.view {
			current = index
		}
	}
	return wordmark.Styled(" skid ") + " " + strip(tabs, current, room, m.page == "" && !m.busy)
}

// tab is one view in the strip: fixed is skid's own words and never truncates;
// label is an operator's group label and may.
type tab struct{ fixed, label string }

// strip lays out tabs in at most room cells, for room ≥ 45. needs input and all
// always show whole. group labels share one cap, the largest from 24 down to 7
// that fits; while a group is current the fit reserves room for the widest
// label whole, so stepping between groups keeps the cap. below the floor, the
// groups farthest from the current view hide behind counted markers. the
// current label shows whole up to 24 cells.
func strip(tabs []tab, current, room int, stepping bool) string {
	cells := func(t tab, limit int) int {
		if t.label != "" {
			return min(ansi.StringWidth(t.label), limit)
		}
		return ansi.StringWidth(t.fixed)
	}
	marker := func(hidden int) int {
		if hidden == 0 {
			return 0
		}
		return len(strconv.Itoa(hidden)) + 3
	}
	// width measures groups first..last shown: the current one capped at
	// whole, the others at limit.
	width := func(limit, whole, first, last int) int {
		total := cells(tabs[0], 0) + cells(tabs[1], 0) + 4 + marker(first-2) + marker(len(tabs)-1-last)
		for index := first; index <= last; index++ {
			if index == current {
				total += cells(tabs[index], whole) + 2
			} else {
				total += cells(tabs[index], limit) + 2
			}
		}
		return total
	}
	fits := func(limit, first, last int) bool {
		reserve := 0
		if current >= 2 {
			for _, t := range tabs[2:] {
				reserve = max(reserve, cells(t, 24)-cells(t, limit))
			}
		}
		return width(limit, limit, first, last)+reserve <= room
	}
	limit, first, last := 24, 2, len(tabs)-1
	for limit > 7 && !fits(limit, first, last) {
		limit--
	}
	for !fits(limit, first, last) && first < last {
		// hide the group farthest from the current view; ties hide the left
		// one, and with needs input or all current, groups hide from the right.
		if current-first >= last-current && first != current {
			first++
		} else {
			last--
		}
	}
	// only a current group standing alone can still overflow; its label
	// gives up the excess.
	whole := 24
	if over := width(limit, whole, first, last) - room; over > 0 && current >= 2 {
		whole = max(1, min(ansi.StringWidth(tabs[current].label), 24)-over)
	}
	line := ""
	for index, t := range tabs {
		if index >= 2 && (index < first || index > last) {
			continue
		}
		if index == first && first > 2 {
			line += " ←" + strconv.Itoa(first-2) + " "
		}
		size := limit
		if index == current {
			size = whole
		}
		text := t.fixed
		if t.label != "" {
			text = ansi.Truncate(t.label, size, "…")
		}
		switch {
		case index == current && stepping:
			line += here.Styled("‹") + bold.Styled(text) + here.Styled("›")
		case index == current:
			line += " " + bold.Styled(text) + " "
		default:
			line += " " + text + " "
		}
	}
	if last < len(tabs)-1 {
		line += " " + strconv.Itoa(len(tabs)-1-last) + "→ "
	}
	return line
}

// footerLines holds notices, quota context, the content boundary, and the keys.
// Only pages name a captured target in the boundary.
func (m *model) footerLines() []string {
	width := m.width - 2
	notices := m.noticeLines(width)
	usage := m.usageSummary()
	keys := keyLines(width, m.hints()...)
	legend, end := "", ""
	switch m.page {
	case "":
		if m.selectedRow() != nil {
			layout := m.tableLayout(width)
			height := m.height - 3 - len(usage) - len(notices) - len(keys)
			if m.top > 0 || layout.viewport(m.top, height).end < len(m.rows) {
				end = faint.Styled(fmt.Sprintf(" %d of %d", m.cursor+1, len(m.rows)))
			}
		}
	case "confirm":
		legend = target(m.pendingName, m.pendingLabel, width)
	case "details", "group-edit", "name-edit":
		legend = target(m.pageRow.session.Name, m.pageRow.label, width)
	case "create":
		legend = "new session on " + ansi.Truncate(attention.SingleLine(m.form[0]), width/2, "…")
	case "usage":
		legend = "usage · read only"
	}
	lines := append(notices, usage...)
	lines = append(lines, rule(legend, end, width))
	return append(lines, keys...)
}

// outcomeLines is the notice, wrapped, in its tone.
func (m *model) outcomeLines(width int) []string {
	lines := []string{}
	if m.notice == "" {
		return lines
	}
	style := plain
	if m.noticeFailure {
		style = ember
	}
	for _, line := range wrapped(m.notice, width) {
		lines = append(lines, style.Styled(line))
	}
	return lines
}

func (m *model) noticeLines(width int) []string {
	// Outcomes precede host notices: unavailable labels cannot conceal uncertainty.
	lines := m.outcomeLines(width)
	if m.notificationFailed {
		lines = append(lines, faint.Styled("notifications unavailable"))
	}
	// A peer without an error is unobserved or being checked, not unavailable.
	for _, peer := range m.peers {
		if !peer.OK && peer.Error != nil {
			suffix := " unavailable (" + attention.SingleLine(peer.Error.Code) + "); showing its last observation"
			lines = append(lines, ansi.Truncate(ansi.Truncate(attention.SingleLine(peer.Label), max(1, width-ansi.StringWidth(suffix)), "…")+suffix, width, "…"))
		}
		text, failure := fleetclient.RecoveryNotice(peer)
		if text != "" {
			style := muted
			if failure {
				style = ember
			}
			for _, line := range wrapped(text, width) {
				lines = append(lines, style.Styled(line))
			}
		}
	}
	return lines
}

func (m *model) hints() [][]hint {
	switch m.page {
	case "confirm":
		effect := "interrupt and close terminal"
		if m.pending.Operation == "stop" {
			effect = "send interrupt"
		}
		return [][]hint{{{"enter", effect}, {"escape", "cancel"}}}
	case "group-edit", "name-edit":
		if m.metadata.checking {
			return [][]hint{{{"ctrl-r", "refresh"}, {"escape", "info"}}}
		}
		if m.page == "group-edit" {
			keys := []hint{{"enter", "save"}, {"tab", "group"}, {"shift-tab", "back"}, {"ctrl-s", "save draft"}, {"escape", "cancel"}}
			if !m.metadata.saveFocused {
				keys[0].label, keys[1].label = "uses and continues", "uses and continues"
				keys = append([]hint{{"←→", "choose"}, {"ctrl-u", "unassigned"}}, keys...)
			}
			return [][]hint{keys}
		}
		keys := []hint{{"enter", "save name"}, {"escape", "cancel"}}
		if row := m.rowForReference(m.pageRow.session.Ref); row != nil && row.available && row.session.NameMode == "manual" {
			keys = append(keys, hint{"ctrl-a", "use automatic title"})
		}
		return [][]hint{keys}
	case "create":
		enter := hint{"enter", "next"}
		next := hint{"tab", "next"}
		if m.field == len(m.form) {
			enter.label = "create"
		} else if m.field == 4 {
			enter.label = "uses and continues"
			next.label = "uses and continues"
		}
		hints := []hint{next, {"shift-tab", "back"}, enter, {"escape", "cancel"}}
		if m.field == 0 || m.field == 1 || m.field == 3 || m.field == 4 {
			hints = append([]hint{{"←→", "choose"}}, hints...)
		}
		if m.field == 3 {
			hints = append(hints, hint{"ctrl-u", "home"})
		} else if m.field == 4 {
			hints = append(hints, hint{"ctrl-u", "unassigned"})
		}
		return [][]hint{hints}
	case "details":
		keys := []hint{{"↑↓", "scroll"}, {"pgup pgdn", "page"}, {"escape", "back"}}
		if row := m.rowForReference(m.pageRow.session.Ref); row != nil && row.available && (m.metadata == nil || !m.metadata.checking) {
			keys = append([]hint{{"r", "name"}, {"g", "group"}}, keys...)
		}
		return [][]hint{keys}
	case "usage":
		return [][]hint{{{"↑↓ j/k", "scroll"}, {"pgup pgdn", "page"}, {"ctrl-r", "refresh usage"}, {"q escape", "back"}}}
	}
	session := []hint{}
	global := []hint{{"n", "quick terminal"}, {"N", "new terminal options"}}
	if row := m.selectedRow(); row != nil && row.available {
		session = append(session, hint{"enter", "attach"}, hint{"space", "info"})
		session = append(session, hint{"s", "send interrupt"}, hint{"x", "interrupt and close terminal"})
		if !m.busy && row.session.Connection == nil {
			global = append(global, hint{"T", "new terminal here"})
		}
	} else if row != nil {
		session = append(session, hint{"space", "info"})
	}
	global = append(global, hint{"u", "usage"}, hint{"q", "quit"})
	return [][]hint{session, global}
}

// keyLines keeps each group on one line when it fits, otherwise wraps it by
// whole hints, so a key never separates from its label.
func keyLines(width int, groups ...[]hint) []string {
	lines := []string{}
	for _, group := range groups {
		items := []string{}
		for _, hint := range group {
			items = append(items, bold.Styled(hint.key)+" "+hint.label)
		}
		if len(items) == 0 {
			continue
		}
		joined := strings.Join(items, "  ")
		if last := len(lines) - 1; last >= 0 && ansi.StringWidth(lines[last])+4+ansi.StringWidth(joined) <= width {
			lines[last] += "    " + joined
			continue
		}
		line := ""
		for _, item := range items {
			if line != "" && ansi.StringWidth(line)+2+ansi.StringWidth(item) > width {
				lines, line = append(lines, line), ""
			}
			if line != "" {
				line += "  "
			}
			line += item
		}
		lines = append(lines, line)
	}
	return lines
}

// rule separates content and controls. A page legend names its target; the
// table's styled end carries its scrolled position.
func rule(legend, end string, width int) string {
	if end != "" {
		end += faint.Styled(" ──")
	}
	fill := width - ansi.StringWidth(end)
	if legend == "" {
		return faint.Styled(strings.Repeat("─", fill)) + end
	}
	legend = ansi.Truncate(legend, max(1, fill-5), "…")
	return faint.Styled("── ") + legend + " " + faint.Styled(strings.Repeat("─", max(0, fill-4-ansi.StringWidth(legend)))) + end
}

func target(name, machine string, width int) string {
	half := max(1, (width-10)/2)
	return bold.Styled(ansi.Truncate(attention.SingleLine(name), half, "…")) + faint.Styled(" on ") + ansi.Truncate(attention.SingleLine(machine), half, "…")
}

func (m *model) bodyLines(height int) []string {
	if height <= 0 {
		return nil
	}
	width := m.width - 2
	switch m.page {
	case "confirm":
		action, effect := "interrupt and close terminal", "send one interrupt to the selected pane, then close this entire session. closure proceeds even if interruption fails. work shared elsewhere or running remotely may continue."
		if m.pending.Operation == "stop" {
			action, effect = "send interrupt", "send one interrupt to the selected pane; retain this session. stopping is unconfirmed."
		}
		lines := []string{}
		for _, line := range wrapped(action+" "+capturedHeading(m.pendingName, m.pendingLabel, width-len(action)-2)+"?", width) {
			lines = append(lines, danger.Styled(line))
		}
		ref, _ := fleetclient.DecodeReference(m.pending.Ref)
		lines = append(lines, wrapped("selected pane "+ref.PaneID+" · terminal host "+attention.SingleLine(m.pendingLabel), width)...)
		return window(append(lines, wrapped(effect, width)...), 0, 0, height)
	case "group-edit", "name-edit":
		editor := m.metadata
		title, label, draft, axis := "group", "group", groupDraftDisplay(editor.draft), 7
		if m.page == "name-edit" {
			title, label, draft, axis = "rename session", "session name", editor.draft, 12
			if !fleetclient.ValidSessionName(draft) {
				draft = strconv.QuoteToASCII(draft)
			}
		}
		lines := []string{bold.Styled(title), ""}
		if editor.failure != nil {
			lines = append(lines, wrapped(fleetclient.ErrorMessage(*editor.failure, editor.request, false), width)...)
			lines = append(lines, "")
		}
		current := m.rowForReference(editor.request.Ref)
		if current != nil {
			value := fleetclient.GroupHeading(current.session.Group)
			if m.page == "name-edit" {
				value = current.session.Name + " (" + current.session.NameMode + ")"
			}
			lines = append(lines, field("current", value, false, false, axis, width)...)
		}
		focusStart := len(lines)
		lines = append(lines, field(label, draft, !editor.checking && !m.busy && !editor.saveFocused, false, axis, width)...)
		focusEnd := len(lines)
		status := "enter saves"
		if m.page == "group-edit" {
			if editor.saveFocused {
				focusStart = len(lines)
			} else {
				lines = append(lines, m.groupPreview(editor.draft, &editor.selection, axis, width)...)
				focusEnd = len(lines)
				status = "left/right chooses; enter uses and continues"
			}
			lines = append(lines, field("", "save", editor.saveFocused && !editor.checking && !m.busy, false, axis, width)...)
			if editor.saveFocused {
				focusEnd = len(lines)
			}
		}
		switch {
		case m.busy:
			status = "saving; delivery will be reported"
		case editor.checking:
			status = "checking inventory; escape returns to info"
		case current == nil || !current.available:
			status = "session unavailable; save disabled"
		case m.page == "group-edit":
			value, err := group.ParseDraft(editor.draft)
			if err != nil {
				status = group.ErrInvalid.Error()
			} else if current.session.Group == value {
				status = "unchanged; save disabled"
			}
		case !fleetclient.ValidSessionName(editor.draft):
			status = fleetclient.ErrorMessage(fleetclient.Failure{Code: "SessionNameInvalid"}, editor.request, false)
		case current.session.NameMode == "manual" && current.session.Name == editor.draft:
			status = "unchanged; save disabled"
		}
		lines = append(lines, "")
		lines = append(lines, wrapped(status, width)...)
		lines = append(lines, "")
		if m.page == "name-edit" {
			lines = append(lines, "saving a name stops automatic naming.")
			if current != nil && current.session.NameMode == "automatic" {
				lines = append(lines, "follows the active pane's terminal title.")
			}
		}
		return window(lines, focusStart, focusEnd, height)
	case "create":
		lines := []string{bold.Styled("new session"), ""}
		focusStart, focusEnd := 0, 0
		for index, label := range []string{"machine", "launch", "name", "directory", "group"} {
			value := m.form[index]
			if index == 3 {
				if _, _, problem := parseDirectoryDraft(value); problem != "" {
					value = strconv.QuoteToASCII(value)
				}
				if value == "" && m.field != 3 {
					value = "home (~)"
				}
			}
			if index == 4 {
				value = groupDraftDisplay(value)
			}
			if index == m.field {
				focusStart = len(lines)
			}
			lines = append(lines, field(label, value, index == m.field, index < 2, 9, width)...)
			if index == 3 && m.field == 3 {
				if value == "" {
					lines[len(lines)-1] += faint.Styled(" home (~); type to search")
				}
				if len(m.searchDirectories) > 0 {
					preview := m.directorySelection.preview(m.searchDirectories, func(path string) string { return path })
					lines = append(lines, field("use", preview, false, false, 9, width)...)
				}
				if status := m.directoryStatus(); status != "" {
					lines = append(lines, field("", status, false, false, 9, width)...)
				}
			}
			if index == 4 && m.field == 4 {
				lines = append(lines, m.groupPreview(m.form[4], &m.groupSelection, 9, width)...)
			}
			if index == m.field {
				focusEnd = len(lines)
			}
		}
		if m.field == len(m.form) {
			focusStart = len(lines)
		}
		lines = append(lines, field("", "create", m.field == len(m.form), false, 9, width)...)
		if m.field == len(m.form) {
			focusEnd = len(lines)
		}
		lines = append(lines, "", "name: leave blank to follow the terminal title.")
		if m.field == 3 {
			lines = append(lines, "type search words or a path (/… or ~/…); tab uses the match.")
		}
		if !m.createAvailable() {
			status := "host unavailable; create disabled"
			if m.targetChecking(m.form[0]) {
				status = "checking inventory; create disabled"
			}
			lines = append(lines, status)
		}
		if _, err := group.ParseDraft(m.form[4]); err != nil {
			lines = append(lines, wrapped(group.ErrInvalid.Error(), width)...)
		}
		return window(lines, focusStart, focusEnd, height)
	case "details":
		title := bold.Styled("info")
		// The title stays pinned; only the captured snapshot scrolls.
		body := m.detailLines()
		offset := min(m.offset, max(0, len(body)-(height-2)))
		return append([]string{title, ""}, body[offset:min(len(body), offset+max(0, height-2))]...)
	case "usage":
		return m.usageBody(height)
	}
	if len(m.rows) == 0 {
		partial := false
		for _, peer := range m.peers {
			partial = partial || !peer.OK
		}
		switch {
		case !m.inventoryReceived:
			return []string{"checking inventory"}
		case m.view.kind == viewNeedsInput:
			return []string{"no sessions currently need input in this view"}
		case partial:
			return []string{"no matching sessions in available inventory"}
		}
		return []string{"no sessions in this view"}
	}
	layout := m.tableLayout(width)
	return m.tableLines(layout, layout.viewport(m.top, height))
}

// window keeps a form's focused field visible; a field taller than the page
// keeps its label and current tail, without a second editor.
func window(lines []string, focusStart, focusEnd, height int) []string {
	start := 0
	if focusEnd > height {
		start = focusEnd - height
		if focusEnd-focusStart > height {
			return append([]string{lines[focusStart]}, lines[focusEnd-height+1:focusEnd]...)
		}
	}
	return lines[start:min(len(lines), start+height)]
}

// field right-aligns its label so a form reads down one axis; long values wrap
// under the value column. focused choices show ‹ › and focused text a caret.
func field(label, value string, focused, choice bool, labelWidth, width int) []string {
	gutter, name := "  ", faint.Styled(fmt.Sprintf("%*s", labelWidth, label))
	if focused {
		gutter, name = here.Styled("▌")+" ", bold.Styled(fmt.Sprintf("%*s", labelWidth, label))
	}
	indent := strings.Repeat(" ", labelWidth+4)
	valueWidth := max(1, width-labelWidth-4-4)
	if focused && choice {
		return []string{gutter + name + "  " + faint.Styled("‹ ") + bold.Styled(ansi.Truncate(attention.SingleLine(value), valueWidth, "…")) + faint.Styled(" ›")}
	}
	if value == "" && !focused {
		return []string{gutter + name + "  " + faint.Styled("—")}
	}
	parts := strings.Split(ansi.Hardwrap(attention.SingleLine(value), valueWidth, true), "\n")
	if focused {
		parts[len(parts)-1] += wordmark.Styled(" ")
	}
	lines := []string{gutter + name + "  " + parts[0]}
	for _, part := range parts[1:] {
		lines = append(lines, indent+part)
	}
	return lines
}

func (m *model) groupPreview(draft string, selection *groupSelection, axis, width int) []string {
	choices := m.groupChoices(draft)
	lines := field("", "observed groups", false, false, axis, width)
	if choices[0].literal || choices[0].label.IsUnassigned() {
		lines = append(lines, field("", "no observed matches", false, false, axis, width)...)
	}
	preview := selection.preview(choices, func(choice groupChoice) string {
		value := "label: " + choice.label.String()
		if choice.label.IsUnassigned() {
			value = "unassigned (no membership)"
		} else if choice.literal {
			value = "use label: " + choice.label.String()
		}
		return value
	})
	return append(lines, field("use", preview, false, false, axis, width)...)
}

func (m *model) detailLines() []string {
	width := m.width - 2
	facts := m.details(&m.pageRow)
	row := m.rowForReference(m.pageRow.session.Ref)
	axis := 16
	for index := range facts {
		label := facts[index][0]
		if row != nil && row.available && (m.metadata == nil || !m.metadata.checking) {
			if label == "name" {
				label += " [r]"
			} else if label == "group" {
				label += " [g]"
			}
		}
		facts[index][0] = label
		axis = max(axis, ansi.StringWidth(attention.SingleLine(label)))
	}
	lines := []string{}
	for _, fact := range facts {
		if fact[0] == "" {
			for _, line := range wrapped(fact[1], max(1, width-axis-8)) {
				lines = append(lines, strings.Repeat(" ", axis+4)+line)
			}
			continue
		}
		lines = append(lines, field(fact[0], fact[1], false, false, axis, width)...)
	}
	return lines
}

func (m *model) pageCapacity() int {
	return max(1, m.height-4-len(m.footerLines()))
}

func (m *model) headings() bool {
	return m.view.kind == viewAll
}

// opensGroup reports whether a group heading precedes this row.
func (m *model) opensGroup(index int) bool {
	return m.headings() && (index == 0 || m.rows[index].session.Group != m.rows[index-1].session.Group)
}

// statusView projects a row's terminal status. A row without remote actions
// is stale; readiness also requires a working notification store.
func (m *model) statusView(row listedRow) fleetclient.StatusView {
	view := fleetclient.ProjectStatus(row.session, row.available, !m.notificationFailed && m.notificationSnapshot.Ready(row.session))
	if m.view.kind == viewNeedsInput && view.QueueDetail != "" {
		view.Label += " · " + view.QueueDetail
		view.Detail += " · " + view.QueueDetail
	}
	return view
}

// hostChecking reports a row without remote actions whose host has not failed a
// read: its host is unobserved or being re-read after a metadata change.
func (m *model) hostChecking(row listedRow) bool {
	if row.available {
		return false
	}
	for _, peer := range m.peers {
		if peer.Machine == row.machine && peer.Error != nil {
			return false
		}
	}
	return true
}

// tableStatus is a row's status cell. a checking row makes no status claim and
// reads faint `checking`; any other row reads the projected label in its tone,
// so a row whose host failed a read reads its last observation. an errored
// host also keeps its notice.
func (m *model) tableStatus(row listedRow) (string, ansi.Style) {
	if m.hostChecking(row) {
		return "checking", faint
	}
	view := m.statusView(row)
	switch view.Tone {
	case fleetclient.ToneMuted:
		return view.Label, muted
	case fleetclient.ToneFrost:
		return view.Label, frost
	case fleetclient.ToneEmber:
		return view.Label, ember
	case fleetclient.ToneMoss:
		return view.Label, moss
	}
	panic("invalid status tone") // justify-defect: Tone is a closed enum.
}

// tail keeps the end of a path, where directories differ.
func tail(text string, width int) string {
	if ansi.StringWidth(text) <= width {
		return text
	}
	// TruncateLeft keeps a wide character that straddles the cut, so widen it.
	cut := ansi.StringWidth(text) - width + 1
	for ansi.StringWidth(ansi.TruncateLeft(text, cut, "")) > width-1 {
		cut++
	}
	return "…" + ansi.TruncateLeft(text, cut, "")
}

func capturedHeading(name, machine string, width int) string {
	return ansi.Truncate(attention.SingleLine(name), (width-4)/2, "…") + " on " + ansi.Truncate(attention.SingleLine(machine), (width-4)/2, "…")
}

func groupDraftDisplay(draft string) string {
	if _, err := group.ParseDraft(draft); err != nil {
		return strconv.QuoteToASCII(draft)
	}
	return draft
}

func wrapped(text string, width int) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = ansi.Wrap(attention.SingleLine(line), max(1, width), "")
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

func cell(text string, width int) string {
	text = ansi.Truncate(attention.SingleLine(text), width, "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}
