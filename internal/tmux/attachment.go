package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/creack/pty"
)

const (
	attachmentConfigurationMarker = "SKIDBLADNIR_TERMINAL_CONFIGURATION_UNSUPPORTED"
	attachmentReadyLimit          = 5 * time.Second
	attachmentReadyPollInterval   = 25 * time.Millisecond
	attachmentExitLimit           = 2 * time.Second
)

var (
	ErrAttachmentIdentityMismatch         = errors.New("tmux attachment identity changed")
	ErrAttachmentCleanupFailed            = errors.New("tmux attachment startup cleanup failed")
	ErrAttachmentConfigurationUnsupported = errors.New("tmux terminal configuration is unsupported")
)

type AttachmentSpec struct {
	SourceID   string
	SourceName string
	Columns    int
	Rows       int
	Server     ServerIdentity
}

type Attachment struct {
	client        Client
	spec          AttachmentSpec
	pty           *os.File
	command       *exec.Cmd
	processDone   chan struct{}
	startupOutput bytes.Buffer
	clientTTY     string

	ptyMutex        sync.Mutex
	ptyClosed       bool
	closePTYErr     error
	closeClientOnce sync.Once
	closeClientErr  error
}

func (client Client) StartAttachment(ctx context.Context, spec AttachmentSpec) (*Attachment, error) {
	arguments, err := attachmentCommandArguments(spec)
	if err != nil {
		return nil, err
	}
	attachment := &Attachment{client: client, spec: spec, processDone: make(chan struct{})}
	// Tmux renders through the terminal descriptor inherited as stdin. Ordinary
	// command stdout carries only the closed startup failure marker, not the screen.
	command := client.commandWithStderr(ctx, &attachment.startupOutput, nil, arguments[0], arguments[1:]...)
	command.Env = attachmentEnvironment(command.Env)
	terminalPTY, err := pty.StartWithSize(command, &pty.Winsize{Cols: uint16(spec.Columns), Rows: uint16(spec.Rows)})
	if err != nil {
		return nil, fmt.Errorf("start tmux terminal client: %w", err)
	}
	attachment.pty = terminalPTY
	attachment.command = command
	go func() {
		_ = command.Wait() // justify-ignore-error: readiness observes the exact client and startup marker; completion only needs the owned child reaped.
		close(attachment.processDone)
	}()
	if err := attachment.awaitAttached(ctx); err != nil {
		ptyErr := attachment.ClosePTY()
		clientErr := attachment.CloseClient()
		return nil, attachmentStartFailure(err, errors.Join(ptyErr, clientErr))
	}
	return attachment, nil
}

func attachmentStartFailure(cause, cleanupErr error) error {
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(cause, ErrAttachmentCleanupFailed, cleanupErr)
}

func attachmentCommandArguments(spec AttachmentSpec) ([]string, error) {
	if !sessionIDPattern.MatchString(spec.SourceID) || spec.SourceName == "" || !spec.Server.valid() ||
		spec.Columns < terminal.MinimumColumns || spec.Columns > terminal.MaximumColumns || spec.Rows < terminal.MinimumRows || spec.Rows > terminal.MaximumRows {
		return nil, errors.New("tmux attachment identity or geometry is invalid")
	}
	options := "#{&&:#{==:#{window-size},latest},#{&&:#{==:#{destroy-unattached},off},#{==:#{detach-on-destroy},on}}}"
	return []string{
		"-T", "RGB", "if-shell", "-F", "-t", spec.SourceID,
		mutationIdentityCondition(spec.SourceID, spec.SourceName, spec.Server),
		"if-shell -F -t '" + spec.SourceID + "' '" + options + "' \"attach-session -E -t '" + spec.SourceID + "'\" \"display-message -p -l '" + attachmentConfigurationMarker + "'\"",
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func (attachment *Attachment) Read(contents []byte) (int, error) {
	return attachment.pty.Read(contents)
}
func (attachment *Attachment) Write(contents []byte) (int, error) {
	return attachment.pty.Write(contents)
}

func (attachment *Attachment) Resize(columns, rows int) error {
	// The descriptor must not race close and be reused for an unrelated file.
	attachment.ptyMutex.Lock()
	defer attachment.ptyMutex.Unlock()
	if attachment.ptyClosed {
		return errors.New("terminal attachment is closed")
	}
	return pty.Setsize(attachment.pty, &pty.Winsize{Cols: uint16(columns), Rows: uint16(rows)})
}

func (attachment *Attachment) AttachedClients(ctx context.Context) (int, error) {
	count, tty, found, err := attachment.observeClient(ctx)
	if err != nil {
		return 0, err
	}
	if !found || tty != attachment.clientTTY {
		return 0, ErrAttachmentIdentityMismatch
	}
	return count, nil
}

func (attachment *Attachment) observeClient(ctx context.Context) (int, string, bool, error) {
	output, err := attachment.client.Output(ctx, "read-terminal-client", "list-clients", "-F",
		"#{client_pid}|#{client_tty}|#{"+ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}|#{session_attached}")
	if err != nil {
		return 0, "", false, err
	}
	return parseAttachmentClient(output, attachment.command.Process.Pid, attachment.spec)
}

func parseAttachmentClient(output string, pid int, spec AttachmentSpec) (int, string, bool, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "|")
		if fields[0] != strconv.Itoa(pid) {
			continue
		}
		if len(fields) != 7 || fields[1] == "" {
			return 0, "", false, errors.New("tmux terminal client observation is invalid")
		}
		server := ServerIdentity{Epoch: fields[2], PID: fields[3], StartTime: fields[4]}
		if server != spec.Server || fields[5] != spec.SourceID {
			return 0, "", false, ErrAttachmentIdentityMismatch
		}
		count, err := strconv.Atoi(fields[6])
		if err != nil || count < 1 {
			return 0, "", false, errors.New("tmux terminal client count is invalid")
		}
		return count, fields[1], true, nil
	}
	return 0, "", false, nil
}

func (attachment *Attachment) awaitAttached(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, attachmentReadyLimit)
	defer cancel()
	// justify-polling: the spawned tmux client exposes no attach notification;
	// read its identity every 25ms for at most 5s without consuming screen bytes.
	ticker := time.NewTicker(attachmentReadyPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-attachment.processDone:
			return attachment.exitBeforeReadyError()
		default:
		}
		_, tty, found, err := attachment.observeClient(ctx)
		if err != nil {
			select {
			case <-attachment.processDone:
				return attachment.exitBeforeReadyError()
			default:
				return err
			}
		}
		if found {
			attachment.clientTTY = tty
			return nil
		}
		select {
		case <-attachment.processDone:
			return attachment.exitBeforeReadyError()
		case <-ctx.Done():
			return fmt.Errorf("tmux attachment readiness ended: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (attachment *Attachment) exitBeforeReadyError() error {
	switch strings.TrimSpace(attachment.startupOutput.String()) {
	case identityMismatchMarker:
		return ErrAttachmentIdentityMismatch
	case attachmentConfigurationMarker:
		return ErrAttachmentConfigurationUnsupported
	default:
		return errors.New("tmux client exited before attachment")
	}
}

func (attachment *Attachment) ClosePTY() error {
	attachment.ptyMutex.Lock()
	defer attachment.ptyMutex.Unlock()
	if !attachment.ptyClosed {
		attachment.ptyClosed = true
		attachment.closePTYErr = attachment.pty.Close()
	}
	return attachment.closePTYErr
}

func (attachment *Attachment) CloseClient() error {
	attachment.closeClientOnce.Do(func() {
		select {
		case <-attachment.processDone:
			return
		case <-time.After(attachmentExitLimit):
		}
		if err := attachment.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			attachment.closeClientErr = fmt.Errorf("stop owned tmux terminal client: %w", err)
			return
		}
		select {
		case <-attachment.processDone:
		case <-time.After(attachmentExitLimit):
			attachment.closeClientErr = errors.New("owned tmux terminal client did not exit")
		}
	})
	return attachment.closeClientErr
}

func attachmentEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "TERM=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "TERM=xterm-256color")
}
