//go:build live

package live

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/runtime/appserver"
)

const liveReviewedHookCommand = "/home/niels/.local/bin/skidbladnir-hook --pinned-codex /home/niels/.local/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /home/niels/.local/state/skidbladnir/hook-gaps"

type liveHooksLock struct {
	CodexVersion string `json:"codexVersion"`
	HooksSHA256  string `json:"hooksSha256"`
}

type liveReviewedHook struct {
	keyLabel    string
	hash        string
	timeoutSec  uint64
	configEvent string
}

var liveReviewedHooks = map[appserver.HookEvent]liveReviewedHook{
	appserver.HookEventSessionStart:     {keyLabel: "session_start", hash: "sha256:3b0f379949cf7a4fc934e108351d3a6ac7cc3cca55859f1001bcf3170308ec0a", timeoutSec: 5, configEvent: "SessionStart"},
	appserver.HookEventUserPromptSubmit: {keyLabel: "user_prompt_submit", hash: "sha256:ac2aaff534d83744941500927ed45277506e74bf3fdebea0459a7a445a1f54e2", timeoutSec: 5, configEvent: "UserPromptSubmit"},
	appserver.HookEventPostToolUse:      {keyLabel: "post_tool_use", hash: "sha256:34df58c6aa6ff5db26dc6bb788c6de2de980a7ca177c1f8099b9db87e84aebc1", timeoutSec: 5, configEvent: "PostToolUse"},
	appserver.HookEventSubagentStart:    {keyLabel: "subagent_start", hash: "sha256:cb1f5da901d975d405346b5affb9c76324328f2a36a93855490cc3baa2878ad9", timeoutSec: 5, configEvent: "SubagentStart"},
	appserver.HookEventSubagentStop:     {keyLabel: "subagent_stop", hash: "sha256:79ad6e8e0e9f5d0c1cdcffb3001d26d8fa315fff0e5e1789d822342f4773c391", timeoutSec: 5, configEvent: "SubagentStop"},
	appserver.HookEventStop:             {keyLabel: "stop", hash: "sha256:98227f57d2ee854732044e21a34e399f522551e10686b8ac88a1c1368a552dba", timeoutSec: 5, configEvent: "Stop"},
	appserver.HookEventSessionEnd:       {keyLabel: "session_end", hash: "sha256:6a15c5edd98abc2b5dbba6a2566dfc31d8bce3e501f6e1d306cc39c49c9cf821", timeoutSec: 1, configEvent: "SessionEnd"},
}

func TestLivePinnedHooksTrustDiscovery(t *testing.T) {
	runLiveHooksTrustProbe(t)
}

func runLiveHooksTrustProbe(t *testing.T) {
	t.Helper()
	root := repositoryRoot(t)
	codex := readCodexLock(t, root)
	hooksBytes := readReviewedHooksConfig(t, root)

	var lock liveHooksLock
	lockBytes, err := os.ReadFile(filepath.Join(root, "deploy/codex/hooks.lock.json"))
	if err != nil {
		t.Fatalf("read hook lock: %v", err)
	}
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatalf("decode hook lock: %v", err)
	}
	digest := sha256.Sum256(hooksBytes)
	if lock.CodexVersion != livePin || lock.CodexVersion != codex.Version || lock.HooksSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("hook lock does not bind the exact pin and reviewed config bytes")
	}

	home, err := os.MkdirTemp("", "skid-hooks-")
	if err != nil {
		t.Fatalf("create short test Codex home: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(home); err != nil {
			t.Errorf("remove test Codex home: %v", err)
		}
	})
	cwd := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatalf("restrict test Codex home: %v", err)
	}
	hooksPath := filepath.Join(home, "hooks.json")
	writeLiveFixture(t, hooksPath, hooksBytes)
	writeLiveFixture(t, filepath.Join(home, "config.toml"), []byte("[features]\nhooks = true\n"))

	profile := liveProfile{Name: "hook-untrusted", Home: home}
	untrustedServer := startAppServer(t, codex.BinaryPath, profile)
	untrusted := listLiveHooks(t, untrustedServer.path, home, cwd)
	assertLiveReviewedHooks(t, untrusted, hooksPath, appserver.HookTrustUntrusted)
	stopAppServer(t, untrustedServer)

	writeLiveFixture(t, filepath.Join(home, "config.toml"), trustedHookConfig(hooksPath))
	profile.Name = "hook-trusted"
	trustedServer := startAppServer(t, codex.BinaryPath, profile)
	trusted := listLiveHooks(t, trustedServer.path, home, cwd)
	assertLiveReviewedHooks(t, trusted, hooksPath, appserver.HookTrustTrusted)

	t.Logf("codex=%s cwd_count=1 reviewed_hooks=%d absent_trust=%s persisted_trust=%s source=user managed=false", codex.Version, len(trusted.Hooks), appserver.HookTrustUntrusted, appserver.HookTrustTrusted)
}

func readReviewedHooksConfig(t *testing.T, root string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, "deploy/codex/hooks.json"))
	if err != nil {
		t.Fatalf("read reviewed hooks config: %v", err)
	}
	return content
}

func listLiveHooks(t *testing.T, socket, home, cwd string) appserver.HookSnapshot {
	t.Helper()
	var snapshot appserver.HookSnapshot
	if err := withAppServer(socket, func(ctx context.Context, connection appserver.Connection) error {
		var err error
		snapshot, err = appserver.ListHooks(ctx, connection, home, cwd)
		return err
	}); err != nil {
		t.Fatalf("list exact-pin hooks: %v", err)
	}
	return snapshot
}

func assertLiveReviewedHooks(t *testing.T, snapshot appserver.HookSnapshot, sourcePath string, trust appserver.HookTrust) {
	t.Helper()
	if snapshot.CWD == "" || len(snapshot.Hooks) != len(liveReviewedHooks) {
		t.Fatalf("hook projection has cwd=%t count=%d, want one cwd and %d hooks", snapshot.CWD != "", len(snapshot.Hooks), len(liveReviewedHooks))
	}
	seen := make(map[appserver.HookEvent]bool, len(liveReviewedHooks))
	for _, hook := range snapshot.Hooks {
		expected, ok := liveReviewedHooks[hook.Event]
		if !ok || seen[hook.Event] {
			t.Fatalf("hook projection contains foreign or duplicate event %q", hook.Event)
		}
		seen[hook.Event] = true
		expectedKey := sourcePath + ":" + expected.keyLabel + ":0:0"
		if hook.Handler != appserver.HookHandlerCommand || hook.Command != liveReviewedHookCommand || hook.Async || hook.Key != expectedKey || hook.SourcePath != sourcePath || hook.Source != appserver.HookSourceUser || !hook.Enabled || hook.Managed || hook.CurrentHash != expected.hash || hook.Trust != trust || hook.Matcher != nil || hook.TimeoutSec != expected.timeoutSec {
			t.Fatalf("event %q differs from the exact reviewed %s projection", hook.Event, trust)
		}
	}
}

func trustedHookConfig(sourcePath string) []byte {
	events := make([]liveReviewedHook, 0, len(liveReviewedHooks))
	for _, event := range liveReviewedHooks {
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].configEvent < events[j].configEvent })
	var config strings.Builder
	config.WriteString("[features]\nhooks = true\n")
	for _, event := range events {
		key := sourcePath + ":" + event.keyLabel + ":0:0"
		fmt.Fprintf(&config, "\n[hooks.state.%s]\ntrusted_hash = %s\n", strconv.Quote(key), strconv.Quote(event.hash))
	}
	return []byte(config.String())
}

func writeLiveFixture(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test-owned live fixture: %v", err)
	}
}
