package fleetclient

import (
	"context"
	"encoding/base64"
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
const testSession = `{"tmuxId":"$3","tmuxName":"reviewer","identityToken":"lifetime","character":{"key":"a.b","displayName":"name"},"attachedClients":0,"cwd":"/tmp/project","agent":{"provider":"Claude","pid":321,"paneId":"%4","startIdentity":"1234","status":{"state":"idle","source":"native"},"methods":{"read":"native","send":"terminal","interrupt":"terminal"}}}`

func TestSpaceResponsePresence(t *testing.T) {
	for index, field := range []string{``, `,"space":"alpha"`, `,"space":"é"`} {
		encoded := []byte(`{"observedAt":"2026-09-15T00:00:00Z","session":` + strings.TrimSuffix(testSession, "}") + field + `}}`)
		if !validResponse("start", encoded, testMachine) {
			t.Errorf("valid optional membership rejected: case=%d", index)
		}
	}
	for index, field := range []string{`null`, `""`, `"e\u0301"`, `"\ud800"`, `" a"`, `"a\nb"`, `42`} {
		encoded := []byte(`{"observedAt":"2026-09-15T00:00:00Z","session":` + strings.TrimSuffix(testSession, "}") + `,"space":` + field + `}}`)
		if validResponse("start", encoded, testMachine) {
			t.Errorf("invalid present membership admitted: case=%d", index)
		}
	}
}

func testRef() string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"machine":"` + testMachine + `","tmuxId":"$3","identityToken":"lifetime","agent":{"paneId":"%4","pid":321,"startIdentity":"1234"}}`))
}

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

func inventory(w http.ResponseWriter, machine, session string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"machine":{"handle":%q,"platform":"Linux"},"observedAt":"2026-09-12T00:00:00Z","profiles":[{"key":"work","label":"work","provider":"Codex"}],"sessions":[%s]}`, machine, session)
}

func TestNamesResolveWithoutTargetAssemblyAndWritesRetainText(t *testing.T) {
	var reads, writes atomic.Int32
	text := "first line\nsecond line: `$(literal)`\n"
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reads.Add(1)
			inventory(w, testMachine, testSession)
			return
		}
		writes.Add(1)
		if r.URL.Path != "/v1/sessions/$3/agent/send" || r.Header.Get("Authorization") != "Bearer "+testBearer || r.Header.Get("Skidbladnir-Machine") != testMachine {
			t.Error("wrong target or authentication")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["text"] != text || body["pid"] != float64(321) || body["paneId"] != "%4" {
			t.Error("input or target changed")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"method":"terminal","outcome":"written"}`)
	})
	result := client.Execute(context.Background(), Request{Operation: "send", Name: "reviewer", Text: text})
	if !result.OK || reads.Load() != 1 || writes.Load() != 1 || result.ExitCode("send") != 0 {
		t.Fatal("name did not resolve and dispatch once")
	}
}

func TestAmbiguousAndIncompleteNamesNeverDispatch(t *testing.T) {
	var writes atomic.Int32
	var offline atomic.Bool
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
			return
		}
		if r.Host == "second.example.com:8443" {
			if offline.Load() {
				w.WriteHeader(503)
				return
			}
			inventory(w, "mh-22222222222222222222222222222222", testSession)
			return
		}
		inventory(w, testMachine, testSession)
	})
	client.peers = append(client.peers, peer{"second", "https://second.example.com:8443", "mh-22222222222222222222222222222222", testBearer})
	for _, code := range []string{"name_ambiguous", "inventory_incomplete"} {
		result := client.Execute(context.Background(), Request{Operation: "send", Name: "reviewer", Text: "fixture"})
		if result.OK || result.Error.Code != code || result.Error.Dispatch != "not_sent" {
			t.Fatalf("wrong selector failure: expected %s", code)
		}
		offline.Store(true)
	}
	result := client.Execute(context.Background(), Request{Operation: "info", Name: "reviewer", Machine: "arch"})
	if !result.OK || writes.Load() != 0 {
		t.Fatal("qualified read failed or selector dispatched a mutation")
	}
	list := client.Execute(context.Background(), Request{Operation: "list"})
	var listed Inventory
	if json.Unmarshal(list.Value, &listed) != nil || !listed.Partial || len(listed.Peers) != 2 || list.ExitCode("list") != 1 {
		t.Fatal("partial fleet hidden")
	}
}

func TestRefsSurviveRenameButAgentWritesNeverRefreshProcess(t *testing.T) {
	var reads, writes atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reads.Add(1)
			inventory(w, testMachine, strings.ReplaceAll(strings.ReplaceAll(testSession, "reviewer", "renamed"), "321", "999"))
			return
		}
		writes.Add(1)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "DELETE" {
			if body["tmuxName"] != "renamed" || body["identityToken"] != "lifetime" {
				t.Error("kill lost current name or lifetime")
			}
			w.WriteHeader(204)
			return
		}
		if body["pid"] != float64(321) {
			t.Error("stale mutation reference was refreshed")
		}
		w.WriteHeader(409)
		io.WriteString(w, `{"code":"AgentTargetStale","message":"fixture","dispatch":"not_sent"}`)
	})
	old := testRef()
	info := client.Execute(context.Background(), Request{Operation: "info", Ref: old})
	var observed ObservedSession
	if json.Unmarshal(info.Value, &observed) != nil || observed.Session.Name != "renamed" || observed.Session.Ref == old {
		t.Fatal("info failed to reobserve session")
	}
	send := client.Execute(context.Background(), Request{Operation: "send", Ref: old, Text: "fixture"})
	if send.OK || reads.Load() != 1 {
		t.Fatal("agent write refreshed its process")
	}
	kill := client.Execute(context.Background(), Request{Operation: "kill", Ref: old})
	if !kill.OK || writes.Load() != 2 || reads.Load() != 2 {
		t.Fatal("exact session closure failed after rename")
	}
}

func TestStartAndShellReferencesSupportImmediateSessionOperations(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			inventory(w, testMachine, `{"tmuxId":"$3","tmuxName":"shell","identityToken":"lifetime","character":{"key":"a","displayName":"a"},"attachedClients":0}`)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["cwd"] != "~" || body["profile"] != "work" || body["optionalTmuxName"] != "shell" {
			t.Error("start defaults changed")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		io.WriteString(w, `{"observedAt":"2026-09-12T00:00:00Z","session":{"tmuxId":"$3","tmuxName":"shell","identityToken":"lifetime","character":{"key":"a","displayName":"a"},"attachedClients":0}}`)
	})
	start := client.Execute(context.Background(), Request{Operation: "start", Kind: LaunchAgent, Machine: "arch", Name: "shell", Profile: "work"})
	var observed ObservedSession
	if !start.OK || json.Unmarshal(start.Value, &observed) != nil || observed.Session.Agent != nil || observed.Session.Ref == "" {
		t.Fatal("start did not return a shell reference")
	}
	info := client.Execute(context.Background(), Request{Operation: "info", Ref: observed.Session.Ref})
	read := client.Execute(context.Background(), Request{Operation: "read", Ref: observed.Session.Ref})
	if !info.OK || read.OK || read.Error.Code != "agent_unavailable" {
		t.Fatal("shell capability boundary incorrect")
	}
}

func TestLostWriteAndUnconfirmedStopAreNotSuccessOrReplayed(t *testing.T) {
	var writes atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writes.Add(1)
		if strings.HasSuffix(r.URL.Path, "/stop") {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"agent":"unconfirmed","terminal":"closed"}`)
			return
		}
		io.Copy(io.Discard, r.Body)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close()
	})
	send := client.Execute(context.Background(), Request{Operation: "send", Ref: testRef(), Text: "fixture"})
	if send.OK || send.Error.Dispatch != "unknown" || writes.Load() != 1 {
		t.Fatal("lost write replayed or made definitive")
	}
	stop := client.Execute(context.Background(), Request{Operation: "stop", Ref: testRef()})
	if !stop.OK || stop.ExitCode("stop") != 1 || writes.Load() != 2 {
		t.Fatal("partial stop lost or declared complete")
	}
}

func TestInvalidInputsNeverReachPeer(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	invalidRefs := []string{"", testRef() + "=", base64.RawURLEncoding.EncodeToString([]byte(`{"machine":"` + testMachine + `","tmuxId":"$3","identityToken":"x","agent":null}`)), base64.RawURLEncoding.EncodeToString([]byte(`{"machine":"` + testMachine + `","machine":"` + testMachine + `","tmuxId":"$3","identityToken":"x"}`))}
	for _, ref := range invalidRefs {
		if result := client.Execute(context.Background(), Request{Operation: "send", Ref: ref, Text: "fixture"}); result.OK {
			t.Error("invalid reference admitted")
		}
	}
	for _, request := range []Request{{Operation: "send", Ref: testRef(), Name: "reviewer", Text: "fixture"}, {Operation: "keys", Ref: testRef(), Keys: []string{"bad"}}, {Operation: "read", Ref: testRef(), MaxBytes: 32769}, {Operation: "start", Kind: LaunchAgent, Machine: "missing", Name: "x", Profile: "work"}} {
		result := client.Execute(context.Background(), request)
		if result.OK || result.Error.Dispatch != "not_sent" {
			t.Error("invalid request admitted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached peer")
	}
}

func TestOutputBoundsAndTruncatedReadStatus(t *testing.T) {
	result := Result{OK: true, Value: json.RawMessage(`{"text":"tail","source":"terminal","scope":"terminal_history","truncated":true}`)}
	if result.ExitCode("read") != 0 {
		t.Fatal("bounded successful read was treated as failure")
	}
	result.Value, _ = json.Marshal(strings.Repeat("x", MaximumInventoryBytes))
	if _, err := result.Encode("list"); err == nil {
		t.Fatal("final aggregate exceeded bound")
	}
}

func TestConfiguredOriginsAndPrivateFile(t *testing.T) {
	for _, origin := range []string{"https://100.100.100.1:8443", "https://[fd7a:115c:a1e0::1]:8443", "https://Arch.example:8443/"} {
		path := filepath.Join(t.TempDir(), "client.json")
		encoded, _ := json.Marshal(struct {
			Peers []peer `json:"peers"`
		}{[]peer{{"arch", origin, testMachine, testBearer}}})
		os.WriteFile(path, encoded, 0600)
		if _, err := Open(path); err != nil {
			t.Error("valid existing origin rejected")
		}
		os.Chmod(path, 0644)
		if _, err := Open(path); err == nil {
			t.Error("public credential file admitted")
		}
	}
}

func TestProjectedInventoryCannotExceedAggregateLimit(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Each host response fits, but repeated opaque refs make the public fleet result larger.
		token := strings.Repeat("a", 1200)
		session := strings.Replace(testSession, "lifetime", token, 1)
		inventory(w, testMachine, strings.TrimSuffix(strings.Repeat(session+",", 550), ","))
	})
	result := client.Execute(context.Background(), Request{Operation: "list"})
	if result.OK || result.Error.Code != "output_limit" {
		t.Fatal("projected inventory overflow was hidden or rows dropped")
	}
}

func TestExactReferenceRejectsNewSessionLifetimeAndDoesNotQueryOtherPeers(t *testing.T) {
	var mutations, otherReads atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "second.example.com:8443" {
			otherReads.Add(1)
			w.WriteHeader(503)
			return
		}
		if r.Method != "GET" {
			mutations.Add(1)
			return
		}
		inventory(w, testMachine, strings.ReplaceAll(testSession, "lifetime", "replacement"))
	})
	client.peers = append(client.peers, peer{"second", "https://second.example.com:8443", "mh-22222222222222222222222222222222", testBearer})
	result := client.Execute(context.Background(), Request{Operation: "kill", Ref: testRef()})
	if result.OK || result.Error.Code != "SessionIdentityMismatch" || otherReads.Load() != 0 || mutations.Load() != 0 {
		t.Fatal("stale ref was rebound or depended on unrelated peer")
	}
}

func TestMalformedAcknowledgementsDoNotReplayWrites(t *testing.T) {
	for _, kind := range []string{"redirect", "oversized", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch kind {
				case "redirect":
					w.Header().Set("Location", "https://example.com:8443/again")
					w.WriteHeader(307)
				case "oversized":
					io.WriteString(w, strings.Repeat("x", MaximumControlBytes+1))
				case "duplicate":
					io.WriteString(w, `{"method":"terminal","outcome":"written","outcome":"unknown"}`)
				}
			})
			result := client.Execute(context.Background(), Request{Operation: "send", Ref: testRef(), Text: "fixture"})
			if result.OK || result.Error.Dispatch != "unknown" || calls.Load() != 1 {
				t.Fatal("invalid acknowledgement became success or replay")
			}
		})
	}
}

func TestTerminalHandshakePreservesStaleTargetWithoutRetry(t *testing.T) {
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		io.WriteString(w, `{"code":"SessionIdentityMismatch","message":"private fixture detail"}`)
	})
	connection, failure := client.OpenTerminal(context.Background(), Request{Ref: testRef()})
	if connection != nil || failure == nil || failure.Code != "SessionIdentityMismatch" || failure.Dispatch != "not_sent" || calls.Load() != 1 {
		t.Fatal("terminal stale refusal was hidden or retried")
	}
}

func TestMissingAttachmentCountIsNotInventedAsZero(t *testing.T) {
	for _, replacement := range []string{"", `"attachedClients":null,`} {
		client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			inventory(w, testMachine, strings.Replace(testSession, `"attachedClients":0,`, replacement, 1))
		})
		result := client.Execute(context.Background(), Request{Operation: "list"})
		var value Inventory
		if !result.OK || json.Unmarshal(result.Value, &value) != nil || !value.Partial || value.Peers[0].OK {
			t.Fatal("missing attachment count became a fabricated observation")
		}
	}
}
