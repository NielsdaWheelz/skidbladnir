package fleetclient

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"time"
)

// RecoveryNotice only projects fresh inventory; retained rows make no recovery claim.
func RecoveryNotice(peer Peer) (text string, failure bool) {
	if !peer.OK || peer.Recovery == nil {
		return "", false
	}
	r := peer.Recovery.fact
	switch r.State {
	case sessions.RecoveryTracking:
		return "", false
	case sessions.RecoveryRecovered:
		return peer.Label + ": terminals restored as shells. resume conversations using the same provider account.", false
	case sessions.RecoveryDelayed, sessions.RecoveryBroken:
		savedAt := "no confirmed save"
		if r.SavedAt != nil {
			savedAt = "last confirmed save: " + r.SavedAt.In(time.Local).Format("2006-01-02 15:04:05 utc-07:00")
		}
		var phase string
		switch r.Failure.Phase {
		case sessions.RecoveryPhaseOwnership:
			phase = "writer lock"
		case sessions.RecoveryPhaseCheckpointRead:
			phase = "checkpoint read"
		case sessions.RecoveryPhaseSource:
			phase = "tmux server check"
		case sessions.RecoveryPhaseCapture:
			phase = "workspace capture"
		case sessions.RecoveryPhaseCheckpointWrite:
			phase = "checkpoint save"
		case sessions.RecoveryPhaseRestore:
			phase = "workspace restore"
		default:
			panic("invalid recovery phase") // justify-defect: Recovery fields are private and admitted by UnmarshalJSON.
		}
		var cause string
		switch r.Failure.Cause {
		case sessions.RecoveryCauseTimeout:
			cause = "timed out"
		case sessions.RecoveryCauseChanged:
			cause = "workspace or server changed"
		case sessions.RecoveryCauseUnavailable:
			cause = "required resource unavailable"
		case sessions.RecoveryCauseInvalid:
			cause = "invalid data or configuration"
		case sessions.RecoveryCauseLimit:
			cause = "supported limit exceeded"
		case sessions.RecoveryCauseBusy:
			cause = "lock already held"
		case sessions.RecoveryCausePermission:
			cause = "access denied"
		case sessions.RecoveryCauseNoSpace:
			cause = "storage space exhausted"
		case sessions.RecoveryCauseIO:
			cause = "input/output error"
		case sessions.RecoveryCauseAlreadyAttempted:
			cause = "prior restore may be incomplete"
		case sessions.RecoveryCauseNotEmpty:
			cause = "replacement server has sessions"
		default:
			panic("invalid recovery cause") // justify-defect: Recovery fields are private and admitted by UnmarshalJSON.
		}
		status, action := "workspace saving delayed", "retrying automatically"
		if r.State == sessions.RecoveryBroken {
			status, action = "workspace recovery stopped", "manual repair required"
		}
		return peer.Label + ": " + status + ". " + phase + ": " + cause + ". " + savedAt + ". " + action + "; live terminals remain usable.", r.State == sessions.RecoveryBroken
	default:
		panic("invalid recovery state") // justify-defect: Recovery fields are private and admitted by UnmarshalJSON.
	}
}

// ErrorMessage is shared by command output and the desktop browser.
func ErrorMessage(failure Failure, request Request, native bool) string {
	if (request.Operation == "start" || request.Operation == "shell") && failure.Target != "" {
		ref, _ := DecodeReference(failure.Target)
		return "terminal was created (" + ref.Handle() + "); creation follow-up failed. refresh to inspect it."
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
		if native {
			return "could not confirm the native request. inspect the conversation before trying again."
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
	case "AgentTargetStale":
		return "the native conversation changed. inspect it before trying again."
	case "AgentUnavailable":
		return "this native action is unavailable for the selected conversation."
	default:
		return failure.Code + " (" + failure.Dispatch + ")"
	}
}

func WriteText(operation string, receipt WriteResult) string {
	if receipt.Outcome == "unknown" {
		if receipt.Method == "native" {
			return "could not confirm native interruption. inspect the conversation before trying again."
		}
		return "could not confirm terminal input. inspect the terminal before trying again."
	}
	if receipt.Method == "native" {
		return "native work: " + receipt.Outcome + "; pending input may remain."
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
