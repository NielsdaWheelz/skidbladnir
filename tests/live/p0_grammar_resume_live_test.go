//go:build live

package live

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestP0PinnedCodexResumeArgvGrammar(t *testing.T) {
	t.Parallel()
	lock := readDirectTUILock(t)
	const threadID = "123e4567-e89b-42d3-a456-426614174000"

	// This freezes the raw pinned-Codex parser, not the later launcher policy.
	// Raw acceptance of names and conflicting options is therefore a fact that
	// the P1 launcher must narrow; it is not permission to forward them.
	tests := []struct {
		name      string
		argv      []string
		accepted  bool
		errorText string
	}{
		{
			name:     "bare picker",
			argv:     []string{"resume", "--help"},
			accepted: true,
		},
		{
			name:     "canonical UUID",
			argv:     []string{"resume", threadID, "--help"},
			accepted: true,
		},
		{
			name:     "raw session name",
			argv:     []string{"resume", "ga-untracked-name", "--help"},
			accepted: true,
		},
		{
			name:     "UUID and prompt",
			argv:     []string{"resume", threadID, "continue here", "--help"},
			accepted: true,
		},
		{
			name:     "resume options before UUID",
			argv:     []string{"resume", "--strict-config", "-C", directTUICWD, "-m", "gpt-5.1-codex-mini", threadID, "--help"},
			accepted: true,
		},
		{
			name:     "interactive options before subcommand",
			argv:     []string{"--strict-config", "-C", directTUICWD, "resume", threadID, "--help"},
			accepted: true,
		},
		{
			name:     "raw remote option",
			argv:     []string{"resume", "--remote", "ws://127.0.0.1:1", threadID, "--help"},
			accepted: true,
		},
		{
			name:     "raw config and policy overrides",
			argv:     []string{"resume", "-c", "model=\"o3\"", "--sandbox", "read-only", "--ask-for-approval", "on-request", "--dangerously-bypass-hook-trust", threadID, "--help"},
			accepted: true,
		},
		{
			name:      "third positional",
			argv:      []string{"resume", threadID, "prompt", "extra", "--help"},
			errorText: "unexpected argument 'extra' found",
		},
		{
			name:      "unknown option",
			argv:      []string{"resume", "--not-in-this-pin", "--help"},
			errorText: "unexpected argument '--not-in-this-pin' found",
		},
		{
			name:      "missing cd value",
			argv:      []string{"resume", "--cd", "--help"},
			errorText: "a value is required for '--cd <DIR>'",
		},
		{
			name:      "invalid sandbox value",
			argv:      []string{"resume", "--sandbox", "invalid", "--help"},
			errorText: "invalid value 'invalid' for '--sandbox <SANDBOX_MODE>'",
		},
		{
			name:      "duplicate singleton option",
			argv:      []string{"resume", "--last", "--last", "--help"},
			errorText: "the argument '--last' cannot be used multiple times",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output, err := exec.Command(lock.BinaryPath, test.argv...).CombinedOutput()
			if test.accepted {
				if err != nil {
					t.Fatalf("pinned resume parser rejected argv %q: %v: %s", test.argv, err, boundedDirectTUIError(output))
				}
				for _, exact := range []string{
					"Usage: codex resume [OPTIONS] [SESSION_ID] [PROMPT]",
					"Session id (UUID) or session name.",
				} {
					if !strings.Contains(string(output), exact) {
						t.Fatalf("pinned resume help for argv %q omits %q", test.argv, exact)
					}
				}
				return
			}

			var exitError *exec.ExitError
			if !errors.As(err, &exitError) {
				t.Fatalf("pinned resume parser accepted rejected argv %q", test.argv)
			}
			if exitError.ExitCode() != 2 {
				t.Fatalf("pinned resume parser exit for argv %q = %d, want grammar error 2: %s", test.argv, exitError.ExitCode(), boundedDirectTUIError(output))
			}
			if !strings.Contains(string(output), test.errorText) {
				t.Fatalf("pinned resume parser error for argv %q omits %q: %s", test.argv, test.errorText, boundedDirectTUIError(output))
			}
		})
	}
}
