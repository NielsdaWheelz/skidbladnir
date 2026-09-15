// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"github.com/charmbracelet/x/ansi"
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
	ctx                              context.Context
	client                           *fleetclient.Client
	input, output                    *os.File
	peers                            []fleetclient.Peer
	rows                             []listedRow
	cursor                           int
	initialized, refreshing, busy    bool
	refreshAfterAction               bool
	width, height                    int
	notice, page                     string
	text                             []string
	offset                           int
	pending                          fleetclient.Request
	pendingLabel, pendingName        string
	form                             [5]string
	field                            int
	machine                          string
	spaceFilter                      space.Filter
	scopeReady                       bool
	picker                           int
	spaceDraft                       string
	spaceChecking, spaceAcknowledged bool
	spaceFailure                     *fleetclient.Failure
	top                              itemKey
	items                            []collectionItem
}
type collectionItem struct {
	row   int
	label space.Label
}
type itemKey struct {
	heading bool
	label   space.Label
	session fleetclient.Reference
}

func (m *model) itemKey(item collectionItem) itemKey {
	if item.row < 0 {
		return itemKey{heading: true, label: item.label}
	}
	ref, _ := fleetclient.DecodeReference(m.rows[item.row].session.Ref)
	return itemKey{session: ref}
}
func (key itemKey) equal(other itemKey) bool {
	if key.heading || other.heading {
		return key.heading == other.heading && key.label == other.label
	}
	return key.session.SessionEqual(other.session)
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
		if result.OK && json.Unmarshal(result.Value, &message.value) != nil {
			message.failure = &fleetclient.Failure{Code: "protocol_error", Dispatch: "not_sent"}
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
	defer m.fitViewport()
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
		m.initialized = true
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
			if json.Unmarshal(message.result.Value, &read) != nil {
				m.notice = "invalid read response"
				break
			}
			m.page = "output"
			m.offset = 0
			m.text = strings.Split(fmt.Sprintf("%s · %s · truncated: %t\n\n%s", read.Source, read.Scope, read.Truncated, read.Text), "\n")
		case "start", "shell":
			var value fleetclient.ObservedSession
			if json.Unmarshal(message.result.Value, &value) != nil {
				m.notice = "invalid creation response"
				break
			}
			m.page = ""
			if m.machine != "" && m.machine != value.Label {
				m.machine = value.Label
				m.scopeReady = false
			}
			if !m.spaceFilter.Matches(value.Session.Space) {
				m.spaceFilter = filterFor(value.Session.Space)
				m.top = itemKey{}
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
			if json.Unmarshal(message.result.Value, &value) != nil {
				m.notice = "invalid stop response"
				break
			}
			m.notice = "agent halt: " + value.Agent + "; terminal: " + value.Terminal
		case "kill":
			m.notice = "terminal closed; shared work may continue"
		default:
			var value fleetclient.WriteResult
			if json.Unmarshal(message.result.Value, &value) != nil {
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
		if !m.busy {
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
		if key == "ctrl+r" {
			return m, m.refresh()
		}
		if m.page == "machine-picker" || m.page == "space-picker" {
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
				m.offset = max(0, m.offset-max(1, m.height-5))
			case "pgdown":
				m.offset = min(max(0, len(m.detailLines())-1), m.offset+max(1, m.height-5))
			}
			return m, nil
		}
		switch key {
		case "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if len(m.rows) > 0 {
				m.cursor = max(0, m.cursor-1)
			}
			return m, nil
		case "down", "j":
			if len(m.rows) > 0 {
				m.cursor = min(len(m.rows)-1, m.cursor+1)
			}
			return m, nil
		case "g", "m":
			m.picker = 0
			m.page = "space-picker"
			if key == "m" {
				m.page = "machine-picker"
			}
			for index, option := range m.pickerOptions() {
				if option == m.pickerSelection() {
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
		row := m.selectedRow()
		if row == nil || !row.available {
			return m, nil
		}
		request := fleetclient.Request{Ref: row.session.Ref}
		switch key {
		case "t":
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
		case "space", " ":
			value := row.session
			details := fmt.Sprintf("session: %s\nmachine: %s\nmachine id: %s\ndirectory: %s\ncommand: %s\nattached clients: %d\n", value.Name, row.label, row.machine, value.CWD, value.ActiveCommand, value.AttachedClients)
			details += fleetclient.SpaceHeading(value.Space) + "\n"
			if value.Agent == nil {
				details += "agent: none (shell)\n"
			} else {
				a := value.Agent
				details += fmt.Sprintf("provider: %s\nprofile: %s\nstate: %s (%s)\nreason: %s\nread: %s; send: %s; interrupt: %s\n", a.Provider, a.Profile, a.Status.State, a.Status.Source, a.Status.Reason, a.Methods.Read, a.Methods.Send, a.Methods.Interrupt)
				if a.ProviderSession != nil {
					details += fmt.Sprintf("provider session: %s %s\n", a.ProviderSession.ID, a.ProviderSession.Name)
				}
			}
			if value.LaunchProfile != "" {
				details += "launch profile: " + value.LaunchProfile + "\n"
			}
			for _, peer := range m.peers {
				if peer.Machine == row.machine {
					details += "observed: " + peer.ObservedAt + "\n"
					break
				}
			}
			details += "reference: " + value.Ref
			m.page = "details"
			m.text = strings.Split(details, "\n")
			m.offset = 0
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
func (m *model) rebuild() {
	var selected fleetclient.Reference
	if row := m.selectedRow(); row != nil {
		selected, _ = fleetclient.DecodeReference(row.session.Ref)
	}
	previous := m.cursor
	m.rows, m.items = nil, nil
	for _, group := range fleetclient.Groups(m.scopedPeers(), m.spaceFilter) {
		m.items = append(m.items, collectionItem{row: -1, label: group.Space})
		for _, row := range group.Rows {
			m.items = append(m.items, collectionItem{row: len(m.rows)})
			m.rows = append(m.rows, listedRow{row.Label, row.Machine, row.Session, row.Available && m.scopeReady})
		}
	}
	m.cursor = -1
	for index, row := range m.rows {
		ref, _ := fleetclient.DecodeReference(row.session.Ref)
		if ref.SessionEqual(selected) {
			m.cursor = index
			return
		}
	}
	if len(m.rows) > 0 {
		m.cursor = min(max(0, previous), len(m.rows)-1)
	}
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
	if m.page == "machine-picker" {
		options := []string{"all machines"}
		for _, machine := range m.client.Machines() {
			options = append(options, machine.Label)
		}
		return options
	}
	options := []string{"all spaces", "unassigned"}
	for _, label := range m.pickerSpaces() {
		options = append(options, fleetclient.SpaceHeading(label))
	}
	return options
}
func (m *model) pickerSpaces() []space.Label {
	labels := fleetclient.ObservedSpaces(m.scopedPeers())
	selected := m.spaceFilter.Label()
	if m.spaceFilter.Kind() == space.FilterNamed {
		found := false
		for _, label := range labels {
			if label == selected {
				found = true
				break
			}
		}
		if !found {
			labels = append(labels, selected)
			slices.SortFunc(labels, space.Compare)
		}
	}
	return labels
}
func (m *model) pickerSelection() string {
	if m.page == "machine-picker" {
		if m.machine == "" {
			return "all machines"
		}
		return m.machine
	}
	return fleetclient.SpaceFilterHeading(m.spaceFilter)
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
		if m.page == "machine-picker" {
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
			m.rebuild()
			return m.refresh()
		}
		filter := space.Filter{}
		if m.picker == 1 {
			filter = space.UnassignedFilter()
		}
		if m.picker > 1 {
			filter = filterFor(m.pickerSpaces()[m.picker-2])
		}
		m.page = ""
		if filter != m.spaceFilter {
			m.spaceFilter = filter
			m.rebuild()
		}
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
func (m *model) viewport() (int, int) {
	offline := 0
	for _, peer := range m.scopedPeers() {
		if !peer.OK {
			offline++
		}
	}
	visible := max(1, m.height-11-offline)
	start, selected := 0, -1
	for index, item := range m.items {
		if m.itemKey(item).equal(m.top) {
			start = index
		}
		if item.row == m.cursor && item.row >= 0 {
			selected = index
		}
	}
	start = min(start, max(0, len(m.items)-visible))
	if selected >= 0 {
		if selected < start {
			start = selected
		}
		if selected >= start+visible {
			start = selected - visible + 1
		}
	}
	return start, min(len(m.items), start+visible)
}
func (m *model) fitViewport() {
	start, end := m.viewport()
	if start < end {
		m.top = m.itemKey(m.items[start])
	} else {
		m.top = itemKey{}
	}
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

func (m *model) View() tea.View {
	var body strings.Builder
	fmt.Fprintln(&body, "skid · shared sessions")
	if m.refreshing {
		fmt.Fprintln(&body, "refreshing…")
	} else {
		fmt.Fprintln(&body)
	}
	switch m.page {
	case "confirm":
		fmt.Fprintf(&body, "%s %s on %s?\n\n", m.pending.Operation, m.pendingName, m.pendingLabel)
		if m.pending.Operation == "stop" {
			fmt.Fprintln(&body, "attempt agent halt, then close this session. shared work may be affected.")
		} else {
			fmt.Fprintln(&body, "close this session. work shared through another session may survive.")
		}
		fmt.Fprintln(&body, "\ny/enter confirms · n/escape cancels")
	case "machine-picker", "space-picker":
		fmt.Fprintln(&body, m.pickerSelection())
		if m.page == "space-picker" {
			fmt.Fprintln(&body, "observed spaces")
		}
		options := m.pickerOptions()
		start := max(0, m.picker-max(1, m.height-9)+1)
		for index := start; index < min(len(options), start+max(1, m.height-9)); index++ {
			option := options[index]
			marker := "  "
			if index == m.picker {
				marker = "> "
			}
			fmt.Fprintln(&body, marker+option)
		}
		fmt.Fprintln(&body, "\n↑↓/j/k choose · enter selects · escape cancels")
	case "space-edit":
		if m.spaceFailure != nil {
			fmt.Fprintf(&body, "%s (unknown); not repeated\n", m.spaceFailure.Code)
		}
		fmt.Fprintf(&body, "space for %s on %s\n", m.pendingName, m.pendingLabel)
		current := m.pendingRow()
		if current != nil {
			fmt.Fprintln(&body, "current: "+fleetclient.SpaceHeading(current.session.Space))
		}
		fmt.Fprintln(&body, "> space: "+spaceDraftDisplay(m.spaceDraft))
		if m.spaceChecking {
			fmt.Fprintln(&body, "checking inventory; escape returns")
		} else {
			label, err := space.ParseDraft(m.spaceDraft)
			switch {
			case err != nil:
				fmt.Fprintln(&body, space.ErrInvalid.Error())
			case current == nil || !current.available:
				fmt.Fprintln(&body, "session unavailable; save disabled")
			case current.session.Space == label:
				fmt.Fprintln(&body, "unchanged; save disabled")
			default:
				fmt.Fprintln(&body, "enter/ctrl-s saves")
			}
			m.writeSpaceSuggestions(&body)
			fmt.Fprintln(&body, "left/right fill suggestion · ctrl-u unassigned · escape cancels")
		}
	case "create":
		for i, label := range []string{"machine", "launch", "name", "directory", "space"} {
			marker := "  "
			if i == m.field {
				marker = "> "
			}
			text := m.form[i]
			if i == 4 {
				text = spaceDraftDisplay(text)
			}
			fmt.Fprintf(&body, "%s%s: %s\n", marker, label, text)
		}
		m.writeSpaceSuggestions(&body)
		if !m.createAvailable() {
			fmt.Fprintln(&body, "host unavailable; create disabled")
		}
		if _, err := space.ParseDraft(m.form[4]); err != nil {
			fmt.Fprintln(&body, space.ErrInvalid.Error())
		}
		fmt.Fprintln(&body, "\nleft/right choose machine/launch/space · tab/enter next\nenter on space creates · ctrl-u unassigned · escape cancels")
	case "output", "details":
		lines := m.detailLines()
		for i := m.offset; i < min(len(lines), m.offset+max(1, m.height-7)); i++ {
			fmt.Fprintln(&body, lines[i])
		}
		fmt.Fprintln(&body, "\nup/down/page-up/page-down scroll · q/escape returns")
	default:
		fmt.Fprintf(&body, "   %s %s %s %s directory\n", cell("machine", 8), cell("session", 16), cell("provider/profile", 18), cell("state · source", 18))
		fmt.Fprintf(&body, "%s · %s\n", m.machineHeading(), fleetclient.SpaceFilterHeading(m.spaceFilter))
		start, end := m.viewport()
		for _, item := range m.items[start:end] {
			if item.row < 0 {
				fmt.Fprintln(&body, fleetclient.SpaceHeading(item.label))
				continue
			}
			i := item.row
			row := m.rows[i]
			marker := "  "
			if i == m.cursor {
				marker = "> "
			}
			provider, state := "shell", "—"
			if row.session.Agent != nil {
				a := row.session.Agent
				provider = a.Provider
				if a.Profile != "" {
					provider += "/" + a.Profile
				}
				state = a.Status.State + " · " + a.Status.Source
			}
			if !row.available {
				state = "unavailable"
			}
			fmt.Fprintf(&body, "%s %s %s %s %s %s\n", marker, cell(row.label, 8), cell(row.session.Name, 16), cell(provider, 18), cell(state, 18), ansi.TruncateLeft(singleLine(row.session.CWD), max(0, ansi.StringWidth(row.session.CWD)-max(1, m.width-67)+1), "…"))
		}
		for _, peer := range m.scopedPeers() {
			if !peer.OK {
				fmt.Fprintf(&body, "%s: unavailable\n", peer.Label)
			}
		}
		if len(m.rows) == 0 {
			partial := false
			for _, peer := range m.scopedPeers() {
				partial = partial || !peer.OK
			}
			switch {
			case !m.initialized || !m.scopeReady:
				fmt.Fprintln(&body, "checking inventory")
			case partial:
				fmt.Fprintln(&body, "no matching sessions in available inventory")
			default:
				fmt.Fprintln(&body, "no sessions in this view")
			}
		}
		fmt.Fprintln(&body, "\n↑↓/j/k select · enter attach · space info · r read · i interrupt\ns stop · x kill · n new · t terminal here · ctrl-r refresh · q quit\ng space filter · m machine filter · e edit space")
	}
	if m.notice != "" {
		fmt.Fprintln(&body, "\n"+m.notice)
	}
	lines := strings.Split(body.String(), "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(singleLine(line), max(1, m.width), "…")
	}
	if len(lines) > m.height {
		lines = lines[:max(1, m.height)]
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}

func (m *model) machineHeading() string {
	if m.machine == "" {
		return "all machines"
	}
	return "machine: " + m.machine
}
func spaceDraftDisplay(draft string) string {
	if _, err := space.ParseDraft(draft); err != nil {
		return strconv.QuoteToASCII(draft)
	}
	return draft
}

func (m *model) writeSpaceSuggestions(body *strings.Builder) {
	options := []string{}
	for _, label := range fleetclient.ObservedSpaces(m.scopedPeers()) {
		options = append(options, fleetclient.SpaceHeading(label))
	}
	fmt.Fprintln(body, "observed spaces: "+strings.Join(options, " · "))
}

func (m *model) detailLines() []string {
	lines := make([]string, len(m.text))
	for i, line := range m.text {
		lines[i] = ansi.Hardwrap(singleLine(line), max(1, m.width), true)
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

func singleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}
func cell(text string, width int) string {
	text = ansi.Truncate(singleLine(text), width, "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
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
