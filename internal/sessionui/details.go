package sessionui

import (
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// current is where the session's work runs: this host, a resolved remote host,
// or an unresolved remote connection.
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

func (m *model) details(row *listedRow) [][2]string {
	value := row.session
	current := m.current(row)
	facts := [][2]string{{"session", value.Name}, {"terminal on", row.label}, {"machine id", row.machine}}
	switch current.Kind {
	case "remote":
		facts = append(facts, [2]string{"running on", current.Label})
	case "remoteUnknown":
		facts = append(facts, [2]string{"running on", "remote context unknown"})
	}
	if current.Kind != "remoteUnknown" {
		cwd := current.CWD
		if cwd == "" {
			cwd = "directory unavailable"
		}
		facts = append(facts, [2]string{"directory", cwd})
	}
	facts = append(facts, [2]string{"command", value.ActiveCommand},
		[2]string{"attached clients", strconv.Itoa(value.AttachedClients)},
		[2]string{"membership", fleetclient.GroupHeading(value.Group)})
	if current.Agent == nil && current.Kind != "remoteUnknown" {
		facts = append(facts, [2]string{"agent", "not detected"})
	}
	if a := current.Agent; a != nil {
		profile := a.Label
		if profile == "" {
			profile = "profile unknown"
		}
		facts = append(facts, [2]string{"provider", a.Provider}, [2]string{"profile", profile})
		if current.Kind == "local" && value.Agent != nil {
			local := value.Agent
			if local.ProviderSession != nil {
				facts = append(facts, [2]string{"provider session", local.ProviderSession.ID + " " + local.ProviderSession.Name})
			}
		}
	}
	if value.Conversation != nil {
		facts = append(facts, [2]string{"tracking", value.Conversation.Binding.Conversation.ConversationID}, [2]string{"state", fleetclient.StatusText(value.Conversation.Status)})
	}

	if value.LaunchProfile != "" {
		facts = append(facts, [2]string{"started with", value.LaunchProfile})
	}
	for _, peer := range m.peers {
		if peer.Machine == row.machine {
			facts = append(facts, [2]string{"observed", peer.ObservedAt})
			break
		}
	}
	facts = append(facts, [2]string{"reference", value.Ref})
	if !row.available {
		facts = append(facts, [2]string{"availability", "unavailable"})
	}
	return facts
}
