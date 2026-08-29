package sessions

import (
	"errors"
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
