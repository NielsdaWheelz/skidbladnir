package sessions

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
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
		return nil, osFailure(api.RecoveryPhaseOwnership, "mkdir", err, api.RecoveryCauseIO)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, osFailure(api.RecoveryPhaseOwnership, "chmod", err, api.RecoveryCauseIO)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "workspace.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, osFailure(api.RecoveryPhaseOwnership, "open", err, api.RecoveryCauseIO)
	}
	if err := lock.Chmod(0600); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		return nil, osFailure(api.RecoveryPhaseOwnership, "chmod", err, api.RecoveryCauseIO)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		failure := osFailure(api.RecoveryPhaseOwnership, "lock", err, api.RecoveryCauseIO)
		if errors.Is(err, syscall.EWOULDBLOCK) {
			failure.cause = api.RecoveryCauseBusy
		}
		return nil, failure
	}
	return lock, nil
}

// validSavedAt admits a non-zero instant inside the four-digit UTC years Go's
// RFC3339 JSON encoder writes, so the inventory can always publish it.
func validSavedAt(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	_, err := value.UTC().MarshalJSON()
	return err == nil
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
		return nil, osFailure(api.RecoveryPhaseCheckpointRead, "open", err, api.RecoveryCauseIO)
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maximumCheckpointBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, osFailure(api.RecoveryPhaseCheckpointRead, "read", readErr, api.RecoveryCauseIO)
	}
	if closeErr != nil {
		return nil, osFailure(api.RecoveryPhaseCheckpointRead, "close", closeErr, api.RecoveryCauseIO)
	}
	invalid := &recoveryFailure{phase: api.RecoveryPhaseCheckpointRead, cause: api.RecoveryCauseInvalid, operation: "validate"}
	if len(encoded) > maximumCheckpointBytes {
		invalid.cause = api.RecoveryCauseLimit
		return nil, invalid
	}
	var recipe checkpoint
	if strictjson.Decode(encoded, &recipe) != nil || recipe.Schema != 1 ||
		recipe.Machine != manager.recovery.machine.String() || !validSavedAt(recipe.SavedAt) {
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

// writeCheckpoint returns success only after the replacement is durable. The
// caller retains its last acknowledged fact when replacement is uncertain.
func (manager *Manager) writeCheckpoint(recipe checkpoint) *recoveryFailure {
	encoded, err := json.Marshal(recipe)
	if err != nil {
		return &recoveryFailure{phase: api.RecoveryPhaseCheckpointWrite, cause: api.RecoveryCauseIO, operation: "write"}
	}
	if len(encoded) > maximumCheckpointBytes {
		return &recoveryFailure{phase: api.RecoveryPhaseCheckpointWrite, cause: api.RecoveryCauseLimit, operation: "write"}
	}
	if err := privatefile.WriteFile(manager.recovery.path, encoded); err != nil {
		return osFailure(api.RecoveryPhaseCheckpointWrite, "write", err, api.RecoveryCauseIO)
	}
	return nil
}
