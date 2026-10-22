package sessions

import (
	"context"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// The terminal target sentinels are tmux's own; consumers keep these names.
var (
	ErrTerminalTargetChanged      = tmuxclient.ErrTargetChanged
	ErrTerminalUnavailable        = tmuxclient.ErrUnavailable
	ErrTerminalWriteUnknown       = tmuxclient.ErrWriteUnknown
	ErrTerminalScreenChanged      = tmuxclient.ErrScreenChanged // a CapturePanes block refused: only ObservePane can tell why
	ErrTerminalObservationChanged = errors.New("terminal foreground changed during observation")
	ErrTerminalProcessFailed      = errors.New("terminal foreground process could not be identified")
	ErrTerminalCaptureFailed      = errors.New("terminal screen could not be captured")
	ErrSessionDeleteUnknown       = errors.New("session deletion is unknown")
)

type TerminalTarget struct {
	TmuxID        string `json:"-"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

// Called only after resolution has accepted this exact lifetime token.
func (target TerminalTarget) paneTarget() tmuxclient.PaneTarget {
	server, _ := parseLifetime(target.TmuxID, target.IdentityToken)
	return tmuxclient.PaneTarget{SessionID: target.TmuxID, PaneID: target.PaneID, Server: server}
}

// Terminal observations/effects require no manager mutex: immutable configuration
// is read here, and tmux synchronously guards each effect against external clients.
func (manager *Manager) ResolveTerminal(ctx context.Context, target TerminalTarget) (Session, error) {
	if ctx.Err() != nil {
		return Session{}, ErrTerminalUnavailable
	}
	server, row, err := manager.resolve(ctx, target.TmuxID, target.IdentityToken)
	if err != nil {
		var sessionErr *Error
		if errors.As(err, &sessionErr) {
			return Session{}, err
		}
		return Session{}, ErrTerminalUnavailable
	}
	if row.PaneID != target.PaneID {
		return Session{}, ErrTerminalTargetChanged
	}
	session := enrichSession(server, row, manager.settings.Load())
	err = manager.RequireServer(ctx, server)
	// An expired ctx fails the identity read and degrades enrichment without
	// saying anything about the target, so it outranks the read's failure.
	if ctx.Err() != nil {
		return Session{}, ErrTerminalUnavailable
	}
	if err != nil {
		return Session{}, ErrTerminalTargetChanged
	}
	return session, nil
}

// CaptureTerminal is the public plain-text read between two full resolutions
// of target. A foreground that could not be sampled, or that differs between
// the resolutions, makes the read ErrTerminalUnavailable.
func (manager *Manager) CaptureTerminal(ctx context.Context, target TerminalTarget, maxBytes int) (tmuxclient.Capture, error) {
	before, err := manager.ResolveTerminal(ctx, target)
	if err == nil && before.ForegroundFailed() {
		err = ErrTerminalUnavailable
	}
	if err != nil {
		return tmuxclient.Capture{}, err
	}
	capture, err := manager.tmux.CapturePane(ctx, target.paneTarget(), maxBytes)
	if err != nil {
		return tmuxclient.Capture{}, err
	}
	after, err := manager.ResolveTerminal(ctx, target)
	if err == nil && (after.ForegroundFailed() || !sameForeground(before, after)) {
		err = ErrTerminalUnavailable
	}
	if err != nil {
		return tmuxclient.Capture{}, err
	}
	return capture, nil
}

// ObservePane captures the bounded screen regions of session's exact target and
// then revalidates the pane foreground against session's own sample. session
// comes from List or ResolveTerminal in the same request and is never
// re-resolved. Precondition: session has an Agent, so its sample holds a
// foreground.
// Errors: ErrTerminalCaptureFailed (tmux failure or dimension/screen change),
// ErrTerminalProcessFailed (re-observing the foreground failed),
// ErrTerminalObservationChanged (foreground absent or different in the same
// target), ErrTerminalTargetChanged (session lifetime or selected pane
// changed).
// Callers check ctx first: an expired context is a timeout at any stage.
func (manager *Manager) ObservePane(ctx context.Context, session Session) (tmuxclient.PaneObservation, error) {
	observation, err := manager.tmux.ObservePane(ctx, session.screen.PaneTarget)
	if errors.Is(err, ErrTerminalTargetChanged) {
		return tmuxclient.PaneObservation{}, err
	}
	if err != nil {
		return tmuxclient.PaneObservation{}, ErrTerminalCaptureFailed
	}
	if err := resample(session); err != nil {
		return tmuxclient.PaneObservation{}, err
	}
	return observation, nil
}

// CapturePanes captures targets, agent sessions of one inventory of server, in
// one tmux client (tmux-exec-budget.md a2) and re-samples each captured pane's
// foreground against the session's own sample. On a nil error captures[i] is
// targets[i]'s observation or its error: ErrTerminalScreenChanged (refused),
// ErrTerminalCaptureFailed, ErrTerminalProcessFailed or
// ErrTerminalObservationChanged. err is tmuxclient.WorkspaceSourceChanged when
// the bracket proved another lifetime or none; any other err means no
// bracketed transcript exists.
func (manager *Manager) CapturePanes(ctx context.Context, server tmuxclient.ServerIdentity, targets []Session) ([]tmuxclient.PaneCapture, error) {
	screens := make([]tmuxclient.ScreenTarget, len(targets))
	for index, session := range targets {
		screens[index] = session.screen
	}
	observed, captures, err := manager.tmux.CapturePanes(ctx, screens)
	if err != nil {
		return nil, err
	}
	if observed != server {
		return nil, tmuxclient.WorkspaceSourceChanged
	}
	for index := range captures {
		switch err := captures[index].Err; {
		case errors.Is(err, ErrTerminalScreenChanged):
		case err != nil:
			captures[index].Err = ErrTerminalCaptureFailed
		default:
			captures[index].Err = resample(targets[index])
		}
	}
	return captures, nil
}

// resample requires the pane's foreground to still be session's own sample.
func resample(session Session) error {
	foreground, err := paneForeground(processinfo.PID(session.screen.PanePID))
	if err != nil {
		return ErrTerminalProcessFailed
	}
	if foreground == nil || *foreground != *session.foreground {
		return ErrTerminalObservationChanged
	}
	return nil
}

// expected guards provider-specific input. nil deliberately permits generic
// input without requiring process recognition or an available kernel sample.
func (manager *Manager) SendTerminal(ctx context.Context, target TerminalTarget, text string, expected *agentruntime.Agent) error {
	session, err := manager.ResolveTerminal(ctx, target)
	if err != nil {
		return err
	}
	if err := requireForeground(session, expected); err != nil {
		return err
	}
	return manager.tmux.Paste(ctx, target.paneTarget(), text, func() error {
		return manager.revalidateForeground(ctx, target, session, expected)
	})
}

func (manager *Manager) TerminalKeys(ctx context.Context, target TerminalTarget, keys []string, expected *agentruntime.Agent) error {
	session, err := manager.ResolveTerminal(ctx, target)
	if err != nil {
		return err
	}
	if err := requireForeground(session, expected); err != nil {
		return err
	}
	if expected != nil {
		if err := manager.revalidateForeground(ctx, target, session, expected); err != nil {
			return err
		}
	}
	return manager.tmux.Keys(ctx, target.paneTarget(), keys)
}

func (manager *Manager) revalidateForeground(ctx context.Context, target TerminalTarget, before Session, expected *agentruntime.Agent) error {
	after, err := manager.ResolveTerminal(ctx, target)
	if err != nil {
		return err
	}
	if err := requireForeground(after, expected); err != nil {
		return err
	}
	if expected != nil && !sameForeground(before, after) {
		return ErrTerminalTargetChanged
	}
	return nil
}

func requireForeground(session Session, expected *agentruntime.Agent) error {
	if expected == nil {
		return nil
	}
	if session.foreground == nil {
		return ErrTerminalUnavailable
	}
	if session.Agent == nil || *session.Agent != *expected {
		return ErrTerminalTargetChanged
	}
	return nil
}

func sameForeground(left, right Session) bool {
	if left.foreground == nil || right.foreground == nil {
		return left.foreground == nil && right.foreground == nil
	}
	return *left.foreground == *right.foreground
}

func (manager *Manager) Profile(key agentruntime.ProfileKey) (agentruntime.Profile, bool) {
	profile, found := manager.settings.Load().byKey[key]
	return profile, found
}
