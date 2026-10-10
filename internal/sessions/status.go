package sessions

// TerminalStatus is one screen observation of a terminal's current foreground.
// Activity, interaction and notice are independent facts; none claims task
// success, completion or cancellation. Inventory freshness is its only clock.
type TerminalStatus struct {
	Activity    Activity     `json:"activity"`
	Interaction Interaction  `json:"interaction"`
	Notice      Notice       `json:"notice"`
	Source      StatusSource `json:"source"`
	Reason      StatusReason `json:"reason"`
}

type Activity string

const (
	ActivityStarting Activity = "starting"
	ActivityWorking  Activity = "working"
	ActivityIdle     Activity = "idle"
	ActivityUnknown  Activity = "unknown"
)

type Interaction string

const (
	InteractionNone         Interaction = "none"
	InteractionPermission   Interaction = "permission"
	InteractionQuestion     Interaction = "question"
	InteractionConfirmation Interaction = "confirmation"
	InteractionSetup        Interaction = "setup"
	InteractionInput        Interaction = "input"
	InteractionMenu         Interaction = "menu"
	InteractionUnknown      Interaction = "unknown"
)

type Notice string

const (
	NoticeNone        Notice = "none"
	NoticeInterrupted Notice = "interrupted"
	NoticeError       Notice = "error"
)

type StatusSource string

const (
	SourceTerminal    StatusSource = "terminal"
	SourceUnavailable StatusSource = "unavailable"
)

type StatusReason string

const (
	ReasonRecognized           StatusReason = "recognized"
	ReasonPartial              StatusReason = "partial"
	ReasonLayoutUnknown        StatusReason = "layout_unknown"
	ReasonEvidenceClipped      StatusReason = "evidence_clipped"
	ReasonEvidenceConflict     StatusReason = "evidence_conflict"
	ReasonProviderUnrecognized StatusReason = "provider_unrecognized"
	ReasonRemoteContext        StatusReason = "remote_context"
	ReasonForegroundChanged    StatusReason = "foreground_changed"
	ReasonObservationTimeout   StatusReason = "observation_timeout"
	ReasonCaptureFailed        StatusReason = "capture_failed"
	ReasonProcessFailed        StatusReason = "process_failed"
)

// Failed reports the reasons of a failed observation stage, the only reasons
// of an unavailable status.
func (reason StatusReason) Failed() bool {
	switch reason {
	case ReasonObservationTimeout, ReasonCaptureFailed, ReasonProcessFailed:
		return true
	default:
		return false
	}
}

// Valid admits exactly the closed interaction set.
func (interaction Interaction) Valid() bool {
	switch interaction {
	case InteractionNone, InteractionPermission, InteractionQuestion, InteractionConfirmation,
		InteractionSetup, InteractionInput, InteractionMenu, InteractionUnknown:
		return true
	default:
		return false
	}
}

// Request reports an interaction that asks the human for a response, the one
// owner of the request set; a menu is navigation, not a request.
func (interaction Interaction) Request() bool {
	switch interaction {
	case InteractionPermission, InteractionQuestion, InteractionConfirmation, InteractionSetup, InteractionInput:
		return true
	case InteractionNone, InteractionMenu, InteractionUnknown:
		return false
	}
	panic("invalid terminal interaction") // justify-defect: callers ask only of a Valid status.
}

// Valid admits exactly the closed combinations. Only a failed stage is
// unavailable; recognized, partial and layout_unknown count the classified
// dimensions; clipping and conflict must have cost at least one dimension.
func (status TerminalStatus) Valid() bool {
	switch status.Activity {
	case ActivityStarting, ActivityWorking, ActivityIdle, ActivityUnknown:
	default:
		return false
	}
	if !status.Interaction.Valid() {
		return false
	}
	switch status.Notice {
	case NoticeNone, NoticeInterrupted, NoticeError:
	default:
		return false
	}
	activity := status.Activity != ActivityUnknown
	interaction := status.Interaction != InteractionUnknown
	terminal := status.Source == SourceTerminal
	if status.Reason.Failed() {
		return status.Source == SourceUnavailable && !activity && !interaction && status.Notice == NoticeNone
	}
	switch status.Reason {
	case ReasonProviderUnrecognized, ReasonRemoteContext, ReasonForegroundChanged:
		return terminal && !activity && !interaction && status.Notice == NoticeNone
	case ReasonRecognized:
		return terminal && activity && interaction
	case ReasonPartial:
		return terminal && activity != interaction
	case ReasonLayoutUnknown:
		return terminal && !activity && !interaction
	case ReasonEvidenceClipped, ReasonEvidenceConflict:
		return terminal && (!activity || !interaction)
	default:
		return false
	}
}

// UnknownStatus classifies neither activity nor interaction. The reason
// selects the source: a failed stage is unavailable; every other reason is a
// successful terminal sample.
func UnknownStatus(reason StatusReason) TerminalStatus {
	source := SourceTerminal
	if reason.Failed() {
		source = SourceUnavailable
	}
	status := TerminalStatus{Activity: ActivityUnknown, Interaction: InteractionUnknown, Notice: NoticeNone, Source: source, Reason: reason}
	if !status.Valid() {
		panic("unknown terminal status requires an unclassifying reason") // justify-defect: callers pass a constant reason.
	}
	return status
}
