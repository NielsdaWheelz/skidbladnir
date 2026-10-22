package agentruntime

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"golang.org/x/text/unicode/norm"
)

type Provider string

const (
	ProviderCodex  Provider = "Codex"
	ProviderClaude Provider = "Claude"
)

func (provider Provider) String() string { return string(provider) }

type ProfileKey string

type Profile struct {
	Key      ProfileKey
	Label    string
	Command  string
	Provider Provider
	Home     string // canonical; set exactly when Provider is
}

// ResolveProfiles is the one constructor and the one validation of profiles.
// configured is hostconfig.Resolve output, so provider homes are defaulted,
// expanded and absolute. It repeats hostconfig's checks because a member adopts
// its hub's settings without parsing them, and it is stricter on labels, which
// clients render. Command text stays opaque: it never names a native executable.
func ResolveProfiles(configured []hostconfig.Profile) ([]Profile, error) {
	keyPattern := regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	profiles := make([]Profile, 0, len(configured))
	keys := make(map[ProfileKey]bool, len(configured))
	for _, item := range configured {
		profile := Profile{Key: ProfileKey(item.Name), Label: item.Label, Command: item.Command}
		switch item.Provider {
		case "":
		case "codex":
			profile.Provider = ProviderCodex
		case "claude":
			profile.Provider = ProviderClaude
		default:
			return nil, errors.New("profile provider is invalid")
		}
		if !keyPattern.MatchString(item.Name) {
			return nil, errors.New("profile key is invalid")
		}
		if keys[profile.Key] {
			return nil, errors.New("profile key is duplicated")
		}
		keys[profile.Key] = true
		if profile.Label == "" {
			profile.Label = item.Name
		}
		label := profile.Label
		if strings.TrimSpace(label) == "" || !utf8.ValidString(label) || utf8.RuneCountInString(label) > 64 || !norm.NFC.IsNormalString(label) ||
			strings.IndexFunc(label, func(r rune) bool {
				return unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || r == '\u2028' || r == '\u2029'
			}) >= 0 {
			return nil, fmt.Errorf("profile %s label is invalid", profile.Key)
		}
		if strings.TrimSpace(profile.Command) == "" || !utf8.ValidString(profile.Command) || strings.ContainsRune(profile.Command, 0) {
			return nil, fmt.Errorf("profile %s command is invalid", profile.Key)
		}
		if profile.Provider == "" && item.Home != "" {
			return nil, fmt.Errorf("profile %s home requires provider", profile.Key)
		}
		if profile.Provider != "" {
			profile.Home = canonicalHome(item.Home)
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// MatchProfile is the one same-provider profile whose canonical home the
// environment names (CODEX_HOME or CLAUDE_CONFIG_DIR, else HOME/.codex or
// HOME/.claude), or "" when none or several do.
func MatchProfile(profiles []Profile, provider Provider, environment map[string]string) ProfileKey {
	name, directory := "CODEX_HOME", ".codex"
	if provider == ProviderClaude {
		name, directory = "CLAUDE_CONFIG_DIR", ".claude"
	}
	home, found := environment[name]
	if !found {
		if !filepath.IsAbs(environment["HOME"]) {
			return ""
		}
		home = filepath.Join(environment["HOME"], directory)
	}
	if !filepath.IsAbs(home) {
		return ""
	}
	home = canonicalHome(home)
	var key ProfileKey
	for _, profile := range profiles {
		if profile.Provider == provider && profile.Home == home {
			if key != "" {
				return ""
			}
			key = profile.Key
		}
	}
	return key
}

func canonicalHome(home string) string {
	canonical, err := filepath.EvalSymlinks(home)
	if err == nil {
		return canonical
	}
	return filepath.Clean(home)
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
