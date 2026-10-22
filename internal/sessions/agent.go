package sessions

import (
	"errors"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
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

// observeAgent samples the pane's foreground once, recognizes a local provider
// and attributes its profile from profiles. A foreground change during the
// environment read fails the sample; another failed read leaves the profile
// unattributed.
func observeAgent(panePID processinfo.PID, profiles []agentruntime.Profile) (*agentruntime.Agent, *processinfo.Observation, error) {
	foreground, err := paneForeground(panePID)
	if err != nil || foreground == nil {
		return nil, nil, err
	}
	agent, found := agentruntime.Classify(*foreground)
	if !found {
		return nil, foreground, nil
	}
	environment, err := processinfo.ObserveForegroundEnvironment(panePID, *foreground)
	if errors.Is(err, processinfo.ErrForegroundMismatch) {
		return nil, nil, err
	}
	if err == nil {
		agent.Profile = agentruntime.MatchProfile(profiles, agent.Provider, environment)
	}
	return &agent, foreground, nil
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

// connectionID is the connection's SKIDBLADNIR_CONNECTION when it is a valid
// connection id, else "".
func connectionID(environment map[string]string) string {
	if id := environment["SKIDBLADNIR_CONNECTION"]; terminalcontext.ValidConnectionID(id) {
		return id
	}
	return ""
}

// TerminalContext describes the foreground of a registered connection's
// terminal in one pass. Each read is bracketed by process identity; the reads
// together may mix moments, which a display-only lookup accepts. A foreground
// change during a read is unavailable; another failed read leaves its field
// empty. A remote-connection client reports only its connection, so its cwd is
// not read.
func (manager *Manager) TerminalContext(connection string) (TerminalContext, error) {
	root, err := terminalcontext.Observe(connection)
	if err != nil {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	foreground, err := processinfo.ObserveForeground(root.PID)
	if err != nil || foreground.SessionID != root.PID || foreground.TerminalDevice != root.TerminalDevice || foreground.ExecutableBase() == "tmux" || foreground.ExecutableBase() == "screen" {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	environment, err := processinfo.ObserveForegroundEnvironment(root.PID, foreground)
	if errors.Is(err, processinfo.ErrForegroundMismatch) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	if transport := connectionTransport(foreground); transport != "" {
		return TerminalContext{ObservedAt: time.Now().UTC(), Connection: &Connection{Transport: transport, ID: connectionID(environment)}}, nil
	}
	cwd, err := processinfo.ObserveCurrentDirectory(foreground.PID, foreground)
	if errors.Is(err, processinfo.ErrForegroundMismatch) {
		return TerminalContext{}, terminalcontext.ErrUnavailable
	}
	result := TerminalContext{ObservedAt: time.Now().UTC(), CWD: cwd}
	if agent, found := agentruntime.Classify(foreground); found {
		result.Agent = &RemoteAgent{Provider: agent.Provider, Profile: agentruntime.MatchProfile(manager.Profiles(), agent.Provider, environment)}
	}
	return result, nil
}
