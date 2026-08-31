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

const (
	providerLiveProcessGroupCleanupTimeout = 10 * time.Second
	providerLiveProcessGroupPollInterval   = 250 * time.Millisecond
)

type providerLiveProcessGroup struct {
	id          processinfo.PID
	leaderStart processinfo.StartIdentity
}

type providerLiveProcessGroupTracker struct {
	retained []providerLiveProcessGroup
}

func (tracker *providerLiveProcessGroupTracker) record(listed []sessions.Session) error {
	failed := false
	for _, session := range listed {
		if session.Agent == nil {
			continue
		}
		observation, err := processinfo.Observe(session.Agent.PID)
		if err != nil {
			failed = true
			continue
		}
		if err := tracker.retainObservation(observation); err != nil {
			failed = true
		}
	}
	if failed {
		return errors.New("provider runtime process-group capture failed")
	}
	return nil
}

func (tracker *providerLiveProcessGroupTracker) retainObservation(observation processinfo.Observation) error {
	group, err := providerLiveProcessGroupFromObservation(observation)
	if err != nil {
		return err
	}
	tracker.retain(group)
	return nil
}

func providerLiveProcessGroupFromObservation(observation processinfo.Observation) (providerLiveProcessGroup, error) {
	if observation.PID <= 0 || observation.ProcessGroup <= 1 || observation.SessionID <= 0 {
		return providerLiveProcessGroup{}, errors.New("provider runtime process group is invalid")
	}
	leader, err := processinfo.Observe(observation.ProcessGroup)
	if err != nil || leader.PID != observation.ProcessGroup ||
		leader.ProcessGroup != observation.ProcessGroup || leader.SessionID != observation.SessionID ||
		leader.StartIdentity == "" {
		return providerLiveProcessGroup{}, errors.New("provider runtime process-group leader is invalid")
	}
	return providerLiveProcessGroup{id: leader.PID, leaderStart: leader.StartIdentity}, nil
}

func (tracker *providerLiveProcessGroupTracker) retain(group providerLiveProcessGroup) {
	for _, retained := range tracker.retained {
		if retained == group {
			return
		}
	}
	tracker.retained = append(tracker.retained, group)
}

func (tracker *providerLiveProcessGroupTracker) forCleanup() []providerLiveProcessGroup {
	return append([]providerLiveProcessGroup(nil), tracker.retained...)
}

func terminateProviderLiveProcessGroups(groups []providerLiveProcessGroup) error {
	for _, group := range groups {
		if err := validateProviderLiveProcessGroupForSignal(group); err != nil {
			return err
		}
	}

	stopped := make([]providerLiveProcessGroup, 0, len(groups))
	for _, group := range groups {
		if err := syscall.Kill(-int(group.id), syscall.SIGSTOP); err != nil {
			return errors.Join(
				errors.New("stop exact provider-live process group"),
				killStoppedProviderLiveProcessGroups(stopped),
			)
		}
		stopped = append(stopped, group)
	}
	for _, group := range stopped {
		if err := validateProviderLiveProcessGroupForSignal(group); err != nil {
			return errors.Join(err, killStoppedProviderLiveProcessGroups(stopped))
		}
	}
	return killStoppedProviderLiveProcessGroups(stopped)
}

func validateProviderLiveProcessGroupForSignal(group providerLiveProcessGroup) error {
	if group.id <= 1 || group.id == processinfo.PID(syscall.Getpgrp()) || group.leaderStart == "" {
		return errors.New("refuse invalid provider-live process-group identity")
	}
	leader, err := processinfo.Observe(group.id)
	if err != nil || leader.PID != group.id || leader.ProcessGroup != group.id ||
		leader.StartIdentity != group.leaderStart {
		return errors.New("provider-live process-group identity changed before signal")
	}
	return nil
}

func killStoppedProviderLiveProcessGroups(groups []providerLiveProcessGroup) error {
	var result error
	for _, group := range groups {
		if err := validateProviderLiveProcessGroupForSignal(group); err != nil {
			result = errors.Join(result, err)
			continue
		}
		if err := syscall.Kill(-int(group.id), syscall.SIGKILL); err != nil {
			result = errors.Join(result, errors.New("kill stopped provider-live process group"))
		}
	}
	return result
}

func waitForProviderLiveProcessGroupsToEnd(groups []providerLiveProcessGroup) error {
	deadline := time.Now().Add(providerLiveProcessGroupCleanupTimeout)
	for {
		remaining := 0
		for _, group := range groups {
			if err := syscall.Kill(-int(group.id), 0); err == nil || !errors.Is(err, syscall.ESRCH) {
				remaining++
			}
		}
		if remaining == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("provider runtime process group survived exact tmux server cleanup")
		}
		time.Sleep(providerLiveProcessGroupPollInterval)
	}
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

func TestProviderLiveProcessGroupTrackerRetainsEveryValidObservationForCleanup(t *testing.T) {
	pid := processinfo.PID(os.Getpid())
	observation, err := processinfo.Observe(pid)
	if err != nil || observation.PID != pid || observation.StartIdentity == "" {
		t.Fatal("observe the test process lifetime")
	}
	wantObserved, err := providerLiveProcessGroupFromObservation(observation)
	if err != nil {
		t.Fatal("resolve the test process group")
	}
	tracker := providerLiveProcessGroupTracker{}
	absent := sessions.Session{Agent: &agentruntime.AgentRuntime{
		Provider: agentruntime.ProviderCodex,
		PID:      processinfo.PID(1 << 30),
	}}
	observed := sessions.Session{Agent: &agentruntime.AgentRuntime{
		Provider: agentruntime.ProviderCodex,
		PID:      pid,
	}}
	if err := tracker.record([]sessions.Session{absent, observed}); err == nil {
		t.Fatal("partial provider observation did not fail closed")
	}
	partial := tracker.forCleanup()
	if len(partial) != 1 || partial[0] != wantObserved {
		t.Fatal("an absent earlier observation suppressed a later valid cleanup process group")
	}
	if err := tracker.record([]sessions.Session{observed}); err != nil {
		t.Fatal("record overlapping provider observation")
	}
	if deduplicated := tracker.forCleanup(); len(deduplicated) != 1 || deduplicated[0] != wantObserved {
		t.Fatal("an overlapping observation duplicated its cleanup process group")
	}

	replacementStart := processinfo.StartIdentity("1")
	if replacementStart == wantObserved.leaderStart {
		replacementStart = "2"
	}
	tracker.retain(providerLiveProcessGroup{id: wantObserved.id, leaderStart: replacementStart})
	want := []providerLiveProcessGroup{
		wantObserved,
		{id: wantObserved.id, leaderStart: replacementStart},
	}
	got := tracker.forCleanup()
	if len(got) != len(want) {
		t.Fatalf("cleanup process-group count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("cleanup process group %d did not preserve its exact observed identity", index)
		}
	}
}

func TestTerminateProviderLiveProcessGroupsHardStopsExactOwnedGroup(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal("resolve provider-live process-group test command")
	}
	command := exec.Command(shell, "-c", "sleep 30 & wait")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal("start provider-live process-group test command")
	}
	waited := false
	t.Cleanup(func() {
		if waited {
			return
		}
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
	})
	pid := processinfo.PID(command.Process.Pid)
	var observation processinfo.Observation
	deadline := time.Now().Add(time.Second)
	// justify-polling: Start returns before the child necessarily completes
	// exec and process-group creation, while identity is available only after
	// those boundaries.
	for time.Now().Before(deadline) {
		observation, err = processinfo.Observe(pid)
		if err == nil && observation.StartIdentity != "" && observation.ProcessGroup == pid {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || observation.StartIdentity == "" || observation.ProcessGroup != pid {
		t.Fatal("observe provider-live process-group test command")
	}
	group, err := providerLiveProcessGroupFromObservation(observation)
	if err != nil {
		t.Fatal("resolve exact provider-live process group")
	}
	if err := terminateProviderLiveProcessGroups([]providerLiveProcessGroup{group}); err != nil {
		t.Fatal("terminate exact provider-live process group")
	}
	waitErr := command.Wait()
	waited = true
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		t.Fatal("provider-live lifetime did not end by signal")
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("provider-live process-group leader did not end by SIGKILL")
	}
	if err := waitForProviderLiveProcessGroupsToEnd([]providerLiveProcessGroup{group}); err != nil {
		t.Fatal("provider-live process-group descendant survived exact hard stop")
	}
	if _, err := processinfo.Observe(pid); err == nil {
		t.Fatal("provider-live process-group leader survived exact hard stop")
	}
}

func TestTerminateProviderLiveProcessGroupsRefusesTheCallerGroup(t *testing.T) {
	pid := processinfo.PID(os.Getpid())
	observation, err := processinfo.Observe(pid)
	if err != nil || observation.StartIdentity == "" {
		t.Fatal("observe provider-live lifetime test caller")
	}
	group, err := providerLiveProcessGroupFromObservation(observation)
	if err != nil {
		t.Fatal("resolve provider-live caller process group")
	}
	if err := terminateProviderLiveProcessGroups([]providerLiveProcessGroup{group}); err == nil {
		t.Fatal("provider-live process-group termination accepted its caller group")
	}
	after, err := processinfo.Observe(pid)
	if err != nil || after.StartIdentity != observation.StartIdentity {
		t.Fatal("provider-live process-group refusal changed its caller")
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
