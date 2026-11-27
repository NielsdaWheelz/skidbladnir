package sessionui

import (
	"image/color"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/charmbracelet/x/ansi"
)

type role uint8

const (
	inkRole role = iota
	quietRole
	mutedRole
	goldRole
	emberRole
	mossRole
	frostRole
	bronzeRole
)

type pointerKind uint8

const (
	pointerAttach pointerKind = iota + 1
	pointerDisclose
	pointerChoose
)

type pointerTarget struct {
	kind   pointerKind
	ref    fleetclient.Reference
	parent api.SessionRef
	view   selectedView
}

type pointerHit struct {
	target  pointerTarget
	preview *branchDialog
}

type pointerControl struct {
	target pointerTarget
	cells  int
}

// Layout uses plain sanitized text; finished fragments receive design-language
// accents. Labels remain complete without color.
type span struct {
	background   color.Color
	pointer      *pointerControl
	preview      *branchDialog
	text         string
	role         role
	under        ansi.Underline
	underRole    role
	bold, italic bool
}

type line []span

func (l line) width() int {
	width := 0
	for _, part := range l {
		width += ansi.StringWidth(part.text)
	}
	return width
}

// The first styled fragment carries the range for the whole control.
func (l line) mark(target pointerTarget) {
	if cells := l.width(); cells > 0 {
		l[0].pointer = &pointerControl{target: target, cells: cells}
	}
}

func (m *model) renderLine(l line) string {
	var rendered strings.Builder
	for _, part := range l {
		var style ansi.Style
		if pigment := m.ground.pigments[part.role]; pigment != nil {
			style = style.ForegroundColor(pigment)
		}
		if m.ground.raise == nil {
			switch part.role {
			case quietRole:
				style = style.Faint()
			case bronzeRole:
				style = style.Reverse(true)
			}
		}
		if part.background != nil {
			style = style.BackgroundColor(part.background)
		}
		if part.bold {
			style = style.Bold()
		}
		if part.italic {
			style = style.Italic(true)
		}
		if part.under != ansi.UnderlineNone {
			style = style.UnderlineStyle(part.under)
			if pigment := m.ground.pigments[part.underRole]; pigment != nil {
				style = style.UnderlineColor(pigment)
			}
		}
		rendered.WriteString(style.Styled(part.text))
	}
	return rendered.String()
}

func signSpan(sign api.StatusSign) span {
	switch sign {
	case api.SignRequest:
		return span{text: "?", role: emberRole, bold: true}
	case api.SignError:
		return span{text: "!", role: emberRole, bold: true}
	case api.SignReady:
		return span{text: "●", role: mossRole}
	case api.SignInterrupted:
		return span{text: "■", role: mutedRole}
	case api.SignMenu:
		return span{text: "≡", role: mutedRole}
	case api.SignWorking:
		return span{text: "▸", role: frostRole}
	case api.SignIdle:
		return span{text: "·", role: mutedRole}
	case api.SignUnknown:
		return span{text: "—", role: mutedRole}
	case api.SignTerminal:
		return span{text: "$", role: mutedRole}
	}
	panic("invalid status sign")
}

type paintedBrowser struct {
	lines           []line
	width, rowWidth int
}

func (p paintedBrowser) at(x, y int) pointerHit {
	if x < 0 || x >= p.width || y < 0 || y >= len(p.lines) {
		return pointerHit{}
	}
	left := 0
	for _, part := range p.lines[y] {
		if part.preview != nil && x >= left && x < p.rowWidth {
			return pointerHit{preview: part.preview}
		}
		if c := part.pointer; c != nil && x >= left && x < left+c.cells {
			return pointerHit{target: c.target}
		}
		left += ansi.StringWidth(part.text)
	}
	return pointerHit{}
}

func (m *model) currentPointer(x, y int) pointerHit {
	return m.paint().at(x, y)
}

func (m *model) paint() paintedBrowser {
	f := m.frame()
	margin := func(l line) line {
		if m.width >= 4 {
			return append(line{{text: " "}}, l...)
		}
		return l
	}
	lines := make([]line, 0, m.height)
	if f.strip {
		lines = append(lines, margin(m.header(f.width)))
	}
	if f.blank {
		lines = append(lines, nil)
	}
	body := m.tableLines(f)
	if len(m.rows) == 0 && len(body) > 0 {
		body[0] = margin(line{{text: ansi.Truncate(m.emptyTable(), f.width, "…")}})
	}
	copy(body[f.foldAt:], f.fold)
	lines = append(lines, body...)
	for _, item := range f.notices {
		lines = append(lines, margin(item))
	}
	if f.caption {
		lines = append(lines, margin(f.rule))
	}
	for _, item := range f.band {
		lines = append(lines, margin(item))
	}
	if f.hintLine {
		lines = append(lines, margin(f.hints))
	}
	return paintedBrowser{lines: lines, width: m.width, rowWidth: f.rowWidth}
}

func (m *model) View() tea.View {
	painted := m.paint()
	lines := make([]string, len(painted.lines))
	for y, l := range painted.lines {
		lines[y] = m.renderLine(l)
	}
	content := strings.Join(lines, "\n")
	if m.ground.noColor {
		content = ansi.Strip(content)
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeCellMotion
	// Bubble Tea calls the last flushed view's handler before Update.
	view.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		mouse := msg.Mouse()
		m.paintedPointer = painted.at(mouse.X, mouse.Y)
		return nil
	}
	return view
}

func (m *model) header(room int) line {
	tabs, current := []tab{}, 0
	for index, view := range m.viewOptions() {
		t := tab{view: view}
		switch view.kind {
		case viewNeedsInput:
			t.fixed, t.sign = "needs input ", span{text: " "}
			if queue := m.queue(); len(queue) > 0 {
				t.sign = signSpan(m.statusView(queue[0]).Sign)
			}
		case viewAll:
			t.fixed = "all"
		case viewGroup:
			if view.label.IsUnassigned() {
				t.fixed = "unassigned"
			} else {
				t.label = attention.SingleLine(view.label.String())
			}
		default:
			panic("invalid selected view")
		}
		if view == m.view {
			current = index
		}
		tabs = append(tabs, t)
	}
	mark := line{{text: "▜", role: bronzeRole}, {text: "skid"}, {text: "▛", role: bronzeRole}, {text: " "}}
	if views := strip(tabs, current, room-mark.width(), m.ground.raise); views != nil {
		return append(mark, views...)
	}
	if views := strip(tabs, current, room, m.ground.raise); views != nil {
		return views
	}
	tabs[0].fixed = ""
	if views := strip(tabs, current, room, m.ground.raise); views != nil {
		return views
	}

	left, right := span{text: " "}, span{text: " "}
	if current == 0 {
		left = span{text: "‹", role: goldRole, background: m.ground.raise}
		right = span{text: "›", role: goldRole, background: m.ground.raise}
		tabs[0].sign.background = m.ground.raise
	}
	views := line{left, tabs[0].sign, right}
	needs := pointerTarget{kind: pointerChoose, view: tabs[0].view}
	if current == 0 {
		views.mark(needs)
	} else if tabs[0].sign.text != " " {
		views[1:2].mark(needs)
	}
	label := tabs[current].fixed + tabs[current].label
	if keep := room - views.width() - 2; current != 0 && keep >= 3 {
		active := line{
			{text: "‹", role: goldRole, background: m.ground.raise},
			{text: ansi.Truncate(label, keep, "…"), bold: true, background: m.ground.raise},
			{text: "›", role: goldRole, background: m.ground.raise},
		}
		active.mark(pointerTarget{kind: pointerChoose, view: tabs[current].view})
		views = append(views, active...)
	}
	return views
}

// tab is one view in the strip: fixed is skid's own words and never truncates;
// label is an operator's group label and may.
type tab struct {
	view         selectedView
	fixed, label string
	sign         span
}

// strip tries both fixed tabs and the current group, with other groups cut
// together or hidden behind counted markers. Nil means even these do not fit.
func strip(tabs []tab, current, room int, raise color.Color) line {
	cells := func(t tab, limit int) int {
		if t.label != "" {
			return min(ansi.StringWidth(t.label), limit)
		}
		return ansi.StringWidth(t.fixed) + ansi.StringWidth(t.sign.text)
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
	for !fits(limit, first, last) && (first < last || current < 2 && first <= last) {
		// hide the group farthest from the current view; ties hide the left
		// one, and with needs input or all current, groups hide from the right.
		if current-first >= last-current && first != current {
			first++
		} else {
			last--
		}
	}
	const currentLimit = 24
	if width(limit, currentLimit, first, last) > room {
		return nil
	}
	var result line
	for index, t := range tabs {
		if index >= 2 && (index < first || index > last) {
			continue
		}
		if index == first && first > 2 {
			result = append(result, span{text: " ←" + strconv.Itoa(first-2) + " ", role: quietRole})
		}
		size := limit
		if index == current {
			size = currentLimit
		}
		text := t.fixed
		if t.label != "" {
			text = ansi.Truncate(t.label, size, "…")
		}
		left, right := span{text: " "}, span{text: " "}
		name := span{text: text}
		if index == current {
			left = span{text: "‹", role: goldRole, background: raise}
			right = span{text: "›", role: goldRole, background: raise}
			name.bold, name.background = true, raise
			t.sign.background = raise
		}
		start := len(result)
		result = append(result, left, name)
		if t.sign.text != "" {
			result = append(result, t.sign)
		}
		result = append(result, right)
		choice := pointerTarget{kind: pointerChoose, view: t.view}
		if index == current {
			result[start:].mark(choice)
		} else if text != "" || strings.TrimSpace(t.sign.text) != "" {
			result[start+1 : len(result)-1].mark(choice)
		}
	}
	if last < len(tabs)-1 {
		result = append(result, span{text: " " + strconv.Itoa(len(tabs)-1-last) + "→ ", role: quietRole})
	}
	return result
}

func (m *model) noticeLines(width int) []line {
	// Outcomes precede host notices: unavailable labels cannot conceal uncertainty.
	var lines []line
	if m.notice != "" {
		role := inkRole
		if m.noticeFailure {
			role = emberRole
		}
		for _, text := range wrapped(m.notice, width) {
			lines = append(lines, line{{text: text, role: role}})
		}
	}
	if m.notificationFailed {
		lines = append(lines, line{{text: "notifications unavailable", role: quietRole}})
	}
	// A peer without an error is unobserved or being checked, not unavailable.
	for _, peer := range m.peers {
		if !peer.OK && peer.Error != nil {
			suffix := " unavailable (" + attention.SingleLine(peer.Error.Code) + ")"
			if len(peer.Sessions) > 0 {
				suffix += "; its listing is stale"
			}
			text := ansi.Truncate(attention.SingleLine(peer.Label), 16, "…") + suffix
			lines = append(lines, line{{text: text}})
		} else if checking(peer) && len(peer.Sessions) == 0 {
			suffix := ": checking inventory"
			host := ansi.Truncate(attention.SingleLine(peer.Label), 16, "…")
			lines = append(lines, line{{text: host + suffix, role: quietRole}})
		}
		peer.OK = peer.Error == nil
		text, failure := fleetview.RecoveryNotice(peer)
		if text != "" {
			role := mutedRole
			if failure {
				role = emberRole
			}
			for _, part := range wrapped(text, width) {
				lines = append(lines, line{{text: part, role: role}})
			}
		}
	}
	// Trim after wrapping: a single wide grapheme can exceed a one-cell line.
	for i := range lines {
		lines[i][0].text = ansi.Truncate(lines[i][0].text, width, "…")
	}
	return lines
}

func rule(caption, quota line, width int) line {
	var result line
	room := width - caption.width() - quota.width()
	edge := 1
	if room >= 10 {
		edge = 2
	}
	end := strings.Repeat("─", edge)
	if len(caption) > 0 {
		result = append(result, span{text: end + " ", role: quietRole})
		result = append(result, caption...)
		result = append(result, span{text: " "})
		room -= edge + 2
	}
	if len(quota) > 0 {
		room -= edge + 2
	}
	result = append(result, span{text: strings.Repeat("─", max(0, room)), role: quietRole})
	if len(quota) > 0 {
		result = append(result, span{text: " "})
		result = append(result, quota...)
		result = append(result, span{text: " " + end, role: quietRole})
	}
	return result
}

func target(name, machine string, width int) string {
	name, machine = attention.SingleLine(name), attention.SingleLine(machine)
	if ansi.StringWidth(name+" on "+machine) <= width {
		return name + " on " + machine
	}
	machine = ansi.Truncate(machine, (width-4)/2, "…")
	return ansi.Truncate(name, width-4-ansi.StringWidth(machine), "…") + " on " + machine
}

func (m *model) emptyTable() string {
	for _, lane := range m.inventoryLanes {
		if !lane.settled {
			return "checking inventory"
		}
	}
	if m.view.kind == viewNeedsInput {
		return "no sessions currently need input in this view"
	}
	total, partial := 0, false
	for _, peer := range m.peers {
		total += len(peer.Sessions)
		partial = partial || !peer.OK
	}
	if total == 0 && !partial {
		return "no sessions"
	}
	if partial {
		return "no matching sessions in available inventory"
	}
	return "no sessions in this view"
}

func (m *model) statusView(row fleetclient.Row) fleetview.StatusView {
	ready := row.Session.Parent == nil && row.Available && m.notificationQualified && m.notificationReady(row.Session) || m.heldReady(row)
	return fleetview.ProjectStatus(row.Session, m.rowAttachable(row), ready)
}

func (m *model) rowChecking(row fleetclient.Row) bool {
	return m.hostChecking(row) || m.heldReady(row)
}

// hostChecking reports a row without remote actions whose host has not failed a
// read: its host is unobserved or being re-read after a metadata change.
func (m *model) hostChecking(row fleetclient.Row) bool {
	index := m.peerIndex(row.Machine)
	return !row.Available && index >= 0 && checking(m.peers[index])
}

func (m *model) rowAvailable(row fleetclient.Row) bool {
	return row.Available && !m.heldReady(row)
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
