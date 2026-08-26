package sessions

import (
	"testing"
	"time"
)

func TestLifecycleStatusIsIndependentFromAttention(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	process := foregroundProcess{pid: 4312, startTime: "991827", executableBase: "codex"}
	status, valid := parseLifecycleStatus("v1:4312:991827:working:1787745590", process, now)
	if !valid || status.Kind != StatusWorking || status.Signal != StatusSignalLifecycle {
		t.Fatalf("working lifecycle = (%+v,%t), want Working/Lifecycle", status, valid)
	}
	if status.SignalAt != time.Date(2026, time.August, 26, 11, 59, 50, 0, time.UTC) {
		t.Fatalf("working lifecycle timestamp = %s", status.SignalAt)
	}

	status, valid = parseLifecycleStatus("v1:4312:991827:idle:1787745580", process, now)
	if !valid || status.Kind != StatusIdle || status.Signal != StatusSignalLifecycle {
		t.Fatalf("idle lifecycle = (%+v,%t), want Idle/Lifecycle", status, valid)
	}
}

func TestLifecycleStatusRejectsAbsentMalformedAndFutureFacts(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	process := foregroundProcess{pid: 4312, startTime: "991827", executableBase: "codex"}
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
		if status, valid := parseLifecycleStatus(value, process, now); valid {
			t.Fatalf("accepted lifecycle %q as %+v", value, status)
		}
	}
}

func TestLifecycleStatusIsBoundToTheExactForegroundProcessLifetime(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	stale := "v1:4312:991827:idle:1787745590"
	newProcess := foregroundProcess{pid: 4312, startTime: "991828", executableBase: "codex"}
	if status, valid := parseLifecycleStatus(stale, newProcess, now); valid {
		t.Fatalf("accepted stale lifecycle from a reused process id: %+v", status)
	}
}

func TestLiveAgentWithoutLifecycleEvidenceIsRunningNotWorking(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	manager := &Manager{profiles: []Profile{{
		ForegroundSignatures: []ForegroundSignature{{Argument0: "/home/niels/.local/bin/claude"}},
	}}}
	process := foregroundProcess{
		pid:            4312,
		startTime:      "991827",
		executableBase: "claude-2.1.231",
		argument0:      "/home/niels/.local/bin/claude",
	}
	if !manager.matchesAgent(process) {
		t.Fatalf("exact Claude argv[0] was not recognized: %+v", process)
	}
	if lifecycle, valid := parseLifecycleStatus("", process, now); valid {
		t.Fatalf("absent Claude lifecycle was accepted as %+v", lifecycle)
	}
	for _, nearMiss := range []foregroundProcess{
		{executableBase: "claude-2.1.231", argument0: "/home/niels/.local/bin/claude-beta"},
		{executableBase: "node", argument0: "node"},
	} {
		if manager.matchesAgent(nearMiss) {
			t.Fatalf("non-Claude foreground process matched exact argv[0] signature: %+v", nearMiss)
		}
	}
	status := runningStatus(now)
	if status.Kind != StatusRunning || status.Signal != StatusSignalProcess || status.SignalAt != now {
		t.Fatalf("unobserved live agent status = %+v, want Running/Process at observation", status)
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
	exact := foregroundProcess{
		executableBase: "claude-2.1.231",
		argument0:      "/home/niels/.local/bin/claude",
		argument1:      "--permission-mode",
	}
	if !manager.matchesAgent(exact) {
		t.Fatalf("exact populated foreground selectors did not match: %+v", exact)
	}

	for _, test := range []struct {
		name    string
		process foregroundProcess
	}{
		{name: "executable near miss", process: foregroundProcess{executableBase: "claude-2.1.232", argument0: exact.argument0, argument1: exact.argument1}},
		{name: "argument zero prefix", process: foregroundProcess{executableBase: exact.executableBase, argument0: exact.argument0 + "-beta", argument1: exact.argument1}},
		{name: "argument zero basename", process: foregroundProcess{executableBase: exact.executableBase, argument0: "claude", argument1: exact.argument1}},
		{name: "argument one near miss", process: foregroundProcess{executableBase: exact.executableBase, argument0: exact.argument0, argument1: "--permission-modes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if manager.matchesAgent(test.process) {
				t.Fatalf("foreground near miss matched exact signature: %+v", test.process)
			}
		})
	}
}
