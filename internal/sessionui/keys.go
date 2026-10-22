package sessionui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
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
		case "", "details", "usage", "quick-create":
			cancel = cancel || key == "q"
		case "confirm":
			cancel = cancel || key == "q" || key == "n"
		}
		if !cancel {
			return nil
		}
	}
	if m.page == "usage" {
		m.usageKey(key)
		return nil
	}
	if key == "ctrl+r" {
		if m.page == "" {
			return tea.Batch(m.refresh(true), m.refreshUsage(true))
		}
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
	return m.tableKey(key)
}

func (m *model) confirmKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.offset -= m.pageCapacity()
	case "pgdown":
		m.offset += m.pageCapacity()
	case "y", "enter":
		current := m.rowForReference(m.pending.Ref)
		if current == nil || !current.Available {
			m.inform("session unavailable; refresh before confirming")
			return nil
		}
		request := m.pending
		m.page = ""
		m.inform(request.Operation + " in progress")
		return m.execute(request)
	case "n", "q", "esc":
		m.page = ""
		m.pending = fleetclient.Request{}
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
		m.offset = min(max(0, len(m.detailLines())-m.pageCapacity()), m.offset+1)
	case "pgup":
		m.offset = max(0, m.offset-m.pageCapacity())
	case "pgdown":
		m.offset = min(max(0, len(m.detailLines())-m.pageCapacity()), m.offset+m.pageCapacity())
	}
}

func (m *model) tableKey(key string) tea.Cmd {
	switch key {
	case "q":
		return tea.Quit
	case "u":
		m.page, m.offset = "usage", 0
		m.pending = fleetclient.Request{}
		return nil
	case "esc":
		// escape closes pages; the table is its floor, never an exit.
		return nil
	case "up", "k":
		m.move(-1)
		return nil
	case "down", "j":
		m.move(1)
		return nil
	case "left", "h":
		m.step(-1)
		return nil
	case "right", "l":
		m.step(1)
		return nil
	case "n":
		m.page = "quick-create"
		m.pending = fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchTerminal, Machine: m.client.DefaultMachine().Label, CWD: "~", Group: m.view.label}
		m.inform("")
		return nil
	case "N":
		peer := m.creationPeer()
		if peer == nil {
			m.inform("target unavailable; refresh before creating")
			return nil
		}
		m.page, m.field = "create", formMachine
		m.clearDirectorySearch()
		m.form = [5]string{formMachine: peer.Label, formLaunch: "terminal", formGroup: m.view.label.String()}
		m.groupSelection.prefill(m.form[formGroup], m.groupChoices(m.form[formGroup]))
		m.inform("")
		return nil
	}
	row := m.selectedRow()
	if row == nil {
		return nil
	}
	if key == "space" || key == " " {
		m.page, m.offset = "details", 0
		m.pageRow = *row
		return nil
	}
	if !row.Available {
		return nil
	}
	ref := row.Session.Ref.Reference
	switch key {
	case "T":
		if row.Session.Connection != nil {
			m.inform("terminal here unavailable for a remote connection; use n to choose a machine")
			return nil
		}
		m.inform("opening terminal on " + row.Label + "…")
		return m.execute(fleetclient.Request{Operation: "shell", Ref: ref})
	case "enter":
		return m.enter(ref)
	case "s", "x":
		m.pending = fleetclient.Request{Operation: "stop", Ref: ref}
		if key == "x" {
			m.pending.Operation = "close"
		}
		m.pendingName, m.pendingLabel = row.Session.Name, row.Label
		m.page, m.offset = "confirm", 0
	}
	return nil
}
