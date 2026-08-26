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

func TestValidateProfilesAcceptsAnAbsoluteArgumentZeroSignature(t *testing.T) {
	profiles, err := validateProfiles([]Profile{{
		Key:     "claude-work",
		Label:   "Claude · Work",
		Command: "/home/niels/bin/claude-work",
		ForegroundSignatures: []ForegroundSignature{{
			Argument0: "/home/niels/.local/bin/claude",
		}},
	}})
	if err != nil {
		t.Fatalf("validate exact argument-zero signature: %v", err)
	}
	if got := profiles[0].ForegroundSignatures[0].Argument0; got != "/home/niels/.local/bin/claude" {
		t.Fatalf("validated argument zero = %q, want exact launcher path", got)
	}
}

func TestValidateProfilesRejectsInvalidArgumentZeroSignatures(t *testing.T) {
	for _, test := range []struct {
		name      string
		signature ForegroundSignature
	}{
		{name: "no process identity", signature: ForegroundSignature{Argument1: "--permission-mode"}},
		{name: "relative path", signature: ForegroundSignature{Argument0: "claude"}},
		{name: "invalid utf8", signature: ForegroundSignature{Argument0: string([]byte{'/', 0xff})}},
		{name: "nul", signature: ForegroundSignature{Argument0: "/home/niels/.local/bin/claude\x00other"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateProfiles([]Profile{{
				Key:                  "claude-work",
				Label:                "Claude · Work",
				Command:              "/home/niels/bin/claude-work",
				ForegroundSignatures: []ForegroundSignature{test.signature},
			}})
			if err == nil {
				t.Fatalf("validate foreground signature %+v: got nil error, want rejection", test.signature)
			}
		})
	}
}
