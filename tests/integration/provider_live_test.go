//go:build integration && providerlive

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
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

func TestInstalledProviderHooksProjectTheApprovedPlatformSample(t *testing.T) {
	preflight := requireProviderLivePreflight(t)
	socket := randomTmuxSocketName(t, "skid-provider-live")
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      preflight.host.Tmux.Path,
		SocketName:    socket,
		Home:          preflight.home,
		CataloguePath: preflight.cataloguePath,
		Profiles:      preflight.host.Profiles,
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
		"-c", preflight.repositoryRoot,
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
	})

	operationContext, cancel := context.WithTimeout(context.Background(), providerLiveOperationTimeout)
	_, err = manager.Create(operationContext, sessions.CreateInput{
		CWD:              preflight.repositoryRoot,
		Profile:          string(preflight.plan.managedProfile.Key),
		OptionalTmuxName: preflight.plan.managedTmuxName,
	})
	cancel()
	if err != nil {
		t.Fatal("launch managed provider through the production session boundary")
	}
	launchProviderLiveLaptop(t, socket, preflight.repositoryRoot, preflight.plan)

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

func launchProviderLiveLaptop(t *testing.T, socket, cwd string, plan providerLivePlan) {
	t.Helper()
	arguments := []string{"-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", plan.laptopTmuxName, "-c", cwd}
	for _, variable := range plan.laptopProfile.Environment {
		arguments = append(arguments, "-e", variable.Name+"="+variable.Value)
	}
	arguments = append(arguments, "--", plan.laptopProfile.Command)
	switch plan.laptopProfile.Provider {
	case agentruntime.ProviderCodex:
		if plan.laptopSessionName != "" {
			t.Fatal("Codex laptop sample cannot declare a provider session name")
		}
	case agentruntime.ProviderClaude:
		if plan.laptopSessionName == "" {
			t.Fatal("Claude laptop sample requires one explicit safe name")
		}
		arguments = append(arguments, "--name", plan.laptopSessionName)
	default:
		t.Fatal("provider-live laptop sample has an unsupported provider")
	}
	arguments = append(arguments, plan.laptopProfile.Arguments...)
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
