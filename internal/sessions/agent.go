package sessions

import (
	"context"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

func (manager *Manager) observeAgent(ctx context.Context, paneID string, panePID processinfo.PID) (*agentruntime.AgentRuntime, *processinfo.Observation) {
	// An empty tmux pane has no process to observe or registered identity to accept.
	if panePID == 0 {
		return nil, nil
	}
	registration := ""
	if observedRegistration, optionErr := manager.paneOption(ctx, paneID, agentruntime.PaneOption); optionErr == nil {
		registration = observedRegistration
	}
	// justify-ignore-error: an exited or unstable foreground process omits optional agent identity.
	foreground, err := processinfo.ObserveForeground(panePID)
	if err != nil {
		return nil, nil
	}
	agent, found := agentruntime.Project(manager.profiles, foreground, registration)
	if !found {
		return nil, &foreground
	}
	agent.PaneID = paneID
	return &agent, &foreground
}
