package main

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestStatusHookDoesNotRequireAHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("TMUX_PANE", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"status-hook", "SessionStart"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("status-hook exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("status-hook output = (%q, %q), want quiet success", stdout.String(), stderr.String())
	}
}

func TestGatewayProfilesExposeExactProductionCapsules(t *testing.T) {
	home := "/home/niels"
	want := []sessions.Profile{
		{
			Key:     "personal",
			Label:   "Codex · Personal",
			Command: "/home/niels/bin/codex-personal",
			Environment: []sessions.EnvironmentVariable{
				{Name: "CODEX_HOME", Value: "/home/niels/.codex-personal"},
			},
			ForegroundSignatures: []sessions.ForegroundSignature{
				{ExecutableBase: "codex"},
				{ExecutableBase: "node", Argument1: "/home/niels/.local/bin/codex"},
			},
			Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			Key:     "work",
			Label:   "Codex · Work",
			Command: "/home/niels/bin/codex-work",
			Environment: []sessions.EnvironmentVariable{
				{Name: "CODEX_HOME", Value: "/home/niels/.codex-work"},
			},
			ForegroundSignatures: []sessions.ForegroundSignature{
				{ExecutableBase: "codex"},
				{ExecutableBase: "node", Argument1: "/home/niels/.local/bin/codex"},
			},
			Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			Key:     "work2",
			Label:   "Codex · Work 2",
			Command: "/home/niels/bin/codex-work2",
			Environment: []sessions.EnvironmentVariable{
				{Name: "CODEX_HOME", Value: "/home/niels/.codex-work2"},
			},
			ForegroundSignatures: []sessions.ForegroundSignature{
				{ExecutableBase: "codex"},
				{ExecutableBase: "node", Argument1: "/home/niels/.local/bin/codex"},
			},
			Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			Key:     "claude-work",
			Label:   "Claude · Work",
			Command: "/home/niels/bin/claude-work",
			Environment: []sessions.EnvironmentVariable{
				{Name: "CLAUDE_CONFIG_DIR", Value: "/home/niels/.claude-work"},
			},
			ForegroundSignatures: []sessions.ForegroundSignature{
				{Argument0: "/home/niels/.local/bin/claude"},
			},
			Arguments: []string{"--permission-mode", "auto"},
		},
	}

	got := gatewayProfiles(home, platform.Descriptor{Kind: platform.KindLinux, CodexNodeEntrypoint: "/home/niels/.local/bin/codex"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("production profiles differ:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDarwinGatewayProfilesDoNotAdvertiseTheDevboxOnlyClaudeCapsule(t *testing.T) {
	profiles := gatewayProfiles("/Users/nnandal", platform.Descriptor{
		Kind:                platform.KindDarwin,
		CodexNodeEntrypoint: "/Users/nnandal/.local/bin/codex",
	})
	if len(profiles) != 3 {
		t.Fatalf("Darwin profile count = %d, want 3 Codex profiles", len(profiles))
	}
	for _, profile := range profiles {
		if profile.Key == "claude-work" {
			t.Fatal("Darwin advertised the Devbox-only Claude capsule")
		}
	}
}
