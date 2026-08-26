package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/coder/websocket"
)

const (
	sessionIdentityHeader      = "Skidbladnir-Session-Identity"
	terminalPresenceInterval   = 2 * time.Second
	terminalLivenessInterval   = 10 * time.Second
	terminalLivenessTimeout    = 5 * time.Second
	terminalObservationTimeout = 3 * time.Second
	terminalWriteTimeout       = 5 * time.Second
	terminalFinalFrameTimeout  = 5 * time.Second
	terminalShutdownTimeout    = 8 * time.Second
	terminalPTYReadBufferBytes = 32 * 1024
)

type liveTerminal struct {
	sessionID string
	cancel    context.CancelFunc
	done      chan error
}

type terminalEnd uint8

const (
	terminalDetached terminalEnd = iota + 1
	terminalPeerClosed
	terminalReconnect
	terminalInvalidRequest
	terminalRequestTooLarge
	terminalInternalFailure
	terminalTransportFailure
)

func (gateway *Gateway) openTerminal(writer http.ResponseWriter, request *http.Request) {
	id, valid := terminalSessionID(request.URL.Path)
	if !valid {
		writeError(writer, errorInvalidRequest)
		return
	}
	identityValues := request.Header.Values(sessionIdentityHeader)
	if len(identityValues) != 1 || identityValues[0] == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	identityToken := identityValues[0]
	if err := gateway.sessions.ValidateTerminal(request.Context(), id, identityToken); err != nil {
		writeSessionError(writer, err)
		return
	}

	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		if tracked, ok := writer.(interface{ setErrorCode(logging.ErrorCode) }); ok {
			tracked.setErrorCode(logging.ErrorInvalidRequest)
		}
		return
	}
	connection.SetReadLimit(-1)

	terminalContext, cancel := context.WithCancel(context.Background())
	gateway.terminalLifecycle.Lock()
	registration, registered := gateway.registerLiveTerminal(id, cancel)
	gateway.terminalLifecycle.Unlock()
	if !registered {
		cancel()
		writeTerminalErrorAndClose(request.Context(), connection, terminal.ErrorReconnectRequired)
		return
	}
	defer cancel()
	completionErr := error(nil)
	defer func() { gateway.unregisterLiveTerminal(registration, completionErr) }()

	attachment, err := gateway.sessions.OpenTerminal(terminalContext, id, identityToken)
	if err != nil {
		if errors.Is(err, sessions.ErrTerminalCleanupFailed) {
			completionErr = errors.New("terminal startup cleanup failed")
		}
		writeTerminalErrorAndClose(terminalContext, connection, terminalCodeForSessionError(err))
		return
	}
	completionErr = errors.New("terminal cleanup completion was not reported")
	completionErr = gateway.runTerminal(terminalContext, connection, attachment, request.Header.Get("Authorization"))
}

func (gateway *Gateway) runTerminal(
	ctx context.Context,
	connection *websocket.Conn,
	attachment *sessions.TerminalAttachment,
	authorization string,
) (cleanupErr error) {
	runtimeContext, cancel := context.WithCancel(ctx)
	queue := terminal.NewOutboundQueue()
	cleanup := terminal.NewCleanup(terminal.OwnedResources{
		ClosePTY:      attachment.ClosePTY,
		CloseClient:   attachment.CloseClient,
		ReleaseShadow: attachment.ReleaseShadow,
	})
	var workers sync.WaitGroup
	defer func() {
		cancel()
		queue.Close()
		_ = connection.CloseNow() // justify-ignore-error: owned terminal cleanup must not wait for a WebSocket close handshake.
		cleanupErr = cleanup.Close()
		workers.Wait()
	}()

	presence, err := observeTerminalPresence(runtimeContext, attachment)
	if err != nil {
		writeTerminalErrorAndClose(runtimeContext, connection, terminal.ErrorReconnectRequired)
		return
	}
	hello, err := terminal.EncodeHello(presence.AttachedClients, terminalGeometry(presence))
	if err != nil {
		writeTerminalErrorAndClose(runtimeContext, connection, terminal.ErrorInternal)
		return
	}
	if err := queue.EnqueueText(hello); err != nil {
		writeTerminalErrorAndClose(runtimeContext, connection, terminal.ErrorInternal)
		return
	}

	workerResults := make(chan terminalEnd, 4)
	writerDone := make(chan error, 1)
	workers.Add(5)
	go func() {
		defer workers.Done()
		workerResults <- pumpTerminalLiveness(runtimeContext, connection, terminalLivenessInterval, terminalLivenessTimeout)
	}()
	go func() {
		defer workers.Done()
		writerDone <- pumpTerminalOutput(runtimeContext, connection, queue)
	}()
	go func() {
		defer workers.Done()
		workerResults <- pumpTerminalPTY(attachment, queue)
	}()
	go func() {
		defer workers.Done()
		workerResults <- pumpTerminalInput(runtimeContext, connection, attachment)
	}()
	go func() {
		defer workers.Done()
		workerResults <- gateway.monitorTerminal(runtimeContext, authorization, attachment, queue, presence)
	}()

	select {
	case <-ctx.Done():
		return
	case <-writerDone:
		return
	case ending := <-workerResults:
		if ending == terminalDetached {
			queue.Close()
			waitForTerminalWriter(ctx, writerDone)
			_ = connection.CloseNow() // justify-ignore-error: Detach commits to prompt owned-resource teardown, not a peer handshake.
			return
		}
		code, final := terminalErrorForEnd(ending)
		if final {
			payload, encodeErr := terminal.EncodeError(code)
			if encodeErr != nil {
				panic("closed terminal error failed encoding") // justify-defect: terminalErrorForEnd returns only the closed error universe.
			}
			if queue.TerminateWithText(payload) == nil {
				waitForTerminalWriter(ctx, writerDone)
				_ = connection.CloseNow() // justify-ignore-error: the bounded final-frame attempt already settled this transport.
			}
		}
	}
	return nil
}

func pumpTerminalPTY(attachment *sessions.TerminalAttachment, queue *terminal.OutboundQueue) terminalEnd {
	buffer := make([]byte, terminalPTYReadBufferBytes)
	for {
		count, err := attachment.Read(buffer)
		if count > 0 {
			if enqueueErr := queue.EnqueueBinary(buffer[:count]); enqueueErr != nil {
				return terminalTransportFailure
			}
		}
		if err != nil {
			return terminalReconnect
		}
		if count == 0 {
			return terminalInternalFailure
		}
	}
}

func pumpTerminalInput(ctx context.Context, connection *websocket.Conn, attachment *sessions.TerminalAttachment) terminalEnd {
	for {
		messageType, payload, oversized, err := readTerminalMessage(ctx, connection)
		if err != nil {
			return terminalPeerClosed
		}
		if oversized {
			return terminalRequestTooLarge
		}
		switch messageType {
		case websocket.MessageBinary:
			if err := terminal.ValidateClientBinary(payload); errors.Is(err, terminal.ErrFrameTooLarge) {
				return terminalRequestTooLarge
			} else if err != nil {
				return terminalInvalidRequest
			}
			if err := writeTerminalInput(attachment, payload); err != nil {
				return terminalReconnect
			}
		case websocket.MessageText:
			frame, err := terminal.ParseClientText(payload)
			if errors.Is(err, terminal.ErrFrameTooLarge) {
				return terminalRequestTooLarge
			}
			if err != nil {
				return terminalInvalidRequest
			}
			switch frame := frame.(type) {
			case terminal.ResizeFrame:
				if err := attachment.Resize(frame.Columns, frame.Rows); err != nil {
					return terminalReconnect
				}
			case terminal.DetachFrame:
				return terminalDetached
			default:
				panic("unknown terminal client frame") // justify-defect: ParseClientText returns only Resize or Detach.
			}
		default:
			panic("unknown WebSocket data message type") // justify-defect: coder/websocket exposes only text and binary data messages.
		}
	}
}

func pumpTerminalOutput(ctx context.Context, connection *websocket.Conn, queue *terminal.OutboundQueue) error {
	for {
		frame, err := queue.Next(ctx)
		if err != nil {
			return err
		}
		messageType := websocket.MessageText
		switch frame.Kind {
		case terminal.OutboundText:
		case terminal.OutboundBinary:
			messageType = websocket.MessageBinary
		default:
			panic("unknown terminal outbound frame") // justify-defect: OutboundQueue constructs the closed outbound-kind universe.
		}
		writeContext, cancel := context.WithTimeout(ctx, terminalWriteTimeout)
		err = connection.Write(writeContext, messageType, frame.Payload)
		cancel()
		if err != nil {
			return err
		}
	}
}

func pumpTerminalLiveness(ctx context.Context, connection *websocket.Conn, interval, timeout time.Duration) terminalEnd {
	// justify-polling: a vanished peer emits no transport close, and the WebSocket
	// pong reply is observable only to a Ping caller; one ping per interval with a
	// bounded reply wait releases a dead phone's PTY, tmux client, and shadow
	// within interval+timeout.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return terminalPeerClosed
		case <-ticker.C:
		}
		pingContext, cancelPing := context.WithTimeout(ctx, timeout)
		err := connection.Ping(pingContext)
		cancelPing()
		if err != nil {
			return terminalPeerClosed
		}
	}
}

func (gateway *Gateway) monitorTerminal(
	ctx context.Context,
	authorization string,
	attachment *sessions.TerminalAttachment,
	queue *terminal.OutboundQueue,
	initial sessions.TerminalPresence,
) terminalEnd {
	// justify-polling: tmux 3.4 and the bearer file expose neither client-topology
	// nor rotation notifications; two seconds bounds handoff and revocation lag
	// without coupling terminal bytes to inventory polling.
	ticker := time.NewTicker(terminalPresenceInterval)
	defer ticker.Stop()
	previous := initial
	for {
		select {
		case <-ctx.Done():
			return terminalPeerClosed
		case <-ticker.C:
		}
		valid, err := gateway.bearer.Verify(authorization)
		if err != nil {
			return terminalInternalFailure
		}
		if !valid {
			return terminalReconnect
		}
		observed, err := observeTerminalPresence(ctx, attachment)
		if err != nil {
			return terminalReconnect
		}
		if observed == previous {
			continue
		}
		payload, err := terminal.EncodePresence(observed.AttachedClients, terminalGeometry(observed))
		if err != nil {
			return terminalInternalFailure
		}
		if err := queue.EnqueueText(payload); err != nil {
			return terminalTransportFailure
		}
		previous = observed
	}
}

func observeTerminalPresence(ctx context.Context, attachment *sessions.TerminalAttachment) (sessions.TerminalPresence, error) {
	observationContext, cancel := context.WithTimeout(ctx, terminalObservationTimeout)
	defer cancel()
	return attachment.Presence(observationContext)
}

func writeTerminalInput(destination io.Writer, payload []byte) error {
	for len(payload) > 0 {
		count, err := destination.Write(payload)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(payload) {
			return io.ErrNoProgress
		}
		payload = payload[count:]
	}
	return nil
}

func readTerminalMessage(ctx context.Context, connection *websocket.Conn) (websocket.MessageType, []byte, bool, error) {
	messageType, reader, err := connection.Reader(ctx)
	if err != nil {
		return 0, nil, false, err
	}
	payload, err := io.ReadAll(io.LimitReader(reader, terminal.MaximumFrameBytes+1))
	if err != nil {
		return 0, nil, false, err
	}
	if len(payload) > terminal.MaximumFrameBytes {
		return messageType, nil, true, nil
	}
	return messageType, payload, false, nil
}

func terminalGeometry(presence sessions.TerminalPresence) terminal.Geometry {
	if presence.OwnsGeometry {
		return terminal.GeometryOwner
	}
	return terminal.GeometryConstrained
}

func terminalErrorForEnd(ending terminalEnd) (terminal.ErrorCode, bool) {
	switch ending {
	case terminalReconnect:
		return terminal.ErrorReconnectRequired, true
	case terminalInvalidRequest:
		return terminal.ErrorInvalidRequest, true
	case terminalRequestTooLarge:
		return terminal.ErrorRequestTooLarge, true
	case terminalInternalFailure:
		return terminal.ErrorInternal, true
	case terminalDetached, terminalPeerClosed, terminalTransportFailure:
		return "", false
	default:
		panic("unknown terminal ending") // justify-defect: terminal workers return only the closed terminal-end universe.
	}
}

func terminalCodeForSessionError(err error) terminal.ErrorCode {
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		return terminal.ErrorInternal
	}
	switch sessionError.Code {
	case sessions.ErrorSessionNotFound, sessions.ErrorSessionIdentityMismatch:
		return terminal.ErrorReconnectRequired
	case sessions.ErrorWorkingDirectoryInvalid, sessions.ErrorWorkingDirectoryUnavailable,
		sessions.ErrorProfileUnknown, sessions.ErrorSessionNameInvalid, sessions.ErrorObjectiveInvalid,
		sessions.ErrorSessionNameConflict:
		return terminal.ErrorInternal
	default:
		return terminal.ErrorInternal
	}
}

func terminalSessionID(path string) (string, bool) {
	const prefix = "/v1/sessions/"
	const suffix = "/terminal"
	if len(path) <= len(prefix)+len(suffix) || !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	id := path[len(prefix) : len(path)-len(suffix)]
	return id, id != "" && !strings.ContainsRune(id, '/')
}

func writeTerminalErrorAndClose(parent context.Context, connection *websocket.Conn, code terminal.ErrorCode) {
	payload, err := terminal.EncodeError(code)
	if err != nil {
		panic("closed terminal error failed encoding") // justify-defect: every caller supplies a closed terminal error code.
	}
	ctx, cancel := context.WithTimeout(parent, terminalWriteTimeout)
	_ = connection.Write(ctx, websocket.MessageText, payload) // justify-ignore-error: a failed upgraded stream has no alternate response path.
	cancel()
	_ = connection.CloseNow() // justify-ignore-error: an error-only upgrade owns no terminal resources and must not wait for a peer handshake.
}

func waitForTerminalWriter(ctx context.Context, writerDone <-chan error) {
	timer := time.NewTimer(terminalFinalFrameTimeout)
	defer timer.Stop()
	select {
	case <-writerDone:
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (gateway *Gateway) registerLiveTerminal(sessionID string, cancel context.CancelFunc) (uint64, bool) {
	gateway.liveMutex.Lock()
	defer gateway.liveMutex.Unlock()
	if gateway.closing {
		return 0, false
	}
	gateway.nextLiveTerminal++
	if gateway.nextLiveTerminal == 0 {
		panic("terminal registration sequence exhausted") // justify-defect: uint64 exhaustion is unreachable for one-user process lifetime.
	}
	key := gateway.nextLiveTerminal
	gateway.liveTerminals[key] = &liveTerminal{sessionID: sessionID, cancel: cancel, done: make(chan error, 1)}
	return key, true
}

func (gateway *Gateway) unregisterLiveTerminal(key uint64, cleanupErr error) {
	gateway.liveMutex.Lock()
	terminalSession, found := gateway.liveTerminals[key]
	if found {
		delete(gateway.liveTerminals, key)
		terminalSession.done <- cleanupErr
		close(terminalSession.done)
	}
	gateway.liveMutex.Unlock()
}

func (gateway *Gateway) closeLiveTerminals(ctx context.Context, sessionID string) error {
	gateway.liveMutex.Lock()
	selected := make([]*liveTerminal, 0)
	for _, terminalSession := range gateway.liveTerminals {
		if terminalSession.sessionID == sessionID {
			selected = append(selected, terminalSession)
		}
	}
	gateway.liveMutex.Unlock()
	for _, terminalSession := range selected {
		terminalSession.cancel()
	}
	return waitForLiveTerminals(ctx, selected)
}

func (gateway *Gateway) CloseLiveTerminals(ctx context.Context) error {
	gateway.terminalLifecycle.Lock()
	defer gateway.terminalLifecycle.Unlock()
	gateway.liveMutex.Lock()
	gateway.closing = true
	selected := make([]*liveTerminal, 0, len(gateway.liveTerminals))
	for _, terminalSession := range gateway.liveTerminals {
		selected = append(selected, terminalSession)
	}
	gateway.liveMutex.Unlock()
	for _, terminalSession := range selected {
		terminalSession.cancel()
	}
	return waitForLiveTerminals(ctx, selected)
}

func waitForLiveTerminals(ctx context.Context, terminals []*liveTerminal) error {
	waitContext, cancel := context.WithTimeout(ctx, terminalShutdownTimeout)
	defer cancel()
	var cleanupErrors []error
	for _, terminalSession := range terminals {
		select {
		case cleanupErr := <-terminalSession.done:
			if cleanupErr != nil {
				cleanupErrors = append(cleanupErrors, errors.New("terminal resource cleanup failed"))
			}
		case <-waitContext.Done():
			return errors.New("terminal cleanup did not finish")
		}
	}
	return errors.Join(cleanupErrors...)
}
