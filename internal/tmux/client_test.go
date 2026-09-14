package tmux

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestAttachmentStartFailureDistinguishesIncompleteCleanup(t *testing.T) {
	cause := errors.New("closed start failure")
	if err := attachmentStartFailure(cause, nil); !errors.Is(err, cause) || errors.Is(err, ErrAttachmentCleanupFailed) {
		t.Fatalf("cleanly aborted start = %v, want only original failure", err)
	}
	cleanup := errors.New("closed cleanup failure")
	err := attachmentStartFailure(cause, cleanup)
	if !errors.Is(err, cause) || !errors.Is(err, cleanup) || !errors.Is(err, ErrAttachmentCleanupFailed) {
		t.Fatalf("incompletely aborted start lost failure identity: %v", err)
	}
}

func TestTmuxEnvironmentCannotFollowAnInvokingClient(t *testing.T) {
	inherited := []string{
		"PATH=/usr/bin",
		"TMUX=/tmp/tmux-1000/default,123,0",
		"TMUX_PANE=%7",
		"TMUX_TMPDIR=/tmp/custom",
	}

	filtered := filterTmuxEnvironment(inherited)
	if slices.Contains(filtered, inherited[1]) || slices.Contains(filtered, inherited[2]) {
		t.Fatalf("tmux command retained invoking-client identity: %q", filtered)
	}
	if !slices.Contains(filtered, inherited[3]) {
		t.Fatalf("tmux command discarded the operator's explicit socket root: %q", filtered)
	}
	if !slices.Contains(filtered, inherited[0]) {
		t.Fatalf("tmux environment discarded an unrelated value: %q", filtered)
	}
}

func TestMutationIdentityConditionEscapesFormatLiterals(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	condition := mutationIdentityCondition("$7", "name#,}suffix", server)
	want := "#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{==:#{session_name},name###,#}suffix}}}}}"
	if condition != want {
		t.Fatalf("conditional mutation format = %q, want %q", condition, want)
	}
}

func TestRenameUsesOneNarrowConditionalCommand(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	arguments, err := renameSessionArguments("$7", "laptop#,}name", "new_name-1", server)
	if err != nil {
		t.Fatalf("build rename command: %v", err)
	}
	want := []string{
		"if-shell", "-F", "-t", "$7",
		"#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{==:#{session_name},laptop###,#}name}}}}}",
		"rename-session -t '$7' 'new_name-1' ; display-message -p -l 'SKIDBLADNIR_RENAME_SUCCESS_V1'",
		"display-message -p -l 'SKIDBLADNIR_IDENTITY_MISMATCH_V1'",
	}
	if !slices.Equal(arguments, want) {
		t.Fatalf("rename arguments\nwant: %q\n got: %q", want, arguments)
	}
	if strings.Contains(strings.Join(arguments, " "), "=new_name-1") {
		t.Fatal("rename command targeted the desired name")
	}
	if _, err := renameSessionArguments("$7", "laptop", "unsafe;name", server); err == nil {
		t.Fatal("unsafe desired rename token was accepted")
	}
}

func TestCharacterAssignmentUsesOneNarrowConditionalCommand(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	arguments, err := characterAssignmentArguments("$7", "old#,}value", "norse.durinn", server)
	if err != nil {
		t.Fatalf("build character assignment command: %v", err)
	}
	want := []string{
		"if-shell", "-F", "-t", "$7",
		"#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{==:#{@skid_character},old###,#}value}}}}}",
		"set-option -t '$7' -- @skid_character norse.durinn",
		"display-message -p -l 'SKIDBLADNIR_IDENTITY_MISMATCH_V1'",
	}
	if !slices.Equal(arguments, want) {
		t.Fatalf("character assignment arguments\nwant: %q\n got: %q", want, arguments)
	}
	if strings.Contains(arguments[4], "session_name") {
		t.Fatalf("character assignment incorrectly depends on mutable tmux name: %q", arguments[4])
	}
	if _, err := characterAssignmentArguments("$7", "", "norse.durinn ; kill-server", server); err == nil {
		t.Fatal("unsafe character command token was accepted")
	}
}

func TestAttachmentClientPinsTerminalWithoutChangingTheSocketRoot(t *testing.T) {
	environment := attachmentEnvironment([]string{
		"PATH=/usr/bin",
		"TERM=dumb",
		"TMUX_TMPDIR=/tmp/private",
	})
	if slices.Contains(environment, "TERM=dumb") || !slices.Contains(environment, "TERM=xterm-256color") {
		t.Fatalf("attachment terminal environment is not pinned: %q", environment)
	}
	if !slices.Contains(environment, "TMUX_TMPDIR=/tmp/private") {
		t.Fatalf("attachment environment changed the selected socket root: %q", environment)
	}
}
