package sessions

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type RecoveryState string

const (
	RecoveryTracking  RecoveryState = "tracking"
	RecoveryRecovered RecoveryState = "recovered"
	RecoveryDelayed   RecoveryState = "delayed"
	RecoveryBroken    RecoveryState = "broken"
)

type RecoveryPhase string

const (
	RecoveryPhaseOwnership       RecoveryPhase = "ownership"
	RecoveryPhaseCheckpointRead  RecoveryPhase = "checkpoint_read"
	RecoveryPhaseSource          RecoveryPhase = "source"
	RecoveryPhaseCapture         RecoveryPhase = "capture"
	RecoveryPhaseCheckpointWrite RecoveryPhase = "checkpoint_write"
	RecoveryPhaseRestore         RecoveryPhase = "restore"
)

type RecoveryCause string

const (
	RecoveryCauseTimeout          RecoveryCause = "timeout"
	RecoveryCauseChanged          RecoveryCause = "changed"
	RecoveryCauseUnavailable      RecoveryCause = "unavailable"
	RecoveryCauseInvalid          RecoveryCause = "invalid"
	RecoveryCauseLimit            RecoveryCause = "limit"
	RecoveryCauseBusy             RecoveryCause = "busy"
	RecoveryCausePermission       RecoveryCause = "permission"
	RecoveryCauseNoSpace          RecoveryCause = "no_space"
	RecoveryCauseIO               RecoveryCause = "io"
	RecoveryCauseAlreadyAttempted RecoveryCause = "already_attempted"
	RecoveryCauseNotEmpty         RecoveryCause = "not_empty"
)

type RecoveryFailure struct {
	At    time.Time
	Phase RecoveryPhase
	Cause RecoveryCause
}

func (failure RecoveryFailure) Valid() bool {
	if !ValidProjectionInstant(failure.At) || failure.At.Location() != time.UTC {
		return false
	}
	switch failure.Phase {
	case RecoveryPhaseOwnership:
		return failure.Cause == RecoveryCauseBusy || failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseNoSpace || failure.Cause == RecoveryCauseIO
	case RecoveryPhaseCheckpointRead:
		return failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseIO || failure.Cause == RecoveryCauseInvalid || failure.Cause == RecoveryCauseLimit || failure.Cause == RecoveryCauseAlreadyAttempted
	case RecoveryPhaseSource:
		return failure.Cause == RecoveryCauseTimeout || failure.Cause == RecoveryCauseChanged || failure.Cause == RecoveryCauseUnavailable || failure.Cause == RecoveryCauseInvalid || failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseIO
	case RecoveryPhaseCapture:
		return failure.Cause == RecoveryCauseTimeout || failure.Cause == RecoveryCauseChanged || failure.Cause == RecoveryCauseUnavailable || failure.Cause == RecoveryCauseInvalid || failure.Cause == RecoveryCauseLimit || failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseIO
	case RecoveryPhaseCheckpointWrite:
		return failure.Cause == RecoveryCauseLimit || failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseNoSpace || failure.Cause == RecoveryCauseIO
	case RecoveryPhaseRestore:
		return failure.Cause == RecoveryCauseTimeout || failure.Cause == RecoveryCauseChanged || failure.Cause == RecoveryCauseUnavailable || failure.Cause == RecoveryCauseInvalid || failure.Cause == RecoveryCauseLimit || failure.Cause == RecoveryCausePermission || failure.Cause == RecoveryCauseNoSpace || failure.Cause == RecoveryCauseIO || failure.Cause == RecoveryCauseNotEmpty
	default:
		return false
	}
}

// Recovery is a passive capability fact, never saved inventory or a control.
type Recovery struct {
	State   RecoveryState
	SavedAt *time.Time
	Failure *RecoveryFailure
}

func (recovery Recovery) Valid() bool {
	if recovery.SavedAt != nil && (!ValidProjectionInstant(*recovery.SavedAt) || recovery.SavedAt.Location() != time.UTC) {
		return false
	}
	switch recovery.State {
	case RecoveryTracking:
		return recovery.Failure == nil
	case RecoveryRecovered:
		return recovery.Failure == nil && recovery.SavedAt != nil
	case RecoveryDelayed:
		return recovery.Failure != nil && recovery.Failure.Valid() && (recovery.Failure.Phase == RecoveryPhaseCapture || recovery.Failure.Phase == RecoveryPhaseCheckpointWrite || recovery.Failure.Phase == RecoveryPhaseSource && recovery.Failure.Cause == RecoveryCauseTimeout)
	case RecoveryBroken:
		return recovery.Failure != nil && recovery.Failure.Valid()
	default:
		return false
	}
}

// RecoveryChange carries only content-free evidence; the observer must not reenter Manager.
type RecoveryChange struct {
	At        time.Time
	Recovery  Recovery
	Operation string
	Errno     syscall.Errno
}

func (change RecoveryChange) Valid() bool {
	if !ValidProjectionInstant(change.At) || change.At.Location() != time.UTC || !change.Recovery.Valid() || change.Errno > 1<<31-1 {
		return false
	}
	if change.Recovery.Failure == nil {
		return change.Operation == "" && change.Errno == 0
	}
	switch change.Operation {
	case "", "mkdir", "chmod", "open", "lock", "stat", "read", "close", "create", "write", "file_sync", "rename", "directory_open", "directory_sync", "directory_close", "cleanup", "boot_identity", "process_observe", "server_inspect", "server_identity", "normalize", "capture", "validate", "restore_preflight", "restore_execute":
		return true
	default:
		return false
	}
}

// recoveryError preserves boundary evidence until the owner chooses delayed or broken.
type recoveryError struct {
	phase     RecoveryPhase
	cause     RecoveryCause
	operation string
	errno     syscall.Errno
	broken    bool
	err       error
}

func (failure *recoveryError) Error() string {
	return "workspace recovery " + string(failure.phase) + " failed"
}
func (failure *recoveryError) Unwrap() error { return failure.err }

var errRecoveryUnavailable = errors.New("workspace recovery dependency unavailable")

func recoveryOperationError(ctx context.Context, phase RecoveryPhase, operation string, err error) *recoveryError {
	if errors.Is(err, errSessionServerObservation) {
		phase, operation = RecoveryPhaseSource, "server_identity"
	}
	failure := &recoveryError{phase: phase, operation: operation, err: err}
	var storage *checkpointError
	if errors.As(err, &storage) {
		failure.operation = storage.operation
	}
	var native tmuxclient.WorkspaceError
	var exitError *exec.ExitError
	var commandError *exec.Error
	var sessionError *Error
	directoryCode, directoryError := workdir.ErrorCodeOf(err)
	switch {
	case errors.As(err, &native):
		switch native {
		case tmuxclient.WorkspaceTopologyChanged:
			failure.cause = RecoveryCauseChanged
		case tmuxclient.WorkspaceSourceChanged:
			failure.cause, failure.broken = RecoveryCauseChanged, true
			if phase != RecoveryPhaseRestore {
				failure.phase = RecoveryPhaseSource
			}
		case tmuxclient.WorkspaceUnsupported:
			failure.cause = RecoveryCauseInvalid
		case tmuxclient.WorkspaceProtocolInvalid:
			failure.cause, failure.broken = RecoveryCauseInvalid, true
		case tmuxclient.WorkspaceLimit:
			failure.cause = RecoveryCauseLimit
		default:
			panic("unknown workspace failure") // justify-defect: tmux owns the closed WorkspaceError set.
		}
	case errors.Is(err, errCheckpointInvalid):
		failure.cause, failure.broken = RecoveryCauseInvalid, true
	case errors.Is(err, errCheckpointLimit):
		failure.cause = RecoveryCauseLimit
	case errors.As(err, &sessionError):
		switch sessionError.Code {
		case ErrorSessionIdentityMismatch:
			failure.phase, failure.operation, failure.cause, failure.broken = RecoveryPhaseSource, "server_identity", RecoveryCauseChanged, true
		case ErrorSessionNotFound:
			failure.cause = RecoveryCauseChanged
		default:
			panic("unexpected session failure during recovery") // justify-defect: recovery normalization admits only session disappearance or lifetime mismatch.
		}
	case errors.As(err, &failure.errno):
		switch failure.errno {
		case syscall.EACCES, syscall.EPERM:
			failure.cause = RecoveryCausePermission
		case syscall.ENOSPC, syscall.EDQUOT:
			if phase == RecoveryPhaseOwnership || phase == RecoveryPhaseCheckpointWrite || phase == RecoveryPhaseRestore {
				failure.cause = RecoveryCauseNoSpace
			} else {
				failure.cause = RecoveryCauseIO
			}
		case syscall.EWOULDBLOCK:
			if phase == RecoveryPhaseOwnership && failure.operation == "lock" {
				failure.cause = RecoveryCauseBusy
			} else {
				failure.cause = RecoveryCauseIO
			}
		case syscall.ENOENT:
			if phase == RecoveryPhaseRestore && operation == "restore_preflight" {
				failure.cause = RecoveryCauseUnavailable
			} else {
				failure.cause = RecoveryCauseIO
			}
		default:
			failure.cause = RecoveryCauseIO
		}
	case errors.Is(err, io.ErrShortWrite), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		failure.cause = RecoveryCauseIO
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &exitError) && errors.Is(ctx.Err(), context.DeadlineExceeded):
		failure.cause = RecoveryCauseTimeout
	case errors.Is(err, errRecoveryUnavailable), errors.Is(err, tmuxclient.ErrWorkspaceUnavailable), errors.Is(err, exec.ErrWaitDelay), errors.As(err, &exitError), errors.As(err, &commandError), errors.Is(err, context.Canceled):
		failure.cause = RecoveryCauseUnavailable
	case phase == RecoveryPhaseSource && (operation == "process_observe" || operation == "boot_identity"):
		// These kernel observers also expose malformed or incoherent reads without errno.
		failure.cause = RecoveryCauseUnavailable
	case phase == RecoveryPhaseRestore && operation == "restore_preflight" && directoryError && directoryCode == workdir.Unavailable:
		failure.cause = RecoveryCauseUnavailable
	default:
		panic("unclassified workspace recovery failure") // justify-defect: recovery handles named external failures; internal construction failures remain defects.
	}
	if failure.phase == RecoveryPhaseOwnership || failure.phase == RecoveryPhaseCheckpointRead || failure.phase == RecoveryPhaseRestore || failure.phase == RecoveryPhaseSource && failure.cause != RecoveryCauseTimeout {
		failure.broken = true
	}
	return failure
}

type workspaceRecovery struct {
	path       string
	machine    machine.Handle
	owner      context.Context
	lock       *os.File
	recipe     *checkpoint
	fact       Recovery
	observe    func(RecoveryChange)
	lastChange RecoveryChange
}

// StartRecovery runs admission and the first observation before returning.
// Call after listening, before serving requests; call the returned cleanup once
// to cancel and join recovery before releasing this process's writer lock.
func (manager *Manager) StartRecovery(parent context.Context, observe func(RecoveryChange)) func() error {
	ctx, cancel := context.WithCancel(parent)
	if err := manager.acquire(ctx); err != nil {
		cancel()
		return func() error { return nil }
	}
	if manager.recovery.owner != nil {
		panic("workspace recovery already started") // justify-defect: gateway composition starts one owner per manager lifetime.
	}
	manager.recovery.owner = ctx
	manager.recovery.observe = observe
	lock, err := manager.acquireRecoveryLock()
	if err != nil {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseOwnership, "lock", err), true)
	} else {
		manager.recovery.lock = lock
		recipe, err := manager.readCheckpoint()
		if err != nil {
			manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseCheckpointRead, "validate", err), true)
		} else {
			manager.recovery.recipe = recipe
			if recipe != nil {
				savedAt := recipe.SavedAt
				manager.recovery.fact.SavedAt = &savedAt
			}
			manager.reconcileRecovery()
		}
	}
	manager.release()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// justify-polling: capture native tmux changes without connected clients
		// every 30 seconds; gateway cancellation or broken recovery stops work.
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if err := manager.acquire(ctx); err != nil {
				return
			}
			broken := manager.recovery.fact.State == RecoveryBroken
			manager.release()
			if broken {
				<-ctx.Done()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := manager.acquire(ctx); err != nil {
					return
				}
				if ctx.Err() == nil {
					manager.reconcileRecovery()
				}
				manager.release()
			}
		}
	}()
	return func() error {
		cancel()
		<-done
		if err := manager.acquire(context.Background()); err != nil {
			return err
		}
		defer manager.release()
		manager.recovery.lock = nil
		if lock != nil && lock.Close() != nil {
			return errors.New("workspace recovery ownership could not be released")
		}
		return nil
	}
}

func (manager *Manager) publishRecovery(fact Recovery, operation string, errno syscall.Errno) {
	previous := manager.recovery.lastChange
	change := RecoveryChange{At: time.Now().UTC(), Recovery: fact, Operation: operation, Errno: errno}
	if !change.Valid() {
		panic("invalid workspace recovery fact")
	} // justify-defect: recovery detectors construct the closed fact before publication.
	manager.recovery.fact = fact
	changed := previous.Recovery.State != fact.State || previous.Operation != operation || previous.Errno != errno
	if previous.Recovery.Failure != nil && fact.Failure != nil {
		changed = changed || previous.Recovery.Failure.Phase != fact.Failure.Phase || previous.Recovery.Failure.Cause != fact.Failure.Cause
	}
	manager.recovery.lastChange = change
	if changed && manager.recovery.observe != nil && !(previous.Recovery.State == "" && fact.State == RecoveryTracking) {
		manager.recovery.observe(change)
	}
}

func (manager *Manager) failRecovery(failure *recoveryError, forceBroken bool) {
	if manager.recovery.owner.Err() != nil {
		return
	}
	state := RecoveryDelayed
	if failure.broken || forceBroken {
		state = RecoveryBroken
	}
	manager.publishRecovery(Recovery{State: state, SavedAt: manager.recovery.fact.SavedAt, Failure: &RecoveryFailure{At: time.Now().UTC(), Phase: failure.phase, Cause: failure.cause}}, failure.operation, failure.errno)
}

// The mutation is already confirmed; checkpoint failure never changes its outcome.
func (manager *Manager) checkpointAfterMutation(mutation context.Context, expected tmuxclient.ServerIdentity) {
	if manager.recovery.lock == nil || manager.recovery.fact.State == RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	if original, ok := mutation.Deadline(); ok && original.Before(deadline) {
		deadline = original
	}
	ctx, cancel := context.WithDeadline(manager.recovery.owner, deadline)
	defer cancel()
	if ctx.Err() != nil {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", ctx.Err()), false)
		return
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", err), false)
		return
	}
	if !present {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", errRecoveryUnavailable), false)
		return
	}
	if server.Identity != expected {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceSourceChanged), false)
		return
	}
	if failure := manager.saveCurrentCheckpoint(ctx, expected); failure != nil {
		manager.failRecovery(failure, false)
	}
}

// Called only under mutations. Native clients remain independent of this lock.
func (manager *Manager) reconcileRecovery() {
	if manager.recovery.fact.State == RecoveryBroken || manager.recovery.owner.Err() != nil {
		return
	}
	recipe := manager.recovery.recipe
	if recipe != nil && recipe.Attempted {
		manager.failRecovery(&recoveryError{phase: RecoveryPhaseCheckpointRead, cause: RecoveryCauseAlreadyAttempted, operation: "validate", broken: true}, true)
		return
	}
	operation, cancelOperation := context.WithTimeout(manager.recovery.owner, 30*time.Second)
	defer cancelOperation()
	ctx, cancel := context.WithTimeout(operation, 10*time.Second)
	defer cancel()
	gone := true
	if recipe != nil {
		var failure *recoveryError
		gone, failure = manager.checkpointSourceGone(ctx, recipe.Source)
		if failure != nil {
			manager.failRecovery(failure, false)
			return
		}
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", err), false)
		return
	}
	if recipe != nil && !gone {
		if !present {
			manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", errRecoveryUnavailable), false)
			return
		}
		if server.Identity != recipe.workspace().Server {
			manager.failRecovery(recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceSourceChanged), false)
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
	if present && server.SessionCount != 0 {
		manager.failRecovery(&recoveryError{phase: RecoveryPhaseRestore, cause: RecoveryCauseNotEmpty, operation: "restore_preflight", broken: true}, true)
		return
	}
	restore := operation
	for _, window := range recipe.Windows {
		for _, pane := range window.Panes {
			candidate, err := manager.workdir.ParseCandidate(pane.CWD)
			if err != nil {
				panic("admitted checkpoint cwd became invalid")
			} // justify-defect: checkpoint admission already parsed every cwd.
			if _, err := manager.workdir.ValidateStart(candidate); err != nil {
				manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_preflight", err), true)
				return
			}
		}
	}
	if present {
		if err := manager.tmux.ValidateRecoveryServer(restore, server.Identity); err != nil {
			manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_preflight", err), true)
			return
		}
	}
	if restore.Err() != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_preflight", restore.Err()), true)
		return
	}
	claimed := *recipe
	claimed.Attempted = true
	if err := manager.writeCheckpoint(claimed); err != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseCheckpointWrite, "write", err), true)
		return
	}
	manager.recovery.recipe = &claimed
	if !present {
		if err := manager.tmux.StartRecoveryServer(restore); err != nil {
			manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_execute", err), true)
			return
		}
		server, present, err = manager.tmux.InspectServer(restore)
		if err != nil {
			manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseSource, "server_inspect", err), true)
			return
		}
		if !present {
			manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseSource, "server_inspect", errRecoveryUnavailable), true)
			return
		}
	}
	destination, err := manager.admitRecoveryServer(restore, server.Identity)
	if err != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseSource, "server_identity", err), true)
		return
	}
	if err := manager.tmux.ValidateRecoveryServer(restore, destination); err != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_preflight", err), true)
		return
	}
	if _, err := manager.tmux.RestoreWorkspace(restore, destination, recipe.workspace()); err != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseRestore, "restore_execute", err), true)
		return
	}
	saved, failure := manager.captureCheckpoint(restore, destination)
	if failure != nil {
		manager.failRecovery(failure, true)
		return
	}
	if err := manager.writeCheckpoint(saved); err != nil {
		manager.failRecovery(recoveryOperationError(restore, RecoveryPhaseCheckpointWrite, "write", err), true)
		return
	}
	manager.commitCheckpoint(saved, RecoveryRecovered)
}

func (manager *Manager) checkpointSourceGone(ctx context.Context, source checkpointSource) (bool, *recoveryError) {
	if ctx.Err() != nil {
		return false, recoveryOperationError(ctx, RecoveryPhaseSource, "boot_identity", ctx.Err())
	}
	boot, err := processinfo.BootIdentity()
	if err != nil {
		return false, recoveryOperationError(ctx, RecoveryPhaseSource, "boot_identity", err)
	}
	if boot != source.BootID {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, recoveryOperationError(ctx, RecoveryPhaseSource, "process_observe", ctx.Err())
	}
	pid, _ := strconv.Atoi(source.Server.PID)
	observed, err := processinfo.Observe(processinfo.PID(pid))
	if errors.Is(err, processinfo.ErrProcessAbsent) {
		return true, nil
	}
	if err != nil {
		return false, recoveryOperationError(ctx, RecoveryPhaseSource, "process_observe", err)
	}
	return string(observed.StartIdentity) != source.ProcessStartIdentity, nil
}

func (manager *Manager) commitCheckpoint(recipe checkpoint, state RecoveryState) {
	manager.recovery.recipe = &recipe
	savedAt := recipe.SavedAt
	manager.publishRecovery(Recovery{State: state, SavedAt: &savedAt}, "", 0)
}

func (manager *Manager) saveCurrentCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) *recoveryError {
	saved, failure := manager.captureCheckpoint(ctx, expected)
	if failure != nil {
		return failure
	}
	previous := manager.recovery.recipe
	if previous != nil {
		if previous.Attempted {
			return &recoveryError{phase: RecoveryPhaseCheckpointRead, cause: RecoveryCauseAlreadyAttempted, operation: "validate", broken: true}
		}
		if previous.Source != saved.Source {
			if len(previous.Sessions) != 0 {
				return recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceSourceChanged)
			}
			gone, failure := manager.checkpointSourceGone(ctx, previous.Source)
			if failure != nil {
				return failure
			}
			if !gone {
				return recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceSourceChanged)
			}
		}
	}
	if ctx.Err() != nil {
		return recoveryOperationError(ctx, RecoveryPhaseCapture, "capture", ctx.Err())
	}
	if err := manager.writeCheckpoint(saved); err != nil {
		return recoveryOperationError(ctx, RecoveryPhaseCheckpointWrite, "write", err)
	}
	manager.commitCheckpoint(saved, RecoveryTracking)
	return nil
}

func (manager *Manager) admitRecoveryServer(ctx context.Context, expected tmuxclient.ServerIdentity) (tmuxclient.ServerIdentity, error) {
	proposed, err := newServerEpoch()
	if err != nil {
		panic("workspace recovery epoch entropy unavailable") // justify-defect: a canonical server epoch requires cryptographic entropy, not a retryable observation.
	}
	return manager.tmux.EnsureServerIdentity(ctx, expected, proposed)
}

func (manager *Manager) captureCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) (checkpoint, *recoveryError) {
	if ctx.Err() != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", ctx.Err())
	}
	server, err := manager.admitRecoveryServer(ctx, expected)
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", err)
	}
	pid, err := strconv.ParseInt(server.PID, 10, 32)
	if err != nil || pid <= 0 {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceProtocolInvalid)
	}
	before, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "process_observe", err)
	}
	capture, err := manager.tmux.ReadInventory(ctx, server)
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "normalize", err)
	}
	scan := manager.scanCapture(capture)
	visible, err := manager.normalizeCharacters(ctx, scan, server)
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "normalize", err)
	}
	if _, err := manager.normalizeNames(ctx, visible, scan.names, server); err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "normalize", err)
	}
	workspace, present, err := manager.tmux.CaptureWorkspace(ctx)
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "capture", err)
	}
	if !present {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_inspect", errRecoveryUnavailable)
	}
	if workspace.Server != server {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", tmuxclient.WorkspaceSourceChanged)
	}
	boot, err := processinfo.BootIdentity()
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "boot_identity", err)
	}
	observed, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "process_observe", err)
	}
	if observed.StartIdentity != before.StartIdentity {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "process_observe", tmuxclient.WorkspaceSourceChanged)
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseSource, "server_identity", err)
	}
	recipe := checkpoint{Schema: 1, Machine: manager.recovery.machine.String(), SavedAt: time.Now().UTC(), Source: checkpointSource{BootID: boot, ProcessStartIdentity: string(observed.StartIdentity), Server: checkpointServer{Epoch: server.Epoch, PID: server.PID, StartTime: server.StartTime}}, Sessions: workspace.Sessions, Windows: workspace.Windows}
	if err := manager.validateCheckpoint(recipe); err != nil {
		if errors.Is(err, errCheckpointLimit) {
			return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "validate", err)
		}
		return checkpoint{}, recoveryOperationError(ctx, RecoveryPhaseCapture, "validate", tmuxclient.WorkspaceUnsupported)
	}
	return recipe, nil
}
