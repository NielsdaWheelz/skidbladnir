package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

const coherentObservationAttempts = 8

// ErrProcessAbsent and ErrProcessNotPermitted are the closed set of stable
// per-process observation failures. Every other observation error is a
// transient or malformed kernel read.
var (
	ErrProcessAbsent       = errors.New("process is absent")
	ErrProcessNotPermitted = errors.New("process is outside the caller's observation boundary")
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

func (observation Observation) Argument(index int) string {
	if index < 0 || index >= len(observation.Argv) {
		return ""
	}
	return observation.Argv[index]
}

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
		if hasPrevious && equalObservation(previous, current) {
			return current, nil
		}
		previous, hasPrevious = current, true
	}
	if lastFailure != nil {
		return Observation{}, fmt.Errorf("process observation did not stabilize: %w", lastFailure)
	}
	return Observation{}, errors.New("process observation did not stabilize")
}

func equalObservation(left, right Observation) bool {
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

// TerminalDeviceAt resolves the kernel device identity of an exact character
// device path. Tmux's pane_tty and the hook process observation must name the
// same value before runtime identity may mutate a pane option.
func TerminalDeviceAt(path string) (TerminalDevice, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat terminal device: %w", err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return 0, errors.New("terminal path is not a character device")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Rdev == 0 {
		return 0, errors.New("terminal device identity is unavailable")
	}
	return TerminalDevice(stat.Rdev), nil
}

// ObserveAncestry walks from initial toward PID 1 and returns the observable
// prefix of the chain. The walk ends successfully at PID 1, at a parentless
// process, or at the first ancestor outside the caller's observation boundary
// (a protected or already-exited process); the caller-owned chain the walk
// exists to capture is always complete before that frontier.
func ObserveAncestry(initial PID, limit int) ([]Observation, error) {
	if initial <= 0 || limit <= 0 {
		return nil, errors.New("invalid process ancestry request")
	}
	ancestry := make([]Observation, 0, 8)
	seen := make(map[PID]struct{}, 8)
	pid := initial
	for len(ancestry) < limit {
		if _, exists := seen[pid]; exists {
			return nil, errors.New("process ancestry contains a cycle")
		}
		seen[pid] = struct{}{}
		observation, err := Observe(pid)
		if err != nil {
			if pid != initial && (errors.Is(err, ErrProcessAbsent) || errors.Is(err, ErrProcessNotPermitted)) {
				// justify-ignore-error: an unobservable ancestor is the privilege or
				// lifetime frontier of the walk, and the observed prefix is the
				// modeled successful result; only the initial process must be
				// observable for the walk to mean anything.
				return ancestry, nil
			}
			return nil, err
		}
		ancestry = append(ancestry, observation)
		if observation.ParentPID <= 0 || observation.PID == 1 {
			return ancestry, nil
		}
		pid = observation.ParentPID
	}
	return nil, errors.New("process ancestry exceeds its closed bound")
}

func ObserveForeground(panePID PID) (Observation, error) {
	foreground, err := foregroundProcessGroup(panePID)
	if err != nil {
		return Observation{}, err
	}
	return Observe(foreground)
}
