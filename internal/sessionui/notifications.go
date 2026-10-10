package sessionui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func (m *model) observeNotifications(message inventoryMsg) {
	m.notificationFailed = message.notificationErr != nil
	if message.notificationErr == nil {
		m.notificationSnapshot = message.notifications
	}
}

// tea.Exec pauses browser reads while the terminal owns input and output.
func (m *model) enter(request fleetclient.Request) tea.Cmd {
	m.usageAttached = true
	m.cancelUsage()
	if m.refreshing {
		m.refreshAfterAction = true
	}
	notificationErr := error(nil)
	if m.notificationFailed {
		notificationErr = attention.ErrUnavailable
	}
	a := &attachment{notificationErr: notificationErr, ctx: m.ctx, client: m.client, request: request, input: m.input, output: m.output, snapshot: m.notificationSnapshot}
	return tea.Exec(a, func(err error) tea.Msg { return attachedMsg{err, a.snapshot, a.notificationErr} })
}
