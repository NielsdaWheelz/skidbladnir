package integration_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

type providerLiveLifetime struct {
	pid   processinfo.PID
	start processinfo.StartIdentity
}

type providerLiveLifetimeTracker struct {
	retained []providerLiveLifetime
}

func (tracker *providerLiveLifetimeTracker) record(listed []sessions.Session) error {
	failed := false
	for _, session := range listed {
		if session.Agent == nil {
			continue
		}
		observation, err := processinfo.Observe(session.Agent.PID)
		if err != nil || observation.PID <= 0 || observation.StartIdentity == "" {
			failed = true
			continue
		}
		tracker.retain(providerLiveLifetime{pid: observation.PID, start: observation.StartIdentity})
	}
	if failed {
		return errors.New("provider runtime lifetime capture failed")
	}
	return nil
}

func (tracker *providerLiveLifetimeTracker) retain(lifetime providerLiveLifetime) {
	for _, retained := range tracker.retained {
		if retained == lifetime {
			return
		}
	}
	tracker.retained = append(tracker.retained, lifetime)
}

func (tracker *providerLiveLifetimeTracker) forCleanup() []providerLiveLifetime {
	return append([]providerLiveLifetime(nil), tracker.retained...)
}

func terminateProviderLiveLifetimes(lifetimes []providerLiveLifetime) error {
	for _, lifetime := range lifetimes {
		if err := validateProviderLiveLifetimeForSignal(lifetime); err != nil {
			return err
		}
	}

	stopped := make([]providerLiveLifetime, 0, len(lifetimes))
	for _, lifetime := range lifetimes {
		if err := syscall.Kill(int(lifetime.pid), syscall.SIGSTOP); err != nil {
			return errors.Join(
				errors.New("stop exact provider-live runtime"),
				killStoppedProviderLiveLifetimes(stopped),
			)
		}
		stopped = append(stopped, lifetime)
	}
	for _, lifetime := range stopped {
		if err := validateProviderLiveLifetimeForSignal(lifetime); err != nil {
			return errors.Join(err, killStoppedProviderLiveLifetimes(stopped))
		}
	}
	return killStoppedProviderLiveLifetimes(stopped)
}

func validateProviderLiveLifetimeForSignal(lifetime providerLiveLifetime) error {
	if lifetime.pid <= 1 || lifetime.pid == processinfo.PID(os.Getpid()) || lifetime.start == "" {
		return errors.New("refuse invalid provider-live runtime identity")
	}
	observation, err := processinfo.Observe(lifetime.pid)
	if err != nil || observation.PID != lifetime.pid || observation.StartIdentity != lifetime.start {
		return errors.New("provider-live runtime identity changed before signal")
	}
	return nil
}

func killStoppedProviderLiveLifetimes(lifetimes []providerLiveLifetime) error {
	var result error
	for _, lifetime := range lifetimes {
		if err := validateProviderLiveLifetimeForSignal(lifetime); err != nil {
			result = errors.Join(result, err)
			continue
		}
		if err := syscall.Kill(int(lifetime.pid), syscall.SIGKILL); err != nil {
			result = errors.Join(result, errors.New("kill stopped provider-live runtime"))
		}
	}
	return result
}

func scanProviderLivePanePIDs(output []byte, providerTmuxNames []string) ([]processinfo.PID, error) {
	if len(providerTmuxNames) != 2 || providerTmuxNames[0] == providerTmuxNames[1] {
		return nil, errors.New("final provider runtime foreground capture failed")
	}
	targets := map[string]struct{}{
		providerTmuxNames[0]: {},
		providerTmuxNames[1]: {},
	}
	seen := make(map[string]int, len(targets))
	panePIDs := make([]processinfo.PID, 0, len(targets))
	failed := false
	for _, line := range bytes.Split(bytes.TrimSuffix(output, []byte("\n")), []byte("\n")) {
		fields := bytes.Split(line, []byte("|"))
		if len(fields) != 2 {
			failed = true
			continue
		}
		name := string(fields[0])
		if _, target := targets[name]; !target {
			continue
		}
		seen[name]++
		if seen[name] > 1 {
			failed = true
		}
		panePID, err := strconv.Atoi(string(fields[1]))
		if err != nil || panePID <= 0 {
			failed = true
			continue
		}
		panePIDs = append(panePIDs, processinfo.PID(panePID))
	}
	for target := range targets {
		if seen[target] != 1 {
			failed = true
		}
	}
	if failed {
		return panePIDs, errors.New("final provider runtime foreground capture failed")
	}
	return panePIDs, nil
}

func TestProviderLiveLifetimeTrackerRetainsEveryValidObservationForCleanup(t *testing.T) {
	pid := processinfo.PID(os.Getpid())
	observation, err := processinfo.Observe(pid)
	if err != nil || observation.PID != pid || observation.StartIdentity == "" {
		t.Fatal("observe the test process lifetime")
	}
	tracker := providerLiveLifetimeTracker{}
	invalid := sessions.Session{Agent: &agentruntime.AgentRuntime{PID: -1}}
	observed := sessions.Session{Agent: &agentruntime.AgentRuntime{PID: pid}}
	if err := tracker.record([]sessions.Session{invalid, observed}); err == nil {
		t.Fatal("partial provider observation did not fail closed")
	}
	partial := tracker.forCleanup()
	wantObserved := providerLiveLifetime{pid: pid, start: observation.StartIdentity}
	if len(partial) != 1 || partial[0] != wantObserved {
		t.Fatal("an invalid earlier observation suppressed a later valid cleanup lifetime")
	}
	if err := tracker.record([]sessions.Session{observed}); err != nil {
		t.Fatal("record overlapping provider observation")
	}
	if deduplicated := tracker.forCleanup(); len(deduplicated) != 1 || deduplicated[0] != wantObserved {
		t.Fatal("an overlapping observation duplicated its cleanup lifetime")
	}

	replacementStart := processinfo.StartIdentity("1")
	if replacementStart == observation.StartIdentity {
		replacementStart = "2"
	}
	tracker.retain(providerLiveLifetime{pid: pid, start: replacementStart})
	want := []providerLiveLifetime{
		wantObserved,
		{pid: pid, start: replacementStart},
	}
	got := tracker.forCleanup()
	if len(got) != len(want) {
		t.Fatalf("cleanup lifetime count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("cleanup lifetime %d did not preserve its exact observed identity", index)
		}
	}
}

func TestTerminateProviderLiveLifetimesHardStopsExactOwnedRuntime(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal("resolve provider-live lifetime test command")
	}
	command := exec.Command(sleep, "30")
	if err := command.Start(); err != nil {
		t.Fatal("start provider-live lifetime test command")
	}
	waited := false
	t.Cleanup(func() {
		if waited {
			return
		}
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	pid := processinfo.PID(command.Process.Pid)
	var observation processinfo.Observation
	deadline := time.Now().Add(time.Second)
	// justify-polling: Start returns before the child necessarily completes
	// exec, while process identity is available only after that boundary.
	for time.Now().Before(deadline) {
		observation, err = processinfo.Observe(pid)
		if err == nil && observation.StartIdentity != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || observation.StartIdentity == "" {
		t.Fatal("observe provider-live lifetime test command")
	}
	if err := terminateProviderLiveLifetimes([]providerLiveLifetime{{
		pid:   pid,
		start: observation.StartIdentity,
	}}); err != nil {
		t.Fatal("terminate exact provider-live lifetime")
	}
	waitErr := command.Wait()
	waited = true
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		t.Fatal("provider-live lifetime did not end by signal")
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("provider-live lifetime did not end by SIGKILL")
	}
	if _, err := processinfo.Observe(pid); err == nil {
		t.Fatal("provider-live lifetime survived exact hard stop")
	}
}

func TestTerminateProviderLiveLifetimesRefusesTheCaller(t *testing.T) {
	pid := processinfo.PID(os.Getpid())
	observation, err := processinfo.Observe(pid)
	if err != nil || observation.StartIdentity == "" {
		t.Fatal("observe provider-live lifetime test caller")
	}
	if err := terminateProviderLiveLifetimes([]providerLiveLifetime{{
		pid:   pid,
		start: observation.StartIdentity,
	}}); err == nil {
		t.Fatal("provider-live lifetime termination accepted its caller")
	}
	after, err := processinfo.Observe(pid)
	if err != nil || after.StartIdentity != observation.StartIdentity {
		t.Fatal("provider-live lifetime refusal changed its caller")
	}
}

func TestProviderLivePaneScanReturnsLaterValidCandidateAndFailsClosed(t *testing.T) {
	want := processinfo.PID(os.Getpid())
	for name, output := range map[string][]byte{
		"malformed first target": []byte("managed\nlaptop|" + strconv.Itoa(int(want)) + "\n"),
		"invalid first target":   []byte("managed|invalid\nlaptop|" + strconv.Itoa(int(want)) + "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			panePIDs, err := scanProviderLivePanePIDs(output, []string{"managed", "laptop"})
			if err == nil {
				t.Fatal("invalid provider-live pane inventory did not fail closed")
			}
			if len(panePIDs) != 1 || panePIDs[0] != want {
				t.Fatal("an invalid earlier target suppressed the later valid pane candidate")
			}
		})
	}
}
