//go:build live

package live

import (
	"bytes"
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
	stderr *bytes.Buffer
	waited bool
}

type tmuxPane struct {
	PID       int
	StartTime uint64
	TTY       string
}

func runLiveProfileProbe(t *testing.T, profileName string) {
	t.Helper()
	root := repositoryRoot(t)
	lock := readCodexLock(t, root)
	profile := profileFor(profileName)
	assertProfilePreconditions(t, profile)

	server := startAppServer(t, lock.BinaryPath, profile)
	var listed []appserver.ThreadSummary
	if err := withAppServer(server.path, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		listed, err = appserver.ListThreadSummaries(ctx, connection, profile.Home, "")
		return err
	}); err != nil {
		t.Fatalf("%s bounded unfiltered thread/list failed: %v", profile.Name, err)
	}
	var started appserver.ThreadRef
	if err := withAppServer(server.path, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		started, err = appserver.ProbeEmptyThread(ctx, connection, profile.Home)
		return err
	}); err != nil {
		t.Fatalf("%s empty thread/start -> unsubscribe failed: %v", profile.Name, err)
	}
	if started.ThreadID == "" || started.SessionID == "" {
		t.Fatalf("%s empty thread returned incomplete identity", profile.Name)
	}

	var readBefore appserver.ThreadSummary
	if err := withAppServer(server.path, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		readBefore, err = appserver.ReadThreadSummary(ctx, connection, profile.Home, started)
		return err
	}); err != nil {
		t.Fatalf("%s bounded thread/read failed: %v", profile.Name, err)
	}
	assertRootSummary(t, profile.Name, readBefore, started)

	session := fmt.Sprintf("skidbladnir-live-%s-%d", profile.Name, time.Now().UnixNano())
	launchRemoteTUI(t, lock.BinaryPath, profile, server.path, session, started.ThreadID)
	t.Cleanup(func() {
		if err := killOwnedTmuxSession(session); err != nil {
			t.Errorf("clean up test-owned tmux session %s: %v", session, err)
		}
		if tmuxSessionExists(session) {
			t.Errorf("test-owned tmux session survived cleanup: %s", session)
		}
	})

	pane := awaitExactPinnedTUI(t, session, lock.BinaryPath, started.ThreadID)
	var listedInCWD []appserver.ThreadSummary
	if err := withAppServer(server.path, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		listedInCWD, err = appserver.ListThreadSummaries(ctx, connection, profile.Home, liveFixedCWD)
		return err
	}); err != nil {
		t.Fatalf("%s bounded cwd thread/list failed: %v", profile.Name, err)
	}
	if containsRootThread(listedInCWD, started) {
		t.Fatalf("%s unmaterialized empty thread unexpectedly appeared in thread/list", profile.Name)
	}

	if err := killOwnedTmuxSession(session); err != nil {
		t.Fatalf("%s stop owned remote TUI session: %v", profile.Name, err)
	}
	if tmuxSessionExists(session) {
		t.Fatalf("%s owned tmux session survived stop", profile.Name)
	}
	awaitExactProcessGone(t, pane)

	stoppedAt := time.Now()
	var readAfter appserver.ThreadSummary
	if err := withAppServer(server.path, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		readAfter, err = appserver.ReadThreadSummary(ctx, connection, profile.Home, started)
		return err
	}); err != nil {
		t.Fatalf("%s thread/read after TUI stop failed: %v", profile.Name, err)
	}
	assertRootSummary(t, profile.Name, readAfter, started)
	if readAfter.Status.Type != "idle" {
		t.Fatalf("%s stopped exact thread status = %q, want idle during the pin's 30-minute unload delay", profile.Name, readAfter.Status.Type)
	}
	stopReadLatency := time.Since(stoppedAt)

	t.Logf("profile=%s codex=%s cwd=%s thread=%s session=%s pane_pid=%d pane_start=%d pane_tty=%s immediate_stopped_status=%s stop_read_ms=%d materialized_listed=%d", profile.Name, lock.Version, liveFixedCWD, started.ThreadID, started.SessionID, pane.PID, pane.StartTime, pane.TTY, readAfter.Status.Type, stopReadLatency.Milliseconds(), len(listed))
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

	command := exec.Command(binary, "app-server", "--strict-config", "--listen", "unix://"+path)
	command.Env = withCodexHome(profile.Home)
	command.Stdout = io.Discard
	stderr := &bytes.Buffer{}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start pinned app-server for %s: %v", profile.Name, err)
	}
	process := &appServerProcess{cmd: command, result: make(chan error, 1), path: path, stderr: stderr}
	go func() { process.result <- command.Wait() }()
	t.Cleanup(func() { stopAppServer(t, process) })

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return process
		}
		select {
		case err := <-process.result:
			process.waited = true
			t.Fatalf("pinned app-server for %s exited before socket readiness: %v: %s", profile.Name, err, boundedAppServerError(process.stderr.String()))
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	t.Fatalf("pinned app-server for %s did not create its unique profile socket", profile.Name)
	return nil
}

func boundedAppServerError(message string) string {
	const limit = 2048
	message = strings.TrimSpace(message)
	if len(message) > limit {
		return message[:limit] + "..."
	}
	return message
}

func stopAppServer(t *testing.T, process *appServerProcess) {
	t.Helper()
	if process == nil {
		return
	}
	if !process.waited && process.cmd.Process != nil {
		if err := process.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("stop test-owned App Server: %v", err)
		}
	}
	if !process.waited {
		select {
		case <-process.result:
			process.waited = true
		case <-time.After(5 * time.Second):
			t.Errorf("test-owned App Server did not exit within cleanup deadline")
		}
	}
	if err := os.Remove(process.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("remove test-owned App Server socket: %v", err)
	}
	if _, err := os.Lstat(process.path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("test-owned App Server socket survived cleanup: %v", err)
	}
}

func withAppServer(socket string, operation func(context.Context, appserver.Connection) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	connection, err := appserver.DialUnix(ctx, socket)
	if err != nil {
		return err
	}
	defer connection.Close()
	return operation(ctx, connection)
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
		"--strict-config",
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
				if startTime := processStartTime(pid); startTime != 0 && sameTTY(pid, pane.TTY) && sameCWD(pid, liveFixedCWD) {
					return tmuxPane{PID: pid, StartTime: startTime, TTY: pane.TTY}
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

func awaitExactProcessGone(t *testing.T, pane tmuxPane) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if observed := processStartTime(pane.PID); observed == 0 || observed != pane.StartTime {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("exact pinned TUI process survived tmux stop: pid=%d start=%d", pane.PID, pane.StartTime)
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

func processStartTime(pid int) uint64 {
	bytes, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	closeParen := strings.LastIndexByte(string(bytes), ')')
	if closeParen < 0 {
		return 0
	}
	fields := strings.Fields(string(bytes)[closeParen+1:])
	if len(fields) <= 19 {
		return 0
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0
	}
	return startTime
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
