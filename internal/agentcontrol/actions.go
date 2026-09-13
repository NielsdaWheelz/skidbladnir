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

func (service *Service) Send(parent context.Context, target sessions.AgentTarget, text, mode string) (WriteResult, error) {
	if text == "" || len(text) > 32768 || !utf8.ValidString(text) || strings.ContainsRune(text, 0) || mode != "" && mode != "auto" && mode != "terminal" {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	if mode != "terminal" {
		capture, err := service.sessions.CaptureAgent(ctx, target, 8192)
		if err != nil {
			return WriteResult{}, err
		}
		status := Detect(session.Agent.Provider, capture.Text)
		if status.State != "idle" && status.State != "working" {
			return WriteResult{}, ErrBlocked
		}
	}
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

func (service *Service) Interrupt(parent context.Context, target sessions.AgentTarget) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	return terminalWriteResult(service.sessions.AgentKeys(ctx, target, []string{interruptKey(session.Agent.Provider)}))
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

// The gateway supplies its existing terminal closer so active phone shadows are
// detached before the existing exact-session kill. No provider owns this step.
func (service *Service) Stop(parent context.Context, target sessions.AgentTarget, closeTerminal func(context.Context, sessions.AgentTarget) error) (StopResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return StopResult{}, err
	}
	result := StopResult{Agent: "unconfirmed", Terminal: "unconfirmed"}
	haltContext, cancelHalt := context.WithDeadline(ctx, time.Now().Add(8*time.Second))
	profile, native, inspected, ok := service.inspect(haltContext, session)
	if ok {
		if _, err := service.sessions.ResolveAgent(haltContext, target); err != nil {
			cancelHalt()
			return StopResult{}, err
		}
		var halted struct {
			Agent  string `json:"agent"`
			Reason string `json:"reason,omitempty"`
		}
		if service.native(haltContext, profile, "stop", []nativeTarget{native}, nil, &halted) == nil {
			switch halted.Agent {
			case "stopped", "interrupted", "idle":
				result.Agent = halted.Agent
			}
		}
	}
	if session.Agent.Provider == agentruntime.ProviderCodex {
		// A terminal key is a halt attempt, never confirmation of cancellation.
		_ = service.sessions.AgentKeys(haltContext, target, []string{interruptKey(session.Agent.Provider)}) // justify-ignore-error: stop reports the halt unconfirmed and still attempts its separately reported terminal closure.
	}
	cancelHalt()
	closeContext, cancelClose := context.WithTimeout(ctx, 2*time.Second)
	defer cancelClose()
	err = closeTerminal(closeContext, target)
	if err != nil && !isSessionAbsent(err) {
		result.Reason = "unavailable"
		if errors.Is(err, sessions.ErrAgentTargetStale) || isSessionIdentityMismatch(err) {
			result.Reason = "stale"
		}
		return result, nil
	}
	result.Terminal = "closed"
	if session.Agent.Provider == agentruntime.ProviderClaude && inspected.TerminalOwnsAgent && service.sessions.AgentProcessExited(target) {
		result.Agent = "stopped"
	}
	return result, nil
}

func isSessionAbsent(err error) bool {
	var failure *sessions.Error
	return errors.As(err, &failure) && failure.Code == sessions.ErrorSessionNotFound
}

func isSessionIdentityMismatch(err error) bool {
	var failure *sessions.Error
	return errors.As(err, &failure) && failure.Code == sessions.ErrorSessionIdentityMismatch
}

func interruptKey(provider agentruntime.Provider) string {
	if provider == agentruntime.ProviderCodex {
		return "escape"
	}
	return "ctrl-c"
}
