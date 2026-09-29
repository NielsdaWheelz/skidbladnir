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
	captured, observed, failure := client.resolve(resolveContext, request)
	cancelResolve()
	if parent.Err() != nil {
		return Failed("cancelled", "not_sent")
	}
	if failure != nil {
		return *failure
	}
	result := WaitResult{Target: captured.Encode(), Outcome: "timeout"}
	if captured.Agent == nil || captured.Agent.Binding == nil {
		return Failed("AgentUnavailable", "not_sent")
	}
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
		if observed.Machine == "" {
			sampleContext, cancelSample := context.WithTimeout(ctx, Timeout)
			sampled := client.list(sampleContext, target.Label)
			cancelSample()
			if parent.Err() != nil {
				return Failed("cancelled", "not_sent")
			}
			if ctx.Err() != nil {
				return success(result)
			}
			if !sampled.OK {
				return sampled
			}
			inventory := sampled.Value.(Inventory)
			if inventory.Partial {
				return Failed("AgentUnavailable", "not_sent")
			}
			for _, peer := range inventory.Peers {
				for _, session := range peer.Sessions {
					ref, _ := DecodeReference(session.Ref)
					if ref.SessionEqual(captured) {
						observed = ObservedSession{Machine: peer.Machine, Session: session}
						break
					}
				}
			}
		}
		current, err := DecodeReference(observed.Session.Ref)
		if err != nil || !current.SessionEqual(captured) || current.Agent == nil || current.Agent.PID != captured.Agent.PID || current.Agent.StartIdentity != captured.Agent.StartIdentity || current.Agent.PaneID != captured.Agent.PaneID {
			return success(WaitResult{Target: result.Target, Outcome: "target_changed"})
		}
		agent := observed.Session.Agent
		if agent.Status.Source != "native" || current.Agent.Binding == nil {
			return Failed("AgentUnavailable", "not_sent")
		}
		if !current.Agent.Binding.Equal(*captured.Agent.Binding) {
			return success(WaitResult{Target: result.Target, Outcome: "target_changed"})
		}
		result.Observation = &agentruntime.Observation{Binding: *agent.Binding, Status: agent.Status, Turn: agent.Turn}
		if agent.Status.State == state {
			result.Outcome = "matched"
			return success(result)
		}
		observed = ObservedSession{}
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
		IdentityToken string                    `json:"identityToken"`
		Conversation  agentruntime.Conversation `json:"conversation"`
		Cursor        string                    `json:"cursor,omitempty"`
	}{ref.IdentityToken, conversation, cursor}
	encoded, _ := json.Marshal(body)
	if len(encoded) > MaximumInputBytes {
		return Failed("input_limit", "not_sent")
	}
	result := client.call(ctx, target, "results", "/v1/sessions/"+ref.TmuxID+"/agent/results", encoded)
	if result.OK && result.Value.(ResultsResult).Conversation != conversation {
		return Failed("protocol_error", "not_sent")
	}
	return result
}
