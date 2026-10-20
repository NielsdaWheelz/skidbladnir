package runtimeenv

import "strings"

// WithoutLaunchContext removes only skid private inherited launch markers.
func WithoutLaunchContext(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "SKIDBLADNIR_STARTUP_") {
			continue
		}
		switch name {
		case "SKIDBLADNIR_SHELL", "SKIDBLADNIR_AGENT", "SKIDBLADNIR_CLAUDE_COMMAND", "SKIDBLADNIR_CONNECTION", "SKIDBLADNIR_TERMINAL_CONTEXT":
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
