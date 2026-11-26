// Package fleetview is the cli's and the tui's wording of fleetclient's decoded facts: a
// row's status with the client's stale and ready overlays, recovery notices, failure and
// receipt messages and group headings. Nothing here reaches the wire; the host's copy is api's.
package fleetview

import (
	"fmt"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// QueueRank orders the needs-input collection. A negative rank excludes the sign.
func QueueRank(sign api.StatusSign) int {
	switch sign {
	case api.SignRequest:
		return 0
	case api.SignError:
		return 1
	case api.SignReady:
		return 2
	case api.SignInterrupted:
		return 3
	default:
		return -1
	}
}

// StatusView is the status shared by the cli and the desktop browser. State
// joins the label, detail and evidence for surfaces with room for all three.
type StatusView struct {
	Sign          api.StatusSign
	WorkContinues bool
	Label, Detail string
	State         string
	Reason        string
}

// ProjectStatus adds inventory freshness and the hub's pending ready to the
// host's projection. A stale view keeps its last observed words.
func ProjectStatus(session fleetclient.Session, fresh, ready bool) StatusView {
	host := api.PresentStatus(session.TerminalStatus, session.Agent != nil && session.Connection == nil)
	view := StatusView{
		Sign: host.Sign, WorkContinues: host.WorkContinues,
		Label: host.Label, Detail: host.Detail, Reason: host.Reason,
	}
	switch {
	case !fresh:
		view.Sign, view.WorkContinues, view.Label = api.SignUnknown, false, host.Label+" · stale"
	case host.Sign == api.SignIdle && ready:
		view.Sign, view.Label, view.Detail = api.SignReady, "ready", ""
	}
	view.State = view.Label
	for _, fact := range []string{view.Detail, host.Evidence} {
		if fact != "" {
			view.State += " · " + fact
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
		if request.Operation == "promote" {
			return "could not confirm promotion. refresh before taking another action."
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
	case "ParentUnavailable":
		return "run from a skid terminal or select an existing session with --parent."
	case "BranchChanged":
		if failure.Message != "" {
			return failure.Message
		}
		return "the branch changed. open a fresh preview."
	case "BranchPending", "BranchUnresolved":
		if failure.Message != "" {
			return failure.Message
		}
		return "branch membership is unresolved. inspect the involved machines."
	case "BranchConfirmationRequired":
		return "confirm the complete branch from an interactive terminal or the session browser."
	default:
		return failure.Code + " (" + failure.Dispatch + ")"
	}
}

// CloseQuestion names the confirmed mode and the full branch.
func CloseQuestion(targets []fleetclient.CloseTarget, terminalOnly bool) string {
	action, suffix := "interrupt and close", ""
	if terminalOnly {
		action, suffix = "close", " without interrupting"
	}
	root := targets[len(targets)-1]
	name := attention.SingleLine(root.Name)
	if len(targets) == 1 {
		return action + " " + name + " on " + attention.SingleLine(root.Label) + suffix + "?"
	}
	machines := map[string]bool{}
	for _, target := range targets {
		machines[target.Ref.Machine] = true
	}
	descendants, hosts := "descendant", "machine"
	if len(targets) > 2 {
		descendants = "descendants"
	}
	if len(machines) > 1 {
		hosts = "machines"
	}
	return fmt.Sprintf("%s %s and %d %s on %d %s%s?", action, name, len(targets)-1, descendants, len(machines), hosts, suffix)
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
