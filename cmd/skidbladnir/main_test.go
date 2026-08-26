package main

import (
	"bytes"
	"testing"
)

func TestStatusHookDoesNotRequireAHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("TMUX_PANE", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := run([]string{"status-hook", "SessionStart"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("status-hook exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("status-hook output = (%q, %q), want quiet success", stdout.String(), stderr.String())
	}
}
