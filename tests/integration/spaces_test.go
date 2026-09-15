//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcli"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

// This journey uses only existing public HTTP shapes and fixture primitives so
// the same test executes against the pre-feature production baseline.
func TestSpacesAuthenticatedMembershipPreservesSessionLifetime(t *testing.T) {
	fixture := newSessionFixture(t)
	server, bearer := newMachineGateway(t, fixture, integrationMachineText)
	createBody := spaceJSON(t, map[string]string{"kind": "agent", "cwd": fixture.project, "profile": "personal", "optionalTmuxName": "space-source", "space": "alpha"})
	response := request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "", createBody)
	assertStatus(t, response, http.StatusCreated)
	created := decodeObject(t, response)["session"].(map[string]any)
	if created["space"] != "alpha" {
		t.Fatal("create did not report observed assigned membership")
	}
	id, token := created["tmuxId"].(string), created["identityToken"].(string)
	target := server.URL + "/v1/sessions/" + id + "/space"
	read := func(id string) map[string]any {
		response := request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
		assertStatus(t, response, http.StatusOK)
		return findSessionID(t, decodeObject(t, response), id)
	}
	assign := func(label string) {
		body := spaceJSON(t, map[string]string{"identityToken": token, "space": label})
		assertBodylessStatus(t, request(t, server.Client(), http.MethodPut, target, bearer, "", body), http.StatusNoContent)
	}
	fixture.waitForAgent(t, context.Background(), "space-source")
	fixture.attachClient(t, id)
	waitForTerminalAttachmentCount(t, fixture, id, 1, tmuxConvergenceTimeout)
	before := spaceStableFacts(t, fixture, id)
	for _, label := range []string{"beta", "beta", "", "", "alpha"} {
		assign(label)
		card := read(id)
		value, present := card["space"]
		if present != (label != "") || present && value != label {
			t.Fatal("set/change/clear/no-op projection differs from acknowledged assignment")
		}
		if spaceStableFacts(t, fixture, id) != before {
			t.Fatal("assignment changed a session/process/geometry/attachment fact")
		}
		if card["identityToken"] != token {
			t.Fatal("assignment changed the exact session reference")
		}
	}
	if got := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_space_b64"); got != base64.RawURLEncoding.EncodeToString([]byte("alpha")) {
		t.Fatal("metadata is not canonical unpadded base64url")
	}
	shellID := fixture.tmux(t, "new-session", "-d", "-P", "-F", "#{session_id}", "-s", "space-shell", "--", "/bin/sh")
	shellCard := read(shellID)
	if _, present := shellCard["agent"]; present {
		t.Fatal("plain shell fixture unexpectedly projects an agent")
	}
	shellBody := spaceJSON(t, map[string]string{"identityToken": shellCard["identityToken"].(string), "space": "shell-space"})
	assertBodylessStatus(t, request(t, server.Client(), http.MethodPut, server.URL+"/v1/sessions/"+shellID+"/space", bearer, "", shellBody), http.StatusNoContent)
	if read(shellID)["space"] != "shell-space" {
		t.Fatal("ordinary shell without agent could not be filed")
	}

	// A grouped member shares windows, but owns its own membership.
	grouped := fixture.tmux(t, "new-session", "-d", "-P", "-F", "#{session_id}", "-t", id, "-s", "space-grouped")
	if _, present := read(grouped)["space"]; present {
		t.Fatal("grouped session inherited another member's membership")
	}
	groupToken := read(grouped)["identityToken"].(string)
	groupBody := spaceJSON(t, map[string]string{"identityToken": groupToken, "space": "group-only"})
	assertBodylessStatus(t, request(t, server.Client(), http.MethodPut, server.URL+"/v1/sessions/"+grouped+"/space", bearer, "", groupBody), http.StatusNoContent)
	if read(id)["space"] != "alpha" || read(grouped)["space"] != "group-only" {
		t.Fatal("grouped memberships were coupled")
	}

	fixture.tmux(t, "rename-session", "-t", id, "space-renamed")
	assign("after-rename")
	if read(id)["tmuxName"] != "space-renamed" {
		t.Fatal("filing changed the tmux name")
	}
	fixture.tmux(t, "respawn-pane", "-k", "-t", id, "--", "/bin/sh", "-c", "while :; do /bin/sleep 300; done")
	assign("after-process")
	if read(id)["identityToken"] != token {
		t.Fatal("foreground replacement invalidated session-only filing")
	}

	// Optional metadata is local-only, display-safe, canonical and read-only.
	for index, encoded := range []string{"", "YQ==", "YR", "YQ\n", "YQ\r\n", "!", strings.Repeat("a", 343), base64.RawURLEncoding.EncodeToString([]byte("e\u0301")), base64.RawURLEncoding.EncodeToString([]byte("a\u200fb")), base64.RawURLEncoding.EncodeToString([]byte{0xff})} {
		fixture.tmux(t, "set-option", "-t", id, "--", "@skid_space_b64", encoded)
		stored := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_space_b64")
		if _, present := read(id)["space"]; present {
			t.Fatalf("invalid optional metadata projected a label: case=%d", index)
		}
		if got := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_space_b64"); got != stored {
			t.Fatalf("inventory repaired optional metadata: case=%d", index)
		}
	}
	fixture.tmux(t, "set-option", "-u", "-t", id, "--", "@skid_space_b64")
	fixture.tmux(t, "set-option", "-g", "--", "@skid_space_b64", "Z2xvYmFs")
	if _, present := read(id)["space"]; present {
		t.Fatal("global-only metadata was inherited")
	}
	assign("")
	if got := fixture.tmux(t, "show-options", "-qv", "-t", id, "@skid_space_b64"); got != "" {
		t.Fatal("clear left local metadata")
	}
	fixture.tmux(t, "set-option", "-gu", "--", "@skid_space_b64")

	validBody := spaceJSON(t, map[string]string{"identityToken": token, "space": "stable"})
	assign("stable")
	for _, test := range []struct {
		name, body, code string
		status           int
	}{
		{"missing space", spaceJSON(t, map[string]string{"identityToken": token}), "InvalidRequest", 400},
		{"missing token", `{"space":"other"}`, "InvalidRequest", 400},
		{"empty token", `{"identityToken":"","space":"other"}`, "InvalidRequest", 400},
		{"null space", strings.Replace(validBody, `"stable"`, "null", 1), "InvalidRequest", 400},
		{"wrong type", strings.Replace(validBody, `"stable"`, "42", 1), "InvalidRequest", 400},
		{"extra field", strings.TrimSuffix(validBody, "}") + `,"tmuxName":"space-renamed"}`, "InvalidRequest", 400},
		{"wrong case", strings.Replace(validBody, `"space"`, `"Space"`, 1), "InvalidRequest", 400},
		{"duplicate", strings.TrimSuffix(validBody, "}") + `,"space":"other"}`, "InvalidRequest", 400},
		{"decomposed", spaceJSON(t, map[string]string{"identityToken": token, "space": "e\u0301"}), "SpaceInvalid", 422},
		{"display control", spaceJSON(t, map[string]string{"identityToken": token, "space": "a\u061cb"}), "SpaceInvalid", 422},
		{"invalid escaped scalar", strings.Replace(validBody, `"stable"`, `"\ud800"`, 1), "InvalidRequest", 400},
		{"oversized", spaceJSON(t, map[string]string{"identityToken": token, "space": strings.Repeat("a", 65536)}), "RequestTooLarge", 413},
		{"stale token", `{"identityToken":"invalid","space":"other"}`, "SessionIdentityMismatch", 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, server.Client(), http.MethodPut, target, bearer, "", test.body)
			assertStatus(t, response, test.status)
			failure := decodeObject(t, response)
			if failure["code"] != test.code || failure["dispatch"] != "not_sent" {
				t.Fatal("rejection lost code or definite non-dispatch")
			}
			if read(id)["space"] != "stable" {
				t.Fatal("rejected input mutated membership")
			}
		})
	}
	for _, malformed := range []string{"not-a-session", id + "/nested", id + "%2fextra"} {
		response := request(t, server.Client(), http.MethodPut, server.URL+"/v1/sessions/"+malformed+"/space", bearer, "", validBody)
		assertStatus(t, response, http.StatusBadRequest)
		if decodeObject(t, response)["dispatch"] != "not_sent" {
			t.Fatal("path rejection lost definite non-dispatch")
		}
	}
	for _, test := range []struct {
		bearer, machine string
		status          int
	}{
		{"", integrationMachineText, 401}, {bearer, "mh-22222222222222222222222222222222", 409},
	} {
		response := requestForMachine(t, server.Client(), http.MethodPut, target, test.bearer, test.machine, "", validBody)
		assertStatus(t, response, test.status)
		if decodeObject(t, response)["dispatch"] != "not_sent" {
			t.Fatal("access rejection lost definite non-dispatch")
		}
	}
	for _, replacement := range []string{`"space":""`, `"space":null`} {
		invalidCreate := strings.Replace(createBody, `"space":"alpha"`, replacement, 1)
		response := request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "", invalidCreate)
		want := http.StatusUnprocessableEntity
		if strings.Contains(replacement, "null") {
			want = http.StatusBadRequest
		}
		assertStatus(t, response, want)
		response.Body.Close()
	}
	unassignedBody := spaceJSON(t, map[string]string{"kind": "agent", "cwd": fixture.project, "profile": "personal", "optionalTmuxName": "space-unassigned"})
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "", unassignedBody)
	assertStatus(t, response, http.StatusCreated)
	if _, present := decodeObject(t, response)["session"].(map[string]any)["space"]; present {
		t.Fatal("unassigned create emitted optional membership")
	}

	var writers sync.WaitGroup
	statuses := make(chan int, 2)
	for _, label := range []string{"concurrent-a", "concurrent-b"} {
		body := spaceJSON(t, map[string]string{"identityToken": token, "space": label})
		writers.Go(func() {
			response := request(t, server.Client(), http.MethodPut, target, bearer, "", body)
			statuses <- response.StatusCode
			response.Body.Close()
		})
	}
	writers.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusNoContent {
			t.Fatal("concurrent absolute assignment was rejected")
		}
	}
	if value := read(id)["space"]; value != "concurrent-a" && value != "concurrent-b" {
		t.Fatal("concurrent assignment produced a torn value")
	}
	assign("final")
	if read(id)["space"] != "final" {
		t.Fatal("last acknowledged assignment did not apply")
	}

	rebuilt, err := sessions.New(sessions.Config{TmuxPath: tmuxPath, SocketName: fixture.socket, Workdir: fixture.workingDirectories, CataloguePath: fixture.cataloguePath, Profiles: fixture.manager.Profiles()})
	if err != nil {
		t.Fatal("reconstruct membership owner")
	}
	rebuiltFixture := fixture
	rebuiltFixture.manager = rebuilt
	rebuiltFixture.root = t.TempDir()
	second, secondBearer := newMachineGateway(t, rebuiltFixture, integrationMachineText)
	response = request(t, second.Client(), http.MethodGet, second.URL+"/v1/sessions", secondBearer, "", "")
	if findSessionID(t, decodeObject(t, response), id)["space"] != "final" {
		t.Fatal("gateway reconstruction lost membership")
	}

	fixture.tmux(t, "kill-session", "-t", id)
	response = request(t, server.Client(), http.MethodPut, target, bearer, "", validBody)
	assertStatus(t, response, http.StatusNotFound)
	if decodeObject(t, response)["dispatch"] != "not_sent" {
		t.Fatal("absent original lifetime lost definite non-dispatch")
	}
	if read(grouped)["space"] != "group-only" {
		t.Fatal("destroying grouped peer changed surviving membership")
	}
}

func TestSpacesLostCommandCompletionStaysUnknown(t *testing.T) {
	for _, mode := range []string{"lost completion", "vanished after dispatch", "failed required read"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newSessionFixture(t)
			created, err := fixture.manager.Create(context.Background(), sessions.CreateInput{Kind: sessions.LaunchAgent, CWD: fixture.project, Profile: "personal", OptionalTmuxName: "space-uncertain"})
			if err != nil {
				t.Fatal("create uncertainty fixture")
			}
			wrapper := filepath.Join(fixture.root, "space-completion-tmux")
			count := filepath.Join(fixture.root, "space-write-count")
			// Fault injection is at the external executable boundary. The real tmux
			// assignment executes, then its acknowledgement is replaced by failure.
			before, after := "", ""
			if mode == "failed required read" {
				before = "if [ \"$5\" = display-message ] && [ \"$#\" = 7 ]; then exit 42; fi\n"
			}
			if mode == "vanished after dispatch" {
				after = fmt.Sprintf("%s -L %s -f /dev/null kill-session -t %s\n", shellQuote(tmuxPath), shellQuote(fixture.socket), shellQuote(created.Session.TmuxID))
			}
			script := fmt.Sprintf("#!/bin/sh\nset -eu\n%sif [ \"$5\" = if-shell ]; then\n  printf x >> %s\n  %s \"$@\" >/dev/null\n  %sexit 42\nfi\nexec %s \"$@\"\n", before, shellQuote(count), shellQuote(tmuxPath), after, shellQuote(tmuxPath))
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal("write completion fault fixture")
			}
			manager, err := sessions.New(sessions.Config{TmuxPath: wrapper, SocketName: fixture.socket, Workdir: fixture.workingDirectories, CataloguePath: fixture.cataloguePath, Profiles: fixture.manager.Profiles()})
			if err != nil {
				t.Fatal("construct completion fault gateway")
			}
			fixture.manager = manager
			server, bearer := newMachineGateway(t, fixture, integrationMachineText)
			body := spaceJSON(t, map[string]string{"identityToken": created.Session.IdentityToken, "space": "applied"})
			response := request(t, server.Client(), http.MethodPut, server.URL+"/v1/sessions/"+created.Session.TmuxID+"/space", bearer, "", body)
			assertStatus(t, response, http.StatusInternalServerError)
			failure := decodeObject(t, response)
			if mode == "failed required read" {
				if failure["code"] != "InternalError" || failure["dispatch"] != "not_sent" {
					t.Fatal("failed required observation was treated as mismatched lifetime or possible dispatch")
				}
				if _, err := os.Stat(count); !os.IsNotExist(err) {
					t.Fatal("failed required observation reached the assignment queue")
				}
				return
			}
			if failure["code"] != "InternalError" || failure["dispatch"] != "unknown" {
				t.Fatal("possible dispatch was relabelled as definite rejection")
			}
			if mode == "lost completion" {
				observed := fixture.tmux(t, "show-options", "-qv", "-t", created.Session.TmuxID, "@skid_space_b64")
				if observed != base64.RawURLEncoding.EncodeToString([]byte("applied")) {
					t.Fatal("uncertainty fixture did not execute the real assignment")
				}
			}
			attempts, err := os.ReadFile(count)
			if err != nil || string(attempts) != "x" {
				t.Fatal("uncertain assignment was replayed")
			}
		})
	}
}

func TestSpacesRejectRecycledSessionIDAfterServerRestart(t *testing.T) {
	fixture := newSessionFixture(t)
	// Reuse catalogue/profile/workdir setup on a second empty registered socket,
	// so ending its one exact session ends the server without a blanket kill.
	fixture.socket = randomTmuxSocketName(t, "skid-space-restart")
	fixture.socketPath = namedTmuxSocketPath(fixture.socket)
	manager, err := sessions.New(sessions.Config{TmuxPath: tmuxPath, SocketName: fixture.socket, Workdir: fixture.workingDirectories, CataloguePath: fixture.cataloguePath, Profiles: fixture.manager.Profiles()})
	if err != nil {
		t.Fatal("construct isolated restart owner")
	}
	fixture.manager = manager
	fixture.root = t.TempDir()
	server, bearer := newMachineGateway(t, fixture, integrationMachineText)
	var cleanup *sessions.Session
	t.Cleanup(func() {
		if cleanup == nil {
			return
		}
		if err := manager.Kill(context.Background(), sessions.KillInput{TmuxID: cleanup.TmuxID, TmuxName: cleanup.TmuxName, IdentityToken: cleanup.IdentityToken}); err != nil {
			t.Error("clean exact restart session")
		}
	})
	first, err := manager.Create(context.Background(), sessions.CreateInput{Kind: sessions.LaunchAgent, CWD: fixture.project, Profile: "personal", OptionalTmuxName: "space-recycled"})
	if err != nil {
		t.Fatal("create original server lifetime")
	}
	cleanup = &first.Session
	oldServer := captureTestTmuxServer(t, tmuxPath, fixture.socketPath)
	if err := manager.Kill(context.Background(), sessions.KillInput{TmuxID: first.Session.TmuxID, TmuxName: first.Session.TmuxName, IdentityToken: first.Session.IdentityToken}); err != nil {
		t.Fatal("end exact original session")
	}
	cleanup = nil
	waitForTerminalCondition(t, "original test server exits", func() bool { return processStartIdentity(oldServer.pid) != oldServer.kernelStartTime })
	second, err := manager.Create(context.Background(), sessions.CreateInput{Kind: sessions.LaunchAgent, CWD: fixture.project, Profile: "personal", OptionalTmuxName: "space-recycled"})
	if err != nil {
		t.Fatal("create replacement server lifetime")
	}
	cleanup = &second.Session
	if first.Session.TmuxID != second.Session.TmuxID || first.Session.IdentityToken == second.Session.IdentityToken {
		t.Fatal("fixture did not recycle local address under distinct server lifetime")
	}
	body := spaceJSON(t, map[string]string{"identityToken": first.Session.IdentityToken, "space": "stale"})
	response := request(t, server.Client(), http.MethodPut, server.URL+"/v1/sessions/"+second.Session.TmuxID+"/space", bearer, "", body)
	assertStatus(t, response, http.StatusConflict)
	failure := decodeObject(t, response)
	if failure["code"] != "SessionIdentityMismatch" || failure["dispatch"] != "not_sent" {
		t.Fatal("recycled address admitted stale server reference")
	}
	if fixture.tmux(t, "show-options", "-qv", "-t", second.Session.TmuxID, "@skid_space_b64") != "" {
		t.Fatal("stale assignment changed replacement membership")
	}
}

func spaceJSON(t *testing.T, value map[string]string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal("encode synthetic space request")
	}
	return string(encoded)
}

func spaceStableFacts(t *testing.T, fixture sessionFixture, id string) string {
	t.Helper()
	return fixture.tmux(t, "display-message", "-p", "-t", id,
		"#{session_id}|#{session_name}|#{session_attached}|#{session_width}|#{session_height}|#{window_id}|#{pane_id}|#{pane_pid}|#{pane_width}|#{pane_height}|#{pane_current_path}|#{@skid_character}|#{@skid_profile}|#{@skid_objective_b64}")
}

func TestSpacesRealFleetClientAndCLIComposeTwoHosts(t *testing.T) {
	// This top-level test is sequential: DefaultTransport is restored before
	// parallel top-level tests resume. Open clones this real TLS transport.
	first := newSessionFixture(t)
	second := newSessionFixture(t)
	firstGateway, firstBearer := newMachineGateway(t, first, integrationMachineText)
	secondHandle := "mh-22222222222222222222222222222222"
	secondGateway, secondBearer := newMachineGateway(t, second, secondHandle)
	var writes atomic.Int32
	var loseNextReply atomic.Bool
	firstTLS := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		if incoming.Method != http.MethodPut {
			firstGateway.Config.Handler.ServeHTTP(writer, incoming)
			return
		}
		writes.Add(1)
		if !loseNextReply.Swap(false) {
			firstGateway.Config.Handler.ServeHTTP(writer, incoming)
			return
		}
		// Execute the actual gateway/tmux write, then lose its network reply.
		recorded := httptest.NewRecorder()
		firstGateway.Config.Handler.ServeHTTP(recorded, incoming)
		if recorded.Code != http.StatusNoContent {
			t.Error("lost-reply fixture did not execute the membership write")
		}
		connection, _, err := writer.(http.Hijacker).Hijack()
		if err != nil {
			t.Error("hijack owned TLS fixture connection")
			return
		}
		connection.Close()
	}))
	t.Cleanup(firstTLS.Close)
	secondTLS := httptest.NewTLSServer(secondGateway.Config.Handler)
	t.Cleanup(secondTLS.Close)
	transport := firstTLS.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		target := ""
		switch address {
		case "example.com:8443":
			target = firstTLS.Listener.Addr().String()
		case "second.example.com:8443":
			target = secondTLS.Listener.Addr().String()
		default:
			return nil, fmt.Errorf("unregistered integration peer")
		}
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	config := filepath.Join(t.TempDir(), "peers.json")
	encoded, err := json.Marshal(map[string]any{"peers": []map[string]string{
		{"label": "arch", "origin": "https://example.com:8443", "machine": integrationMachineText, "bearer": firstBearer},
		{"label": "second", "origin": "https://second.example.com:8443", "machine": secondHandle, "bearer": secondBearer},
	}})
	if err != nil || os.WriteFile(config, encoded, 0o600) != nil {
		t.Fatal("write real fleet client configuration")
	}
	client, err := fleetclient.Open(config)
	if err != nil {
		t.Fatal("open real fleet client")
	}
	label, err := space.Parse("shared")
	if err != nil {
		t.Fatal("parse synthetic membership")
	}
	var createdRefs []string
	for index, machine := range []string{"arch", "second"} {
		result := client.Execute(context.Background(), fleetclient.Request{Operation: "start", Kind: fleetclient.LaunchAgent, Machine: machine, Name: "space-client", Profile: "personal", CWD: "~", Space: label})
		if !result.OK {
			t.Fatalf("real fleet create failed: host=%d", index)
		}
		var created struct {
			Session fleetclient.Session `json:"session"`
		}
		if json.Unmarshal(result.Value, &created) != nil || created.Session.Space != label {
			t.Fatal("fleet create lost observed membership")
		}
		createdRefs = append(createdRefs, created.Session.Ref)
	}
	first.waitForAgent(t, context.Background(), "space-client")
	initial := client.Execute(context.Background(), fleetclient.Request{Operation: "info", Ref: createdRefs[0]})
	var initialObservation struct {
		Session fleetclient.Session `json:"session"`
	}
	if !initial.OK || json.Unmarshal(initial.Value, &initialObservation) != nil {
		t.Fatal("observe initial agent reference")
	}
	initialReference, err := fleetclient.DecodeReference(initialObservation.Session.Ref)
	if err != nil || initialReference.Agent == nil {
		t.Fatal("retained reference does not include the original agent lifetime")
	}
	createdRefs[0] = initialObservation.Session.Ref
	filter, err := space.NamedFilter(label)
	if err != nil {
		t.Fatal("construct named filter")
	}
	list := client.Execute(context.Background(), fleetclient.Request{Operation: "list", SpaceFilter: filter})
	var inventory fleetclient.Inventory
	if !list.OK || json.Unmarshal(list.Value, &inventory) != nil || inventory.Partial || len(inventory.Peers) != 2 {
		t.Fatal("real fleet list lost complete two-host inventory")
	}
	for _, peer := range inventory.Peers {
		if !peer.OK || len(peer.Sessions) != 1 || peer.Sessions[0].Space != label {
			t.Fatal("equal named filter failed across hosts")
		}
	}
	retained, err := fleetclient.DecodeReference(createdRefs[0])
	if err != nil {
		t.Fatal("retain returned exact reference")
	}
	first.tmux(t, "rename-session", "-t", retained.TmuxID, "space-client-renamed")
	first.tmux(t, "respawn-pane", "-k", "-t", retained.TmuxID, "--", "/bin/sh", "-c", "while :; do /bin/sleep 300; done")
	var output, stderr bytes.Buffer
	status := agentcli.Run(context.Background(), []string{"space", "--ref", createdRefs[0], "--set", "e\u0301", "--config", config, "--json"}, strings.NewReader(""), &output, &stderr)
	if status != 0 || strings.TrimSpace(output.String()) != `{"ok":true,"result":{"space":"é"}}` {
		t.Fatal("cli did not acknowledge normalized membership on retained session reference")
	}
	output.Reset()
	stderr.Reset()
	status = agentcli.Run(context.Background(), []string{"space", "--ref", createdRefs[0], "--clear", "--config", config, "--json"}, strings.NewReader(""), &output, &stderr)
	if status != 0 || strings.TrimSpace(output.String()) != `{"ok":true,"result":{"space":""}}` {
		t.Fatal("cli explicit clear did not use the bodyless membership contract")
	}
	beforeWrites := writes.Load()
	loseNextReply.Store(true)
	result := client.Execute(context.Background(), fleetclient.Request{Operation: "space", Ref: createdRefs[0], Space: label})
	if result.OK || result.Error == nil || result.Error.Dispatch != "unknown" || writes.Load() != beforeWrites+1 {
		t.Fatal("lost network acknowledgement was retried or treated as definite rejection")
	}
	info := client.Execute(context.Background(), fleetclient.Request{Operation: "info", Ref: createdRefs[0]})
	var observed struct {
		Session fleetclient.Session `json:"session"`
	}
	if !info.OK || json.Unmarshal(info.Value, &observed) != nil || observed.Session.Space != label {
		t.Fatal("later info did not report current membership after uncertain assignment")
	}
	if writes.Load() != beforeWrites+1 {
		t.Fatal("observation replayed uncertain membership")
	}
	secondTLS.Close()
	missing, _ := space.Parse("not-observed")
	missingFilter, _ := space.NamedFilter(missing)
	list = client.Execute(context.Background(), fleetclient.Request{Operation: "list", SpaceFilter: missingFilter})
	if !list.OK || json.Unmarshal(list.Value, &inventory) != nil || !inventory.Partial || len(inventory.Peers) != 2 {
		t.Fatal("zero-match filter hid unavailable peer")
	}
	if !inventory.Peers[0].OK || inventory.Peers[0].Sessions == nil || len(inventory.Peers[0].Sessions) != 0 || inventory.Peers[1].OK {
		t.Fatal("filtered peer JSON lost source availability or required empty array")
	}
}
