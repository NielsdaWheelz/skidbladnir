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
	environment, err := processinfo.ObserveForegroundEnvironment(panePID, *foreground)
	if errors.Is(err, processinfo.ErrForegroundMismatch) {
		return nil, nil, err
	}
	if err == nil {
		agent.Profile = observedProfile(manager.Profiles(), agent.Provider, environment)
	}
	agent.PaneID = paneID
	return &agent, foreground, nil
}

func observedProfile(profiles []agentruntime.Profile, provider agentruntime.Provider, environment map[string]string) agentruntime.ProfileKey {
	profile, _ := agentruntime.MatchProfileEnvironment(profiles, provider, func(name string) (string, bool) {
		value, present := environment[name]
		return value, present
	})
	return profile
}

// connectionTransport names the remote-connection client a foreground runs, or "".
func connectionTransport(foreground processinfo.Observation) string {
	switch foreground.ExecutableBase() {
	case "ssh":
		return "ssh"
	case "mosh-client":
		return "mosh"
	}
	return ""
}

// connectionID is the connection's SKIDBLADNIR_CONNECTION when it is 32
// lowercase hex digits, else "".
func connectionID(environment map[string]string) string {
	id := environment["SKIDBLADNIR_CONNECTION"]
	if len(id) != 32 {
		return ""
	}
	for _, character := range id {
		if character < '0' || character > '9' && (character < 'a' || character > 'f') {
			return ""
		}
	}
	return id
}
