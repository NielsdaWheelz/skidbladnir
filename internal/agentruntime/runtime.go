package agentruntime

import (
	"os"
	"path/filepath"
	"strings"

	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

// Agent is a recognized local provider process: what Classify proves from one
// foreground sample, plus the profile sessions attributes from its environment
// ("" until attributed; Classify never sets it).
type Agent struct {
	Provider      Provider
	PID           processinfo.PID
	StartIdentity processinfo.StartIdentity
	Profile       ProfileKey
}

// Classify recognizes a provider from one foreground sample: an executable
// named codex or claude, or one directly inside claude's canonical versions
// directory, which outranks the name. The directory is resolved per call, so
// one created after start is recognized.
func Classify(observation processinfo.Observation) (Agent, bool) {
	var provider Provider
	switch observation.ExecutableBase() {
	case "codex":
		provider = ProviderCodex
	case "claude":
		provider = ProviderClaude
	}
	if home, err := os.UserHomeDir(); err == nil && !strings.HasSuffix(observation.Executable, " (deleted)") && filepath.Dir(observation.Executable) == canonicalHome(filepath.Join(home, ".local/share/claude/versions")) {
		provider = ProviderClaude
	}
	if provider == "" {
		return Agent{}, false
	}
	return Agent{Provider: provider, PID: observation.PID, StartIdentity: observation.StartIdentity}, true
}
