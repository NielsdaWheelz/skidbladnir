package appserver

import (
	"context"
	"net"
	"net/http"
	"path/filepath"

	"github.com/coder/websocket"
)

// Connection is one framed App Server transport connection.
type Connection interface {
	Send(context.Context, []byte) error
	Receive(context.Context) ([]byte, error)
}

// UnixConnection is a WebSocket connection over an App Server Unix socket.
type UnixConnection struct {
	connection *websocket.Conn
}

// DialUnix performs the WebSocket upgrade required by an App Server Unix
// listener. It never uses codex app-server proxy, which only copies raw bytes.
func DialUnix(ctx context.Context, socket string) (*UnixConnection, error) {
	if !filepath.IsAbs(socket) {
		return nil, errTransport
	}
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DisableCompression: true,
		DisableKeepAlives:  true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
	}
	connection, _, err := websocket.Dial(ctx, "ws://localhost/", &websocket.DialOptions{
		HTTPClient:      &http.Client{Transport: transport},
		CompressionMode: websocket.CompressionDisabled,
	})
	transport.CloseIdleConnections()
	if err != nil {
		return nil, errTransport
	}
	connection.SetReadLimit(maximumFrameBytes)
	return &UnixConnection{connection: connection}, nil
}

func (connection *UnixConnection) Send(ctx context.Context, message []byte) error {
	if len(message) == 0 || len(message) > maximumFrameBytes {
		return errTransport
	}
	if err := connection.connection.Write(ctx, websocket.MessageText, message); err != nil {
		return errTransport
	}
	return nil
}

func (connection *UnixConnection) Receive(ctx context.Context) ([]byte, error) {
	messageType, message, err := connection.connection.Read(ctx)
	if err != nil || messageType != websocket.MessageText || len(message) == 0 || len(message) > maximumFrameBytes {
		return nil, errTransport
	}
	return message, nil
}

func (connection *UnixConnection) Close() {
	connection.connection.CloseNow()
}
