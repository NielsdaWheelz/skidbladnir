package sessions

import (
	"context"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// The attachment sentinels are tmux's own; consumers keep these names.
var (
	ErrTerminalCleanupFailed            = tmuxclient.ErrAttachmentCleanupFailed
	ErrTerminalConfigurationUnsupported = tmuxclient.ErrAttachmentConfigurationUnsupported
)

type TerminalAttachment struct {
	runtime *tmuxclient.Attachment
}

// OpenTerminal attaches to the token's lifetime; the attachment's own tmux queue
// enforces it, and one read classifies a refusal or failure whose session is
// gone.
func (manager *Manager) OpenTerminal(lifetimeCtx, startupCtx context.Context, input OpenTerminalInput) (*TerminalAttachment, error) {
	server, err := parseLifetime(input.TmuxID, input.IdentityToken)
	if err != nil {
		return nil, err
	}
	runtime, err := manager.tmux.StartAttachment(lifetimeCtx, startupCtx, tmuxclient.AttachmentSpec{
		SourceID: input.TmuxID,
		Columns:  input.Columns, Rows: input.Rows, Server: server,
	})
	if err != nil {
		// err itself carries the configuration and cleanup sentinels; a
		// replacement classification carries cleanup forward.
		var classified error
		switch {
		case errors.Is(err, tmuxclient.ErrAttachmentIdentityMismatch):
			classified = manager.refused(startupCtx, input.TmuxID)
		case errors.Is(err, ErrTerminalConfigurationUnsupported):
			return nil, err
		default:
			var sessionErr *Error
			_, _, resolveErr := manager.resolve(startupCtx, input.TmuxID, input.IdentityToken)
			if !errors.As(resolveErr, &sessionErr) || sessionErr.Code != ErrorSessionNotFound {
				return nil, err
			}
			classified = resolveErr
		}
		if errors.Is(err, ErrTerminalCleanupFailed) {
			classified = errors.Join(classified, ErrTerminalCleanupFailed)
		}
		return nil, classified
	}
	return &TerminalAttachment{runtime: runtime}, nil
}

type TerminalGreeting struct {
	AttachedClients int
	PaneID          string
	Foreground      *agentruntime.Foreground
}

// Greeting samples the owned client's final pane without inventory or account
// environment reads. Optional kernel uncertainty never refuses entry.
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
		if foreground, found := agentruntime.ClassifyForeground(*b); found {
			greeting.Foreground = &foreground
		}
	}
	return greeting, nil
}

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
