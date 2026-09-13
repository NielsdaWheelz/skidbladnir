//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestAgentControlRetainsHistoryDeliversOnceAndRejectsChangedPane(t *testing.T) {
	fixture := newSessionFixture(t)
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("python3 is required for the isolated terminal fixture")
	}
	script := filepath.Join(fixture.root, "terminal-agent.py")
	received := filepath.Join(fixture.root, "received")
	if err := os.WriteFile(script, []byte(`import os, sys, tty
tty.setraw(0)
os.write(1, b"history-before-viewport\r\n" + b"history line\r\n" * 80)
os.write(1, b"\x1b[?2004h? for shortcuts\r\n")
with open(sys.argv[1], "wb", buffering=0) as output:
    while True:
        chunk = os.read(0, 4096)
        if not chunk:
            break
        output.write(chunk)
`), 0o600); err != nil {
		t.Fatal("write terminal fixture")
	}
	manager, err := sessions.New(sessions.Config{
		TmuxPath: tmuxPath, SocketName: fixture.socket,
		Workdir: fixture.workingDirectories, CataloguePath: fixture.cataloguePath,
		Profiles: []agentruntime.Profile{{
			Key: "personal", Label: "Codex · Personal", Provider: agentruntime.ProviderCodex,
			Command: python, Arguments: []string{script, received},
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: fixture.profileHomes["personal"]}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{Argument0: python, Argument1: script}},
		}},
	})
	if err != nil {
		t.Fatal("construct control fixture manager")
	}
	fixture.manager = manager
	control, err := agentcontrol.New(manager, filepath.Join(fixture.root, "unavailable-native-helper"))
	if err != nil {
		t.Fatal("construct agent control")
	}
	ctx := context.Background()
	// Ordinary tmux creation is the discovery boundary; no skid enrollment.
	fixture.tmux(t, "new-session", "-d", "-s", "ordinary-agent", "-c", fixture.project,
		"-e", "CODEX_HOME="+fixture.profileHomes["personal"], "--", python, script, received)
	observed := fixture.waitForAgent(t, ctx, "ordinary-agent")
	target := sessions.TargetOf(observed)
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	for {
		if _, err := os.Stat(received); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("terminal fixture did not become ready")
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	history, err := control.Read(ctx, target, "terminal", 16384)
	if err != nil || history.Source != "terminal" || history.Scope != "terminal_history" ||
		!strings.Contains(history.Text, "history-before-viewport") {
		t.Fatal("read did not retrieve retained history beyond the viewport")
	}
	if strings.Contains(fixture.tmux(t, "capture-pane", "-p", "-t", target.PaneID), "history-before-viewport") {
		t.Fatal("history fixture did not extend beyond the visible screen")
	}
	fixture.tmux(t, "set-buffer", "-b", "fixture-clipboard", "preserve")
	input := "first line\nsecond line $() ' \" λ"
	result, err := control.Send(ctx, target, input, "terminal")
	if err != nil || result.Method != "terminal" || result.Outcome != "written" {
		t.Fatal("explicit terminal send was not delivered")
	}
	expected := []byte("\x1b[200~" + input + "\x1b[201~\r")
	deadline = time.Now().Add(tmuxConvergenceTimeout)
	for {
		actual, err := os.ReadFile(received)
		if err != nil {
			t.Fatal("read fixture input receipt")
		}
		if len(actual) >= len(expected) {
			if !bytes.Equal(actual, expected) {
				t.Fatal("terminal send changed, duplicated, or split its bracketed input")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("terminal input receipt did not arrive")
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	if fixture.tmux(t, "show-buffer", "-b", "fixture-clipboard") != "preserve" ||
		strings.Contains(fixture.tmux(t, "list-buffers", "-F", "#{buffer_name}"), "skid-agent-") {
		t.Fatal("send changed another buffer or retained its own")
	}
	// A different active pane must make the old target stale.
	fixture.tmux(t, "split-window", "-t", observed.TmuxID, "--", sleepPath, "300")
	if _, err := control.Send(ctx, target, "must not arrive", "terminal"); !errors.Is(err, sessions.ErrAgentTargetStale) {
		t.Fatal("changed active pane was not rejected")
	}
	actual, err := os.ReadFile(received)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("stale target received input")
	}
	fixture.tmux(t, "select-pane", "-t", target.PaneID)
	stopped, err := control.Stop(ctx, target, manager.KillAgentTerminal)
	if err != nil || stopped.Terminal != "closed" || stopped.Agent != "unconfirmed" {
		t.Fatal("terminal-only stop misreported provider halt or closure")
	}
	inventory, err := manager.List(ctx)
	if err != nil {
		t.Fatal("list surviving isolated sessions")
	}
	requireSessionNamed(t, inventory, "skid-test-bootstrap")
	for _, session := range inventory.Sessions {
		if session.TmuxID == target.TmuxID {
			t.Fatal("stopped terminal remains")
		}
	}
}
