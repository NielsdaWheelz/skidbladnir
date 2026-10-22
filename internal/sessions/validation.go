package sessions

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"syscall"
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

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("is not a regular file")
	}
	if err := syscall.Access(path, 1); err != nil {
		return fmt.Errorf("is not executable: %w", err)
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
