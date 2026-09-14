package terminalclient

import (
	"context"
	"errors"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/creack/pty"
	"golang.org/x/term"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/coder/websocket"
)

func TestRemoteExitReleasesInputBeforeReturning(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.CloseNow()
		_, body, err := connection.Read(r.Context())
		if err != nil {
			t.Error(err)
			return
		}
		frame, err := terminal.ParseClientText(body)
		if err != nil || frame != (terminal.ResizeFrame{Columns: 300, Rows: 40}) {
			t.Error("first frame was not actual geometry")
		}
		hello, _ := terminal.EncodeHello(1)
		connection.Write(r.Context(), websocket.MessageText, hello)
		<-release
	}))
	defer server.Close()
	conn, _, err := websocket.Dial(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() { done <- stream(context.Background(), conn, input, io.Discard, 300, 40, nil, nil) }()
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("terminal reader remained blocked after remote exit")
	}
	writer.Write([]byte("next"))
	observed := make(chan []byte, 1)
	go func() { b := make([]byte, 4); n, _ := input.Read(b); observed <- b[:n] }()
	select {
	case got := <-observed:
		if string(got) != "next" {
			t.Fatal("resumed reader lost input")
		}
	case <-time.After(time.Second):
		t.Fatal("old reader consumed resumed input")
	}
}

func TestDetachPrefixAndLiteralKeysAcrossReads(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		c.Read(r.Context())
		hello, _ := terminal.EncodeHello(1)
		c.Write(r.Context(), websocket.MessageText, hello)
		var all []byte
		for {
			kind, body, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if kind == websocket.MessageText {
				frame, err := terminal.ParseClientText(body)
				if err != nil {
					t.Error(err)
				}
				if _, ok := frame.(terminal.DetachFrame); ok {
					received <- all
					return
				}
			} else {
				all = append(all, body...)
			}
		}
	}))
	defer server.Close()
	c, _, err := websocket.Dial(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() { done <- stream(context.Background(), c, input, io.Discard, 80, 24, nil, nil) }()
	for _, part := range []string{"\x1b\x03hello\n\x1d", "\x1d", "\x1dx", "\x1d", "d"} {
		writer.Write([]byte(part))
	}
	select {
	case err := <-done:
		if err != nil {
			t.Error("explicit detach failed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("detach did not settle")
	}
	select {
	case got := <-received:
		if string(got) != "\x1b\x03hello\n\x1d\x1dx" {
			t.Fatal("terminal input was altered")
		}
	case <-time.After(time.Second):
		t.Fatal("detach frame missing")
	}
}

func TestRunRestoresRealPTYAndUsesExistingAuthenticatedRoute(t *testing.T) {
	const machine = "mh-11111111111111111111111111111111"
	const bearer = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ready := make(chan struct{})
	handshakeStarted := make(chan struct{})
	releaseHandshake := make(chan struct{})
	detached := make(chan bool, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions/$3/terminal" || r.Header.Get("Skidbladnir-Session-Identity") != "lifetime" || r.Header.Get("Skidbladnir-Machine") != machine || r.Header.Get("Authorization") != "Bearer "+bearer {
			t.Error("terminal route lost exact target/authentication")
		}
		close(handshakeStarted)
		<-releaseHandshake
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		kind, body, err := c.Read(r.Context())
		if err != nil || kind != websocket.MessageText {
			t.Error("missing resize")
			return
		}
		frame, err := terminal.ParseClientText(body)
		if err != nil || frame != (terminal.ResizeFrame{Columns: 300, Rows: 40}) {
			t.Error("PTY geometry changed")
		}
		hello, _ := terminal.EncodeHello(1)
		c.Write(r.Context(), websocket.MessageText, hello)
		close(ready)
		kind, body, err = c.Read(r.Context())
		if err != nil {
			return
		}
		frame, err = terminal.ParseClientText(body)
		_, ok := frame.(terminal.DetachFrame)
		detached <- err == nil && kind == websocket.MessageText && ok
	}))
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	old := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old; transport.CloseIdleConnections() })
	config := filepath.Join(t.TempDir(), "client.json")
	os.WriteFile(config, []byte(`{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"`+machine+`","bearer":"`+bearer+`"}]}`), 0600)
	client, err := fleetclient.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if pty.Setsize(master, &pty.Winsize{Cols: 80, Rows: 24}) != nil {
		t.Fatal("cannot size test PTY")
	}
	original, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	reference := fleetclient.Reference{Machine: machine, TmuxID: "$3", IdentityToken: "lifetime"}.Encode()
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), client, fleetclient.Request{Ref: reference}, slave, slave) }()
	select {
	case <-handshakeStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake did not begin")
	}
	if pty.Setsize(master, &pty.Winsize{Cols: 300, Rows: 40}) != nil {
		t.Fatal("cannot resize during handshake")
	}
	close(releaseHandshake)
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("terminal not ready")
	}
	master.Write([]byte{0x1d, 'd'})
	select {
	case err := <-done:
		if err != nil {
			t.Error("PTY detach failed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("PTY detach hung")
	}
	restored, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(original, restored) {
		t.Fatal("raw mode was not restored")
	}
	select {
	case ok := <-detached:
		if !ok {
			t.Error("detach changed")
		}
	case <-time.After(time.Second):
		t.Fatal("detach missing")
	}
}

func TestUnsupportedResizeDisconnectsAndReleasesReader(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		c.Read(r.Context())
		hello, _ := terminal.EncodeHello(1)
		c.Write(r.Context(), websocket.MessageText, hello)
		close(ready)
		c.Read(r.Context())
	}))
	defer server.Close()
	c, _, err := websocket.Dial(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	resized := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- stream(context.Background(), c, input, io.Discard, 80, 24, resized, func() (int, int, error) { return 10, 3, nil })
	}()
	<-ready
	resized <- syscall.SIGWINCH
	select {
	case err := <-done:
		if !errors.Is(err, errDimensions) {
			t.Fatal("unsupported resize was not explicit")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("resize left reader blocked")
	}
}
