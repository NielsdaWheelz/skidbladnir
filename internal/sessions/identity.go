package sessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var (
	sessionIDPattern     = regexp.MustCompile(`^\$[0-9]+$`)
	paneIDPattern        = regexp.MustCompile(`^%[0-9]+$`)
	serverEpochPattern   = regexp.MustCompile(`^v1-[0-9a-f]{32}$`)
	serverPIDPattern     = regexp.MustCompile(`^[1-9][0-9]*$`)
	serverStartPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
	identityTokenPattern = regexp.MustCompile(`^v1-[0-9a-f]{32}\.[1-9][0-9]*\.[1-9][0-9]*\.[0-9]+$`)
)

// errSessionServerObservation is never produced now; recovery.go still matches
// it (W2.8 removes it).
var errSessionServerObservation = errors.New("session server observation failed")

func sessionNotFound() *Error {
	return newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
}

func sessionIdentityMismatch() *Error {
	return newSessionError(ErrorSessionIdentityMismatch, "The session changed. Refresh and try again.")
}

func newServerEpoch() (string, error) {
	random := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("mint tmux server epoch: %w", err)
	}
	return "v1-" + hex.EncodeToString(random), nil
}

func validServerIdentity(identity tmuxclient.ServerIdentity) bool {
	return serverEpochPattern.MatchString(identity.Epoch) &&
		serverPIDPattern.MatchString(identity.PID) && serverStartPattern.MatchString(identity.StartTime)
}

// Admission precedence for a client token: a malformed id is not found (1); a
// bad or foreign token is a mismatch (2); no server, or the id absent from it,
// is not found (3); a different server identity is a mismatch (4).
// parseLifetime applies 1 and 2 without reading tmux; the guarded effect that
// follows enforces the lifetime itself.
func parseLifetime(id, token string) (tmuxclient.ServerIdentity, error) {
	if !sessionIDPattern.MatchString(id) {
		return tmuxclient.ServerIdentity{}, sessionNotFound()
	}
	// The pattern admits exactly four dot-separated fields, each canonical.
	fields := strings.Split(token, ".")
	if !identityTokenPattern.MatchString(token) || "$"+fields[len(fields)-1] != id {
		return tmuxclient.ServerIdentity{}, sessionIdentityMismatch()
	}
	return tmuxclient.ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}, nil
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
