package sessionui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/charmbracelet/x/ansi"
)

// onPaste appends to the focused text: a metadata draft or a create field.
func (m *model) onPaste(message tea.PasteMsg) tea.Cmd {
	if m.busy || m.tooSmall() || m.fold.loss != "" {
		return nil
	}
	if m.fold.metadata() && !m.metadata.checking && !m.metadata.saveFocused {
		m.editMetadataText(m.metadata.draft + message.Content)
	}
	if m.fold.kind == foldDraft && m.field >= formName && m.field < formCreate {
		return m.setDraftText(m.draftText() + message.Content)
	}
	return nil
}

// onKey applies the same admission rule used by the hint and verdict.
func (m *model) onKey(message tea.KeyPressMsg) tea.Cmd {
	pressed := message.String()
	if m.busy {
		m.inform("operation in flight; delivery will be reported")
		return nil
	}
	if pressed == "ctrl+c" {
		return tea.Quit
	}

	for _, entry := range m.keys() {
		if !slices.Contains(entry.keys, pressed) {
			continue
		}
		if m.tooSmall() && !entry.closes {
			return nil
		}
		if refusal := entry.refusal(m); refusal != "" {
			if entry.onRefusal != nil {
				entry.onRefusal(m)
			}
			m.fail(refusal)
			return nil
		}
		if m.fold.kind == foldKeys {
			m.fold = fold{}
		}
		return entry.act(m)
	}
	if m.tooSmall() {
		return nil
	}
	if pressed == "ctrl+r" {
		return tea.Batch(m.refresh(true), m.refreshUsage(true))
	}
	if m.fold.loss == "" && m.fold.metadata() && !m.metadata.checking && !m.metadata.saveFocused && message.Text != "" {
		m.editMetadataText(m.metadata.draft + message.Text)
	}
	if m.fold.kind == foldDraft && m.field >= formName && m.field < formCreate && message.Text != "" {
		return m.setDraftText(m.draftText() + message.Text)
	}
	return nil
}

type key struct {
	keys         []string
	show, label  string
	group        string
	rank         int
	keep, closes bool
	can          func(*model) string
	onRefusal    func(*model)
	act          func(*model) tea.Cmd
}

func (k key) refusal(m *model) string {
	if k.can != nil {
		return k.can(m)
	}
	return ""
}

func (m *model) keys() []key {
	if m.fold.loss != "" {
		return []key{{keys: []string{"esc"}, show: "escape", label: "dismiss", keep: true, closes: true,
			act: func(m *model) tea.Cmd {
				if m.fold.metadata() {
					m.metadata = nil
				}
				m.fold = fold{}
				return nil
			}}}
	}
	switch m.fold.kind {
	case foldOpen:
		return m.openKeys()
	case foldConfirm:
		return []key{
			{keys: []string{"enter", "y"}, show: "enter", label: "send", rank: 1,
				can: (*model).confirmRefusal,
				act: func(m *model) tea.Cmd {
					question := m.fold
					m.fold = fold{}
					subject := question.name + " on " + question.machine
					m.inform(subject + ": " + question.request.Operation + " in progress")
					return m.execute(question.request, subject)
				}},
			{keys: []string{"esc", "n", "q"}, show: "escape", label: "cancel", keep: true, closes: true,
				act: func(m *model) tea.Cmd { m.fold = fold{}; return nil }},
			m.usageKey(0),
		}
	case foldRename, foldGroup:
		return m.metadataKeys()
	case foldDraft:
		return m.draftKeys()
	case foldTerminal:
		return m.terminalKeys()
	case foldKeys:
		close := key{keys: []string{"esc", "q", "?"}, show: "escape", label: "close", keep: true, closes: true,
			act: func(m *model) tea.Cmd { m.fold = fold{}; return nil }}
		entries := []key{close}
		for _, entry := range m.tableKeys() {
			if entry.keys[0] != "q" && entry.keys[0] != "?" {
				entry.rank = 0
				entries = append(entries, entry)
			}
		}
		return entries
	}
	return m.tableKeys()
}

// The table is the authority for input, visible hints and the key map.
func (m *model) tableKeys() []key {
	defaultIndex := m.peerIndex(m.client.DefaultMachine().Handle)
	return []key{
		{keys: []string{"enter"}, show: "enter", label: "attach", group: "session", rank: 1,
			can: rowGuard("attach", true),
			act: func(m *model) tea.Cmd { return m.enter(m.selectedRow().Session.Ref.Reference) }},
		{keys: []string{"space", " "}, show: "space", label: "open", group: "session", rank: 2,
			can: func(m *model) string {
				if m.selectedRow() == nil {
					return "no session selected."
				}
				return ""
			},
			act: func(m *model) tea.Cmd {
				m.fold = fold{kind: foldOpen}
				return m.askScreen()
			}},
		{keys: []string{"tab"}, show: "tab", label: "next needing input", group: "session", rank: 3,
			can: queuedGuard(1), act: queuedMove(1)},
		{keys: []string{"shift+tab"}, show: "shift-tab", label: "previous", group: "session",
			can: queuedGuard(-1), act: queuedMove(-1)},
		{keys: []string{"x"}, show: "x", label: "interrupt and close…", group: "session", rank: 4,
			can: rowGuard("close", false), act: confirmAction("close")},
		{keys: []string{"c"}, show: "c", label: "another like this…", group: "create", rank: 5,
			can: localRowGuard("create"), act: (*model).seededDraft},
		{keys: []string{"N"}, show: "N", label: "new session…", group: "create", rank: 6,
			can: func(m *model) string {
				if defaultIndex < 0 {
					return m.hostRefusal(m.client.DefaultMachine().Label)
				}
				return ""
			},
			act: func(m *model) tea.Cmd {
				peer := m.peers[defaultIndex]
				return m.openDraft(createDraft{machine: peer.Label, launch: launchChoice{kind: fleetclient.LaunchTerminal}, group: m.view.label.String()}, formMachine, false)
			}},
		{keys: []string{"n"}, show: "n", label: "new terminal…", group: "create", rank: 7,
			act: func(m *model) tea.Cmd {
				m.fold = fold{kind: foldTerminal, request: fleetclient.Request{
					Operation: "start", Kind: fleetclient.LaunchTerminal, Machine: m.client.DefaultMachine().Label,
					CWD: "~", Group: m.view.label,
				}}
				m.inform("")
				return nil
			}},
		{keys: []string{"s"}, show: "s", label: "interrupt…", group: "session", rank: 8,
			can: rowGuard("interrupt", false), act: confirmAction("stop")},
		{keys: []string{"T"}, show: "T", label: "terminal here", group: "create", rank: 9,
			can: func(m *model) string {
				if refusal := localRowGuard("terminal here")(m); refusal != "" {
					return refusal
				}
				return m.ground.attachRefusal("T")
			},
			act: func(m *model) tea.Cmd {
				row := m.selectedRow()
				m.inform("opening terminal on " + row.Label + "…")
				return m.execute(fleetclient.Request{Operation: "shell", Ref: row.Session.Ref.Reference}, row.Session.Name+" on "+row.Label)
			}},
		m.usageKey(10),
		{keys: []string{"?"}, show: "?", label: "keys", group: "browser", keep: true,
			act: func(m *model) tea.Cmd { m.fold = fold{kind: foldKeys}; return nil }},
		{keys: []string{"q"}, show: "q", label: "quit", group: "browser", keep: true,
			act: func(*model) tea.Cmd { return tea.Quit }},
		{keys: []string{"up", "k"}, show: "↑↓ jk", label: "move", group: "browser",
			act: func(m *model) tea.Cmd { m.move(-1); return nil }},
		{keys: []string{"down", "j"}, act: func(m *model) tea.Cmd { m.move(1); return nil }},
		{keys: []string{"left", "h"}, show: "←→ hl", label: "step view", group: "browser",
			act: func(m *model) tea.Cmd { m.step(-1); return nil }},
		{keys: []string{"right", "l"}, act: func(m *model) tea.Cmd { m.step(1); return nil }},
		{keys: []string{"ctrl+r"}, show: "ctrl-r", label: "refresh", group: "browser",
			act: func(m *model) tea.Cmd { return tea.Batch(m.refresh(true), m.refreshUsage(true)) }},
	}
}

func (m *model) usageKey(rank int) key {
	return key{keys: []string{"u"}, show: "u", label: "usage", group: "browser", rank: rank,
		can: func(m *model) string { return m.ground.attachRefusal("u") },
		act: func(m *model) tea.Cmd {
			m.band = !m.band
			m.revealGeneration++
			if m.band && m.ground.raise != nil {
				m.reveal = m.now
				return m.revealTick()
			}
			m.reveal = time.Time{}
			return nil
		}}
}

func rowGuard(action string, attach bool) func(*model) string {
	return func(m *model) string {
		row := m.selectedRow()
		if row == nil {
			return "no session selected; nothing was sent."
		}
		if attach && m.rowAttachable(*row) {
			return m.ground.attachRefusal("enter")
		}
		if !attach && m.rowAvailable(*row) {
			return ""
		}
		return m.rowRefusal(*row, action)
	}
}

func localRowGuard(action string) func(*model) string {
	available := rowGuard(action, false)
	return func(m *model) string {
		if refusal := available(m); refusal != "" {
			return refusal
		}
		row := m.selectedRow()
		if row.Session.Connection != nil {
			return row.Session.Name + " on " + row.Label + " has a remote connection; nothing was sent. use n to choose a machine."
		}
		return ""
	}
}

func confirmAction(operation string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		row := m.selectedRow()
		m.fold = fold{
			kind: foldConfirm, request: fleetclient.Request{Operation: operation, Ref: row.Session.Ref.Reference},
			name: row.Session.Name, machine: row.Label,
		}
		return nil
	}
}

func (m *model) nextQueued(direction int) int {
	count := len(m.rows)
	for distance := 1; distance < count; distance++ {
		index := (m.cursor + count + direction*distance) % count
		if fleetview.QueueRank(m.statusView(m.rows[index]).Sign) >= 0 {
			return index
		}
	}
	return -1
}

func queuedGuard(direction int) func(*model) string {
	return func(m *model) string {
		if m.nextQueued(direction) < 0 {
			return "no other session in this view reads as needing input."
		}
		return ""
	}
}

func queuedMove(direction int) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		m.selectRow(m.nextQueued(direction))
		if m.fold.kind == foldOpen {
			m.fold.facts = false
			return m.askScreen()
		}
		return nil
	}
}

func (m *model) keyHints(width int) line {
	if m.busy {
		return nil
	}

	entries := m.keys()
	slices.SortStableFunc(entries, func(a, b key) int { return a.rank - b.rank })
	for _, entry := range entries {
		if entry.keep {
			width -= ansi.StringWidth(entry.show+" "+entry.label) + 2
		}
	}
	var hints line
	for _, entry := range entries {
		cells := ansi.StringWidth(entry.show + " " + entry.label)
		if entry.rank != 0 && cells <= width && entry.refusal(m) == "" {
			hints = appendHint(hints, entry)
			width -= cells + 2
		}
	}
	for _, entry := range entries {
		if entry.keep {
			hints = appendHint(hints, entry)
		}
	}
	return hints
}

func appendHint(hints line, entry key) line {
	if len(hints) != 0 {
		hints = append(hints, span{text: "  "})
	}
	return append(hints, span{text: entry.show, bold: true}, span{text: " " + entry.label, role: quietRole})
}

func (m *model) keyMap(width int) []line {
	groups := []string{"session", "create", "browser"}
	columns := make([][]span, len(groups))
	columnWidth := width / len(groups)
	for index, group := range groups {
		columns[index] = append(columns[index], span{text: group, bold: true})
	}
	rows := 0
	for _, entry := range m.tableKeys() {
		if entry.show == "" {
			continue
		}
		col := slices.Index(groups, entry.group)
		text := entry.show + " " + entry.label
		role := inkRole
		if entry.refusal(m) != "" {
			role = quietRole
		}
		columns[col] = append(columns[col], span{text: text, role: role})
		rows = max(rows, len(columns[col]))
	}
	lines := []line{{{text: "keys", bold: true}}}
	for row := 0; row < rows; row++ {
		var keyRow line
		for _, col := range columns {
			if row >= len(col) {
				keyRow = append(keyRow, span{text: strings.Repeat(" ", columnWidth)})
				continue
			}
			part := col[row]
			part.text = cell(part.text, columnWidth)
			keyRow = append(keyRow, part)
		}
		lines = append(lines, keyRow)
	}
	var legend line
	used := 0
	for _, entry := range []struct {
		sign  api.StatusSign
		label string
	}{
		{api.SignRequest, " request"}, {api.SignError, " error"}, {api.SignReady, " ready"},
		{api.SignInterrupted, " interruption"}, {api.SignMenu, " menu"},
		{api.SignWorking, " working"}, {api.SignIdle, " idle"}, {api.SignUnknown, " unknown"},
		{api.SignTerminal, " terminal"}, {"", ": work continues"},
	} {
		sign := span{text: "underlined", role: quietRole, under: ansi.UnderlineDouble, underRole: frostRole}
		if entry.sign != "" {
			sign = signSpan(entry.sign)
		}
		cells := ansi.StringWidth(sign.text + entry.label)
		if used > 0 && used+2+cells > width {
			lines = append(lines, legend)
			legend, used = nil, 0
		}
		if used > 0 {
			legend = append(legend, span{text: "  "})
			used += 2
		}
		legend = append(legend, sign, span{text: entry.label, role: quietRole})
		used += cells
	}
	lines = append(lines, legend)
	shortcuts := line{{text: "ctrl-] d detach  ctrl-] ctrl-] send ctrl-]", role: quietRole}}
	for _, entry := range m.openKeys() {
		switch entry.show {
		case "i", "r", "g":
			label := entry.label
			if entry.show == "i" {
				label = "facts"
			}
			shortcuts = append(shortcuts, span{text: "  " + entry.show + " " + label, role: quietRole})
		}
	}
	note := m.ground.note
	if note == "" {
		note = "colours: reading the terminal's own."
	}
	return append(lines, shortcuts, line{{text: note, role: quietRole}})
}

func (m *model) rowAttachable(row fleetclient.Row) bool {
	return row.Available || m.hostChecking(row)
}

func (m *model) rowRefusal(row fleetclient.Row, action string) string {
	reason := m.rowUnavailability(row)
	return row.Session.Name + " on " + row.Label + " is " + reason + "; " + action + " is disabled."
}

func (m *model) rowUnavailability(row fleetclient.Row) string {
	if m.rowChecking(row) {
		return "being checked"
	}
	return "unavailable"
}
