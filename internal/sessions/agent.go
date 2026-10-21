package sessions

import (
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
)

// paneForeground samples the foreground process of a pane's terminal. An empty
// pane (no root process), an exited root process and an exited foreground
// process-group leader have no foreground; other kernel failures remain
// distinguishable from a successful sample.
func paneForeground(panePID processinfo.PID) (*processinfo.Observation, error) {
	if panePID == 0 {
		return nil, nil
	}
	foreground, err := processinfo.ObserveForeground(panePID)
	if errors.Is(err, processinfo.ErrProcessAbsent) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &foreground, nil
}

func (manager *Manager) observeAgent(paneID string, panePID processinfo.PID) (*agentruntime.AgentRuntime, *processinfo.Observation, error) {
	foreground, err := paneForeground(panePID)
	if err != nil || foreground == nil {
		return nil, nil, err
	}
	agent, found := agentruntime.Project(*foreground)
	if !found {
		return nil, foreground, nil
	}
	{
		environment, readErr := processinfo.ObserveForegroundEnvironment(panePID, *foreground)
		if errors.Is(readErr, processinfo.ErrForegroundMismatch) {
			return nil, nil, readErr
		}
		if readErr == nil {
			agent.Profile = observedProfile(manager.Profiles(), agent.Provider, environment)
		}
	}
	agent.PaneID = paneID
	return &agent, foreground, nil
}

func observeRemoteAgent(foreground processinfo.Observation, environment map[string]string, profiles []agentruntime.Profile) *agentruntime.AgentRuntime {
	agent, found := agentruntime.Project(foreground)
	if !found {
		return nil
	}
	{
		agent.Profile = observedProfile(profiles, agent.Provider, environment)
	}
	return &agent
}

func observedProfile(profiles []agentruntime.Profile, provider agentruntime.Provider, environment map[string]string) agentruntime.ProfileKey {
	profile, _ := agentruntime.MatchProfileEnvironment(profiles, provider, func(name string) (string, bool) {
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
