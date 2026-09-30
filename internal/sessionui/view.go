package sessionui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/charmbracelet/x/ansi"
)

// The browser speaks only the sixteen-colour protocol, so the operator's
// terminal theme owns the actual colours; the design language maps these slots
// to its accents. Layout works on plain sanitized text, then styles wrap the
// finished fragments. Each fragment resets itself, so styles never nest.
var (
	plain    = ansi.Style{}
	bold     = ansi.Style{}.Bold()
	faint    = ansi.Style{}.Faint()
	wordmark = ansi.Style{}.Reverse(true)
	here     = ansi.Style{}.ForegroundColor(ansi.BrightYellow)
	alive    = ansi.Style{}.ForegroundColor(ansi.BrightGreen)
	alarm    = ansi.Style{}.ForegroundColor(ansi.BrightRed)
	danger   = ansi.Style{}.Bold().ForegroundColor(ansi.BrightRed)
)

type hint struct{ key, label string }

func (m *model) View() tea.View {
	content := ansi.Truncate("resize to at least 80 × 24; escape cancels or quits", max(1, m.width), "…")
	if m.width >= 80 && m.height >= 24 {
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
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m *model) header() string {
	scope := "agents"
	if !m.agentsView {
		scope = fleetclient.GroupFilterHeading(m.groupFilter)
	}
	machine := "all machines"
	if m.machine != "" {
		machine = "machine: " + m.machine
	}
	return wordmark.Styled(" skid ") + "  " + ansi.Truncate(singleLine(scope), 40, "…") + faint.Styled(" · ") + ansi.Truncate(singleLine(machine), 24, "…")
}

// footerLines holds notices, the rule naming the session the keys act on, that
// session's facts, and the keys.
func (m *model) footerLines() []string {
	width := m.width - 2
	notices := m.noticeLines(width)
	keys := keyLines(width, m.hints()...)
	legend, detail, position := "", []string{}, ""
	switch m.page {
	case "":
		if m.busy {
			legend = target(m.pendingName, m.pendingLabel, width)
		} else if row := m.selectedRow(); row != nil {
			legend = target(row.session.Name, row.label, width)
			detail = []string{faint.Styled(ansi.Truncate(m.rowDetail(*row), width, "…"))}
			if m.tableLength() > m.height-3-len(notices)-len(detail)-len(keys) {
				position = fmt.Sprintf("%d of %d", m.cursor+1, len(m.rows))
			}
		}
	case "confirm", "group-edit":
		legend = target(m.pendingName, m.pendingLabel, width)
	case "details":
		legend = target(m.pageName, m.pageMachine, width)
	case "create", "search":
		legend = "new session on " + ansi.Truncate(singleLine(m.form[0]), width/2, "…")
	}
	lines := append(notices, rule(legend, position, width))
	lines = append(lines, detail...)
	return append(lines, keys...)
}

func (m *model) noticeLines(width int) []string {
	lines := []string{}
	// Outcomes precede host notices: unavailable labels cannot conceal uncertainty.
	if m.notice != "" {
		style := plain
		if m.noticeFailure {
			style = alarm
		}
		for _, line := range wrapped(m.notice, width) {
			lines = append(lines, style.Styled(line))
		}
	}
	if m.unreadFailed {
		lines = append(lines, faint.Styled("unread unavailable"))
	}
	unavailable := false
	for _, row := range m.rows {
		ref, _ := fleetclient.DecodeReference(row.session.Ref)
		if conversation, found := m.unreadSnapshot.Conversation(ref, row.session.ActivePaneID); found && m.repliesUnavailable[fleetclient.ReplyKey(row.machine, conversation)] {
			unavailable = true
			break
		}
	}
	if unavailable {
		lines = append(lines, faint.Styled("replies unavailable"))
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
		effect := "close terminal only"
		if m.pending.Operation == "stop" {
			effect = "stop tracked conversation"
		} else if !m.pending.TerminalOnly {
			effect = "stop tracked conversation and close terminal"
		}
		return [][]hint{{{"enter", effect}, {"escape", "cancel"}}}
	case "machine-picker":
		return [][]hint{{{"↑↓", "choose"}, {"enter", "select"}, {"escape", "cancel"}}}
	case "group-edit":
		if m.groupChecking {
			return [][]hint{{{"ctrl-r", "refresh"}, {"escape", "close"}}}
		}
		return [][]hint{{{"←→", "suggestion"}, {"ctrl-u", "unassigned"}, {"enter", "save"}, {"escape", "cancel"}}}
	case "create":
		enter := hint{"enter", "next"}
		if m.field == 4 {
			enter.label = "create"
		}
		hints := []hint{{"tab", "next"}, {"shift-tab", "back"}, {"←→", "choose"}, enter, {"ctrl-u", "unassigned"}, {"escape", "cancel"}}
		if m.field == 3 {
			hints = append(hints, hint{"z words", "search visited directories"})
		}
		return [][]hint{hints}
	case "search":
		return [][]hint{{{"↑↓", "choose"}, {"enter", "use directory"}, {"escape", "back"}}}
	case "details":
		return [][]hint{{{"↑↓", "scroll"}, {"pgup pgdn", "page"}, {"escape", "back"}}}
	}
	session := []hint{}
	if row := m.selectedRow(); row != nil && row.available {
		session = append(session, hint{"enter", "attach"}, hint{"space", "info"})
		if _, readable := m.replyReference(*row); readable {
			session = append(session, hint{"r", "view replies"})
		}
		session = append(session, hint{"e", "group"})
		if row.session.Connection == nil {
			session = append(session, hint{"T", "here"})
		}
		if row.session.Conversation != nil && row.session.Conversation.Methods.Stop == "native" {
			session = append(session, hint{"s", "stop tracked conversation"}, hint{"c", "stop tracked conversation and close terminal"})
		}
		session = append(session, hint{"x", "close terminal only"})
	} else if row != nil {
		session = append(session, hint{"space", "info"})
	}
	target := m.machine
	if target == "" {
		target = m.client.DefaultMachine().Label
	}
	return [][]hint{session, {{"a", "agents"}, {"←→", "group"}, {"m", "machine"}, {"n", "terminal on " + singleLine(target)}, {"N", "options"}, {"q", "quit"}}}
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
// target, and a scrolled table reports the cursor position at its end.
func rule(legend, position string, width int) string {
	if position != "" {
		position = faint.Styled(" " + position + " ──")
	}
	fill := width - ansi.StringWidth(position)
	if legend == "" {
		return faint.Styled(strings.Repeat("─", fill)) + position
	}
	legend = ansi.Truncate(legend, max(1, fill-5), "…")
	return faint.Styled("── ") + legend + " " + faint.Styled(strings.Repeat("─", max(0, fill-4-ansi.StringWidth(legend)))) + position
}

func target(name, machine string, width int) string {
	half := max(1, (width-10)/2)
	return bold.Styled(ansi.Truncate(singleLine(name), half, "…")) + faint.Styled(" on ") + ansi.Truncate(singleLine(machine), half, "…")
}

func (m *model) rowDetail(row listedRow) string {
	current := m.current(&row)
	facts := "terminal"
	if row.session.ActiveCommand != "" {
		facts += ": " + row.session.ActiveCommand
	}
	if current.Agent != nil || row.session.Conversation != nil {
		facts = fleetclient.SessionStatus(row.session)
	}
	if row.session.Conversation != nil {
		id := row.session.Conversation.Binding.Conversation.ConversationID
		facts = "tracking " + fleetclient.ShortConversationID(id) + " · " + facts
	}
	if reply := m.replyText(row); reply != "" {
		facts += " · " + reply
	}
	if !row.available {
		facts = m.rowStatus(row) + "; last observed " + facts
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
	return singleLine(fmt.Sprintf("%s  ·  %d attached  ·  %s", facts, row.session.AttachedClients, where))
}

func (m *model) bodyLines(height int) []string {
	width := m.width - 2
	switch m.page {
	case "confirm":
		action, effect := "close terminal only", "close this session. work shared through another session may survive."
		if m.pending.Operation == "stop" {
			action, effect = "stop tracked conversation", "halt captured current work and retain the terminal. pending input may remain; saved history is retained."
		}
		if m.pending.Operation == "close" && !m.pending.TerminalOnly {
			action, effect = "stop tracked conversation and close terminal", "halt and terminal closure have separate outcomes. pending input may remain; saved history is retained."
		}
		lines := []string{}
		for _, line := range wrapped(action+" "+capturedHeading(m.pendingName, m.pendingLabel, width-len(action)-2)+"?", width) {
			lines = append(lines, danger.Styled(line))
		}
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
	case "group-edit":
		lines := []string{bold.Styled("group"), ""}
		if m.groupFailure != nil {
			lines = append(lines, alarm.Styled(singleLine(m.groupFailure.Code)+" (unknown); not repeated"), "")
		}
		current := m.pendingRow()
		if current != nil {
			lines = append(lines, field("current", fleetclient.GroupHeading(current.session.Group), false, false, 7, width)...)
		}
		focusStart := len(lines)
		lines = append(lines, field("group", groupDraftDisplay(m.groupDraft), !m.groupChecking, false, 7, width)...)
		focusEnd := len(lines)
		label, err := group.ParseDraft(m.groupDraft)
		status := "enter saves"
		switch {
		case m.groupChecking:
			status = "checking inventory; escape returns"
		case err != nil:
			status = group.ErrInvalid.Error()
		case current == nil || !current.available:
			status = "session unavailable; save disabled"
		case current.session.Group == label:
			status = "unchanged; save disabled"
		}
		lines = append(lines, "")
		lines = append(lines, wrapped(status, width)...)
		lines = append(lines, "")
		return window(append(lines, m.suggestions(width)...), focusStart, focusEnd, height)
	case "create":
		lines := []string{bold.Styled("new session"), ""}
		focusStart, focusEnd := 0, 0
		for index, label := range []string{"machine", "launch", "name", "directory", "group"} {
			value := m.form[index]
			if index == 4 {
				value = groupDraftDisplay(value)
			}
			if index == m.field {
				focusStart = len(lines)
			}
			lines = append(lines, field(label, value, index == m.field, index < 2, 9, width)...)
			if index == m.field {
				focusEnd = len(lines)
			}
		}
		lines = append(lines, "", "leave blank to follow the terminal title.")
		if !m.createAvailable() {
			lines = append(lines, "host unavailable; create disabled")
		}
		if _, err := group.ParseDraft(m.form[4]); err != nil {
			lines = append(lines, wrapped(group.ErrInvalid.Error(), width)...)
		}
		return window(append(lines, m.suggestions(width)...), focusStart, focusEnd, height)
	case "search":
		lines := []string{bold.Styled("directory search on " + ansi.Truncate(singleLine(m.form[0]), width-24, "…")), ""}
		switch {
		case m.searching:
			lines = append(lines, "searching…")
		case len(m.searchDirectories) == 0:
			lines = append(lines, "no matching directories")
		}
		focusStart, focusEnd := 0, 0
		for index, directory := range m.searchDirectories {
			if index == m.searchCursor {
				focusStart = len(lines)
			}
			// Ranked paths show in full, wrapped under their marker.
			for part, text := range strings.Split(ansi.Hardwrap(singleLine(directory), width-2, true), "\n") {
				switch {
				case index == m.searchCursor && part == 0:
					text = here.Styled("▌") + " " + bold.Styled(text)
				case index == m.searchCursor:
					text = "  " + bold.Styled(text)
				default:
					text = "  " + text
				}
				lines = append(lines, text)
			}
			if index == m.searchCursor {
				focusEnd = len(lines)
			}
		}
		return window(lines, focusStart, focusEnd, height)
	case "details":
		title := bold.Styled(m.page)
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
	for _, fact := range m.facts {
		lines = append(lines, field(fact[0], fact[1], false, false, 16, width)...)
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
// all-groups view enters another group. the agents view names each row's group
// in a column instead; machine appears only when all machines are in scope.
func (m *model) tableLines(width int) []string {
	name, status, attention, agent, label, machine := 4, 0, 0, 0, 0, 0
	for _, row := range m.rows {
		name = max(name, ansi.StringWidth(singleLine(row.session.Name)))
		status = max(status, len(m.rowStatus(row)))
		attention = max(attention, ansi.StringWidth(m.replyText(row)))
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
	for 2+name+span(status)+span(attention)+span(agent)+span(label)+span(machine) > width && max(name, agent, label, machine) > 8 {
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
	for 2+name+span(status)+span(attention)+span(agent)+span(label)+span(machine) > width && (agent > 0 || label > 0) {
		if agent > 0 {
			agent = 0
		} else {
			label = 0
		}
	}
	directory := width - 2 - name - span(status) - span(attention) - span(agent) - span(label) - span(machine) - 2
	lines := []string{}
	for index, row := range m.rows {
		if m.opensGroup(index) {
			lines = append(lines, faint.Styled(ansi.Truncate(singleLine(fleetclient.GroupHeading(row.session.Group)), width, "…")))
		}
		gutter, nameStyle, statusStyle := "  ", plain, plain
		switch state := fleetclient.SessionStatus(row.session); {
		case !row.available:
			nameStyle, statusStyle = faint, faint
		case state == "working":
			statusStyle = alive
		case state == "waiting" || state == "failed":
			statusStyle = alarm
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
		line := gutter + nameStyle.Styled(cell(row.session.Name, name)) + "  " + statusStyle.Styled(cell(m.rowStatus(row), status))
		if attention > 0 {
			line += "  " + bold.Styled(cell(m.replyText(row), attention))
		}
		if len(facts) > 0 {
			line += "  " + faint.Styled(strings.Join(facts, "  "))
		}
		lines = append(lines, line)
	}
	return lines
}

// rowStatus names a row without remote actions by why: its host failed a read,
// or a scoped read is still checking it.
func (m *model) rowStatus(row listedRow) string {
	switch {
	case !row.available:
		for _, peer := range m.peers {
			if peer.Machine == row.machine && peer.Error != nil {
				return "unavailable"
			}
		}
		return "checking"
	}
	if row.session.Conversation != nil {
		id := row.session.Conversation.Binding.Conversation.ConversationID
		return "tracking " + fleetclient.ShortConversationID(id) + " · " + fleetclient.SessionStatus(row.session)
	}
	current := m.current(&row)
	switch {
	case current.Kind == "remoteUnknown":
		return "status unavailable"
	case current.Agent == nil && row.session.Conversation == nil:
		return "terminal"
	}
	return fleetclient.SessionStatus(row.session)
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
