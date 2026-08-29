package sessions

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeWorkingDirectoryExpandsTildeForms(t *testing.T) {
	const home = "/home/niels"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "home", input: "~", want: home},
		{name: "home slash", input: "~/", want: home},
		{name: "home child", input: "~/project with spaces", want: "/home/niels/project with spaces"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeWorkingDirectory(test.input, home)
			if err != nil {
				t.Fatalf("validate working directory: %v", err)
			}
			if got != test.want {
				t.Fatalf("expanded working directory = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeWorkingDirectoryRejectsRelativeTildeLookalike(t *testing.T) {
	_, err := normalizeWorkingDirectory("~other/project", "/home/niels")
	var sessionError *Error
	if !errors.As(err, &sessionError) || sessionError.Code != ErrorWorkingDirectoryInvalid {
		t.Fatalf("validate working directory error = %v, want %s", err, ErrorWorkingDirectoryInvalid)
	}
}

func TestValidateTmuxNameOwnsTheRequiredWireGrammar(t *testing.T) {
	for _, valid := range []string{"a", "A0_-", strings.Repeat("a", 64)} {
		if err := validateTmuxName(valid); err != nil {
			t.Fatalf("valid tmux name %q was rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "_leading", "-leading", "contains.dot", "a b", strings.Repeat("a", 65)} {
		err := validateTmuxName(invalid)
		var sessionError *Error
		if !errors.As(err, &sessionError) || sessionError.Code != ErrorSessionNameInvalid {
			t.Fatalf("invalid tmux name %q error = %v, want %s", invalid, err, ErrorSessionNameInvalid)
		}
	}
}
