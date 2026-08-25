//go:build live

package live

import (
	"crypto/rand"
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
	directTUIVersion      = "0.149.1"
	directTUICWD          = "/home/niels/src"
	directTUITimeout      = 10 * time.Second
	directTUIReadyTimeout = 30 * time.Second
	directTUIPersistInput = "Reply with only P0."
)

var (
	directTUIRootID      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	directTUIRolloutName = regexp.MustCompile(`^rollout-[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:_[0-9a-f-]{36})?\.jsonl$`)
)

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

type directTUIServer struct {
	process    directTUIProcess
	socketPath string
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
			var threadID string
			if !t.Run("start", func(t *testing.T) {
				threadID = assertDirectTUILaunch(t, lock.BinaryPath, profile, nil)
			}) {
				return
			}
			t.Run("resume", func(t *testing.T) {
				assertDirectTUILaunch(t, lock.BinaryPath, profile, []string{"resume", threadID})
			})
		})
	}
}

func TestDirectTUIStatusFieldsUseOnlyNewestCard(t *testing.T) {
	t.Parallel()
	completeOld := strings.Join([]string{
		"OpenAI Codex (v" + directTUIVersion + ")",
		"Model: old-model",
		"Directory: /old",
		"Permissions: Full Access",
		"Session: 11111111-1111-4111-8111-111111111111",
	}, "\n")
	newestPartial := strings.Join([]string{
		"OpenAI Codex (v" + directTUIVersion + ")",
		"Model: new-model",
	}, "\n")
	newestComplete := strings.Join([]string{
		"OpenAI Codex (v" + directTUIVersion + ")",
		"Model: new-model",
		"Directory: /new",
		"Permissions: Full Access",
		"Session: 22222222-2222-4222-8222-222222222222",
	}, "\n")

	tests := []struct {
		name              string
		screen            string
		wantCards         int
		wantModel         string
		wantDirectory     string
		wantSession       string
		wantAllFourFields bool
	}{
		{
			name:              "partial newest never falls back",
			screen:            completeOld + "\n" + newestPartial,
			wantCards:         2,
			wantModel:         "new-model",
			wantAllFourFields: false,
		},
		{
			name:              "complete newest wins",
			screen:            completeOld + "\n" + newestComplete,
			wantCards:         2,
			wantModel:         "new-model",
			wantDirectory:     "/new",
			wantSession:       "22222222-2222-4222-8222-222222222222",
			wantAllFourFields: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fields, versionSeen, cards := directTUIStatusFields(test.screen)
			allFourFields := directTUIStatusFieldsComplete(fields)
			if !versionSeen || cards != test.wantCards || fields["Model"] != test.wantModel || fields["Directory"] != test.wantDirectory || fields["Session"] != test.wantSession || allFourFields != test.wantAllFourFields {
				t.Fatalf("newest status-card parse differs: version=%t cards=%t model=%t directory=%t session=%t complete=%t", versionSeen, cards == test.wantCards, fields["Model"] == test.wantModel, fields["Directory"] == test.wantDirectory, fields["Session"] == test.wantSession, allFourFields)
			}
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

func assertDirectTUILaunch(t *testing.T, binary string, profile directTUIProfile, prefix []string) string {
	t.Helper()
	var baselineRoots map[string]struct{}
	if len(prefix) == 0 {
		baselineRoots = snapshotDirectTUIRoots(t, profile.home)
	}
	socket := fmt.Sprintf("skidbladnir_direct_%d_%d", os.Getpid(), time.Now().UnixNano())
	runtimeID := newDirectTUIRuntimeID(t)
	startMarker := filepath.Join(t.TempDir(), "start")
	codexArguments := []string{binary}
	if len(prefix) == 0 {
		codexArguments = append(codexArguments, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD)
	} else {
		codexArguments = append(codexArguments, prefix[0], "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD, prefix[1])
	}
	quotedCodex := make([]string, len(codexArguments))
	for index, argument := range codexArguments {
		quotedCodex[index] = shellQuote(argument)
	}
	launch := "while [ ! -e " + shellQuote(startMarker) + " ]; do /usr/bin/sleep 0.01; done; exec " + strings.Join(quotedCodex, " ")
	arguments := []string{"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "direct", "-x", "100", "-y", "30", "-c", directTUICWD, "-e", "CODEX_HOME=" + profile.home, "-e", "SKIDBLADNIR_RUNTIME_ID=" + runtimeID, "/bin/sh", "-c", launch}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start %s direct TUI: %v: %s", profile.name, err, boundedDirectTUIError(output))
	}
	server := resolveDirectTUIServer(t, socket)
	t.Cleanup(func() {
		cleanupDirectTUIServer(t, socket, server)
	})
	client := startNamedTUIBehaviorClient(t, socket, "direct")
	wrapperProcess := directTUIProcess{pid: client.command.Process.Pid, startTime: processStartTime(client.command.Process.Pid)}
	if wrapperProcess.startTime == 0 {
		t.Fatal("test-owned direct TUI client has no stable process identity")
	}
	if err := os.WriteFile(startMarker, nil, 0o600); err != nil {
		t.Fatalf("release %s direct TUI launch: %v", profile.name, err)
	}

	process := awaitDirectTUIProcess(t, socket)
	assertDirectTUIProcess(t, binary, profile, runtimeID, process, prefix)
	expectedRoot := ""
	if len(prefix) > 0 {
		expectedRoot = prefix[1]
	}
	expectedModel := readDirectTUIModel(t, profile.home)
	threadID := assertDirectTUIReady(t, socket, client, expectedRoot, expectedModel, true)
	assertDirectTUIRuntimeBoundary(t, binary, runtimeID, process)
	if len(prefix) == 0 {
		if _, existed := baselineRoots[threadID]; existed {
			t.Fatal("fresh direct TUI reused a canonical rollout UUID present before launch")
		}
		persistDirectTUIRoot(t, socket, client, profile.home, threadID)
		assertDirectTUIRuntimeBoundary(t, binary, runtimeID, process)
		assertDirectTUIReady(t, socket, client, threadID, expectedModel, false)
	}
	assertDirectTUIRuntimeBoundary(t, binary, runtimeID, process)
	submitDirectTUICommand(t, socket, client, "/quit")
	awaitDirectTUITeardown(t, socket, server, wrapperProcess, client.process, process, binary, runtimeID)
	return threadID
}

func newDirectTUIRuntimeID(t *testing.T) string {
	t.Helper()
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		t.Fatalf("create unique direct TUI runtime marker: %v", err)
	}
	identifier[6] = (identifier[6] & 0x0f) | 0x40
	identifier[8] = (identifier[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", identifier[0:4], identifier[4:6], identifier[6:8], identifier[8:10], identifier[10:16])
}

func awaitDirectTUIComposer(t *testing.T, socket string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	lastFacts := directTUIReadyFacts{}
	for time.Now().Before(deadline) {
		screen, err := captureDirectTUIPane(socket, false)
		lastFacts = directTUIReadyFactsForScreen(screen)
		if err == nil && lastFacts.ready(false) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("direct TUI did not return to the Ready composer: composer=%t model=%t loading=%t review=%t invalid_config=%t failed_resume=%t", lastFacts.composer, lastFacts.model, lastFacts.loading, lastFacts.review, lastFacts.invalidConfig, lastFacts.failedResume)
}

func persistDirectTUIRoot(t *testing.T, socket string, client *tuiBehaviorClient, home, threadID string) {
	t.Helper()
	if directTUIRolloutExists(t, home, threadID) {
		t.Fatal("fresh canonical root existed before test-owned persistence input")
	}
	sendTUIClientBytes(t, client, "\x15")
	time.Sleep(100 * time.Millisecond)
	emptyCursor := readDirectTUICursor(t, socket)
	for _, character := range directTUIPersistInput {
		sendTUIClientBytes(t, client, string(character))
		time.Sleep(25 * time.Millisecond)
	}
	deadline := time.Now().Add(directTUITimeout)
	draftCursor := ""
	for time.Now().Before(deadline) {
		cursor := readDirectTUICursor(t, socket)
		if cursor != emptyCursor {
			draftCursor = cursor
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if draftCursor == "" {
		t.Fatal("test-owned persistence input did not reach the Ready composer")
	}
	sendTUIClientBytes(t, client, "\r")

	deadline = time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if directTUIRolloutExists(t, home, threadID) {
			// The synthetic turn's terminal content is neither inspected nor retained.
			sendTUIClientBytes(t, client, "\x1b")
			awaitDirectTUIComposer(t, socket, directTUIReadyTimeout)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("test-owned persistence input did not create its canonical rollout")
}

func snapshotDirectTUIRoots(t *testing.T, home string) map[string]struct{} {
	t.Helper()
	roots := make(map[string]struct{})
	err := filepath.WalkDir(filepath.Join(home, "sessions"), func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		match := directTUIRolloutName.FindStringSubmatch(entry.Name())
		if match != nil {
			roots[match[1]] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot canonical rollout basenames: %v", err)
	}
	return roots
}

func directTUIRolloutExists(t *testing.T, home, threadID string) bool {
	t.Helper()
	_, found := snapshotDirectTUIRoots(t, home)[threadID]
	return found
}

func assertDirectTUIReady(t *testing.T, socket string, client *tuiBehaviorClient, expectedRoot, expectedModel string, requireVisibleModel bool) string {
	t.Helper()
	statusDirectory := directTUIStatusDirectory()
	deadline := time.Now().Add(directTUIReadyTimeout)
	ready := false
	lastFacts := directTUIReadyFacts{}
	for time.Now().Before(deadline) {
		screen, err := captureDirectTUIPane(socket, false)
		lastFacts = directTUIReadyFactsForScreen(screen)
		if err == nil && lastFacts.ready(requireVisibleModel) {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("direct TUI did not reach a model-resolved Ready composer: composer=%t model=%t loading=%t review=%t invalid_config=%t failed_resume=%t", lastFacts.composer, lastFacts.model, lastFacts.loading, lastFacts.review, lastFacts.invalidConfig, lastFacts.failedResume)
	}
	time.Sleep(500 * time.Millisecond)

	deadline = time.Now().Add(directTUITimeout)
	var lastFields map[string]string
	lastVersionSeen := false
	lastCurrentStatus := false
	for time.Now().Before(deadline) {
		priorStatusCards := requestDirectTUIStatus(t, socket, client)
		attemptDeadline := time.Now().Add(time.Second)
		for time.Now().Before(attemptDeadline) {
			screen, err := captureDirectTUIPane(socket, true)
			if err == nil {
				fields, versionSeen, statusCards := directTUIStatusFields(screen)
				lastFields = fields
				lastVersionSeen = versionSeen
				lastCurrentStatus = statusCards > priorStatusCards
				if lastCurrentStatus && versionSeen && directTUIStatusFieldsComplete(fields) && fields["Directory"] == statusDirectory && fields["Permissions"] == "Full Access" && directTUIStatusModelMatches(fields["Model"], expectedModel) && directTUIRootID.MatchString(fields["Session"]) {
					if expectedRoot == "" || fields["Session"] == expectedRoot {
						return fields["Session"]
					}
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	statusSeen := lastFields != nil
	directoryMatches := lastFields["Directory"] == statusDirectory
	permissionsMatch := lastFields["Permissions"] == "Full Access"
	modelMatches := directTUIStatusModelMatches(lastFields["Model"], expectedModel)
	modelPresent := lastFields["Model"] != ""
	canonicalRootSeen := directTUIRootID.MatchString(lastFields["Session"])
	if expectedRoot == "" {
		t.Fatalf("direct TUI status did not confirm exact version, cwd, model, Full Access, and canonical test-owned root: status=%t current=%t version=%t directory=%t model_present=%t model=%t permissions=%t root=%t", statusSeen, lastCurrentStatus, lastVersionSeen, directoryMatches, modelPresent, modelMatches, permissionsMatch, canonicalRootSeen)
	}
	t.Fatalf("direct TUI status did not confirm exact version, cwd, model, Full Access, and requested root: status=%t current=%t version=%t directory=%t model_present=%t model=%t permissions=%t root=%t requested=%t", statusSeen, lastCurrentStatus, lastVersionSeen, directoryMatches, modelPresent, modelMatches, permissionsMatch, canonicalRootSeen, lastFields["Session"] == expectedRoot)
	return ""
}

type directTUIReadyFacts struct {
	composer      bool
	model         bool
	loading       bool
	review        bool
	invalidConfig bool
	failedResume  bool
}

func directTUIReadyFactsForScreen(screen string) directTUIReadyFacts {
	lower := strings.ToLower(screen)
	return directTUIReadyFacts{
		composer:      strings.Contains(screen, "Ask Codex to do anything"),
		model:         strings.Contains(lower, "model:"),
		loading:       strings.Contains(lower, "model:       loading"),
		review:        strings.Contains(lower, "hooks need review"),
		invalidConfig: strings.Contains(lower, "invalid configuration") || strings.Contains(lower, "configuration error") || strings.Contains(lower, "failed to load configuration"),
		failedResume:  strings.Contains(lower, "no rollout found for thread id") || strings.Contains(lower, "failed to resume") || strings.Contains(lower, "unable to resume") || strings.Contains(lower, "resume failed"),
	}
}

func (facts directTUIReadyFacts) ready(requireVisibleModel bool) bool {
	modelReady := !facts.loading
	if requireVisibleModel {
		modelReady = facts.model && modelReady
	}
	return facts.composer && modelReady && !facts.review && !facts.invalidConfig && !facts.failedResume
}

func directTUIStatusModelMatches(statusModel, configuredModel string) bool {
	return statusModel == configuredModel || strings.HasPrefix(statusModel, configuredModel+" ")
}

func requestDirectTUIStatus(t *testing.T, socket string, client *tuiBehaviorClient) int {
	t.Helper()
	screen, err := captureDirectTUIPane(socket, true)
	if err != nil {
		t.Fatalf("capture direct TUI before /status: %v", err)
	}
	_, _, priorStatusCards := directTUIStatusFields(screen)
	submitDirectTUICommand(t, socket, client, "/status")
	return priorStatusCards
}

func submitDirectTUICommand(t *testing.T, socket string, client *tuiBehaviorClient, command string) {
	t.Helper()
	draftMarker := "› " + command
	for attempt := 0; attempt < 3; attempt++ {
		sendTUIClientBytes(t, client, "\x15")
		time.Sleep(100 * time.Millisecond)
		for _, character := range command {
			sendTUIClientBytes(t, client, string(character))
			time.Sleep(25 * time.Millisecond)
		}
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			screen, err := captureDirectTUIPane(socket, false)
			if err == nil && strings.Contains(screen, draftMarker) {
				sendTUIClientBytes(t, client, "\r")
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	t.Fatal("direct TUI did not render the content-free local command draft after bounded retries")
}

func readDirectTUICursor(t *testing.T, socket string) string {
	t.Helper()
	output, err := exec.Command("tmux", "-L", socket, "display-message", "-p", "-t", "direct:0.0", "#{cursor_x}|#{cursor_y}").Output()
	if err != nil {
		t.Fatalf("read test-owned direct TUI cursor: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func directTUIStatusDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return directTUICWD
	}
	relative, err := filepath.Rel(home, directTUICWD)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return directTUICWD
	}
	if relative == "." {
		return "~"
	}
	return "~" + string(filepath.Separator) + relative
}

func readDirectTUIModel(t *testing.T, home string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read direct TUI profile config: %v", err)
	}
	modelLine := regexp.MustCompile(`(?m)^model\s*=\s*"([^"]+)"\s*$`)
	match := modelLine.FindSubmatch(contents)
	if len(match) != 2 || len(match[1]) == 0 {
		t.Fatal("direct TUI profile config does not declare one simple model")
	}
	return string(match[1])
}

func captureDirectTUIPane(socket string, history bool) (string, error) {
	arguments := []string{"-L", socket, "capture-pane", "-p", "-J"}
	if history {
		arguments = append(arguments, "-S", "-")
	}
	arguments = append(arguments, "-t", "direct:0.0")
	output, err := exec.Command("tmux", arguments...).Output()
	return string(output), err
}

func directTUIStatusFields(screen string) (map[string]string, bool, int) {
	lines := strings.Split(screen, "\n")
	versionLines := make([]int, 0, 2)
	for index, line := range lines {
		line = trimDirectTUIStatusLine(line)
		if strings.Contains(line, "OpenAI Codex (v"+directTUIVersion+")") {
			versionLines = append(versionLines, index)
		}
	}
	if len(versionLines) == 0 {
		return nil, false, 0
	}

	newestVersionLine := versionLines[len(versionLines)-1]
	fields := make(map[string]string, 4)
	for _, line := range lines[newestVersionLine+1:] {
		line = trimDirectTUIStatusLine(line)
		for _, name := range []string{"Model", "Directory", "Permissions", "Session"} {
			prefix := name + ":"
			if strings.HasPrefix(line, prefix) {
				fields[name] = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return fields, true, len(versionLines)
}

func directTUIStatusFieldsComplete(fields map[string]string) bool {
	for _, name := range []string{"Model", "Directory", "Permissions", "Session"} {
		if fields[name] == "" {
			return false
		}
	}
	return true
}

func trimDirectTUIStatusLine(line string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
}

func resolveDirectTUIServer(t *testing.T, socket string) directTUIServer {
	t.Helper()
	output, err := exec.Command("tmux", "-L", socket, "display-message", "-p", "#{pid}|#{socket_path}").Output()
	if err != nil {
		t.Fatalf("resolve test-owned tmux server: %v", err)
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 2 {
		t.Fatal("test-owned tmux server identity is incomplete")
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatal("test-owned tmux server PID is invalid")
	}
	socketPath := filepath.Clean(parts[1])
	if !filepath.IsAbs(socketPath) || filepath.Base(socketPath) != socket {
		t.Fatal("test-owned tmux server returned an unexpected socket path")
	}
	startTime := processStartTime(pid)
	if startTime == 0 {
		t.Fatal("test-owned tmux server has no stable process identity")
	}
	return directTUIServer{
		process:    directTUIProcess{pid: pid, startTime: startTime},
		socketPath: socketPath,
	}
}

func cleanupDirectTUIServer(t *testing.T, socket string, server directTUIServer) {
	t.Helper()
	if directTUIProcessAlive(server.process) {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: cleanup accepts an already-exited exact test-owned server.
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && directTUIProcessAlive(server.process) {
		time.Sleep(20 * time.Millisecond)
	}
	if directTUIProcessAlive(server.process) {
		t.Error("exact test-owned tmux server survived cleanup")
		return
	}
	removeDeadDirectTUISocket(t, socket, server)
}

func removeDeadDirectTUISocket(t *testing.T, socket string, server directTUIServer) {
	t.Helper()
	if directTUIProcessAlive(server.process) {
		t.Error("refused to remove a socket for a live test-owned tmux server")
		return
	}
	if !filepath.IsAbs(server.socketPath) || filepath.Base(server.socketPath) != socket {
		t.Error("refused to remove an unverified tmux socket path")
		return
	}
	if err := os.Remove(server.socketPath); err != nil && !os.IsNotExist(err) {
		t.Errorf("remove dead test-owned tmux socket: %v", err)
		return
	}
	if _, err := os.Lstat(server.socketPath); !os.IsNotExist(err) {
		t.Error("dead test-owned tmux socket remains after cleanup")
	}
}

func awaitDirectTUITeardown(t *testing.T, socket string, server directTUIServer, wrapper, client, pane directTUIProcess, binary, runtimeID string) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	lastPaneGone := false
	lastWrapperGone := false
	lastClientGone := false
	lastServerGone := false
	lastClientTTYGone := false
	lastPaneTTYGone := false
	lastRuntimeGone := false
	lastRuntimeScanned := false
	lastSocketGone := false
	for time.Now().Before(deadline) {
		lastPaneGone = !directTUIProcessAlive(pane) && directTUIPaneGone(socket, pane)
		lastWrapperGone = !directTUIProcessAlive(wrapper)
		lastClientGone = !directTUIProcessAlive(client)
		lastServerGone = !directTUIProcessAlive(server.process)
		lastClientTTYGone = directTUITTYGone(client.tty)
		lastPaneTTYGone = directTUITTYGone(pane.tty)
		runtimePresent, runtimeErr := directTUIRuntimeMarkerPresent(binary, runtimeID)
		lastRuntimeScanned = runtimeErr == nil
		lastRuntimeGone = lastRuntimeScanned && !runtimePresent
		if lastServerGone {
			removeDeadDirectTUISocket(t, socket, server)
		}
		_, socketErr := os.Lstat(server.socketPath)
		lastSocketGone = os.IsNotExist(socketErr)
		if lastPaneGone && lastWrapperGone && lastClientGone && lastServerGone && lastClientTTYGone && lastPaneTTYGone && lastRuntimeGone && lastSocketGone {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("direct TUI teardown was incomplete: pane=%t wrapper=%t client=%t server=%t client_tty=%t pane_tty=%t runtime_scan=%t runtime=%t socket=%t", lastPaneGone, lastWrapperGone, lastClientGone, lastServerGone, lastClientTTYGone, lastPaneTTYGone, lastRuntimeScanned, lastRuntimeGone, lastSocketGone)
}

func directTUIPaneGone(socket string, pane directTUIProcess) bool {
	output, err := exec.Command("tmux", "-L", socket, "list-panes", "-a", "-F", "#{pane_pid}|#{pane_tty}").Output()
	if err != nil {
		return true
	}
	want := strconv.Itoa(pane.pid) + "|" + pane.tty
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == want {
			return false
		}
	}
	return true
}

func directTUITTYGone(tty string) bool {
	_, err := os.Lstat(tty)
	return os.IsNotExist(err)
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

type directTUIRuntimeFacts struct {
	rootMarked  bool
	markedCount int
	detachedPin bool
	forbidden   bool
	duplicate   directTUIDuplicateFacts
}

type directTUIDuplicateFacts struct {
	found      bool
	pid        int
	startTime  uint64
	parentPID  int
	processGrp int
	tty        string
	exactPin   bool
	descendant bool
	category   string
}

func assertDirectTUIRuntimeBoundary(t *testing.T, binary, runtimeID string, process directTUIProcess) {
	t.Helper()
	if !directTUIProcessAlive(process) {
		t.Fatal("exact direct TUI process disappeared at a runtime boundary")
	}
	assertNoDirectTUIAppServerChild(t, process.pid)
	facts, err := inspectDirectTUIRuntime(binary, runtimeID, process)
	if err != nil {
		t.Fatalf("inspect direct TUI runtime boundary: %v", err)
	}
	if !facts.rootMarked || facts.detachedPin || facts.forbidden {
		t.Fatalf("direct TUI runtime boundary is not isolated: root=%t detached_pin=%t forbidden=%t duplicate=%t pid=%d start=%d ppid=%d pgrp=%d tty=%q exact_pin=%t descendant=%t category=%s", facts.rootMarked, facts.detachedPin, facts.forbidden, facts.duplicate.found, facts.duplicate.pid, facts.duplicate.startTime, facts.duplicate.parentPID, facts.duplicate.processGrp, facts.duplicate.tty, facts.duplicate.exactPin, facts.duplicate.descendant, facts.duplicate.category)
	}
}

func directTUIRuntimeMarkerPresent(binary, runtimeID string) (bool, error) {
	facts, err := inspectDirectTUIRuntime(binary, runtimeID, directTUIProcess{})
	return facts.markedCount != 0, err
}

func inspectDirectTUIRuntime(binary, runtimeID string, root directTUIProcess) (directTUIRuntimeFacts, error) {
	facts := directTUIRuntimeFacts{}
	pinned, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return facts, err
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return facts, err
	}
	marker := "SKIDBLADNIR_RUNTIME_ID=" + runtimeID
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		environment, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "environ"))
		if err != nil || !containsDirectTUIToken(strings.Split(strings.TrimRight(string(environment), "\x00"), "\x00"), marker) {
			continue
		}
		facts.markedCount++
		startTime := processStartTime(pid)
		if pid == root.pid && startTime != 0 && startTime == root.startTime {
			facts.rootMarked = true
		}
		executable, err := filepath.EvalSymlinks(filepath.Join("/proc", entry.Name(), "exe"))
		if err == nil && executable == pinned && (pid != root.pid || startTime != root.startTime) {
			facts.detachedPin = true
			if !facts.duplicate.found {
				facts.duplicate = inspectDirectTUIDuplicate(pid, root.pid, true)
			}
		}
		commandLine, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(commandLine), "\x00"), "\x00")
		for _, forbidden := range []string{"app-server", "agents", "--remote"} {
			if containsDirectTUIOption(argv, forbidden) {
				facts.forbidden = true
			}
		}
	}
	return facts, nil
}

func inspectDirectTUIDuplicate(pid, rootPID int, exactPin bool) directTUIDuplicateFacts {
	facts := directTUIDuplicateFacts{
		found:      true,
		pid:        pid,
		startTime:  processStartTime(pid),
		exactPin:   exactPin,
		descendant: directTUIProcessDescendsFrom(pid, rootPID),
		category:   "other",
	}
	fields, err := procStatFieldsForLive(pid)
	if err == nil && len(fields) >= 3 {
		facts.parentPID, _ = strconv.Atoi(fields[1])
		facts.processGrp, _ = strconv.Atoi(fields[2])
	}
	if tty, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", "0")); err == nil {
		facts.tty = tty
	}
	if commandLine, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline")); err == nil {
		argv := strings.Split(strings.TrimRight(string(commandLine), "\x00"), "\x00")
		for _, category := range []string{"app-server", "agents", "--remote"} {
			if containsDirectTUIToken(argv, category) {
				facts.category = category
				break
			}
		}
	}
	return facts
}

func directTUIProcessDescendsFrom(pid, rootPID int) bool {
	visited := make(map[int]struct{})
	for pid > 1 {
		if pid == rootPID {
			return true
		}
		if _, seen := visited[pid]; seen {
			return false
		}
		visited[pid] = struct{}{}
		fields, err := procStatFieldsForLive(pid)
		if err != nil || len(fields) < 2 {
			return false
		}
		pid, err = strconv.Atoi(fields[1])
		if err != nil {
			return false
		}
	}
	return false
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

func containsDirectTUIOption(values []string, target string) bool {
	for _, value := range values {
		if value == target || strings.HasPrefix(value, target+"=") {
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
		if containsDirectTUIOption(argv, "app-server") || containsDirectTUIOption(argv, "agents") || containsDirectTUIOption(argv, "--remote") {
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
