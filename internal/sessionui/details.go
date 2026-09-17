package sessionui

import (
	"fmt"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) details(row *listedRow) string {
	value := row.session
	details := fmt.Sprintf("session: %s\nmachine: %s\nmachine id: %s\ndirectory: %s\ncommand: %s\nattached clients: %d\n", value.Name, row.label, row.machine, value.CWD, value.ActiveCommand, value.AttachedClients)
	details += fleetclient.SpaceHeading(value.Space) + "\n"
	if value.Agent == nil {
		details += "agent: none (shell)\n"
	} else {
		a := value.Agent
		details += fmt.Sprintf("provider: %s\nprofile: %s\nstate: %s (%s)\nreason: %s\nread: %s; send: %s; interrupt: %s\n", a.Provider, a.Profile, a.Status.State, a.Status.Source, a.Status.Reason, a.Methods.Read, a.Methods.Send, a.Methods.Interrupt)
		if a.ProviderSession != nil {
			details += fmt.Sprintf("provider session: %s %s\n", a.ProviderSession.ID, a.ProviderSession.Name)
		}
	}
	if value.LaunchProfile != "" {
		details += "launch profile: " + value.LaunchProfile + "\n"
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
