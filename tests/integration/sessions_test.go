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

	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/creack/pty"
)

const (
	sleepPath              = "/bin/sleep"
	yoloFlag               = "--dangerously-bypass-approvals-and-sandbox"
	tmuxConvergenceTimeout = 5 * time.Second
	tmuxCleanupTimeout     = 3 * time.Second

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
		Profiles: []sessions.Profile{{
			Key:                  "future",
			Label:                "Future provider",
			Command:              filepath.Join(root, "not-installed"),
			ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "future-agent"}},
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
		Profiles: []sessions.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Command:              agent,
			Environment:          []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHome}},
			ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
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
		if len(listed) != len(duplicateIDs)+len(unassignedIDs) {
			t.Fatalf("controlled inventory size = %d, want %d: %+v", len(listed), len(duplicateIDs)+len(unassignedIDs), listed)
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
		for _, session := range listed {
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
				if len(after) != len(before) {
					t.Fatalf("rejected %s mutated isolated tmux: before_count=%d after_count=%d", test.name, len(before), len(after))
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
			Profiles: []sessions.Profile{{
				Key:                  "personal",
				Label:                "Personal",
				Command:              fixture.agent,
				Environment:          []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
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
		if laptop.Profile != "" || laptop.Objective != "" || invalid.Profile != "" || invalid.Objective != "" {
			t.Fatalf("inventory guessed profile or objective metadata: laptop=%+v invalid=%+v", laptop, invalid)
		}
		if laptop.AttachedClients != 1 {
			t.Fatalf("inventory did not preserve the test-owned laptop client: %+v", laptop)
		}
		for _, session := range []sessions.Session{laptop, invalid} {
			if persisted := fixture.tmux(t, "show-options", "-qv", "-t", session.ID, "@skid_character"); persisted != session.Character.Key {
				t.Fatalf("required character was not persisted: session=%+v persisted=%q", session, persisted)
			}
		}
		if after := sessionNonCharacterSnapshot(t, fixture, laptopID); after != laptopBefore {
			t.Fatalf("automatic assignment changed laptop facts:\nbefore=%q\n after=%q", laptopBefore, after)
		}
		if after := sessionNonCharacterSnapshot(t, fixture, invalidID); after != invalidBefore {
			t.Fatalf("invalid-character repair changed non-character facts:\nbefore=%q\n after=%q", invalidBefore, after)
		}
		if laptop.Status.Kind != sessions.StatusRunning || laptop.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("uninstrumented allowlisted laptop process should report only process liveness: %+v", laptop.Status)
		}
		shell := fixture.waitForSession(t, ctx, "shell", sessions.StatusShell)
		if shell.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("ordinary laptop shell should be reported literally: %+v", shell.Status)
		}

		repeated, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("repeat laptop-created inventory: %v", err)
		}
		if observed := requireSessionID(t, repeated, laptop.ID); observed.Character != laptop.Character {
			t.Fatalf("repeated inventory changed persisted character: first=%+v repeated=%+v", laptop, observed)
		}
		if observed := requireSessionID(t, repeated, invalid.ID); observed.Character != invalid.Character {
			t.Fatalf("repeated inventory changed repaired character: first=%+v repeated=%+v", invalid, observed)
		}

		panePID := fixture.tmux(t, "display-message", "-p", "-t", laptop.ID, "#{pane_pid}")
		fixture.tmux(t, "rename-session", "-t", laptop.ID, "laptop-renamed")
		fixture.tmux(t, "respawn-pane", "-k", "-t", laptop.ID, "--", sleepPath, "300")
		if replacementPID := fixture.tmux(t, "display-message", "-p", "-t", laptop.ID, "#{pane_pid}"); replacementPID == panePID {
			t.Fatalf("test-owned process replacement retained pane PID %s", panePID)
		}
		afterReplacement, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list renamed process replacement: %v", err)
		}
		replaced := requireSessionID(t, afterReplacement, laptop.ID)
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
		for _, session := range baseline {
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
			Profiles: []sessions.Profile{{
				Key: "personal", Label: "Codex · Personal", Command: fixture.agent,
				Environment:          []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
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
			Profiles: []sessions.Profile{{
				Key: "personal", Label: "Codex · Personal", Command: fixture.agent,
				Environment:          []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
				ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
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
		for _, session := range listed {
			if session.ID == targetID {
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

	t.Run("foreground signatures are exact and profile metadata is never inferred", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "owned-node", "-c", fixture.project, "--", fixture.nodePath, fixture.nodeScript)
		fixture.tmux(t, "new-session", "-d", "-s", "plain-node", "-c", fixture.project, "--", fixture.nodePath, "-e", "setInterval(() => {}, 300000)")

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list exact foreground signature fixtures")
		}
		for _, name := range []string{"owned-node"} {
			session := requireSessionNamed(t, listed, name)
			if session.Status.Kind != sessions.StatusRunning || session.Profile != "" {
				t.Fatalf("exact signature status mismatch: kind=%s profile_present=%t", session.Status.Kind, session.Profile != "")
			}
		}
		plainNode := requireSessionNamed(t, listed, "plain-node")
		if plainNode.Status.Kind != sessions.StatusShell || plainNode.Profile != "" {
			t.Fatalf("arbitrary node process matched an owned signature: kind=%s profile_present=%t", plainNode.Status.Kind, plainNode.Profile != "")
		}
	})

	t.Run("card facts come only from the current window active pane", func(t *testing.T) {
		otherDirectory := filepath.Join(fixture.root, "other pane")
		if err := os.Mkdir(otherDirectory, 0o700); err != nil {
			t.Fatal("create conflicting pane cwd")
		}
		fixture.tmux(t, "new-session", "-d", "-s", "anchor", "-c", fixture.project, "--", sleepPath, "300")
		activePane := fixture.tmux(t, "display-message", "-p", "-t", "anchor:0.0", "#{pane_id}")
		fixture.tmux(t, "split-window", "-d", "-t", "anchor:0", "-c", otherDirectory)
		inactivePane := fixture.tmux(t, "display-message", "-p", "-t", "anchor:0.1", "#{pane_id}")
		fixture.tmux(t, "set-option", "-p", "-t", inactivePane, "--", "@skid_attention", fmt.Sprint(time.Now().Unix()))
		fixture.tmux(t, "new-window", "-d", "-t", "anchor", "-n", "other", "-c", otherDirectory)
		fixture.tmux(t, "select-window", "-t", "anchor:0")
		fixture.tmux(t, "select-pane", "-t", activePane)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list multi-window anchor fixture")
		}
		anchor := requireSessionNamed(t, listed, "anchor")
		if anchor.CWD != fixture.project || anchor.ActiveCommand != "sleep" || anchor.Attention || anchor.Status.Kind != sessions.StatusRunning {
			t.Fatalf(
				"inactive pane/window facts leaked into card anchor: cwd_match=%t command_match=%t attention=%t kind=%s",
				anchor.CWD == fixture.project,
				anchor.ActiveCommand == "sleep",
				anchor.Attention,
				anchor.Status.Kind,
			)
		}
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
			if slices.ContainsFunc(listed, func(session sessions.Session) bool { return session.TmuxName == phoneShadow }) {
				t.Fatalf("gateway-owned phone shadow leaked into inventory: %+v", listed)
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
		persistedCharacter := fixture.tmux(t, "show-options", "-qv", "-t", reclaimed.ID, "@skid_character")
		if persistedCharacter != reclaimed.Character.Key {
			t.Fatalf("reclaimed last link did not persist its character: session=%+v persisted=%q", reclaimed, persistedCharacter)
		}
		repeated, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("repeat reclaimed last-link inventory: %v", err)
		}
		repeatedReclaimed := requireSessionID(t, repeated, reclaimed.ID)
		if repeatedReclaimed.Character != reclaimed.Character {
			t.Fatalf("repeated inventory changed reclaimed character: first=%+v repeated=%+v", reclaimed, repeatedReclaimed)
		}
		if persisted := fixture.tmux(t, "show-options", "-qv", "-t", reclaimed.ID, "@skid_character"); persisted != persistedCharacter {
			t.Fatalf("repeated inventory changed reclaimed persisted character: before=%q after=%q", persistedCharacter, persisted)
		}
	})

	t.Run("an enumerated pane with an unobservable process is unknown", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "unknown", "-c", fixture.project, "--", "/bin/sh")
		fixture.tmux(t, "set-option", "-w", "-t", "unknown", "remain-on-exit", "on")
		fixture.tmux(t, "send-keys", "-t", "unknown", "exit", "Enter")

		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for {
			listed, err := fixture.manager.List(ctx)
			if err != nil {
				t.Fatal("list process-observation failure fixture")
			}
			unknown := requireSessionNamed(t, listed, "unknown")
			if unknown.Status.Kind == sessions.StatusUnknown {
				requireValidCharacter(t, unknown)
				if unknown.Status.Signal != sessions.StatusSignalPollFailure {
					t.Fatalf("unknown status signal mismatch: signal=%s", unknown.Status.Signal)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("dead pane status did not converge: kind=%s signal=%s", unknown.Status.Kind, unknown.Status.Signal)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
	})

	t.Run("create keeps tmux names independent from balanced required characters", func(t *testing.T) {
		beforeCreate, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list before managed creates: %v", err)
		}
		liveCharacterUse := map[string]int{"norse.modsognir": 0, "norse.durinn": 0}
		for _, session := range beforeCreate {
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
		first, err := fixture.manager.Create(ctx, sessions.CreateInput{
			CWD:       "~/project with spaces",
			Profile:   "personal",
			Objective: "Inspect Ω",
		})
		if err != nil {
			t.Fatalf("create first generated session: %v", err)
		}
		requireCreatedLeastUsed("first generated", first)
		second, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work"})
		if err != nil {
			t.Fatalf("create second generated session: %v", err)
		}
		requireCreatedLeastUsed("second generated", second)
		third, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work2"})
		if err != nil {
			t.Fatalf("create generated session beyond catalogue base names: %v", err)
		}
		requireCreatedLeastUsed("third generated", third)
		custom, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "hand_named"})
		if err != nil {
			t.Fatalf("create custom named session: %v", err)
		}
		requireCreatedLeastUsed("custom named", custom)
		claude, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "claude-work"})
		if err != nil {
			t.Fatalf("create Claude work session: %v", err)
		}
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
		if first.Profile != "personal" || first.Objective != "Inspect Ω" || first.CWD != fixture.project {
			t.Fatalf("managed session facts mismatch: %+v", first)
		}

		for _, profile := range []string{"personal", "work", "work2"} {
			argvPath := filepath.Join(fixture.profileHomes[profile], "observed-argv")
			waitForFileLine(t, argvPath, yoloFlag)
			waitForFileLine(t, filepath.Join(fixture.profileHomes[profile], "observed-home"), fixture.profileHomes[profile])
		}
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-argv"), "--permission-mode\nauto")
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-home"), fixture.profileHomes["claude-work"])
		waitForFileLine(t, filepath.Join(fixture.profileHomes["claude-work"], "observed-cwd"), fixture.project)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after managed creates: %v", err)
		}
		for _, created := range []sessions.Session{first, second, third, custom, claude} {
			observed := requireSessionID(t, listed, created.ID)
			if observed.TmuxName != created.TmuxName || observed.Profile != created.Profile ||
				observed.Character != created.Character || observed.IdentityToken != created.IdentityToken {
				t.Fatalf("create response did not converge to tmux inventory: created=%+v listed=%+v", created, observed)
			}
		}
		characterUse := map[string]int{"norse.modsognir": 0, "norse.durinn": 0}
		for _, session := range listed {
			requireValidCharacter(t, session)
			characterUse[session.Character.Key]++
		}
		if difference := characterUse["norse.modsognir"] - characterUse["norse.durinn"]; difference < -1 || difference > 1 {
			t.Fatalf("live character allocation is not balanced: %+v", characterUse)
		}
	})

	t.Run("the real status hook publishes the exact foreground lifetime that drives working and idle", func(t *testing.T) {
		hookCommand := buildSkidbladnirCommand(t, fixture.repositoryRoot, fixture.root)
		codexCommand := buildForegroundCodexCommand(t, fixture.root)
		hostConfig := writeStatusHookHostConfig(t, fixture.root, codexCommand)
		hookEvents := filepath.Join(fixture.root, "codex-hook-events")
		if err := os.Mkdir(hookEvents, 0o700); err != nil {
			t.Fatalf("create the status-hook event directory: %v", err)
		}

		const sessionName = "hook-lifetime"
		fixture.tmux(t, "new-session", "-d", "-s", sessionName, "-c", fixture.project, "--", codexCommand, hookCommand, hostConfig, hookEvents)
		target := fixture.waitForSession(t, ctx, sessionName, sessions.StatusRunning)
		if target.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("foreground Codex lifetime without hook evidence signal mismatch: kind=%s signal=%s", target.Status.Kind, target.Status.Signal)
		}
		paneID := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_id}")
		panePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"))
		if err != nil {
			t.Fatalf("parse the foreground Codex pane pid: %v", err)
		}
		lifetime := string(processStartIdentity(panePID))
		if lifetime == "" {
			t.Fatal("observe the foreground Codex process start identity")
		}

		endedAt := requestHookEvent(t, hookEvents, "Stop")
		requireLifecycleFact(t, fixture.tmux(t, "show-options", "-pqv", "-t", paneID, "@skid_lifecycle"), "idle", panePID, lifetime, endedAt)
		if got := readHookResponse(t, hookEvents, "Stop"); got != "{}\n" {
			t.Fatalf("Stop hook response = %q, want the Codex acknowledgement {}", got)
		}
		fixture.tmux(t, "send-keys", "-t", target.ID, "activity-without-output")
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after pane input activity")
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusIdle || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("tmux activity changed the published idle lifecycle fact: kind=%s signal=%s", target.Status.Kind, target.Status.Signal)
		}

		fixture.tmux(t, "set-option", "-p", "-t", paneID, "--", "@skid_attention", fmt.Sprint(time.Now().Unix()))
		startedAt := requestHookEvent(t, hookEvents, "UserPromptSubmit")
		requireLifecycleFact(t, fixture.tmux(t, "show-options", "-pqv", "-t", paneID, "@skid_lifecycle"), "working", panePID, lifetime, startedAt)
		if got := readHookResponse(t, hookEvents, "UserPromptSubmit"); got != "" {
			t.Fatalf("UserPromptSubmit hook wrote %d response bytes, want silence", len(got))
		}
		if got := fixture.tmux(t, "show-options", "-pqv", "-t", paneID, "@skid_attention"); got != "" {
			t.Fatalf("a new Codex turn left the pane attention flag set: attention_bytes=%d", len(got))
		}
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after the working lifecycle fact")
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusWorking || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("published working lifecycle fact did not project: kind=%s signal=%s", target.Status.Kind, target.Status.Signal)
		}
	})

	t.Run("kill rechecks exact id and tmux name", func(t *testing.T) {
		victim, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "kill_victim"})
		if err != nil {
			t.Fatal("create kill victim")
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalTmuxName: "kill_survivor"})
		if err != nil {
			t.Fatal("create kill survivor")
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, TmuxName: survivor.TmuxName, IdentityToken: victim.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
		listed, listErr := fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatal("list after refused mismatched kill")
		}
		requireSessionID(t, listed, victim.ID)
		requireSessionID(t, listed, survivor.ID)

		missingToken := strings.TrimSuffix(victim.IdentityToken, strings.TrimPrefix(victim.ID, "$")) + "999999"
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "$999999", TmuxName: victim.TmuxName, IdentityToken: missingToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "kill_survivor", TmuxName: survivor.TmuxName, IdentityToken: survivor.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, TmuxName: victim.TmuxName, IdentityToken: victim.IdentityToken}); err != nil {
			t.Fatal("kill exact confirmed session")
		}
		listed, listErr = fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatal("list after exact kill")
		}
		if slices.ContainsFunc(listed, func(session sessions.Session) bool { return session.ID == victim.ID }) {
			t.Fatalf("exactly killed session remains listed: session_count=%d", len(listed))
		}
		requireSessionID(t, listed, survivor.ID)
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
		panePID := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}")
		if siblingPID := fixture.tmux(t, "display-message", "-p", "-t", sibling.ID, "#{pane_pid}"); siblingPID != panePID {
			t.Fatal("grouped kill fixture does not share one pane process")
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, TmuxName: target.TmuxName, IdentityToken: target.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionGroupedConflict)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after refused grouped kill")
		}
		if observed := requireSessionID(t, listed, target.ID); observed.TmuxName != target.TmuxName {
			t.Fatal("refused grouped kill changed target identity")
		}
		if observed := requireSessionID(t, listed, sibling.ID); observed.TmuxName != sibling.TmuxName {
			t.Fatal("refused grouped kill changed sibling identity")
		}
		if after := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"); after != panePID {
			t.Fatal("refused grouped kill changed shared pane process")
		}
	})

	t.Run("one kill reconciles every stale owned shadow before ending only the source", func(t *testing.T) {
		target, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalTmuxName: "shadowed_kill_target"})
		if err != nil {
			t.Fatal("create shadowed kill target")
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalTmuxName: "shadowed_kill_survivor"})
		if err != nil {
			t.Fatal("create shadowed kill survivor")
		}
		panePIDText := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}")
		panePID, err := strconv.Atoi(panePIDText)
		if err != nil {
			t.Fatal("parse shadowed target pane pid")
		}
		paneStartTime := processStartIdentity(panePID)
		if paneStartTime == "" {
			t.Fatal("capture shadowed target pane process identity")
		}
		survivorPanePIDText := fixture.tmux(t, "display-message", "-p", "-t", survivor.ID, "#{pane_pid}")
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
			fixture.tmux(t, "new-session", "-d", "-t", target.ID, "-s", shadowName)
			fixture.tmux(t, "set-option", "-t", "="+shadowName+":", "--", "@skid_internal", "phone-shadow")
		}

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, TmuxName: target.TmuxName, IdentityToken: target.IdentityToken}); err != nil {
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
		if observed := fixture.tmux(t, "display-message", "-p", "-t", survivor.ID, "#{pane_pid}"); observed != survivorPanePIDText || processStartIdentity(survivorPanePID) != survivorPaneStartTime {
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
		Profiles: []sessions.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Command:              sleepPath,
			ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
			Arguments:            []string{"300"},
		}},
	})
	if err != nil {
		t.Fatal("construct lifetime-token manager")
	}

	ctx := context.Background()
	var cleanup *sessions.Session
	t.Cleanup(func() {
		if cleanup == nil {
			return
		}
		if err := manager.Kill(ctx, sessions.KillInput{ID: cleanup.ID, TmuxName: cleanup.TmuxName, IdentityToken: cleanup.IdentityToken}); err != nil {
			t.Error("kill exact test-owned lifetime fixture")
		}
	})

	first, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalTmuxName: "epoch-reuse"})
	if err != nil {
		t.Fatal("create first lifetime fixture")
	}
	cleanup = &first
	firstServer := captureTestTmuxServer(t, tmuxPath, socketPath)
	if err := manager.Kill(ctx, sessions.KillInput{ID: first.ID, TmuxName: first.TmuxName, IdentityToken: first.IdentityToken}); err != nil {
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

	second, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalTmuxName: first.TmuxName})
	if err != nil {
		t.Fatal("recreate lifetime fixture")
	}
	cleanup = &second
	if second.ID != first.ID || second.TmuxName != first.TmuxName || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"fixture did not recycle id and name under a new lifetime: id_match=%t name_match=%t identity_changed=%t",
			second.ID == first.ID,
			second.TmuxName == first.TmuxName,
			second.IdentityToken != first.IdentityToken,
		)
	}
	firstIdentityFields := strings.Split(first.IdentityToken, ".")
	secondIdentityFields := strings.Split(second.IdentityToken, ".")
	if len(firstIdentityFields) != 4 || len(secondIdentityFields) != 4 {
		t.Fatalf("fixture returned malformed lifetime identities: first_fields=%d second_fields=%d", len(firstIdentityFields), len(secondIdentityFields))
	}
	conditionalClient, err := tmuxclient.New(tmuxPath, socket)
	if err != nil {
		t.Fatalf("construct stale-write tmux client: %v", err)
	}
	currentCharacter, err := conditionalClient.Output(ctx, "read-recycled-character", "show-options", "-qv", "-t", second.ID, "@skid_character")
	if err != nil || currentCharacter != second.Character.Key {
		t.Fatalf("read recycled session character before stale write: card=%q persisted=%q error=%v", second.Character.Key, currentCharacter, err)
	}
	staleWriteCandidate := "norse.durinn"
	if currentCharacter == staleWriteCandidate {
		staleWriteCandidate = "norse.modsognir"
	}
	committed, err := conditionalClient.AssignCharacterIfUnchanged(ctx, second.ID, currentCharacter, staleWriteCandidate, tmuxclient.ServerIdentity{
		Epoch: firstIdentityFields[0], PID: firstIdentityFields[1], StartTime: firstIdentityFields[2],
	})
	if err != nil {
		t.Fatalf("attempt character write with stale server lifetime: %v", err)
	}
	if committed {
		t.Fatalf("stale server lifetime wrote recycled session: first=%+v second=%+v", first, second)
	}
	persistedCharacter, err := conditionalClient.Output(ctx, "reread-recycled-character", "show-options", "-qv", "-t", second.ID, "@skid_character")
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
	second = requireSessionID(t, listed, second.ID)
	cleanup = &second
	if !strings.HasPrefix(second.IdentityToken, oldEpoch+".") || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"built-in server lifetime did not distinguish restored epoch: epoch_restored=%t identity_changed=%t",
			strings.HasPrefix(second.IdentityToken, oldEpoch+"."),
			second.IdentityToken != first.IdentityToken,
		)
	}
	err = manager.Kill(ctx, sessions.KillInput{ID: second.ID, TmuxName: second.TmuxName, IdentityToken: first.IdentityToken})
	assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
	listed, err = manager.List(ctx)
	if err != nil {
		t.Fatal("list after stale lifetime kill")
	}
	requireSessionID(t, listed, second.ID)
}

type sessionFixture struct {
	manager        *sessions.Manager
	repositoryRoot string
	root           string
	home           string
	project        string
	agent          string
	cataloguePath  string
	nodePath       string
	nodeScript     string
	profileHomes   map[string]string
	socket         string
	socketPath     string
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()

	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("resolve repository root")
	}
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
	if err := os.WriteFile(claudeAgent, []byte(`#!/bin/sh
set -eu
/usr/bin/printf '%s\n' "$@" > "$CLAUDE_CONFIG_DIR/observed-argv"
/usr/bin/printf '%s\n' "$CLAUDE_CONFIG_DIR" > "$CLAUDE_CONFIG_DIR/observed-home"
pwd -P > "$CLAUDE_CONFIG_DIR/observed-cwd"
exec /bin/sleep 300
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
		Profiles: []sessions.Profile{
			{Key: "personal", Label: "Codex · Personal", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["personal"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work", Label: "Codex · Work", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work2", Label: "Codex · Work 2", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work2"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "claude-work", Label: "Claude · Work", Command: claudeAgent, Environment: []sessions.EnvironmentVariable{{Name: "CLAUDE_CONFIG_DIR", Value: profileHomes["claude-work"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}}, Arguments: []string{"--permission-mode", "auto"}},
		},
	})
	if err != nil {
		t.Fatal("construct session manager")
	}

	return sessionFixture{
		manager:        manager,
		repositoryRoot: repositoryRoot,
		root:           root,
		home:           home,
		project:        project,
		agent:          agent,
		cataloguePath:  cataloguePath,
		nodePath:       nodePath,
		nodeScript:     nodeScript,
		profileHomes:   profileHomes,
		socket:         socket,
		socketPath:     socketPath,
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

// foregroundCodexProgram is the pane's foreground Codex lifetime for the
// status-hook proof. It owns the pane's foreground process group and runs the
// real skidbladnir status hook as its own child whenever the test publishes an
// event request, which is exactly how Codex invokes its lifecycle hooks. It is
// compiled rather than scripted because the hook and the card projection both
// identify Codex by the kernel's executable, not by a wrapper's arguments.
const foregroundCodexProgram = `package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// justify-polling: this stand-in owns no channel back to the test process, so
// a published request file is the only signal available; 50 ms keeps the
// bounded proof responsive and the loop ends with the pane.
const requestPollInterval = 50 * time.Millisecond

func main() {
	if len(os.Args) != 4 {
		os.Exit(2)
	}
	hook, hostConfig, events := os.Args[1], os.Args[2], os.Args[3]
	for {
		requests, err := filepath.Glob(filepath.Join(events, "*.request"))
		if err != nil {
			os.Exit(3)
		}
		for _, request := range requests {
			base := strings.TrimSuffix(request, ".request")
			event := filepath.Base(base)
			response, runErr := exec.Command(hook, "status-hook", "--host-config="+hostConfig, event).CombinedOutput()
			code := 0
			var exit *exec.ExitError
			if errors.As(runErr, &exit) {
				code = exit.ExitCode()
			} else if runErr != nil {
				code = -1
			}
			if err := os.WriteFile(base+".response", response, 0o600); err != nil {
				os.Exit(4)
			}
			if err := os.WriteFile(base+".status", []byte(strconv.Itoa(code)+"\n"), 0o600); err != nil {
				os.Exit(5)
			}
			if err := os.Remove(request); err != nil {
				os.Exit(6)
			}
		}
		time.Sleep(requestPollInterval)
	}
}
`

// buildSkidbladnirCommand compiles the shipped CLI so the proof exercises the
// real "skidbladnir status-hook" binary instead of a test double of it.
func buildSkidbladnirCommand(t *testing.T, repositoryRoot, destination string) string {
	t.Helper()
	command := filepath.Join(destination, "skidbladnir")
	build := exec.Command(goToolPath(t), "build", "-o", command, "./cmd/skidbladnir")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the skidbladnir status-hook command: output_bytes=%d", len(output))
	}
	return command
}

func writeStatusHookHostConfig(t *testing.T, destination, codexCommand string) string {
	t.Helper()
	path := filepath.Join(destination, "status-hook-host.json")
	tmuxVersionOutput, err := exec.Command(tmuxPath, "-V").Output()
	if err != nil || len(tmuxVersionOutput) < 2 || tmuxVersionOutput[len(tmuxVersionOutput)-1] != '\n' ||
		bytes.ContainsRune(tmuxVersionOutput[:len(tmuxVersionOutput)-1], '\n') {
		t.Fatal("observe the exact tmux version for the status-hook host fixture")
	}
	tmuxVersion := string(tmuxVersionOutput[:len(tmuxVersionOutput)-1])
	encoded := fmt.Sprintf(`{
  "platform":%q,
  "tmux":{"path":%q,"version":%q},
  "codexNodeEntrypoint":%q,
  "profiles":[
    {"key":"personal","label":"Codex · Personal","command":"/bin/false","environment":[],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work","label":"Codex · Work","command":"/bin/false","environment":[],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"work2","label":"Codex · Work 2","command":"/bin/false","environment":[],"foregroundSignatures":[{"executableBase":"codex"}],"arguments":[]},
    {"key":"claude-personal","label":"Claude · Personal","command":"/bin/false","environment":[],"foregroundSignatures":[{"executableBase":"claude"}],"arguments":[]},
    {"key":"claude-work","label":"Claude · Work","command":"/bin/false","environment":[],"foregroundSignatures":[{"executableBase":"claude"}],"arguments":[]}
  ]
}`, platform.Current().Kind, tmuxPath, tmuxVersion, codexCommand)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		t.Fatalf("write status-hook host config: %v", err)
	}
	return path
}

func buildForegroundCodexCommand(t *testing.T, destination string) string {
	t.Helper()
	source := filepath.Join(destination, "codex-source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatalf("create the foreground Codex source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "main.go"), []byte(foregroundCodexProgram), 0o600); err != nil {
		t.Fatalf("write the foreground Codex source: %v", err)
	}
	command := filepath.Join(destination, "codex")
	build := exec.Command(goToolPath(t), "build", "-o", command, "main.go")
	build.Dir = source
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the foreground Codex command: output_bytes=%d", len(output))
	}
	return command
}

func goToolPath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("resolve the Go toolchain: %v", err)
	}
	return path
}

// requestHookEvent publishes one Codex lifecycle event to the foreground Codex
// stand-in and returns when the real hook has exited successfully, so every
// tmux fact it published is already durable.
func requestHookEvent(t *testing.T, events, event string) time.Time {
	t.Helper()
	requestedAt := time.Now()
	if err := os.WriteFile(filepath.Join(events, event+".request"), nil, 0o600); err != nil {
		t.Fatalf("request the %s status hook: %v", event, err)
	}
	statusPath := filepath.Join(events, event+".status")
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	for {
		contents, err := os.ReadFile(statusPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read the %s status-hook exit status: %v", event, err)
		}
		if status := strings.TrimSpace(string(contents)); err == nil && status != "" {
			if status != "0" {
				t.Fatalf("the %s status hook exited with status %s, want 0", event, status)
			}
			return requestedAt
		}
		if time.Now().After(deadline) {
			t.Fatalf("the %s status hook did not report an exit status inside the proof window", event)
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
}

func readHookResponse(t *testing.T, events, event string) string {
	t.Helper()
	response, err := os.ReadFile(filepath.Join(events, event+".response"))
	if err != nil {
		t.Fatalf("read the %s status-hook response: %v", event, err)
	}
	return string(response)
}

// requireLifecycleFact holds the published pane fact to the exact foreground
// lifetime: the same PID, the same kernel start identity, the expected state,
// and a signal time inside this proof's window. A fact that names anything
// else could survive process replacement.
func requireLifecycleFact(t *testing.T, value, wantState string, panePID int, lifetime string, notBefore time.Time) {
	t.Helper()
	fields := strings.Split(value, ":")
	if len(fields) != 5 {
		t.Fatalf("published lifecycle fact has %d fields, want 5", len(fields))
	}
	if fields[0] != "v1" || fields[1] != strconv.Itoa(panePID) || fields[2] != lifetime || fields[3] != wantState {
		t.Fatalf(
			"published lifecycle fact does not name the exact foreground lifetime: version=%q pid_match=%t start_identity_match=%t state=%q want_state=%q",
			fields[0], fields[1] == strconv.Itoa(panePID), fields[2] == lifetime, fields[3], wantState,
		)
	}
	seconds, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		t.Fatalf("published lifecycle fact has an unparsable signal time: %v", err)
	}
	signalledAt := time.Unix(seconds, 0)
	if signalledAt.Before(notBefore.Add(-time.Second)) || signalledAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("published lifecycle signal time is outside the proof window: signal_unix=%d requested_unix=%d", seconds, notBefore.Unix())
	}
}

// waitForSession waits for one named session to reach a status kind. That is
// the pane's own convergence, not a retry of a failing assertion.
func (fixture sessionFixture) waitForSession(t *testing.T, ctx context.Context, name string, want sessions.StatusKind) sessions.Session {
	t.Helper()
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	for {
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list while waiting for session %s", name)
		}
		observed := requireSessionNamed(t, listed, name)
		if observed.Status.Kind == want {
			return observed
		}
		if time.Now().After(deadline) {
			t.Fatalf("session %s did not converge: kind=%s want_kind=%s signal=%s", name, observed.Status.Kind, want, observed.Status.Signal)
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

func requireSessionNamed(t *testing.T, listed []sessions.Session, name string) sessions.Session {
	t.Helper()
	for _, session := range listed {
		if session.TmuxName == name {
			return session
		}
	}
	t.Fatalf("expected session name not listed: session_count=%d", len(listed))
	return sessions.Session{}
}

func requireSessionID(t *testing.T, listed []sessions.Session, id string) sessions.Session {
	t.Helper()
	for _, session := range listed {
		if session.ID == id {
			return session
		}
	}
	t.Fatalf("expected session id not listed: session_count=%d", len(listed))
	return sessions.Session{}
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
