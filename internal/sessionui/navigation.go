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

func (m *model) rebuild() {
	var selected fleetclient.Reference
	if row := m.selectedRow(); row != nil {
		selected = row.Session.Ref.Reference
	}
	previous := m.cursor
	m.rows = nil
	switch m.view.kind {
	case viewNeedsInput:
		for _, peer := range m.peers {
			for _, session := range peer.Sessions {
				row := peer.Row(session)
				if m.statusView(row).Queue != fleetclient.QueueExcluded {
					m.rows = append(m.rows, row)
				}
			}
		}
		slices.SortStableFunc(m.rows, func(a, b fleetclient.Row) int {
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
		for _, group := range fleetclient.Groups(m.peers, filter) {
			m.rows = append(m.rows, group.Rows...)
		}
	default:
		panic("invalid selected view") // justify-defect: only the closed view kinds populate the model.
	}
	m.cursor = -1
	for index, row := range m.rows {
		if row.Session.Ref.SessionEqual(selected) {
			m.cursor = index
			break
		}
	}
	if m.cursor < 0 && len(m.rows) > 0 {
		m.cursor = min(max(0, previous), len(m.rows)-1)
	}
}

// viewOptions includes every group observed in the fleet plus the selected group,
// in group order, so an empty selection survives and unassigned comes last.
// step walks it and header draws it, so drawn and stepped order are one.
func (m *model) viewOptions() []selectedView {
	labels := []group.Label{}
	for _, observed := range fleetclient.Groups(m.peers, group.Filter{}) {
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
	var selected fleetclient.Reference
	if row := m.selectedRow(); row != nil {
		selected = row.Session.Ref.Reference
	}
	m.view = view
	m.rebuild()
	// keep a surviving selection; otherwise begin at the first row of the view.
	if row := m.selectedRow(); row != nil && !row.Session.Ref.SessionEqual(selected) {
		m.cursor = 0
	}
	m.top = 0
}

func (m *model) move(delta int) {
	if len(m.rows) > 0 {
		m.cursor = min(max(0, m.cursor+delta), len(m.rows)-1)
	}
}
