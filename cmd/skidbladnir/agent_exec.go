package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/runtimeenv"
)

// The new pane may inherit environment from an already running tmux server.
// Clear the other product's context at the last boundary before the provider.
func agentExec(encoded []string) error {
	if len(encoded) < 3 {
		return errors.New("invalid agent invocation")
	}
	arguments := make([]string, len(encoded))
	for index, value := range encoded {
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value || strings.ContainsRune(string(decoded), 0) {
			return errors.New("invalid agent invocation")
		}
		arguments[index] = string(decoded)
	}
	command, homeName, home := arguments[0], arguments[1], arguments[2]
	if !filepath.IsAbs(command) || !filepath.IsAbs(home) || (homeName != "CODEX_HOME" && homeName != "CLAUDE_CONFIG_DIR") {
		return errors.New("invalid agent invocation")
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range runtimeenv.WithoutHerdr(os.Environ()) {
		name, _, _ := strings.Cut(entry, "=")
		if name != "CODEX_HOME" && name != "CLAUDE_CONFIG_DIR" && name != "SKIDBLADNIR_SHELL" && name != "SKIDBLADNIR_CLAUDE_COMMAND" {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, homeName+"="+home)
	return syscall.Exec(command, append([]string{command}, arguments[3:]...), environment)
}
