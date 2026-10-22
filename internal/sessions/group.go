package sessions

import (
	"context"
	"errors"
)

// ErrGroupDispatchUnknown means the assignment may have executed. Never replay it.
var ErrGroupDispatchUnknown = errors.New("group assignment completion is unknown")

func (manager *Manager) SetGroup(ctx context.Context, input SetGroupInput) error {
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	defer manager.release()

	server, err := parseLifetime(input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	assigned, err := manager.tmux.SetSessionGroupIfIdentity(ctx, input.TmuxID, input.Group, server)
	if err != nil {
		return ErrGroupDispatchUnknown
	}
	if !assigned {
		return manager.refused(ctx, input.TmuxID)
	}
	manager.checkpointAfterMutation(ctx, server)
	return nil
}
