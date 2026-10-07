package sessions

import (
	"context"
	"errors"
)

// ErrGroupDispatchUnknown means the assignment may have executed. Never replay it.
var ErrGroupDispatchUnknown = errors.New("group assignment completion is unknown")

func (manager *Manager) SetGroup(ctx context.Context, input SetGroupInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	assigned, err := manager.tmux.SetSessionGroupIfIdentity(ctx, input.TmuxID, input.Group, server)
	if err != nil {
		return ErrGroupDispatchUnknown
	}
	if !assigned {
		return sessionIdentityMismatch()
	}
	manager.checkpointAfterMutation(ctx, server)
	return nil
}
