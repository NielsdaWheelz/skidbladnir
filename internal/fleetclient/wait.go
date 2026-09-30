package fleetclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func (client *Client) wait(parent context.Context, request Request) Result {
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	if !request.Valid() {
		return Failed("invalid_input", "not_sent")
	}
	timeout := request.WaitTimeout
	if timeout == 0 {
		timeout = time.Minute
	}
	state := request.State
	if state == "" {
		state = "idle"
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
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
		operation, path := "terminal_inspect", "/v1/sessions/"+captured.TmuxID+"/terminal/inspect"
		body := map[string]any{"identityToken": captured.IdentityToken, "paneId": captured.PaneID}
		if captured.Conversation != nil {
			operation, path = "inspect", "/v1/conversations/inspect"
			body = map[string]any{"conversation": captured.Conversation.Binding.Conversation}
		}
		encoded, _ := json.Marshal(body)
		sampled := client.call(sampleContext, target, operation, path, encoded)
		cancelSample()
		if parent.Err() != nil {
			return Failed("cancelled", "not_sent")
		}
		if ctx.Err() != nil {
			return success(result)
		}
		if !sampled.OK {
			if sampled.Error.Code == "AgentTargetStale" || sampled.Error.Code == "TerminalTargetChanged" || sampled.Error.Code == "SessionNotFound" || sampled.Error.Code == "SessionIdentityMismatch" {
				result.Outcome = "target_changed"
				return success(result)
			}
			return sampled
		}
		matched := false
		if captured.Conversation == nil {
			status := sampled.Value.(TerminalInspectResult).TerminalStatus
			if status.Source == "unavailable" {
				return Failed("TerminalUnavailable", "not_sent")
			}
			result.TerminalStatus = &status
			matched = status.State == state
		} else {
			runtime := sampled.Value.(agentruntime.ConversationRuntime)
			if !runtime.Binding.Equal(captured.Conversation.Binding) {
				result.Outcome = "target_changed"
				return success(result)
			}
			if runtime.Status.Source != "native" {
				return Failed("AgentUnavailable", "not_sent")
			}
			result.Observation = &agentruntime.Observation{Binding: runtime.Binding, Status: runtime.Status, Turn: runtime.Turn}
			matched = runtime.Status.State == state
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

func (client *Client) Results(parent context.Context, ref Reference, conversation agentruntime.Conversation, cursor string) Result {
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	target, found := client.peerByMachine(ref.Machine)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	if !conversation.Valid() {
		return Failed("invalid_input", "not_sent")
	}
	body := struct {
		Conversation agentruntime.Conversation `json:"conversation"`
		Cursor       string                    `json:"cursor,omitempty"`
	}{conversation, cursor}
	encoded, _ := json.Marshal(body)
	if len(encoded) > MaximumInputBytes {
		return Failed("input_limit", "not_sent")
	}
	result := client.call(ctx, target, "results", "/v1/conversations/results", encoded)
	if result.OK && result.Value.(ResultsResult).Conversation != conversation {
		return Failed("protocol_error", "not_sent")
	}
	return result
}
