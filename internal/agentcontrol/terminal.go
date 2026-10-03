package agentcontrol

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"sync"
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

// Enrich sets each session's TerminalStatus from one focused observation of
// the identity List or creation captured, never resolving it again. Every
// session is observed concurrently under one shared two-second deadline. It
// returns the unavailable observations, each timed from Enrich's start, for
// content-free logging.
func (service *Service) Enrich(parent context.Context, observed []sessions.Session) []ObservationFailure {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	elapsed := make([]time.Duration, len(observed))
	defects := make([]string, len(observed))
	var work sync.WaitGroup
	for index := range observed {
		work.Go(func() {
			// A panic on a worker goroutine would end the gateway; the defect is
			// raised again below on the caller's goroutine, where net/http confines
			// it to the one request, as it does for inspect and send.
			defer func() {
				if defect := recover(); defect != nil {
					defects[index] = fmt.Sprintf("%v\n\n%s", defect, debug.Stack())
				}
			}()
			status, _, _, err := service.sample(ctx, observed[index])
			if errors.Is(err, sessions.ErrTerminalTargetChanged) {
				// justify-ignore-error: inventory has no target to reject, so a lifetime or pane change during capture is a failed capture; explicit operations return it.
				status = sessions.UnknownStatus(sessions.ReasonCaptureFailed)
			}
			observed[index].TerminalStatus = status
			elapsed[index] = time.Since(startedAt)
		})
	}
	work.Wait()
	for _, defect := range defects {
		if defect != "" {
			panic(defect) // justify-defect: a worker's classifier or observation defect, with the worker's stack.
		}
	}
	var failures []ObservationFailure
	for index, session := range observed {
		if session.TerminalStatus.Source == sessions.SourceUnavailable {
			failures = append(failures, ObservationFailure{TmuxID: session.TmuxID, Reason: session.TerminalStatus.Reason, Elapsed: elapsed[index]})
		}
	}
	return failures
}

// sample is the one observation that inventory, inspect and guarded send
// share. A failed foreground sample leaves Agent and Connection unknown, not
// absent, so it is decided before them. An expired ctx outranks the stage that
// failed, whose error is then the deadline's doing. Its only error is
// sessions.ErrTerminalTargetChanged: inventory reports it as capture_failed and
// explicit operations return it.
func (service *Service) sample(ctx context.Context, session sessions.Session) (sessions.TerminalStatus, composer, Diagnostics, error) {
	switch {
	case session.ForegroundFailed():
		return sessions.UnknownStatus(sessions.ReasonProcessFailed), composerUnknown, Diagnostics{}, nil
	case session.Connection != nil:
		return sessions.UnknownStatus(sessions.ReasonRemoteContext), composerUnknown, Diagnostics{}, nil
	case session.Agent == nil:
		return sessions.UnknownStatus(sessions.ReasonProviderUnrecognized), composerUnknown, Diagnostics{}, nil
	}
	startedAt := time.Now()
	observation, err := service.sessions.ObservePane(ctx, session)
	diagnostics := Diagnostics{ElapsedMs: StageElapsed{Capture: milliseconds(time.Since(startedAt))}}
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
	resolveElapsed := time.Since(startedAt)
	status, _, diagnostics, err := service.sample(ctx, session)
	if err != nil {
		return sessions.TerminalStatus{}, Diagnostics{}, err
	}
	diagnostics.ElapsedMs.Resolve = milliseconds(resolveElapsed)
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
// composer: activity working or idle, interaction none. Initial sends also
// require the requested provider/profile, idle activity and no notice. It
// resolves target, observes it once, and refuses before writing: a request
// interaction is a dialog, a composer holding input a draft, anything else
// unknown. An unavailable observation is ErrTerminalUnavailable.
func (service *Service) TerminalSend(parent context.Context, target sessions.TerminalTarget, text string, initialProfile agentruntime.ProfileKey) (WriteResult, error) {
	if !validText(text) {
		return WriteResult{}, ErrInvalidInput
	}
	var profile agentruntime.Profile
	if initialProfile != "" {
		var found bool
		profile, found = service.sessions.Profile(initialProfile)
		if !found {
			return WriteResult{}, ErrInvalidInput
		}
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
	switch {
	case status.Source == sessions.SourceUnavailable:
		return WriteResult{}, sessions.ErrTerminalUnavailable
	case status.Interaction.Request():
		return WriteResult{}, &TerminalInputBlockedError{Reason: "dialog"}
	case composerState == composerDraft:
		return WriteResult{}, &TerminalInputBlockedError{Reason: "draft"}
	case (status.Activity != sessions.ActivityWorking && status.Activity != sessions.ActivityIdle) ||
		status.Interaction != sessions.InteractionNone || composerState != composerEmpty:
		return WriteResult{}, &TerminalInputBlockedError{Reason: "unknown"}
	case initialProfile != "" && (session.Agent == nil || session.Agent.Provider != profile.Provider || session.Agent.Profile != initialProfile ||
		status.Activity != sessions.ActivityIdle || status.Notice != sessions.NoticeNone):
		return WriteResult{}, &TerminalInputBlockedError{Reason: "initial_not_ready"}
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
	// Agent is nil for a shell or a remote connection, which take an unguarded ctrl-c.
	key := "ctrl-c"
	if session.Agent != nil && session.Agent.Provider == agentruntime.ProviderCodex {
		key = "escape"
	}
	return terminalWriteResult(service.sessions.TerminalKeys(ctx, target, []string{key}, session.Agent))
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
