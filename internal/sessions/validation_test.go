package sessions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

func TestCreateRevalidatesWorkingDirectoryAfterPreflightBeforeTmuxCreate(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	cwd := filepath.Join(home, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("create working directory fixture: %v", err)
	}
	service, err := workdir.New(home)
	if err != nil {
		t.Fatalf("create working directory service: %v", err)
	}

	createMarker := filepath.Join(root, "create-invoked")
	t.Setenv("SKIDBLADNIR_REVALIDATION_CWD", cwd)
	t.Setenv("SKIDBLADNIR_REVALIDATION_CREATE_MARKER", createMarker)
	fakeTmux := filepath.Join(root, "fake-tmux")
	fakeTmuxProgram := `#!/bin/sh
set -u
case "$1" in
list-sessions)
  rmdir "$SKIDBLADNIR_REVALIDATION_CWD" || exit 90
  ;;
new-session)
  : > "$SKIDBLADNIR_REVALIDATION_CREATE_MARKER" || exit 91
  exit 92
  ;;
*)
  exit 93
  ;;
esac
`
	if err := os.WriteFile(fakeTmux, []byte(fakeTmuxProgram), 0o700); err != nil {
		t.Fatalf("write tmux test double: %v", err)
	}
	cataloguePath := filepath.Join(root, "characters.json")
	if err := os.WriteFile(
		cataloguePath,
		[]byte(`[{"key":"test.character","displayName":"Test Character"}]`),
		0o600,
	); err != nil {
		t.Fatalf("write character catalogue fixture: %v", err)
	}

	manager, err := New(Config{
		TmuxPath:      fakeTmux,
		Workdir:       service,
		CataloguePath: cataloguePath,
		Profiles: []agentruntime.Profile{{
			Key:      "personal",
			Label:    "Codex Personal",
			Provider: agentruntime.ProviderCodex,
			Command:  fakeTmux,
			Environment: []agentruntime.EnvironmentVariable{{
				Name: "CODEX_HOME", Value: home,
			}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "codex"}},
		}},
	})
	if err != nil {
		t.Fatalf("create session manager: %v", err)
	}

	_, createErr := manager.Create(context.Background(), CreateInput{
		CWD: cwd, Profile: "personal", OptionalTmuxName: "revalidation-test",
	})
	var sessionError *Error
	invokedCreate := false
	if _, err := os.Stat(createMarker); err == nil {
		invokedCreate = true
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect tmux create marker: %v", err)
	}
	if !errors.As(createErr, &sessionError) ||
		sessionError.Code != ErrorWorkingDirectoryUnavailable || invokedCreate {
		t.Fatalf(
			"Create error = %v, tmux create invoked = %t; want %s before tmux create",
			createErr, invokedCreate, ErrorWorkingDirectoryUnavailable,
		)
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
