package sessionui

import (
	"cmp"
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// attention keeps inferred waiting and idle ahead of unknown and working.
var attention = []string{"blocked", "idle", "unknown", "working"}

func sameSession(a, b fleetclient.Session) bool {
	left, _ := fleetclient.DecodeReference(a.Ref)
	right, _ := fleetclient.DecodeReference(b.Ref)
	return left.SessionEqual(right)
}

func (m *model) rebuild() {
	var selected fleetclient.Session
	if row := m.selectedRow(); row != nil {
		selected = row.session
	}
	previous := m.cursor
	m.rows = nil
	if m.agentsView {
		for _, peer := range m.scopedPeers() {
			for _, session := range peer.Sessions {
				row := listedRow{peer.Label, peer.Machine, session, peer.OK && m.scopeReady}
				if m.current(&row).Agent != nil {
					m.rows = append(m.rows, row)
				}
			}
		}
		slices.SortStableFunc(m.rows, func(a, b listedRow) int {
			if a.available != b.available {
				if a.available {
					return -1
				}
				return 1
			}
			return cmp.Compare(slices.Index(attention, statusState(a.session)), slices.Index(attention, statusState(b.session)))
		})
	} else {
		for _, group := range fleetclient.Groups(m.scopedPeers(), m.groupFilter) {
			for _, row := range group.Rows {
				m.rows = append(m.rows, listedRow{row.Label, row.Machine, row.Session, row.Available && m.scopeReady})
			}
		}
	}
	m.cursor = -1
	for index, row := range m.rows {
		if sameSession(row.session, selected) {
			m.cursor = index
			break
		}
	}
	if m.cursor < 0 && len(m.rows) > 0 {
		m.cursor = min(max(0, previous), len(m.rows)-1)
	}
}

// rebuildForFilter keeps the selected session when it survives a scope change;
// otherwise it selects the first row.
func (m *model) rebuildForFilter() {
	var selected fleetclient.Session
	if row := m.selectedRow(); row != nil {
		selected = row.session
	}
	m.rebuild()
	if row := m.selectedRow(); row != nil && !sameSession(row.session, selected) {
		m.cursor = 0
	}
	m.top = 0
}

// groupOptions is the view sequence after agents: all, then each group observed
// in scope plus the current one, in group order, so unassigned comes last.
// step walks it and header draws it, so drawn and stepped order are one.
func (m *model) groupOptions() []group.Filter {
	labels := []group.Label{}
	for _, observed := range fleetclient.Groups(m.scopedPeers(), group.Filter{}) {
		labels = append(labels, observed.Label)
	}
	if m.groupFilter.Kind() != group.FilterAll && !slices.Contains(labels, m.groupFilter.Label()) {
		labels = append(labels, m.groupFilter.Label())
		slices.SortFunc(labels, group.Compare)
	}
	options := []group.Filter{{}}
	for _, label := range labels {
		options = append(options, filterFor(label))
	}
	return options
}

// step moves along the strip's views: agents, all, then each group. the
// agents view always spans all groups.
func (m *model) step(delta int) {
	options := m.groupOptions()
	index := 0
	if !m.agentsView {
		index = 1 + slices.Index(options, m.groupFilter)
	}
	next := min(max(0, index+delta), len(options))
	if next == index {
		return
	}
	m.agentsView = next == 0
	m.groupFilter = group.Filter{}
	if next > 0 {
		m.groupFilter = options[next-1]
	}
	m.rebuildForFilter()
}

// showAgents opens the agents view on its most urgent row.
func (m *model) showAgents() {
	m.agentsView, m.groupFilter = true, group.Filter{}
	m.rebuild()
	m.cursor = min(0, len(m.rows)-1)
	m.top = 0
}

func (m *model) move(delta int) {
	if len(m.rows) > 0 {
		m.cursor = min(max(0, m.cursor+delta), len(m.rows)-1)
	}
}

func statusState(session fleetclient.Session) string {
	return session.TerminalStatus.State
}
