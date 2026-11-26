package agentcontrol

import (
	"context"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/managedworker"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func sampleWorker(ctx context.Context, socket string) detection {
	value, err := managedworker.Call(ctx, socket, managedworker.Request{Op: "status"})
	if err != nil {
		return failedDetection(ctx, err)
	}
	status := api.TerminalStatus{Source: api.SourceTerminal, Reason: api.ReasonRecognized, Notice: api.NoticeNone, Interaction: api.InteractionNone}
	switch value.State {
	case "idle":
		status.Activity = api.ActivityIdle
	case "working", "needs-input":
		status.Activity = api.ActivityWorking
	default:
		return detection{status: api.UnknownStatus(api.ReasonCaptureFailed)}
	}
	switch value.Interaction {
	case "":
	case "approval":
		status.Interaction = api.InteractionPermission
	case "question":
		status.Interaction = api.InteractionQuestion
	default:
		return detection{status: api.UnknownStatus(api.ReasonCaptureFailed)}
	}
	composerState := composerEmpty
	if value.Draft {
		composerState = composerDraft
	}
	return detection{status: status, composer: composerState, rules: []string{"worker.status"}}
}

func (service *Service) sendWorker(ctx context.Context, expected sessions.Session, target sessions.TerminalTarget, text string) (api.WriteResult, error) {
	current, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return api.WriteResult{}, err
	}
	if current.Agent == nil || *current.Agent != *expected.Agent {
		return api.WriteResult{}, sessions.ErrTerminalTargetChanged
	}
	value, err := managedworker.Call(ctx, current.Agent.WorkerSocket, managedworker.Request{Op: "peer", Brief: text})
	if err != nil {
		return terminalWriteResult(sessions.ErrTerminalWriteUnknown)
	}
	switch {
	case value.State == "accepted":
		return terminalWriteResult(nil)
	case value.State == "failed":
		return api.WriteResult{}, &TerminalInputBlockedError{Reason: value.Reason}
	default:
		return terminalWriteResult(sessions.ErrTerminalWriteUnknown)
	}
}
