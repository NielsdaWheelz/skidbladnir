package fleetclient

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// QueueCategory derives human collection membership and order, never control
// admission. Only Ready and Action participate, in that order.
type QueueCategory int

const (
	QueueExcluded QueueCategory = iota
	QueueReady
	QueueAction
)

// StatusView is the one terminal-status projection shared by the cli and the
// desktop browser. It derives from observed facts only.
type StatusView struct {
	Label string
	Tone  sessions.StatusTone
	Queue QueueCategory
	// Detail is the label with its secondary facts, for the existing detail space.
	Detail string
	// QueueDetail is the notice phrase a menu hides. Queue rows append it to
	// their label and detail so their inclusion is visible without selection.
	QueueDetail string
	// Reason explains the observation without terminal content.
	Reason string
}

// ProjectStatus renders the row's status with sessions.PresentStatus, the
// function behind the wire's `status`, and overlays the client's facts: fresh
// is inventory freshness; ready is the notification owner's pending attention.
// A stale view names its last observation and is never ready.
func ProjectStatus(session Session, fresh, ready bool) StatusView {
	host := sessions.PresentStatus(session.TerminalStatus, session.Agent != nil && session.Connection == nil)
	view := StatusView{Label: host.Label, Tone: host.Tone, Reason: host.Reason}
	switch {
	case !fresh:
		view.Label, view.Tone = "last observed: "+host.Label, sessions.ToneMuted
	case host.Queue == sessions.QueueAction:
		view.Queue, view.QueueDetail = QueueAction, host.QueueNotice
	case host.Queue == sessions.QueueReady && ready:
		view.Label, view.Tone, view.Queue = "ready", sessions.ToneReady, QueueReady
	}
	view.Detail = view.Label
	for _, fact := range []string{host.Detail, host.Evidence} {
		if fact != "" {
			view.Detail += " · " + fact
		}
	}
	return view
}

// NeedsInput reports a human request interaction, whatever the activity,
// notice or reason; menus, notices and readiness never qualify. A request
// already implies a terminal source (TerminalStatus.Valid). Callers own
// freshness: a stale observation never needs input.
func NeedsInput(status sessions.TerminalStatus) bool {
	return status.Interaction.Request()
}
