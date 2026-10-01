package sessions

import (
	"context"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// Native callers retain their separate exact-conversation stale error.
var ErrAgentTargetStale = errors.New("agent target changed")
var (
	ErrTerminalTargetChanged      = errors.New("terminal target changed")
	ErrTerminalUnavailable        = errors.New("terminal unavailable")
	ErrTerminalObservationChanged = errors.New("terminal foreground changed during observation")
	ErrTerminalProcessFailed      = errors.New("terminal foreground process could not be identified")
	ErrTerminalCaptureFailed      = errors.New("terminal screen could not be captured")
	ErrTerminalWriteUnknown       = errors.New("terminal input delivery is unknown")
	ErrSessionDeleteUnknown       = errors.New("session deletion is unknown")
)

type TerminalTarget struct {
	TmuxID        string `json:"-"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

func TargetOf(session Session) TerminalTarget {
	return TerminalTarget{TmuxID: session.TmuxID, IdentityToken: session.IdentityToken, PaneID: session.ActivePaneID}
}

// Called only after resolution has accepted this exact lifetime token.
func (target TerminalTarget) paneTarget() tmuxclient.PaneTarget {
	identity, _ := parseIdentityToken(target.IdentityToken, target.TmuxID)
	return tmuxclient.PaneTarget{SessionID: target.TmuxID, PaneID: target.PaneID, Server: identity}
}

// Terminal observations/effects require no manager mutex: immutable configuration
// is read here, and tmux synchronously guards each effect against external clients.
func (manager *Manager) ResolveTerminal(ctx context.Context, target TerminalTarget) (Session, error) {
	if ctx.Err() != nil {
		return Session{}, ErrTerminalUnavailable
	}
	if !paneIDPattern.MatchString(target.PaneID) {
		return Session{}, ErrTerminalTargetChanged
	}
	identity, _, err := manager.sessionLifetimeIdentity(ctx, target.TmuxID, target.IdentityToken)
	if err != nil {
		return Session{}, terminalError(err)
	}
	observed, found, err := manager.scanSession(ctx, target.TmuxID)
	if err != nil {
		return Session{}, terminalError(err)
	}
	if !found {
		return Session{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	inspected, present, err := manager.inspectAnchor(ctx, Session{
		TmuxID: target.TmuxID, TmuxName: observed.tmuxName, NameMode: effectiveNameMode(observed.tmuxName, observed.autoMarker),
		IdentityToken: target.IdentityToken, Character: observed.character,
	})
	if err != nil {
		return Session{}, terminalError(err)
	}
	if !present || inspected.paneID != target.PaneID {
		return Session{}, ErrTerminalTargetChanged
	}
	session := manager.enrichSession(ctx, inspected)
	if err := manager.requireServerIdentity(ctx, identity); err != nil {
		return Session{}, ErrTerminalTargetChanged
	}
	if ctx.Err() != nil {
		return Session{}, ErrTerminalUnavailable
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
		return tmuxclient.Capture{}, terminalError(err)
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
// re-resolved. Precondition: the session's foreground sample either failed or
// found a foreground (Agent != nil guarantees one); a session sampled without a
// foreground is a caller defect.
// Errors: ErrTerminalProcessFailed (the session's foreground sample failed, or
// re-observing it failed), ErrTerminalCaptureFailed (tmux failure or
// dimension/screen change), ErrTerminalObservationChanged (foreground absent or
// different in the same target), ErrTerminalTargetChanged (session lifetime or
// selected pane changed).
// Callers check ctx first: an expired context is a timeout at any stage.
func (manager *Manager) ObservePane(ctx context.Context, session Session) (tmuxclient.PaneObservation, error) {
	if session.ForegroundFailed() {
		return tmuxclient.PaneObservation{}, ErrTerminalProcessFailed
	}
	if session.foreground == nil {
		panic("observed terminal session has no foreground") // justify-defect: callers observe only a recognized agent, whose sample holds its foreground.
	}
	observation, err := manager.tmux.ObservePane(ctx, TargetOf(session).paneTarget())
	switch {
	case err == nil:
	case errors.Is(err, tmuxclient.ErrTargetChanged):
		return tmuxclient.PaneObservation{}, ErrTerminalTargetChanged
	case errors.Is(err, tmuxclient.ErrScreenChanged), errors.Is(err, tmuxclient.ErrUnavailable):
		return tmuxclient.PaneObservation{}, ErrTerminalCaptureFailed
	default:
		panic("observed terminal target is invalid") // justify-defect: List, ResolveTerminal and creation return only canonical targets.
	}
	foreground, err := paneForeground(session.panePID)
	if err != nil {
		return tmuxclient.PaneObservation{}, ErrTerminalProcessFailed
	}
	if foreground == nil || !processinfo.SameObservation(*session.foreground, *foreground) {
		return tmuxclient.PaneObservation{}, ErrTerminalObservationChanged
	}
	return observation, nil
}

// expected guards provider-specific input. nil deliberately permits generic
// input without requiring process recognition or an available kernel sample.
func (manager *Manager) SendTerminal(ctx context.Context, target TerminalTarget, text string, expected *agentruntime.AgentRuntime) error {
	session, err := manager.ResolveTerminal(ctx, target)
	if err != nil {
		return err
	}
	if err := requireForeground(session, expected); err != nil {
		return err
	}
	err = manager.tmux.Paste(ctx, target.paneTarget(), text, func() error {
		return manager.revalidateForeground(ctx, target, session, expected)
	})
	return terminalError(err)
}

func (manager *Manager) TerminalKeys(ctx context.Context, target TerminalTarget, keys []string, expected *agentruntime.AgentRuntime) error {
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
	return terminalError(manager.tmux.Keys(ctx, target.paneTarget(), keys))
}

func (manager *Manager) revalidateForeground(ctx context.Context, target TerminalTarget, before Session, expected *agentruntime.AgentRuntime) error {
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

func requireForeground(session Session, expected *agentruntime.AgentRuntime) error {
	if expected == nil {
		return nil
	}
	if session.foreground == nil {
		return ErrTerminalUnavailable
	}
	agent := session.Agent
	if agent == nil || agent.PaneID != expected.PaneID || agent.PID != expected.PID || agent.StartIdentity != expected.StartIdentity || agent.Provider != expected.Provider || agent.Profile != expected.Profile {
		return ErrTerminalTargetChanged
	}
	return nil
}

func sameForeground(left, right Session) bool {
	if left.foreground == nil || right.foreground == nil {
		return left.foreground == nil && right.foreground == nil
	}
	return processinfo.SameObservation(*left.foreground, *right.foreground)
}

func terminalError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tmuxclient.ErrTargetChanged):
		return ErrTerminalTargetChanged
	case errors.Is(err, tmuxclient.ErrWriteUnknown):
		return ErrTerminalWriteUnknown
	case errors.Is(err, tmuxclient.ErrInputInvalid):
		return err
	case errors.Is(err, ErrTerminalTargetChanged), errors.Is(err, ErrTerminalUnavailable):
		return err
	}
	var sessionErr *Error
	if errors.As(err, &sessionErr) {
		return err
	}
	return ErrTerminalUnavailable
}

func (manager *Manager) Profile(key agentruntime.ProfileKey) (agentruntime.Profile, bool) {
	profile, found := manager.profilesByKey[key]
	return profile, found
}

func (manager *Manager) ResolveSession(ctx context.Context, tmuxID, identityToken string) error {
	_, _, err := manager.sessionLifetimeIdentity(ctx, tmuxID, identityToken)
	return err
}
