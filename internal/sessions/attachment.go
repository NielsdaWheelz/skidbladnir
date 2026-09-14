package sessions

import (
	"context"
	"errors"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

var (
	ErrTerminalCleanupFailed            = errors.New("terminal attachment cleanup failed")
	ErrTerminalConfigurationUnsupported = errors.New("tmux terminal configuration is unsupported")
)

type TerminalAttachment struct {
	sourceID string
	runtime  *tmuxclient.Attachment
}

func (manager *Manager) ValidateTerminal(ctx context.Context, id, identityToken string) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, _, err := manager.terminalIdentity(ctx, id, identityToken)
	return err
}

func (manager *Manager) OpenTerminal(ctx context.Context, input OpenTerminalInput) (*TerminalAttachment, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, name, err := manager.terminalIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return nil, err
	}
	runtime, err := manager.tmux.StartAttachment(ctx, tmuxclient.AttachmentSpec{
		SourceID: input.TmuxID, SourceName: name,
		Columns: input.Columns, Rows: input.Rows, Server: server,
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
	return &TerminalAttachment{sourceID: input.TmuxID, runtime: runtime}, nil
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

func (manager *Manager) terminalIdentity(ctx context.Context, id, identityToken string) (tmuxclient.ServerIdentity, string, error) {
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
	if err != nil || observed != server {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it.")
	}
	return server, name, nil
}
