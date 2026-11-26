package sessionui

import (
	"context"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// inventoryLane owns both row-producing reads for one continuous roster membership.
// writing blocks reads during a mutation; settled records the first current reply.
type inventoryLane struct {
	lane
	contextRead      lane
	settled, writing bool
	// Exact lifetimes that read ready before the host or hub was fenced.
	ready []fleetclient.Reference
}

// observing gates every read: the browser owns the screen and is running.
func (m *model) observing() bool {
	return m.attachment == nil && m.ctx.Err() == nil
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
	owner := m.inventoryLanes[index]
	if owner.writing { // a mutation owns this peer until refreshMutation
		owner.trailing = owner.trailing || manual
		return nil
	}
	client, machine := m.client, m.peers[index].Machine
	return owner.read(m.ctx, manual, func(ctx context.Context, generation uint64) tea.Msg {
		return inventoryMsg{owner: owner, generation: generation, result: client.InventoryMachine(ctx, machine)}
	})
}

func (m *model) acceptInventory(message inventoryMsg) tea.Cmd {
	owner := message.owner
	index := slices.Index(m.inventoryLanes, owner)
	if index < 0 {
		return nil
	}
	landed, current := owner.land(message.generation)
	if !landed {
		return nil
	}
	var usage tea.Cmd
	if current && m.observing() {
		machine := m.peers[index].Machine
		owner.settled = true
		m.cancelContext(index)
		if message.result.OK {
			received := message.result.Value.(fleetclient.Peer)
			received.Label = m.peers[index].Label
			if m.reconcileUsageProfiles(index, received) {
				usage = m.refreshUsage(false)
			}
			previous := m.peers[index]
			for i, session := range received.Sessions {
				if session.Connection == nil {
					continue
				}
				owner.contextRead.trailing = true
				previousIndex := slices.IndexFunc(previous.Sessions, func(old fleetclient.Session) bool {
					return old.Ref == session.Ref && old.Connection != nil && old.Connection.ID == session.Connection.ID
				})
				if previousIndex >= 0 {
					received.Sessions[i].Execution = previous.Sessions[previousIndex].Execution
				}
			}
			m.peers[index] = received
		} else {
			m.peers[index].OK, m.peers[index].Error = false, message.result.Error
			m.forgetContext(index)
		}
		if !message.result.OK || m.notificationQualified || m.notificationFailed {
			owner.ready = nil
		}
		m.afterInventory(machine)
	}
	var trailing tea.Cmd
	if owner.trailing {
		trailing = m.refreshInventory(index, false)
	}
	return tea.Batch(usage, trailing, m.readContext(index))
}

func (m *model) rebuildFromPeers() {
	if m.fold.kind == foldDraft && m.searchState != directorySearchIdle && !m.directoryAvailable() {
		m.clearDirectorySearch()
	}
	m.rebuild()
}

func (m *model) afterInventory(machine string) {
	m.rebuildFromPeers()
	observed := fleetclient.ObservedGroups(m.peers)
	m.groupSelection.retain(groupChoices(m.form.group, observed))
	m.reconcileMetadata(machine, observed)
}

func (m *model) cancelContext(index int) {
	contextRead := &m.inventoryLanes[index].contextRead
	contextRead.fence()
	contextRead.trailing = false
}

func (m *model) forgetContext(index int) {
	for rowIndex := range m.peers[index].Sessions {
		m.peers[index].Sessions[rowIndex].Execution = nil
	}
}

// readContext enriches a snapshot of the peer's current rows.
func (m *model) readContext(index int) tea.Cmd {
	owner := m.inventoryLanes[index]
	if !m.observing() || !owner.contextRead.trailing {
		return nil
	}
	source := m.peers[index]
	source.Sessions = slices.Clone(source.Sessions)
	peers, client := slices.Clone(m.peers), m.client
	return owner.contextRead.read(m.ctx, true, func(ctx context.Context, generation uint64) tea.Msg {
		return contextMsg{owner: owner, generation: generation, peer: client.EnrichContexts(ctx, source, peers)}
	})
}

// acceptContext adopts the enriched rows, never the peer: reconcileFleet may
// have relabelled it. A current pass means nothing replaced these rows since
// it began, because every row writer fences the lane first.
func (m *model) acceptContext(message contextMsg) tea.Cmd {
	owner := message.owner
	index := slices.Index(m.inventoryLanes, owner)
	if index < 0 {
		return nil
	}
	landed, current := owner.contextRead.land(message.generation)
	if !landed {
		return nil
	}
	if current && m.peers[index].OK && m.observing() {
		m.peers[index].Sessions = message.peer.Sessions
		m.rebuild()
	}
	return m.readContext(index)
}

func (m *model) fenceInventory(index int) {
	m.inventoryLanes[index].fence()
	m.cancelContext(index)
}

func (m *model) checkInventory(index int) {
	peer := &m.peers[index]
	if peer.OK {
		var ready []fleetclient.Reference
		for _, session := range peer.Sessions {
			if m.statusView(peer.Row(session)).Sign == api.SignReady {
				ready = append(ready, session.Ref.Reference)
			}
		}
		m.inventoryLanes[index].ready = ready
		peer.OK = false
	}
	m.fenceInventory(index)
}

// cancelObservations retires every browser read while a terminal owns the screen.
func (m *model) cancelObservations() {
	for index := range m.inventoryLanes {
		m.checkInventory(index)
		m.inventoryLanes[index].trailing = false
	}
	m.notificationLane.fence()
	m.notificationLane.trailing = false
	m.notificationQualified = false
}

func (m *model) refreshMutation(index int) tea.Cmd {
	if index < 0 {
		return nil
	}
	m.fenceInventory(index)
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
		// An unreadable fleet.json keeps the last adopted roster;
		// direct inventories own visible availability.
		_ = client.Refresh()
		return fleetMsg{}
	}
}

func (m *model) reconcileFleet() bool {
	machines := m.client.Machines()
	if slices.EqualFunc(machines, m.peers, func(machine fleetclient.Machine, peer fleetclient.Peer) bool {
		return machine.Handle == peer.Machine && machine.Label == peer.Label
	}) {
		return false
	}
	peers := make([]fleetclient.Peer, len(machines))
	inventories := make([]*inventoryLane, len(machines))
	usages := make([]usagePeer, len(machines))
	for index, machine := range machines {
		peers[index] = fleetclient.Peer{Label: machine.Label, Machine: machine.Handle}
		usages[index] = usagePeer{machine: machine}
		if old := m.peerIndex(machine.Handle); old >= 0 {
			peers[index], inventories[index] = m.peers[old], m.inventoryLanes[old]
			peers[index].Label = machine.Label
			usages[index] = m.usagePeers[old]
			usages[index].machine = machine
		} else {
			inventories[index] = &inventoryLane{}
		}
	}
	for index, owner := range m.inventoryLanes {
		if !slices.Contains(inventories, owner) {
			m.fenceInventory(index)
		}
	}
	m.peers, m.inventoryLanes, m.usagePeers = peers, inventories, usages
	m.claudeUsage = m.client.KnownClaudeUsage()
	m.usageLane.fence()
	m.usageLane.trailing = true
	m.usageNext = time.Time{}
	m.rebuildFromPeers()
	return true
}
