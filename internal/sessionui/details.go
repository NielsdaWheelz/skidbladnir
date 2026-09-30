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
	facts := [][2]string{{"name", value.Name}, {"naming", value.NameMode}, {"group", fleetclient.GroupHeading(value.Group)}, {"selected pane", value.ActivePaneID}, {"terminal on", row.label}, {"machine id", row.machine}, {"terminal handle", value.TerminalHandle}}
	if value.ConversationHandle != "" {
		facts = append(facts, [2]string{"conversation handle", value.ConversationHandle})
	}
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
		[2]string{"attached clients", strconv.Itoa(value.AttachedClients)})
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
		facts = append(facts, [2]string{"recorded native conversation", value.Conversation.ConversationID + "; may differ from terminal"})
	}

	view := m.statusView(*row)
	facts = append(facts, [2]string{"state", view.Detail()}, [2]string{"status reason", view.Reason})
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

func (m *model) rowForReference(encoded string) *listedRow {
	target, _ := fleetclient.DecodeReference(encoded)
	for _, peer := range m.scopedPeers() {
		for _, session := range peer.Sessions {
			ref, _ := fleetclient.DecodeReference(session.Ref)
			if ref.SessionEqual(target) {
				return &listedRow{peer.Label, peer.Machine, session, peer.OK && m.scopeReady}
			}
		}
	}
	return nil
}

func (m *model) refreshInfo() {
	if row := m.rowForReference(m.pageRef); row != nil {
		m.facts = m.details(row)
		m.pageName = row.session.Name
		return
	}
	for index := range m.facts {
		if m.facts[index][0] == "state" {
			m.facts[index][1] = "unavailable"
		}
	}
}
