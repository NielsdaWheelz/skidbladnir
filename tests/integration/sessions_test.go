//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/creack/pty"
)

const (
	sleepPath                  = "/bin/sleep"
	yoloFlag                   = "--dangerously-bypass-approvals-and-sandbox"
	tmuxConvergenceTimeout     = 5 * time.Second
	activityConvergenceTimeout = 15 * time.Second
	tmuxCleanupTimeout         = 3 * time.Second

	// justify-polling: tmux and launched processes expose state only through
	// their external query boundaries; 25 ms keeps the bounded integration
	// proof responsive without assuming synchronous process publication.
	tmuxConvergencePollInterval = 25 * time.Millisecond
	defaultSocketHelperVariable = "SKIDBLADNIR_DEFAULT_TMUX_HELPER"
	defaultSocketRootVariable   = "SKIDBLADNIR_DEFAULT_TMUX_ROOT"
	defaultSocketNonceVariable  = "SKIDBLADNIR_DEFAULT_TMUX_NONCE"
)

func TestUnavailableProfileCommandDoesNotPreventInventory(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal("create service home")
	}
	catalogue := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(catalogue, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatal("write catalogue")
	}
	_, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    randomTmuxSocketName(t, "skid-unavailable"),
		Home:          home,
		CataloguePath: catalogue,
		Profiles: []agentruntime.Profile{{
			Key:                  "future",
			Label:                "Future provider",
			Provider:             agentruntime.ProviderCodex,
			Command:              filepath.Join(root, "not-installed"),
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: filepath.Join(root, "future-home")}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "future-agent"}},
		}},
	})
	if err != nil {
		t.Fatal("an unavailable opaque profile command took down inventory")
	}
}

func TestDefaultSocketInvocationPreservesExplicitRootAndUserConfiguration(t *testing.T) {
	if os.Getenv(defaultSocketHelperVariable) == "1" {
		runDefaultSocketHelper(t)
		return
	}

	root, err := os.MkdirTemp(testSocketRoot(), "skid-default-socket-helper-")
	if err != nil {
		t.Fatalf("create default-socket subprocess root: %v", err)
	}
	if !pathStrictlyInside(testSocketRoot(), root) {
		t.Fatalf("default-socket subprocess root escapes the private integration root: %q", root)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("secure default-socket subprocess root: %v", err)
	}
	t.Cleanup(func() {
		if err := removeExactDefaultSocketHelperFile(filepath.Join(root, ".default-socket-capability")); err != nil {
			t.Errorf("remove default-socket parent capability: %v", err)
			return
		}
		if err := removeExactPrivateDirectory(root); err != nil {
			t.Errorf("remove default-socket parent root: %v", err)
		}
	})
	nonce := randomTmuxSocketName(t, "cap")
	capabilityPath := filepath.Join(root, ".default-socket-capability")
	if err := os.WriteFile(capabilityPath, []byte(nonce+"\n"), 0o600); err != nil {
		t.Fatalf("write default-socket subprocess capability: %v", err)
	}
	command := exec.Command(
		os.Args[0],
		"-test.run=^TestDefaultSocketInvocationPreservesExplicitRootAndUserConfiguration$",
		"-test.count=1",
		"-skidbladnir-isolated-tmux-capability="+isolatedTmuxCLICapabilityV1,
		"-skidbladnir-tmux-path="+tmuxPath,
	)
	command.Env = append(withoutEnvironment(os.Environ(), "HOME", "TMUX", "TMUX_PANE", "TMUX_TMPDIR",
		defaultSocketHelperVariable, defaultSocketRootVariable, defaultSocketNonceVariable),
		defaultSocketHelperVariable+"=1",
		defaultSocketRootVariable+"="+root,
		defaultSocketNonceVariable+"="+nonce,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("default-socket subprocess proof failed: output=%q error=%v", output, err)
	}
}

func cleanupDefaultSocketHelper(t *testing.T, root, socketPath string) {
	t.Helper()
	if err := cleanupRegisteredTmuxSocket(tmuxPath, socketPath); err != nil {
		t.Errorf("clean default-socket subprocess server: %v", err)
		return
	}
	for _, path := range []string{
		filepath.Join(root, ".default-socket-capability"),
		filepath.Join(root, "home", ".tmux.conf"),
		filepath.Join(root, "catalogue.json"),
		filepath.Join(root, "agent"),
		filepath.Join(root, "guarded-tmux"),
	} {
		if err := removeExactDefaultSocketHelperFile(path); err != nil {
			t.Errorf("remove default-socket subprocess file: %v", err)
			return
		}
	}
	for _, path := range []string{
		filepath.Join(root, "home"),
		filepath.Join(root, "tmux"),
		filepath.Join(root, "profile"),
		filepath.Join(root, "project"),
		root,
	} {
		if err := removeExactPrivateDirectory(path); err != nil {
			t.Errorf("remove empty default-socket subprocess directory %s: %v", path, err)
			return
		}
	}
}

func removeExactDefaultSocketHelperFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to remove non-file helper path: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func runDefaultSocketHelper(t *testing.T) {
	capabilityRoot := os.Getenv(defaultSocketRootVariable)
	nonce := os.Getenv(defaultSocketNonceVariable)
	if !filepath.IsAbs(capabilityRoot) || filepath.Clean(capabilityRoot) != capabilityRoot {
		t.Fatal("default-socket subprocess has no exact private root")
	}
	rootInfo, err := os.Stat(capabilityRoot)
	if err != nil {
		t.Fatalf("stat default-socket subprocess root: path=%q error=%v", capabilityRoot, err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("default-socket subprocess root is not private: path=%q mode=%v", capabilityRoot, rootInfo.Mode())
	}
	capabilityPath := filepath.Join(capabilityRoot, ".default-socket-capability")
	capabilityInfo, err := os.Stat(capabilityPath)
	if err != nil {
		t.Fatalf("stat default-socket subprocess capability: path=%q error=%v", capabilityPath, err)
	}
	if !capabilityInfo.Mode().IsRegular() || capabilityInfo.Mode().Perm() != 0o600 {
		t.Fatalf("default-socket subprocess capability is invalid: path=%q mode=%v", capabilityPath, capabilityInfo.Mode())
	}
	capability, err := os.ReadFile(capabilityPath)
	if err != nil || string(capability) != nonce+"\n" || !strings.HasPrefix(nonce, "cap-") || len(nonce) != len("cap-")+32 {
		t.Fatalf("default-socket subprocess capability mismatch: error=%v", err)
	}
	root, err := os.MkdirTemp(testSocketRoot(), "skid-default-socket-runtime-")
	if err != nil {
		t.Fatalf("create default-socket runtime root: %v", err)
	}
	if !pathStrictlyInside(testSocketRoot(), root) {
		t.Fatalf("default-socket runtime root escapes the child integration root: %q", root)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("secure default-socket runtime root: %v", err)
	}
	home := filepath.Join(root, "home")
	tmuxRoot := filepath.Join(root, "tmux")
	profileHome := filepath.Join(root, "profile")
	project := filepath.Join(root, "project")
	for _, path := range []string{home, tmuxRoot, profileHome, project} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("create default-socket fixture directory: path=%s error=%v", path, err)
		}
	}
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("select explicit default-socket home: %v", err)
	}
	if err := os.Setenv(defaultSocketRootVariable, root); err != nil {
		t.Fatalf("select explicit default-socket runtime capability: %v", err)
	}
	if err := os.Setenv("TMUX_TMPDIR", tmuxRoot); err != nil {
		t.Fatalf("select explicit default-socket root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".tmux.conf"), []byte("set-option -g @skid_test_config loaded\n"), 0o600); err != nil {
		t.Fatalf("write isolated user tmux config: %v", err)
	}
	catalogue := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(catalogue, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatalf("write default-socket catalogue fixture: %v", err)
	}
	agent := filepath.Join(root, "agent")
	agentScript := fmt.Sprintf("#!/bin/sh\nexec %s 300\n", shellQuote(sleepPath))
	if err := os.WriteFile(agent, []byte(agentScript), 0o700); err != nil {
		t.Fatalf("write default-socket agent fixture: %v", err)
	}
	socketPath := filepath.Join(tmuxRoot, "owned-default.sock")
	registerTestOwnedSocketPath(t, socketPath)
	t.Cleanup(func() {
		cleanupDefaultSocketHelper(t, root, socketPath)
	})
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated default tmux socket unexpectedly exists before test: path=%s error=%v", socketPath, err)
	}
	guardedTmux := filepath.Join(root, "guarded-tmux")
	guardScript := fmt.Sprintf(`#!/bin/sh
set -eu
expected_root="${SKIDBLADNIR_DEFAULT_TMUX_ROOT:?}/tmux"
[ "${TMUX_TMPDIR-}" = "$expected_root" ] || exit 97
for argument in "$@"; do
  case "$argument" in
    -L*|-S*|-f*) exit 98 ;;
  esac
done
exec %s -S "$expected_root/owned-default.sock" "$@"
`, shellQuote(tmuxPath))
	if err := os.WriteFile(guardedTmux, []byte(guardScript), 0o700); err != nil {
		t.Fatalf("write guarded default-socket tmux fixture: %v", err)
	}

	manager, err := sessions.New(sessions.Config{
		TmuxPath:      guardedTmux,
		Home:          home,
		CataloguePath: catalogue,
		Profiles: []agentruntime.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Provider:             agentruntime.ProviderCodex,
			Command:              agent,
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHome}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
			Arguments:            []string{yoloFlag},
		}},
	})
	if err != nil {
		t.Fatalf("construct default-socket manager: %v", err)
	}
	if _, err := manager.Create(context.Background(), sessions.CreateInput{CWD: project, Profile: "personal"}); err != nil {
		t.Fatalf("create through ordinary default tmux invocation: %v", err)
	}
	serverIdentity := captureTestTmuxServer(t, tmuxPath, socketPath)
	t.Cleanup(func() {
		stopTestTmuxServer(t, tmuxPath, socketPath, serverIdentity)
	})
	output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath, "show-options", "-gv", "@skid_test_config").CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "loaded" {
		t.Fatalf("default tmux did not load the user's config: output=%q error=%v", output, err)
	}
}

func TestSessionManagerAgainstRealTmux(t *testing.T) {
	t.Parallel()

	if output, err := isolatedTmuxCommand(tmuxPath, "-V").CombinedOutput(); err != nil || !strings.HasPrefix(strings.TrimSpace(string(output)), "tmux ") {
		t.Fatalf("configured tmux version is unavailable or noncanonical: command_ok=%t output=%q", err == nil, output)
	}

	fixture := newSessionFixture(t)
	ctx := context.Background()
	const fixtureServerEpoch = "v1-00000000000000000000000000000000"
	fixture.tmux(t, "set-option", "-s", "--", tmuxclient.ServerEpochOption, fixtureServerEpoch)

	t.Run("inventory preserves valid duplicates and balances new characters", func(t *testing.T) {
		const duplicateKey = "norse.durinn"
		for _, name := range []string{
			"valid-duplicate-a", "valid-duplicate-b",
			"unassigned-a", "unassigned-b", "unassigned-c", "unassigned-d", "unassigned-e",
		} {
			fixture.tmux(t, "new-session", "-d", "-s", name, "-c", fixture.project, "--", sleepPath, "300")
		}
		duplicateIDs := []string{
			fixture.tmux(t, "display-message", "-p", "-t", "valid-duplicate-a", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "valid-duplicate-b", "#{session_id}"),
		}
		for _, id := range duplicateIDs {
			fixture.tmux(t, "set-option", "-t", id, "--", "@skid_character", duplicateKey)
		}
		unassignedIDs := []string{
			fixture.tmux(t, "display-message", "-p", "-t", "skid-test-bootstrap", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "unassigned-a", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "unassigned-b", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "unassigned-c", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "unassigned-d", "#{session_id}"),
			fixture.tmux(t, "display-message", "-p", "-t", "unassigned-e", "#{session_id}"),
		}

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list controlled character allocation: %v", err)
		}
		if len(listed.Sessions) != len(duplicateIDs)+len(unassignedIDs) {
			t.Fatalf("controlled inventory size = %d, want %d: %+v", len(listed.Sessions), len(duplicateIDs)+len(unassignedIDs), listed.Sessions)
		}
		for _, id := range duplicateIDs {
			observed := requireSessionID(t, listed, id)
			if observed.Character.Key != duplicateKey {
				t.Fatalf("valid external duplicate changed: id=%s session=%+v", id, observed)
			}
			if persisted := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_character"); persisted != duplicateKey {
				t.Fatalf("valid external duplicate was not preserved byte-for-byte: id=%s persisted=%q", id, persisted)
			}
		}
		for _, id := range unassignedIDs {
			observed := requireSessionID(t, listed, id)
			requireValidCharacter(t, observed)
			if persisted := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_character"); persisted != observed.Character.Key {
				t.Fatalf("balanced character assignment was not persisted: session=%+v persisted=%q", observed, persisted)
			}
		}
		usage := map[string]int{"norse.modsognir": 0, duplicateKey: 0}
		for _, session := range listed.Sessions {
			usage[session.Character.Key]++
		}
		if usage["norse.modsognir"] != usage[duplicateKey] {
			t.Fatalf("controlled allocation is not balanced after sequential commits: %+v", usage)
		}
	})

	t.Run("invalid starts mutate nothing", func(t *testing.T) {
		before, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list sessions before invalid starts")
		}

		file := filepath.Join(fixture.root, "not-a-directory")
		if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
			t.Fatal("write non-directory cwd fixture")
		}
		unsearchable := filepath.Join(fixture.root, "unsearchable")
		if err := os.Mkdir(unsearchable, 0o600); err != nil {
			t.Fatal("create unsearchable cwd fixture")
		}
		t.Cleanup(func() {
			if err := os.Chmod(unsearchable, 0o700); err != nil {
				t.Error("restore test-owned directory permissions")
			}
		})

		cases := []struct {
			name  string
			input sessions.CreateInput
			code  sessions.ErrorCode
		}{
			{name: "relative cwd", input: sessions.CreateInput{CWD: "relative", Profile: "personal"}, code: sessions.ErrorWorkingDirectoryInvalid},
			{name: "invalid UTF-8 cwd", input: sessions.CreateInput{CWD: string([]byte{0xff}), Profile: "personal"}, code: sessions.ErrorWorkingDirectoryInvalid},
			{name: "control in cwd", input: sessions.CreateInput{CWD: fixture.root + "\nelsewhere", Profile: "personal"}, code: sessions.ErrorWorkingDirectoryInvalid},
			{name: "oversized cwd", input: sessions.CreateInput{CWD: "/" + strings.Repeat("a", 4096), Profile: "personal"}, code: sessions.ErrorWorkingDirectoryInvalid},
			{name: "missing cwd", input: sessions.CreateInput{CWD: filepath.Join(fixture.root, "missing"), Profile: "personal"}, code: sessions.ErrorWorkingDirectoryUnavailable},
			{name: "file cwd", input: sessions.CreateInput{CWD: file, Profile: "personal"}, code: sessions.ErrorWorkingDirectoryUnavailable},
			{name: "unsearchable cwd", input: sessions.CreateInput{CWD: unsearchable, Profile: "personal"}, code: sessions.ErrorWorkingDirectoryUnavailable},
			{name: "unknown profile", input: sessions.CreateInput{CWD: fixture.project, Profile: "other"}, code: sessions.ErrorProfileUnknown},
			{name: "unsafe name", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "unsafe:name"}, code: sessions.ErrorSessionNameInvalid},
			{name: "non-NFC objective", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", Objective: "e\u0301"}, code: sessions.ErrorObjectiveInvalid},
			{name: "objective control", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", Objective: "inspect\u2028later"}, code: sessions.ErrorObjectiveInvalid},
			{name: "objective bidi isolate", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", Objective: "inspect\u2066later"}, code: sessions.ErrorObjectiveInvalid},
			{name: "oversized objective", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", Objective: strings.Repeat("x", 241)}, code: sessions.ErrorObjectiveInvalid},
		}

		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				_, err := fixture.manager.Create(ctx, test.input)
				assertSessionError(t, err, test.code)
				after, listErr := fixture.manager.List(ctx)
				if listErr != nil {
					t.Fatalf("list sessions after rejected %s", test.name)
				}
				if len(after.Sessions) != len(before.Sessions) {
					t.Fatalf("rejected %s mutated isolated tmux: before_count=%d after_count=%d", test.name, len(before.Sessions), len(after.Sessions))
				}
			})
		}
	})

	t.Run("a command-boundary name collision mutates no existing session", func(t *testing.T) {
		const collisionName = "race-conflict"
		wrapper := filepath.Join(fixture.root, "collision-tmux")
		marker := filepath.Join(fixture.root, "collision-injected")
		identity := filepath.Join(fixture.root, "collision-identity")
		script := fmt.Sprintf(`#!/bin/sh
set -eu
socket=%q
project=%q
marker=%q
identity=%q
is_create=0
is_target=0
previous=
for argument in "$@"; do
  if [ "$argument" = new-session ]; then
    is_create=1
  fi
  if [ "$previous" = -s ] && [ "$argument" = %q ]; then
    is_target=1
  fi
  previous=$argument
done
if [ "$is_create" -eq 1 ] && [ "$is_target" -eq 1 ] && [ ! -e "$marker" ]; then
  : > "$marker"
	  %s -L "$socket" -f /dev/null new-session -d -s %q -c "$project" -- %s 300
	  %s -L "$socket" -f /dev/null set-option -t '=%s:' -- @collision_guard untouched
	  %s -L "$socket" -f /dev/null display-message -p -t '=%s:' '#{session_id}|#{pane_pid}|#{@collision_guard}|#{@skid_profile}|#{@skid_character}|#{@skid_objective_b64}' > "$identity"
fi
exec %s "$@"
`, fixture.socket, fixture.project, marker, identity, collisionName, tmuxPath, collisionName, sleepPath, tmuxPath, collisionName, tmuxPath, collisionName, tmuxPath)
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatal("write collision tmux wrapper")
		}
		manager, err := sessions.New(sessions.Config{
			TmuxPath:      wrapper,
			SocketName:    fixture.socket,
			Home:          fixture.home,
			CataloguePath: fixture.cataloguePath,
			Profiles: []agentruntime.Profile{{
				Key:                  "personal",
				Label:                "Personal",
				Provider:             agentruntime.ProviderCodex,
				Command:              fixture.agent,
				Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
				Arguments:            []string{yoloFlag},
			}},
		})
		if err != nil {
			t.Fatal("construct collision manager")
		}
		_, err = manager.Create(ctx, sessions.CreateInput{
			CWD:              fixture.project,
			Profile:          "personal",
			OptionalTmuxName: collisionName,
			Objective:        "must not land",
		})
		assertSessionError(t, err, sessions.ErrorSessionNameConflict)

		beforeBytes, err := os.ReadFile(identity)
		if err != nil {
			t.Fatal("read injected session identity")
		}
		before := strings.TrimSpace(string(beforeBytes))
		after := fixture.tmux(t, "display-message", "-p", "-t", "="+collisionName+":",
			"#{session_id}|#{pane_pid}|#{@collision_guard}|#{@skid_profile}|#{@skid_character}|#{@skid_objective_b64}")
		if after != before {
			t.Fatal("name collision mutated the pre-existing session")
		}
	})

	t.Run("inventory persists required characters without changing other session facts", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "laptop", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-session", "-d", "-s", "shell", "-c", fixture.project)
		fixture.tmux(t, "new-session", "-d", "-s", "invalid-character", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", "invalid-character", "--", "@skid_character", "not.in-catalogue")
		laptopActivePane := fixture.tmux(t, "display-message", "-p", "-t", "laptop:0.0", "#{pane_id}")
		fixture.tmux(t, "split-window", "-d", "-t", "laptop:0", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-window", "-d", "-t", "laptop", "-n", "laptop-aux", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "select-window", "-t", "laptop:0")
		fixture.tmux(t, "select-pane", "-t", laptopActivePane)
		fixture.attachClient(t, "laptop")

		laptopID := fixture.tmux(t, "display-message", "-p", "-t", "laptop", "#{session_id}")
		invalidID := fixture.tmux(t, "display-message", "-p", "-t", "invalid-character", "#{session_id}")
		laptopBefore := sessionNonCharacterSnapshot(t, fixture, laptopID)
		invalidBefore := sessionNonCharacterSnapshot(t, fixture, invalidID)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list laptop-created sessions: %v", err)
		}
		laptop := requireSessionNamed(t, listed, "laptop")
		invalid := requireSessionNamed(t, listed, "invalid-character")
		requireValidCharacter(t, laptop)
		requireValidCharacter(t, invalid)
		if laptop.LaunchProfile != "" || laptop.Objective != "" || invalid.LaunchProfile != "" || invalid.Objective != "" {
			t.Fatalf("inventory guessed profile or objective metadata: laptop=%+v invalid=%+v", laptop, invalid)
		}
		if laptop.AttachedClients != 1 {
			t.Fatalf("inventory did not preserve the test-owned laptop client: %+v", laptop)
		}
		for _, session := range []sessions.Session{laptop, invalid} {
			if persisted := fixture.tmux(t, "show-options", "-qv", "-t", session.TmuxID, "@skid_character"); persisted != session.Character.Key {
				t.Fatalf("required character was not persisted: session=%+v persisted=%q", session, persisted)
			}
		}
		if after := sessionNonCharacterSnapshot(t, fixture, laptopID); after != laptopBefore {
			t.Fatalf("automatic assignment changed laptop facts:\nbefore=%q\n after=%q", laptopBefore, after)
		}
		if after := sessionNonCharacterSnapshot(t, fixture, invalidID); after != invalidBefore {
			t.Fatalf("invalid-character repair changed non-character facts:\nbefore=%q\n after=%q", invalidBefore, after)
		}
		laptopAgent := requireAgentProjection(t, laptop)
		if laptopAgent.Agent.Provider != agentruntime.ProviderCodex || laptopAgent.Agent.PID <= 0 {
			t.Fatalf("uninstrumented allowlisted laptop process should project exact agent presence: %+v", laptopAgent)
		}
		shell := requireSessionNamed(t, listed, "shell")
		if shell.Agent != nil {
			t.Fatal("ordinary laptop shell projected an agent")
		}

		repeated, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("repeat laptop-created inventory: %v", err)
		}
		if observed := requireSessionID(t, repeated, laptop.TmuxID); observed.Character != laptop.Character {
			t.Fatalf("repeated inventory changed persisted character: first=%+v repeated=%+v", laptop, observed)
		}
		if observed := requireSessionID(t, repeated, invalid.TmuxID); observed.Character != invalid.Character {
			t.Fatalf("repeated inventory changed repaired character: first=%+v repeated=%+v", invalid, observed)
		}

		panePID := fixture.tmux(t, "display-message", "-p", "-t", laptop.TmuxID, "#{pane_pid}")
		fixture.tmux(t, "rename-session", "-t", laptop.TmuxID, "laptop-renamed")
		fixture.tmux(t, "respawn-pane", "-k", "-t", laptop.TmuxID, "--", sleepPath, "300")
		if replacementPID := fixture.tmux(t, "display-message", "-p", "-t", laptop.TmuxID, "#{pane_pid}"); replacementPID == panePID {
			t.Fatalf("test-owned process replacement retained pane PID %s", panePID)
		}
		afterReplacement, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list renamed process replacement: %v", err)
		}
		replaced := requireSessionID(t, afterReplacement, laptop.TmuxID)
		if replaced.TmuxName != "laptop-renamed" || replaced.Character != laptop.Character {
			t.Fatalf("rename or process replacement changed character: before=%+v after=%+v", laptop, replaced)
		}
	})

	t.Run("a concurrent valid character assignment wins the conditional race", func(t *testing.T) {
		baseline, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list before concurrent-character fixture: %v", err)
		}
		baselineUse := map[string]int{"norse.modsognir": 0, "norse.durinn": 0}
		for _, session := range baseline.Sessions {
			requireValidCharacter(t, session)
			baselineUse[session.Character.Key]++
		}
		if baselineUse["norse.modsognir"] != baselineUse["norse.durinn"] {
			t.Fatalf("concurrent-character fixture requires balanced prior use: %+v", baselineUse)
		}
		epoch := fixture.tmux(t, "show-options", "-sqv", tmuxclient.ServerEpochOption)
		if epoch != fixtureServerEpoch {
			t.Fatalf("concurrent-character fixture server epoch = %q, want %q", epoch, fixtureServerEpoch)
		}
		fixture.tmux(t, "new-session", "-d", "-s", "concurrent-character", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", "concurrent-character", "--", "@skid_character", "invalid.concurrent")
		id := fixture.tmux(t, "display-message", "-p", "-t", "concurrent-character", "#{session_id}")
		expectedAttempted := expectedTestCharacter(epoch, id, baselineUse)
		expectedInjected := "norse.durinn"
		if expectedAttempted == expectedInjected {
			expectedInjected = "norse.modsognir"
		}
		const followerName = "concurrent-follower"
		fixture.tmux(t, "new-session", "-d", "-s", followerName, "-c", fixture.project, "--", sleepPath, "300")
		followerID := fixture.tmux(t, "display-message", "-p", "-t", followerName, "#{session_id}")
		if id >= followerID || expectedTestCharacter(epoch, followerID, baselineUse) != expectedInjected {
			t.Fatalf("deterministic concurrent fixture has no later counterfactual follower: concurrent=%s follower=%s attempted=%s injected=%s", id, followerID, expectedAttempted, expectedInjected)
		}
		before := sessionNonCharacterSnapshot(t, fixture, id)
		wrapper := filepath.Join(fixture.root, "concurrent-character-tmux")
		record := filepath.Join(fixture.root, "concurrent-character-record")
		script := fmt.Sprintf(`#!/bin/sh
set -eu
socket=%s
target_id=%s
record=%s
is_if_shell=0
attempted=
target=
previous=
for argument in "$@"; do
	if [ "$argument" = if-shell ]; then
	  is_if_shell=1
	fi
  if [ "$previous" = -t ]; then
    target=$argument
  fi
  case "$argument" in
    *' -- @skid_character '*) attempted=${argument##* } ;;
  esac
  previous=$argument
done
if [ "$is_if_shell" -eq 1 ] && [ -n "$attempted" ] && [ "$target" = "$target_id" ] && [ ! -e "$record" ]; then
  case "$attempted" in
    norse.durinn) injected=norse.modsognir ;;
    norse.modsognir) injected=norse.durinn ;;
    *) exit 96 ;;
  esac
  printf '%%s\n%%s\n' "$attempted" "$injected" > "$record"
  %s -L "$socket" -f /dev/null set-option -t "$target_id" -- @skid_character "$injected"
fi
exec %s "$@"
`, shellQuote(fixture.socket), shellQuote(id), shellQuote(record), shellQuote(tmuxPath), shellQuote(tmuxPath))
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatalf("write concurrent-character tmux wrapper: %v", err)
		}
		manager, err := sessions.New(sessions.Config{
			TmuxPath: wrapper, SocketName: fixture.socket, Home: fixture.home, CataloguePath: fixture.cataloguePath,
			Profiles: []agentruntime.Profile{{
				Key: "personal", Label: "Codex · Personal", Provider: agentruntime.ProviderCodex, Command: fixture.agent,
				Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
				Arguments:            []string{yoloFlag},
			}},
		})
		if err != nil {
			t.Fatalf("construct concurrent-character manager: %v", err)
		}
		listed, err := manager.List(ctx)
		if err != nil {
			t.Fatalf("list through concurrent-character boundary: %v", err)
		}
		recordBytes, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("read concurrent-character record: %v", err)
		}
		attemptedAndInjected := strings.Split(strings.TrimSpace(string(recordBytes)), "\n")
		if len(attemptedAndInjected) != 2 || attemptedAndInjected[0] != expectedAttempted || attemptedAndInjected[1] != expectedInjected {
			t.Fatalf("concurrent-character fixture recorded unexpected keys: want=%q/%q got=%q", expectedAttempted, expectedInjected, recordBytes)
		}
		observed := requireSessionID(t, listed, id)
		requireValidCharacter(t, observed)
		if observed.Character.Key != attemptedAndInjected[1] {
			t.Fatalf("concurrent valid character did not win: attempted=%q injected=%q observed=%+v", attemptedAndInjected[0], attemptedAndInjected[1], observed)
		}
		if persisted := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_character"); persisted != observed.Character.Key {
			t.Fatalf("concurrent valid character was not preserved: card=%+v persisted=%q", observed, persisted)
		}
		follower := requireSessionID(t, listed, followerID)
		if follower.Character.Key != expectedAttempted {
			t.Fatalf("later assignment did not count the re-read concurrent winner: attempted=%q injected=%q follower=%+v", expectedAttempted, expectedInjected, follower)
		}
		if persisted := fixture.tmux(t, "show-options", "-qv", "-t", followerID, "@skid_character"); persisted != expectedAttempted {
			t.Fatalf("later assignment after concurrent winner was not persisted: follower=%+v persisted=%q", follower, persisted)
		}
		if after := sessionNonCharacterSnapshot(t, fixture, id); after != before {
			t.Fatalf("conditional race changed non-character facts:\nbefore=%q\n after=%q", before, after)
		}
	})

	t.Run("a session disappearing during assignment is omitted without a collateral write", func(t *testing.T) {
		const sentinelName = "disappearance-sentinel"
		fixture.tmux(t, "new-session", "-d", "-s", sentinelName, "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", sentinelName, "--", "@skid_character", "norse.durinn")
		sentinelID := fixture.tmux(t, "display-message", "-p", "-t", sentinelName, "#{session_id}")
		const targetName = "disappearing-character"
		fixture.tmux(t, "new-session", "-d", "-s", targetName, "-c", fixture.project, "--", sleepPath, "300")
		targetID := fixture.tmux(t, "display-message", "-p", "-t", targetName, "#{session_id}")

		wrapper := filepath.Join(fixture.root, "disappearing-character-tmux")
		record := filepath.Join(fixture.root, "disappearing-character-record")
		script := fmt.Sprintf(`#!/bin/sh
set -eu
socket=%s
target_id=%s
record=%s
is_assignment=0
target=
previous=
for argument in "$@"; do
  if [ "$previous" = -t ]; then
    target=$argument
  fi
  case "$argument" in
    *' -- @skid_character '*) is_assignment=1 ;;
  esac
  previous=$argument
done
if [ "$is_assignment" -eq 1 ] && [ "$target" = "$target_id" ] && [ ! -e "$record" ]; then
  printf '%%s\n' "$target_id" > "$record"
  %s -L "$socket" -f /dev/null kill-session -t "$target_id"
fi
exec %s "$@"
`, shellQuote(fixture.socket), shellQuote(targetID), shellQuote(record), shellQuote(tmuxPath), shellQuote(tmuxPath))
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatalf("write disappearing-character tmux wrapper: %v", err)
		}
		manager, err := sessions.New(sessions.Config{
			TmuxPath: wrapper, SocketName: fixture.socket, Home: fixture.home, CataloguePath: fixture.cataloguePath,
			Profiles: []agentruntime.Profile{{
				Key: "personal", Label: "Codex · Personal", Provider: agentruntime.ProviderCodex, Command: fixture.agent,
				Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
				Arguments:            []string{yoloFlag},
			}},
		})
		if err != nil {
			t.Fatalf("construct disappearing-character manager: %v", err)
		}
		listed, err := manager.List(ctx)
		if err != nil {
			t.Fatalf("list through disappearing-character boundary: %v", err)
		}
		recordedID, err := os.ReadFile(record)
		if err != nil || strings.TrimSpace(string(recordedID)) != targetID {
			t.Fatalf("disappearing-character fixture did not remove exact target %s: record=%q error=%v", targetID, recordedID, err)
		}
		for _, session := range listed.Sessions {
			if session.TmuxID == targetID {
				t.Fatalf("inventory returned session that vanished during assignment: %+v", session)
			}
		}
		sentinel := requireSessionID(t, listed, sentinelID)
		if sentinel.Character.Key != "norse.durinn" {
			t.Fatalf("disappearing assignment changed sentinel character: %+v", sentinel)
		}
		if persisted := fixture.tmux(t, "show-options", "-qv", "-t", sentinelID, "@skid_character"); persisted != sentinel.Character.Key {
			t.Fatalf("disappearing assignment wrote a collateral session: sentinel=%+v persisted=%q", sentinel, persisted)
		}
	})

	t.Run("foreground registration is exact and lifetime-bound", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "owned-node", "-c", fixture.project, "--", fixture.nodePath, fixture.nodeScript)
		fixture.tmux(t, "new-session", "-d", "-s", "plain-node", "-c", fixture.project, "--", fixture.nodePath, "-e", "setInterval(() => {}, 300000)")

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list exact foreground signature fixtures")
		}
		owned := requireSessionNamed(t, listed, "owned-node")
		ownedAgent := requireAgentProjection(t, owned)
		if owned.LaunchProfile != "" || ownedAgent.Agent.Provider != agentruntime.ProviderCodex ||
			ownedAgent.Agent.PID <= 0 || ownedAgent.Agent.Profile != "" || ownedAgent.Agent.ProviderSession != nil {
			t.Fatalf("exact signature identity mismatch: %+v", owned)
		}
		plainNode := requireSessionNamed(t, listed, "plain-node")
		if plainNode.Agent != nil || plainNode.LaunchProfile != "" {
			t.Fatalf("arbitrary node process matched an owned signature: agent_present=%t profile_present=%t", plainNode.Agent != nil, plainNode.LaunchProfile != "")
		}

		paneID := fixture.tmux(t, "display-message", "-p", "-t", owned.TmuxID, "#{pane_id}")
		panePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", owned.TmuxID, "#{pane_pid}"))
		if err != nil || panePID <= 0 {
			t.Fatalf("parse owned foreground pane pid: %v", err)
		}
		startIdentity := processStartIdentity(panePID)
		if startIdentity == "" || ownedAgent.Agent.PID != processinfo.PID(panePID) {
			t.Fatalf("observed foreground lifetime mismatch: pane_pid=%d agent=%+v", panePID, ownedAgent.Agent)
		}
		registration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
			Provider:      agentruntime.ProviderCodex,
			PID:           processinfo.PID(panePID),
			StartIdentity: startIdentity,
		}, "personal", "codex-owned-node")
		if err != nil {
			t.Fatalf("encode exact foreground registration: %v", err)
		}
		fixture.tmux(t, "set-option", "-p", "-t", paneID, "--", agentruntime.PaneOption, registration)
		beforeList := sessionNonCharacterSnapshot(t, fixture, owned.TmuxID)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list exact registered foreground")
		}
		registered := requireSessionID(t, listed, owned.TmuxID)
		registeredAgent := requireAgentProjection(t, registered)
		if registeredAgent.Agent.Profile != "personal" || registeredAgent.Agent.ProviderSession == nil ||
			registeredAgent.Agent.ProviderSession.ID() != "codex-owned-node" || registeredAgent.Agent.ProviderSession.Name() != "" {
			t.Fatalf("valid foreground registration did not project: %+v", registeredAgent.Agent)
		}
		if afterList := sessionNonCharacterSnapshot(t, fixture, owned.TmuxID); afterList != beforeList {
			t.Fatalf("identity projection mutated tmux inventory:\nbefore=%q\n after=%q", beforeList, afterList)
		}

		const renamed = "owned-node-renamed"
		fixture.tmux(t, "rename-session", "-t", owned.TmuxID, renamed)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list registered foreground after tmux rename")
		}
		registered = requireSessionID(t, listed, owned.TmuxID)
		registeredAgent = requireAgentProjection(t, registered)
		if registered.TmuxName != renamed || registeredAgent.Agent.Profile != "personal" ||
			registeredAgent.Agent.ProviderSession == nil || registeredAgent.Agent.ProviderSession.ID() != "codex-owned-node" {
			t.Fatalf("tmux rename changed provider-owned identity: %+v", registered)
		}

		oldPID := registeredAgent.Agent.PID
		fixture.tmux(t, "respawn-pane", "-k", "-t", paneID, "--", fixture.nodePath, fixture.nodeScript)
		deadline := time.Now().Add(tmuxConvergenceTimeout)
		var replacement sessions.Session
		for {
			replacementPID, parseErr := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", paneID, "#{pane_pid}"))
			replacementStart := processStartIdentity(replacementPID)
			listed, err = fixture.manager.List(ctx)
			if err != nil {
				t.Fatal("list replacement foreground")
			}
			replacement = requireSessionID(t, listed, owned.TmuxID)
			replacementAgent := replacement.Agent
			if parseErr == nil && replacementStart != "" && (processinfo.PID(replacementPID) != oldPID || replacementStart != startIdentity) &&
				replacementAgent != nil && replacementAgent.PID == processinfo.PID(replacementPID) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("replacement foreground did not converge: old_pid=%d session=%+v", oldPID, replacement)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		replacementAgent := requireAgentProjection(t, replacement)
		if replacement.TmuxName != renamed || replacement.LaunchProfile != "" || replacementAgent.Agent.Provider != agentruntime.ProviderCodex ||
			replacementAgent.Agent.Profile != "" || replacementAgent.Agent.ProviderSession != nil {
			t.Fatalf("stale registration survived process replacement: %+v", replacement)
		}
		if persisted := fixture.tmux(t, "show-options", "-pqv", "-t", paneID, agentruntime.PaneOption); persisted != registration {
			t.Fatalf("inventory repaired or rewrote the stale registration: value_match=%t", persisted == registration)
		}
	})

	t.Run("optional agent identity follows the current window active pane", func(t *testing.T) {
		otherDirectory := filepath.Join(fixture.root, "agent anchor other pane")
		if err := os.Mkdir(otherDirectory, 0o700); err != nil {
			t.Fatal("create agent-anchor pane cwd")
		}
		fixture.tmux(t, "new-session", "-d", "-s", "agent-anchor", "-c", fixture.project, "--", sleepPath, "300")
		activePane := fixture.tmux(t, "display-message", "-p", "-t", "agent-anchor:0.0", "#{pane_id}")
		activePanePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", activePane, "#{pane_pid}"))
		if err != nil || activePanePID <= 0 {
			t.Fatalf("parse active agent-anchor pane pid: valid=%t", err == nil && activePanePID > 0)
		}
		inactivePane := fixture.tmux(t, "split-window", "-d", "-P", "-F", "#{pane_id}", "-t", "agent-anchor:0", "-c", otherDirectory, "--", sleepPath, "300")
		inactivePanePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", inactivePane, "#{pane_pid}"))
		if err != nil || inactivePanePID <= 0 {
			t.Fatalf("parse inactive agent-anchor pane pid: valid=%t", err == nil && inactivePanePID > 0)
		}
		inactiveStartIdentity := processStartIdentity(inactivePanePID)
		if inactiveStartIdentity == "" {
			t.Fatal("capture inactive agent-anchor process identity")
		}
		inactiveRegistration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
			Provider:      agentruntime.ProviderCodex,
			PID:           processinfo.PID(inactivePanePID),
			StartIdentity: inactiveStartIdentity,
		}, "work", "inactive-pane-session")
		if err != nil {
			t.Fatalf("encode inactive agent-anchor registration: %v", err)
		}
		fixture.tmux(t, "set-option", "-p", "-t", inactivePane, "--", agentruntime.PaneOption, inactiveRegistration)
		fixture.tmux(t, "select-pane", "-t", inactivePane)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list inactive agent-anchor pane")
		}
		inactive := requireSessionNamed(t, listed, "agent-anchor")
		inactiveAgent := requireAgentProjection(t, inactive).Agent
		if inactive.CWD != otherDirectory || inactiveAgent.Provider != agentruntime.ProviderCodex ||
			inactiveAgent.PID != processinfo.PID(inactivePanePID) || inactiveAgent.Profile != "work" ||
			inactiveAgent.ProviderSession == nil || inactiveAgent.ProviderSession.ID() != "inactive-pane-session" ||
			inactiveAgent.ProviderSession.Name() != "" {
			t.Fatalf("inactive agent-anchor projection is not exact: session=%+v agent=%+v", inactive, inactiveAgent)
		}

		fixture.tmux(t, "select-pane", "-t", activePane)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list active agent-anchor pane")
		}
		active := requireSessionNamed(t, listed, "agent-anchor")
		activeAgent := requireAgentProjection(t, active).Agent
		if active.CWD != fixture.project || activeAgent.Provider != agentruntime.ProviderCodex ||
			activeAgent.PID != processinfo.PID(activePanePID) || activeAgent.Profile != "" || activeAgent.ProviderSession != nil {
			t.Fatalf("inactive pane metadata leaked into active agent-anchor projection: session=%+v agent=%+v", active, activeAgent)
		}
	})

	t.Run("current-window tmux activity is the complete provider-neutral card state", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "activity-shell", "-c", fixture.project, "--", "/bin/sh")
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list initial generic-shell activity: %v", err)
		}
		initial := requireSessionNamed(t, listed, "activity-shell")
		if initial.Activity != sessions.SessionActivityActive {
			t.Fatalf("new generic-shell window activity = %q, want Active", initial.Activity)
		}
		beforeActivity := fixture.tmux(t, "display-message", "-p", "-t", initial.TmuxID, "#{window_activity}")
		beforeCharacter := fixture.tmux(t, "show-options", "-qv", "-t", initial.TmuxID, "@skid_character")
		beforeOptions := sessionNonCharacterSnapshot(t, fixture, initial.TmuxID)
		beforeMonitor := fixture.tmux(t, "show-options", "-wv", "-t", initial.TmuxID, "monitor-activity") + "|" +
			fixture.tmux(t, "show-options", "-wv", "-t", initial.TmuxID, "monitor-silence")
		for range 2 {
			if _, err := fixture.manager.List(ctx); err != nil {
				t.Fatalf("repeat read-only activity inventory: %v", err)
			}
		}
		if afterActivity := fixture.tmux(t, "display-message", "-p", "-t", initial.TmuxID, "#{window_activity}"); afterActivity != beforeActivity {
			t.Fatalf("inventory read changed tmux window_activity: before=%q after=%q", beforeActivity, afterActivity)
		}
		if afterCharacter := fixture.tmux(t, "show-options", "-qv", "-t", initial.TmuxID, "@skid_character"); afterCharacter != beforeCharacter {
			t.Fatalf("inventory read changed the existing character: before=%q after=%q", beforeCharacter, afterCharacter)
		}
		if afterOptions := sessionNonCharacterSnapshot(t, fixture, initial.TmuxID); afterOptions != beforeOptions {
			t.Fatal("activity inventory mutated tmux structure or options")
		}
		afterMonitor := fixture.tmux(t, "show-options", "-wv", "-t", initial.TmuxID, "monitor-activity") + "|" +
			fixture.tmux(t, "show-options", "-wv", "-t", initial.TmuxID, "monitor-silence")
		if afterMonitor != beforeMonitor {
			t.Fatalf("activity inventory changed monitor options: before=%q after=%q", beforeMonitor, afterMonitor)
		}
		fixture.waitForActivities(t, ctx, sessions.SessionActivityQuiet, "activity-shell")
		fixture.tmux(t, "new-window", "-d", "-t", "activity-shell", "-n", "other", "-c", fixture.project, "--", "/bin/sh")
		otherPane := fixture.tmux(t, "display-message", "-p", "-t", "activity-shell:other", "#{pane_id}")
		fixture.tmux(t, "send-keys", "-t", otherPane, "-l", "printf other-window-output")
		fixture.tmux(t, "send-keys", "-t", otherPane, "Enter")
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after non-current-window output: %v", err)
		}
		if activity := requireSessionNamed(t, listed, "activity-shell").Activity; activity != sessions.SessionActivityQuiet {
			t.Fatalf("non-current-window output changed current-window activity to %q", activity)
		}

		fixture.tmux(t, "select-window", "-t", "activity-shell:other")
		fixture.waitForActivities(t, ctx, sessions.SessionActivityActive, "activity-shell")
		fixture.tmux(t, "select-window", "-t", "activity-shell:0")
		fixture.waitForActivities(t, ctx, sessions.SessionActivityActive, "activity-shell")

		siblingPane := fixture.tmux(t, "split-window", "-d", "-P", "-F", "#{pane_id}", "-t", "activity-shell:0", "-c", fixture.project, "--", "/bin/sh")
		fixture.tmux(t, "new-session", "-d", "-t", "activity-shell", "-s", "activity-link")
		fixture.waitForActivities(t, ctx, sessions.SessionActivityQuiet, "activity-shell", "activity-link")
		fixture.tmux(t, "send-keys", "-t", siblingPane, "-l", "printf sibling-pane-output")
		fixture.tmux(t, "send-keys", "-t", siblingPane, "Enter")
		fixture.waitForActivities(t, ctx, sessions.SessionActivityActive, "activity-shell", "activity-link")

		fixture.waitForActivities(t, ctx, sessions.SessionActivityQuiet, "activity-shell", "activity-link")
		outputDone := make(chan error, 1)
		go func() {
			_, commandErr := isolatedTmuxCommand(
				tmuxPath, "-L", fixture.socket, "-f", "/dev/null",
				"send-keys", "-t", siblingPane, "-l", "printf concurrent-output", ";",
				"send-keys", "-t", siblingPane, "Enter",
			).CombinedOutput()
			outputDone <- commandErr
		}()
		concurrent, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("inventory concurrent with generic-shell output: %v", err)
		}
		concurrentActivity := requireSessionNamed(t, concurrent, "activity-shell").Activity
		if concurrentActivity != sessions.SessionActivityActive && concurrentActivity != sessions.SessionActivityQuiet {
			t.Fatalf("concurrent output produced activity outside the closed union: %q", concurrentActivity)
		}
		if commandErr := <-outputDone; commandErr != nil {
			t.Fatal("emit concurrent generic-shell output")
		}
		fixture.waitForActivities(t, ctx, sessions.SessionActivityActive, "activity-shell", "activity-link")
	})

	t.Run("phone shadows are excluded while reclaimed last links receive characters", func(t *testing.T) {
		const phoneShadow = "skid-phone-00112233445566778899aabbccddeeff"
		const reclaimedShadow = "skid-phone-ffeeddccbbaa99887766554433221100"
		fixture.tmux(t, "new-session", "-d", "-s", phoneShadow, "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", phoneShadow, "--", "@skid_internal", "phone-shadow")
		fixture.attachClient(t, phoneShadow)
		fixture.tmux(t, "new-session", "-d", "-s", reclaimedShadow, "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", reclaimedShadow, "--", "@skid_internal", "phone-shadow")
		fixture.tmux(t, "new-session", "-d", "-s", "group-source", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-session", "-d", "-t", "group-source", "-s", "group-link")
		fixture.attachClient(t, "group-source")
		fixture.attachClient(t, "group-link")

		deadline := time.Now().Add(tmuxConvergenceTimeout)
		var reclaimed sessions.Session
		for {
			listed, err := fixture.manager.List(ctx)
			if err != nil {
				t.Fatalf("list grouped client fixture: %v", err)
			}
			if slices.ContainsFunc(listed.Sessions, func(session sessions.Session) bool { return session.TmuxName == phoneShadow }) {
				t.Fatalf("gateway-owned phone shadow leaked into inventory: %+v", listed.Sessions)
			}
			if character := fixture.tmux(t, "show-options", "-qv", "-t", phoneShadow, "@skid_character"); character != "" {
				t.Fatalf("phone shadow received a character: %q", character)
			}
			reclaimed = requireSessionNamed(t, listed, reclaimedShadow)
			requireValidCharacter(t, reclaimed)
			if marker := fixture.tmux(t, "show-options", "-qv", "-t", reclaimedShadow, "@skid_internal"); marker != "" {
				t.Fatalf("reclaimed last link retained phone-shadow marker: %q", marker)
			}
			source := requireSessionNamed(t, listed, "group-source")
			link := requireSessionNamed(t, listed, "group-link")
			requireValidCharacter(t, source)
			requireValidCharacter(t, link)
			if source.AttachedClients == 2 && link.AttachedClients == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("group attachment count did not converge: source=%+v link=%+v", source, link)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		persistedCharacter := fixture.tmux(t, "show-options", "-qv", "-t", reclaimed.TmuxID, "@skid_character")
		if persistedCharacter != reclaimed.Character.Key {
			t.Fatalf("reclaimed last link did not persist its character: session=%+v persisted=%q", reclaimed, persistedCharacter)
		}
		repeated, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("repeat reclaimed last-link inventory: %v", err)
		}
		repeatedReclaimed := requireSessionID(t, repeated, reclaimed.TmuxID)
		if repeatedReclaimed.Character != reclaimed.Character {
			t.Fatalf("repeated inventory changed reclaimed character: first=%+v repeated=%+v", reclaimed, repeatedReclaimed)
		}
		if persisted := fixture.tmux(t, "show-options", "-qv", "-t", reclaimed.TmuxID, "@skid_character"); persisted != persistedCharacter {
			t.Fatalf("repeated inventory changed reclaimed persisted character: before=%q after=%q", persistedCharacter, persisted)
		}
	})

	t.Run("an enumerated dead pane keeps activity while omitting agent identity", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "unobservable", "-c", fixture.project, "--", "/bin/sh")
		fixture.tmux(t, "set-option", "-w", "-t", "unobservable", "remain-on-exit", "on")
		fixture.tmux(t, "send-keys", "-t", "unobservable", "exit", "Enter")

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list process-observation failure fixture")
		}
		unobservable := requireSessionNamed(t, listed, "unobservable")
		requireValidCharacter(t, unobservable)
		if unobservable.Agent != nil {
			t.Fatalf("dead pane projected optional agent identity: %+v", unobservable.Agent)
		}
		if unobservable.Activity != sessions.SessionActivityActive && unobservable.Activity != sessions.SessionActivityQuiet {
			t.Fatalf("dead pane activity escaped the closed union: %q", unobservable.Activity)
		}
	})

	t.Run("create keeps tmux names independent from balanced required characters", func(t *testing.T) {
		beforeCreate, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list before managed creates: %v", err)
		}
		liveCharacterUse := map[string]int{"norse.modsognir": 0, "norse.durinn": 0}
		for _, session := range beforeCreate.Sessions {
			requireValidCharacter(t, session)
			liveCharacterUse[session.Character.Key]++
		}
		requireCreatedLeastUsed := func(label string, session sessions.Session) {
			t.Helper()
			requireValidCharacter(t, session)
			selectedUse := liveCharacterUse[session.Character.Key]
			leastUse := min(liveCharacterUse["norse.modsognir"], liveCharacterUse["norse.durinn"])
			if selectedUse != leastUse {
				t.Fatalf("%s create selected an overused character: before=%+v created=%+v", label, liveCharacterUse, session)
			}
			liveCharacterUse[session.Character.Key]++
		}
		requireObservedCreate := func(label string, observed sessions.ObservedSession) sessions.Session {
			t.Helper()
			if observed.ObservedAt.IsZero() || observed.ObservedAt.Location() != time.UTC || observed.ObservedAt.Before(beforeCreate.ObservedAt) {
				t.Fatalf("%s create did not return one UTC service observation clock: before=%s created=%s", label, beforeCreate.ObservedAt, observed.ObservedAt)
			}
			return observed.Session
		}
		firstObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{
			CWD:       "~/project with spaces",
			Profile:   "personal",
			Objective: "Inspect Ω",
		})
		if err != nil {
			t.Fatalf("create first generated session: %v", err)
		}
		first := requireObservedCreate("first generated", firstObserved)
		requireCreatedLeastUsed("first generated", first)
		secondObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work"})
		if err != nil {
			t.Fatalf("create second generated session: %v", err)
		}
		second := requireObservedCreate("second generated", secondObserved)
		requireCreatedLeastUsed("second generated", second)
		thirdObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work2"})
		if err != nil {
			t.Fatalf("create generated session beyond catalogue base names: %v", err)
		}
		third := requireObservedCreate("third generated", thirdObserved)
		requireCreatedLeastUsed("third generated", third)
		customObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "hand_named"})
		if err != nil {
			t.Fatalf("create custom named session: %v", err)
		}
		custom := requireObservedCreate("custom named", customObserved)
		requireCreatedLeastUsed("custom named", custom)
		claudeObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "claude-work"})
		if err != nil {
			t.Fatalf("create Claude work session: %v", err)
		}
		claude := requireObservedCreate("Claude work", claudeObserved)
		requireCreatedLeastUsed("Claude work", claude)

		for _, test := range []struct {
			want    string
			session sessions.Session
		}{
			{want: "skidbladnir-personal-1", session: first},
			{want: "skidbladnir-work-1", session: second},
			{want: "skidbladnir-work2-1", session: third},
			{want: "hand_named", session: custom},
			{want: "skidbladnir-claude-work-1", session: claude},
		} {
			if test.session.TmuxName != test.want {
				t.Fatalf("created tmux name = %q, want %q: %+v", test.session.TmuxName, test.want, test.session)
			}
			requireValidCharacter(t, test.session)
		}
		if first.LaunchProfile != "personal" || first.Objective != "Inspect Ω" || first.CWD != fixture.project {
			t.Fatalf("managed session facts mismatch: %+v", first)
		}

		for _, profile := range []string{"personal", "work", "work2"} {
			argvPath := filepath.Join(fixture.profileHomes[profile], "observed-argv")
			waitForFileLine(t, argvPath, yoloFlag)
			waitForFileLine(t, filepath.Join(fixture.profileHomes[profile], "observed-home"), fixture.profileHomes[profile])
		}
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-argv"), "--name\nskidbladnir-claude-work-1\n--permission-mode\nauto")
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-home"), fixture.profileHomes["claude-work"])
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-cwd"), fixture.project)
		for _, created := range []sessions.Session{first, second, third, custom, claude} {
			fixture.waitForAgent(t, ctx, created.TmuxName)
		}

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after managed creates: %v", err)
		}
		for _, created := range []sessions.Session{first, second, third, custom, claude} {
			observed := requireSessionID(t, listed, created.TmuxID)
			if observed.TmuxName != created.TmuxName || observed.LaunchProfile != created.LaunchProfile ||
				observed.Character != created.Character || observed.IdentityToken != created.IdentityToken {
				t.Fatalf("create response did not converge to tmux inventory: created=%+v listed=%+v", created, observed)
			}
			observedAgent := requireAgentProjection(t, observed)
			if observedAgent.Agent.Profile != "" {
				t.Fatalf("managed launch inferred a hook-owned runtime profile: %+v", observed)
			}
			if created.TmuxID == claude.TmuxID {
				if observedAgent.Agent.Provider != agentruntime.ProviderClaude || observedAgent.Agent.ProviderSession == nil ||
					observedAgent.Agent.ProviderSession.ID() != "" || observedAgent.Agent.ProviderSession.Name() != observed.TmuxName {
					t.Fatalf("managed Claude launch did not project its exact provider name: %+v", observed)
				}
			} else if observedAgent.Agent.Provider != agentruntime.ProviderCodex || observedAgent.Agent.ProviderSession != nil {
				t.Fatalf("managed Codex launch invented provider-owned identity: %+v", observed)
			}
		}
		characterUse := map[string]int{"norse.modsognir": 0, "norse.durinn": 0}
		for _, session := range listed.Sessions {
			requireValidCharacter(t, session)
			characterUse[session.Character.Key]++
		}
		if difference := characterUse["norse.modsognir"] - characterUse["norse.durinn"]; difference < -1 || difference > 1 {
			t.Fatalf("live character allocation is not balanced: %+v", characterUse)
		}
	})

	t.Run("kill rechecks exact id and tmux name", func(t *testing.T) {
		victimObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "kill_victim"})
		if err != nil {
			t.Fatal("create kill victim")
		}
		victim := victimObserved.Session
		survivorObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalTmuxName: "kill_survivor"})
		if err != nil {
			t.Fatal("create kill survivor")
		}
		survivor := survivorObserved.Session

		err = fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: victim.TmuxID, TmuxName: survivor.TmuxName, IdentityToken: victim.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
		listed, listErr := fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatal("list after refused mismatched kill")
		}
		requireSessionID(t, listed, victim.TmuxID)
		requireSessionID(t, listed, survivor.TmuxID)

		missingToken := strings.TrimSuffix(victim.IdentityToken, strings.TrimPrefix(victim.TmuxID, "$")) + "999999"
		err = fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: "$999999", TmuxName: victim.TmuxName, IdentityToken: missingToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)
		err = fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: "kill_survivor", TmuxName: survivor.TmuxName, IdentityToken: survivor.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)

		if err := fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: victim.TmuxID, TmuxName: victim.TmuxName, IdentityToken: victim.IdentityToken}); err != nil {
			t.Fatal("kill exact confirmed session")
		}
		listed, listErr = fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatal("list after exact kill")
		}
		if slices.ContainsFunc(listed.Sessions, func(session sessions.Session) bool { return session.TmuxID == victim.TmuxID }) {
			t.Fatalf("exactly killed session remains listed: session_count=%d", len(listed.Sessions))
		}
		requireSessionID(t, listed, survivor.TmuxID)
	})

	t.Run("kill refuses an ordinary grouped sibling without mutation", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "grouped-kill-target", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-session", "-d", "-t", "grouped-kill-target", "-s", "grouped-kill-sibling")
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list grouped kill fixture")
		}
		target := requireSessionNamed(t, listed, "grouped-kill-target")
		sibling := requireSessionNamed(t, listed, "grouped-kill-sibling")
		panePID := fixture.tmux(t, "display-message", "-p", "-t", target.TmuxID, "#{pane_pid}")
		if siblingPID := fixture.tmux(t, "display-message", "-p", "-t", sibling.TmuxID, "#{pane_pid}"); siblingPID != panePID {
			t.Fatal("grouped kill fixture does not share one pane process")
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: target.TmuxID, TmuxName: target.TmuxName, IdentityToken: target.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionGroupedConflict)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after refused grouped kill")
		}
		if observed := requireSessionID(t, listed, target.TmuxID); observed.TmuxName != target.TmuxName {
			t.Fatal("refused grouped kill changed target identity")
		}
		if observed := requireSessionID(t, listed, sibling.TmuxID); observed.TmuxName != sibling.TmuxName {
			t.Fatal("refused grouped kill changed sibling identity")
		}
		if after := fixture.tmux(t, "display-message", "-p", "-t", target.TmuxID, "#{pane_pid}"); after != panePID {
			t.Fatal("refused grouped kill changed shared pane process")
		}
	})

	t.Run("one kill reconciles every stale owned shadow before ending only the source", func(t *testing.T) {
		targetObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "shadowed_kill_target"})
		if err != nil {
			t.Fatal("create shadowed kill target")
		}
		target := targetObserved.Session
		survivorObserved, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalTmuxName: "shadowed_kill_survivor"})
		if err != nil {
			t.Fatal("create shadowed kill survivor")
		}
		survivor := survivorObserved.Session
		panePIDText := fixture.tmux(t, "display-message", "-p", "-t", target.TmuxID, "#{pane_pid}")
		panePID, err := strconv.Atoi(panePIDText)
		if err != nil {
			t.Fatal("parse shadowed target pane pid")
		}
		paneStartTime := processStartIdentity(panePID)
		if paneStartTime == "" {
			t.Fatal("capture shadowed target pane process identity")
		}
		survivorPanePIDText := fixture.tmux(t, "display-message", "-p", "-t", survivor.TmuxID, "#{pane_pid}")
		survivorPanePID, err := strconv.Atoi(survivorPanePIDText)
		if err != nil {
			t.Fatal("parse shadowed survivor pane pid")
		}
		survivorPaneStartTime := processStartIdentity(survivorPanePID)
		if survivorPaneStartTime == "" {
			t.Fatal("capture shadowed survivor pane process identity")
		}
		shadowNames := []string{
			"skid-phone-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"skid-phone-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"skid-phone-cccccccccccccccccccccccccccccccc",
		}
		for _, shadowName := range shadowNames {
			fixture.tmux(t, "new-session", "-d", "-t", target.TmuxID, "-s", shadowName)
			fixture.tmux(t, "set-option", "-t", "="+shadowName+":", "--", "@skid_internal", "phone-shadow")
		}

		if err := fixture.manager.Kill(ctx, sessions.KillInput{TmuxID: target.TmuxID, TmuxName: target.TmuxName, IdentityToken: target.IdentityToken}); err != nil {
			t.Fatal("kill source after multi-shadow fixed-point recovery")
		}
		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for processStartIdentity(panePID) == paneStartTime {
			if time.Now().After(deadline) {
				t.Fatal("source process survived successful kill after stale-shadow recovery")
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		remainingNames := strings.Split(fixture.tmux(t, "list-sessions", "-F", "#{session_name}"), "\n")
		for _, name := range append([]string{target.TmuxName}, shadowNames...) {
			if slices.Contains(remainingNames, name) {
				t.Fatalf("successful kill left a grouped link: remaining_session_count=%d", len(remainingNames))
			}
		}
		if !slices.Contains(remainingNames, survivor.TmuxName) {
			t.Fatalf("multi-shadow recovery killed the unrelated survivor: remaining_session_count=%d", len(remainingNames))
		}
		if observed := fixture.tmux(t, "display-message", "-p", "-t", survivor.TmuxID, "#{pane_pid}"); observed != survivorPanePIDText || processStartIdentity(survivorPanePID) != survivorPaneStartTime {
			t.Fatal("multi-shadow recovery changed the survivor process")
		}
	})
}

func TestStaleLifetimeTokenCannotKillRecycledSession(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal("create lifetime-token fixture")
	}
	cataloguePath := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(cataloguePath, []byte(`[
  {"key":"norse.modsognir","displayName":"Móðsognir"},
  {"key":"norse.durinn","displayName":"Durinn"}
]`), 0o600); err != nil {
		t.Fatal("write lifetime-token catalogue")
	}
	socket := randomTmuxSocketName(t, "skid-reuse")
	socketPath := namedTmuxSocketPath(socket)
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("isolated lifetime-token socket unexpectedly exists before test")
	}
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    socket,
		Home:          home,
		CataloguePath: cataloguePath,
		Profiles: []agentruntime.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Provider:             agentruntime.ProviderCodex,
			Command:              sleepPath,
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: filepath.Join(home, ".codex-personal")}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
			Arguments:            []string{"300"},
		}},
	})
	if err != nil {
		t.Fatal("construct lifetime-token manager")
	}

	ctx := context.Background()
	conditionalClient, err := tmuxclient.New(tmuxPath, socket)
	if err != nil {
		t.Fatalf("construct lifetime-token tmux client: %v", err)
	}
	var cleanup *sessions.Session
	t.Cleanup(func() {
		if cleanup == nil {
			return
		}
		if err := manager.Kill(ctx, sessions.KillInput{TmuxID: cleanup.TmuxID, TmuxName: cleanup.TmuxName, IdentityToken: cleanup.IdentityToken}); err != nil {
			t.Error("kill exact test-owned lifetime fixture")
		}
	})

	firstObserved, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalTmuxName: "epoch-reuse"})
	if err != nil {
		t.Fatal("create first lifetime fixture")
	}
	first := firstObserved.Session
	cleanup = &first
	firstServer := captureTestTmuxServer(t, tmuxPath, socketPath)
	firstPane, err := conditionalClient.Output(ctx, "read-first-runtime-pane", "display-message", "-p", "-t", first.TmuxID, "#{pane_id}|#{pane_pid}")
	if err != nil {
		t.Fatal("read first runtime pane")
	}
	firstPaneFields := strings.Split(firstPane, "|")
	if len(firstPaneFields) != 2 {
		t.Fatalf("first runtime pane shape is invalid: field_count=%d", len(firstPaneFields))
	}
	firstPanePID, err := strconv.Atoi(firstPaneFields[1])
	if err != nil || firstPanePID <= 0 {
		t.Fatalf("parse first runtime pane pid: valid=%t", err == nil && firstPanePID > 0)
	}
	firstStartIdentity := processStartIdentity(firstPanePID)
	if firstStartIdentity == "" {
		t.Fatal("capture first runtime process identity")
	}
	firstRegistration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider:      agentruntime.ProviderCodex,
		PID:           processinfo.PID(firstPanePID),
		StartIdentity: firstStartIdentity,
	}, "personal", "first-server-session")
	if err != nil {
		t.Fatalf("encode first runtime registration: %v", err)
	}
	if err := conditionalClient.Run(ctx, "register-first-runtime", "set-option", "-p", "-t", firstPaneFields[0], "--", agentruntime.PaneOption, firstRegistration); err != nil {
		t.Fatal("install first runtime registration")
	}
	firstInventory, err := manager.List(ctx)
	if err != nil {
		t.Fatal("list first registered runtime")
	}
	registeredFirst := requireSessionID(t, firstInventory, first.TmuxID)
	registeredFirstAgent := requireAgentProjection(t, registeredFirst)
	if registeredFirstAgent.Agent.Provider != agentruntime.ProviderCodex ||
		registeredFirstAgent.Agent.PID != processinfo.PID(firstPanePID) ||
		registeredFirstAgent.Agent.Profile != "personal" ||
		registeredFirstAgent.Agent.ProviderSession == nil ||
		registeredFirstAgent.Agent.ProviderSession.ID() != "first-server-session" ||
		registeredFirstAgent.Agent.ProviderSession.Name() != "" {
		t.Fatalf(
			"first runtime registration was not exact: provider_match=%t pid_match=%t profile_match=%t session_present=%t",
			registeredFirstAgent.Agent.Provider == agentruntime.ProviderCodex,
			registeredFirstAgent.Agent.PID == processinfo.PID(firstPanePID),
			registeredFirstAgent.Agent.Profile == "personal",
			registeredFirstAgent.Agent.ProviderSession != nil,
		)
	}
	if err := manager.Kill(ctx, sessions.KillInput{TmuxID: first.TmuxID, TmuxName: first.TmuxName, IdentityToken: first.IdentityToken}); err != nil {
		t.Fatal("kill first exact lifetime fixture")
	}
	deadline := time.Now().Add(tmuxCleanupTimeout)
	for processStartIdentity(firstServer.pid) == firstServer.kernelStartTime {
		if time.Now().After(deadline) {
			t.Fatal("first isolated tmux server did not exit")
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	cleanup = nil

	secondObserved, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalTmuxName: first.TmuxName})
	if err != nil {
		t.Fatal("recreate lifetime fixture")
	}
	second := secondObserved.Session
	cleanup = &second
	if second.TmuxID != first.TmuxID || second.TmuxName != first.TmuxName || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"fixture did not recycle id and name under a new lifetime: id_match=%t name_match=%t identity_changed=%t",
			second.TmuxID == first.TmuxID,
			second.TmuxName == first.TmuxName,
			second.IdentityToken != first.IdentityToken,
		)
	}
	recreatedInventory, err := manager.List(ctx)
	if err != nil {
		t.Fatal("list recreated runtime")
	}
	recreated := requireSessionID(t, recreatedInventory, second.TmuxID)
	recreatedAgent := requireAgentProjection(t, recreated)
	if recreatedAgent.Agent.Provider != agentruntime.ProviderCodex ||
		recreatedAgent.Agent.Profile != "" || recreatedAgent.Agent.ProviderSession != nil {
		t.Fatalf(
			"recreated runtime inherited prior registration facts: provider_match=%t profile_absent=%t session_absent=%t",
			recreatedAgent.Agent.Provider == agentruntime.ProviderCodex,
			recreatedAgent.Agent.Profile == "",
			recreatedAgent.Agent.ProviderSession == nil,
		)
	}
	firstIdentityFields := strings.Split(first.IdentityToken, ".")
	secondIdentityFields := strings.Split(second.IdentityToken, ".")
	if len(firstIdentityFields) != 4 || len(secondIdentityFields) != 4 {
		t.Fatalf("fixture returned malformed lifetime identities: first_fields=%d second_fields=%d", len(firstIdentityFields), len(secondIdentityFields))
	}
	currentCharacter, err := conditionalClient.Output(ctx, "read-recycled-character", "show-options", "-qv", "-t", second.TmuxID, "@skid_character")
	if err != nil || currentCharacter != second.Character.Key {
		t.Fatalf("read recycled session character before stale write: card=%q persisted=%q error=%v", second.Character.Key, currentCharacter, err)
	}
	staleWriteCandidate := "norse.durinn"
	if currentCharacter == staleWriteCandidate {
		staleWriteCandidate = "norse.modsognir"
	}
	committed, err := conditionalClient.AssignCharacterIfUnchanged(ctx, second.TmuxID, currentCharacter, staleWriteCandidate, tmuxclient.ServerIdentity{
		Epoch: firstIdentityFields[0], PID: firstIdentityFields[1], StartTime: firstIdentityFields[2],
	})
	if err != nil {
		t.Fatalf("attempt character write with stale server lifetime: %v", err)
	}
	if committed {
		t.Fatalf("stale server lifetime wrote recycled session: first=%+v second=%+v", first, second)
	}
	persistedCharacter, err := conditionalClient.Output(ctx, "reread-recycled-character", "show-options", "-qv", "-t", second.TmuxID, "@skid_character")
	if err != nil {
		t.Fatalf("read recycled session character after stale write: %v", err)
	}
	if persistedCharacter != currentCharacter {
		t.Fatalf("stale server lifetime changed recycled session character: want=%q got=%q", currentCharacter, persistedCharacter)
	}
	oldEpoch := firstIdentityFields[0]
	if output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath,
		"set-option", "-s", tmuxclient.ServerEpochOption, oldEpoch).CombinedOutput(); err != nil {
		t.Fatalf("restore old epoch on isolated replacement server: output_bytes=%d", len(output))
	}
	restored := second
	restored.IdentityToken = strings.Join([]string{oldEpoch, secondIdentityFields[1], secondIdentityFields[2], secondIdentityFields[3]}, ".")
	cleanup = &restored
	listed, err := manager.List(ctx)
	if err != nil {
		t.Fatal("list replacement after old epoch restoration")
	}
	second = requireSessionID(t, listed, second.TmuxID)
	cleanup = &second
	if !strings.HasPrefix(second.IdentityToken, oldEpoch+".") || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"built-in server lifetime did not distinguish restored epoch: epoch_restored=%t identity_changed=%t",
			strings.HasPrefix(second.IdentityToken, oldEpoch+"."),
			second.IdentityToken != first.IdentityToken,
		)
	}
	err = manager.Kill(ctx, sessions.KillInput{TmuxID: second.TmuxID, TmuxName: second.TmuxName, IdentityToken: first.IdentityToken})
	assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
	listed, err = manager.List(ctx)
	if err != nil {
		t.Fatal("list after stale lifetime kill")
	}
	requireSessionID(t, listed, second.TmuxID)
}

type sessionFixture struct {
	manager       *sessions.Manager
	root          string
	home          string
	project       string
	agent         string
	cataloguePath string
	nodePath      string
	nodeScript    string
	profileHomes  map[string]string
	socket        string
	socketPath    string
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()

	root := t.TempDir()
	home := filepath.Join(root, "service home")
	project := filepath.Join(home, "project with spaces")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal("create project fixture")
	}
	cataloguePath := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(cataloguePath, []byte(`[
  {"key":"norse.modsognir","displayName":"Móðsognir"},
  {"key":"norse.durinn","displayName":"Durinn"}
]`), 0o600); err != nil {
		t.Fatal("write catalogue fixture")
	}

	profileHomes := map[string]string{}
	for _, name := range []string{"personal", "work", "work2", "claude-work"} {
		profileHomes[name] = filepath.Join(root, "profile-"+name)
		if err := os.Mkdir(profileHomes[name], 0o700); err != nil {
			t.Fatalf("create %s profile home", name)
		}
	}
	agent := filepath.Join(root, "agent-fixture")
	if err := os.WriteFile(agent, []byte(`#!/bin/sh
set -eu
/usr/bin/printf '%s\n' "$@" > "$CODEX_HOME/observed-argv"
/usr/bin/printf '%s\n' "$CODEX_HOME" > "$CODEX_HOME/observed-home"
exec /bin/sleep 300
`), 0o700); err != nil {
		t.Fatal("write agent fixture")
	}
	claudeAgent := filepath.Join(root, "claude-agent-fixture")
	if err := os.WriteFile(claudeAgent, []byte(`#!/bin/bash
set -eu
/usr/bin/printf '%s\n' "$@" > "$CLAUDE_CONFIG_DIR/observed-argv"
/usr/bin/printf '%s\n' "$CLAUDE_CONFIG_DIR" > "$CLAUDE_CONFIG_DIR/observed-home"
pwd -P > "$CLAUDE_CONFIG_DIR/observed-cwd"
runtime="$CLAUDE_CONFIG_DIR/claude"
exec -a "$runtime" /bin/bash -c 'while :; do /bin/sleep 300; done' "$runtime" "$@"
`), 0o700); err != nil {
		t.Fatal("write Claude agent fixture")
	}
	nodeScript := filepath.Join(root, "owned-codex.js")
	if err := os.WriteFile(nodeScript, []byte("setInterval(() => {}, 300000)\n"), 0o600); err != nil {
		t.Fatal("write node foreground fixture")
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("resolve Node foreground fixture")
	}

	socket := randomTmuxSocketName(t, "skid-s1")
	socketPath := namedTmuxSocketPath(socket)
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("isolated tmux socket unexpectedly exists before test")
	}
	bootstrapName := "skid-test-bootstrap"
	if output, err := isolatedTmuxCommand(tmuxPath, "-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", bootstrapName, "-c", project, "--", sleepPath, "300").CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux fixture server: output_bytes=%d", len(output))
	}
	serverIdentity := captureTestTmuxServer(t, tmuxPath, socketPath)
	t.Cleanup(func() {
		stopTestTmuxServer(t, tmuxPath, socketPath, serverIdentity)
	})
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    socket,
		Home:          home,
		CataloguePath: cataloguePath,
		Profiles: []agentruntime.Profile{
			{Key: "personal", Label: "Codex · Personal", Provider: agentruntime.ProviderCodex, Command: agent, Environment: []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["personal"]}}, ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work", Label: "Codex · Work", Provider: agentruntime.ProviderCodex, Command: agent, Environment: []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work"]}}, ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work2", Label: "Codex · Work 2", Provider: agentruntime.ProviderCodex, Command: agent, Environment: []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work2"]}}, ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "claude-work", Label: "Claude · Work", Provider: agentruntime.ProviderClaude, Command: claudeAgent, Environment: []agentruntime.EnvironmentVariable{{Name: "CLAUDE_CONFIG_DIR", Value: profileHomes["claude-work"]}}, ForegroundSignatures: []agentruntime.ForegroundSignature{{Argument0: filepath.Join(profileHomes["claude-work"], "claude")}}, Arguments: []string{"--permission-mode", "auto"}},
		},
	})
	if err != nil {
		t.Fatal("construct session manager")
	}

	return sessionFixture{
		manager:       manager,
		root:          root,
		home:          home,
		project:       project,
		agent:         agent,
		cataloguePath: cataloguePath,
		nodePath:      nodePath,
		nodeScript:    nodeScript,
		profileHomes:  profileHomes,
		socket:        socket,
		socketPath:    socketPath,
	}
}

func (fixture sessionFixture) attachClient(t *testing.T, session string) {
	t.Helper()
	command := isolatedTmuxCommand(tmuxPath, "-S", fixture.socketPath, "-f", "/dev/null", "attach-session", "-t", session)
	command.Env = append(withoutEnvironment(command.Env, "TERM"), "TERM=xterm-256color")
	terminalPTY, err := pty.Start(command)
	if err != nil {
		t.Fatal("start real test-owned tmux client")
	}
	done := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.Discard, terminalPTY)
		done <- command.Wait()
	}()
	t.Cleanup(func() {
		_ = terminalPTY.Close()
		if command.Process != nil {
			_ = command.Process.Kill() // justify-ignore-error: cleanup accepts an already-detached test-owned PTY helper.
		}
		select {
		case <-done:
		case <-time.After(tmuxCleanupTimeout):
			t.Error("test-owned tmux PTY helper did not exit")
		}
	})
}

// waitForAgent observes process-start convergence; it never participates in
// terminal-activity derivation.
func (fixture sessionFixture) waitForAgent(t *testing.T, ctx context.Context, name string) sessions.Session {
	t.Helper()
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("session %s convergence observation exceeded %s", name, tmuxConvergenceTimeout)
		}
		pollContext, cancel := context.WithTimeout(ctx, remaining)
		listed, err := fixture.manager.List(pollContext)
		pollContextError := pollContext.Err()
		cancel()
		if err != nil {
			if errors.Is(pollContextError, context.DeadlineExceeded) {
				t.Fatalf("session %s convergence observation exceeded %s", name, tmuxConvergenceTimeout)
			}
			t.Fatalf("list while waiting for session %s", name)
		}
		observed := requireSessionNamed(t, listed, name)
		if observed.Agent != nil {
			return observed
		}
		if time.Now().After(deadline) {
			t.Fatalf("session %s did not project agent identity", name)
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
}

// waitForActivities waits only for the host's fixed activity window to cross
// or for emitted terminal output to become observable through the next poll.
func (fixture sessionFixture) waitForActivities(
	t *testing.T,
	ctx context.Context,
	want sessions.SessionActivity,
	names ...string,
) sessions.Inventory {
	t.Helper()
	deadline := time.Now().Add(activityConvergenceTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("activity convergence exceeded %s: want=%s sessions=%v", activityConvergenceTimeout, want, names)
		}
		pollContext, cancel := context.WithTimeout(ctx, remaining)
		listed, err := fixture.manager.List(pollContext)
		pollContextError := pollContext.Err()
		cancel()
		if err != nil {
			if errors.Is(pollContextError, context.DeadlineExceeded) {
				t.Fatalf("activity convergence exceeded %s: want=%s sessions=%v", activityConvergenceTimeout, want, names)
			}
			t.Fatalf("list while waiting for %s activity: %v", want, err)
		}
		converged := true
		for _, name := range names {
			observed := requireSessionNamed(t, listed, name)
			if observed.Activity != sessions.SessionActivityActive && observed.Activity != sessions.SessionActivityQuiet {
				t.Fatalf("session %s exposed activity outside the closed union: %q", name, observed.Activity)
			}
			converged = converged && observed.Activity == want
		}
		if converged {
			return listed
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
}

func (fixture sessionFixture) tmux(t *testing.T, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-L", fixture.socket, "-f", "/dev/null"}, args...)
	output, err := isolatedTmuxCommand(tmuxPath, commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("test-owned tmux command failed: argument_count=%d output_bytes=%d", len(args), len(output))
	}
	return strings.TrimSuffix(string(output), "\n")
}

func assertSessionError(t *testing.T, err error, want sessions.ErrorCode) {
	t.Helper()
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		t.Fatalf("expected sessions.Error %s: type_match=false", want)
	}
	if sessionError.Code != want {
		t.Fatalf("wrong sessions error code: want=%s got=%s", want, sessionError.Code)
	}
}

func sessionNonCharacterSnapshot(t *testing.T, fixture sessionFixture, id string) string {
	t.Helper()
	session := fixture.tmux(t, "display-message", "-p", "-t", id,
		"#{session_id}|#{session_name}|#{session_attached}|#{session_group}|#{session_group_size}|#{session_group_attached}|#{window_id}|#{pane_id}|#{@skid_internal}|#{@skid_profile}|#{@skid_objective_b64}")
	windows := strings.Split(fixture.tmux(t, "list-windows", "-t", id, "-F",
		"#{session_id}|#{window_id}|#{window_index}|#{window_active}|#{window_name}|#{window_panes}|#{window_width}|#{window_height}|#{window_layout}|#{window_active_clients}"), "\n")
	panes := strings.Split(fixture.tmux(t, "list-panes", "-s", "-t", id, "-F",
		"#{session_id}|#{window_id}|#{pane_id}|#{pane_index}|#{pane_active}|#{pane_pid}|#{pane_width}|#{pane_height}|#{pane_current_path}|#{pane_current_command}"), "\n")
	clients := make([]string, 0)
	for _, client := range strings.Split(fixture.tmux(t, "list-clients", "-F",
		"#{client_name}|#{session_id}|#{session_name}|#{client_width}|#{client_height}"), "\n") {
		fields := strings.Split(client, "|")
		if len(fields) == 5 && fields[1] == id {
			clients = append(clients, client)
		}
	}
	slices.Sort(windows)
	slices.Sort(panes)
	slices.Sort(clients)
	sessionOptions := make([]string, 0)
	for _, option := range strings.Split(fixture.tmux(t, "show-options", "-t", id), "\n") {
		if option != "" && !strings.HasPrefix(option, "@skid_character ") {
			sessionOptions = append(sessionOptions, option)
		}
	}
	windowOptions := make([]string, 0)
	for _, window := range windows {
		fields := strings.Split(window, "|")
		if len(fields) < 2 {
			t.Fatalf("malformed test-owned window snapshot: %q", window)
		}
		for _, option := range strings.Split(fixture.tmux(t, "show-options", "-w", "-t", fields[1]), "\n") {
			if option != "" {
				windowOptions = append(windowOptions, fields[1]+"|"+option)
			}
		}
	}
	paneOptions := make([]string, 0)
	for _, pane := range panes {
		fields := strings.Split(pane, "|")
		if len(fields) < 3 {
			t.Fatalf("malformed test-owned pane snapshot: %q", pane)
		}
		for _, option := range strings.Split(fixture.tmux(t, "show-options", "-p", "-t", fields[2]), "\n") {
			if option != "" {
				paneOptions = append(paneOptions, fields[2]+"|"+option)
			}
		}
	}
	slices.Sort(sessionOptions)
	slices.Sort(windowOptions)
	slices.Sort(paneOptions)
	return strings.Join([]string{
		"session=" + session,
		"windows=" + strings.Join(windows, "\n"),
		"panes=" + strings.Join(panes, "\n"),
		"clients=" + strings.Join(clients, "\n"),
		"session-options=" + strings.Join(sessionOptions, "\n"),
		"window-options=" + strings.Join(windowOptions, "\n"),
		"pane-options=" + strings.Join(paneOptions, "\n"),
	}, "\n")
}

func requireValidCharacter(t *testing.T, session sessions.Session) {
	t.Helper()
	wantDisplayName := map[string]string{
		"norse.modsognir": "Móðsognir",
		"norse.durinn":    "Durinn",
	}[session.Character.Key]
	if wantDisplayName == "" || session.Character.DisplayName != wantDisplayName {
		t.Fatalf("session has no valid required character: %+v", session)
	}
}

func expectedTestCharacter(epoch, id string, usage map[string]int) string {
	const modsognir = "norse.modsognir"
	const durinn = "norse.durinn"
	if usage[modsognir] < usage[durinn] {
		return modsognir
	}
	if usage[durinn] < usage[modsognir] {
		return durinn
	}
	modsognirScore := sha256.Sum256([]byte(epoch + "\x00" + id + "\x00" + modsognir))
	durinnScore := sha256.Sum256([]byte(epoch + "\x00" + id + "\x00" + durinn))
	if bytes.Compare(modsognirScore[:], durinnScore[:]) >= 0 {
		return modsognir
	}
	return durinn
}

func requireSessionNamed(t *testing.T, listed sessions.Inventory, name string) sessions.Session {
	t.Helper()
	for _, session := range listed.Sessions {
		if session.TmuxName == name {
			return session
		}
	}
	t.Fatalf("expected session name not listed: session_count=%d", len(listed.Sessions))
	return sessions.Session{}
}

func requireSessionID(t *testing.T, listed sessions.Inventory, id string) sessions.Session {
	t.Helper()
	for _, session := range listed.Sessions {
		if session.TmuxID == id {
			return session
		}
	}
	t.Fatalf("expected session id not listed: session_count=%d", len(listed.Sessions))
	return sessions.Session{}
}

type requiredAgentProjection struct {
	Agent agentruntime.AgentRuntime
}

func requireAgentProjection(t *testing.T, session sessions.Session) requiredAgentProjection {
	t.Helper()
	if session.Agent == nil {
		t.Fatal("session omitted expected optional agent identity")
	}
	return requiredAgentProjection{Agent: *session.Agent}
}

func waitForFileLine(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	for {
		contents, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(contents)) == want {
			return
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read process-observation fixture")
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"process observation did not converge: read_ok=%t value_match=%t content_bytes=%d",
				err == nil,
				strings.TrimSpace(string(contents)) == want,
				len(contents),
			)
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
}

func withoutEnvironment(environment []string, names ...string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(names, name) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
