package sessions

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const maximumCheckpointBytes = 8 << 20

var (
	errCheckpointInvalid = errors.New("workspace checkpoint is invalid or unreadable")
	errCheckpointLimit   = errors.New("workspace checkpoint exceeds the supported limit")
	bootIdentityPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type checkpointError struct {
	operation string
	err       error
}

func (failure *checkpointError) Error() string {
	return "workspace checkpoint " + failure.operation + " failed"
}
func (failure *checkpointError) Unwrap() error { return failure.err }

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
	BootID               string           `json:"bootId"`
	ProcessStartIdentity string           `json:"processStartIdentity"`
	Server               checkpointServer `json:"server"`
}

type checkpointServer struct {
	Epoch     string `json:"epoch"`
	PID       string `json:"pid"`
	StartTime string `json:"startTime"`
}

func (recipe checkpoint) workspace() tmuxclient.Workspace {
	return tmuxclient.Workspace{
		Server:   tmuxclient.ServerIdentity{Epoch: recipe.Source.Server.Epoch, PID: recipe.Source.Server.PID, StartTime: recipe.Source.Server.StartTime},
		Sessions: recipe.Sessions, Windows: recipe.Windows,
	}
}

func (manager *Manager) acquireRecoveryLock() (*os.File, error) {
	directory := filepath.Dir(manager.recovery.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, &checkpointError{"mkdir", err}
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, &checkpointError{"chmod", err}
	}
	lock, err := os.OpenFile(filepath.Join(directory, "workspace.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, &checkpointError{"open", err}
	}
	if err := lock.Chmod(0600); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		return nil, &checkpointError{"chmod", err}
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close() // justify-ignore-error: ownership was not acquired; that failure is authoritative.
		return nil, &checkpointError{"lock", err}
	}
	return lock, nil
}

func (manager *Manager) readCheckpoint() (*checkpoint, error) {
	file, err := os.Open(manager.recovery.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &checkpointError{"open", err}
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close() // justify-ignore-error: the stat failure is authoritative.
		return nil, &checkpointError{"stat", err}
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		_ = file.Close() // justify-ignore-error: invalid checkpoint storage is authoritative.
		return nil, errCheckpointInvalid
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, maximumCheckpointBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, &checkpointError{"read", readErr}
	}
	if closeErr != nil {
		return nil, &checkpointError{"close", closeErr}
	}
	if len(encoded) > maximumCheckpointBytes {
		return nil, errCheckpointLimit
	}
	var recipe checkpoint
	if strictjson.Decode(encoded, &recipe) != nil || !checkpointFieldsPresent(encoded) {
		return nil, errCheckpointInvalid
	}
	if err := manager.validateCheckpoint(recipe); err != nil {
		return nil, err
	}
	return &recipe, nil
}

// writeCheckpoint returns success only after the directory entry is durable.
// The caller retains its last acknowledged fact when replacement is uncertain.
func (manager *Manager) writeCheckpoint(recipe checkpoint) (resultErr error) {
	encoded, err := json.Marshal(recipe)
	if err != nil {
		panic("admitted workspace checkpoint cannot be encoded") // justify-defect: the admitted schema contains only json-supported values and valid timestamps.
	}
	if len(encoded) > maximumCheckpointBytes {
		return errCheckpointLimit
	}
	directory := filepath.Dir(manager.recovery.path)
	file, err := os.CreateTemp(directory, ".workspace-*")
	if err != nil {
		return &checkpointError{"create", err}
	}
	name := file.Name()
	defer func() {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) && resultErr == nil {
			resultErr = &checkpointError{"cleanup", err}
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close() // justify-ignore-error: the write failure is authoritative; nothing was published.
		return &checkpointError{"write", err}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close() // justify-ignore-error: the sync failure is authoritative; nothing was published.
		return &checkpointError{"file_sync", err}
	}
	if err := file.Close(); err != nil {
		return &checkpointError{"close", err}
	}
	if err := os.Rename(name, manager.recovery.path); err != nil {
		return &checkpointError{"rename", err}
	}
	parent, err := os.Open(directory)
	if err != nil {
		return &checkpointError{"directory_open", err}
	}
	syncErr := parent.Sync()
	closeErr := parent.Close()
	if syncErr != nil {
		return &checkpointError{"directory_sync", syncErr}
	}
	if closeErr != nil {
		return &checkpointError{"directory_close", closeErr}
	}
	return nil
}

func (manager *Manager) validateCheckpoint(recipe checkpoint) error {
	server := recipe.workspace().Server
	pid, err := strconv.ParseInt(server.PID, 10, 32)
	start, startErr := strconv.ParseUint(recipe.Source.ProcessStartIdentity, 10, 64)
	if recipe.Schema != 1 || recipe.Machine != manager.recovery.machine.String() || !validServerIdentity(server) ||
		err != nil || pid <= 0 || len(server.StartTime) > 20 || startErr != nil || start == 0 || !bootIdentityPattern.MatchString(recipe.Source.BootID) ||
		!serverStartPattern.MatchString(recipe.Source.ProcessStartIdentity) || len(recipe.Source.ProcessStartIdentity) > 20 ||
		!ValidProjectionInstant(recipe.SavedAt) || recipe.SavedAt.Location() != time.UTC {
		return errCheckpointInvalid
	}
	if err := tmuxclient.ValidateWorkspace(recipe.workspace()); err != nil {
		if errors.Is(err, tmuxclient.WorkspaceLimit) {
			return errCheckpointLimit
		}
		return errCheckpointInvalid
	}
	for _, session := range recipe.Sessions {
		if _, valid := manager.catalogue.Character(session.Character); !valid {
			return errCheckpointInvalid
		}
		if _, err := group.Parse(session.Group); err != nil {
			return errCheckpointInvalid
		}
	}
	for _, window := range recipe.Windows {
		for _, pane := range window.Panes {
			if !filepath.IsAbs(pane.CWD) || filepath.Clean(pane.CWD) != pane.CWD {
				return errCheckpointInvalid
			}
			if _, err := manager.workdir.ParseStart(pane.CWD); err != nil {
				return errCheckpointInvalid
			}
		}
	}
	return nil
}

// strictjson closes syntax and unknown/duplicate fields. This finite schema
// check additionally rejects null and omitted required false/zero/empty arrays.
func checkpointFieldsPresent(encoded []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || token == nil {
			return false
		}
	}
	root, valid := checkpointObject(encoded, "schema", "machine", "savedAt", "source", "attempted", "sessions", "windows")
	if !valid {
		return false
	}
	var savedAt string
	if json.Unmarshal(root["savedAt"], &savedAt) != nil {
		return false
	}
	if _, err := ParseProjectionInstant(savedAt); err != nil {
		return false
	}
	source, valid := checkpointObject(root["source"], "bootId", "processStartIdentity", "server")
	if !valid {
		return false
	}
	if _, valid := checkpointObject(source["server"], "epoch", "pid", "startTime"); !valid {
		return false
	}
	var sessions, windows []json.RawMessage
	if json.Unmarshal(root["sessions"], &sessions) != nil || json.Unmarshal(root["windows"], &windows) != nil {
		return false
	}
	for _, session := range sessions {
		value, valid := checkpointObject(session, "key", "name", "character", "activeWindowIndex", "links")
		if !valid {
			return false
		}
		for _, optional := range []string{"group", "nativeGroup"} {
			if field, present := value[optional]; present {
				var text string
				if json.Unmarshal(field, &text) != nil || text == "" {
					return false
				}
			}
		}
		var links []json.RawMessage
		if json.Unmarshal(value["links"], &links) != nil {
			return false
		}
		for _, link := range links {
			if _, valid := checkpointObject(link, "index", "windowKey"); !valid {
				return false
			}
		}
	}
	for _, window := range windows {
		value, valid := checkpointObject(window, "key", "name", "width", "height", "layout", "activePaneKey", "zoomed", "panes")
		if !valid {
			return false
		}
		var panes []json.RawMessage
		if json.Unmarshal(value["panes"], &panes) != nil {
			return false
		}
		for _, pane := range panes {
			if _, valid := checkpointObject(pane, "key", "cwd"); !valid {
				return false
			}
		}
	}
	return true
}

func checkpointObject(encoded []byte, required ...string) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(encoded, &object) != nil || object == nil {
		return nil, false
	}
	for _, key := range required {
		if _, present := object[key]; !present {
			return nil, false
		}
	}
	return object, true
}
