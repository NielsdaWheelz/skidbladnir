package sessionui

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
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
		lines := append([]string{m.header(), ""}, m.bodyLines(m.height-2-len(footer))...)
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
	return view
}

// header is the wordmark, then the strip of views in stepping order, then the
// machine filter at the right edge when one narrows the scope.
func (m *model) header() string {
	machine, room := "", m.width-2-7
	if m.machine != "" {
		machine = ansi.Truncate("machine: "+singleLine(m.machine), 24, "…")
		room -= ansi.StringWidth(machine) + 2
	}
	tabs, current := []tab{{fixed: "agents"}, {fixed: "all"}}, 0
	if !m.agentsView {
		current = 1
	}
	for index, filter := range m.groupOptions()[1:] {
		label := singleLine(filter.Label().String())
		if filter.Kind() == group.FilterUnassigned {
			tabs = append(tabs, tab{fixed: "unassigned"})
		} else {
			tabs = append(tabs, tab{label: label})
		}
		if !m.agentsView && filter == m.groupFilter {
			current = index + 2
		}
	}
	line := wordmark.Styled(" skid ") + " " + strip(tabs, current, room, m.page == "" && !m.busy)
	if machine == "" {
		return line
	}
	return line + strings.Repeat(" ", max(0, m.width-2-ansi.StringWidth(line)-ansi.StringWidth(machine))) + machine
}

// tab is one view in the strip: fixed is skid's own words and never truncates;
// label is an operator's group label and may.
type tab struct{ fixed, label string }

// strip lays out tabs in at most room cells, for room ≥ 45. agents and all
// always show whole. group labels share one cap, the largest from 24 down to 7
// that fits; while a group is current the fit reserves room for the widest
// label whole, so stepping between groups keeps the cap. below the floor, the
// groups farthest from the current view hide behind counted markers. the
// current label shows whole up to 24 cells unless a long machine filter leaves
// no room.
func strip(tabs []tab, current, room int, stepping bool) string {
	cells := func(t tab, limit int) int {
		return ansi.StringWidth(t.fixed) + min(ansi.StringWidth(t.label), limit)
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
		// one, and with agents or all current, groups hide from the right.
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
		text := t.fixed + ansi.Truncate(t.label, size, "…")
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

// footerLines holds notices, the rule naming the session the keys act on, that
// session's facts, and the keys.
func (m *model) footerLines() []string {
	width := m.width - 2
	notices := m.noticeLines(width)
	keys := keyLines(width, m.hints()...)
	legend, detail, end := "", []string{}, ""
	switch m.page {
	case "":
		position := ""
		if m.busy {
			legend = target(m.pendingName, m.pendingLabel, width)
		} else if row := m.selectedRow(); row != nil {
			legend = target(row.session.Name, row.label, width)
			detail = m.rowDetailLines(*row, width)
			if m.tableLength() > m.height-3-len(notices)-len(detail)-len(keys) {
				position = fmt.Sprintf("%d of %d", m.cursor+1, len(m.rows))
			}
		}
		// The filter hides rows, so it reads plainly; the scrolled position recedes.
		if m.needsInputOnly {
			end = " needs input"
		}
		if position != "" {
			if end != "" {
				position = "· " + position
			}
			end += faint.Styled(" " + position)
		}
	case "confirm":
		legend = target(m.pendingName, m.pendingLabel, width)
	case "details", "group-edit", "name-edit":
		legend = target(m.pageRow.session.Name, m.pageRow.label, width)
	case "create":
		legend = "new session on " + ansi.Truncate(singleLine(m.form[0]), width/2, "…")
	}
	lines := append(notices, rule(legend, end, width))
	lines = append(lines, detail...)
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
	for _, peer := range m.scopedPeers() {
		if !peer.OK && peer.Error != nil {
			suffix := " unavailable (" + singleLine(peer.Error.Code) + "); showing its last observation"
			lines = append(lines, ansi.Truncate(ansi.Truncate(singleLine(peer.Label), max(1, width-ansi.StringWidth(suffix)), "…")+suffix, width, "…"))
		}
	}
	if limit := max(1, m.height-16); len(lines) > limit {
		lines = append(lines[:limit-1], "…")
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
	case "machine-picker":
		return [][]hint{{{"↑↓", "choose"}, {"enter", "select"}, {"escape", "cancel"}}}
	case "group-edit", "name-edit":
		if m.metadata.checking {
			return [][]hint{{{"ctrl-r", "refresh"}, {"escape", "info"}}}
		}
		keys := []hint{{"enter", "save"}, {"escape", "cancel"}}
		if m.page == "group-edit" {
			keys = append([]hint{{"←→", "suggestion"}, {"ctrl-u", "unassigned"}}, keys...)
		} else {
			keys[0].label = "save name"
			if row := m.rowForReference(m.pageRow.session.Ref); row != nil && row.available && row.session.NameMode == "manual" {
				keys = append(keys, hint{"ctrl-a", "use automatic title"})
			}
		}
		return [][]hint{keys}
	case "create":
		enter := hint{"enter", "next"}
		if m.field == 4 {
			enter.label = "create"
		}
		hints := []hint{{"tab", "next"}, {"shift-tab", "back"}, {"←→", "choose"}, enter, {"escape", "cancel"}}
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
	}
	session := []hint{}
	if row := m.selectedRow(); row != nil && row.available {
		session = append(session, hint{"enter", "attach"}, hint{"space", "info"})
		session = append(session, hint{"s", "send interrupt"}, hint{"x", "interrupt and close terminal"})
	} else if row != nil {
		session = append(session, hint{"space", "info"})
	}
	target := m.machine
	if target == "" {
		target = m.client.DefaultMachine().Label
	}
	// f toggles the filter; the rule's end shows when it is on. The strip's
	// chevrons and --help teach ←→, so the global keys keep one 80-column line
	// for host labels up to 9 cells.
	return [][]hint{session, {{"a", "agents"}, {"f", "needs input"}, {"m", "machine"}, {"n", "terminal on " + singleLine(target)}, {"N", "options"}, {"q", "quit"}}}
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

// rule is the one boundary between content and controls; its legend names the
// target, and its styled end carries the table's filter and scrolled position.
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
	return bold.Styled(ansi.Truncate(singleLine(name), half, "…")) + faint.Styled(" on ") + ansi.Truncate(singleLine(machine), half, "…")
}

func (m *model) rowDetailLines(row listedRow, width int) []string {
	current := m.current(&row)
	facts := m.statusView(row).Detail
	if row.session.Agent == nil && row.session.ActiveCommand != "" {
		facts += ": " + row.session.ActiveCommand
	}
	// a row without remote actions names why: its host failed a read, or it
	// is being checked by a pending scoped read or a re-read after metadata.
	if !row.available {
		host := "checking"
		for _, peer := range m.peers {
			if peer.Machine == row.machine && peer.Error != nil {
				host = "unavailable"
				break
			}
		}
		facts = host + "; " + facts
	}
	where := current.CWD
	if where == "" {
		where = "directory unavailable"
	}
	switch current.Kind {
	case "remote":
		where = "running on " + current.Label + ": " + where
	case "remoteUnknown":
		where = "remote context unknown"
	}
	status := singleLine(fmt.Sprintf("%s  ·  %d attached", facts, row.session.AttachedClients))
	directory := ansi.Truncate(singleLine(where), width, "…")
	if row.available && row.session.Connection == nil {
		action := bold.Styled("shift+t") + " new shell here"
		directory = tail(singleLine(where), width-ansi.StringWidth(action)-2)
		directory += strings.Repeat(" ", width-ansi.StringWidth(directory)-ansi.StringWidth(action)) + action
	}
	return []string{faint.Styled(ansi.Truncate(status, width, "…")), directory}
}

func (m *model) bodyLines(height int) []string {
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
		lines = append(lines, wrapped("selected pane "+ref.PaneID+" · terminal host "+singleLine(m.pendingLabel), width)...)
		return window(append(lines, wrapped(effect, width)...), 0, 0, height)
	case "machine-picker":
		lines := []string{bold.Styled("machine"), ""}
		for index, option := range m.pickerOptions() {
			text := ansi.Truncate(singleLine(option), width-2, "…")
			if index == m.picker {
				text = here.Styled("▌") + " " + bold.Styled(text)
			} else {
				text = "  " + text
			}
			lines = append(lines, text)
		}
		return window(lines, m.picker+2, m.picker+3, height)
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
		lines = append(lines, field(label, draft, !editor.checking && !m.busy, false, axis, width)...)
		focusEnd := len(lines)
		status := "enter saves"
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
		if m.page == "group-edit" {
			lines = append(lines, m.suggestions(width)...)
		} else {
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
			if index == 3 && strings.TrimSpace(value) == "" {
				value = ""
				if m.field != 3 {
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
				if strings.TrimSpace(value) == "" {
					lines[len(lines)-1] += faint.Styled(" home (~); type to search")
				}
				switch {
				case m.searching:
					lines = append(lines, field("", "searching…", false, false, 9, width)...)
				case len(m.searchDirectories) > 0:
					preview := fmt.Sprintf("‹ %d/%d › %s", m.searchCursor+1, len(m.searchDirectories), m.searchDirectories[m.searchCursor])
					lines = append(lines, field("use", preview, false, false, 9, width)...)
				}
			}
			if index == m.field {
				focusEnd = len(lines)
			}
		}
		lines = append(lines, "", "name: leave blank to follow the terminal title.")
		if m.field == 3 {
			lines = append(lines, "type search words or a path (/… or ~/…); tab uses the match.")
		}
		if !m.createAvailable() {
			lines = append(lines, "host unavailable; create disabled")
		}
		if _, err := group.ParseDraft(m.form[4]); err != nil {
			lines = append(lines, wrapped(group.ErrInvalid.Error(), width)...)
		}
		return window(append(lines, m.suggestions(width)...), focusStart, focusEnd, height)
	case "details":
		title := bold.Styled("info")
		// The title stays pinned; only the captured snapshot scrolls.
		body := m.detailLines()
		offset := min(m.offset, max(0, len(body)-(height-2)))
		return append([]string{title, ""}, body[offset:min(len(body), offset+max(0, height-2))]...)
	}
	if len(m.rows) == 0 {
		partial := false
		for _, peer := range m.scopedPeers() {
			partial = partial || !peer.OK
		}
		switch {
		case !m.scopeReady:
			return []string{"checking inventory"}
		case m.needsInputOnly:
			return []string{"no sessions currently need input in this view"}
		case partial:
			return []string{"no matching sessions in available inventory"}
		case m.agentsView:
			return []string{"no agents in this view"}
		}
		return []string{"no sessions in this view"}
	}
	lines := m.tableLines(width)
	top := min(m.top, max(0, len(lines)-height))
	return lines[top:min(len(lines), top+height)]
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
		return []string{gutter + name + "  " + faint.Styled("‹ ") + bold.Styled(ansi.Truncate(singleLine(value), valueWidth, "…")) + faint.Styled(" ›")}
	}
	if value == "" && !focused {
		return []string{gutter + name + "  " + faint.Styled("—")}
	}
	parts := strings.Split(ansi.Hardwrap(singleLine(value), valueWidth, true), "\n")
	if focused {
		parts[len(parts)-1] += wordmark.Styled(" ")
	}
	lines := []string{gutter + name + "  " + parts[0]}
	for _, part := range parts[1:] {
		lines = append(lines, indent+part)
	}
	return lines
}

func (m *model) suggestions(width int) []string {
	labels := []string{}
	for _, label := range fleetclient.ObservedGroups(m.scopedPeers()) {
		labels = append(labels, fleetclient.GroupHeading(label))
	}
	return wrapped("observed groups: "+strings.Join(labels, " · "), width)
}

func (m *model) detailLines() []string {
	width := m.width - 2
	lines := []string{}
	row := m.rowForReference(m.pageRow.session.Ref)
	for _, fact := range m.details(&m.pageRow) {
		label := fact[0]
		if row != nil && row.available && (m.metadata == nil || !m.metadata.checking) {
			if label == "name" {
				label += " [r]"
			} else if label == "group" {
				label += " [g]"
			}
		}
		lines = append(lines, field(label, fact[1], false, false, 16, width)...)
	}
	return lines
}

func (m *model) pageCapacity() int {
	return max(1, m.height-4-len(m.footerLines()))
}

func (m *model) headings() bool {
	return !m.agentsView && m.groupFilter.Kind() == group.FilterAll
}

// opensGroup reports whether a group heading precedes this row.
func (m *model) opensGroup(index int) bool {
	return m.headings() && (index == 0 || m.rows[index].session.Group != m.rows[index-1].session.Group)
}

func (m *model) tableLength() int {
	length := len(m.rows)
	for index := range m.rows {
		if m.opensGroup(index) {
			length++
		}
	}
	return length
}

func (m *model) fitViewports() {
	if m.width < 80 || m.height < 24 || m.page != "" || m.cursor < 0 {
		return
	}
	height := m.height - 2 - len(m.footerLines())
	// anchor is the cursor's group heading when its row opens a group.
	line, anchor := 0, 0
	for index := range m.rows {
		anchor = line
		if m.opensGroup(index) {
			line++
		}
		if index == m.cursor {
			break
		}
		line++
	}
	m.top = min(m.top, max(0, m.tableLength()-height), anchor)
	m.top = max(m.top, line-height+1, 0)
}

// tableLines lays out one line per row, with a quiet heading wherever the
// all view enters another group. the agents view names each row's group
// in a column instead; machine appears only when all machines are in scope.
func (m *model) tableLines(width int) []string {
	name, status, agent, label, machine := 4, 0, 0, 0, 0
	for _, row := range m.rows {
		name = max(name, ansi.StringWidth(singleLine(row.session.Name)))
		printed, _ := m.tableStatus(row)
		status = max(status, ansi.StringWidth(printed))
		agent = max(agent, ansi.StringWidth(m.agentText(row)))
		if m.agentsView {
			label = max(label, ansi.StringWidth(singleLine(row.session.Group.String())))
		}
		if m.machine == "" {
			machine = max(machine, ansi.StringWidth(singleLine(row.label)))
		}
	}
	name, agent, label, machine = min(name, 24), min(agent, 20), min(label, 16), min(machine, 12)
	span := func(width int) int {
		if width == 0 {
			return 0
		}
		return width + 2
	}
	for 2+name+span(status)+span(agent)+span(label)+span(machine) > width && max(name, agent, label, machine) > 8 {
		switch max(name, agent, label, machine) {
		case name:
			name--
		case agent:
			agent--
		case label:
			label--
		default:
			machine--
		}
	}
	for 2+name+span(status)+span(agent)+span(label)+span(machine) > width && (agent > 0 || label > 0) {
		if agent > 0 {
			agent = 0
		} else {
			label = 0
		}
	}
	directory := width - 2 - name - span(status) - span(agent) - span(label) - span(machine) - 2
	lines := []string{}
	for index, row := range m.rows {
		if m.opensGroup(index) {
			lines = append(lines, faint.Styled(ansi.Truncate(singleLine(fleetclient.GroupHeading(row.session.Group)), width, "…")))
		}
		// the projected tone keys colour; unavailable rows recede.
		gutter, nameStyle := "  ", plain
		printed, statusStyle := m.tableStatus(row)
		if !row.available {
			nameStyle = faint
		}
		if index == m.cursor {
			gutter, nameStyle = here.Styled("▌")+" ", bold
		}
		facts := []string{}
		for _, column := range []struct {
			text  string
			width int
		}{{m.agentText(row), agent}, {singleLine(row.session.Group.String()), label}, {singleLine(row.label), machine}} {
			if column.width > 0 {
				facts = append(facts, cell(column.text, column.width))
			}
		}
		if directory >= 8 {
			facts = append(facts, m.whereText(row, directory))
		}
		line := gutter + nameStyle.Styled(cell(row.session.Name, name)) + "  " + statusStyle.Styled(cell(printed, status))
		if len(facts) > 0 {
			line += "  " + faint.Styled(strings.Join(facts, "  "))
		}
		lines = append(lines, line)
	}
	return lines
}

// statusView projects a row's terminal status. A row without remote actions
// is stale; readiness also requires a working notification store.
func (m *model) statusView(row listedRow) fleetclient.StatusView {
	return fleetclient.ProjectStatus(row.session, row.available, !m.notificationFailed && m.notificationSnapshot.Ready(row.session))
}

// tableStatus is a row's status cell: the projected label in its tone, so a
// stale row reads its last observation. the selected row's facts name its
// host unavailable or checking; an errored host also keeps its notice.
func (m *model) tableStatus(row listedRow) (string, ansi.Style) {
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

// agentText is the configured profile label, per terminal continuity's identity copy.
func (m *model) agentText(row listedRow) string {
	agent := m.current(&row).Agent
	switch {
	case agent == nil:
		return ""
	case agent.Label == "":
		return singleLine(agent.Provider + " · profile unknown")
	}
	return singleLine(agent.Label)
}

// whereText is the directory where the work runs; remote work reads host:path.
func (m *model) whereText(row listedRow, width int) string {
	current := m.current(&row)
	switch current.Kind {
	case "remote":
		host := ansi.Truncate(singleLine(current.Label), width/3, "…") + ":"
		return host + tail(singleLine(current.CWD), width-ansi.StringWidth(host))
	case "remoteUnknown":
		return ansi.Truncate("remote context unknown", width, "…")
	}
	return tail(singleLine(current.CWD), width)
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
	return ansi.Truncate(singleLine(name), (width-4)/2, "…") + " on " + ansi.Truncate(singleLine(machine), (width-4)/2, "…")
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
		lines[index] = ansi.Wrap(singleLine(line), max(1, width), "")
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

func cell(text string, width int) string {
	text = ansi.Truncate(singleLine(text), width, "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}
