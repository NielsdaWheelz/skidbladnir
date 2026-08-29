//go:build integration && providerlive

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const (
	providerLiveEnvironmentCapability = "provider-live-v1"
	providerLiveCLICapabilityV1       = "provider-live-cli-v1"
	providerLiveOperationTimeout      = 10 * time.Second
	providerLiveConvergenceTimeout    = 45 * time.Second
	providerLiveCleanupTimeout        = 10 * time.Second
	providerLivePollInterval          = 250 * time.Millisecond
	providerLiveHoldTimeoutSeconds    = 120
	providerLiveHoldDeadlineSeconds   = 100
	providerLiveLifetimeCapSeconds    = 110
	providerLiveTerminationGrace      = 5
	providerLiveOpaqueInputBytes      = 32
	providerLiveWorkspacePrefix       = ".skid-provider-live-"
	providerLiveCodexModel            = "skidbladnir-provider-live-offline"
	providerLiveCodexProvider         = "skidbladnir_provider_live"
	providerLiveCodexProviderName     = "Skidbladnir provider live"
	providerLiveClaudeBaseURL         = "http://127.0.0.1:0"
	providerLiveNoProxy               = "127.0.0.1,localhost"
	providerLiveTestCommandTimeout    = 5 * time.Second
	providerLiveTestFailClosedTimeout = 6 * time.Second
	providerLiveTestObserveTimeout    = time.Second
	providerLiveTestCleanupTimeout    = 2 * time.Second
	providerLiveTestPollInterval      = 10 * time.Millisecond
	providerLiveTestDeadlineSeconds   = 1
	providerLiveTestGraceSeconds      = 1
	providerLiveTestChildSeconds      = 30
)

var (
	providerLiveCLICapability = flag.String(
		"skidbladnir-provider-live-capability",
		"",
		"second explicit capability required for installed live-provider execution",
	)
	providerLiveReleaseTag = flag.String(
		"skidbladnir-provider-live-release-tag",
		"",
		"exact installed release tag required by the live-provider proof",
	)
	providerLiveSourceSHA = flag.String(
		"skidbladnir-provider-live-source-sha",
		"",
		"exact installed source SHA required by the live-provider proof",
	)
	providerLiveTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	providerLiveSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type providerLivePreflight struct {
	repositoryRoot string
	home           string
	cataloguePath  string
	host           hostconfig.Config
	plan           providerLivePlan
}

type providerLivePlan struct {
	managedProfile    agentruntime.Profile
	managedTmuxName   string
	laptopProfile     agentruntime.Profile
	laptopTmuxName    string
	laptopSessionName string
}

type providerLiveHarness struct {
	workspace        string
	codexLauncher    string
	codexBaseURL     string
	claudeLauncher   string
	claudeHoldPlugin string
	dispatchSentinel *providerLiveDispatchSentinel
}

type providerLiveDispatchSentinel struct {
	listener   net.Listener
	accepted   atomic.Uint64
	closeOnce  sync.Once
	assertOnce sync.Once
	done       chan struct{}
	errMu      sync.Mutex
	serveErr   error
}

type providerLiveExpectation struct {
	tmuxName            string
	provider            agentruntime.Provider
	runtimeProfile      agentruntime.ProfileKey
	launchProfile       agentruntime.ProfileKey
	providerSessionName string
}

type providerLiveProjectionSummary struct {
	sessionPresent         bool
	agentPresent           bool
	providerMatches        bool
	pidPresent             bool
	runtimeProfileMatches  bool
	providerSessionPresent bool
	providerSessionID      bool
	providerSessionName    bool
	launchProfileMatches   bool
}

type providerLiveFakeObservation struct {
	arguments   []string
	stdin       []byte
	stdinKind   string
	environment map[string]string
}

type providerLiveOwnedCommand struct {
	command *exec.Cmd
	groupID int
	psPath  string
	cleaned bool
}

func TestProviderLiveLaunchersEnforceOfflineInputBoundary(t *testing.T) {
	home := t.TempDir()
	harness := newProviderLiveHarness(t, home)
	fakeProvider := newProviderLiveFakeProvider(t, home)
	probeDirectory := filepath.Join(home, "probe")
	if err := os.Mkdir(probeDirectory, 0o700); err != nil {
		t.Fatal("create provider-live fake-provider probe directory")
	}

	codexProfile := providerLiveExecutionProfile(agentruntime.Profile{
		Provider: agentruntime.ProviderCodex,
		Command:  fakeProvider,
		Environment: []agentruntime.EnvironmentVariable{
			{Name: "CODEX_HOME", Value: "/profile/codex"},
			{Name: "SKIDBLADNIR_PROVIDER_LIVE_PRESERVED", Value: "codex-preserved"},
		},
		Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
	}, harness)
	codexFirst := runProviderLiveFakeProvider(t, codexProfile, "", harness.workspace, probeDirectory, "codex-first")
	codexSecond := runProviderLiveFakeProvider(t, codexProfile, "", harness.workspace, probeDirectory, "codex-second")
	wantCodexArguments := []string{
		"exec", "--ephemeral", "--dangerously-bypass-hook-trust",
		"--dangerously-bypass-approvals-and-sandbox",
		"--config", "features.hooks=true",
		"--model", "skidbladnir-provider-live-offline",
		"--config", `model_provider="skidbladnir_provider_live"`,
		"--config", `model_providers.skidbladnir_provider_live.name="Skidbladnir provider live"`,
		"--config", `model_providers.skidbladnir_provider_live.base_url="` + harness.codexBaseURL + `"`,
		"--config", `model_providers.skidbladnir_provider_live.wire_api="responses"`,
		"--config", "model_providers.skidbladnir_provider_live.requires_openai_auth=false",
		"--config", "model_providers.skidbladnir_provider_live.request_max_retries=0",
		"--config", "model_providers.skidbladnir_provider_live.stream_max_retries=0",
		"--config", "model_providers.skidbladnir_provider_live.supports_websockets=false",
		"--config", `otel.exporter="none"`,
		"--config", `otel.metrics_exporter="none"`,
		"--config", `otel.trace_exporter="none"`,
		"--config", "otel.log_user_prompt=false",
		"-",
	}
	hexInput := regexp.MustCompile(fmt.Sprintf(`^[0-9a-f]{%d}$`, providerLiveOpaqueInputBytes*2))
	for _, observation := range []providerLiveFakeObservation{codexFirst, codexSecond} {
		if !slices.Equal(observation.arguments, wantCodexArguments) ||
			observation.stdinKind != "pipe" || !hexInput.Match(observation.stdin) {
			t.Fatal("Codex launcher did not preserve the bounded anonymous offline execution contract")
		}
		if observation.environment["CODEX_HOME"] != "/profile/codex" ||
			observation.environment["SKIDBLADNIR_PROVIDER_LIVE_PRESERVED"] != "codex-preserved" ||
			observation.environment["NO_PROXY"] != providerLiveNoProxy ||
			observation.environment["no_proxy"] != providerLiveNoProxy {
			t.Fatal("Codex launcher lost profile environment or loopback proxy isolation")
		}
		for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			if observation.environment[name+"_SET"] != "" {
				t.Fatal("Codex launcher retained an ambient provider proxy")
			}
		}
	}
	if bytes.Equal(codexFirst.stdin, codexSecond.stdin) {
		t.Fatal("Codex launcher reused its generated opaque input")
	}

	claudeName := "provider-live-fake-claude"
	claudeProfile := providerLiveExecutionProfile(agentruntime.Profile{
		Provider: agentruntime.ProviderClaude,
		Command:  fakeProvider,
		Environment: []agentruntime.EnvironmentVariable{
			{Name: "CLAUDE_CONFIG_DIR", Value: "/profile/claude"},
			{Name: "SKIDBLADNIR_PROVIDER_LIVE_PRESERVED", Value: "claude-preserved"},
		},
		Arguments: []string{"--permission-mode", "auto"},
	}, harness)
	claude := runProviderLiveFakeProvider(t, claudeProfile, claudeName, harness.workspace, probeDirectory, "claude")
	wantClaudeArguments := []string{
		"--name", claudeName,
		"--permission-mode", "auto",
		"-p", "--no-session-persistence", "--plugin-dir", harness.claudeHoldPlugin,
	}
	if !slices.Equal(claude.arguments, wantClaudeArguments) ||
		claude.stdinKind != "character" || len(claude.stdin) != 0 {
		t.Fatal("Claude launcher did not preserve null input and no-persistence execution")
	}
	if claude.environment["CLAUDE_CONFIG_DIR"] != "/profile/claude" ||
		claude.environment["SKIDBLADNIR_PROVIDER_LIVE_PRESERVED"] != "claude-preserved" ||
		claude.environment["ANTHROPIC_BASE_URL"] != providerLiveClaudeBaseURL {
		t.Fatal("Claude launcher lost profile environment or its defense-in-depth base URL")
	}
}

func TestProviderLiveFailClosedScriptsTerminateOwnedProcessGroup(t *testing.T) {
	for _, testCase := range []struct {
		name             string
		useHold          bool
		providerIdentity string
	}{
		{
			name:             "hold with live provider identity",
			useHold:          true,
			providerIdentity: "export SKIDBLADNIR_PROVIDER_LIVE_PID=$$",
		},
		{
			name:             "hold with missing provider identity",
			useHold:          true,
			providerIdentity: "unset SKIDBLADNIR_PROVIDER_LIVE_PID",
		},
		{
			name:             "watchdog with live provider identity",
			providerIdentity: "export SKIDBLADNIR_PROVIDER_LIVE_PID=$$",
		},
		{
			name:             "watchdog with malformed provider identity",
			providerIdentity: "export SKIDBLADNIR_PROVIDER_LIVE_PID=invalid",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			directory := t.TempDir()
			childPIDPath := filepath.Join(directory, "child.pid")
			bashPath := requireProviderLivePATHExecutable(t, "bash")
			action := providerLiveLifetimeGuardWithBounds(
				providerLiveTestDeadlineSeconds,
				providerLiveTestGraceSeconds,
			)
			if testCase.useHold {
				holdPath := filepath.Join(directory, "hold")
				writeProviderLiveFile(t, holdPath, []byte(providerLiveHoldScriptWithBounds(
					providerLiveTestDeadlineSeconds,
					providerLiveTestGraceSeconds,
				)), 0o700)
				action = shellQuote(holdPath) + " </dev/null &"
			}
			controllerPath := filepath.Join(directory, "controller")
			controllerScript := fmt.Sprintf(`#!%s
set -euo pipefail
trap '' HUP INT TERM
%s %d &
child=$!
printf '%%s\n' "$child" >%s
IFS= read -r ready
%s
%s
wait "$child"
`, bashPath, shellQuote(sleepPath), providerLiveTestChildSeconds, shellQuote(childPIDPath), testCase.providerIdentity, action)
			writeProviderLiveFile(t, controllerPath, []byte(controllerScript), 0o700)

			operationContext, cancel := context.WithTimeout(context.Background(), providerLiveTestFailClosedTimeout)
			defer cancel()
			command := exec.CommandContext(operationContext, controllerPath)
			command.Env = withoutEnvironment(os.Environ(), "SKIDBLADNIR_PROVIDER_LIVE_PID")
			gate, err := command.StdinPipe()
			if err != nil {
				t.Fatal("create provider-live fail-closed start gate")
			}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			owned := startProviderLiveOwnedCommand(t, command)
			childPID, childStart := waitForProviderLiveFakeChild(t, childPIDPath)
			if _, err := gate.Write([]byte{'\n'}); err != nil {
				t.Fatal("release provider-live fail-closed process-group probe")
			}
			if err := gate.Close(); err != nil {
				t.Fatal("close provider-live fail-closed start gate")
			}
			waitErr := owned.command.Wait()
			groupEnded, groupErr := owned.waitForEnd(providerLiveTestObserveTimeout)
			childEnded := waitForProviderLiveProcessEnd(childPID, childStart, providerLiveTestObserveTimeout)
			cleanupErr := owned.cleanup()
			if cleanupErr != nil {
				t.Fatal("clean exact provider-live process-group probe")
			}
			if groupErr != nil {
				t.Fatal("observe exact provider-live process-group probe")
			}
			var exitErr *exec.ExitError
			status, signaled := syscall.WaitStatus(0), false
			if errors.As(waitErr, &exitErr) {
				if observed, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					status, signaled = observed, true
				}
			}
			if operationContext.Err() != nil || !signaled || !status.Signaled() || status.Signal() != syscall.SIGKILL ||
				!groupEnded || !childEnded || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal("provider-live fail-closed script did not kill its exact owned process group")
			}
		})
	}
}

func waitForProviderLiveFakeChild(t *testing.T, path string) (int, processinfo.StartIdentity) {
	t.Helper()
	// justify-polling: the exact grandchild exposes no wait handle to the Go
	// parent, so its private PID file is the narrow cross-process handshake.
	deadline := time.Now().Add(providerLiveTestObserveTimeout)
	for time.Now().Before(deadline) {
		encoded, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(encoded)))
			if parseErr != nil || pid <= 1 {
				t.Fatal("parse provider-live fake child PID")
			}
			if start := processStartIdentity(pid); start != "" {
				return pid, start
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read provider-live fake child PID")
		}
		time.Sleep(providerLiveTestPollInterval)
	}
	t.Fatal("provider-live fake child did not start")
	return 0, ""
}

func waitForProviderLiveProcessEnd(pid int, start processinfo.StartIdentity, timeout time.Duration) bool {
	// justify-polling: start-identity observation is the only exact liveness
	// boundary available for a grandchild that the Go parent cannot wait on.
	deadline := time.Now().Add(timeout)
	for {
		if processStartIdentity(pid) != start {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(providerLiveTestPollInterval)
	}
}

func startProviderLiveOwnedCommand(t *testing.T, command *exec.Cmd) *providerLiveOwnedCommand {
	t.Helper()
	psPath := requireProviderLivePATHExecutable(t, "ps")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = providerLiveTestObserveTimeout
	if err := command.Start(); err != nil {
		t.Fatal("start exact provider-live test process group")
	}
	groupID := command.Process.Pid
	observedGroup, err := syscall.Getpgid(groupID)
	if err != nil || observedGroup != groupID {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatal("provider-live test process did not lead its isolated process group")
	}
	owned := &providerLiveOwnedCommand{
		command: command,
		groupID: groupID,
		psPath:  psPath,
	}
	t.Cleanup(func() {
		if err := owned.cleanup(); err != nil {
			t.Error("clean exact provider-live test process group")
		}
	})
	return owned
}

func (owned *providerLiveOwnedCommand) cleanup() error {
	if owned.cleaned {
		return nil
	}
	running, err := providerLiveOwnedGroupRunning(owned.psPath, owned.groupID)
	if err != nil {
		return err
	}
	if running {
		if err := syscall.Kill(-owned.groupID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	ended, err := owned.waitForEnd(providerLiveTestCleanupTimeout)
	if err != nil {
		return err
	}
	if !ended {
		return errors.New("provider-live test process group survived exact cleanup")
	}
	owned.cleaned = true
	return nil
}

func providerLiveOwnedGroupRunning(psPath string, groupID int) (bool, error) {
	if groupID <= 1 {
		return false, errors.New("invalid provider-live test process group")
	}
	observationContext, cancel := context.WithTimeout(context.Background(), providerLiveTestObserveTimeout)
	defer cancel()
	command := exec.CommandContext(observationContext, psPath, "-ax", "-o", "pid=", "-o", "pgid=", "-o", "stat=")
	command.Env = append(withoutEnvironment(os.Environ(), "LC_ALL"), "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		return false, errors.New("observe provider-live exact process group")
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return false, errors.New("parse provider-live process-group observation")
		}
		pid, pidErr := strconv.Atoi(fields[0])
		observedGroup, groupErr := strconv.Atoi(fields[1])
		if pidErr != nil || groupErr != nil || pid <= 0 || observedGroup < 0 || fields[2] == "" {
			return false, errors.New("parse provider-live process-group observation")
		}
		if observedGroup != groupID {
			continue
		}
		if fields[2][0] != 'Z' {
			return true, nil
		}
	}
	// A zombie retains only a kernel process-table entry: it cannot execute,
	// hold a socket, or survive its eventual parent reap. All-Z is drained.
	return false, nil
}

func (owned *providerLiveOwnedCommand) waitForEnd(timeout time.Duration) (bool, error) {
	// justify-polling: content-free process-state observation is the only
	// aggregate drain boundary for test-owned grandchildren after their leader
	// exits. Kernel signal-zero includes inert zombies and is therefore too broad.
	deadline := time.Now().Add(timeout)
	for {
		running, err := providerLiveOwnedGroupRunning(owned.psPath, owned.groupID)
		if err != nil || !running {
			return !running, err
		}
		if !time.Now().Before(deadline) {
			return false, nil
		}
		time.Sleep(providerLiveTestPollInterval)
	}
}

func newProviderLiveFakeProvider(t *testing.T, directory string) string {
	t.Helper()
	catPath := requireProviderLivePATHExecutable(t, "cat")
	path := filepath.Join(directory, "fake-provider")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
umask 077
argv_path=${SKIDBLADNIR_PROVIDER_LIVE_FAKE_ARGV_PATH:?}
stdin_path=${SKIDBLADNIR_PROVIDER_LIVE_FAKE_STDIN_PATH:?}
metadata_path=${SKIDBLADNIR_PROVIDER_LIVE_FAKE_METADATA_PATH:?}
stdin_kind=other
if [ -p /dev/fd/0 ]; then
  stdin_kind=pipe
elif [ -c /dev/fd/0 ]; then
  stdin_kind=character
elif [ -f /dev/fd/0 ]; then
  stdin_kind=regular
fi
printf '%%s\n' "$@" >"$argv_path"
{
  printf 'stdin_kind=%%s\n' "$stdin_kind"
  printf 'CODEX_HOME=%%s\n' "${CODEX_HOME-}"
  printf 'CLAUDE_CONFIG_DIR=%%s\n' "${CLAUDE_CONFIG_DIR-}"
  printf 'SKIDBLADNIR_PROVIDER_LIVE_PRESERVED=%%s\n' "${SKIDBLADNIR_PROVIDER_LIVE_PRESERVED-}"
  printf 'ANTHROPIC_BASE_URL=%%s\n' "${ANTHROPIC_BASE_URL-}"
  printf 'HTTP_PROXY_SET=%%s\n' "${HTTP_PROXY+x}"
  printf 'HTTPS_PROXY_SET=%%s\n' "${HTTPS_PROXY+x}"
  printf 'ALL_PROXY_SET=%%s\n' "${ALL_PROXY+x}"
  printf 'http_proxy_SET=%%s\n' "${http_proxy+x}"
  printf 'https_proxy_SET=%%s\n' "${https_proxy+x}"
  printf 'all_proxy_SET=%%s\n' "${all_proxy+x}"
  printf 'NO_PROXY=%%s\n' "${NO_PROXY-}"
  printf 'no_proxy=%%s\n' "${no_proxy-}"
} >"$metadata_path"
exec %s >"$stdin_path"
`, shellQuote(catPath))
	writeProviderLiveFile(t, path, []byte(script), 0o700)
	return path
}

func runProviderLiveFakeProvider(
	t *testing.T,
	profile agentruntime.Profile,
	providerName string,
	workingDirectory string,
	probeDirectory string,
	probeName string,
) providerLiveFakeObservation {
	t.Helper()
	argvPath := filepath.Join(probeDirectory, probeName+".argv")
	stdinPath := filepath.Join(probeDirectory, probeName+".stdin")
	metadataPath := filepath.Join(probeDirectory, probeName+".metadata")
	operationContext, cancel := context.WithTimeout(context.Background(), providerLiveTestCommandTimeout)
	defer cancel()
	command := exec.CommandContext(operationContext, profile.Command, agentruntime.LaunchArguments(profile, providerName)...)
	command.Dir = workingDirectory
	environment := withoutEnvironment(
		os.Environ(),
		"CODEX_HOME",
		"CLAUDE_CONFIG_DIR",
		"SKIDBLADNIR_PROVIDER_LIVE_PRESERVED",
		"ANTHROPIC_BASE_URL",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"ALL_PROXY",
		"http_proxy",
		"https_proxy",
		"all_proxy",
		"NO_PROXY",
		"no_proxy",
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_ARGV_PATH",
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_STDIN_PATH",
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_METADATA_PATH",
	)
	for _, variable := range profile.Environment {
		environment = append(environment, variable.Name+"="+variable.Value)
	}
	environment = append(environment,
		"ANTHROPIC_BASE_URL=https://example.invalid",
		"HTTP_PROXY=http://192.0.2.1:9",
		"HTTPS_PROXY=http://192.0.2.1:9",
		"ALL_PROXY=http://192.0.2.1:9",
		"http_proxy=http://192.0.2.1:9",
		"https_proxy=http://192.0.2.1:9",
		"all_proxy=http://192.0.2.1:9",
		"NO_PROXY=example.invalid",
		"no_proxy=example.invalid",
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_ARGV_PATH="+argvPath,
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_STDIN_PATH="+stdinPath,
		"SKIDBLADNIR_PROVIDER_LIVE_FAKE_METADATA_PATH="+metadataPath,
	)
	command.Env = environment
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	owned := startProviderLiveOwnedCommand(t, command)
	waitErr := owned.command.Wait()
	cleanupErr := owned.cleanup()
	if cleanupErr != nil {
		t.Fatal("clean exact provider-live fake-provider process group")
	}
	if waitErr != nil || operationContext.Err() != nil {
		t.Fatalf(
			"execute provider-live fake provider: stdout_bytes=%d stderr_bytes=%d",
			stdout.Len(),
			stderr.Len(),
		)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf(
			"provider-live fake provider emitted output: stdout_bytes=%d stderr_bytes=%d",
			stdout.Len(),
			stderr.Len(),
		)
	}
	argumentBytes, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal("read provider-live fake-provider arguments")
	}
	stdin, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatal("read provider-live fake-provider stdin")
	}
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal("read provider-live fake-provider metadata")
	}
	metadata := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(metadataBytes), "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found || key == "" {
			t.Fatal("parse provider-live fake-provider metadata")
		}
		if _, duplicate := metadata[key]; duplicate {
			t.Fatal("provider-live fake-provider metadata contains a duplicate key")
		}
		metadata[key] = value
	}
	arguments := strings.Split(strings.TrimSuffix(string(argumentBytes), "\n"), "\n")
	return providerLiveFakeObservation{
		arguments:   arguments,
		stdin:       stdin,
		stdinKind:   metadata["stdin_kind"],
		environment: metadata,
	}
}

func providerLiveExecutionProfile(profile agentruntime.Profile, harness providerLiveHarness) agentruntime.Profile {
	installedCommand := profile.Command
	switch profile.Provider {
	case agentruntime.ProviderCodex:
		profile.Command = harness.codexLauncher
		arguments := []string{
			installedCommand,
			"exec",
			"--ephemeral",
			"--dangerously-bypass-hook-trust",
		}
		arguments = append(arguments, profile.Arguments...)
		arguments = append(arguments, providerLiveCodexOfflineArguments(harness.codexBaseURL)...)
		profile.Arguments = append(arguments, "-")
	case agentruntime.ProviderClaude:
		profile.Command = harness.claudeLauncher
		arguments := append([]string{installedCommand}, profile.Arguments...)
		profile.Arguments = append(
			arguments,
			"-p",
			"--no-session-persistence",
			"--plugin-dir",
			harness.claudeHoldPlugin,
		)
	default:
		panic("invalid validated provider-live profile")
	}
	return profile
}

func providerLiveCodexOfflineArguments(baseURL string) []string {
	providerPrefix := "model_providers." + providerLiveCodexProvider + "."
	return []string{
		"--config", "features.hooks=true",
		"--model", providerLiveCodexModel,
		"--config", "model_provider=" + strconv.Quote(providerLiveCodexProvider),
		"--config", providerPrefix + "name=" + strconv.Quote(providerLiveCodexProviderName),
		"--config", providerPrefix + "base_url=" + strconv.Quote(baseURL),
		"--config", providerPrefix + `wire_api="responses"`,
		"--config", providerPrefix + "requires_openai_auth=false",
		"--config", providerPrefix + "request_max_retries=0",
		"--config", providerPrefix + "stream_max_retries=0",
		"--config", providerPrefix + "supports_websockets=false",
		"--config", `otel.exporter="none"`,
		"--config", `otel.metrics_exporter="none"`,
		"--config", `otel.trace_exporter="none"`,
		"--config", "otel.log_user_prompt=false",
	}
}

func newProviderLiveDispatchSentinel(t *testing.T) *providerLiveDispatchSentinel {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("start content-free provider-live dispatch sentinel")
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.Equal(net.IPv4(127, 0, 0, 1)) || address.Port <= 0 {
		_ = listener.Close()
		t.Fatal("provider-live dispatch sentinel is not loopback-bound")
	}
	sentinel := &providerLiveDispatchSentinel{
		listener: listener,
		done:     make(chan struct{}),
	}
	go sentinel.serve()
	t.Cleanup(func() {
		sentinel.assertNoDispatch(t)
	})
	return sentinel
}

func (sentinel *providerLiveDispatchSentinel) baseURL() string {
	return "http://" + sentinel.listener.Addr().String()
}

// The sentinel deliberately never reads connection bytes. A connection count
// is sufficient to prove whether Codex reached its test-only provider route.
func (sentinel *providerLiveDispatchSentinel) serve() {
	defer close(sentinel.done)
	for {
		connection, err := sentinel.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				sentinel.recordServeError(err)
			}
			return
		}
		sentinel.accepted.Add(1)
		if err := connection.Close(); err != nil {
			sentinel.recordServeError(err)
			return
		}
	}
}

func (sentinel *providerLiveDispatchSentinel) recordServeError(err error) {
	sentinel.errMu.Lock()
	defer sentinel.errMu.Unlock()
	if sentinel.serveErr == nil {
		sentinel.serveErr = err
	}
}

func (sentinel *providerLiveDispatchSentinel) assertNoDispatch(t *testing.T) {
	t.Helper()
	sentinel.assertOnce.Do(func() {
		sentinel.closeOnce.Do(func() {
			if err := sentinel.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				sentinel.recordServeError(err)
			}
		})
		<-sentinel.done
		sentinel.errMu.Lock()
		serveFailed := sentinel.serveErr != nil
		sentinel.errMu.Unlock()
		if serveFailed {
			t.Error("provider-live content-free dispatch sentinel failed")
		}
		if accepted := sentinel.accepted.Load(); accepted != 0 {
			t.Errorf("provider-live held Codex reached its offline provider route: connection_count=%d", accepted)
		}
	})
}

func newProviderLiveHarness(t *testing.T, home string) providerLiveHarness {
	t.Helper()
	maximumHarnessDuration := 2*providerLiveOperationTimeout +
		providerLiveConvergenceTimeout +
		2*testTmuxCleanupTimeout +
		providerLiveCleanupTimeout
	if maximumHarnessDuration >= time.Duration(providerLiveLifetimeCapSeconds)*time.Second ||
		maximumHarnessDuration >= time.Duration(providerLiveHoldDeadlineSeconds)*time.Second ||
		providerLiveHoldDeadlineSeconds >= providerLiveLifetimeCapSeconds ||
		providerLiveLifetimeCapSeconds >= providerLiveHoldTimeoutSeconds {
		t.Fatal("provider-live lifetime bounds could release a held provider input")
	}
	dispatchSentinel := newProviderLiveDispatchSentinel(t)
	workspace, err := os.MkdirTemp(home, providerLiveWorkspacePrefix)
	if err != nil {
		t.Fatal("create private provider-live workspace")
	}
	t.Cleanup(func() {
		if err := removeProviderLiveWorkspace(home, workspace); err != nil {
			t.Error(err)
		}
	})

	gitCommand := exec.Command("git", "-C", workspace, "init", "--quiet")
	gitCommand.Env = withoutEnvironment(
		os.Environ(),
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_OBJECT_DIRECTORY",
	)
	if output, err := gitCommand.CombinedOutput(); err != nil {
		t.Fatalf("initialize private provider-live project: output_bytes=%d", len(output))
	}

	hold := filepath.Join(workspace, "session-start-hold")
	writeProviderLiveFile(t, hold, []byte(providerLiveHoldScript()), 0o700)

	bashPath := requireProviderLivePATHExecutable(t, "bash")
	odPath := requireProviderLivePATHExecutable(t, "od")
	trPath := requireProviderLivePATHExecutable(t, "tr")
	codexLauncher := filepath.Join(workspace, "codex-launcher")
	lifetimeGuard := providerLiveLifetimeGuard()
	codexScript := fmt.Sprintf(`#!%s
set -euo pipefail
if [ "$#" -lt 1 ]; then
  exit 64
fi
provider=$1
shift
export SKIDBLADNIR_PROVIDER_LIVE_PID=$$
%s
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy NO_PROXY no_proxy
NO_PROXY=%s
no_proxy=%s
export NO_PROXY no_proxy
LC_ALL=C
export LC_ALL
exec "$provider" "$@" < <(
  %s -An -N%d -tx1 /dev/urandom |
    %s -d '[:space:]'
)
`, bashPath, lifetimeGuard, shellQuote(providerLiveNoProxy), shellQuote(providerLiveNoProxy), shellQuote(odPath), providerLiveOpaqueInputBytes, shellQuote(trPath))
	writeProviderLiveFile(t, codexLauncher, []byte(codexScript), 0o700)

	claudeLauncher := filepath.Join(workspace, "claude-launcher")
	claudeScript := fmt.Sprintf(`#!%s
set -euo pipefail
if [ "$#" -lt 3 ] || [ "$1" != "--name" ]; then
  exit 64
fi
provider_name=$2
shift 2
provider=$1
shift
export SKIDBLADNIR_PROVIDER_LIVE_PID=$$
%s
ANTHROPIC_BASE_URL=%s
export ANTHROPIC_BASE_URL
exec "$provider" --name "$provider_name" "$@" </dev/null
`, bashPath, lifetimeGuard, shellQuote(providerLiveClaudeBaseURL))
	writeProviderLiveFile(t, claudeLauncher, []byte(claudeScript), 0o700)

	codexDirectory := filepath.Join(workspace, ".codex")
	claudeHoldPlugin := filepath.Join(workspace, "claude-hold")
	for _, directory := range []string{
		codexDirectory,
		filepath.Join(claudeHoldPlugin, ".claude-plugin"),
		filepath.Join(claudeHoldPlugin, "hooks"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal("create provider-live hook directory")
		}
	}
	writeProviderLiveJSON(t, filepath.Join(codexDirectory, "hooks.json"), map[string]any{
		"description": "Skidbladnir provider-live synchronous hold",
		"hooks": map[string]any{
			"SessionStart": []any{map[string]any{
				"matcher": "^startup$",
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": "exec " + shellQuote(hold),
					"timeout": providerLiveHoldTimeoutSeconds,
					"async":   false,
				}},
			}},
		},
	})
	writeProviderLiveJSON(t, filepath.Join(claudeHoldPlugin, ".claude-plugin", "plugin.json"), map[string]string{
		"name":        "skidbladnir-provider-live-hold",
		"description": "Skidbladnir provider-live synchronous hold",
	})
	writeProviderLiveJSON(t, filepath.Join(claudeHoldPlugin, "hooks", "hooks.json"), map[string]any{
		"description": "Skidbladnir provider-live synchronous hold",
		"hooks": map[string]any{
			"SessionStart": []any{map[string]any{
				"matcher": "startup",
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": hold,
					"args":    []string{},
					"timeout": providerLiveHoldTimeoutSeconds,
					"async":   false,
				}},
			}},
		},
	})

	return providerLiveHarness{
		workspace:        workspace,
		codexLauncher:    codexLauncher,
		codexBaseURL:     dispatchSentinel.baseURL(),
		claudeLauncher:   claudeLauncher,
		claudeHoldPlugin: claudeHoldPlugin,
		dispatchSentinel: dispatchSentinel,
	}
}

func providerLiveHoldScript() string {
	return providerLiveHoldScriptWithBounds(providerLiveHoldDeadlineSeconds, providerLiveTerminationGrace)
}

func providerLiveHoldScriptWithBounds(deadlineSeconds, graceSeconds int) string {
	if deadlineSeconds <= 0 || graceSeconds <= 0 {
		panic("invalid provider-live hold bounds")
	}
	return fmt.Sprintf(`#!/bin/sh
set -eu
provider=${SKIDBLADNIR_PROVIDER_LIVE_PID:-}
provider_is_live() {
  case "$provider" in
    ''|*[!0-9]*|0) return 1 ;;
  esac
  kill -0 "$provider" 2>/dev/null
}
terminate_pane_process_group() {
  trap '' HUP INT TERM
  kill -TERM 0 2>/dev/null || :
  grace=%d
  while provider_is_live && [ "$grace" -gt 0 ]; do
    %s 1
    grace=$((grace - 1))
  done
  kill -KILL 0 2>/dev/null || :
  exit 70
}
trap terminate_pane_process_group HUP INT TERM
case "$provider" in
  ''|*[!0-9]*|0) terminate_pane_process_group ;;
esac
while IFS= read -r discarded; do
  :
done
remaining=%d
while provider_is_live; do
  if [ "$remaining" -le 0 ]; then
    terminate_pane_process_group
  fi
  %s 1
  remaining=$((remaining - 1))
done
exit 70
`, graceSeconds, shellQuote(sleepPath), deadlineSeconds, shellQuote(sleepPath))
}

// The exec-preserved provider PID is only the liveness identity. Fail-closed
// signals target the current test-owned pane process group, so PID reuse cannot
// redirect them outside the test boundary.
func providerLiveLifetimeGuard() string {
	return providerLiveLifetimeGuardWithBounds(providerLiveLifetimeCapSeconds, providerLiveTerminationGrace)
}

func providerLiveLifetimeGuardWithBounds(capSeconds, graceSeconds int) string {
	if capSeconds <= 0 || graceSeconds <= 0 {
		panic("invalid provider-live lifetime bounds")
	}
	return fmt.Sprintf(`(
  provider_live_pid=${SKIDBLADNIR_PROVIDER_LIVE_PID:-}
  remaining=%d
  case "$provider_live_pid" in
    ''|*[!0-9]*|0) remaining=0 ;;
  esac
  while [ "$remaining" -gt 0 ]; do
    %s 1
    if ! kill -0 "$provider_live_pid" 2>/dev/null; then
      exit 0
    fi
    remaining=$((remaining - 1))
  done
  trap '' HUP INT TERM
  kill -TERM 0 2>/dev/null || :
  grace=%d
  while kill -0 "$provider_live_pid" 2>/dev/null && [ "$grace" -gt 0 ]; do
    %s 1
    grace=$((grace - 1))
  done
  kill -KILL 0 2>/dev/null || :
  exit 70
) </dev/null >/dev/null 2>&1 &`, capSeconds, shellQuote(sleepPath), graceSeconds, shellQuote(sleepPath))
}

func requireProviderLivePATHExecutable(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil || !filepath.IsAbs(path) {
		t.Fatal("resolve provider-live support executable")
	}
	path = filepath.Clean(path)
	requireProviderLiveExecutable(t, path)
	return path
}

func writeProviderLiveFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal("write private provider-live harness file")
	}
}

func writeProviderLiveJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal("encode private provider-live hook configuration")
	}
	writeProviderLiveFile(t, path, append(encoded, '\n'), 0o600)
}

func removeProviderLiveWorkspace(home, workspace string) error {
	relative, err := filepath.Rel(home, workspace)
	if err != nil || filepath.Dir(relative) != "." ||
		!strings.HasPrefix(relative, providerLiveWorkspacePrefix) || relative == providerLiveWorkspacePrefix {
		return errors.New("refuse unsafe provider-live workspace cleanup")
	}
	if err := os.RemoveAll(workspace); err != nil {
		return errors.New("remove private provider-live workspace")
	}
	if _, err := os.Lstat(workspace); !errors.Is(err, os.ErrNotExist) {
		return errors.New("private provider-live workspace survived cleanup")
	}
	return nil
}

func providerLiveManagedProfiles(
	t *testing.T,
	profiles []agentruntime.Profile,
	managed agentruntime.Profile,
	harness providerLiveHarness,
) []agentruntime.Profile {
	t.Helper()
	profiles = agentruntime.CloneProfiles(profiles)
	found := false
	for index, profile := range profiles {
		if profile.Key != managed.Key || profile.Provider != managed.Provider {
			continue
		}
		if found {
			t.Fatal("provider-live managed profile is duplicated")
		}
		profiles[index] = providerLiveExecutionProfile(profile, harness)
		found = true
	}
	if !found {
		t.Fatal("provider-live managed profile is absent")
	}
	return profiles
}

func TestInstalledProviderHooksProjectTheApprovedPlatformSample(t *testing.T) {
	preflight := requireProviderLivePreflight(t)
	harness := newProviderLiveHarness(t, preflight.home)
	socket := randomTmuxSocketName(t, "skid-provider-live")
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      preflight.host.Tmux.Path,
		SocketName:    socket,
		Home:          preflight.home,
		CataloguePath: preflight.cataloguePath,
		Profiles: providerLiveManagedProfiles(
			t,
			preflight.host.Profiles,
			preflight.plan.managedProfile,
			harness,
		),
	})
	if err != nil {
		t.Fatal("construct installed provider-live session manager")
	}

	socketPath := namedTmuxSocketPath(socket)
	output, err := isolatedTmuxCommand(
		tmuxPath,
		"-L", socket,
		"-f", "/dev/null",
		"new-session", "-d", "-s", "provider-live-bootstrap",
		"-c", harness.workspace,
		"--", sleepPath, "300",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("start provider-live isolated tmux server: output_bytes=%d", len(output))
	}
	serverIdentity := captureTestTmuxServer(t, tmuxPath, socketPath)
	serverStopped := false
	cleanupComplete := false
	lifetimes := providerLiveLifetimeTracker{}
	providerTmuxNames := []string{preflight.plan.managedTmuxName, preflight.plan.laptopTmuxName}
	t.Cleanup(func() {
		if cleanupComplete {
			return
		}
		if !serverStopped {
			if cleanupErr := captureProviderLiveForegroundLifetimes(
				tmuxPath,
				socketPath,
				providerTmuxNames,
				&lifetimes,
			); cleanupErr != nil {
				t.Error(cleanupErr)
			}
			if cleanupErr := killVerifiedTestTmuxServer(tmuxPath, socketPath, serverIdentity); cleanupErr != nil {
				t.Errorf("stop exact provider-live tmux server: %v", cleanupErr)
				return
			}
			serverStopped = true
		}
		if cleanupErr := waitForProviderLiveLifetimesToEnd(lifetimes.forCleanup()); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		harness.dispatchSentinel.assertNoDispatch(t)
	})

	operationContext, cancel := context.WithTimeout(context.Background(), providerLiveOperationTimeout)
	_, err = manager.Create(operationContext, sessions.CreateInput{
		CWD:              harness.workspace,
		Profile:          string(preflight.plan.managedProfile.Key),
		OptionalTmuxName: preflight.plan.managedTmuxName,
	})
	cancel()
	if err != nil {
		t.Fatal("launch managed provider through the production session boundary")
	}
	launchProviderLiveLaptop(t, socket, harness.workspace, preflight.plan, harness)

	managedSessionName := ""
	if preflight.plan.managedProfile.Provider == agentruntime.ProviderClaude {
		managedSessionName = preflight.plan.managedTmuxName
	}
	expectations := []providerLiveExpectation{
		{
			tmuxName:            preflight.plan.managedTmuxName,
			provider:            preflight.plan.managedProfile.Provider,
			runtimeProfile:      preflight.plan.managedProfile.Key,
			launchProfile:       preflight.plan.managedProfile.Key,
			providerSessionName: managedSessionName,
		},
		{
			tmuxName:            preflight.plan.laptopTmuxName,
			provider:            preflight.plan.laptopProfile.Provider,
			runtimeProfile:      preflight.plan.laptopProfile.Key,
			providerSessionName: preflight.plan.laptopSessionName,
		},
	}
	waitForProviderLiveProjections(t, manager, expectations, &lifetimes)
	captureErr := captureProviderLiveForegroundLifetimes(
		tmuxPath,
		socketPath,
		providerTmuxNames,
		&lifetimes,
	)
	if captureErr != nil {
		t.Error(captureErr)
	}

	if err := killVerifiedTestTmuxServer(tmuxPath, socketPath, serverIdentity); err != nil {
		t.Fatal("stop exact provider-live tmux server")
	}
	serverStopped = true
	cleanupErr := waitForProviderLiveLifetimesToEnd(lifetimes.forCleanup())
	cleanupComplete = true
	if cleanupErr != nil {
		t.Fatal(cleanupErr)
	}
	harness.dispatchSentinel.assertNoDispatch(t)
}

func requireProviderLivePreflight(t *testing.T) providerLivePreflight {
	t.Helper()
	if os.Getenv("SKIDBLADNIR_ALLOW_PROVIDER_LIVE_TESTS") != providerLiveEnvironmentCapability ||
		*providerLiveCLICapability != providerLiveCLICapabilityV1 {
		t.Fatal("provider-live proof requires the exact environment and CLI capabilities")
	}
	if !validProviderLiveTag(*providerLiveReleaseTag) || !providerLiveSHAPattern.MatchString(*providerLiveSourceSHA) {
		t.Fatal("provider-live proof requires one canonical release tag and exact source SHA")
	}

	sourceRoot := repositoryRoot(t)
	requireProviderLiveSource(t, sourceRoot, *providerLiveReleaseTag, *providerLiveSourceSHA)
	account, err := user.Current()
	if err != nil {
		t.Fatal("resolve provider-live host account")
	}
	accountUID, err := strconv.Atoi(account.Uid)
	if err != nil || accountUID != os.Geteuid() || !filepath.IsAbs(account.HomeDir) ||
		filepath.Clean(account.HomeDir) != account.HomeDir || os.Getenv("HOME") != account.HomeDir {
		t.Fatal("provider-live proof requires the exact current host account and home")
	}

	candidate := filepath.Join(account.HomeDir, ".local", "bin", "skidbladnir")
	configPath := filepath.Join(account.HomeDir, ".config", "skidbladnir", "host-config.json")
	cataloguePath := filepath.Join(account.HomeDir, ".local", "share", "skidbladnir", "characters.json")
	requireProviderLiveOwnedFile(t, candidate, 0o755, accountUID)
	requireProviderLiveOwnedFile(t, configPath, 0o600, accountUID)
	requireProviderLiveOwnedFile(t, cataloguePath, 0o644, accountUID)
	requireProviderLiveCandidateVersion(t, candidate, *providerLiveReleaseTag, *providerLiveSourceSHA)

	host, err := hostconfig.Load(configPath, platform.Current().Kind)
	if err != nil {
		t.Fatal("load strict installed provider-live host configuration")
	}
	if host.Tmux.Path != tmuxPath {
		t.Fatal("installed host configuration does not name the approved tmux binary")
	}
	requireProviderLiveExecutable(t, host.Tmux.Path)
	plan := providerLivePlanForHost(t, host.Profiles)
	requireProviderLiveExecutable(t, plan.managedProfile.Command)
	requireProviderLiveExecutable(t, plan.laptopProfile.Command)

	return providerLivePreflight{
		repositoryRoot: sourceRoot,
		home:           account.HomeDir,
		cataloguePath:  cataloguePath,
		host:           host,
		plan:           plan,
	}
}

func requireProviderLiveSource(t *testing.T, root, tag, sha string) {
	t.Helper()
	if got := providerLiveGit(t, root, "rev-parse", "--show-toplevel"); got != root+"\n" {
		t.Fatal("provider-live checkout is not the exact repository root")
	}
	if got := providerLiveGit(t, root, "rev-parse", "HEAD"); got != sha+"\n" {
		t.Fatal("provider-live checkout HEAD does not match the declared source SHA")
	}
	if got := providerLiveGit(t, root, "rev-parse", "--verify", "refs/tags/"+tag+"^{commit}"); got != sha+"\n" {
		t.Fatal("provider-live release tag does not resolve to the declared source SHA")
	}
	if got := providerLiveGit(t, root, "status", "--porcelain=v1", "--untracked-files=all"); got != "" {
		t.Fatal("provider-live checkout must be clean at the declared source SHA")
	}
}

func providerLiveGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Env = withoutEnvironment(os.Environ(), "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil || stderr.Len() != 0 {
		t.Fatalf("provider-live source command failed: argument_count=%d", len(arguments))
	}
	return stdout.String()
}

func validProviderLiveTag(tag string) bool {
	components := providerLiveTagPattern.FindStringSubmatch(tag)
	if len(components) != 4 {
		return false
	}
	limits := [...]int{2100, 999, 999}
	lengths := [...]int{4, 3, 3}
	values := [3]int{}
	for index := range values {
		if len(components[index+1]) > lengths[index] {
			return false
		}
		value, err := strconv.Atoi(components[index+1])
		if err != nil || value > limits[index] {
			return false
		}
		values[index] = value
	}
	versionCode := values[0]*1_000_000 + values[1]*1_000 + values[2]
	return versionCode > 1 && versionCode <= 2_100_000_000
}

func requireProviderLiveOwnedFile(t *testing.T, path string, mode os.FileMode, owner int) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode ||
		info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		t.Fatal("installed provider-live owned file has the wrong type or mode")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != owner {
		t.Fatal("installed provider-live owned file has the wrong owner")
	}
}

func requireProviderLiveCandidateVersion(t *testing.T, candidate, tag, sha string) {
	t.Helper()
	command := exec.Command(candidate, "version")
	command.Env = withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "TMUX_TMPDIR")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil || stderr.Len() != 0 || stdout.String() != tag+" "+sha+"\n" {
		t.Fatal("installed provider-live candidate does not report the exact release tag and source SHA")
	}
}

func requireProviderLiveExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || syscall.Access(path, 1) != nil {
		t.Fatal("provider-live selected command is not an executable regular file")
	}
}

func providerLivePlanForHost(t *testing.T, profiles []agentruntime.Profile) providerLivePlan {
	t.Helper()
	switch platform.Current().Kind {
	case platform.KindLinux:
		return providerLivePlan{
			managedProfile:    requireProviderLiveProfile(t, profiles, "personal", agentruntime.ProviderCodex),
			managedTmuxName:   "provider-live-managed-codex",
			laptopProfile:     requireProviderLiveProfile(t, profiles, "claude-personal", agentruntime.ProviderClaude),
			laptopTmuxName:    "provider-live-laptop-claude",
			laptopSessionName: "provider-live-explicit-claude",
		}
	case platform.KindDarwin:
		return providerLivePlan{
			managedProfile:  requireProviderLiveProfile(t, profiles, "claude-personal", agentruntime.ProviderClaude),
			managedTmuxName: "provider-live-managed-claude",
			laptopProfile:   requireProviderLiveProfile(t, profiles, "personal", agentruntime.ProviderCodex),
			laptopTmuxName:  "provider-live-laptop-codex",
		}
	default:
		t.Fatal("provider-live proof supports only the declared host platforms")
		return providerLivePlan{}
	}
}

func requireProviderLiveProfile(
	t *testing.T,
	profiles []agentruntime.Profile,
	key agentruntime.ProfileKey,
	provider agentruntime.Provider,
) agentruntime.Profile {
	t.Helper()
	for _, profile := range profiles {
		if profile.Key == key && profile.Provider == provider {
			return profile
		}
	}
	t.Fatal("installed host configuration omits a provider-live sample profile")
	return agentruntime.Profile{}
}

func launchProviderLiveLaptop(
	t *testing.T,
	socket string,
	cwd string,
	plan providerLivePlan,
	harness providerLiveHarness,
) {
	t.Helper()
	profile := providerLiveExecutionProfile(plan.laptopProfile, harness)
	arguments := []string{"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", plan.laptopTmuxName, "-c", cwd}
	for _, variable := range profile.Environment {
		arguments = append(arguments, "-e", variable.Name+"="+variable.Value)
	}
	arguments = append(arguments, "--", profile.Command)
	providerSessionName := ""
	switch profile.Provider {
	case agentruntime.ProviderCodex:
		if plan.laptopSessionName != "" {
			t.Fatal("Codex laptop sample cannot declare a provider session name")
		}
	case agentruntime.ProviderClaude:
		if plan.laptopSessionName == "" {
			t.Fatal("Claude laptop sample requires one explicit safe name")
		}
		providerSessionName = plan.laptopSessionName
	default:
		t.Fatal("provider-live laptop sample has an unsupported provider")
	}
	arguments = append(arguments, agentruntime.LaunchArguments(profile, providerSessionName)...)
	output, err := isolatedTmuxCommand(tmuxPath, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("launch laptop provider on isolated tmux: output_bytes=%d", len(output))
	}
}

// The provider hooks and native process observation converge asynchronously.
// Polling reads only the supported Manager.List boundary and never sends input.
func waitForProviderLiveProjections(
	t *testing.T,
	manager *sessions.Manager,
	expectations []providerLiveExpectation,
	lifetimes *providerLiveLifetimeTracker,
) {
	t.Helper()
	if len(expectations) != 2 {
		t.Fatal("provider-live sample requires exactly one managed and one laptop expectation")
	}
	deadline := time.Now().Add(providerLiveConvergenceTimeout)
	last := make([]providerLiveProjectionSummary, len(expectations))
	listOK := false
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf(
				"provider-live projection did not converge: list_ok=%t managed={session:%t agent:%t provider:%t pid:%t runtime_profile:%t provider_session:%t session_id:%t session_name:%t launch_profile:%t} laptop={session:%t agent:%t provider:%t pid:%t runtime_profile:%t provider_session:%t session_id:%t session_name:%t launch_profile:%t}",
				listOK,
				last[0].sessionPresent, last[0].agentPresent, last[0].providerMatches, last[0].pidPresent,
				last[0].runtimeProfileMatches, last[0].providerSessionPresent, last[0].providerSessionID,
				last[0].providerSessionName, last[0].launchProfileMatches,
				last[1].sessionPresent, last[1].agentPresent, last[1].providerMatches, last[1].pidPresent,
				last[1].runtimeProfileMatches, last[1].providerSessionPresent, last[1].providerSessionID,
				last[1].providerSessionName, last[1].launchProfileMatches,
			)
		}
		pollTimeout := providerLiveOperationTimeout
		if remaining < pollTimeout {
			pollTimeout = remaining
		}
		ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
		listed, err := manager.List(ctx)
		cancel()
		if err == nil {
			listOK = true
			if err := lifetimes.record(listed); err != nil {
				t.Fatal(err)
			}
			complete := true
			for index, expectation := range expectations {
				last[index] = summarizeProviderLiveProjection(listed, expectation)
				complete = complete && providerLiveProjectionComplete(last[index])
			}
			if complete {
				return
			}
		} else {
			listOK = false
			last = make([]providerLiveProjectionSummary, len(expectations))
		}
		time.Sleep(providerLivePollInterval)
	}
}

func summarizeProviderLiveProjection(
	listed []sessions.Session,
	expectation providerLiveExpectation,
) providerLiveProjectionSummary {
	for _, session := range listed {
		if session.TmuxName != expectation.tmuxName {
			continue
		}
		summary := providerLiveProjectionSummary{
			sessionPresent:       true,
			launchProfileMatches: session.LaunchProfile == expectation.launchProfile,
		}
		if session.Agent == nil {
			return summary
		}
		summary.agentPresent = true
		summary.providerMatches = session.Agent.Provider == expectation.provider
		summary.pidPresent = session.Agent.PID > 0
		summary.runtimeProfileMatches = session.Agent.Profile == expectation.runtimeProfile
		if session.Agent.ProviderSession != nil {
			summary.providerSessionPresent = true
			summary.providerSessionID = session.Agent.ProviderSession.ID() != ""
			summary.providerSessionName = session.Agent.ProviderSession.Name() == expectation.providerSessionName
		}
		return summary
	}
	return providerLiveProjectionSummary{}
}

func providerLiveProjectionComplete(summary providerLiveProjectionSummary) bool {
	return summary.sessionPresent && summary.agentPresent && summary.providerMatches && summary.pidPresent &&
		summary.runtimeProfileMatches && summary.providerSessionPresent && summary.providerSessionID &&
		summary.providerSessionName && summary.launchProfileMatches
}

func captureProviderLiveForegroundLifetimes(
	tmuxPath string,
	socketPath string,
	providerTmuxNames []string,
	lifetimes *providerLiveLifetimeTracker,
) error {
	if len(providerTmuxNames) != 2 || providerTmuxNames[0] == providerTmuxNames[1] {
		return errors.New("final provider runtime foreground capture failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerLiveOperationTimeout)
	output, commandErr := isolatedTmuxCommandContext(
		ctx,
		tmuxPath,
		"-S", socketPath,
		"list-sessions", "-F", "#{session_name}|#{pane_pid}",
	).CombinedOutput()
	cancel()
	panePIDs, scanErr := scanProviderLivePanePIDs(output, providerTmuxNames)
	failed := commandErr != nil || scanErr != nil
	for _, panePID := range panePIDs {
		paneStart := processStartIdentity(int(panePID))
		if paneStart == "" {
			failed = true
		} else {
			lifetimes.retain(providerLiveLifetime{pid: panePID, start: paneStart})
		}
		foreground, observeErr := processinfo.ObserveForeground(panePID)
		if observeErr != nil || foreground.PID <= 0 || foreground.StartIdentity == "" {
			failed = true
			continue
		}
		lifetimes.retain(providerLiveLifetime{pid: foreground.PID, start: foreground.StartIdentity})
	}
	if failed {
		return errors.New("final provider runtime foreground capture failed")
	}
	return nil
}

func waitForProviderLiveLifetimesToEnd(lifetimes []providerLiveLifetime) error {
	deadline := time.Now().Add(providerLiveCleanupTimeout)
	for {
		remaining := 0
		for _, lifetime := range lifetimes {
			if processStartIdentity(int(lifetime.pid)) == lifetime.start {
				remaining++
			}
		}
		if remaining == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("provider runtime lifetime survived exact tmux server cleanup")
		}
		time.Sleep(providerLivePollInterval)
	}
}
