//go:build integration

package integration_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

const (
	agentHookSessionID = "integration-session-1"

	// justify-polling: the hook publishes through tmux and exposes completion
	// only through the test-owned filesystem and pane option boundaries.
	agentHookPollInterval = 25 * time.Millisecond
	agentHookTimeout      = 5 * time.Second
)

func TestAgentHookPublishesOnlyTheExactContentFreePaneRegistration(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("resolve repository root")
	}
	root := t.TempDir()
	hook := buildAgentHookCLI(t, repositoryRoot, root)
	provider := buildAgentHookProvider(t, root)
	hostConfig := writeAgentHookHostConfig(t, root, provider)
	start := filepath.Join(root, "start")
	done := filepath.Join(root, "done")
	claudeHome := filepath.Join(root, "claude-work")
	if err := os.Mkdir(claudeHome, 0o700); err != nil {
		t.Fatal("create provider home")
	}

	socket := randomTmuxSocketName(t, "skid-agent-hook")
	socketPath := namedTmuxSocketPath(socket)
	const sessionName = "agent-hook-publication"
	output, err := isolatedTmuxCommand(
		tmuxPath,
		"-L", socket,
		"-f", "/dev/null",
		"new-session", "-d", "-s", sessionName,
		"-e", "CLAUDE_CONFIG_DIR="+claudeHome,
		"--", provider, hook, hostConfig, start, done,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("start test-owned provider runtime: output_bytes=%d", len(output))
	}
	serverIdentity := captureTestTmuxServer(t, tmuxPath, socketPath)
	t.Cleanup(func() {
		stopTestTmuxServer(t, tmuxPath, socketPath, serverIdentity)
	})

	pane := agentHookTmux(t, socket, "display-message", "-p", "-t", "="+sessionName+":", "#{pane_id}")
	panePIDText := agentHookTmux(t, socket, "display-message", "-p", "-t", pane, "#{pane_pid}")
	panePID, err := strconv.Atoi(panePIDText)
	if err != nil || panePID <= 0 {
		t.Fatal("read test-owned provider process id")
	}
	agentHookTmux(t, socket, "set-option", "-p", "-t", pane, "--", "@skid_attention", "42")
	if got := agentHookTmux(t, socket, "display-message", "-p", "-t", pane,
		"#{@skid_agent_runtime}|#{@skid_lifecycle}|#{@skid_attention}|#{@skid_profile}|#{@skid_objective_b64}"); got != "||42||" {
		t.Fatal("test-owned pane did not begin with only its attention sentinel")
	}

	if err := os.WriteFile(start, nil, 0o600); err != nil {
		t.Fatal("trigger provider hook")
	}
	waitForAgentHookCompletion(t, done)
	registration := waitForAgentHookRegistration(t, socket, pane)
	observation, err := processinfo.Observe(processinfo.PID(panePID))
	if err != nil {
		t.Fatal("observe test-owned provider lifetime")
	}
	want, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider:      agentruntime.ProviderClaude,
		PID:           observation.PID,
		StartIdentity: observation.StartIdentity,
	}, "claude-work", agentHookSessionID)
	if err != nil {
		t.Fatal("encode expected test registration")
	}
	if registration != want {
		t.Fatal("published registration did not match the exact provider lifetime, profile, and session")
	}
	if got := agentHookTmux(t, socket, "display-message", "-p", "-t", pane,
		"#{@skid_lifecycle}|#{@skid_attention}|#{@skid_profile}|#{@skid_objective_b64}"); got != "|42||" {
		t.Fatal("Claude identity publication changed lifecycle, attention, or unrelated metadata")
	}
}

func agentHookTmux(t *testing.T, socket string, arguments ...string) string {
	t.Helper()
	commandArguments := append([]string{"-L", socket, "-f", "/dev/null"}, arguments...)
	output, err := isolatedTmuxCommand(tmuxPath, commandArguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("test-owned tmux command failed: argument_count=%d output_bytes=%d", len(arguments), len(output))
	}
	return strings.TrimSuffix(string(output), "\n")
}

func waitForAgentHookCompletion(t *testing.T, done string) {
	t.Helper()
	deadline := time.Now().Add(agentHookTimeout)
	for {
		encoded, err := os.ReadFile(done)
		if err == nil {
			if string(encoded) != "0\n" {
				t.Fatal("provider hook exited unsuccessfully")
			}
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read provider hook completion")
		}
		if time.Now().After(deadline) {
			t.Fatal("provider hook did not complete inside the proof window")
		}
		time.Sleep(agentHookPollInterval)
	}
}

func waitForAgentHookRegistration(t *testing.T, socket, pane string) string {
	t.Helper()
	deadline := time.Now().Add(agentHookTimeout)
	for {
		registration := agentHookTmux(t, socket, "display-message", "-p", "-t", pane, "#{@skid_agent_runtime}")
		if registration != "" {
			return registration
		}
		if time.Now().After(deadline) {
			t.Fatal("agent runtime registration did not appear inside the proof window")
		}
		time.Sleep(agentHookPollInterval)
	}
}

func buildAgentHookCLI(t *testing.T, repositoryRoot, destination string) string {
	t.Helper()
	command := filepath.Join(destination, "skidbladnir")
	build := exec.Command(agentHookGoTool(t), "build", "-o", command, "./cmd/skidbladnir")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build agent-hook CLI: output_bytes=%d", len(output))
	}
	return command
}

func buildAgentHookProvider(t *testing.T, destination string) string {
	t.Helper()
	source := filepath.Join(destination, "provider-source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal("create provider fixture source")
	}
	if err := os.WriteFile(filepath.Join(source, "main.go"), []byte(agentHookProviderProgram), 0o600); err != nil {
		t.Fatal("write provider fixture source")
	}
	command := filepath.Join(destination, "claude-hook-fixture")
	build := exec.Command(agentHookGoTool(t), "build", "-o", command, "main.go")
	build.Dir = source
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider fixture: output_bytes=%d", len(output))
	}
	return command
}

func agentHookGoTool(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("resolve Go toolchain")
	}
	return path
}

func writeAgentHookHostConfig(t *testing.T, destination, provider string) string {
	t.Helper()
	tmuxVersionOutput, err := exec.Command(tmuxPath, "-V").Output()
	if err != nil || len(tmuxVersionOutput) < 2 || tmuxVersionOutput[len(tmuxVersionOutput)-1] != '\n' ||
		bytes.ContainsRune(tmuxVersionOutput[:len(tmuxVersionOutput)-1], '\n') {
		t.Fatal("observe configured tmux version")
	}
	config := filepath.Join(destination, "host.json")
	encoded := fmt.Sprintf(`{
  "platform":%q,
  "tmux":{"path":%q,"testedVersion":%q},
  "profiles":[
    {"key":"personal","label":"Codex · Personal","provider":"Codex","command":"/bin/false","environment":[{"name":"CODEX_HOME","value":%q}],"foregroundSignatures":[{"executableBase":"codex-hook-fixture"}],"arguments":[]},
    {"key":"work","label":"Codex · Work","provider":"Codex","command":"/bin/false","environment":[{"name":"CODEX_HOME","value":%q}],"foregroundSignatures":[{"executableBase":"codex-hook-fixture"}],"arguments":[]},
    {"key":"work2","label":"Codex · Work 2","provider":"Codex","command":"/bin/false","environment":[{"name":"CODEX_HOME","value":%q}],"foregroundSignatures":[{"executableBase":"codex-hook-fixture"}],"arguments":[]},
    {"key":"claude-personal","label":"Claude · Personal","provider":"Claude","command":"/bin/false","environment":[{"name":"CLAUDE_CONFIG_DIR","value":%q}],"foregroundSignatures":[{"argument0":%q}],"arguments":[]},
    {"key":"claude-work","label":"Claude · Work","provider":"Claude","command":%q,"environment":[{"name":"CLAUDE_CONFIG_DIR","value":%q}],"foregroundSignatures":[{"argument0":%q}],"arguments":[]}
  ]
}`,
		platform.Current().Kind,
		tmuxPath,
		string(tmuxVersionOutput[:len(tmuxVersionOutput)-1]),
		filepath.Join(destination, "codex-personal"),
		filepath.Join(destination, "codex-work"),
		filepath.Join(destination, "codex-work2"),
		filepath.Join(destination, "claude-personal"), provider,
		provider, filepath.Join(destination, "claude-work"), provider,
	)
	if err := os.WriteFile(config, []byte(encoded), 0o600); err != nil {
		t.Fatal("write agent-hook host config")
	}
	return config
}

const agentHookProviderProgram = `package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	hookInput = "{\"session_id\":\"integration-session-1\"}"
	pollInterval = 25 * time.Millisecond
)

func main() {
	if len(os.Args) != 5 {
		os.Exit(2)
	}
	hook, hostConfig, start, done := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	for {
		if _, err := os.Stat(start); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			os.Exit(4)
		}
		time.Sleep(pollInterval)
	}
	command := exec.Command(hook, "agent-hook", "--host-config="+hostConfig, "Claude", "SessionStart")
	command.Stdin = strings.NewReader(hookInput)
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	status := []byte("0\n")
	if command.Run() != nil {
		status = []byte("1\n")
	}
	if os.WriteFile(done, status, 0o600) != nil {
		os.Exit(3)
	}
	for {
		time.Sleep(time.Hour)
	}
}
`
