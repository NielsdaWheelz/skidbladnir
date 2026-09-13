package agentcontrol

import (
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestNativeClaudeEnvironmentDistinguishesDefaultFromWork(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/unrelated/claude")
	t.Setenv("CODEX_HOME", "/unrelated/codex")
	service := Service{sessions: &sessions.Manager{}}
	for _, test := range []struct {
		name, want string
		env        []agentruntime.EnvironmentVariable
	}{
		{name: "default"},
		{name: "work", want: "/home/test/.claude-work", env: []agentruntime.EnvironmentVariable{{Name: "CLAUDE_CONFIG_DIR", Value: "/home/test/.claude-work"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, present := "", false
			for _, entry := range service.nativeEnvironment(agentruntime.Profile{Provider: agentruntime.ProviderClaude, Environment: test.env}) {
				name, value, _ := strings.Cut(entry, "=")
				if name == "CODEX_HOME" {
					t.Fatal("Claude helper inherited another provider's root")
				}
				if name == "CLAUDE_CONFIG_DIR" {
					root, present = value, true
				}
			}
			if root != test.want || present != (test.want != "") {
				t.Fatalf("helper root=(%q,%t), want %q", root, present, test.want)
			}
		})
	}
}
