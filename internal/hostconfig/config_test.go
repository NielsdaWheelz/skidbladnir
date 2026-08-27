package hostconfig

import (
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
)

func TestParseAcceptsTheClosedDeploymentHostConfig(t *testing.T) {
	config, err := parse([]byte(validLinuxConfig), platform.KindLinux)
	if err != nil {
		t.Fatalf("parse valid Linux host config: %v", err)
	}
	if config.Platform != platform.KindLinux {
		t.Fatalf("platform = %q, want %q", config.Platform, platform.KindLinux)
	}
	if config.Tmux.Path != "/usr/bin/tmux" || config.Tmux.Version != "tmux 3.4" {
		t.Fatalf("tmux config = %+v, want exact deployment values", config.Tmux)
	}
	wantKeys := []string{"personal", "work", "work2", "claude-personal", "claude-work"}
	if len(config.Profiles) != len(wantKeys) {
		t.Fatalf("profile count = %d, want %d", len(config.Profiles), len(wantKeys))
	}
	for index, want := range wantKeys {
		if got := config.Profiles[index].Key; got != want {
			t.Fatalf("profile[%d].key = %q, want %q", index, got, want)
		}
	}
}

func TestParseAcceptsDeploymentOwnedDevboxArchAndMacBookPaths(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		runtime platform.Kind
	}{
		{name: "Devbox", encoded: validLinuxConfig, runtime: platform.KindLinux},
		{name: "Arch", encoded: strings.ReplaceAll(validLinuxConfig, "/home/niels", "/home/archuser"), runtime: platform.KindLinux},
		{name: "MacBook", encoded: strings.NewReplacer(
			`"platform":"Linux"`, `"platform":"Darwin"`,
			`/usr/bin/tmux`, `/opt/homebrew/bin/tmux`,
			`tmux 3.4`, `tmux 3.7b`,
			`/home/niels`, `/Users/nnandal`,
		).Replace(validLinuxConfig), runtime: platform.KindDarwin},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := parse([]byte(test.encoded), test.runtime)
			if err != nil {
				t.Fatalf("parse %s deployment fixture: %v", test.name, err)
			}
			if config.Platform != test.runtime || len(config.Profiles) != 5 {
				t.Fatalf("%s config = platform %q, profiles %d", test.name, config.Platform, len(config.Profiles))
			}
		})
	}
}

func TestParseRejectsEveryNoncanonicalHostConfigShape(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		runtime platform.Kind
	}{
		{name: "unknown top-level member", encoded: strings.Replace(validLinuxConfig, `"platform":"Linux"`, `"platform":"Linux","fallback":true`, 1), runtime: platform.KindLinux},
		{name: "duplicate member", encoded: strings.Replace(validLinuxConfig, `"platform":"Linux"`, `"platform":"Linux","platform":"Linux"`, 1), runtime: platform.KindLinux},
		{name: "null member", encoded: strings.Replace(validLinuxConfig, `"codexNodeEntrypoint":"/home/niels/.local/bin/codex"`, `"codexNodeEntrypoint":null`, 1), runtime: platform.KindLinux},
		{name: "runtime mismatch", encoded: validLinuxConfig, runtime: platform.KindDarwin},
		{name: "relative tmux path", encoded: strings.Replace(validLinuxConfig, `"path":"/usr/bin/tmux"`, `"path":"bin/tmux"`, 1), runtime: platform.KindLinux},
		{name: "relative Codex entrypoint", encoded: strings.Replace(validLinuxConfig, `"codexNodeEntrypoint":"/home/niels/.local/bin/codex"`, `"codexNodeEntrypoint":"bin/codex"`, 1), runtime: platform.KindLinux},
		{name: "profile order changed", encoded: strings.Replace(validLinuxConfig, `"key":"personal"`, `"key":"work"`, 1), runtime: platform.KindLinux},
		{name: "unknown profile member", encoded: strings.Replace(validLinuxConfig, `"key":"personal"`, `"key":"personal","fallback":true`, 1), runtime: platform.KindLinux},
		{name: "null optional signature member", encoded: strings.Replace(validLinuxConfig, `"executableBase":"codex"`, `"executableBase":null`, 1), runtime: platform.KindLinux},
		{name: "null argument", encoded: strings.Replace(validLinuxConfig, `"arguments":["--dangerously-bypass-approvals-and-sandbox"]`, `"arguments":[null]`, 1), runtime: platform.KindLinux},
		{name: "relative command", encoded: strings.Replace(validLinuxConfig, `"command":"/home/niels/bin/codex-personal"`, `"command":"bin/codex-personal"`, 1), runtime: platform.KindLinux},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parse([]byte(test.encoded), test.runtime); err == nil {
				t.Fatal("parse accepted a noncanonical host config")
			}
		})
	}
}

func TestConfigRequiresTheExactObservedTmuxVersion(t *testing.T) {
	config, err := parse([]byte(validLinuxConfig), platform.KindLinux)
	if err != nil {
		t.Fatalf("parse valid config: %v", err)
	}
	if err := config.ValidateTmuxVersion("tmux 3.4"); err != nil {
		t.Fatalf("exact tmux version rejected: %v", err)
	}
	if err := config.ValidateTmuxVersion("tmux 3.4\n"); err == nil {
		t.Fatal("noncanonical observed tmux version accepted")
	}
	if err := config.ValidateTmuxVersion("tmux 3.5"); err == nil {
		t.Fatal("wrong observed tmux version accepted")
	}
}

const validLinuxConfig = `{
  "platform":"Linux",
  "tmux":{"path":"/usr/bin/tmux","version":"tmux 3.4"},
  "codexNodeEntrypoint":"/home/niels/.local/bin/codex",
  "profiles":[
    {"key":"personal","label":"Codex · Personal","command":"/home/niels/bin/codex-personal","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"work","label":"Codex · Work","command":"/home/niels/bin/codex-work","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"work2","label":"Codex · Work 2","command":"/home/niels/bin/codex-work2","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work2"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"claude-personal","label":"Claude · Personal","command":"/home/niels/bin/claude-personal","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-personal"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":["--permission-mode","auto"]},
    {"key":"claude-work","label":"Claude · Work","command":"/home/niels/bin/claude-work","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-work"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":["--permission-mode","auto"]}
  ]
}`
