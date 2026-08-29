package agentruntime

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"golang.org/x/text/unicode/norm"
)

const (
	PaneOption = "@skid_agent_runtime"

	maxPIDTextLength                  = 19
	maxStartIdentityTextLength        = 20
	maxProviderTextLength             = len("Claude")
	maxProviderSessionIDLength        = 128
	maxEncodedProviderSessionIDLength = (maxProviderSessionIDLength*8 + 5) / 6
	maxEncodedRegistrationLength      = len("v1") + 5 + maxPIDTextLength + maxStartIdentityTextLength + maxProviderTextLength + maxProfileKeyTextLength + maxEncodedProviderSessionIDLength
)

type Foreground struct {
	Provider      Provider
	PID           processinfo.PID
	StartIdentity processinfo.StartIdentity
}

type AgentRuntime struct {
	Provider        Provider
	PID             processinfo.PID
	Profile         ProfileKey
	ProviderSession *ProviderSessionFacts
}

type ProviderSessionFacts struct {
	id   string
	name string
}

func NewProviderSessionFacts(id, name string) (*ProviderSessionFacts, error) {
	if err := validateProviderSessionFacts(id, name); err != nil {
		return nil, err
	}
	return &ProviderSessionFacts{id: id, name: name}, nil
}

func validateProviderSessionFacts(id, name string) error {
	if id == "" && name == "" {
		return errors.New("provider session facts are empty")
	}
	if id != "" && !validProviderSessionID(id) {
		return errors.New("provider session id is invalid")
	}
	if name != "" && !validProviderSessionName(name) {
		return errors.New("provider session name is invalid")
	}
	return nil
}

func (facts ProviderSessionFacts) ID() string   { return facts.id }
func (facts ProviderSessionFacts) Name() string { return facts.name }

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
	if agent.ProviderSession != nil {
		if err := validateProviderSessionFacts(agent.ProviderSession.id, agent.ProviderSession.name); err != nil {
			return err
		}
		if agent.Provider == ProviderCodex && agent.ProviderSession.name != "" {
			return errors.New("Codex provider session name is invalid")
		}
	}
	return nil
}

func ClassifyForeground(profiles []Profile, observation processinfo.Observation) (Foreground, bool) {
	if observation.PID <= 0 || !validStartIdentity(observation.StartIdentity) {
		return Foreground{}, false
	}
	var provider Provider
	for _, profile := range profiles {
		for _, signature := range profile.ForegroundSignatures {
			if !matchesSignature(observation, signature) {
				continue
			}
			if provider != "" && provider != profile.Provider {
				return Foreground{}, false
			}
			provider = profile.Provider
		}
	}
	if provider == "" {
		return Foreground{}, false
	}
	return Foreground{Provider: provider, PID: observation.PID, StartIdentity: observation.StartIdentity}, true
}

func HookOrigin(
	profiles []Profile,
	ancestry []processinfo.Observation,
	paneTerminal processinfo.TerminalDevice,
) (Foreground, bool) {
	terminalAncestry, valid := paneTerminalAncestry(ancestry, paneTerminal)
	if !valid {
		return Foreground{}, false
	}
	type match struct {
		foreground  Foreground
		observation processinfo.Observation
	}
	matches := make([]match, 0, 2)
	for _, observation := range terminalAncestry {
		if foreground, found := ClassifyForeground(profiles, observation); found {
			matches = append(matches, match{foreground: foreground, observation: observation})
		}
	}

	var origin match
	switch len(matches) {
	case 1:
		origin = matches[0]
	case 2:
		native, wrapper := matches[0], matches[1]
		if native.foreground.Provider != ProviderCodex || wrapper.foreground.Provider != ProviderCodex ||
			native.observation.ExecutableBase() != "codex" || wrapper.observation.ExecutableBase() != "node" ||
			native.observation.ParentPID != wrapper.observation.PID {
			return Foreground{}, false
		}
		origin = wrapper
	default:
		return Foreground{}, false
	}
	if origin.foreground.PID != terminalAncestry[0].ForegroundProcessGroup {
		return Foreground{}, false
	}
	return origin.foreground, true
}

func EncodeRegistration(foreground Foreground, profile ProfileKey, providerSessionID string) (string, error) {
	if foreground.PID <= 0 || !validStartIdentity(foreground.StartIdentity) {
		return "", errors.New("foreground lifetime is invalid")
	}
	if _, err := ParseProvider(foreground.Provider.String()); err != nil {
		return "", errors.New("foreground provider is invalid")
	}
	profileValue := "-"
	if profile != "" {
		if _, err := ParseProfileKey(string(profile)); err != nil {
			return "", errors.New("runtime profile is invalid")
		}
		profileValue = string(profile)
	}
	if !validProviderSessionID(providerSessionID) {
		return "", errors.New("provider session id is invalid")
	}
	return strings.Join([]string{
		"v1",
		strconv.Itoa(int(foreground.PID)),
		string(foreground.StartIdentity),
		foreground.Provider.String(),
		profileValue,
		base64.RawURLEncoding.EncodeToString([]byte(providerSessionID)),
	}, ":"), nil
}

func Project(profiles []Profile, observation processinfo.Observation, encodedRegistration string) (AgentRuntime, bool) {
	foreground, found := ClassifyForeground(profiles, observation)
	if !found {
		return AgentRuntime{}, false
	}
	agent := AgentRuntime{Provider: foreground.Provider, PID: foreground.PID}
	providerSessionID := ""
	if registration, valid := acceptRegistration(profiles, foreground, encodedRegistration); valid {
		agent.Profile = registration.profile
		providerSessionID = registration.providerSessionID
	}
	providerSessionName := ""
	if foreground.Provider == ProviderClaude {
		providerSessionName = claudeName(observation.Argv)
	}
	if providerSessionID != "" || providerSessionName != "" {
		facts, err := NewProviderSessionFacts(providerSessionID, providerSessionName)
		if err != nil {
			panic("validated provider session facts became invalid")
		}
		agent.ProviderSession = facts
	}
	if err := ValidateAgentRuntime(profiles, agent); err != nil {
		panic("validated agent runtime became invalid")
	}
	return agent, true
}

type registration struct {
	pid               processinfo.PID
	startIdentity     processinfo.StartIdentity
	provider          Provider
	profile           ProfileKey
	providerSessionID string
}

func acceptRegistration(profiles []Profile, foreground Foreground, encoded string) (registration, bool) {
	parsed, valid := parseRegistration(encoded)
	if !valid || parsed.pid != foreground.PID || parsed.startIdentity != foreground.StartIdentity || parsed.provider != foreground.Provider {
		return registration{}, false
	}
	if parsed.profile != "" {
		if !configuredProfileMatchesProvider(profiles, parsed.profile, parsed.provider) {
			return registration{}, false
		}
	}
	return parsed, true
}

func parseRegistration(encoded string) (registration, bool) {
	if len(encoded) > maxEncodedRegistrationLength {
		return registration{}, false
	}
	fields := strings.Split(encoded, ":")
	if len(fields) != 6 || fields[0] != "v1" {
		return registration{}, false
	}
	pid, err := strconv.Atoi(fields[1])
	if err != nil || pid <= 0 || strconv.Itoa(pid) != fields[1] || !validStartIdentity(processinfo.StartIdentity(fields[2])) {
		return registration{}, false
	}
	provider, err := ParseProvider(fields[3])
	if err != nil {
		return registration{}, false
	}
	var profile ProfileKey
	if fields[4] != "-" {
		parsedProfile, err := ParseProfileKey(fields[4])
		if err != nil {
			return registration{}, false
		}
		profile = parsedProfile
	}
	if len(fields[5]) > maxEncodedProviderSessionIDLength {
		return registration{}, false
	}
	providerSessionID, err := base64.RawURLEncoding.DecodeString(fields[5])
	if err != nil || base64.RawURLEncoding.EncodeToString(providerSessionID) != fields[5] || !validProviderSessionID(string(providerSessionID)) {
		return registration{}, false
	}
	return registration{
		pid:               processinfo.PID(pid),
		startIdentity:     processinfo.StartIdentity(fields[2]),
		provider:          provider,
		profile:           profile,
		providerSessionID: string(providerSessionID),
	}, true
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

func paneTerminalAncestry(ancestry []processinfo.Observation, paneTerminal processinfo.TerminalDevice) ([]processinfo.Observation, bool) {
	if paneTerminal == 0 {
		return nil, false
	}
	start := -1
	for index, observation := range ancestry {
		if observation.TerminalDevice == paneTerminal {
			start = index
			break
		}
	}
	if start < 0 || ancestry[start].SessionID <= 0 || ancestry[start].ForegroundProcessGroup <= 0 {
		return nil, false
	}
	session := ancestry[start].SessionID
	foreground := ancestry[start].ForegroundProcessGroup
	for index := start; index < len(ancestry); index++ {
		observation := ancestry[index]
		if observation.TerminalDevice != paneTerminal || observation.SessionID != session || observation.ForegroundProcessGroup != foreground {
			return nil, false
		}
		if observation.PID == session {
			return ancestry[start : index+1], true
		}
	}
	return nil, false
}

func matchesSignature(observation processinfo.Observation, signature ForegroundSignature) bool {
	return (signature.ExecutableBase == "" || observation.ExecutableBase() == signature.ExecutableBase) &&
		(signature.Argument0 == "" || observation.Argument(0) == signature.Argument0) &&
		(signature.Argument1 == "" || observation.Argument(1) == signature.Argument1)
}

func claudeName(argv []string) string {
	name := ""
	found := false
	for index := 1; index < len(argv); index++ {
		argument := argv[index]
		if argument == "--" {
			break
		}
		candidate := ""
		switch {
		case argument == "-n" || argument == "--name":
			if index+1 >= len(argv) {
				return ""
			}
			index++
			candidate = argv[index]
		case strings.HasPrefix(argument, "--name="):
			candidate = strings.TrimPrefix(argument, "--name=")
		default:
			continue
		}
		if found || !validProviderSessionName(candidate) {
			return ""
		}
		name, found = candidate, true
	}
	return name
}

func validProviderSessionID(value string) bool {
	if len(value) == 0 || len(value) > maxProviderSessionIDLength {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validProviderSessionName(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > 128 || !norm.NFC.IsNormalString(value) {
		return false
	}
	for _, character := range value {
		if forbiddenIdentityTextRune(character) {
			return false
		}
	}
	return true
}

func forbiddenIdentityTextRune(character rune) bool {
	return unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character) ||
		character == '\u2028' || character == '\u2029'
}
