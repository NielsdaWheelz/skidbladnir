package fleetclient

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// Tone is a status label's colour role, named for the palette it selects.
// Renderers own the palette; colour and behaviour never key on printed copy.
type Tone int

const (
	ToneMuted Tone = iota
	ToneFrost
	ToneEmber
	ToneMoss
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
	Tone  Tone
	Queue QueueCategory
	// Detail is the label with its secondary facts, for the existing detail space.
	Detail string
	// QueueDetail is the notice phrase a menu hides. Queue rows append it to
	// their label and detail so their inclusion is visible without selection.
	QueueDetail string
	// Reason explains the observation without terminal content.
	Reason string
}

// ProjectStatus names a session's terminal status. fresh is inventory
// freshness; ready is the notification owner's pending attention. A stale
// view names its last observation and is never ready.
func ProjectStatus(session Session, fresh, ready bool) StatusView {
	status := session.TerminalStatus
	if !status.Valid() {
		panic("invalid owned terminal status") // justify-defect: ingress admits only Valid statuses.
	}
	local := session.Agent != nil && session.Connection == nil
	queue := QueueExcluded
	if fresh && local && status.Source == sessions.SourceTerminal {
		switch {
		case NeedsInput(status) || status.Notice != sessions.NoticeNone:
			queue = QueueAction
		case status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone && ready:
			queue = QueueReady
		}
	}
	// The first matching row names the status. overlay marks a request, menu or
	// notice row, which outranks visible activity; inspect marks the rows only
	// the terminal itself can settle.
	label, tone, overlay, inspect := "", ToneMuted, false, false
	switch {
	case status.Source == sessions.SourceUnavailable:
		label, inspect = "status unavailable", true
	case !local:
		label = "terminal"
	case status.Interaction == sessions.InteractionPermission:
		label, tone, overlay = attention.RequestText(sessions.InteractionPermission), ToneEmber, true
	case status.Interaction == sessions.InteractionQuestion:
		label, tone, overlay = attention.RequestText(sessions.InteractionQuestion), ToneEmber, true
	case status.Interaction == sessions.InteractionSetup:
		label, tone, overlay = attention.RequestText(sessions.InteractionSetup), ToneEmber, true
	case status.Interaction == sessions.InteractionConfirmation:
		label, tone, overlay = attention.RequestText(sessions.InteractionConfirmation), ToneEmber, true
	case status.Interaction == sessions.InteractionInput:
		label, tone, overlay = attention.RequestText(sessions.InteractionInput), ToneEmber, true
	case status.Interaction == sessions.InteractionMenu:
		label, overlay = "menu open", true
	case status.Notice == sessions.NoticeInterrupted:
		label, overlay = attention.NoticeText(sessions.NoticeInterrupted), true
	case status.Notice == sessions.NoticeError:
		label, tone, overlay = attention.NoticeText(sessions.NoticeError), ToneEmber, true
	case status.Activity == sessions.ActivityStarting:
		label, tone = "starting", ToneFrost
	case status.Activity == sessions.ActivityWorking:
		label, tone = "working", ToneFrost
	case queue == QueueReady:
		label, tone = "ready", ToneMoss
	case status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone:
		label = "idle"
	default:
		label, inspect = "status unknown", true
	}
	if !fresh {
		label, tone = "last observed: "+label, ToneMuted
	}
	detail := label
	queueDetail := ""
	if queue == QueueAction && status.Interaction == sessions.InteractionMenu {
		switch status.Notice {
		case sessions.NoticeError:
			queueDetail = attention.NoticeText(sessions.NoticeError)
		case sessions.NoticeInterrupted:
			queueDetail = attention.NoticeText(sessions.NoticeInterrupted)
		}
	}
	if overlay && status.Activity == sessions.ActivityWorking {
		detail += " · work continues"
	}
	if local && status.Source == sessions.SourceTerminal {
		detail += " · inferred from terminal"
	}
	var reason string
	switch status.Reason {
	case sessions.ReasonRecognized:
		reason = "terminal controls recognized"
	case sessions.ReasonPartial:
		reason = "some terminal controls were not recognized"
	case sessions.ReasonLayoutUnknown:
		reason = "terminal layout not recognized"
	case sessions.ReasonEvidenceClipped:
		reason = "required screen region clipped"
	case sessions.ReasonEvidenceConflict:
		reason = "current screen signals conflict"
	case sessions.ReasonProviderUnrecognized:
		reason = "foreground program not recognized as a local agent"
	case sessions.ReasonRemoteContext:
		reason = "remote shell; agent status unknown"
	case sessions.ReasonForegroundChanged:
		reason = "foreground changed during observation"
	case sessions.ReasonObservationTimeout:
		reason = "terminal observation timed out"
	case sessions.ReasonCaptureFailed:
		reason = "terminal screen could not be captured"
	case sessions.ReasonProcessFailed:
		reason = "foreground process could not be identified"
	default:
		panic("terminal status reason without copy") // justify-defect: the copy covers every reason Valid admits.
	}
	if inspect {
		reason += "; open the terminal to inspect"
	}
	return StatusView{Label: label, Tone: tone, Queue: queue, Detail: detail, QueueDetail: queueDetail, Reason: reason}
}

// NeedsInput reports a human request interaction, whatever the activity,
// notice or reason; menus, notices and readiness never qualify. A request
// already implies a terminal source (TerminalStatus.Valid). Callers own
// freshness: a stale observation never needs input.
func NeedsInput(status sessions.TerminalStatus) bool {
	return status.Interaction.Request()
}
