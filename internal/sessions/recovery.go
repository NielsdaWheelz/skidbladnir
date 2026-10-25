package sessions

import (
	"context"
	"encoding/base64"
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

// RecoveryChange carries only content-free evidence; the observer must not reenter Manager.
type RecoveryChange struct {
	At              time.Time
	Recovery        api.Recovery
	Operation       string
	Errno           syscall.Errno
	Protocol        *tmuxclient.ProtocolError
	DispatchUnknown bool
}

// recoveryFailure carries one failed step's wire classification and content-free evidence.
type recoveryFailure struct {
	phase           api.RecoveryPhase
	cause           api.RecoveryCause
	operation       string
	errno           syscall.Errno
	protocol        *tmuxclient.ProtocolError
	dispatchUnknown bool
}

// osFailure classifies by errno: EACCES/EPERM permission; ENOSPC/EDQUOT
// no_space where the phase admits it, else io; any other errno io. An error
// without errno takes fallback.
func osFailure(phase api.RecoveryPhase, operation string, err error, fallback api.RecoveryCause) *recoveryFailure {
	failure := &recoveryFailure{phase: phase, cause: fallback, operation: operation}
	if errors.As(err, &failure.errno) {
		failure.cause = api.RecoveryCauseIO
		switch failure.errno {
		case syscall.EACCES, syscall.EPERM:
			failure.cause = api.RecoveryCausePermission
		case syscall.ENOSPC, syscall.EDQUOT:
			if phase == api.RecoveryPhaseOwnership || phase == api.RecoveryPhaseCheckpointWrite || phase == api.RecoveryPhaseRestore {
				failure.cause = api.RecoveryCauseNoSpace
			}
		}
	}
	return failure
}

// tmuxFailure classifies a tmux client failure in the source, capture or
// restore phase. tmux's typed evidence names the cause; a lifetime
// contradiction during capture is a source failure. An errno is an OS failure.
// Otherwise an expired budget is timeout and anything else leaves tmux
// unavailable. Positive evidence outranks the deadline.
func tmuxFailure(ctx context.Context, phase api.RecoveryPhase, operation string, err error) *recoveryFailure {
	var errno syscall.Errno
	errors.As(err, &errno)
	failure := &recoveryFailure{phase: phase, cause: api.RecoveryCauseUnavailable, operation: operation, errno: errno}
	switch {
	case errors.Is(err, tmuxclient.WorkspaceSourceChanged):
		failure.cause = api.RecoveryCauseChanged
		if phase == api.RecoveryPhaseCapture {
			failure.phase = api.RecoveryPhaseSource
		}
	case errors.Is(err, tmuxclient.WorkspaceTopologyChanged):
		failure.cause = api.RecoveryCauseChanged
	case errors.Is(err, tmuxclient.WorkspaceUnsupported), errors.Is(err, tmuxclient.WorkspaceProtocolInvalid):
		failure.cause = api.RecoveryCauseInvalid
	case errors.Is(err, tmuxclient.WorkspaceLimit):
		failure.cause = api.RecoveryCauseLimit
	case errno != 0:
		failure = osFailure(phase, operation, err, api.RecoveryCauseIO)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		failure.cause = api.RecoveryCauseTimeout
	}
	errors.As(err, &failure.protocol)
	failure.dispatchUnknown = errors.Is(err, tmuxclient.ErrDispatchUnknown)
	return failure
}

type workspaceRecovery struct {
	path       string
	machine    machine.Handle
	owner      context.Context
	lock       *os.File
	recipe     *checkpoint
	fact       api.Recovery
	observe    func(RecoveryChange)
	lastChange RecoveryChange
}

// StartRecovery takes the checkpoint lock, reads the checkpoint and runs the
// first reconcile before returning. Call it once, after listening and before
// serving requests; call the returned cleanup once to cancel and join recovery
// before releasing this process's writer lock.
func (manager *Manager) StartRecovery(parent context.Context, observe func(RecoveryChange)) func() error {
	ctx, cancel := context.WithCancel(parent)
	if err := manager.acquire(ctx); err != nil {
		cancel()
		return func() error { return nil }
	}
	manager.recovery.owner = ctx
	manager.recovery.observe = observe
	lock, failure := manager.acquireRecoveryLock()
	if failure == nil {
		manager.recovery.lock = lock
		var recipe *checkpoint
		recipe, failure = manager.readCheckpoint()
		if recipe != nil {
			manager.recovery.recipe = recipe
			savedAt := recipe.SavedAt.UTC() // the wire instant is UTC whatever offset the file spelled
			manager.recovery.fact.SavedAt = &savedAt
		}
	}
	if failure != nil {
		manager.failRecovery(failure, api.RecoveryBroken)
	}
	manager.release()
	if failure == nil {
		manager.reconcileRecovery()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// justify-polling: capture native tmux changes without connected clients
		// every 30 seconds; gateway cancellation stops work, and a broken
		// recovery reconciles nothing.
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			manager.reconcileRecovery()
		}
	}()
	return func() error {
		cancel()
		<-done
		if lock == nil {
			return nil
		}
		// A mutation checkpoint may still be writing; the cancelled owner stops
		// later ones. A background context never fails acquire.
		manager.acquire(context.Background())
		defer manager.release()
		if lock.Close() != nil {
			return errors.New("workspace recovery ownership could not be released")
		}
		return nil
	}
}

func (manager *Manager) publishRecovery(fact api.Recovery, failure *recoveryFailure) {
	previous := manager.recovery.lastChange
	change := RecoveryChange{At: time.Now().UTC(), Recovery: fact}
	if failure != nil {
		change.Operation, change.Errno = failure.operation, failure.errno
		change.Protocol, change.DispatchUnknown = failure.protocol, failure.dispatchUnknown
	}
	manager.recovery.fact = fact
	changed := previous.Recovery.State != fact.State || previous.Operation != change.Operation || previous.Errno != change.Errno || previous.DispatchUnknown != change.DispatchUnknown || (previous.Protocol == nil) != (change.Protocol == nil)
	if previous.Protocol != nil && change.Protocol != nil {
		changed = changed || *previous.Protocol != *change.Protocol
	}
	if previous.Recovery.Failure != nil && fact.Failure != nil {
		changed = changed || previous.Recovery.Failure.Phase != fact.Failure.Phase || previous.Recovery.Failure.Cause != fact.Failure.Cause
	}
	manager.recovery.lastChange = change
	if changed && !(previous.Recovery.State == "" && fact.State == api.RecoveryTracking) {
		manager.recovery.observe(change)
	}
}

// The lifecycle callsite chooses whether a failed step delays saving or fences recovery.
func (manager *Manager) failRecovery(failure *recoveryFailure, state api.RecoveryState) {
	if manager.recovery.owner.Err() != nil {
		return
	}
	manager.publishRecovery(api.Recovery{State: state, SavedAt: manager.recovery.fact.SavedAt, Failure: &api.RecoveryFailure{At: time.Now().UTC(), Phase: failure.phase, Cause: failure.cause}}, failure)
}

// The mutation is already confirmed; checkpoint failure never changes its outcome.
func (manager *Manager) checkpointAfterMutation(mutation context.Context, expected tmuxclient.ServerIdentity) {
	deadline := time.Now().Add(10 * time.Second)
	if original, ok := mutation.Deadline(); ok && original.Before(deadline) {
		deadline = original
	}
	ctx, cancel := context.WithDeadline(manager.recovery.owner, deadline)
	defer cancel()
	manager.saveCurrentCheckpoint(ctx, expected)
}

// Native clients remain independent of the manager gate.
func (manager *Manager) reconcileRecovery() {
	if manager.acquire(manager.recovery.owner) != nil {
		return
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	if manager.recovery.fact.State == api.RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	recipe := manager.recovery.recipe
	if recipe != nil && recipe.Attempted {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCheckpointRead, cause: api.RecoveryCauseAlreadyAttempted, operation: "validate"}, api.RecoveryBroken)
		return
	}
	restore, cancelRestore := context.WithTimeout(manager.recovery.owner, 30*time.Second)
	defer cancelRestore()
	// An ordinary save gets 10 seconds, so a hung tmux holds the lock briefly.
	ctx, cancel := context.WithTimeout(restore, 10*time.Second)
	defer cancel()
	gone := true
	if recipe != nil {
		var failure *recoveryFailure
		gone, failure = sourceGone(recipe.Source)
		if failure != nil {
			manager.failRecovery(failure, api.RecoveryDelayed)
			return
		}
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(tmuxFailure(ctx, api.RecoveryPhaseSource, "server_inspect", err), api.RecoveryDelayed)
		return
	}
	if recipe != nil && !gone {
		if !present {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseUnavailable, operation: "server_inspect"}, api.RecoveryDelayed)
			return
		}
		if server.Identity != recipe.Source.Server {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, api.RecoveryBroken)
			return
		}
	}
	if recipe == nil || !gone || len(recipe.Sessions) == 0 {
		if !present {
			return
		}
		manager.release()
		locked = false
		manager.saveCurrentCheckpoint(ctx, server.Identity)
		return
	}
	// Preflight failures leave the file untouched, so a restart after repair retries.
	if present && server.SessionCount != 0 {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseNotEmpty, operation: "restore_preflight"}, api.RecoveryBroken)
		return
	}
	for _, window := range recipe.Windows {
		for _, pane := range window.Panes {
			var cause api.RecoveryCause
			if parsed, err := manager.workdir.ParseStart(pane.CWD); err != nil || parsed != pane.CWD {
				cause = api.RecoveryCauseInvalid
			} else if workdir.CheckStart(parsed) != nil {
				cause = api.RecoveryCauseUnavailable
			}
			if cause != "" {
				manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: cause, operation: "restore_preflight"}, api.RecoveryBroken)
				return
			}
		}
	}
	started := !present
	var shell string
	if present {
		var failure *recoveryFailure
		shell, failure = manager.destinationShell(restore, server.Identity)
		if failure != nil {
			manager.failRecovery(failure, api.RecoveryBroken)
			return
		}
	}
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_preflight"}, api.RecoveryBroken)
		return
	}
	settings := manager.settings.Load()
	startup, dispatched, counts := planRecoveryStartups(recipe, settings)
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_preflight"}, api.RecoveryBroken)
		return
	}
	claimed := *recipe
	claimed.Attempted = true
	if failure := manager.writeCheckpoint(claimed); failure != nil {
		manager.failRecovery(failure, api.RecoveryBroken)
		return
	}
	manager.recovery.recipe = &claimed
	if started {
		if err := manager.tmux.StartRecoveryServer(restore); err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseRestore, "restore_execute", err), api.RecoveryBroken)
			return
		}
		server, present, err = manager.tmux.InspectServer(restore)
		if err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseSource, "server_inspect", err), api.RecoveryBroken)
			return
		}
		if !present {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseUnavailable, operation: "server_inspect"}, api.RecoveryBroken)
			return
		}
	}
	destination := server.Identity
	if destination.Epoch == "" {
		destination, err = manager.tmux.EnsureServerIdentity(restore, destination, newServerEpoch())
		if err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseSource, "server_identity", err), api.RecoveryBroken)
			return
		}
	}
	if started {
		var failure *recoveryFailure
		shell, failure = manager.destinationShell(restore, destination)
		if failure != nil {
			manager.failRecovery(failure, api.RecoveryBroken)
			return
		}
	}
	paneMap, err := manager.tmux.WithShell(settings.shellPath).RestoreWorkspace(restore, destination, shell, recipe.workspace(), startup)
	if err != nil {
		manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseRestore, "restore_execute", err), api.RecoveryBroken)
		return
	}
	capture, err := manager.tmux.ReadInventory(restore, destination)
	if err != nil {
		manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseCapture, "inventory_read", err), api.RecoveryBroken)
		return
	}
	ids := make([]string, len(capture.Rows))
	for index, row := range capture.Rows {
		ids[index] = row.ID
	}
	saved, failure := manager.captureCheckpoint(restore, destination, ids)
	if failure != nil {
		manager.failRecovery(failure, api.RecoveryBroken)
		return
	}
	for oldKey, agent := range dispatched {
		saved.Agents[paneMap[oldKey]] = agent
	}
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_execute"}, api.RecoveryBroken)
		return
	}
	if failure := manager.writeCheckpoint(saved); failure != nil {
		manager.failRecovery(failure, api.RecoveryBroken)
		return
	}
	manager.commitCheckpoint(saved, api.RecoveryRecovered, &counts)
}

// destinationShell admits an empty destination's configuration and returns its
// default shell; a missing shell is unavailable, not io.
func (manager *Manager) destinationShell(ctx context.Context, destination tmuxclient.ServerIdentity) (string, *recoveryFailure) {
	shell, err := manager.tmux.ValidateRecoveryServer(ctx, destination)
	if err != nil {
		failure := tmuxFailure(ctx, api.RecoveryPhaseRestore, "restore_preflight", err)
		if errors.Is(err, fs.ErrNotExist) {
			failure.cause = api.RecoveryCauseUnavailable
		}
		return "", failure
	}
	return shell, nil
}

// sourceGone reports whether source's tmux server process no longer runs: the
// boot changed, or its pid is absent or names another process. The kernel
// reads ignore deadlines; the next tmux step reports an expired one.
func sourceGone(source checkpointSource) (bool, *recoveryFailure) {
	boot, err := processinfo.BootIdentity()
	if err != nil {
		return false, osFailure(api.RecoveryPhaseSource, "boot_identity", err, api.RecoveryCauseUnavailable)
	}
	if boot != source.BootID {
		return true, nil
	}
	pid, _ := strconv.Atoi(source.Server.PID)
	observed, err := processinfo.Observe(processinfo.PID(pid))
	if errors.Is(err, processinfo.ErrProcessAbsent) {
		return true, nil
	}
	if err != nil {
		return false, osFailure(api.RecoveryPhaseSource, "process_observe", err, api.RecoveryCauseUnavailable)
	}
	return string(observed.StartIdentity) != source.ProcessStartIdentity, nil
}

func (manager *Manager) commitCheckpoint(recipe checkpoint, state api.RecoveryState, resume *api.RecoveryResume) {
	manager.recovery.recipe = &recipe
	savedAt := recipe.SavedAt
	manager.publishRecovery(api.Recovery{State: state, SavedAt: &savedAt, Resume: resume}, nil)
}

// saveCurrentCheckpoint owns the gate, releasing it only for provider metadata.
func (manager *Manager) saveCurrentCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) {
	if manager.acquire(ctx) != nil {
		return
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	if manager.recovery.lock == nil || manager.recovery.fact.State == api.RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	anchor, settings := manager.recovery.recipe, manager.settings.Load()
	gone := false
	if anchor != nil {
		var failure *recoveryFailure
		gone, failure = sourceGone(anchor.Source)
		if failure != nil {
			manager.failRecovery(failure, api.RecoveryDelayed)
			return
		}
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(tmuxFailure(ctx, api.RecoveryPhaseSource, "server_inspect", err), api.RecoveryDelayed)
		return
	}
	if anchor != nil && present && (server.Identity != anchor.Source.Server || gone) {
		if !gone || len(anchor.Sessions) != 0 {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, api.RecoveryBroken)
			return
		}
	}
	if !present || server.Identity != expected {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_inspect"}, api.RecoveryDelayed)
		return
	}
	admitted, ids, failure := manager.prepareCheckpoint(ctx, server.Identity)
	if failure != nil {
		manager.failRecovery(failure, api.RecoveryDelayed)
		return
	}
	saved, failure := manager.captureCheckpoint(ctx, admitted, ids)
	if failure != nil {
		manager.failRecovery(failure, api.RecoveryDelayed)
		return
	}
	if anchor != nil && anchor.Source != saved.Source && (len(anchor.Sessions) != 0 || !gone) {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, api.RecoveryBroken)
		return
	}
	observations := observeRecoveryAgents(ctx, saved.workspace(), settings)
	manager.release()
	locked = false
	discovery, cancel := context.WithTimeout(ctx, 2*time.Second)
	results := make(map[string]agentruntime.ConversationResult)
	for _, profile := range settings.profiles {
		var queries []agentruntime.ConversationQuery
		for key, observation := range observations {
			if observation.profile == profile.Key && observation.scope != "" && observation.reason == "" {
				queries = append(queries, agentruntime.ConversationQuery{PaneKey: key, ForegroundPID: observation.foreground.PID, RawTitle: observation.pane.Title, CWD: observation.pane.CWD})
			}
		}
		if len(queries) == 0 {
			continue
		}
		found := agentruntime.DiscoverConversations(discovery, profile, queries)
		found = agentruntime.CloseConversationObservations(discovery, profile, queries, found)
		for _, result := range found {
			results[result.PaneKey] = result
		}
	}
	cancel()
	if manager.acquire(ctx) != nil {
		return
	}
	locked = true
	if manager.recovery.recipe != anchor || manager.recovery.fact.State == api.RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	if manager.settings.Load() != settings {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseChanged, operation: "capture"}, api.RecoveryDelayed)
		return
	}
	fresh, failure := manager.captureCheckpoint(ctx, saved.Source.Server, ids)
	if failure != nil {
		manager.failRecovery(failure, api.RecoveryDelayed)
		return
	}
	if fresh.Source != saved.Source || !sameRecoveryGraph(saved.workspace(), fresh.workspace()) {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseChanged, operation: "capture"}, api.RecoveryDelayed)
		return
	}
	current := observeRecoveryAgents(ctx, fresh.workspace(), settings)
	for key, observation := range observations {
		agent := checkpointAgent{Provider: observation.provider, Reason: observation.reason}
		after, found := current[key]
		result, resolved := results[key]
		switch {
		case !found || after.reason == agentruntime.ConversationObservationChanged || observation.pane.PID != after.pane.PID || observation.pane.CWD != after.pane.CWD || observation.foreground != after.foreground || observation.profile != after.profile || observation.scope != after.scope || !maps.Equal(observation.environment, after.environment):
			agent.Reason = agentruntime.ConversationObservationChanged
		case agent.Reason != "":
		case !resolved:
			agent.Reason = agentruntime.ConversationMetadataUnavailable
		case result.Reason != "":
			agent.Reason = result.Reason
		case result.Source == agentruntime.ConversationTitleMatch && !agentruntime.ConversationTitleMatches(observation.provider, result, after.pane.Title):
			agent.Reason = agentruntime.ConversationObservationChanged
		default:
			agent.Resume = &checkpointResume{Profile: observation.profile, AccountScope: observation.scope, ID: result.ID, Source: result.Source}
		}
		fresh.Agents[key] = agent
	}
	for key, observation := range current {
		if _, found := observations[key]; !found {
			fresh.Agents[key] = checkpointAgent{Provider: observation.provider, Reason: agentruntime.ConversationObservationChanged}
		}
	}
	if ctx.Err() != nil || manager.recovery.owner.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseTimeout, operation: "capture"}, api.RecoveryDelayed)
		return
	}
	if failure := manager.writeCheckpoint(fresh); failure != nil {
		manager.failRecovery(failure, api.RecoveryDelayed)
		return
	}
	manager.commitCheckpoint(fresh, api.RecoveryTracking, nil)
}

// prepareCheckpoint admits the epoch and repairs required character metadata once.
// Checkpointing preserves native names; inventory owns automatic naming.
func (manager *Manager) prepareCheckpoint(ctx context.Context, server tmuxclient.ServerIdentity) (tmuxclient.ServerIdentity, []string, *recoveryFailure) {
	if server.Epoch == "" {
		var err error
		server, err = manager.tmux.EnsureServerIdentity(ctx, server, newServerEpoch())
		if err != nil {
			return tmuxclient.ServerIdentity{}, nil, tmuxFailure(ctx, api.RecoveryPhaseSource, "server_identity", err)
		}
	}
	capture, err := manager.tmux.ReadInventory(ctx, server)
	if err != nil {
		return tmuxclient.ServerIdentity{}, nil, tmuxFailure(ctx, api.RecoveryPhaseCapture, "inventory_read", err)
	}
	rows, err := manager.normalizeCharacters(ctx, manager.scanCapture(capture), server)
	if err != nil {
		return tmuxclient.ServerIdentity{}, nil, tmuxFailure(ctx, api.RecoveryPhaseCapture, "character_assign", err)
	}
	ids := make([]string, len(rows))
	for index, row := range rows {
		ids[index] = row.ID
	}
	return server, ids, nil
}

// captureCheckpoint only reads. B == E == server proves one lifetime for the
// graph; that lifetime was observed live before its process identity was read.
func (manager *Manager) captureCheckpoint(ctx context.Context, server tmuxclient.ServerIdentity, ids []string) (checkpoint, *recoveryFailure) {
	pid, _ := strconv.Atoi(server.PID)
	process, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, osFailure(api.RecoveryPhaseSource, "process_observe", err, api.RecoveryCauseUnavailable)
	}
	workspace, err := manager.tmux.CaptureWorkspace(ctx, server, ids)
	if err != nil {
		return checkpoint{}, tmuxFailure(ctx, api.RecoveryPhaseCapture, "graph_capture", err)
	}
	if !manager.restorable(workspace) {
		return checkpoint{}, &recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseInvalid, operation: "validate"}
	}
	boot, err := processinfo.BootIdentity()
	if err != nil {
		return checkpoint{}, osFailure(api.RecoveryPhaseSource, "boot_identity", err, api.RecoveryCauseUnavailable)
	}
	return checkpoint{
		Schema: 2, Machine: manager.recovery.machine.String(), SavedAt: time.Now().UTC(),
		Source:   checkpointSource{BootID: boot, ProcessStartIdentity: string(process.StartIdentity), Server: server},
		Sessions: workspace.Sessions, Windows: workspace.Windows, Agents: make(map[string]checkpointAgent),
	}, nil
}

// restorable is the manager's half of restore admission, beside
// tmux.ValidateWorkspace: catalogue characters, skid group labels and parseable
// local cwds. tmux reports any option text and the file holds any decodable
// value; restore must never build what verification will refuse.
func (manager *Manager) restorable(workspace tmuxclient.Workspace) bool {
	for _, session := range workspace.Sessions {
		if _, found := catalog.Character(session.Character); !found {
			return false
		}
		if _, err := group.Parse(session.Group); err != nil {
			return false
		}
	}
	for _, window := range workspace.Windows {
		for _, pane := range window.Panes {
			if _, err := manager.workdir.ParseStart(pane.CWD); err != nil {
				return false
			}
		}
	}
	return true
}

// Recovery observations retain only enough evidence to close this checkpoint's
// process and profile bracket. Nothing here is persisted except the result.
type recoveryAgentObservation struct {
	pane        tmuxclient.WorkspacePane
	provider    agentruntime.Provider
	foreground  processinfo.Observation
	environment map[string]string
	profile     agentruntime.ProfileKey
	scope       string
	reason      agentruntime.ConversationReason
}

func observeRecoveryAgents(ctx context.Context, workspace tmuxclient.Workspace, settings *runtimeSettings) map[string]recoveryAgentObservation {
	observations := make(map[string]recoveryAgentObservation)
	for _, window := range workspace.Windows {
		for _, pane := range window.Panes {
			if ctx.Err() != nil {
				return observations
			}
			if _, found := observations[pane.Key]; found || pane.PID == 0 {
				continue
			}
			foreground, err := processinfo.ObserveForeground(processinfo.PID(pane.PID))
			if err != nil {
				continue
			}
			agent, recognized := agentruntime.Classify(foreground)
			if !recognized {
				continue
			}
			observation := recoveryAgentObservation{pane: pane, provider: agent.Provider, foreground: foreground}
			observation.environment, err = processinfo.ObserveForegroundEnvironment(processinfo.PID(pane.PID), foreground)
			if errors.Is(err, processinfo.ErrForegroundMismatch) {
				observation.reason = agentruntime.ConversationObservationChanged
			} else if err != nil {
				observation.reason = agentruntime.ConversationProfileUnavailable
			} else {
				observation.profile = agentruntime.MatchProfile(settings.profiles, agent.Provider, observation.environment)
				if profile, found := settings.byKey[observation.profile]; found {
					observation.scope, err = agentruntime.AccountScope(profile)
					if err != nil {
						observation.reason = agentruntime.ConversationProfileUnavailable
					}
				} else {
					observation.reason = agentruntime.ConversationProfileUnavailable
				}
			}
			observations[pane.Key] = observation
		}
	}
	return observations
}

// Captured names, labels, cwd and raw process/title facts may advance during
// discovery. Membership, links, layout and selection must remain unchanged.
func sameRecoveryGraph(before, after tmuxclient.Workspace) bool {
	if len(before.Sessions) != len(after.Sessions) || len(before.Windows) != len(after.Windows) {
		return false
	}
	for i, left := range before.Sessions {
		right := after.Sessions[i]
		if left.Key != right.Key || left.Character != right.Character || left.NativeGroup != right.NativeGroup || left.ActiveWindowIndex != right.ActiveWindowIndex || !slices.Equal(left.Links, right.Links) {
			return false
		}
	}
	for i, left := range before.Windows {
		right := after.Windows[i]
		if left.Key != right.Key || left.Width != right.Width || left.Height != right.Height || left.Layout != right.Layout || left.ActivePaneKey != right.ActivePaneKey || left.Zoomed != right.Zoomed || len(left.Panes) != len(right.Panes) {
			return false
		}
		for j, pane := range left.Panes {
			if pane.Key != right.Panes[j].Key {
				return false
			}
		}
	}
	return true
}

func planRecoveryStartups(recipe *checkpoint, settings *runtimeSettings) (map[string]tmuxclient.WorkspaceStartup, map[string]checkpointAgent, api.RecoveryResume) {
	startup := make(map[string]tmuxclient.WorkspaceStartup)
	dispatched := make(map[string]checkpointAgent)
	var counts api.RecoveryResume
	keys := make([]string, 0, len(recipe.Agents))
	for key := range recipe.Agents {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		// tmux admits unbounded digit strings; numeric ordering needs no integer conversion.
		left, right := strings.TrimLeft(keys[i][1:], "0"), strings.TrimLeft(keys[j][1:], "0")
		if len(left) != len(right) {
			return len(left) < len(right)
		}
		if left != right {
			return left < right
		}
		return keys[i] < keys[j]
	})
	used := make(map[string]bool)
	for _, key := range keys {
		agent := recipe.Agents[key]
		target := agent.Resume
		reason, duplicate := "", false
		var profile agentruntime.Profile
		if target == nil {
			reason = "no id saved"
		} else {
			var found bool
			profile, found = settings.byKey[target.Profile]
			switch {
			case !found:
				reason = "profile removed"
			case profile.Provider != agent.Provider:
				reason = "provider/home changed"
			default:
				scope, err := agentruntime.AccountScope(profile)
				info, statErr := os.Stat(profile.Home)
				switch {
				case err != nil || statErr != nil || !info.IsDir() || syscall.Access(profile.Home, 5) != nil:
					reason = "home unavailable"
				case scope != target.AccountScope:
					reason = "provider/home changed"
				case profile.ResumeCommand == "":
					reason = "no resume command"
				}
			}
			if reason == "" {
				identity := string(agent.Provider) + "\x00" + target.AccountScope + "\x00" + target.ID
				if used[identity] {
					reason, duplicate = "conversation assigned to another restored pane", true
				} else {
					used[identity] = true
				}
			}
		}
		var command string
		if reason == "" {
			command = "set -- '" + target.ID + "'\n" + profile.ResumeCommand
			dispatched[key] = agent
			counts.RequestedPanes++
			if target.Source == agentruntime.ConversationTitleMatch {
				counts.InferredPanes++
			}
		} else {
			message := "skidbladnir: conversation resume skipped: " + reason + "."
			if !duplicate {
				if target != nil {
					message += "\nsaved conversation id: " + target.ID + "."
				}
				message += "\nresume manually in this shell using the original provider profile."
			}
			command = "printf '%s\\n' '" + strings.ReplaceAll(message, "'", "'\\''") + "'"
			counts.SkippedAgentPanes++
		}
		startup[key] = tmuxclient.WorkspaceStartup{Launch: base64.RawURLEncoding.EncodeToString([]byte(command)), MayChangeCwd: reason == ""}
	}
	return startup, dispatched, counts
}
