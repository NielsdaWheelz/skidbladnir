//go:build system

package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/runtime/appserver"
)

const (
	liveFixedCWD = "/home/niels/src"
	livePin      = "0.149.1"
)

type codexLock struct {
	Version      string `json:"version"`
	BinaryPath   string `json:"binaryPath"`
	BinarySHA256 string `json:"binarySha256"`
}

type liveProfile struct {
	Name string
	Home string
}

type appServerProcess struct {
	cmd    *exec.Cmd
	result chan error
	path   string
}

type tmuxPane struct {
	PID int
	TTY string
}

func runLiveProfileProbe(t *testing.T, profileName string) {
	t.Helper()
	root := repositoryRoot(t)
	lock := readCodexLock(t, root)
	profile := profileFor(profileName)
	assertProfilePreconditions(t, profile)

	server := startAppServer(t, lock.BinaryPath, profile)
	t.Cleanup(func() { stopAppServer(server) })

	var started appserver.ThreadRef
	if err := withProxy(t, lock.BinaryPath, profile, server.path, func(connection io.ReadWriter) error {
		var err error
		started, err = appserver.ProbeEmptyThread(connection)
		return err
	}); err != nil {
		t.Fatalf("%s empty thread/start -> unsubscribe failed: %v", profile.Name, err)
	}
	if started.ThreadID == "" || started.SessionID == "" {
		t.Fatalf("%s empty thread returned incomplete identity", profile.Name)
	}

	var listed []appserver.ThreadSummary
	if err := withProxy(t, lock.BinaryPath, profile, server.path, func(connection io.ReadWriter) error {
		var err error
		listed, err = appserver.ListThreadSummaries(connection)
		return err
	}); err != nil {
		t.Fatalf("%s bounded thread/list failed: %v", profile.Name, err)
	}
	if !containsRootThread(listed, started) {
		t.Fatalf("%s bounded thread/list did not return the started root thread", profile.Name)
	}

	var readBefore appserver.ThreadSummary
	if err := withProxy(t, lock.BinaryPath, profile, server.path, func(connection io.ReadWriter) error {
		var err error
		readBefore, err = appserver.ReadThreadSummary(connection, started)
		return err
	}); err != nil {
		t.Fatalf("%s bounded thread/read failed: %v", profile.Name, err)
	}
	assertRootSummary(t, profile.Name, readBefore, started)

	session := fmt.Sprintf("skidbladnir-live-%s-%d", profile.Name, time.Now().UnixNano())
	launchRemoteTUI(t, lock.BinaryPath, profile, server.path, session, started.ThreadID)
	t.Cleanup(func() { killOwnedTmuxSession(session) })

	pane := awaitExactPinnedTUI(t, session, lock.BinaryPath, started.ThreadID)
	var listedAfter []appserver.ThreadSummary
	if err := withProxy(t, lock.BinaryPath, profile, server.path, func(connection io.ReadWriter) error {
		var err error
		listedAfter, err = appserver.ListThreadSummaries(connection)
		return err
	}); err != nil {
		t.Fatalf("%s thread/list after remote TUI launch failed: %v", profile.Name, err)
	}
	if !containsRootThread(listedAfter, started) {
		t.Fatalf("%s remote TUI did not preserve the exact thread identity", profile.Name)
	}

	if err := killOwnedTmuxSession(session); err != nil {
		t.Fatalf("%s stop owned remote TUI session: %v", profile.Name, err)
	}
	if tmuxSessionExists(session) {
		t.Fatalf("%s owned tmux session survived stop", profile.Name)
	}

	var readAfter appserver.ThreadSummary
	if err := withProxy(t, lock.BinaryPath, profile, server.path, func(connection io.ReadWriter) error {
		var err error
		readAfter, err = appserver.ReadThreadSummary(connection, started)
		return err
	}); err != nil {
		t.Fatalf("%s thread/read after TUI stop failed: %v", profile.Name, err)
	}
	assertRootSummary(t, profile.Name, readAfter, started)
	if readAfter.Status.Type != "notLoaded" {
		t.Fatalf("%s stopped exact thread status = %q, want notLoaded resumability", profile.Name, readAfter.Status.Type)
	}

	t.Logf("profile=%s codex=%s cwd=%s thread=%s session=%s pane_pid=%d pane_tty=%s stopped_status=%s", profile.Name, lock.Version, liveFixedCWD, started.ThreadID, started.SessionID, pane.PID, pane.TTY, readAfter.Status.Type)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate live test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readCodexLock(t *testing.T, root string) codexLock {
	t.Helper()
	lockBytes, err := os.ReadFile(filepath.Join(root, "codex.lock"))
	if err != nil {
		t.Fatalf("read codex.lock: %v", err)
	}
	var lock codexLock
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatalf("decode codex.lock: %v", err)
	}
	if lock.Version != livePin || lock.BinaryPath == "" || lock.BinarySHA256 == "" {
		t.Fatalf("codex.lock pin premise false: version=%q binary_path_present=%t digest_present=%t", lock.Version, lock.BinaryPath != "", lock.BinarySHA256 != "")
	}
	binaryBytes, err := os.ReadFile(lock.BinaryPath)
	if err != nil {
		t.Fatalf("read pinned Codex binary: %v", err)
	}
	digest := sha256.Sum256(binaryBytes)
	if got := hex.EncodeToString(digest[:]); got != lock.BinarySHA256 {
		t.Fatalf("codex.lock binary digest mismatch: observed=%s expected=%s", got, lock.BinarySHA256)
	}
	return lock
}

func profileFor(name string) liveProfile {
	return map[string]liveProfile{
		"personal": {Name: "personal", Home: "/home/niels/.codex-personal"},
		"work":     {Name: "work", Home: "/home/niels/.codex-work"},
		"work2":    {Name: "work2", Home: "/home/niels/.codex-work2"},
	}[name]
}

func assertProfilePreconditions(t *testing.T, profile liveProfile) {
	t.Helper()
	if profile.Name == "" || profile.Home == "" {
		t.Fatalf("unknown live profile")
	}
	info, err := os.Stat(profile.Home)
	if err != nil || !info.IsDir() {
		t.Fatalf("profile %s home unavailable: %v", profile.Name, err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("profile %s home permissions are broader than owner-only: %o", profile.Name, info.Mode().Perm())
	}
	if _, err := os.Stat(liveFixedCWD); err != nil {
		t.Fatalf("fixed cwd unavailable: %v", err)
	}
}

func startAppServer(t *testing.T, binary string, profile liveProfile) *appServerProcess {
	t.Helper()
	path := filepath.Join(profile.Home, fmt.Sprintf(".skidbladnir-live-%s-%d.sock", profile.Name, time.Now().UnixNano()))
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("test-owned socket path already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect test-owned socket path: %v", err)
	}

	command := exec.Command(binary, "app-server", "--listen", "unix://"+path)
	command.Env = withCodexHome(profile.Home)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatalf("start pinned app-server for %s: %v", profile.Name, err)
	}
	process := &appServerProcess{cmd: command, result: make(chan error, 1), path: path}
	go func() { process.result <- command.Wait() }()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return process
		}
		select {
		case err := <-process.result:
			t.Fatalf("pinned app-server for %s exited before socket readiness: %v", profile.Name, err)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	t.Fatalf("pinned app-server for %s did not create its unique profile socket", profile.Name)
	return nil
}

func stopAppServer(process *appServerProcess) {
	if process == nil {
		return
	}
	if process.cmd.Process != nil {
		_ = process.cmd.Process.Kill() // justify-ignore-error: cleanup targets only this test-owned process.
	}
	select {
	case <-process.result:
	case <-time.After(5 * time.Second):
		_ = process.cmd.Process.Kill() // justify-ignore-error: bounded cleanup retry for this test-owned process.
	}
	_ = os.Remove(process.path) // justify-ignore-error: cleanup targets only this test-owned socket.
}

func withProxy(t *testing.T, binary string, profile liveProfile, socket string, operation func(io.ReadWriter) error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "app-server", "proxy", "--sock", socket)
	command.Env = withCodexHome(profile.Home)
	command.Stderr = io.Discard
	input, err := command.StdinPipe()
	if err != nil {
		return err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() {
		result <- operation(struct {
			io.Reader
			io.Writer
		}{Reader: output, Writer: input})
	}()
	var operationErr error
	select {
	case operationErr = <-result:
	case <-ctx.Done():
		operationErr = fmt.Errorf("proxy timed out: %w", ctx.Err())
	}
	_ = input.Close() // justify-ignore-error: proxy cleanup closes this test-owned pipe.
	waitErr := command.Wait()
	if operationErr != nil {
		return operationErr
	}
	if ctx.Err() != nil {
		return fmt.Errorf("proxy context: %w", ctx.Err())
	}
	if waitErr != nil {
		return fmt.Errorf("proxy exited: %w", waitErr)
	}
	return nil
}

func withCodexHome(home string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "CODEX_HOME=") {
			environment = append(environment, value)
		}
	}
	return append(environment, "CODEX_HOME="+home)
}

func containsRootThread(summaries []appserver.ThreadSummary, reference appserver.ThreadRef) bool {
	for _, summary := range summaries {
		if summary.ThreadID == reference.ThreadID && summary.SessionID == reference.SessionID {
			return summary.ForkedFromID == nil && summary.ParentThreadID == nil && summary.CWD == liveFixedCWD
		}
	}
	return false
}

func assertRootSummary(t *testing.T, profile string, summary appserver.ThreadSummary, reference appserver.ThreadRef) {
	t.Helper()
	if summary.ThreadID != reference.ThreadID || summary.SessionID != reference.SessionID || summary.CWD != liveFixedCWD || summary.CreatedAt <= 0 || summary.ForkedFromID != nil || summary.ParentThreadID != nil {
		t.Fatalf("%s summary failed exact content-free identity check", profile)
	}
	if summary.Status.Type == "" || len(summary.Status.ActiveFlags) > 0 && summary.Status.Type != "active" {
		t.Fatalf("%s summary has invalid status projection", profile)
	}
}

func launchRemoteTUI(t *testing.T, binary string, profile liveProfile, socket, session, threadID string) {
	t.Helper()
	command := exec.Command(
		"tmux", "new-session", "-d", "-s", session, "-c", liveFixedCWD,
		"-e", "CODEX_HOME="+profile.Home,
		binary, "resume", "--remote", "unix://"+socket,
		"--ask-for-approval", "never", "--sandbox", "danger-full-access",
		"--cd", liveFixedCWD, threadID,
	)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		t.Fatalf("launch exact remote TUI in owned tmux session: %v", err)
	}
}

func awaitExactPinnedTUI(t *testing.T, session, binary, threadID string) tmuxPane {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		pane, err := readTmuxPane(session)
		if err == nil {
			if pid := findPinnedProcess(pane.PID, binary, threadID); pid != 0 {
				if sameTTY(pid, pane.TTY) && sameCWD(pid, liveFixedCWD) {
					return tmuxPane{PID: pid, TTY: pane.TTY}
				}
			}
		}
		if !tmuxSessionExists(session) {
			t.Fatalf("owned tmux session exited before exact pinned TUI identity was observed")
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for exact pinned Codex process/thread/TTY in owned tmux session")
	return tmuxPane{}
}

func readTmuxPane(session string) (tmuxPane, error) {
	output, err := exec.Command("tmux", "list-panes", "-t", session, "-F", "#{pane_pid}\t#{pane_tty}").Output()
	if err != nil {
		return tmuxPane{}, err
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		return tmuxPane{}, errors.New("tmux pane identity missing")
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 || fields[1] == "" {
		return tmuxPane{}, errors.New("tmux pane identity malformed")
	}
	return tmuxPane{PID: pid, TTY: fields[1]}, nil
}

func findPinnedProcess(rootPID int, binary, threadID string) int {
	for _, pid := range processDescendants(rootPID) {
		executable, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		if err != nil || executable != binary {
			continue
		}
		commandLine, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
		if err != nil || !containsArgument(commandLine, threadID) || !containsArgument(commandLine, "--remote") {
			continue
		}
		return pid
	}
	return 0
}

func processDescendants(rootPID int) []int {
	seen := map[int]bool{rootPID: true}
	queue := []int{rootPID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		entries, _ := os.ReadDir("/proc") // justify-ignore-error: exited processes are absent during the observation window.
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || seen[pid] {
				continue
			}
			if processParent(pid) == parent {
				seen[pid] = true
				queue = append(queue, pid)
			}
		}
	}
	result := make([]int, 0, len(seen))
	for pid := range seen {
		result = append(result, pid)
	}
	return result
}

func processParent(pid int) int {
	bytes, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	closeParen := strings.LastIndexByte(string(bytes), ')')
	if closeParen < 0 {
		return 0
	}
	fields := strings.Fields(string(bytes)[closeParen+1:])
	if len(fields) < 2 {
		return 0
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0
	}
	return parent
}

func containsArgument(commandLine []byte, argument string) bool {
	for _, value := range strings.Split(string(commandLine), "\x00") {
		if value == argument {
			return true
		}
	}
	return false
}

func sameTTY(pid int, tty string) bool {
	fd, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", "0"))
	return err == nil && fd == tty
}

func sameCWD(pid int, cwd string) bool {
	actual, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
	return err == nil && actual == cwd
}

func tmuxSessionExists(session string) bool {
	return exec.Command("tmux", "has-session", "-t", session).Run() == nil
}

func killOwnedTmuxSession(session string) error {
	if !tmuxSessionExists(session) {
		return nil
	}
	return exec.Command("tmux", "kill-session", "-t", session).Run()
}
