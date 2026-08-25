package hooks

import (
	"encoding/json"
	"testing"
)

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
		"transcript_path":null,
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
		"transcript_path":null,
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
		"transcript_path":null,
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
