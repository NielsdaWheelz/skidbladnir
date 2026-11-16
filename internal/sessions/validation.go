package sessions

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
	"golang.org/x/text/unicode/norm"
)

var (
	sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

func validateTmuxName(name string) error {
	if !sessionNamePattern.MatchString(name) {
		return newSessionError(ErrorSessionNameInvalid, "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.")
	}
	return nil
}

func validateObjective(objective string) error {
	if objective == "" {
		return nil
	}
	if !utf8.ValidString(objective) || utf8.RuneCountInString(objective) > 240 || !norm.NFC.IsNormalString(objective) {
		return newSessionError(ErrorObjectiveInvalid, "Use 1–240 characters without terminal controls.")
	}
	for _, value := range objective {
		if isC0OrC1(value) || value == '\u2028' || value == '\u2029' || value >= '\u202a' && value <= '\u202e' || value >= '\u2066' && value <= '\u2069' {
			return newSessionError(ErrorObjectiveInvalid, "Use 1–240 characters without terminal controls.")
		}
	}
	return nil
}

func isC0OrC1(value rune) bool {
	return value >= 0 && value <= 0x1f || value >= 0x7f && value <= 0x9f
}

func newSessionError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func (manager *Manager) validateCreate(input CreateInput) (string, agentruntime.Profile, error) {
	cwd, err := manager.workdir.ParseStart(input.CWD)
	if err != nil {
		return "", agentruntime.Profile{}, mapWorkingDirectoryError(err)
	}
	if err := workdir.CheckStart(cwd); err != nil {
		return "", agentruntime.Profile{}, mapWorkingDirectoryError(err)
	}
	var profile agentruntime.Profile
	switch input.Kind {
	case LaunchAgent:
		var found bool
		profile, found = manager.Profile(agentruntime.ProfileKey(input.Profile))
		if !found {
			return "", agentruntime.Profile{}, newSessionError(ErrorProfileUnknown, "Choose an available profile.")
		}
	case LaunchTerminal:
		if input.Model != nil || input.Effort != nil {
			return "", agentruntime.Profile{}, newSessionError(ErrorLaunchOptionsInvalid, "terminal launches do not accept model or effort options.")
		}
		if input.Profile != "" {
			panic("terminal launch carries agent options") // justify-defect: creation ingress forbids terminal agent options.
		}
	default:
		panic("unknown launch kind") // justify-defect: creation ingress admits the closed launch union.
	}
	if input.OptionalTmuxName != "" {
		if err := validateTmuxName(input.OptionalTmuxName); err != nil {
			return "", agentruntime.Profile{}, err
		}
	}
	if err := validateObjective(input.Objective); err != nil {
		return "", agentruntime.Profile{}, err
	}

	return cwd, profile, nil
}

// launchCommand keeps configured shell text unchanged unless native options
// require its single forwarding command to receive literal arguments.
func launchCommand(profile agentruntime.Profile, model, effort *string) (string, error) {
	if model == nil && effort == nil {
		return profile.Command, nil
	}
	for _, option := range []struct {
		name    string
		value   *string
		maximum int
	}{{"model", model, 255}, {"effort", effort, 64}} {
		if option.value == nil {
			continue
		}
		value := *option.value
		if value == "" || len(value) > option.maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) {
			return "", newSessionError(ErrorLaunchOptionsInvalid, "use a "+option.name+" of 1–"+strconv.Itoa(option.maximum)+" UTF-8 bytes, without controls or edge whitespace.")
		}
	}
	if profile.Provider != agentruntime.ProviderCodex && profile.Provider != agentruntime.ProviderClaude {
		return "", newSessionError(ErrorLaunchOptionsInvalid, "model and effort options require a codex or claude profile.")
	}
	if profile.Command == "" || strings.ContainsAny(profile.Command, "\\\"'`$;&|<>(){}[]*?!~=#") || strings.ContainsFunc(profile.Command, func(value rune) bool {
		return unicode.IsSpace(value) || unicode.IsControl(value)
	}) {
		return "", newSessionError(ErrorLaunchOptionsInvalid, "model and effort options require one plain profile command that forwards native provider arguments.")
	}
	var arguments []string
	if model != nil {
		arguments = append(arguments, "--model", *model)
	}
	if effort != nil {
		if profile.Provider == agentruntime.ProviderCodex {
			// A JSON string is a TOML basic string; encoding happens before shell quoting.
			encoded, _ := json.Marshal(*effort)
			arguments = append(arguments, "-c", "model_reasoning_effort="+string(encoded))
		} else {
			arguments = append(arguments, "--effort", *effort)
		}
	}
	for index, argument := range arguments {
		arguments[index] = "'" + strings.ReplaceAll(argument, "'", "'\\''") + "'"
	}
	return profile.Command + " " + strings.Join(arguments, " "), nil
}
