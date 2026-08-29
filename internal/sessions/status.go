package sessions

import (
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func deriveStatus(
	observed processinfo.Observation,
	agent *agentruntime.AgentRuntime,
	lifecycleValue string,
	now time.Time,
) Status {
	now = now.UTC()
	if agent == nil {
		return Status{Kind: StatusShell, Signal: StatusSignalProcess, SignalAt: now}
	}
	if agent.Provider == agentruntime.ProviderCodex {
		if lifecycle, valid := parseLifecycleStatus(lifecycleValue, observed, now); valid {
			return lifecycle
		}
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
