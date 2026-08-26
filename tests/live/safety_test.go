//go:build live

package live

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

var isolatedTmuxCLICapability = flag.String(
	"skidbladnir-isolated-tmux-capability",
	"",
	"second explicit capability required for isolated tmux mutation",
)

var liveSocketRegistry = struct {
	sync.Mutex
	root  string
	paths map[string]struct{}
}{paths: make(map[string]struct{})}

type registeredLiveTmuxIdentity struct {
	pid             int
	kernelStartTime uint64
	tmuxStartTime   string
}

func TestMain(testingMain *testing.M) {
	flag.Parse()
	if os.Getenv("SKIDBLADNIR_ALLOW_ISOLATED_TMUX_TESTS") != "isolated-v1" {
		fmt.Fprintln(os.Stderr, "live tmux proofs require explicit isolated tmux approval")
		os.Exit(2)
	}
	if *isolatedTmuxCLICapability != "isolated-cli-v1" {
		fmt.Fprintln(os.Stderr, "live tmux proofs require the explicit CLI tmux capability")
		os.Exit(2)
	}
	if os.Getenv("TMUX") != "" || os.Getenv("TMUX_PANE") != "" {
		fmt.Fprintln(os.Stderr, "live tmux proofs refuse an invoking tmux client")
		os.Exit(2)
	}
	privateRoot, err := os.MkdirTemp("", "skidbladnir-live-tmux-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create private live tmux root: %v\n", err)
		os.Exit(2)
	}
	if err := os.Chmod(privateRoot, 0o700); err != nil || os.Setenv("TMUX_TMPDIR", privateRoot) != nil {
		fmt.Fprintln(os.Stderr, "secure private live tmux root")
		os.Exit(2)
	}
	liveSocketRegistry.Lock()
	liveSocketRegistry.root = privateRoot
	liveSocketRegistry.Unlock()
	status := testingMain.Run()
	if err := cleanupRegisteredLiveTmuxSockets(liveTmuxPath, privateRoot); err != nil {
		fmt.Fprintf(os.Stderr, "clean registered live tmux sockets: %v\n", err)
		status = 1
	}
	if err := removeExactLiveDirectory(privateRoot); err != nil {
		fmt.Fprintf(os.Stderr, "remove private live tmux root: %v\n", err)
		status = 1
	}
	os.Exit(status)
}

func registerLiveTmuxSocket(t *testing.T) string {
	t.Helper()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("mint live tmux socket name: %v", err)
	}
	liveSocketRegistry.Lock()
	defer liveSocketRegistry.Unlock()
	if liveSocketRegistry.root == "" {
		t.Fatal("live tmux socket root is not initialized")
	}
	path := filepath.Join(liveSocketRegistry.root, "socket-"+hex.EncodeToString(random))
	if !pathStrictlyInsideLiveRoot(liveSocketRegistry.root, path) {
		t.Fatalf("live tmux socket escapes the private root: %q", path)
	}
	if _, exists := liveSocketRegistry.paths[path]; exists {
		t.Fatalf("live tmux socket path was already registered: %q", path)
	}
	liveSocketRegistry.paths[path] = struct{}{}
	return path
}

func registeredLiveTmuxCommand(ctx context.Context, tmuxPath, socketPath string, arguments ...string) (*exec.Cmd, error) {
	if err := validateRegisteredLiveSocket(socketPath); err != nil {
		return nil, err
	}
	commandArguments := append([]string{"-S", socketPath, "-f", "/dev/null"}, arguments...)
	command := exec.CommandContext(ctx, tmuxPath, commandArguments...)
	command.Env = withoutTmuxEnvironment(os.Environ())
	return command, nil
}

func validateRegisteredLiveSocket(socketPath string) error {
	liveSocketRegistry.Lock()
	defer liveSocketRegistry.Unlock()
	if !pathStrictlyInsideLiveRoot(liveSocketRegistry.root, socketPath) {
		return fmt.Errorf("live tmux socket escapes the private root: %q", socketPath)
	}
	if _, exists := liveSocketRegistry.paths[socketPath]; !exists {
		return fmt.Errorf("live tmux socket is not registered: %q", socketPath)
	}
	return nil
}

func cleanupRegisteredLiveTmuxSockets(tmuxPath, privateRoot string) error {
	liveSocketRegistry.Lock()
	paths := make([]string, 0, len(liveSocketRegistry.paths))
	for path := range liveSocketRegistry.paths {
		paths = append(paths, path)
	}
	registeredRoot := liveSocketRegistry.root
	liveSocketRegistry.Unlock()
	if registeredRoot != privateRoot {
		return fmt.Errorf("private live socket root changed: registered=%q cleanup=%q", registeredRoot, privateRoot)
	}
	sort.Strings(paths)
	var cleanupErrors []error
	for _, path := range paths {
		if !pathStrictlyInsideLiveRoot(privateRoot, path) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("registered live socket escapes private root: %q", path))
			continue
		}
		if err := cleanupRegisteredLiveTmuxSocket(tmuxPath, path); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

func cleanupRegisteredLiveTmuxSocket(tmuxPath, socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat registered live socket %s: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("registered live socket path is not a socket: %s", socketPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), tmuxBehaviorTimeout)
	identity, live, err := queryRegisteredLiveTmuxServer(ctx, tmuxPath, socketPath)
	cancel()
	if err != nil {
		return err
	}
	if live {
		if err := verifyRegisteredLiveTmuxProcess(tmuxPath, socketPath, identity); err != nil {
			return err
		}
		if err := killRegisteredLiveTmuxServer(tmuxPath, socketPath, identity); err != nil {
			return err
		}
	}
	return removeRegisteredLiveStaleSocket(tmuxPath, socketPath)
}

func queryRegisteredLiveTmuxServer(
	ctx context.Context,
	tmuxPath, socketPath string,
) (registeredLiveTmuxIdentity, bool, error) {
	command, err := registeredLiveTmuxCommand(ctx, tmuxPath, socketPath,
		"display-message", "-p", "#{pid}|#{start_time}")
	if err != nil {
		return registeredLiveTmuxIdentity{}, false, err
	}
	output, err := command.CombinedOutput()
	contextErr := ctx.Err()
	if err != nil {
		if contextErr != nil {
			return registeredLiveTmuxIdentity{}, false, fmt.Errorf("query registered live tmux socket %s: %w", socketPath, contextErr)
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return registeredLiveTmuxIdentity{}, false, nil
		}
		return registeredLiveTmuxIdentity{}, false, fmt.Errorf("query registered live tmux socket %s: %w", socketPath, err)
	}
	pidText, tmuxStartTime, separated := strings.Cut(strings.TrimSpace(string(output)), "|")
	pid, parseErr := strconv.Atoi(pidText)
	if parseErr != nil || pid <= 1 || !separated || tmuxStartTime == "" {
		return registeredLiveTmuxIdentity{}, false,
			fmt.Errorf("invalid registered live tmux identity: socket=%s output=%q", socketPath, output)
	}
	kernelStartTime := processStartTime(pid)
	if kernelStartTime == 0 {
		return registeredLiveTmuxIdentity{}, false,
			fmt.Errorf("registered live tmux server has no kernel start time: socket=%s pid=%d", socketPath, pid)
	}
	return registeredLiveTmuxIdentity{pid: pid, kernelStartTime: kernelStartTime, tmuxStartTime: tmuxStartTime}, true, nil
}

func verifyRegisteredLiveTmuxProcess(
	tmuxPath, socketPath string,
	identity registeredLiveTmuxIdentity,
) error {
	processRoot := filepath.Join("/proc", strconv.Itoa(identity.pid))
	executable, err := os.Readlink(filepath.Join(processRoot, "exe"))
	if err != nil {
		return fmt.Errorf("read registered live tmux executable: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	commandLine, err := os.ReadFile(filepath.Join(processRoot, "cmdline"))
	if err != nil {
		return fmt.Errorf("read registered live tmux command line: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	expectedExecutable, err := os.Stat(tmuxPath)
	if err != nil {
		return fmt.Errorf("stat expected live tmux executable %s: %w", tmuxPath, err)
	}
	observedExecutable, err := os.Stat(filepath.Join(processRoot, "exe"))
	if err != nil {
		return fmt.Errorf("stat registered live tmux executable: socket=%s pid=%d: %w", socketPath, identity.pid, err)
	}
	if !os.SameFile(expectedExecutable, observedExecutable) {
		return fmt.Errorf("registered live socket process is not the expected tmux executable: socket=%s pid=%d exe=%q",
			socketPath, identity.pid, executable)
	}
	if !liveCommandLineContainsExactSocket(commandLine, socketPath) {
		return fmt.Errorf("registered live tmux command line does not name its exact socket: socket=%s pid=%d", socketPath, identity.pid)
	}
	if observed := processStartTime(identity.pid); observed != identity.kernelStartTime {
		return fmt.Errorf("registered live tmux process identity changed during inspection: socket=%s pid=%d captured=%d observed=%d",
			socketPath, identity.pid, identity.kernelStartTime, observed)
	}
	return nil
}

func liveCommandLineContainsExactSocket(commandLine []byte, socketPath string) bool {
	for _, argument := range bytes.Split(commandLine, []byte{0}) {
		text := string(argument)
		if text == socketPath || strings.Contains(text, "("+socketPath+")") {
			return true
		}
	}
	return false
}

func killRegisteredLiveTmuxServer(
	tmuxPath, socketPath string,
	identity registeredLiveTmuxIdentity,
) error {
	if observed := processStartTime(identity.pid); observed != identity.kernelStartTime {
		return fmt.Errorf("refusing live tmux cleanup after process identity changed: socket=%s pid=%d captured=%d observed=%d",
			socketPath, identity.pid, identity.kernelStartTime, observed)
	}
	condition := "#{&&:#{==:#{pid}," + strconv.Itoa(identity.pid) + "},#{==:#{start_time}," + identity.tmuxStartTime + "}}"
	const mismatchMarker = "SKIDBLADNIR_TEST_SERVER_MISMATCH_V1"
	ctx, cancel := context.WithTimeout(context.Background(), tmuxBehaviorTimeout)
	command, err := registeredLiveTmuxCommand(ctx, tmuxPath, socketPath,
		"if-shell", "-F", condition,
		"kill-server",
		"display-message -p -l '"+mismatchMarker+"'")
	if err != nil {
		cancel()
		return err
	}
	output, err := command.CombinedOutput()
	contextErr := ctx.Err()
	cancel()
	if err != nil {
		if contextErr != nil {
			return fmt.Errorf("kill verified registered live tmux server: socket=%s: %w", socketPath, contextErr)
		}
		return fmt.Errorf("kill verified registered live tmux server: socket=%s output=%q error=%w", socketPath, output, err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("refusing live tmux cleanup after routed identity mismatch: socket=%s output=%q", socketPath, output)
	}

	deadline := time.Now().Add(tmuxBehaviorTimeout)
	for processStartTime(identity.pid) == identity.kernelStartTime {
		if time.Now().After(deadline) {
			return fmt.Errorf("verified registered live tmux server survived cleanup: socket=%s pid=%d", socketPath, identity.pid)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

func removeRegisteredLiveStaleSocket(tmuxPath, socketPath string) error {
	if err := validateRegisteredLiveSocket(socketPath); err != nil {
		return err
	}
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat stale registered live socket %s: %w", socketPath, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to unlink non-socket registered live path: %s", socketPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tmuxBehaviorTimeout)
	_, live, err := queryRegisteredLiveTmuxServer(ctx, tmuxPath, socketPath)
	cancel()
	if err != nil {
		return err
	}
	if live {
		return fmt.Errorf("registered live tmux server still answers after cleanup: %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("unlink stale registered live socket %s: %w", socketPath, err)
	}
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("registered live socket survived exact unlink: %s: %w", socketPath, err)
	}
	return nil
}

func pathStrictlyInsideLiveRoot(root, path string) bool {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func removeExactLiveDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to remove non-directory live path: %s", path)
	}
	return os.Remove(path)
}
