package process

import (
	"bytes"
	"errors"
	"path/filepath"
)

// ErrProcessAbsent and ErrProcessNotPermitted are the closed set of stable
// per-process observation failures. Every other observation error is a
// transient or malformed kernel read.
var (
	ErrProcessAbsent       = errors.New("process is absent")
	ErrProcessNotPermitted = errors.New("process is outside the caller's observation boundary")
	ErrForegroundMismatch  = errors.New("foreground process changed")
)

type PID int
type StartIdentity string
type TerminalDevice uint64

// Observation is one kernel sample of a process. Its identity is PID and
// StartIdentity, which exec keeps, so Executable is the only witness of an exec.
// It is comparable: == is sample equality.
type Observation struct {
	PID                    PID
	ParentPID              PID
	ProcessGroup           PID
	SessionID              PID
	TerminalDevice         TerminalDevice
	ForegroundProcessGroup PID
	Executable             string
	StartIdentity          StartIdentity
}

func (observation Observation) ExecutableBase() string { return filepath.Base(observation.Executable) }

// Observe is one kernel sample of pid, never retried: a transient read error
// fails it. Exec keeps PID and StartIdentity, so one sample proves identity
// (recovery's server check); it can mix two images' other fields, so a
// foreground decision requires a bracket that closes with an equal sample.
func Observe(pid PID) (Observation, error) {
	if pid <= 0 {
		return Observation{}, errors.New("invalid process id")
	}
	return observeOnce(pid)
}

func ObserveForeground(panePID PID) (Observation, error) {
	foreground, err := foregroundProcessGroup(panePID)
	if err != nil {
		return Observation{}, err
	}
	return Observe(foreground)
}

// ObserveForegroundEnvironment reads expected.PID's allowlisted environment,
// then requires terminalPID's foreground to still equal expected. expected must
// be a sample the caller has just taken: it opens the bracket this closes.
func ObserveForegroundEnvironment(terminalPID PID, expected Observation) (map[string]string, error) {
	environment, readErr := observeEnvironment(expected.PID)
	if after, err := ObserveForeground(terminalPID); err != nil || after != expected {
		return nil, ErrForegroundMismatch
	}
	return environment, readErr
}

// ObserveCurrentDirectory reads pid's cwd, then requires pid to still equal
// expected, the caller's fresh sample. A stable missing cwd is the read error.
func ObserveCurrentDirectory(pid PID, expected Observation) (string, error) {
	cwd, readErr := observeCurrentDirectory(pid)
	if after, err := Observe(pid); err != nil || after != expected {
		return "", ErrForegroundMismatch
	}
	return cwd, readErr
}

func allowedEnvironment(contents []byte) (map[string]string, error) {
	if len(contents) > 1<<20 || len(contents) > 0 && contents[len(contents)-1] != 0 {
		return nil, errors.New("process environment is incomplete or too large")
	}
	result := make(map[string]string, 3)
	if len(contents) == 0 {
		return result, nil
	}
	for _, entry := range bytes.Split(contents[:len(contents)-1], []byte{0}) {
		name, value, found := bytes.Cut(entry, []byte{'='})
		if !found {
			return nil, errors.New("process environment has an incomplete record")
		}
		key := string(name)
		if key != "CODEX_HOME" && key != "CLAUDE_CONFIG_DIR" && key != "HOME" && key != "SKIDBLADNIR_CONNECTION" {
			continue
		}
		if _, exists := result[key]; exists {
			return nil, errors.New("process environment has a duplicate requested key")
		}
		result[key] = string(value)
	}
	return result, nil
}
