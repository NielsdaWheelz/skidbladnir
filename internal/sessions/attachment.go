package sessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

const shadowReleaseTimeout = 3 * time.Second

var ErrTerminalCleanupFailed = errors.New("terminal attachment cleanup failed")

type TerminalPresence struct {
	AttachedClients int
	OwnsGeometry    bool
}

type TerminalAttachment struct {
	manager    *Manager
	sourceID   string
	shadowName string
	runtime    *tmuxclient.Attachment
}

func (manager *Manager) ValidateTerminal(ctx context.Context, id, identityToken string) error {
	manager.mutations.RLock()
	defer manager.mutations.RUnlock()
	_, _, err := manager.terminalIdentity(ctx, id, identityToken)
	return err
}

func (manager *Manager) OpenTerminal(ctx context.Context, id, identityToken string) (*TerminalAttachment, error) {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()

	server, name, err := manager.terminalIdentity(ctx, id, identityToken)
	if err != nil {
		return nil, err
	}
	shadowName, err := newShadowName()
	if err != nil {
		return nil, err
	}
	runtime, err := manager.tmux.StartAttachment(ctx, tmuxclient.AttachmentSpec{
		SourceID: id, SourceName: name, ShadowName: shadowName, Server: server,
	})
	if errors.Is(err, tmuxclient.ErrAttachmentIdentityMismatch) {
		classified := error(newSessionError(ErrorSessionIdentityMismatch, "The session changed; refresh before opening it."))
		if errors.Is(err, tmuxclient.ErrAttachmentCleanupFailed) {
			classified = errors.Join(classified, ErrTerminalCleanupFailed)
		}
		return nil, classified
	}
	if err != nil {
		classified := manager.classifyMissingSession(ctx, id, err)
		if errors.Is(err, tmuxclient.ErrAttachmentCleanupFailed) {
			classified = errors.Join(classified, ErrTerminalCleanupFailed)
		}
		return nil, classified
	}
	if _, exists := manager.activeShadows[shadowName]; exists {
		panic("phone shadow name collision") // justify-defect: 128-bit names are unique within one manager lifetime.
	}
	manager.activeShadows[shadowName] = struct{}{}
	return &TerminalAttachment{manager: manager, sourceID: id, shadowName: shadowName, runtime: runtime}, nil
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

func (attachment *TerminalAttachment) Presence(ctx context.Context) (TerminalPresence, error) {
	presence, err := attachment.runtime.Presence(ctx)
	if err != nil {
		return TerminalPresence{}, err
	}
	return TerminalPresence{AttachedClients: presence.AttachedClients, OwnsGeometry: presence.OwnsGeometry}, nil
}

func (attachment *TerminalAttachment) ClosePTY() error {
	return attachment.runtime.ClosePTY()
}

func (attachment *TerminalAttachment) CloseClient() error {
	return attachment.runtime.CloseClient()
}

func (attachment *TerminalAttachment) ReleaseShadow() error {
	attachment.manager.mutations.Lock()
	defer attachment.manager.mutations.Unlock()
	defer delete(attachment.manager.activeShadows, attachment.shadowName)
	ctx, cancel := context.WithTimeout(context.Background(), shadowReleaseTimeout)
	defer cancel()
	return attachment.runtime.ReleaseShadow(ctx)
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
	internal, err := manager.sessionOption(ctx, id, "@skid_internal")
	if err != nil {
		return tmuxclient.ServerIdentity{}, "", err
	}
	if tmuxclient.IsPhoneShadow(name, internal) {
		return tmuxclient.ServerIdentity{}, "", newSessionError(ErrorSessionNotFound, "That tmux session no longer exists.")
	}
	return server, name, nil
}

func newShadowName() (string, error) {
	return shadowNameFromEntropy(rand.Reader)
}

func shadowNameFromEntropy(entropy io.Reader) (string, error) {
	random := make([]byte, 16)
	if _, err := io.ReadFull(entropy, random); err != nil {
		return "", fmt.Errorf("mint phone shadow name: %w", err)
	}
	return "skid-phone-" + hex.EncodeToString(random), nil
}
