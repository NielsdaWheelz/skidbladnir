package agentcontrol

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
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

func (service *Service) Text(parent context.Context, target sessions.AgentTarget, text string) (WriteResult, error) {
	if !validText(text) {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return terminalWriteResult(service.sessions.SendAgent(ctx, target, text))
}

func (service *Service) Keys(parent context.Context, target sessions.AgentTarget, keys []string) (WriteResult, error) {
	if len(keys) < 1 || len(keys) > 16 {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return terminalWriteResult(service.sessions.AgentKeys(ctx, target, keys))
}

func terminalWriteResult(err error) (WriteResult, error) {
	if err == nil {
		return WriteResult{Method: "terminal", Outcome: "written"}, nil
	}
	if errors.Is(err, sessions.ErrAgentWriteUnknown) {
		return WriteResult{Method: "terminal", Outcome: "unknown"}, nil
	}
	if errors.Is(err, tmuxclient.ErrInputInvalid) {
		return WriteResult{}, ErrInvalidInput
	}
	return WriteResult{}, err
}

func (service *Service) Stop(parent context.Context, conversation agentruntime.Conversation, turn *agentruntime.Turn) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return service.halt(ctx, conversation, turn)
}

func (service *Service) halt(ctx context.Context, conversation agentruntime.Conversation, turn *agentruntime.Turn) (WriteResult, error) {
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

func (service *Service) TerminalStop(parent context.Context, target sessions.AgentTarget) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	return terminalWriteResult(service.sessions.AgentKeys(ctx, target, []string{interruptKey(session.Agent.Provider)}))
}

// Conversation halt and terminal closure have independent targets and effects.
func (service *Service) Close(parent context.Context, conversation agentruntime.Conversation, turn *agentruntime.Turn, closeTerminal func(context.Context) error) (CloseResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	haltContext, cancelHalt := context.WithTimeout(ctx, 8*time.Second)
	halted, haltErr := service.halt(haltContext, conversation, turn)
	cancelHalt()
	if errors.Is(haltErr, ErrInvalidInput) {
		return CloseResult{}, haltErr
	}
	result := CloseResult{Agent: "unconfirmed", Terminal: "unconfirmed"}
	if halted.Outcome == "interrupted" || halted.Outcome == "stopped" || halted.Outcome == "finished" {
		result.Agent = halted.Outcome
	}
	closeContext, cancelClose := context.WithTimeout(ctx, 2*time.Second)
	defer cancelClose()
	if err := closeTerminal(closeContext); err != nil {
		result.Reason = "unavailable"
		if isSessionIdentityMismatch(err) {
			result.Reason = "stale"
		}
		return result, nil
	}
	result.Terminal = "closed"
	return result, nil
}

func (service *Service) TerminalClose(parent context.Context, target sessions.AgentTarget, closeTerminal func(context.Context, sessions.AgentTarget) error) (CloseResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	haltContext, cancelHalt := context.WithTimeout(ctx, 8*time.Second)
	halted, err := service.TerminalStop(haltContext, target)
	cancelHalt()
	if err != nil {
		return CloseResult{}, err
	}
	result := CloseResult{Agent: "unconfirmed", Terminal: "unconfirmed"}
	if halted.Outcome != "written" {
		result.Reason = "unavailable"
	}
	closeContext, cancelClose := context.WithTimeout(ctx, 2*time.Second)
	defer cancelClose()
	if err := closeTerminal(closeContext, target); err != nil {
		result.Reason = "unavailable"
		if errors.Is(err, sessions.ErrAgentTargetStale) || isSessionIdentityMismatch(err) {
			result.Reason = "stale"
		}
		return result, nil
	}
	result.Terminal = "closed"
	return result, nil
}

func isSessionIdentityMismatch(err error) bool {
	var failure *sessions.Error
	return errors.As(err, &failure) && failure.Code == sessions.ErrorSessionIdentityMismatch
}

func interruptKey(provider agentruntime.Provider) string {
	switch provider {
	case agentruntime.ProviderCodex:
		return "escape"
	case agentruntime.ProviderClaude:
		return "ctrl-c"
	default:
		panic("invalid agent provider") // justify-defect: foreground projection admits only validated providers.
	}
}
