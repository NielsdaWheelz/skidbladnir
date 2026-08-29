package agenthook

import (
	"strings"
	"testing"
)

func TestSessionStartReadsOnlyOneBoundedVisibleProviderSessionID(t *testing.T) {
	sessionID, err := readSessionStartID(strings.NewReader(
		`{"session_id":"thr_123","prompt":"must remain opaque","transcript_path":"must remain opaque"}`,
	))
	if err != nil {
		t.Fatalf("read valid SessionStart identity: %v", err)
	}
	if sessionID != "thr_123" {
		t.Fatalf("provider session id = %q, want exact documented field", sessionID)
	}

	tests := []struct {
		name  string
		input string
	}{
		{name: "missing id", input: `{}`},
		{name: "duplicate id", input: `{"session_id":"one","session_id":"two"}`},
		{name: "empty id", input: `{"session_id":""}`},
		{name: "non-string id", input: `{"session_id":7}`},
		{name: "space is not visible", input: `{"session_id":"two words"}`},
		{name: "non-visible id", input: `{"session_id":"line\nbreak"}`},
		{name: "non-ASCII id", input: `{"session_id":"þr"}`},
		{name: "id over 128 bytes", input: `{"session_id":"` + strings.Repeat("a", 129) + `"}`},
		{name: "malformed JSON", input: `{"session_id":`},
		{name: "trailing JSON", input: `{"session_id":"thr_123"}{}`},
		{name: "input over 64 KiB", input: `{"session_id":"thr_123","ignored":"` + strings.Repeat("a", maximumSessionStartBytes) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readSessionStartID(strings.NewReader(test.input)); err == nil {
				t.Fatal("SessionStart accepted invalid identity input")
			}
		})
	}
}
