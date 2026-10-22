// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"io"
	"os"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type inventoryMsg struct {
	machine    string
	generation uint64
	result     fleetclient.Result
}
type contextMsg struct {
	machine    string
	generation uint64
	peer       fleetclient.Peer
}
type notificationMsg struct {
	generation uint64
	state      attention.State
	err        error
}
type fleetMsg struct{}
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
type attachedMsg struct {
	err             error
	notificationErr error
}
type model struct {
	ctx                       context.Context
	client                    *fleetclient.Client
	fleetPending              bool
	input, output             *os.File
	peers                     []fleetclient.Peer
	rows                      []fleetclient.Row
	cursor                    int
	busy                      bool
	width, height             int
	notice, page              string
	noticeFailure             bool
	offset                    int
	pending                   fleetclient.Request
	pendingLabel, pendingName string
	form                      [5]string
	field                     int
	groupSelection            groupSelection
	view                      selectedView
	metadata                  *metadataEditor
	top                       int
	// pageRow is info's captured lifetime as last observed; refresh follows it
	// without retargeting.
	pageRow               fleetclient.Row
	searchRevision        int
	searchCancel          context.CancelFunc
	searchState           directorySearchState
	searchProblem         string
	searchOmitted         bool
	searchDirectories     []string
	directorySelection    fieldSelection[string]
	blurred               bool
	notifications         attention.State
	notificationFailed    bool
	notificationQualified bool
	// attached: a terminal owns the screen, so no lane reads.
	attached         bool
	inventoryLanes   []inventoryLane
	contextLanes     []lane
	notificationLane lane
	usageLane        lane
	usagePeers       []usagePeer
	usageNext        time.Time
}

// Run browses until quit. Its cancel aborts every read and search still in
// flight; bubble tea joins no commands.
func Run(ctx context.Context, client *fleetclient.Client, input, output *os.File) error {
	browserContext, cancel := context.WithCancel(ctx)
	defer cancel()
	m := newModel(browserContext, client, input, output)
	_, err := tea.NewProgram(m, tea.WithContext(browserContext), tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
}
func newModel(ctx context.Context, client *fleetclient.Client, input, output *os.File) *model {
	peers := []fleetclient.Peer{}
	usagePeers := []usagePeer{}
	for _, machine := range client.Machines() {
		peers = append(peers, fleetclient.Peer{Label: machine.Label, Machine: machine.Handle})
		usagePeers = append(usagePeers, usagePeer{machine: machine})
	}
	return &model{ctx: ctx, client: client, input: input, output: output, peers: peers, usagePeers: usagePeers, cursor: -1, width: 100, height: 30, inventoryLanes: make([]inventoryLane, len(peers)), contextLanes: make([]lane, len(peers))}
}
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.refresh(false), m.refreshUsage(false), tick())
}
func tick() tea.Cmd { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }

// creationPeer is the configured default, never the selected or first reachable host.
func (m *model) creationPeer() *fleetclient.Peer {
	handle := m.client.DefaultMachine().Handle
	for index := range m.peers {
		if m.peers[index].Machine == handle {
			return &m.peers[index]
		}
	}
	return nil
}

func (m *model) execute(request fleetclient.Request) tea.Cmd {
	m.pending = request
	m.busy = true
	if index := m.mutationPeer(request); index >= 0 {
		m.fenceInventory(index)
		m.inventoryLanes[index].writing = true
		m.peers[index].OK, m.peers[index].Error = false, nil
		m.rebuild()
	}
	ctx, client := m.ctx, m.client
	return func() tea.Msg {
		return actionMsg{operation: request.Operation, result: client.Execute(ctx, request)}
	}
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	wasVisible, previousPage := m.usageVisible(), m.page
	var command tea.Cmd
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case tea.BlurMsg:
		m.blurred = true
		m.clearDirectorySearch()
	case tea.FocusMsg:
		m.blurred = false
	case tickMsg:
		command = tea.Batch(m.refresh(false), m.refreshUsage(false), tick())
	case fleetMsg:
		command = m.onFleet()
	case usageMsg:
		command = m.acceptUsage(message)
	case inventoryMsg:
		command = m.acceptInventory(message)
	case contextMsg:
		command = m.acceptContext(message)
	case notificationMsg:
		command = m.acceptNotifications(message)
	case actionMsg:
		command = m.onAction(message)
	case searchMsg:
		command = m.onSearch(message)
	case attachedMsg:
		command = m.onAttached(message)
	case tea.PasteMsg:
		command = m.onPaste(message)
	case tea.KeyPressMsg:
		command = m.onKey(message)
	}
	return m, tea.Batch(command, m.settle(previousPage, wasVisible))
}

// settle follows every message: a change of page or usage disclosure retires
// the usage read and asks for a prompt successor; pages clamp their scroll;
// the table keeps its cursor in view.
func (m *model) settle(previousPage string, wasVisible bool) tea.Cmd {
	var command tea.Cmd
	if previousPage != m.page || wasVisible != m.usageVisible() {
		m.usageLane.fence()
		m.usageLane.trailing = true
		// disclosure changes read immediately. a hidden form or
		// fullscreen visit retires work while preserving the attempt cadence.
		if previousPage == "usage" || m.page == "usage" {
			m.usageNext = time.Time{}
		}
		command = m.refreshUsage(false)
	}
	if m.page == "usage" && !m.tooSmall() {
		m.offset = min(max(0, m.offset), max(0, len(m.usageLines())-m.pageCapacity()))
	}
	if m.page == "confirm" && !m.tooSmall() {
		_, facts := m.confirmationContent()
		m.offset = min(max(0, m.offset), max(0, len(facts)-m.pageCapacity()))
	}
	m.fitViewports()
	return command
}

func (m *model) onFleet() tea.Cmd {
	m.fleetPending = false
	m.reconcileFleet()
	commands := []tea.Cmd{m.refreshUsage(false)}
	for index := range m.peers {
		commands = append(commands, m.refreshInventory(index, false))
	}
	return tea.Batch(commands...)
}

// onAction reports a finished action. Keys and paste are refused while busy,
// so the action's pending request and page are still the ones that sent it.
func (m *model) onAction(message actionMsg) tea.Cmd {
	operation, result := message.operation, message.result
	if (operation == "start" || operation == "shell") && m.ctx.Err() != nil {
		return nil // the browser is closing: never enter what it created
	}
	m.busy = false
	if index := m.mutationPeer(m.pending); index >= 0 {
		m.inventoryLanes[index].writing = false
	}
	if operation == "start" && result.OK {
		created := result.Value.(fleetclient.StartResult)
		if created.Failure != nil {
			failure := *created.Failure
			failure.Target = created.Target
			result = fleetclient.Result{Error: &failure}
		} else {
			result.Value = *created.Terminal
		}
	}
	if !result.OK {
		return m.onActionFailure(operation, result.Error)
	}
	switch operation {
	case "start", "shell":
		return m.adoptCreated(result.Value.(fleetclient.ObservedSession))
	case "group", "rename":
		m.metadata.checking = true
		m.metadata.acknowledged = true
		m.invalidatePendingPeer()
		m.inform("metadata saved; checking inventory")
	case "close":
		text := fleetclient.CloseText(result.Value.(api.CloseResult))
		if result.ExitCode("close") != 0 {
			m.fail(text)
		} else {
			m.inform(text)
		}
	default:
		receipt := result.Value.(api.WriteResult)
		text := fleetclient.WriteText(operation, receipt)
		if receipt.Outcome == "unknown" {
			m.fail(text)
		} else {
			m.inform(text)
		}
	}
	return m.refreshMutation()
}

func (m *model) onActionFailure(operation string, failure *fleetclient.Failure) tea.Cmd {
	text := fleetclient.ErrorMessage(*failure, m.pending)
	if operation == "start" {
		text = m.pending.Machine + ": " + text
		m.invalidateAccess(m.pending.Machine, failure)
		if m.page == "create" && (failure.Code == "WorkingDirectoryInvalid" || failure.Code == "WorkingDirectoryUnavailable") {
			m.focusForm(formDirectory)
		}
	}
	m.fail(text)
	if operation == "group" || operation == "rename" {
		m.metadata.failure = failure
		if failure.Dispatch == "unknown" || failure.Code == "SessionNotFound" || failure.Code == "SessionIdentityMismatch" || failure.Code == "InternalError" || accessRejected(failure) || failure.Code == "SessionNameConflict" || failure.Code == "SessionNameChanged" {
			m.metadata.checking = true
			m.metadata.acknowledged = false
			m.invalidatePendingPeer()
		}
	}
	return m.refreshMutation()
}

// adoptCreated reveals a confirmed creation in its group, selects it and
// enters it. A peer the roster dropped meanwhile leaves the row to inventory.
func (m *model) adoptCreated(value fleetclient.ObservedSession) tea.Cmd {
	// Creation is a separate sample. Do not attach its observation metadata
	// to this peer's earlier inventory receipt.
	value.Session.TerminalUsage = nil
	value.Session.TerminalFooter = nil
	m.page = ""
	index := m.peerIndex(value.Machine)
	if index < 0 {
		return m.refreshMutation()
	}
	if m.view.kind == viewNeedsInput || m.view.kind == viewGroup && m.view.label != value.Session.Group {
		m.view = selectedView{kind: viewGroup, label: value.Session.Group}
	}
	created := value.Session.Ref.Reference
	isCreated := func(session fleetclient.Session) bool { return session.Ref.SessionEqual(created) }
	if row := slices.IndexFunc(m.peers[index].Sessions, isCreated); row >= 0 {
		m.peers[index].Sessions[row] = value.Session
	} else {
		m.peers[index].Sessions = append(m.peers[index].Sessions, value.Session)
	}
	m.rebuild()
	if row := slices.IndexFunc(m.rows, func(row fleetclient.Row) bool { return isCreated(row.Session) }); row >= 0 {
		m.cursor = row
	}
	m.inform("opening terminal on " + value.Label + "…")
	return m.enter(created)
}

func (m *model) onSearch(message searchMsg) tea.Cmd {
	if m.page != "create" || m.form[formMachine] != message.machine || m.searchRevision != message.revision || m.blurred || !m.directoryAvailable() || m.ctx.Err() != nil {
		return nil
	}
	initial := m.searchState == directorySearchLoading
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
	if !message.result.OK {
		failure := message.result.Error
		m.searchState, m.searchDirectories, m.directorySelection.choice = directorySearchFailed, nil, nil
		problem := "directory search unavailable on " + message.machine
		switch {
		case failure != nil && failure.Code == "DirectorySearchTooLarge":
			problem = "too many results; narrow your search"
		case accessRejected(failure):
			problem = fleetclient.ErrorMessage(*failure, fleetclient.Request{})
		}
		m.searchProblem = problem
		m.invalidateAccess(message.machine, failure)
		m.fail(problem)
		return nil
	}
	value := message.result.Value.(workdir.SearchResult)
	m.searchState, m.searchDirectories, m.searchOmitted = directorySearchReady, value.Directories, value.Omitted
	m.directorySelection.retain(value.Directories)
	if initial && len(value.Directories) > 0 {
		m.directorySelection.choice = &value.Directories[0]
	}
	m.inform("")
	if len(value.Directories) == 0 {
		m.inform("no matching directories")
	} else if value.Omitted {
		m.inform("some directories are not shown")
	}
	return nil
}

func (m *model) onAttached(message attachedMsg) tea.Cmd {
	m.attached = false
	m.notificationFailed = m.notificationFailed || message.notificationErr != nil
	m.rebuild()
	if message.err != nil {
		m.fail(message.err.Error())
	} else {
		m.inform("detached; work continues")
	}
	return m.refresh(true)
}

// A notice's tone travels with its text: failures read as ember, all else plain.
func (m *model) inform(text string) { m.notice, m.noticeFailure = text, false }
func (m *model) fail(text string)   { m.notice, m.noticeFailure = text, true }

func (m *model) invalidatePendingPeer() {
	for index := range m.peers {
		if m.peers[index].Machine == m.pending.Ref.Machine {
			m.peers[index].OK = false
		}
	}
	m.rebuild()
}

// accessRejected reports a failure that holds for its whole host until the next inventory.
func accessRejected(failure *fleetclient.Failure) bool {
	return failure != nil && (failure.Code == "Unauthenticated" || failure.Code == "MachineIdentityMismatch")
}

// Auth and identity failures are authoritative even before the next inventory.
func (m *model) invalidateAccess(label string, failure *fleetclient.Failure) {
	if !accessRejected(failure) {
		return
	}
	for index := range m.peers {
		if m.peers[index].Label == label {
			m.fenceInventory(index)
			m.inventoryLanes[index].trailing = true
			m.peers[index].OK, m.peers[index].Error = false, failure
		}
	}
	if m.page == "create" && m.form[formMachine] == label {
		m.clearDirectorySearch()
	}
	m.rebuild()
}
func (m *model) selectedRow() *fleetclient.Row {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

// enter pauses the browser while this exact terminal owns input and output.
func (m *model) enter(ref fleetclient.Reference) tea.Cmd {
	m.attached = true
	m.usageLane.fence()
	m.cancelObservations()
	m.clearDirectorySearch()
	a := &attachment{ctx: m.ctx, client: m.client, ref: ref, input: m.input, output: m.output}
	return tea.Exec(a, func(err error) tea.Msg { return attachedMsg{err: err, notificationErr: a.notificationErr} })
}

type attachment struct {
	ctx             context.Context
	client          *fleetclient.Client
	ref             fleetclient.Reference
	input, output   *os.File
	notificationErr error
}

func (a *attachment) Run() error {
	result := terminalclient.Run(a.ctx, a.client, a.ref, a.input, a.output)
	a.notificationErr = result.NotificationErr
	return result.Err
}
func (a *attachment) SetStdin(io.Reader)  {}
func (a *attachment) SetStdout(io.Writer) {}
func (a *attachment) SetStderr(io.Writer) {}
