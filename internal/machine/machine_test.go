package machine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

func TestParseAcceptsOnlyCanonicalHandles(t *testing.T) {
	const canonical = "mh-0123456789abcdef0123456789abcdef"
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "canonical", value: canonical, valid: true},
		{name: "empty", value: ""},
		{name: "missing prefix", value: "0123456789abcdef0123456789abcdef"},
		{name: "short", value: "mh-0123456789abcdef0123456789abcde"},
		{name: "long", value: "mh-0123456789abcdef0123456789abcdef0"},
		{name: "uppercase", value: "mh-0123456789ABCDEF0123456789ABCDEF"},
		{name: "nonhex", value: "mh-0123456789abcdef0123456789abcdeg"},
		{name: "newline", value: canonical + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handle, err := machine.Parse(test.value)
			if test.valid {
				if err != nil || handle.String() != test.value {
					t.Fatal("canonical machine handle was rejected or changed")
				}
				return
			}
			if err == nil {
				t.Fatal("noncanonical machine handle was accepted")
			}
		})
	}
}

// Machine identity is the root of every pairing, so its create-once, 0600, and
// never-repair guarantees regress loudly on every routine run. The proof needs
// only a writable temporary directory: no tmux, device, or network.
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
			t.Fatalf("concurrent machine-handle initialization failed: %v", result.err)
		}
		if published.String() == "" {
			published = result.handle
			continue
		}
		if result.handle != published {
			t.Fatalf("concurrent initialization returned different machine handles: caller_count=%d", callers)
		}
	}
	firstInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat initialized machine-handle file: %v", err)
	}
	if !firstInfo.Mode().IsRegular() || firstInfo.Mode().Perm() != 0o600 {
		t.Fatalf("initialized machine-handle file mode = %v, want a regular file with mode 0600", firstInfo.Mode())
	}
	loaded, err := machine.Load(path)
	if err != nil {
		t.Fatalf("load published machine-handle file: %v", err)
	}
	if loaded != published {
		t.Fatal("published machine-handle file did not load unchanged")
	}
	second, err := machine.Init(path)
	if err != nil {
		t.Fatalf("reinitialize existing machine-handle file: %v", err)
	}
	secondInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat reinitialized machine-handle file: %v", err)
	}
	if second != published || !os.SameFile(firstInfo, secondInfo) {
		t.Fatalf(
			"reinitialization replaced the published machine handle: handle_unchanged=%t file_unchanged=%t",
			second == published,
			os.SameFile(firstInfo, secondInfo),
		)
	}

	invalidPath := filepath.Join(t.TempDir(), "machine-handle")
	original := []byte("not-a-machine-handle\n")
	if err := os.WriteFile(invalidPath, original, 0o600); err != nil {
		t.Fatalf("write invalid existing machine-handle file: %v", err)
	}
	if _, err := machine.Init(invalidPath); err == nil {
		t.Fatal("initialization accepted an invalid existing machine handle")
	}
	contents, err := os.ReadFile(invalidPath)
	if err != nil {
		t.Fatalf("read invalid existing machine-handle file: %v", err)
	}
	if string(contents) != string(original) {
		t.Fatalf("initialization repaired or replaced an invalid existing machine handle: content_bytes=%d", len(contents))
	}
}
