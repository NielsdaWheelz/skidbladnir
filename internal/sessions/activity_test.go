package sessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

func TestWindowActivityTimestampDerivesTheClosedSessionActivity(t *testing.T) {
	const observedSecond int64 = 2_000_000_000
	observedAt := time.Unix(observedSecond, 987_654_321).UTC()
	tests := []struct {
		name     string
		encoded  string
		activity SessionActivity
		invalid  bool
	}{
		{name: "now", encoded: "2000000000", activity: SessionActivityActive},
		{name: "one second inside boundary", encoded: "1999999991", activity: SessionActivityActive},
		{name: "exact inclusive boundary", encoded: "1999999990", activity: SessionActivityActive},
		{name: "one second outside boundary", encoded: "1999999989", activity: SessionActivityQuiet},
		{name: "missing", encoded: "", invalid: true},
		{name: "zero", encoded: "0", invalid: true},
		{name: "negative", encoded: "-1", invalid: true},
		{name: "positive sign", encoded: "+1", invalid: true},
		{name: "leading whitespace", encoded: " 1", invalid: true},
		{name: "trailing whitespace", encoded: "1 ", invalid: true},
		{name: "leading zero", encoded: "01", invalid: true},
		{name: "suffix", encoded: "1s", invalid: true},
		{name: "overflow", encoded: "9223372036854775808", invalid: true},
		{name: "future", encoded: "2000000001", invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			second, parseErr := parseActivitySecond(test.encoded)
			if parseErr != nil {
				if test.invalid {
					return
				}
				t.Fatalf("parse canonical window_activity %q: %v", test.encoded, parseErr)
			}
			activity, deriveErr := deriveActivity(second, observedAt)
			if test.invalid {
				if deriveErr == nil {
					t.Fatalf("invalid window_activity %q derived %q", test.encoded, activity)
				}
				return
			}
			if deriveErr != nil || activity != test.activity {
				t.Fatalf("window_activity %q projected (%q, %v), want %q", test.encoded, activity, deriveErr, test.activity)
			}
		})
	}
}

func TestManagerListRequiresCanonicalWindowActivity(t *testing.T) {
	for _, test := range []struct {
		name          string
		encoded       string
		wantActivity  SessionActivity
		wantListError bool
	}{
		{name: "canonical old timestamp", encoded: "1", wantActivity: SessionActivityQuiet},
		{name: "malformed timestamp on existing target", encoded: "not-a-timestamp", wantListError: true},
		{name: "future timestamp on existing target", encoded: "9223372036854775807", wantListError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := newActivityProjectionManager(t, test.encoded, false)
			inventory, err := manager.List(context.Background())
			if test.wantListError {
				if err == nil {
					t.Fatalf("existing target activity %q produced a successful inventory", test.encoded)
				}
				return
			}
			if err != nil || !ValidProjectionInstant(inventory.ObservedAt) || len(inventory.Sessions) != 1 ||
				inventory.Sessions[0].Activity != test.wantActivity {
				t.Fatalf(
					"public session projection = (clock_valid=%t session_count=%d activity=%q err=%v), want %q",
					ValidProjectionInstant(inventory.ObservedAt),
					len(inventory.Sessions),
					projectedActivity(inventory),
					err,
					test.wantActivity,
				)
			}
		})
	}
}

func TestManagerListCapturesItsClockBeforeOptionalEnrichment(t *testing.T) {
	const (
		optionalBoundaryTimeout      = 3 * time.Second
		optionalBoundaryPollInterval = 5 * time.Millisecond
	)
	harness := newActivityProjectionHarness(t, "1", false, true)
	t.Cleanup(func() {
		if err := os.WriteFile(harness.optionalRelease, []byte("release\n"), 0o600); err != nil {
			t.Errorf("release optional-enrichment boundary during cleanup: %v", err)
		}
	})
	type listResult struct {
		inventory Inventory
		err       error
	}
	result := make(chan listResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		inventory, err := harness.manager.List(ctx)
		result <- listResult{inventory: inventory, err: err}
	}()

	deadline := time.Now().Add(optionalBoundaryTimeout)
	// justify-polling: the external fake executable can expose phase entry only
	// through its marker file; 5 ms keeps this bounded synchronization prompt.
	for {
		if _, err := os.Stat(harness.optionalStarted); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatalf("observe optional-enrichment marker: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("session inventory did not enter optional enrichment")
		}
		time.Sleep(optionalBoundaryPollInterval)
	}
	optionalBoundary := time.Now().UTC()
	if err := os.WriteFile(harness.optionalRelease, []byte("release\n"), 0o600); err != nil {
		t.Fatalf("release optional-enrichment boundary: %v", err)
	}

	select {
	case listed := <-result:
		if listed.err != nil || len(listed.inventory.Sessions) != 1 ||
			listed.inventory.Sessions[0].Activity != SessionActivityQuiet ||
			!listed.inventory.ObservedAt.Before(optionalBoundary) {
			t.Fatalf(
				"optional enrichment moved the activity clock: observed_at=%s optional_boundary=%s activity=%q err=%v",
				listed.inventory.ObservedAt,
				optionalBoundary,
				projectedActivity(listed.inventory),
				listed.err,
			)
		}
	case <-time.After(optionalBoundaryTimeout):
		t.Fatal("session inventory did not return after optional enrichment was released")
	}
}

func TestWindowActivityFailureReconcilesOnlyAVanishedTarget(t *testing.T) {
	for _, test := range []struct {
		name    string
		encoded string
	}{
		{name: "malformed required snapshot", encoded: "not-a-timestamp"},
		{name: "future required snapshot", encoded: "9223372036854775807"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := newActivityProjectionManager(t, test.encoded, true)
			inventory, err := manager.List(context.Background())
			if err != nil || !ValidProjectionInstant(inventory.ObservedAt) || len(inventory.Sessions) != 0 {
				t.Fatalf(
					"vanished target reconciliation = (clock_valid=%t session_count=%d err=%v), want successful empty inventory",
					ValidProjectionInstant(inventory.ObservedAt),
					len(inventory.Sessions),
					err,
				)
			}
		})
	}
}

func projectedActivity(inventory Inventory) SessionActivity {
	if len(inventory.Sessions) != 1 {
		return ""
	}
	return inventory.Sessions[0].Activity
}

type activityProjectionHarness struct {
	manager         *Manager
	optionalStarted string
	optionalRelease string
}

func newActivityProjectionManager(
	t *testing.T,
	encodedActivity string,
	vanishAfterAnchor bool,
) *Manager {
	t.Helper()
	return newActivityProjectionHarness(t, encodedActivity, vanishAfterAnchor, false).manager
}

func newActivityProjectionHarness(
	t *testing.T,
	encodedActivity string,
	vanishAfterAnchor bool,
	blockOptional bool,
) activityProjectionHarness {
	t.Helper()
	root := t.TempDir()
	vanishMarker := filepath.Join(root, "anchor-read")
	optionalStarted := filepath.Join(root, "optional-started")
	optionalRelease := filepath.Join(root, "optional-release")
	mode := "existing"
	if vanishAfterAnchor {
		mode = "vanish"
	}
	optionalMode := "open"
	paneID := "not-a-pane"
	panePID := "0"
	if blockOptional {
		optionalMode = "block"
		paneID = "%1"
		panePID = "999999999"
	}
	const (
		serverEpoch = "v1-0123456789abcdef0123456789abcdef"
		sessionID   = "$1"
	)
	script := fmt.Sprintf(`#!/bin/sh
set -eu
activity=%s
mode=%s
vanish_marker=%s
optional_mode=%s
optional_started=%s
optional_release=%s
pane_id=%s
pane_pid=%s
cwd=%s
if [ "${1-}" = "-L" ]; then
  shift 4
fi
command=${1-}
if [ "$#" -gt 0 ]; then
  shift
fi
if [ "$command" = "list-sessions" ]; then
  if [ "$mode" = "vanish" ] && [ -e "$vanish_marker" ]; then
    exit 0
  fi
  if [ "$*" = '-F #{session_id}' ]; then
    printf '%%s\n' %s
    exit 0
  fi
  printf '%%s\n' %s
  exit 0
fi
if [ "$command" = "show-options" ]; then
  case "$*" in
    *'@skid_server_epoch') printf '%%s\n' %s ;;
    *'@skid_character') printf '%%s\n' 'norse.fixture' ;;
    *'@skid_internal') ;;
    *) exit 64 ;;
  esac
  exit 0
fi
if [ "$command" = "display-message" ]; then
  case "$*" in
    *'#{@skid_server_epoch}|#{pid}|#{start_time}') printf '%%s\n' %s ;;
    *'#{session_id}|#{session_name}') printf '%%s\n' %s ;;
    *'#{window_activity}'*)
      if [ "$mode" = "vanish" ]; then
		: > "$vanish_marker"
      fi
	  printf '%%s|%%s|%%s|%%s|0|0\n' %s "$pane_id" "$pane_pid" "$activity"
	  ;;
	*'#{pane_current_path}')
	  if [ "$optional_mode" = "block" ]; then
		: > "$optional_started"
		while [ ! -e "$optional_release" ]; do
		  /bin/sleep 0.01
		done
	  fi
	  printf '%%s\n' "$cwd"
	  ;;
	*'#{pane_current_command}')
	  printf '%%s\n' 'sh'
      ;;
    *) exit 64 ;;
  esac
  exit 0
fi
exit 64
`,
		testShellLiteral(encodedActivity),
		testShellLiteral(mode),
		testShellLiteral(vanishMarker),
		testShellLiteral(optionalMode),
		testShellLiteral(optionalStarted),
		testShellLiteral(optionalRelease),
		testShellLiteral(paneID),
		testShellLiteral(panePID),
		testShellLiteral(root),
		testShellLiteral(sessionID),
		testShellLiteral(sessionID+"|activity-fixture||0|1|"+serverEpoch+"|1234|1720000000"),
		testShellLiteral(serverEpoch),
		testShellLiteral(serverEpoch+"|1234|1720000000"),
		testShellLiteral(sessionID+"|activity-fixture"),
		testShellLiteral(sessionID),
	)
	tmuxPath := filepath.Join(root, "tmux-fixture")
	if err := os.WriteFile(tmuxPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write read-only tmux fixture: %v", err)
	}
	cataloguePath := filepath.Join(root, "characters.json")
	if err := os.WriteFile(cataloguePath, []byte(`[{"key":"norse.fixture","displayName":"Fixture"}]`), 0o600); err != nil {
		t.Fatalf("write activity character catalogue: %v", err)
	}
	workingDirectories, err := workdir.New(root)
	if err != nil {
		t.Fatalf("construct activity working-directory service: %v", err)
	}
	manager, err := New(Config{
		TmuxPath: tmuxPath, SocketName: "activity-unit", Workdir: workingDirectories, CataloguePath: cataloguePath,
		Profiles: []agentruntime.Profile{{
			Key: "unit", Label: "Unit", Provider: agentruntime.ProviderCodex, Command: "/bin/false",
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: filepath.Join(root, "codex-home")}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "activity-unit-agent"}},
		}},
	})
	if err != nil {
		t.Fatalf("construct activity projection manager: %v", err)
	}
	return activityProjectionHarness{
		manager: manager, optionalStarted: optionalStarted, optionalRelease: optionalRelease,
	}
}

func testShellLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestAgentIdentityProjectionRemainsOptionalAndProcessBound(t *testing.T) {
	profiles := []agentruntime.Profile{{
		Key:      "work",
		Label:    "Codex · Work",
		Provider: agentruntime.ProviderCodex,
		ForegroundSignatures: []agentruntime.ForegroundSignature{{
			ExecutableBase: "codex",
		}},
	}}
	observed := processinfo.Observation{
		PID:           4312,
		StartIdentity: "991827",
		Executable:    "/opt/skid/bin/codex",
	}
	registration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider:      agentruntime.ProviderCodex,
		PID:           observed.PID,
		StartIdentity: observed.StartIdentity,
	}, "work", "thr_123")
	if err != nil {
		t.Fatalf("encode exact runtime registration: %v", err)
	}

	agent := deriveAgent(profiles, observed, registration)
	if agent == nil || agent.Provider != agentruntime.ProviderCodex || agent.PID != observed.PID ||
		agent.Profile != "work" || agent.ProviderSession == nil || agent.ProviderSession.ID() != "thr_123" {
		t.Fatalf("exact process-bound agent identity = %+v", agent)
	}

	stale, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider: agentruntime.ProviderCodex, PID: observed.PID, StartIdentity: "991826",
	}, "work", "thr_stale")
	if err != nil {
		t.Fatalf("encode stale runtime registration: %v", err)
	}
	agent = deriveAgent(profiles, observed, stale)
	if agent == nil || agent.PID != observed.PID || agent.Profile != "" || agent.ProviderSession != nil {
		t.Fatalf("stale registration crossed into current agent identity: %+v", agent)
	}

	if agent := deriveAgent(profiles, processinfo.Observation{
		PID: 4313, StartIdentity: "991828", Executable: "/bin/zsh",
	}, registration); agent != nil {
		t.Fatalf("unsupported foreground projected an agent: %+v", agent)
	}
	if agent := deriveAgent(profiles, processinfo.Observation{}, registration); agent != nil {
		t.Fatalf("failed process observation projected an agent: %+v", agent)
	}
}
