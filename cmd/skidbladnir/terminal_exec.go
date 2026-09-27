package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/runtimeenv"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// This process already belongs to the new tmux pane. The shell consumes the
// requested directory after startup and remains available if entry fails.
func terminalExec(arguments []string) error {
	if len(arguments) != 4 {
		return errors.New("invalid terminal invocation")
	}
	if arguments[3] != "" {
		if _, err := agentruntime.DecodeLaunch(arguments[3]); err != nil {
			return err
		}
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
	if base := filepath.Base(shell); base != "bash" && base != "zsh" {
		return errors.New("default shell has no startup integration")
	}
	if err := os.Setenv("SHELL", shell); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return errors.New("terminal home is unavailable")
	}
	environment := runtimeenv.WithoutLaunchContext(os.Environ())
	environment = append(environment,
		"SKIDBLADNIR_SHELL=1",
		"CODEX_HOME="+filepath.Join(home, ".codex"),
		"SKIDBLADNIR_STARTUP_PID="+strconv.Itoa(os.Getpid()),
		"SKIDBLADNIR_STARTUP_CWD="+paths[0],
		"SKIDBLADNIR_STARTUP_HELPER="+os.Args[0],
		"SKIDBLADNIR_STARTUP_AGENT="+arguments[3])
	return syscall.Exec(shell, []string{"-" + filepath.Base(shell)}, environment)
}
