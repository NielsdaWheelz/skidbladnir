package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/coder/websocket"
)

const (
	sessionIdentityHeader        = "Skidbladnir-Session-Identity"
	terminalPresenceInterval     = 2 * time.Second
	terminalObservationTimeout   = 3 * time.Second
	terminalLivenessInterval     = 10 * time.Second
	terminalLivenessTimeout      = 5 * time.Second
	terminalInitialResizeTimeout = 5 * time.Second
	terminalStartupTimeout       = 5 * time.Second
	terminalWriteTimeout         = 5 * time.Second
	terminalShutdownTimeout      = 8 * time.Second
	terminalPTYReadBufferBytes   = 32 * 1024
)

// liveConnection is one terminal or notification stream. cancel ends it; done closes after its cleanup,
// and cleanupErr is written before done closes.
type liveConnection struct {
	deviceID, sessionID, identityToken string
	cancel                             context.CancelFunc
	done                               chan struct{}
	cleanupErr                         error
}

func (gateway *Gateway) openTerminal(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("tmuxId")
	identityValues := request.Header.Values(sessionIdentityHeader)
	if len(identityValues) != 1 || identityValues[0] == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	identityToken := identityValues[0]
	resolve, cancelResolve := context.WithTimeout(request.Context(), 5*time.Second)
	err := gateway.sessions.ResolveSession(resolve, id, identityToken)
	cancelResolve()
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		writer.(*trackedResponseWriter).errorCode = logging.ErrorInvalidRequest
		return
	}
	connection.SetReadLimit(-1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	live := gateway.register(request.Context().Value(deviceContextKey{}).(string), id, identityToken, cancel)
	if live == nil {
		writeTerminalErrorAndClose(request.Context(), connection, terminal.ErrorReconnectRequired)
		return
	}
	var cleanupErr error
	defer func() { gateway.unregister(live, cleanupErr) }()
	initial, ok := readInitialResize(ctx, connection)
	if !ok {
		return
	}
	cleanupErr = gateway.runTerminal(ctx, connection, sessions.OpenTerminalInput{
		TmuxID: id, IdentityToken: identityToken, Columns: initial.Columns, Rows: initial.Rows,
	})
}

// runTerminal attaches the pty, writes Hello, then pumps until ctx ends or a worker ends the connection,
// writing at most one final Error frame. It returns the pty and tmux client release error.
func (gateway *Gateway) runTerminal(ctx context.Context, connection *websocket.Conn, input sessions.OpenTerminalInput) (cleanupErr error) {
	startup, cancelStartup := context.WithTimeout(ctx, terminalStartupTimeout)
	defer cancelStartup()
	attachment, err := gateway.sessions.OpenTerminal(ctx, startup, input)
	if err != nil {
		writeTerminalErrorAndClose(ctx, connection, terminalCodeForSessionError(err))
		if errors.Is(err, sessions.ErrTerminalCleanupFailed) {
			return errors.New("terminal startup cleanup failed")
		}
		return nil
	}
	runtime, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		_ = connection.CloseNow()
		cleanupErr = errors.Join(attachment.ClosePTY(), attachment.CloseClient())
		workers.Wait()
	}()

	greeting, err := attachment.Greeting(startup)
	if err != nil {
		writeTerminalErrorAndClose(runtime, connection, terminal.ErrorReconnectRequired)
		return nil
	}
	hello := terminal.HelloFrame{AttachedClients: greeting.AttachedClients, PaneID: greeting.PaneID}
	if foreground := greeting.Foreground; foreground != nil {
		hello.Foreground = &terminal.Foreground{
			Provider: foreground.Provider.String(), PID: int64(foreground.PID), StartIdentity: string(foreground.StartIdentity),
		}
	}
	if connection.Write(startup, websocket.MessageText, terminal.EncodeHello(hello)) != nil {
		return nil
	}
	cancelStartup()

	// gate serializes data frames so that nothing follows the final frame.
	var gate sync.Mutex
	ended := false
	send := func(kind websocket.MessageType, payload []byte) bool {
		gate.Lock()
		defer gate.Unlock()
		if ended {
			return false
		}
		write, cancelWrite := context.WithTimeout(runtime, terminalWriteTimeout)
		defer cancelWrite()
		return connection.Write(write, kind, payload) == nil
	}
	endings := make(chan terminal.ErrorCode, 4)
	workers.Add(4)
	go func() { defer workers.Done(); endings <- pumpPTY(attachment, send) }()
	go func() { defer workers.Done(); endings <- pumpInput(runtime, connection, attachment) }()
	go func() {
		defer workers.Done()
		endings <- watchPresence(runtime, attachment, greeting.AttachedClients, send)
	}()
	go func() { defer workers.Done(); endings <- pingPeer(runtime, connection) }()
	var code terminal.ErrorCode
	select {
	case <-ctx.Done():
		return nil
	case code = <-endings:
	}
	// a worker cut short by ctx may still report a code; revocation, close and shutdown end without a frame.
	if code == "" || ctx.Err() != nil {
		return nil
	}
	// coder/websocket closes the connection when a Read or Write ctx ends, so runtime stays live until the
	// final frame is written; the timer bounds the in-flight frame plus the final one.
	stop := time.AfterFunc(terminalWriteTimeout, func() { _ = connection.CloseNow() })
	defer stop.Stop()
	gate.Lock()
	ended = true
	_ = connection.Write(ctx, websocket.MessageText, terminal.EncodeError(code))
	gate.Unlock()
	return nil
}

func pumpPTY(attachment *sessions.TerminalAttachment, send func(websocket.MessageType, []byte) bool) terminal.ErrorCode {
	buffer := make([]byte, terminalPTYReadBufferBytes)
	for {
		count, err := attachment.Read(buffer)
		if count > 0 && !send(websocket.MessageBinary, buffer[:count]) {
			return ""
		}
		if err != nil {
			return terminal.ErrorReconnectRequired
		}
		if count == 0 {
			return terminal.ErrorInternal
		}
	}
}

func pumpInput(ctx context.Context, connection *websocket.Conn, attachment *sessions.TerminalAttachment) terminal.ErrorCode {
	for {
		kind, payload, oversized, err := readMessage(ctx, connection)
		if err != nil {
			return ""
		}
		if oversized {
			return terminal.ErrorRequestTooLarge
		}
		if kind == websocket.MessageBinary {
			if len(payload) > 0 {
				if _, err := attachment.Write(payload); err != nil {
					return terminal.ErrorReconnectRequired
				}
			}
			continue
		}
		frame, err := terminal.ParseClientText(payload)
		if err != nil {
			return terminal.ErrorInvalidRequest
		}
		if frame.Detach {
			return ""
		}
		if err := attachment.Resize(frame.Columns, frame.Rows); err != nil {
			return terminal.ErrorReconnectRequired
		}
	}
}

// watchPresence polls tmux for attached clients, which it does not stream, and sends each change.
func watchPresence(
	ctx context.Context,
	attachment *sessions.TerminalAttachment,
	previous int,
	send func(websocket.MessageType, []byte) bool,
) terminal.ErrorCode {
	ticker := time.NewTicker(terminalPresenceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
		}
		observation, cancel := context.WithTimeout(ctx, terminalObservationTimeout)
		observed, err := attachment.AttachedClients(observation)
		cancel()
		if err != nil {
			return terminal.ErrorReconnectRequired
		}
		if observed == previous {
			continue
		}
		if !send(websocket.MessageText, terminal.EncodePresence(observed)) {
			return ""
		}
		previous = observed
	}
}

// pingPeer pings the peer, whose silent loss emits no transport close, releasing a dead peer's pty
// and tmux client within the liveness interval plus timeout.
func pingPeer(ctx context.Context, connection *websocket.Conn) terminal.ErrorCode {
	ticker := time.NewTicker(terminalLivenessInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
		}
		ping, cancel := context.WithTimeout(ctx, terminalLivenessTimeout)
		err := connection.Ping(ping)
		cancel()
		if err != nil {
			return ""
		}
	}
}

func readMessage(ctx context.Context, connection *websocket.Conn) (websocket.MessageType, []byte, bool, error) {
	kind, reader, err := connection.Reader(ctx)
	if err != nil {
		return 0, nil, false, err
	}
	payload, err := io.ReadAll(io.LimitReader(reader, terminal.MaximumFrameBytes+1))
	if err != nil {
		return 0, nil, false, err
	}
	if len(payload) > terminal.MaximumFrameBytes {
		return kind, nil, true, nil
	}
	return kind, payload, false, nil
}

// readInitialResize reads the mandatory first frame, which decides the pty size, before any terminal
// resource exists. A timer, not a deadline ctx, bounds the wait: a ctx ending under the read would make
// the library close the connection beneath the refusal frame owed to the peer.
func readInitialResize(ctx context.Context, connection *websocket.Conn) (terminal.ClientFrame, bool) {
	type firstFrame struct {
		kind      websocket.MessageType
		payload   []byte
		oversized bool
		err       error
	}
	frames := make(chan firstFrame, 1)
	go func() {
		kind, payload, oversized, err := readMessage(ctx, connection)
		frames <- firstFrame{kind: kind, payload: payload, oversized: oversized, err: err}
	}()
	budget := time.NewTimer(terminalInitialResizeTimeout)
	defer budget.Stop()
	var first firstFrame
	select {
	case <-budget.C:
		writeTerminalErrorAndClose(ctx, connection, terminal.ErrorInvalidRequest)
		return terminal.ClientFrame{}, false
	case first = <-frames:
	}
	if first.err != nil {
		_ = connection.CloseNow()
		return terminal.ClientFrame{}, false
	}
	if first.oversized {
		writeTerminalErrorAndClose(ctx, connection, terminal.ErrorRequestTooLarge)
		return terminal.ClientFrame{}, false
	}
	if first.kind != websocket.MessageText {
		writeTerminalErrorAndClose(ctx, connection, terminal.ErrorInvalidRequest)
		return terminal.ClientFrame{}, false
	}
	frame, err := terminal.ParseClientText(first.payload)
	if err != nil {
		writeTerminalErrorAndClose(ctx, connection, terminal.ErrorInvalidRequest)
		return terminal.ClientFrame{}, false
	}
	if frame.Detach {
		_ = connection.CloseNow()
		return terminal.ClientFrame{}, false
	}
	return frame, true
}

func writeTerminalErrorAndClose(parent context.Context, connection *websocket.Conn, code terminal.ErrorCode) {
	ctx, cancel := context.WithTimeout(parent, terminalWriteTimeout)
	_ = connection.Write(ctx, websocket.MessageText, terminal.EncodeError(code))
	cancel()
	_ = connection.CloseNow()
}

func terminalCodeForSessionError(err error) terminal.ErrorCode {
	if errors.Is(err, sessions.ErrTerminalConfigurationUnsupported) {
		return terminal.ErrorTerminalConfigurationUnsupported
	}
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		return terminal.ErrorInternal
	}
	switch sessionError.Code {
	case sessions.ErrorSessionNotFound, sessions.ErrorSessionIdentityMismatch:
		return terminal.ErrorReconnectRequired
	default:
		return terminal.ErrorInternal
	}
}

// closeSession ends the session's live connections, waits for their cleanup for at most six seconds while
// keeping the last two seconds of ctx's deadline for deletion, then deletes the session only if its lifetime
// still matches identityToken. Cleanup failure is logged and never vetoes deletion.
func (gateway *Gateway) closeSession(ctx context.Context, tmuxID, identityToken string) error {
	startedAt := time.Now()
	deadline, _ := ctx.Deadline()
	cleanupDeadline := startedAt.Add(6 * time.Second)
	if reserve := deadline.Add(-2 * time.Second); reserve.Before(cleanupDeadline) {
		cleanupDeadline = reserve
	}
	cleanup, cancelCleanup := context.WithDeadline(ctx, cleanupDeadline)
	err := gateway.endConnections(cleanup, func(connection *liveConnection) bool {
		return connection.sessionID == tmuxID && connection.identityToken == identityToken
	})
	cancelCleanup()
	if err != nil {
		gateway.log(logging.NewTerminalCleanupFailed())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := gateway.sessions.Kill(ctx, sessions.KillInput{TmuxID: tmuxID, IdentityToken: identityToken}); err != nil {
		return err
	}
	event, err := logging.NewSessionKilled(tmuxID, time.Since(startedAt))
	if err != nil {
		panic("invalid session-killed log event")
	}
	gateway.log(event)
	return nil
}

// register records a live connection; nil means the device is closed or the gateway is closing.
func (gateway *Gateway) register(deviceID, sessionID, identityToken string, cancel context.CancelFunc) *liveConnection {
	gateway.liveMutex.Lock()
	defer gateway.liveMutex.Unlock()
	if gateway.closing || gateway.closedDevices[deviceID] {
		return nil
	}
	connection := &liveConnection{deviceID: deviceID, sessionID: sessionID, identityToken: identityToken, cancel: cancel, done: make(chan struct{})}
	gateway.live[connection] = struct{}{}
	return connection
}

func (gateway *Gateway) unregister(connection *liveConnection, cleanupErr error) {
	gateway.liveMutex.Lock()
	delete(gateway.live, connection)
	gateway.liveMutex.Unlock()
	connection.cleanupErr = cleanupErr
	close(connection.done)
}

// CloseDevice fences deviceID for the process lifetime and cancels its live connections.
func (gateway *Gateway) CloseDevice(deviceID string) {
	gateway.liveMutex.Lock()
	defer gateway.liveMutex.Unlock()
	gateway.closedDevices[deviceID] = true
	for connection := range gateway.live {
		if connection.deviceID == deviceID {
			connection.cancel()
		}
	}
}

// endConnections cancels the matching connections and waits for each cleanup until ctx ends.
func (gateway *Gateway) endConnections(ctx context.Context, match func(*liveConnection) bool) error {
	gateway.liveMutex.Lock()
	var ending []*liveConnection
	for connection := range gateway.live {
		if match(connection) {
			connection.cancel()
			ending = append(ending, connection)
		}
	}
	gateway.liveMutex.Unlock()
	var cleanupErrors []error
	for _, connection := range ending {
		select {
		case <-connection.done:
			if connection.cleanupErr != nil {
				cleanupErrors = append(cleanupErrors, errors.New("live connection cleanup failed"))
			}
		case <-ctx.Done():
			return errors.New("live connection cleanup did not finish")
		}
	}
	return errors.Join(cleanupErrors...)
}

// closeLiveConnections refuses later admissions, then ends every live connection, waiting at most eight seconds.
func (gateway *Gateway) closeLiveConnections(ctx context.Context) error {
	gateway.liveMutex.Lock()
	gateway.closing = true
	gateway.liveMutex.Unlock()
	ctx, cancel := context.WithTimeout(ctx, terminalShutdownTimeout)
	defer cancel()
	return gateway.endConnections(ctx, func(*liveConnection) bool { return true })
}
