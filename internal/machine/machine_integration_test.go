//go:build integration

package machine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

func TestInitAtomicallyCreatesOnceWithMode0600AndNeverRepairs(t *testing.T) {
	const callers = 8
	type result struct {
		handle machine.Handle
		err    error
	}

	path := filepath.Join(t.TempDir(), "config", "machine-handle")
	start := make(chan struct{})
	results := make(chan result, callers)
	for range callers {
		go func() {
			<-start
			handle, err := machine.Init(path)
			results <- result{handle: handle, err: err}
		}()
	}
	close(start)

	var published machine.Handle
	for range callers {
		result := <-results
		if result.err != nil {
			t.Fatal("concurrent machine-handle initialization failed")
		}
		if published.String() == "" {
			published = result.handle
		} else if result.handle != published {
			t.Fatal("concurrent initialization returned different machine handles")
		}
	}
	firstInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal("stat initialized machine-handle file")
	}
	if !firstInfo.Mode().IsRegular() || firstInfo.Mode().Perm() != 0o600 {
		t.Fatal("initialized machine-handle file is not regular mode 0600")
	}
	loaded, err := machine.Load(path)
	if err != nil || loaded != published {
		t.Fatal("published machine-handle file did not load unchanged")
	}
	second, err := machine.Init(path)
	if err != nil {
		t.Fatal("reinitialize existing machine-handle file")
	}
	secondInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal("stat reinitialized machine-handle file")
	}
	if second != published || !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("reinitialization replaced the published machine handle")
	}

	invalidPath := filepath.Join(t.TempDir(), "machine-handle")
	original := []byte("not-a-machine-handle\n")
	if err := os.WriteFile(invalidPath, original, 0o600); err != nil {
		t.Fatal("write invalid existing machine-handle file")
	}
	if _, err := machine.Init(invalidPath); err == nil {
		t.Fatal("initialization accepted an invalid existing machine handle")
	}
	contents, err := os.ReadFile(invalidPath)
	if err != nil {
		t.Fatal("read invalid existing machine-handle file")
	}
	if string(contents) != string(original) {
		t.Fatal("initialization repaired or replaced an invalid existing machine handle")
	}
}
