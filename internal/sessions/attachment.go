package sessions

import (
	"context"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var (
	ErrTerminalCleanupFailed            = errors.New("terminal attachment cleanup failed")
	ErrTerminalConfigurationUnsupported = errors.New("tmux terminal configuration is unsupported")
	errSessionServerObservation         = errors.New("session server observation failed")
)

type TerminalAttachment struct {
	sourceID string
	profiles []agentruntime.Profile
	runtime  *tmuxclient.Attachment
}

func (manager *Manager) ValidateTerminal(ctx context.Context, id, identityToken string) error {
	_, _, err := manager.sessionLifetimeIdentity(ctx, id, identityToken)
	if errors.Is(err, errSessionServerObservation) {
		return newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
	}
	return err
}

func (manager *Manager) OpenTerminal(lifetimeCtx, startupCtx context.Context, input OpenTerminalInput) (*TerminalAttachment, error) {
	ctx := startupCtx

	server, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if errors.Is(err, errSessionServerObservation) {
		return nil, newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
	}
	if err != nil {
		return nil, err
	}
	runtime, err := manager.tmux.StartAttachment(lifetimeCtx, startupCtx, tmuxclient.AttachmentSpec{
		SourceID: input.TmuxID,
		Columns:  input.Columns, Rows: input.Rows, Server: server,
	})
	if err != nil {
		var classified error
		switch {
		case errors.Is(err, tmuxclient.ErrAttachmentIdentityMismatch):
			classified = newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
		case errors.Is(err, tmuxclient.ErrAttachmentConfigurationUnsupported):
			classified = ErrTerminalConfigurationUnsupported
		default:
			classified = manager.classifyMissingSession(ctx, input.TmuxID, err)
		}
		if errors.Is(err, tmuxclient.ErrAttachmentCleanupFailed) {
			classified = errors.Join(classified, ErrTerminalCleanupFailed)
		}
		return nil, classified
	}
	return &TerminalAttachment{sourceID: input.TmuxID, runtime: runtime, profiles: manager.Profiles()}, nil
}

type TerminalGreeting struct {
	AttachedClients int
	PaneID          string
	Foreground      *agentruntime.Foreground
}

// Greeting samples the owned client's final pane without inventory, registration
// or profile-environment reads. Optional kernel uncertainty never refuses entry.
func (attachment *TerminalAttachment) Greeting(ctx context.Context) (TerminalGreeting, error) {
	before, err := attachment.runtime.ObserveClient(ctx)
	if err != nil {
		return TerminalGreeting{}, err
	}
	a, aErr := paneForeground(processinfo.PID(before.PanePID))
	after, err := attachment.runtime.ObserveClient(ctx)
	if err != nil {
		return TerminalGreeting{}, err
	}
	b, bErr := paneForeground(processinfo.PID(after.PanePID))
	greeting := TerminalGreeting{AttachedClients: after.AttachedClients, PaneID: after.PaneID}
	if before.PaneID == after.PaneID && before.PanePID == after.PanePID && aErr == nil && bErr == nil && a != nil && b != nil && processinfo.SameObservation(*a, *b) {
		if foreground, found := agentruntime.ClassifyForeground(attachment.profiles, *b); found {
			greeting.Foreground = &foreground
		}
	}
	return greeting, nil
}

func (attachment *TerminalAttachment) SourceID() string { return attachment.sourceID }

func (attachment *TerminalAttachment) Read(contents []byte) (int, error) {
	return attachment.runtime.Read(contents)
}

func (attachment *TerminalAttachment) Write(contents []byte) (int, error) {
	return attachment.runtime.Write(contents)
}

func (attachment *TerminalAttachment) Resize(columns, rows int) error {
	return attachment.runtime.Resize(columns, rows)
}

func (attachment *TerminalAttachment) AttachedClients(ctx context.Context) (int, error) {
	return attachment.runtime.AttachedClients(ctx)
}

func (attachment *TerminalAttachment) ClosePTY() error {
	return attachment.runtime.ClosePTY()
}

func (attachment *TerminalAttachment) CloseClient() error {
	return attachment.runtime.CloseClient()
}

func (manager *Manager) sessionLifetimeIdentity(ctx context.Context, id, identityToken string) (tmuxclient.ServerIdentity, string, error) {
	if !sessionIDPattern.MatchString(id) {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	server, valid := parseIdentityToken(identityToken, id)
	if !valid {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
	}
	name, found, err := manager.sessionIdentity(ctx, id)
	if err != nil {
		return tmuxclient.ServerIdentity{}, "", err
	}
	if !found {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	observed, err := manager.tmux.ServerIdentity(ctx)
	if err != nil {
		return tmuxclient.ServerIdentity{}, "", errors.Join(errSessionServerObservation, err)
	}
	if observed != server {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
	}
	return server, name, nil
}
