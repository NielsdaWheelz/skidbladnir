package sessions

import (
	"context"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var ErrAgentTargetStale = errors.New("agent target changed")
var ErrAgentWriteUnknown = errors.New("agent input delivery is unknown")

type AgentTarget struct {
	TmuxID        string                    `json:"-"`
	IdentityToken string                    `json:"identityToken"`
	PaneID        string                    `json:"paneId"`
	PID           processinfo.PID           `json:"pid"`
	StartIdentity processinfo.StartIdentity `json:"startIdentity"`
}

func TargetOf(session Session) AgentTarget {
	return AgentTarget{TmuxID: session.TmuxID, IdentityToken: session.IdentityToken, PaneID: session.Agent.PaneID, PID: session.Agent.PID, StartIdentity: session.Agent.StartIdentity}
}

func (manager *Manager) ResolveAgent(ctx context.Context, target AgentTarget) (Session, error) {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	return manager.resolveAgent(ctx, target)
}

func (manager *Manager) resolveAgent(ctx context.Context, target AgentTarget) (Session, error) {
	session, err := manager.resolveAgentTerminal(ctx, target)
	if err != nil {
		return Session{}, err
	}
	if session.Agent == nil || session.Agent.PID != target.PID || session.Agent.StartIdentity != target.StartIdentity {
		return Session{}, ErrAgentTargetStale
	}
	return session, nil
}

// resolveAgentTerminal is called with the manager lock held. Its pane check is
// also used after a native stop, when the original process may have exited.
func (manager *Manager) resolveAgentTerminal(ctx context.Context, target AgentTarget) (Session, error) {
	if !paneIDPattern.MatchString(target.PaneID) || target.PID <= 0 || target.StartIdentity == "" {
		return Session{}, ErrAgentTargetStale
	}
	name, exists, err := manager.sessionIdentity(ctx, target.TmuxID)
	if err != nil {
		return Session{}, err
	}
	if !exists {
		return Session{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	identity, err := manager.mutationIdentity(ctx, target.TmuxID, name, target.IdentityToken)
	if err != nil {
		return Session{}, err
	}
	if identity.phoneShadow {
		return Session{}, ErrAgentTargetStale
	}
	observed, found, err := manager.scanSession(ctx, target.TmuxID)
	if err != nil {
		return Session{}, err
	}
	if !found {
		return Session{}, newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	inspected, present, err := manager.inspectRequired(ctx, observed, identity.server)
	if err != nil {
		return Session{}, err
	}
	if !present || inspected.paneID != target.PaneID {
		return Session{}, ErrAgentTargetStale
	}
	return manager.enrichSession(ctx, inspected), nil
}

func (manager *Manager) CaptureAgent(ctx context.Context, target AgentTarget, maxBytes int) (tmuxclient.Capture, error) {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, err := manager.resolveAgent(ctx, target)
	if err != nil {
		return tmuxclient.Capture{}, err
	}
	return manager.tmux.CapturePane(ctx, target.PaneID, maxBytes)
}

func (manager *Manager) SendAgent(ctx context.Context, target AgentTarget, text string) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, err := manager.resolveAgent(ctx, target)
	if err != nil {
		return err
	}
	if err := manager.tmux.Paste(ctx, target.PaneID, text); err != nil {
		if errors.Is(err, tmuxclient.ErrInputInvalid) {
			return err
		}
		return ErrAgentWriteUnknown
	}
	return nil
}

func (manager *Manager) AgentKeys(ctx context.Context, target AgentTarget, keys []string) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, err := manager.resolveAgent(ctx, target)
	if err != nil {
		return err
	}
	if err := manager.tmux.Keys(ctx, target.PaneID, keys); err != nil {
		if errors.Is(err, tmuxclient.ErrInputInvalid) {
			return err
		}
		return ErrAgentWriteUnknown
	}
	return nil
}

func (manager *Manager) AgentTerminalKillInput(ctx context.Context, target AgentTarget) (KillInput, error) {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	return manager.agentTerminalKillInput(ctx, target)
}

func (manager *Manager) agentTerminalKillInput(ctx context.Context, target AgentTarget) (KillInput, error) {
	session, err := manager.resolveAgentTerminal(ctx, target)
	if err != nil {
		return KillInput{}, err
	}
	if session.foreground == nil || session.Agent != nil && (session.Agent.PID != target.PID || session.Agent.StartIdentity != target.StartIdentity) {
		return KillInput{}, ErrAgentTargetStale
	}
	return KillInput{TmuxID: session.TmuxID, TmuxName: session.TmuxName, IdentityToken: session.IdentityToken}, nil
}

func (manager *Manager) AgentProcessExited(target AgentTarget) bool {
	observed, err := processinfo.Observe(target.PID)
	return errors.Is(err, processinfo.ErrProcessAbsent) || err == nil && observed.StartIdentity != target.StartIdentity
}

func (manager *Manager) Profile(key agentruntime.ProfileKey) (agentruntime.Profile, bool) {
	profile, found := manager.profilesByKey[key]
	return profile, found
}

// KillAgentTerminal rechecks the pane and foreground after phone detach and
// shadow reconciliation, under the same mutation lock as the exact-session kill.
func (manager *Manager) KillAgentTerminal(ctx context.Context, target AgentTarget) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()
	input, err := manager.agentTerminalKillInput(ctx, target)
	if err != nil {
		return err
	}
	return manager.kill(ctx, input, &target)
}
