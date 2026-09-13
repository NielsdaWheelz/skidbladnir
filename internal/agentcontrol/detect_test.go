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

func TestDetectorRecognizesConfiguredCodexFooterOnlyAtCurrentPrompt(t *testing.T) {
	footer := "gpt-5.6-sol xhigh · ~/project · Context 12% used · weekly 83% left"
	for _, test := range []struct {
		name, text, want string
		provider         agentruntime.Provider
	}{
		{"configured ready", "›\n\n" + footer + "\n", "idle", agentruntime.ProviderCodex},
		{"draft", "› explain this\n" + footer, "idle", agentruntime.ProviderCodex},
		{"no reasoning", "›\ngpt-5.4 · /tmp/project · Context 0% used · weekly 100% left", "idle", agentruntime.ProviderCodex},
		{"fast mode", "›\ngpt-5.6-sol xhigh Fast · ~ · Context 2% used · weekly 70% left", "idle", agentruntime.ProviderCodex},
		{"no quota", "›\ngpt-5.6-sol xhigh · ~/project · Context 12% used", "idle", agentruntime.ProviderCodex},
		{"model and context", "›\ngpt-5.4 high · Context 12% used", "idle", agentruntime.ProviderCodex},
		{"five hour quota", "›\ngpt-5.4 high · /work · Context 12% used · 5h 56% left", "idle", agentruntime.ProviderCodex},
		{"both quotas", "›\ngpt-5.4 high · Context 12% used · 5h 56% left · weekly 83% left", "idle", agentruntime.ProviderCodex},
		{"agent switcher hint", "›\n" + footer + " · ← for agents", "idle", agentruntime.ProviderCodex},
		{"hint without quota", "›\ngpt-5.4 high · Context 12% used · ← for agents", "idle", agentruntime.ProviderCodex},
		{"working wins", "• Working (4s • esc to interrupt)\n›\n" + footer, "working", agentruntime.ProviderCodex},
		{"dialog wins", "› 1. Yes, proceed\nPress enter to confirm or esc to cancel\n" + footer, "blocked", agentruntime.ProviderCodex},
		{"no prompt", footer, "unknown", agentruntime.ProviderCodex},
		{"quoted footer", "›\nThe footer says \"" + footer + "\".", "unknown", agentruntime.ProviderCodex},
		{"quoted block", "›\n> " + footer, "unknown", agentruntime.ProviderCodex},
		{"quoted hint", "›\n> " + footer + " · ← for agents", "unknown", agentruntime.ProviderCodex},
		{"prose directory", "›\ngpt-5.6-sol xhigh · directory example · Context 12% used · weekly 83% left", "unknown", agentruntime.ProviderCodex},
		{"old footer", footer + "\n›\nmore tutorial text", "unknown", agentruntime.ProviderCodex},
		{"prompt only", "›", "unknown", agentruntime.ProviderCodex},
		{"missing context", "›\ngpt-5.6-sol xhigh · ~/project · weekly 83% left", "unknown", agentruntime.ProviderCodex},
		{"wrong provider", "❯\n" + footer, "unknown", agentruntime.ProviderClaude},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect(test.provider, test.text); got.State != test.want {
				t.Fatalf("fixture state=%s want=%s", got.State, test.want)
			}
		})
	}
}

func TestDetectorRecognizesCodexAgentFooterWithoutContext(t *testing.T) {
	footer := "gpt-5.6-sol xhigh · ~ · ← for agents"
	for _, test := range []struct{ name, text, want string }{
		{"ready home", "›\n\n" + footer, "idle"},
		{"project directory", "›\ngpt-5.6-sol high · ~/project · ← for agents", "idle"},
		{"absolute directory", "›\ngpt-5.4 medium · /work/project · ← for agents", "idle"},
		{"fast mode", "›\ngpt-5.6-sol xhigh Fast · ~ · ← for agents", "idle"},
		{"working wins", "• Working (4s • esc to interrupt)\n›\n" + footer, "working"},
		{"dialog wins", "› 1. Yes, proceed\nPress enter to confirm or esc to cancel\n" + footer, "blocked"},
		{"no prompt", footer, "unknown"},
		{"wrong prompt", "❯\n" + footer, "unknown"},
		{"quoted footer", "›\n> " + footer, "unknown"},
		{"footer in prose", "›\nThe footer says " + footer, "unknown"},
		{"historical footer", footer + "\n›\nmore text", "unknown"},
		{"missing reasoning", "›\ngpt-5.6-sol · ~ · ← for agents", "unknown"},
		{"unknown reasoning", "›\ngpt-5.6-sol extreme · ~ · ← for agents", "unknown"},
		{"missing directory", "›\ngpt-5.6-sol xhigh · ← for agents", "unknown"},
		{"relative directory", "›\ngpt-5.6-sol xhigh · project · ← for agents", "unknown"},
		{"missing hint", "›\ngpt-5.6-sol xhigh · ~", "unknown"},
		{"partial hint", "›\ngpt-5.6-sol xhigh · ~ · ← for agen…", "unknown"},
		{"clipped directory", "›\ngpt-5.6-sol xhigh · ~/proj… · ← for agents", "unknown"},
		{"extra thread name", "›\ngpt-5.6-sol xhigh · ~/project · ordinary thread · ← for agents", "unknown"},
		{"unknown suffix", "›\n" + footer + " waiting for input", "unknown"},
		{"wrapped hint", "›\ngpt-5.6-sol xhigh · ~ · ← for\nagents", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect(agentruntime.ProviderCodex, test.text); got.State != test.want {
				t.Fatalf("fixture state=%s want=%s", got.State, test.want)
			}
		})
	}
	if got := Detect(agentruntime.ProviderClaude, "❯\n"+footer); got.State != "unknown" {
		t.Fatal("Codex agent footer established Claude state")
	}
}

func TestDetectorRecognizesClaudePermissionFooterAtCurrentPrompt(t *testing.T) {
	footer := "⏵⏵ auto mode on (shift+tab to cycle) · ← 0 agents"
	for _, test := range []struct{ name, text, want string }{
		{"auto", "❯\n" + footer, "idle"},
		{"draft", "❯ explain this\n" + footer, "idle"},
		{"plan", "❯\n⏸ plan mode on (shift+tab to cycle)", "idle"},
		{"manual", "❯\n⏸ manual mode on (shift+tab to cycle)", "idle"},
		{"accept edits", "❯\n⏵⏵ accept edits on (shift+tab to cycle) · ← 1 agent", "idle"},
		{"bypass", "❯\n⏵⏵ bypass permissions on (shift+tab to cycle) · ← 99+ agents", "idle"},
		{"dont ask", "❯\n⏵⏵ don't ask on (shift+tab to cycle)", "idle"},
		{"working wins", "✻ Considering… (esc to interrupt)\n❯\n" + footer, "working"},
		{"no prompt", footer, "unknown"},
		{"wrong prompt", "›\n" + footer, "unknown"},
		{"quoted", "❯\n> " + footer, "unknown"},
		{"prose", "❯\npermissions can change with shift+tab", "unknown"},
		{"old footer", footer + "\n❯\nmore text", "unknown"},
		{"incomplete footer", "❯\n⏵⏵ auto mode on (shift+tab to", "unknown"},
		{"unrecognized suffix", "❯\n" + footer + " waiting for input", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect(agentruntime.ProviderClaude, test.text); got.State != test.want {
				t.Fatalf("fixture state=%s want=%s", got.State, test.want)
			}
		})
	}
	if got := Detect(agentruntime.ProviderCodex, "›\n"+footer); got.State != "unknown" {
		t.Fatal("Claude permission footer established Codex state")
	}
}

func TestDetectorRecognizesClaudeFolderTrustOnlyWithClosedChoicesAndFooter(t *testing.T) {
	choices := "❯ No, exit\n  Yes, I trust this folder"
	footer := "Enter to confirm · Esc to cancel"
	for _, test := range []struct{ name, text, want string }{
		{"cancel selected", choices + "\n" + footer, "blocked"},
		{"trust selected", "No, exit\n❯ Yes, I trust this folder\n" + footer, "blocked"},
		{"unselected choices", "No, exit\nYes, I trust this folder\n" + footer, "unknown"},
		{"missing footer", choices, "unknown"},
		{"one choice", "❯ No, exit\n" + footer, "unknown"},
		{"quoted choices", "> " + choices + "\n" + footer, "unknown"},
		{"prose footer", choices + "\nEnter to confirm the documentation says Esc to cancel", "unknown"},
		{"old footer", choices + "\n" + footer + "\nmore text", "unknown"},
		{"stale idle footer", choices + "\n⏵⏵ auto mode on (shift+tab to cycle)", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect(agentruntime.ProviderClaude, test.text); got.State != test.want {
				t.Fatalf("fixture state=%s want=%s", got.State, test.want)
			}
		})
	}
}
