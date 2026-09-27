package sessions

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
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
	return nil
}

func decodeGroupMetadata(encoded string) group.Label {
	if encoded == "" || len(encoded) > 342 {
		return group.Label{}
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return group.Label{}
	}
	label, err := group.Parse(string(decoded))
	if err != nil {
		return group.Label{}
	}
	return label
}
