package sessions

import (
	"context"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminalservice"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func terminalServiceUnavailable(unmanaged bool) *Error {
	message := "the terminal service is unavailable on this machine."
	if unmanaged {
		message = "the default tmux server is not managed by skid. complete terminal-service setup on this machine."
	}
	return newSessionError(ErrorTerminalServiceUnavailable, message)
}

func (manager *Manager) admittedServer() tmuxclient.ServerIdentity {
	if server := manager.admitted.Load(); server != nil {
		return *server
	}
	return tmuxclient.ServerIdentity{}
}

// Ownership is cached only for the exact admitted lifetime. Every caller proves
// that lifetime from a fresh tmux observation before reusing it.
func (manager *Manager) terminalState(ctx context.Context, observed tmuxclient.ServerObservation, present bool) (terminalservice.State, error) {
	if present && observed.Identity == manager.admittedServer() {
		return terminalservice.Managed, nil
	}
	state, err := terminalservice.Inspect(ctx, observed, present)
	if err == nil && state == terminalservice.Managed {
		server := observed.Identity
		manager.admitted.Store(&server)
	}
	return state, err
}

func (manager *Manager) requireManaged(ctx context.Context, observed tmuxclient.ServerObservation, present bool) error {
	state, err := manager.terminalState(ctx, observed, present)
	if err != nil {
		return err
	}
	if state != terminalservice.Managed {
		return terminalServiceUnavailable(state == terminalservice.Unmanaged)
	}
	return nil
}

// admitServer proves native ownership before stamping an epoch, even when empty.
func (manager *Manager) admitServer(ctx context.Context, observed tmuxclient.ServerObservation, present bool) (tmuxclient.ServerIdentity, error) {
	if err := manager.requireManaged(ctx, observed, present); err != nil {
		return tmuxclient.ServerIdentity{}, err
	}
	server := observed.Identity
	if server.Epoch == "" {
		var err error
		server, err = manager.tmux.EnsureServerIdentity(ctx, server, newServerEpoch())
		if err != nil {
			return tmuxclient.ServerIdentity{}, err
		}
		manager.admitted.Store(&server)
	}
	return server, nil
}

func (manager *Manager) requireManagedLifetime(ctx context.Context, expected tmuxclient.ServerIdentity) error {
	observed, present, err := manager.tmux.InspectServer(ctx)
	if err != nil {
		return err
	}
	if !present {
		return sessionNotFound()
	}
	if observed.Identity != expected {
		return sessionIdentityMismatch()
	}
	return manager.requireManaged(ctx, observed, present)
}
