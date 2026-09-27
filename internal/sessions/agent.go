package sessions

import (
	"context"
	"errors"

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
	if agent.Provider == agentruntime.ProviderCodex && foreground.ExecutableBase() == "codex" {
		environment, readErr := processinfo.ObserveForegroundEnvironment(panePID, foreground)
		if errors.Is(readErr, processinfo.ErrForegroundMismatch) {
			return nil, nil
		}
		if readErr == nil {
			agent.Profile = codexProfile(manager.profiles, environment)
		}
	}
	agent.PaneID = paneID
	return &agent, &foreground
}

func observeRemoteAgent(foreground processinfo.Observation, environment map[string]string, profiles []agentruntime.Profile) *agentruntime.AgentRuntime {
	agent, found := agentruntime.Project(profiles, foreground, "")
	if !found {
		return nil
	}
	if agent.Provider == agentruntime.ProviderCodex && foreground.ExecutableBase() == "codex" {
		agent.Profile = codexProfile(profiles, environment)
	}
	return &agent
}

func codexProfile(profiles []agentruntime.Profile, environment map[string]string) agentruntime.ProfileKey {
	profile, _ := agentruntime.MatchProfileEnvironment(profiles, agentruntime.ProviderCodex, func(name string) (string, bool) {
		value, present := environment[name]
		return value, present
	})
	return profile
}

func observedTransport(foreground processinfo.Observation, environment map[string]string) (string, string, bool) {
	transport := ""
	switch foreground.ExecutableBase() {
	case "ssh":
		transport = "ssh"
	case "mosh-client":
		transport = "mosh"
	default:
		return "", "", false
	}
	id := environment["SKIDBLADNIR_CONNECTION"]
	if len(id) != 32 {
		return transport, "", true
	}
	for _, character := range id {
		if character < '0' || character > '9' && (character < 'a' || character > 'f') {
			return transport, "", true
		}
	}
	return transport, id, true
}
