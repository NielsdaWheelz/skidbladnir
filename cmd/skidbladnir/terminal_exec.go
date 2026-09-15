package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

// This process already belongs to the new tmux pane. Fail before user code if
// the required cwd or configured shell disappeared after gateway validation.
func terminalExec(arguments []string) error {
	if len(arguments) != 3 {
		return errors.New("invalid terminal invocation")
	}
	paths := make([]string, 2)
	for index := range paths {
		decoded, err := base64.RawURLEncoding.DecodeString(arguments[index])
		if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != arguments[index] || !filepath.IsAbs(string(decoded)) {
			return errors.New("invalid terminal path")
		}
		paths[index] = string(decoded)
	}
	client, err := tmuxclient.New(paths[1], arguments[2])
	if err != nil {
		return err
	}
	shell, err := client.Output(context.Background(), "read-default-shell", "show-options", "-gv", "default-shell")
	if err != nil || !filepath.IsAbs(shell) {
		return errors.New("default shell is unavailable")
	}
	if _, err := exec.LookPath(shell); err != nil {
		return err
	}
	directories, err := workdir.New("/")
	if err != nil {
		return err
	}
	candidate, err := directories.ParseCandidate(paths[0])
	if err != nil {
		return err
	}
	directory, err := directories.ValidateStart(candidate)
	if err != nil {
		return err
	}
	if err := os.Chdir(directory.String()); err != nil {
		return err
	}
	if err := os.Setenv("PWD", directory.String()); err != nil {
		return err
	}
	if err := os.Setenv("SHELL", shell); err != nil {
		return err
	}
	return syscall.Exec(shell, []string{"-" + filepath.Base(shell)}, os.Environ())
}
