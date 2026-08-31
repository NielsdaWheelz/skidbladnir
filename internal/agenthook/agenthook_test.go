package agenthook

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestUnsupportedHookEventIsRejectedWithoutReadingProviderInput(t *testing.T) {
	input := &countingHookReader{}
	if _, err := Prepare("Codex", "UnsupportedEvent", input); !errors.Is(err, ErrInvocationRejected) {
		t.Fatalf("unsupported invocation error = %v, want ErrInvocationRejected", err)
	}
	if input.reads != 0 {
		t.Fatalf("unsupported event read provider input %d times", input.reads)
	}
}

type countingHookReader struct {
	reads int
}

func (reader *countingHookReader) Read([]byte) (int, error) {
	reader.reads++
	return 0, errors.New("provider input must not be read")
}

func TestOversizeSessionStartInputStopsAtTheAdmissionBound(t *testing.T) {
	const extraBytes = 4096
	input := strings.NewReader(strings.Repeat("x", maximumSessionStartBytes+extraBytes))
	inputBytes := input.Len()
	if _, err := Prepare("Codex", "SessionStart", input); err == nil {
		t.Fatal("oversize SessionStart payload was accepted")
	}
	if input.Len() == 0 {
		t.Fatal("oversize SessionStart input was drained after its rejection was known")
	}
	if consumed := inputBytes - input.Len(); consumed > maximumSessionStartBytes+1 {
		t.Fatalf("oversize SessionStart admission read %d bytes, beyond its %d-byte bound", consumed, maximumSessionStartBytes+1)
	}
}

func TestProviderHookEventsAreClosed(t *testing.T) {
	for _, valid := range []struct {
		provider string
		event    string
	}{
		{provider: "Codex", event: "SessionStart"},
		{provider: "Claude", event: "SessionStart"},
	} {
		if _, err := parseInvocation(valid.provider, valid.event); err != nil {
			t.Fatalf("valid %s %s hook rejected: %v", valid.provider, valid.event, err)
		}
	}
	for _, invalid := range []struct {
		provider string
		event    string
	}{
		{provider: "", event: "SessionStart"},
		{provider: "codex", event: "SessionStart"},
		{provider: "Other", event: "SessionStart"},
		{provider: "Codex", event: "UnsupportedEvent"},
		{provider: "Claude", event: "UnsupportedEvent"},
	} {
		if _, err := parseInvocation(invalid.provider, invalid.event); !errors.Is(err, ErrInvocationRejected) {
			t.Fatalf("provider/event outside the hook contract returned %v: %q %q", err, invalid.provider, invalid.event)
		}
	}
}

func TestPublicationOwnsOnlySessionStartIdentity(t *testing.T) {
	tests := []struct {
		name         string
		registration string
		want         []string
	}{
		{
			name:         "Codex SessionStart publishes identity unconditionally",
			registration: "v1:4312:991827:Codex:work:dGhyXzEyMw",
			want: []string{
				"set-option", "-p", "-t", "%7", "--",
				"@skid_agent_runtime", "v1:4312:991827:Codex:work:dGhyXzEyMw",
			},
		},
		{
			name:         "Claude SessionStart publishes identity unconditionally",
			registration: "v1:4312:991827:Claude:claude-work:dGhyXzEyMw",
			want: []string{
				"set-option", "-p", "-t", "%7", "--",
				"@skid_agent_runtime", "v1:4312:991827:Claude:claude-work:dGhyXzEyMw",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := publicationArguments("%7", test.registration)
			if !slices.Equal(got, test.want) {
				t.Fatalf("publication arguments = %q, want %q", got, test.want)
			}
			if slices.Contains(got, "if-shell") {
				t.Fatal("SessionStart identity was made conditional on a prior pane value")
			}
		})
	}
}
