//go:build live

package live

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	directTUIVersion = "0.149.1"
	directTUICWD     = "/home/niels/src"
	directTUITimeout = 10 * time.Second
)

var directTUIRolloutName = regexp.MustCompile(`^rollout-[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:_[0-9a-f-]{36})?\.jsonl$`)

type directTUILock struct {
	Version      string `json:"version"`
	BinaryPath   string `json:"binaryPath"`
	BinarySHA256 string `json:"binarySha256"`
}

type directTUIProfile struct {
	name string
	home string
}

type directTUIProcess struct {
	pid       int
	startTime uint64
	tty       string
}

func TestDirectTUIAcrossProfiles(t *testing.T) {
	requireExactTmux(t)
	lock := readDirectTUILock(t)
	profiles := []directTUIProfile{
		{name: "personal", home: "/home/niels/.codex-personal"},
		{name: "work", home: "/home/niels/.codex-work"},
		{name: "work2", home: "/home/niels/.codex-work2"},
	}
	for _, profile := range profiles {
		profile := profile
		t.Run(profile.name, func(t *testing.T) {
			assertDirectTUIProfile(t, profile)
			threadID := findDirectTUIThreadID(t, profile.home)
			assertDirectTUILaunch(t, lock.BinaryPath, profile, nil)
			assertDirectTUILaunch(t, lock.BinaryPath, profile, []string{"resume", threadID})
		})
	}
}

func requireExactTmux(t *testing.T) {
	t.Helper()
	version, err := exec.Command("tmux", "-V").Output()
	if err != nil {
		t.Fatalf("read tmux version: %v", err)
	}
	if got := strings.TrimSpace(string(version)); got != "tmux 3.4" {
		t.Fatalf("tmux version = %q, want exact tmux 3.4", got)
	}
}

func readDirectTUILock(t *testing.T) directTUILock {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate direct-TUI live test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	contents, err := os.ReadFile(filepath.Join(root, "codex.lock"))
	if err != nil {
		t.Fatalf("read codex.lock: %v", err)
	}
	var lock directTUILock
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatalf("decode codex.lock: %v", err)
	}
	if lock.Version != directTUIVersion || !filepath.IsAbs(lock.BinaryPath) || len(lock.BinarySHA256) != 64 {
		t.Fatal("codex.lock does not name the exact direct-TUI pin")
	}
	binary, err := os.ReadFile(lock.BinaryPath)
	if err != nil {
		t.Fatalf("read pinned Codex binary: %v", err)
	}
	digest := sha256.Sum256(binary)
	if hex.EncodeToString(digest[:]) != lock.BinarySHA256 {
		t.Fatal("pinned Codex binary digest differs from codex.lock")
	}
	return lock
}

func assertDirectTUIProfile(t *testing.T, profile directTUIProfile) {
	t.Helper()
	info, err := os.Stat(profile.home)
	if err != nil || !info.IsDir() {
		t.Fatalf("profile %s home is unavailable: %v", profile.name, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("profile %s home permissions = %o, want owner-only", profile.name, info.Mode().Perm())
	}
	cwd, err := os.Stat(directTUICWD)
	if err != nil || !cwd.IsDir() {
		t.Fatalf("direct-TUI cwd is unavailable: %v", err)
	}
}

func findDirectTUIThreadID(t *testing.T, home string) string {
	t.Helper()
	var selected string
	var selectedModTime time.Time
	err := filepath.WalkDir(filepath.Join(home, "sessions"), func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		match := directTUIRolloutName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if selected == "" || info.ModTime().Before(selectedModTime) {
			selected = match[1]
			selectedModTime = info.ModTime()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inventory profile rollout basenames: %v", err)
	}
	if selected == "" {
		t.Fatal("profile has no canonical root UUID available for resume proof")
	}
	return selected
}

func assertDirectTUILaunch(t *testing.T, binary string, profile directTUIProfile, prefix []string) {
	t.Helper()
	socket := fmt.Sprintf("skidbladnir_direct_%d_%d", os.Getpid(), time.Now().UnixNano())
	runtimeID := fmt.Sprintf("%08x-1111-4111-8111-%012x", uint32(time.Now().UnixNano()), uint64(os.Getpid()))
	arguments := []string{"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "direct", "-x", "100", "-y", "30", "-c", directTUICWD, "-e", "CODEX_HOME=" + profile.home, "-e", "SKIDBLADNIR_RUNTIME_ID=" + runtimeID, binary}
	if len(prefix) == 0 {
		arguments = append(arguments, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD)
	} else {
		arguments = append(arguments, prefix[0], "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD, prefix[1])
	}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start %s direct TUI: %v: %s", profile.name, err, boundedDirectTUIError(output))
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: cleanup accepts an already-exited test-owned tmux server.
	})

	process := awaitDirectTUIProcess(t, socket)
	assertDirectTUIProcess(t, binary, profile, runtimeID, process, prefix)
	if output, err := exec.Command("tmux", "-L", socket, "kill-server").CombinedOutput(); err != nil {
		t.Fatalf("stop test-owned direct TUI: %v: %s", err, boundedDirectTUIError(output))
	}
	waitForDirectTUIProcessGone(t, process)
}

func awaitDirectTUIProcess(t *testing.T, socket string) directTUIProcess {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		output, err := exec.Command("tmux", "-L", socket, "list-panes", "-t", "direct", "-F", "#{pane_pid}|#{pane_tty}|#{pane_dead}|#{pane_current_command}").Output()
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
	t.Fatal("test-owned direct TUI did not reach a live foreground process")
	return directTUIProcess{}
}

func assertDirectTUIProcess(t *testing.T, binary string, profile directTUIProfile, runtimeID string, process directTUIProcess, prefix []string) {
	t.Helper()
	executable, err := filepath.EvalSymlinks(filepath.Join("/proc", strconv.Itoa(process.pid), "exe"))
	if err != nil {
		t.Fatalf("resolve direct TUI executable: %v", err)
	}
	pinned, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatalf("resolve pinned Codex executable: %v", err)
	}
	if executable != pinned {
		t.Fatalf("foreground executable = %q, want exact pin", executable)
	}
	if process.startTime != processStartTime(process.pid) {
		t.Fatal("direct TUI process identity changed during verification")
	}
	cwd, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(process.pid), "cwd"))
	if err != nil || cwd != directTUICWD {
		t.Fatalf("direct TUI cwd = %q (error=%v), want %q", cwd, err, directTUICWD)
	}
	tty, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(process.pid), "fd", "0"))
	if err != nil || tty != process.tty {
		t.Fatalf("direct TUI TTY = %q (error=%v), want registered pane TTY", tty, err)
	}
	environment := readNULTokens(t, filepath.Join("/proc", strconv.Itoa(process.pid), "environ"))
	assertDirectTUIEnvironment(t, environment, "CODEX_HOME", profile.home)
	assertDirectTUIEnvironment(t, environment, "SKIDBLADNIR_RUNTIME_ID", runtimeID)
	argv := readNULTokens(t, filepath.Join("/proc", strconv.Itoa(process.pid), "cmdline"))
	joined := strings.Join(argv, "\x00")
	for _, required := range []string{"--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD} {
		if !containsDirectTUIToken(argv, required) {
			t.Fatalf("direct TUI argv omits required token %q", required)
		}
	}
	for _, forbidden := range []string{"app-server", "agents", "--remote", "--dangerously-bypass-hook-trust"} {
		if containsDirectTUIToken(argv, forbidden) || strings.Contains(joined, forbidden+"=") {
			t.Fatalf("direct TUI argv contains forbidden transport or trust token %q", forbidden)
		}
	}
	if len(prefix) > 0 && (!containsDirectTUIToken(argv, "resume") || !containsDirectTUIToken(argv, prefix[1])) {
		t.Fatal("direct resume TUI argv lost its exact root UUID")
	}
	assertNoDirectTUIAppServerChild(t, process.pid)
}

func readNULTokens(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read process boundary: %v", err)
	}
	trimmed := strings.TrimRight(string(contents), "\x00")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\x00")
}

func assertDirectTUIEnvironment(t *testing.T, environment []string, name, want string) {
	t.Helper()
	for _, value := range environment {
		if value == name+"="+want {
			return
		}
	}
	t.Fatalf("direct TUI environment omits exact %s", name)
}

func containsDirectTUIToken(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertNoDirectTUIAppServerChild(t *testing.T, rootPID int) {
	t.Helper()
	children := map[int][]int{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("read /proc: %v", err)
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fields, err := procStatFieldsForLive(pid)
		if err != nil || len(fields) < 2 {
			continue
		}
		parent, err := strconv.Atoi(fields[1])
		if err == nil {
			children[parent] = append(children[parent], pid)
		}
	}
	queue := append([]int(nil), children[rootPID]...)
	for len(queue) > 0 {
		pid := queue[0]
		queue = append(queue[1:], children[pid]...)
		contents, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(contents), "\x00"), "\x00")
		if containsDirectTUIToken(argv, "app-server") || containsDirectTUIToken(argv, "agents") || containsDirectTUIToken(argv, "--remote") {
			t.Fatal("direct TUI spawned a forbidden shared/external App Server child")
		}
	}
}

func waitForDirectTUIProcessGone(t *testing.T, process directTUIProcess) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if !directTUIProcessAlive(process) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("test-owned direct TUI process survived tmux teardown")
}

func directTUIProcessAlive(process directTUIProcess) bool {
	fields, err := procStatFieldsForLive(process.pid)
	if err != nil || len(fields) < 20 || fields[0] == "Z" {
		return false
	}
	value, err := strconv.ParseUint(fields[19], 10, 64)
	return err == nil && value == process.startTime
}

func processStartTime(pid int) uint64 {
	fields, err := procStatFieldsForLive(pid)
	if err != nil || len(fields) < 20 {
		return 0
	}
	value, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func procStatFieldsForLive(pid int) ([]string, error) {
	contents, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, err
	}
	end := strings.LastIndexByte(string(contents), ')')
	if end < 0 || end+2 >= len(contents) {
		return nil, errors.New("invalid proc stat")
	}
	return strings.Fields(string(contents[end+2:])), nil
}

func boundedDirectTUIError(contents []byte) string {
	message := strings.TrimSpace(string(contents))
	if len(message) > 2048 {
		return message[:2048] + "..."
	}
	return message
}
