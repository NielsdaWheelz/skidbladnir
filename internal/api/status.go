package api

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

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

// TerminalUsage is literal displayed provider usage, not a native account report.
type TerminalUsage struct {
	FiveHour *QuotaWindow `json:"fiveHour,omitempty"`
	SevenDay *QuotaWindow `json:"sevenDay,omitempty"`
}

// TerminalFooter is literal displayed provider text, not an execution setting.
type TerminalFooter string

func (footer TerminalFooter) Valid() bool {
	text := string(footer)
	return len(text) > 0 && len(text) <= 1024 && utf8.ValidString(text) && strings.TrimSpace(text) == text &&
		!strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) })
}

// StatusSign identifies the status at a glance. Clients choose its glyph and ink.
type StatusSign string

const (
	SignRequest     StatusSign = "request"
	SignError       StatusSign = "error"
	SignInterrupted StatusSign = "interrupted"
	SignMenu        StatusSign = "menu"
	SignWorking     StatusSign = "working"
	SignIdle        StatusSign = "idle"
	SignUnknown     StatusSign = "unknown"
	SignTerminal    StatusSign = "terminal"
	SignReady       StatusSign = "ready" // client overlay only
)

// StatusPresentation is the host's sign and wording of one observation.
type StatusPresentation struct {
	Sign          StatusSign `json:"sign"`
	WorkContinues bool       `json:"workContinues"`
	Label         string     `json:"label"`
	Detail        string     `json:"detail,omitempty"`
	Evidence      string     `json:"evidence,omitempty"`
	Reason        string     `json:"-"` // explains without terminal content
}

// PresentStatus renders a status. A local agent has no remote connection.
func PresentStatus(status TerminalStatus, localAgent bool) StatusPresentation {
	if !status.Known() {
		panic("unknown terminal status") // justify-defect: agentcontrol produces only Valid statuses and fleetclient admits only Known ones.
	}
	const menu = "menu open"
	working := status.Activity == ActivityWorking
	var view StatusPresentation
	switch {
	case status.Source == SourceUnavailable:
		view.Sign, view.Label = SignUnknown, "status unavailable"
	case !localAgent:
		view.Sign, view.Label = SignTerminal, "terminal"
	case status.Interaction.Request():
		view.Sign, view.Label, view.WorkContinues = SignRequest, status.Interaction.RequestText(), working
		if status.Notice != NoticeNone {
			view.Detail = status.Notice.Text()
		}
	case status.Notice != NoticeNone:
		view.Sign = SignInterrupted
		if status.Notice == NoticeError {
			view.Sign = SignError
		}
		view.Label, view.WorkContinues = status.Notice.Text(), working
		if status.Interaction == InteractionMenu {
			view.Detail = menu
		}
	case status.Interaction == InteractionMenu:
		view.Sign, view.Label, view.WorkContinues = SignMenu, menu, working
	case status.Activity == ActivityStarting || working:
		view.Sign, view.Label = SignWorking, string(status.Activity)
	case status.Activity == ActivityIdle && status.Interaction == InteractionNone:
		view.Sign, view.Label = SignIdle, "idle"
	default:
		view.Sign, view.Label = SignUnknown, "status unknown"
	}
	if localAgent && status.Source == SourceTerminal {
		view.Evidence = "inferred from terminal"
	}
	if view.WorkContinues {
		if view.Detail != "" {
			view.Detail += " · "
		}
		view.Detail += "work continues"
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
	if view.Sign == SignUnknown {
		view.Reason += "; open the terminal to inspect"
	}
	return view
}
