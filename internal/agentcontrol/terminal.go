package agentcontrol

import (
	"context"
	"errors"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// TerminalInputBlockedError refuses a guarded send before anything is written.
// Reason is dialog, draft, initial_not_ready or unknown.
type TerminalInputBlockedError struct{ Reason string }

func (err *TerminalInputBlockedError) Error() string { return "terminal input blocked: " + err.Reason }

// unobserved is the status of a session whose screen is never read:
// process_failed, remote_context or provider_unrecognized; false for a
// recognized local agent. A failed foreground sample leaves Agent and
// Connection unknown, not absent, so it is decided before them.
func unobserved(session sessions.Session) (detection, bool) {
	switch {
	case session.ForegroundFailed():
		return detection{status: api.UnknownStatus(api.ReasonProcessFailed)}, true
	case session.Connection != nil:
		return detection{status: api.UnknownStatus(api.ReasonRemoteContext)}, true
	case session.Agent == nil:
		return detection{status: api.UnknownStatus(api.ReasonProviderUnrecognized)}, true
	}
	return detection{}, false
}

// failedDetection is the unavailable status of a failed observation stage. An
// expired ctx outranks the stage, whose error is then the deadline's doing.
func failedDetection(ctx context.Context, err error) detection {
	reason := api.ReasonCaptureFailed
	switch {
	case ctx.Err() != nil:
		reason = api.ReasonObservationTimeout
	case errors.Is(err, sessions.ErrTerminalObservationChanged):
		reason = api.ReasonForegroundChanged
	case errors.Is(err, sessions.ErrTerminalProcessFailed):
		reason = api.ReasonProcessFailed
	}
	return detection{status: api.UnknownStatus(reason)}
}

// sample is the two-phase observation that inspect, guarded send and
// enrichment's fallback share. Its only error is
// sessions.ErrTerminalTargetChanged while ctx is live: enrichment reports it as
// capture_failed and explicit operations return it.
func (service *Service) sample(ctx context.Context, session sessions.Session) (detection, error) {
	if read, ok := unobserved(session); ok {
		return read, nil
	}
	observation, err := service.sessions.ObservePane(ctx, session)
	switch {
	case err == nil:
		return detect(session.Agent.Provider, observation), nil
	case ctx.Err() == nil && errors.Is(err, sessions.ErrTerminalTargetChanged):
		return detection{}, err
	}
	return failedDetection(ctx, err), nil
}

// TerminalInspect resolves target fresh and observes it once under a
// two-second deadline. Resolution admits the target: its errors, and a target
// that changes during the capture, are errors. Every later failed stage is an
// unavailable status. Its diagnostics name the rules of this same sample.
func (service *Service) TerminalInspect(parent context.Context, target sessions.TerminalTarget) (api.TerminalStatus, api.Diagnostics, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return api.TerminalStatus{}, api.Diagnostics{}, err
	}
	read, err := service.sample(ctx, session)
	if err != nil {
		return api.TerminalStatus{}, api.Diagnostics{}, err
	}
	return read.status, api.Diagnostics{Rules: read.rules}, nil
}

// TerminalRead captures at most limit bytes of terminal text: the retained
// history, or the visible screen of an alternate-screen program.
func (service *Service) TerminalRead(ctx context.Context, target sessions.TerminalTarget, limit int) (api.ReadResult, error) {
	capture, err := service.sessions.CaptureTerminal(ctx, target, limit)
	if err != nil {
		return api.ReadResult{}, err
	}
	scope := "terminal_history"
	if capture.Alternate {
		scope = "visible"
	}
	return api.ReadResult{Text: capture.Text, Source: "terminal", Scope: scope, Truncated: capture.Truncated}, nil
}

// TerminalSend pastes only into a freshly recognized local provider's empty
// ordinary composer: activity working or idle, interaction none. Initial sends
// also require the requested profile, idle activity and no notice. It resolves
// target, observes it once, and refuses before writing: a request interaction
// is a dialog, a composer holding input a draft, anything else unknown. An
// unavailable observation is ErrTerminalUnavailable. An unknown initialProfile
// is ErrInvalidInput, before resolution.
func (service *Service) TerminalSend(ctx context.Context, target sessions.TerminalTarget, text string, initialProfile agentruntime.ProfileKey) (api.WriteResult, error) {
	if initialProfile != "" {
		if _, found := service.sessions.Profile(initialProfile); !found {
			return api.WriteResult{}, ErrInvalidInput
		}
	}
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return api.WriteResult{}, err
	}
	read, err := service.sample(ctx, session)
	if err != nil {
		return api.WriteResult{}, err
	}
	// Only a classified screen has an empty composer, so past the composer
	// checks session.Agent is the recognized local provider, attributed only to
	// a profile of its own provider, whose exact foreground the paste requires.
	status := read.status
	switch {
	case status.Source == api.SourceUnavailable:
		return api.WriteResult{}, sessions.ErrTerminalUnavailable
	case status.Interaction.Request():
		return api.WriteResult{}, &TerminalInputBlockedError{Reason: "dialog"}
	case read.composer == composerDraft:
		return api.WriteResult{}, &TerminalInputBlockedError{Reason: "draft"}
	case (status.Activity != api.ActivityWorking && status.Activity != api.ActivityIdle) ||
		status.Interaction != api.InteractionNone || read.composer != composerEmpty:
		return api.WriteResult{}, &TerminalInputBlockedError{Reason: "unknown"}
	case initialProfile != "" && (session.Agent.Profile != initialProfile || status.Activity != api.ActivityIdle || status.Notice != api.NoticeNone):
		return api.WriteResult{}, &TerminalInputBlockedError{Reason: "initial_not_ready"}
	}
	return terminalWriteResult(service.sessions.SendTerminal(ctx, target, text, session.Agent))
}

func (service *Service) Text(ctx context.Context, target sessions.TerminalTarget, text string) (api.WriteResult, error) {
	return terminalWriteResult(service.sessions.SendTerminal(ctx, target, text, nil))
}

func (service *Service) Keys(ctx context.Context, target sessions.TerminalTarget, keys []string) (api.WriteResult, error) {
	return terminalWriteResult(service.sessions.TerminalKeys(ctx, target, keys, nil))
}

func (service *Service) TerminalStop(ctx context.Context, target sessions.TerminalTarget) (api.WriteResult, error) {
	session, err := service.sessions.ResolveTerminal(ctx, target)
	if err != nil {
		return api.WriteResult{}, err
	}
	// Agent is nil for a shell or a remote connection, which take an unguarded ctrl-c.
	key := "ctrl-c"
	if session.Agent != nil && session.Agent.Provider == agentruntime.ProviderCodex {
		key = "escape"
	}
	return terminalWriteResult(service.sessions.TerminalKeys(ctx, target, []string{key}, session.Agent))
}

func terminalWriteResult(err error) (api.WriteResult, error) {
	if err == nil {
		return api.WriteResult{Method: "terminal", Outcome: "written"}, nil
	}
	if errors.Is(err, sessions.ErrTerminalWriteUnknown) {
		return api.WriteResult{Method: "terminal", Outcome: "unknown"}, nil
	}
	if errors.Is(err, tmuxclient.ErrInputInvalid) {
		return api.WriteResult{}, ErrInvalidInput
	}
	return api.WriteResult{}, err
}
