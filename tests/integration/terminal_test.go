//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/coder/websocket"
	"github.com/creack/pty"
)

const terminalIntegrationTimeout = 10 * time.Second

var terminalPhoneShadowName = regexp.MustCompile(`^skid-phone-[0-9a-f]{32}$`)

func TestTerminalWebSocketSharesOneSessionWithoutStealingTheLaptop(t *testing.T) {
	fixture := newSessionFixture(t)
	logA := filepath.Join(fixture.root, "pane-a.log")
	logB := filepath.Join(fixture.root, "pane-b.log")
	fixture.tmux(t, "new-session", "-d", "-s", "shared-terminal", "-x", "120", "-y", "40", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logA, "A")
	fixture.tmux(t, "split-window", "-h", "-t", "shared-terminal:0", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logB, "B")
	fixture.tmux(t, "select-pane", "-t", "shared-terminal:0.0")
	fixture.tmux(t, "set-option", "-p", "-t", "shared-terminal:0.0", "--", "@skid_attention", "1")

	listed, err := fixture.manager.List(context.Background())
	if err != nil {
		t.Fatalf("list terminal source: %v", err)
	}
	source := requireSessionNamed(t, listed, "shared-terminal")
	laptop := startTerminalLaptop(t, fixture, source.ID, 120, 40)
	waitForTerminalCondition(t, "laptop attachment", func() bool {
		listed, listErr := fixture.manager.List(context.Background())
		if listErr != nil {
			return false
		}
		return requireSessionID(t, listed, source.ID).AttachedClients == 1
	})
	laptopShapeBefore := terminalLaptopShape(t, fixture, "shared-terminal")
	windowShapeBefore := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{window_width}x#{window_height}")

	bearerPath := filepath.Join(fixture.root, "terminal-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint terminal bearer: %v", err)
	}
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Logger:   logging.New(io.Discard),
	}))
	t.Cleanup(server.Close)

	terminalURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/sessions/" + source.ID + "/terminal"
	staleToken := source.IdentityToken[:len(source.IdentityToken)-1] + differentASCII(source.IdentityToken[len(source.IdentityToken)-1])
	sessionsBefore := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}")
	staleHeaders := http.Header{
		"Authorization":                []string{"Bearer " + bearer},
		"Skidbladnir-Session-Identity": []string{staleToken},
	}
	_, response, err := websocket.Dial(context.Background(), terminalURL, &websocket.DialOptions{HTTPHeader: staleHeaders})
	if err == nil || response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("stale terminal identity result: response=%v error=%v", response, err)
	}
	if response.Body != nil {
		_ = response.Body.Close()
	}
	if sessionsAfter := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}"); sessionsAfter != sessionsBefore {
		t.Fatalf("stale terminal identity mutated sessions:\nbefore=%q\nafter=%q", sessionsBefore, sessionsAfter)
	}

	headers := http.Header{
		"Authorization":                []string{"Bearer " + bearer},
		"Skidbladnir-Session-Identity": []string{source.IdentityToken},
	}
	connection, response, err := websocket.Dial(context.Background(), terminalURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("open terminal WebSocket: status=%d error=%v", status, err)
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { connection.CloseNow() })

	readContext, cancelRead := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	messageType, helloPayload, err := connection.Read(readContext)
	cancelRead()
	if err != nil || messageType != websocket.MessageText {
		t.Fatalf("read terminal Hello: type=%v payload=%q error=%v", messageType, helloPayload, err)
	}
	var hello struct {
		Kind            string `json:"kind"`
		AttachedClients int    `json:"attachedClients"`
		Geometry        string `json:"geometry"`
	}
	if err := json.Unmarshal(helloPayload, &hello); err != nil {
		t.Fatalf("decode terminal Hello: %v: %s", err, helloPayload)
	}
	if hello.Kind != "Hello" || hello.AttachedClients != 2 || hello.Geometry != "Constrained" {
		t.Fatalf("terminal Hello = %+v, want two-client Constrained", hello)
	}
	readTerminalBinaryUntil(t, connection, []string{"FRAME=A BUFFER=seed-A"}, nil)
	if attention := fixture.tmux(t, "show-options", "-pqv", "-t", "shared-terminal:0.0", "@skid_attention"); attention != "" {
		t.Fatalf("opening terminal did not clear active-pane attention: %q", attention)
	}

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Resize","columns":100,"rows":30}`))
	waitForTerminalCondition(t, "phone resize", func() bool {
		for _, line := range strings.Split(fixture.tmux(t, "list-clients", "-F", "#{client_flags}|#{client_width}x#{client_height}"), "\n") {
			fields := strings.Split(line, "|")
			if len(fields) == 2 && fields[1] == "100x30" &&
				strings.Contains(fields[0], "active-pane") && strings.Contains(fields[0], "ignore-size") {
				return true
			}
		}
		return false
	})
	if got := terminalLaptopShape(t, fixture, "shared-terminal"); got != laptopShapeBefore {
		t.Fatalf("phone resize changed laptop client shape: before=%q after=%q", laptopShapeBefore, got)
	}
	if got := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{window_width}x#{window_height}"); got != windowShapeBefore {
		t.Fatalf("phone resize changed laptop-owned window geometry: before=%q after=%q", windowShapeBefore, got)
	}

	writeTerminalFrame(t, connection, websocket.MessageBinary, []byte("\x02oPHONE-B\r"))
	waitForTerminalFile(t, logB, "PHONE-B")
	readTerminalBinaryUntil(t, connection, []string{"TOKEN=PHONE-B"}, nil)
	if contents, readErr := os.ReadFile(logA); readErr == nil && bytes.Contains(contents, []byte("PHONE-B")) {
		t.Fatalf("phone input reached laptop-selected pane: %q", contents)
	}
	if active := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{pane_index}"); active != "0" {
		t.Fatalf("phone active-pane key sequence stole laptop selection: %q", active)
	}

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	closeContext, cancelClose := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	_, _, closeErr := connection.Read(closeContext)
	cancelClose()
	if closeErr == nil {
		t.Fatal("Detach left the terminal WebSocket open")
	}
	waitForTerminalCondition(t, "phone shadow teardown", func() bool {
		listed, listErr := fixture.manager.List(context.Background())
		if listErr != nil {
			return false
		}
		for _, session := range listed {
			if session.ID == source.ID {
				return session.AttachedClients == 1
			}
		}
		return false
	})
	if _, err := laptop.Write([]byte("LAPTOP-A\r")); err != nil {
		t.Fatalf("write through surviving laptop client: %v", err)
	}
	waitForTerminalFile(t, logA, "LAPTOP-A")
}

func TestTerminalBearerRotationRevokesLiveStreamAndReconnectsWithoutReplay(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "rotation.log")
	fixture.tmux(t, "new-session", "-d", "-s", "rotation-terminal", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalRotationScript(), "fixture", logPath)
	source := terminalSource(t, fixture, "rotation-terminal")
	gatewayFixture := newTerminalGateway(t, fixture, 0)

	oldConnection := dialTerminal(t, nil, gatewayFixture.url(source.ID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, oldConnection, "Hello", 1, "Owner")
	writeTerminalFrame(t, oldConnection, websocket.MessageBinary, []byte("ROTATE\r"))
	readTerminalBinaryUntil(t, oldConnection,
		[]string{"REPLAY-CANDIDATE-V1", "SCREEN=AFTER-ROTATION"}, nil)
	waitForTerminalCondition(t, "rotation screen cleared before bearer remint", func() bool {
		visible := fixture.tmux(t, "capture-pane", "-p", "-J", "-t", source.ID)
		return strings.Contains(visible, "SCREEN=AFTER-ROTATION") && !strings.Contains(visible, "REPLAY-CANDIDATE-V1")
	})

	freshBearer, err := auth.Mint(auth.MintOptions{Path: gatewayFixture.bearerPath})
	if err != nil {
		t.Fatalf("remint terminal bearer: %v", err)
	}
	requireTerminalError(t, oldConnection, "ReconnectRequired", "Reconnect required.")
	requireTerminalClosed(t, oldConnection)
	waitForTerminalAttachmentCount(t, fixture, source.ID, 0, terminalIntegrationTimeout)
	waitForTerminalCondition(t, "revoked terminal shadow teardown", func() bool {
		return len(terminalPhoneShadows(t, fixture)) == 0
	})

	freshConnection := dialTerminal(t, nil, gatewayFixture.url(source.ID), freshBearer, source.IdentityToken)
	requireTerminalPresence(t, freshConnection, "Hello", 1, "Owner")
	writeTerminalFrame(t, freshConnection, websocket.MessageBinary, []byte("POST-RECONNECT\r"))
	readTerminalBinaryUntil(t, freshConnection,
		[]string{"SCREEN=AFTER-ROTATION", "TOKEN=POST-RECONNECT"}, []string{"REPLAY-CANDIDATE-V1"})
	writeTerminalFrame(t, freshConnection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	requireTerminalClosed(t, freshConnection)
	waitForTerminalAttachmentCount(t, fixture, source.ID, 0, terminalIntegrationTimeout)
}

func TestTerminalPresenceAndLastLinkDetachPreserveThePane(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "last-link.log")
	fixture.tmux(t, "new-session", "-d", "-s", "last-link-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "LAST-LINK")
	source := terminalSource(t, fixture, "last-link-source")
	startTerminalLaptop(t, fixture, source.ID, 80, 24)
	waitForTerminalAttachmentCount(t, fixture, source.ID, 1, terminalIntegrationTimeout)
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.ID)
	gatewayFixture := newTerminalGateway(t, fixture, 0)

	connection := dialTerminal(t, nil, gatewayFixture.url(source.ID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 2, "Constrained")
	shadow := requireTerminalPhoneShadow(t, fixture)
	fixture.tmux(t, "detach-client", "-t", requireTerminalLaptopClient(t, fixture, source.ID, source.Name))
	requireTerminalPresence(t, connection, "Presence", 1, "Owner")

	removeExactTerminalSourceLink(t, fixture, source, panePID)
	waitForTerminalCondition(t, "phone shadow became the last grouped link", func() bool {
		return !terminalSessionExists(t, fixture, source.ID, source.Name) && terminalSessionExists(t, fixture, shadow.id, shadow.name)
	})
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)
	writeTerminalFrame(t, connection, websocket.MessageBinary, []byte("PHONE-SURVIVES\r"))
	waitForTerminalFile(t, logPath, "PHONE-SURVIVES")

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	requireTerminalClosed(t, connection)
	waitForTerminalCondition(t, "last phone link promoted to an ordinary session", func() bool {
		return fixture.tmux(t, "show-options", "-qv", "-t", shadow.id, "@skid_internal") == ""
	})
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)
	if !terminalSessionExists(t, fixture, shadow.id, shadow.name) {
		t.Fatalf("promoted last phone link disappeared: id=%s name=%s", shadow.id, shadow.name)
	}
}

func TestTerminalDetachLeavesAnAttachedPhoneShadowUntouched(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "attached-shadow.log")
	fixture.tmux(t, "new-session", "-d", "-s", "attached-shadow-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "ATTACHED-SHADOW")
	source := terminalSource(t, fixture, "attached-shadow-source")
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.ID)
	gatewayFixture := newTerminalGateway(t, fixture, 0)

	connection := dialTerminal(t, nil, gatewayFixture.url(source.ID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")
	shadow := requireTerminalPhoneShadow(t, fixture)
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)

	unrelated := startTerminalLaptop(t, fixture, shadow.id, 100, 30)
	waitForTerminalAttachmentCount(t, fixture, source.ID, 2, terminalIntegrationTimeout)
	unrelatedClient := requireTerminalLaptopClient(t, fixture, shadow.id, shadow.name)
	requireTerminalPresence(t, connection, "Presence", 2, "Constrained")

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	requireTerminalClosed(t, connection)
	waitForTerminalCondition(t, "attached phone shadow retained with unrelated client", func() bool {
		return terminalHasOnlyExactClient(t, fixture, unrelatedClient, shadow.id, shadow.name)
	})

	if !terminalSessionExists(t, fixture, source.ID, source.Name) {
		t.Fatalf("terminal detach removed source session: id=%s name=%s", source.ID, source.Name)
	}
	if !terminalSessionExists(t, fixture, shadow.id, shadow.name) {
		t.Fatalf("terminal detach removed attached phone shadow: id=%s name=%s", shadow.id, shadow.name)
	}
	if marker := fixture.tmux(t, "show-options", "-qv", "-t", shadow.id, "@skid_internal"); marker != "phone-shadow" {
		t.Fatalf("terminal detach changed attached phone-shadow marker: %q", marker)
	}
	requireTerminalPaneIdentity(t, fixture, source.ID, panePID, paneStartTime)
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)
	if _, err := unrelated.Write([]byte("UNRELATED-SURVIVES\r")); err != nil {
		t.Fatalf("write through unrelated shadow client after phone detach: %v", err)
	}
	waitForTerminalFile(t, logPath, "UNRELATED-SURVIVES")
}

func TestTerminalSlowReaderIsTornDownWithoutKillingTheSource(t *testing.T) {
	fixture := newSessionFixture(t)
	fixture.tmux(t, "new-session", "-d", "-s", "slow-terminal", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalFloodScript())
	source := terminalSource(t, fixture, "slow-terminal")
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.ID)
	gatewayFixture := newTerminalGateway(t, fixture, 1024)
	slowClient := terminalSlowHTTPClient(t)

	connection := dialTerminal(t, slowClient, gatewayFixture.url(source.ID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")
	writeTerminalFrame(t, connection, websocket.MessageBinary, []byte("FLOOD\r"))
	waitForTerminalAttachmentCount(t, fixture, source.ID, 0, 20*time.Second)
	waitForTerminalConditionWithin(t, "slow-reader shadow teardown", 20*time.Second, func() bool {
		return len(terminalPhoneShadows(t, fixture)) == 0
	})
	requireTerminalPaneIdentity(t, fixture, source.ID, panePID, paneStartTime)
}

func terminalPaneScript() string {
	return `log=$1
frame=$2
	printf 'FRAME=%s BUFFER=seed-%s\n' "$frame" "$frame"
while IFS= read -r token; do
  printf '%s\n' "$token" >> "$log"
  printf 'TOKEN=%s\n' "$token"
done`
}

func terminalRotationScript() string {
	return `log=$1
printf '\033[3J\033[2J\033[HSCREEN=INITIAL\n'
while IFS= read -r token; do
  printf '%s\n' "$token" >> "$log"
  if [ "$token" = ROTATE ]; then
    printf 'REPLAY-CANDIDATE-V1\n'
    /usr/bin/sleep 0.05
    printf '\033[3J\033[2J\033[HSCREEN=AFTER-ROTATION\n'
  else
    printf 'TOKEN=%s\n' "$token"
  fi
done`
}

func terminalFloodScript() string {
	return `printf 'FLOOD-READY\n'
while IFS= read -r token; do
  if [ "$token" = FLOOD ]; then
    /usr/bin/yes 'SKIDBLADNIR-SLOW-READER-0123456789abcdefghijklmnopqrstuvwxyz' | /usr/bin/head -c 33554432
    printf '\nFLOOD-DONE\n'
  fi
done`
}

type terminalGatewayFixture struct {
	server     *httptest.Server
	bearerPath string
	bearer     string
}

func newTerminalGateway(t *testing.T, fixture sessionFixture, writeBuffer int) terminalGatewayFixture {
	t.Helper()
	bearerPath := filepath.Join(fixture.root, "terminal-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint terminal bearer: %v", err)
	}
	server := httptest.NewUnstartedServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Logger:   logging.New(io.Discard),
	}))
	if writeBuffer > 0 {
		server.Config.ConnContext = func(ctx context.Context, connection net.Conn) context.Context {
			if tcp, ok := connection.(*net.TCPConn); ok {
				if err := tcp.SetWriteBuffer(writeBuffer); err != nil {
					t.Errorf("set terminal server write buffer: %v", err)
				}
			}
			return ctx
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	return terminalGatewayFixture{server: server, bearerPath: bearerPath, bearer: bearer}
}

func (fixture terminalGatewayFixture) url(sessionID string) string {
	return "ws" + strings.TrimPrefix(fixture.server.URL, "http") + "/v1/sessions/" + sessionID + "/terminal"
}

func dialTerminal(t *testing.T, client *http.Client, url, bearer, identityToken string) *websocket.Conn {
	t.Helper()
	options := &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization":                []string{"Bearer " + bearer},
		"Skidbladnir-Session-Identity": []string{identityToken},
	}}
	if client != nil {
		options.HTTPClient = client
	}
	connection, response, err := websocket.Dial(context.Background(), url, options)
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("open terminal WebSocket: status=%d error=%v", status, err)
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { connection.CloseNow() })
	return connection
}

func terminalSlowHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	dialer := &net.Dialer{}
	transport := &http.Transport{DisableKeepAlives: true}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if tcp, ok := connection.(*net.TCPConn); ok {
			if err := tcp.SetReadBuffer(1024); err != nil {
				_ = connection.Close()
				return nil, err
			}
		}
		return connection, nil
	}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

type terminalControlFrame struct {
	Kind            string `json:"kind"`
	AttachedClients int    `json:"attachedClients"`
	Geometry        string `json:"geometry"`
	Error           struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func readTerminalControl(t *testing.T, connection *websocket.Conn, wantedKind string) terminalControlFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	for {
		messageType, payload, err := connection.Read(ctx)
		if err != nil {
			t.Fatalf("read terminal %s: %v", wantedKind, err)
		}
		if messageType == websocket.MessageBinary {
			continue
		}
		var frame terminalControlFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatalf("decode terminal control frame: %v: %s", err, payload)
		}
		if frame.Kind == wantedKind {
			return frame
		}
		if frame.Kind == "Error" {
			t.Fatalf("terminal returned Error while waiting for %s: %+v", wantedKind, frame.Error)
		}
	}
}

func requireTerminalPresence(t *testing.T, connection *websocket.Conn, kind string, attached int, geometry string) {
	t.Helper()
	frame := readTerminalControl(t, connection, kind)
	if frame.AttachedClients != attached || frame.Geometry != geometry {
		t.Fatalf("terminal %s = %+v, want attachedClients=%d geometry=%s", kind, frame, attached, geometry)
	}
}

func requireTerminalError(t *testing.T, connection *websocket.Conn, code, message string) {
	t.Helper()
	frame := readTerminalControl(t, connection, "Error")
	if frame.Error.Code != code || frame.Error.Message != message {
		t.Fatalf("terminal Error = %+v, want code=%s message=%q", frame.Error, code, message)
	}
}

func requireTerminalClosed(t *testing.T, connection *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	for {
		if _, _, err := connection.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatalf("terminal WebSocket did not close before deadline: %v", ctx.Err())
			}
			return
		}
	}
}

func readTerminalBinaryUntil(t *testing.T, connection *websocket.Conn, required, forbidden []string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	observed := make([]byte, 0, 4096)
	for {
		messageType, payload, err := connection.Read(ctx)
		if err != nil {
			t.Fatalf("read terminal binary output: required=%q observed=%q error=%v", required, observed, err)
		}
		if messageType == websocket.MessageText {
			var frame terminalControlFrame
			if json.Unmarshal(payload, &frame) == nil && frame.Kind == "Error" {
				t.Fatalf("terminal returned Error while waiting for binary output: %+v", frame.Error)
			}
			continue
		}
		observed = append(observed, payload...)
		if len(observed) > 4*1024*1024 {
			t.Fatalf("terminal binary proof exceeded bound while waiting for %q", required)
		}
		for _, marker := range forbidden {
			if bytes.Contains(observed, []byte(marker)) {
				t.Fatalf("terminal replayed forbidden marker %q", marker)
			}
		}
		complete := true
		for _, marker := range required {
			complete = complete && bytes.Contains(observed, []byte(marker))
		}
		if complete {
			return observed
		}
	}
}

func terminalSource(t *testing.T, fixture sessionFixture, name string) sessions.Session {
	t.Helper()
	listed, err := fixture.manager.List(context.Background())
	if err != nil {
		t.Fatalf("list terminal source: %v", err)
	}
	return requireSessionNamed(t, listed, name)
}

type terminalShadow struct {
	id   string
	name string
}

func terminalPhoneShadows(t *testing.T, fixture sessionFixture) []terminalShadow {
	t.Helper()
	output := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}")
	shadows := make([]terminalShadow, 0, 1)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) == 3 && fields[2] == "phone-shadow" && terminalPhoneShadowName.MatchString(fields[1]) {
			shadows = append(shadows, terminalShadow{id: fields[0], name: fields[1]})
		}
	}
	return shadows
}

func requireTerminalPhoneShadow(t *testing.T, fixture sessionFixture) terminalShadow {
	t.Helper()
	shadows := terminalPhoneShadows(t, fixture)
	if len(shadows) != 1 {
		t.Fatalf("terminal phone shadows = %+v, want exactly one", shadows)
	}
	return shadows[0]
}

func requireTerminalLaptopClient(t *testing.T, fixture sessionFixture, sourceID, sourceName string) string {
	t.Helper()
	output := fixture.tmux(t, "list-clients", "-F", "#{client_name}|#{session_id}|#{session_name}|#{client_flags}")
	matches := make([]string, 0, 1)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) == 4 && fields[1] == sourceID && fields[2] == sourceName && !strings.Contains(fields[3], "active-pane") {
			matches = append(matches, fields[0])
		}
	}
	if len(matches) != 1 {
		t.Fatalf("test-owned laptop client matches = %q, want exactly one: clients=%q", matches, output)
	}
	return matches[0]
}

func terminalHasOnlyExactClient(t *testing.T, fixture sessionFixture, clientName, sessionID, sessionName string) bool {
	t.Helper()
	output := fixture.tmux(t, "list-clients", "-F", "#{client_name}|#{session_id}|#{session_name}")
	matches := 0
	exact := false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 3 || fields[1] != sessionID || fields[2] != sessionName {
			continue
		}
		matches++
		exact = exact || fields[0] == clientName
	}
	return matches == 1 && exact
}

func terminalPaneIdentity(t *testing.T, fixture sessionFixture, target string) (int, uint64) {
	t.Helper()
	pidText := fixture.tmux(t, "display-message", "-p", "-t", target, "#{pane_pid}")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 {
		t.Fatalf("terminal pane has invalid PID: target=%s pid=%q error=%v", target, pidText, err)
	}
	startTime := linuxProcessStartTime(pid)
	if startTime == 0 {
		t.Fatalf("terminal pane has no kernel start time: target=%s pid=%d", target, pid)
	}
	return pid, startTime
}

func requireTerminalPaneIdentity(t *testing.T, fixture sessionFixture, target string, pid int, startTime uint64) {
	t.Helper()
	observedPID, observedStartTime := terminalPaneIdentity(t, fixture, target)
	if observedPID != pid || observedStartTime != startTime {
		t.Fatalf("terminal pane identity changed: target=%s want=%d/%d got=%d/%d", target, pid, startTime, observedPID, observedStartTime)
	}
}

func removeExactTerminalSourceLink(t *testing.T, fixture sessionFixture, source sessions.Session, panePID int) {
	t.Helper()
	parts := strings.Split(source.IdentityToken, ".")
	if len(parts) != 4 || "$"+parts[3] != source.ID || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(source.Name) {
		t.Fatalf("test source has invalid lifetime identity: id=%q name=%q token=%q", source.ID, source.Name, source.IdentityToken)
	}
	conditions := []string{
		"#{==:#{@skid_server_epoch}," + parts[0] + "}",
		"#{==:#{pid}," + parts[1] + "}",
		"#{==:#{start_time}," + parts[2] + "}",
		"#{==:#{session_id}," + source.ID + "}",
		"#{==:#{session_name}," + source.Name + "}",
		"#{==:#{pane_pid}," + strconv.Itoa(panePID) + "}",
	}
	condition := conditions[len(conditions)-1]
	for index := len(conditions) - 2; index >= 0; index-- {
		condition = "#{&&:" + conditions[index] + "," + condition + "}"
	}
	const mismatch = "SKIDBLADNIR_TEST_SOURCE_MISMATCH_V1"
	output := fixture.tmux(t, "if-shell", "-F", "-t", source.ID, condition,
		"kill-session -t '"+source.ID+"'", "display-message -p -l '"+mismatch+"'")
	if output != "" {
		t.Fatalf("refused exact test source-link removal: id=%s name=%s output=%q", source.ID, source.Name, output)
	}
}

func terminalSessionExists(t *testing.T, fixture sessionFixture, id, name string) bool {
	t.Helper()
	output := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}")
	return strings.Contains("\n"+output+"\n", "\n"+id+"|"+name+"\n")
}

func waitForTerminalAttachmentCount(t *testing.T, fixture sessionFixture, sourceID string, want int, timeout time.Duration) {
	t.Helper()
	waitForTerminalConditionWithin(t, "attached client count", timeout, func() bool {
		listed, err := fixture.manager.List(context.Background())
		if err != nil {
			return false
		}
		for _, session := range listed {
			if session.ID == sourceID {
				return session.AttachedClients == want
			}
		}
		return false
	})
}

type terminalLaptop struct {
	pty     *os.File
	command *exec.Cmd
	done    chan error
}

func startTerminalLaptop(t *testing.T, fixture sessionFixture, target string, columns, rows uint16) *terminalLaptop {
	t.Helper()
	command := isolatedTmuxCommand(tmuxPath, "-L", fixture.socket, "-f", "/dev/null", "attach-session", "-t", target)
	command.Env = append(withoutEnvironment(command.Env, "TERM"), "TERM=xterm-256color")
	terminalPTY, err := pty.StartWithSize(command, &pty.Winsize{Cols: columns, Rows: rows})
	if err != nil {
		t.Fatalf("start test-owned laptop tmux client: %v", err)
	}
	laptop := &terminalLaptop{pty: terminalPTY, command: command, done: make(chan error, 1)}
	go func() {
		_, _ = io.Copy(io.Discard, terminalPTY)
		laptop.done <- command.Wait()
	}()
	t.Cleanup(func() {
		_ = terminalPTY.Close()
		select {
		case <-laptop.done:
		case <-time.After(2 * time.Second):
			if command.Process != nil {
				_ = command.Process.Kill()
			}
			select {
			case <-laptop.done:
			case <-time.After(2 * time.Second):
				t.Errorf("test-owned laptop tmux client did not exit")
			}
		}
	})
	return laptop
}

func (laptop *terminalLaptop) Write(contents []byte) (int, error) {
	return laptop.pty.Write(contents)
}

func terminalLaptopShape(t *testing.T, fixture sessionFixture, sessionName string) string {
	t.Helper()
	for _, line := range strings.Split(fixture.tmux(t, "list-clients", "-F", "#{session_name}|#{client_width}x#{client_height}|#{client_flags}"), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) == 3 && fields[0] == sessionName {
			return fields[1] + "|" + fields[2]
		}
	}
	t.Fatalf("no laptop client attached to %q", sessionName)
	return ""
}

func writeTerminalFrame(t *testing.T, connection *websocket.Conn, messageType websocket.MessageType, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	if err := connection.Write(ctx, messageType, payload); err != nil {
		t.Fatalf("write terminal frame: %v", err)
	}
}

func waitForTerminalCondition(t *testing.T, description string, condition func() bool) {
	t.Helper()
	waitForTerminalConditionWithin(t, description, terminalIntegrationTimeout, condition)
}

func waitForTerminalConditionWithin(t *testing.T, description string, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("terminal condition did not converge: %s", description)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func waitForTerminalFile(t *testing.T, path, marker string) {
	t.Helper()
	waitForTerminalCondition(t, "file marker "+marker, func() bool {
		contents, err := os.ReadFile(path)
		return err == nil && bytes.Contains(contents, []byte(marker))
	})
}

func differentASCII(value byte) string {
	if value == '0' {
		return "1"
	}
	return "0"
}
