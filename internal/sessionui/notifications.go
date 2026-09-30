package sessionui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) observeNotifications(message inventoryMsg) {
	if message.failure != nil {
		m.predecessors = map[fleetclient.TerminalKey]fleetclient.WorkingPredecessor{}
		return
	}
	if m.notificationStore == nil || message.notificationErr != nil {
		m.notificationFailed = true
		m.predecessors = map[fleetclient.TerminalKey]fleetclient.WorkingPredecessor{}
		return
	}
	snapshot, next, err := m.notificationStore.Observe(message.value.Peers, m.client.Machines(), message.expected, m.predecessors)
	m.notificationFailed = err != nil
	if err != nil {
		m.predecessors = map[fleetclient.TerminalKey]fleetclient.WorkingPredecessor{}
		return
	}
	m.notificationSnapshot = snapshot
	m.predecessors = next
}

// tea.Exec pauses browser observations for the entire attachment. Invalidating
// the pending inventory and visited predecessor closes its pre-visit path.
func (m *model) enter(request fleetclient.Request) tea.Cmd {
	ref, _ := fleetclient.DecodeReference(request.Ref)
	delete(m.predecessors, fleetclient.NotificationKey(ref))
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
