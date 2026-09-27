package terminalcontext

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
)

var ErrUnavailable = errors.New("terminal context is unavailable")

var connectionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type record struct {
	ConnectionID string                 `json:"connectionId"`
	BootID       string                 `json:"bootId"`
	PID          process.PID            `json:"pid"`
	Start        process.StartIdentity  `json:"startIdentity"`
	TTY          process.TerminalDevice `json:"tty"`
}

func NewConnectionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
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
	lock, err := lockDirectory(directory)
	if err != nil {
		return err
	}
	defer lock.Close()
	count, err := prune(directory, boot)
	if err != nil {
		return err
	}
	if count >= 256 {
		return ErrUnavailable
	}
	value, err := json.Marshal(record{ConnectionID: connectionID, BootID: boot, PID: root.PID, Start: root.StartIdentity, TTY: root.TerminalDevice})
	if err != nil || len(value) > 1024 {
		return ErrUnavailable
	}
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

func Observe(connectionID string) (process.Observation, error) {
	if !ValidConnectionID(connectionID) {
		return process.Observation{}, ErrUnavailable
	}
	directory, err := contextDirectory()
	if err != nil {
		return process.Observation{}, err
	}
	boot, err := process.BootIdentity()
	if err != nil {
		return process.Observation{}, err
	}
	lock, err := lockDirectory(directory)
	if err != nil {
		return process.Observation{}, ErrUnavailable
	}
	defer lock.Close()
	if _, err := prune(directory, boot); err != nil {
		return process.Observation{}, err
	}
	registration, err := readRecord(filepath.Join(directory, connectionID+".json"))
	if err != nil {
		return process.Observation{}, ErrUnavailable
	}
	if registration.ConnectionID != connectionID || registration.BootID != boot {
		return process.Observation{}, ErrUnavailable
	}
	root, err := process.Observe(registration.PID)
	if err != nil || root.StartIdentity != registration.Start || root.TerminalDevice != registration.TTY || root.SessionID != root.PID || root.TerminalDevice == 0 {
		return process.Observation{}, ErrUnavailable
	}
	return root, nil
}

func lockDirectory(directory string) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(directory, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func contextDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", errors.New("home directory is unavailable")
	}
	return filepath.Join(home, ".cache", "skidbladnir", "terminal-contexts"), nil
}

func prune(directory, boot string) (int, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || !ValidConnectionID(strings.TrimSuffix(name, ".json")) {
			continue
		}
		path := filepath.Join(directory, name)
		registration, readErr := readRecord(path)
		if readErr == nil && registration.ConnectionID == strings.TrimSuffix(name, ".json") && registration.BootID == boot {
			root, observeErr := process.Observe(registration.PID)
			if observeErr == nil && root.StartIdentity == registration.Start && root.TerminalDevice == registration.TTY && root.SessionID == root.PID && root.TerminalDevice != 0 {
				count++
				continue
			}
		}
		_ = os.Remove(path)
	}
	return count, nil
}

func readRecord(path string) (record, error) {
	file, err := os.Open(path)
	if err != nil {
		return record{}, err
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, 1025))
	if err != nil || len(value) > 1024 {
		return record{}, ErrUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var registration record
	if err := decoder.Decode(&registration); err != nil {
		return record{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return record{}, ErrUnavailable
	}
	return registration, nil
}
