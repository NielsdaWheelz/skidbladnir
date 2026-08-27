//go:build integration

package integration_test

import (
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

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
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
	kernelStartTime processinfo.StartIdentity
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
	if os.Getenv("TMUX") != "" || os.Getenv("TMUX_PANE") != "" || os.Getenv("TMUX_TMPDIR") != "" {
		fmt.Fprintln(os.Stderr, "integration tests refuse an invoking tmux client")
		os.Exit(2)
	}
	temporaryParent, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve short integration tmux parent: %v\n", err)
		os.Exit(2)
	}
	privateRoot, err := os.MkdirTemp(temporaryParent, "sk-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create private integration tmux root: %v\n", err)
		os.Exit(2)
	}
	if err := os.Chmod(privateRoot, 0o700); err != nil || os.Setenv("TMPDIR", privateRoot) != nil || os.Setenv("TMUX_TMPDIR", privateRoot) != nil {
		fmt.Fprintln(os.Stderr, "secure private integration tmux root")
		os.Exit(2)
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
	if len(arguments) < 3 || (arguments[0] != "-L" && arguments[0] != "-S") || arguments[1] == "" {
		return false
	}
	commandIndex := 2
	if arguments[commandIndex] == "-f" {
		if len(arguments) < 5 || arguments[commandIndex+1] != "/dev/null" {
			return false
		}
		commandIndex += 2
	}
	if arguments[commandIndex] == "" || strings.HasPrefix(arguments[commandIndex], "-") {
		return false
	}
	kind, target := arguments[0], arguments[1]
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

func TestRegisteredTmuxTargetRequiresOneLeadingRegisteredSelector(t *testing.T) {
	name := randomTmuxSocketName(t, "selector-contract")
	path := namedTmuxSocketPath(name)
	for _, testCase := range []struct {
		name      string
		arguments []string
		want      bool
	}{
		{name: "version", arguments: []string{"-V"}, want: true},
		{name: "registered name", arguments: []string{"-L", name, "list-sessions"}, want: true},
		{name: "registered path", arguments: []string{"-S", path, "list-sessions"}, want: true},
		{name: "missing selector", arguments: []string{"list-sessions"}},
		{name: "misplaced selector", arguments: []string{"list-sessions", "-L", name}},
		{name: "empty selector", arguments: []string{"-L", "", "list-sessions"}},
		{name: "unregistered selector", arguments: []string{"-L", name + "-other", "list-sessions"}},
		{name: "duplicate name selector", arguments: []string{"-L", name, "-L", name, "list-sessions"}},
		{name: "mixed selector", arguments: []string{"-S", path, "-L", name, "list-sessions"}},
		{name: "attached duplicate name selector", arguments: []string{"-L", name, "-L" + name, "list-sessions"}},
		{name: "attached mixed path selector", arguments: []string{"-L", name, "-S" + path, "list-sessions"}},
		{name: "clustered mixed path selector", arguments: []string{"-L", name, "-vS" + path, "list-sessions"}},
		{name: "clustered duplicate name selector", arguments: []string{"-S", path, "-qL" + name, "list-sessions"}},
		{name: "unexpected global option", arguments: []string{"-L", name, "-fother", "list-sessions"}},
		{name: "exact config prefix", arguments: []string{"-L", name, "-f", "/dev/null", "list-sessions"}, want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := registeredTmuxTarget(testCase.arguments); got != testCase.want {
				t.Fatalf("registered tmux target decision mismatch: got=%t want=%t", got, testCase.want)
			}
		})
	}
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
		t.Fatal("mint isolated tmux socket name")
	}
	name := prefix + "-" + hex.EncodeToString(random)
	if len(name) > 64 {
		t.Fatalf("isolated tmux socket name is too long: name_bytes=%d", len(name))
	}
	registerTestOwnedSocket(t, name)
	return name
}

func registerTestOwnedSocket(t *testing.T, name string) string {
	t.Helper()
	if name == "" || strings.ContainsRune(name, '/') {
		t.Fatalf("invalid test-owned tmux socket name: empty=%t contains_separator=%t", name == "", strings.ContainsRune(name, '/'))
	}
	path := filepath.Join(testSocketRoot(), fmt.Sprintf("tmux-%d", os.Getuid()), name)
	registerTestOwnedSocketPath(t, path)
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if _, found := testSocketRegistry.names[name]; found {
		t.Fatal("test-owned tmux socket name was already registered")
	}
	testSocketRegistry.names[name] = path
	return path
}

func registerTestOwnedSocketPath(t *testing.T, path string) {
	t.Helper()
	root := testSocketRoot()
	clean := filepath.Clean(path)
	if !filepath.IsAbs(path) || clean != path || !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		t.Fatalf("test-owned tmux socket escapes the private root: absolute=%t clean=%t inside=%t", filepath.IsAbs(path), clean == path, strings.HasPrefix(path, root+string(os.PathSeparator)))
	}
	testSocketRegistry.Lock()
	defer testSocketRegistry.Unlock()
	if _, found := testSocketRegistry.paths[path]; found {
		t.Fatal("test-owned tmux socket path was already registered")
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
		return errors.New("private socket root changed")
	}
	sort.Strings(paths)
	var cleanupErrors []error
	for _, path := range paths {
		if !pathStrictlyInside(privateRoot, path) {
			cleanupErrors = append(cleanupErrors, errors.New("registered socket escapes private root"))
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
		return errors.New("stat registered socket")
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("registered socket path is not a socket")
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
		if err := killVerifiedTestTmuxServer(tmuxPath, socketPath, identity); err != nil {
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
		return errors.New("registered tmux server still answers after cleanup")
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
			return testTmuxServerIdentity{}, false, errors.New("query registered tmux socket timed out")
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return testTmuxServerIdentity{}, false, nil
		}
		return testTmuxServerIdentity{}, false, errors.New("query registered tmux socket")
	}
	identity, err := parseTestTmuxServerIdentity(output)
	if err != nil {
		return testTmuxServerIdentity{}, false, err
	}
	return identity, true, nil
}

func parseTestTmuxServerIdentity(output []byte) (testTmuxServerIdentity, error) {
	pidText, tmuxStartTime, separated := strings.Cut(strings.TrimSpace(string(output)), "|")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 || !separated || tmuxStartTime == "" {
		return testTmuxServerIdentity{}, errors.New("invalid test-owned tmux server identity")
	}
	kernelStartTime := processStartIdentity(pid)
	if kernelStartTime == "" {
		return testTmuxServerIdentity{}, errors.New("test-owned tmux server has no kernel start time")
	}
	return testTmuxServerIdentity{pid: pid, kernelStartTime: kernelStartTime, tmuxStartTime: tmuxStartTime}, nil
}

func verifyRegisteredTmuxProcess(tmuxPath, socketPath string, identity testTmuxServerIdentity) error {
	observation, err := processinfo.Observe(processinfo.PID(identity.pid))
	if err != nil {
		return errors.New("observe registered tmux process")
	}
	expectedExecutable, err := os.Stat(tmuxPath)
	if err != nil {
		return errors.New("stat expected tmux executable")
	}
	observedExecutable, err := os.Stat(observation.Executable)
	if err != nil {
		return errors.New("stat registered tmux executable")
	}
	if !os.SameFile(expectedExecutable, observedExecutable) {
		return errors.New("registered socket process is not the expected tmux executable")
	}
	if !commandLineContainsExactSocket(observation.Argv, socketPath) {
		return errors.New("registered tmux command line does not name its exact socket")
	}
	if observed := processStartIdentity(identity.pid); observed != identity.kernelStartTime {
		return errors.New("registered tmux process identity changed during inspection")
	}
	return nil
}

func commandLineContainsExactSocket(commandLine []string, socketPath string) bool {
	for _, argument := range commandLine {
		if argument == socketPath || strings.Contains(argument, "("+socketPath+")") {
			return true
		}
	}
	return false
}

// killVerifiedTestTmuxServer is the single owner of identity-guarded teardown
// for a test-created tmux server. It refuses unless the exact captured PID is
// still running its captured kernel lifetime, routes the kill through a tmux
// condition that the addressed server must satisfy with its own PID and start
// time, and then waits for that exact process to leave. Every teardown path
// calls it so the guard cannot drift between callers.
func killVerifiedTestTmuxServer(tmuxPath, socketPath string, identity testTmuxServerIdentity) error {
	if observed := processStartIdentity(identity.pid); observed != identity.kernelStartTime {
		return errors.New("refusing tmux cleanup after process identity changed")
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
			return errors.New("kill verified test-owned tmux server timed out")
		}
		return fmt.Errorf("kill verified test-owned tmux server: output_bytes=%d", len(output))
	}
	if strings.TrimSpace(string(output)) != "" {
		return errors.New("refusing tmux cleanup after routed identity mismatch")
	}

	deadline := time.Now().Add(testTmuxCleanupTimeout)
	for processStartIdentity(identity.pid) == identity.kernelStartTime {
		if time.Now().After(deadline) {
			return errors.New("verified test-owned tmux server survived cleanup")
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	return nil
}

func unlinkRegisteredStaleSocket(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("stat stale registered socket")
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("refusing to unlink non-socket registered path")
	}
	if err := os.Remove(socketPath); err != nil {
		return errors.New("unlink stale registered socket")
	}
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		return errors.New("registered socket survived exact unlink")
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
		return errors.New("stat private directory")
	}
	if !info.IsDir() {
		return errors.New("refusing to remove non-directory private path")
	}
	if err := os.Remove(path); err != nil {
		return errors.New("remove private directory")
	}
	return nil
}

func captureTestTmuxServer(t *testing.T, tmuxPath, socketPath string) testTmuxServerIdentity {
	t.Helper()
	output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath, "display-message", "-p", "#{pid}|#{start_time}").CombinedOutput()
	if err != nil {
		t.Fatalf("capture test-owned tmux server identity: output_bytes=%d", len(output))
	}
	identity, err := parseTestTmuxServerIdentity(output)
	if err != nil {
		t.Fatal("parse test-owned tmux server identity")
	}
	return identity
}

func stopTestTmuxServer(t *testing.T, tmuxPath, socketPath string, identity testTmuxServerIdentity) {
	t.Helper()
	if err := killVerifiedTestTmuxServer(tmuxPath, socketPath, identity); err != nil {
		t.Errorf("stop test-owned tmux server: %v", err)
	}
}

func processStartIdentity(pid int) processinfo.StartIdentity {
	observation, err := processinfo.Observe(processinfo.PID(pid))
	if err != nil {
		return ""
	}
	return observation.StartIdentity
}
