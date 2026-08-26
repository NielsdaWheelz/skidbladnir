package sessions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var (
	profileKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	environmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

func validateProfiles(profiles []Profile) ([]Profile, error) {
	if len(profiles) == 0 {
		return nil, errors.New("at least one profile is required")
	}
	validated := make([]Profile, 0, len(profiles))
	keys := make(map[string]struct{}, len(profiles))
	labels := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if !profileKeyPattern.MatchString(profile.Key) {
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
		if !filepath.IsAbs(profile.Command) {
			return nil, fmt.Errorf("profile %s command must be absolute", profile.Key)
		}
		environmentNames := make(map[string]struct{}, len(profile.Environment))
		for _, variable := range profile.Environment {
			if !environmentPattern.MatchString(variable.Name) || !utf8.ValidString(variable.Value) || strings.ContainsRune(variable.Value, 0) {
				return nil, fmt.Errorf("profile %s environment is invalid", profile.Key)
			}
			if _, found := environmentNames[variable.Name]; found {
				return nil, fmt.Errorf("profile %s environment name is duplicated", profile.Key)
			}
			environmentNames[variable.Name] = struct{}{}
		}
		if len(profile.ForegroundSignatures) == 0 {
			return nil, fmt.Errorf("profile %s has no foreground signature", profile.Key)
		}
		for _, signature := range profile.ForegroundSignatures {
			if signature.ExecutableBase == "" && signature.Argument0 == "" {
				return nil, fmt.Errorf("profile %s has no foreground process identity", profile.Key)
			}
			if signature.ExecutableBase != "" && (filepath.Base(signature.ExecutableBase) != signature.ExecutableBase || hasTerminalControl(signature.ExecutableBase)) {
				return nil, fmt.Errorf("profile %s has an invalid foreground executable", profile.Key)
			}
			if signature.Argument0 != "" && (!filepath.IsAbs(signature.Argument0) || !utf8.ValidString(signature.Argument0) || strings.ContainsRune(signature.Argument0, 0)) {
				return nil, fmt.Errorf("profile %s has an invalid foreground argument zero", profile.Key)
			}
			if !utf8.ValidString(signature.Argument1) || strings.ContainsRune(signature.Argument1, 0) {
				return nil, fmt.Errorf("profile %s has an invalid foreground argument", profile.Key)
			}
		}
		for _, argument := range profile.Arguments {
			if !utf8.ValidString(argument) || strings.ContainsRune(argument, 0) {
				return nil, fmt.Errorf("profile %s has an invalid argument", profile.Key)
			}
		}
		profile.Environment = append([]EnvironmentVariable(nil), profile.Environment...)
		profile.ForegroundSignatures = append([]ForegroundSignature(nil), profile.ForegroundSignatures...)
		profile.Arguments = append([]string(nil), profile.Arguments...)
		validated = append(validated, profile)
		keys[profile.Key] = struct{}{}
		labels[profile.Label] = struct{}{}
	}
	return validated, nil
}

func safeProfileLabel(label string) bool {
	if strings.TrimSpace(label) == "" || !utf8.ValidString(label) || utf8.RuneCountInString(label) > 64 || !norm.NFC.IsNormalString(label) {
		return false
	}
	for _, value := range label {
		if isC0OrC1(value) || value == '\u2028' || value == '\u2029' || value >= '\u202a' && value <= '\u202e' || value >= '\u2066' && value <= '\u2069' {
			return false
		}
	}
	return true
}

func validateWorkingDirectory(input, home string) (string, error) {
	path, err := normalizeWorkingDirectory(input, home)
	if err != nil {
		return "", err
	}
	if err := requireSearchableDirectory(path); err != nil {
		return "", newSessionError(ErrorWorkingDirectoryUnavailable, "That directory is unavailable.")
	}
	return path, nil
}

func normalizeWorkingDirectory(input, home string) (string, error) {
	if input == "" || len(input) > 4096 || !utf8.ValidString(input) || hasTerminalControl(input) {
		return "", newSessionError(ErrorWorkingDirectoryInvalid, "Use an absolute directory path or ~/… without terminal controls.")
	}
	path := input
	if input == "~" {
		path = home
	} else if strings.HasPrefix(input, "~/") {
		path = filepath.Join(home, input[2:])
	}
	if !filepath.IsAbs(path) {
		return "", newSessionError(ErrorWorkingDirectoryInvalid, "Use an absolute directory path or ~/… without terminal controls.")
	}
	return filepath.Clean(path), nil
}

func validateOptionalTmuxName(name string) error {
	if name != "" && !sessionNamePattern.MatchString(name) {
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

func requireSearchableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("is not a directory")
	}
	if err := syscall.Access(path, 1); err != nil {
		return fmt.Errorf("is not searchable: %w", err)
	}
	return nil
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

func newSessionError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}
