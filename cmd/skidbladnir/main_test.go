package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
)

func TestVersionReportsExactReleaseIdentity(t *testing.T) {
	releaseVersion = "v0.2.0"
	releaseSHA = "0123456789abcdef0123456789abcdef01234567"
	t.Cleanup(func() {
		releaseVersion = "dev"
		releaseSHA = "unknown"
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"version"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("version exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "v0.2.0 0123456789abcdef0123456789abcdef01234567\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q, want empty", stderr.String())
	}
}

func TestStatusHookDoesNotRequireAHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("TMUX_PANE", "")
	tmuxPath := writeTmuxVersion(t, "tmux test")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"status-hook", "--host-config=" + hostConfigPath, "SessionStart"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("status-hook exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("status-hook output = (%q, %q), want quiet success", stdout.String(), stderr.String())
	}
}

func TestStatusHookRejectsConfiguredTmuxVersionDriftBeforeReadingTheEvent(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	tmuxPath := writeTmuxVersion(t, "tmux changed")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux expected")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"status-hook", "--host-config=" + hostConfigPath, "SessionStart"}, &stdout, &stderr); exitCode != exitFailure {
		t.Fatalf("status-hook exit code = %d, want %d", exitCode, exitFailure)
	}
	if stdout.Len() != 0 || stderr.String() != "status-hook host configuration failed\n" {
		t.Fatalf("status-hook output = (%q, %q), want quiet config failure", stdout.String(), stderr.String())
	}
}

func TestPairingInviteCommandBoundaryUsesOnlyTransientAuthority(t *testing.T) {
	bearerPath := filepath.Join(t.TempDir(), "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (auth.FileVerifier{Path: bearerPath}).Read()
	if err != nil {
		t.Fatal(err)
	}
	handle, err := machine.Init(filepath.Join(t.TempDir(), "machine-handle"))
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339Nano)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/pairing-invites" || request.ContentLength != 0 {
			t.Errorf("pairing request = %s %s length %d", request.Method, request.URL.Path, request.ContentLength)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+bearer {
			t.Errorf("authorization = %q, want current bearer", got)
		}
		if got := request.Header.Get("Skidbladnir-Machine"); got != handle.String() {
			t.Errorf("machine = %q, want %q", got, handle.String())
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(writer, `{"pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","expiresAt":%q,"machine":{"handle":%q,"platform":"Linux"}}`, expiresAt, handle.String())
	}))
	defer server.Close()

	invite, err := requestPairingInvitation(context.Background(), server.Client(), server.URL, handle, platform.KindLinux, credential)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(invite)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","expiresAt":%q,"machine":{"handle":%q,"platform":"Linux"}}`, expiresAt, handle.String())
	if string(encoded) != want {
		t.Fatalf("pairing invitation = %s, want %s", encoded, want)
	}
}

func TestPairingInviteCommandRejectsAmbiguousOrUnboundedResponses(t *testing.T) {
	bearerPath := filepath.Join(t.TempDir(), "bearer")
	_, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (auth.FileVerifier{Path: bearerPath}).Read()
	if err != nil {
		t.Fatal(err)
	}
	handle, err := machine.Init(filepath.Join(t.TempDir(), "machine-handle"))
	if err != nil {
		t.Fatal(err)
	}
	canonicalExpiry := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339Nano)
	tests := []struct {
		name string
		body string
	}{
		{
			name: "duplicate top-level token",
			body: fmt.Sprintf(`{"pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","expiresAt":%q,"machine":{"handle":%q,"platform":"Linux"}}`, canonicalExpiry, handle.String()),
		},
		{
			name: "duplicate nested handle",
			body: fmt.Sprintf(`{"pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","expiresAt":%q,"machine":{"handle":%q,"handle":%q,"platform":"Linux"}}`, canonicalExpiry, handle.String(), handle.String()),
		},
		{
			name: "far-future expiry",
			body: fmt.Sprintf(`{"pairingInviteToken":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","expiresAt":%q,"machine":{"handle":%q,"platform":"Linux"}}`, time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), handle.String()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			if _, err := requestPairingInvitation(context.Background(), server.Client(), server.URL, handle, platform.KindLinux, credential); err == nil {
				t.Fatal("pairing invitation accepted an ambiguous or unbounded response")
			}
		})
	}
}

func writeTmuxVersion(t *testing.T, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' "+fmt.Sprintf("%q", version)+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeHostConfig(t *testing.T, tmuxPath, tmuxVersion string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host.json")
	encoded := fmt.Sprintf(`{
  "platform": %q,
	  "tmux": {"path": %q, "version": %q},
  "codexNodeEntrypoint": "/home/niels/.local/bin/codex",
  "profiles": [
    {"key":"personal","label":"Codex · Personal","command":"/home/niels/bin/codex-personal","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work","label":"Codex · Work","command":"/home/niels/bin/codex-work","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work2","label":"Codex · Work 2","command":"/home/niels/bin/codex-work2","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work2"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"claude-personal","label":"Claude · Personal","command":"/home/niels/bin/claude-personal","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-personal"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":[]},
    {"key":"claude-work","label":"Claude · Work","command":"/home/niels/bin/claude-work","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-work"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":[]}
  ]
}`, platform.Current().Kind, tmuxPath, tmuxVersion)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
