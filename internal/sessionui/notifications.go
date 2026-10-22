package sessionui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) refreshNotifications(manual bool) tea.Cmd {
	if !m.observing() {
		return nil
	}
	client := m.client
	return m.notificationLane.read(m.ctx, manual, func(ctx context.Context, generation uint64) tea.Msg {
		state, err := client.NotificationState(ctx)
		return notificationMsg{generation: generation, state: state, err: err}
	})
}

func (m *model) acceptNotifications(message notificationMsg) tea.Cmd {
	landed, current := m.notificationLane.land(message.generation)
	if !landed {
		return nil
	}
	if current && m.observing() {
		m.notifications, m.notificationFailed, m.notificationQualified = message.state, message.err != nil, message.err == nil
		m.rebuild()
	}
	if m.notificationLane.trailing {
		return m.refreshNotifications(false)
	}
	return nil
}

// notificationReady reports the hub's pending ready for this exact lifetime and
// agent. ProjectStatus shows it only while the direct row is idle: direct
// inventory qualifies readiness and never creates it.
func (m *model) notificationReady(session fleetclient.Session) bool {
	encoded := session.Ref.Encode() // the hub's rows carry encoded references
	for _, machine := range m.notifications.Machines {
		for _, row := range machine.Sessions {
			if row.Ref == encoded {
				return row.ReadyGeneration != 0 && attention.SameForeground(row.Foreground, fleetclient.NotificationForeground(session))
			}
		}
	}
	return false
}
