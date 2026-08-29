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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
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
		t.Fatal("list terminal source")
	}
	source := requireSessionNamed(t, listed, "shared-terminal")
	laptop := startTerminalLaptop(t, fixture, source.TmuxID, 120, 40)
	waitForTerminalCondition(t, "laptop attachment", func() bool {
		listed, listErr := fixture.manager.List(context.Background())
		if listErr != nil {
			return false
		}
		return requireSessionID(t, listed, source.TmuxID).AttachedClients == 1
	})
	laptopShapeBefore := terminalLaptopShape(t, fixture, "shared-terminal")
	windowShapeBefore := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{window_width}x#{window_height}")

	bearerPath := filepath.Join(fixture.root, "terminal-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint terminal bearer")
	}
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)

	terminalURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/sessions/" + source.TmuxID + "/terminal"
	staleToken := source.IdentityToken[:len(source.IdentityToken)-1] + differentASCII(source.IdentityToken[len(source.IdentityToken)-1])
	sessionsBefore := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}")
	staleHeaders := http.Header{
		"Authorization":                []string{"Bearer " + bearer},
		"Skidbladnir-Machine":          []string{integrationMachineText},
		"Skidbladnir-Session-Identity": []string{staleToken},
	}
	_, response, err := websocket.Dial(context.Background(), terminalURL, &websocket.DialOptions{HTTPHeader: staleHeaders})
	if err == nil || response == nil || response.StatusCode != http.StatusConflict {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("stale terminal identity result mismatch: error_present=%t response_present=%t status=%d", err != nil, response != nil, status)
	}
	if response.Body != nil {
		_ = response.Body.Close()
	}
	if sessionsAfter := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}"); sessionsAfter != sessionsBefore {
		t.Fatal("stale terminal identity mutated sessions")
	}

	headers := http.Header{
		"Authorization":                []string{"Bearer " + bearer},
		"Skidbladnir-Machine":          []string{integrationMachineText},
		"Skidbladnir-Session-Identity": []string{source.IdentityToken},
	}
	connection, response, err := websocket.Dial(context.Background(), terminalURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("open terminal WebSocket: status=%d", status)
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { connection.CloseNow() })

	readContext, cancelRead := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	messageType, helloPayload, err := connection.Read(readContext)
	cancelRead()
	if err != nil || messageType != websocket.MessageText {
		t.Fatalf("read terminal Hello: type=%v error_present=%t payload_bytes=%d", messageType, err != nil, len(helloPayload))
	}
	var hello struct {
		Kind            string `json:"kind"`
		AttachedClients int    `json:"attachedClients"`
		Geometry        string `json:"geometry"`
	}
	if err := json.Unmarshal(helloPayload, &hello); err != nil {
		t.Fatalf("decode terminal Hello: payload_bytes=%d", len(helloPayload))
	}
	if hello.Kind != "Hello" || hello.AttachedClients != 2 || hello.Geometry != "Constrained" {
		t.Fatalf(
			"terminal Hello mismatch: kind_match=%t attached_clients=%d geometry_match=%t",
			hello.Kind == "Hello",
			hello.AttachedClients,
			hello.Geometry == "Constrained",
		)
	}
	readTerminalBinaryUntil(t, connection, []string{"FRAME=A BUFFER=seed-A"}, nil)
	if attention := fixture.tmux(t, "show-options", "-pqv", "-t", "shared-terminal:0.0", "@skid_attention"); attention != "" {
		t.Fatal("opening terminal did not clear active-pane attention")
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
		t.Fatalf("phone input reached laptop-selected pane: content_bytes=%d", len(contents))
	}
	if active := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{pane_index}"); active != "0" {
		t.Fatalf("phone active-pane key sequence stole laptop selection: %q", active)
	}

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	closeContext, cancelClose := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	for {
		// PTY bytes committed before Detach may already be in the transport. Drain those bounded
		// frames and prove the server closes the WebSocket, rather than assuming the next read is
		// necessarily the close observation.
		_, _, closeErr := connection.Read(closeContext)
		if closeErr == nil {
			continue
		}
		if closeContext.Err() != nil {
			cancelClose()
			t.Fatal("Detach left the terminal WebSocket open")
		}
		break
	}
	cancelClose()
	waitForTerminalCondition(t, "phone shadow teardown", func() bool {
		listed, listErr := fixture.manager.List(context.Background())
		if listErr != nil {
			return false
		}
		for _, session := range listed {
			if session.TmuxID == source.TmuxID {
				return session.AttachedClients == 1
			}
		}
		return false
	})
	if got := terminalLaptopShape(t, fixture, "shared-terminal"); got != laptopShapeBefore {
		t.Fatalf("phone detach changed laptop geometry: before=%q after=%q", laptopShapeBefore, got)
	}
	if got := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{window_width}x#{window_height}"); got != windowShapeBefore {
		t.Fatalf("phone detach changed laptop-owned window geometry: before=%q after=%q", windowShapeBefore, got)
	}
	if active := fixture.tmux(t, "display-message", "-p", "-t", "shared-terminal:0", "#{pane_index}"); active != "0" {
		t.Fatalf("phone detach moved the shared active pane: index=%s", active)
	}
	if _, err := laptop.Write([]byte("LAPTOP-A\r")); err != nil {
		t.Fatal("write through surviving laptop client")
	}
	waitForTerminalFile(t, logA, "LAPTOP-A")
}

func TestTerminalBearerRotationRevokesLiveStreamAndReconnectsWithoutReplay(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "rotation.log")
	fixture.tmux(t, "new-session", "-d", "-s", "rotation-terminal", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalRotationScript(), "fixture", logPath)
	source := terminalSource(t, fixture, "rotation-terminal")
	gatewayFixture := newTerminalGateway(t, fixture, nil)

	oldConnection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, oldConnection, "Hello", 1, "Owner")
	writeTerminalFrame(t, oldConnection, websocket.MessageBinary, []byte("ROTATE\r"))
	readTerminalBinaryUntil(t, oldConnection,
		[]string{"REPLAY-CANDIDATE-V1", "SCREEN=AFTER-ROTATION"}, nil)
	waitForTerminalCondition(t, "rotation screen cleared before bearer remint", func() bool {
		visible := fixture.tmux(t, "capture-pane", "-p", "-J", "-t", source.TmuxID)
		return strings.Contains(visible, "SCREEN=AFTER-ROTATION") && !strings.Contains(visible, "REPLAY-CANDIDATE-V1")
	})

	freshBearer, err := auth.Mint(auth.MintOptions{Path: gatewayFixture.bearerPath})
	if err != nil {
		t.Fatal("remint terminal bearer")
	}
	requireTerminalError(t, oldConnection, "ReconnectRequired", "Reconnect required.")
	requireTerminalClosed(t, oldConnection)
	waitForTerminalAttachmentCount(t, fixture, source.TmuxID, 0, terminalIntegrationTimeout)
	waitForTerminalCondition(t, "revoked terminal shadow teardown", func() bool {
		return len(terminalPhoneShadows(t, fixture)) == 0
	})

	freshConnection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), freshBearer, source.IdentityToken)
	requireTerminalPresence(t, freshConnection, "Hello", 1, "Owner")
	writeTerminalFrame(t, freshConnection, websocket.MessageBinary, []byte("POST-RECONNECT\r"))
	readTerminalBinaryUntil(t, freshConnection,
		[]string{"SCREEN=AFTER-ROTATION", "TOKEN=POST-RECONNECT"}, []string{"REPLAY-CANDIDATE-V1"})
	writeTerminalFrame(t, freshConnection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	requireTerminalClosed(t, freshConnection)
	waitForTerminalAttachmentCount(t, fixture, source.TmuxID, 0, terminalIntegrationTimeout)
}

func TestTerminalPresenceAndLastLinkDetachPreserveThePane(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "last-link.log")
	fixture.tmux(t, "new-session", "-d", "-s", "last-link-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "LAST-LINK")
	source := terminalSource(t, fixture, "last-link-source")
	startTerminalLaptop(t, fixture, source.TmuxID, 80, 24)
	waitForTerminalAttachmentCount(t, fixture, source.TmuxID, 1, terminalIntegrationTimeout)
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.TmuxID)
	gatewayFixture := newTerminalGateway(t, fixture, nil)

	connection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 2, "Constrained")
	shadow := requireTerminalPhoneShadow(t, fixture)
	fixture.tmux(t, "detach-client", "-t", requireTerminalLaptopClient(t, fixture, source.TmuxID, source.TmuxName))
	requireTerminalPresence(t, connection, "Presence", 1, "Owner")

	removeExactTerminalSourceLink(t, fixture, source, panePID)
	waitForTerminalCondition(t, "phone shadow became the last grouped link", func() bool {
		return !terminalSessionExists(t, fixture, source.TmuxID, source.TmuxName) && terminalSessionExists(t, fixture, shadow.id, shadow.name)
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
		t.Fatal("promoted last phone link disappeared")
	}
}

func TestTerminalDetachLeavesAnAttachedPhoneShadowUntouched(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "attached-shadow.log")
	fixture.tmux(t, "new-session", "-d", "-s", "attached-shadow-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "ATTACHED-SHADOW")
	source := terminalSource(t, fixture, "attached-shadow-source")
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.TmuxID)
	gatewayFixture := newTerminalGateway(t, fixture, nil)

	connection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")
	shadow := requireTerminalPhoneShadow(t, fixture)
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)

	unrelated := startTerminalLaptop(t, fixture, shadow.id, 100, 30)
	waitForTerminalAttachmentCount(t, fixture, source.TmuxID, 2, terminalIntegrationTimeout)
	unrelatedClient := requireTerminalLaptopClient(t, fixture, shadow.id, shadow.name)
	requireTerminalPresence(t, connection, "Presence", 2, "Constrained")

	writeTerminalFrame(t, connection, websocket.MessageText, []byte(`{"kind":"Detach"}`))
	requireTerminalClosed(t, connection)
	waitForTerminalCondition(t, "attached phone shadow retained with unrelated client", func() bool {
		return terminalHasOnlyExactClient(t, fixture, unrelatedClient, shadow.id, shadow.name)
	})

	if !terminalSessionExists(t, fixture, source.TmuxID, source.TmuxName) {
		t.Fatal("terminal detach removed source session")
	}
	if !terminalSessionExists(t, fixture, shadow.id, shadow.name) {
		t.Fatal("terminal detach removed attached phone shadow")
	}
	if marker := fixture.tmux(t, "show-options", "-qv", "-t", shadow.id, "@skid_internal"); marker != "phone-shadow" {
		t.Fatal("terminal detach changed attached phone-shadow marker")
	}
	requireTerminalPaneIdentity(t, fixture, source.TmuxID, panePID, paneStartTime)
	requireTerminalPaneIdentity(t, fixture, shadow.id, panePID, paneStartTime)
	if _, err := unrelated.Write([]byte("UNRELATED-SURVIVES\r")); err != nil {
		t.Fatal("write through unrelated shadow client after phone detach")
	}
	waitForTerminalFile(t, logPath, "UNRELATED-SURVIVES")
}

func TestTerminalSlowReaderIsTornDownWithoutKillingTheSource(t *testing.T) {
	fixture := newSessionFixture(t)
	fixture.tmux(t, "new-session", "-d", "-s", "slow-terminal", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalFloodScript())
	source := terminalSource(t, fixture, "slow-terminal")
	panePID, paneStartTime := terminalPaneIdentity(t, fixture, source.TmuxID)
	writeGate := newTerminalWriteGate()
	gatewayFixture := newTerminalGateway(t, fixture, writeGate)

	connection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")
	writeGate.arm()
	writeTerminalFrame(t, connection, websocket.MessageBinary, []byte("FLOOD\r"))
	waitForTerminalAttachmentCount(t, fixture, source.TmuxID, 0, 20*time.Second)
	waitForTerminalConditionWithin(t, "slow-reader shadow teardown", 20*time.Second, func() bool {
		return len(terminalPhoneShadows(t, fixture)) == 0
	})
	requireTerminalPaneIdentity(t, fixture, source.TmuxID, panePID, paneStartTime)
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
    /bin/sleep 0.05
    printf '\033[3J\033[2J\033[HSCREEN=AFTER-ROTATION\n'
  else
    printf 'TOKEN=%s\n' "$token"
  fi
done`
}

func terminalFloodScript() string {
	return `while IFS= read -r token; do
  if [ "$token" = FLOOD ]; then
    /usr/bin/yes 'SKIDBLADNIR-SLOW-READER-0123456789abcdefghijklmnopqrstuvwxyz' | /usr/bin/head -c 33554432
    printf '\nFLOOD-DONE\n'
  fi
done`
}

// A grouped session initially opens on its lowest-index window. The phone
// shadow must instead open on the source's current window without retargeting
// the source session itself.
func TestTerminalAttachOpensTheSourceCurrentWindow(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "current-window.log")
	fixture.tmux(t, "new-session", "-d", "-s", "windowed-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "WINDOW-0")
	fixture.tmux(t, "new-window", "-t", "windowed-source:", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath+".1", "WINDOW-1")
	fixture.tmux(t, "select-window", "-t", "windowed-source:1")
	sourceWindow := fixture.tmux(t, "display-message", "-p", "-t", "windowed-source:", "#{window_id}")

	source := terminalSource(t, fixture, "windowed-source")
	gatewayFixture := newTerminalGateway(t, fixture, nil)
	connection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")

	shadow := requireTerminalPhoneShadow(t, fixture)
	if got := fixture.tmux(t, "display-message", "-p", "-t", shadow.id, "#{window_id}"); got != sourceWindow {
		t.Fatalf("phone shadow current-window mismatch: got=%s want=%s", got, sourceWindow)
	}
	if got := fixture.tmux(t, "display-message", "-p", "-t", "windowed-source:", "#{window_id}"); got != sourceWindow {
		t.Fatalf("attach moved the source current window: got=%s want=%s", got, sourceWindow)
	}
}

// Kill settles the live phone attachment and its owned shadow before applying
// the exact machine-bound kill, while unrelated sessions remain untouched.
func TestKillWithOpenTerminalClosesTheStreamAndKillsExactly(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "kill-open-terminal.log")
	fixture.tmux(t, "new-session", "-d", "-s", "kill-me", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "KILL-ME")
	fixture.tmux(t, "new-session", "-d", "-s", "bystander", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath+".bystander", "BYSTANDER")
	source := terminalSource(t, fixture, "kill-me")
	bystander := terminalSource(t, fixture, "bystander")
	bystanderPID, bystanderStart := terminalPaneIdentity(t, fixture, bystander.TmuxID)
	gatewayFixture := newTerminalGateway(t, fixture, nil)
	connection := dialTerminal(t, nil, gatewayFixture.url(source.TmuxID), gatewayFixture.bearer, source.IdentityToken)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")

	body, err := json.Marshal(map[string]string{"tmuxName": source.TmuxName, "identityToken": source.IdentityToken})
	if err != nil {
		t.Fatal("encode kill request")
	}
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		gatewayFixture.server.URL+"/v1/sessions/"+source.TmuxID,
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal("build kill request")
	}
	request.Header.Set("Authorization", "Bearer "+gatewayFixture.bearer)
	request.Header.Set("Skidbladnir-Machine", integrationMachineText)
	request.Header.Set("Content-Type", "application/json")
	response, err := gatewayFixture.server.Client().Do(request)
	if err != nil {
		t.Fatal("kill with open terminal")
	}
	responseBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("kill with open terminal failed: response_read=%t status=%d body_bytes=%d", readErr == nil, response.StatusCode, len(responseBody))
	}
	requireTerminalClosed(t, connection)
	waitForTerminalCondition(t, "killed session and its shadow disappear", func() bool {
		return !terminalSessionExists(t, fixture, source.TmuxID, source.TmuxName) && len(terminalPhoneShadows(t, fixture)) == 0
	})
	if !terminalSessionExists(t, fixture, bystander.TmuxID, bystander.TmuxName) {
		t.Fatal("kill with open terminal destroyed the bystander session")
	}
	requireTerminalPaneIdentity(t, fixture, bystander.TmuxID, bystanderPID, bystanderStart)
}

// The terminal endpoint is shell-equivalent authority. Bearer, machine, and
// session identity all travel in headers and reject before any tmux mutation.
func TestTerminalEndpointRejectsMissingCredentialsBeforeAnyMutation(t *testing.T) {
	fixture := newSessionFixture(t)
	logPath := filepath.Join(fixture.root, "auth-chokepoint.log")
	fixture.tmux(t, "new-session", "-d", "-s", "auth-source", "-x", "80", "-y", "24", "-c", fixture.project,
		"--", "/bin/sh", "-c", terminalPaneScript(), "fixture", logPath, "AUTH")
	source := terminalSource(t, fixture, "auth-source")
	gatewayFixture := newTerminalGateway(t, fixture, nil)
	sessionsBefore := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}")

	tests := []struct {
		name    string
		url     string
		headers http.Header
		status  int
	}{
		{
			name: "missing bearer",
			url:  gatewayFixture.url(source.TmuxID),
			headers: http.Header{
				"Skidbladnir-Machine":          []string{integrationMachineText},
				"Skidbladnir-Session-Identity": []string{source.IdentityToken},
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "missing machine",
			url:  gatewayFixture.url(source.TmuxID),
			headers: http.Header{
				"Authorization":                []string{"Bearer " + gatewayFixture.bearer},
				"Skidbladnir-Session-Identity": []string{source.IdentityToken},
			},
			status: http.StatusConflict,
		},
		{
			name: "wrong machine",
			url:  gatewayFixture.url(source.TmuxID),
			headers: http.Header{
				"Authorization":                []string{"Bearer " + gatewayFixture.bearer},
				"Skidbladnir-Machine":          []string{"mh-ffffffffffffffffffffffffffffffff"},
				"Skidbladnir-Session-Identity": []string{source.IdentityToken},
			},
			status: http.StatusConflict,
		},
		{
			name: "missing session identity",
			url:  gatewayFixture.url(source.TmuxID),
			headers: http.Header{
				"Authorization":       []string{"Bearer " + gatewayFixture.bearer},
				"Skidbladnir-Machine": []string{integrationMachineText},
			},
			status: http.StatusBadRequest,
		},
		{
			name: "session identity in query",
			url:  gatewayFixture.url(source.TmuxID) + "?identity=" + source.IdentityToken,
			headers: http.Header{
				"Authorization":       []string{"Bearer " + gatewayFixture.bearer},
				"Skidbladnir-Machine": []string{integrationMachineText},
			},
			status: http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		connection, response, dialErr := websocket.Dial(context.Background(), test.url, &websocket.DialOptions{HTTPHeader: test.headers})
		if connection != nil {
			_ = connection.CloseNow()
		}
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		if dialErr == nil || response == nil || status != test.status {
			t.Fatalf("terminal credential rejection mismatch: case=%s error_present=%t response_present=%t status=%d want=%d", test.name, dialErr != nil, response != nil, status, test.status)
		}
		if response.Body != nil {
			_ = response.Body.Close()
		}
	}
	if sessionsAfter := fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{@skid_internal}"); sessionsAfter != sessionsBefore {
		t.Fatal("rejected terminal credentials mutated the tmux session set")
	}
	if shadows := terminalPhoneShadows(t, fixture); len(shadows) != 0 {
		t.Fatalf("rejected terminal credentials created phone shadows: count=%d", len(shadows))
	}
}

type terminalGatewayFixture struct {
	server     *httptest.Server
	bearerPath string
	bearer     string
}

type terminalWriteGate struct {
	armed     atomic.Bool
	closed    chan struct{}
	closeOnce sync.Once
}

func newTerminalWriteGate() *terminalWriteGate {
	return &terminalWriteGate{closed: make(chan struct{})}
}

func (gate *terminalWriteGate) arm() {
	gate.armed.Store(true)
}

func (gate *terminalWriteGate) close() {
	gate.closeOnce.Do(func() { close(gate.closed) })
}

type terminalWriteGateListener struct {
	net.Listener
	gate *terminalWriteGate
}

func (listener terminalWriteGateListener) Accept() (net.Conn, error) {
	connection, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &terminalWriteGateConnection{Conn: connection, gate: listener.gate}, nil
}

type terminalWriteGateConnection struct {
	net.Conn
	gate *terminalWriteGate
}

func (connection *terminalWriteGateConnection) Write(contents []byte) (int, error) {
	if !connection.gate.armed.Load() {
		return connection.Conn.Write(contents)
	}
	<-connection.gate.closed
	return 0, net.ErrClosed
}

func (connection *terminalWriteGateConnection) Close() error {
	connection.gate.close()
	return connection.Conn.Close()
}

func newTerminalGateway(t *testing.T, fixture sessionFixture, writeGate *terminalWriteGate) terminalGatewayFixture {
	t.Helper()
	bearerPath := filepath.Join(fixture.root, "terminal-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint terminal bearer")
	}
	server := httptest.NewUnstartedServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	if writeGate != nil {
		server.Listener = terminalWriteGateListener{Listener: server.Listener, gate: writeGate}
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
		"Skidbladnir-Machine":          []string{integrationMachineText},
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
		t.Fatalf("open terminal WebSocket: status=%d", status)
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	t.Cleanup(func() { connection.CloseNow() })
	return connection
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
			t.Fatalf("read terminal control frame: wanted_kind=%s", wantedKind)
		}
		if messageType == websocket.MessageBinary {
			continue
		}
		var frame terminalControlFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatalf("decode terminal control frame: payload_bytes=%d", len(payload))
		}
		if frame.Kind == wantedKind {
			return frame
		}
		if frame.Kind == "Error" {
			t.Fatalf("terminal returned Error while waiting for control frame: wanted_kind=%s", wantedKind)
		}
	}
}

func requireTerminalPresence(t *testing.T, connection *websocket.Conn, kind string, attached int, geometry string) {
	t.Helper()
	frame := readTerminalControl(t, connection, kind)
	if frame.AttachedClients != attached || frame.Geometry != geometry {
		t.Fatalf("terminal presence mismatch: kind=%s attached_clients=%d geometry_match=%t", kind, frame.AttachedClients, frame.Geometry == geometry)
	}
}

func requireTerminalError(t *testing.T, connection *websocket.Conn, code, message string) {
	t.Helper()
	frame := readTerminalControl(t, connection, "Error")
	if frame.Error.Code != code || frame.Error.Message != message {
		t.Fatalf("terminal Error mismatch: code_match=%t message_match=%t", frame.Error.Code == code, frame.Error.Message == message)
	}
}

func requireTerminalClosed(t *testing.T, connection *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	for {
		if _, _, err := connection.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("terminal WebSocket did not close before deadline")
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
			t.Fatalf("read terminal binary output: observed_bytes=%d required_marker_count=%d forbidden_marker_count=%d", len(observed), len(required), len(forbidden))
		}
		if messageType == websocket.MessageText {
			var frame terminalControlFrame
			if json.Unmarshal(payload, &frame) == nil && frame.Kind == "Error" {
				t.Fatal("terminal returned Error while waiting for binary output")
			}
			continue
		}
		observed = append(observed, payload...)
		if len(observed) > 4*1024*1024 {
			t.Fatalf("terminal binary proof exceeded bound: observed_bytes=%d required_marker_count=%d", len(observed), len(required))
		}
		for _, marker := range forbidden {
			if bytes.Contains(observed, []byte(marker)) {
				t.Fatal("terminal replayed a forbidden marker")
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
		t.Fatal("list terminal source")
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
		t.Fatalf("terminal phone shadow count=%d, want exactly one", len(shadows))
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
		t.Fatalf("test-owned laptop client match count=%d, want exactly one", len(matches))
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

func terminalPaneIdentity(t *testing.T, fixture sessionFixture, target string) (int, processinfo.StartIdentity) {
	t.Helper()
	pidText := fixture.tmux(t, "display-message", "-p", "-t", target, "#{pane_pid}")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 {
		t.Fatalf("terminal pane has invalid PID: parse_ok=%t positive=%t", err == nil, pid > 1)
	}
	startTime := processStartIdentity(pid)
	if startTime == "" {
		t.Fatal("terminal pane has no kernel start time")
	}
	return pid, startTime
}

func requireTerminalPaneIdentity(t *testing.T, fixture sessionFixture, target string, pid int, startTime processinfo.StartIdentity) {
	t.Helper()
	observedPID, observedStartTime := terminalPaneIdentity(t, fixture, target)
	if observedPID != pid || observedStartTime != startTime {
		t.Fatalf("terminal pane identity changed: pid_match=%t start_match=%t", observedPID == pid, observedStartTime == startTime)
	}
}

func removeExactTerminalSourceLink(t *testing.T, fixture sessionFixture, source sessions.Session, panePID int) {
	t.Helper()
	parts := strings.Split(source.IdentityToken, ".")
	if len(parts) != 4 || "$"+parts[3] != source.TmuxID || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(source.TmuxName) {
		t.Fatalf("test source has invalid lifetime identity: field_count=%d id_match=%t name_valid=%t", len(parts), len(parts) == 4 && "$"+parts[3] == source.TmuxID, regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(source.TmuxName))
	}
	conditions := []string{
		"#{==:#{@skid_server_epoch}," + parts[0] + "}",
		"#{==:#{pid}," + parts[1] + "}",
		"#{==:#{start_time}," + parts[2] + "}",
		"#{==:#{session_id}," + source.TmuxID + "}",
		"#{==:#{session_name}," + source.TmuxName + "}",
		"#{==:#{pane_pid}," + strconv.Itoa(panePID) + "}",
	}
	condition := conditions[len(conditions)-1]
	for index := len(conditions) - 2; index >= 0; index-- {
		condition = "#{&&:" + conditions[index] + "," + condition + "}"
	}
	const mismatch = "SKIDBLADNIR_TEST_SOURCE_MISMATCH_V1"
	output := fixture.tmux(t, "if-shell", "-F", "-t", source.TmuxID, condition,
		"kill-session -t '"+source.TmuxID+"'", "display-message -p -l '"+mismatch+"'")
	if output != "" {
		t.Fatalf("refused exact test source-link removal: output_bytes=%d", len(output))
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
			if session.TmuxID == sourceID {
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
		t.Fatal("start test-owned laptop tmux client")
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
	t.Fatal("no laptop client attached")
	return ""
}

func writeTerminalFrame(t *testing.T, connection *websocket.Conn, messageType websocket.MessageType, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	if err := connection.Write(ctx, messageType, payload); err != nil {
		t.Fatal("write terminal frame")
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
	waitForTerminalCondition(t, "terminal file marker", func() bool {
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
