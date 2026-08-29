package agentruntime

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var environmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const maxProfileKeyTextLength = 32

type Provider string

const (
	ProviderCodex  Provider = "Codex"
	ProviderClaude Provider = "Claude"
)

func ParseProvider(value string) (Provider, error) {
	provider := Provider(value)
	switch provider {
	case ProviderCodex, ProviderClaude:
		return provider, nil
	default:
		return "", errors.New("provider is invalid")
	}
}

func (provider Provider) String() string { return string(provider) }

type ProfileKey string

func ParseProfileKey(value string) (ProfileKey, error) {
	if len(value) == 0 || len(value) > maxProfileKeyTextLength {
		return "", errors.New("profile key is invalid")
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' {
			continue
		}
		if index > 0 && (character >= '0' && character <= '9' || character == '_' || character == '-') {
			continue
		}
		return "", errors.New("profile key is invalid")
	}
	return ProfileKey(value), nil
}

type ForegroundSignature struct {
	ExecutableBase string
	Argument0      string
	Argument1      string
}

type EnvironmentVariable struct {
	Name  string
	Value string
}

type Profile struct {
	Key                  ProfileKey
	Label                string
	Provider             Provider
	Command              string
	Environment          []EnvironmentVariable
	ForegroundSignatures []ForegroundSignature
	Arguments            []string
}

func ValidateProfiles(profiles []Profile) ([]Profile, error) {
	if len(profiles) == 0 {
		return nil, errors.New("at least one profile is required")
	}
	validated := make([]Profile, 0, len(profiles))
	keys := make(map[ProfileKey]struct{}, len(profiles))
	labels := make(map[string]struct{}, len(profiles))
	homes := map[Provider]map[string]struct{}{
		ProviderCodex:  {},
		ProviderClaude: {},
	}
	type providerSignature struct {
		provider  Provider
		signature ForegroundSignature
	}
	allSignatures := make([]providerSignature, 0, len(profiles))

	for _, profile := range profiles {
		if _, err := ParseProfileKey(string(profile.Key)); err != nil {
			return nil, errors.New("profile key is invalid")
		}
		if _, found := keys[profile.Key]; found {
			return nil, errors.New("profile key is duplicated")
		}
		if !safeProfileLabel(profile.Label) {
			return nil, fmt.Errorf("profile %s label is invalid", profile.Key)
		}
		if _, found := labels[profile.Label]; found {
			return nil, errors.New("profile label is duplicated")
		}
		if _, err := ParseProvider(profile.Provider.String()); err != nil {
			return nil, fmt.Errorf("profile %s provider is invalid", profile.Key)
		}
		if !filepath.IsAbs(profile.Command) {
			return nil, fmt.Errorf("profile %s command must be absolute", profile.Key)
		}

		homeName := providerHomeEnvironment(profile.Provider)
		home := ""
		environmentNames := make(map[string]struct{}, len(profile.Environment))
		for _, variable := range profile.Environment {
			if !environmentPattern.MatchString(variable.Name) || !utf8.ValidString(variable.Value) || strings.ContainsRune(variable.Value, 0) {
				return nil, fmt.Errorf("profile %s environment is invalid", profile.Key)
			}
			if _, found := environmentNames[variable.Name]; found {
				return nil, fmt.Errorf("profile %s environment name is duplicated", profile.Key)
			}
			environmentNames[variable.Name] = struct{}{}
			if variable.Name == "CODEX_HOME" || variable.Name == "CLAUDE_CONFIG_DIR" {
				if variable.Name != homeName {
					return nil, fmt.Errorf("profile %s declares another provider's home", profile.Key)
				}
				home = variable.Value
			}
		}
		if home == "" || !filepath.IsAbs(home) {
			return nil, fmt.Errorf("profile %s provider home must be absolute", profile.Key)
		}
		if _, found := homes[profile.Provider][home]; found {
			return nil, fmt.Errorf("provider %s home is duplicated", profile.Provider)
		}

		if len(profile.ForegroundSignatures) == 0 {
			return nil, fmt.Errorf("profile %s has no foreground signature", profile.Key)
		}
		for _, signature := range profile.ForegroundSignatures {
			if signature.ExecutableBase == "" && signature.Argument0 == "" {
				return nil, fmt.Errorf("profile %s has no foreground process identity", profile.Key)
			}
			if signature.ExecutableBase != "" && (!utf8.ValidString(signature.ExecutableBase) || filepath.Base(signature.ExecutableBase) != signature.ExecutableBase || hasTerminalControl(signature.ExecutableBase)) {
				return nil, fmt.Errorf("profile %s has an invalid foreground executable", profile.Key)
			}
			if signature.Argument0 != "" && (!filepath.IsAbs(signature.Argument0) || !utf8.ValidString(signature.Argument0) || strings.ContainsRune(signature.Argument0, 0)) {
				return nil, fmt.Errorf("profile %s has an invalid foreground argument zero", profile.Key)
			}
			if !utf8.ValidString(signature.Argument1) || strings.ContainsRune(signature.Argument1, 0) {
				return nil, fmt.Errorf("profile %s has an invalid foreground argument", profile.Key)
			}
			allSignatures = append(allSignatures, providerSignature{provider: profile.Provider, signature: signature})
		}

		for _, argument := range profile.Arguments {
			if !utf8.ValidString(argument) || strings.ContainsRune(argument, 0) {
				return nil, fmt.Errorf("profile %s has an invalid argument", profile.Key)
			}
			if profile.Provider == ProviderClaude && claudeNameArgument(argument) {
				return nil, fmt.Errorf("profile %s arguments conflict with managed Claude name", profile.Key)
			}
		}

		validated = append(validated, cloneProfile(profile))
		keys[profile.Key] = struct{}{}
		labels[profile.Label] = struct{}{}
		homes[profile.Provider][home] = struct{}{}
	}

	for leftIndex, left := range allSignatures {
		for _, right := range allSignatures[leftIndex+1:] {
			if left.provider != right.provider && left.signature == right.signature {
				return nil, errors.New("foreground signatures overlap across providers")
			}
		}
	}
	return validated, nil
}

func CloneProfiles(profiles []Profile) []Profile {
	cloned := make([]Profile, len(profiles))
	for index, profile := range profiles {
		cloned[index] = cloneProfile(profile)
	}
	return cloned
}

func MatchProfileEnvironment(profiles []Profile, provider Provider, lookup func(string) (string, bool)) (ProfileKey, bool) {
	homeName := providerHomeEnvironment(provider)
	if homeName == "" {
		return "", false
	}
	home, found := lookup(homeName)
	if !found {
		return "", false
	}
	for _, profile := range profiles {
		if profile.Provider != provider {
			continue
		}
		for _, variable := range profile.Environment {
			if variable.Name == homeName && variable.Value == home {
				return profile.Key, true
			}
		}
	}
	return "", false
}

func LaunchArguments(profile Profile, tmuxName string) []string {
	arguments := make([]string, 0, len(profile.Arguments)+2)
	switch profile.Provider {
	case ProviderCodex:
	case ProviderClaude:
		arguments = append(arguments, "--name", tmuxName)
	default:
		panic("invalid validated profile provider")
	}
	return append(arguments, profile.Arguments...)
}

func cloneProfile(profile Profile) Profile {
	profile.Environment = append([]EnvironmentVariable(nil), profile.Environment...)
	profile.ForegroundSignatures = append([]ForegroundSignature(nil), profile.ForegroundSignatures...)
	profile.Arguments = append([]string(nil), profile.Arguments...)
	return profile
}

func providerHomeEnvironment(provider Provider) string {
	switch provider {
	case ProviderCodex:
		return "CODEX_HOME"
	case ProviderClaude:
		return "CLAUDE_CONFIG_DIR"
	default:
		return ""
	}
}

func safeProfileLabel(label string) bool {
	if strings.TrimSpace(label) == "" || !utf8.ValidString(label) || utf8.RuneCountInString(label) > 64 || !norm.NFC.IsNormalString(label) {
		return false
	}
	for _, value := range label {
		if forbiddenIdentityTextRune(value) {
			return false
		}
	}
	return true
}

func claudeNameArgument(argument string) bool {
	return argument == "-n" || argument == "--name" || strings.HasPrefix(argument, "--name=")
}

func hasTerminalControl(value string) bool {
	for _, character := range value {
		if isC0OrC1(character) {
			return true
		}
	}
	return false
}

func isC0OrC1(value rune) bool {
	return value >= 0 && value <= 0x1f || value >= 0x7f && value <= 0x9f
}
