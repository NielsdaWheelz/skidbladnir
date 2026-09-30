// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"github.com/muesli/cancelreader"
)

type listedRow struct {
	label, machine string
	session        fleetclient.Session
	available      bool
}
type inventoryMsg struct {
	machine string
	value   fleetclient.Inventory
	failure *fleetclient.Failure
}
type tickMsg struct{}
type actionMsg struct {
	operation string
	result    fleetclient.Result
}
type searchMsg struct {
	machine  string
	revision int
	result   fleetclient.Result
}
type attachedMsg struct{ err error }
type outputPresentedMsg struct{ err error }
type model struct {
	ctx                              context.Context
	client                           *fleetclient.Client
	input, output                    *os.File
	peers                            []fleetclient.Peer
	rows                             []listedRow
	cursor                           int
	refreshing, busy                 bool
	refreshAfterAction               bool
	width, height                    int
	notice, page                     string
	noticeFailure                    bool
	facts                            [][2]string
	offset                           int
	pending                          fleetclient.Request
	pendingLabel, pendingName        string
	form                             [5]string
	field                            int
	machine                          string
	groupFilter                      group.Filter
	scopeReady                       bool
	picker                           int
	groupDraft                       string
	groupChecking, groupAcknowledged bool
	groupFailure                     *fleetclient.Failure
	agentsView                       bool
	top                              int
	pageName, pageMachine            string
	searchRevision                   int
	searching                        bool
	searchDirectories                []string
	searchCursor                     int
	searchOmitted                    bool
	unreadStore                      *fleetclient.UnreadStore
	unreadSnapshot                   fleetclient.UnreadSnapshot
	outputAck                        func() error
	unreadFailed                     bool
	replyScans                       map[fleetclient.UnreadKey]*replyScan
	replyBusy                        map[string]bool
	replyNext                        map[string]int
	repliesUnavailable               map[fleetclient.UnreadKey]bool
}

func Run(ctx context.Context, client *fleetclient.Client, input, output *os.File) error {
	_, err := tea.NewProgram(newModel(ctx, client, input, output), tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
}
func newModel(ctx context.Context, client *fleetclient.Client, input, output *os.File) *model {
	peers := []fleetclient.Peer{}
	for _, machine := range client.Machines() {
		peers = append(peers, fleetclient.Peer{Label: machine.Label, Machine: machine.Handle})
	}
	store, storeErr := fleetclient.DefaultUnreadStore()
	return &model{unreadStore: store, unreadFailed: storeErr != nil, replyScans: map[fleetclient.UnreadKey]*replyScan{}, replyBusy: map[string]bool{}, replyNext: map[string]int{}, repliesUnavailable: map[fleetclient.UnreadKey]bool{}, ctx: ctx, client: client, input: input, output: output, peers: peers, cursor: -1, width: 100, height: 30, refreshing: true, agentsView: true}
}
func (m *model) Init() tea.Cmd { return tea.Batch(m.fetch(), tick()) }
func tick() tea.Cmd            { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m *model) fetch() tea.Cmd {
	machine := m.machine
	return func() tea.Msg {
		result := m.client.Execute(m.ctx, fleetclient.Request{Operation: "list", Machine: machine})
		message := inventoryMsg{machine: machine, failure: result.Error}
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
		return actionMsg{operation: request.Operation, result: m.client.Execute(m.ctx, request)}
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
					if !received.OK {
						received.Sessions = previous.Sessions
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

		m.rebuild()
		if m.page == "group-edit" || m.groupChecking {
			target := m.pendingRow()
			if target == nil && m.pendingPeerAvailable() {
				m.page = ""
				m.groupChecking = false
				m.inform("session unavailable; editor closed")
			} else if target != nil && target.available && m.groupChecking {
				m.groupChecking = false
				if m.groupAcknowledged {
					m.page = ""
					m.inform("group assigned")
					if m.pending.Group.IsUnassigned() {
						m.inform("group cleared")
					}
				}
			}
		}
		if m.unreadStore != nil {
			snapshot, err := m.unreadStore.Sync(m.peers, m.client.Machines())
			if err != nil {
				m.unreadFailed = true
			} else {
				m.unreadFailed = false
				m.unreadSnapshot = snapshot
			}
		}
		return m, m.recoverReplies()
	case repliesMsg:
		m.receiveReplies(message)
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
			failureText := message.result.Error.Code
			if message.result.Error.Dispatch == "unknown" {
				failureText = "could not confirm the request. inspect the conversation before trying again."
			} else if message.result.Error.Code == "AgentTargetStale" || message.result.Error.Code == "SessionIdentityMismatch" {
				failureText = "the session changed. refresh and try again."
			} else if message.result.Error.Code == "AgentUnavailable" {
				failureText = "this action is unavailable for this session."
			}

			m.fail(failureText)
			failure := message.result.Error
			if message.operation == "group" && (failure.Dispatch == "unknown" || failure.Code == "SessionNotFound" || failure.Code == "SessionIdentityMismatch" || failure.Code == "InternalError" || failure.Code == "Unauthenticated" || failure.Code == "MachineIdentityMismatch") {
				m.groupChecking = true
				m.groupAcknowledged = false
				if failure.Dispatch == "unknown" {
					m.groupFailure = failure
				}
				m.invalidatePendingPeer()
				return m, m.refresh()
			}
			return m, nil
		}
		switch message.operation {
		case "read":
			read := message.result.Value.(fleetclient.ReadResult)
			acknowledgement := m.outputAck
			m.outputAck = nil
			return m, tea.Exec(&replyPresentation{ctx: m.ctx, read: read, input: m.input, output: m.output, acknowledge: acknowledgement}, func(err error) tea.Msg { return outputPresentedMsg{err: err} })

		case "start", "shell":
			value := message.result.Value.(fleetclient.ObservedSession)
			// Confirmed creation reveals the new session in its group.
			m.page, m.agentsView = "", false
			if m.machine != "" && m.machine != value.Label {
				m.machine = value.Label
				m.scopeReady = false
			}
			if !m.groupFilter.Matches(value.Session.Group) {
				m.groupFilter = filterFor(value.Session.Group)
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
			return m, tea.Exec(&attachment{ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output}, func(err error) tea.Msg { return attachedMsg{err: err} })
		case "group":
			m.groupChecking = true
			m.groupAcknowledged = true
			m.invalidatePendingPeer()
			m.inform("group assigned; checking inventory")
			if m.pending.Group.IsUnassigned() {
				m.inform("group cleared; checking inventory")
			}

		case "close":
			if _, ok := message.result.Value.(fleetclient.TerminalCloseResult); ok {
				m.inform("terminal closed; pending input may remain")
				break
			}
			value := message.result.Value.(fleetclient.CloseResult)
			if value.Reason == "stale" {
				m.fail("terminal left open because the session changed.")
			} else if value.Terminal == "closed" && value.Agent == "unconfirmed" {
				m.fail("terminal closed; conversation stop unconfirmed.")
			} else {
				m.inform("current work: " + value.Agent + "; terminal: " + value.Terminal + "; pending input may remain; saved history is retained.")
			}
		case "stop":
			value := message.result.Value.(fleetclient.WriteResult)
			if value.Outcome == "unknown" {
				m.fail("could not confirm the request. inspect the conversation before trying again.")
			} else if value.Method == "terminal" {
				m.inform("keys sent; agent state not confirmed.")
			} else {
				m.inform("current work: " + value.Outcome + "; pending input may remain")
			}

		default:
			value := message.result.Value.(fleetclient.WriteResult)
			if value.Outcome == "unknown" {
				m.fail("could not confirm the request. inspect the conversation before trying again.")
			} else {
				m.inform("keys sent; agent state not confirmed.")
			}
		}
		return m, m.refresh()
	case searchMsg:
		if m.page != "search" || m.form[0] != message.machine || m.searchRevision != message.revision {
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
					m.inform("enter 1–8 search words")
				}
			}
			return m, nil
		}
		value := message.result.Value.(fleetclient.DirectorySearchResult)
		m.searchDirectories, m.searchOmitted, m.searchCursor = value.Directories, value.Omitted, 0
		m.inform("")
		if len(value.Directories) == 0 {
			m.inform("no matching directories")
		} else if value.Omitted {
			m.inform("some directories are not shown")
		}
		return m, nil
	case outputPresentedMsg:
		if message.err != nil {
			m.fail(message.err.Error())
		}
		return m, nil
	case attachedMsg:
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
			if m.page == "group-edit" && !m.groupChecking {
				m.groupDraft += message.Content
			}
			if m.page == "create" && m.field >= 2 {
				if m.field == 4 {
					m.form[m.field] += message.Content
				} else {
					m.form[m.field] += singleLine(message.Content)
				}
			}
		}
	case tea.KeyPressMsg:
		key := message.String()
		if m.busy {
			m.inform("operation in flight; delivery will be reported")
			return m, nil
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
		if m.page == "group-edit" {
			return m, m.editGroup(message)
		}
		if m.page == "confirm" {
			switch key {
			case "y", "enter":
				current := m.pendingRow()
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
		if m.page == "search" {
			switch key {
			case "esc", "q":
				m.page = "create"
			case "up", "k":
				m.searchCursor = max(0, m.searchCursor-1)
			case "down", "j":
				m.searchCursor = min(max(0, len(m.searchDirectories)-1), m.searchCursor+1)
			case "enter":
				if !m.searching && len(m.searchDirectories) > 0 {
					m.form[3] = m.searchDirectories[m.searchCursor]
					m.field, m.page = 3, "create"
					m.inform("")
				}
			}
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
		switch key {
		case "q", "esc":
			return m, tea.Quit
		case "a":
			m.showAgents()
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
			return m, m.execute(fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchTerminal, Machine: peer.Label, CWD: "~", Group: m.groupFilter.Label()})
		case "N":
			peer := m.creationPeer()
			if peer == nil {
				m.inform("target unavailable; refresh before creating")
				return m, nil
			}
			m.page, m.field = "create", 0
			m.form = [5]string{peer.Label, "terminal", "", "~", m.groupFilter.Label().String()}
			m.inform("")
			return m, nil
		}
		row := m.selectedRow()
		if row == nil {
			return m, nil
		}
		if key == "space" || key == " " {
			m.page, m.offset = "details", 0
			m.pageName, m.pageMachine = row.session.Name, row.label
			m.facts = m.details(row)
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
			m.inform("creating terminal here")
			return m, m.execute(request)
		case "e":
			request.Operation = "group"
			m.pending = request
			m.groupDraft = row.session.Group.String()
			m.groupChecking, m.groupAcknowledged = false, false
			m.groupFailure = nil
			m.page = "group-edit"
			m.inform("")
			return m, nil
		case "enter":
			request.Operation = "enter"
			return m, tea.Exec(&attachment{ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output}, func(err error) tea.Msg { return attachedMsg{err: err} })
		case "x":
			request.Operation = "close"
			request.TerminalOnly = true
		case "s":
			if row.session.Conversation == nil || row.session.Conversation.Methods.Stop != "native" {
				return m, nil
			}
			request.Operation = "stop"
		case "c":
			if row.session.Conversation == nil || row.session.Conversation.Methods.Stop != "native" {
				return m, nil
			}
			request.Operation = "close"
		case "r":
			ref, readable := m.replyReference(*row)
			if !readable {
				return m, nil
			}
			request.Operation, request.Ref = "read", ref.Encode()
			m.outputAck = m.acknowledgement(request)
			return m, m.execute(request)
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

func filterFor(label group.Label) group.Filter {
	if label.IsUnassigned() {
		return group.UnassignedFilter()
	}
	filter, _ := group.NamedFilter(label)
	return filter
}
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
func (m *model) pendingRow() *listedRow {
	target, _ := fleetclient.DecodeReference(m.pending.Ref)
	for _, peer := range m.peers {
		for _, session := range peer.Sessions {
			ref, _ := fleetclient.DecodeReference(session.Ref)
			if ref.SessionEqual(target) {
				return &listedRow{peer.Label, peer.Machine, session, peer.OK}
			}
		}
	}
	return nil
}
func (m *model) pendingPeerAvailable() bool {
	target, _ := fleetclient.DecodeReference(m.pending.Ref)
	for _, peer := range m.peers {
		if peer.Machine == target.Machine {
			return peer.OK
		}
	}
	return false
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
func (m *model) nextGroupDraft(draft string, previous bool) string {
	options := []string{""}
	for _, label := range fleetclient.ObservedGroups(m.scopedPeers()) {
		options = append(options, label.String())
	}
	index := -1
	canonical, err := group.ParseDraft(draft)
	if err == nil {
		for i, option := range options {
			if canonical.String() == option {
				index = i
				break
			}
		}
	}
	if previous {
		index = (max(0, index) + len(options) - 1) % len(options)
	} else {
		index = (index + 1) % len(options)
	}
	return options[index]
}
func (m *model) editGroup(key tea.KeyPressMsg) tea.Cmd {
	if key.String() == "esc" {
		m.page = ""
		m.groupChecking = false
		m.groupAcknowledged = false
		return nil
	}
	if m.groupChecking {
		return nil
	}
	switch key.String() {
	case "enter", "ctrl+s":
		label, err := group.ParseDraft(m.groupDraft)
		if err != nil {
			m.inform(group.ErrInvalid.Error())
			return nil
		}
		current := m.pendingRow()
		if current == nil || !current.available {
			m.inform("session unavailable; refresh before saving")
			return nil
		}
		if current.session.Group == label {
			return nil
		}
		m.pending.Group = label
		m.groupFailure = nil
		return m.execute(m.pending)
	case "ctrl+u":
		m.groupDraft = ""
	case "left", "right":
		m.groupDraft = m.nextGroupDraft(m.groupDraft, key.String() == "left")
	case "backspace":
		if m.groupDraft != "" {
			_, size := utf8.DecodeLastRuneInString(m.groupDraft)
			m.groupDraft = m.groupDraft[:len(m.groupDraft)-size]
		}
	default:
		m.groupDraft += key.Text
	}
	return nil
}
func (m *model) selectedRow() *listedRow {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

func (m *model) createAvailable() bool {
	if !m.scopeReady {
		return false
	}
	for _, peer := range m.peers {
		if peer.Label != m.form[0] || !peer.OK {
			continue
		}
		if m.form[1] == "terminal" {
			return true
		}
		for _, profile := range peer.Profiles {
			if profile.Key == m.form[1] {
				return true
			}
		}
	}
	return false
}

func (m *model) editForm(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.page = ""
	case "tab":
		m.field = (m.field + 1) % 5
	case "shift+tab":
		m.field = (m.field + 4) % 5
	case "enter":
		if m.field == 3 && (m.form[3] == "z" || strings.HasPrefix(m.form[3], "z ")) {
			terms := strings.Fields(strings.TrimPrefix(m.form[3], "z"))
			if len(terms) == 0 || len(terms) > 8 {
				m.inform("enter 1–8 search words")
				return nil
			}
			m.searchRevision++
			m.searching, m.searchDirectories, m.searchCursor, m.searchOmitted = true, nil, 0, false
			m.page = "search"
			m.inform("searching…")
			machine, revision := m.form[0], m.searchRevision
			return func() tea.Msg {
				return searchMsg{machine: machine, revision: revision, result: m.client.SearchDirectories(m.ctx, machine, terms)}
			}
		}
		if m.field < 4 {
			m.field++
			return nil
		}
		if !m.createAvailable() {
			m.inform("host unavailable; refresh before creating")
			return nil
		}
		label, err := group.ParseDraft(m.form[4])
		if err != nil {
			m.inform(group.ErrInvalid.Error())
			return nil
		}
		request := fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchAgent, Machine: m.form[0], Profile: m.form[1], Name: m.form[2], CWD: m.form[3], Group: label}
		if m.form[1] == "terminal" {
			request.Kind, request.Profile = fleetclient.LaunchTerminal, ""
		}
		if !request.Valid() {
			m.inform("machine, launch, and directory are required")
			return nil
		}
		return m.execute(request)
	case "left", "right":
		if m.field == 4 {
			m.form[4] = m.nextGroupDraft(m.form[4], key.String() == "left")
			return nil
		}
		if m.field > 1 {
			return nil
		}
		options := []string{}
		for _, peer := range m.peers {
			if !peer.OK {
				continue
			}
			if m.field == 0 {
				options = append(options, peer.Label)
			}
			if m.field == 1 && peer.Label == m.form[0] {
				for _, profile := range peer.Profiles {
					options = append(options, profile.Key)
				}
				options = append(options, "terminal")
			}
		}
		if len(options) == 0 {
			return nil
		}
		index := 0
		for i, value := range options {
			if value == m.form[m.field] {
				index = i
				break
			}
		}
		if key.String() == "left" {
			index = (index + len(options) - 1) % len(options)
		} else {
			index = (index + 1) % len(options)
		}
		m.form[m.field] = options[index]
		if m.field == 0 {
			m.form[3] = "~"
			if m.form[1] != "terminal" {
				m.form[1] = "terminal"
				for _, peer := range m.peers {
					if peer.Label == m.form[0] && len(peer.Profiles) != 0 {
						m.form[1] = peer.Profiles[0].Key
						break
					}
				}
			}
		}
	case "ctrl+u":
		if m.field == 4 {
			m.form[4] = ""
		}
	case "backspace":
		if m.field >= 2 && m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	default:
		if m.field >= 2 {
			if m.field == 4 {
				m.form[m.field] += key.Text
			} else {
				m.form[m.field] += singleLine(key.Text)
			}
		}
	}
	return nil
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
	ctx           context.Context
	client        *fleetclient.Client
	request       fleetclient.Request
	input, output *os.File
}

func (a *attachment) Run() error {
	return terminalclient.Run(a.ctx, a.client, a.request, a.input, a.output)
}
func (a *attachment) SetStdin(io.Reader)  {}
func (a *attachment) SetStdout(io.Writer) {}
func (a *attachment) SetStderr(io.Writer) {}

type replyPresentation struct {
	ctx         context.Context
	read        fleetclient.ReadResult
	input       io.Reader
	output      io.Writer
	acknowledge func() error
}

func (presentation *replyPresentation) SetStdin(input io.Reader)   { presentation.input = input }
func (presentation *replyPresentation) SetStdout(output io.Writer) { presentation.output = output }
func (presentation *replyPresentation) SetStderr(io.Writer)        {}
func (presentation *replyPresentation) Run() error {
	safeLines := strings.Split(presentation.read.Text, "\n")
	for index, line := range safeLines {
		safeLines[index] = singleLine(line)
	}
	if _, err := fmt.Fprintf(presentation.output, "conversation %s\n%s · truncated: %t\n\n%s\n\npress enter to return\n", presentation.read.Observation.Binding.Conversation.ConversationID, presentation.read.Scope, presentation.read.Truncated, strings.Join(safeLines, "\n")); err != nil {
		return err
	}
	var acknowledgementErr error
	if presentation.acknowledge != nil {
		acknowledgementErr = presentation.acknowledge()
	}
	reader, err := cancelreader.NewReader(presentation.input)
	if err != nil {
		return err
	}
	defer reader.Close()
	stop := context.AfterFunc(presentation.ctx, func() { reader.Cancel() })
	defer stop()
	buffer := make([]byte, 1)
	for {
		_, err := reader.Read(buffer)
		if err != nil {
			return err
		}
		if buffer[0] == '\n' || buffer[0] == '\r' {
			break
		}
	}
	return acknowledgementErr
}
