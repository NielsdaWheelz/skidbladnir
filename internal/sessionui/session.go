// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
)

// listedRow is a session as its peer's inventory listed it at observedAt.
type listedRow struct {
	label, machine, observedAt string
	session                    fleetclient.Session
	available                  bool
}
type inventoryMsg struct {
	machine         string
	value           fleetclient.Inventory
	failure         *fleetclient.Failure
	expected        fleetclient.NotificationSnapshot
	notificationErr error
}
type tickMsg struct{}
type actionMsg struct {
	operation       string
	result          fleetclient.Result
	expected        fleetclient.NotificationSnapshot
	notificationErr error
}
type searchMsg struct {
	machine  string
	revision int
	result   fleetclient.Result
}
type attachedMsg struct {
	err             error
	snapshot        fleetclient.NotificationSnapshot
	notificationErr error
}
type model struct {
	ctx                       context.Context
	client                    *fleetclient.Client
	input, output             *os.File
	peers                     []fleetclient.Peer
	rows                      []listedRow
	cursor                    int
	refreshing, busy          bool
	refreshAfterAction        bool
	width, height             int
	notice, page              string
	noticeFailure             bool
	offset                    int
	pending                   fleetclient.Request
	pendingLabel, pendingName string
	form                      [5]string
	field                     int
	groupSelection            groupSelection
	machine                   string
	view                      selectedView
	scopeReady                bool
	picker                    int
	metadata                  *metadataEditor
	top                       int
	// pageRow is info's captured lifetime as last observed; refresh follows it
	// without retargeting.
	pageRow              listedRow
	searchRevision       int
	searchCancel         context.CancelFunc
	searching            bool
	searchDirectories    []string
	searchCursor         int
	notificationStore    *fleetclient.NotificationStore
	notificationSnapshot fleetclient.NotificationSnapshot
	notificationFailed   bool
}

func Run(ctx context.Context, client *fleetclient.Client, input, output *os.File) error {
	m := newModel(ctx, client, input, output)
	defer m.clearDirectorySearch()
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
}
func newModel(ctx context.Context, client *fleetclient.Client, input, output *os.File) *model {
	peers := []fleetclient.Peer{}
	for _, machine := range client.Machines() {
		peers = append(peers, fleetclient.Peer{Label: machine.Label, Machine: machine.Handle})
	}
	store, storeErr := fleetclient.DefaultNotificationStore()
	return &model{notificationStore: store, notificationFailed: storeErr != nil, ctx: ctx, client: client, input: input, output: output, peers: peers, cursor: -1, width: 100, height: 30, refreshing: true}
}
func (m *model) Init() tea.Cmd { return tea.Batch(m.fetch(), tick()) }
func tick() tea.Cmd            { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m *model) fetch() tea.Cmd {
	machine := m.machine
	return func() tea.Msg {
		var expected fleetclient.NotificationSnapshot
		notificationErr := fleetclient.ErrNotificationsUnavailable
		if m.notificationStore != nil {
			expected, notificationErr = m.notificationStore.Read()
		}
		result := m.client.Execute(m.ctx, fleetclient.Request{Operation: "list", Machine: machine})
		message := inventoryMsg{machine: machine, failure: result.Error, expected: expected, notificationErr: notificationErr}
		if result.OK {
			message.value = result.Value.(fleetclient.Inventory)
		}
		return message
	}
}
func (m *model) refresh() tea.Cmd {
	if m.refreshing {
		return nil
	}
	m.refreshing = true
	return m.fetch()
}

// creationPeer is the machine filter, otherwise the configured default; never the first reachable peer.
func (m *model) creationPeer() *fleetclient.Peer {
	handle := m.client.DefaultMachine().Handle
	for index := range m.peers {
		if m.machine != "" && m.peers[index].Label == m.machine || m.machine == "" && m.peers[index].Machine == handle {
			return &m.peers[index]
		}
	}
	return nil
}

func (m *model) execute(request fleetclient.Request) tea.Cmd {
	m.pending = request
	m.busy = true
	return func() tea.Msg {
		message := actionMsg{operation: request.Operation}
		if request.Operation == "start" || request.Operation == "shell" {
			message.notificationErr = fleetclient.ErrNotificationsUnavailable
			if m.notificationStore != nil {
				message.expected, message.notificationErr = m.notificationStore.Read()
			}
		}
		message.result = m.client.Execute(m.ctx, request)
		return message
	}
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	defer m.fitViewports()
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case inventoryMsg:
		m.refreshing = false
		if m.refreshAfterAction || message.machine != m.machine {
			m.refreshAfterAction = false
			return m, m.refresh()
		}
		m.scopeReady = true
		if message.failure != nil {
			for index := range m.peers {
				if m.machine == "" || m.peers[index].Label == m.machine {
					m.peers[index].OK = false
					m.peers[index].Error = message.failure
				}
			}
		} else {
			for _, received := range message.value.Peers {
				found := false
				for index, previous := range m.peers {
					if previous.Machine != received.Machine {
						continue
					}
					// A failed host keeps its last sessions with the time that observed them.
					if !received.OK {
						received.Sessions, received.ObservedAt = previous.Sessions, previous.ObservedAt
					}
					m.peers[index] = received
					found = true
					break
				}
				if !found {
					m.peers = append(m.peers, received)
				}
			}
		}

		m.observeNotifications(message)
		m.rebuild()
		observed := fleetclient.ObservedGroups(m.scopedPeers())
		m.groupSelection.retain(groupChoices(m.form[4], observed))
		if m.metadata != nil && m.metadata.request.Operation == "group" {
			m.metadata.selection.retain(groupChoices(m.metadata.draft, observed))
		}
		m.reconcileMetadata()
		if m.page == "details" || m.page == "name-edit" || m.page == "group-edit" {
			m.refreshInfo()
		}
		return m, nil
	case actionMsg:
		if message.operation == "start" || message.operation == "shell" {
			if m.ctx.Err() != nil {
				return m, nil
			}
			// Keyboard navigation is blocked while busy, so this pending request
			// and page identify the only interaction allowed to adopt creation:
			// n and T from the table, N from its form.
			if !m.busy || m.pending.Operation != message.operation || m.page != "" && !(message.operation == "start" && m.page == "create") {
				return m, m.refresh()
			}
		}
		m.busy = false
		if m.refreshing {
			m.refreshAfterAction = true
		}
		if !message.result.OK {
			failureText := fleetclient.ErrorMessage(*message.result.Error, m.pending, message.operation == "read")
			m.fail(failureText)
			failure := message.result.Error
			if message.operation == "group" || message.operation == "rename" {
				m.metadata.failure = failure
				if failure.Dispatch == "unknown" || failure.Code == "SessionNotFound" || failure.Code == "SessionIdentityMismatch" || failure.Code == "InternalError" || failure.Code == "Unauthenticated" || failure.Code == "MachineIdentityMismatch" || failure.Code == "SessionNameConflict" || failure.Code == "SessionNameChanged" {
					m.metadata.checking = true
					m.metadata.acknowledged = false
					m.invalidatePendingPeer()
					return m, m.refresh()
				}
			}
			return m, nil
		}
		switch message.operation {
		case "start", "shell":
			value := message.result.Value.(fleetclient.ObservedSession)
			if m.notificationStore == nil || message.notificationErr != nil {
				m.notificationFailed = true
			} else {
				snapshot, err := m.notificationStore.ObserveSession(value.Session, m.client.Machines(), message.expected)
				m.notificationFailed = err != nil
				if err == nil {
					m.notificationSnapshot = snapshot
				}
			}
			// Confirmed creation reveals the new session in its group.
			m.page = ""
			if m.machine != "" && m.machine != value.Label {
				m.machine = value.Label
				m.scopeReady = false
			}
			if m.view.kind == viewNeedsInput || m.view.kind == viewGroup && m.view.label != value.Session.Group {
				m.view = selectedView{kind: viewGroup, label: value.Session.Group}
			}
			created, _ := fleetclient.DecodeReference(value.Session.Ref)
			found := false
			for index := range m.peers {
				if m.peers[index].Machine == value.Machine {
					rows := m.peers[index].Sessions
					present := false
					for rowIndex, row := range rows {
						ref, _ := fleetclient.DecodeReference(row.Ref)
						if ref.SessionEqual(created) {
							rows[rowIndex] = value.Session
							present = true
							break
						}
					}
					if !present {
						rows = append(rows, value.Session)
					}
					m.peers[index].Sessions = rows
					found = true
					break
				}
			}
			if !found {
				m.peers = append(m.peers, fleetclient.Peer{Label: value.Label, Machine: value.Machine, OK: true, Sessions: []fleetclient.Session{value.Session}})
			}
			m.rebuild()
			for index, row := range m.rows {
				ref, _ := fleetclient.DecodeReference(row.session.Ref)
				if ref.SessionEqual(created) {
					m.cursor = index
					break
				}
			}
			m.inform("opening terminal on " + value.Label + "…")
			request := fleetclient.Request{Operation: "enter", Ref: value.Session.Ref}
			return m, m.enter(request)
		case "group", "rename":
			m.metadata.checking = true
			m.metadata.acknowledged = true
			m.invalidatePendingPeer()
			m.inform("metadata saved; checking inventory")

		case "close":
			text := fleetclient.CloseText(message.result.Value.(fleetclient.CloseResult))
			if message.result.ExitCode("close") != 0 {
				m.fail(text)
			} else {
				m.inform(text)
			}
		default:
			receipt := message.result.Value.(fleetclient.WriteResult)
			text := fleetclient.WriteText(message.operation, receipt)
			if receipt.Outcome == "unknown" {
				m.fail(text)
			} else {
				m.inform(text)
			}
		}

		return m, m.refresh()
	case searchMsg:
		if m.page != "create" || m.form[0] != message.machine || m.searchRevision != message.revision {
			return m, nil
		}
		m.searching = false
		if !message.result.OK {
			m.fail("directory search unavailable on " + message.machine)
			if message.result.Error != nil {
				switch message.result.Error.Code {
				case "DirectorySearchTooLarge":
					m.inform("too many results; narrow your search")
				case "invalid_input":
					m.inform("enter 1–8 search words, at most 256 bytes")
				}
			}
			return m, nil
		}
		value := message.result.Value.(fleetclient.DirectorySearchResult)
		m.searchDirectories, m.searchCursor = value.Directories, 0
		m.inform("")
		if len(value.Directories) == 0 {
			m.inform("no matching directories")
		} else if value.Omitted {
			m.inform("some directories are not shown")
		}
		return m, nil

	case attachedMsg:
		m.notificationFailed = message.notificationErr != nil
		if message.notificationErr == nil {
			m.notificationSnapshot = message.snapshot
		}
		m.rebuild()
		if m.refreshing {
			m.refreshAfterAction = true
		}
		if message.err != nil {
			m.fail(message.err.Error())
		} else {
			m.inform("detached; work continues")
		}
		return m, m.refresh()
	case tea.PasteMsg:
		if !m.busy && m.width >= 80 && m.height >= 24 {
			if (m.page == "group-edit" || m.page == "name-edit") && !m.metadata.checking && !m.metadata.saveFocused {
				m.metadata.draft += message.Content
				if m.page == "group-edit" {
					m.metadata.selection.reset(m.metadata.draft, m.groupChoices(m.metadata.draft))
				}
			}
			if m.page == "create" && m.field >= 2 && m.field < len(m.form) {
				if m.field == 4 {
					m.form[m.field] += message.Content
				} else {
					m.form[m.field] += singleLine(message.Content)
				}
				if m.field == 3 {
					return m, m.searchDirectory()
				}
				if m.field == 4 {
					m.groupSelection.reset(m.form[4], m.groupChoices(m.form[4]))
				}
			}
		}
	case tea.KeyPressMsg:
		key := message.String()
		if m.busy {
			m.inform("operation in flight; delivery will be reported")
			return m, nil
		}
		if key == "ctrl+c" {
			m.clearDirectorySearch()
			return m, tea.Quit
		}
		if m.width < 80 || m.height < 24 {
			cancel := key == "esc"
			switch m.page {
			case "", "machine-picker", "details":
				cancel = cancel || key == "q"
			case "confirm":
				cancel = cancel || key == "q" || key == "n"
			}
			if !cancel {
				return m, nil
			}
		}
		if key == "ctrl+r" {
			return m, m.refresh()
		}
		if m.page == "machine-picker" {
			return m, m.editPicker(key)
		}
		if m.page == "group-edit" || m.page == "name-edit" {
			return m, m.editMetadata(message)
		}
		if m.page == "confirm" {
			switch key {
			case "y", "enter":
				current := m.rowForReference(m.pending.Ref)
				if current == nil || !current.available || !m.scopeReady {
					m.inform("session unavailable; refresh before confirming")
					return m, nil
				}
				request := m.pending
				m.page = ""
				m.inform(request.Operation + " in progress")
				return m, m.execute(request)
			case "n", "q", "esc":
				m.page = ""
				m.pending = fleetclient.Request{}
			}
			return m, nil
		}
		if m.page == "create" {
			return m, m.editForm(message)
		}
		if m.page == "details" && (key == "r" || key == "g") {
			operation := "group"
			if key == "r" {
				operation = "rename"
			}
			m.openMetadata(operation)
			return m, nil
		}
		if m.page != "" {
			switch key {
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
			return m, nil
		}
		if m.metadata != nil && m.metadata.checking && (key == "m" || key == "N") {
			m.inform("checking metadata change; refresh before changing machine")
			return m, nil
		}
		switch key {
		case "q":
			return m, tea.Quit
		case "esc":
			// escape closes pages; the table is its floor, never an exit.
			return m, nil
		case "f":
			m.selectView(selectedView{kind: viewNeedsInput})
			return m, nil
		case "up", "k":
			m.move(-1)
			return m, nil
		case "down", "j":
			m.move(1)
			return m, nil
		case "left", "h":
			m.step(-1)
			return m, nil
		case "right", "l":
			m.step(1)
			return m, nil
		case "m":
			m.picker = 0
			m.page = "machine-picker"
			for index, option := range m.pickerOptions() {
				if option == m.machine {
					m.picker = index
					break
				}
			}
			return m, nil
		case "n":
			peer := m.creationPeer()
			if peer == nil || !peer.OK || !m.scopeReady {
				m.inform("target unavailable; refresh before creating")
				if peer != nil {
					m.inform(peer.Label + " unavailable")
				}
				return m, nil
			}
			m.inform("opening terminal on " + peer.Label + "…")
			return m, m.execute(fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchTerminal, Machine: peer.Label, CWD: "~", Group: m.view.label})
		case "N":
			peer := m.creationPeer()
			if peer == nil {
				m.inform("target unavailable; refresh before creating")
				return m, nil
			}
			m.page, m.field = "create", 0
			m.clearDirectorySearch()
			m.form = [5]string{peer.Label, "terminal", "", "", m.view.label.String()}
			m.groupSelection.prefill(m.form[4], m.groupChoices(m.form[4]))
			m.inform("")
			return m, nil
		}
		row := m.selectedRow()
		if row == nil {
			return m, nil
		}
		if key == "space" || key == " " {
			m.page, m.offset = "details", 0
			m.pageRow = *row
			return m, nil
		}
		if !row.available {
			return m, nil
		}
		request := fleetclient.Request{Ref: row.session.Ref}
		// The rule names this captured target until the action completes.
		m.pendingName, m.pendingLabel = row.session.Name, row.label
		switch key {
		case "T":
			if row.session.Connection != nil {
				m.inform("new terminal on " + row.label + ": use n or N")
				return m, nil
			}
			request.Operation = "shell"
			m.inform("opening new shell on " + row.label + "…")
			return m, m.execute(request)
		case "enter":
			request.Operation = "enter"
			return m, m.enter(request)
		case "x":
			request.Operation = "close"
		case "s":
			request.Operation = "stop"
		}
		if request.Operation == "stop" || request.Operation == "close" {
			m.pending = request
			m.page = "confirm"
		}
	}
	return m, nil
}

// A notice's tone travels with its text: failures read as ember, all else plain.
func (m *model) inform(text string) { m.notice, m.noticeFailure = text, false }
func (m *model) fail(text string)   { m.notice, m.noticeFailure = text, true }

func (m *model) scopedPeers() []fleetclient.Peer {
	if m.machine == "" {
		return m.peers
	}
	for _, peer := range m.peers {
		if peer.Label == m.machine {
			return []fleetclient.Peer{peer}
		}
	}
	return nil
}
func (m *model) invalidatePendingPeer() {
	target, _ := fleetclient.DecodeReference(m.pending.Ref)
	for index := range m.peers {
		if m.peers[index].Machine == target.Machine {
			m.peers[index].OK = false
		}
	}
	m.rebuild()
}
func (m *model) pickerOptions() []string {
	options := []string{"all machines"}
	for _, machine := range m.client.Machines() {
		options = append(options, machine.Label)
	}
	return options
}
func (m *model) editPicker(key string) tea.Cmd {
	options := m.pickerOptions()
	m.picker = min(m.picker, len(options)-1)
	switch key {
	case "esc", "q":
		m.page = ""
	case "up", "k":
		m.picker = max(0, m.picker-1)
	case "down", "j":
		m.picker = min(len(options)-1, m.picker+1)
	case "enter":
		selected := ""
		if m.picker > 0 {
			selected = options[m.picker]
		}
		m.page = ""
		if selected == m.machine {
			return nil
		}
		m.machine, m.scopeReady = selected, false
		if m.refreshing {
			m.refreshAfterAction = true
		}
		m.rebuildForFilter()
		return m.refresh()
	}
	return nil
}
func (m *model) selectedRow() *listedRow {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

// singleLine replaces controls, invisible format characters such as bidi
// overrides, and line/paragraph separators, so observed text cannot reorder or
// break what the screen shows.
func singleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, text)
}

type attachment struct {
	ctx             context.Context
	client          *fleetclient.Client
	request         fleetclient.Request
	input, output   *os.File
	store           *fleetclient.NotificationStore
	snapshot        fleetclient.NotificationSnapshot
	notificationErr error
}

func (a *attachment) Run() error {
	return terminalclient.Run(a.ctx, a.client, a.request, a.input, a.output, a.store, func(snapshot fleetclient.NotificationSnapshot, err error) {
		a.notificationErr = err
		if err == nil {
			a.snapshot = snapshot
		}
	})
}
func (a *attachment) SetStdin(io.Reader)  {}
func (a *attachment) SetStdout(io.Writer) {}
func (a *attachment) SetStderr(io.Writer) {}
