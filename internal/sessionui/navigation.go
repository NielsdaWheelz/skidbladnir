package sessionui

import (
	"cmp"
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

type region int

const (
	tabs region = iota
	groups
	agents
)

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
	for _, group := range fleetclient.Groups(m.scopedPeers(), m.groupFilter) {
		for _, row := range group.Rows {
			m.rows = append(m.rows, listedRow{row.Label, row.Machine, row.Session, row.Available && m.scopeReady})
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

	current := []listedRow{}
	for _, peer := range m.scopedPeers() {
		for _, session := range peer.Sessions {
			if session.Agent != nil {
				current = append(current, listedRow{peer.Label, peer.Machine, session, peer.OK && m.scopeReady})
			}
		}
	}
	if m.focus == agents {
		ordered := make([]listedRow, 0, len(current))
		for _, previous := range m.agents {
			for index, row := range current {
				if sameSession(previous.session, row.session) {
					ordered = append(ordered, row)
					current = slices.Delete(current, index, index+1)
					break
				}
			}
		}
		m.agents = append(ordered, current...)
	} else {
		m.agents = current
		m.sortAgents()
	}
}

func (m *model) sortAgents() {
	states := []string{"blocked", "failed", "done", "working", "idle", "stopped", "unknown"}
	slices.SortStableFunc(m.agents, func(a, b listedRow) int {
		if a.available != b.available {
			if a.available {
				return -1
			}
			return 1
		}
		return cmp.Compare(slices.Index(states, a.session.Agent.Status.State), slices.Index(states, b.session.Agent.Status.State))
	})
}

func (m *model) rebuildForFilter() {
	var selected fleetclient.Session
	if row := m.selectedRow(); row != nil {
		selected = row.session
	}
	m.rebuild()
	if row := m.selectedRow(); row != nil && !sameSession(row.session, selected) {
		m.cursor = 0
	}
	m.groupsTop, m.agentsTop, m.tabsTop = 0, 0, 0
}

func (m *model) setFocus(focus region) {
	if m.focus == focus {
		return
	}
	m.focus = focus
	if focus != agents {
		// Rebuild from host order before sorting, so old focus order cannot break ties.
		m.rebuild()
	}
}

func (m *model) groupOptions() []group.Filter {
	labels := fleetclient.ObservedGroups(m.scopedPeers())
	if m.groupFilter.Kind() == group.FilterNamed && !slices.Contains(labels, m.groupFilter.Label()) {
		labels = append(labels, m.groupFilter.Label())
		slices.SortFunc(labels, group.Compare)
	}
	options := []group.Filter{{}, group.UnassignedFilter()}
	for _, label := range labels {
		options = append(options, filterFor(label))
	}
	return options
}

func (m *model) agentIndex() int {
	selected := m.selectedRow()
	if selected != nil {
		for index, row := range m.agents {
			if sameSession(selected.session, row.session) {
				return index
			}
		}
	}
	return -1
}

func (m *model) move(key string) {
	delta := 1
	if key == "up" || key == "k" || key == "left" || key == "h" {
		delta = -1
	}
	horizontal := key == "left" || key == "h" || key == "right" || key == "l"
	if horizontal != (m.focus == tabs) {
		return
	}
	switch m.focus {
	case tabs:
		if len(m.rows) > 0 {
			m.cursor = min(max(0, m.cursor+delta), len(m.rows)-1)
		}
	case groups:
		options := m.groupOptions()
		index := slices.Index(options, m.groupFilter)
		next := min(max(0, index+delta), len(options)-1)
		if next != index {
			m.groupFilter = options[next]
			m.rebuildForFilter()
		}
	case agents:
		if len(m.agents) == 0 {
			return
		}
		index := m.agentIndex()
		if index < 0 && delta < 0 {
			index = len(m.agents)
		}
		index = min(max(0, index+delta), len(m.agents)-1)
		selected := m.agents[index].session
		if m.groupFilter != filterFor(selected.Group) {
			m.groupFilter = filterFor(selected.Group)
			m.rebuildForFilter()
		}
		for index, row := range m.rows {
			if sameSession(row.session, selected) {
				m.cursor = index
				break
			}
		}
	}
}
