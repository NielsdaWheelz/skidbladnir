package agentruntime

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

type Foreground struct {
	Provider      Provider
	PID           processinfo.PID
	StartIdentity processinfo.StartIdentity
}

type AgentRuntime struct {
	PaneID        string
	StartIdentity processinfo.StartIdentity
	Provider      Provider
	PID           processinfo.PID
	Profile       ProfileKey
}

func ValidateAgentRuntime(profiles []Profile, agent AgentRuntime) error {
	if _, err := ParseProvider(agent.Provider.String()); err != nil {
		return errors.New("agent runtime provider is invalid")
	}
	if agent.PID <= 0 {
		return errors.New("agent runtime pid is invalid")
	}
	if agent.Profile != "" {
		if _, err := ParseProfileKey(string(agent.Profile)); err != nil {
			return errors.New("agent runtime profile is invalid")
		}
		if !configuredProfileMatchesProvider(profiles, agent.Profile, agent.Provider) {
			return errors.New("agent runtime profile does not match configured provider")
		}
	}
	return nil
}
func ClassifyForeground(observation processinfo.Observation) (Foreground, bool) {
	if observation.PID <= 0 || !validStartIdentity(observation.StartIdentity) {
		return Foreground{}, false
	}
	var provider Provider
	switch observation.ExecutableBase() {
	case "codex":
		provider = ProviderCodex
	case "claude":
		provider = ProviderClaude
	}
	if home, err := os.UserHomeDir(); err == nil && !strings.HasSuffix(observation.Executable, " (deleted)") && filepath.Dir(observation.Executable) == canonicalHome(filepath.Join(home, ".local/share/claude/versions")) {
		provider = ProviderClaude
	}

	if provider == "" {
		return Foreground{}, false
	}
	return Foreground{Provider: provider, PID: observation.PID, StartIdentity: observation.StartIdentity}, true
}
func Project(observation processinfo.Observation) (AgentRuntime, bool) {
	foreground, found := ClassifyForeground(observation)
	if !found {
		return AgentRuntime{}, false
	}
	return AgentRuntime{Provider: foreground.Provider, PID: foreground.PID, StartIdentity: foreground.StartIdentity}, true
}

func configuredProfileMatchesProvider(profiles []Profile, key ProfileKey, provider Provider) bool {
	for _, profile := range profiles {
		if profile.Key == key && profile.Provider == provider {
			return true
		}
	}
	return false
}

func validStartIdentity(value processinfo.StartIdentity) bool {
	parsed, err := strconv.ParseUint(string(value), 10, 64)
	return err == nil && parsed > 0 && strconv.FormatUint(parsed, 10) == string(value)
}

func forbiddenIdentityTextRune(character rune) bool {
	return unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character) ||
		character == '\u2028' || character == '\u2029'
}
