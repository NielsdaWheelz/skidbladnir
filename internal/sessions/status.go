package sessions

import (
	"strconv"
	"strings"
	"time"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func (manager *Manager) deriveStatus(panePID int, lifecycleValue string, now time.Time) Status {
	now = now.UTC()
	observed, err := processinfo.ObserveForeground(processinfo.PID(panePID))
	if err != nil {
		// justify-ignore-error: every foreground observation failure (absent,
		// protected, or unstable process) is this card's modeled
		// Unknown/PollFailure state; the scheduled poll is the only retry.
		return Status{Kind: StatusUnknown, Signal: StatusSignalPollFailure, SignalAt: now}
	}
	if !manager.matchesAgent(observed) {
		return Status{Kind: StatusShell, Signal: StatusSignalProcess, SignalAt: now}
	}
	if lifecycle, valid := parseLifecycleStatus(lifecycleValue, observed, now); valid {
		return lifecycle
	}
	return runningStatus(now)
}

func parseLifecycleStatus(value string, observed processinfo.Observation, now time.Time) (Status, bool) {
	fields := strings.Split(value, ":")
	if len(fields) != 5 || fields[0] != "v1" || observed.PID <= 0 || observed.StartIdentity == "" {
		return Status{}, false
	}
	originPID, err := strconv.Atoi(fields[1])
	if err != nil || originPID != int(observed.PID) || fields[2] != string(observed.StartIdentity) {
		return Status{}, false
	}
	seconds, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil || seconds <= 0 {
		return Status{}, false
	}
	signalAt := time.Unix(seconds, 0).UTC()
	if signalAt.After(now.UTC()) {
		return Status{}, false
	}
	status := Status{Signal: StatusSignalLifecycle, SignalAt: signalAt}
	switch fields[3] {
	case "working":
		status.Kind = StatusWorking
	case "idle":
		status.Kind = StatusIdle
	default:
		return Status{}, false
	}
	return status, true
}

func runningStatus(now time.Time) Status {
	return Status{Kind: StatusRunning, Signal: StatusSignalProcess, SignalAt: now.UTC()}
}

func (manager *Manager) matchesAgent(observed processinfo.Observation) bool {
	base, argument1 := observed.ExecutableBase(), observed.Argument(1)
	for _, profile := range manager.profiles {
		for _, signature := range profile.ForegroundSignatures {
			if base == signature.ExecutableBase && (signature.Argument1 == "" || argument1 == signature.Argument1) {
				return true
			}
		}
	}
	return false
}

func parseAttentionTime(value string, now time.Time) (time.Time, bool) {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	observed := time.Unix(seconds, 0).UTC()
	if observed.After(now) {
		return time.Time{}, false
	}
	return observed, true
}
