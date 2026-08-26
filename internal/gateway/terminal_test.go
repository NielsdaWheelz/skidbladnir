package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/coder/websocket"
)

func TestTerminalSessionPathIsExact(t *testing.T) {
	id, valid := terminalSessionID("/v1/sessions/$17/terminal")
	if !valid || id != "$17" {
		t.Fatalf("valid terminal path decoded as id=%q valid=%t", id, valid)
	}
	for _, path := range []string{
		"/v1/sessions/terminal",
		"/v1/sessions//terminal",
		"/v1/sessions/$17/terminal/",
		"/v1/sessions/$17/child/terminal",
		"/v1/sessions/$17/not-terminal",
	} {
		if id, valid := terminalSessionID(path); valid {
			t.Fatalf("invalid terminal path %q decoded as %q", path, id)
		}
	}
}

func TestTerminalInputCompletesShortWritesWithoutLosingBytes(t *testing.T) {
	destination := &oneByteWriter{}
	payload := []byte("prompt\r")
	if err := writeTerminalInput(destination, payload); err != nil {
		t.Fatalf("write terminal input: %v", err)
	}
	if !bytes.Equal(destination.contents, payload) {
		t.Fatalf("terminal input = %q, want %q", destination.contents, payload)
	}
}

func TestTerminalEndHasClosedProtocolProjection(t *testing.T) {
	tests := []struct {
		ending terminalEnd
		code   terminal.ErrorCode
		final  bool
	}{
		{terminalDetached, "", false},
		{terminalPeerClosed, "", false},
		{terminalReconnect, terminal.ErrorReconnectRequired, true},
		{terminalInvalidRequest, terminal.ErrorInvalidRequest, true},
		{terminalRequestTooLarge, terminal.ErrorRequestTooLarge, true},
		{terminalInternalFailure, terminal.ErrorInternal, true},
		{terminalTransportFailure, "", false},
	}
	for _, test := range tests {
		code, final := terminalErrorForEnd(test.ending)
		if code != test.code || final != test.final {
			t.Fatalf("terminal end %d projected as (%q,%t), want (%q,%t)", test.ending, code, final, test.code, test.final)
		}
	}
}

func TestTerminalLivenessEndsOnSilentPeerAndSurvivesResponsivePeer(t *testing.T) {
	endings := make(chan terminalEnd, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			t.Errorf("accept liveness test connection: %v", err)
			return
		}
		defer connection.CloseNow()
		go func() {
			for {
				if _, _, err := connection.Read(context.Background()); err != nil {
					return
				}
			}
		}()
		endings <- pumpTerminalLiveness(request.Context(), connection, 25*time.Millisecond, 100*time.Millisecond)
	}))
	defer server.Close()

	dial := func() *websocket.Conn {
		dialContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		connection, _, err := websocket.Dial(dialContext, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatalf("dial liveness test server: %v", err)
		}
		return connection
	}

	responsive := dial()
	readerContext, stopReader := context.WithCancel(context.Background())
	go func() {
		for {
			if _, _, err := responsive.Read(readerContext); err != nil {
				return
			}
		}
	}()
	select {
	case ending := <-endings:
		t.Fatalf("responsive peer ended liveness with %d", ending)
	case <-time.After(300 * time.Millisecond):
	}
	stopReader()
	responsive.CloseNow()
	select {
	case <-endings:
	case <-time.After(5 * time.Second):
		t.Fatal("closed peer never ended the liveness pump")
	}

	silent := dial()
	defer silent.CloseNow()
	select {
	case ending := <-endings:
		if ending != terminalPeerClosed {
			t.Fatalf("silent peer ended liveness with %d, want terminalPeerClosed", ending)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("silent peer was never detected as dead")
	}
}

func TestTerminalSessionFailuresRequireReconnectOnlyAfterIdentityLoss(t *testing.T) {
	for _, code := range []sessions.ErrorCode{sessions.ErrorSessionNotFound, sessions.ErrorSessionIdentityMismatch} {
		if got := terminalCodeForSessionError(&sessions.Error{Code: code, Message: "closed test error"}); got != terminal.ErrorReconnectRequired {
			t.Fatalf("session error %q projected as %q", code, got)
		}
	}
	if got := terminalCodeForSessionError(errors.New("unmodeled")); got != terminal.ErrorInternal {
		t.Fatalf("unmodeled session error projected as %q", got)
	}
}

func TestClosingLiveTerminalsSelectsOneSessionAndRejectsNewWorkAfterShutdown(t *testing.T) {
	gateway := &Gateway{liveTerminals: make(map[uint64]*liveTerminal)}
	firstContext, cancelFirstContext := context.WithCancel(context.Background())
	defer cancelFirstContext()
	secondContext, cancelSecondContext := context.WithCancel(context.Background())
	defer cancelSecondContext()
	firstKey, firstRegistered := gateway.registerLiveTerminal("$1", cancelFirstContext)
	secondKey, secondRegistered := gateway.registerLiveTerminal("$2", cancelSecondContext)
	if !firstRegistered || !secondRegistered {
		t.Fatal("gateway rejected terminal registration before shutdown")
	}
	go func() {
		<-firstContext.Done()
		gateway.unregisterLiveTerminal(firstKey, nil)
	}()

	deadline, cancelDeadline := context.WithTimeout(context.Background(), time.Second)
	defer cancelDeadline()
	if err := gateway.closeLiveTerminals(deadline, "$1"); err != nil {
		t.Fatalf("close selected live terminal: %v", err)
	}
	if secondContext.Err() != nil {
		t.Fatal("closing one session canceled an unrelated terminal")
	}

	go func() {
		<-secondContext.Done()
		gateway.unregisterLiveTerminal(secondKey, nil)
	}()
	if err := gateway.CloseLiveTerminals(deadline); err != nil {
		t.Fatalf("close remaining live terminals: %v", err)
	}
	_, registered := gateway.registerLiveTerminal("$3", func() {})
	if registered {
		t.Fatal("gateway accepted a new terminal after shutdown began")
	}
}

func TestTerminalCleanupFailurePreventsKillFromProceeding(t *testing.T) {
	gateway := &Gateway{liveTerminals: make(map[uint64]*liveTerminal)}
	terminalContext, cancelTerminal := context.WithCancel(context.Background())
	defer cancelTerminal()
	key, registered := gateway.registerLiveTerminal("$1", cancelTerminal)
	if !registered {
		t.Fatal("gateway rejected terminal registration")
	}
	go func() {
		<-terminalContext.Done()
		gateway.unregisterLiveTerminal(key, errors.New("closed cleanup failure"))
	}()

	deadline, cancelDeadline := context.WithTimeout(context.Background(), time.Second)
	defer cancelDeadline()
	if err := gateway.closeLiveTerminals(deadline, "$1"); err == nil {
		t.Fatal("terminal cleanup failure was reported as successful completion")
	}
}

func TestTerminalWriterWaitObservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error)
	returned := make(chan struct{})
	go func() {
		waitForTerminalWriter(ctx, done)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("terminal writer wait ignored cancellation")
	}
}

type oneByteWriter struct {
	contents []byte
}

func (writer *oneByteWriter) Write(contents []byte) (int, error) {
	if len(contents) == 0 {
		return 0, nil
	}
	writer.contents = append(writer.contents, contents[0])
	return 1, nil
}
