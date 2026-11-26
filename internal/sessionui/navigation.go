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
	indexOf := func(ref fleetclient.Reference) int {
		return slices.IndexFunc(m.rows, func(row fleetclient.Row) bool { return row.Session.Ref.SessionEqual(ref) })
	}
	if index := indexOf(selected.Session.Ref.Reference); index >= 0 {
		m.cursor = index
		return
	}
	if retained != nil && !(m.view.kind == viewNeedsInput && retained.Session.Parent != nil) {
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
		delete(m.expanded, id)
		m.fold = fold{}
		m.rebuild()
		return nil
	}
	if m.fold.kind == foldOpen {
		m.fold = fold{}
		return nil
	}
	m.fold = fold{kind: foldOpen}
	if m.view.kind != viewNeedsInput && len(node.children) > 0 {
		m.expanded[id] = true
		m.rebuild()
	}
	return nil
}
