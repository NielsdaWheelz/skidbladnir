package hooks

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeProjectsBindingAndBoundedLaptopObjective(t *testing.T) {
	session, err := decode([]byte(`{
		"session_id":"session_123",
		"transcript_path":"/tmp/sessions/2026/08/25/rollout-2026-08-25T01-02-03-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":"/home/niels/src",
		"hook_event_name":"SessionStart",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"source":"startup"
	}`))
	if err != nil {
		t.Fatalf("decode SessionStart: %v", err)
	}
	if session.EffectiveCWD != "/home/niels/src" || session.Model != "gpt-5.6" || session.SessionSource != "startup" {
		t.Fatalf("binding projection = %#v", session)
	}

	prompt := strings.Repeat("x", 300) + "\nsecond\u202eline"
	input := map[string]any{
		"session_id":      "session_123",
		"transcript_path": "/tmp/sessions/2026/08/25/rollout-2026-08-25T01-02-03-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":             "/home/niels/src",
		"hook_event_name": "UserPromptSubmit",
		"turn_id":         "turn_123",
		"prompt":          prompt,
		"model":           "gpt-5.6",
		"permission_mode": "bypassPermissions",
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := decode(encoded)
	if err != nil {
		t.Fatalf("decode UserPromptSubmit: %v", err)
	}
	if len([]rune(submitted.Objective)) != 240 || strings.ContainsAny(submitted.Objective, "\n\u202e") {
		t.Fatalf("objective preview is not safe and bounded: %q", submitted.Objective)
	}
}

func TestDecodeAcceptsLargeDiscardedAssistantMessage(t *testing.T) {
	input := map[string]any{
		"session_id":             "session_123",
		"transcript_path":        "/tmp/sessions/2026/08/25/rollout-2026-08-25T01-02-03-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":                    "/home/niels/src",
		"hook_event_name":        "Stop",
		"model":                  "gpt-5.6",
		"permission_mode":        "bypassPermissions",
		"turn_id":                "turn_123",
		"stop_hook_active":       false,
		"last_assistant_message": strings.Repeat("discarded", 1024),
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := decode(encoded)
	if err != nil {
		t.Fatalf("decode large assistant message: %v", err)
	}
	if fact.Projection != stopObserved {
		t.Fatalf("projection = %v, want StopObserved", fact.Projection)
	}
}

func TestObjectivePreviewNormalizesAndCollapsesUnsafeSeparators(t *testing.T) {
	got := objectivePreview("  e\u0301\talpha\n\u202ebeta\u2066gamma  ")
	if got != "é alpha beta gamma" {
		t.Fatalf("objective preview = %q", got)
	}
}

func TestDeliverRejectsOversizedProjectionBeforeDial(t *testing.T) {
	err := deliver("/definitely/not/a/socket", deliveryMessage{
		Type:      "HookFact",
		Objective: strings.Repeat("x", maxProjectedHookBytes),
	})
	if err == nil || !strings.Contains(err.Error(), "projected hook message exceeds") {
		t.Fatalf("deliver oversized projection error = %v", err)
	}
}

func TestRunRejectsRawHookInputOverLimit(t *testing.T) {
	err := run(strings.NewReader(strings.Repeat("x", maxHookInputBytes+1)), 1, "/unused", "/unused", "/unused")
	if err == nil || !strings.Contains(err.Error(), "hook input exceeds") {
		t.Fatalf("run oversized hook input error = %v", err)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null,
		"unexpected":"no"
	}`))
	if err == nil {
		t.Fatal("decode accepted an unknown hook field")
	}
}

func TestDecodeRejectsDuplicateFields(t *testing.T) {
	_, err := decode([]byte(`{
		"session_id":"thr_123",
		"session_id":"thr_456",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":false,
		"last_assistant_message":null
	}`))
	if err == nil {
		t.Fatal("decode accepted duplicate hook fields")
	}
}

func TestDecodeRejectsNullRequiredBoolean(t *testing.T) {
	_, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"Stop",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"turn_id":"turn_123",
		"stop_hook_active":null,
		"last_assistant_message":null
	}`))
	if err == nil {
		t.Fatal("decode accepted null for required boolean")
	}
}

func TestDecodeRequiresEveryPinnedCommonField(t *testing.T) {
	for _, missing := range []string{"model", "permission_mode", "transcript_path"} {
		input := map[string]any{
			"session_id":      "thr_123",
			"transcript_path": nil,
			"cwd":             "/home/niels/src",
			"hook_event_name": "UserPromptSubmit",
			"turn_id":         "turn_123",
			"prompt":          "synthetic proof prompt",
			"model":           "gpt-5.6",
			"permission_mode": "bypassPermissions",
		}
		delete(input, missing)
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("encode %s fixture: %v", missing, err)
		}
		if _, err := decode(encoded); err == nil {
			t.Fatalf("decode accepted UserPromptSubmit without %s", missing)
		}
	}
}

func TestDecodeRejectsFieldsOutsideThePinnedEventSchema(t *testing.T) {
	_, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":null,
		"cwd":"/home/niels/src",
		"hook_event_name":"SessionEnd",
		"reason":"other",
		"model":"gpt-5.6"
	}`))
	if err == nil {
		t.Fatal("decode accepted model on pinned SessionEnd input")
	}
}

func TestDecodeProjectsDiscriminatedSubagentPromptOnlyAsActivity(t *testing.T) {
	fact, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":"/tmp/sessions/2026/08/24/rollout-2026-08-24T12-34-56-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":"/home/niels/src",
		"hook_event_name":"UserPromptSubmit",
		"turn_id":"turn_123",
		"prompt":"synthetic child prompt",
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions",
		"agent_id":"child_123",
		"agent_type":"worker"
	}`))
	if err != nil {
		t.Fatalf("decode discriminated subagent prompt: %v", err)
	}
	if fact.Projection != activityObserved {
		t.Fatalf("subagent prompt projection = %v, want activity only", fact.Projection)
	}
}

func TestDecodeAcceptsExactPinnedSessionEndShape(t *testing.T) {
	fact, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":"/tmp/sessions/2026/08/24/rollout-2026-08-24T12-34-56-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":"/home/niels/src",
		"hook_event_name":"SessionEnd",
		"reason":"other"
	}`))
	if err != nil {
		t.Fatalf("decode exact SessionEnd: %v", err)
	}
	if fact.Projection != sessionEnded {
		t.Fatalf("SessionEnd projection = %v, want session ended", fact.Projection)
	}
}

func TestDecodeProjectsSubagentOnlyAsActivity(t *testing.T) {
	fact, err := decode([]byte(`{
		"session_id":"thr_123",
		"transcript_path":"/tmp/sessions/2026/08/24/rollout-2026-08-24T12-34-56-11111111-1111-4111-8111-111111111111.jsonl",
		"cwd":"/home/niels/src",
		"hook_event_name":"SubagentStop",
		"turn_id":"turn_123",
		"agent_id":"child_123",
		"agent_type":"worker",
		"agent_transcript_path":null,
		"stop_hook_active":false,
		"last_assistant_message":null,
		"model":"gpt-5.6",
		"permission_mode":"bypassPermissions"
	}`))
	if err != nil {
		t.Fatalf("decode subagent hook: %v", err)
	}
	if fact.Projection != activityObserved {
		t.Fatalf("subagent projection = %v, want activity only", fact.Projection)
	}
}
