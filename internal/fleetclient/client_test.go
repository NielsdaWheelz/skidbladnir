package fleetclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const testMachine = "mh-11111111111111111111111111111111"
const testBearer = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const testTarget = `{"machine":"` + testMachine + `","tmuxId":"$3","identityToken":"lifetime","paneId":"%4","pid":321,"startIdentity":"1234"}`

func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	path := filepath.Join(t.TempDir(), "client.json")
	config := `{"peers":[{"label":"arch","origin":"https://example.com:8443","machine":"` + testMachine + `","bearer":"` + testBearer + `"}]}`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client.http.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	return client, server
}

func TestSendReachesOnlySelectedPeerWithExactText(t *testing.T) {
	var calls atomic.Int32
	text := "first line\nsecond line: `$(literal)`"
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/v1/sessions/$3/agent/send" {
			t.Error("wrong target route")
		}
		if r.Header.Get("Authorization") != "Bearer "+testBearer || r.Header.Get("Skidbladnir-Machine") != testMachine {
			t.Error("wrong peer credential binding")
		}
		var input struct {
			IdentityToken string `json:"identityToken"`
			PaneID        string `json:"paneId"`
			PID           int    `json:"pid"`
			StartIdentity string `json:"startIdentity"`
			Text          string `json:"text"`
			Mode          string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error("invalid outgoing json")
		}
		if input.Text != text || input.PaneID != "%4" || input.PID != 321 {
			t.Error("input or process target changed")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"method":"terminal","outcome":"written"}`)
	})
	encodedText, _ := json.Marshal(text)
	result := client.Execute(context.Background(), "send", []byte(`{"target":`+testTarget+`,"text":`+string(encodedText)+`}`))
	if calls.Load() != 1 {
		t.Fatalf("wanted exactly one peer dispatch, got %d", calls.Load())
	}
	if !result.OK {
		t.Fatal("accepted terminal delivery reported failure")
	}
}

func TestLostWriteAcknowledgementIsUnknownWithoutReplay(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error("could not close fixture connection")
			return
		}
		_ = conn.Close()
	})
	result := client.Execute(context.Background(), "send", []byte(`{"target":`+testTarget+`,"text":"dispatch once"}`))
	if result.OK || result.Error.Dispatch != "unknown" || calls.Load() != 1 {
		t.Fatal("lost acknowledgement was replayed or reported definitive")
	}
}

func TestRejectedRequestsRemainNotSentAndDoNotExposeMessages(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"code":"AgentBlocked","message":"private detail","dispatch":"not_sent"}`)
	})
	result := client.Execute(context.Background(), "send", []byte(`{"target":`+testTarget+`,"text":"hello"}`))
	encoded, err := result.Encode("send")
	if err != nil || result.OK || result.Error.Dispatch != "not_sent" || strings.Contains(string(encoded), "private detail") {
		t.Fatal("rejected write classification or error content changed")
	}
}

func TestInvalidTargetAndInputsDoNotDispatch(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	cases := []struct{ operation, input string }{
		{"send", `{"target":` + strings.Replace(testTarget, testMachine, "arch", 1) + `,"text":"hello"}`},
		{"send", `{"target":` + testTarget + `,"text":"hello","text":"again"}`},
		{"send", `{"target":` + testTarget + `,"text":"hello","keys":["enter"]}`},
		{"send", `{"target":` + testTarget + `,"text":"hello","mode":null}`},
		{"keys", `{"target":` + testTarget + `,"keys":["shell-command"]}`},
		{"read", `{"target":` + testTarget + `,"maxBytes":32769}`},
		{"start", `{"machine":"missing","profile":"work","cwd":"/tmp"}`},
		{"list", `{"machine":""}`},
		{"list", `{"machine":null}`},
		{"list", `{"machine":"missing"}`},
	}
	for _, value := range cases {
		result := client.Execute(context.Background(), value.operation, []byte(value.input))
		if result.OK || result.Error.Dispatch != "not_sent" {
			t.Errorf("%s admitted invalid input", value.operation)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached a peer")
	}
}

func TestStartAndReadPreservePublicFields(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sessions":
			var fields map[string]any
			if json.NewDecoder(r.Body).Decode(&fields) != nil || len(fields) != 3 || fields["optionalTmuxName"] != "reviewer" || fields["profile"] != "claude-work" || fields["cwd"] != "/tmp/project" {
				t.Error("creation fields were not translated")
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"observedAt":"2026-09-12T00:00:00Z","session":{"tmuxId":"$3","tmuxName":"reviewer","identityToken":"lifetime","character":{"key":"a.b","displayName":"name"},"attachedClients":0}}`)
		case "/v1/sessions/$3/agent/read":
			var fields map[string]any
			if json.NewDecoder(r.Body).Decode(&fields) != nil || fields["mode"] != "auto" || fields["maxBytes"] != float64(16384) {
				t.Error("read defaults changed")
			}
			_, _ = io.WriteString(w, `{"text":"older output","source":"native","scope":"recent_messages","truncated":false}`)
		default:
			t.Error("unexpected target route")
		}
	})
	start := client.Execute(context.Background(), "start", []byte(`{"machine":"arch","profile":"claude-work","cwd":"/tmp/project","name":"reviewer"}`))
	read := client.Execute(context.Background(), "read", []byte(`{"target":`+testTarget+`}`))
	if !start.OK || !read.OK || !strings.Contains(string(read.Value), "recent_messages") || calls.Load() != 2 {
		t.Fatal("start/read result was lost")
	}
}

func TestFleetListPreservesPartialResultsAndMachineBinding(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Host == "second.example.com:8443" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"code":"InternalError","message":"unavailable"}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"machine":{"handle":%q,"platform":"Linux"},"observedAt":"2026-09-12T00:00:00Z","profiles":[],"sessions":[]}`, testMachine)
	})
	// Use the real private-config ingress for an additional peer; no fixed fleet membership.
	config := struct {
		Peers []peer `json:"peers"`
	}{append(client.peers, peer{"second", "https://second.example.com:8443", "mh-22222222222222222222222222222222", testBearer})}
	encoded, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "peers.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	configured, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	configured.http = client.http
	result := configured.Execute(context.Background(), "list", []byte(`{}`))
	var list struct {
		Peers []struct {
			Label string   `json:"label"`
			OK    bool     `json:"ok"`
			Error *Failure `json:"error"`
		} `json:"peers"`
	}
	if json.Unmarshal(result.Value, &list) != nil || !result.OK || len(list.Peers) != 2 || !list.Peers[0].OK || list.Peers[1].OK {
		t.Fatal("one peer failure hid or failed the remaining fleet")
	}
	if list.Peers[1].Error.Dispatch != "not_sent" {
		t.Fatal("inventory failure was classified as a mutation")
	}
	configured.peers[0].Machine = "mh-33333333333333333333333333333333"
	result = configured.Execute(context.Background(), "list", []byte(`{"machine":"arch"}`))
	if json.Unmarshal(result.Value, &list) != nil || list.Peers[0].OK {
		t.Fatal("inventory from a different machine identity was accepted")
	}
}

func TestRedirectAndOversizedResponsesDoNotReplayWrites(t *testing.T) {
	for _, kind := range []string{"redirect", "oversized", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch kind {
				case "redirect":
					w.Header().Set("Location", "https://example.com:8443/second")
					w.WriteHeader(http.StatusTemporaryRedirect)
				case "oversized":
					_, _ = io.WriteString(w, strings.Repeat("x", MaximumControlBytes+1))
				case "malformed":
					_, _ = io.WriteString(w, `{"method":"terminal","outcome":"written","outcome":"accepted"}`)
				}
			})
			result := client.Execute(context.Background(), "send", []byte(`{"target":`+testTarget+`,"text":"hello"}`))
			if result.OK || result.Error.Dispatch != "unknown" || calls.Load() != 1 {
				t.Fatal("invalid acknowledgement was replayed or accepted")
			}
		})
	}
}

func TestControlAndAggregateOutputBounds(t *testing.T) {
	encoded, _ := json.Marshal(strings.Repeat("x", MaximumControlBytes))
	result := Result{OK: true, Value: encoded}
	if _, err := result.Encode("send"); err == nil {
		t.Fatal("control envelope exceeded bound")
	}
	if _, err := result.Encode("list"); err != nil {
		t.Fatal("fleet inventory inherited control-output bound")
	}
	result.Value, _ = json.Marshal(strings.Repeat("x", MaximumInventoryBytes))
	if _, err := result.Encode("list"); err == nil {
		t.Fatal("aggregate fleet envelope exceeded bound")
	}
}

func TestNullStartNameDoesNotCreateSession(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	result := client.Execute(context.Background(), "start", []byte(`{"machine":"arch","profile":"work","cwd":"/tmp","name":null}`))
	if result.OK || calls.Load() != 0 {
		t.Fatal("null optional name dispatched a create")
	}
}

func TestInventoryRejectsMissingAgentControlFacts(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"machine":{"handle":%q,"platform":"Linux"},"observedAt":"2026-09-12T00:00:00Z","profiles":[],"sessions":[{"tmuxId":"$3","tmuxName":"reviewer","identityToken":"lifetime","character":{"key":"a.b","displayName":"name"},"attachedClients":0,"agent":{"provider":"Claude","pid":321}}]}`, testMachine)
	})
	result := client.Execute(context.Background(), "list", []byte(`{}`))
	var list struct {
		Peers []struct {
			OK bool `json:"ok"`
		} `json:"peers"`
	}
	if json.Unmarshal(result.Value, &list) != nil || !result.OK || len(list.Peers) != 1 || list.Peers[0].OK {
		t.Fatal("incomplete agent control identity was accepted")
	}
}

func TestClientAcceptsExistingTailnetOriginForms(t *testing.T) {
	for _, origin := range []string{"https://100.100.100.1:8443", "https://[fd7a:115c:a1e0::1]:8443", "https://Arch.example:8443/"} {
		path := filepath.Join(t.TempDir(), "client.json")
		encoded, _ := json.Marshal(struct {
			Peers []peer `json:"peers"`
		}{[]peer{{"arch", origin, testMachine, testBearer}}})
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); err != nil {
			t.Error("existing machine origin form rejected")
		}
	}
}
