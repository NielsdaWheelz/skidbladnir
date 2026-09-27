package sessionui

import (
	"fmt"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) current(row *listedRow) fleetclient.ExecutionContext {
	if !row.available && row.session.Connection != nil {
		return fleetclient.ExecutionContext{Kind: "remoteUnknown"}
	}
	for _, peer := range m.peers {
		if peer.Machine == row.machine {
			return row.session.Current(peer)
		}
	}
	return row.session.Current(fleetclient.Peer{Label: row.label, Machine: row.machine})
}

func (m *model) details(row *listedRow) string {
	value := row.session
	current := m.current(row)
	cwd := current.CWD
	if cwd == "" {
		cwd = "directory unavailable"
	}
	details := fmt.Sprintf("session: %s\nterminal on: %s\nmachine id: %s\n", value.Name, row.label, row.machine)
	if current.Kind == "remote" {
		details += "running on: " + current.Label + "\n"
	}
	if current.Kind == "remoteUnknown" {
		details += "remote context unknown\n"
	} else {
		details += "directory: " + cwd + "\n"
	}
	details += fmt.Sprintf("command: %s\nattached clients: %d\n", value.ActiveCommand, value.AttachedClients)
	details += fleetclient.GroupHeading(value.Group) + "\n"
	if current.Kind != "remoteUnknown" && current.Agent == nil {
		details += "agent: not detected\n"
	} else if current.Agent != nil {
		a := current.Agent
		profile := a.Label
		if profile == "" {
			profile = "profile unknown"
		}
		details += fmt.Sprintf("provider: %s\nprofile: %s\nstate: %s\n", a.Provider, profile, a.State)
		if current.Kind == "local" && value.Agent != nil {
			details += fmt.Sprintf("read: %s; send: %s; interrupt: %s\n", value.Agent.Methods.Read, value.Agent.Methods.Send, value.Agent.Methods.Interrupt)
			if value.Agent.ProviderSession != nil {
				details += fmt.Sprintf("provider session: %s %s\n", value.Agent.ProviderSession.ID, value.Agent.ProviderSession.Name)
			}
		}
	}
	if value.LaunchProfile != "" {
		details += "started with: " + value.LaunchProfile + "\n"
	}
	for _, peer := range m.peers {
		if peer.Machine == row.machine {
			details += "observed: " + peer.ObservedAt + "\n"
			break
		}
	}
	details += "reference: " + value.Ref

	if !row.available {
		details += "\navailability: unavailable"
	}
	return details
}
