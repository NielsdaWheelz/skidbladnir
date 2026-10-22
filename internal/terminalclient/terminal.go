// Package terminalclient lends a local tty to one exact remote tmux session.
package terminalclient

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/charmbracelet/x/ansi"
	"github.com/coder/websocket"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

var errDimensions = errors.New("terminal size must be 20–1024 columns and 5–512 rows; detached, work continues")

// Result keeps optional notification failures separate from terminal ownership.
type Result struct {
	Err             error
	NotificationErr error
}

// Run owns one exact terminal visit. Notification failures remain secondary.
func Run(ctx context.Context, client *fleetclient.Client, encodedRef string, seed *attention.ReadySeed, input, output *os.File) (result Result) {
	nonce := os.Getenv("SKID_NOTIFICATION_LAUNCH")
	var visit *entryVisit
	defer func() {
		if visit != nil {
			result.NotificationErr = visit.close()
		} else {
			result.NotificationErr = abandonLaunch(context.WithoutCancel(ctx), nonce)
		}
	}()
	ref, err := fleetclient.DecodeReference(encodedRef)
	if err != nil {
		return Result{Err: errors.New("enter requires an exact terminal reference")}
	}
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return Result{Err: errors.New("enter requires stdin and stdout ttys")}
	}
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	columns, rows, err := term.GetSize(int(output.Fd()))
	if err != nil {
		return Result{Err: errors.New("cannot measure terminal")}
	}
	if !terminal.ValidSize(columns, rows) {
		return Result{Err: errDimensions}
	}
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	visit = startEntryVisit(ctx, ref, seed, nonce)
	connection, failure := client.OpenTerminal(ctx, fleetclient.Request{Operation: "enter", Ref: encodedRef})
	if failure != nil {
		return Result{Err: errors.New(failure.Code)}
	}
	defer connection.CloseNow()
	columns, rows, err = term.GetSize(int(output.Fd()))
	if err != nil {
		return Result{Err: errors.New("cannot measure terminal")}
	}
	if !terminal.ValidSize(columns, rows) {
		return Result{Err: errDimensions}
	}
	original, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return Result{Err: errors.New("cannot enter raw terminal mode")}
	}
	defer func() {
		// The remote program may have enabled presentation modes before transport loss.
		_, writeErr := io.WriteString(output, ansi.ResetStyle+ansi.ResetModeCursorKeys+ansi.KeypadNumericMode+
			ansi.ResetModeFocusEvent+ansi.ShowCursor+ansi.ResetModeMouseNormal+ansi.ResetModeMouseButtonEvent+
			ansi.ResetModeMouseAnyEvent+ansi.ResetModeMouseExtSgr+ansi.ResetModeBracketedPaste+ansi.ResetModeLightDark+
			ansi.ResetModifyOtherKeys+ansi.ResetModeLeftRightMargin+ansi.SetTopBottomMargins(0, 0)+ansi.ResetModeAltScreenSaveCursor)
		restoreErr := term.Restore(int(input.Fd()), original)
		if writeErr != nil || restoreErr != nil {
			result.Err = errors.Join(result.Err, errors.New("terminal restoration failed"))
		}
	}()
	result.Err = stream(ctx, connection, input, output, columns, rows, resized, func() (int, int, error) { return term.GetSize(int(output.Fd())) }, visit)
	return result
}

// stream cancels and joins both readers before returning ownership of stdin.
func stream(ctx context.Context, connection *websocket.Conn, input *os.File, output io.Writer, columns, rows int, resized <-chan os.Signal, size func() (int, int, error), visit *entryVisit) (result error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	connection.SetReadLimit(terminal.MaximumFrameBytes)
	if !terminal.ValidSize(columns, rows) {
		return errDimensions
	}
	if err := write(ctx, connection, websocket.MessageText, terminal.EncodeResize(columns, rows)); err != nil {
		return err
	}
	helloContext, cancelHello := context.WithTimeout(ctx, fleetclient.Timeout)
	kind, body, err := connection.Read(helloContext)
	cancelHello()
	if err != nil {
		return errors.New("terminal connection ended before ready")
	}
	frame, err := terminal.ParseServerText(body)
	if err != nil || kind != websocket.MessageText {
		return errors.New("invalid terminal greeting")
	}
	var hello terminal.HelloFrame
	switch frame := frame.(type) {
	case terminal.HelloFrame:
		hello = frame
		visit.greet(hello)
	case terminal.ErrorFrame:
		return errors.New(frame.Message)
	default:
		return errors.New("invalid terminal greeting")
	}
	reader, err := cancelreader.NewReader(input)
	if err != nil {
		return errors.New("cannot acquire cancellable terminal input")
	}
	type event struct {
		kind websocket.MessageType
		body []byte
		err  error
	}
	inputs := make(chan event)
	outputs := make(chan event)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for {
			buffer := make([]byte, 32*1024)
			n, err := reader.Read(buffer)
			select {
			case inputs <- event{body: buffer[:n], err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for {
			kind, body, err := connection.Read(ctx)
			select {
			case outputs <- event{kind: kind, body: body, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() { cancel(); reader.Cancel(); connection.CloseNow(); workers.Wait(); reader.Close() }()
	prefix := false
	presentedOutput := false
	for {
		select {
		case <-ctx.Done():
			return errors.New("terminal detached")
		case <-resized:
			columns, rows, err := size()
			if err != nil {
				return errors.New("cannot measure terminal; detached")
			}
			if !terminal.ValidSize(columns, rows) {
				return errDimensions
			}
			if err := write(ctx, connection, websocket.MessageText, terminal.EncodeResize(columns, rows)); err != nil {
				return err
			}
		case event := <-outputs:
			if event.err != nil {
				return errors.New("terminal connection ended; work may continue")
			}
			if event.kind == websocket.MessageBinary {
				var frozen attention.ReadySeed
				if len(event.body) > 0 && !presentedOutput {
					frozen = visit.freeze(hello)
				}
				if len(event.body) > 0 {
					if _, err := output.Write(event.body); err != nil {
						return errors.New("terminal output unavailable")
					}
				}
				if len(event.body) > 0 && !presentedOutput {
					presentedOutput = true
					visit.written <- frozen
				}
				continue
			}
			frame, err := terminal.ParseServerText(event.body)
			if err != nil {
				return errors.New("invalid terminal response")
			}
			switch frame := frame.(type) {
			case terminal.PresenceFrame:
			case terminal.ErrorFrame:
				return errors.New(frame.Message)
			default:
				return errors.New("unexpected terminal greeting")
			}
		case event := <-inputs:
			forwarded := make([]byte, 0, len(event.body)+1)
			detach := false
			for _, b := range event.body {
				if prefix {
					prefix = false
					if b == 'd' {
						detach = true
						break
					}
					forwarded = append(forwarded, 0x1d)
					if b != 0x1d {
						forwarded = append(forwarded, b)
					}
				} else if b == 0x1d {
					prefix = true
				} else {
					forwarded = append(forwarded, b)
				}
			}
			if len(forwarded) > 0 {
				if err := write(ctx, connection, websocket.MessageBinary, forwarded); err != nil {
					return err
				}
			}
			if detach || event.err != nil {
				return write(ctx, connection, websocket.MessageText, terminal.EncodeDetach())
			}
		}
	}
}

func write(ctx context.Context, connection *websocket.Conn, kind websocket.MessageType, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if connection.Write(ctx, kind, body) != nil {
		return errors.New("terminal write unconfirmed; not repeated")
	}
	return nil
}
