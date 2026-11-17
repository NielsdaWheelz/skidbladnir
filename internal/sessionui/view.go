package sessionui

import (
	"fmt"
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
	bold     = ansi.Style{}.Bold()
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
		if m.page == "" {
			if len(f.rows) == 0 {
				lines = append(lines, " "+m.emptyTable())
			} else {
				for index, item := range m.tableLines(f) {
					if index >= f.foldAt && index < f.foldAt+len(f.fold) {
						lines = append(lines, m.renderLine(f.fold[index-f.foldAt]))
					} else {
						lines = append(lines, " "+m.renderLine(item))
					}
				}
			}
		} else {
			for _, text := range m.bodyLines(f.body) {
				lines = append(lines, " "+text)
			}
		}
		for len(lines) < f.body+2 {
			lines = append(lines, "")
		}
		for _, item := range f.notices {
			lines = append(lines, " "+m.renderLine(item))
		}
		lines = append(lines, " "+m.renderLine(f.rule))
		for _, item := range f.hints {
			lines = append(lines, " "+m.renderLine(item))
		}
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

func (m *model) pageCaption(width int) string {
	switch m.page {
	case "keys":
		if row := m.selectedRow(); row != nil {
			return target(row.Session.Name, row.Label, width-5)
		}
	case "quick-create":
		return "new terminal on " + ansi.Truncate(attention.SingleLine(m.pending.Machine), width-21, "…")
	case "create":
		return "new session on " + ansi.Truncate(attention.SingleLine(m.form[formMachine]), width-20, "…")
	}
	return ""
}

// outcomeLines is the notice, wrapped, in its tone.
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

type hint struct{ key, label string }

func (m *model) pageHints() [][]hint {
	switch m.page {
	case "quick-create":
		return [][]hint{{{"←→", "machine"}, {"enter", "open terminal"}, {"ctrl-r", "refresh"}, {"escape", "cancel"}}}
	case "create":
		enter := hint{"enter", "next"}
		next := hint{"tab", "next"}
		if m.field == formCreate {
			enter.label = "create"
		} else if m.field == formGroup {
			enter.label = "uses and continues"
			next.label = "uses and continues"
		}
		hints := []hint{next, {"shift-tab", "back"}, enter, {"escape", "cancel"}}
		if m.field == formMachine || m.field == formLaunch || m.field == formDirectory || m.field == formGroup {
			hints = append([]hint{{"←→", "choose"}}, hints...)
		}
		if m.field == formDirectory {
			hints = append(hints, hint{"ctrl-u", "home"})
		} else if m.field == formGroup {
			hints = append(hints, hint{"ctrl-u", "unassigned"})
		}
		return [][]hint{hints}
	case "usage":
		return [][]hint{{{"q escape", "back"}}}
	}
	return nil
}

// Legacy pages use their own keys until each becomes a fold.
func keyLines(width int, groups ...[]hint) []line {
	var lines []line
	for _, group := range groups {
		var result line
		used := 0
		for _, hint := range group {
			cells := ansi.StringWidth(hint.key) + 1 + ansi.StringWidth(hint.label)
			if used != 0 && used+2+cells > width {
				lines = append(lines, result)
				result, used = nil, 0
			}
			if used != 0 {
				result = append(result, span{text: "  "})
				used += 2
			}
			result = append(result, span{text: hint.key, bold: true}, span{text: " " + hint.label, role: quietRole})
			used += cells
		}
		if len(result) != 0 {
			lines = append(lines, result)
		}
	}
	return lines
}

func rule(caption, quota string, width int) line {
	var result line
	room := width - ansi.StringWidth(caption) - ansi.StringWidth(quota)
	edge := 1
	if room >= 10 {
		edge = 2
	}
	end := strings.Repeat("─", edge)
	if caption != "" {
		result = append(result, span{text: end + " ", role: quietRole}, span{text: caption}, span{text: " "})
		room -= edge + 2
	}
	if quota != "" {
		room -= edge + 2
	}
	result = append(result, span{text: strings.Repeat("─", room), role: quietRole})
	if quota != "" {
		result = append(result, span{text: " "}, span{text: quota}, span{text: " " + end, role: quietRole})
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

func (m *model) bodyLines(height int) []string {
	if height <= 0 {
		return nil
	}
	width := m.width - 2
	switch m.page {
	case "quick-create":
		lines := []string{bold.Styled("quick terminal"), ""}
		focusStart := len(lines)
		lines = append(lines, field("machine", m.pending.Machine, true, true, 7, width)...)
		focusEnd := len(lines)
		context := "home (~) · unassigned"
		if !m.pending.Group.IsUnassigned() {
			context = "home (~) · group: " + m.pending.Group.String()
		}
		lines = append(lines, wrapped(context, width)...)
		if peer := m.peerLabeled(m.pending.Machine); peer == nil || !peer.OK {
			status := attention.SingleLine(m.pending.Machine) + " unavailable; opening disabled"
			if m.targetChecking(m.pending.Machine) {
				status = "checking inventory; opening disabled"
			}
			lines = append(lines, "")
			lines = append(lines, wrapped(status, width)...)
		}
		return window(lines, focusStart, focusEnd, height)
	case "create":
		lines := []string{bold.Styled("new session"), ""}
		focusStart, focusEnd := 0, 0
		for index, label := range []string{"machine", "launch", "name", "directory", "group"} {
			value := m.form[index]
			if index == formDirectory {
				if _, _, problem := parseDirectoryDraft(value); problem != "" {
					value = strconv.QuoteToASCII(value)
				}
				if value == "" && m.field != formDirectory {
					value = "home (~)"
				}
			}
			if index == formGroup {
				value = groupDraftDisplay(value)
			}
			if index == m.field {
				focusStart = len(lines)
			}
			lines = append(lines, field(label, value, index == m.field, index < formName, 9, width)...)
			if index == formDirectory && m.field == formDirectory {
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
			if index == formGroup && m.field == formGroup {
				lines = append(lines, m.groupPreview(m.form[formGroup], &m.groupSelection, 9, width)...)
			}
			if index == m.field {
				focusEnd = len(lines)
			}
		}
		if m.field == formCreate {
			focusStart = len(lines)
		}
		lines = append(lines, field("", "create", m.field == formCreate, false, 9, width)...)
		if m.field == formCreate {
			focusEnd = len(lines)
		}
		lines = append(lines, "", "name: leave blank to follow the terminal title.")
		if m.field == formDirectory {
			lines = append(lines, "type search words or a path (/… or ~/…); tab uses the match.")
		}
		if !m.createAvailable() {
			status := "host unavailable; create disabled"
			if m.targetChecking(m.form[formMachine]) {
				status = "checking inventory; create disabled"
			}
			lines = append(lines, status)
		}
		if _, err := group.ParseDraft(m.form[formGroup]); err != nil {
			lines = append(lines, wrapped(group.ErrInvalid.Error(), width)...)
		}
		return window(lines, focusStart, focusEnd, height)
	case "usage":
		return m.usageBody(height)
	}
	if m.page == "keys" {
		lines := m.keyMap()
		rendered := make([]string, min(len(lines), height))
		for index := range rendered {
			rendered[index] = m.renderLine(lines[index])
		}
		return rendered
	}
	return nil
}

func (m *model) emptyTable() string {
	if !m.initialInventorySettled() {
		return "checking inventory"
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

func (m *model) pageCapacity(body int) int {
	return max(0, body-3)
}

// statusView projects a row's terminal status. A row without remote actions
// is stale; readiness also requires a good read of the hub's notification state.
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
