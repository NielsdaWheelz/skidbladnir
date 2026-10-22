package sessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func sessionNotFound() *Error {
	return newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
}

func sessionIdentityMismatch() *Error {
	return newSessionError(ErrorSessionIdentityMismatch, "The session changed. Refresh and try again.")
}

// newServerEpoch mints the epoch skid stamps on a server it adopts.
func newServerEpoch() string {
	var random [16]byte
	rand.Read(random[:]) // never fails since go 1.24
	return "v1-" + hex.EncodeToString(random[:])
}

// Admission precedence for a client token: a malformed id is not found (1); a
// bad or foreign token is a mismatch (2); no server, or the id absent from it,
// is not found (3); a different server identity is a mismatch (4).
// parseLifetime is the one admission of a client session id and identity token
// (epoch.pid.start.<id digits>), under tmux's grammar. It applies 1 and 2
// without reading tmux; the guarded effect that follows enforces the lifetime.
func parseLifetime(id, token string) (tmuxclient.ServerIdentity, error) {
	if !tmuxclient.ValidID(id, '$') {
		return tmuxclient.ServerIdentity{}, sessionNotFound()
	}
	fields := strings.Split(token, ".")
	if len(fields) != 4 || "$"+fields[3] != id {
		return tmuxclient.ServerIdentity{}, sessionIdentityMismatch()
	}
	server, err := tmuxclient.ParseServerIdentity(fields[0], fields[1], fields[2])
	if err != nil || server.Epoch == "" {
		return tmuxclient.ServerIdentity{}, sessionIdentityMismatch()
	}
	return server, nil
}

// resolve applies the whole precedence with one read of id and returns its row.
func (manager *Manager) resolve(ctx context.Context, id, token string) (tmuxclient.ServerIdentity, tmuxclient.InventoryRow, error) {
	server, err := parseLifetime(id, token)
	if err != nil {
		return tmuxclient.ServerIdentity{}, tmuxclient.InventoryRow{}, err
	}
	capture, err := manager.tmux.ReadSession(ctx, id)
	if errors.Is(err, tmuxclient.WorkspaceSourceChanged) {
		return tmuxclient.ServerIdentity{}, tmuxclient.InventoryRow{}, sessionIdentityMismatch()
	}
	if err != nil {
		return tmuxclient.ServerIdentity{}, tmuxclient.InventoryRow{}, err
	}
	if len(capture.Rows) == 0 {
		return tmuxclient.ServerIdentity{}, tmuxclient.InventoryRow{}, sessionNotFound()
	}
	// An epochless server never equals a token's server.
	if capture.Server != server {
		return tmuxclient.ServerIdentity{}, tmuxclient.InventoryRow{}, sessionIdentityMismatch()
	}
	return server, capture.Rows[0], nil
}

// refused classifies a definite guard refusal by one read: a session that is
// gone (or a server that is gone) is not found; anything else changed.
func (manager *Manager) refused(ctx context.Context, id string) error {
	capture, err := manager.tmux.ReadSession(ctx, id)
	if err == nil && len(capture.Rows) == 0 {
		return sessionNotFound()
	}
	return sessionIdentityMismatch()
}

func (manager *Manager) ResolveSession(ctx context.Context, id, token string) error {
	_, _, err := manager.resolve(ctx, id, token)
	return err
}

func (manager *Manager) requireServerIdentity(ctx context.Context, expected tmuxclient.ServerIdentity) error {
	observed, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		return err
	}
	if !present || observed.Identity != expected {
		return tmuxclient.WorkspaceSourceChanged
	}
	return nil
}
