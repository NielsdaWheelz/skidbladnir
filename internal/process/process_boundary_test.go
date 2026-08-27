package process

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestObserveAncestryFromTheTestProcessEndsCleanly(t *testing.T) {
	ancestry, err := ObserveAncestry(PID(os.Getpid()), 128)
	if err != nil {
		t.Fatalf("ancestry walk from the test process = %v, want success up to the observation boundary", err)
	}
	if len(ancestry) == 0 || ancestry[0].PID != PID(os.Getpid()) {
		t.Fatalf("ancestry walk did not start at the test process: length=%d ancestry=%+v", len(ancestry), ancestry)
	}
	for index := 1; index < len(ancestry); index++ {
		if got, want := ancestry[index].PID, ancestry[index-1].ParentPID; got != want {
			t.Fatalf("ancestry link %d is not the recorded parent: observed=%d parent_of_previous=%d", index, got, want)
		}
	}
}

func TestObservingAProtectedProcessIsATypedBoundary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a root test process can observe every process")
	}
	_, err := Observe(1)
	if err == nil {
		t.Skip("PID 1 is observable by the test user, so it is not a privilege boundary here")
	}
	if !errors.Is(err, ErrProcessNotPermitted) {
		t.Fatalf("observing PID 1 as an unprivileged user = %v, want ErrProcessNotPermitted", err)
	}
}

func TestObservingAnAbsentProcessIsATypedOutcome(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^$")
	if err := command.Run(); err != nil {
		t.Fatalf("run observation helper process: %v", err)
	}
	if _, err := Observe(PID(command.Process.Pid)); !errors.Is(err, ErrProcessAbsent) {
		t.Fatalf("observing an exited process = %v, want ErrProcessAbsent", err)
	}
}

func TestObservationCoherenceIncludesTerminalSessionIdentity(t *testing.T) {
	left := Observation{PID: 10, SessionID: 10, TerminalDevice: 42}
	right := left
	if !equalObservation(left, right) {
		t.Fatal("identical terminal-session observations were not coherent")
	}
	right.SessionID++
	if equalObservation(left, right) {
		t.Fatal("a changed terminal session was treated as the same observation")
	}
	right = left
	right.TerminalDevice++
	if equalObservation(left, right) {
		t.Fatal("a changed terminal device was treated as the same observation")
	}
}

func TestTerminalDeviceAtRejectsARegularFile(t *testing.T) {
	path := t.TempDir() + "/regular"
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal("create regular-file fixture")
	}
	if _, err := TerminalDeviceAt(path); err == nil {
		t.Fatal("accepted a regular file as a terminal device")
	}
}
