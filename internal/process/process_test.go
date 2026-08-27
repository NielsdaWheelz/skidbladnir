//go:build integration

package process

import (
	"os"
	"testing"
)

func TestObserveReturnsAnExactCurrentProcessLifetime(t *testing.T) {
	first, err := Observe(PID(os.Getpid()))
	if err != nil {
		t.Fatal("observe current process")
	}
	second, err := Observe(PID(os.Getpid()))
	if err != nil {
		t.Fatal("observe current process a second time")
	}
	if first.PID != PID(os.Getpid()) || first.ParentPID <= 0 || first.ProcessGroup <= 0 || first.Executable == "" || len(first.Argv) == 0 {
		t.Fatal("current-process observation omitted required identity facts")
	}
	if first.StartIdentity == "" || first.StartIdentity != second.StartIdentity {
		t.Fatal("current-process lifetime identity was empty or unstable")
	}
}
