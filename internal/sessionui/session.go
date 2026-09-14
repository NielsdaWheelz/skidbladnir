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
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"github.com/charmbracelet/x/ansi"
)

type listedRow struct {
	label, machine string
	session        fleetclient.Session
	available      bool
}
type inventoryMsg struct {
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
	ctx                           context.Context
	client                        *fleetclient.Client
	input, output                 *os.File
	peers                         []fleetclient.Peer
	rows                          []listedRow
	cursor                        int
	initialized, refreshing, busy bool
	refreshAfterAction            bool
	width, height                 int
	notice, page                  string
	text                          []string
	offset                        int
	pending                       fleetclient.Request
	pendingLabel, pendingName     string
	form                          [4]string
	field                         int
}

func Run(ctx context.Context, client *fleetclient.Client, input, output *os.File) error {
	_, err := tea.NewProgram(newModel(ctx, client, input, output), tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
}
func newModel(ctx context.Context, client *fleetclient.Client, input, output *os.File) *model {
	return &model{ctx: ctx, client: client, input: input, output: output, cursor: -1, width: 100, height: 30, refreshing: true}
}
func (m *model) Init() tea.Cmd { return tea.Batch(m.fetch(), tick()) }
func tick() tea.Cmd            { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m *model) fetch() tea.Cmd {
	return func() tea.Msg {
		result := m.client.Execute(m.ctx, fleetclient.Request{Operation: "list"})
		message := inventoryMsg{failure: result.Error}
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
	m.busy = true
	return func() tea.Msg {
		return actionMsg{operation: request.Operation, result: m.client.Execute(m.ctx, request)}
	}
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
	case tickMsg:
		return m, tea.Batch(m.refresh(), tick())
	case inventoryMsg:
		m.refreshing = false
		if m.refreshAfterAction {
			m.refreshAfterAction = false
			return m, m.refresh()
		}
		if message.failure != nil {
			for i := range m.rows {
				m.rows[i].available = false
			}
			for i := range m.peers {
				m.peers[i].OK = false
			}
			m.notice = "inventory unavailable: " + message.failure.Code
			return m, nil
		}
		var selected fleetclient.Reference
		if row := m.selectedRow(); row != nil {
			selected, _ = fleetclient.DecodeReference(row.session.Ref)
		}
		previous := m.rows
		m.rows = nil
		m.peers = message.value.Peers
		for _, peer := range m.peers {
			if !peer.OK {
				for _, row := range previous {
					if row.machine == peer.Machine {
						row.available = false
						m.rows = append(m.rows, row)
					}
				}
				continue
			}
			for _, session := range peer.Sessions {
				m.rows = append(m.rows, listedRow{peer.Label, peer.Machine, session, true})
			}
		}
		m.cursor = -1
		for index, row := range m.rows {
			ref, _ := fleetclient.DecodeReference(row.session.Ref)
			if ref.SessionEqual(selected) {
				m.cursor = index
				break
			}
		}
		if !m.initialized && len(m.rows) > 0 {
			m.cursor = 0
		}
		m.initialized = true
	case actionMsg:
		m.busy = false
		if m.refreshing {
			m.refreshAfterAction = true
		}
		if !message.result.OK {
			m.notice = message.result.Error.Code + " (" + message.result.Error.Dispatch + "); not repeated"
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
		case "start":
			var value fleetclient.ObservedSession
			if json.Unmarshal(message.result.Value, &value) != nil {
				m.notice = "invalid creation response"
				break
			}
			m.page = ""
			m.rows = append(m.rows, listedRow{value.Label, value.Machine, value.Session, true})
			m.cursor = len(m.rows) - 1
			m.notice = "created " + value.Session.Name + "; enter to handle startup"
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
		if m.page == "create" && m.field >= 2 && !m.busy {
			m.form[m.field] += singleLine(message.Content)
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
		if m.page == "confirm" {
			switch key {
			case "y", "enter":
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
		case "n":
			for _, peer := range m.peers {
				if peer.OK && len(peer.Profiles) > 0 {
					m.page = "create"
					m.field = 0
					m.form = [4]string{peer.Label, peer.Profiles[0].Key, "", "~"}
					m.notice = ""
					return m, nil
				}
			}
			m.notice = "no available host profiles"
			return m, nil
		}
		row := m.selectedRow()
		if row == nil || !row.available {
			return m, nil
		}
		request := fleetclient.Request{Ref: row.session.Ref}
		switch key {
		case "enter":
			request.Operation = "enter"
			return m, tea.Exec(&attachment{ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output}, func(err error) tea.Msg { return attachedMsg{err: err} })
		case "space", " ":
			value := row.session
			details := fmt.Sprintf("session: %s\nmachine: %s\nmachine id: %s\ndirectory: %s\ncommand: %s\nattached clients: %d\n", value.Name, row.label, row.machine, value.CWD, value.ActiveCommand, value.AttachedClients)
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

func (m *model) selectedRow() *listedRow {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

func (m *model) editForm(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.page = ""
	case "tab":
		m.field = (m.field + 1) % 4
	case "shift+tab":
		m.field = (m.field + 3) % 4
	case "enter":
		if m.field < 3 {
			m.field++
			return nil
		}
		request := fleetclient.Request{Operation: "start", Machine: m.form[0], Profile: m.form[1], Name: m.form[2], CWD: m.form[3]}
		if !request.Valid() {
			m.notice = "name, machine, profile, and directory are required"
			return nil
		}
		return m.execute(request)
	case "left", "right":
		if m.field > 1 {
			return nil
		}
		options := []string{}
		for _, peer := range m.peers {
			if !peer.OK {
				continue
			}
			if m.field == 0 && len(peer.Profiles) > 0 {
				options = append(options, peer.Label)
			}
			if m.field == 1 && peer.Label == m.form[0] {
				for _, profile := range peer.Profiles {
					options = append(options, profile.Key)
				}
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
			for _, peer := range m.peers {
				if peer.Label == m.form[0] && len(peer.Profiles) > 0 {
					m.form[1] = peer.Profiles[0].Key
					break
				}
			}
		}
	case "backspace":
		if m.field >= 2 && m.form[m.field] != "" {
			_, size := utf8.DecodeLastRuneInString(m.form[m.field])
			m.form[m.field] = m.form[m.field][:len(m.form[m.field])-size]
		}
	default:
		if m.field >= 2 {
			m.form[m.field] += singleLine(key.Text)
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
	case "create":
		for i, label := range []string{"machine", "profile", "name", "directory"} {
			marker := "  "
			if i == m.field {
				marker = "> "
			}
			fmt.Fprintf(&body, "%s%s: %s\n", marker, label, m.form[i])
		}
		fmt.Fprintln(&body, "\nleft/right choose machine/profile · tab/enter next\nenter on directory creates · escape cancels")
	case "output", "details":
		lines := m.detailLines()
		for i := m.offset; i < min(len(lines), m.offset+max(1, m.height-7)); i++ {
			fmt.Fprintln(&body, lines[i])
		}
		fmt.Fprintln(&body, "\nup/down/page-up/page-down scroll · q/escape returns")
	default:
		fmt.Fprintf(&body, "   %s %s %s %s directory\n", cell("machine", 8), cell("session", 16), cell("provider/profile", 18), cell("state · source", 18))
		offline := 0
		for _, peer := range m.peers {
			if !peer.OK {
				offline++
			}
		}
		visible := max(1, m.height-9-offline)
		start := max(0, m.cursor-visible+1)
		for i := start; i < min(len(m.rows), start+visible); i++ {
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
		for _, peer := range m.peers {
			if !peer.OK {
				fmt.Fprintf(&body, "%s: unavailable\n", peer.Label)
			}
		}
		if len(m.rows) == 0 {
			fmt.Fprintln(&body, "no sessions")
		}
		fmt.Fprintln(&body, "\n↑↓/j/k select · enter attach · space info · r read · i interrupt\ns stop · x kill · n new · ctrl-r refresh · q quit")
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
