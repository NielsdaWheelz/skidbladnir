// Package terminalclient lends a local tty to one exact remote tmux session.
package terminalclient

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/charmbracelet/x/ansi"
	"github.com/coder/websocket"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

var errDimensions = errors.New("terminal size must be 20–1024 columns and 5–512 rows; detached, work continues")

// Run owns one exact terminal visit. Notification failures remain secondary.
func Run(ctx context.Context, client *fleetclient.Client, session fleetclient.Session, input, output *os.File, changed func(NotificationView, error)) (result error) {
	var notificationMu sync.Mutex
	notify := func(view NotificationView, err error) {
		notificationMu.Lock()
		defer notificationMu.Unlock()
		changed(view, err)
	}
	request := fleetclient.Request{Operation: "enter", Ref: session.Ref}
	ref, err := fleetclient.DecodeReference(session.Ref)
	if err != nil || ref.Conversation != nil {
		return errors.New("enter requires an exact terminal reference")
	}
	key := fleetclient.NotificationKey(ref)
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("enter requires stdin and stdout ttys")
	}
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	columns, rows, err := term.GetSize(int(output.Fd()))
	if err != nil {
		return errors.New("cannot measure terminal")
	}
	if _, err := terminal.EncodeResize(columns, rows); err != nil {
		return errDimensions
	}
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	// Optional notification work must precede the websocket admission deadline.
	view, notificationErr := ReadNotifications(ctx, client)
	if notificationErr != nil {
		notify(view, notificationErr)
	}
	foreground := fleetclient.NotificationForeground(session)
	token := view.readyToken(key, foreground)
	var visit *macnotifications.Visit
	visitEnded := false
	var store *fleetclient.NotificationStore
	defer func() {
		if visitEnded {
			// The owner has confirmed retirement before this secondary refresh.
			current, err := ReadNotifications(ctx, client)
			notify(current, err)
		}
	}()
	if runtime.GOOS == "darwin" {
		args := macnotifications.RegisterArgs{Key: key, Foreground: foreground, LaunchNonce: os.Getenv("SKID_NOTIFICATION_LAUNCH")}
		if token.Generation > 0 {
			args.ReadyToken = &token
		}
		visit, err = macnotifications.Register(ctx, args)
		if err != nil {
			notify(NotificationView{}, err)
		} else {
			visitCtx, cancelVisit := context.WithCancel(ctx)
			visitDone := make(chan struct{})
			go func() {
				defer close(visitDone)
				for {
					select {
					case <-visitCtx.Done():
						return
					case <-visit.Disconnected():
					}
					notify(NotificationView{}, attention.ErrUnavailable)
					for delay := time.Second; ; delay = min(2*delay, 30*time.Second) {
						timer := time.NewTimer(delay)
						select {
						case <-visitCtx.Done():
							timer.Stop()
							return
						case <-timer.C:
						}
						if visit.Reconnect(visitCtx) == nil {
							break
						}
					}
				}
			}()
			defer func() {
				cancelVisit()
				<-visitDone
				endCtx, cancelEnd := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
				endErr := visit.End(endCtx)
				cancelEnd()
				if endErr != nil {
					notify(NotificationView{}, endErr)
				} else {
					visitEnded = true
				}
				if err := visit.Close(); err != nil {
					notify(NotificationView{}, err)
				}
			}()
		}
	} else {
		store, err = fleetclient.DefaultNotificationStore()
		if err != nil {
			notify(NotificationView{}, err)
		}
	}
	connection, failure := client.OpenTerminal(ctx, request)
	if failure != nil {
		return errors.New(failure.Code)
	}
	defer connection.CloseNow()
	columns, rows, err = term.GetSize(int(output.Fd()))
	if err != nil {
		return errors.New("cannot measure terminal")
	}
	if _, err := terminal.EncodeResize(columns, rows); err != nil {
		return errDimensions
	}
	original, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return errors.New("cannot enter raw terminal mode")
	}
	defer func() {
		// The remote program may have enabled presentation modes before transport loss.
		_, writeErr := io.WriteString(output, ansi.ResetStyle+ansi.ResetModeCursorKeys+ansi.KeypadNumericMode+
			ansi.ResetModeFocusEvent+ansi.ShowCursor+ansi.ResetModeMouseNormal+ansi.ResetModeMouseButtonEvent+
			ansi.ResetModeMouseAnyEvent+ansi.ResetModeMouseExtSgr+ansi.ResetModeBracketedPaste+ansi.ResetModeLightDark+
			ansi.ResetModifyOtherKeys+ansi.ResetModeLeftRightMargin+ansi.SetTopBottomMargins(0, 0)+ansi.ResetModeAltScreenSaveCursor)
		restoreErr := term.Restore(int(input.Fd()), original)
		if writeErr != nil || restoreErr != nil {
			result = errors.Join(result, errors.New("terminal restoration failed"))
		}
	}()
	return stream(ctx, connection, input, output, columns, rows, resized, func() (int, int, error) { return term.GetSize(int(output.Fd())) }, func() {
		if runtime.GOOS == "darwin" {
			if visit == nil {
				return
			}
			if err := visit.Presented(ctx, token); err != nil {
				notify(NotificationView{}, err)
			}
			return
		}
		if store == nil || token.Generation == 0 {
			return
		}
		device, err := store.Presented(key, foreground, token)
		view.Device = device
		notify(view, err)
	})
}

// stream cancels and joins both readers before returning ownership of stdin.
func stream(ctx context.Context, connection *websocket.Conn, input *os.File, output io.Writer, columns, rows int, resized <-chan os.Signal, size func() (int, int, error), presented func()) (result error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	connection.SetReadLimit(terminal.MaximumFrameBytes)
	resize, err := terminal.EncodeResize(columns, rows)
	if err != nil {
		return errDimensions
	}
	if err := write(ctx, connection, websocket.MessageText, resize); err != nil {
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
	switch frame := frame.(type) {
	case terminal.HelloFrame:
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
			resize, err := terminal.EncodeResize(columns, rows)
			if err != nil {
				return errDimensions
			}
			if err := write(ctx, connection, websocket.MessageText, resize); err != nil {
				return err
			}
		case event := <-outputs:
			if event.err != nil {
				return errors.New("terminal connection ended; work may continue")
			}
			if event.kind == websocket.MessageBinary {
				remaining := event.body
				for len(remaining) > 0 {
					n, err := output.Write(remaining)
					if err != nil || n <= 0 {
						return errors.New("terminal output unavailable")
					}
					remaining = remaining[n:]
				}
				if len(event.body) > 0 && !presentedOutput {
					presentedOutput = true
					presented()
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
