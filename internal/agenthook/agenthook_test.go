package agenthook

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func TestPromptBearingCodexEventsAreDrainedWithoutParsing(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	for _, event := range []string{"UserPromptSubmit", "Stop"} {
		t.Run(event, func(t *testing.T) {
			input := strings.NewReader("opaque input that is deliberately not JSON")
			var output strings.Builder
			if err := Run(context.Background(), Config{}, "Codex", event, input, &output); err != nil {
				t.Fatalf("drain %s input: %v", event, err)
			}
			if input.Len() != 0 {
				t.Fatalf("%s left %d opaque input bytes unread", event, input.Len())
			}
			if event == "Stop" && output.String() != "{}\n" {
				t.Fatalf("Stop output = %q, want empty JSON object", output.String())
			}
		})
	}
}

func TestProviderHookEventsAreClosed(t *testing.T) {
	for _, valid := range []struct {
		provider string
		event    string
	}{
		{provider: "Codex", event: "SessionStart"},
		{provider: "Codex", event: "UserPromptSubmit"},
		{provider: "Codex", event: "Stop"},
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
		{provider: "Codex", event: "PostToolUse"},
		{provider: "Claude", event: "UserPromptSubmit"},
		{provider: "Claude", event: "Stop"},
	} {
		if _, err := parseInvocation(invalid.provider, invalid.event); err == nil {
			t.Fatalf("accepted provider/event outside the hook contract: %q %q", invalid.provider, invalid.event)
		}
	}
}

func TestPublicationPreservesCodexLifecycleAndAddsOnlyClaudeIdentity(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	origin := agentruntime.Foreground{
		Provider:      agentruntime.ProviderCodex,
		PID:           processinfo.PID(4312),
		StartIdentity: "991827",
	}
	tests := []struct {
		name         string
		hook         invocation
		registration string
		want         []string
	}{
		{
			name:         "Codex SessionStart publishes identity and idle in one queue",
			hook:         invocation{provider: agentruntime.ProviderCodex, event: hookSessionStart},
			registration: "registration",
			want: []string{
				"set-option", "-p", "-t", "%7", "--", "@skid_agent_runtime", "registration",
				";", "set-option", "-p", "-t", "%7", "--", "@skid_lifecycle", "v1:4312:991827:idle:1787745600",
			},
		},
		{
			name: "Codex prompt publishes working and clears attention",
			hook: invocation{provider: agentruntime.ProviderCodex, event: hookUserPromptSubmit},
			want: []string{
				"set-option", "-p", "-t", "%7", "--", "@skid_lifecycle", "v1:4312:991827:working:1787745600",
				";", "set-option", "-pqu", "-t", "%7", "--", "@skid_attention",
			},
		},
		{
			name: "Codex Stop publishes idle",
			hook: invocation{provider: agentruntime.ProviderCodex, event: hookStop},
			want: []string{
				"set-option", "-p", "-t", "%7", "--", "@skid_lifecycle", "v1:4312:991827:idle:1787745600",
			},
		},
		{
			name:         "Claude SessionStart publishes identity only",
			hook:         invocation{provider: agentruntime.ProviderClaude, event: hookSessionStart},
			registration: "registration",
			want: []string{
				"set-option", "-p", "-t", "%7", "--", "@skid_agent_runtime", "registration",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := publicationArguments("%7", test.hook, origin, test.registration, now); !slices.Equal(got, test.want) {
				t.Fatalf("publication arguments = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStopWritesTheRequiredCodexAcknowledgement(t *testing.T) {
	if got := hookSuccessOutput(t, hookStop); got != "{}\n" {
		t.Fatalf("Stop success output = %q, want empty JSON object", got)
	}
	if got := hookSuccessOutput(t, hookSessionStart); got != "" {
		t.Fatalf("SessionStart success output = %q, want silence", got)
	}
}

func hookSuccessOutput(t *testing.T, event hookEvent) string {
	t.Helper()
	var output strings.Builder
	if err := writeSuccess(&output, event); err != nil {
		t.Fatalf("write %s success output: %v", event, err)
	}
	return output.String()
}
