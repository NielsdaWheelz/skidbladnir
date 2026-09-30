package agentcontrol

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func validText(text string) bool {
	return text != "" && len(text) <= 32768 && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}

func (service *Service) Send(parent context.Context, conversation agentruntime.Conversation, text, input, delivery string) (SendResult, error) {
	if !validText(text) || input != "peer" && input != "user" || delivery != "direct" && delivery != "queue" || input == "peer" && delivery == "queue" {
		return SendResult{}, ErrInvalidInput
	}
	if delivery == "queue" || conversation.Provider == agentruntime.ProviderClaude {
		return SendResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	profile, native, inspected, _, err := service.bound(ctx, conversation)
	if err != nil {
		return SendResult{}, err
	}
	method := inspected.Methods.SendPeer
	if input == "user" {
		method = inspected.Methods.SendUser
	}
	if method != "native" {
		return SendResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	var result SendResult
	if err := service.native(ctx, profile, "send", []nativeTarget{native}, struct {
		Text     string `json:"text"`
		Input    string `json:"input"`
		Delivery string `json:"delivery"`
	}{text, input, delivery}, &result); err != nil {
		return SendResult{}, err
	}
	if result.Method != "native" || result.Input != input || result.Delivery != delivery || result.Outcome != "accepted" {
		return SendResult{}, &UnavailableError{Dispatch: "unknown"}
	}
	if result.TurnID == "" {
		return SendResult{}, &UnavailableError{Dispatch: "unknown"}
	}
	return result, nil
}

func (service *Service) Stop(parent context.Context, conversation agentruntime.Conversation, turn *agentruntime.Turn) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	profile, native, inspected, observation, err := service.bound(ctx, conversation)
	if err != nil {
		return WriteResult{}, err
	}
	if inspected.Methods.Stop != "native" {
		return WriteResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	if observation.Status.State == "idle" && (observation.Turn == nil || observation.Turn.State != "inProgress") {
		return WriteResult{Method: "native", Outcome: "finished"}, nil
	}
	operation := "stop"
	if profile.Provider == agentruntime.ProviderCodex {
		if turn != nil {
			if turn.State != "inProgress" {
				return WriteResult{}, ErrInvalidInput
			}
			if observation.Turn == nil || observation.Turn.State != "inProgress" || observation.Turn.ID != turn.ID {
				return WriteResult{}, sessions.ErrAgentTargetStale
			}
			native.TurnID = turn.ID
		} else {
			return WriteResult{}, sessions.ErrAgentTargetStale
		}
		operation = "interrupt"
	}

	var result struct {
		Method  string `json:"method"`
		Outcome string `json:"outcome"`
		TurnID  string `json:"turnId,omitempty"`
	}
	if err := service.native(ctx, profile, operation, []nativeTarget{native}, nil, &result); err != nil {
		return WriteResult{}, err
	}
	if result.Method != "native" || native.TurnID != "" && result.TurnID != "" && result.TurnID != native.TurnID {
		return WriteResult{}, &UnavailableError{Dispatch: "unknown"}
	}
	switch result.Outcome {
	case "interrupted", "stopped", "finished", "unknown":
		return WriteResult{Method: result.Method, Outcome: result.Outcome}, nil
	default:
		return WriteResult{}, &UnavailableError{Dispatch: "unknown"}
	}
}
