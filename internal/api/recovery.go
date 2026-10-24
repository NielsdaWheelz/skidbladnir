package api

import (
	"fmt"
	"time"
)

type RecoveryState string

const (
	RecoveryTracking  RecoveryState = "tracking"
	RecoveryRecovered RecoveryState = "recovered"
	RecoveryDelayed   RecoveryState = "delayed"
	RecoveryBroken    RecoveryState = "broken"
)

type RecoveryPhase string

const (
	RecoveryPhaseOwnership       RecoveryPhase = "ownership"
	RecoveryPhaseCheckpointRead  RecoveryPhase = "checkpoint_read"
	RecoveryPhaseSource          RecoveryPhase = "source"
	RecoveryPhaseCapture         RecoveryPhase = "capture"
	RecoveryPhaseCheckpointWrite RecoveryPhase = "checkpoint_write"
	RecoveryPhaseRestore         RecoveryPhase = "restore"
)

type RecoveryCause string

const (
	RecoveryCauseTimeout          RecoveryCause = "timeout"
	RecoveryCauseChanged          RecoveryCause = "changed"
	RecoveryCauseUnavailable      RecoveryCause = "unavailable"
	RecoveryCauseInvalid          RecoveryCause = "invalid"
	RecoveryCauseLimit            RecoveryCause = "limit"
	RecoveryCauseBusy             RecoveryCause = "busy"
	RecoveryCausePermission       RecoveryCause = "permission"
	RecoveryCauseNoSpace          RecoveryCause = "no_space"
	RecoveryCauseIO               RecoveryCause = "io"
	RecoveryCauseAlreadyAttempted RecoveryCause = "already_attempted"
	RecoveryCauseNotEmpty         RecoveryCause = "not_empty"
)

type RecoveryFailure struct {
	At    time.Time     `json:"at"`
	Phase RecoveryPhase `json:"phase"`
	Cause RecoveryCause `json:"cause"`
}

// Recovery is a passive capability fact, never saved inventory or a control.
// It is also the inventory's wire recovery document.
type Recovery struct {
	State   RecoveryState    `json:"state"`
	SavedAt *time.Time       `json:"savedAt,omitempty"`
	Failure *RecoveryFailure `json:"failure,omitempty"`
	Resume  *RecoveryResume  `json:"resume,omitempty"`
}

type RecoveryResume struct {
	RequestedPanes    int `json:"requestedPanes"`
	InferredPanes     int `json:"inferredPanes"`
	SkippedAgentPanes int `json:"skippedAgentPanes"`
}

// RecoveryNotice is the owner-visible recovery copy without the machine label
// and the save time, which each client renders in its own zone.
type RecoveryNotice struct {
	Summary string `json:"summary"`
	Action  string `json:"action,omitempty"`
}

var recoveryPhaseText = map[RecoveryPhase]string{
	RecoveryPhaseOwnership:       "writer lock",
	RecoveryPhaseCheckpointRead:  "checkpoint read",
	RecoveryPhaseSource:          "tmux server check",
	RecoveryPhaseCapture:         "workspace capture",
	RecoveryPhaseCheckpointWrite: "checkpoint save",
	RecoveryPhaseRestore:         "workspace restore",
}

var recoveryCauseText = map[RecoveryCause]string{
	RecoveryCauseTimeout:          "timed out",
	RecoveryCauseChanged:          "workspace or server changed",
	RecoveryCauseUnavailable:      "required resource unavailable",
	RecoveryCauseInvalid:          "invalid data or configuration",
	RecoveryCauseLimit:            "supported limit exceeded",
	RecoveryCauseBusy:             "lock already held",
	RecoveryCausePermission:       "access denied",
	RecoveryCauseNoSpace:          "storage space exhausted",
	RecoveryCauseIO:               "input/output error",
	RecoveryCauseAlreadyAttempted: "prior restore may be incomplete",
	RecoveryCauseNotEmpty:         "replacement server has sessions",
}

// Known reports that Notice can word the fact: a known state and, with a
// failure, a known phase and cause. It is a consumer's check of a peer's
// recovery; the producer also upholds which combinations occur.
func (recovery Recovery) Known() bool {
	switch recovery.State {
	case RecoveryTracking, RecoveryRecovered, RecoveryDelayed, RecoveryBroken:
	default:
		return false
	}
	if recovery.Failure == nil {
		return true
	}
	_, phase := recoveryPhaseText[recovery.Failure.Phase]
	_, cause := recoveryCauseText[recovery.Failure.Cause]
	return phase && cause
}

// Notice is nil while tracking. A delayed or broken recovery names its
// failure's phase and cause; sessions always gives it one.
func (recovery Recovery) Notice() *RecoveryNotice {
	if !recovery.Known() {
		panic("unknown recovery") // justify-defect: sessions publishes and fleetclient admits only Known recoveries.
	}
	var status, action string
	switch recovery.State {
	case RecoveryTracking:
		return nil
	case RecoveryRecovered:
		notice := &RecoveryNotice{Summary: "terminals restored."}
		if counts := recovery.Resume; counts != nil {
			if counts.RequestedPanes > 0 {
				pane := "panes"
				if counts.RequestedPanes == 1 {
					pane = "pane"
				}
				notice.Summary += fmt.Sprintf(" conversation resume requested in %d %s.", counts.RequestedPanes, pane)
			}
			if counts.InferredPanes > 0 {
				request := "requests"
				if counts.InferredPanes == 1 {
					request = "request"
				}
				notice.Summary += fmt.Sprintf(" %d %s used title inference.", counts.InferredPanes, request)
			}
			if counts.SkippedAgentPanes > 0 {
				phrase := "agent panes restored as shells"
				if counts.SkippedAgentPanes == 1 {
					phrase = "agent pane restored as a shell"
				}
				notice.Summary += fmt.Sprintf(" %d %s.", counts.SkippedAgentPanes, phrase)
				notice.Action = "open those terminals for details."
				if counts.SkippedAgentPanes == 1 {
					notice.Action = "open that terminal for details."
				}
			}
		}
		return notice
	case RecoveryDelayed:
		status, action = "workspace saving delayed", "retrying automatically; live terminals remain usable."
	case RecoveryBroken:
		status, action = "workspace recovery stopped", "manual repair required; live terminals remain usable."
	}
	if failure := recovery.Failure; failure != nil {
		status += ". " + recoveryPhaseText[failure.Phase] + ": " + recoveryCauseText[failure.Cause]
	}
	return &RecoveryNotice{Summary: status, Action: action}
}
