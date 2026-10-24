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
	At        time.Time
	Recovery  api.Recovery
	Operation string
	Errno     syscall.Errno
}

// recoveryFailure is one failed step: its wire pair, its content-free log
// evidence, and whether native output was malformed (never retried).
type recoveryFailure struct {
	phase     api.RecoveryPhase
	cause     api.RecoveryCause
	operation string
	errno     syscall.Errno
	malformed bool
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
	var native tmuxclient.WorkspaceError
	var errno syscall.Errno
	failure := &recoveryFailure{phase: phase, cause: api.RecoveryCauseUnavailable, operation: operation}
	switch {
	case errors.As(err, &native):
		switch native {
		case tmuxclient.WorkspaceTopologyChanged:
			failure.cause = api.RecoveryCauseChanged
		case tmuxclient.WorkspaceSourceChanged:
			failure.cause = api.RecoveryCauseChanged
			if phase == api.RecoveryPhaseCapture {
				failure.phase = api.RecoveryPhaseSource
			}
		case tmuxclient.WorkspaceUnsupported:
			failure.cause = api.RecoveryCauseInvalid
		case tmuxclient.WorkspaceProtocolInvalid:
			failure.cause, failure.malformed = api.RecoveryCauseInvalid, true
		case tmuxclient.WorkspaceLimit:
			failure.cause = api.RecoveryCauseLimit
		}
	case errors.As(err, &errno):
		return osFailure(phase, operation, err, api.RecoveryCauseIO)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		failure.cause = api.RecoveryCauseTimeout
	}
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
		manager.failRecovery(failure, true)
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

func (manager *Manager) publishRecovery(fact api.Recovery, operation string, errno syscall.Errno) {
	previous := manager.recovery.lastChange
	change := RecoveryChange{At: time.Now().UTC(), Recovery: fact, Operation: operation, Errno: errno}
	manager.recovery.fact = fact
	changed := previous.Recovery.State != fact.State || previous.Operation != operation || previous.Errno != errno
	if previous.Recovery.Failure != nil && fact.Failure != nil {
		changed = changed || previous.Recovery.Failure.Phase != fact.Failure.Phase || previous.Recovery.Failure.Cause != fact.Failure.Cause
	}
	manager.recovery.lastChange = change
	if changed && !(previous.Recovery.State == "" && fact.State == api.RecoveryTracking) {
		manager.recovery.observe(change)
	}
}

// failRecovery publishes failure. latch makes it broken whatever its phase
// (startup admission, every restore step). Otherwise the wire's own rule
// decides: capture, checkpoint_write and source/timeout are delayed unless
// native output was malformed.
func (manager *Manager) failRecovery(failure *recoveryFailure, latch bool) {
	if manager.recovery.owner.Err() != nil {
		return
	}
	state := api.RecoveryBroken
	if !latch && !failure.malformed && (failure.phase == api.RecoveryPhaseCapture || failure.phase == api.RecoveryPhaseCheckpointWrite || failure.phase == api.RecoveryPhaseSource && failure.cause == api.RecoveryCauseTimeout) {
		state = api.RecoveryDelayed
	}
	manager.publishRecovery(api.Recovery{State: state, SavedAt: manager.recovery.fact.SavedAt, Failure: &api.RecoveryFailure{At: time.Now().UTC(), Phase: failure.phase, Cause: failure.cause}}, failure.operation, failure.errno)
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
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCheckpointRead, cause: api.RecoveryCauseAlreadyAttempted, operation: "validate"}, true)
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
			manager.failRecovery(failure, false)
			return
		}
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(tmuxFailure(ctx, api.RecoveryPhaseSource, "server_inspect", err), false)
		return
	}
	if recipe != nil && !gone {
		if !present {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseUnavailable, operation: "server_inspect"}, false)
			return
		}
		if server.Identity != recipe.Source.Server {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, false)
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
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseNotEmpty, operation: "restore_preflight"}, true)
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
				manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: cause, operation: "restore_preflight"}, true)
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
			manager.failRecovery(failure, true)
			return
		}
	}
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_preflight"}, true)
		return
	}
	settings := manager.settings.Load()
	startup, dispatched, counts := planRecoveryStartups(recipe, settings)
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_preflight"}, true)
		return
	}
	claimed := *recipe
	claimed.Attempted = true
	if failure := manager.writeCheckpoint(claimed); failure != nil {
		manager.failRecovery(failure, true)
		return
	}
	manager.recovery.recipe = &claimed
	if started {
		if err := manager.tmux.StartRecoveryServer(restore); err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseRestore, "restore_execute", err), true)
			return
		}
		server, present, err = manager.tmux.InspectServer(restore)
		if err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseSource, "server_inspect", err), true)
			return
		}
		if !present {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseUnavailable, operation: "server_inspect"}, true)
			return
		}
	}
	destination := server.Identity
	if destination.Epoch == "" {
		destination, err = manager.tmux.EnsureServerIdentity(restore, destination, newServerEpoch())
		if err != nil {
			manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseSource, "server_identity", err), true)
			return
		}
	}
	if started {
		var failure *recoveryFailure
		shell, failure = manager.destinationShell(restore, destination)
		if failure != nil {
			manager.failRecovery(failure, true)
			return
		}
	}
	paneMap, err := manager.tmux.WithShell(settings.shellPath).RestoreWorkspace(restore, destination, shell, recipe.workspace(), startup)
	if err != nil {
		manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseRestore, "restore_execute", err), true)
		return
	}
	saved, failure := manager.captureCheckpoint(restore, destination)
	if failure != nil {
		manager.failRecovery(failure, true)
		return
	}
	for oldKey, agent := range dispatched {
		saved.Agents[paneMap[oldKey]] = agent
	}
	if restore.Err() != nil {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseRestore, cause: api.RecoveryCauseTimeout, operation: "restore_execute"}, true)
		return
	}
	if failure := manager.writeCheckpoint(saved); failure != nil {
		manager.failRecovery(failure, true)
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
	manager.publishRecovery(api.Recovery{State: state, SavedAt: &savedAt, Resume: resume}, "", 0)
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
	saved, failure := manager.captureCheckpoint(ctx, expected)
	if failure != nil {
		manager.failRecovery(failure, false)
		return
	}
	if anchor != nil && anchor.Source != saved.Source {
		if len(anchor.Sessions) != 0 {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, false)
			return
		}
		gone, failure := sourceGone(anchor.Source)
		if failure != nil {
			manager.failRecovery(failure, false)
			return
		}
		if !gone {
			manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, false)
			return
		}
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
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseChanged, operation: "capture"}, false)
		return
	}
	fresh, failure := manager.captureCheckpoint(ctx, saved.Source.Server)
	if failure != nil {
		manager.failRecovery(failure, false)
		return
	}
	if fresh.Source != saved.Source || !sameRecoveryGraph(saved.workspace(), fresh.workspace()) {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseChanged, operation: "capture"}, false)
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
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseTimeout, operation: "capture"}, false)
		return
	}
	if failure := manager.writeCheckpoint(fresh); failure != nil {
		manager.failRecovery(failure, false)
		return
	}
	manager.commitCheckpoint(fresh, api.RecoveryTracking, nil)
}

// captureCheckpoint captures expected's normalized workspace. B == E == server
// proves one lifetime for the whole batch; that lifetime was already observed
// live before the process read, so the pid observed is its process.
func (manager *Manager) captureCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) (checkpoint, *recoveryFailure) {
	server := expected
	if server.Epoch == "" {
		var err error
		server, err = manager.tmux.EnsureServerIdentity(ctx, expected, newServerEpoch())
		if err != nil {
			return checkpoint{}, tmuxFailure(ctx, api.RecoveryPhaseSource, "server_identity", err)
		}
	}
	pid, _ := strconv.Atoi(server.PID)
	process, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, osFailure(api.RecoveryPhaseSource, "process_observe", err, api.RecoveryCauseUnavailable)
	}
	capture, err := manager.tmux.ReadInventory(ctx, server)
	if err != nil {
		return checkpoint{}, tmuxFailure(ctx, api.RecoveryPhaseCapture, "normalize", err)
	}
	scan := manager.scanCapture(capture)
	rows, err := manager.normalizeCharacters(ctx, scan, server)
	if err == nil {
		rows, err = manager.normalizeNames(ctx, rows, scan.names, server)
	}
	if err != nil {
		return checkpoint{}, tmuxFailure(ctx, api.RecoveryPhaseCapture, "normalize", err)
	}
	ids := make([]string, len(rows))
	for index, row := range rows {
		ids[index] = row.ID
	}
	workspace, err := manager.tmux.CaptureWorkspace(ctx, server, ids)
	if err != nil {
		return checkpoint{}, tmuxFailure(ctx, api.RecoveryPhaseCapture, "capture", err)
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
