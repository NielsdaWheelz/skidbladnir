package sessionui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
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

	m.buildTree(retained)
	if m.view.kind == viewNeedsInput {
		m.rows = m.queue()
	}
	m.guest = false
	m.cursor = min(0, len(m.rows)-1)
	if selected == nil {
		return
	}
	if index := m.rowIndex(selected.Session.Ref.Reference); index >= 0 {
		m.cursor = index
		return
	}
	if retained != nil && m.selectGuest(previous, *retained) {
		return
	}
	for _, row := range previousRows[previous+1:] {
		if index := m.rowIndex(row.Session.Ref.Reference); index >= 0 {
			m.cursor = index
			return
		}
	}
	for _, row := range slices.Backward(previousRows[:previous]) {
		if index := m.rowIndex(row.Session.Ref.Reference); index >= 0 {
			m.cursor = index
			return
		}
	}
}

func (m *model) rowIndex(ref fleetclient.Reference) int {
	return slices.IndexFunc(m.rows, func(row fleetclient.Row) bool { return row.Session.Ref.SessionEqual(ref) })
}

func (m *model) selectVisit(ref fleetclient.Reference) {
	if index := m.rowIndex(ref); index >= 0 {
		if index != m.cursor {
			m.selectRow(index)
		}
	} else if row := m.rowForReference(ref); row != nil {
		m.selectGuest(m.cursor, *row)
	}
}

func (m *model) selectGuest(index int, row fleetclient.Row) bool {
	if m.view.kind == viewNeedsInput && row.Session.Parent != nil {
		return false
	}
	if m.guest {
		m.rows = slices.Delete(m.rows, m.cursor, m.cursor+1)
	}
	m.cursor, m.guest = min(max(0, index), len(m.rows)), true
	m.rows = slices.Insert(m.rows, m.cursor, row)
	return true
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
	m.chooseView(options[min(max(0, index+delta), len(options)-1)])
}

func (m *model) chooseView(view selectedView) {
	if m.view == view {
		return
	}
	var previous fleetclient.Reference
	if row := m.selectedRow(); row != nil {
		previous = row.Session.Ref.Reference
	}
	m.fold = fold{}
	m.view = view
	m.rebuild()
	// keep a surviving selection; otherwise begin at the first row of the view.
	if row := m.selectedRow(); row != nil && (m.guest || !row.Session.Ref.SessionEqual(previous)) {
		m.selectRow(0)
	}
	m.top = 0
}

func (m *model) move(delta int) {
	next := m.cursor + delta
	if next >= 0 && next < len(m.rows) {
		m.navigateRow(next)
	}
}

func (m *model) navigateRow(index int) {
	if index == m.cursor {
		return
	}
	if m.fold.kind == foldOpen {
		m.fold = fold{}
	}
	m.selectRow(index)
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

func (m *model) queue() []fleetclient.Row {
	var rows []fleetclient.Row
	for _, peer := range m.peers {
		for _, session := range peer.Sessions {
			row := peer.Row(session)
			if m.queued(row) {
				rows = append(rows, row)
			}
		}
	}
	slices.SortStableFunc(rows, func(a, b fleetclient.Row) int {
		return cmp.Compare(fleetview.QueueRank(m.statusView(a).Sign), fleetview.QueueRank(m.statusView(b).Sign))
	})
	return rows
}

func (m *model) queued(row fleetclient.Row) bool {
	return row.Session.Parent == nil && fleetview.QueueRank(m.statusView(row).Sign) >= 0
}

// A row keeps its own group. Its heading in all is the visible root's group.
type treeNode struct {
	row      fleetclient.Row
	children []*treeNode
	count    int
	parent   *treeNode
	depth    int
	heading  group.Label
	context  string
}

func (m *model) buildTree(selected *fleetclient.Row) {
	m.tree = make(map[api.SessionRef]*treeNode)
	if m.expanded == nil {
		m.expanded = make(map[api.SessionRef]bool)
	}
	var nodes []*treeNode
	for _, peer := range m.peers {
		sessions := slices.Clone(peer.Sessions)
		slices.SortStableFunc(sessions, fleetclient.CompareSessions)
		for _, s := range sessions {
			node := &treeNode{row: peer.Row(s), heading: s.Group}
			m.tree[s.Ref.SessionRef()] = node
			for _, former := range s.Previous {
				m.tree[former] = node
			}
			nodes = append(nodes, node)
		}
	}
	for id := range m.expanded {
		if m.tree[id] == nil {
			delete(m.expanded, id)
		}
	}
	inView := func(node *treeNode) bool { return m.view.kind != viewGroup || m.view.label == node.row.Session.Group }
	var roots []*treeNode
	for _, node := range nodes {
		p := node.row.Session.Parent
		if p != nil {
			parent := m.tree[*p]
			if parent != nil {
				parent.count++
			}
			switch {
			case parent == nil || !m.rowAttachable(parent.row):
				node.context = "parent unavailable"
			case !inView(parent):
				node.context = "under " + parent.row.Session.Name + " on " + parent.row.Label
			case inView(node):
				node.parent = parent
				parent.children = append(parent.children, node)
			}
		}
		if node.parent == nil && inView(node) {
			roots = append(roots, node)
		}
	}
	// A returning parent can move a selected orphan under a folded branch.
	if selected != nil && m.view.kind != viewNeedsInput {
		for node := m.tree[selected.Session.Ref.SessionRef()]; node != nil && node.parent != nil; node = node.parent {
			m.expanded[node.parent.row.Session.Ref.SessionRef()] = true
		}
	}
	slices.SortStableFunc(roots, func(a, b *treeNode) int { return group.Compare(a.heading, b.heading) })
	m.rows = nil
	var appendBranch func(*treeNode, int, group.Label)
	appendBranch = func(node *treeNode, depth int, heading group.Label) {
		node.depth, node.heading = depth, heading
		m.rows = append(m.rows, node.row)
		if m.expanded[node.row.Session.Ref.SessionRef()] {
			for _, child := range node.children {
				appendBranch(child, depth+1, heading)
			}
		}
	}
	for _, root := range roots {
		appendBranch(root, 0, root.heading)
	}
}

func (m *model) toggleInfo() tea.Cmd {
	row := m.selectedRow()
	id := row.Session.Ref.SessionRef()
	node := m.tree[id]
	if m.view.kind != viewNeedsInput && len(node.children) > 0 && m.expanded[id] {
		m.fold = fold{}
		m.setExpanded(id, false)
		return nil
	}
	if m.fold.kind == foldOpen {
		m.fold = fold{}
		return nil
	}
	m.fold = fold{kind: foldOpen}
	if m.view.kind != viewNeedsInput && len(node.children) > 0 {
		m.setExpanded(id, true)
	}
	return nil
}

func (m *model) setExpanded(id api.SessionRef, expand bool) {
	if expand {
		m.expanded[id] = true
	} else {
		delete(m.expanded, id)
	}
	m.rebuild()
}
