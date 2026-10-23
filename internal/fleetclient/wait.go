package fleetclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

func (client *Client) wait(parent context.Context, request Request) Result {
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	ctx, cancel := context.WithTimeout(parent, request.WaitTimeout)
	defer cancel()
	resolveContext, cancelResolve := context.WithTimeout(ctx, Timeout)
	resolved, failure := client.resolve(resolveContext, request)
	cancelResolve()
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	if failure != nil {
		return Result{Error: failure}
	}
	captured := resolved.ref
	result := WaitResult{Target: Ref{captured}, Outcome: "timeout"}
	// justify-polling: one foreground waiter watches one captured target. Each
	// sample finishes before the next five-second cadence; no work is mutated.
	for {
		if parent.Err() != nil {
			return Failed("cancelled", "not_sent")
		}
		if ctx.Err() != nil {
			return success(result)
		}
		sampleContext, cancelSample := context.WithTimeout(ctx, Timeout)
		body := map[string]any{"identityToken": captured.IdentityToken, "paneId": captured.PaneID}
		encoded, _ := json.Marshal(body)
		sampled := client.call(sampleContext, resolved.peer, "terminal_inspect", captured.TmuxID, encoded)
		cancelSample()
		if parent.Err() != nil {
			return Failed("cancelled", "not_sent")
		}
		if ctx.Err() != nil {
			return success(result)
		}
		if !sampled.OK {
			if sampled.Error.Code == "TerminalTargetChanged" || sampled.Error.Code == "SessionNotFound" || sampled.Error.Code == "SessionIdentityMismatch" {
				result.Outcome = "target_changed"
				return success(result)
			}
			return sampled
		}
		matched := false
		status := sampled.Value.(api.Inspect).TerminalStatus
		if status.Source == api.SourceUnavailable {
			return Failed("TerminalUnavailable", "not_sent")
		}
		result.TerminalStatus = &status
		// Each state tests its own dimensions; an unknown tested dimension never matches.
		switch request.State {
		case "idle":
			matched = status.Activity == api.ActivityIdle && status.Interaction == api.InteractionNone
		case "working":
			matched = status.Activity == api.ActivityWorking
		case "needs-input":
			// A human request, whatever the activity, notice, source or reason; menus,
			// notices and readiness never qualify (the producer only reports a request
			// from a terminal source). The sample is fresh by construction.
			matched = status.Interaction.Request()
		default:
			panic("terminal wait state was not validated") // justify-defect: skid parse admits only these three states.
		}
		if matched {
			result.Outcome = "matched"
			return success(result)
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			if parent.Err() != nil {
				return Failed("cancelled", "not_sent")
			}
			return success(result)
		case <-timer.C:
		}
	}
}
