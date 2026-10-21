package process

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
)

const coherentObservationAttempts = 8

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

type Observation struct {
	PID                    PID
	ParentPID              PID
	ProcessGroup           PID
	SessionID              PID
	TerminalDevice         TerminalDevice
	ForegroundProcessGroup PID
	Executable             string
	Argv                   []string
	StartIdentity          StartIdentity
}

func (observation Observation) ExecutableBase() string { return filepath.Base(observation.Executable) }

func Observe(pid PID) (Observation, error) {
	if pid <= 0 {
		return Observation{}, errors.New("invalid process id")
	}
	var previous Observation
	var lastFailure error
	hasPrevious := false
	for attempt := 0; attempt < coherentObservationAttempts; attempt++ {
		current, err := observeOnce(pid)
		if err != nil {
			if errors.Is(err, ErrProcessAbsent) || errors.Is(err, ErrProcessNotPermitted) {
				return Observation{}, err
			}
			lastFailure = err
			hasPrevious = false
			continue
		}
		if hasPrevious && SameObservation(previous, current) {
			return current, nil
		}
		previous, hasPrevious = current, true
	}
	if lastFailure != nil {
		return Observation{}, fmt.Errorf("process observation did not stabilize: %w", lastFailure)
	}
	return Observation{}, errors.New("process observation did not stabilize")
}

// SameObservation compares every process identity and foreground fact sampled
// by Observe, including executable arguments across exec of the same path.
func SameObservation(left, right Observation) bool {
	return left.PID == right.PID &&
		left.ParentPID == right.ParentPID &&
		left.ProcessGroup == right.ProcessGroup &&
		left.SessionID == right.SessionID &&
		left.TerminalDevice == right.TerminalDevice &&
		left.ForegroundProcessGroup == right.ForegroundProcessGroup &&
		left.Executable == right.Executable &&
		left.StartIdentity == right.StartIdentity &&
		slices.Equal(left.Argv, right.Argv)
}

func ObserveForeground(panePID PID) (Observation, error) {
	foreground, err := foregroundProcessGroup(panePID)
	if err != nil {
		return Observation{}, err
	}
	return Observe(foreground)
}

// ObserveForegroundEnvironment reads only the fields needed for terminal
// context. The caller's observation must still be the terminal foreground
// process on both sides of the native read.
func ObserveForegroundEnvironment(terminalPID PID, expected Observation) (map[string]string, error) {
	before, err := ObserveForeground(terminalPID)
	if err != nil || !SameObservation(before, expected) {
		return nil, ErrForegroundMismatch
	}
	environment, readErr := observeEnvironment(expected.PID)
	after, err := ObserveForeground(terminalPID)
	if err != nil || !SameObservation(after, expected) {
		return nil, ErrForegroundMismatch
	}
	return environment, readErr
}

// ObserveCurrentDirectory brackets the native cwd read with exact process
// identity validation. A stable missing cwd is represented by the read error.
func ObserveCurrentDirectory(pid PID, expected Observation) (string, error) {
	before, err := Observe(pid)
	if err != nil || !SameObservation(before, expected) {
		return "", ErrForegroundMismatch
	}
	cwd, readErr := observeCurrentDirectory(pid)
	after, err := Observe(pid)
	if err != nil || !SameObservation(after, expected) {
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
