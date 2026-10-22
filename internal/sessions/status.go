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
	panic("invalid terminal interaction") // justify-defect: callers ask only of a Known status.
}

// Known reports that every dimension is a value of its closed set, the check a
// consumer of a peer's status makes before branching on it; Valid also checks
// the combination.
func (status TerminalStatus) Known() bool {
	switch status.Activity {
	case ActivityStarting, ActivityWorking, ActivityIdle, ActivityUnknown:
	default:
		return false
	}
	switch status.Notice {
	case NoticeNone, NoticeInterrupted, NoticeError:
	default:
		return false
	}
	switch status.Source {
	case SourceTerminal, SourceUnavailable:
	default:
		return false
	}
	switch status.Reason {
	case ReasonRecognized, ReasonPartial, ReasonLayoutUnknown, ReasonEvidenceClipped, ReasonEvidenceConflict,
		ReasonProviderUnrecognized, ReasonRemoteContext, ReasonForegroundChanged,
		ReasonObservationTimeout, ReasonCaptureFailed, ReasonProcessFailed:
	default:
		return false
	}
	return status.Interaction.Valid()
}

// RequestText names a request; it panics outside the request set: only
// admitted requests reach copy.
func (interaction Interaction) RequestText() string {
	switch interaction {
	case InteractionPermission:
		return "needs permission"
	case InteractionQuestion:
		return "needs answer"
	case InteractionSetup:
		return "needs setup"
	case InteractionConfirmation:
		return "needs review"
	case InteractionInput:
		return "needs input"
	default:
		panic("request without content")
	}
}

// Text names an attention notice; it panics outside them: only admitted
// notices reach copy.
func (notice Notice) Text() string {
	switch notice {
	case NoticeInterrupted:
		return "interruption shown"
	case NoticeError:
		return "error shown"
	default:
		panic("notice without content")
	}
}

// Valid admits exactly the closed combinations. Only a failed stage is
// unavailable; recognized, partial and layout_unknown count the classified
// dimensions; clipping and conflict must have cost at least one dimension.
func (status TerminalStatus) Valid() bool {
	if !status.Known() {
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

// StatusTone is a status label's colour role; renderers own the palette and
// never key colour on printed copy.
type StatusTone string

const (
	ToneWorking   StatusTone = "working"
	ToneAttention StatusTone = "attention"
	ToneReady     StatusTone = "ready" // clients only: the hub's pending ready over an eligible row
	ToneMuted     StatusTone = "muted"
)

// StatusQueue is human-collection membership before client overlays, never
// control admission. Ready means eligible: a client keeps it only while its
// inventory is fresh and the hub holds a pending ready.
type StatusQueue string

const (
	QueueNone   StatusQueue = "none"
	QueueReady  StatusQueue = "ready"
	QueueAction StatusQueue = "action"
)

// StatusPresentation is the owner-visible copy of one observation, rendered as
// if fresh and not ready; clients add staleness and the hub's ready
// (terminal-observation.md §6).
type StatusPresentation struct {
	Label       string      `json:"label"`
	Tone        StatusTone  `json:"tone"`
	Queue       StatusQueue `json:"queue"`
	Detail      string      `json:"detail,omitempty"`
	Evidence    string      `json:"evidence,omitempty"`
	QueueNotice string      `json:"queueNotice,omitempty"` // the notice a menu label hides
	Reason      string      `json:"-"`                     // desktop only; explains without terminal content
}

// PresentStatus renders a status; localAgent is an agent row without a
// connection. The first matching row names it: overlay marks a request, menu
// or notice row, which outranks visible activity; inspect marks the rows only
// the terminal itself can settle. The idle row is reached only for a local
// terminal sample without a notice, so it is exactly ready-eligible.
func PresentStatus(status TerminalStatus, localAgent bool) StatusPresentation {
	if !status.Known() {
		panic("unknown terminal status") // justify-defect: agentcontrol produces only Valid statuses and fleetclient admits only Known ones.
	}
	view := StatusPresentation{Tone: ToneMuted, Queue: QueueNone}
	inferred := localAgent && status.Source == SourceTerminal
	overlay, inspect := false, false
	switch {
	case status.Source == SourceUnavailable:
		view.Label, inspect = "status unavailable", true
	case !localAgent:
		view.Label = "terminal"
	case status.Interaction.Request():
		view.Label, view.Tone, overlay = status.Interaction.RequestText(), ToneAttention, true
	case status.Interaction == InteractionMenu:
		view.Label, overlay = "menu open", true
	case status.Notice == NoticeInterrupted:
		view.Label, overlay = status.Notice.Text(), true
	case status.Notice == NoticeError:
		view.Label, view.Tone, overlay = status.Notice.Text(), ToneAttention, true
	case status.Activity == ActivityStarting:
		view.Label, view.Tone = "starting", ToneWorking
	case status.Activity == ActivityWorking:
		view.Label, view.Tone = "working", ToneWorking
	case status.Activity == ActivityIdle && status.Interaction == InteractionNone:
		view.Label, view.Queue = "idle", QueueReady
	default:
		view.Label, inspect = "status unknown", true
	}
	if inferred {
		view.Evidence = "inferred from terminal"
		if status.Interaction.Request() || status.Notice != NoticeNone {
			view.Queue = QueueAction
			if status.Interaction == InteractionMenu {
				view.QueueNotice = status.Notice.Text()
			}
		}
	}
	if overlay && status.Activity == ActivityWorking {
		view.Detail = "work continues"
	}
	switch status.Reason {
	case ReasonRecognized:
		view.Reason = "terminal controls recognized"
	case ReasonPartial:
		view.Reason = "some terminal controls were not recognized"
	case ReasonLayoutUnknown:
		view.Reason = "terminal layout not recognized"
	case ReasonEvidenceClipped:
		view.Reason = "required screen region clipped"
	case ReasonEvidenceConflict:
		view.Reason = "current screen signals conflict"
	case ReasonProviderUnrecognized:
		view.Reason = "foreground program not recognized as a local agent"
	case ReasonRemoteContext:
		view.Reason = "remote shell; agent status unknown"
	case ReasonForegroundChanged:
		view.Reason = "foreground changed during observation"
	case ReasonObservationTimeout:
		view.Reason = "terminal observation timed out"
	case ReasonCaptureFailed:
		view.Reason = "terminal screen could not be captured"
	case ReasonProcessFailed:
		view.Reason = "foreground process could not be identified"
	}
	if inspect {
		view.Reason += "; open the terminal to inspect"
	}
	return view
}
