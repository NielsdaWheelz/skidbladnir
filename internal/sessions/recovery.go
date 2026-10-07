package sessions

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

type RecoveryState string

const (
	RecoveryTracking  RecoveryState = "tracking"
	RecoveryRecovered RecoveryState = "recovered"
	RecoveryBroken    RecoveryState = "broken"
)

type RecoveryReason string

const (
	RecoveryCheckpointInvalid RecoveryReason = "checkpoint_invalid"
	RecoveryCheckpointFailed  RecoveryReason = "checkpoint_failed"
	RecoveryServerUnreachable RecoveryReason = "server_unreachable"
	RecoveryServerNotEmpty    RecoveryReason = "server_not_empty"
	RecoveryRestoreFailed     RecoveryReason = "restore_failed"
)

// Recovery is a passive capability fact, never saved inventory or a control.
type Recovery struct {
	State   RecoveryState
	SavedAt *time.Time
	Reason  RecoveryReason
}

func (recovery Recovery) Valid() bool {
	if recovery.SavedAt != nil && (!ValidProjectionInstant(*recovery.SavedAt) || recovery.SavedAt.Location() != time.UTC) {
		return false
	}
	switch recovery.State {
	case RecoveryTracking:
		return recovery.Reason == ""
	case RecoveryRecovered:
		return recovery.Reason == "" && recovery.SavedAt != nil
	case RecoveryBroken:
		switch recovery.Reason {
		case RecoveryCheckpointInvalid, RecoveryCheckpointFailed, RecoveryServerUnreachable, RecoveryServerNotEmpty, RecoveryRestoreFailed:
			return true
		}
	}
	return false
}

type workspaceRecovery struct {
	path    string
	machine machine.Handle
	owner   context.Context
	lock    *os.File
	recipe  *checkpoint
	fact    Recovery
}

// StartRecovery runs admission and the first observation before returning.
// Call after listening, before serving requests; call the returned cleanup once
// to cancel and join recovery before releasing this process's writer lock.
func (manager *Manager) StartRecovery(parent context.Context) func() error {
	ctx, cancel := context.WithCancel(parent)
	manager.mutations.Lock()
	if manager.recovery.owner != nil {
		panic("workspace recovery already started") // justify-defect: gateway composition starts one owner per manager lifetime.
	}
	manager.recovery.owner = ctx
	lock, err := manager.acquireRecoveryLock()
	if err != nil {
		manager.breakRecovery(RecoveryCheckpointFailed)
	} else {
		manager.recovery.lock = lock
		recipe, err := manager.readCheckpoint()
		if err != nil {
			manager.breakRecovery(RecoveryCheckpointInvalid)
		} else {
			manager.recovery.recipe = recipe
			if recipe != nil {
				savedAt := recipe.SavedAt
				manager.recovery.fact.SavedAt = &savedAt
			}
			manager.reconcileRecovery()
		}
	}
	manager.mutations.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// justify-polling: capture native tmux changes without connected clients
		// every 30 seconds; gateway cancellation or broken recovery stops work.
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			manager.mutations.RLock()
			broken := manager.recovery.fact.State == RecoveryBroken
			manager.mutations.RUnlock()
			if broken {
				<-ctx.Done()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				manager.mutations.Lock()
				if ctx.Err() == nil {
					manager.reconcileRecovery()
				}
				manager.mutations.Unlock()
			}
		}
	}()
	return func() error {
		cancel()
		<-done
		manager.mutations.Lock()
		defer manager.mutations.Unlock()
		manager.recovery.lock = nil
		if lock != nil && lock.Close() != nil {
			return errors.New("workspace recovery ownership could not be released")
		}
		return nil
	}
}

func (manager *Manager) breakRecovery(reason RecoveryReason) {
	manager.recovery.fact.State = RecoveryBroken
	manager.recovery.fact.Reason = reason
}

// The mutation is already confirmed. A failed save freezes only recovery; it
// cannot turn that confirmation into a retryable or unknown terminal effect.
func (manager *Manager) checkpointAfterMutation(mutation context.Context, expected tmuxclient.ServerIdentity) {
	if manager.recovery.lock == nil || manager.recovery.fact.State == RecoveryBroken {
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	if original, ok := mutation.Deadline(); ok && original.Before(deadline) {
		deadline = original
	}
	ctx, cancel := context.WithDeadline(manager.recovery.owner, deadline)
	defer cancel()
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil || !present || server.Identity != expected || manager.saveCurrentCheckpoint(ctx, expected) != nil {
		manager.breakRecovery(RecoveryCheckpointFailed)
	}
}

// Called only under mutations. Native clients remain independent of this lock.
func (manager *Manager) reconcileRecovery() {
	if manager.recovery.fact.State == RecoveryBroken {
		return
	}
	recipe := manager.recovery.recipe
	if recipe != nil && recipe.Attempted {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	operation, cancelOperation := context.WithTimeout(manager.recovery.owner, 30*time.Second)
	defer cancelOperation()
	ctx, cancel := context.WithTimeout(operation, 10*time.Second)
	defer cancel()
	boot, err := processinfo.BootIdentity()
	if err != nil {
		reason := RecoveryServerUnreachable
		if recipe == nil {
			reason = RecoveryCheckpointFailed
		}
		manager.breakRecovery(reason)
		return
	}
	gone := recipe == nil || boot != recipe.Source.BootID
	if !gone {
		pid, _ := strconv.Atoi(recipe.Source.Server.PID)
		observed, err := processinfo.Observe(processinfo.PID(pid))
		switch {
		case errors.Is(err, processinfo.ErrProcessAbsent):
			gone = true
		case err != nil:
			manager.breakRecovery(RecoveryServerUnreachable)
			return
		default:
			gone = string(observed.StartIdentity) != recipe.Source.ProcessStartIdentity
		}
	}
	server, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		reason := RecoveryCheckpointFailed
		if recipe != nil {
			if gone {
				reason = RecoveryRestoreFailed
			} else {
				reason = RecoveryServerUnreachable
			}
		}
		manager.breakRecovery(reason)
		return
	}
	if recipe != nil && !gone {
		if !present || server.Identity != recipe.workspace().Server {
			manager.breakRecovery(RecoveryServerUnreachable)
			return
		}
	}
	if recipe == nil || !gone || len(recipe.Sessions) == 0 {
		if !present {
			return
		}
		if manager.saveCurrentCheckpoint(ctx, server.Identity) != nil {
			manager.breakRecovery(RecoveryCheckpointFailed)
			return
		}
		return
	}
	if present && server.SessionCount != 0 {
		manager.breakRecovery(RecoveryServerNotEmpty)
		return
	}
	// Reconstruction has its own gateway-owned budget, unaffected by the
	// preceding inventory request or checkpoint observation's deadline.
	restore := operation
	for _, window := range recipe.Windows {
		for _, pane := range window.Panes {
			candidate, err := manager.workdir.ParseCandidate(pane.CWD)
			if err != nil {
				panic("admitted checkpoint cwd became invalid") // justify-defect: readCheckpoint validates every stored cwd through ParseCandidate.
			}
			if _, err := manager.workdir.ValidateStart(candidate); err != nil {
				manager.breakRecovery(RecoveryRestoreFailed)
				return
			}
		}
	}
	if present && manager.tmux.ValidateRecoveryServer(restore, server.Identity) != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	if restore.Err() != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	claimed := *recipe
	claimed.Attempted = true
	if manager.writeCheckpoint(claimed) != nil {
		manager.breakRecovery(RecoveryCheckpointFailed)
		return
	}
	manager.recovery.recipe = &claimed
	if !present && manager.tmux.StartRecoveryServer(restore) != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	if !present {
		server, present, err = manager.tmux.InspectServer(restore)
		if err != nil || !present {
			manager.breakRecovery(RecoveryRestoreFailed)
			return
		}
	}
	destination, err := manager.admitRecoveryServer(restore, server.Identity)
	if err != nil || manager.tmux.ValidateRecoveryServer(restore, destination) != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	if _, err := manager.tmux.RestoreWorkspace(restore, destination, recipe.workspace()); err != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	// RestoreWorkspace verifies topology and metadata. This fresh capture adds
	// kernel lifetime facts and independently admits every observed local cwd.
	saved, err := manager.captureCheckpoint(restore, destination)
	if err != nil {
		manager.breakRecovery(RecoveryRestoreFailed)
		return
	}
	if manager.writeCheckpoint(saved) != nil {
		manager.breakRecovery(RecoveryCheckpointFailed)
		return
	}
	manager.commitCheckpoint(saved, RecoveryRecovered)
}

func (manager *Manager) commitCheckpoint(recipe checkpoint, state RecoveryState) {
	manager.recovery.recipe = &recipe
	savedAt := recipe.SavedAt
	manager.recovery.fact = Recovery{State: state, SavedAt: &savedAt}
}

func (manager *Manager) saveCurrentCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) error {
	saved, err := manager.captureCheckpoint(ctx, expected)
	if err != nil {
		return err
	}
	previous := manager.recovery.recipe
	if previous != nil && len(previous.Sessions) != 0 && previous.Source != saved.Source {
		return errors.New("workspace source lifetime changed")
	}
	if err := manager.writeCheckpoint(saved); err != nil {
		return err
	}
	manager.commitCheckpoint(saved, RecoveryTracking)
	return nil
}

func (manager *Manager) admitRecoveryServer(ctx context.Context, expected tmuxclient.ServerIdentity) (tmuxclient.ServerIdentity, error) {
	proposed, err := newServerEpoch()
	if err != nil {
		return tmuxclient.ServerIdentity{}, err
	}
	return manager.tmux.EnsureServerIdentity(ctx, expected, proposed)
}

func (manager *Manager) captureCheckpoint(ctx context.Context, expected tmuxclient.ServerIdentity) (checkpoint, error) {
	server, err := manager.admitRecoveryServer(ctx, expected)
	if err != nil {
		return checkpoint{}, err
	}
	pid, err := strconv.Atoi(server.PID)
	if err != nil {
		return checkpoint{}, err
	}
	before, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, err
	}
	scan, err := manager.scanSessions(ctx)
	if err != nil {
		return checkpoint{}, err
	}
	visible, err := manager.normalizeCharacters(ctx, scan, server)
	if err != nil {
		return checkpoint{}, err
	}
	if _, err := manager.normalizeNames(ctx, visible, scan.names, server); err != nil {
		return checkpoint{}, err
	}
	workspace, present, err := manager.tmux.CaptureWorkspace(ctx)
	if err != nil || !present || workspace.Server != server {
		return checkpoint{}, errors.New("workspace capture source changed or could not be observed")
	}
	boot, err := processinfo.BootIdentity()
	if err != nil {
		return checkpoint{}, err
	}
	observed, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return checkpoint{}, err
	}
	if observed.StartIdentity != before.StartIdentity {
		return checkpoint{}, errors.New("workspace capture process lifetime changed")
	}
	if err := manager.requireServerIdentity(ctx, server); err != nil {
		return checkpoint{}, err
	}
	recipe := checkpoint{
		Schema: 1, Machine: manager.recovery.machine.String(), SavedAt: time.Now().UTC(),
		Source:   checkpointSource{BootID: boot, ProcessStartIdentity: string(observed.StartIdentity), Server: checkpointServer{Epoch: server.Epoch, PID: server.PID, StartTime: server.StartTime}},
		Sessions: workspace.Sessions, Windows: workspace.Windows,
	}
	if err := manager.validateCheckpoint(recipe); err != nil {
		return checkpoint{}, err
	}
	return recipe, nil
}
