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

func TestKillIdentityConditionEscapesFormatLiterals(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	condition := killIdentityCondition("$7", "name#,}suffix", server)
	want := "#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{==:#{session_name},name###,#}suffix}}}}}"
	if condition != want {
		t.Fatalf("conditional kill format = %q, want %q", condition, want)
	}
}

func TestKillEligibilityRequiresTheLastGroupedLink(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	condition := killEligibilityCondition("$7", "agent", server)
	if !strings.Contains(condition, killIdentityCondition("$7", "agent", server)) ||
		!strings.Contains(condition, "#{||:#{==:#{session_group_size},},#{==:#{session_group_size},1}}") {
		t.Fatalf("kill eligibility is not closed over identity and last-link topology: %q", condition)
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
		"#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{&&:#{==:#{@skid_character},old###,#}value},#{!=:#{@skid_internal},phone-shadow}}}}}}",
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

func TestAttachmentCreationUsesOneIdentityGateBeforeEveryMutation(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	arguments, err := attachmentCommandArguments(AttachmentSpec{
		SourceID:   "$7",
		SourceName: "laptop",
		ShadowName: "skid-phone-00112233445566778899aabbccddeeff",
		Server:     server,
	})
	if err != nil {
		t.Fatalf("build attachment command: %v", err)
	}
	want := []string{
		"if-shell", "-F", "-t", "$7",
		"#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$7},#{==:#{session_name},laptop}}}}}",
		"new-session -d -E -t '$7' -s 'skid-phone-00112233445566778899aabbccddeeff' ; set-option -t '=skid-phone-00112233445566778899aabbccddeeff:' -- @skid_internal phone-shadow ; set-option -pqu -t '$7' -- @skid_attention ; display-message -p -t '=skid-phone-00112233445566778899aabbccddeeff:' '#{session_id}' ; display-message -p -t '$7' '#{window_id}'",
		"display-message -p -l 'SKIDBLADNIR_IDENTITY_MISMATCH_V1'",
	}
	if !slices.Equal(arguments, want) {
		t.Fatalf("attachment arguments\nwant: %q\n got: %q", want, arguments)
	}
}

func TestAttachmentClientTargetsOnlyTheCapturedShadowID(t *testing.T) {
	arguments, err := attachmentClientArguments("$11")
	if err != nil {
		t.Fatalf("build attachment client command: %v", err)
	}
	want := []string{"-T", "RGB", "attach-session", "-E", "-f", "active-pane,ignore-size", "-t", "$11"}
	if !slices.Equal(arguments, want) {
		t.Fatalf("attachment client arguments = %q, want %q", arguments, want)
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

func TestAttachmentArmIsIdentityGatedAndRequiresAnAttachedClient(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	spec := AttachmentSpec{
		SourceID: "$7", SourceName: "laptop",
		ShadowName: "skid-phone-00112233445566778899aabbccddeeff", Server: server,
	}
	arguments, err := attachmentArmArguments(spec, "$11")
	if err != nil {
		t.Fatalf("build attachment arm command: %v", err)
	}
	want := []string{
		"if-shell", "-F", "-t", "$11",
		"#{&&:#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$11},#{==:#{session_name},skid-phone-00112233445566778899aabbccddeeff}}}}},#{&&:#{==:#{@skid_internal},phone-shadow},#{>:#{session_attached},0}}}",
		"set-option -t '$11:' destroy-unattached keep-last",
		"display-message -p -l 'SKIDBLADNIR_IDENTITY_MISMATCH_V1'",
	}
	if !slices.Equal(arguments, want) {
		t.Fatalf("attachment arm arguments\nwant: %q\n got: %q", want, arguments)
	}
}

// The normal detach path destroys the armed keep-last shadow before release
// runs, and if-shell's failable -t lookup then prints the mismatch marker with
// a zero exit, so marker-plus-absent must settle as success, marker-plus-live
// must retry until the disconnect window closes, and only a live shadow past
// the settle bound is a real identity conflict.
func TestShadowReleaseClassifiesDestroyedRetryingAndConflictingOutcomes(t *testing.T) {
	tests := []struct {
		output    string
		exists    bool
		retryable bool
		settled   bool
		err       error
	}{
		{"", true, true, true, nil},
		{identityMismatchMarker, false, true, true, nil},
		{identityMismatchMarker, false, false, true, nil},
		{identityMismatchMarker, true, true, false, nil},
		{identityMismatchMarker, true, false, false, ErrAttachmentIdentityMismatch},
	}
	for _, test := range tests {
		settled, err := classifyShadowRelease(test.output, test.exists, test.retryable)
		if settled != test.settled || !errors.Is(err, test.err) {
			t.Fatalf("release output %q exists=%t retryable=%t = (%t,%v), want (%t,%v)",
				test.output, test.exists, test.retryable, settled, err, test.settled, test.err)
		}
	}
	if settled, err := classifyShadowRelease("garbage", true, true); settled || err == nil {
		t.Fatalf("unexpected release output classified as (%t,%v)", settled, err)
	}
}

func TestAttachmentControlOutputDistinguishesCreationAndReadinessFailures(t *testing.T) {
	shadowID, sourceWindowID, err := parseAttachmentCreationOutput("$11\n@3")
	if err != nil || shadowID != "$11" || sourceWindowID != "@3" {
		t.Fatalf("valid attachment creation output = (%q,%q,%v), want ($11,@3,nil)", shadowID, sourceWindowID, err)
	}
	if _, _, err := parseAttachmentCreationOutput(identityMismatchMarker); !errors.Is(err, ErrAttachmentIdentityMismatch) {
		t.Fatalf("identity-mismatch creation output = %v", err)
	}
	for _, malformed := range []string{"not-a-session", "$11", "$11\nnot-a-window", "$11\n@3\nextra"} {
		if _, _, err := parseAttachmentCreationOutput(malformed); err == nil || errors.Is(err, ErrAttachmentIdentityMismatch) {
			t.Fatalf("invalid attachment creation output %q = %v", malformed, err)
		}
	}

	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	spec := AttachmentSpec{
		SourceID: "$7", SourceName: "laptop",
		ShadowName: "skid-phone-00112233445566778899aabbccddeeff", Server: server,
	}
	output := strings.Join([]string{server.Epoch, server.PID, server.StartTime, "$11", spec.ShadowName, phoneShadowMarker, "1"}, "|")
	attached, err := parseAttachmentStartupObservation(output, spec, "$11")
	if err != nil || attached != 1 {
		t.Fatalf("valid attachment observation = (%d,%v), want (1,nil)", attached, err)
	}
	changed := strings.Replace(output, spec.ShadowName, "changed", 1)
	if _, err := parseAttachmentStartupObservation(changed, spec, "$11"); !errors.Is(err, ErrAttachmentIdentityMismatch) {
		t.Fatalf("changed attachment observation = %v", err)
	}
}

func TestAttachmentCleanupPostconditionRequiresTheExactShadowToBeAbsentOrPromoted(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	spec := AttachmentSpec{
		SourceID: "$7", SourceName: "laptop",
		ShadowName: "skid-phone-00112233445566778899aabbccddeeff", Server: server,
	}
	header := server.Epoch + "|" + server.PID + "|" + server.StartTime + "|"

	tests := []struct {
		name   string
		output string
		want   error
	}{
		{name: "absent with unrestricted name", output: header + "|name|with|pipes", want: nil},
		{name: "promoted", output: header + "|" + spec.ShadowName, want: nil},
		{name: "still marked", output: header + phoneShadowMarker + "|" + spec.ShadowName, want: errAttachmentCleanupIncomplete},
		{name: "server changed", output: server.Epoch + "|9999|" + server.StartTime + "||laptop", want: ErrAttachmentIdentityMismatch},
		{name: "empty readback", output: "", want: errAttachmentCleanupReadbackInvalid},
		{name: "malformed readback", output: "malformed", want: errAttachmentCleanupReadbackInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := parseAttachmentCleanupPostcondition(test.output, spec)
			if test.want == nil && err != nil {
				t.Fatalf("cleanup postcondition = %v, want nil", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("cleanup postcondition = %v, want %v", err, test.want)
			}
		})
	}
}

func TestShadowReleaseConditionCannotDestroyTheLastLink(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	condition, err := shadowReleaseCondition("$11", "skid-phone-00112233445566778899aabbccddeeff", server)
	if err != nil {
		t.Fatalf("build shadow release condition: %v", err)
	}
	want := "#{&&:#{&&:#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef},#{&&:#{==:#{pid},1234},#{&&:#{==:#{start_time},1720000000},#{&&:#{==:#{session_id},$11},#{==:#{session_name},skid-phone-00112233445566778899aabbccddeeff}}}}},#{&&:#{==:#{@skid_internal},phone-shadow},#{==:#{session_attached},0}}}"
	if condition != want {
		t.Fatalf("shadow release condition = %q, want %q", condition, want)
	}

	release, err := shadowReleaseCommand("$11", "skid-phone-00112233445566778899aabbccddeeff", server)
	if err != nil {
		t.Fatalf("build shadow release command: %v", err)
	}
	if !strings.Contains(release, "#{>:#{session_group_size},1}") ||
		!strings.Contains(release, "kill-session -t '$11'") ||
		!strings.Contains(release, "set-option -u -t '$11' destroy-unattached") ||
		!strings.Contains(release, "set-option -qu -t '$11' -- @skid_internal") {
		t.Fatalf("shadow release omitted guarded kill or complete last-link promotion: %q", release)
	}
}

func TestPhoneShadowReconciliationIsExactAndTopologySpecific(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	duplicate := phoneShadowRecord{
		id: "$11", name: "skid-phone-00112233445566778899aabbccddeeff",
		groupSize: 2, groupSizeText: "2", server: server,
	}
	arguments, err := phoneShadowReconciliationArguments(duplicate)
	if err != nil {
		t.Fatalf("build duplicate-shadow reconciliation: %v", err)
	}
	for _, required := range []string{
		"#{==:#{@skid_server_epoch},v1-0123456789abcdef0123456789abcdef}",
		"#{==:#{pid},1234}",
		"#{==:#{start_time},1720000000}",
		"#{==:#{session_id},$11}",
		"#{==:#{session_name},skid-phone-00112233445566778899aabbccddeeff}",
		"#{==:#{@skid_internal},phone-shadow}",
		"#{==:#{session_attached},0}",
		"#{==:#{session_group_size},2}",
	} {
		if !strings.Contains(arguments[4], required) {
			t.Fatalf("reconciliation condition omitted %q: %q", required, arguments[4])
		}
	}
	if arguments[5] != "kill-session -t '$11'" {
		t.Fatalf("duplicate-link reconciliation = %q", arguments[5])
	}

	lastLink := duplicate
	lastLink.groupSize = 1
	lastLink.groupSizeText = ""
	arguments, err = phoneShadowReconciliationArguments(lastLink)
	if err != nil {
		t.Fatalf("build last-link reconciliation: %v", err)
	}
	if strings.Contains(arguments[5], "kill-session") ||
		!strings.Contains(arguments[5], "set-option -u -t '$11' destroy-unattached") ||
		!strings.Contains(arguments[5], "set-option -qu -t '$11' -- @skid_internal") {
		t.Fatalf("last-link reconciliation did not promote the session: %q", arguments[5])
	}

	attached := duplicate
	attached.attached = 1
	if _, err := phoneShadowReconciliationArguments(attached); err == nil {
		t.Fatal("attached phone shadow received a destructive reconciliation command")
	}
}

func TestPhoneShadowDiscoveryLeavesAttachedProtectedAndOrdinarySessionsUntouched(t *testing.T) {
	server := ServerIdentity{Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000"}
	output := strings.Join([]string{
		"$1|ordinary|phone-shadow|0|1|" + server.Epoch + "|" + server.PID + "|" + server.StartTime,
		"$2|skid-phone-00112233445566778899aabbccddeeff|phone-shadow|1|2|" + server.Epoch + "|" + server.PID + "|" + server.StartTime,
		"$3|skid-phone-ffeeddccbbaa99887766554433221100|phone-shadow|0|2|" + server.Epoch + "|" + server.PID + "|" + server.StartTime,
		"$4|skid-phone-0123456789abcdef0123456789abcdef|phone-shadow|0||" + server.Epoch + "|" + server.PID + "|" + server.StartTime,
	}, "\n")
	records, err := parsePhoneShadowRecords(output, server)
	if err != nil {
		t.Fatalf("parse phone-shadow inventory: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("owned phone-shadow records = %d, want 3: %+v", len(records), records)
	}
	if phoneShadowNeedsReconciliation(records[0], nil) {
		t.Fatal("attached phone shadow was selected for reconciliation")
	}
	protected := map[string]struct{}{records[1].name: {}}
	if phoneShadowNeedsReconciliation(records[1], protected) {
		t.Fatal("live protected phone shadow was selected for reconciliation")
	}
	if !phoneShadowNeedsReconciliation(records[1], nil) {
		t.Fatal("stale unattached phone shadow was not selected for reconciliation")
	}
	if records[2].groupSize != 1 || records[2].groupSizeText != "" {
		t.Fatalf("ungrouped last link topology = (%d,%q), want (1,empty)", records[2].groupSize, records[2].groupSizeText)
	}
	arguments, err := phoneShadowReconciliationArguments(records[2])
	if err != nil {
		t.Fatalf("build ungrouped last-link reconciliation: %v", err)
	}
	if !strings.Contains(arguments[4], "#{==:#{session_group_size},}") || strings.Contains(arguments[5], "kill-session") {
		t.Fatalf("ungrouped last link was not exactly promoted: %q", arguments)
	}
	if IsPhoneShadow("ordinary", phoneShadowMarker) || IsPhoneShadow(records[1].name, "") {
		t.Fatal("partial ownership facts classified an ordinary session as a phone shadow")
	}
}
