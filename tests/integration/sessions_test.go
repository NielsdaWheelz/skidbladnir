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
)

var tmuxPath = platform.Current().TmuxPath

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

func TestSessionManagerAgainstRealTmux(t *testing.T) {
	t.Parallel()

	if output, err := isolatedTmuxCommand(tmuxPath, "-V").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != platform.Current().TmuxVersion {
		t.Fatalf("host tmux pin mismatch: command_ok=%t version_match=%t", err == nil, strings.TrimSpace(string(output)) == platform.Current().TmuxVersion)
	}

	fixture := newSessionFixture(t)
	ctx := context.Background()

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
			CWD:          fixture.project,
			Profile:      "personal",
			OptionalName: collisionName,
			Objective:    "must not land",
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

	t.Run("lists laptop and managed sessions without guessing metadata", func(t *testing.T) {
		fixture.tmux(t, "new-session", "-d", "-s", "laptop", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-session", "-d", "-s", "shell", "-c", fixture.project)

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list laptop-created sessions")
		}
		laptop := requireSessionNamed(t, listed, "laptop")
		if laptop.Profile != "" || laptop.Objective != "" || laptop.CharacterKey != "" || laptop.CharacterDisplayName != "" {
			t.Fatalf(
				"laptop session metadata was guessed: profile_present=%t objective_present=%t character_present=%t",
				laptop.Profile != "",
				laptop.Objective != "",
				laptop.CharacterKey != "" || laptop.CharacterDisplayName != "",
			)
		}
		if laptop.Status.Kind != sessions.StatusRunning || laptop.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("uninstrumented allowlisted laptop status mismatch: kind=%s signal=%s", laptop.Status.Kind, laptop.Status.Signal)
		}
		shell := requireSessionNamed(t, listed, "shell")
		if shell.Status.Kind != sessions.StatusShell || shell.Status.Signal != sessions.StatusSignalProcess {
			t.Fatalf("ordinary laptop shell status mismatch: kind=%s signal=%s", shell.Status.Kind, shell.Status.Signal)
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

	t.Run("phone shadows are excluded and grouped cards report group clients", func(t *testing.T) {
		const phoneShadow = "skid-phone-00112233445566778899aabbccddeeff"
		fixture.tmux(t, "new-session", "-d", "-s", phoneShadow, "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "set-option", "-t", phoneShadow, "--", "@skid_internal", "phone-shadow")
		fixture.attachClient(t, phoneShadow)
		fixture.tmux(t, "new-session", "-d", "-s", "group-source", "-c", fixture.project, "--", sleepPath, "300")
		fixture.tmux(t, "new-session", "-d", "-t", "group-source", "-s", "group-link")
		fixture.attachClient(t, "group-source")
		fixture.attachClient(t, "group-link")

		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for {
			listed, err := fixture.manager.List(ctx)
			if err != nil {
				t.Fatal("list grouped client fixture")
			}
			if slices.ContainsFunc(listed, func(session sessions.Session) bool { return session.Name == phoneShadow }) {
				t.Fatalf("gateway-owned phone shadow leaked into inventory: session_count=%d", len(listed))
			}
			source := requireSessionNamed(t, listed, "group-source")
			link := requireSessionNamed(t, listed, "group-link")
			if source.AttachedClients == 2 && link.AttachedClients == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("group attachment count did not converge: source_clients=%d link_clients=%d", source.AttachedClients, link.AttachedClients)
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
				t.Fatal("list process-observation failure fixture")
			}
			unknown := requireSessionNamed(t, listed, "unknown")
			if unknown.Status.Kind == sessions.StatusUnknown {
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

	t.Run("creates profiles metadata balanced portraits and unbounded generated names", func(t *testing.T) {
		first, err := fixture.manager.Create(ctx, sessions.CreateInput{
			CWD:       "~/project with spaces",
			Profile:   "personal",
			Objective: "Inspect Ω",
		})
		if err != nil {
			t.Fatal("create first generated session")
		}
		second, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work"})
		if err != nil {
			t.Fatal("create second generated session")
		}
		third, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work2"})
		if err != nil {
			t.Fatal("create generated session beyond catalogue base names")
		}
		custom, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "hand_named"})
		if err != nil {
			t.Fatal("create custom named session")
		}

		if first.Name != "ga-modsognir" || first.CharacterKey != "norse.modsognir" || first.CharacterDisplayName != "Móðsognir" {
			t.Fatal("first catalogue assignment mismatch")
		}
		if second.Name != "ga-durinn" || second.CharacterKey != "norse.durinn" || second.CharacterDisplayName != "Durinn" {
			t.Fatal("second catalogue assignment mismatch")
		}
		if third.Name != "ga-modsognir-2" || third.CharacterKey != "norse.modsognir" {
			t.Fatal("generated names did not continue at the smallest suffix round")
		}
		if custom.Name != "hand_named" || custom.CharacterKey != "norse.durinn" || custom.CharacterDisplayName != "Durinn" {
			t.Fatal("custom name did not retain a least-used dwarf portrait")
		}
		if first.Profile != "personal" || first.Objective != "Inspect Ω" || first.CWD != fixture.project {
			t.Fatalf(
				"managed session facts mismatch: profile_match=%t objective_match=%t cwd_match=%t",
				first.Profile == "personal",
				first.Objective == "Inspect Ω",
				first.CWD == fixture.project,
			)
		}

		for _, profile := range []string{"personal", "work", "work2"} {
			argvPath := filepath.Join(fixture.profileHomes[profile], "observed-argv")
			waitForFileLine(t, argvPath, yoloFlag)
			if got, err := os.ReadFile(filepath.Join(fixture.profileHomes[profile], "observed-home")); err != nil || strings.TrimSpace(string(got)) != fixture.profileHomes[profile] {
				t.Fatalf("profile %s did not receive its explicit CODEX_HOME: read_ok=%t value_match=%t", profile, err == nil, strings.TrimSpace(string(got)) == fixture.profileHomes[profile])
			}
		}

		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after managed creates")
		}
		for _, created := range []sessions.Session{first, second, third, custom} {
			observed := requireSessionID(t, listed, created.ID)
			if observed.Name != created.Name || observed.Profile != created.Profile || observed.CharacterKey != created.CharacterKey || observed.IdentityToken != created.IdentityToken {
				t.Fatalf(
					"create response did not converge to tmux inventory: name_match=%t profile_match=%t character_match=%t identity_match=%t",
					observed.Name == created.Name,
					observed.Profile == created.Profile,
					observed.CharacterKey == created.CharacterKey,
					observed.IdentityToken == created.IdentityToken,
				)
			}
		}
	})

	t.Run("pane lifecycle facts drive working and idle without tmux activity guesses", func(t *testing.T) {
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list before lifecycle projection")
		}
		target := requireSessionNamed(t, listed, "ga-modsognir")
		if target.Status.Kind != sessions.StatusRunning {
			t.Fatalf("agent without lifecycle evidence status mismatch: kind=%s", target.Status.Kind)
		}
		panePID, err := strconv.Atoi(fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"))
		if err != nil {
			t.Fatal("parse lifecycle fixture pane pid")
		}
		processStartTime := processStartIdentity(panePID)
		if processStartTime == "" {
			t.Fatal("observe lifecycle fixture process start time")
		}

		idleAt := time.Now().Add(-2 * time.Second).Unix()
		fixture.tmux(t, "set-option", "-p", "-t", target.ID, "--", "@skid_lifecycle",
			fmt.Sprintf("v1:%d:%s:idle:%d", panePID, processStartTime, idleAt))
		fixture.tmux(t, "send-keys", "-t", target.ID, "activity-without-output")
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after pane input activity")
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusIdle || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("tmux activity changed an explicit idle lifecycle fact: kind=%s signal=%s", target.Status.Kind, target.Status.Signal)
		}

		workingAt := time.Now().Unix()
		fixture.tmux(t, "set-option", "-p", "-t", target.ID, "--", "@skid_lifecycle",
			fmt.Sprintf("v1:%d:%s:working:%d", panePID, processStartTime, workingAt))
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after working lifecycle fact")
		}
		target = requireSessionID(t, listed, target.ID)
		if target.Status.Kind != sessions.StatusWorking || target.Status.Signal != sessions.StatusSignalLifecycle {
			t.Fatalf("working lifecycle fact did not project: kind=%s signal=%s", target.Status.Kind, target.Status.Signal)
		}
	})

	t.Run("five-line notify surfaces honest attention", func(t *testing.T) {
		notifyPath := filepath.Join(fixture.repositoryRoot, "deploy", integrationDeploymentHost(), "skid-notify")
		outsideTmux := exec.Command(notifyPath)
		outsideTmux.Env = withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "TMUX_TMPDIR")
		if output, err := outsideTmux.CombinedOutput(); err != nil {
			t.Fatalf("skid-notify was not a safe no-op outside tmux: output_bytes=%d", len(output))
		}

		fixture.tmux(t, "new-session", "-d", "-s", "notify", "-c", fixture.repositoryRoot)
		listed, err := fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list notify fixture")
		}
		notify := requireSessionNamed(t, listed, "notify")
		fixture.tmux(t, "send-keys", "-t", notify.ID, "-l", "--", notifyPath)
		fixture.tmux(t, "send-keys", "-t", notify.ID, "Enter")

		statusBeforeNotify := notify.Status
		deadline := time.Now().Add(tmuxConvergenceTimeout)
		for {
			listed, err = fixture.manager.List(ctx)
			if err != nil {
				t.Fatal("list while waiting for attention")
			}
			notify = requireSessionID(t, listed, notify.ID)
			if notify.Attention {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("skid-notify did not surface attention: attention=%t", notify.Attention)
			}
			time.Sleep(tmuxConvergencePollInterval)
		}
		if notify.Status.Kind != statusBeforeNotify.Kind || notify.Status.Signal != statusBeforeNotify.Signal {
			t.Fatalf(
				"attention replaced the independent lifecycle status: kind_match=%t signal_match=%t",
				notify.Status.Kind == statusBeforeNotify.Kind,
				notify.Status.Signal == statusBeforeNotify.Signal,
			)
		}

	})

	t.Run("kill rechecks exact id and displayed name", func(t *testing.T) {
		victim, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "kill_victim"})
		if err != nil {
			t.Fatal("create kill victim")
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalName: "kill_survivor"})
		if err != nil {
			t.Fatal("create kill survivor")
		}

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, DisplayedName: survivor.Name, IdentityToken: victim.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionIdentityMismatch)
		listed, listErr := fixture.manager.List(ctx)
		if listErr != nil {
			t.Fatal("list after refused mismatched kill")
		}
		requireSessionID(t, listed, victim.ID)
		requireSessionID(t, listed, survivor.ID)

		missingToken := strings.TrimSuffix(victim.IdentityToken, strings.TrimPrefix(victim.ID, "$")) + "999999"
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "$999999", DisplayedName: victim.Name, IdentityToken: missingToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)
		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: "kill_survivor", DisplayedName: survivor.Name, IdentityToken: survivor.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionNotFound)

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: victim.ID, DisplayedName: victim.Name, IdentityToken: victim.IdentityToken}); err != nil {
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

		err = fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, DisplayedName: target.Name, IdentityToken: target.IdentityToken})
		assertSessionError(t, err, sessions.ErrorSessionGroupedConflict)
		listed, err = fixture.manager.List(ctx)
		if err != nil {
			t.Fatal("list after refused grouped kill")
		}
		if observed := requireSessionID(t, listed, target.ID); observed.Name != target.Name {
			t.Fatal("refused grouped kill changed target identity")
		}
		if observed := requireSessionID(t, listed, sibling.ID); observed.Name != sibling.Name {
			t.Fatal("refused grouped kill changed sibling identity")
		}
		if after := fixture.tmux(t, "display-message", "-p", "-t", target.ID, "#{pane_pid}"); after != panePID {
			t.Fatal("refused grouped kill changed shared pane process")
		}
	})

	t.Run("one kill reconciles every stale owned shadow before ending only the source", func(t *testing.T) {
		target, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "personal", OptionalName: "shadowed_kill_target"})
		if err != nil {
			t.Fatal("create shadowed kill target")
		}
		survivor, err := fixture.manager.Create(ctx, sessions.CreateInput{CWD: fixture.project, Profile: "work", OptionalName: "shadowed_kill_survivor"})
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

		if err := fixture.manager.Kill(ctx, sessions.KillInput{ID: target.ID, DisplayedName: target.Name, IdentityToken: target.IdentityToken}); err != nil {
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
		for _, name := range append([]string{target.Name}, shadowNames...) {
			if slices.Contains(remainingNames, name) {
				t.Fatalf("successful kill left a grouped link: remaining_session_count=%d", len(remainingNames))
			}
		}
		if !slices.Contains(remainingNames, survivor.Name) {
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
	if err := os.WriteFile(cataloguePath, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
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
		if err := manager.Kill(ctx, sessions.KillInput{ID: cleanup.ID, DisplayedName: cleanup.Name, IdentityToken: cleanup.IdentityToken}); err != nil {
			t.Error("kill exact test-owned lifetime fixture")
		}
	})

	first, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalName: "epoch-reuse"})
	if err != nil {
		t.Fatal("create first lifetime fixture")
	}
	cleanup = &first
	firstServer := captureTestTmuxServer(t, tmuxPath, socketPath)
	if err := manager.Kill(ctx, sessions.KillInput{ID: first.ID, DisplayedName: first.Name, IdentityToken: first.IdentityToken}); err != nil {
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

	second, err := manager.Create(ctx, sessions.CreateInput{CWD: project, Profile: "personal", OptionalName: first.Name})
	if err != nil {
		t.Fatal("recreate lifetime fixture")
	}
	cleanup = &second
	if second.ID != first.ID || second.Name != first.Name || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"fixture did not recycle id and name under a new lifetime: id_match=%t name_match=%t identity_changed=%t",
			second.ID == first.ID,
			second.Name == first.Name,
			second.IdentityToken != first.IdentityToken,
		)
	}
	firstIdentityFields := strings.Split(first.IdentityToken, ".")
	secondIdentityFields := strings.Split(second.IdentityToken, ".")
	if len(firstIdentityFields) != 4 || len(secondIdentityFields) != 4 {
		t.Fatalf("fixture returned malformed lifetime identities: first_fields=%d second_fields=%d", len(firstIdentityFields), len(secondIdentityFields))
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
	err = manager.Kill(ctx, sessions.KillInput{ID: second.ID, DisplayedName: second.Name, IdentityToken: first.IdentityToken})
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
	for _, name := range []string{"personal", "work", "work2"} {
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
			{Key: "personal", Label: "Personal", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["personal"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work", Label: "Work", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
			{Key: "work2", Label: "Work 2", Command: agent, Environment: []sessions.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHomes["work2"]}}, ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}, {ExecutableBase: "codex"}, {ExecutableBase: "node", Argument1: nodeScript}}, Arguments: []string{yoloFlag}},
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

func integrationDeploymentHost() string {
	if platform.Current().Kind == platform.KindDarwin {
		return "macbook"
	}
	return "devbox"
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

func requireSessionNamed(t *testing.T, listed []sessions.Session, name string) sessions.Session {
	t.Helper()
	for _, session := range listed {
		if session.Name == name {
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
