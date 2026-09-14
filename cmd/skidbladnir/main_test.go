package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agenthook"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
)

func TestClientCommandReturnsStructuredFailureWithoutServiceHome(t *testing.T) {
	t.Setenv("HOME", "")
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"--config", filepath.Join(t.TempDir(), "missing.json"), "list", "--json"},
		commandInput(t, "{}"), &stdout, &stderr,
	)
	var result struct {
		OK    bool `json:"ok"`
		Error struct {
			Code     string `json:"code"`
			Dispatch string `json:"dispatch"`
		} `json:"error"`
	}
	if code != 1 || json.Unmarshal(stdout.Bytes(), &result) != nil ||
		result.OK || result.Error.Code == "" || result.Error.Dispatch != "not_sent" || stderr.Len() != 0 {
		t.Fatalf("agent entry point did not return a structured pre-dispatch failure: exit=%d stdout_bytes=%d stderr_bytes=%d", code, stdout.Len(), stderr.Len())
	}
}

func TestClientHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, emptyCommandInput(t), &stdout, &stderr); code != 0 {
		t.Fatalf("human client help failed: exit=%d", code)
	}
	for _, word := range []string{"list", "enter", "--ref", "--stdin"} {
		if !bytes.Contains(stdout.Bytes(), []byte(word)) {
			t.Errorf("help is missing %s", word)
		}
	}

}

func TestVersionReportsExactReleaseIdentity(t *testing.T) {
	releaseVersion = "v0.2.0"
	releaseSHA = "0123456789abcdef0123456789abcdef01234567"
	t.Cleanup(func() {
		releaseVersion = "dev"
		releaseSHA = "unknown"
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"version"}, emptyCommandInput(t), &stdout, &stderr); exitCode != 0 {
		t.Fatalf("version exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "v0.2.0 0123456789abcdef0123456789abcdef01234567\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q, want empty", stderr.String())
	}
}

func TestAgentHookDoesNotRequireAHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("TMUX_PANE", "")
	hostConfigPath := filepath.Join(t.TempDir(), "missing.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"agent-hook", "--host-config=" + hostConfigPath, "Codex", "SessionStart"}, commandInput(t, `{"session_id":"thr_123"}`), &stdout, &stderr); exitCode != 0 {
		t.Fatalf("agent-hook exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("agent-hook output = (%q, %q), want quiet success", stdout.String(), stderr.String())
	}
}

func TestAgentHookWithoutAPaneDoesNotProbeTmux(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	probePath := filepath.Join(t.TempDir(), "probe")
	t.Setenv("SKID_TEST_TMUX_PROBE", probePath)
	tmuxPath := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nprintf probe > \"$SKID_TEST_TMUX_PROBE\"\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	input := commandInput(t, `{"session_id":"thr_123"}`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"agent-hook", "--host-config=" + hostConfigPath, "Codex", "SessionStart"}, input, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("agent-hook exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if remaining, err := io.ReadAll(input); err != nil || len(remaining) != 0 {
		t.Fatalf("no-pane hook did not drain admitted provider input: remaining=%d err=%v", len(remaining), err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("agent-hook output = (%q, %q), want quiet success", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(probePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no-pane hook invoked configured tmux: %v", err)
	}
}

func TestRuntimeHostConfigAllowsTmuxVersionDrift(t *testing.T) {
	for _, version := range []string{"tmux changed", "not a tmux version"} {
		t.Run(version, func(t *testing.T) {
			tmuxPath := writeTmuxVersion(t, version)
			hostConfigPath := writeHostConfig(t, tmuxPath, "tmux expected")
			_, err := loadRuntimeHostConfig(context.Background(), hostConfigPath, platform.Current().Kind)
			if admitted := err == nil; admitted != (version == "tmux changed") {
				t.Fatalf("runtime config admission = %t for observed version %q", admitted, version)
			}
		})
	}
}

func TestAgentHookFailsOpenSoAProviderNeverLosesTheUsersWork(t *testing.T) {
	// A pane id and valid SessionStart payload are present, and the configured
	// executable passes admission before failing the first publication read.
	// The provider must still be allowed to start its session.
	t.Setenv("TMUX_PANE", "%7")
	tmuxPath := writeTmuxVersionThenFail(t, "tmux test")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run(
		[]string{"agent-hook", "--host-config=" + hostConfigPath, "Codex", "SessionStart"},
		commandInput(t, `{"session_id":"thr_123"}`),
		&stdout,
		&stderr,
	); exitCode != 0 {
		t.Fatalf("agent-hook exit code = %d, want 0 on a failed projection; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stderr.String(), "agent-hook did not publish\n"; got != want {
		t.Fatalf("agent-hook diagnostic = %q, want the content-free %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("agent-hook stdout = %q, want empty", stdout.String())
	}
}

func TestAgentHookRejectsAnUnconfiguredProviderEventPair(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	tmuxPath := writeTmuxVersion(t, "tmux test")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	for _, provider := range []string{"Codex", "Claude"} {
		t.Run(provider, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := run([]string{"agent-hook", "--host-config=" + hostConfigPath, provider, "UnsupportedEvent"}, emptyCommandInput(t), &stdout, &stderr)
			if exitCode != exitUsage {
				t.Fatalf("agent-hook exit code = %d, want %d for an unsupported provider event", exitCode, exitUsage)
			}
			if got, want := stderr.String(), "agent-hook did not publish\n"; got != want {
				t.Fatalf("agent-hook diagnostic = %q, want content-free %q", got, want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("agent-hook stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestAgentHookRejectedPairReturnsWithoutReadingProviderInput(t *testing.T) {
	tmuxPath := writeTmuxVersion(t, "tmux test")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	input, provider, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = provider.Close()
	})
	returned := make(chan int, 1)
	go func() {
		returned <- run(
			[]string{"agent-hook", "--host-config=" + hostConfigPath, "Claude", "UnsupportedEvent"},
			input,
			io.Discard,
			io.Discard,
		)
	}()
	select {
	case exitCode := <-returned:
		if exitCode != exitUsage {
			t.Fatalf("agent-hook exit code = %d, want %d for an unsupported event", exitCode, exitUsage)
		}
	case <-time.After(time.Second):
		_ = provider.Close()
		<-returned
		t.Fatal("unsupported provider event waited for stdin")
	}
}

func TestAgentHookRejectsAnUnconfiguredPairIndependentlyOfHostConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"agent-hook", "--host-config=" + filepath.Join(t.TempDir(), "missing.json"), "Claude", "UnsupportedEvent"},
		emptyCommandInput(t),
		&stdout,
		&stderr,
	)
	if exitCode != exitUsage {
		t.Fatalf("agent-hook exit code = %d, want %d for a rejected invocation independent of host config", exitCode, exitUsage)
	}
}

func TestAgentHookDrainsFiniteProviderInputWhenHostConfigFails(t *testing.T) {
	t.Setenv("TMUX_PANE", "%7")
	input, provider, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	payload := append([]byte(`{"session_id":"thr_123","opaque":"`), bytes.Repeat([]byte("x"), 48*1024)...)
	payload = append(payload, []byte(`"}`)...)
	written := make(chan error, 1)
	go func() {
		_, writeErr := provider.Write(payload)
		if closeErr := provider.Close(); writeErr == nil {
			writeErr = closeErr
		}
		written <- writeErr
	}()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run(
		[]string{"agent-hook", "--host-config=" + filepath.Join(t.TempDir(), "missing.json"), "Codex", "SessionStart"},
		input,
		&stdout,
		&stderr,
	); exitCode != 0 {
		t.Fatalf("agent-hook exit code = %d, want fail-open success", exitCode)
	}
	select {
	case writeErr := <-written:
		if writeErr != nil {
			t.Fatalf("provider payload write failed before the hook drained it: %v", writeErr)
		}
	case <-time.After(time.Second):
		_ = input.Close()
		<-written
		t.Fatal("agent-hook returned from host-config failure before draining finite provider input")
	}
}

func TestAwaitAgentHookInputReportsAnUnexpectedCancellationCloseFailure(t *testing.T) {
	input := newCloseErrorHookInput()
	result := prepareAgentHookInput("Codex", "SessionStart", input)
	<-input.readStarted
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := awaitAgentHookInput(ctx, input, result)
	if !errors.Is(err, errAgentHookInputClose) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation close error = %v, want content-free owned error", err)
	}
}

type closeErrorHookInput struct {
	readStarted chan struct{}
	closed      chan struct{}
	readOnce    sync.Once
	closeOnce   sync.Once
}

func newCloseErrorHookInput() *closeErrorHookInput {
	return &closeErrorHookInput{readStarted: make(chan struct{}), closed: make(chan struct{})}
}

func (input *closeErrorHookInput) Read([]byte) (int, error) {
	input.readOnce.Do(func() { close(input.readStarted) })
	<-input.closed
	return 0, io.EOF
}

func (input *closeErrorHookInput) Close() error {
	input.closeOnce.Do(func() { close(input.closed) })
	return errors.New("injected close failure")
}

func TestAgentHookTotalDeadlineBoundsBlockingProviderInput(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	tmuxPath := writeTmuxVersion(t, "tmux test")
	hostConfigPath := writeHostConfig(t, tmuxPath, "tmux test")
	input, provider, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = provider.Close()
	})
	returned := make(chan int, 1)
	startedAt := time.Now()
	go func() {
		returned <- run(
			[]string{"agent-hook", "--host-config=" + hostConfigPath, "Codex", "SessionStart"},
			input,
			io.Discard,
			io.Discard,
		)
	}()

	select {
	case exitCode := <-returned:
		if exitCode != 0 {
			t.Fatalf("agent-hook exit code = %d, want fail-open success", exitCode)
		}
		if elapsed := time.Since(startedAt); elapsed > agenthook.PublicationDeadline+time.Second {
			t.Fatalf("agent-hook returned after %s, beyond its total deadline", elapsed)
		}
	case <-time.After(agenthook.PublicationDeadline + time.Second):
		_ = provider.Close()
		<-returned
		t.Fatal("agent-hook outlived its total deadline on blocking provider input")
	}
}

func TestAgentHookRejectsANonRegularHostConfigWithoutOutlivingItsDeadline(t *testing.T) {
	t.Setenv("TMUX_PANE", "%7")
	configPath := filepath.Join(t.TempDir(), "host-config.fifo")
	if err := syscall.Mkfifo(configPath, 0o600); err != nil {
		t.Fatal(err)
	}
	input := commandInput(t, `{"session_id":"thr_123"}`)
	returned := make(chan int, 1)
	go func() {
		returned <- run(
			[]string{"agent-hook", "--host-config=" + configPath, "Codex", "SessionStart"},
			input,
			io.Discard,
			io.Discard,
		)
	}()

	select {
	case exitCode := <-returned:
		if exitCode != 0 {
			t.Fatalf("agent-hook exit code = %d, want fail-open success", exitCode)
		}
	case <-time.After(agenthook.PublicationDeadline + time.Second):
		writer, err := os.OpenFile(configPath, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_ = writer.Close()
		<-returned
		t.Fatal("agent-hook blocked on a non-regular host-config path beyond its total deadline")
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

func emptyCommandInput(t *testing.T) *os.File {
	t.Helper()
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	return input
}

func commandInput(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	return input
}

func writeTmuxVersionThenFail(t *testing.T, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tmux")
	script := "#!/bin/sh\nif [ \"${1:-}\" = -V ]; then printf '%s\\n' " + fmt.Sprintf("%q", version) + "; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeHostConfig(t *testing.T, tmuxPath, tmuxTestedVersion string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host.json")
	encoded := fmt.Sprintf(`{
  "platform": %q,
  "nativeControlPath": "/usr/local/bin/provider-runtime-control",
	  "tmux": {"path": %q, "testedVersion": %q},
  "profiles": [
    {"key":"personal","label":"Codex · Personal","provider":"Codex","command":"/home/niels/bin/codex-personal","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-personal"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work","label":"Codex · Work","provider":"Codex","command":"/home/niels/bin/codex-work","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work2","label":"Codex · Work 2","provider":"Codex","command":"/home/niels/bin/codex-work2","environment":[{"name":"CODEX_HOME","value":"/home/niels/.codex-work2"}],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"claude-work","label":"Claude · Work","provider":"Claude","command":"/home/niels/bin/claude-work","environment":[{"name":"CLAUDE_CONFIG_DIR","value":"/home/niels/.claude-work"}],"foregroundSignatures":[{"argument0":"/home/niels/.local/bin/claude"}],"arguments":[]}
  ]
}`, platform.Current().Kind, tmuxPath, tmuxTestedVersion)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClientSubprocess(t *testing.T) {
	if os.Getenv("SKID_TEST_CLIENT_CHILD") != "1" {
		return
	}
	certificate, err := os.ReadFile(os.Getenv("SKID_TEST_CLIENT_CA"))
	if err != nil {
		os.Exit(90)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificate) {
		os.Exit(91)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	address := os.Getenv("SKID_TEST_CLIENT_ADDRESS")
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	http.DefaultTransport = transport
	start := 0
	for i, arg := range os.Args {
		if arg == "--" {
			start = i + 1
			break
		}
	}
	os.Exit(run(os.Args[start:], os.Stdin, os.Stdout, os.Stderr))
}

func TestClientRealSubprocessHTTPBoundary(t *testing.T) {
	const machine = "mh-11111111111111111111111111111111"
	const bearer = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	const fixture = "first line\nsecond `$(literal)`\n"
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+bearer || r.Header.Get("Skidbladnir-Machine") != machine {
			t.Error("authentication did not cross subprocess boundary")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["paneId"] != "%4" || body["pid"] != float64(321) {
			t.Error("target did not cross subprocess boundary")
		}
		switch r.URL.Path {
		case "/v1/sessions/$3/agent/send":
			if body["text"] != fixture {
				t.Error("stdin bytes changed across subprocess/http")
			}
		case "/v1/sessions/$3/agent/keys":
			keys, ok := body["keys"].([]any)
			if !ok || len(keys) != 2 || keys[0] != "up" || keys[1] != "enter" {
				t.Error("keys changed across subprocess/http")
			}
		default:
			t.Error("unexpected operation")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"method":"terminal","outcome":"written"}`)
	}))
	defer server.Close()
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skidbladnir")
	if os.MkdirAll(configDir, 0700) != nil {
		t.Fatal("cannot create fixture config directory")
	}
	if os.WriteFile(filepath.Join(configDir, "client.json"), []byte(`{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"`+machine+`","bearer":"`+bearer+`"}]}`), 0600) != nil {
		t.Fatal("cannot create private client config")
	}
	ca := filepath.Join(home, "fixture-ca.pem")
	if os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600) != nil {
		t.Fatal("cannot write fixture certificate")
	}
	ref := base64.RawURLEncoding.EncodeToString([]byte(`{"machine":"` + machine + `","tmuxId":"$3","identityToken":"lifetime","agent":{"paneId":"%4","pid":321,"startIdentity":"1234"}}`))
	for _, operation := range []string{"send", "keys"} {
		args := []string{"-test.run=^TestClientSubprocess$", "--", operation, "--json", "--ref", ref}
		if operation == "send" {
			args = append(args, "--stdin")
		} else {
			args = append(args, "up", "enter")
		}
		child := exec.Command(os.Args[0], args...)
		child.Env = append(os.Environ(), "SKID_TEST_CLIENT_CHILD=1", "SKID_TEST_CLIENT_CA="+ca, "SKID_TEST_CLIENT_ADDRESS="+server.Listener.Addr().String(), "HOME="+home)
		child.Stdin = strings.NewReader(fixture)
		var stdout, stderr bytes.Buffer
		child.Stdout = &stdout
		child.Stderr = &stderr
		if child.Run() != nil || !json.Valid(stdout.Bytes()) || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 || stderr.Len() != 0 {
			t.Fatalf("subprocess failed: stdout_bytes=%d stderr_bytes=%d", stdout.Len(), stderr.Len())
		}
	}
	if calls.Load() != 2 {
		t.Fatal("subprocess replayed or omitted operation")
	}
}
