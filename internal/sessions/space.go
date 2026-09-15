package sessions

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

// ErrSpaceDispatchUnknown means the assignment may have executed. Never replay it.
var ErrSpaceDispatchUnknown = errors.New("space assignment completion is unknown")

func (manager *Manager) SetSpace(ctx context.Context, input SetSpaceInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	assigned, err := manager.tmux.SetSessionSpaceIfIdentity(ctx, input.TmuxID, input.Space, server)
	if err != nil {
		return ErrSpaceDispatchUnknown
	}
	if !assigned {
		return sessionIdentityMismatch()
	}
	return nil
}

func decodeSpaceMetadata(encoded string) space.Label {
	if encoded == "" || len(encoded) > 342 {
		return space.Label{}
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return space.Label{}
	}
	label, err := space.Parse(string(decoded))
	if err != nil {
		return space.Label{}
	}
	return label
}
