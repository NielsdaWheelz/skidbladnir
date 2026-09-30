package agentcontrol

import (
	"context"
	"errors"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var ErrTerminalInputBlocked = errors.New("terminal input blocked")

type TerminalInputBlockedError struct{ Reason string }

func (err *TerminalInputBlockedError) Error() string         { return "terminal input blocked: " + err.Reason }
func (err *TerminalInputBlockedError) Unwrap() error         { return ErrTerminalInputBlocked }
func (err *TerminalInputBlockedError) DispatchState() string { return "not_sent" }

func (service *Service) sample(ctx context.Context, target sessions.TerminalTarget) (sessions.Session, detection, error) {
	session, capture, err := service.sessions.CaptureTerminal(ctx, target, 8192, true)
	result := detection{state: "unknown", composer: "unknown"}
	if errors.Is(err, sessions.ErrTerminalObservationChanged) {
		return session, result, nil
	}
	if err != nil {
		return session, result, err
	}
	if session.Agent != nil && session.Connection == nil && !capture.Truncated {
		result = detect(session.Agent.Provider, capture.Text)
	}
	return session, result, nil
}

func (service *Service) TerminalInspect(parent context.Context, target sessions.TerminalTarget) (sessions.TerminalStatus, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	_, result, err := service.sample(ctx, target)
	if err != nil {
		return sessions.TerminalStatus{State: "unknown", Source: "unavailable"}, err
	}
	return sessions.TerminalStatus{State: result.state, Source: "terminal"}, nil
}

func (service *Service) TerminalRead(parent context.Context, target sessions.TerminalTarget, maxBytes int) (ReadResult, error) {
	if maxBytes == 0 {
		maxBytes = 16384
	}
	if maxBytes < 1 || maxBytes > 32768 {
		return ReadResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	_, capture, err := service.sessions.CaptureTerminal(ctx, target, maxBytes, false)
	if err != nil {
		return ReadResult{}, err
	}
	scope := "terminal_history"
	if capture.Alternate {
		scope = "visible"
	}
	return ReadResult{Text: capture.Text, Source: "terminal", Scope: scope, Truncated: capture.Truncated}, nil
}

func (service *Service) TerminalSend(parent context.Context, target sessions.TerminalTarget, text string) (WriteResult, error) {
	if !validText(text) {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, result, err := service.sample(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	if result.state == "blocked" {
		return WriteResult{}, &TerminalInputBlockedError{Reason: "dialog"}
	}
	if result.composer == "draft" {
		return WriteResult{}, &TerminalInputBlockedError{Reason: "draft"}
	}
	if session.Agent == nil || result.state != "working" && result.state != "idle" || result.composer != "empty" {
		return WriteResult{}, &TerminalInputBlockedError{Reason: "unknown"}
	}
	return terminalWriteResult(service.sessions.SendTerminal(ctx, target, text, session.Agent))
}

func (service *Service) Text(parent context.Context, target sessions.TerminalTarget, text string) (WriteResult, error) {
	if !validText(text) {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return terminalWriteResult(service.sessions.SendTerminal(ctx, target, text, nil))
}

func (service *Service) Keys(parent context.Context, target sessions.TerminalTarget, keys []string) (WriteResult, error) {
	if len(keys) < 1 || len(keys) > 16 {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	return terminalWriteResult(service.sessions.TerminalKeys(ctx, target, keys, nil))
}

func (service *Service) TerminalStop(parent context.Context, target sessions.TerminalTarget) (WriteResult, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	key := "ctrl-c"
	expected := session.Agent
	if session.Connection != nil {
		expected = nil
	}
	if expected != nil && expected.Provider == agentruntime.ProviderCodex {
		key = "escape"
	}
	return terminalWriteResult(service.sessions.TerminalKeys(ctx, target, []string{key}, expected))
}

func terminalWriteResult(err error) (WriteResult, error) {
	if err == nil {
		return WriteResult{Method: "terminal", Outcome: "written"}, nil
	}
	if errors.Is(err, sessions.ErrTerminalWriteUnknown) {
		return WriteResult{Method: "terminal", Outcome: "unknown"}, nil
	}
	if errors.Is(err, tmuxclient.ErrInputInvalid) {
		return WriteResult{}, ErrInvalidInput
	}
	return WriteResult{}, err
}
