//go:build live

package live

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type tuiBehaviorProvider struct {
	server   *http.Server
	listener net.Listener
	requests atomic.Int64
}

type tuiBehaviorResponder func(http.ResponseWriter, *http.Request, []byte, int64)

type tuiBehaviorHarness struct {
	socket   string
	process  directTUIProcess
	provider *tuiBehaviorProvider
	client   *tuiBehaviorClient
}

type tuiBehaviorClient struct {
	command *exec.Cmd
	input   io.WriteCloser
	done    chan error
	mu      sync.Mutex
	process directTUIProcess
}

func TestDirectTUIKeys(t *testing.T) {
	requireExactTmux(t)
	lock := readDirectTUILock(t)
	harness := startTUIBehaviorHarness(t, lock.BinaryPath)
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "Ask Codex to do anything")
	}, "stock composer readiness")

	if alternate := readTmuxFormat(t, harness.socket, "#{alternate_on}"); alternate != "0" {
		t.Fatalf("stock TUI normal-buffer state = %q, want 0", alternate)
	}
	sendTUILiteral(t, harness.socket, "ac")
	sendTUIKey(t, harness.socket, "Left")
	sendTUILiteral(t, harness.socket, "b")
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "abc")
	}, "intra-line edit")

	sendTUIClientBytes(t, harness.client, "\x0a")
	sendTUILiteral(t, harness.socket, "def")
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "› ab") && strings.Contains(capture, "  defc")
	}, "Ctrl-J newline without submit")
	time.Sleep(200 * time.Millisecond)
	if requests := harness.provider.requests.Load(); requests != 0 {
		t.Fatalf("Ctrl-J newline submitted %d provider requests, want 0", requests)
	}

	sendTUIClientBytes(t, harness.client, "\x0d")
	waitForTUIProviderRequests(t, harness.provider, 1)
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "synthetic-line-079") && strings.Contains(capture, "Ask Codex to do anything")
	}, "idle composer after loopback response")
	sendTUIKey(t, harness.socket, "Up")
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "› ab") && strings.Contains(capture, "  defc")
	}, "local prompt history recall")

	sendTUIKey(t, harness.socket, "C-t")
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return strings.Contains(capture, "T R A N S C R I P T")
	}, "transcript pager")
	before := captureTUIPane(t, harness.socket)
	sendTUIKey(t, harness.socket, "PageUp")
	waitForTUICapture(t, harness.socket, func(capture string) bool {
		return capture != before
	}, "transcript page-up")
	sendTUIKey(t, harness.socket, "End")
	sendTUIKey(t, harness.socket, "q")
	waitForTmuxFormat(t, harness.socket, "#{alternate_on}", "0", "transcript pager close")
}

func startTUIBehaviorHarness(t *testing.T, binary string) tuiBehaviorHarness {
	t.Helper()
	provider := startTUIBehaviorProvider(t)
	home := t.TempDir()
	config := fmt.Sprintf(`model = "gpt-5.1-codex-mini"
model_provider = "skidbladnir-loopback"
approval_policy = "never"
sandbox_mode = "danger-full-access"

[projects.%q]
trust_level = "trusted"

[model_providers.skidbladnir-loopback]
name = "Skidbladnir loopback"
base_url = %q
wire_api = "responses"
requires_openai_auth = false
request_max_retries = 0
stream_max_retries = 0
supports_websockets = false
`, directTUICWD, "http://"+provider.listener.Addr().String()+"/v1")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatalf("write loopback Codex config: %v", err)
	}

	socket := fmt.Sprintf("skidbladnir_keys_%d_%d", os.Getpid(), time.Now().UnixNano())
	runtimeID := fmt.Sprintf("%08x-2222-4222-8222-%012x", uint32(time.Now().UnixNano()), uint64(os.Getpid()))
	startMarker := filepath.Join(t.TempDir(), "start")
	codexArguments := []string{binary, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD}
	quotedCodex := make([]string, len(codexArguments))
	for index, argument := range codexArguments {
		quotedCodex[index] = shellQuote(argument)
	}
	launch := "while [ ! -e " + shellQuote(startMarker) + " ]; do /usr/bin/sleep 0.01; done; exec " + strings.Join(quotedCodex, " ")
	arguments := []string{
		"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "keys",
		"-x", "100", "-y", "30", "-c", directTUICWD,
		"-e", "CODEX_HOME=" + home,
		"-e", "SKIDBLADNIR_RUNTIME_ID=" + runtimeID,
		"/bin/sh", "-c", launch,
	}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start loopback stock TUI: %v: %s", err, boundedDirectTUIError(output))
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: cleanup accepts an already-exited test-owned tmux server.
	})
	client := startTUIBehaviorClient(t, socket)
	if err := os.WriteFile(startMarker, nil, 0o600); err != nil {
		t.Fatalf("release loopback TUI launch: %v", err)
	}
	process := awaitNamedTUIProcess(t, socket, "keys")
	return tuiBehaviorHarness{socket: socket, process: process, provider: provider, client: client}
}

func startTUIBehaviorClient(t *testing.T, socket string) *tuiBehaviorClient {
	return startNamedTUIBehaviorClient(t, socket, "keys")
}

func startNamedTUIBehaviorClient(t *testing.T, socket, session string) *tuiBehaviorClient {
	t.Helper()
	attach := "stty cols 100 rows 30; exec env -u TMUX -u TMUX_PANE TERM=xterm-256color tmux -L " + shellQuote(socket) + " -f /dev/null attach-session -t " + shellQuote(session)
	command := exec.Command("/usr/bin/script", "-qefc", attach, "/dev/null")
	command.Env = withoutTmuxEnvironment(os.Environ())
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create terminal probe input: %v", err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create terminal probe output: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start terminal probe client: %v", err)
	}
	client := &tuiBehaviorClient{command: command, input: input, done: make(chan error, 1)}
	go func() {
		_, copyErr := io.Copy(io.Discard, output)
		waitErr := command.Wait()
		if copyErr != nil {
			client.done <- copyErr
			return
		}
		client.done <- waitErr
	}()
	t.Cleanup(func() {
		_ = input.Close() // justify-ignore-error: cleanup closes only the test-owned terminal probe input.
		if directTUIProcessAlive(client.process) {
			_ = syscall.Kill(client.process.pid, syscall.SIGTERM) // justify-ignore-error: cleanup targets the exact test-owned tmux client PID/start.
		}
		if command.Process != nil {
			_ = command.Process.Kill() // justify-ignore-error: cleanup accepts an already-exited test-owned terminal probe.
		}
		select {
		case <-client.done:
		case <-time.After(2 * time.Second):
			t.Error("terminal probe client did not exit")
		}
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && directTUIProcessAlive(client.process) {
			time.Sleep(20 * time.Millisecond)
		}
		if directTUIProcessAlive(client.process) {
			_ = syscall.Kill(client.process.pid, syscall.SIGKILL) // justify-ignore-error: exact PID/start cleanup follows a bounded graceful wait.
			t.Error("exact terminal probe tmux client survived cleanup")
		}
	})
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		output, err := exec.Command("tmux", "-L", socket, "list-clients", "-F", "#{client_pid}|#{client_tty}|#{client_session}").Output()
		if err == nil {
			var matches []directTUIProcess
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				parts := strings.Split(line, "|")
				if len(parts) != 3 || parts[2] != session {
					continue
				}
				pid, parseErr := strconv.Atoi(parts[0])
				startTime := processStartTime(pid)
				if parseErr == nil && pid > 0 && startTime > 0 && filepath.IsAbs(parts[1]) {
					matches = append(matches, directTUIProcess{pid: pid, startTime: startTime, tty: filepath.Clean(parts[1])})
				}
			}
			if len(matches) > 1 {
				t.Fatal("terminal probe resolved multiple tmux clients for one test-owned session")
			}
			if len(matches) == 1 {
				client.process = matches[0]
				return client
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("terminal probe client did not attach")
	return nil
}

func sendTUIClientBytes(t *testing.T, client *tuiBehaviorClient, value string) {
	t.Helper()
	client.mu.Lock()
	defer client.mu.Unlock()
	if _, err := io.WriteString(client.input, value); err != nil {
		t.Fatalf("send exact terminal-client bytes: %v", err)
	}
}

func startTUIBehaviorProvider(t *testing.T) *tuiBehaviorProvider {
	return startTUIBehaviorProviderWithResponder(t, nil)
}

func startTUIBehaviorProviderWithResponder(t *testing.T, responder tuiBehaviorResponder) *tuiBehaviorProvider {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for loopback Responses provider: %v", err)
	}
	provider := &tuiBehaviorProvider{listener: listener}
	provider.server = &http.Server{
		ReadHeaderTimeout: 2 * time.Second,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPost || request.URL.Path != "/v1/responses" {
				http.Error(writer, "closed test provider route", http.StatusNotFound)
				return
			}
			body, err := io.ReadAll(io.LimitReader(request.Body, (1<<20)+1))
			if err != nil || len(body) > 1<<20 {
				http.Error(writer, "read request", http.StatusBadRequest)
				return
			}
			sequence := provider.requests.Add(1)
			if responder != nil {
				responder(writer, request, body, sequence)
				return
			}
			writeTUIBehaviorAssistant(writer, sequence, true)
		}),
	}
	go func() {
		_ = provider.server.Serve(listener) // justify-ignore-error: test cleanup closes the listener and owns the resulting server error.
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := provider.server.Shutdown(ctx); err != nil {
			t.Errorf("stop loopback Responses provider: %v", err)
		}
	})
	return provider
}

func writeTUIBehaviorAssistant(writer http.ResponseWriter, sequence int64, long bool) {
	identifier := fmt.Sprintf("resp-%d", sequence)
	text := "synthetic-done"
	if long {
		lines := make([]string, 80)
		for index := range lines {
			lines[index] = fmt.Sprintf("synthetic-line-%03d", index)
		}
		text = strings.Join(lines, "\n")
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	_, _ = fmt.Fprintf(writer, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":%q}}\n\n", identifier)                                                                                                                                        // justify-ignore-error: a disconnected test TUI is asserted through the missing completion below.
	_, _ = fmt.Fprintf(writer, "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"role\":\"assistant\",\"id\":\"msg-1\",\"content\":[{\"type\":\"output_text\",\"text\":%q}]}}\n\n", text)                        // justify-ignore-error: a disconnected test TUI is asserted through the missing completion below.
	_, _ = fmt.Fprintf(writer, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":%q,\"usage\":{\"input_tokens\":0,\"input_tokens_details\":null,\"output_tokens\":0,\"output_tokens_details\":null,\"total_tokens\":0}}}\n\n", identifier) // justify-ignore-error: a disconnected test TUI is asserted through the missing completion below.
}

func awaitNamedTUIProcess(t *testing.T, socket, session string) directTUIProcess {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		output, err := exec.Command("tmux", "-L", socket, "list-panes", "-t", session, "-F", "#{pane_pid}|#{pane_tty}|#{pane_dead}|#{pane_current_command}").Output()
		if err == nil {
			parts := strings.Split(strings.TrimSpace(string(output)), "|")
			if len(parts) == 4 && parts[2] == "0" && parts[3] == "codex" {
				pid, parseErr := strconv.Atoi(parts[0])
				if parseErr == nil {
					startTime := processStartTime(pid)
					if startTime != 0 {
						return directTUIProcess{pid: pid, startTime: startTime, tty: parts[1]}
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("loopback stock TUI did not reach a live foreground process")
	return directTUIProcess{}
}

func waitForTUICapture(t *testing.T, socket string, condition func(string) bool, behavior string) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if condition(captureTUIPane(t, socket)) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("stock TUI did not expose %s before the live deadline", behavior)
}

func captureTUIPane(t *testing.T, socket string) string {
	t.Helper()
	output, err := exec.Command("tmux", "-L", socket, "capture-pane", "-p", "-J", "-t", "keys:0.0").Output()
	if err != nil {
		t.Fatalf("capture test-owned TUI pane: %v", err)
	}
	return string(output)
}

func sendTUILiteral(t *testing.T, socket, value string) {
	sendNamedTUILiteral(t, socket, "keys", value)
}

func sendNamedTUILiteral(t *testing.T, socket, session, value string) {
	t.Helper()
	if output, err := exec.Command("tmux", "-L", socket, "send-keys", "-t", session+":0.0", "-l", value).CombinedOutput(); err != nil {
		t.Fatalf("send literal TUI input: %v: %s", err, boundedDirectTUIError(output))
	}
}

func sendTUIKey(t *testing.T, socket, key string) {
	sendNamedTUIKey(t, socket, "keys", key)
}

func sendNamedTUIKey(t *testing.T, socket, session, key string) {
	t.Helper()
	if output, err := exec.Command("tmux", "-L", socket, "send-keys", "-t", session+":0.0", key).CombinedOutput(); err != nil {
		t.Fatalf("send TUI key %s: %v: %s", key, err, boundedDirectTUIError(output))
	}
}

func sendNamedTUIHex(t *testing.T, socket, session, value string) {
	t.Helper()
	if output, err := exec.Command("tmux", "-L", socket, "send-keys", "-t", session+":0.0", "-H", value).CombinedOutput(); err != nil {
		t.Fatalf("send hexadecimal TUI input %s: %v: %s", value, err, boundedDirectTUIError(output))
	}
}

func readTmuxFormat(t *testing.T, socket, format string) string {
	return readNamedTmuxFormat(t, socket, "keys", format)
}

func readNamedTmuxFormat(t *testing.T, socket, session, format string) string {
	t.Helper()
	output, err := exec.Command("tmux", "-L", socket, "display-message", "-p", "-t", session+":0.0", format).Output()
	if err != nil {
		t.Fatalf("read test-owned tmux format: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func waitForTmuxFormat(t *testing.T, socket, format, want, behavior string) {
	waitForNamedTmuxFormat(t, socket, "keys", format, want, behavior)
}

func waitForNamedTmuxFormat(t *testing.T, socket, session, format, want, behavior string) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if readNamedTmuxFormat(t, socket, session, format) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tmux did not expose %s before the live deadline", behavior)
}

func waitForTUIProviderRequests(t *testing.T, provider *tuiBehaviorProvider, count int64) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if provider.requests.Load() >= count {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("loopback provider requests = %d, want at least %d", provider.requests.Load(), count)
}
