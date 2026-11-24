package sessionui

import (
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
)

// current is where the session's work runs: this host, a resolved remote host,
// or an unresolved remote connection.
func (m *model) current(row *fleetclient.Row) fleetclient.ExecutionContext {
	return row.Session.Current(m.peers[m.peerIndex(row.Machine)])
}

// rowFooter discloses observed text only for a fresh local agent. A known agent
// without usable evidence keeps a dash; terminals and unknown remotes stay blank.
func (m *model) rowFooter(row fleetclient.Row) string {
	current := m.current(&row)
	if current.Agent == nil {
		return ""
	}
	if current.Kind == "local" && row.Session.TerminalFooter != nil {
		return string(*row.Session.TerminalFooter)
	}
	return "—"
}

func (m *model) details(row *fleetclient.Row) [][2]string {
	value := row.Session
	current := m.current(row)
	view := m.statusView(*row)
	availability := "available"
	if !m.rowAvailable(*row) {
		availability = "unavailable"
		if m.rowChecking(*row) {
			availability = "checking"
		}
	}
	machine, cwd := current.Label, current.CWD
	if current.Kind == "remoteUnknown" {
		machine = "remote context unknown"
	}
	if cwd == "" {
		cwd = "directory unavailable"
	}
	facts := [][2]string{
		{"state", view.State}, {"availability", availability}, {"name", value.Name},
		{"running on", machine}, {"directory", cwd}, {"terminal handle", value.TerminalHandle},
		{"naming", value.NameMode}, {"group", fleetview.GroupHeading(value.Group)},
		{"selected pane", value.ActivePaneID}, {"terminal on", row.Label}, {"machine id", row.Machine},
		{"command", value.ActiveCommand}, {"attached clients", strconv.Itoa(value.AttachedClients)},
	}
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

	facts = append(facts, [2]string{"status reason", view.Reason})
	if value.LaunchProfile != "" {
		facts = append(facts, [2]string{"started with", value.LaunchProfile})
	}
	facts = append(facts, [2]string{"observed", row.ObservedAt}, [2]string{"reference", value.Ref.Encode()})
	return facts
}

func (m *model) rowForReference(ref fleetclient.Reference) *fleetclient.Row {
	index := m.peerIndex(ref.Machine)
	if index < 0 {
		return nil
	}
	peer := m.peers[index]
	for _, session := range peer.Sessions {
		if session.Ref.SessionEqual(ref) {
			row := peer.Row(session)
			return &row
		}
	}
	return nil
}
