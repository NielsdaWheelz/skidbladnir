package process

import (
	"errors"
	"path/filepath"
	"slices"
)

const coherentObservationAttempts = 8

type PID int
type StartIdentity string

type Observation struct {
	PID                    PID
	ParentPID              PID
	ProcessGroup           PID
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
	hasPrevious := false
	for attempt := 0; attempt < coherentObservationAttempts; attempt++ {
		current, err := observeOnce(pid)
		if err != nil {
			hasPrevious = false
			continue
		}
		if hasPrevious && equalObservation(previous, current) {
			return current, nil
		}
		previous, hasPrevious = current, true
	}
	return Observation{}, errors.New("process observation did not stabilize")
}

func equalObservation(left, right Observation) bool {
	return left.PID == right.PID &&
		left.ParentPID == right.ParentPID &&
		left.ProcessGroup == right.ProcessGroup &&
		left.ForegroundProcessGroup == right.ForegroundProcessGroup &&
		left.Executable == right.Executable &&
		left.StartIdentity == right.StartIdentity &&
		slices.Equal(left.Argv, right.Argv)
}

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

func ObserveForeground(panePID PID, paneTTY string) (Observation, error) {
	foreground, err := foregroundProcessGroup(panePID, paneTTY)
	if err != nil {
		return Observation{}, err
	}
	return Observe(foreground)
}
