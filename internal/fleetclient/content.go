package fleetclient

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"time"
)

// RecoveryNotice only projects fresh inventory; retained rows make no recovery
// claim. The host words the notice; the client adds its label and the save
// time in the local zone.
func RecoveryNotice(peer Peer) (text string, failure bool) {
	if !peer.OK || peer.Recovery == nil {
		return "", false
	}
	notice := peer.Recovery.fact.Notice()
	if notice == nil {
		return "", false
	}
	text = peer.Label + ": " + notice.Summary
	if notice.Action != "" {
		save := "no confirmed save"
		if savedAt := peer.Recovery.fact.SavedAt; savedAt != nil {
			save = "last confirmed save: " + savedAt.In(time.Local).Format("2006-01-02 15:04:05 utc-07:00")
		}
		text += ". " + save + ". " + notice.Action
	}
	return text, peer.Recovery.fact.State == sessions.RecoveryBroken
}

// ErrorMessage is shared by command output and the desktop browser.
func ErrorMessage(failure Failure, request Request) string {
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
		if failure.terminalInputMessage != "" {
			return failure.terminalInputMessage
		}
		return "send unavailable for this screen. open the terminal or use text/keys."
	case "TerminalUnavailable":
		return "terminal action unavailable. open the terminal or refresh before trying again."
	default:
		return failure.Code + " (" + failure.Dispatch + ")"
	}
}

func WriteText(operation string, receipt WriteResult) string {
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

func CloseText(receipt CloseResult) string {
	if receipt.Terminal == "closed" {
		if receipt.Interrupt != "written" {
			return "terminal closed; interruption unconfirmed."
		}
		return "interrupt key sent; terminal closed. work shared elsewhere or running remotely may continue."
	}
	return "interruption: " + receipt.Interrupt + "; terminal: " + receipt.Terminal + ". work shared elsewhere or running remotely may continue."
}
