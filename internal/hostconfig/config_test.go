package hostconfig

import (
	"errors"
	"os"
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
	if config.Tmux.Path != "/usr/bin/tmux" {
		t.Fatalf("tmux path = %q, want deployment path", config.Tmux.Path)
	}
	wantKeys := []string{"personal", "work", "work2", "claude-personal", "claude-work"}
	if len(config.Profiles) != len(wantKeys) {
		t.Fatalf("profile count = %d, want %d", len(config.Profiles), len(wantKeys))
	}
	for index, want := range wantKeys {
		if got := config.Profiles[index].Key; string(got) != want {
			t.Fatalf("profile[%d].key = %q, want %q", index, got, want)
		}
	}
}

func TestLoadOpenedHostConfigPropagatesACloseFailure(t *testing.T) {
	path := t.TempDir() + "/host-config.json"
	if err := os.WriteFile(path, []byte(validLinuxConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	config, err := loadOpenedHostConfig(closeErrorHostConfigFile{File: file}, platform.KindLinux)
	if err == nil || !strings.Contains(err.Error(), "close host config") {
		t.Fatalf("close failure = %v, want owned host-config close error", err)
	}
	if config.Platform != "" || config.Tmux != (Tmux{}) || config.Profiles != nil {
		t.Fatalf("close failure returned a usable config: %+v", config)
	}
}

type closeErrorHostConfigFile struct {
	*os.File
}

func (file closeErrorHostConfigFile) Close() error {
	if err := file.File.Close(); err != nil {
		return err
	}
	return errors.New("injected close failure")
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
		{name: "null member", encoded: strings.Replace(validLinuxConfig, `"tmux":{"path":"/usr/bin/tmux","testedVersion":"tmux 3.4"}`, `"tmux":null`, 1), runtime: platform.KindLinux},
		{name: "runtime mismatch", encoded: validLinuxConfig, runtime: platform.KindDarwin},
		{name: "relative tmux path", encoded: strings.Replace(validLinuxConfig, `"path":"/usr/bin/tmux"`, `"path":"bin/tmux"`, 1), runtime: platform.KindLinux},
		{name: "legacy tmux version member", encoded: strings.Replace(validLinuxConfig, `"testedVersion":"tmux 3.4"`, `"version":"tmux 3.4"`, 1), runtime: platform.KindLinux},
		{name: "retired Codex entrypoint", encoded: strings.Replace(validLinuxConfig, `"platform":"Linux",`, `"platform":"Linux","codexNodeEntrypoint":"/home/niels/.local/bin/codex",`, 1), runtime: platform.KindLinux},
		{name: "profile order changed", encoded: strings.Replace(validLinuxConfig, `"key":"personal"`, `"key":"work"`, 1), runtime: platform.KindLinux},
		{name: "missing provider", encoded: strings.Replace(validLinuxConfig, `,"provider":"Codex"`, ``, 1), runtime: platform.KindLinux},
		{name: "unknown provider", encoded: strings.Replace(validLinuxConfig, `"provider":"Codex"`, `"provider":"OpenAI"`, 1), runtime: platform.KindLinux},
		{name: "unknown profile member", encoded: strings.Replace(validLinuxConfig, `"key":"personal"`, `"key":"personal","fallback":true`, 1), runtime: platform.KindLinux},
		{name: "null optional signature member", encoded: strings.Replace(validLinuxConfig, `"executableBase":"codex"`, `"executableBase":null`, 1), runtime: platform.KindLinux},
		{name: "null argument", encoded: strings.Replace(validLinuxConfig, `"arguments":["--dangerously-bypass-approvals-and-sandbox"]`, `"arguments":[null]`, 1), runtime: platform.KindLinux},
		{name: "relative command", encoded: strings.Replace(validLinuxConfig, `"command":"/home/niels/bin/codex-personal"`, `"command":"bin/codex-personal"`, 1), runtime: platform.KindLinux},
		{name: "invalid UTF-8", encoded: strings.Replace(validLinuxConfig, "Codex · Personal", "Codex \xff Personal", 1), runtime: platform.KindLinux},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parse([]byte(test.encoded), test.runtime); err == nil {
				t.Fatal("parse accepted a noncanonical host config")
			}
		})
	}
}

func TestParseRequiresOneUniqueAbsoluteProviderHomePerProfile(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{
			name:    "Codex home omitted",
			encoded: strings.Replace(validLinuxConfig, `{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}`, `{"name":"OTHER_HOME","value":"/home/niels/.codex-personal"}`, 1),
		},
		{
			name:    "Claude declares Codex home",
			encoded: strings.Replace(validLinuxConfig, `{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-personal"}`, `{"name":"CODEX_HOME","value":"/home/niels/.claude-personal"}`, 1),
		},
		{
			name:    "relative provider home",
			encoded: strings.Replace(validLinuxConfig, `"value":"/home/niels/.codex-personal"`, `"value":".codex-personal"`, 1),
		},
		{
			name:    "duplicate provider home",
			encoded: strings.Replace(validLinuxConfig, `"value":"/home/niels/.codex-work"`, `"value":"/home/niels/.codex-personal"`, 1),
		},
		{
			name:    "cross-provider foreground signature",
			encoded: strings.Replace(validLinuxConfig, `{"argument0":"/home/niels/.local/bin/claude"}`, `{"executableBase":"codex"}`, 1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parse([]byte(test.encoded), platform.KindLinux); err == nil {
				t.Fatal("parse accepted ambiguous provider ownership")
			}
		})
	}
}

func TestParseRejectsAProfileProviderSwapOutsideTheClosedTable(t *testing.T) {
	swapped := strings.Replace(validLinuxConfig, `"provider":"Codex"`, `"provider":"Claude"`, 1)
	swapped = strings.Replace(swapped,
		`{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}`,
		`{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.codex-personal"}`,
		1,
	)
	swapped = strings.Replace(swapped,
		`{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}`,
		`{"argument0":"/home/niels/.local/bin/claude-swapped"}`,
		1,
	)
	swapped = strings.Replace(swapped,
		`"label":"Claude · Personal","provider":"Claude"`,
		`"label":"Claude · Personal","provider":"Codex"`,
		1,
	)
	swapped = strings.Replace(swapped,
		`{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-personal"}`,
		`{"name":"CODEX_HOME","value":"/home/niels/.claude-personal"}`,
		1,
	)
	swapped = strings.Replace(swapped,
		`{"argument0":"/home/niels/.local/bin/claude"}`,
		`{"executableBase":"codex-swapped"}`,
		1,
	)

	if _, err := parse([]byte(swapped), platform.KindLinux); err == nil {
		t.Fatal("parse accepted valid provider mechanics assigned to the wrong closed profile keys")
	}
}

func TestValidateTmuxVersionAcceptsCanonicalUnpinnedVersions(t *testing.T) {
	for _, version := range []string{"tmux 3.4", "tmux 3.7c", "tmux next-3.8"} {
		if err := ValidateTmuxVersion(version); err != nil {
			t.Fatalf("canonical tmux version %q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"", "3.8", "tmux ", "tmux  3.8", "tmux 3.8 ", "tmux 3.8\n", "tmux \x7f", "tmux α"} {
		if err := ValidateTmuxVersion(version); err == nil {
			t.Fatalf("noncanonical tmux version %q accepted", version)
		}
	}
}

const validLinuxConfig = `{
  "platform":"Linux",
  "tmux":{"path":"/usr/bin/tmux","testedVersion":"tmux 3.4"},
  "profiles":[
    {"key":"personal","label":"Codex · Personal","provider":"Codex","command":"/home/niels/bin/codex-personal","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"work","label":"Codex · Work","provider":"Codex","command":"/home/niels/bin/codex-work","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"work2","label":"Codex · Work 2","provider":"Codex","command":"/home/niels/bin/codex-work2","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work2"}],"foregroundSignatures":[{"executableBase":"codex"},{"executableBase":"node","argument1":"/home/niels/.local/bin/codex"}],"arguments":["--dangerously-bypass-approvals-and-sandbox"]},
    {"key":"claude-personal","label":"Claude · Personal","provider":"Claude","command":"/home/niels/bin/claude-personal","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-personal"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":["--permission-mode","auto"]},
    {"key":"claude-work","label":"Claude · Work","provider":"Claude","command":"/home/niels/bin/claude-work","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-work"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":["--permission-mode","auto"]}
  ]
}`
