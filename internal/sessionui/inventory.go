package sessionui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

type readFlight struct {
	generation uint64
	cancel     context.CancelFunc
}

type inventoryLane struct {
	generation   uint64
	flight       *readFlight
	trailingRead bool
	settled      bool
	writing      bool
}

type contextLane struct {
	sourceGeneration uint64
	flight           *readFlight
	pending          bool
}

func (m *model) observationsVisible() bool {
	return !m.observationsStopped && !m.usageAttached && m.ctx.Err() == nil
}

// Ordinary ticks skip occupied lanes. Manual refresh keeps one successor.
func (m *model) refresh(manual bool) tea.Cmd {
	if !m.observationsVisible() {
		return nil
	}
	commands := []tea.Cmd{m.refreshNotifications(manual), m.refreshFleet()}
	for index := range m.peers {
		commands = append(commands, m.refreshInventory(index, manual))
	}
	return tea.Batch(commands...)
}

func (m *model) refreshInventory(index int, manual bool) tea.Cmd {
	if !m.observationsVisible() {
		return nil
	}
	lane := &m.inventoryLanes[index]
	if lane.flight != nil || lane.writing {
		lane.trailingRead = lane.trailingRead || manual
		return nil
	}
	lane.generation++
	lane.trailingRead = false
	ctx, cancel := context.WithCancel(m.ctx)
	lane.flight = &readFlight{generation: lane.generation, cancel: cancel}
	client, machine, generation := m.client, m.peers[index].Machine, lane.generation
	completed := make(chan inventoryMsg, 1)
	m.observationWorkers.Add(1)
	go func() {
		defer m.observationWorkers.Done()
		defer cancel()
		completed <- inventoryMsg{machine: machine, generation: generation, result: client.InventoryMachine(ctx, machine)}
	}()
	return func() tea.Msg { return <-completed }
}

func (m *model) acceptInventory(message inventoryMsg) tea.Cmd {
	for index := range m.peers {
		if m.peers[index].Machine != message.machine {
			continue
		}
		lane := &m.inventoryLanes[index]
		if lane.flight == nil || lane.flight.generation != message.generation {
			return nil
		}
		lane.flight = nil
		var usage tea.Cmd
		if message.generation == lane.generation && m.observationsVisible() {
			lane.settled = true
			previous := m.peers[index]
			m.retireContext(index)
			if message.result.OK {
				received := message.result.Value.(fleetclient.Peer)
				if m.reconcileUsageProfiles(received) {
					usage = m.refreshUsage(time.Now(), false)
				}
				m.peers[index] = received
				for rowIndex := range received.Sessions {
					if received.Sessions[rowIndex].Connection != nil {
						m.contextLanes[index].pending = true
						break
					}
				}
				m.contextLanes[index].sourceGeneration = message.generation
			} else {
				previous.OK, previous.Error = false, message.result.Error
				m.peers[index] = previous
			}
			m.afterInventory(message.machine, message.generation)
		}
		var trailing tea.Cmd
		if lane.trailingRead {
			trailing = m.refreshInventory(index, false)
		}
		return tea.Batch(usage, trailing, m.scheduleContexts())
	}
	return nil
}

func (m *model) afterInventory(machine string, generation uint64) {
	if m.page == "create" && !m.directoryAvailable() {
		m.clearDirectorySearch()
	}
	m.rebuild()
	observed := fleetclient.ObservedGroups(m.peers)
	m.groupSelection.retain(groupChoices(m.form[4], observed))
	if editor := m.metadata; editor != nil {
		if editor.request.Operation == "group" {
			editor.selection.retain(groupChoices(editor.draft, observed))
		}
		ref, _ := fleetclient.DecodeReference(editor.request.Ref)
		if ref.Machine == machine && (!editor.checking || generation > editor.readAfter) {
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
		if lane.flight != nil || lane.writing || lane.trailingRead {
			return true
		}
	}
	return false
}

func (m *model) retireContext(index int) {
	lane := &m.contextLanes[index]
	lane.sourceGeneration, lane.pending = 0, false
	if lane.flight != nil {
		lane.flight.cancel()
	}
	for rowIndex := range m.peers[index].Sessions {
		m.peers[index].Sessions[rowIndex].Execution = nil
	}
}

func (m *model) scheduleContexts() tea.Cmd {
	if !m.observationsVisible() {
		return nil
	}
	active := 0
	for _, lane := range m.contextLanes {
		if lane.flight != nil {
			active++
		}
	}
	commands := []tea.Cmd{}
	for index := range m.contextLanes {
		lane := &m.contextLanes[index]
		if active == 3 {
			break
		}
		if !lane.pending || lane.flight != nil {
			continue
		}
		lane.pending = false
		ctx, cancel := context.WithCancel(m.ctx)
		lane.flight = &readFlight{generation: lane.sourceGeneration, cancel: cancel}
		source, generation := m.peers[index], lane.sourceGeneration
		source.Sessions = append([]fleetclient.Session(nil), source.Sessions...)
		catalogues, client := append([]fleetclient.Peer(nil), m.peers...), m.client
		completed := make(chan contextMsg, 1)
		m.observationWorkers.Add(1)
		go func() {
			defer m.observationWorkers.Done()
			defer cancel()
			completed <- contextMsg{machine: source.Machine, generation: generation, values: client.EnrichContexts(ctx, source, catalogues)}
		}()
		commands = append(commands, func() tea.Msg { return <-completed })
		active++
	}
	return tea.Batch(commands...)
}

func (m *model) acceptContext(message contextMsg) tea.Cmd {
	for index := range m.peers {
		if m.peers[index].Machine != message.machine {
			continue
		}
		lane := &m.contextLanes[index]
		if lane.flight == nil || lane.flight.generation != message.generation {
			return nil
		}
		lane.flight = nil
		if lane.sourceGeneration == message.generation && m.peers[index].OK && m.observationsVisible() {
			for _, observation := range message.values {
				for rowIndex := range m.peers[index].Sessions {
					session := &m.peers[index].Sessions[rowIndex]
					if session.Ref == observation.SourceRef && session.Connection != nil && session.Connection.ID == observation.ConnectionID {
						execution := observation.Execution
						session.Execution = &execution
					}
				}
			}
			m.rebuild()
			if m.page == "details" || m.page == "name-edit" || m.page == "group-edit" {
				m.refreshInfo()
			}
		}
		break
	}
	return m.scheduleContexts()
}

func (m *model) fenceInventory(index int) {
	lane := &m.inventoryLanes[index]
	lane.generation++
	if lane.flight != nil {
		lane.flight.cancel()
	}
	m.retireContext(index)
}

func (m *model) cancelObservations() {
	for index := range m.inventoryLanes {
		m.fenceInventory(index)
		m.inventoryLanes[index].trailingRead = false
	}
	m.notificationLane.generation++
	m.notificationLane.trailingRead = false
	if m.notificationLane.flight != nil {
		m.notificationLane.flight.cancel()
	}
	m.notificationQualified = false
}

func (m *model) mutationPeer(request fleetclient.Request) int {
	ref, _ := fleetclient.DecodeReference(request.Ref)
	for index, peer := range m.peers {
		if peer.Machine == ref.Machine || request.Ref == "" && peer.Label == request.Machine {
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
	completed := make(chan fleetMsg, 1)
	m.observationWorkers.Add(1)
	go func() {
		defer m.observationWorkers.Done()
		_ = m.client.Refresh() // justify-ignore-error: retain the last verified roster; direct inventories own visible availability.
		completed <- fleetMsg{}
	}()
	return func() tea.Msg { return <-completed }
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
	contexts := make([]contextLane, len(machines))
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
	m.cancelUsage()
	m.usageNext = time.Time{}
	m.rebuild()
}
