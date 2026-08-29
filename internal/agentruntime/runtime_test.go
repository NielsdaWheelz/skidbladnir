package agentruntime

import (
	"strings"
	"testing"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func TestProjectBindsRegistrationToTheExactForegroundLifetime(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	process := processinfo.Observation{
		PID: 4312, StartIdentity: "991827", Executable: "/home/niels/.local/share/codex/codex",
		Argv: []string{"codex"},
	}
	foreground, found := ClassifyForeground(profiles, process)
	if !found || foreground.Provider != ProviderCodex || foreground.PID != process.PID {
		t.Fatalf("classified foreground = %+v, found=%t", foreground, found)
	}
	registration, err := EncodeRegistration(foreground, "work", "019abcXYZ")
	if err != nil {
		t.Fatalf("encode registration: %v", err)
	}
	const want = "v1:4312:991827:Codex:work:MDE5YWJjWFla"
	if registration != want {
		t.Fatalf("registration = %q, want %q", registration, want)
	}

	agent, found := Project(profiles, process, registration)
	if !found || agent.Provider != ProviderCodex || agent.PID != process.PID || agent.Profile != "work" || agent.ProviderSession == nil {
		t.Fatalf("projected agent = %+v, found=%t", agent, found)
	}
	if agent.ProviderSession.ID() != "019abcXYZ" || agent.ProviderSession.Name() != "" {
		t.Fatalf("projected provider session = id %q name %q", agent.ProviderSession.ID(), agent.ProviderSession.Name())
	}

	invalid := map[string]string{
		"reused pid":         strings.Replace(registration, ":991827:", ":991828:", 1),
		"other pid":          strings.Replace(registration, ":4312:", ":4313:", 1),
		"other provider":     strings.Replace(registration, ":Codex:", ":Claude:", 1),
		"unknown profile":    strings.Replace(registration, ":work:", ":other:", 1),
		"padded session id":  registration + "=",
		"invalid session id": strings.Replace(registration, "MDE5YWJjWFla", "IA", 1),
	}
	for name, value := range invalid {
		t.Run(name, func(t *testing.T) {
			projected, found := Project(profiles, process, value)
			if !found || projected.Provider != ProviderCodex || projected.PID != process.PID {
				t.Fatalf("invalid registration changed process-derived agent: %+v found=%t", projected, found)
			}
			if projected.Profile != "" || projected.ProviderSession != nil {
				t.Fatalf("accepted fields from invalid registration: %+v", projected)
			}
		})
	}
}

func TestProjectRejectsOversizedRegistration(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	process := processinfo.Observation{
		PID: 4312, StartIdentity: "991827", Executable: "/home/niels/.local/share/codex/codex",
		Argv: []string{"codex"},
	}
	registration := "v1:4312:991827:Codex:work:" + strings.Repeat("QUFB", 1024)

	projected, found := Project(profiles, process, registration)
	if !found || projected.Provider != ProviderCodex || projected.PID != process.PID {
		t.Fatalf("oversized registration changed process-derived agent: %+v found=%t", projected, found)
	}
	if projected.Profile != "" || projected.ProviderSession != nil {
		t.Fatalf("accepted fields from oversized registration: %+v", projected)
	}
}

func TestProjectAcceptsTheMaximumLengthRegistration(t *testing.T) {
	profileKey := ProfileKey("p" + strings.Repeat("x", 31))
	configured := testProfiles()
	configured[1].Key = profileKey
	profiles, err := ValidateProfiles(configured)
	if err != nil {
		t.Fatalf("validate maximum-length profile fixture: %v", err)
	}
	process := processinfo.Observation{
		PID: processinfo.PID(int(^uint(0) >> 1)), StartIdentity: "18446744073709551615",
		Executable: "/home/niels/.local/share/claude/versions/2.1.236",
		Argv:       []string{"/home/niels/.local/bin/claude"},
	}
	foreground, found := ClassifyForeground(profiles, process)
	if !found {
		t.Fatal("maximum-length foreground was not classified")
	}
	registration, err := EncodeRegistration(foreground, profileKey, strings.Repeat("x", 128))
	if err != nil {
		t.Fatalf("encode maximum-length registration: %v", err)
	}
	if len(registration) != 255 {
		t.Fatalf("maximum-length registration has %d bytes, want 255", len(registration))
	}

	agent, found := Project(profiles, process, registration)
	if !found || agent.Profile != profileKey || agent.ProviderSession == nil || agent.ProviderSession.ID() != strings.Repeat("x", 128) {
		t.Fatalf("maximum-length projected agent = %+v found=%t", agent, found)
	}
}

func TestRuntimeLifetimeRequiresACanonicalPositiveUint64StartIdentity(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	process := processinfo.Observation{
		PID: 4312, Executable: "/home/niels/.local/share/codex/codex", Argv: []string{"codex"},
	}
	invalid := []struct {
		name  string
		value processinfo.StartIdentity
	}{
		{name: "empty", value: ""},
		{name: "zero", value: "0"},
		{name: "leading zero", value: "01"},
		{name: "leading plus", value: "+1"},
		{name: "above uint64", value: "18446744073709551616"},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			startIdentity := test.value
			process.StartIdentity = startIdentity
			if foreground, found := ClassifyForeground(profiles, process); found {
				t.Fatalf("classified invalid start identity %q as %+v", startIdentity, foreground)
			}
			foreground := Foreground{Provider: ProviderCodex, PID: process.PID, StartIdentity: startIdentity}
			if registration, err := EncodeRegistration(foreground, "work", "session"); err == nil {
				t.Fatalf("encoded invalid start identity %q as %q", startIdentity, registration)
			}
			registration := "v1:4312:" + string(startIdentity) + ":Codex:work:c2Vzc2lvbg"
			if parsed, valid := parseRegistration(registration); valid {
				t.Fatalf("parsed invalid start identity %q as %+v", startIdentity, parsed)
			}
		})
	}
}

func TestValidateAgentRuntimeOwnsTheCompleteRuntimeBoundary(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	facts, err := NewProviderSessionFacts("session-1", "")
	if err != nil {
		t.Fatalf("construct provider session facts: %v", err)
	}
	valid := AgentRuntime{Provider: ProviderCodex, PID: 4312, Profile: "work", ProviderSession: facts}
	if err := ValidateAgentRuntime(profiles, valid); err != nil {
		t.Fatalf("validate complete agent runtime: %v", err)
	}
	if err := ValidateAgentRuntime(profiles, AgentRuntime{Provider: ProviderClaude, PID: 5221}); err != nil {
		t.Fatalf("validate agent runtime with optional facts absent: %v", err)
	}

	invalid := map[string]AgentRuntime{
		"unknown provider":            {Provider: "Other", PID: valid.PID},
		"nonpositive pid":             {Provider: valid.Provider, PID: 0},
		"malformed profile":           {Provider: valid.Provider, PID: valid.PID, Profile: "Work"},
		"unconfigured profile":        {Provider: valid.Provider, PID: valid.PID, Profile: "other"},
		"profile from other provider": {Provider: ProviderClaude, PID: valid.PID, Profile: "work"},
		"empty provider session":      {Provider: valid.Provider, PID: valid.PID, ProviderSession: &ProviderSessionFacts{}},
		"unsafe provider session":     {Provider: valid.Provider, PID: valid.PID, ProviderSession: &ProviderSessionFacts{id: "contains space"}},
	}
	for name, agent := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateAgentRuntime(profiles, agent); err == nil {
				t.Fatalf("validated invalid agent runtime: %+v", agent)
			}
		})
	}
}

func TestValidateAgentRuntimeRejectsACodexProviderSessionName(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	facts, err := NewProviderSessionFacts("session-1", "ga-worker")
	if err != nil {
		t.Fatalf("construct bounded provider session facts: %v", err)
	}
	agent := AgentRuntime{
		Provider:        ProviderCodex,
		PID:             4312,
		Profile:         "work",
		ProviderSession: facts,
	}
	if err := ValidateAgentRuntime(profiles, agent); err == nil {
		t.Fatalf("validated a Codex runtime with a provider session name: %+v", agent)
	}
	agent.Provider = ProviderClaude
	agent.Profile = "claude-work"
	if err := ValidateAgentRuntime(profiles, agent); err != nil {
		t.Fatalf("rejected legal Claude provider session id/name facts: %v", err)
	}
}

func TestClassifyForegroundMatchesSignaturesExactly(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	exact := processinfo.Observation{
		PID: 4312, StartIdentity: "991827", Executable: "/opt/codex",
		Argv: []string{"codex"},
	}
	if foreground, found := ClassifyForeground(profiles, exact); !found || foreground.Provider != ProviderCodex {
		t.Fatalf("exact Codex foreground = %+v, found=%t", foreground, found)
	}
	for _, nearMiss := range []processinfo.Observation{
		{PID: 4312, StartIdentity: "991827", Executable: "/opt/codex-beta", Argv: exact.Argv},
		{PID: 4312, StartIdentity: "991827", Executable: "/usr/bin/node", Argv: []string{"node"}},
		{PID: 4312, Executable: exact.Executable, Argv: exact.Argv},
	} {
		if foreground, found := ClassifyForeground(profiles, nearMiss); found {
			t.Fatalf("near-miss process classified as %+v: %+v", foreground, nearMiss)
		}
	}
	ambiguous := processinfo.Observation{
		PID: 4312, StartIdentity: "991827", Executable: "/opt/codex",
		Argv: []string{"/home/niels/.local/bin/claude"},
	}
	if foreground, found := ClassifyForeground(profiles, ambiguous); found {
		t.Fatalf("concrete cross-provider match classified as %+v", foreground)
	}
}

func TestProjectReturnsObservedClaudeNameWithoutARegistration(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	process := processinfo.Observation{
		PID: 5221, StartIdentity: "118234", Executable: "/home/niels/.local/share/claude/versions/2.1.236",
		Argv: []string{"/home/niels/.local/bin/claude", "--name=ga-worker", "--permission-mode", "auto"},
	}
	agent, found := Project(profiles, process, "")
	if !found || agent.Provider != ProviderClaude || agent.PID != process.PID || agent.Profile != "" || agent.ProviderSession == nil {
		t.Fatalf("unregistered named Claude projection = %+v, found=%t", agent, found)
	}
	if agent.ProviderSession.ID() != "" || agent.ProviderSession.Name() != "ga-worker" {
		t.Fatalf("Claude provider session = id %q name %q", agent.ProviderSession.ID(), agent.ProviderSession.Name())
	}

	process.Argv = []string{"/home/niels/.local/bin/claude"}
	agent, found = Project(profiles, process, "")
	if !found || agent.ProviderSession != nil {
		t.Fatalf("unnamed Claude projection = %+v, found=%t", agent, found)
	}
}

func TestClaudeNameRecognizesOnlyOneExactSafeFlag(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "short", argv: []string{"claude", "-n", "worker one"}, want: "worker one"},
		{name: "long", argv: []string{"claude", "--name", "worker-two"}, want: "worker-two"},
		{name: "assignment", argv: []string{"claude", "--name=worker-three"}, want: "worker-three"},
		{name: "resume has no name", argv: []string{"claude", "--resume", "session-1"}},
		{name: "missing value", argv: []string{"claude", "--name"}},
		{name: "empty assignment", argv: []string{"claude", "--name="}},
		{name: "duplicate", argv: []string{"claude", "-n", "one", "--name=two"}},
		{name: "after delimiter", argv: []string{"claude", "--", "--name=prompt-text"}},
		{name: "bidi", argv: []string{"claude", "--name=worker\u2066hidden"}},
		{name: "bidi mark", argv: []string{"claude", "--name=worker\u200ehidden"}},
		{name: "line separator", argv: []string{"claude", "--name=worker\u2028hidden"}},
		{name: "paragraph separator", argv: []string{"claude", "--name=worker\u2029hidden"}},
		{name: "too long", argv: []string{"claude", "--name=" + strings.Repeat("x", 129)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := claudeName(test.argv); got != test.want {
				t.Fatalf("Claude name from %q = %q, want %q", test.argv, got, test.want)
			}
		})
	}
}

func TestHookOriginAcceptsATtylessHelperOnlyForOneExactOuterRuntime(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	const terminal processinfo.TerminalDevice = 34817
	wrapper := processinfo.Observation{
		PID: 101, ParentPID: 50, SessionID: 101, TerminalDevice: terminal,
		ForegroundProcessGroup: 101, StartIdentity: "1001", Executable: "node",
		Argv: []string{"node", "/home/niels/.local/bin/codex"},
	}
	native := processinfo.Observation{
		PID: 102, ParentPID: wrapper.PID, SessionID: wrapper.SessionID, TerminalDevice: terminal,
		ForegroundProcessGroup: wrapper.PID, StartIdentity: "1002", Executable: "codex",
	}
	ttylessHelper := processinfo.Observation{
		PID: 103, ParentPID: native.PID, SessionID: 103, TerminalDevice: 0,
		ForegroundProcessGroup: 0, StartIdentity: "1003", Executable: "skidbladnir",
	}
	origin, found := HookOrigin(profiles, []processinfo.Observation{ttylessHelper, native, wrapper}, terminal)
	if !found || origin.Provider != ProviderCodex || origin.PID != wrapper.PID || origin.StartIdentity != wrapper.StartIdentity {
		t.Fatalf("tty-less helper origin = %+v, found=%t, want outer wrapper", origin, found)
	}

	nested := processinfo.Observation{
		PID: 201, ParentPID: native.PID, SessionID: wrapper.SessionID, TerminalDevice: terminal,
		ForegroundProcessGroup: 201, StartIdentity: "2001", Executable: "/home/niels/.local/share/claude/versions/2.1.236",
		Argv: []string{"/home/niels/.local/bin/claude"},
	}
	ttylessHelper.ParentPID = nested.PID
	if _, found := HookOrigin(profiles, []processinfo.Observation{ttylessHelper, nested, native, wrapper}, terminal); found {
		t.Fatal("accepted a nested cross-provider runtime after it took foreground control")
	}

	ttylessHelper.ParentPID = native.PID
	wrapper.ForegroundProcessGroup = native.PID
	native.ForegroundProcessGroup = native.PID
	if _, found := HookOrigin(profiles, []processinfo.Observation{ttylessHelper, native, wrapper}, terminal); found {
		t.Fatal("accepted the native child instead of requiring the outer wrapped runtime to lead the foreground process group")
	}
	if _, found := HookOrigin(profiles, []processinfo.Observation{ttylessHelper, native, wrapper}, terminal+1); found {
		t.Fatal("accepted a provider runtime from another pane terminal")
	}
}

func TestProviderSessionFactsCannotBeEmptyOrUnsafe(t *testing.T) {
	if _, err := NewProviderSessionFacts("", ""); err == nil {
		t.Fatal("constructed empty provider session facts")
	}
	if _, err := NewProviderSessionFacts("contains space", ""); err == nil {
		t.Fatal("constructed provider session facts with a non-visible id")
	}
	if _, err := NewProviderSessionFacts("", "unsafe\u202ename"); err == nil {
		t.Fatal("constructed provider session facts with a bidi name")
	}
	for _, name := range []string{"unsafe\u2028name", "unsafe\u2029name"} {
		if _, err := NewProviderSessionFacts("", name); err == nil {
			t.Fatalf("constructed provider session facts with a Unicode separator: %q", name)
		}
	}
	facts, err := NewProviderSessionFacts("session-1", "worker")
	if err != nil || facts.ID() != "session-1" || facts.Name() != "worker" {
		t.Fatalf("provider session facts = id %q name %q error %v", facts.ID(), facts.Name(), err)
	}
}
