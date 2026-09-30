package runtimeenv

import "strings"

// WithoutLaunchContext clears account and terminal startup data before a new
// shell or foreground provider receives its own environment.
func WithoutLaunchContext(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "SKIDBLADNIR_STARTUP_") {
			continue
		}
		switch name {
		case "CODEX_HOME", "CLAUDE_CONFIG_DIR", "SKIDBLADNIR_SHELL", "SKIDBLADNIR_AGENT", "SKIDBLADNIR_CLAUDE_COMMAND", "SKIDBLADNIR_CONNECTION", "SKIDBLADNIR_TERMINAL_CONTEXT":
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
