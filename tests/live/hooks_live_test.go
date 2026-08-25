//go:build live

package live

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type directHookLock struct {
	Helper struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"helper"`
	Profiles []struct {
		TargetPath string            `json:"targetPath"`
		TrustState map[string]string `json:"trustState"`
	} `json:"profiles"`
}

type capturedHookFact struct {
	Type          string `json:"type"`
	Hook          string `json:"hook"`
	Projection    string `json:"projection"`
	RuntimeID     string `json:"runtime_id"`
	PID           int    `json:"pid"`
	StartTime     uint64 `json:"start_time"`
	TTY           string `json:"tty"`
	ThreadID      string `json:"thread_id"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	EffectiveCWD  string `json:"effective_cwd,omitempty"`
	Model         string `json:"model,omitempty"`
	SessionSource string `json:"session_source,omitempty"`
	Objective     string `json:"objective,omitempty"`
}

type directHookListEntry struct {
	Key         string  `json:"key"`
	EventName   string  `json:"eventName"`
	HandlerType string  `json:"handlerType"`
	Command     string  `json:"command"`
	Async       bool    `json:"async"`
	Matcher     *string `json:"matcher"`
	TimeoutSec  uint64  `json:"timeoutSec"`
	SourcePath  string  `json:"sourcePath"`
	Source      string  `json:"source"`
	PluginID    *string `json:"pluginId"`
	Enabled     bool    `json:"enabled"`
	IsManaged   bool    `json:"isManaged"`
	CurrentHash string  `json:"currentHash"`
	TrustStatus string  `json:"trustStatus"`
}

type directHookListResponse struct {
	Data []struct {
		CWD      string   `json:"cwd"`
		Warnings []string `json:"warnings"`
		Errors   []struct {
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"errors"`
		Hooks []directHookListEntry `json:"hooks"`
	} `json:"data"`
}

type directHookConfigFile struct {
	Hooks map[string][]directHookMatcherGroup `json:"hooks"`
}

type directHookMatcherGroup struct {
	Hooks []directHookHandler `json:"hooks"`
}

type directHookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout uint64 `json:"timeout"`
	Async   bool   `json:"async"`
}

type directHookCapture struct {
	listener     *net.UnixListener
	socketPath   string
	gapDirectory string
	mu           sync.Mutex
	facts        []capturedHookFact
	changed      chan struct{}
	done         chan struct{}
	connections  sync.WaitGroup
	sessionEnd   chan struct{}
	releaseEnd   sync.Once
}

type directHookHarness struct {
	socket    string
	runtimeID string
	process   directTUIProcess
	provider  *tuiBehaviorProvider
	client    *tuiBehaviorClient
}

type directHookTurn struct {
	ThreadID  string
	SessionID string
	TurnID    string
	Source    string
}

func TestDirectTUIHookOrigin(t *testing.T) {
	requireExactTmux(t)
	lock := readDirectTUILock(t)
	t.Run("untrusted hook requires review", func(t *testing.T) {
		proveDirectTUIUntrustedHookReview(t, lock.BinaryPath)
	})
	capture := startDirectHookCapture(t)
	harness := startDirectHookHarness(t, lock.BinaryPath, capture)
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "Ask Codex to do anything")
	}, "trusted-hook TUI composer readiness")

	sendDirectHookInput(t, harness, "origin-probe")
	waitForTUIProviderRequests(t, harness.provider, 1)
	rootFacts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
		return hookNamesForPID(facts, harness.process.pid)["SessionStart"] &&
			hookNamesForPID(facts, harness.process.pid)["UserPromptSubmit"] &&
			hookNamesForPID(facts, harness.process.pid)["Stop"]
	})
	assertRootHookFacts(t, rootFacts, harness)
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "Ask Codex to do anything") && strings.Contains(screen, "Ready")
	}, "idle composer after root origin turn")

	nestedCommand := "!" + shellQuote(lock.BinaryPath) +
		" exec --strict-config --dangerously-bypass-approvals-and-sandbox" +
		" --skip-git-repo-check -C " + shellQuote(directTUICWD) + " nested-origin-probe"
	sendDirectHookInput(t, harness, nestedCommand)
	waitForTUIProviderRequests(t, harness.provider, 2)
	nestedFacts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
		for _, fact := range facts {
			if fact.RuntimeID == harness.runtimeID && fact.PID != harness.process.pid && fact.Hook == "SessionEnd" {
				return true
			}
		}
		return false
	})
	assertNestedHookFactsAreUnauthoritative(t, nestedFacts, harness)
}

func proveDirectTUIUntrustedHookReview(t *testing.T, binary string) {
	t.Helper()
	capture := startDirectHookCapture(t)
	provider := startTUIBehaviorProvider(t)
	home := t.TempDir()
	writeTrustedDirectHookProfile(t, binary, home, provider.listener.Addr().String(), capture.socketPath, capture.gapDirectory, false)
	untrustedCommand, untrustedSentinel := appendDirectTUIUntrustedHook(t, filepath.Join(home, "hooks.json"))
	listed := readDirectHookUniverse(t, binary, home, directTUICWD)
	if len(listed.Data) != 1 || len(listed.Data[0].Hooks) != 8 || len(listed.Data[0].Warnings) != 0 || len(listed.Data[0].Errors) != 0 {
		t.Fatal("foreign-hook discovery did not expose seven reviewed handlers and one clean negative fixture")
	}
	foreign := 0
	for _, hook := range listed.Data[0].Hooks {
		if hook.Command != untrustedCommand {
			continue
		}
		foreign++
		if hook.TrustStatus != "untrusted" || !hook.Enabled || hook.IsManaged || hook.Source != "user" {
			t.Fatal("foreign-hook discovery did not expose the expected enabled-but-untrusted shape")
		}
	}
	if foreign != 1 {
		t.Fatalf("foreign-hook discovery count = %d, want 1", foreign)
	}

	socket := fmt.Sprintf("skidbladnir_hook_review_%d_%d", os.Getpid(), time.Now().UnixNano())
	runtimeID := fmt.Sprintf("%08x-4444-4444-8444-%012x", uint32(time.Now().UnixNano()), uint64(os.Getpid()))
	arguments := []string{
		"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "hooks",
		"-x", "100", "-y", "30", "-c", directTUICWD,
		"-e", "CODEX_HOME=" + home,
		"-e", "SKIDBLADNIR_RUNTIME_ID=" + runtimeID,
		binary, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD,
	}
	if output, err := exec.Command("tmux", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start foreign-hook review TUI: %v: %s", err, boundedDirectTUIError(output))
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: cleanup accepts an already-exited test-owned tmux server.
	})
	startNamedTUIBehaviorClient(t, socket, "hooks")
	process := awaitNamedTUIProcess(t, socket, "hooks")
	waitForDirectHookTUICapture(t, socket, func(screen string) bool {
		return strings.Contains(screen, "Hooks need review") && strings.Contains(screen, "1 hook is new or changed")
	}, "foreign-hook review screen")
	if process.startTime != processStartTime(process.pid) {
		t.Fatal("foreign-hook review replaced or ended the exact TUI process")
	}
	if _, err := os.Stat(untrustedSentinel); err == nil {
		t.Fatal("untrusted hook executed before review")
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect untrusted hook sentinel: %v", err)
	}
	if facts := capture.snapshot(); len(facts) != 0 {
		t.Fatal("foreign-hook review emitted lifecycle facts before the user continued")
	}
}

func appendDirectTUIUntrustedHook(t *testing.T, hooksPath string) (string, string) {
	t.Helper()
	hooksBytes, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read isolated reviewed hooks for negative fixture: %v", err)
	}
	var hookConfig directHookConfigFile
	if err := json.Unmarshal(hooksBytes, &hookConfig); err != nil {
		t.Fatalf("decode isolated reviewed hooks for negative fixture: %v", err)
	}
	groups := hookConfig.Hooks["SessionStart"]
	if len(groups) != 1 || len(groups[0].Hooks) != 1 {
		t.Fatal("reviewed SessionStart hook shape changed")
	}
	untrustedSentinel := filepath.Join(t.TempDir(), "untrusted-hook-ran")
	untrustedCommand := "/usr/bin/touch " + untrustedSentinel
	groups[0].Hooks = append(groups[0].Hooks, directHookHandler{
		Type: "command", Command: untrustedCommand, Timeout: 1, Async: false,
	})
	hookConfig.Hooks["SessionStart"] = groups
	withUntrusted, err := json.MarshalIndent(hookConfig, "", "  ")
	if err != nil {
		t.Fatalf("encode isolated negative hook fixture: %v", err)
	}
	withUntrusted = append(withUntrusted, '\n')
	if err := os.WriteFile(hooksPath, withUntrusted, 0o600); err != nil {
		t.Fatalf("write isolated negative hook fixture: %v", err)
	}
	return untrustedCommand, untrustedSentinel
}

func TestDirectHookDiscoveryMustMatchFrozenTrustHashes(t *testing.T) {
	lock := directHookLock{}
	for profileIndex := 0; profileIndex < 3; profileIndex++ {
		profile := struct {
			TargetPath string            `json:"targetPath"`
			TrustState map[string]string `json:"trustState"`
		}{
			TargetPath: fmt.Sprintf("/profile-%d/hooks.json", profileIndex),
			TrustState: map[string]string{},
		}
		for hookIndex := 0; hookIndex < 7; hookIndex++ {
			profile.TrustState[fmt.Sprintf("%s:event-%d:0:0", profile.TargetPath, hookIndex)] = fmt.Sprintf("sha256:%064x", hookIndex+1)
		}
		lock.Profiles = append(lock.Profiles, profile)
	}
	hooks := make([]directHookListEntry, 7)
	for index := range hooks {
		hooks[index].CurrentHash = fmt.Sprintf("sha256:%064x", index+1)
	}
	if err := validateDirectHookTrustLock(lock, hooks); err != nil {
		t.Fatalf("matching frozen trust hashes rejected: %v", err)
	}
	hooks[0].CurrentHash = "sha256:" + strings.Repeat("f", 64)
	if err := validateDirectHookTrustLock(lock, hooks); err == nil {
		t.Fatal("discovered hook hash drift was accepted")
	}
}

func TestDirectTUIHookIdentity(t *testing.T) {
	requireExactTmux(t)
	lock := readDirectTUILock(t)
	capture := startDirectHookCapture(t)
	provider := startDirectHookIdentityProvider(t, capture)
	harness := startDirectHookHarnessWithProvider(t, lock.BinaryPath, provider, capture, true)
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "Ask Codex to do anything")
	}, "identity-probe TUI composer readiness")

	turns := []directHookTurn{submitDirectHookPrompt(t, capture, harness, "identity-root", 1)}
	for _, rollover := range []struct {
		command string
		prompt  string
		source  string
	}{
		{command: "/new", prompt: "identity-new", source: "startup"},
		{command: "/clear", prompt: "identity-clear", source: "clear"},
		{command: "/fork", prompt: "identity-fork", source: "startup"},
	} {
		sendDirectHookSlash(t, harness, rollover.command)
		turn := submitDirectHookPrompt(t, capture, harness, rollover.prompt, int64(len(turns)+1))
		if turn.Source != rollover.source {
			t.Fatalf("%s SessionStart source = %q, want %q", rollover.command, turn.Source, rollover.source)
		}
		turns = append(turns, turn)
	}
	seen := make(map[string]bool, len(turns))
	for _, turn := range turns {
		if turn.ThreadID == "" || turn.SessionID != turn.ThreadID || turn.TurnID == "" || seen[turn.ThreadID] {
			t.Fatal("root rollover did not produce a distinct thread.id/session identity")
		}
		seen[turn.ThreadID] = true
	}
	if harness.process.startTime != processStartTime(harness.process.pid) {
		t.Fatal("/new, /clear, or /fork replaced the pinned TUI process")
	}
	branchSource := turns[len(turns)-1]
	reverted := backtrackAndResubmitDirectHookPrompt(t, capture, harness, branchSource, "identity-fork", 5)
	if reverted.ThreadID == branchSource.ThreadID || reverted.SessionID != reverted.ThreadID || reverted.Source != "startup" {
		t.Fatal("backtrack did not create the pin's source-preserving branch identity")
	}
	native := submitDirectHookPrompt(t, capture, harness, "native-spawn-probe", 8)
	if native.ThreadID != reverted.ThreadID || native.SessionID != native.ThreadID {
		t.Fatal("native subagent turn changed the selected root identity")
	}
	assertNativeSubagentActivityOnly(t, capture.snapshot(), harness, native)
}

func TestDirectTUILifecycle(t *testing.T) {
	requireExactTmux(t)
	lock := readDirectTUILock(t)

	t.Run("active interrupt then ordinary exit", func(t *testing.T) {
		capture := startDirectHookCaptureHoldingSessionEnd(t)
		held := startDirectHookHeldProvider(t)
		harness := startDirectHookHarnessWithProvider(t, lock.BinaryPath, held.provider, capture, false)
		interrupted := startDirectHookTurn(t, capture, harness, "hold-active-probe", 1)
		waitForDirectHookSignal(t, held.accepted, "held provider acceptance")
		sendTUIClientBytes(t, harness.client, "\x1b")
		waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
			return strings.Contains(screen, "Ask Codex to do anything") && strings.Contains(screen, "Ready")
		}, "idle composer after active Escape")
		assertDirectHookProcessLive(t, harness)
		assertNoStopForDirectHookTurn(t, capture.snapshot(), interrupted)
		select {
		case <-held.canceled:
			t.Fatal("pin unexpectedly canceled the held provider transport on active Escape")
		default:
		}
		held.releaseResponse()
		waitForDirectHookSignal(t, held.finished, "held provider release after active Escape")
		time.Sleep(100 * time.Millisecond)
		assertNoStopForDirectHookTurn(t, capture.snapshot(), interrupted)

		recovery := submitDirectHookPrompt(t, capture, harness, "interrupt-recovery-probe", 2)
		if recovery.ThreadID != interrupted.ThreadID || recovery.SessionID != interrupted.SessionID {
			t.Fatal("post-interrupt continuation changed the selected root thread")
		}
		assertNoStopForDirectHookTurn(t, capture.snapshot(), interrupted)

		sendDirectHookSlash(t, harness, "/new p0-empty-root")
		waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
			return strings.Contains(screen, "p0-empty-root") && strings.Contains(screen, "Ready")
		}, "named empty replacement root")
		setDirectHookRemainOnExit(t, harness)
		sendDirectHookInput(t, harness, "/quit")
		facts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
			ended := directHookEndedThreads(facts, harness)
			return ended[recovery.ThreadID] && len(ended) >= 2
		})
		assertGracefulDirectHookEndSet(t, facts, harness, recovery.ThreadID)
		assertDirectHookProcessLive(t, harness)
		assertDirectHookPaneAlive(t, harness)
		capture.releaseSessionEnd()
		assertDirectHookPaneDead(t, harness, true)
		waitForDirectTUIProcessGone(t, harness.process)
		assertNoStopForDirectHookTurn(t, capture.snapshot(), interrupted)
	})

	t.Run("idle control-c exit", func(t *testing.T) {
		capture := startDirectHookCaptureHoldingSessionEnd(t)
		harness := startDirectHookHarness(t, lock.BinaryPath, capture)
		root := submitDirectHookPrompt(t, capture, harness, "idle-exit-prime", 1)
		waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
			return strings.Contains(screen, "synthetic-line-079") && strings.Contains(screen, "Ask Codex to do anything") && strings.Contains(screen, "Ready")
		}, "fully rendered idle turn before Ctrl-C")
		setDirectHookRemainOnExit(t, harness)
		sendNamedTUIKey(t, harness.socket, "hooks", "C-c")
		waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
			return directHookEndedThreads(facts, harness)[root.ThreadID]
		})
		assertDirectHookProcessLive(t, harness)
		assertDirectHookPaneAlive(t, harness)
		capture.releaseSessionEnd()
		assertDirectHookPaneDead(t, harness, true)
		waitForDirectTUIProcessGone(t, harness.process)
	})

	t.Run("killed work", func(t *testing.T) {
		capture := startDirectHookCapture(t)
		held := startDirectHookHeldProvider(t)
		harness := startDirectHookHarnessWithProvider(t, lock.BinaryPath, held.provider, capture, false)
		working := startDirectHookTurn(t, capture, harness, "hold-kill-probe", 1)
		waitForDirectHookSignal(t, held.accepted, "held provider acceptance before kill")
		setDirectHookRemainOnExit(t, harness)
		assertDirectHookProcessLive(t, harness)
		if err := syscall.Kill(harness.process.pid, syscall.SIGKILL); err != nil {
			t.Fatalf("SIGKILL exact test-owned Codex PID: %v", err)
		}
		waitForDirectHookSignal(t, held.canceled, "provider cancellation after exact PID kill")
		assertDirectHookPaneDead(t, harness, false)
		waitForDirectTUIProcessGone(t, harness.process)
		time.Sleep(250 * time.Millisecond)
		facts := capture.snapshot()
		assertNoStopForDirectHookTurn(t, facts, working)
		for _, fact := range facts {
			if fact.PID == harness.process.pid && fact.ThreadID == working.ThreadID && fact.Hook == "SessionEnd" {
				t.Fatal("SIGKILL emitted a graceful SessionEnd hook")
			}
		}
	})
}

type directHookHeldProvider struct {
	provider *tuiBehaviorProvider
	accepted chan struct{}
	canceled chan struct{}
	release  chan struct{}
	finished chan struct{}
	once     sync.Once
}

func startDirectHookHeldProvider(t *testing.T) directHookHeldProvider {
	t.Helper()
	accepted := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	release := make(chan struct{})
	finished := make(chan struct{}, 1)
	provider := startTUIBehaviorProviderWithResponder(t, func(writer http.ResponseWriter, request *http.Request, body []byte, sequence int64) {
		if sequence != 1 || (!bytes.Contains(body, []byte("hold-active-probe")) && !bytes.Contains(body, []byte("hold-kill-probe"))) {
			writeTUIBehaviorAssistant(writer, sequence, false)
			return
		}
		select {
		case accepted <- struct{}{}:
		default:
		}
		identifier := fmt.Sprintf("resp-%d", sequence)
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(writer, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":%q}}\n\n", identifier) // justify-ignore-error: cancellation is the asserted outcome of this held live response.
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-request.Context().Done():
			select {
			case canceled <- struct{}{}:
			default:
			}
		case <-release:
		}
		select {
		case finished <- struct{}{}:
		default:
		}
	})
	return directHookHeldProvider{provider: provider, accepted: accepted, canceled: canceled, release: release, finished: finished}
}

func (held *directHookHeldProvider) releaseResponse() {
	held.once.Do(func() { close(held.release) })
}

func startDirectHookTurn(t *testing.T, capture *directHookCapture, harness directHookHarness, prompt string, providerCount int64) directHookTurn {
	t.Helper()
	before := make(map[string]bool)
	for _, fact := range capture.snapshot() {
		if fact.Hook == "UserPromptSubmit" {
			before[fact.TurnID] = true
		}
	}
	sendDirectHookInput(t, harness, prompt)
	waitForTUIProviderRequests(t, harness.provider, providerCount)
	facts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
		for _, fact := range facts {
			if fact.PID == harness.process.pid && fact.Hook == "UserPromptSubmit" && fact.Objective == prompt && !before[fact.TurnID] {
				return true
			}
		}
		return false
	})
	for submittedIndex, submitted := range facts {
		if submitted.PID != harness.process.pid || submitted.Hook != "UserPromptSubmit" || submitted.Objective != prompt || before[submitted.TurnID] {
			continue
		}
		for startedIndex, started := range facts {
			if started.PID == harness.process.pid && started.Hook == "SessionStart" && started.ThreadID == submitted.ThreadID {
				if startedIndex >= submittedIndex || started.RuntimeID != harness.runtimeID || started.StartTime != harness.process.startTime || started.TTY != harness.process.tty {
					t.Fatal("held turn did not preserve SessionStart before UserPromptSubmit on the registered process")
				}
				return directHookTurn{ThreadID: submitted.ThreadID, SessionID: submitted.SessionID, TurnID: submitted.TurnID, Source: started.SessionSource}
			}
		}
	}
	t.Fatal("held turn omitted its root SessionStart identity")
	return directHookTurn{}
}

func waitForDirectHookSignal(t *testing.T, signal <-chan struct{}, behavior string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(directTUITimeout):
		t.Fatalf("live provider did not expose %s", behavior)
	}
}

func assertNoStopForDirectHookTurn(t *testing.T, facts []capturedHookFact, turn directHookTurn) {
	t.Helper()
	for _, fact := range facts {
		if fact.Hook == "Stop" && fact.ThreadID == turn.ThreadID && fact.TurnID == turn.TurnID {
			t.Fatal("interrupted or killed turn emitted a Stop hook")
		}
	}
}

func directHookEndedThreads(facts []capturedHookFact, harness directHookHarness) map[string]bool {
	ended := make(map[string]bool)
	for _, fact := range facts {
		if fact.PID == harness.process.pid && fact.RuntimeID == harness.runtimeID && fact.StartTime == harness.process.startTime && fact.TTY == harness.process.tty && fact.Hook == "SessionEnd" && fact.Projection == "SessionEnded" && fact.ThreadID == fact.SessionID {
			ended[fact.ThreadID] = true
		}
	}
	return ended
}

func assertGracefulDirectHookEndSet(t *testing.T, facts []capturedHookFact, harness directHookHarness, confirmedRoot string) {
	t.Helper()
	started := make(map[string]bool)
	for _, fact := range facts {
		if fact.PID == harness.process.pid && fact.Hook == "SessionStart" {
			started[fact.ThreadID] = true
		}
	}
	ended := directHookEndedThreads(facts, harness)
	if !ended[confirmedRoot] {
		t.Fatal("graceful shutdown omitted the confirmed loaded root")
	}
	emptyRoot := false
	for threadID := range ended {
		if threadID != confirmedRoot && !started[threadID] {
			emptyRoot = true
		}
	}
	if !emptyRoot {
		t.Fatal("graceful shutdown omitted the loaded empty replacement root without SessionStart")
	}
}

func setDirectHookRemainOnExit(t *testing.T, harness directHookHarness) {
	t.Helper()
	if output, err := exec.Command("tmux", "-L", harness.socket, "set-option", "-p", "-t", "hooks:0.0", "remain-on-exit", "on").CombinedOutput(); err != nil {
		t.Fatalf("set test-owned pane remain-on-exit: %v: %s", err, boundedDirectTUIError(output))
	}
}

func assertDirectHookProcessLive(t *testing.T, harness directHookHarness) {
	t.Helper()
	if !directTUIProcessAlive(harness.process) {
		t.Fatal("exact pinned Codex PID/start-time is not live")
	}
}

func assertDirectHookPaneAlive(t *testing.T, harness directHookHarness) {
	t.Helper()
	if dead := readNamedTmuxFormat(t, harness.socket, "hooks", "#{pane_dead}"); dead != "0" {
		t.Fatalf("test-owned pane_dead = %q, want 0", dead)
	}
}

func assertDirectHookPaneDead(t *testing.T, harness directHookHarness, clean bool) {
	t.Helper()
	waitForNamedTmuxFormat(t, harness.socket, "hooks", "#{pane_dead}", "1", "test-owned pane death")
	status := readNamedTmuxFormat(t, harness.socket, "hooks", "#{pane_dead_status}")
	signal := readNamedTmuxFormat(t, harness.socket, "hooks", "#{pane_dead_signal}")
	if clean && signal != "" {
		t.Fatalf("clean TUI pane exit signal = %q, want none (status %q)", signal, status)
	}
	if !clean && status == "0" && signal == "" {
		t.Fatal("exact SIGKILL unexpectedly produced an explicit clean pane exit")
	}
}

func startDirectHookIdentityProvider(t *testing.T, capture *directHookCapture) *tuiBehaviorProvider {
	t.Helper()
	return startTUIBehaviorProviderWithResponder(t, func(writer http.ResponseWriter, request *http.Request, body []byte, sequence int64) {
		if bytes.Contains(body, []byte("spawn-p0-1")) {
			deadline := time.Now().Add(directTUITimeout)
			for time.Now().Before(deadline) {
				for _, fact := range capture.snapshot() {
					if fact.Hook == "SubagentStop" {
						writeTUIBehaviorAssistant(writer, sequence, false)
						return
					}
				}
				select {
				case <-request.Context().Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
			http.Error(writer, "subagent stop was not ACKed", http.StatusGatewayTimeout)
			return
		}
		if bytes.Contains(body, []byte("p0-child")) {
			writeTUIBehaviorAssistant(writer, sequence, false)
			return
		}
		if bytes.Contains(body, []byte("native-spawn-probe")) {
			identifier := fmt.Sprintf("resp-%d", sequence)
			arguments := `{"message":"p0-child","task_name":"p0-child","agent_type":"worker"}`
			writer.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(writer, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":%q}}\n\n", identifier)                                                                                                                                        // justify-ignore-error: the live test asserts the resulting subagent hook sequence.
			_, _ = fmt.Fprintf(writer, "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"spawn-p0-1\",\"namespace\":\"multi_agent_v1\",\"name\":\"spawn_agent\",\"arguments\":%q}}\n\n", arguments)    // justify-ignore-error: the live test asserts the resulting subagent hook sequence.
			_, _ = fmt.Fprintf(writer, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":%q,\"usage\":{\"input_tokens\":0,\"input_tokens_details\":null,\"output_tokens\":0,\"output_tokens_details\":null,\"total_tokens\":0}}}\n\n", identifier) // justify-ignore-error: the live test asserts the resulting subagent hook sequence.
			return
		}
		writeTUIBehaviorAssistant(writer, sequence, false)
	})
}

func assertNativeSubagentActivityOnly(t *testing.T, facts []capturedHookFact, harness directHookHarness, root directHookTurn) {
	t.Helper()
	rootPromptIndex, childStartIndex, childPromptIndex, childStopIndex, rootStopIndex := -1, -1, -1, -1, -1
	childThreadID := ""
	for index, fact := range facts {
		if fact.RuntimeID != harness.runtimeID || fact.PID != harness.process.pid || fact.StartTime != harness.process.startTime || fact.TTY != harness.process.tty {
			continue
		}
		switch {
		case fact.Hook == "UserPromptSubmit" && fact.ThreadID == root.ThreadID && fact.TurnID == root.TurnID && fact.Projection == "PromptSubmitted":
			rootPromptIndex = index
		case fact.Hook == "SubagentStart" && fact.SessionID == root.SessionID && fact.ThreadID != root.ThreadID && fact.Projection == "ActivityObserved":
			childStartIndex = index
			childThreadID = fact.ThreadID
		case fact.Hook == "UserPromptSubmit" && fact.SessionID == root.SessionID && fact.ThreadID != root.ThreadID && fact.Projection == "ActivityObserved":
			childPromptIndex = index
			if childThreadID != "" && childThreadID != fact.ThreadID {
				t.Fatal("native subagent start/prompt thread identities differ")
			}
			childThreadID = fact.ThreadID
			if fact.Objective != "" {
				t.Fatal("native subagent prompt leaked an objective projection")
			}
		case fact.Hook == "SubagentStop" && fact.SessionID == root.SessionID && fact.ThreadID == root.ThreadID && fact.Projection == "ActivityObserved":
			childStopIndex = index
		case fact.Hook == "Stop" && fact.ThreadID == root.ThreadID && fact.TurnID == root.TurnID && fact.Projection == "StopObserved":
			rootStopIndex = index
		case fact.ThreadID == childThreadID && (fact.Projection == "SessionStarted" || fact.Projection == "StopObserved" || fact.Projection == "SessionEnded"):
			t.Fatal("native subagent received root lifecycle authority")
		}
	}
	if childThreadID == "" || !(rootPromptIndex >= 0 && rootPromptIndex < childStartIndex && childStartIndex < childPromptIndex && childPromptIndex < childStopIndex && childStopIndex < rootStopIndex) {
		t.Fatal("native subagent hooks did not preserve the reviewed activity-only partial order")
	}
}

func startDirectHookCapture(t *testing.T) *directHookCapture {
	return startDirectHookCaptureWithOptions(t, false)
}

func startDirectHookCaptureHoldingSessionEnd(t *testing.T) *directHookCapture {
	return startDirectHookCaptureWithOptions(t, true)
}

func startDirectHookCaptureWithOptions(t *testing.T, holdSessionEnd bool) *directHookCapture {
	t.Helper()
	isolationRoot, err := os.MkdirTemp("/tmp", "skidbladnir-hook-live-")
	if err != nil {
		t.Fatalf("create isolated hook root: %v", err)
	}
	if err := os.Chmod(isolationRoot, 0o700); err != nil {
		_ = os.RemoveAll(isolationRoot) // justify-ignore-error: cleanup targets only the just-created test directory.
		t.Fatalf("restrict isolated hook root: %v", err)
	}
	socketPath := filepath.Join(isolationRoot, "hook.sock")
	gapDirectory := filepath.Join(isolationRoot, "gaps")
	if err := os.Mkdir(gapDirectory, 0o700); err != nil {
		_ = os.RemoveAll(isolationRoot) // justify-ignore-error: cleanup targets only the just-created test directory.
		t.Fatalf("create isolated hook gap directory: %v", err)
	}
	address := &net.UnixAddr{Name: socketPath, Net: "unix"}
	listener, err := net.ListenUnix("unix", address)
	if err != nil {
		_ = os.RemoveAll(isolationRoot) // justify-ignore-error: cleanup targets only the just-created test directory.
		t.Fatalf("listen on reviewed hook socket: %v", err)
	}
	listener.SetUnlinkOnClose(true)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close()
		_ = os.RemoveAll(isolationRoot) // justify-ignore-error: cleanup targets only the just-created test directory.
		t.Fatalf("restrict reviewed hook socket: %v", err)
	}
	capture := &directHookCapture{
		listener:     listener,
		socketPath:   socketPath,
		gapDirectory: gapDirectory,
		changed:      make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
	if holdSessionEnd {
		capture.sessionEnd = make(chan struct{})
	}
	go capture.accept()
	t.Cleanup(func() {
		capture.releaseSessionEnd()
		if err := listener.Close(); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			t.Errorf("close hook capture: %v", err)
		}
		select {
		case <-capture.done:
		case <-time.After(2 * time.Second):
			t.Error("hook capture did not stop")
		}
		connectionsDone := make(chan struct{})
		go func() {
			capture.connections.Wait()
			close(connectionsDone)
		}()
		select {
		case <-connectionsDone:
		case <-time.After(2 * time.Second):
			t.Error("hook capture connections did not stop")
		}
		entries, err := os.ReadDir(gapDirectory)
		if err != nil {
			t.Errorf("read isolated hook gaps: %v", err)
		} else if len(entries) != 0 {
			t.Errorf("ACKed live hooks wrote %d isolated gap markers", len(entries))
		}
		if err := os.RemoveAll(isolationRoot); err != nil {
			t.Errorf("remove isolated hook root: %v", err)
		}
	})
	return capture
}

func (capture *directHookCapture) accept() {
	defer close(capture.done)
	for {
		connection, err := capture.listener.AcceptUnix()
		if err != nil {
			return
		}
		capture.connections.Add(1)
		go func() {
			defer capture.connections.Done()
			capture.readOne(connection)
		}()
	}
}

func (capture *directHookCapture) readOne(connection *net.UnixConn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	decoder := json.NewDecoder(bufio.NewReaderSize(connection, 16*1024+1))
	var fact capturedHookFact
	if err := decoder.Decode(&fact); err != nil {
		return
	}
	if fact.Type != "HookFact" || fact.Hook == "" || fact.Projection == "" {
		return
	}
	capture.mu.Lock()
	capture.facts = append(capture.facts, fact)
	capture.mu.Unlock()
	select {
	case capture.changed <- struct{}{}:
	default:
	}
	if fact.Hook == "SessionEnd" && capture.sessionEnd != nil {
		<-capture.sessionEnd
	}
	_ = json.NewEncoder(connection).Encode(struct {
		Type string `json:"type"`
	}{Type: "Ack"})
}

func (capture *directHookCapture) releaseSessionEnd() {
	if capture.sessionEnd != nil {
		capture.releaseEnd.Do(func() { close(capture.sessionEnd) })
	}
}

func (capture *directHookCapture) snapshot() []capturedHookFact {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]capturedHookFact(nil), capture.facts...)
}

func waitForDirectHookFacts(t *testing.T, capture *directHookCapture, ready func([]capturedHookFact) bool) []capturedHookFact {
	t.Helper()
	deadline := time.NewTimer(directTUITimeout)
	defer deadline.Stop()
	for {
		facts := capture.snapshot()
		if ready(facts) {
			return facts
		}
		select {
		case <-capture.changed:
		case <-deadline.C:
			t.Fatal("reviewed hook facts did not reach the required content-free sequence")
		}
	}
}

func submitDirectHookPrompt(t *testing.T, capture *directHookCapture, harness directHookHarness, prompt string, providerCount int64) directHookTurn {
	t.Helper()
	sendDirectHookInput(t, harness, prompt)
	waitForTUIProviderRequests(t, harness.provider, providerCount)
	facts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
		for _, submitted := range facts {
			if submitted.PID != harness.process.pid || submitted.Hook != "UserPromptSubmit" || submitted.Objective != prompt {
				continue
			}
			for _, stopped := range facts {
				if stopped.PID == harness.process.pid && stopped.Hook == "Stop" && stopped.ThreadID == submitted.ThreadID && stopped.TurnID == submitted.TurnID {
					return true
				}
			}
		}
		return false
	})
	for _, submitted := range facts {
		if submitted.PID != harness.process.pid || submitted.Hook != "UserPromptSubmit" || submitted.Objective != prompt {
			continue
		}
		for _, started := range facts {
			if started.PID == harness.process.pid && started.Hook == "SessionStart" && started.ThreadID == submitted.ThreadID {
				if started.RuntimeID != harness.runtimeID || started.StartTime != harness.process.startTime || started.TTY != harness.process.tty || started.EffectiveCWD != directTUICWD {
					t.Fatal("root rollover SessionStart lost the registered runtime tuple")
				}
				waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
					return strings.Contains(screen, "Ask Codex to do anything")
				}, "idle composer after synthetic turn")
				return directHookTurn{ThreadID: submitted.ThreadID, SessionID: submitted.SessionID, TurnID: submitted.TurnID, Source: started.SessionSource}
			}
		}
	}
	t.Fatal("root prompt completed without a matching SessionStart")
	return directHookTurn{}
}

func backtrackAndResubmitDirectHookPrompt(t *testing.T, capture *directHookCapture, harness directHookHarness, previous directHookTurn, prompt string, providerCount int64) directHookTurn {
	t.Helper()
	existingTurns := make(map[string]bool)
	for _, fact := range capture.snapshot() {
		if fact.Hook == "UserPromptSubmit" {
			existingTurns[fact.TurnID] = true
		}
	}
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "Ask Codex to do anything")
	}, "idle composer before backtrack")
	sendNamedTUIKey(t, harness.socket, "hooks", "Escape")
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "again to edit previous message")
	}, "backtrack priming hint")
	sendNamedTUIKey(t, harness.socket, "hooks", "Escape")
	waitForNamedTmuxFormat(t, harness.socket, "hooks", "#{alternate_on}", "1", "backtrack transcript overlay")
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, "T R A N S C R I P T")
	}, "backtrack transcript selection")
	sendNamedTUIHex(t, harness.socket, "hooks", "0d")
	waitForNamedTmuxFormat(t, harness.socket, "hooks", "#{alternate_on}", "0", "backtrack branch replacement")
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, prompt)
	}, "restored backtrack draft")
	sendNamedTUIKey(t, harness.socket, "hooks", "Enter")
	waitForTUIProviderRequests(t, harness.provider, providerCount)
	facts := waitForDirectHookFacts(t, capture, func(facts []capturedHookFact) bool {
		for _, submitted := range facts {
			if submitted.PID != harness.process.pid || submitted.Hook != "UserPromptSubmit" || submitted.Objective != prompt || existingTurns[submitted.TurnID] {
				continue
			}
			for _, stopped := range facts {
				if stopped.PID == harness.process.pid && stopped.Hook == "Stop" && stopped.ThreadID == submitted.ThreadID && stopped.TurnID == submitted.TurnID {
					return true
				}
			}
		}
		return false
	})
	for _, submitted := range facts {
		if submitted.PID != harness.process.pid || submitted.Hook != "UserPromptSubmit" || submitted.Objective != prompt || existingTurns[submitted.TurnID] {
			continue
		}
		for _, started := range facts {
			if started.PID == harness.process.pid && started.Hook == "SessionStart" && started.ThreadID == submitted.ThreadID && started.ThreadID != previous.ThreadID {
				waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
					return strings.Contains(screen, "Ask Codex to do anything")
				}, "idle composer after backtrack turn")
				return directHookTurn{ThreadID: submitted.ThreadID, SessionID: submitted.SessionID, TurnID: submitted.TurnID, Source: started.SessionSource}
			}
		}
	}
	t.Fatal("backtrack resubmission omitted its new root SessionStart")
	return directHookTurn{}
}

func sendDirectHookSlash(t *testing.T, harness directHookHarness, command string) {
	t.Helper()
	sendDirectHookInput(t, harness, command)
	deadline := time.Now().Add(directTUITimeout)
	for time.Now().Before(deadline) {
		if harness.process.startTime != processStartTime(harness.process.pid) {
			t.Fatal("thread-rollover slash command replaced the pinned TUI process")
		}
		screen, err := exec.Command("tmux", "-L", harness.socket, "capture-pane", "-p", "-J", "-t", "hooks:0.0").Output()
		if err == nil && strings.Contains(string(screen), "Ask Codex to do anything") && !strings.Contains(string(screen), "› "+command) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not settle to an idle composer", command)
}

func sendDirectHookInput(t *testing.T, harness directHookHarness, literal string) {
	t.Helper()
	sendNamedTUILiteral(t, harness.socket, "hooks", literal)
	witness := literal
	if fields := strings.Fields(literal); len(fields) != 0 {
		witness = fields[len(fields)-1]
	}
	waitForDirectHookTUICapture(t, harness.socket, func(screen string) bool {
		return strings.Contains(screen, witness)
	}, "synthetic input draft")
	sendNamedTUIHex(t, harness.socket, "hooks", "0d")
}

func startDirectHookHarness(t *testing.T, binary string, capture *directHookCapture) directHookHarness {
	t.Helper()
	provider := startTUIBehaviorProvider(t)
	return startDirectHookHarnessWithProvider(t, binary, provider, capture, false)
}

func startDirectHookHarnessWithProvider(t *testing.T, binary string, provider *tuiBehaviorProvider, capture *directHookCapture, nativeAgents bool) directHookHarness {
	t.Helper()
	home := t.TempDir()
	writeTrustedDirectHookProfile(t, binary, home, provider.listener.Addr().String(), capture.socketPath, capture.gapDirectory, nativeAgents)
	assertDirectHookUniverse(t, binary, home, directTUICWD)
	socket := fmt.Sprintf("skidbladnir_hooks_%d_%d", os.Getpid(), time.Now().UnixNano())
	runtimeID := fmt.Sprintf("%08x-3333-4333-8333-%012x", uint32(time.Now().UnixNano()), uint64(os.Getpid()))
	startMarker := filepath.Join(t.TempDir(), "start")
	arguments := []string{binary, "--strict-config", "--dangerously-bypass-approvals-and-sandbox", "-C", directTUICWD}
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = shellQuote(argument)
	}
	launch := "while [ ! -e " + shellQuote(startMarker) + " ]; do /usr/bin/sleep 0.01; done; exec " + strings.Join(quoted, " ")
	command := []string{
		"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "hooks",
		"-x", "100", "-y", "30", "-c", directTUICWD,
		"-e", "CODEX_HOME=" + home,
		"-e", "SKIDBLADNIR_RUNTIME_ID=" + runtimeID,
		"/bin/sh", "-c", launch,
	}
	if output, err := exec.Command("tmux", command...).CombinedOutput(); err != nil {
		t.Fatalf("start trusted-hook stock TUI: %v: %s", err, boundedDirectTUIError(output))
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: cleanup accepts an already-exited test-owned tmux server.
	})
	client := startNamedTUIBehaviorClient(t, socket, "hooks")
	if err := os.WriteFile(startMarker, nil, 0o600); err != nil {
		t.Fatalf("release trusted-hook TUI launch: %v", err)
	}
	process := awaitNamedTUIProcess(t, socket, "hooks")
	waitForDirectHookTUICapture(t, socket, func(screen string) bool {
		return strings.Contains(screen, "Ask Codex to do anything") &&
			strings.Contains(screen, "Ready") &&
			!strings.Contains(screen, "model:       loading")
	}, "model-resolved idle composer")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run() // justify-ignore-error: deterministic cleanup accepts an already-exited test-owned tmux server.
		waitForDirectTUIProcessGone(t, process)
	})
	return directHookHarness{socket: socket, runtimeID: runtimeID, process: process, provider: provider, client: client}
}

func assertDirectHookUniverse(t *testing.T, binary, home, cwd string) {
	t.Helper()
	listed := readDirectHookUniverse(t, binary, home, cwd)
	if len(listed.Data) != 1 || listed.Data[0].CWD != cwd || len(listed.Data[0].Warnings) != 0 || len(listed.Data[0].Errors) != 0 {
		t.Fatal("hook-universe probe did not return one clean exact-cwd result")
	}
	if len(listed.Data[0].Hooks) != 7 {
		t.Fatalf("runnable hook universe contains %d handlers, want exactly 7", len(listed.Data[0].Hooks))
	}
	hooksPath := filepath.Join(home, "hooks.json")
	seen := make(map[string]bool, 7)
	for _, hook := range listed.Data[0].Hooks {
		if hook.HandlerType != "command" || hook.Command == "" || hook.Async || hook.Matcher != nil || hook.SourcePath != hooksPath || hook.Source != "user" || hook.PluginID != nil || !hook.Enabled || hook.IsManaged || hook.TrustStatus != "trusted" || !strings.HasPrefix(hook.CurrentHash, "sha256:") {
			t.Fatal("hook-universe probe exposed a handler outside the reviewed trusted user shape")
		}
		if hook.TimeoutSec != 1 && hook.TimeoutSec != 5 {
			t.Fatal("hook-universe probe exposed an unreviewed timeout")
		}
		seen[hook.EventName] = true
	}
	for _, event := range []string{"sessionStart", "userPromptSubmit", "postToolUse", "subagentStart", "subagentStop", "stop", "sessionEnd"} {
		if !seen[event] {
			t.Fatalf("hook-universe probe omits reviewed event %s", event)
		}
	}
}

func readDirectHookUniverse(t *testing.T, binary, home, cwd string) directHookListResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), directTUITimeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "app-server", "--strict-config", "--stdio")
	command.Dir = cwd
	command.Env = replaceEnvironment(os.Environ(), "CODEX_HOME", home)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create hook-universe probe stdin: %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create hook-universe probe stdout: %v", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start hook-universe probe: %v", err)
	}
	finished := false
	defer func() {
		_ = stdin.Close() // justify-ignore-error: cleanup closes only the isolated read-only probe.
		if !finished && command.Process != nil {
			_ = command.Process.Kill() // justify-ignore-error: cleanup stops only the isolated read-only probe.
			_ = command.Wait()         // justify-ignore-error: cleanup accepts the killed probe status.
		}
	}()
	encoder := json.NewEncoder(stdin)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	if err := encoder.Encode(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{"clientInfo": map[string]string{
			"name": "skidbladnir_test", "title": "Skidbladnir Test", "version": "0",
		}},
	}); err != nil {
		t.Fatalf("write hook-universe initialize: %v", err)
	}
	readDirectHookRPCResult(t, scanner, 1, nil)
	if err := encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatalf("write hook-universe initialized: %v", err)
	}
	if err := encoder.Encode(map[string]any{
		"method": "hooks/list",
		"id":     2,
		"params": map[string]any{"cwds": []string{cwd}},
	}); err != nil {
		t.Fatalf("write hook-universe list request: %v", err)
	}
	var listed directHookListResponse
	readDirectHookRPCResult(t, scanner, 2, &listed)
	if err := stdin.Close(); err != nil {
		t.Fatalf("close hook-universe probe stdin: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("hook-universe probe exit: %v", err)
	}
	finished = true
	return listed
}

func readDirectHookRPCResult(t *testing.T, scanner *bufio.Scanner, id int, target any) {
	t.Helper()
	wantID := strconv.Itoa(id)
	for scanner.Scan() {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			t.Fatalf("decode hook-universe response: %v", err)
		}
		if string(envelope.ID) != wantID {
			continue
		}
		if len(envelope.Error) != 0 && string(envelope.Error) != "null" {
			t.Fatal("hook-universe probe returned a JSON-RPC error")
		}
		if target != nil {
			if err := json.Unmarshal(envelope.Result, target); err != nil {
				t.Fatalf("decode hook-universe result: %v", err)
			}
		}
		return
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read hook-universe response: %v", err)
	}
	t.Fatalf("hook-universe probe ended before response id %d", id)
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	replaced := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			replaced = append(replaced, entry)
		}
	}
	return append(replaced, prefix+value)
}

func writeTrustedDirectHookProfile(t *testing.T, binary, home, providerAddress, socketPath, gapDirectory string, nativeAgents bool) {
	t.Helper()
	root := directTUIRepositoryRoot(t)
	hooksBytes, err := os.ReadFile(filepath.Join(root, "deploy", "codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read reviewed hooks.json: %v", err)
	}
	const productionSocket = "/run/user/1000/skidbladnir/hook.sock"
	const productionGaps = "/home/niels/.local/state/skidbladnir/hook-gaps"
	if bytes.Count(hooksBytes, []byte(productionSocket)) != 7 || bytes.Count(hooksBytes, []byte(productionGaps)) != 7 {
		t.Fatal("reviewed hooks.json no longer exposes seven exact delivery destinations")
	}
	productionHooksBytes := append([]byte(nil), hooksBytes...)
	hooksBytes = bytes.ReplaceAll(hooksBytes, []byte(productionSocket), []byte(socketPath))
	hooksBytes = bytes.ReplaceAll(hooksBytes, []byte(productionGaps), []byte(gapDirectory))
	hooksPath := filepath.Join(home, "hooks.json")
	if err := os.WriteFile(hooksPath, hooksBytes, 0o600); err != nil {
		t.Fatalf("write isolated reviewed hooks.json: %v", err)
	}
	lockBytes, err := os.ReadFile(filepath.Join(root, "deploy", "codex", "hooks.lock.json"))
	if err != nil {
		t.Fatalf("read reviewed hook lock: %v", err)
	}
	var lock directHookLock
	if err := json.Unmarshal(lockBytes, &lock); err != nil || len(lock.Profiles) == 0 || len(lock.Profiles[0].TrustState) != 7 {
		t.Fatalf("decode reviewed hook lock: %v", err)
	}
	helperBytes, err := os.ReadFile(lock.Helper.Path)
	if err != nil {
		t.Fatalf("read installed reviewed helper: %v", err)
	}
	helperDigest := sha256.Sum256(helperBytes)
	if hex.EncodeToString(helperDigest[:]) != lock.Helper.SHA256 {
		t.Fatal("installed reviewed helper digest differs from hook lock")
	}
	nativeFeatures := ""
	if nativeAgents {
		nativeFeatures = "multi_agent = true\nmulti_agent_v2 = false\n"
	}
	renderConfig := func(trust string) string {
		state := ""
		if trust != "" {
			state = "\n[hooks.state]\n" + trust
		}
		return fmt.Sprintf(`model = "gpt-5.1-codex-mini"
model_provider = "skidbladnir-loopback"
approval_policy = "never"
sandbox_mode = "danger-full-access"

[features]
hooks = true
%s

[projects.%q]
trust_level = "trusted"

[tui]
status_line = ["thread-title", "run-state"]

[model_providers.skidbladnir-loopback]
name = "Skidbladnir loopback"
base_url = %q
wire_api = "responses"
requires_openai_auth = false
request_max_retries = 0
stream_max_retries = 0
supports_websockets = false
%s`, nativeFeatures, directTUICWD, "http://"+providerAddress+"/v1", state)
	}
	configPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(configPath, []byte(renderConfig("")), 0o600); err != nil {
		t.Fatalf("write isolated trusted-hook config: %v", err)
	}
	productionHashHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(productionHashHome, "hooks.json"), productionHooksBytes, 0o600); err != nil {
		t.Fatalf("write exact production hook hash fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(productionHashHome, "config.toml"), []byte(renderConfig("")), 0o600); err != nil {
		t.Fatalf("write exact production hook hash config: %v", err)
	}
	productionListed := readDirectHookUniverse(t, binary, productionHashHome, directTUICWD)
	if len(productionListed.Data) != 1 || len(productionListed.Data[0].Hooks) != 7 || len(productionListed.Data[0].Warnings) != 0 || len(productionListed.Data[0].Errors) != 0 {
		t.Fatal("exact production hook hash discovery did not expose seven clean handlers")
	}
	if err := validateDirectHookTrustLock(lock, productionListed.Data[0].Hooks); err != nil {
		t.Fatalf("production hook hashes differ from the frozen three-profile trust lock: %v", err)
	}
	listed := readDirectHookUniverse(t, binary, home, directTUICWD)
	if len(listed.Data) != 1 || len(listed.Data[0].Hooks) != 7 || len(listed.Data[0].Warnings) != 0 || len(listed.Data[0].Errors) != 0 {
		t.Fatal("isolated untrusted hook discovery did not expose exactly seven clean handlers")
	}
	hooks := append([]directHookListEntry(nil), listed.Data[0].Hooks...)
	sort.Slice(hooks, func(left, right int) bool { return hooks[left].Key < hooks[right].Key })
	var trust strings.Builder
	for _, hook := range hooks {
		if hook.SourcePath != hooksPath || hook.Source != "user" || hook.IsManaged || hook.TrustStatus != "untrusted" || !strings.HasPrefix(hook.CurrentHash, "sha256:") {
			t.Fatal("isolated hook discovery escaped the expected untrusted user source")
		}
		fmt.Fprintf(&trust, "%s = { trusted_hash = %s }\n", strconv.Quote(hook.Key), strconv.Quote(hook.CurrentHash))
	}
	if err := os.WriteFile(configPath, []byte(renderConfig(trust.String())), 0o600); err != nil {
		t.Fatalf("write isolated trusted-hook state: %v", err)
	}
}

func validateDirectHookTrustLock(lock directHookLock, hooks []directHookListEntry) error {
	if len(lock.Profiles) != 3 || len(hooks) != 7 {
		return fmt.Errorf("profile/hook count = %d/%d, want 3/7", len(lock.Profiles), len(hooks))
	}
	expected := make(map[string]bool, 7)
	for profileIndex, profile := range lock.Profiles {
		if len(profile.TrustState) != 7 {
			return fmt.Errorf("profile %d trust count = %d, want 7", profileIndex, len(profile.TrustState))
		}
		profileHashes := make(map[string]bool, 7)
		for _, hash := range profile.TrustState {
			profileHashes[hash] = true
		}
		if len(profileHashes) != 7 {
			return fmt.Errorf("profile %d contains duplicate trust hashes", profileIndex)
		}
		if profileIndex == 0 {
			expected = profileHashes
			continue
		}
		if !sameStringSet(expected, profileHashes) {
			return fmt.Errorf("profile %d trust hashes differ", profileIndex)
		}
	}
	discovered := make(map[string]bool, 7)
	for _, hook := range hooks {
		discovered[hook.CurrentHash] = true
	}
	if len(discovered) != 7 || !sameStringSet(expected, discovered) {
		return fmt.Errorf("discovered trust hash set differs")
	}
	return nil
}

func sameStringSet(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if !right[value] {
			return false
		}
	}
	return true
}

func directTUIRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate direct hook live test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func hookNamesForPID(facts []capturedHookFact, pid int) map[string]bool {
	names := make(map[string]bool)
	for _, fact := range facts {
		if fact.PID == pid {
			names[fact.Hook] = true
		}
	}
	return names
}

func assertRootHookFacts(t *testing.T, facts []capturedHookFact, harness directHookHarness) {
	t.Helper()
	var threadID, sessionID string
	rootCount := 0
	for _, fact := range facts {
		if fact.PID != harness.process.pid {
			continue
		}
		rootCount++
		if fact.RuntimeID != harness.runtimeID || fact.StartTime != harness.process.startTime || fact.TTY != harness.process.tty {
			t.Fatal("root hook fact does not authenticate to the registered runtime tuple")
		}
		if !runtimeIDPatternForLive.MatchString(fact.ThreadID) || fact.SessionID == "" {
			t.Fatal("root hook fact omits canonical thread/session identity")
		}
		if threadID == "" {
			threadID, sessionID = fact.ThreadID, fact.SessionID
		} else if fact.ThreadID != threadID || fact.SessionID != sessionID {
			t.Fatal("root hook facts changed thread/session identity during one turn")
		}
		if fact.Hook == "SessionStart" && (fact.Projection != "SessionStarted" || fact.EffectiveCWD != directTUICWD || fact.SessionSource != "startup" || fact.Model == "") {
			t.Fatal("SessionStart did not retain the reviewed binding projection")
		}
		if fact.Hook == "UserPromptSubmit" && (fact.Projection != "PromptSubmitted" || fact.TurnID == "" || fact.Objective != "origin-probe") {
			t.Fatal("UserPromptSubmit did not retain the bounded synthetic objective projection")
		}
		if fact.Hook == "Stop" && (fact.Projection != "StopObserved" || fact.TurnID == "") {
			t.Fatal("Stop did not retain the reviewed completion-candidate projection")
		}
	}
	if rootCount < 3 {
		t.Fatal("root hook sequence is incomplete")
	}
}

func assertNestedHookFactsAreUnauthoritative(t *testing.T, facts []capturedHookFact, harness directHookHarness) {
	t.Helper()
	nestedByPID := make(map[int]map[string]bool)
	for _, fact := range facts {
		if fact.RuntimeID != harness.runtimeID || fact.PID == harness.process.pid {
			continue
		}
		if fact.PID <= 0 || fact.StartTime == 0 || fact.TTY == "" {
			t.Fatal("nested hook fact omits its nearest exact pinned-Codex identity")
		}
		if nestedByPID[fact.PID] == nil {
			nestedByPID[fact.PID] = make(map[string]bool)
		}
		nestedByPID[fact.PID][fact.Hook] = true
	}
	for pid, names := range nestedByPID {
		if names["SessionStart"] && names["UserPromptSubmit"] && names["Stop"] && names["SessionEnd"] {
			if pid == harness.process.pid {
				t.Fatal("nested hook traffic collided with the registered root PID")
			}
			return
		}
	}
	t.Fatal("nested CLI did not emit a distinct unauthoritative hook sequence")
}

var runtimeIDPatternForLive = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func waitForDirectHookTUICapture(t *testing.T, socket string, condition func(string) bool, behavior string) {
	t.Helper()
	deadline := time.Now().Add(directTUITimeout)
	last := ""
	for time.Now().Before(deadline) {
		output, err := exec.Command("tmux", "-L", socket, "capture-pane", "-p", "-J", "-t", "hooks:0.0").Output()
		if err == nil {
			last = string(output)
			if condition(last) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("trusted-hook stock TUI did not expose %s before the live deadline; bounded synthetic screen: %q", behavior, boundedDirectTUIError([]byte(last)))
}
