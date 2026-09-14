package tmux

import (
	"errors"
	"strings"
	"testing"
)

func TestAttachmentCommandSharesNavigationWithoutCreatingSessions(t *testing.T) {
	arguments, err := attachmentCommandArguments(AttachmentSpec{
		SourceID: "$7", SourceName: "agent", Columns: 100, Rows: 30,
		Server: ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"},
	})
	if err != nil {
		t.Fatal("direct attachment rejected an ordinary session")
	}
	command := strings.Join(arguments, " ")
	if !strings.Contains(command, "attach-session -E -t '$7'") {
		t.Fatal("attachment does not directly enter its selected session")
	}
	for _, forbidden := range []string{"new-session", "set-option", "active-pane", "session_group"} {
		if strings.Contains(command, forbidden) {
			t.Fatal("attachment changes session topology, options, or navigation isolation")
		}
	}
}

func TestAttachmentPresenceRequiresTheOwnedClientAndExactSessionLifetime(t *testing.T) {
	spec := AttachmentSpec{SourceID: "$7", SourceName: "old-name", Server: ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}}
	row := "5678|/dev/tty-test|" + spec.Server.Epoch + "|1234|1720000000|$7|2"
	count, tty, found, err := parseAttachmentClient(row, 5678, spec)
	if err != nil || !found || count != 2 || tty != "/dev/tty-test" {
		t.Fatal("exact attached client was not observed")
	}
	spec.SourceName = "renamed"
	if _, _, found, err := parseAttachmentClient(row, 5678, spec); err != nil || !found {
		t.Fatal("rename invalidated an attachment")
	}
	if _, _, found, err := parseAttachmentClient(row, 9999, spec); err != nil || found {
		t.Fatal("another client was accepted")
	}
	for _, changed := range []string{strings.Replace(row, "|$7|", "|$8|", 1), strings.Replace(row, "|1720000000|", "|1720000001|", 1)} {
		if _, _, _, err := parseAttachmentClient(changed, 5678, spec); !errors.Is(err, ErrAttachmentIdentityMismatch) {
			t.Fatal("changed session or server lifetime was accepted")
		}
	}
}
