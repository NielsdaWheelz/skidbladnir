package sessionui

import (
	"fmt"
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/charmbracelet/x/ansi"
)

type foldKind uint8

const (
	foldNone foldKind = iota
	foldOpen
	foldConfirm
	foldRename
	foldGroup
	foldDraft
	foldTerminal
	foldUsage
	foldKeys
)

type fold struct {
	kind          foldKind
	request       fleetclient.Request
	name, machine string
	facts, seeded bool
	offset        int
}

func (m *model) openKeys() []key {
	i := "all facts"
	if m.fold.facts {
		i = "screen"
	}
	entries := []key{
		{keys: []string{"i"}, show: "i", label: i, rank: 3,
			act: func(m *model) tea.Cmd { m.fold.facts = !m.fold.facts; return nil }},
		{keys: []string{"r"}, show: "r", label: "rename", rank: 5,
			can: editGuard("rename"), act: func(m *model) tea.Cmd { m.openMetadata("rename"); return nil }},
		{keys: []string{"g"}, show: "g", label: "group", rank: 6,
			can: editGuard("group"), act: func(m *model) tea.Cmd { m.openMetadata("group"); return nil }},
		{keys: []string{"space", " ", "esc", "q"}, show: "space", label: "fold", keep: true, closes: true,
			act: func(m *model) tea.Cmd { m.fold = fold{}; return nil }},
	}
	for _, entry := range m.tableKeys() {
		switch entry.keys[0] {
		case "enter", "shift+tab":
		case "tab":
			entry.rank = 2
		case "ctrl+r":
			entry.rank, entry.label = 4, "read again"
		case "x":
			entry.rank = 7
		case "s":
			entry.rank = 8
		case "up", "down", "left", "right":
			act := entry.act
			entry.act = func(m *model) tea.Cmd {
				m.fold = fold{}
				return act(m)
			}
		default:
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func (m *model) confirmRefusal() string {
	f := m.fold
	current := m.rowForReference(f.request.Ref)
	if current == nil || !m.rowAvailable(*current) {
		return f.name + " on " + f.machine + " is " + m.rowUnavailability(current) + "; nothing was sent."
	}
	return ""
}

func (f *frame) composeInline(cursor int, lines []line) {
	if len(lines) == 0 {
		return
	}
	anchor := 0
	for _, row := range f.rows {
		if row.index == cursor {
			anchor = row.line
			break
		}
	}
	above := anchor+len(lines) >= f.body
	f.foldAt, f.fold = anchor+1, lines
	if above {
		f.foldAt = anchor - len(lines)
	}
	visible := f.rows[:0]
	for _, row := range f.rows {
		switch {
		case above && row.line <= anchor:
			if row.line != anchor {
				row.line -= len(lines)
			}
			if row.heading >= 0 {
				row.heading -= len(lines)
			}
		case !above && row.line > anchor:
			row.line += len(lines)
			if row.heading >= 0 {
				row.heading += len(lines)
			}
		}
		if row.line >= 0 && row.line < f.body {
			visible = append(visible, row)
		}
	}
	f.rows = visible
}

func (f fold) foot() bool {
	return f.kind == foldTerminal || f.kind == foldUsage || f.kind == foldKeys || f.kind == foldDraft && !f.seeded
}

func (f *frame) composeFoot(lines []line) {
	if len(lines) == 0 {
		return
	}
	f.foldAt, f.fold = f.body-len(lines), lines
	if covered := slices.IndexFunc(f.rows, func(row tablePlacement) bool { return row.line >= f.foldAt }); covered >= 0 {
		f.rows = f.rows[:covered]
	}
}

func (m *model) foldLines(capacity, width int) ([]line, int) {
	var lines []line
	offset := 0
	background := m.ground.raise
	switch m.fold.kind {
	case foldConfirm:
		action, effect := "interrupt and close", "send one interrupt to the selected pane, then close this entire session. closure proceeds even if interruption fails. work shared elsewhere or running remotely may continue."
		if m.fold.request.Operation == "stop" {
			action, effect = "interrupt", "send one interrupt to the selected pane; retain this session. stopping is unconfirmed."
		}
		question := wrapped(action+" "+m.fold.name+" on "+m.fold.machine+"?", width)
		if len(question) > min(capacity, 5) || slices.ContainsFunc(question, func(part string) bool { return ansi.StringWidth(part) > width }) {
			question = []string{action + " " + target(m.fold.name, m.fold.machine, width-ansi.StringWidth(action)-2) + "?"}
		}
		for _, part := range question {
			lines = append(lines, line{{text: part, bold: true, role: emberRole}})
		}
		for _, part := range wrapped(effect, width) {
			lines = append(lines, line{{text: part}})
		}
		lines = lines[:min(5, len(lines))]
	case foldOpen:
		row := m.selectedRow()
		if m.fold.facts {
			facts := m.details(row)
			for index, fact := range facts {
				label := fact[0]
				if label != "" {
					label += "  "
				}
				parts := wrapped(label+fact[1], width)
				reserve := 0
				if index+1 < len(facts) {
					reserve = 1
				}
				if len(lines)+len(parts)+reserve > capacity {
					lines = append(lines, line{{text: fmt.Sprintf("… %d more facts in a taller window", len(facts)-index), role: quietRole}})
					break
				}
				for _, part := range parts {
					lines = append(lines, line{{text: part}})
				}
			}
		} else {
			current := m.current(row)
			cwd := current.CWD
			if cwd == "" {
				cwd = "directory unavailable"
			}
			summary := row.Session.TerminalHandle + "  pane " + row.Session.ActivePaneID + "  in " + cwd
			if row.Session.ActiveCommand != "" {
				summary += "  runs " + row.Session.ActiveCommand
			}
			if row.Session.AttachedClients > 0 {
				summary += "  attached clients " + strconv.Itoa(row.Session.AttachedClients)
			}
			parts := wrapped(summary, width)
			for _, part := range parts[:min(2, len(parts))] {
				lines = append(lines, line{{text: part}})
			}
		}
	case foldRename, foldGroup:
		editor := m.metadata
		notice := m.notice
		if notice == "" && editor.checking {
			notice = "checking inventory"
		}
		draft := editor.draft
		editing := !m.busy && !editor.checking && !editor.saveFocused
		if m.fold.kind == foldRename {
			if !fleetclient.ValidSessionName(draft) {
				draft = strconv.QuoteToASCII(draft)
			}
			value := ansi.Truncate(attention.SingleLine(draft), width-6, "…")
			lines = append(lines, line{{text: "name  ", role: quietRole}, {text: value, bold: editing}})
			if notice == "" {
				_, notice, _ = m.metadataRequest(false)
			}
		} else {
			limit := (width - 7) / 2
			draft = ansi.Truncate(attention.SingleLine(groupDraftDisplay(draft)), limit, "…")
			lines = append(lines, line{{text: "group  ", role: quietRole}, {text: draft, bold: editing}})
			preview := m.groupChoicePreview(editor.draft, &editor.selection)
			rest := width - ansi.StringWidth(draft) - 13
			pickerRole := quietRole
			if editing {
				pickerRole = goldRole
			}
			lines[0] = append(lines[0], span{text: "  ‹ " + ansi.Truncate(attention.SingleLine(preview), rest, "…") + " ›", role: pickerRole})
			_, verdict, _ := m.metadataRequest(false)
			lines = append(lines, line{{text: "save", bold: !m.busy && !editor.checking && editor.saveFocused}, {text: "  " + verdict, role: quietRole}})
			if editor.saveFocused && capacity == 1 {
				lines = lines[1:]
			}
		}
		if notice != "" {
			lines = append(lines, line{{text: notice, role: quietRole}})
		}
	case foldDraft:
		background = m.ground.forge
		lines = m.draftLines(capacity)
	case foldTerminal:
		lines = []line{
			{{text: "new terminal", bold: true}},
			{{text: "machine  ", role: quietRole}, {text: "‹ " + m.fold.request.Machine + " ›", role: goldRole, bold: true}},
			{{text: "directory  ", role: quietRole}, {text: "home (~)"}},
			{{text: "group  ", role: quietRole}, {text: fleetview.GroupHeading(m.fold.request.Group)}},
		}
		if refusal := m.terminalRefusal(); refusal != "" {
			lines = append(lines, line{{text: refusal, role: quietRole}})
		}
	case foldUsage:
		lines, offset = m.usageFold(capacity)
	case foldKeys:
		lines = m.keyMap(width)
	}
	lines = lines[:min(capacity, len(lines))]
	for index := range lines {
		room := width
		for part := range lines[index] {
			item := &lines[index][part]
			item.text = ansi.Truncate(attention.SingleLine(item.text), max(0, room), "…")
			room -= ansi.StringWidth(item.text)
		}
		lines[index] = append(line{{text: "▌", role: goldRole}, {text: "   "}}, lines[index]...).filled(background, width+4)
	}
	return lines, offset
}

func (m *model) groupChoicePreview(draft string, selection *groupSelection) string {
	return selection.preview(m.groupChoices(draft), func(choice groupChoice) string {
		if choice.label.IsUnassigned() {
			return "unassigned"
		}
		return choice.label.String()
	})
}

func (m *model) draftLines(capacity int) []line {
	var title []line
	if !m.fold.seeded {
		title = []line{{{text: "new session", bold: true}}}
		capacity--
	}
	names := [...]string{"machine", "launch", "name", "directory", "group"}
	values := [...]string{m.form.machine, m.form.launch.label(), m.form.name, m.form.directory, m.form.group}
	lines := make([]line, 0, formCreate+2)
	note := ""
	noteRole := quietRole
	for field, name := range names {
		value := values[field]
		focused := m.field == field
		role := inkRole
		switch field {
		case formMachine, formLaunch:
			if value == "" {
				value = "choose"
			}
			if focused {
				value, role = "‹ "+value+" ›", goldRole
			}
		case formName:
			if value == "" {
				value = "—"
				if focused {
					note = "blank: follows the terminal title"
				}
			}
		case formDirectory:
			if focused {
				note = m.directoryStatus()
				if choice, _, selected := m.directorySelection.current(m.searchDirectories); selected {
					preview := "‹ " + choice + " ›"
					if note != "" {
						preview += "  " + note
					}
					note, noteRole = preview, goldRole
				}
			}
			if value == "" {
				value = "home (~)"
			}
		case formGroup:
			if focused {
				note, noteRole = m.groupChoicePreview(value, &m.groupSelection), goldRole
			}
			if value == "" {
				value = "unassigned"
			} else {
				value = groupDraftDisplay(value)
			}
		}
		lines = append(lines, line{{text: fmt.Sprintf("%-9s  ", name), role: quietRole}, {text: value, role: role, bold: focused}})
	}
	lines = append(lines, line{{text: "create", bold: m.field == formCreate}})
	switch {
	case m.busy:
		note, noteRole = "operation in flight; delivery will be reported", quietRole
	case m.noticeFailure:
		note, noteRole = m.notice, quietRole
	case m.field == formCreate:
		_, note, _ = m.draftRequest()
	case m.field == formLaunch && m.fold.seeded && m.form.launch.kind == "":
		note = m.selectedRow().Session.Name + " does not say how it was started"
	}
	if note != "" {
		lines = append(lines, line{{text: note, role: noteRole}})
	}
	if capacity <= 0 {
		return title
	}
	focus := m.field
	if note != "" && capacity > 1 && focus == formCreate {
		focus++
	}
	start := max(0, focus-capacity+1)
	return append(title, lines[start:min(len(lines), start+capacity)]...)
}
