package main

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/runtimeenv"
)

// This process already belongs to the new tmux pane. The shell consumes the
// requested directory after startup and remains available if entry fails.
func terminalExec(arguments []string) error {
	if len(arguments) != 3 {
		return errors.New("invalid terminal invocation")
	}
	if arguments[1] != "" {
		if _, err := agentruntime.DecodeLaunch(arguments[1]); err != nil {
			return err
		}
	}
	decodedCWD, err := base64.RawURLEncoding.Strict().DecodeString(arguments[0])
	if err != nil || base64.RawURLEncoding.EncodeToString(decodedCWD) != arguments[0] || !filepath.IsAbs(string(decodedCWD)) {
		return errors.New("invalid terminal directory")
	}
	decodedShell, err := base64.RawURLEncoding.Strict().DecodeString(arguments[2])
	if err != nil || base64.RawURLEncoding.EncodeToString(decodedShell) != arguments[2] || !filepath.IsAbs(string(decodedShell)) {
		return errors.New("invalid terminal shell")
	}
	shell := string(decodedShell)
	if _, err := exec.LookPath(shell); err != nil {
		return err
	}
	if base := filepath.Base(shell); base != "bash" && base != "zsh" {
		return errors.New("configured shell has no startup integration")
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
		"SKIDBLADNIR_STARTUP_PID="+strconv.Itoa(os.Getpid()),
		"SKIDBLADNIR_STARTUP_CWD="+string(decodedCWD),
		"SKIDBLADNIR_STARTUP_HELPER="+os.Args[0],
		"SKIDBLADNIR_STARTUP_AGENT="+arguments[1])
	return syscall.Exec(shell, []string{"-" + filepath.Base(shell)}, environment)
}
