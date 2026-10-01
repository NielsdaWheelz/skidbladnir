package agentcontrol

import (
	"context"
	"errors"
	"math"
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

// sample observes session once, by the composition inventory, inspect and
// guarded send share. A failed foreground sample is process_failed; a remote
// connection or an unrecognized foreground is unknown without a capture.
// Otherwise the focused observation runs; any failure of it is a timeout once
// ctx has expired, and only otherwise its own failure. A successful capture is
// classified by its provider's grammar; the composer stays unknown unless a
// screen was classified. The diagnostics hold the rules, the capture iff one
// succeeded, and the stages after resolution that ran. The only error is
// sessions.ErrTerminalTargetChanged: the session's lifetime or selected pane
// changed during the capture.
func (service *Service) sample(ctx context.Context, session sessions.Session) (sessions.TerminalStatus, composer, Diagnostics, error) {
	var diagnostics Diagnostics
	switch {
	case session.ForegroundFailed():
		return sessions.UnknownStatus(sessions.ReasonProcessFailed), composerUnknown, diagnostics, nil
	case session.Connection != nil:
		return sessions.UnknownStatus(sessions.ReasonRemoteContext), composerUnknown, diagnostics, nil
	case session.Agent == nil:
		return sessions.UnknownStatus(sessions.ReasonProviderUnrecognized), composerUnknown, diagnostics, nil
	}
	startedAt := time.Now()
	observation, err := service.sessions.ObservePane(ctx, session)
	diagnostics.ElapsedMs.Capture = milliseconds(time.Since(startedAt))
	if err != nil {
		var reason sessions.StatusReason
		switch {
		case ctx.Err() != nil:
			reason = sessions.ReasonObservationTimeout
		case errors.Is(err, sessions.ErrTerminalObservationChanged):
			reason = sessions.ReasonForegroundChanged
		case errors.Is(err, sessions.ErrTerminalProcessFailed):
			reason = sessions.ReasonProcessFailed
		case errors.Is(err, sessions.ErrTerminalCaptureFailed):
			reason = sessions.ReasonCaptureFailed
		case errors.Is(err, sessions.ErrTerminalTargetChanged):
			return sessions.TerminalStatus{}, composerUnknown, Diagnostics{}, err
		default:
			panic("unknown terminal observation error") // justify-defect: ObservePane's errors are closed.
		}
		return sessions.UnknownStatus(reason), composerUnknown, diagnostics, nil
	}
	diagnostics.Capture = &CaptureDiagnostics{Width: observation.Width, Height: observation.Height, Alternate: observation.Alternate}
	for _, region := range observation.Regions {
		switch region.Kind {
		case tmuxclient.RegionTop:
			diagnostics.Capture.TopClipped = region.Clipped
		case tmuxclient.RegionBottom:
			diagnostics.Capture.BottomClipped = region.Clipped
		default:
			panic("unknown screen region") // justify-defect: tmux has exactly two region kinds.
		}
	}
	startedAt = time.Now()
	detected := detect(session.Agent.Provider, observation)
	diagnostics.ElapsedMs.Classify = milliseconds(time.Since(startedAt))
	diagnostics.Rules = detected.rules
	return detected.status, detected.composer, diagnostics, nil
}

// milliseconds is a stage's elapsed time as the diagnostics wire carries it:
// whole milliseconds clamped to 0..2147483647.
func milliseconds(elapsed time.Duration) *int64 {
	value := min(max(elapsed.Milliseconds(), 0), math.MaxInt32)
	return &value
}

// TerminalInspect resolves target fresh and observes it once under a
// two-second deadline. Resolution admits the target: its errors, and a target
// that changes during the capture, are errors. Every later failed stage is an
// unavailable status. The diagnostics describe this same sample.
func (service *Service) TerminalInspect(parent context.Context, target sessions.TerminalTarget) (sessions.TerminalStatus, Diagnostics, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	startedAt := time.Now()
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return sessions.TerminalStatus{}, Diagnostics{}, err
	}
	resolved := milliseconds(time.Since(startedAt))
	status, _, diagnostics, err := service.sample(ctx, session)
	if err != nil {
		return sessions.TerminalStatus{}, Diagnostics{}, err
	}
	diagnostics.ElapsedMs.Resolve = resolved
	if !diagnostics.Valid() {
		panic("terminal inspection produced invalid diagnostics") // justify-defect: grammars name content-free rule ids, tmux reports positive dimensions and stages are clamped.
	}
	return status, diagnostics, nil
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
	capture, err := service.sessions.CaptureTerminal(ctx, target, maxBytes)
	if err != nil {
		return ReadResult{}, err
	}
	scope := "terminal_history"
	if capture.Alternate {
		scope = "visible"
	}
	return ReadResult{Text: capture.Text, Source: "terminal", Scope: scope, Truncated: capture.Truncated}, nil
}

// TerminalSend pastes only into a fresh local provider's empty ordinary
// composer: activity working or idle, interaction none, notice none. It
// resolves target, observes it once, and refuses before writing: a request
// interaction is a dialog, a composer holding input a draft, anything else
// unknown. An unavailable observation is ErrTerminalUnavailable.
func (service *Service) TerminalSend(parent context.Context, target sessions.TerminalTarget, text string) (WriteResult, error) {
	if !validText(text) {
		return WriteResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return WriteResult{}, err
	}
	status, composerState, _, err := service.sample(ctx, session)
	if err != nil {
		return WriteResult{}, err
	}
	if status.Source == sessions.SourceUnavailable {
		return WriteResult{}, sessions.ErrTerminalUnavailable
	}
	switch status.Interaction {
	case sessions.InteractionPermission, sessions.InteractionQuestion, sessions.InteractionConfirmation, sessions.InteractionSetup, sessions.InteractionInput:
		return WriteResult{}, &TerminalInputBlockedError{Reason: "dialog"}
	case sessions.InteractionNone, sessions.InteractionMenu, sessions.InteractionUnknown:
	default:
		panic("invalid terminal interaction") // justify-defect: sample returns only valid statuses.
	}
	if composerState == composerDraft {
		return WriteResult{}, &TerminalInputBlockedError{Reason: "draft"}
	}
	if status.Activity != sessions.ActivityWorking && status.Activity != sessions.ActivityIdle || status.Interaction != sessions.InteractionNone || status.Notice != sessions.NoticeNone || composerState != composerEmpty {
		return WriteResult{}, &TerminalInputBlockedError{Reason: "unknown"}
	}
	// Only a classified screen has an empty composer, so session.Agent is the
	// recognized local provider whose exact foreground the paste requires.
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
