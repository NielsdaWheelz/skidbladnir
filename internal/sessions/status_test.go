package sessions

import (
	"testing"
	"time"

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
	manager := &Manager{profiles: []Profile{{
		ForegroundSignatures: []ForegroundSignature{{Argument0: "/home/niels/.local/bin/claude"}},
	}}}
	process := processinfo.Observation{
		PID:           4312,
		StartIdentity: "991827",
		Executable:    "/home/niels/.local/share/claude/versions/2.1.231",
		Argv:          []string{"/home/niels/.local/bin/claude"},
	}
	if !manager.matchesAgent(process) {
		t.Fatalf("exact Claude argv[0] was not recognized: %+v", process)
	}
	if lifecycle, valid := parseLifecycleStatus("", process, now); valid {
		t.Fatalf("absent Claude lifecycle was accepted as %+v", lifecycle)
	}
	for _, nearMiss := range []processinfo.Observation{
		{Executable: "/home/niels/.local/share/claude/versions/2.1.231", Argv: []string{"/home/niels/.local/bin/claude-beta"}},
		{Executable: "/usr/bin/node", Argv: []string{"node"}},
	} {
		if manager.matchesAgent(nearMiss) {
			t.Fatalf("non-Claude foreground process matched exact argv[0] signature: %+v", nearMiss)
		}
	}
	status := runningStatus(now)
	if status.Kind != StatusRunning || status.Signal != StatusSignalProcess || status.SignalAt != now {
		t.Fatalf("unobserved live agent status = kind=%s signal=%s signalAt=%s, want kind=%s signal=%s signalAt=%s", status.Kind, status.Signal, status.SignalAt, StatusRunning, StatusSignalProcess, now)
	}
}

func TestForegroundSignatureSelectorsMatchExactlyAndConjunctively(t *testing.T) {
	manager := &Manager{profiles: []Profile{{
		ForegroundSignatures: []ForegroundSignature{{
			ExecutableBase: "claude-2.1.231",
			Argument0:      "/home/niels/.local/bin/claude",
			Argument1:      "--permission-mode",
		}},
	}}}
	exact := processinfo.Observation{
		Executable: "/home/niels/.local/share/claude/claude-2.1.231",
		Argv:       []string{"/home/niels/.local/bin/claude", "--permission-mode"},
	}
	if !manager.matchesAgent(exact) {
		t.Fatalf("exact populated foreground selectors did not match: %+v", exact)
	}

	for _, test := range []struct {
		name    string
		process processinfo.Observation
	}{
		{name: "executable near miss", process: processinfo.Observation{Executable: "/home/niels/.local/share/claude/claude-2.1.232", Argv: exact.Argv}},
		{name: "argument zero prefix", process: processinfo.Observation{Executable: exact.Executable, Argv: []string{exact.Argv[0] + "-beta", exact.Argv[1]}}},
		{name: "argument zero basename", process: processinfo.Observation{Executable: exact.Executable, Argv: []string{"claude", exact.Argv[1]}}},
		{name: "argument one near miss", process: processinfo.Observation{Executable: exact.Executable, Argv: []string{exact.Argv[0], "--permission-modes"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if manager.matchesAgent(test.process) {
				t.Fatalf("foreground near miss matched exact signature: %+v", test.process)
			}
		})
	}
}
