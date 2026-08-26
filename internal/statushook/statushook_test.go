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
	root := processObservation{pid: 101, parentPID: 50, foregroundProcessGroup: 101, startTime: "1001", executableBase: "codex"}
	nested := processObservation{pid: 202, parentPID: 101, foregroundProcessGroup: 101, startTime: "2002", executableBase: "codex"}
	helper := processObservation{pid: 303, parentPID: 202, foregroundProcessGroup: 101, executableBase: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processObservation{helper, root}); !valid || origin != root {
		t.Fatal("rejected the foreground Codex origin")
	}
	if _, valid := foregroundCodexOrigin([]processObservation{helper, nested, root}); valid {
		t.Fatal("accepted a nested Codex origin that inherited the pane environment")
	}
	nested.foregroundProcessGroup = nested.pid
	helper.foregroundProcessGroup = nested.pid
	if _, valid := foregroundCodexOrigin([]processObservation{helper, nested, root}); valid {
		t.Fatal("accepted a nested Codex origin after it became the foreground process group")
	}
	if _, valid := foregroundCodexOrigin([]processObservation{helper}); valid {
		t.Fatal("accepted a process ancestry with no Codex origin")
	}
}

func TestNodeWrapperAndNativeCodexAreOneForegroundRuntime(t *testing.T) {
	wrapper := processObservation{
		pid:                    101,
		parentPID:              50,
		foregroundProcessGroup: 101,
		startTime:              "1001",
		executableBase:         "node",
		argument1:              codexNodeEntrypoint,
	}
	native := processObservation{
		pid:                    102,
		parentPID:              wrapper.pid,
		foregroundProcessGroup: wrapper.pid,
		startTime:              "1002",
		executableBase:         "codex",
	}
	helper := processObservation{pid: 103, parentPID: native.pid, foregroundProcessGroup: wrapper.pid, executableBase: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processObservation{helper, native, wrapper}); !valid || origin != wrapper {
		t.Fatalf("wrapper/native origin = (%+v,%t), want foreground wrapper", origin, valid)
	}
}

func TestNestedWrappedCodexCannotPublishEvenWhenForeground(t *testing.T) {
	rootWrapper := processObservation{pid: 101, parentPID: 50, foregroundProcessGroup: 201, startTime: "1001", executableBase: "node", argument1: codexNodeEntrypoint}
	rootNative := processObservation{pid: 102, parentPID: rootWrapper.pid, foregroundProcessGroup: 201, startTime: "1002", executableBase: "codex"}
	nestedWrapper := processObservation{pid: 201, parentPID: rootNative.pid, foregroundProcessGroup: 201, startTime: "2001", executableBase: "node", argument1: codexNodeEntrypoint}
	nestedNative := processObservation{pid: 202, parentPID: nestedWrapper.pid, foregroundProcessGroup: 201, startTime: "2002", executableBase: "codex"}
	helper := processObservation{pid: 203, parentPID: nestedNative.pid, foregroundProcessGroup: 201, executableBase: "skidbladnir"}

	if _, valid := foregroundCodexOrigin([]processObservation{helper, nestedNative, nestedWrapper, rootNative, rootWrapper}); valid {
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
}

func TestStopHookEmitsTheRequiredEmptyJSONObject(t *testing.T) {
	if got := successOutput(HookStop); got != "{}\n" {
		t.Fatalf("Stop success output = %q, want empty JSON object", got)
	}
	if got := successOutput(HookSessionStart); got != "" {
		t.Fatalf("SessionStart success output = %q, want no model context", got)
	}
}
