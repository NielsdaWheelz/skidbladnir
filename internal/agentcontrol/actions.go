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

func (service *Service) Send(parent context.Context, target sessions.AgentTarget, text, input, delivery string) (SendResult, error) {
	if !validText(text) || input != "peer" && input != "user" || delivery != "direct" && delivery != "queue" || input == "peer" && delivery == "queue" {
		return SendResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	profile, native, inspected, _, err := service.bound(ctx, target)
	if err != nil {
		return SendResult{}, err
	}
	method := inspected.Methods.SendPeer
	if input == "user" {
		method = inspected.Methods.SendUser
	}
	if delivery == "queue" {
		method = inspected.Methods.QueueUser
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
	if delivery == "direct" && result.TurnID == "" || delivery == "queue" && (result.QueueItemID == "" || result.ClientMessageID == "") {
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

func (service *Service) Stop(parent context.Context, target sessions.AgentTarget, method string) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	result, _, err := service.halt(ctx, target, method)
	return result, err
}

func (service *Service) halt(ctx context.Context, target sessions.AgentTarget, method string) (WriteResult, bool, error) {
	if method != "native" && method != "terminal" {
		return WriteResult{}, false, ErrInvalidInput
	}
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return WriteResult{}, false, err
	}
	if method == "terminal" {
		_, _, inspected, observation, inspectErr := service.inspect(ctx, session)
		if inspectErr == nil {
			if target.Binding != nil && !target.Binding.Equal(observation.Binding) {
				return WriteResult{}, false, sessions.ErrAgentTargetStale
			}
			if inspected.Methods.Stop != "terminal" {
				return WriteResult{}, false, &UnavailableError{Dispatch: "not_sent"}
			}
		}
		result, err := terminalWriteResult(service.sessions.AgentKeys(ctx, target, []string{interruptKey(session.Agent.Provider)}))
		return result, inspectErr == nil && inspected.TerminalOwnsAgent, err
	}
	profile, native, inspected, observation, err := service.bound(ctx, target)
	if err != nil {
		return WriteResult{}, false, err
	}
	if inspected.Methods.Stop != "native" {
		return WriteResult{}, false, &UnavailableError{Dispatch: "not_sent"}
	}
	if observation.Status.State == "idle" && (observation.Turn == nil || observation.Turn.State != "inProgress") {
		return WriteResult{Method: "native", Outcome: "finished"}, inspected.TerminalOwnsAgent, nil
	}
	operation := "stop"
	if profile.Provider == agentruntime.ProviderCodex {
		if target.Turn == nil || target.Turn.State != "inProgress" {
			return WriteResult{}, false, ErrInvalidInput
		}
		if observation.Turn == nil || observation.Turn.State != "inProgress" || observation.Turn.ID != target.Turn.ID {
			return WriteResult{}, false, sessions.ErrAgentTargetStale
		}
		native.TurnID = target.Turn.ID
		operation = "interrupt"
	}
	var result struct {
		Method  string `json:"method"`
		Outcome string `json:"outcome"`
		TurnID  string `json:"turnId,omitempty"`
	}
	if err := service.native(ctx, profile, operation, []nativeTarget{native}, nil, &result); err != nil {
		return WriteResult{}, false, err
	}
	if result.Method != "native" || native.TurnID != "" && result.TurnID != "" && result.TurnID != native.TurnID {
		return WriteResult{}, false, &UnavailableError{Dispatch: "unknown"}
	}
	switch result.Outcome {
	case "interrupted", "stopped", "finished", "unknown":
		return WriteResult{Method: result.Method, Outcome: result.Outcome}, inspected.TerminalOwnsAgent, nil
	default:
		return WriteResult{}, false, &UnavailableError{Dispatch: "unknown"}
	}
}

// Halt and terminal closure have independent effects. Revalidation protects the
// captured process/view; provider calls never hold the terminal manager's lock.
func (service *Service) Close(parent context.Context, target sessions.AgentTarget, method string, closeTerminal func(context.Context, sessions.AgentTarget) error) (CloseResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return CloseResult{}, err
	}
	haltContext, cancelHalt := context.WithTimeout(ctx, 8*time.Second)
	halted, terminalOwnsAgent, haltErr := service.halt(haltContext, target, method)
	if haltErr != nil {
		var unavailable *UnavailableError
		if !errors.As(haltErr, &unavailable) || unavailable.Dispatch != "unknown" {
			cancelHalt()
			return CloseResult{}, haltErr
		}
	}
	result := CloseResult{Agent: "unconfirmed", Terminal: "unconfirmed"}
	if halted.Outcome == "interrupted" || halted.Outcome == "stopped" || halted.Outcome == "finished" {
		result.Agent = halted.Outcome
	}
	exited := service.sessions.AgentProcessExited(target)
	if !exited && target.Binding != nil {
		if _, _, _, _, err := service.bound(haltContext, target); err != nil {
			cancelHalt()
			result.Reason = "unavailable"
			if errors.Is(err, sessions.ErrAgentTargetStale) {
				result.Reason = "stale"
			}
			return result, nil
		}
	}
	cancelHalt()
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
	if session.Agent.Provider == agentruntime.ProviderClaude && terminalOwnsAgent && service.sessions.AgentProcessExited(target) {
		result.Agent = "stopped"
	}
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
		panic("invalid agent provider")
	} // justify-defect: session projection admits only validated providers.
}
