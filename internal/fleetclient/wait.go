package fleetclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func (client *Client) wait(parent context.Context, request Request) Result {
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	if !request.Valid() {
		return Failed("invalid_input", "not_sent")
	}
	ctx, cancel := context.WithTimeout(parent, request.WaitTimeout)
	defer cancel()
	resolveContext, cancelResolve := context.WithTimeout(ctx, Timeout)
	captured, _, failure := client.resolve(resolveContext, request)
	cancelResolve()
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	if failure != nil {
		return *failure
	}
	result := WaitResult{Target: captured.Encode(), Outcome: "timeout"}
	target, _ := client.peerByMachine(captured.Machine)
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
		path := "/v1/sessions/" + captured.TmuxID + "/terminal/inspect"
		body := map[string]any{"identityToken": captured.IdentityToken, "paneId": captured.PaneID}
		encoded, _ := json.Marshal(body)
		sampled := client.call(sampleContext, target, "terminal_inspect", path, encoded)
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
		status := sampled.Value.(TerminalInspectResult).TerminalStatus
		if status.Source == sessions.SourceUnavailable {
			return Failed("TerminalUnavailable", "not_sent")
		}
		result.TerminalStatus = &status
		// Each state tests its own dimensions; an unknown tested dimension never matches.
		switch request.State {
		case "idle":
			matched = status.Activity == sessions.ActivityIdle && status.Interaction == sessions.InteractionNone
		case "working":
			matched = status.Activity == sessions.ActivityWorking
		case "needs-input":
			matched = NeedsInput(status)
		default:
			panic("terminal wait state was not validated") // justify-defect: Request.Valid admits only terminal states for terminal targets.
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
