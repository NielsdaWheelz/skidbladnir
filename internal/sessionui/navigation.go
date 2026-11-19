package sessionui

import (
	"cmp"
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
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
	previous, previousRows, selected := m.cursor, m.rows, m.selectedRow()
	var retained *fleetclient.Row
	lostMachine := ""
	if selected != nil {
		ref := selected.Session.Ref.Reference
		retained = m.rowForReference(ref)
		if retained == nil {
			lostMachine = ref.Machine
		}
	}
	m.reconcileRowLoss(lostMachine)

	m.rows = nil
	switch m.view.kind {
	case viewNeedsInput:
		for _, peer := range m.peers {
			for _, session := range peer.Sessions {
				row := peer.Row(session)
				if m.statusView(row).Queue != fleetview.QueueExcluded {
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
		for _, section := range fleetclient.Groups(m.peers, filter) {
			m.rows = append(m.rows, section.Rows...)
		}
	default:
		panic("invalid selected view")
	}
	m.guest = false
	m.cursor = min(0, len(m.rows)-1)
	if selected == nil {
		return
	}
	indexOf := func(ref fleetclient.Reference) int {
		return slices.IndexFunc(m.rows, func(row fleetclient.Row) bool { return row.Session.Ref.SessionEqual(ref) })
	}
	if index := indexOf(selected.Session.Ref.Reference); index >= 0 {
		m.cursor = index
		return
	}
	if retained != nil {
		m.cursor, m.guest = min(previous, len(m.rows)), true
		m.rows = slices.Insert(m.rows, m.cursor, *retained)
		return
	}
	for _, row := range previousRows[previous+1:] {
		if index := indexOf(row.Session.Ref.Reference); index >= 0 {
			m.cursor = index
			return
		}
	}
	for _, row := range slices.Backward(previousRows[:previous]) {
		if index := indexOf(row.Session.Ref.Reference); index >= 0 {
			m.cursor = index
			return
		}
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
	view := options[min(max(0, index+delta), len(options)-1)]
	if m.view == view {
		return
	}
	m.fold = fold{}
	m.view = view
	m.rebuild()
	// keep a surviving selection; otherwise begin at the first row of the view.
	if m.guest {
		m.selectRow(0)
	}
	m.top = 0
}

func (m *model) move(delta int) {
	next := m.cursor + delta
	if next >= 0 && next < len(m.rows) {
		m.selectRow(next)
	}
}

// selectRow uses the drawn index before removing the guest.
func (m *model) selectRow(index int) {
	if m.guest {
		m.rows = slices.Delete(m.rows, m.cursor, m.cursor+1)
		if index > m.cursor {
			index--
		}
		m.guest = false
	}
	m.cursor = min(index, len(m.rows)-1)
}

func (m *model) guestNote(row fleetclient.Row) string {
	if m.view.kind == viewGroup {
		return "left this group"
	}
	return "left the queue: " + fleetview.ProjectStatus(row.Session, true, false).Label
}
