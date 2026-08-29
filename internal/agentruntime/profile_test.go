package agentruntime

import (
	"slices"
	"strings"
	"testing"
)

func TestProvidersAreClosed(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Provider
	}{
		{input: "Codex", want: ProviderCodex},
		{input: "Claude", want: ProviderClaude},
	} {
		got, err := ParseProvider(test.input)
		if err != nil || got != test.want || got.String() != test.input {
			t.Fatalf("parse provider %q = (%q, %v), want %q", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"", "codex", "ClaudeCode", "Other"} {
		if _, err := ParseProvider(input); err == nil {
			t.Fatalf("accepted provider outside the closed contract: %q", input)
		}
	}
}

func TestProfileKeysUseTheClosedCanonicalGrammar(t *testing.T) {
	maximum := "p" + strings.Repeat("x", 31)
	for _, input := range []string{"p", "claude-work", "work_2", maximum} {
		key, err := ParseProfileKey(input)
		if err != nil || string(key) != input {
			t.Fatalf("parse profile key %q = (%q, %v)", input, key, err)
		}
	}
	for _, input := range []string{"", "Work", "2work", "work.", "work/2", maximum + "x", "wörk"} {
		if key, err := ParseProfileKey(input); err == nil {
			t.Fatalf("parsed invalid profile key %q as %q", input, key)
		}
	}
}

func TestValidateProfilesAcceptsExactProviderHomesAndDisjointSignatures(t *testing.T) {
	input := testProfiles()
	validated, err := ValidateProfiles(input)
	if err != nil {
		t.Fatalf("validate exact provider profiles: %v", err)
	}
	input[0].Environment[0].Value = "/changed"
	input[0].ForegroundSignatures[0].ExecutableBase = "changed"
	input[0].Arguments[0] = "changed"
	if validated[0].Environment[0].Value != "/home/niels/.codex-work" ||
		validated[0].ForegroundSignatures[0].ExecutableBase != "codex" ||
		validated[0].Arguments[0] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("validated profiles retained caller-owned slices: %+v", validated[0])
	}
}

func TestValidateProfilesRejectsProviderAmbiguityAndManagedNameConflicts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]Profile) []Profile
	}{
		{
			name: "unsupported provider",
			mutate: func(profiles []Profile) []Profile {
				profiles[0].Provider = Provider("Other")
				return profiles
			},
		},
		{
			name: "missing provider home",
			mutate: func(profiles []Profile) []Profile {
				profiles[0].Environment = nil
				return profiles
			},
		},
		{
			name: "relative provider home",
			mutate: func(profiles []Profile) []Profile {
				profiles[0].Environment[0].Value = ".codex-work"
				return profiles
			},
		},
		{
			name: "provider home from another provider",
			mutate: func(profiles []Profile) []Profile {
				profiles[1].Environment = append(profiles[1].Environment, EnvironmentVariable{Name: "CODEX_HOME", Value: "/home/niels/.codex-work"})
				return profiles
			},
		},
		{
			name: "duplicate home within provider",
			mutate: func(profiles []Profile) []Profile {
				duplicate := profiles[0]
				duplicate.Key = "personal"
				duplicate.Label = "Codex · Personal"
				profiles = append(profiles, duplicate)
				return profiles
			},
		},
		{
			name: "cross-provider duplicate signatures",
			mutate: func(profiles []Profile) []Profile {
				profiles[1].ForegroundSignatures = []ForegroundSignature{{ExecutableBase: "codex"}}
				return profiles
			},
		},
		{
			name: "Claude short name flag",
			mutate: func(profiles []Profile) []Profile {
				profiles[1].Arguments = []string{"-n", "configured"}
				return profiles
			},
		},
		{
			name: "Claude long name assignment",
			mutate: func(profiles []Profile) []Profile {
				profiles[1].Arguments = []string{"--name=configured"}
				return profiles
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ValidateProfiles(test.mutate(testProfiles())); err == nil {
				t.Fatalf("accepted invalid profile configuration for %s", test.name)
			}
		})
	}
}

func TestMatchProfileEnvironmentReadsOnlyTheProvidersOwnDiscriminator(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	lookups := make([]string, 0, 1)
	profile, found := MatchProfileEnvironment(profiles, ProviderClaude, func(name string) (string, bool) {
		lookups = append(lookups, name)
		return "/home/niels/.claude-work", true
	})
	if !found || profile != "claude-work" || !slices.Equal(lookups, []string{"CLAUDE_CONFIG_DIR"}) {
		t.Fatalf("Claude profile match = (%q, %t), lookups=%q", profile, found, lookups)
	}

	if profile, found := MatchProfileEnvironment(profiles, ProviderCodex, func(string) (string, bool) {
		return "/home/niels/.claude-work", true
	}); found || profile != "" {
		t.Fatalf("cross-provider home matched profile %q", profile)
	}
}

func TestLaunchArgumentsNamesOnlyManagedClaude(t *testing.T) {
	profiles, err := ValidateProfiles(testProfiles())
	if err != nil {
		t.Fatalf("validate profile fixture: %v", err)
	}
	if got, want := LaunchArguments(profiles[0], "ga-worker"), []string{"--dangerously-bypass-approvals-and-sandbox"}; !slices.Equal(got, want) {
		t.Fatalf("managed Codex arguments = %q, want %q", got, want)
	}
	if got, want := LaunchArguments(profiles[1], "ga-worker"), []string{"--name", "ga-worker", "--permission-mode", "auto"}; !slices.Equal(got, want) {
		t.Fatalf("managed Claude arguments = %q, want %q", got, want)
	}
}

func testProfiles() []Profile {
	return []Profile{
		{
			Key:      "work",
			Label:    "Codex · Work",
			Provider: ProviderCodex,
			Command:  "/home/niels/bin/codex-work",
			Environment: []EnvironmentVariable{{
				Name: "CODEX_HOME", Value: "/home/niels/.codex-work",
			}},
			ForegroundSignatures: []ForegroundSignature{
				{ExecutableBase: "codex"},
				{ExecutableBase: "node", Argument1: "/home/niels/.local/bin/codex"},
			},
			Arguments: []string{"--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			Key:      "claude-work",
			Label:    "Claude · Work",
			Provider: ProviderClaude,
			Command:  "/home/niels/bin/claude-work",
			Environment: []EnvironmentVariable{{
				Name: "CLAUDE_CONFIG_DIR", Value: "/home/niels/.claude-work",
			}},
			ForegroundSignatures: []ForegroundSignature{{Argument0: "/home/niels/.local/bin/claude"}},
			Arguments:            []string{"--permission-mode", "auto"},
		},
	}
}
