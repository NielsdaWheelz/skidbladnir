//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testTmuxCleanupTimeout      = 3 * time.Second
	isolatedTmuxCLICapabilityV1 = "isolated-cli-v1"
)

var isolatedTmuxCLICapability = flag.String(
	"skidbladnir-isolated-tmux-capability",
	"",
	"second explicit capability required for isolated tmux mutation",
)

var testSocketRegistry = struct {
	sync.Mutex
	root  string
	names map[string]string
	paths map[string]struct{}
}{names: make(map[string]string), paths: make(map[string]struct{})}

type testTmuxServerIdentity struct {
	pid             int
	kernelStartTime uint64
	tmuxStartTime   string
}

func TestMain(m *testing.M) {
	flag.Parse()
	if os.Getenv("SKIDBLADNIR_ALLOW_ISOLATED_TMUX_TESTS") != "isolated-v1" {
		fmt.Fprintln(os.Stderr, "integration tests require explicit isolated tmux approval")
		os.Exit(2)
	}
	if *isolatedTmuxCLICapability != isolatedTmuxCLICapabilityV1 {
		fmt.Fprintln(os.Stderr, "integration tests require the explicit CLI tmux capability")
		os.Exit(2)
	}
	if os.Getenv("TMUX") != "" || os.Getenv("TMUX_PANE") != "" {
		fmt.Fprintln(os.Stderr, "integration tests refuse an invoking tmux client")
		os.Exit(2)
	}
	privateRoot := ""
	if os.Getenv(defaultSocketHelperVariable) == "1" {
		privateRoot = os.Getenv("TMUX_TMPDIR")
		wantRoot := filepath.Join(os.Getenv(defaultSocketRootVariable), "tmux")
		if !filepath.IsAbs(privateRoot) || filepath.Clean(privateRoot) != privateRoot || privateRoot != wantRoot {
			fmt.Fprintln(os.Stderr, "default-socket helper has no exact private tmux root")
			os.Exit(2)
		}
	} else {
		var err error
		privateRoot, err = os.MkdirTemp("", "skidbladnir-integration-tmux-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create private integration tmux root: %v\n", err)
			os.Exit(2)
		}
		if err := os.Chmod(privateRoot, 0o700); err != nil || os.Setenv("TMUX_TMPDIR", privateRoot) != nil {
			fmt.Fprintln(os.Stderr, "secure private integration tmux root")
			os.Exit(2)
		}
	}
	testSocketRegistry.root = privateRoot
	status := m.Run()
	if err := cleanupRegisteredTmuxSockets(tmuxPath, privateRoot); err != nil {
		fmt.Fprintf(os.Stderr, "clean registered integration tmux sockets: %v\n", err)
		status = 1
	}
	userRoot := filepath.Join(privateRoot, fmt.Sprintf("tmux-%d", os.Getuid()))
	if err := removeExactPrivateDirectory(userRoot); err != nil {
		fmt.Fprintf(os.Stderr, "remove private integration tmux user root: %v\n", err)
		status = 1
	}
	if err := removeExactPrivateDirectory(privateRoot); err != nil {
		fmt.Fprintf(os.Stderr, "remove private integration tmux root: %v\n", err)
		status = 1
	}
	os.Exit(status)
}

func isolatedTmuxCommand(path string, args ...string) *exec.Cmd {
	return isolatedTmuxCommandContext(context.Background(), path, args...)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func isolatedTmuxCommandContext(ctx context.Context, path string, args ...string) *exec.Cmd {
	if !registeredTmuxTarget(args) {
		panic("integration tmux command has no registered test-owned -L or -S target")
	}
	command := exec.CommandContext(ctx, path, args...)
	command.Env = append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "TMUX_TMPDIR"), "TMUX_TMPDIR="+testSocketRoot())
	return command
}

func registeredTmuxTarget(arguments []string) bool {
	if len(arguments) == 1 && arguments[0] == "-V" {
		return true
	}
	kind, target := "", ""
	for index, argument := range arguments {
		if (argument == "-L" || argument == "-S") && index+1 < len(arguments) && arguments[index+1] != "" {
			if kind != "" {
				return false
			}
			kind, target = argument, arguments[index+1]
		}
	}
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if kind == "-L" {
		_, found := testSocketRegistry.names[target]
		return found
	}
	if kind == "-S" {
		_, found := testSocketRegistry.paths[target]
		return found
	}
	return false
}

func namedTmuxSocketPath(name string) string {
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	path, found := testSocketRegistry.names[name]
	if !found {
		panic("integration tmux socket name is not registered")
	}
	return path
}

func randomTmuxSocketName(t *testing.T, prefix string) string {
	t.Helper()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("mint isolated tmux socket name: %v", err)
	}
	name := prefix + "-" + hex.EncodeToString(random)
	if len(name) > 64 {
		t.Fatalf("isolated tmux socket name is too long: %q", name)
	}
	registerTestOwnedSocket(t, name)
	return name
}

func registerTestOwnedSocket(t *testing.T, name string) string {
	t.Helper()
	if name == "" || strings.ContainsRune(name, '/') {
		t.Fatalf("invalid test-owned tmux socket name: %q", name)
	}
	path := filepath.Join(testSocketRoot(), fmt.Sprintf("tmux-%d", os.Getuid()), name)
	registerTestOwnedSocketPath(t, path)
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if _, found := testSocketRegistry.names[name]; found {
		t.Fatalf("test-owned tmux socket name was already registered: %q", name)
	}
	testSocketRegistry.names[name] = path
	return path
}

func registerTestOwnedSocketPath(t *testing.T, path string) {
	t.Helper()
	root := testSocketRoot()
	clean := filepath.Clean(path)
	if !filepath.IsAbs(path) || clean != path || !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		t.Fatalf("test-owned tmux socket escapes the private root: %q", path)
	}
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if _, found := testSocketRegistry.paths[path]; found {
		t.Fatalf("test-owned tmux socket path was already registered: %q", path)
	}
	testSocketRegistry.paths[path] = struct{}{}
}

func testSocketRoot() string {
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if testSocketRegistry.root == "" {
		panic("integration tmux socket root is not initialized")
	}
	return testSocketRegistry.root
}

func cleanupRegisteredTmuxSockets(tmuxPath, privateRoot string) error {
	testSocketRegistry.Lock()
	paths := make([]string, 0, len(testSocketRegistry.paths))
	for path := range testSocketRegistry.paths {
		paths = append(paths, path)
	}
	registeredRoot := testSocketRegistry.root
	testSocketRegistry.Unlock()
	if registeredRoot != privateRoot {
		return fmt.Errorf("private socket root changed: registered=%q cleanup=%q", registeredRoot, privateRoot)
	}
	sort.Strings(paths)
	var cleanupErrors []error
	for _, path := range paths {
		if !pathStrictlyInside(privateRoot, path) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("registered socket escapes private root: %q", path))
			continue
		}
		if err := cleanupRegisteredTmuxSocket(tmuxPath, path); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

func cleanupRegisteredTmuxSocket(tmuxPath, socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat registered socket %s: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("registered socket path is not a socket: %s", socketPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTmuxCleanupTimeout)
	identity, live, err := queryRegisteredTmuxServer(ctx, tmuxPath, socketPath)
	cancel()
	if err != nil {
		return err
	}
	if live {
		if err := verifyRegisteredTmuxProcess(tmuxPath, socketPath, identity); err != nil {
			return err
		}
		if err := killRegisteredTmuxServer(tmuxPath, socketPath, identity); err != nil {
			return err
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), testTmuxCleanupTimeout)
	_, stillLive, err := queryRegisteredTmuxServer(ctx, tmuxPath, socketPath)
	cancel()
	if err != nil {
		return err
	}
	if stillLive {
		return fmt.Errorf("registered tmux server still answers after cleanup: %s", socketPath)
	}
	return unlinkRegisteredStaleSocket(socketPath)
}

func queryRegisteredTmuxServer(
	ctx context.Context,
	tmuxPath, socketPath string,
) (testTmuxServerIdentity, bool, error) {
	output, err := isolatedTmuxCommandContext(ctx, tmuxPath, "-S", socketPath,
		"display-message", "-p", "#{pid}|#{start_time}").CombinedOutput()
	contextErr := ctx.Err()
	if err != nil {
		if contextErr != nil {
			return testTmuxServerIdentity{}, false, fmt.Errorf("query registered tmux socket %s: %w", socketPath, contextErr)
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return testTmuxServerIdentity{}, false, nil
		}
		return testTmuxServerIdentity{}, false, fmt.Errorf("query registered tmux socket %s: %w", socketPath, err)
	}
	identity, err := parseTestTmuxServerIdentity(socketPath, output)
	if err != nil {
		return testTmuxServerIdentity{}, false, err
	}
	return identity, true, nil
}

func parseTestTmuxServerIdentity(socketPath string, output []byte) (testTmuxServerIdentity, error) {
	pidText, tmuxStartTime, separated := strings.Cut(strings.TrimSpace(string(output)), "|")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 || !separated || tmuxStartTime == "" {
		return testTmuxServerIdentity{}, fmt.Errorf("invalid test-owned tmux server identity: socket=%s output=%q", socketPath, output)
	}
	kernelStartTime := linuxProcessStartTime(pid)
	if kernelStartTime == 0 {
		return testTmuxServerIdentity{}, fmt.Errorf("test-owned tmux server has no kernel start time: socket=%s pid=%d", socketPath, pid)
	}
	return testTmuxServerIdentity{pid: pid, kernelStartTime: kernelStartTime, tmuxStartTime: tmuxStartTime}, nil
}

func verifyRegisteredTmuxProcess(tmuxPath, socketPath string, identity testTmuxServerIdentity) error {
	processRoot := filepath.Join("/proc", strconv.Itoa(identity.pid))
	executable, err := os.Readlink(filepath.Join(processRoot, "exe"))
	if err != nil {
		return fmt.Errorf("read registered tmux executable: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	commandLine, err := os.ReadFile(filepath.Join(processRoot, "cmdline"))
	if err != nil {
		return fmt.Errorf("read registered tmux command line: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	expectedExecutable, err := os.Stat(tmuxPath)
	if err != nil {
		return fmt.Errorf("stat expected tmux executable %s: %w", tmuxPath, err)
	}
	observedExecutable, err := os.Stat(filepath.Join(processRoot, "exe"))
	if err != nil {
		return fmt.Errorf("stat registered tmux executable: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	if !os.SameFile(expectedExecutable, observedExecutable) {
		return fmt.Errorf("registered socket process is not the expected tmux executable: socket=%s pid=%d exe=%q", socketPath, identity.pid, executable)
	}
	if !commandLineContainsExactSocket(commandLine, socketPath) {
		return fmt.Errorf("registered tmux command line does not name its exact socket: socket=%s pid=%d", socketPath, identity.pid)
	}
	if observed := linuxProcessStartTime(identity.pid); observed != identity.kernelStartTime {
		return fmt.Errorf("registered tmux process identity changed during inspection: socket=%s pid=%d captured=%d observed=%d",
			socketPath, identity.pid, identity.kernelStartTime, observed)
	}
	return nil
}

func commandLineContainsExactSocket(commandLine []byte, socketPath string) bool {
	for _, argument := range bytes.Split(commandLine, []byte{0}) {
		text := string(argument)
		if text == socketPath || strings.Contains(text, "("+socketPath+")") {
			return true
		}
	}
	return false
}

func killRegisteredTmuxServer(tmuxPath, socketPath string, identity testTmuxServerIdentity) error {
	if observed := linuxProcessStartTime(identity.pid); observed != identity.kernelStartTime {
		return fmt.Errorf("refusing post-run tmux cleanup after process identity changed: socket=%s pid=%d captured=%d observed=%d",
			socketPath, identity.pid, identity.kernelStartTime, observed)
	}
	condition := "#{&&:#{==:#{pid}," + strconv.Itoa(identity.pid) + "},#{==:#{start_time}," + identity.tmuxStartTime + "}}"
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	ctx, cancel := context.WithTimeout(context.Background(), testTmuxCleanupTimeout)
	output, err := isolatedTmuxCommandContext(ctx, tmuxPath, "-S", socketPath,
		"if-shell", "-F", condition,
		"kill-server",
		"display-message -p -l '"+mismatchMarker+"'").CombinedOutput()
	contextErr := ctx.Err()
	cancel()
	if err != nil {
		if contextErr != nil {
			return fmt.Errorf("kill verified registered tmux server: socket=%s: %w", socketPath, contextErr)
		}
		return fmt.Errorf("kill verified registered tmux server: socket=%s output=%q error=%w", socketPath, output, err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("refusing post-run tmux cleanup after routed identity mismatch: socket=%s output=%q", socketPath, output)
	}

	deadline := time.Now().Add(testTmuxCleanupTimeout)
	for linuxProcessStartTime(identity.pid) == identity.kernelStartTime {
		if time.Now().After(deadline) {
			return fmt.Errorf("verified registered tmux server survived cleanup: socket=%s pid=%d", socketPath, identity.pid)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

func unlinkRegisteredStaleSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat stale registered socket %s: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to unlink non-socket registered path: %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("unlink stale registered socket %s: %w", socketPath, err)
	}
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("registered socket survived exact unlink: %s: %w", socketPath, err)
	}
	return nil
}

func pathStrictlyInside(root, path string) bool {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func removeExactPrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to remove non-directory private path: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

func captureTestTmuxServer(t *testing.T, tmuxPath, socketPath string) testTmuxServerIdentity {
	t.Helper()
	output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath, "display-message", "-p", "#{pid}|#{start_time}").CombinedOutput()
	if err != nil {
		t.Fatalf("capture test-owned tmux server identity: socket=%s output=%q error=%v", socketPath, output, err)
	}
	identity, err := parseTestTmuxServerIdentity(socketPath, output)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func stopTestTmuxServer(t *testing.T, tmuxPath, socketPath string, identity testTmuxServerIdentity) {
	t.Helper()
	if observed := linuxProcessStartTime(identity.pid); observed != identity.kernelStartTime {
		t.Errorf("refusing tmux cleanup after process identity changed: socket=%s pid=%d captured=%d observed=%d", socketPath, identity.pid, identity.kernelStartTime, observed)
		return
	}
	epoch := fmt.Sprintf("%d:%s", identity.pid, identity.tmuxStartTime)
	condition := "#{==:#{pid}:#{start_time}," + epoch + "}"
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath,
		"if-shell", "-F", condition,
		"kill-server",
		"display-message -p -l '"+mismatchMarker+"'").CombinedOutput()
	if err != nil {
		t.Errorf("kill verified test-owned tmux server: socket=%s output=%q error=%v", socketPath, output, err)
		return
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("refusing tmux cleanup after routed identity mismatch: socket=%s output=%q", socketPath, output)
		return
	}
	deadline := time.Now().Add(testTmuxCleanupTimeout)
	for linuxProcessStartTime(identity.pid) == identity.kernelStartTime {
		if time.Now().After(deadline) {
			t.Errorf("verified test-owned tmux server survived cleanup: socket=%s pid=%d", socketPath, identity.pid)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func linuxProcessStartTime(pid int) uint64 {
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
