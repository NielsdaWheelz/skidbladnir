package sessionui

import (
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// current is where the session's work runs: this host, a resolved remote host,
// or an unresolved remote connection.
func (m *model) current(row *fleetclient.Row) fleetclient.ExecutionContext {
	if !row.Available && row.Session.Connection != nil {
		return fleetclient.ExecutionContext{Kind: "remoteUnknown"}
	}
	for _, peer := range m.peers {
		if peer.Machine == row.Machine {
			return row.Session.Current(peer)
		}
	}
	return row.Session.Current(fleetclient.Peer{Label: row.Label, Machine: row.Machine})
}

// rowFooter discloses observed text only for a fresh local agent. A known agent
// without usable evidence keeps a dash; terminals and unknown remotes stay blank.
func (m *model) rowFooter(row fleetclient.Row) string {
	current := m.current(&row)
	if current.Agent == nil {
		return ""
	}
	if current.Kind == "local" && row.Available && row.Session.TerminalFooter != nil {
		return string(*row.Session.TerminalFooter)
	}
	return "—"
}

func (m *model) details(row *fleetclient.Row) [][2]string {
	value := row.Session
	current := m.current(row)
	facts := [][2]string{{"name", value.Name}, {"naming", value.NameMode}, {"group", fleetclient.GroupHeading(value.Group)}, {"selected pane", value.ActivePaneID}, {"terminal on", row.Label}, {"machine id", row.Machine}, {"terminal handle", value.TerminalHandle}}
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
			profile = a.Profile
		}
		if profile == "" {
			profile = "profile unknown"
		}
		facts = append(facts, [2]string{"provider", a.Provider}, [2]string{"profile", profile})
		facts = append(facts, [2]string{"displayed model / effort", m.rowFooter(*row)})
		explanation := "native wording may include service tier."
		if current.Kind == "remote" {
			explanation = "remote terminal text is not observed"
		} else {
			prefix := "text from the sampled terminal. — means no usable observation is available."
			if row.Available && value.TerminalFooter == nil {
				prefix += " the footer may be hidden, clipped, ambiguous or unsupported."
			}
			explanation = prefix + " " + explanation
		}
		facts = append(facts, [2]string{"", explanation})
	}

	view := m.statusView(*row)
	facts = append(facts, [2]string{"state", view.Detail}, [2]string{"status reason", view.Reason})
	if value.LaunchProfile != "" {
		facts = append(facts, [2]string{"started with", value.LaunchProfile})
	}
	facts = append(facts, [2]string{"observed", row.ObservedAt}, [2]string{"reference", value.Ref.Encode()})
	if !row.Available {
		availability := "unavailable"
		if m.hostChecking(*row) {
			availability = "checking"
		}
		facts = append(facts, [2]string{"availability", availability})
	}
	return facts
}

func (m *model) rowForReference(ref fleetclient.Reference) *fleetclient.Row {
	for _, peer := range m.peers {
		for _, session := range peer.Sessions {
			if session.Ref.SessionEqual(ref) {
				row := peer.Row(session)
				return &row
			}
		}
	}
	return nil
}

// refreshInfo follows info's lifetime through each inventory. A lifetime that
// leaves inventory keeps its last observed facts and their time,
// labelled unavailable.
func (m *model) refreshInfo() {
	if row := m.rowForReference(m.pageRow.Session.Ref.Reference); row != nil {
		m.pageRow = *row
		return
	}
	m.pageRow.Available = false
}
