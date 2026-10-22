package sessionui

import (
	"context"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// inventoryLane reads one peer. writing blocks reads while a mutation owns the
// peer; settled records its first current reply.
type inventoryLane struct {
	lane
	settled, writing bool
}

// observing gates every read: the browser owns the screen and is running.
func (m *model) observing() bool {
	return !m.attached && m.ctx.Err() == nil
}

func (m *model) peerIndex(machine string) int {
	return slices.IndexFunc(m.peers, func(peer fleetclient.Peer) bool { return peer.Machine == machine })
}

// Ordinary ticks skip occupied lanes. Manual refresh keeps one successor.
func (m *model) refresh(manual bool) tea.Cmd {
	if !m.observing() {
		return nil
	}
	commands := []tea.Cmd{m.refreshNotifications(manual), m.refreshFleet()}
	for index := range m.peers {
		commands = append(commands, m.refreshInventory(index, manual))
	}
	return tea.Batch(commands...)
}

func (m *model) refreshInventory(index int, manual bool) tea.Cmd {
	if !m.observing() {
		return nil
	}
	lane := &m.inventoryLanes[index]
	if lane.writing { // a mutation owns this peer until refreshMutation
		lane.trailing = lane.trailing || manual
		return nil
	}
	client, machine := m.client, m.peers[index].Machine
	return lane.read(m.ctx, manual, func(ctx context.Context, generation uint64) tea.Msg {
		return inventoryMsg{machine: machine, generation: generation, result: client.InventoryMachine(ctx, machine)}
	})
}

func (m *model) acceptInventory(message inventoryMsg) tea.Cmd {
	index := m.peerIndex(message.machine)
	if index < 0 {
		return nil
	}
	lane := &m.inventoryLanes[index]
	landed, current := lane.land(message.generation)
	if !landed {
		return nil
	}
	var usage tea.Cmd
	if current && m.observing() {
		lane.settled = true
		m.retireContext(index)
		if message.result.OK {
			received := message.result.Value.(fleetclient.Peer)
			if m.reconcileUsageProfiles(received) {
				usage = m.refreshUsage(false)
			}
			m.peers[index] = received
			m.contextLanes[index].trailing = slices.ContainsFunc(received.Sessions, func(session fleetclient.Session) bool { return session.Connection != nil })
		} else {
			m.peers[index].OK, m.peers[index].Error = false, message.result.Error
		}
		m.afterInventory(message.machine, message.generation)
	}
	var trailing tea.Cmd
	if lane.trailing {
		trailing = m.refreshInventory(index, false)
	}
	return tea.Batch(usage, trailing, m.readContext(index))
}

func (m *model) afterInventory(machine string, generation uint64) {
	if m.page == "create" && !m.directoryAvailable() {
		m.clearDirectorySearch()
	}
	m.rebuild()
	observed := fleetclient.ObservedGroups(m.peers)
	m.groupSelection.retain(groupChoices(m.form[formGroup], observed))
	if editor := m.metadata; editor != nil {
		if editor.request.Operation == "group" {
			editor.selection.retain(groupChoices(editor.draft, observed))
		}
		if editor.request.Ref.Machine == machine && (!editor.checking || generation > editor.readAfter) {
			m.reconcileMetadata()
		}
	}
	if m.page == "details" || m.page == "name-edit" || m.page == "group-edit" {
		m.refreshInfo()
	}
}

func (m *model) initialInventorySettled() bool {
	for _, lane := range m.inventoryLanes {
		if !lane.settled {
			return false
		}
	}
	return true
}

func (m *model) inventoryPending() bool {
	for _, lane := range m.inventoryLanes {
		if lane.flight != 0 || lane.writing || lane.trailing {
			return true
		}
	}
	return false
}

// retireContext drops the peer's display context and any pass over its rows.
func (m *model) retireContext(index int) {
	m.contextLanes[index].fence()
	m.contextLanes[index].trailing = false
	for rowIndex := range m.peers[index].Sessions {
		m.peers[index].Sessions[rowIndex].Execution = nil
	}
}

// readContext starts the peer's display-context pass when its current
// inventory has remote rows. The pass reads a snapshot: retireContext writes
// the live rows.
func (m *model) readContext(index int) tea.Cmd {
	if !m.observing() || !m.contextLanes[index].trailing {
		return nil
	}
	source := m.peers[index]
	source.Sessions = slices.Clone(source.Sessions)
	peers, client := slices.Clone(m.peers), m.client
	return m.contextLanes[index].read(m.ctx, true, func(ctx context.Context, generation uint64) tea.Msg {
		return contextMsg{machine: source.Machine, generation: generation, peer: client.EnrichContexts(ctx, source, peers)}
	})
}

// acceptContext adopts the enriched rows, never the peer: reconcileFleet may
// have relabelled it. A current pass means nothing replaced these rows since
// it began, because every row writer fences the lane first.
func (m *model) acceptContext(message contextMsg) tea.Cmd {
	index := m.peerIndex(message.machine)
	if index < 0 {
		return nil
	}
	landed, current := m.contextLanes[index].land(message.generation)
	if !landed {
		return nil
	}
	if current && m.peers[index].OK && m.observing() {
		m.peers[index].Sessions = message.peer.Sessions
		m.rebuild()
		if m.page == "details" || m.page == "name-edit" || m.page == "group-edit" {
			m.refreshInfo()
		}
	}
	return m.readContext(index)
}

func (m *model) fenceInventory(index int) {
	m.inventoryLanes[index].fence()
	m.retireContext(index)
}

// cancelObservations retires every browser read while a terminal owns the screen.
func (m *model) cancelObservations() {
	for index := range m.inventoryLanes {
		m.fenceInventory(index)
		m.inventoryLanes[index].trailing = false
	}
	m.notificationLane.fence()
	m.notificationLane.trailing = false
	m.notificationQualified = false
}

func (m *model) mutationPeer(request fleetclient.Request) int {
	for index, peer := range m.peers {
		if peer.Machine == request.Ref.Machine || request.Ref.IsZero() && peer.Label == request.Machine {
			return index
		}
	}
	return -1
}

func (m *model) refreshMutation() tea.Cmd {
	index := m.mutationPeer(m.pending)
	if index < 0 {
		return nil
	}
	m.fenceInventory(index)
	m.inventoryLanes[index].writing = false
	if editor := m.metadata; editor != nil && editor.checking {
		editor.readAfter = m.inventoryLanes[index].generation
	}
	return m.refreshInventory(index, true)
}

// Fleet reconciliation runs on the existing inventory cadence, independently of attachment.
func (m *model) refreshFleet() tea.Cmd {
	if m.fleetPending {
		return nil
	}
	m.fleetPending = true
	client := m.client
	return func() tea.Msg {
		_ = client.Refresh() // justify-ignore-error: an unreadable fleet.json keeps the last adopted roster; direct inventories own visible availability.
		return fleetMsg{}
	}
}

func (m *model) reconcileFleet() {
	machines := m.client.Machines()
	same := len(machines) == len(m.peers)
	for index, machine := range machines {
		if index >= len(m.peers) || m.peers[index].Machine != machine.Handle || m.peers[index].Label != machine.Label {
			same = false
			break
		}
	}
	if same {
		return
	}
	peers := make([]fleetclient.Peer, len(machines))
	inventories := make([]inventoryLane, len(machines))
	contexts := make([]lane, len(machines))
	usages := make([]usagePeer, len(machines))
	retained := map[string]bool{}
	for index, machine := range machines {
		peers[index] = fleetclient.Peer{Label: machine.Label, Machine: machine.Handle}
		usages[index] = usagePeer{machine: machine}
		for old, peer := range m.peers {
			if peer.Machine != machine.Handle {
				continue
			}
			peers[index], inventories[index], contexts[index] = peer, m.inventoryLanes[old], m.contextLanes[old]
			peers[index].Label = machine.Label
			usages[index] = m.usagePeers[old]
			usages[index].machine = machine
			retained[machine.Handle] = true
			break
		}
	}
	for index, peer := range m.peers {
		if !retained[peer.Machine] {
			m.fenceInventory(index)
		}
	}
	m.peers, m.inventoryLanes, m.contextLanes, m.usagePeers = peers, inventories, contexts, usages
	m.usageLane.fence()
	m.usageLane.trailing = true
	m.usageNext = time.Time{}
	m.rebuild()
}
