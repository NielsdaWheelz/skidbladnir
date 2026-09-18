// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
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
type attachedMsg struct{ err error }
type model struct {
	ctx                                       context.Context
	client                                    *fleetclient.Client
	input, output                             *os.File
	peers                                     []fleetclient.Peer
	rows                                      []listedRow
	cursor                                    int
	refreshing, busy                          bool
	refreshAfterAction                        bool
	width, height                             int
	notice, page                              string
	text                                      []string
	offset                                    int
	pending                                   fleetclient.Request
	pendingLabel, pendingName                 string
	form                                      [5]string
	field                                     int
	machine                                   string
	spaceFilter                               space.Filter
	scopeReady                                bool
	picker                                    int
	spaceDraft                                string
	spaceChecking, spaceAcknowledged          bool
	spaceFailure                              *fleetclient.Failure
	focus                                     region
	agents                                    []listedRow
	spacesTop, agentsTop, tabsTop             int
	outputName, outputMachine, outputCoverage string
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
	return &model{ctx: ctx, client: client, input: input, output: output, peers: peers, cursor: -1, width: 100, height: 30, refreshing: true}
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
			m.notice = "inventory unavailable: " + message.failure.Code
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
		if m.page == "space-edit" || m.spaceChecking {
			target := m.pendingRow()
			if target == nil && m.pendingPeerAvailable() {
				m.page = ""
				m.spaceChecking = false
				m.notice = "session unavailable; editor closed"
			} else if target != nil && target.available && m.spaceChecking {
				m.spaceChecking = false
				if m.spaceAcknowledged {
					m.page = ""
					m.notice = "space assigned"
					if m.pending.Space.IsUnassigned() {
						m.notice = "space cleared"
					}
				}
			}
		}
	case actionMsg:
		if message.operation == "start" || message.operation == "shell" {
			if m.ctx.Err() != nil {
				return m, nil
			}
			// Keyboard navigation is blocked while busy, so this pending request
			// and page identify the only interaction allowed to adopt creation.
			if !m.busy || m.pending.Operation != message.operation || message.operation == "start" && m.page != "create" || message.operation == "shell" && m.page != "" {
				return m, m.refresh()
			}
		}
		m.busy = false
		if m.refreshing {
			m.refreshAfterAction = true
		}
		if !message.result.OK {
			m.notice = message.result.Error.Code + " (" + message.result.Error.Dispatch + "); not repeated"
			failure := message.result.Error
			if message.operation == "space" && (failure.Dispatch == "unknown" || failure.Code == "SessionNotFound" || failure.Code == "SessionIdentityMismatch" || failure.Code == "InternalError" || failure.Code == "Unauthenticated" || failure.Code == "MachineIdentityMismatch") {
				m.spaceChecking = true
				m.spaceAcknowledged = false
				if failure.Dispatch == "unknown" {
					m.spaceFailure = failure
				}
				m.invalidatePendingPeer()
				return m, m.refresh()
			}
			return m, nil
		}
		switch message.operation {
		case "read":
			var read fleetclient.ReadResult
			if json.Unmarshal(message.result.Value.(json.RawMessage), &read) != nil {
				m.notice = "invalid read response"
				break
			}
			m.page = "output"
			m.offset = 0
			m.outputName, m.outputMachine = m.pendingName, m.pendingLabel
			m.outputCoverage = fmt.Sprintf("%s · %s · truncated: %t", read.Source, read.Scope, read.Truncated)
			m.text = strings.Split(read.Text, "\n")
		case "start", "shell":
			value := message.result.Value.(fleetclient.ObservedSession)
			m.page = ""
			if m.machine != "" && m.machine != value.Label {
				m.machine = value.Label
				m.scopeReady = false
			}
			if !m.spaceFilter.Matches(value.Session.Space) {
				m.spaceFilter = filterFor(value.Session.Space)
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
			m.setFocus(tabs)
			m.notice = "created " + value.Session.Name + "; enter to attach"
			if message.operation == "shell" {
				request := fleetclient.Request{Operation: "enter", Ref: value.Session.Ref}
				return m, tea.Exec(&attachment{ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output}, func(err error) tea.Msg { return attachedMsg{err: err} })
			}
		case "space":
			m.spaceChecking = true
			m.spaceAcknowledged = true
			m.invalidatePendingPeer()
			m.notice = "space assigned; checking inventory"
			if m.pending.Space.IsUnassigned() {
				m.notice = "space cleared; checking inventory"
			}

		case "stop":
			var value fleetclient.StopResult
			if json.Unmarshal(message.result.Value.(json.RawMessage), &value) != nil {
				m.notice = "invalid stop response"
				break
			}
			m.notice = "agent halt: " + value.Agent + "; terminal: " + value.Terminal
		case "kill":
			m.notice = "terminal closed; shared work may continue"
		default:
			var value fleetclient.WriteResult
			if json.Unmarshal(message.result.Value.(json.RawMessage), &value) != nil {
				m.notice = "invalid control response"
				break
			}
			m.notice = value.Method + ": " + value.Outcome
		}
		return m, m.refresh()
	case attachedMsg:
		if m.refreshing {
			m.refreshAfterAction = true
		}
		if message.err != nil {
			m.notice = message.err.Error()
		} else {
			m.notice = "detached; work continues"
		}
		return m, m.refresh()
	case tea.PasteMsg:
		if !m.busy && m.width >= 80 && m.height >= 24 {
			if m.page == "space-edit" && !m.spaceChecking {
				m.spaceDraft += message.Content
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
			m.notice = "operation in flight; delivery will be reported"
			return m, nil
		}
		if m.width < 80 || m.height < 24 {
			cancel := key == "esc"
			switch m.page {
			case "", "machine-picker", "details", "output":
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
		if m.page == "space-edit" {
			return m, m.editSpace(message)
		}
		if m.page == "confirm" {
			switch key {
			case "y", "enter":
				current := m.pendingRow()
				if current == nil || !current.available || !m.scopeReady {
					m.notice = "session unavailable; refresh before confirming"
					return m, nil
				}
				request := m.pending
				m.page = ""
				m.notice = request.Operation + " in progress"
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
		if m.page != "" {
			switch key {
			case "q", "esc":
				m.page = ""
			case "up", "k":
				m.offset = max(0, m.offset-1)
			case "down", "j":
				m.offset = min(max(0, len(m.detailLines())-1), m.offset+1)
			case "pgup":
				m.offset = max(0, m.offset-m.pageCapacity())
			case "pgdown":
				m.offset = min(max(0, len(m.detailLines())-1), m.offset+m.pageCapacity())
			}
			return m, nil
		}
		switch key {
		case "q", "esc":
			return m, tea.Quit
		case "g":
			m.setFocus(spaces)
			return m, nil
		case "a":
			m.setFocus(agents)
			return m, nil
		case "t":
			m.setFocus(tabs)
			return m, nil
		case "tab":
			m.setFocus((m.focus + 1) % 3)
			return m, nil
		case "shift+tab":
			m.setFocus((m.focus + 2) % 3)
			return m, nil
		case "up", "k", "down", "j", "left", "h", "right", "l":
			m.move(key)
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
			for _, peer := range m.peers {
				if peer.OK && (m.machine == "" || m.machine == peer.Label) && m.scopeReady {
					m.page = "create"
					m.field = 0
					launch := "terminal"
					if len(peer.Profiles) != 0 {
						launch = peer.Profiles[0].Key
					}
					m.form = [5]string{peer.Label, launch, "", "~", m.spaceFilter.Label().String()}
					m.notice = ""
					return m, nil
				}
			}
			m.notice = "no available host"
			return m, nil
		}
		if m.focus == spaces {
			if key == "enter" {
				m.setFocus(tabs)
			}
			return m, nil
		}
		row := m.selectedRow()
		if row == nil || m.focus == agents && m.agentIndex() < 0 {
			return m, nil
		}
		if key == "space" || key == " " {
			m.page, m.offset = "details", 0
			m.text = strings.Split(m.details(row), "\n")
			return m, nil
		}
		if !row.available {
			return m, nil
		}
		request := fleetclient.Request{Ref: row.session.Ref}
		switch key {
		case "T":
			request.Operation = "shell"
			m.notice = "creating terminal here"
			return m, m.execute(request)
		case "e":
			request.Operation = "space"
			m.pending = request
			m.pendingLabel, m.pendingName = row.label, row.session.Name
			m.spaceDraft = row.session.Space.String()
			m.spaceChecking, m.spaceAcknowledged = false, false
			m.spaceFailure = nil
			m.page, m.notice = "space-edit", ""
			return m, nil
		case "enter":
			request.Operation = "enter"
			return m, tea.Exec(&attachment{ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output}, func(err error) tea.Msg { return attachedMsg{err: err} })
		case "x":
			request.Operation = "kill"
		case "s":
			if row.session.Agent == nil {
				return m, nil
			}
			request.Operation = "stop"
		case "i":
			if row.session.Agent == nil {
				return m, nil
			}
			request.Operation = "interrupt"
			return m, m.execute(request)
		case "r":
			if row.session.Agent == nil {
				return m, nil
			}
			request.Operation = "read"
			m.pendingName, m.pendingLabel = row.session.Name, row.label
			return m, m.execute(request)
		}
		if request.Operation == "stop" || request.Operation == "kill" {
			m.pending = request
			m.pendingName = row.session.Name
			m.pendingLabel = row.label
			m.page = "confirm"
		}
	}
	return m, nil
}

func filterFor(label space.Label) space.Filter {
	if label.IsUnassigned() {
		return space.UnassignedFilter()
	}
	filter, _ := space.NamedFilter(label)
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
func (m *model) nextSpaceDraft(draft string, previous bool) string {
	options := []string{""}
	for _, label := range fleetclient.ObservedSpaces(m.scopedPeers()) {
		options = append(options, label.String())
	}
	index := -1
	canonical, err := space.ParseDraft(draft)
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
func (m *model) editSpace(key tea.KeyPressMsg) tea.Cmd {
	if key.String() == "esc" {
		m.page = ""
		m.spaceChecking = false
		m.spaceAcknowledged = false
		return nil
	}
	if m.spaceChecking {
		return nil
	}
	switch key.String() {
	case "enter", "ctrl+s":
		label, err := space.ParseDraft(m.spaceDraft)
		if err != nil {
			m.notice = space.ErrInvalid.Error()
			return nil
		}
		current := m.pendingRow()
		if current == nil || !current.available {
			m.notice = "session unavailable; refresh before saving"
			return nil
		}
		if current.session.Space == label {
			return nil
		}
		m.pending.Space = label
		m.spaceFailure = nil
		return m.execute(m.pending)
	case "ctrl+u":
		m.spaceDraft = ""
	case "left", "right":
		m.spaceDraft = m.nextSpaceDraft(m.spaceDraft, key.String() == "left")
	case "backspace":
		if m.spaceDraft != "" {
			_, size := utf8.DecodeLastRuneInString(m.spaceDraft)
			m.spaceDraft = m.spaceDraft[:len(m.spaceDraft)-size]
		}
	default:
		m.spaceDraft += key.Text
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
		if m.field < 4 {
			m.field++
			return nil
		}
		if !m.createAvailable() {
			m.notice = "host unavailable; refresh before creating"
			return nil
		}
		label, err := space.ParseDraft(m.form[4])
		if err != nil {
			m.notice = space.ErrInvalid.Error()
			return nil
		}
		request := fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchAgent, Machine: m.form[0], Profile: m.form[1], Name: m.form[2], CWD: m.form[3], Space: label}
		if m.form[1] == "terminal" {
			request.Kind, request.Profile = fleetclient.LaunchTerminal, ""
		}
		if !request.Valid() {
			m.notice = "name, machine, launch, and directory are required"
			return nil
		}
		return m.execute(request)
	case "left", "right":
		if m.field == 4 {
			m.form[4] = m.nextSpaceDraft(m.form[4], key.String() == "left")
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

func singleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
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
