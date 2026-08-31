//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/coder/websocket"
)

const secondIntegrationMachineText = "mh-fedcba9876543210fedcba9876543210"

func TestMachineBoundGatewaysKeepCollidingLocalSessionsIndependent(t *testing.T) {
	left := newSessionFixture(t)
	right := newSessionFixture(t)
	leftServer, leftBearer := newMachineGateway(t, left, integrationMachineText)
	rightServer, rightBearer := newMachineGateway(t, right, secondIntegrationMachineText)

	create := func(server *httptest.Server, bearer, handle, cwd string) map[string]any {
		t.Helper()
		body, err := json.Marshal(map[string]string{
			"cwd":              cwd,
			"profile":          "personal",
			"optionalTmuxName": "same-local-agent",
		})
		if err != nil {
			t.Fatal("encode colliding local create")
		}
		response := requestForMachine(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, handle, "", string(body))
		assertStatus(t, response, http.StatusCreated)
		return decodeCreateSessionResponse(t, response)
	}

	leftCreated := create(leftServer, leftBearer, integrationMachineText, left.project)
	rightCreated := create(rightServer, rightBearer, secondIntegrationMachineText, right.project)
	idsCollide := leftCreated["tmuxId"] == rightCreated["tmuxId"]
	namesCollide := leftCreated["tmuxName"] == rightCreated["tmuxName"]
	if !idsCollide || !namesCollide {
		t.Fatalf("fixture did not establish colliding machine-local identities: id_collision=%t name_collision=%t", idsCollide, namesCollide)
	}
	leftTopologyBefore := left.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{pane_pid}|#{@skid_profile}")
	rightTopologyBefore := right.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{pane_pid}|#{@skid_profile}")

	unauthenticatedWrongMachine := requestForMachine(
		t,
		leftServer.Client(),
		http.MethodPost,
		leftServer.URL+"/v1/sessions",
		"",
		secondIntegrationMachineText,
		"",
		fmt.Sprintf(`{"cwd":%q,"profile":"personal","optionalTmuxName":"must-not-exist"}`, left.project),
	)
	assertError(t, unauthenticatedWrongMachine, http.StatusUnauthorized, "Unauthenticated")
	leftBearerAtRight := requestForMachine(
		t, rightServer.Client(), http.MethodGet, rightServer.URL+"/v1/sessions", leftBearer, secondIntegrationMachineText, "", "",
	)
	assertError(t, leftBearerAtRight, http.StatusUnauthorized, "Unauthenticated")
	rightBearerAtLeft := requestForMachine(
		t, leftServer.Client(), http.MethodGet, leftServer.URL+"/v1/sessions", rightBearer, integrationMachineText, "", "",
	)
	assertError(t, rightBearerAtLeft, http.StatusUnauthorized, "Unauthenticated")

	missingMachine := requestForMachine(
		t, leftServer.Client(), http.MethodPost, leftServer.URL+"/v1/sessions", leftBearer, "", "",
		fmt.Sprintf(`{"cwd":%q,"profile":"personal","optionalTmuxName":"must-not-exist"}`, left.project),
	)
	assertError(t, missingMachine, http.StatusConflict, "MachineIdentityMismatch")
	for _, headers := range [][]string{
		{integrationMachineText, integrationMachineText},
		{integrationMachineText + ", " + integrationMachineText},
	} {
		repeatedMachine := requestWithMachineHeaders(
			t,
			leftServer.Client(),
			http.MethodPost,
			leftServer.URL+"/v1/sessions",
			leftBearer,
			headers,
			fmt.Sprintf(`{"cwd":%q,"profile":"personal","optionalTmuxName":"must-not-exist"}`, left.project),
		)
		assertError(t, repeatedMachine, http.StatusBadRequest, "InvalidRequest")
	}

	wrongMachineRead := requestForMachine(
		t, leftServer.Client(), http.MethodGet, leftServer.URL+"/v1/sessions", leftBearer, secondIntegrationMachineText, "", "",
	)
	assertError(t, wrongMachineRead, http.StatusConflict, "MachineIdentityMismatch")
	wrongMachineCreate := requestForMachine(
		t,
		leftServer.Client(),
		http.MethodPost,
		leftServer.URL+"/v1/sessions",
		leftBearer,
		secondIntegrationMachineText,
		"",
		fmt.Sprintf(`{"cwd":%q,"profile":"personal","optionalTmuxName":"must-not-exist"}`, left.project),
	)
	assertError(t, wrongMachineCreate, http.StatusConflict, "MachineIdentityMismatch")
	wrongMachinePressure := requestForMachine(
		t, leftServer.Client(), http.MethodGet, leftServer.URL+"/v1/pressure", leftBearer, secondIntegrationMachineText, "", "",
	)
	assertError(t, wrongMachinePressure, http.StatusConflict, "MachineIdentityMismatch")
	wrongTerminalURL := "ws" + leftServer.URL[len("http"):] + "/v1/sessions/" + url.PathEscape(leftCreated["tmuxId"].(string)) + "/terminal"
	_, terminalResponse, terminalErr := websocket.Dial(context.Background(), wrongTerminalURL, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization":                []string{"Bearer " + leftBearer},
		"Skidbladnir-Machine":          []string{secondIntegrationMachineText},
		"Skidbladnir-Session-Identity": []string{leftCreated["identityToken"].(string)},
	}})
	if terminalErr == nil || terminalResponse == nil {
		t.Fatal("wrong-machine terminal did not return an HTTP rejection")
	}
	assertError(t, terminalResponse, http.StatusConflict, "MachineIdentityMismatch")

	wrongMachineDelete := requestForMachine(
		t,
		leftServer.Client(),
		http.MethodDelete,
		leftServer.URL+"/v1/sessions/"+url.PathEscape(leftCreated["tmuxId"].(string)),
		leftBearer,
		secondIntegrationMachineText,
		"",
		fmt.Sprintf(`{"tmuxName":%q,"identityToken":%q}`, leftCreated["tmuxName"], leftCreated["identityToken"]),
	)
	assertError(t, wrongMachineDelete, http.StatusConflict, "MachineIdentityMismatch")
	if leftTopologyAfter := left.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{pane_pid}|#{@skid_profile}"); leftTopologyAfter != leftTopologyBefore {
		t.Fatal("wrong-machine requests mutated the left tmux topology")
	}
	if rightTopologyAfter := right.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_name}|#{pane_pid}|#{@skid_profile}"); rightTopologyAfter != rightTopologyBefore {
		t.Fatal("cross-bearer requests mutated the right tmux topology")
	}
	leftInventory := readMachineInventory(t, leftServer, leftBearer, integrationMachineText)
	rightInventory := readMachineInventory(t, rightServer, rightBearer, secondIntegrationMachineText)
	findSession(t, leftInventory, "same-local-agent")
	findSession(t, rightInventory, "same-local-agent")

	exactDelete := requestForMachine(
		t,
		leftServer.Client(),
		http.MethodDelete,
		leftServer.URL+"/v1/sessions/"+url.PathEscape(leftCreated["tmuxId"].(string)),
		leftBearer,
		integrationMachineText,
		"",
		fmt.Sprintf(`{"tmuxName":%q,"identityToken":%q}`, leftCreated["tmuxName"], leftCreated["identityToken"]),
	)
	assertStatus(t, exactDelete, http.StatusNoContent)
	exactDelete.Body.Close()
	leftInventory = readMachineInventory(t, leftServer, leftBearer, integrationMachineText)
	rightInventory = readMachineInventory(t, rightServer, rightBearer, secondIntegrationMachineText)
	if hasSessionNamed(leftInventory, "same-local-agent") {
		t.Fatalf("exact left-machine kill left its target behind: session_count=%d", len(leftInventory["sessions"].([]any)))
	}
	findSession(t, rightInventory, "same-local-agent")
}

func requestWithMachineHeaders(
	t *testing.T,
	client *http.Client,
	method string,
	target string,
	bearer string,
	machineHeaders []string,
	body string,
) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal("build repeated-machine-header request")
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set("Content-Type", "application/json")
	for _, header := range machineHeaders {
		request.Header.Add("Skidbladnir-Machine", header)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("perform repeated-machine-header request")
	}
	return response
}

func newMachineGateway(t *testing.T, fixture sessionFixture, handleText string) (*httptest.Server, string) {
	t.Helper()
	handle, err := machine.Parse(handleText)
	if err != nil {
		t.Fatal("parse integration machine handle")
	}
	bearerPath := filepath.Join(fixture.root, "machine-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint machine-scoped bearer")
	}
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  handle,
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)
	return server, bearer
}

func readMachineInventory(t *testing.T, server *httptest.Server, bearer, handle string) map[string]any {
	t.Helper()
	response := requestForMachine(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, handle, "", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	machineEnvelope := inventory["machine"].(map[string]any)
	handleMatches := machineEnvelope["handle"] == handle
	platformMatches := machineEnvelope["platform"] == string(platform.Current().Kind)
	if !handleMatches || !platformMatches {
		t.Fatalf("machine inventory envelope mismatch: handle_match=%t platform_match=%t field_count=%d", handleMatches, platformMatches, len(machineEnvelope))
	}
	return inventory
}

func hasSessionNamed(inventory map[string]any, name string) bool {
	for _, value := range inventory["sessions"].([]any) {
		if value.(map[string]any)["tmuxName"] == name {
			return true
		}
	}
	return false
}
