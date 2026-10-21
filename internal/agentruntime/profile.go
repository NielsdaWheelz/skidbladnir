package agentruntime

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"golang.org/x/text/unicode/norm"
)

const maxProfileKeyTextLength = 32

type Provider string

const (
	ProviderCodex  Provider = "Codex"
	ProviderClaude Provider = "Claude"
)

func ParseProvider(value string) (Provider, error) {
	switch provider := Provider(value); provider {
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
		c := value[index]
		if c >= 'a' && c <= 'z' || index > 0 && (c >= '0' && c <= '9' || c == '_' || c == '-') {
			continue
		}
		return "", errors.New("profile key is invalid")
	}
	return ProfileKey(value), nil
}

type Profile struct {
	Key      ProfileKey
	Label    string
	Command  string
	Provider Provider
	Home     string
}

func ValidateProfiles(profiles []Profile) ([]Profile, error) {
	validated := make([]Profile, 0, len(profiles))
	keys := make(map[ProfileKey]bool, len(profiles))
	for _, profile := range profiles {
		if _, err := ParseProfileKey(string(profile.Key)); err != nil {
			return nil, err
		}
		if keys[profile.Key] {
			return nil, errors.New("profile key is duplicated")
		}
		keys[profile.Key] = true
		if profile.Label == "" {
			profile.Label = string(profile.Key)
		}
		if !safeProfileLabel(profile.Label) {
			return nil, fmt.Errorf("profile %s label is invalid", profile.Key)
		}
		if !validCommand(profile.Command) {
			return nil, fmt.Errorf("profile %s command is invalid", profile.Key)
		}
		if profile.Provider == "" {
			if profile.Home != "" {
				return nil, fmt.Errorf("profile %s home requires provider", profile.Key)
			}
		} else {
			if _, err := ParseProvider(profile.Provider.String()); err != nil {
				return nil, err
			}
			if profile.Home == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return nil, err
				}
				profile.Home = filepath.Join(home, providerDefaultDirectory(profile.Provider))
			}
			if !filepath.IsAbs(profile.Home) || !utf8.ValidString(profile.Home) || strings.ContainsRune(profile.Home, 0) {
				return nil, fmt.Errorf("profile %s provider home must be absolute", profile.Key)
			}
			profile.Home = canonicalHome(profile.Home)
		}

		validated = append(validated, profile)
	}
	return validated, nil
}
func CloneProfiles(profiles []Profile) []Profile {
	cloned := make([]Profile, len(profiles))
	copy(cloned, profiles)

	return cloned
}
func MatchProfileEnvironment(profiles []Profile, provider Provider, lookup func(string) (string, bool)) (ProfileKey, bool) {
	homeName := providerHomeEnvironment(provider)
	if homeName == "" {
		return "", false
	}
	home, found := lookup(homeName)
	if !found {
		userHome, present := lookup("HOME")
		if !present || !filepath.IsAbs(userHome) {
			return "", false
		}
		home = filepath.Join(userHome, providerDefaultDirectory(provider))
	}
	if !filepath.IsAbs(home) {
		return "", false
	}
	home = canonicalHome(home)
	var key ProfileKey
	for _, profile := range profiles {
		if profile.Provider == provider && profile.Home == home {
			if key != "" {
				return "", false
			}
			key = profile.Key
		}
	}
	return key, key != ""
}
func canonicalHome(home string) string {
	canonical, err := filepath.EvalSymlinks(home)
	if err == nil {
		return canonical
	}
	return filepath.Clean(home)
}
func providerDefaultDirectory(provider Provider) string {
	if provider == ProviderCodex {
		return ".codex"
	}
	return ".claude"
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
	for _, r := range label {
		if forbiddenIdentityTextRune(r) {
			return false
		}
	}
	return true
}
func CodexEndpoint(profile Profile) string {
	return filepath.Join(profile.Home, "app-server-control", "app-server-control.sock")
}
func AccountScope(profile Profile) (string, error) {
	if profile.Provider == "" || profile.Home == "" {
		return "", errors.New("profile omits account home")
	}
	home, err := filepath.EvalSymlinks(profile.Home)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(string(profile.Provider)+"\x00"+home))), nil
}

// ResolveProfiles translates accepted configuration into observation metadata.
// Command text remains opaque and never supplies a native executable path.
func ResolveProfiles(configured []hostconfig.Profile) ([]Profile, error) {
	profiles := make([]Profile, 0, len(configured))
	for _, item := range configured {
		profile := Profile{Key: ProfileKey(item.Name), Label: item.Label, Command: item.Command, Home: item.Home}
		switch item.Provider {
		case "":
		case "codex":
			profile.Provider = ProviderCodex
		case "claude":
			profile.Provider = ProviderClaude
		default:
			return nil, errors.New("profile provider is invalid")
		}
		if strings.HasPrefix(profile.Home, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			profile.Home = filepath.Join(home, strings.TrimPrefix(profile.Home, "~/"))
		}
		profiles = append(profiles, profile)
	}
	return ValidateProfiles(profiles)
}
