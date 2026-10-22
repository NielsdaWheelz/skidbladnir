package sessions

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const maximumCheckpointBytes = 8 << 20

type checkpoint struct {
	Schema    int                           `json:"schema"`
	Machine   string                        `json:"machine"`
	SavedAt   time.Time                     `json:"savedAt"`
	Source    checkpointSource              `json:"source"`
	Attempted bool                          `json:"attempted"`
	Sessions  []tmuxclient.WorkspaceSession `json:"sessions"`
	Windows   []tmuxclient.WorkspaceWindow  `json:"windows"`
}

type checkpointSource struct {
	BootID               string                    `json:"bootId"`
	ProcessStartIdentity string                    `json:"processStartIdentity"`
	Server               tmuxclient.ServerIdentity `json:"server"`
}

func (recipe checkpoint) workspace() tmuxclient.Workspace {
	return tmuxclient.Workspace{Sessions: recipe.Sessions, Windows: recipe.Windows}
}

func (manager *Manager) acquireRecoveryLock() (*os.File, *recoveryFailure) {
	directory := filepath.Dir(manager.recovery.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, osFailure(RecoveryPhaseOwnership, "mkdir", err, RecoveryCauseIO)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, osFailure(RecoveryPhaseOwnership, "chmod", err, RecoveryCauseIO)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "workspace.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, osFailure(RecoveryPhaseOwnership, "open", err, RecoveryCauseIO)
	}
	if err := lock.Chmod(0600); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		return nil, osFailure(RecoveryPhaseOwnership, "chmod", err, RecoveryCauseIO)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		failure := osFailure(RecoveryPhaseOwnership, "lock", err, RecoveryCauseIO)
		if errors.Is(err, syscall.EWOULDBLOCK) {
			failure.cause = RecoveryCauseBusy
		}
		return nil, failure
	}
	return lock, nil
}

// readCheckpoint admits the checkpoint file once; no file is a nil recipe. It
// refuses what restore could not rebuild and verify. A malformed boot or start
// identity is only compared and reads as a gone source; the restore preflight
// checks that each cwd exists and is canonical.
func (manager *Manager) readCheckpoint() (*checkpoint, *recoveryFailure) {
	file, err := os.Open(manager.recovery.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, osFailure(RecoveryPhaseCheckpointRead, "open", err, RecoveryCauseIO)
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maximumCheckpointBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, osFailure(RecoveryPhaseCheckpointRead, "read", readErr, RecoveryCauseIO)
	}
	if closeErr != nil {
		return nil, osFailure(RecoveryPhaseCheckpointRead, "close", closeErr, RecoveryCauseIO)
	}
	invalid := &recoveryFailure{phase: RecoveryPhaseCheckpointRead, cause: RecoveryCauseInvalid, operation: "validate"}
	if len(encoded) > maximumCheckpointBytes {
		invalid.cause = RecoveryCauseLimit
		return nil, invalid
	}
	var recipe checkpoint
	if strictjson.Decode(encoded, &recipe) != nil || recipe.Schema != 1 ||
		recipe.Machine != manager.recovery.machine.String() || !ValidProjectionInstant(recipe.SavedAt) {
		return nil, invalid
	}
	// sourceGone reads the pid as a number, so only the canonical grammar keeps
	// it naming the saved process.
	source, err := tmuxclient.ParseServerIdentity(recipe.Source.Server.Epoch, recipe.Source.Server.PID, recipe.Source.Server.StartTime)
	if err != nil || source.Epoch == "" || tmuxclient.ValidateWorkspace(recipe.workspace()) != nil || !manager.restorable(recipe.workspace()) {
		return nil, invalid
	}
	return &recipe, nil
}

// writeCheckpoint returns success only after the directory entry is durable.
// The caller retains its last acknowledged fact when replacement is uncertain.
func (manager *Manager) writeCheckpoint(recipe checkpoint) (failure *recoveryFailure) {
	encoded, err := json.Marshal(recipe)
	if err != nil {
		return &recoveryFailure{phase: RecoveryPhaseCheckpointWrite, cause: RecoveryCauseIO, operation: "write"}
	}
	if len(encoded) > maximumCheckpointBytes {
		return &recoveryFailure{phase: RecoveryPhaseCheckpointWrite, cause: RecoveryCauseLimit, operation: "write"}
	}
	directory := filepath.Dir(manager.recovery.path)
	file, err := os.CreateTemp(directory, ".workspace-*")
	if err != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "create", err, RecoveryCauseIO)
	}
	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) && failure == nil {
			failure = osFailure(RecoveryPhaseCheckpointWrite, "cleanup", err, RecoveryCauseIO)
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close() // justify-ignore-error: the write failure is authoritative; nothing was published.
		return osFailure(RecoveryPhaseCheckpointWrite, "write", err, RecoveryCauseIO)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close() // justify-ignore-error: the sync failure is authoritative; nothing was published.
		return osFailure(RecoveryPhaseCheckpointWrite, "file_sync", err, RecoveryCauseIO)
	}
	if err := file.Close(); err != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "close", err, RecoveryCauseIO)
	}
	if err := os.Rename(name, manager.recovery.path); err != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "rename", err, RecoveryCauseIO)
	}
	parent, err := os.Open(directory)
	if err != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "directory_open", err, RecoveryCauseIO)
	}
	syncErr := parent.Sync()
	closeErr := parent.Close()
	if syncErr != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "directory_sync", syncErr, RecoveryCauseIO)
	}
	if closeErr != nil {
		return osFailure(RecoveryPhaseCheckpointWrite, "directory_close", closeErr, RecoveryCauseIO)
	}
	return nil
}
