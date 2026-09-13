package agentcontrol

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func TestDetectorRequiresCurrentChromeInsteadOfQuotedInstructions(t *testing.T) {
	for _, test := range []struct {
		provider   agentruntime.Provider
		text, want string
	}{
		{agentruntime.ProviderCodex, "the report said would you like to proceed; esc to cancel", "unknown"},
		{agentruntime.ProviderCodex, "documentation: ? for shortcuts", "unknown"},
		{agentruntime.ProviderClaude, "quoted action required instructions", "unknown"},
		{agentruntime.ProviderCodex, "› 1. Yes, proceed\n  2. No\nPress enter to confirm or esc to cancel", "blocked"},
		{agentruntime.ProviderClaude, "❯ 1. Yes\n  2. No\nEnter to select · Esc to cancel", "blocked"},
		{agentruntime.ProviderCodex, "• Working (4s • esc to interrupt)", "working"},
		{agentruntime.ProviderClaude, "✻ Considering… (esc to interrupt)", "working"},
		{agentruntime.ProviderCodex, "›\n? for shortcuts     98% context left", "idle"},
		{agentruntime.ProviderClaude, "❯\n? for shortcuts", "idle"},
	} {
		if got := Detect(test.provider, test.text); got.State != test.want {
			t.Fatalf("provider %s fixture state=%s want=%s", test.provider, got.State, test.want)
		}
	}
}
