package sessions

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func deriveAgent(
	profiles []agentruntime.Profile,
	observed processinfo.Observation,
	registration string,
) *agentruntime.AgentRuntime {
	agent, found := agentruntime.Project(profiles, observed, registration)
	if !found {
		return nil
	}
	return &agent
}
