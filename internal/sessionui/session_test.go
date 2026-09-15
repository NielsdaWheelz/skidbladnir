package sessionui

import (
	"context"
	"encoding/json"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
	"github.com/coder/websocket"
	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func testFleetClient(t *testing.T) *fleetclient.Client {
	t.Helper()
	config := filepath.Join(t.TempDir(), "client.json")
	if os.WriteFile(config, []byte(`{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"mh-11111111111111111111111111111111","bearer":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`), 0600) != nil {
		t.Fatal("write synthetic client configuration")
	}
	client, err := fleetclient.Open(config)
	if err != nil {
		t.Fatal("open synthetic client configuration")
	}
	return client
}

func row(name, id string, pid int) fleetclient.Session {
	ref := fleetclient.Reference{Machine: "mh-11111111111111111111111111111111", TmuxID: id, IdentityToken: "lifetime" + id, Agent: &fleetclient.ProcessReference{PaneID: "%1", PID: pid, StartIdentity: "1"}}
	return fleetclient.Session{Name: name, Ref: ref.Encode(), Agent: &fleetclient.Agent{Provider: "Claude", Status: fleetclient.Status{State: "idle", Source: "native"}}}
}
func observation(rows ...fleetclient.Session) inventoryMsg {
	return inventoryMsg{value: fleetclient.Inventory{Peers: []fleetclient.Peer{{Label: "arch", Machine: "mh-11111111111111111111111111111111", OK: true, Profiles: []fleetclient.Profile{{Key: "work", Provider: "Codex"}}, Sessions: rows}}}}
}
func key(value string) tea.KeyPressMsg {
	if value == "enter" {
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Code: rune(value[0]), Text: value}
}

func TestRefreshRetainsSelectionAndPinsConfirmation(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(row("bravo", "$2", 22), row("charlie", "$3", 33)))
	m.Update(key("j"))
	m.Update(key("s"))
	before := m.View().Content
	if !strings.Contains(before, "charlie") || !strings.Contains(before, "stop") {
		t.Fatal("confirmation does not name selected session")
	}
	m.Update(observation(row("alpha", "$1", 11), row("bravo", "$2", 22), row("charlie", "$3", 99)))
	after := m.View().Content
	if !strings.Contains(after, "charlie") || strings.Contains(after, "stop alpha") {
		t.Fatal("refresh retargeted confirmation")
	}
	if m.pending.Ref != row("charlie", "$3", 33).Ref {
		t.Fatal("confirmation refreshed its agent target")
	}
	m.Update(key("n"))
	if m.selectedRow() == nil || m.selectedRow().session.Name != "charlie" {
		t.Fatal("refresh moved selection by row number")
	}
}

func TestCollectionHeadingsAndRemovedSelection(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(row("alpha", "$1", 11), row("bravo", "$2", 22), row("charlie", "$3", 33)))
	if !strings.Contains(m.View().Content, "unassigned") {
		t.Error("unassigned group has no heading")
	}
	m.Update(key("j"))
	m.Update(observation(row("alpha", "$1", 11), row("charlie", "$3", 33)))
	if m.selectedRow() == nil || m.selectedRow().session.Name != "charlie" {
		t.Error("removed selection did not retain its clamped visible session index")
	}
}

func TestMissingAndOfflineSelectionDisablesActions(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(row("reviewer", "$1", 11)))
	m.Update(inventoryMsg{value: fleetclient.Inventory{Partial: true, Peers: []fleetclient.Peer{{Label: "arch", Machine: "mh-11111111111111111111111111111111", Error: &fleetclient.Failure{Code: "unavailable", Dispatch: "not_sent"}}}}})
	if _, cmd := m.Update(key("s")); cmd != nil {
		t.Fatal("offline row started an action")
	}
	if !strings.Contains(m.View().Content, "unavailable") {
		t.Fatal("offline rows appear live")
	}
	m.Update(observation())
	if _, cmd := m.Update(key("x")); cmd != nil {
		t.Fatal("vanished selection started an action")
	}
}

func TestCreatedSelectionSurvivesEarlierInventoryCompletion(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.Update(observation(row("existing", "$1", 11)))
	m.refreshing = true
	created := row("created", "$2", 22)
	encoded, _ := json.Marshal(fleetclient.ObservedSession{Label: "arch", Machine: "mh-11111111111111111111111111111111", Session: created})
	m.Update(actionMsg{operation: "start", result: fleetclient.Result{OK: true, Value: encoded}})
	_, refresh := m.Update(observation(row("existing", "$1", 11)))
	if refresh == nil || m.selectedRow() == nil || m.selectedRow().session.Name != "created" {
		t.Fatal("older inventory erased the just-created selection")
	}
	m.Update(observation(row("created", "$2", 22), row("existing", "$1", 11)))
	if m.selectedRow() == nil || m.selectedRow().session.Name != "created" {
		t.Fatal("fresh inventory lost created selection")
	}
}

func TestDetailsWrapLongMetadataAndTableShowsDirectory(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width = 80
	session := row("reviewer", "$1", 11)
	session.CWD = "/very/long/parent/directory/code/project"
	m.Update(observation(session))
	if !strings.Contains(m.View().Content, "project") {
		t.Fatal("ordinary terminal width hides directory")
	}
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.detailLines()) <= len(m.text) {
		t.Fatal("long reference metadata was clipped instead of wrapped")
	}
}

func TestConfirmedActionDispatchesOriginalProcessThroughHTTP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body struct {
			PID    int    `json:"pid"`
			PaneID string `json:"paneId"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || r.URL.Path != "/v1/sessions/$2/agent/stop" || body.PID != 22 || body.PaneID != "%1" {
			t.Error("confirmation dispatched refreshed or different target")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		io.WriteString(w, `{"code":"AgentTargetStale","message":"fixture","dispatch":"not_sent"}`)
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
	os.WriteFile(config, []byte(`{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"mh-11111111111111111111111111111111","bearer":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`), 0600)
	client, err := fleetclient.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), client, nil, nil)
	m.Update(observation(row("reviewer", "$2", 22)))
	m.Update(key("s"))
	m.Update(observation(row("earlier", "$1", 11), row("reviewer", "$2", 99)))
	_, dispatch := m.Update(key("y"))
	if dispatch == nil {
		t.Fatal("confirmation did not dispatch")
	}
	m.Update(dispatch())
	if calls.Load() != 1 || !strings.Contains(m.View().Content, "AgentTargetStale") {
		t.Fatal("stale action was replayed or hidden")
	}
}

func TestNormalWidthHintsAndFullMetadataRemainVisible(t *testing.T) {
	m := newModel(context.Background(), testFleetClient(t), nil, nil)
	m.width = 80
	m.height = 24
	observed := observation(row("reviewer", "$1", 11))
	observed.value.Peers[0].ObservedAt = "2026-09-13T00:00:00Z"
	m.Update(observed)
	for _, hint := range []string{"enter attach", "space info", "r read", "i interrupt", "s stop", "x kill", "n new", "ctrl-r refresh", "q quit"} {
		if !strings.Contains(m.View().Content, hint) {
			t.Errorf("normal-width hint missing: %s", hint)
		}
	}
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	all := strings.Join(m.text, "\n")
	if !strings.Contains(all, "machine id: mh-") || !strings.Contains(all, "observed: 2026-09-13T00:00:00Z") || !strings.Contains(all, "reference:") {
		t.Fatal("details omit available identity/observation metadata")
	}
	m.Update(key("q"))
	m.Update(key("n"))
	if !strings.Contains(m.View().Content, "enter on space creates") || !strings.Contains(m.View().Content, "escape cancels") {
		t.Fatal("creation hints clipped at normal width")
	}
}

func TestRealProgramPTYEnterDetachListQuit(t *testing.T) {
	const machine = "mh-11111111111111111111111111111111"
	remoteReady, listedAgain := make(chan struct{}), make(chan struct{})
	var lists atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sessions" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"machine":{"handle":"`+machine+`","platform":"Linux"},"observedAt":"2026-09-13T00:00:00Z","profiles":[],"sessions":[{"tmuxId":"$3","tmuxName":"reviewer","identityToken":"lifetime","character":{"key":"a","displayName":"a"},"attachedClients":0}]}`)
			if lists.Add(1) == 2 {
				close(listedAgain)
			}
			return
		}
		if r.URL.Path != "/v1/sessions/$3/terminal" {
			t.Error("unexpected terminal route")
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		_, body, err := c.Read(r.Context())
		if err != nil {
			return
		}
		frame, err := terminal.ParseClientText(body)
		if err != nil || frame != (terminal.ResizeFrame{Columns: 80, Rows: 24}) {
			t.Error("tui handoff changed geometry")
		}
		hello, _ := terminal.EncodeHello(1)
		c.Write(r.Context(), websocket.MessageText, hello)
		close(remoteReady)
		for {
			kind, body, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if kind == websocket.MessageText {
				frame, err := terminal.ParseClientText(body)
				if err == nil {
					if _, ok := frame.(terminal.DetachFrame); ok {
						return
					}
				}
			}
		}
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
	os.WriteFile(config, []byte(`{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"`+machine+`","bearer":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`), 0600)
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
		t.Fatal("cannot size UI test PTY")
	}
	original, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := cancelreader.NewReader(master)
	if err != nil {
		t.Fatal(err)
	}
	uiReady, readerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(readerDone)
		var screen strings.Builder
		found := false
		buffer := make([]byte, 4096)
		for {
			n, err := reader.Read(buffer)
			if err != nil {
				return
			}
			if !found {
				screen.Write(buffer[:n])
				if strings.Contains(screen.String(), "reviewer") {
					found = true
					close(uiReady)
				}
				if screen.Len() > 1024*1024 {
					return
				}
			}
		}
	}()
	defer func() { reader.Cancel(); <-readerDone; reader.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, client, slave, slave) }()
	select {
	case <-uiReady:
	case <-time.After(5 * time.Second):
		t.Fatal("real tui did not display inventory")
	}
	master.Write([]byte("\r"))
	select {
	case <-remoteReady:
	case <-time.After(5 * time.Second):
		t.Fatal("real tui did not hand tty to terminal")
	}
	master.Write([]byte{0x1d, 'd'})
	select {
	case <-listedAgain:
	case <-time.After(5 * time.Second):
		t.Fatal("detach did not return to inventory")
	}
	master.Write([]byte("q"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("resumed tui failed to quit")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resumed tui did not own keyboard input")
	}
	restored, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(original, restored) {
		t.Fatal("composed tui/terminal path did not restore tty")
	}
}
