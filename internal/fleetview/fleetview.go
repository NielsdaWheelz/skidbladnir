// Package fleetview is the cli's and the tui's wording of fleetclient's decoded facts: a
// row's status with the client's stale and ready overlays, recovery notices, failure and
// receipt messages and group headings. Nothing here reaches the wire; the host's copy is api's.
package fleetview

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

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
	Tone  api.StatusTone
	Queue QueueCategory
	// Detail is the label with its secondary facts, for the existing detail space.
	Detail string
	// QueueDetail is the notice phrase a menu hides. Queue rows append it to
	// their label and detail so their inclusion is visible without selection.
	QueueDetail string
	// Reason explains the observation without terminal content.
	Reason string
}

// ProjectStatus renders the row's status with api.PresentStatus, the
// function behind the wire's `status`, and overlays the client's facts: fresh
// is inventory freshness; ready is the notification owner's pending attention.
// A stale view names its last observation and is never ready.
func ProjectStatus(session fleetclient.Session, fresh, ready bool) StatusView {
	host := api.PresentStatus(session.TerminalStatus, session.Agent != nil && session.Connection == nil)
	view := StatusView{Label: host.Label, Tone: host.Tone, Reason: host.Reason}
	switch {
	case !fresh:
		view.Label, view.Tone = "last observed: "+host.Label, api.ToneMuted
	case host.Queue == api.QueueAction:
		view.Queue, view.QueueDetail = QueueAction, host.QueueNotice
	case host.Queue == api.QueueReady && ready:
		view.Label, view.Tone, view.Queue = "ready", api.ToneReady, QueueReady
	}
	view.Detail = view.Label
	for _, fact := range []string{host.Detail, host.Evidence} {
		if fact != "" {
			view.Detail += " · " + fact
		}
	}
	return view
}

// RecoveryNotice only projects fresh inventory; retained rows make no recovery
// claim. The host words the notice; the client adds its label and the save
// time in the local zone.
func RecoveryNotice(peer fleetclient.Peer) (text string, failure bool) {
	if !peer.OK || peer.Recovery == nil {
		return "", false
	}
	notice := peer.Recovery.Notice()
	if notice == nil {
		return "", false
	}
	text = peer.Label + ": " + notice.Summary
	if notice.Action != "" {
		save := "no confirmed save"
		if savedAt := peer.Recovery.SavedAt; savedAt != nil {
			save = "last confirmed save: " + savedAt.In(time.Local).Format("2006-01-02 15:04:05 utc-07:00")
		}
		text += ". " + save + ". " + notice.Action
	}
	return text, peer.Recovery.State == api.RecoveryBroken
}

// ErrorMessage is shared by command output and the desktop browser.
func ErrorMessage(failure fleetclient.Failure, request fleetclient.Request) string {
	if (request.Operation == "start" || request.Operation == "shell") && !failure.Target.IsZero() {
		return "terminal was created (" + failure.Target.Handle() + "); creation follow-up failed. refresh to inspect it."
	}
	if failure.Dispatch == "unknown" {
		if request.Operation == "rename" {
			return "name change outcome unknown. checking tmux."
		}
		if request.Operation == "group" {
			return "group change outcome unknown. checking tmux."
		}
		if request.Operation == "start" || request.Operation == "shell" {
			return "could not confirm terminal creation. refresh before taking another action."
		}
		if request.Operation == "close" {
			if request.TerminalOnly {
				return "could not confirm terminal deletion. refresh before taking another action."
			}
			return "could not confirm terminal interruption or deletion. inspect the terminal if available and refresh before taking another action."
		}
		return "could not confirm terminal input. inspect the terminal before trying again."
	}
	switch failure.Code {
	case "SessionNameInvalid":
		return "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number."
	case "SessionNameConflict":
		return "another session on this machine uses that name."
	case "SessionNameChanged":
		return "the session name changed. review and save again."
	case "TerminalTargetChanged", "SessionIdentityMismatch", "SessionNotFound":
		return "the terminal changed. refresh before trying again."
	case "TerminalInputBlocked":
		if failure.TerminalInputMessage != "" {
			return failure.TerminalInputMessage
		}
		return "send unavailable for this screen. open the terminal or use text/keys."
	case "TerminalUnavailable":
		return "terminal action unavailable. open the terminal or refresh before trying again."
	default:
		return failure.Code + " (" + failure.Dispatch + ")"
	}
}

func WriteText(operation string, receipt api.WriteResult) string {
	if receipt.Outcome == "unknown" {
		return "could not confirm terminal input. inspect the terminal before trying again."
	}
	switch operation {
	case "stop":
		return "interrupt key sent; stopping is unconfirmed."
	case "text", "send":
		return "text sent; acceptance is unconfirmed."
	default:
		return "keys sent; acceptance is unconfirmed."
	}
}

func GroupHeading(label group.Label) string {
	if label.IsUnassigned() {
		return "unassigned"
	}
	return label.String()
}
