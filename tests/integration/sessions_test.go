//go:build integration

package integration_test

import (
	"context"
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

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const (
	tmuxPath               = "/usr/bin/tmux"
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
		t.Fatalf("create service home: %v", err)
	}
	catalogue := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(catalogue, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatalf("write catalogue: %v", err)
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
		t.Fatalf("an unavailable opaque profile command took down inventory: %v", err)
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
	socketPath := filepath.Join(root, "tmux", "owned-default.sock")
	registerTestOwnedSocketPath(t, socketPath)
	t.Cleanup(func() {
		cleanupDefaultSocketHelper(t, root, socketPath)
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
	)
	command.Env = append(withoutEnvironment(os.Environ(), "HOME", "TMUX", "TMUX_PANE", "TMUX_TMPDIR",
		defaultSocketHelperVariable, defaultSocketRootVariable, defaultSocketNonceVariable),
		defaultSocketHelperVariable+"=1",
		"HOME="+filepath.Join(root, "home"),
		"TMUX_TMPDIR="+filepath.Join(root, "tmux"),
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
	root := os.Getenv(defaultSocketRootVariable)
	nonce := os.Getenv(defaultSocketNonceVariable)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || os.Getenv("HOME") != filepath.Join(root, "home") ||
		os.Getenv("TMUX_TMPDIR") != filepath.Join(root, "tmux") {
		t.Fatal("default-socket subprocess has no exact private root")
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat default-socket subprocess root: path=%q error=%v", root, err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("default-socket subprocess root is not private: path=%q mode=%v", root, rootInfo.Mode())
	}
	capabilityPath := filepath.Join(root, ".default-socket-capability")
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
	home := filepath.Join(root, "home")
	tmuxRoot := filepath.Join(root, "tmux")
	profileHome := filepath.Join(root, "profile")
	project := filepath.Join(root, "project")
	for _, path := range []string{home, tmuxRoot, profileHome, project} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("create default-socket fixture directory: path=%s error=%v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".tmux.conf"), []byte("set-option -g @skid_test_config loaded\n"), 0o600); err != nil {
		t.Fatalf("write isolated user tmux config: %v", err)
	}
	catalogue := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(catalogue, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatalf("write default-socket catalogue fixture: %v", err)
	}
	agent := filepath.Join(root, "agent")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\nexec /usr/bin/sleep 300\n"), 0o700); err != nil {
		t.Fatalf("write default-socket agent fixture: %v", err)
	}
	socketPath := filepath.Join(tmuxRoot, "owned-default.sock")
	registerTestOwnedSocketPath(t, socketPath)
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated default tmux socket unexpectedly exists before test: path=%s error=%v", socketPath, err)
	}
	guardedTmux := filepath.Join(root, "guarded-tmux")
	guardScript := `#!/bin/sh
set -eu
expected_root="${SKIDBLADNIR_DEFAULT_TMUX_ROOT:?}/tmux"
[ "${TMUX_TMPDIR-}" = "$expected_root" ] || exit 97
for argument in "$@"; do
  case "$argument" in
    -L*|-S*|-f*) exit 98 ;;
  esac
done
exec /usr/bin/tmux -S "$expected_root/owned-default.sock" "$@"
`
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

	if output, err := isolatedTmuxCommand(tmuxPath, "-V").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "tmux 3.4" {
		t.Fatalf("S1 integration requires /usr/bin/tmux 3.4: output=%q error=%v", output, err)
	}

	fixture := newSessionFixture(t)
	ctx := context.Background()

	t.Run("invalid starts mutate nothing", func(t *testing.T) {
		before, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list sessions before invalid starts: %v", err)
		}

		file := filepath.Join(fixture.root, "not-a-directory")
		if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
			t.Fatalf("write non-directory cwd fixture: %v", err)
		}
		unsearchable := filepath.Join(fixture.root, "unsearchable")
		if err := os.Mkdir(unsearchable, 0o600); err != nil {
			t.Fatalf("create unsearchable cwd fixture: %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(unsearchable, 0o700); err != nil {
				t.Errorf("restore test-owned directory permissions: %v", err)
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
			{name: "unsafe name", input: sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "unsafe:name"}, code: sessions.ErrorSessionNameInvalid},
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
					t.Fatalf("list sessions after rejected %s: %v", test.name, listErr)
				}
				if len(after) != len(before) {
					t.Fatalf("rejected %s mutated isolated tmux: before=%d after=%d sessions=%+v", test.name, len(before), len(after), after)
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
  /usr/bin/tmux -L "$socket" -f /dev/null new-session -d -s %q -c "$project" -- /usr/bin/sleep 300
  /usr/bin/tmux -L "$socket" -f /dev/null set-option -t '=%s:' -- @collision_guard untouched
  /usr/bin/tmux -L "$socket" -f /dev/null display-message -p -t '=%s:' '#{session_id}|#{pane_pid}|#{@collision_guard}|#{@skid_profile}|#{@skid_character}|#{@skid_objective_b64}' > "$identity"
fi
exec /usr/bin/tmux "$@"
`, fixture.socket, fixture.project, marker, identity, collisionName, collisionName, collisionName, collisionName)
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatalf("write collision tmux wrapper: %v", err)
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
			t.Fatalf("construct collision manager: %v", err)
		}
		_, err = manager.Create(ctx, sessions.CreateInput{
			CWD:          fixture.project,
			Profile:      "personal",
			OptionalName: collisionName,
			Objective:    "must not land",
		})
		assertSessionError(t, err, sessions.ErrorSessionNameConflict)

		beforeBytes, err := os.ReadFile(identity)
		if err != nil {
			t.Fatalf("read injected session identity: %v", err)
		}
		before := strings.TrimSpace(string(beforeBytes))
		after := fixture.tmux(t, "display-message", "-p", "-t", "="+collisionName+":",
			"#{session_id}|#{pane_pid}|#{@collision_guard}|#{@skid_profile}|#{@skid_character}|#{@skid_objective_b64}")
		if after != before {
			t.Fatalf("name collision mutated the pre-existing session: before=%q after=%q", before, after)
		}
	})

	t.Run("lists laptop and managed sessions without guessing metadata", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "laptop", "-c", fixture.project, "--", "/usr/bin/sleep", "300")
		fixture.tmux(t, "new-session", "-d", "-s", "shell", "-c", fixture.project)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list laptop-created sessions: %v", err)
		}
		laptop := requireSessionNamed(t, listed, "laptop")
		if laptop.Profile != "" || laptop.Objective != "" || laptop.CharacterKey != "" || laptop.CharacterDisplayName != "" {
			t.Fatalf("laptop session metadata was guessed: %+v", laptop)
		}
		if laptop.Status.Kind != sessions.StatusRunning || laptop.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("uninstrumented allowlisted laptop process should report only process liveness: %+v", laptop.Status)
		}
		shell := requireSessionNamed(t, listed, "shell")
		if shell.Status.Kind != sessions.StatusShell || shell.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("ordinary laptop shell should be reported literally: %+v", shell.Status)
		}
	})

	t.Run("foreground signatures are exact and profile metadata is never inferred", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "native-codex", "-c", fixture.project, "--", fixture.nativeCodex, "300")
		fixture.tmux(t, "new-session", "-d", "-s", "owned-node", "-c", fixture.project, "--", "/usr/bin/node", fixture.nodeScript)
		fixture.tmux(t, "new-session", "-d", "-s", "plain-node", "-c", fixture.project, "--", "/usr/bin/node", "-e", "setInterval(() => {}, 300000)")

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list exact foreground signature fixtures: %v", err)
		}
		for _, name := range []string{"native-codex", "owned-node"} {
			session := requireSessionNamed(t, listed, name)
			if session.Status.Kind != sessions.StatusRunning || session.Profile != "" {
				t.Fatalf("exact signature should affect only process status: session=%+v", session)
			}
		}
		plainNode := requireSessionNamed(t, listed, "plain-node")
		if plainNode.Status.Kind != sessions.StatusShell || plainNode.Profile != "" {
			t.Fatalf("arbitrary node process must not match an owned Codex argv signature: %+v", plainNode)
		}
	})

	t.Run("card facts come only from the current window active pane", func(t *testing.T) {
		otherDirectory := filepath.Join(fixture.root, "other pane")
		if err := os.Mkdir(otherDirectory, 0o700); err != nil {
			t.Fatalf("create conflicting pane cwd: %v", err)
		}
		fixture.tmux(t, "new-session", "-d", "-s", "anchor", "-c", fixture.project, "--", "/usr/bin/sleep", "300")
		activePane := fixture.tmux(t, "display-message", "-p", "-t", "anchor:0.0", "#{pane_id}")
		fixture.tmux(t, "split-window", "-d", "-t", "anchor:0", "-c", otherDirectory)
		inactivePane := fixture.tmux(t, "display-message", "-p", "-t", "anchor:0.1", "#{pane_id}")
		fixture.tmux(t, "set-option", "-p", "-t", inactivePane, "--", "@skid_attention", fmt.Sprint(time.Now().Unix()))
		fixture.tmux(t, "new-window", "-d", "-t", "anchor", "-n", "other", "-c", otherDirectory)
		fixture.tmux(t, "select-window", "-t", "anchor:0")
		fixture.tmux(t, "select-pane", "-t", activePane)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list multi-window anchor fixture: %v", err)
		}
		anchor := requireSessionNamed(t, listed, "anchor")
		if anchor.CWD != fixture.project || anchor.ActiveCommand != "sleep" || anchor.Attention || anchor.Status.Kind != sessions.StatusRunning {
			t.Fatalf("inactive pane/window facts leaked into card anchor: active=%s inactive=%s card=%+v", activePane, inactivePane, anchor)
		}
	})

	t.Run("phone shadows are excluded and grouped cards report group clients", func(t *testing.T) {
		const phoneShadow = "skid-phone-00112233445566778899aabbccddeeff"
		fixture.tmux(t, "new-session", "-d", "-s", phoneShadow, "-c", fixture.project, "--", "/usr/bin/sleep", "300")
		fixture.tmux(t, "set-option", "-t", phoneShadow, "--", "@skid_internal", "phone-shadow")
		fixture.attachClient(t, phoneShadow)
		fixture.tmux(t, "new-session", "-d", "-s", "group-source", "-c", fixture.project, "--", "/usr/bin/sleep", "300")
		fixture.tmux(t, "new-session", "-d", "-t", "group-source", "-s", "group-link")
		fixture.attachClient(t, "group-source")
		fixture.attachClient(t, "group-link")

		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for {
			listed, err := fixture.manager.List(ctx)
			if err != nil {
				t.Fatalf("list grouped client fixture: %v", err)
			}
			if slices.ContainsFunc(listed, func(session sessions.Session) bool { return session.Name == phoneShadow }) {
				t.Fatalf("gateway-owned phone shadow leaked into inventory: %+v", listed)
			}
			source := requireSessionNamed(t, listed, "group-source")
			link := requireSessionNamed(t, listed, "group-link")
			if source.AttachedClients == 2 && link.AttachedClients == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("group attachment count did not converge: source=%+v link=%+v", source, link)
			}
			time.Sleep(tmuxConvergencePollInterval)
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
				t.Fatalf("list process-observation failure fixture: %v", err)
			}
			unknown := requireSessionNamed(t, listed, "unknown")
			if unknown.Status.Kind == sessions.StatusUnknown {
				if unknown.Status.Signal != sessions.StatusSignalPollFailure {
					t.Fatalf("unknown status must name poll failure: %+v", unknown.Status)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("dead pane process did not become Unknown/PollFailure: %+v", unknown)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
	})

	t.Run("creates profiles metadata balanced portraits and unbounded generated names", func(t *testing.T) {
		first, err := fixture.manager.Create(ctx, sessions.CreateInput{
			CWD:       "~/project with spaces",
			Profile:   "personal",
			Objective: "Inspect Ω",
		})
		if err != nil {
			t.Fatalf("create first generated session: %v", err)
		}
		second, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work"})
		if err != nil {
			t.Fatalf("create second generated session: %v", err)
		}
		third, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work2"})
		if err != nil {
			t.Fatalf("create generated session beyond catalogue base names: %v", err)
		}
		custom, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "hand_named"})
		if err != nil {
			t.Fatalf("create custom named session: %v", err)
		}
		claude, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "claude-work"})
		if err != nil {
			t.Fatalf("create Claude work session: %v", err)
		}

		if first.Name != "ga-modsognir" || first.CharacterKey != "norse.modsognir" || first.CharacterDisplayName != "Móðsognir" {
			t.Fatalf("first catalogue assignment mismatch: %+v", first)
		}
		if second.Name != "ga-durinn" || second.CharacterKey != "norse.durinn" || second.CharacterDisplayName != "Durinn" {
			t.Fatalf("second catalogue assignment mismatch: %+v", second)
		}
		if third.Name != "ga-modsognir-2" || third.CharacterKey != "norse.modsognir" {
			t.Fatalf("generated names must continue at the smallest suffix round: %+v", third)
		}
		if custom.Name != "hand_named" || custom.CharacterKey != "norse.durinn" || custom.CharacterDisplayName != "Durinn" {
			t.Fatalf("custom name must retain a least-used dwarf portrait: %+v", custom)
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
			if observed.Name != created.Name || observed.Profile != created.Profile || observed.CharacterKey != created.CharacterKey || observed.IdentityToken != created.IdentityToken {
				t.Fatalf("create response did not converge to tmux inventory: created=%+v listed=%+v", created, observed)
			}
		}
	})

	t.Run("pane lifecycle facts drive working and idle without tmux activity guesses", func(t *testing.T) {
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list before lifecycle projection: %v", err)
		}
		target := requireSessionNamed(t, listed, "ga-modsognir")
		if target.Status.Kind != sessions.StatusRunning {
			t.Fatalf("agent without lifecycle evidence should be Running: %+v", target.Status)
		}
		panePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"))
		if err != nil {
			t.Fatalf("parse lifecycle fixture pane pid: %v", err)
		}
		processStartTime := linuxProcessStartTime(panePID)
		if processStartTime == 0 {
			t.Fatalf("observe lifecycle fixture process start time: pid=%d", panePID)
		}

		idleAt := time.Now().Add(-2 * time.Second).Unix()
		fixture.tmux(t, "set-option", "-p", "-t", target.ID, "--", "@skid_lifecycle",
			fmt.Sprintf("v1:%d:%d:idle:%d", panePID, processStartTime, idleAt))
		fixture.tmux(t, "send-keys", "-t", target.ID, "activity-without-output")
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after pane input activity: %v", err)
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusIdle || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("tmux activity changed an explicit idle lifecycle fact: %+v", target.Status)
		}

		workingAt := time.Now().Unix()
		fixture.tmux(t, "set-option", "-p", "-t", target.ID, "--", "@skid_lifecycle",
			fmt.Sprintf("v1:%d:%d:working:%d", panePID, processStartTime, workingAt))
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after working lifecycle fact: %v", err)
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusWorking || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("working lifecycle fact did not project: %+v", target.Status)
		}
	})

	t.Run("five-line notify surfaces honest attention", func(t *testing.T) {
		notifyPath := filepath.Join(fixture.repositoryRoot, "deploy", "skid-notify")
		outsideTmux := exec.Command(notifyPath)
		outsideTmux.Env = withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "TMUX_TMPDIR")
		if output, err := outsideTmux.CombinedOutput(); err != nil {
			t.Fatalf("skid-notify must be a safe no-op outside tmux: output=%q error=%v", output, err)
		}

		fixture.tmux(t, "new-session", "-d", "-s", "notify", "-c", fixture.repositoryRoot)
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list notify fixture: %v", err)
		}
		notify := requireSessionNamed(t, listed, "notify")
		fixture.tmux(t, "send-keys", "-t", notify.ID, "-l", "--", notifyPath)
		fixture.tmux(t, "send-keys", "-t", notify.ID, "Enter")

		statusBeforeNotify := notify.Status
		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for {
			listed, err = fixture.manager.List(ctx)
			if err != nil {
				t.Fatalf("list while waiting for attention: %v", err)
			}
			notify = requireSessionID(t, listed, notify.ID)
			if notify.Attention {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("skid-notify did not surface attention: %+v", notify)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		if notify.Status.Kind != statusBeforeNotify.Kind || notify.Status.Signal != statusBeforeNotify.Signal {
			t.Fatalf("attention replaced the independent lifecycle status: before=%+v after=%+v", statusBeforeNotify, notify.Status)
		}

	})

	t.Run("kill rechecks exact id and displayed name", func(t *testing.T) {
		victim, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "kill_victim"})
		if err != nil {
			t.Fatalf("create kill victim: %v", err)
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalName: "kill_survivor"})
		if err != nil {
			t.Fatalf("create kill survivor: %v", err)
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, DisplayedName: survivor.Name, IdentityToken: victim.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
		listed, listErr := fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatalf("list after refused mismatched kill: %v", listErr)
		}
		requireSessionID(t, listed, victim.ID)
		requireSessionID(t, listed, survivor.ID)

		missingToken := strings.TrimSuffix(victim.IdentityToken, strings.TrimPrefix(victim.ID, "$")) + "999999"
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "$999999", DisplayedName: victim.Name, IdentityToken: missingToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "kill_survivor", DisplayedName: survivor.Name, IdentityToken: survivor.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, DisplayedName: victim.Name, IdentityToken: victim.IdentityToken}); err != nil {
			t.Fatalf("kill exact confirmed session: %v", err)
		}
		listed, listErr = fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatalf("list after exact kill: %v", listErr)
		}
		if slices.ContainsFunc(listed, func(session sessions.Session) bool { return session.ID == victim.ID }) {
			t.Fatalf("exactly killed session remains listed: victim=%+v sessions=%+v", victim, listed)
		}
		requireSessionID(t, listed, survivor.ID)
	})

	t.Run("kill refuses an ordinary grouped sibling without mutation", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "grouped-kill-target", "-c", fixture.project, "--", "/usr/bin/sleep", "300")
		fixture.tmux(t, "new-session", "-d", "-t", "grouped-kill-target", "-s", "grouped-kill-sibling")
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list grouped kill fixture: %v", err)
		}
		target := requireSessionNamed(t, listed, "grouped-kill-target")
		sibling := requireSessionNamed(t, listed, "grouped-kill-sibling")
		panePID := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}")
		if siblingPID := fixture.tmux(t, "display-message", "-p", "-t", sibling.ID, "#{pane_pid}"); siblingPID != panePID {
			t.Fatalf("grouped kill fixture does not share one pane process: target=%s sibling=%s", panePID, siblingPID)
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, DisplayedName: target.Name, IdentityToken: target.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionGroupedConflict)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatalf("list after refused grouped kill: %v", err)
		}
		if observed := requireSessionID(t, listed, target.ID); observed.Name != target.Name {
			t.Fatalf("refused grouped kill changed target identity: before=%+v after=%+v", target, observed)
		}
		if observed := requireSessionID(t, listed, sibling.ID); observed.Name != sibling.Name {
			t.Fatalf("refused grouped kill changed sibling identity: before=%+v after=%+v", sibling, observed)
		}
		if after := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"); after != panePID {
			t.Fatalf("refused grouped kill changed shared pane process: before=%s after=%s", panePID, after)
		}
	})

	t.Run("one kill reconciles every stale owned shadow before ending only the source", func(t *testing.T) {
		target, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "shadowed_kill_target"})
		if err != nil {
			t.Fatalf("create shadowed kill target: %v", err)
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalName: "shadowed_kill_survivor"})
		if err != nil {
			t.Fatalf("create shadowed kill survivor: %v", err)
		}
		panePIDText := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}")
		panePID, err := strconv.Atoi(panePIDText)
		if err != nil {
			t.Fatalf("parse shadowed target pane pid %q: %v", panePIDText, err)
		}
		paneStartTime := linuxProcessStartTime(panePID)
		if paneStartTime == 0 {
			t.Fatalf("capture shadowed target pane process identity: pid=%d", panePID)
		}
		survivorPanePIDText := fixture.tmux(t, "display-message", "-p", "-t", survivor.ID, "#{pane_pid}")
		survivorPanePID, err := strconv.Atoi(survivorPanePIDText)
		if err != nil {
			t.Fatalf("parse shadowed survivor pane pid %q: %v", survivorPanePIDText, err)
		}
		survivorPaneStartTime := linuxProcessStartTime(survivorPanePID)
		if survivorPaneStartTime == 0 {
			t.Fatalf("capture shadowed survivor pane process identity: pid=%d", survivorPanePID)
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

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, DisplayedName: target.Name, IdentityToken: target.IdentityToken}); err != nil {
			t.Fatalf("kill source after multi-shadow fixed-point recovery: %v", err)
		}
		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for linuxProcessStartTime(panePID) == paneStartTime {
			if time.Now().After(deadline) {
				t.Fatalf("source process survived successful kill after stale-shadow recovery: pid=%d", panePID)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		remainingNames := strings.Split(fixture.tmux(t, "list-sessions", "-F", "#{session_name}"), "\n")
		for _, name := range append([]string{target.Name}, shadowNames...) {
			if slices.Contains(remainingNames, name) {
				t.Fatalf("successful kill left a grouped link: name=%s sessions=%q", name, remainingNames)
			}
		}
		if !slices.Contains(remainingNames, survivor.Name) {
			t.Fatalf("multi-shadow recovery killed the unrelated survivor: survivor=%+v sessions=%q", survivor, remainingNames)
		}
		if observed := fixture.tmux(t, "display-message", "-p", "-t", survivor.ID, "#{pane_pid}"); observed != survivorPanePIDText || linuxProcessStartTime(survivorPanePID) != survivorPaneStartTime {
			t.Fatalf("multi-shadow recovery changed the survivor process: before=%s/%d after=%s/%d", survivorPanePIDText, survivorPaneStartTime, observed, linuxProcessStartTime(survivorPanePID))
		}
	})
}

func TestStaleLifetimeTokenCannotKillRecycledSession(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatalf("create lifetime-token fixture: %v", err)
	}
	cataloguePath := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(cataloguePath, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatalf("write lifetime-token catalogue: %v", err)
	}
	socket := randomTmuxSocketName(t, "skid-reuse")
	socketPath := namedTmuxSocketPath(socket)
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated lifetime-token socket unexpectedly exists before test: path=%s error=%v", socketPath, err)
	}
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    socket,
		Home:          home,
		CataloguePath: cataloguePath,
		Profiles: []sessions.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Command:              "/usr/bin/sleep",
			ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
			Arguments:            []string{"300"},
		}},
	})
	if err != nil {
		t.Fatalf("construct lifetime-token manager: %v", err)
	}

	ctx := context.Background()
	var cleanup *sessions.Session
	t.Cleanup(func() {
		if cleanup == nil {
			return
		}
		if err := manager.Kill(ctx, sessions.KillInput{ID: cleanup.ID, DisplayedName: cleanup.Name, IdentityToken: cleanup.IdentityToken}); err != nil {
			t.Errorf("kill exact test-owned lifetime fixture: %v", err)
		}
	})

	first, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalName: "epoch-reuse"})
	if err != nil {
		t.Fatalf("create first lifetime fixture: %v", err)
	}
	cleanup = &first
	firstServer := captureTestTmuxServer(t, tmuxPath, socketPath)
	if err := manager.Kill(ctx, sessions.KillInput{ID: first.ID, DisplayedName: first.Name, IdentityToken: first.IdentityToken}); err != nil {
		t.Fatalf("kill first exact lifetime fixture: %v", err)
	}
	deadline := time.Now().Add(tmuxCleanupTimeout)
	for linuxProcessStartTime(firstServer.pid) == firstServer.kernelStartTime {
		if time.Now().After(deadline) {
			t.Fatalf("first isolated tmux server did not exit: pid=%d", firstServer.pid)
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	cleanup = nil

	second, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalName: first.Name})
	if err != nil {
		t.Fatalf("recreate lifetime fixture: %v", err)
	}
	cleanup = &second
	if second.ID != first.ID || second.Name != first.Name || second.IdentityToken == first.IdentityToken {
		t.Fatalf("fixture did not recycle id/name under a new lifetime: first=%+v second=%+v", first, second)
	}
	firstIdentityFields := strings.Split(first.IdentityToken, ".")
	secondIdentityFields := strings.Split(second.IdentityToken, ".")
	if len(firstIdentityFields) != 4 || len(secondIdentityFields) != 4 {
		t.Fatalf("fixture returned malformed lifetime identities: first=%q second=%q", first.IdentityToken, second.IdentityToken)
	}
	oldEpoch := firstIdentityFields[0]
	if output, err := isolatedTmuxCommand(tmuxPath, "-S", socketPath,
		"set-option", "-s", tmuxclient.ServerEpochOption, oldEpoch).CombinedOutput(); err != nil {
		t.Fatalf("restore old epoch on isolated replacement server: output=%q error=%v", output, err)
	}
	restored := second
	restored.IdentityToken = strings.Join([]string{oldEpoch, secondIdentityFields[1], secondIdentityFields[2], secondIdentityFields[3]}, ".")
	cleanup = &restored
	listed, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("list replacement after old epoch restoration: %v", err)
	}
	second = requireSessionID(t, listed, second.ID)
	cleanup = &second
	if !strings.HasPrefix(second.IdentityToken, oldEpoch+".") || second.IdentityToken == first.IdentityToken {
		t.Fatalf("built-in server lifetime did not distinguish restored epoch: first=%+v second=%+v", first, second)
	}
	err = manager.Kill(ctx, sessions.KillInput{ID: second.ID, DisplayedName: second.Name, IdentityToken: first.IdentityToken})
	assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
	listed, err = manager.List(ctx)
	if err != nil {
		t.Fatalf("list after stale lifetime kill: %v", err)
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
	nativeCodex    string
	nodeScript     string
	profileHomes   map[string]string
	socket         string
	socketPath     string
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()

	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	root := t.TempDir()
	home := filepath.Join(root, "service home")
	project := filepath.Join(home, "project with spaces")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatalf("create project fixture: %v", err)
	}
	cataloguePath := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(cataloguePath, []byte(`[
  {"key":"norse.modsognir","displayName":"Móðsognir"},
  {"key":"norse.durinn","displayName":"Durinn"}
]`), 0o600); err != nil {
		t.Fatalf("write catalogue fixture: %v", err)
	}

	profileHomes := map[string]string{}
	for _, name := range []string{"personal", "work", "work2", "claude-work"} {
		profileHomes[name] = filepath.Join(root, "profile-"+name)
		if err := os.Mkdir(profileHomes[name], 0o700); err != nil {
			t.Fatalf("create %s profile home: %v", name, err)
		}
	}
	agent := filepath.Join(root, "agent-fixture")
	if err := os.WriteFile(agent, []byte(`#!/bin/sh
set -eu
/usr/bin/printf '%s\n' "$@" > "$CODEX_HOME/observed-argv"
/usr/bin/printf '%s\n' "$CODEX_HOME" > "$CODEX_HOME/observed-home"
exec /usr/bin/sleep 300
`), 0o700); err != nil {
		t.Fatalf("write agent fixture: %v", err)
	}
	claudeAgent := filepath.Join(root, "claude-agent-fixture")
	if err := os.WriteFile(claudeAgent, []byte(`#!/bin/sh
set -eu
/usr/bin/printf '%s\n' "$@" > "$CLAUDE_CONFIG_DIR/observed-argv"
/usr/bin/printf '%s\n' "$CLAUDE_CONFIG_DIR" > "$CLAUDE_CONFIG_DIR/observed-home"
/usr/bin/pwd -P > "$CLAUDE_CONFIG_DIR/observed-cwd"
exec /usr/bin/sleep 300
`), 0o700); err != nil {
		t.Fatalf("write Claude agent fixture: %v", err)
	}
	nativeCodex := filepath.Join(root, "codex")
	sleepBinary, err := os.ReadFile("/usr/bin/sleep")
	if err != nil {
		t.Fatalf("read native foreground fixture: %v", err)
	}
	if err := os.WriteFile(nativeCodex, sleepBinary, 0o700); err != nil {
		t.Fatalf("write native foreground fixture: %v", err)
	}
	nodeScript := filepath.Join(root, "owned-codex.js")
	if err := os.WriteFile(nodeScript, []byte("setInterval(() => {}, 300000)\n"), 0o600); err != nil {
		t.Fatalf("write node foreground fixture: %v", err)
	}

	socket := randomTmuxSocketName(t, "skid-s1")
	socketPath := namedTmuxSocketPath(socket)
	if _, err := os.Lstat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated tmux socket unexpectedly exists before test: path=%s error=%v", socketPath, err)
	}
	bootstrapName := "skid-test-bootstrap"
	if output, err := isolatedTmuxCommand(tmuxPath, "-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", bootstrapName, "-c", project, "--", "/usr/bin/sleep", "300").CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux fixture server: output=%q error=%v", output, err)
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
		t.Fatalf("construct session manager: %v", err)
	}

	return sessionFixture{
		manager:        manager,
		repositoryRoot: repositoryRoot,
		root:           root,
		home:           home,
		project:        project,
		agent:          agent,
		cataloguePath:  cataloguePath,
		nativeCodex:    nativeCodex,
		nodeScript:     nodeScript,
		profileHomes:   profileHomes,
		socket:         socket,
		socketPath:     socketPath,
	}
}

func (fixture sessionFixture) attachClient(t *testing.T, session string) {
	t.Helper()
	commandText := fmt.Sprintf("exec /usr/bin/env -u TMUX -u TMUX_PANE -u TMUX_TMPDIR TERM=xterm-256color %s -S %s -f /dev/null attach-session -t %s", tmuxPath, fixture.socketPath, session)
	command := exec.Command("/usr/bin/script", "-qefc", commandText, "/dev/null")
	command.Env = withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "TMUX_TMPDIR")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatalf("start real test-owned tmux client: session=%s error=%v", session, err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill() // justify-ignore-error: cleanup accepts an already-detached test-owned PTY helper.
		}
		select {
		case <-done:
		case <-time.After(tmuxCleanupTimeout):
			t.Errorf("test-owned tmux PTY helper did not exit: session=%s", session)
		}
	})
}

func (fixture sessionFixture) tmux(t *testing.T, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-L", fixture.socket, "-f", "/dev/null"}, args...)
	output, err := isolatedTmuxCommand(tmuxPath, commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("test-owned tmux command failed: args=%q output=%q error=%v", args, output, err)
	}
	return strings.TrimSuffix(string(output), "\n")
}

func assertSessionError(t *testing.T, err error, want sessions.ErrorCode) {
	t.Helper()
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		t.Fatalf("expected sessions.Error %s, got %T: %v", want, err, err)
	}
	if sessionError.Code != want {
		t.Fatalf("wrong sessions error: want=%s got=%s message=%q", want, sessionError.Code, sessionError.Message)
	}
}

func requireSessionNamed(t *testing.T, listed []sessions.Session, name string) sessions.Session {
	t.Helper()
	for _, session := range listed {
		if session.Name == name {
			return session
		}
	}
	t.Fatalf("session name not listed: name=%q sessions=%+v", name, listed)
	return sessions.Session{}
}

func requireSessionID(t *testing.T, listed []sessions.Session, id string) sessions.Session {
	t.Helper()
	for _, session := range listed {
		if session.ID == id {
			return session
		}
	}
	t.Fatalf("session id not listed: id=%q sessions=%+v", id, listed)
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
			t.Fatalf("read process-observation fixture: path=%s error=%v", path, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("process observation did not converge: path=%s want=%q got=%q error=%v", path, want, contents, err)
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
