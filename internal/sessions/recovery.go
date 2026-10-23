package sessions

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"syscall"
	"time"

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
	} else {
		manager.reconcileRecovery()
	}
	manager.release()

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
			if manager.acquire(ctx) != nil {
				return
			}
			manager.reconcileRecovery()
			manager.release()
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
	if manager.recovery.lock == nil || manager.recovery.fact.State == api.RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	if original, ok := mutation.Deadline(); ok && original.Before(deadline) {
		deadline = original
	}
	ctx, cancel := context.WithDeadline(manager.recovery.owner, deadline)
	defer cancel()
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(tmuxFailure(ctx, api.RecoveryPhaseSource, "server_inspect", err), false)
		return
	}
	if !present {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseUnavailable, operation: "server_inspect"}, false)
		return
	}
	if server.Identity != expected {
		manager.failRecovery(&recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}, false)
		return
	}
	if failure := manager.saveCurrentCheckpoint(ctx, expected); failure != nil {
		manager.failRecovery(failure, false)
	}
}

// Called only under mutations. Native clients remain independent of this lock.
func (manager *Manager) reconcileRecovery() {
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
		if failure := manager.saveCurrentCheckpoint(ctx, server.Identity); failure != nil {
			manager.failRecovery(failure, false)
		}
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
	if err := manager.tmux.WithShell(manager.settings.Load().shellPath).RestoreWorkspace(restore, destination, shell, recipe.workspace()); err != nil {
		manager.failRecovery(tmuxFailure(restore, api.RecoveryPhaseRestore, "restore_execute", err), true)
		return
	}
	saved, failure := manager.captureCheckpoint(restore, destination)
	if failure != nil {
		manager.failRecovery(failure, true)
		return
	}
	if failure := manager.writeCheckpoint(saved); failure != nil {
		manager.failRecovery(failure, true)
		return
	}
	manager.commitCheckpoint(saved, api.RecoveryRecovered)
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

func (manager *Manager) commitCheckpoint(recipe checkpoint, state api.RecoveryState) {
	manager.recovery.recipe = &recipe
	savedAt := recipe.SavedAt
	manager.publishRecovery(api.Recovery{State: state, SavedAt: &savedAt}, "", 0)
}

// saveCurrentCheckpoint never replaces a nonempty recipe of another source, nor
// an empty one whose source still runs.
func (manager *Manager) saveCurrentCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) *recoveryFailure {
	saved, failure := manager.captureCheckpoint(ctx, expected)
	if failure != nil {
		return failure
	}
	if previous := manager.recovery.recipe; previous != nil && previous.Source != saved.Source {
		if len(previous.Sessions) != 0 {
			return &recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}
		}
		gone, failure := sourceGone(previous.Source)
		if failure != nil {
			return failure
		}
		if !gone {
			return &recoveryFailure{phase: api.RecoveryPhaseSource, cause: api.RecoveryCauseChanged, operation: "server_identity"}
		}
	}
	if ctx.Err() != nil {
		return &recoveryFailure{phase: api.RecoveryPhaseCapture, cause: api.RecoveryCauseTimeout, operation: "capture"}
	}
	if failure := manager.writeCheckpoint(saved); failure != nil {
		return failure
	}
	manager.commitCheckpoint(saved, api.RecoveryTracking)
	return nil
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
		Schema: 1, Machine: manager.recovery.machine.String(), SavedAt: time.Now().UTC(),
		Source:   checkpointSource{BootID: boot, ProcessStartIdentity: string(process.StartIdentity), Server: server},
		Sessions: workspace.Sessions, Windows: workspace.Windows,
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
