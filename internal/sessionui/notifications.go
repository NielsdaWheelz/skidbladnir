package sessionui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) observeNotifications(message inventoryMsg) {
	if message.failure != nil {
		return
	}
	if m.notificationStore == nil || message.notificationErr != nil {
		m.notificationFailed = true
		return
	}
	snapshot, err := m.notificationStore.Observe(message.value.Peers, m.client.Machines(), message.expected)
	m.notificationFailed = err != nil
	if err != nil {
		return
	}
	m.notificationSnapshot = snapshot
}

// tea.Exec pauses browser observations for the attachment. Reject the pending
// inventory so it cannot publish an observation from before this visit.
func (m *model) enter(request fleetclient.Request) tea.Cmd {
	m.usageAttached = true
	m.cancelUsage()
	if m.refreshing {
		m.refreshAfterAction = true
	}
	notificationErr := error(nil)
	if m.notificationFailed {
		notificationErr = fleetclient.ErrNotificationsUnavailable
	}
	a := &attachment{notificationErr: notificationErr, ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output, store: m.notificationStore, snapshot: m.notificationSnapshot}
	return tea.Exec(a, func(err error) tea.Msg { return attachedMsg{err, a.snapshot, a.notificationErr} })
}
