package statushook

import (
	"slices"
	"testing"
	"time"
)

func TestLifecycleValueUsesTheClosedHookEventVocabulary(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	origin := processObservation{pid: 4312, startTime: "991827"}
	tests := []struct {
		event HookEvent
		want  string
	}{
		{event: HookSessionStart, want: "v1:4312:991827:idle:1787745600"},
		{event: HookUserPromptSubmit, want: "v1:4312:991827:working:1787745600"},
		{event: HookStop, want: "v1:4312:991827:idle:1787745600"},
	}
	for _, test := range tests {
		if got := lifecycleValue(test.event, origin, now); got != test.want {
			t.Fatalf("lifecycle value for %s = %q, want %q", test.event, got, test.want)
		}
	}
	if _, err := parseHookEvent("PostToolUse"); err == nil {
		t.Fatal("accepted a hook event outside the lifecycle contract")
	}
}

func TestOnlyTheForegroundCodexAncestorMayPublishPaneLifecycle(t *testing.T) {
	const terminalDevice = 34817
	root := processObservation{pid: 101, parentPID: 50, foregroundProcessGroup: 101, startTime: "1001", executableBase: "codex"}
	nested := processObservation{pid: 202, parentPID: 101, foregroundProcessGroup: 101, startTime: "2002", executableBase: "codex"}
	helper := processObservation{pid: 303, parentPID: 202, terminalDevice: terminalDevice, foregroundProcessGroup: 101, executableBase: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processObservation{helper, root}, terminalDevice); !valid || origin != root {
		t.Fatal("rejected the foreground Codex origin")
	}
	if _, valid := foregroundCodexOrigin([]processObservation{helper, nested, root}, terminalDevice); valid {
		t.Fatal("accepted a nested Codex origin that inherited the pane environment")
	}
	nested.foregroundProcessGroup = nested.pid
	helper.foregroundProcessGroup = nested.pid
	if _, valid := foregroundCodexOrigin([]processObservation{helper, nested, root}, terminalDevice); valid {
		t.Fatal("accepted a nested Codex origin after it became the foreground process group")
	}
	if _, valid := foregroundCodexOrigin([]processObservation{helper}, terminalDevice); valid {
		t.Fatal("accepted a process ancestry with no Codex origin")
	}
}

func TestCodexOriginMustShareTheTargetPaneTerminal(t *testing.T) {
	const processTerminalDevice = 34817
	root := processObservation{
		pid:                    101,
		parentPID:              50,
		terminalDevice:         processTerminalDevice,
		foregroundProcessGroup: 101,
		startTime:              "1001",
		executableBase:         "codex",
	}
	helper := processObservation{
		pid:                    303,
		parentPID:              root.pid,
		terminalDevice:         processTerminalDevice,
		foregroundProcessGroup: root.pid,
		executableBase:         "skidbladnir",
	}

	if _, valid := foregroundCodexOrigin([]processObservation{helper, root}, processTerminalDevice+1); valid {
		t.Fatal("accepted a Codex process session from another terminal")
	}
}

func TestProcessAncestryStopsAtTheTerminalSessionLeader(t *testing.T) {
	const (
		sessionLeaderPID = 50
		terminalDevice   = 34817
	)
	process := processObservation{
		pid:            303,
		parentPID:      102,
		sessionID:      sessionLeaderPID,
		terminalDevice: terminalDevice,
	}
	if nextPID, err := nextTerminalSessionPID(process, sessionLeaderPID, terminalDevice); err != nil || nextPID != 102 {
		t.Fatalf("next terminal-session process = (%d,%v), want 102", nextPID, err)
	}
	process.pid = sessionLeaderPID
	process.parentPID = 1
	if nextPID, err := nextTerminalSessionPID(process, sessionLeaderPID, terminalDevice); err != nil || nextPID != 0 {
		t.Fatalf("process after session leader = (%d,%v), want stop", nextPID, err)
	}
	if _, err := nextTerminalSessionPID(process, sessionLeaderPID, terminalDevice+1); err == nil {
		t.Fatal("accepted a process from another terminal session")
	}
}

func TestProcessStatDefinesTheTerminalSession(t *testing.T) {
	contents := []byte("303 (hook worker) S 102 101 50 34817 101 0 0 0 0 0 0 0 0 0 20 0 1 0 3003\n")
	observation, err := parseProcessStat(303, contents)
	if err != nil {
		t.Fatalf("parse process stat: %v", err)
	}
	if observation.pid != 303 || observation.parentPID != 102 ||
		observation.sessionID != 50 || observation.terminalDevice != 34817 ||
		observation.foregroundProcessGroup != 101 || observation.startTime != "3003" {
		t.Fatalf("process observation = %+v, want exact terminal-session fields", observation)
	}
}

func TestNodeWrapperAndNativeCodexAreOneForegroundRuntime(t *testing.T) {
	const terminalDevice = 34817
	wrapper := processObservation{
		pid:                    101,
		parentPID:              50,
		foregroundProcessGroup: 101,
		startTime:              "1001",
		executableBase:         "node",
		argument1:              CodexNodeEntrypoint,
	}
	native := processObservation{
		pid:                    102,
		parentPID:              wrapper.pid,
		foregroundProcessGroup: wrapper.pid,
		startTime:              "1002",
		executableBase:         "codex",
	}
	helper := processObservation{pid: 103, parentPID: native.pid, terminalDevice: terminalDevice, foregroundProcessGroup: wrapper.pid, executableBase: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processObservation{helper, native, wrapper}, terminalDevice); !valid || origin != wrapper {
		t.Fatalf("wrapper/native origin = (%+v,%t), want foreground wrapper", origin, valid)
	}
}

func TestNestedWrappedCodexCannotPublishEvenWhenForeground(t *testing.T) {
	const terminalDevice = 34817
	rootWrapper := processObservation{pid: 101, parentPID: 50, foregroundProcessGroup: 201, startTime: "1001", executableBase: "node", argument1: CodexNodeEntrypoint}
	rootNative := processObservation{pid: 102, parentPID: rootWrapper.pid, foregroundProcessGroup: 201, startTime: "1002", executableBase: "codex"}
	nestedWrapper := processObservation{pid: 201, parentPID: rootNative.pid, foregroundProcessGroup: 201, startTime: "2001", executableBase: "node", argument1: CodexNodeEntrypoint}
	nestedNative := processObservation{pid: 202, parentPID: nestedWrapper.pid, foregroundProcessGroup: 201, startTime: "2002", executableBase: "codex"}
	helper := processObservation{pid: 203, parentPID: nestedNative.pid, terminalDevice: terminalDevice, foregroundProcessGroup: 201, executableBase: "skidbladnir"}

	if _, valid := foregroundCodexOrigin([]processObservation{helper, nestedNative, nestedWrapper, rootNative, rootWrapper}, terminalDevice); valid {
		t.Fatal("accepted a nested wrapped Codex runtime")
	}
}

func TestLifecyclePublicationClearsStaleAttentionWhenWorkBegins(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	origin := processObservation{pid: 4312, startTime: "991827"}
	arguments := lifecycleTmuxArguments("%7", HookUserPromptSubmit, origin, now)
	want := []string{
		"set-option", "-p", "-t", "%7", "--", "@skid_lifecycle", "v1:4312:991827:working:1787745600",
		";", "set-option", "-pqu", "-t", "%7", "--", "@skid_attention",
	}
	if !slices.Equal(arguments, want) {
		t.Fatalf("prompt lifecycle arguments = %q, want %q", arguments, want)
	}
	for _, event := range []HookEvent{HookSessionStart, HookStop} {
		state := "idle"
		arguments := lifecycleTmuxArguments("%7", event, origin, now)
		want := []string{
			"set-option", "-p", "-t", "%7", "--", "@skid_lifecycle", "v1:4312:991827:" + state + ":1787745600",
		}
		if !slices.Equal(arguments, want) {
			t.Fatalf("%s lifecycle arguments = %q, want %q (attention clears only on a submitted prompt)", event, arguments, want)
		}
	}
}

func TestStopHookEmitsTheRequiredEmptyJSONObject(t *testing.T) {
	if got := successOutput(HookStop); got != "{}\n" {
		t.Fatalf("Stop success output = %q, want empty JSON object", got)
	}
	if got := successOutput(HookSessionStart); got != "" {
		t.Fatalf("SessionStart success output = %q, want no model context", got)
	}
}
