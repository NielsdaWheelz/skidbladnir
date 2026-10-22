package sessionui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
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
		state, err := client.NotificationState(ctx)
		completed <- notificationMsg{generation: generation, state: state, err: err}
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
		m.notifications, m.notificationFailed, m.notificationQualified = message.state, message.err != nil, message.err == nil
		m.rebuild()
	}
	if lane.trailingRead {
		return m.refreshNotifications(false)
	}
	return nil
}

// notificationReady reports the hub's pending ready for this exact lifetime and
// agent. ProjectStatus shows it only while the direct row is idle: direct
// inventory qualifies readiness and never creates it.
func (m *model) notificationReady(session fleetclient.Session) bool {
	for _, machine := range m.notifications.Machines {
		for _, row := range machine.Sessions {
			if row.Ref == session.Ref {
				return row.ReadyGeneration != 0 && attention.SameForeground(row.Foreground, fleetclient.NotificationForeground(session))
			}
		}
	}
	return false
}
