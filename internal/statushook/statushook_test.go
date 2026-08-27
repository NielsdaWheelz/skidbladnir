package statushook

import (
	"reflect"
	"slices"
	"testing"
	"time"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func TestLifecycleValueUsesTheClosedHookEventVocabulary(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	origin := processinfo.Observation{PID: 4312, StartIdentity: "991827"}
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
			t.Fatalf("lifecycle value for event %s = %q, want %q", test.event, got, test.want)
		}
	}
	if _, err := parseHookEvent("PostToolUse"); err == nil {
		t.Fatal("accepted a hook event outside the lifecycle contract")
	}
}

func TestOnlyTheForegroundCodexAncestorMayPublishPaneLifecycle(t *testing.T) {
	root := processinfo.Observation{PID: 101, ParentPID: 50, ForegroundProcessGroup: 101, StartIdentity: "1001", Executable: "codex"}
	nested := processinfo.Observation{PID: 202, ParentPID: 101, ForegroundProcessGroup: 101, StartIdentity: "2002", Executable: "codex"}
	helper := processinfo.Observation{PID: 303, ParentPID: 202, ForegroundProcessGroup: 101, Executable: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processinfo.Observation{helper, root}); !valid || !reflect.DeepEqual(origin, root) {
		t.Fatalf("foreground Codex origin = %+v valid=%t, want %+v", origin, valid, root)
	}
	if _, valid := foregroundCodexOrigin([]processinfo.Observation{helper, nested, root}); valid {
		t.Fatal("accepted a nested Codex origin that inherited the pane environment")
	}
	nested.ForegroundProcessGroup = nested.PID
	helper.ForegroundProcessGroup = nested.PID
	if _, valid := foregroundCodexOrigin([]processinfo.Observation{helper, nested, root}); valid {
		t.Fatal("accepted a nested Codex origin after it became the foreground process group")
	}
	if _, valid := foregroundCodexOrigin([]processinfo.Observation{helper}); valid {
		t.Fatal("accepted a process ancestry with no Codex origin")
	}
}

func TestNodeWrapperAndNativeCodexAreOneForegroundRuntime(t *testing.T) {
	wrapper := processinfo.Observation{
		PID:                    101,
		ParentPID:              50,
		ForegroundProcessGroup: 101,
		StartIdentity:          "1001",
		Executable:             "node",
		Argv:                   []string{"node", codexNodeEntrypoint},
	}
	native := processinfo.Observation{
		PID:                    102,
		ParentPID:              wrapper.PID,
		ForegroundProcessGroup: wrapper.PID,
		StartIdentity:          "1002",
		Executable:             "codex",
	}
	helper := processinfo.Observation{PID: 103, ParentPID: native.PID, ForegroundProcessGroup: wrapper.PID, Executable: "skidbladnir"}

	if origin, valid := foregroundCodexOrigin([]processinfo.Observation{helper, native, wrapper}); !valid || !reflect.DeepEqual(origin, wrapper) {
		t.Fatalf("wrapper/native origin = %+v valid=%t, want wrapper %+v", origin, valid, wrapper)
	}
}

func TestNestedWrappedCodexCannotPublishEvenWhenForeground(t *testing.T) {
	rootWrapper := processinfo.Observation{PID: 101, ParentPID: 50, ForegroundProcessGroup: 201, StartIdentity: "1001", Executable: "node", Argv: []string{"node", codexNodeEntrypoint}}
	rootNative := processinfo.Observation{PID: 102, ParentPID: rootWrapper.PID, ForegroundProcessGroup: 201, StartIdentity: "1002", Executable: "codex"}
	nestedWrapper := processinfo.Observation{PID: 201, ParentPID: rootNative.PID, ForegroundProcessGroup: 201, StartIdentity: "2001", Executable: "node", Argv: []string{"node", codexNodeEntrypoint}}
	nestedNative := processinfo.Observation{PID: 202, ParentPID: nestedWrapper.PID, ForegroundProcessGroup: 201, StartIdentity: "2002", Executable: "codex"}
	helper := processinfo.Observation{PID: 203, ParentPID: nestedNative.PID, ForegroundProcessGroup: 201, Executable: "skidbladnir"}

	if _, valid := foregroundCodexOrigin([]processinfo.Observation{helper, nestedNative, nestedWrapper, rootNative, rootWrapper}); valid {
		t.Fatal("accepted a nested wrapped Codex runtime")
	}
}

func TestLifecyclePublicationClearsStaleAttentionWhenWorkBegins(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	origin := processinfo.Observation{PID: 4312, StartIdentity: "991827"}
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
		t.Fatalf("Stop success output = %q, want %q", got, "{}\n")
	}
	if got := successOutput(HookSessionStart); got != "" {
		t.Fatalf("SessionStart success output = %q, want empty", got)
	}
}
