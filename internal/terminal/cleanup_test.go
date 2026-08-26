package terminal_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
)

func TestCleanupReleasesEachOwnedResourceOnceInOrder(t *testing.T) {
	var called []string
	cleanup := terminal.NewCleanup(terminal.OwnedResources{
		ClosePTY: func() error {
			called = append(called, "pty")
			return nil
		},
		CloseClient: func() error {
			called = append(called, "client")
			return nil
		},
		ReleaseShadow: func() error {
			called = append(called, "shadow")
			return nil
		},
	})

	const callers = 12
	var wait sync.WaitGroup
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			if err := cleanup.Close(); err != nil {
				t.Errorf("close attachment: %v", err)
			}
		}()
	}
	wait.Wait()

	want := []string{"pty", "client", "shadow"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("unexpected cleanup order or multiplicity: want=%v got=%v", want, called)
	}
}

func TestCleanupAttemptsEveryResourceAfterFailures(t *testing.T) {
	ptyFailure := errors.New("pty close failed")
	shadowFailure := errors.New("shadow release failed")
	var called []string
	cleanup := terminal.NewCleanup(terminal.OwnedResources{
		ClosePTY: func() error {
			called = append(called, "pty")
			return ptyFailure
		},
		CloseClient: func() error {
			called = append(called, "client")
			return nil
		},
		ReleaseShadow: func() error {
			called = append(called, "shadow")
			return shadowFailure
		},
	})

	err := cleanup.Close()
	if !errors.Is(err, ptyFailure) || !errors.Is(err, shadowFailure) {
		t.Fatalf("cleanup did not retain both failures: %v", err)
	}
	if want := []string{"pty", "client", "shadow"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("cleanup stopped early: want=%v got=%v", want, called)
	}
}
