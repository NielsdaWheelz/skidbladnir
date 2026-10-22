package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// terminalExec becomes the new tmux pane's shell. Its one producer is
// tmux.TerminalCommand in this binary: base64url cwd, the opaque agent launch
// (or ""), base64url shell, a shell hostconfig.Resolve validated. The shell's
// startup hook enters the directory and decodes the launch through agent-exec.
func terminalExec(arguments []string) error {
	if len(arguments) != 3 {
		return errors.New("invalid terminal invocation")
	}
	cwd, err := base64.RawURLEncoding.DecodeString(arguments[0])
	if err != nil {
		return err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(arguments[2])
	if err != nil {
		return err
	}
	shell := string(decoded)
	if err := os.Setenv("SHELL", shell); err != nil {
		return err
	}
	// Drop launch markers inherited from the pane's parent; this shell gets fresh ones below.
	environment := slices.DeleteFunc(os.Environ(), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return strings.HasPrefix(name, "SKIDBLADNIR_STARTUP_") || slices.Contains([]string{"SKIDBLADNIR_SHELL",
			"SKIDBLADNIR_AGENT", "SKIDBLADNIR_CLAUDE_COMMAND", "SKIDBLADNIR_CONNECTION", "SKIDBLADNIR_TERMINAL_CONTEXT"}, name)
	})
	environment = append(environment,
		"SKIDBLADNIR_SHELL=1",
		"SKIDBLADNIR_STARTUP_PID="+strconv.Itoa(os.Getpid()),
		"SKIDBLADNIR_STARTUP_CWD="+string(cwd),
		"SKIDBLADNIR_STARTUP_HELPER="+os.Args[0],
		"SKIDBLADNIR_STARTUP_AGENT="+arguments[1])
	return syscall.Exec(shell, []string{"-" + filepath.Base(shell)}, environment)
}
