// Package sessionui presents the same fleet client as one terminal browser.
package sessionui

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
)

type inventoryMsg struct {
	owner      *inventoryLane
	generation uint64
	result     fleetclient.Result
}
type contextMsg struct {
	owner      *inventoryLane
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
	owner            *inventoryLane
	machine, subject string
	result           fleetclient.Result
}
type searchMsg struct {
	machine  string
	revision int
	result   fleetclient.Result
}
type attachedMsg struct{}
type revealMsg struct{ generation int }

type model struct {
	ground                ground
	ctx                   context.Context
	client                *fleetclient.Client
	fleetPending          bool
	input, output         *os.File
	peers                 []fleetclient.Peer
	rows                  []fleetclient.Row
	cursor                int
	guest                 bool
	busy                  bool
	width, height         int
	notice                string
	noticeFailure         bool
	pending               fleetclient.Request
	fold                  fold
	band                  bool
	reveal                time.Time // start of the active reveal
	now                   time.Time // shared by every projection of this update
	revealGeneration      int
	form                  createDraft
	field                 int
	groupSelection        groupSelection
	view                  selectedView
	metadata              *metadataEditor
	top                   int
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
	attachment            *attachment
	inventoryLanes        []*inventoryLane
	notificationLane      lane
	usageLane             lane
	tree                  map[api.SessionRef]*treeNode
	expanded              map[api.SessionRef]bool
	usagePeers            []usagePeer
	claudeUsage           []fleetclient.ProfileUsage
	usageNext             time.Time
}

// Run browses until quit. Its cancel aborts every read and search still in
// flight; bubble tea joins no commands.
func Run(ctx context.Context, client *fleetclient.Client, input, output *os.File) error {
	browserContext, cancel := context.WithCancel(ctx)
	defer cancel()
	m := &model{
		ground: ground{pigments: plainPigments},
		ctx:    browserContext, client: client, input: input, output: output,
		cursor: -1, width: 100, height: 30, now: time.Now(),
	}
	m.claudeUsage = client.KnownClaudeUsage()
	for _, machine := range client.Machines() {
		m.peers = append(m.peers, fleetclient.Peer{Label: machine.Label, Machine: machine.Handle})
		m.inventoryLanes = append(m.inventoryLanes, &inventoryLane{})
		m.usagePeers = append(m.usagePeers, usagePeer{machine: machine})
	}
	m.ground.noColor = os.Getenv("NO_COLOR") != ""
	if m.ground.noColor {
		m.ground.profile = colorprofile.ASCII
		m.ground.note = "colours: plain; NO_COLOR requested."
	} else {
		// The device reply can arrive before Bubble Tea's initial profile message.
		m.ground.profile = colorprofile.Detect(output, os.Environ())
	}
	_, err := tea.NewProgram(m, tea.WithContext(browserContext), tea.WithInput(input), tea.WithOutput(output), tea.WithColorProfile(m.ground.profile)).Run()
	return err
}
func (m *model) Init() tea.Cmd {
	var query tea.Cmd
	if !m.ground.noColor {
		query = tea.Raw(groundQuery)
	}
	return tea.Batch(query, m.refresh(false), m.refreshUsage(false), tick())
}
func tick() tea.Cmd { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }

// The reveal is the browser's one motion: fifteen frames a second from u until
// the longest chew has ended. Hiding the band or entering a terminal retires it.
const (
	revealFrame  = time.Second / 15
	revealFrames = 2*usageGauge + 1
)

func (m *model) revealTick() tea.Cmd {
	generation := m.revealGeneration
	delay := revealFrame - m.now.Sub(m.reveal)%revealFrame
	return tea.Tick(delay, func(time.Time) tea.Msg { return revealMsg{generation} })
}

func (m *model) execute(request fleetclient.Request, subject string) tea.Cmd {
	m.pending = request
	m.busy = true
	machine := request.Ref.Machine
	if request.Ref.IsZero() {
		if peer := m.peerLabeled(request.Machine); peer != nil {
			machine = peer.Machine
		}
	}
	var owner *inventoryLane
	if index := m.peerIndex(machine); index >= 0 {
		m.checkInventory(index)
		owner = m.inventoryLanes[index]
		owner.writing = true
		m.rebuild()
	}
	ctx, client := m.ctx, m.client
	return func() tea.Msg {
		return actionMsg{owner: owner, machine: machine, subject: subject, result: client.Execute(ctx, request)}
	}
}

func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var returned, command tea.Cmd
	m.now = time.Now()
	if visit := m.attachment; visit != nil && visit.finished {
		m.attachment = nil
		if visit.err != nil {
			var failure *fleetclient.Failure
			if errors.As(visit.err, &failure) {
				m.invalidateAccess(visit.ref.Machine, failure)
			}
			m.fail(visit.err.Error())
		} else {
			m.inform("detached from " + visit.name + " on " + visit.machine + ".")
		}
		returned = m.refresh(true)
	}
	switch message := message.(type) {
	case tea.RawMsg:
		if m.ground.note == "" && message.Msg == groundQuery {
			ctx := m.ctx
			command = func() tea.Msg {
				timer := time.NewTimer(time.Second)
				defer timer.Stop()
				select {
				case <-timer.C:
					return groundExpired{}
				case <-ctx.Done():
					return nil
				}
			}
		}
	case tea.ForegroundColorMsg:
		if m.ground.note == "" {
			m.ground.ink = message.Color
		}
	case tea.BackgroundColorMsg:
		if m.ground.note == "" {
			m.ground.base = message.Color
		}
	case uv.PrimaryDeviceAttributesEvent:
		if m.ground.note == "" {
			m.ground.settle(true)
		}
	case groundExpired:
		if m.ground.note == "" {
			m.ground.settle(false)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
	case tea.BlurMsg:
		m.blurred = true
		if m.searchState != directorySearchIdle {
			m.clearDirectorySearch()
		}
	case tea.FocusMsg:
		m.blurred = false
	case tickMsg:
		command = tea.Batch(m.refresh(false), m.refreshUsage(false), tick())
	case revealMsg:
		if message.generation == m.revealGeneration && !m.reveal.IsZero() {
			if m.now.Sub(m.reveal) < revealFrames*revealFrame {
				command = m.revealTick()
			} else {
				m.reveal = time.Time{}
			}
		}
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
	case branchPreviewMsg:
		m.onPreview(message)
	case branchClosedMsg:
		command = m.onBranchClosed(message)
	case searchMsg:
		command = m.onSearch(message)
	case tea.PasteMsg:
		if m.attachment == nil {
			command = m.onPaste(message)
		}
	case tea.KeyPressMsg:
		if m.attachment == nil {
			command = m.onKey(message)
		}
	}
	m.settle()
	return m, tea.Batch(returned, command)
}

// settle follows every message so the table keeps the cursor in view.
func (m *model) settle() {
	m.top = m.frame().top
}

func (m *model) onFleet() tea.Cmd {
	m.fleetPending = false
	if !m.reconcileFleet() {
		return nil
	}
	commands := []tea.Cmd{m.refreshUsage(false)}
	for index, lane := range m.inventoryLanes {
		if !lane.settled {
			commands = append(commands, m.refreshInventory(index, false))
		}
	}
	return tea.Batch(commands...)
}

// onAction reports a finished action. Keys and paste are refused while busy,
// so the action's pending request and fold still own its result.
func (m *model) onAction(message actionMsg) tea.Cmd {
	operation, result := m.pending.Operation, message.result
	if (operation == "start" || operation == "shell") && m.ctx.Err() != nil {
		return nil // the browser is closing: never enter what it created
	}
	m.busy = false
	index := slices.Index(m.inventoryLanes, message.owner)
	if index >= 0 {
		message.owner.writing = false
	}
	machine := message.machine
	if operation == "start" && result.OK {
		created := result.Value.(fleetclient.StartResult)
		machine = created.Machine
		if created.Failure != nil {
			failure := *created.Failure
			failure.Target = created.Target
			result = fleetclient.Result{Error: &failure}
		} else {
			result.Value = *created.Terminal
		}
	}
	actual := index
	if machine != message.machine {
		actual = m.peerIndex(machine)
	}
	if !result.OK {
		if actual >= 0 {
			m.invalidateAccess(machine, result.Error)
		}
		m.onActionFailure(operation, message.subject, result.Error)
		refresh := m.refreshMutation(index)
		if actual >= 0 && actual != index {
			return tea.Batch(refresh, m.refreshInventory(actual, true))
		}
		return refresh
	}
	switch operation {
	case "start", "shell":
		m.fold = fold{}
		value := result.Value.(fleetclient.ObservedSession)
		if actual >= 0 {
			if visit := m.adoptCreated(actual, value); visit != nil {
				return visit
			}
			refresh := m.refreshMutation(index)
			if actual != index {
				return tea.Batch(refresh, m.refreshInventory(actual, true))
			}
			return refresh
		}
		m.clearDirectorySearch()
		m.inform("terminal created on " + value.Label + ".")
	case "group", "rename":
		m.completeMetadata(message.subject, nil)
	case "promote":
		m.fold = fold{}
		text := message.subject + " is now top-level."
		if cleanup, _ := result.Value.(*fleetclient.Failure); cleanup != nil {
			m.fail(text + " could not confirm cleanup on the former parent (" + cleanup.Code + ").")
		} else {
			m.inform(text)
		}
	default:
		receipt := result.Value.(api.WriteResult)
		text := message.subject + ": " + fleetview.WriteText(operation, receipt)
		if receipt.Outcome == "unknown" {
			m.fail(text)
		} else {
			m.inform(text)
		}
	}
	return m.refreshMutation(index)
}

func (m *model) onActionFailure(operation, subject string, failure *fleetclient.Failure) {
	if operation == "group" || operation == "rename" {
		m.completeMetadata(subject, failure)
		return
	}
	text := subject + ": " + fleetview.ErrorMessage(*failure, m.pending)
	if operation == "start" && m.fold.kind == foldDraft && (failure.Code == "WorkingDirectoryInvalid" || failure.Code == "WorkingDirectoryUnavailable") {
		m.focusForm(formDirectory)
	}
	m.fail(text)
}

// adoptCreated reveals and selects the new session before attempting to attach.
func (m *model) adoptCreated(index int, value fleetclient.ObservedSession) tea.Cmd {
	// Creation is a separate sample. Do not attach its observation metadata
	// to this peer's earlier inventory receipt.
	value.Session.TerminalUsage = nil
	value.Session.TerminalFooter = nil
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
	for id := value.Session.Parent; id != nil; {
		if m.expanded == nil {
			m.expanded = make(map[api.SessionRef]bool)
		}
		m.expanded[*id] = true
		parent := m.tree[*id]
		if parent == nil {
			break
		}
		id = parent.row.Session.Parent
	}
	m.rebuild()
	if row := slices.IndexFunc(m.rows, func(row fleetclient.Row) bool { return isCreated(row.Session) }); row >= 0 {
		m.selectRow(row)
	}
	if !terminal.ValidSize(m.width, m.height) {
		m.clearDirectorySearch()
		m.fail("terminal created on " + value.Label + "; " + attachSize + ".")
		return nil
	}
	m.inform("opening terminal on " + value.Label + "…")
	return m.enter(created)
}

func (m *model) onSearch(message searchMsg) tea.Cmd {
	if m.fold.kind != foldDraft || m.form.machine != message.machine || m.searchRevision != message.revision || m.blurred || !m.directoryAvailable() || m.ctx.Err() != nil {
		return nil
	}
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
			problem = fleetview.ErrorMessage(*failure, fleetclient.Request{})
		}
		m.searchProblem = problem
		m.invalidateAccess(m.peerLabeled(message.machine).Machine, failure)
		m.fail(problem)
		return nil
	}
	value := message.result.Value.(workdir.SearchResult)
	m.searchState, m.searchDirectories, m.searchOmitted = directorySearchReady, value.Directories, value.Omitted
	if len(value.Directories) > 0 {
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

// A notice's tone travels with its text: failures read as ember, all else plain.
func (m *model) inform(text string) { m.notice, m.noticeFailure = text, false }
func (m *model) fail(text string)   { m.notice, m.noticeFailure = text, true }

// accessRejected reports a failure that holds for its whole host until the next inventory.
func accessRejected(failure *fleetclient.Failure) bool {
	return failure != nil && (failure.Code == "Unauthenticated" || failure.Code == "MachineIdentityMismatch")
}

// Auth and identity failures are authoritative even before the next inventory.
func (m *model) invalidateAccess(machine string, failure *fleetclient.Failure) {
	if !accessRejected(failure) {
		return
	}
	index := m.peerIndex(machine)
	if index < 0 {
		return
	}
	m.fenceInventory(index)
	m.forgetContext(index)
	m.inventoryLanes[index].trailing = true
	m.inventoryLanes[index].ready = nil
	m.peers[index].OK, m.peers[index].Error = false, failure
	m.rebuildFromPeers()
}
func (m *model) selectedRow() *fleetclient.Row {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.rows[m.cursor]
}

// enter pauses the browser while this exact terminal owns input and output.
func (m *model) enter(ref fleetclient.Reference) tea.Cmd {
	m.fold, m.reveal = fold{}, time.Time{}
	m.revealGeneration++
	row := m.rowForReference(ref)
	visit := &attachment{ctx: m.ctx, client: m.client, ref: ref, input: m.input, output: m.output, name: row.Session.Name, machine: row.Label}
	m.attachment = visit
	m.usageLane.fence()
	m.usageLane.trailing = false
	m.cancelObservations()
	m.rebuild()
	m.clearDirectorySearch()
	return tea.Exec(visit, func(err error) tea.Msg {
		// Bubble Tea's blocking Exec evaluates the callback on the update
		// loop before sending its message, which can arrive after a key.
		visit.err, visit.finished = err, true
		return attachedMsg{}
	})
}

type attachment struct {
	ctx           context.Context
	client        *fleetclient.Client
	ref           fleetclient.Reference
	input, output *os.File
	name, machine string
	err           error
	finished      bool
}

func (a *attachment) Run() error {
	return terminalclient.Run(a.ctx, a.client, a.ref, a.input, a.output).Err
}
func (a *attachment) SetStdin(io.Reader)  {}
func (a *attachment) SetStdout(io.Writer) {}
func (a *attachment) SetStderr(io.Writer) {}
