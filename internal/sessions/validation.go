package sessions

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"syscall"
	"unicode/utf8"

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
