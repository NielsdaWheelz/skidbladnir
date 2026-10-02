package sessionui

import (
	"cmp"
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

type viewKind uint8

const (
	viewAll viewKind = iota
	viewNeedsInput
	viewGroup
)

// selectedView is one collection, not intersecting filters. Its zero value is
// all; a group with the zero label selects unassigned sessions.
type selectedView struct {
	kind  viewKind
	label group.Label
}

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
	switch m.view.kind {
	case viewNeedsInput:
		for _, peer := range m.scopedPeers() {
			for _, session := range peer.Sessions {
				row := listedRow{peer.Label, peer.Machine, peer.ObservedAt, session, peer.OK && m.scopeReady}
				if m.statusView(row).Queue != fleetclient.QueueExcluded {
					m.rows = append(m.rows, row)
				}
			}
		}
		slices.SortStableFunc(m.rows, func(a, b listedRow) int {
			return cmp.Compare(m.statusView(a).Queue, m.statusView(b).Queue)
		})
	case viewAll, viewGroup:
		filter := group.Filter{}
		if m.view.kind == viewGroup {
			filter = group.UnassignedFilter()
			if !m.view.label.IsUnassigned() {
				filter, _ = group.NamedFilter(m.view.label)
			}
		}
		for _, group := range fleetclient.Groups(m.scopedPeers(), filter) {
			for _, row := range group.Rows {
				m.rows = append(m.rows, listedRow{row.Label, row.Machine, row.ObservedAt, row.Session, row.Available && m.scopeReady})
			}
		}
	default:
		panic("invalid selected view") // justify-defect: only the closed view kinds populate the model.
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

// viewOptions includes every group observed in scope plus the selected group,
// in group order, so an empty selection survives and unassigned comes last.
// step walks it and header draws it, so drawn and stepped order are one.
func (m *model) viewOptions() []selectedView {
	labels := []group.Label{}
	for _, observed := range fleetclient.Groups(m.scopedPeers(), group.Filter{}) {
		labels = append(labels, observed.Label)
	}
	if m.view.kind == viewGroup && !slices.Contains(labels, m.view.label) {
		labels = append(labels, m.view.label)
		slices.SortFunc(labels, group.Compare)
	}
	options := []selectedView{{kind: viewNeedsInput}, {kind: viewAll}}
	for _, label := range labels {
		options = append(options, selectedView{kind: viewGroup, label: label})
	}
	return options
}

// step clamps at each end of the same sequence the header renders.
func (m *model) step(delta int) {
	options := m.viewOptions()
	index := slices.Index(options, m.view)
	m.selectView(options[min(max(0, index+delta), len(options)-1)])
}

func (m *model) selectView(view selectedView) {
	if m.view == view {
		return
	}
	m.view = view
	m.rebuildForFilter()
}

func (m *model) move(delta int) {
	if len(m.rows) > 0 {
		m.cursor = min(max(0, m.cursor+delta), len(m.rows)-1)
	}
}
