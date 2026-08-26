//go:build live

package live

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	liveTmuxPath        = "/usr/bin/tmux"
	tmuxBehaviorTimeout = 10 * time.Second
)

type tmuxBehaviorServer struct {
	socket          string
	root            string
	shadow          string
	pid             int
	startTime       uint64
	tmuxStartTime   string
	fixtures        []tmuxBehaviorProcess
	lastSessionGone bool
}

type tmuxBehaviorClient struct {
	cmd       *exec.Cmd
	feed      io.WriteCloser
	pid       int
	startTime uint64

	mu   sync.Mutex
	out  bytes.Buffer
	done chan error
}

type tmuxBehaviorPane struct {
	ID        string
	PID       int
	TTY       string
	StartTime uint64
}

func TestTmuxGroupedBehavior(t *testing.T) {
	if os.Getenv("SKIDBLADNIR_ALLOW_ISOLATED_TMUX_TESTS") != "isolated-v1" {
		t.Fatal("live tmux proof requires explicit isolated tmux approval")
	}
	if os.Getenv("TMUX") != "" || os.Getenv("TMUX_PANE") != "" {
		t.Fatal("live tmux proof refuses an invoking tmux client")
	}
	versionCommand := exec.Command(liveTmuxPath, "-V")
	versionCommand.Env = withoutTmuxEnvironment(os.Environ())
	version, err := versionCommand.Output()
	if err != nil || strings.TrimSpace(string(version)) != "tmux 3.4" {
		t.Fatalf("P0 proof requires exact tmux 3.4, found %q (error=%v)", strings.TrimSpace(string(version)), err)
	}
	const scriptPath = "/usr/bin/script"
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("real PTY client helper %s is unavailable: %v", scriptPath, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("real PTY client helper %s is not executable", scriptPath)
	}

	server := newTmuxBehaviorServer(t)
	rootDir := t.TempDir()
	logA := filepath.Join(rootDir, "pane-a.log")
	logB := filepath.Join(rootDir, "pane-b.log")
	server.start(t, rootDir, logA, logB)

	panesBefore := server.panes(t)
	rootBefore := panesBefore[server.root]
	shadowBefore := panesBefore[server.shadow]
	assertSamePaneSet(t, rootBefore, shadowBefore, "group creation")

	laptop := startTmuxBehaviorClient(t, server, server.root, "", 120, 40)
	phone := startTmuxBehaviorClient(t, server, server.shadow, "active-pane,ignore-size", 80, 24)

	assertClientShape(t, server, server.root, "", "120x40")
	assertClientShape(t, server, server.shadow, "active-pane,ignore-size", "80x24")
	assertWindowSize(t, server, "120x40", "unflagged laptop takes geometry")

	server.selectPane(t, server.root, 0)
	waitForCapture(t, server, "FRAME=A BUFFER=seed-A", server.root+":0.0")
	phone.waitForOutput(t, "FRAME=A")

	laptop.send(t, "LAPTOP-A\r")
	waitForLog(t, logA, "LAPTOP-A")
	assertLogAbsent(t, logB, "LAPTOP-A")

	// This is a real client key sequence, rather than tmux send-keys. The
	// active-pane flag makes this selection local to the phone client.
	phone.send(t, "\x02o")
	phone.send(t, "PHONE-B\r")
	waitForLog(t, logB, "PHONE-B")
	assertLogAbsent(t, logA, "PHONE-B")

	laptop.send(t, "LAPTOP-A2\r")
	waitForLog(t, logA, "LAPTOP-A2")
	assertLogAbsent(t, logB, "LAPTOP-A2")
	assertWindowActivePane(t, server, 0)

	server.detachClient(t, server.clientFor(t, server.root))
	waitForClientCount(t, server, server.root, 0)
	assertWindowSize(t, server, "80x24", "sole ignore-size client")
	assertClientShape(t, server, server.shadow, "active-pane,ignore-size", "80x24")

	laptopAfterDetach := startTmuxBehaviorClient(t, server, server.root, "", 120, 40)
	assertClientShape(t, server, server.root, "", "120x40")
	assertWindowSize(t, server, "120x40", "new unflagged laptop retakes geometry")
	assertClientShape(t, server, server.shadow, "active-pane,ignore-size", "80x24")

	shadowPaneAfter := server.panes(t)[server.shadow]
	assertSamePaneSet(t, rootBefore, shadowPaneAfter, "before non-last kill")
	server.killSession(t, server.shadow)
	waitForClientCount(t, server, server.shadow, 0)
	rootAfterNonLast := server.panes(t)[server.root]
	assertSamePaneSet(t, rootBefore, rootAfterNonLast, "non-last grouped session kill")
	laptopAfterDetach.send(t, "\x02o")
	laptopAfterDetach.send(t, "ROOT-POST-SHADOW-KILL\r")
	waitForLog(t, logB, "ROOT-POST-SHADOW-KILL")
	assertLogAbsent(t, logA, "ROOT-POST-SHADOW-KILL")
	laptopAfterDetach.send(t, "\x02o")
	assertWindowActivePane(t, server, 0)

	fresh := startTmuxBehaviorClient(t, server, server.root, "active-pane,ignore-size", 80, 24)
	fresh.waitForOutput(t, "FRAME=A")
	waitForCapture(t, server, "FRAME=A\nTOKEN=LAPTOP-A2", server.root+":0.0")

	oldPIDs := []int{rootBefore[0].PID, rootBefore[1].PID}
	oldTTYs := []string{rootBefore[0].TTY, rootBefore[1].TTY}
	server.assertIdentity(t)
	server.killSession(t, server.root)
	server.lastSessionGone = true
	waitForClientCount(t, server, server.root, 0)
	waitForNoPanes(t, server)
	for _, pid := range oldPIDs {
		waitForProcessGone(t, pid)
	}
	for _, tty := range oldTTYs {
		waitForTTYGone(t, tty)
	}
}

func newTmuxBehaviorServer(t *testing.T) *tmuxBehaviorServer {
	t.Helper()
	socketPath := registerLiveTmuxSocket(t)
	server := &tmuxBehaviorServer{socket: socketPath, root: "root", shadow: "shadow"}
	t.Cleanup(func() {
		if !server.cleanup(t) {
			return
		}
		// tmux 3.4 can leave a stale alternate socket after the server exits;
		// all owned process identities are proven gone before removing it.
		if err := removeRegisteredLiveStaleSocket(liveTmuxPath, server.socketPath()); err != nil {
			t.Errorf("remove stopped registered live tmux socket %s: %v", server.socketPath(), err)
		}
	})
	return server
}

func (s *tmuxBehaviorServer) start(t *testing.T, cwd, logA, logB string) {
	t.Helper()
	s.run(t, "new-session", "-d", "-s", s.root, "-x", "120", "-y", "40", "-c", cwd, fixtureCommand(logA, "A"))
	s.captureIdentity(t)
	s.run(t, "set-option", "-g", "window-size", "latest")
	s.run(t, "set-option", "-g", "status", "off")
	s.run(t, "split-window", "-t", s.root+":0", "-h", fixtureCommand(logB, "B"))
	s.run(t, "select-pane", "-t", s.root+":0.0")
	s.run(t, "new-session", "-d", "-t", s.root, "-s", s.shadow)
	for _, pane := range s.panes(t)[s.root] {
		s.fixtures = append(s.fixtures, tmuxBehaviorProcess{PID: pane.PID, StartTime: pane.StartTime})
	}
}

func fixtureCommand(logPath, frame string) string {
	script := `log=$1
frame=$2
	printf '\033[?1049h\033[2J\033[HFRAME=%s BUFFER=seed-%s\nREADY\n' "$frame" "$frame"
while IFS= read -r token; do
  printf '%s\n' "$token" >> "$log"
  printf '\033[2J\033[HFRAME=%s\nTOKEN=%s\n' "$frame" "$token"
done`
	return "exec /bin/sh -c " + shellQuote(script) + " fixture " + shellQuote(logPath) + " " + shellQuote(frame)
}

func startTmuxBehaviorClient(t *testing.T, server *tmuxBehaviorServer, session, flags string, cols, rows int) *tmuxBehaviorClient {
	t.Helper()
	if err := validateRegisteredLiveSocket(server.socket); err != nil {
		t.Fatal(err)
	}
	attach := fmt.Sprintf("stty cols %d rows %d; exec env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR TERM=xterm-256color %s -S %s -f /dev/null attach-session", cols, rows, shellQuote(liveTmuxPath), shellQuote(server.socket))
	if flags != "" {
		attach += " -f " + shellQuote(flags)
	}
	attach += " -t " + shellQuote(session)
	cmd := exec.Command("/usr/bin/script", "-qefc", attach, "/dev/null")
	cmd.Env = withoutTmuxEnvironment(os.Environ())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("create PTY client stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("create PTY client stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start PTY client: %v", err)
	}
	client := &tmuxBehaviorClient{cmd: cmd, feed: stdin, pid: cmd.Process.Pid, startTime: processStartTime(cmd.Process.Pid), done: make(chan error, 1)}
	go func() {
		var copyErr error
		buffer := make([]byte, 4096)
		for {
			n, readErr := stdout.Read(buffer)
			if n > 0 {
				client.mu.Lock()
				_, _ = client.out.Write(buffer[:n]) // justify-ignore-error: bytes.Buffer.Write cannot fail.
				client.mu.Unlock()
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					copyErr = readErr
				}
				break
			}
		}
		waitErr := cmd.Wait()
		if copyErr != nil && !errors.Is(copyErr, os.ErrClosed) {
			client.done <- copyErr
			return
		}
		client.done <- waitErr
	}()
	t.Cleanup(func() {
		_ = stdin.Close() // justify-ignore-error: cleanup closes only this test-owned PTY input after tmux teardown.
		if cmd.Process != nil {
			_ = cmd.Process.Kill() // justify-ignore-error: cleanup accepts an already-exited test-owned PTY helper.
		}
		select {
		case <-client.done:
		case <-time.After(2 * time.Second):
			t.Errorf("PTY client did not exit: session=%s", session)
		}
		waitForProcessGone(t, client.pid)
	})
	if client.pid <= 0 || client.startTime == 0 || !processAlive(client.pid) {
		t.Fatalf("invalid /usr/bin/script client identity: pid=%d start=%d", client.pid, client.startTime)
	}
	waitForClientAtLeast(t, server, session, 1)
	return client
}

func (c *tmuxBehaviorClient) send(t *testing.T, value string) {
	t.Helper()
	if _, err := io.WriteString(c.feed, value); err != nil {
		t.Fatalf("send PTY client input %q: %v", value, err)
	}
}

func (c *tmuxBehaviorClient) waitForOutput(t *testing.T, marker string) {
	t.Helper()
	waitUntil(t, "PTY output "+marker, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return strings.Contains(c.out.String(), marker)
	})
}

func (s *tmuxBehaviorServer) run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := s.command(args...)
	if err != nil {
		t.Fatalf("tmux %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func (s *tmuxBehaviorServer) command(args ...string) ([]byte, error) {
	cmd, err := registeredLiveTmuxCommand(context.Background(), liveTmuxPath, s.socket, args...)
	if err != nil {
		return nil, err
	}
	out, err := cmd.CombinedOutput()
	return out, err
}

func (s *tmuxBehaviorServer) panes(t *testing.T) map[string][]tmuxBehaviorPane {
	t.Helper()
	format := "#{session_name}\t#{pane_id}\t#{pane_pid}\t#{pane_tty}"
	out := s.run(t, "list-panes", "-a", "-F", format)
	result := map[string][]tmuxBehaviorPane{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || fields[0] == "" {
			continue
		}
		pid, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatalf("parse tmux pane PID %q: %v", line, err)
		}
		startTime := processStartTime(pid)
		if pid <= 0 || startTime == 0 || !processAlive(pid) {
			t.Fatalf("invalid live pane identity: session=%s pid=%d start=%d", fields[0], pid, startTime)
		}
		if fields[3] == "" || !filepath.IsAbs(fields[3]) {
			t.Fatalf("invalid pane TTY: session=%s tty=%q", fields[0], fields[3])
		}
		fdTTY, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", "0"))
		if err != nil {
			t.Fatalf("read pane stdin TTY: session=%s pid=%d: %v", fields[0], pid, err)
		}
		if fdTTY != fields[3] {
			t.Fatalf("pane stdin TTY mismatch: session=%s pid=%d fd0=%q pane=%q", fields[0], pid, fdTTY, fields[3])
		}
		result[fields[0]] = append(result[fields[0]], tmuxBehaviorPane{
			ID: fields[1], PID: pid, TTY: fields[3], StartTime: startTime,
		})
	}
	return result
}

func (s *tmuxBehaviorServer) clientFor(t *testing.T, session string) string {
	t.Helper()
	out := s.run(t, "list-clients", "-F", "#{client_name}\t#{session_name}")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 2 && fields[1] == session {
			return fields[0]
		}
	}
	t.Fatalf("no tmux client for session %s", session)
	return ""
}

func (s *tmuxBehaviorServer) selectPane(t *testing.T, session string, index int) {
	t.Helper()
	s.run(t, "select-pane", "-t", fmt.Sprintf("%s:0.%d", session, index))
}

func (s *tmuxBehaviorServer) detachClient(t *testing.T, client string) {
	t.Helper()
	if observed := processStartTime(s.pid); observed != s.startTime {
		t.Fatalf("refusing client detach after process identity changed: pid=%d captured=%d observed=%d", s.pid, s.startTime, observed)
	}
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	output, err := s.command("if-shell", "-F", s.identityCondition(),
		"detach-client -t "+shellQuote(client),
		"display-message -p -l '"+mismatchMarker+"'")
	if err != nil {
		t.Fatalf("conditionally detach test-owned client: client=%s error=%v output=%q", client, err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("refusing client detach after routed identity mismatch: client=%s output=%q", client, output)
	}
}

func (s *tmuxBehaviorServer) killSession(t *testing.T, session string) {
	t.Helper()
	if observed := processStartTime(s.pid); observed != s.startTime {
		t.Fatalf("refusing session kill after process identity changed: pid=%d captured=%d observed=%d", s.pid, s.startTime, observed)
	}
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	output, err := s.command("if-shell", "-F", s.identityCondition(),
		"kill-session -t "+shellQuote(session),
		"display-message -p -l '"+mismatchMarker+"'")
	if err != nil {
		t.Fatalf("conditionally kill test-owned session: session=%s error=%v output=%q", session, err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("refusing session kill after routed identity mismatch: session=%s output=%q", session, output)
	}
}

type tmuxBehaviorProcess struct {
	PID       int
	StartTime uint64
}

func (s *tmuxBehaviorServer) captureIdentity(t *testing.T) {
	t.Helper()
	value := strings.TrimSpace(s.run(t, "display-message", "-p", "#{pid}|#{start_time}"))
	pidText, tmuxStartTime, separated := strings.Cut(value, "|")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 || !separated || tmuxStartTime == "" {
		t.Fatalf("invalid alternate tmux server identity %q: %v", value, err)
	}
	startTime := processStartTime(pid)
	if startTime == 0 || !processAlive(pid) {
		t.Fatalf("alternate tmux server is not live: pid=%d start=%d", pid, startTime)
	}
	s.pid = pid
	s.startTime = startTime
	s.tmuxStartTime = tmuxStartTime
}

func (s *tmuxBehaviorServer) assertIdentity(t *testing.T) {
	t.Helper()
	if !s.routedIdentityMatches(t) {
		t.FailNow()
	}
}

func (s *tmuxBehaviorServer) cleanup(t *testing.T) bool {
	t.Helper()
	if s.pid <= 0 || s.startTime == 0 || s.tmuxStartTime == "" {
		t.Errorf("refusing cleanup because alternate tmux server identity was not captured")
		return false
	}
	if s.lastSessionGone {
		if observed := processStartTime(s.pid); observed == s.startTime && processAlive(s.pid) {
			t.Errorf("alternate tmux server survived its last session: pid=%d start=%d", s.pid, s.startTime)
			return false
		}
		if output, err := s.command("display-message", "-p", "#{pid}"); err == nil {
			t.Errorf("refusing stale-socket cleanup because a server still answers: socket=%s routed=%q", s.socket, strings.TrimSpace(string(output)))
			return false
		}
		return true
	}
	if observed := processStartTime(s.pid); observed != s.startTime {
		t.Errorf("refusing cleanup after process identity changed: pid=%d captured=%d observed=%d", s.pid, s.startTime, observed)
		return false
	}
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	output, err := s.command("if-shell", "-F", s.identityCondition(),
		"kill-server",
		"display-message -p -l '"+mismatchMarker+"'")
	if err != nil {
		t.Errorf("conditionally kill test-owned tmux server: error=%v output=%q", err, output)
		return false
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("refusing server kill after routed identity mismatch: output=%q", output)
		return false
	}
	waitForProcessGone(t, s.pid)
	for _, fixture := range s.fixtures {
		waitForProcessGone(t, fixture.PID)
	}
	return true
}

func (s *tmuxBehaviorServer) routedIdentityMatches(t *testing.T) bool {
	t.Helper()
	output, err := s.command("display-message", "-p", "#{pid}|#{start_time}")
	if err != nil {
		t.Errorf("refusing destructive tmux command without routed identity: socket=%s error=%v", s.socket, err)
		return false
	}
	routedPIDText, routedTmuxStart, separated := strings.Cut(strings.TrimSpace(string(output)), "|")
	routedPID, err := strconv.Atoi(routedPIDText)
	if err != nil || routedPID != s.pid || !separated || routedTmuxStart != s.tmuxStartTime {
		t.Errorf("refusing destructive tmux command after socket identity changed: socket=%s captured=%d routed=%q error=%v", s.socket, s.pid, output, err)
		return false
	}
	observed := processStartTime(routedPID)
	if observed == 0 || observed != s.startTime || !processAlive(routedPID) {
		t.Errorf("refusing destructive tmux command after process identity changed: pid=%d captured=%d observed=%d", routedPID, s.startTime, observed)
		return false
	}
	return true
}

func (s *tmuxBehaviorServer) identityCondition() string {
	return fmt.Sprintf("#{==:#{pid}:#{start_time},%d:%s}", s.pid, s.tmuxStartTime)
}

func (s *tmuxBehaviorServer) socketPath() string {
	return s.socket
}

func assertSamePaneSet(t *testing.T, want, got []tmuxBehaviorPane, context string) {
	t.Helper()
	if len(want) != 2 || len(got) != 2 {
		t.Fatalf("%s: expected two panes, want=%+v got=%+v", context, want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s: pane identity changed at index %d: want=%+v got=%+v", context, i, want[i], got[i])
		}
	}
}

func assertClientShape(t *testing.T, server *tmuxBehaviorServer, session, requiredFlags, requiredSize string) {
	t.Helper()
	out := server.run(t, "list-clients", "-F", "#{session_name}\t#{client_flags}\t#{client_width}x#{client_height}")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 3 && fields[0] == session {
			flags := strings.Split(fields[1], ",")
			for _, required := range strings.Split(requiredFlags, ",") {
				if required == "" || containsString(flags, required) {
					continue
				}
				t.Fatalf("client %s shape mismatch: got flags=%q size=%q want flags containing %q size=%q", session, fields[1], fields[2], requiredFlags, requiredSize)
			}
			if fields[2] != requiredSize {
				t.Fatalf("client %s shape mismatch: got flags=%q size=%q want flags containing %q size=%q", session, fields[1], fields[2], requiredFlags, requiredSize)
			}
			return
		}
	}
	t.Fatalf("no client shape for session %s", session)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertWindowSize(t *testing.T, server *tmuxBehaviorServer, want, context string) {
	t.Helper()
	out := strings.TrimSpace(server.run(t, "display-message", "-t", server.root+":0", "-p", "#{window_width}x#{window_height}"))
	if out != want {
		t.Fatalf("%s: window size %s, want %s", context, out, want)
	}
}

func assertWindowActivePane(t *testing.T, server *tmuxBehaviorServer, index int) {
	t.Helper()
	out := strings.TrimSpace(server.run(t, "list-panes", "-t", server.root+":0", "-F", "#{pane_index}\t#{pane_active}"))
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 2 && fields[1] == "1" {
			got, err := strconv.Atoi(fields[0])
			if err != nil || got != index {
				t.Fatalf("window active pane is %q, want %d", fields[0], index)
			}
			return
		}
	}
	t.Fatal("tmux reported no active pane")
}

func waitForCapture(t *testing.T, server *tmuxBehaviorServer, marker, target string) {
	t.Helper()
	var last string
	deadline := time.Now().Add(tmuxBehaviorTimeout)
	for time.Now().Before(deadline) {
		last = server.run(t, "capture-pane", "-p", "-J", "-t", target, "-S", "-")
		if strings.Contains(last, marker) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for capture %q; last=%q", marker, last)
}

func waitForLog(t *testing.T, path, marker string) {
	t.Helper()
	waitUntil(t, "log "+marker, func() bool {
		data, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(data), marker)
	})
}

func assertLogAbsent(t *testing.T, path, marker string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err == nil && strings.Contains(string(data), marker) {
		t.Fatalf("unexpected token %q in %s", marker, path)
	}
}

func waitForClientCount(t *testing.T, server *tmuxBehaviorServer, session string, want int) {
	t.Helper()
	waitUntil(t, fmt.Sprintf("client count %s=%d", session, want), func() bool {
		out, _ := server.command("list-clients", "-F", "#{session_name}") // justify-ignore-error: no clients is the expected zero-count teardown state.
		count := 0
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == session {
				count++
			}
		}
		return count == want
	})
}

func waitForClientAtLeast(t *testing.T, server *tmuxBehaviorServer, session string, want int) {
	t.Helper()
	waitUntil(t, fmt.Sprintf("client count %s>=%d", session, want), func() bool {
		out, _ := server.command("list-clients", "-F", "#{session_name}") // justify-ignore-error: no clients is the expected pre-attach state.
		count := 0
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == session {
				count++
			}
		}
		return count >= want
	})
}

func waitForNoPanes(t *testing.T, server *tmuxBehaviorServer) {
	t.Helper()
	waitUntil(t, "all panes destroyed", func() bool {
		out, err := server.command("list-panes", "-a", "-F", "#{pane_id}")
		return err != nil || strings.TrimSpace(string(out)) == ""
	})
}

func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	waitUntil(t, fmt.Sprintf("process %d gone", pid), func() bool {
		return !processAlive(pid)
	})
}

func waitForTTYGone(t *testing.T, tty string) {
	t.Helper()
	if tty == "" {
		t.Fatal("last grouped session reported an empty pane TTY")
	}
	waitUntil(t, "pane TTY "+tty+" gone", func() bool {
		_, err := os.Stat(tty)
		return errors.Is(err, os.ErrNotExist)
	})
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func processStartTime(pid int) uint64 {
	contents, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	closingParenthesis := strings.LastIndexByte(string(contents), ')')
	if closingParenthesis < 0 || closingParenthesis+2 >= len(contents) {
		return 0
	}
	fields := strings.Fields(string(contents[closingParenthesis+2:]))
	if len(fields) < 20 {
		return 0
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0
	}
	return startTime
}

func waitUntil(t *testing.T, description string, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(tmuxBehaviorTimeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func withoutTmuxEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		if strings.HasPrefix(entry, "TMUX=") || strings.HasPrefix(entry, "TMUX_PANE=") || strings.HasPrefix(entry, "TMUX_TMPDIR=") {
			continue
		}
		result = append(result, entry)
	}
	return result
}
