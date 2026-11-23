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

// Layout uses plain sanitized text; finished fragments receive design-language
// accents. Labels remain complete without color.
var (
	plain    = ansi.Style{}
	faint    = ansi.Style{}.Faint()
	wordmark = ansi.Style{}.Reverse(true)
	here     = ansi.Style{}.ForegroundColor(ansi.BrightYellow)
	frost    = ansi.Style{}.ForegroundColor(color.RGBA{R: 0x78, G: 0xA9, B: 0xC6, A: 0xff})
	moss     = ansi.Style{}.ForegroundColor(color.RGBA{R: 0x76, G: 0xB0, B: 0x82, A: 0xff})
	muted    = ansi.Style{}.ForegroundColor(color.RGBA{R: 0xAA, G: 0xA6, B: 0x9D, A: 0xff})
	ember    = ansi.Style{}.ForegroundColor(ansi.BrightRed)
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

type span struct {
	background   color.Color
	text         string
	role         role
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

func (m *model) renderLine(l line) string {
	var rendered strings.Builder
	styles := [...]ansi.Style{plain, faint, muted, here, ember, moss, frost, wordmark}
	for _, part := range l {
		style := styles[part.role]
		if m.ground.raise != nil {
			style = plain
			if pigment := m.ground.pigments[part.role]; pigment != nil {
				style = style.ForegroundColor(pigment)
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
		rendered.WriteString(style.Styled(part.text))
	}
	return rendered.String()
}

func statusRole(tone api.StatusTone) role {
	switch tone {
	case api.ToneMuted:
		return mutedRole
	case api.ToneWorking:
		return frostRole
	case api.ToneAttention:
		return emberRole
	case api.ToneReady:
		return mossRole
	default:
		panic("invalid status tone")
	}
}

func (m *model) View() tea.View {
	var lines []string
	if m.tooSmall() {
		// Keep the outcome visible even below the table's size floor.
		lines = wrapped("80 × 24 minimum; ctrl-c quits", m.width)
		for _, item := range m.outcomeLines(m.width) {
			lines = append(lines, m.renderLine(item))
		}
		lines = lines[:min(len(lines), max(1, m.height))]
	} else {
		f := m.frame()
		lines = []string{" " + m.renderLine(m.header()), ""}
		body := m.tableLines(f)
		if len(m.rows) == 0 {
			body[0] = line{{text: m.emptyTable()}}
		}
		for index, item := range body {
			if index >= f.foldAt && index < f.foldAt+len(f.fold) {
				lines = append(lines, m.renderLine(f.fold[index-f.foldAt]))
			} else {
				lines = append(lines, " "+m.renderLine(item))
			}
		}
		for _, item := range f.notices {
			lines = append(lines, " "+m.renderLine(item))
		}
		lines = append(lines, " "+m.renderLine(f.rule))
		for _, item := range f.band {
			lines = append(lines, " "+m.renderLine(item))
		}
		lines = append(lines, " "+m.renderLine(f.hints))
	}
	content := strings.Join(lines, "\n")
	if m.ground.noColor {
		content = ansi.Strip(content)
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.ReportFocus = true
	return view
}

func (m *model) header() line {
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
			panic("invalid selected view")
		}
		if view == m.view {
			current = index
		}
	}
	mark := line{{text: "▜", role: bronzeRole}, {text: "skid"}, {text: "▛", role: bronzeRole}, {text: " "}}
	return append(mark, strip(tabs, current, m.width-9, m.ground.raise)...)
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
func strip(tabs []tab, current, room int, raise color.Color) line {
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
			size = whole
		}
		text := t.fixed
		if t.label != "" {
			text = ansi.Truncate(t.label, size, "…")
		}
		if index == current {
			result = append(result, span{text: "‹", role: goldRole, background: raise}, span{text: text, bold: true, background: raise}, span{text: "›", role: goldRole, background: raise})
		} else {
			result = append(result, span{text: " " + text + " "})
		}
	}
	if last < len(tabs)-1 {
		result = append(result, span{text: " " + strconv.Itoa(len(tabs)-1-last) + "→ ", role: quietRole})
	}
	return result
}

func (m *model) outcomeLines(width int) []line {
	var lines []line
	if m.notice == "" {
		return lines
	}
	role := inkRole
	if m.noticeFailure {
		role = emberRole
	}
	for _, text := range wrapped(m.notice, width) {
		lines = append(lines, line{{text: text, role: role}})
	}
	return lines
}

func (m *model) noticeLines(width int) []line {
	// Outcomes precede host notices: unavailable labels cannot conceal uncertainty.
	lines := m.outcomeLines(width)
	if m.notificationFailed {
		lines = append(lines, line{{text: "notifications unavailable", role: quietRole}})
	}
	// A peer without an error is unobserved or being checked, not unavailable.
	for _, peer := range m.peers {
		if !peer.OK && peer.Error != nil {
			suffix := " unavailable (" + attention.SingleLine(peer.Error.Code) + ")"
			if len(peer.Sessions) > 0 {
				suffix += "; showing its last observation"
			}
			text := ansi.Truncate(attention.SingleLine(peer.Label), max(1, width-ansi.StringWidth(suffix)), "…") + suffix
			lines = append(lines, line{{text: ansi.Truncate(text, width, "…")}})
		} else if checking(peer) && len(peer.Sessions) == 0 {
			suffix := ": checking inventory"
			host := ansi.Truncate(attention.SingleLine(peer.Label), max(1, width-ansi.StringWidth(suffix)), "…")
			lines = append(lines, line{{text: ansi.Truncate(host+suffix, width, "…"), role: quietRole}})
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
	return lines
}

func rule(caption string, quota line, width int) line {
	var result line
	room := width - ansi.StringWidth(caption) - quota.width()
	edge := 1
	if room >= 10 {
		edge = 2
	}
	end := strings.Repeat("─", edge)
	if caption != "" {
		result = append(result, span{text: end + " ", role: quietRole}, span{text: caption}, span{text: " "})
		room -= edge + 2
	}
	if len(quota) > 0 {
		room -= edge + 2
	}
	result = append(result, span{text: strings.Repeat("─", room), role: quietRole})
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
	return m.projectStatus(row, row.Available || m.hostChecking(row))
}

func (m *model) projectStatus(row fleetclient.Row, fresh bool) fleetview.StatusView {
	view := fleetview.ProjectStatus(row.Session, fresh, row.Available && m.notificationQualified && m.notificationReady(row.Session))
	if m.heldReady(row) {
		view.Label, view.Tone, view.Queue, view.QueueDetail = "ready", api.ToneReady, fleetview.QueueReady, ""
	}
	if m.view.kind == viewNeedsInput && view.QueueDetail != "" {
		view.Label += " · " + view.QueueDetail
		view.Detail += " · " + view.QueueDetail
	}
	return view
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
