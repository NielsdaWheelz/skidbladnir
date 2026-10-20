package sessionui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalclient"
)

type notificationLane struct {
	generation   uint64
	flight       *readFlight
	trailingRead bool
}

func (m *model) refreshNotifications(manual bool) tea.Cmd {
	if !m.observationsVisible() {
		return nil
	}
	lane := &m.notificationLane
	if lane.flight != nil {
		lane.trailingRead = lane.trailingRead || manual
		return nil
	}
	lane.generation++
	lane.trailingRead = false
	ctx, cancel := context.WithCancel(m.ctx)
	lane.flight = &readFlight{generation: lane.generation, cancel: cancel}
	client, generation := m.client, lane.generation
	completed := make(chan notificationMsg, 1)
	m.observationWorkers.Add(1)
	go func() {
		defer m.observationWorkers.Done()
		defer cancel()
		view, err := terminalclient.ReadNotifications(ctx, client)
		completed <- notificationMsg{generation: generation, view: view, err: err}
	}()
	return func() tea.Msg { return <-completed }
}

func (m *model) acceptNotifications(message notificationMsg) tea.Cmd {
	lane := &m.notificationLane
	if lane.flight == nil || lane.flight.generation != message.generation {
		return nil
	}
	lane.flight = nil
	if lane.generation == message.generation && m.observationsVisible() {
		m.notificationFailed = message.err != nil
		m.notificationQualified = message.err == nil
		if message.err == nil {
			m.notificationSnapshot = message.view
		} else if message.view.Device.Valid() {
			m.notificationSnapshot.Device = message.view.Device
		}
		m.rebuild()
	}
	if lane.trailingRead {
		return m.refreshNotifications(false)
	}
	return nil
}

// tea.Exec pauses the browser while this exact terminal owns input and output.
func (m *model) enter(request fleetclient.Request) tea.Cmd {
	var seed *attention.ReadySeed
	if m.notificationSnapshot.Device.Valid() {
		ref, _ := fleetclient.DecodeReference(request.Ref)
		captured := m.notificationSnapshot.Device.ReadySeed(fleetclient.NotificationKey(ref).Slot())
		seed = &captured
	}
	m.usageAttached = true
	m.cancelUsage()
	m.cancelObservations()
	m.clearDirectorySearch()
	a := &attachment{ctx: m.ctx, client: m.client, request: request, seed: seed, input: m.input, output: m.output}
	return tea.Exec(a, func(err error) tea.Msg { return attachedMsg{err: err, notificationErr: a.notificationErr} })
}
