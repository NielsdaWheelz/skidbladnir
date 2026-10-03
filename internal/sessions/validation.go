package sessions

import (
	"context"
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

// PreflightCreate rejects known request failures before provider launch preparation.
func (manager *Manager) PreflightCreate(ctx context.Context, input CreateInput) (CreateInput, error) {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	cwd, profile, err := manager.validateCreate(input)
	if err != nil {
		return CreateInput{}, err
	}
	scan, err := manager.scanSessions(ctx)
	if err != nil {
		return CreateInput{}, err
	}
	if input.OptionalTmuxName == "" {
		prefix := string(profile.Key)
		if input.Kind == LaunchTerminal {
			prefix = "terminal"
		}
		input.preparedName = generatedTmuxName(scan.names, prefix)
	} else if _, exists := scan.names[input.OptionalTmuxName]; exists {
		return CreateInput{}, newSessionError(ErrorSessionNameConflict, "A tmux session already uses that name.")
	}
	input.CWD = cwd.String()
	return input, nil
}

func (manager *Manager) validateCreate(input CreateInput) (workdir.WorkingDirectory, agentruntime.Profile, error) {
	candidate, err := manager.workdir.ParseCandidate(input.CWD)
	if err != nil {
		return workdir.WorkingDirectory{}, agentruntime.Profile{}, mapWorkingDirectoryError(err)
	}
	cwd, err := manager.workdir.ValidateStart(candidate)
	if err != nil {
		return workdir.WorkingDirectory{}, agentruntime.Profile{}, mapWorkingDirectoryError(err)
	}
	var profile agentruntime.Profile
	switch input.Kind {
	case LaunchAgent:
		var found bool
		profile, found = manager.profilesByKey[agentruntime.ProfileKey(input.Profile)]
		if !found {
			return workdir.WorkingDirectory{}, agentruntime.Profile{}, newSessionError(ErrorProfileUnknown, "Choose an available profile.")
		}
		if err := agentruntime.ValidateLaunchOptions(input.Model, input.Effort); err != nil {
			return workdir.WorkingDirectory{}, agentruntime.Profile{}, err
		}
	case LaunchTerminal:
		if input.Profile != "" || input.Model != "" || input.Effort != "" {
			panic("terminal launch carries agent options") // justify-defect: creation ingress forbids terminal agent options.
		}
	default:
		panic("unknown launch kind") // justify-defect: creation ingress admits the closed launch union.
	}
	if input.OptionalTmuxName != "" {
		if err := validateTmuxName(input.OptionalTmuxName); err != nil {
			return workdir.WorkingDirectory{}, agentruntime.Profile{}, err
		}
	}
	if err := validateObjective(input.Objective); err != nil {
		return workdir.WorkingDirectory{}, agentruntime.Profile{}, err
	}

	return cwd, profile, nil
}
