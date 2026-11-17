package sessionui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/charmbracelet/x/ansi"
)

// onPaste appends to the focused text: a metadata draft or a create field.
func (m *model) onPaste(message tea.PasteMsg) tea.Cmd {
	if m.busy || m.tooSmall() {
		return nil
	}
	if (m.page == "group-edit" || m.page == "name-edit") && !m.metadata.checking && !m.metadata.saveFocused {
		m.metadata.draft += message.Content
		if m.page == "group-edit" {
			m.metadata.selection.reset(m.metadata.draft, m.groupChoices(m.metadata.draft))
		}
	}
	if m.page == "create" && m.field >= formName && m.field < formCreate {
		if m.field == formDirectory || m.field == formGroup {
			m.form[m.field] += message.Content
		} else {
			m.form[m.field] += attention.SingleLine(message.Content)
		}
		if m.field == formDirectory {
			return m.searchDirectory()
		}
		if m.field == formGroup {
			m.groupSelection.reset(m.form[formGroup], m.groupChoices(m.form[formGroup]))
		}
	}
	return nil
}

// onKey refuses keys while an action is in flight, quits on ctrl-c from any
// page, and below the minimum size admits only each page's way out.
func (m *model) onKey(message tea.KeyPressMsg) tea.Cmd {
	key := message.String()
	if m.busy {
		m.inform("operation in flight; delivery will be reported")
		return nil
	}
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.tooSmall() {
		cancel := key == "esc"
		switch m.page {
		case "details", "usage", "quick-create":
			cancel = cancel || key == "q"
		case "confirm":
			cancel = cancel || key == "q" || key == "n"
		case "keys":
			cancel = cancel || key == "q" || key == "?"
		}
		if !cancel {
			return nil
		}
	}
	if m.page == "usage" {
		m.usageKey(key)
		return nil
	}
	if key == "ctrl+r" && m.page != "" && m.page != "keys" {
		return m.refresh(true)
	}
	switch m.page {
	case "group-edit", "name-edit":
		return m.editMetadata(message)
	case "confirm":
		return m.confirmKey(key)
	case "create":
		return m.editForm(message)
	case "quick-create":
		return m.editQuickCreate(message)
	case "details":
		m.detailsKey(key)
		return nil
	}
	if m.page == "keys" && (key == "esc" || key == "?" || key == "q") {
		m.page = ""
		return nil
	}
	for _, entry := range m.keys() {
		if !slices.Contains(entry.keys, key) {
			continue
		}
		if refusal := entry.refusal(m); refusal != "" {
			m.fail(refusal)
			return nil
		}
		if m.page == "keys" {
			m.page = ""
		}
		return entry.act(m)
	}
	return nil
}

func (m *model) confirmKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.offset -= m.pageCapacity(m.frame().body)
	case "pgdown":
		m.offset += m.pageCapacity(m.frame().body)
	case "y", "enter":
		current := m.rowForReference(m.pending.Ref)
		if current == nil {
			m.fail(m.pendingName + " on " + m.pendingLabel + " is no longer listed; nothing was sent.")
			return nil
		}
		if !m.rowAvailable(*current) {
			m.fail(m.rowRefusal(*current, m.pending.Operation))
			return nil
		}
		request := m.pending
		m.page = ""
		m.inform(request.Operation + " in progress")
		return m.execute(request)
	case "n", "q", "esc":
		m.page = ""
	}
	return nil
}

func (m *model) detailsKey(key string) {
	switch key {
	case "r":
		m.openMetadata("rename")
	case "g":
		m.openMetadata("group")
	case "q", "esc":
		m.page = ""
	case "up", "k":
		m.offset = max(0, m.offset-1)
	case "down", "j":
		m.offset = min(max(0, len(m.detailLines())-m.pageCapacity(m.frame().body)), m.offset+1)
	case "pgup":
		m.offset = max(0, m.offset-m.pageCapacity(m.frame().body))
	case "pgdown":
		m.offset = min(max(0, len(m.detailLines())-m.pageCapacity(m.frame().body)), m.offset+m.pageCapacity(m.frame().body))
	}
}

type key struct {
	keys        []string
	show, label string
	group       string
	rank        int
	keep        bool
	can         func(*model) string
	act         func(*model) tea.Cmd
}

func (k key) refusal(m *model) string {
	if k.can != nil {
		return k.can(m)
	}
	return ""
}

// The table is the authority for input, visible hints and the key map.
func (m *model) keys() []key {
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
				m.page, m.offset, m.pageRow = "details", 0, *m.selectedRow()
				return nil
			}},
		{keys: []string{"tab"}, show: "tab", label: "next needing input", group: "session", rank: 3,
			can: markedGuard(1), act: markedMove(1)},
		{keys: []string{"x"}, show: "x", label: "interrupt and close…", group: "session", rank: 4,
			can: rowGuard("close", false), act: confirmAction("close")},
		{keys: []string{"N"}, show: "N", label: "new session…", group: "create", rank: 6,
			can: func(m *model) string {
				if defaultIndex < 0 {
					return "target unavailable; refresh before creating."
				}
				return ""
			},
			act: func(m *model) tea.Cmd {
				peer := m.peers[defaultIndex]
				m.page, m.field = "create", formMachine
				m.clearDirectorySearch()
				m.form = [5]string{formMachine: peer.Label, formLaunch: "terminal", formGroup: m.view.label.String()}
				m.groupSelection.prefill(m.form[formGroup], m.groupChoices(m.form[formGroup]))
				m.inform("")
				return nil
			}},
		{keys: []string{"n"}, show: "n", label: "new terminal…", group: "create", rank: 7,
			act: func(m *model) tea.Cmd {
				m.page = "quick-create"
				m.pending = fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchTerminal, Machine: m.client.DefaultMachine().Label, CWD: "~", Group: m.view.label}
				m.inform("")
				return nil
			}},
		{keys: []string{"s"}, show: "s", label: "interrupt…", group: "session", rank: 8,
			can: rowGuard("interrupt", false), act: confirmAction("stop")},
		{keys: []string{"T"}, show: "T", label: "terminal here", group: "create", rank: 9,
			can: func(m *model) string {
				if refusal := rowGuard("terminal here", false)(m); refusal != "" {
					return refusal
				}
				row := m.selectedRow()
				if row.Session.Connection != nil {
					return row.Session.Name + " on " + row.Label + " has a remote connection; use n to choose a machine."
				}
				return ""
			},
			act: func(m *model) tea.Cmd {
				row := m.selectedRow()
				m.inform("opening terminal on " + row.Label + "…")
				return m.execute(fleetclient.Request{Operation: "shell", Ref: row.Session.Ref.Reference})
			}},
		{keys: []string{"u"}, show: "u", label: "usage", group: "browser", rank: 10,
			act: func(m *model) tea.Cmd { m.page, m.offset = "usage", 0; return nil }},
		{keys: []string{"?"}, show: "?", label: "keys", group: "browser", keep: true,
			act: func(m *model) tea.Cmd { m.page = "keys"; return nil }},
		{keys: []string{"q"}, show: "q", label: "quit", group: "browser", keep: true,
			act: func(*model) tea.Cmd { return tea.Quit }},
		{keys: []string{"up", "k"}, show: "↑↓ jk", label: "move", group: "browser",
			act: func(m *model) tea.Cmd { m.move(-1); return nil }},
		{keys: []string{"down", "j"}, act: func(m *model) tea.Cmd { m.move(1); return nil }},
		{keys: []string{"left", "h"}, show: "←→ hl", label: "step view", group: "browser",
			act: func(m *model) tea.Cmd { m.step(-1); return nil }},
		{keys: []string{"right", "l"}, act: func(m *model) tea.Cmd { m.step(1); return nil }},
		{keys: []string{"shift+tab"}, show: "shift-tab", label: "previous mark", group: "session",
			can: markedGuard(-1), act: markedMove(-1)},
		{keys: []string{"ctrl+r"}, show: "ctrl-r", label: "refresh", group: "browser",
			act: func(m *model) tea.Cmd { return tea.Batch(m.refresh(true), m.refreshUsage(true)) }},
	}
}

func rowGuard(action string, attach bool) func(*model) string {
	return func(m *model) string {
		row := m.selectedRow()
		if row == nil {
			return "no session selected; nothing was sent."
		}
		if attach && m.rowAttachable(*row) || !attach && m.rowAvailable(*row) {
			return ""
		}
		return m.rowRefusal(*row, action)
	}
}

func confirmAction(operation string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		row := m.selectedRow()
		m.pending = fleetclient.Request{Operation: operation, Ref: row.Session.Ref.Reference}
		m.pendingName, m.pendingLabel = row.Session.Name, row.Label
		m.page, m.offset = "confirm", 0
		return nil
	}
}

func (m *model) nextMarked(direction int) int {
	count := len(m.rows)
	for distance := 1; distance < count; distance++ {
		index := (m.cursor + count + direction*distance) % count
		if m.statusView(m.rows[index]).Queue != fleetview.QueueExcluded {
			return index
		}
	}
	return -1
}

func markedGuard(direction int) func(*model) string {
	return func(m *model) string {
		if m.nextMarked(direction) < 0 {
			return "no other session in this view reads as needing input."
		}
		return ""
	}
}

func markedMove(direction int) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		m.selectRow(m.nextMarked(direction))
		return nil
	}
}

func (m *model) keyHints(width int) line {
	if m.busy {
		return nil
	}
	if m.page == "keys" {
		return line{{text: "escape", bold: true}, {text: " close", role: quietRole}}
	}
	entries := m.keys()
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

func (m *model) keyMap() []line {
	groups := []string{"session", "create", "browser"}
	columns := make([][]span, len(groups))
	width := (m.width - 2) / len(groups)
	for index, group := range groups {
		columns[index] = append(columns[index], span{text: group, bold: true})
	}
	rows := 0
	for _, entry := range m.keys() {
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
	lines := []line{{{text: "keys", bold: true}}, nil}
	for row := 0; row < rows; row++ {
		var keyRow line
		for _, col := range columns {
			if row >= len(col) {
				keyRow = append(keyRow, span{text: strings.Repeat(" ", width)})
				continue
			}
			part := col[row]
			part.text = cell(part.text, width)
			keyRow = append(keyRow, part)
		}
		lines = append(lines, keyRow)
	}
	return append(lines, nil, line{{text: "ctrl-] d detach  ctrl-] ctrl-] send ctrl-]", role: quietRole}})
}

func (m *model) rowAttachable(row fleetclient.Row) bool {
	return row.Available || m.hostChecking(row) && (!m.guest || m.rowForReference(row.Session.Ref.Reference) != nil)
}

func (m *model) rowRefusal(row fleetclient.Row, action string) string {
	reason := "unavailable"
	if m.rowForReference(row.Session.Ref.Reference) == nil {
		reason = "no longer listed"
	} else if m.hostChecking(row) || m.heldReady(row) {
		reason = "being checked"
	}
	return row.Session.Name + " on " + row.Label + " is " + reason + "; " + action + " is disabled."
}
