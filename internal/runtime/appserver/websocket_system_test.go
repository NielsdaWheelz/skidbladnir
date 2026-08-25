//go:build system

package appserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestUnixWebSocketConnectionExchangesTextFrames(t *testing.T) {
	t.Parallel()

	socket := filepath.Join(t.TempDir(), "app-server.sock")
	handlerResult := make(chan error, 1)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen on test Unix socket: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, acceptErr := websocket.Accept(writer, request, nil)
		if acceptErr != nil {
			handlerResult <- fmt.Errorf("accept WebSocket: %w", acceptErr)
			return
		}
		defer connection.CloseNow()

		messageType, message, readErr := connection.Read(request.Context())
		if readErr != nil {
			handlerResult <- fmt.Errorf("read WebSocket frame: %w", readErr)
			return
		}
		if messageType != websocket.MessageText || string(message) != `{"method":"initialize"}` {
			handlerResult <- fmt.Errorf("request frame type=%d body=%q", messageType, message)
			return
		}
		if writeErr := connection.Write(request.Context(), websocket.MessageText, []byte(`{"id":1,"result":{}}`)); writeErr != nil {
			handlerResult <- fmt.Errorf("write WebSocket frame: %w", writeErr)
			return
		}
		handlerResult <- nil
	})}
	t.Cleanup(func() {
		_ = server.Close() // justify-ignore-error: cleanup targets only this test-owned listener.
	})
	go func() {
		_ = server.Serve(listener) // justify-ignore-error: Server.Close ends this test-owned listener.
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := DialUnix(ctx, socket)
	if err != nil {
		t.Fatalf("dial test Unix WebSocket: %v", err)
	}
	defer connection.Close()
	if err := connection.Send(ctx, []byte(`{"method":"initialize"}`)); err != nil {
		t.Fatalf("send text frame: %v", err)
	}
	message, err := connection.Receive(ctx)
	if err != nil {
		t.Fatalf("receive text frame: %v", err)
	}
	if string(message) != `{"id":1,"result":{}}` {
		t.Fatalf("response = %q", message)
	}
	if err := <-handlerResult; err != nil {
		t.Fatalf("server handler: %v", err)
	}
}
