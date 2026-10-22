package terminalcontext

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
)

var ErrUnavailable = errors.New("terminal context is unavailable")

var connectionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// record names one connection's session-leader root on one boot. The file name
// carries the connection id.
type record struct {
	BootID string                 `json:"bootId"`
	PID    process.PID            `json:"pid"`
	Start  process.StartIdentity  `json:"startIdentity"`
	TTY    process.TerminalDevice `json:"tty"`
}

func NewConnectionID() string {
	var value [16]byte
	rand.Read(value[:]) // never fails since go 1.24
	return hex.EncodeToString(value[:])
}

func ValidConnectionID(value string) bool { return connectionPattern.MatchString(value) }

func Register(connectionID string, shellPID process.PID) error {
	if !ValidConnectionID(connectionID) {
		return errors.New("invalid connection id")
	}
	shell, err := process.Observe(shellPID)
	if err != nil {
		return err
	}
	root, err := process.Observe(shell.SessionID)
	if err != nil {
		return err
	}
	helper, err := process.Observe(process.PID(os.Getpid()))
	if err != nil {
		return err
	}
	if root.PID != root.SessionID || root.TerminalDevice == 0 || shell.TerminalDevice != root.TerminalDevice || helper.ParentPID != shell.PID || helper.SessionID != root.SessionID || helper.TerminalDevice != root.TerminalDevice {
		return ErrUnavailable
	}
	boot, err := process.BootIdentity()
	if err != nil {
		return err
	}
	directory, err := contextDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return err
	}
	if err := prune(directory, boot); err != nil {
		return err
	}
	value, _ := json.Marshal(record{BootID: boot, PID: root.PID, Start: root.StartIdentity, TTY: root.TerminalDevice}) // justify-ignore-error: strings and integers always encode.
	file, err := os.CreateTemp(directory, ".new-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(value); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(directory, connectionID+".json"))
}

// Observe returns the live root registered for connectionID on this boot.
// connectionID must satisfy ValidConnectionID; the gateway admits it first.
func Observe(connectionID string) (process.Observation, error) {
	directory, err := contextDirectory()
	if err != nil {
		return process.Observation{}, err
	}
	boot, err := process.BootIdentity()
	if err != nil {
		return process.Observation{}, err
	}
	registration, err := readRecord(filepath.Join(directory, connectionID+".json"))
	if err != nil || registration.BootID != boot {
		return process.Observation{}, ErrUnavailable
	}
	root, alive := live(registration)
	if !alive {
		return process.Observation{}, ErrUnavailable
	}
	return root, nil
}

// live reports whether the registered root still runs as the same session
// leader on the same terminal.
func live(registration record) (process.Observation, bool) {
	root, err := process.Observe(registration.PID)
	return root, err == nil && root.StartIdentity == registration.Start && root.TerminalDevice == registration.TTY && root.SessionID == root.PID && root.TerminalDevice != 0
}

func contextDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", errors.New("home directory is unavailable")
	}
	return filepath.Join(home, ".cache", "skidbladnir", "terminal-contexts"), nil
}

// prune removes records from another boot, unreadable ones and those whose root
// has gone; Register's temporary .new-* files are not records. It runs unlocked:
// Register renames whole records into place, and one connection id registers
// once, so a racing prune sees no record or one whose root is live.
func prune(directory, boot string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		registration, err := readRecord(path)
		if err == nil && registration.BootID == boot {
			if _, alive := live(registration); alive {
				continue
			}
		}
		_ = os.Remove(path) // justify-ignore-error: a concurrent prune may have removed it; the next Register retries.
	}
	return nil
}

func readRecord(path string) (record, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return record{}, err
	}
	var registration record
	err = json.Unmarshal(encoded, &registration)
	return registration, err
}
