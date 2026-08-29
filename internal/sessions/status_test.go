package sessions

import (
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func TestLifecycleStatusIsIndependentFromAttention(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	process := processinfo.Observation{PID: 4312, StartIdentity: "991827", Executable: "codex"}
	status, valid := parseLifecycleStatus("v1:4312:991827:working:1787745590", process, now)
	if !valid || status.Kind != StatusWorking || status.Signal != StatusSignalLifecycle {
		t.Fatalf("working lifecycle mismatch: valid=%t kind=%s signal=%s", valid, status.Kind, status.Signal)
	}
	if want := time.Date(2026, time.August, 26, 11, 59, 50, 0, time.UTC); status.SignalAt != want {
		t.Fatalf("working lifecycle timestamp = %s, want %s", status.SignalAt, want)
	}

	status, valid = parseLifecycleStatus("v1:4312:991827:idle:1787745580", process, now)
	if !valid || status.Kind != StatusIdle || status.Signal != StatusSignalLifecycle {
		t.Fatalf("idle lifecycle mismatch: valid=%t kind=%s signal=%s", valid, status.Kind, status.Signal)
	}
}

func TestLifecycleStatusRejectsAbsentMalformedAndFutureFacts(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	process := processinfo.Observation{PID: 4312, StartIdentity: "991827", Executable: "codex"}
	for _, value := range []string{
		"",
		"working",
		"v1:4312:991827:attention:1787745590",
		"v2:4312:991827:working:1787745590",
		"v1:4312:991827:working:not-an-epoch",
		"v1:4312:991827:working:1787745601",
		"v1:4313:991827:working:1787745590",
		"v1:4312:991828:working:1787745590",
	} {
		if _, valid := parseLifecycleStatus(value, process, now); valid {
			t.Fatal("accepted an absent, malformed, future, or wrong-lifetime lifecycle fact")
		}
	}
}

func TestLifecycleStatusIsBoundToTheExactForegroundProcessLifetime(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	stale := "v1:4312:991827:idle:1787745590"
	newProcess := processinfo.Observation{PID: 4312, StartIdentity: "991828", Executable: "codex"}
	if _, valid := parseLifecycleStatus(stale, newProcess, now); valid {
		t.Fatal("accepted stale lifecycle from a reused process id")
	}
}

func TestLiveAgentWithoutLifecycleEvidenceIsRunningNotWorking(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	process := processinfo.Observation{
		PID:           4312,
		StartIdentity: "991827",
		Executable:    "/home/niels/.local/share/claude/versions/2.1.231",
		Argv:          []string{"/home/niels/.local/bin/claude"},
	}
	agent := &agentruntime.AgentRuntime{Provider: agentruntime.ProviderClaude, PID: process.PID}
	status := deriveStatus(process, agent, "v1:4312:991827:idle:1787745590", now)
	if status.Kind != StatusRunning || status.Signal != StatusSignalProcess || status.SignalAt != now {
		t.Fatalf("Claude status = kind=%s signal=%s signalAt=%s, want kind=%s signal=%s signalAt=%s", status.Kind, status.Signal, status.SignalAt, StatusRunning, StatusSignalProcess, now)
	}
}

func TestNoAgentIsShell(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	status := deriveStatus(processinfo.Observation{}, nil, "", now)
	if status.Kind != StatusShell || status.Signal != StatusSignalProcess || status.SignalAt != now {
		t.Fatalf("shell status = %+v", status)
	}
}
