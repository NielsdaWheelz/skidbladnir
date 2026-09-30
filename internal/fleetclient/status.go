package fleetclient

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// Tone is a status label's colour role. Renderers own the palette; colour and
// behaviour never key on printed copy.
type Tone int

const (
	ToneMuted Tone = iota
	ToneBlue
	ToneEmber
	ToneGreen
)

// StatusView is the one terminal-status projection shared by the cli and the
// desktop browser. It derives from observed facts only.
type StatusView struct {
	Label string
	Tone  Tone
	// WorkContinues marks a request, menu or notice shown over working activity.
	WorkContinues bool
	// Inferred marks a local agent whose status was read from its terminal.
	Inferred bool
	// Reason explains the observation without terminal content.
	Reason string
}

// Detail is the label with its secondary facts, for the existing detail space.
func (view StatusView) Detail() string {
	text := view.Label
	if view.WorkContinues {
		text += " · work continues"
	}
	if view.Inferred {
		text += " · inferred from terminal"
	}
	return text
}

// ProjectStatus names a session's terminal status. fresh is inventory
// freshness; ready is the notification owner's pending attention. A stale
// view names its last observation and is never ready.
func ProjectStatus(session Session, fresh, ready bool) StatusView {
	status := session.TerminalStatus
	local := session.Agent != nil && session.Connection == nil
	view := StatusView{Tone: ToneMuted, Inferred: local && status.Source == sessions.SourceTerminal}
	switch {
	case status.Source == sessions.SourceUnavailable:
		view.Label = "status unavailable"
	case !local:
		view.Label = "terminal"
	default:
		overlay := false
		view.Label, view.Tone, overlay = agentStatus(status, fresh && ready)
		view.WorkContinues = overlay && status.Activity == sessions.ActivityWorking
	}
	if !fresh {
		view.Label, view.Tone = "last observed: "+view.Label, ToneMuted
	}
	switch status.Reason {
	case sessions.ReasonRecognized:
		view.Reason = "terminal controls recognized"
	case sessions.ReasonPartial:
		view.Reason = "some terminal controls were not recognized"
	case sessions.ReasonLayoutUnknown:
		view.Reason = "terminal layout not recognized"
	case sessions.ReasonEvidenceClipped:
		view.Reason = "required screen region clipped"
	case sessions.ReasonEvidenceConflict:
		view.Reason = "current screen signals conflict"
	case sessions.ReasonProviderUnrecognized:
		view.Reason = "foreground program not recognized as a local agent"
	case sessions.ReasonRemoteContext:
		view.Reason = "remote shell; agent status unknown"
	case sessions.ReasonForegroundChanged:
		view.Reason = "foreground changed during observation"
	case sessions.ReasonObservationTimeout:
		view.Reason = "terminal observation timed out"
	case sessions.ReasonCaptureFailed:
		view.Reason = "terminal screen could not be captured"
	case sessions.ReasonProcessFailed:
		view.Reason = "foreground process could not be identified"
	default:
		panic("invalid owned terminal status reason") // justify-defect: ingress admits only Valid statuses.
	}
	// Only the terminal itself can settle what an agent's screen withheld. An
	// ordinary shell or remote connection has no agent status to inspect.
	if (local || status.Source == sessions.SourceUnavailable) && (status.Activity == sessions.ActivityUnknown || status.Interaction == sessions.InteractionUnknown) {
		view.Reason += "; open the terminal to inspect"
	}
	return view
}

// agentStatus is the projection for a local agent's terminal sample. overlay
// reports a request, menu or notice row, which outranks visible activity.
func agentStatus(status sessions.TerminalStatus, ready bool) (label string, tone Tone, overlay bool) {
	switch status.Interaction {
	case sessions.InteractionPermission:
		return "needs permission", ToneEmber, true
	case sessions.InteractionQuestion:
		return "needs answer", ToneEmber, true
	case sessions.InteractionSetup:
		return "needs setup", ToneEmber, true
	case sessions.InteractionConfirmation:
		return "needs review", ToneEmber, true
	case sessions.InteractionInput:
		return "needs input", ToneEmber, true
	case sessions.InteractionMenu:
		return "menu open", ToneMuted, true
	case sessions.InteractionNone, sessions.InteractionUnknown:
	default:
		panic("invalid owned terminal interaction") // justify-defect: ingress admits only Valid statuses.
	}
	switch status.Notice {
	case sessions.NoticeInterrupted:
		return "interruption shown", ToneMuted, true
	case sessions.NoticeError:
		return "error shown", ToneEmber, true
	case sessions.NoticeNone:
	default:
		panic("invalid owned terminal notice") // justify-defect: ingress admits only Valid statuses.
	}
	switch status.Activity {
	case sessions.ActivityStarting:
		return "starting", ToneBlue, false
	case sessions.ActivityWorking:
		return "working", ToneBlue, false
	case sessions.ActivityIdle:
		switch {
		case status.Interaction == sessions.InteractionUnknown:
			return "status unknown", ToneMuted, false
		case ready:
			return "ready", ToneGreen, false
		}
		return "idle", ToneMuted, false
	case sessions.ActivityUnknown:
		return "status unknown", ToneMuted, false
	}
	panic("invalid owned terminal activity") // justify-defect: ingress admits only Valid statuses.
}

// NeedsInput reports a fresh observation of a human request, whatever its
// activity, notice or reason; menus, notices and readiness never qualify. A
// request interaction already implies a terminal source (TerminalStatus.Valid).
func NeedsInput(status sessions.TerminalStatus, fresh bool) bool {
	switch status.Interaction {
	case sessions.InteractionPermission, sessions.InteractionQuestion, sessions.InteractionConfirmation, sessions.InteractionSetup, sessions.InteractionInput:
		return fresh
	case sessions.InteractionNone, sessions.InteractionMenu, sessions.InteractionUnknown:
		return false
	}
	panic("invalid owned terminal interaction") // justify-defect: ingress admits only Valid statuses.
}
