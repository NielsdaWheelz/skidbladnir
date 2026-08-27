//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestBearerRemintRevokesThePreviousCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bearer")
	first, err := auth.Mint(auth.MintOptions{Path: path})
	if err != nil {
		t.Fatalf("mint first bearer: %v", err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("first bearer shape mismatch: decode_ok=%t decoded_bytes=%d", err == nil, len(decoded))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bearer file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("bearer file mode = %04o, want 0600", got)
	}

	verifier := auth.FileVerifier{Path: path}
	assertBearerResult(t, verifier, "Bearer "+first, true)
	assertBearerResult(t, verifier, first, false)
	assertBearerResult(t, verifier, "Bearer malformed", false)

	second, err := auth.Mint(auth.MintOptions{Path: path})
	if err != nil {
		t.Fatalf("re-mint bearer: %v", err)
	}
	if second == first {
		t.Fatal("re-mint returned the previous bearer")
	}
	assertBearerResult(t, verifier, "Bearer "+first, false)
	assertBearerResult(t, verifier, "Bearer "+second, true)
}

func TestPressureMonitorSamplesTheHost(t *testing.T) {
	monitor := pressure.NewMonitor()
	snapshot := monitor.Snapshot()
	if len(snapshot.Window) != 1 || snapshot.Current.ObservedAt.IsZero() {
		t.Fatalf("monitor initial sample mismatch: window_count=%d observed_at_present=%t", len(snapshot.Window), !snapshot.Current.ObservedAt.IsZero())
	}
	if platform.Current().Kind == platform.KindLinux && snapshot.Current.MemoryAvailablePercent.Status == pressure.StatusUnknown {
		t.Fatalf("Linux host memory availability status=%s", snapshot.Current.MemoryAvailablePercent.Status)
	}
	if platform.Current().Kind == platform.KindDarwin && snapshot.Current.MemoryPressure.Status == pressure.StatusUnknown {
		t.Fatalf("Darwin host memory pressure status=%s", snapshot.Current.MemoryPressure.Status)
	}
	if snapshot.Current.DiskAvailablePercent.Status == pressure.StatusUnknown {
		t.Fatalf("host statfs status=%s", snapshot.Current.DiskAvailablePercent.Status)
	}
}

func TestAuthenticatedGatewayControlsRealTmuxAndExposesHostPressure(t *testing.T) {
	testRoot := t.TempDir()
	socketName := randomTmuxSocketName(t, "skid-gateway")
	socketPath := namedTmuxSocketPath(socketName)
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("isolated tmux socket unexpectedly exists before test: absent=%t", os.IsNotExist(err))
	}
	agentCommand := filepath.Join(testRoot, "agent-command")
	if err := os.WriteFile(agentCommand, []byte("#!/bin/sh\nexec /bin/sleep 300\n"), 0o700); err != nil {
		t.Fatalf("write test agent command: %v", err)
	}
	for _, home := range []string{"personal", "work", "work2", "claude-work"} {
		if err := os.Mkdir(filepath.Join(testRoot, home), 0o700); err != nil {
			t.Fatalf("create %s profile home: %v", home, err)
		}
	}
	tmuxWrapper := filepath.Join(testRoot, "gateway-tmux")
	degradedReadRecord := filepath.Join(testRoot, "degraded-read-record")
	reverseInventoryMarker := filepath.Join(testRoot, "reverse-inventory")
	tmuxScript := fmt.Sprintf(`#!/bin/sh
set -eu
socket=%s
tmux_real=%s
degraded_read_record=%s
reverse_inventory_marker=%s
is_list=0
is_profile_read=0
target=
previous=
for argument in "$@"; do
  if [ "$argument" = list-sessions ]; then
    is_list=1
  fi
  if [ "$argument" = @skid_profile ]; then
    is_profile_read=1
  fi
  if [ "$previous" = -t ]; then
    target=$argument
  fi
  previous=$argument
done
if [ "$is_list" -eq 1 ] && [ -e "$reverse_inventory_marker" ]; then
  output=$("$tmux_real" "$@") || exit $?
  if [ -n "$output" ]; then
    printf '%%s\n' "$output" | /usr/bin/awk '{ lines[NR] = $0 } END { for (line = NR; line > 0; line--) print lines[line] }'
  fi
  exit 0
fi
if [ "$is_profile_read" -eq 1 ] && [ -n "$target" ] && [ ! -e "$degraded_read_record" ]; then
  name=$("$tmux_real" -L "$socket" -f /dev/null display-message -p -t "$target" '#{session_name}') || exec "$tmux_real" "$@"
  if [ "$name" = degraded-create ]; then
    printf '%%s\n' "$target" > "$degraded_read_record"
    exit 1
  fi
fi
exec "$tmux_real" "$@"
`, shellQuote(socketName), shellQuote(tmuxPath), shellQuote(degradedReadRecord), shellQuote(reverseInventoryMarker))
	if err := os.WriteFile(tmuxWrapper, []byte(tmuxScript), 0o700); err != nil {
		t.Fatal("write gateway tmux wrapper")
	}
	managerConfig := sessions.Config{
		TmuxPath:      tmuxWrapper,
		SocketName:    socketName,
		Home:          testRoot,
		CataloguePath: filepath.Join(repositoryRoot(t), "catalog", "characters.json"),
		Profiles: []sessions.Profile{
			gatewayTestProfile("personal", "Codex · Personal", agentCommand, filepath.Join(testRoot, "personal")),
			gatewayTestProfile("work", "Codex · Work", agentCommand, filepath.Join(testRoot, "work")),
			gatewayTestProfile("work2", "Codex · Work 2", agentCommand, filepath.Join(testRoot, "work2")),
			{
				Key:     "claude-work",
				Label:   "Claude · Work",
				Command: agentCommand,
				Environment: []sessions.EnvironmentVariable{
					{Name: "CLAUDE_CONFIG_DIR", Value: filepath.Join(testRoot, "claude-work")},
				},
				ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
				Arguments:            []string{"--permission-mode", "auto"},
			},
		},
	}
	manager, err := sessions.New(managerConfig)
	if err != nil {
		t.Fatalf("create sessions manager: %v", err)
	}
	if output, err := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "new-session", "-d", "-s", "laptop", "-c", testRoot, sleepPath, "300").CombinedOutput(); err != nil {
		t.Fatalf("create laptop tmux session: output_bytes=%d", len(output))
	}
	serverIdentity := captureTestTmuxServer(t, tmuxPath, socketPath)
	t.Cleanup(func() {
		stopTestTmuxServer(t, tmuxPath, socketPath, serverIdentity)
	})

	bearerPath := filepath.Join(testRoot, "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint gateway bearer: %v", err)
	}
	firstBearer := bearer
	monitor := pressure.NewMonitor()
	var logs bytes.Buffer
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: manager,
		Pressure: monitor,
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Logger:   logging.New(&logs),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)

	response := request(t, server.Client(), http.MethodGet, server.URL+"/healthz", "", "", "")
	assertStatus(t, response, http.StatusOK)
	response.Body.Close()
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", "", "", "")
	if got := response.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatal("unauthenticated response omitted the Bearer challenge")
	}
	assertError(t, response, http.StatusUnauthorized, "Unauthenticated")
	response = requestForMachine(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "", "")
	assertStatus(t, response, http.StatusOK)
	pairingInventory := decodeObject(t, response)
	machineEnvelope := pairingInventory["machine"].(map[string]any)
	handleMatches := machineEnvelope["handle"] == integrationMachineText
	platformMatches := machineEnvelope["platform"] == string(platform.Current().Kind)
	if !handleMatches || !platformMatches {
		t.Fatalf("pairing inventory machine envelope mismatch: handle_match=%t platform_match=%t field_count=%d", handleMatches, platformMatches, len(machineEnvelope))
	}
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", "", "niels@example.test", "")
	assertError(t, response, http.StatusUnauthorized, "Unauthenticated")
	duplicateAuthorization, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/sessions", nil)
	if err != nil {
		t.Fatalf("build duplicate-authorization request: %v", err)
	}
	duplicateAuthorization.Header.Add("Authorization", "Bearer "+bearer)
	duplicateAuthorization.Header.Add("Authorization", "Bearer "+bearer)
	response, err = server.Client().Do(duplicateAuthorization)
	if err != nil {
		t.Fatalf("perform duplicate-authorization request: %v", err)
	}
	assertError(t, response, http.StatusUnauthorized, "Unauthenticated")

	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	profiles := inventory["profiles"].([]any)
	gotProfiles := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		fields := profile.(map[string]any)
		gotProfiles = append(gotProfiles, fields["key"].(string)+"="+fields["label"].(string))
	}
	wantProfiles := []string{"personal=Codex · Personal", "work=Codex · Work", "work2=Codex · Work 2", "claude-work=Claude · Work"}
	if !reflect.DeepEqual(gotProfiles, wantProfiles) {
		t.Fatalf("advertised profile contract mismatch: got=%v want=%v", gotProfiles, wantProfiles)
	}
	laptop := findSession(t, inventory, "laptop")
	laptopID := laptop["id"].(string)
	laptopToken := laptop["identityToken"].(string)
	for _, absent := range []string{"profile", "objective"} {
		if _, exists := laptop[absent]; exists {
			t.Fatalf("laptop-created session guessed %s metadata", absent)
		}
	}
	laptopCharacter := laptop["character"].(map[string]any)
	if laptopCharacter["key"] == "" || laptopCharacter["displayName"] == "" {
		t.Fatal("laptop-created session omitted its required normalized character")
	}
	server.Close()
	reconstructedManager, err := sessions.New(managerConfig)
	if err != nil {
		t.Fatal("reconstruct sessions manager")
	}
	server = httptest.NewServer(gateway.New(gateway.Config{
		Sessions: reconstructedManager,
		Pressure: monitor,
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Logger:   logging.New(&logs),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	reconstructedLaptop := findSession(t, inventory, "laptop")
	if reconstructedLaptop["id"] != laptopID || reconstructedLaptop["identityToken"] != laptopToken ||
		!reflect.DeepEqual(reconstructedLaptop["character"], laptopCharacter) {
		t.Fatalf(
			"gateway reconstruction changed persisted laptop identity: id_match=%t token_match=%t character_match=%t",
			reconstructedLaptop["id"] == laptopID,
			reconstructedLaptop["identityToken"] == laptopToken,
			reflect.DeepEqual(reconstructedLaptop["character"], laptopCharacter),
		)
	}

	before := len(inventory["sessions"].([]any))
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"/definitely/not/a/skidbladnir/directory","profile":"personal"}`)
	assertError(t, response, http.StatusUnprocessableEntity, "WorkingDirectoryUnavailable")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"not-allowlisted"}`)
	assertError(t, response, http.StatusUnprocessableEntity, "ProfileUnknown")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	if got := len(decodeObject(t, response)["sessions"].([]any)); got != before {
		t.Fatalf("rejected creates changed tmux inventory: before=%d after=%d", before, got)
	}

	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"personal","unknown":true}`)
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"personal"} {}`)
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"profile":"personal"}`)
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"personal","optionalTmuxName":null}`)
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"personal","optionalTmuxName":""}`)
	assertError(t, response, http.StatusUnprocessableEntity, "SessionNameInvalid")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", `{"cwd":"`+testRoot+`","profile":"personal","objective":""}`)
	assertError(t, response, http.StatusUnprocessableEntity, "ObjectiveInvalid")
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", strings.Repeat("x", int(gateway.MaximumBodyBytes)+1))
	assertError(t, response, http.StatusRequestEntityTooLarge, "RequestTooLarge")

	createBody, err := json.Marshal(map[string]string{"cwd": testRoot, "profile": "claude-work", "optionalTmuxName": "gateway-test", "objective": "Prove the control plane"})
	if err != nil {
		t.Fatalf("encode create request: %v", err)
	}
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", string(createBody))
	assertStatus(t, response, http.StatusCreated)
	created := decodeObject(t, response)
	if created["tmuxName"] != "gateway-test" || created["profile"] != "claude-work" || created["objective"] != "Prove the control plane" {
		t.Fatal("created card did not preserve the requested name, profile, and objective")
	}
	character := created["character"].(map[string]any)
	if character["key"] == "" || character["displayName"] == "" {
		t.Fatal("created card omitted character metadata")
	}
	createdID := created["id"].(string)
	createdToken := created["identityToken"].(string)
	if len(createdToken) < 4 {
		t.Fatal("created card omitted its lifetime identity token")
	}
	degradedLogsStart := logs.Len()
	degradedBody, err := json.Marshal(map[string]string{
		"cwd": testRoot, "profile": "personal", "optionalTmuxName": "degraded-create",
	})
	if err != nil {
		t.Fatal("encode degraded create request")
	}
	response = request(t, server.Client(), http.MethodPost, server.URL+"/v1/sessions", bearer, "niels@example.test", string(degradedBody))
	assertStatus(t, response, http.StatusCreated)
	degraded := decodeObject(t, response)
	degradedStatus := degraded["status"].(map[string]any)
	degradedCharacter := degraded["character"].(map[string]any)
	if degraded["tmuxName"] != "degraded-create" || degradedStatus["kind"] != "Unknown" ||
		degradedStatus["signal"] != "PollFailure" || degradedCharacter["key"] == "" || degradedCharacter["displayName"] == "" {
		t.Fatalf(
			"degraded create stable-card mismatch: name_match=%t status_kind=%v status_signal=%v character_complete=%t",
			degraded["tmuxName"] == "degraded-create",
			degradedStatus["kind"],
			degradedStatus["signal"],
			degradedCharacter["key"] != "" && degradedCharacter["displayName"] != "",
		)
	}
	if _, exists := degraded["profile"]; exists {
		t.Fatal("degraded create guessed unavailable profile metadata")
	}
	degradedID := degraded["id"].(string)
	degradedToken := degraded["identityToken"].(string)
	recordedID, err := os.ReadFile(degradedReadRecord)
	if err != nil || strings.TrimSpace(string(recordedID)) != degradedID {
		t.Fatalf("degraded create did not cross the injected read failure: record_read=%t id_match=%t", err == nil, strings.TrimSpace(string(recordedID)) == degradedID)
	}
	if !strings.Contains(logs.String()[degradedLogsStart:], "personal") {
		t.Fatal("degraded create log lost the requested profile")
	}
	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(degradedID), bearer, "niels@example.test",
		fmt.Sprintf(`{"tmuxName":"degraded-create","identityToken":%q}`, degradedToken))
	assertStatus(t, response, http.StatusNoContent)
	response.Body.Close()

	secondBearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("rotate gateway bearer: %v", err)
	}
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertError(t, response, http.StatusUnauthorized, "Unauthenticated")
	bearer = secondBearer

	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(createdID), bearer, "niels@example.test", `{"tmuxName":"gateway-test"}`)
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")
	staleToken := createdToken
	if createdToken[3] == '0' {
		staleToken = createdToken[:3] + "1" + createdToken[4:]
	} else {
		staleToken = createdToken[:3] + "0" + createdToken[4:]
	}
	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(createdID), bearer, "niels@example.test", fmt.Sprintf(`{"tmuxName":"gateway-test","identityToken":%q}`, staleToken))
	assertError(t, response, http.StatusConflict, "SessionIdentityMismatch")

	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(createdID), bearer, "niels@example.test", fmt.Sprintf(`{"tmuxName":"laptop","identityToken":%q}`, createdToken))
	assertError(t, response, http.StatusConflict, "SessionIdentityMismatch")
	for _, fixture := range []struct {
		name      string
		command   string
		arguments []string
	}{
		{name: "zulu-running", command: sleepPath, arguments: []string{"300"}},
		{name: "aardvark-shell", command: "/usr/bin/tail", arguments: []string{"-f", "/dev/null"}},
		{name: "zulu-shell", command: "/usr/bin/tail", arguments: []string{"-f", "/dev/null"}},
	} {
		arguments := []string{"-L", socketName, "-f", "/dev/null", "new-session", "-d", "-s", fixture.name, "-c", testRoot, "--", fixture.command}
		arguments = append(arguments, fixture.arguments...)
		if output, commandErr := isolatedTmuxCommand(tmuxPath, arguments...).CombinedOutput(); commandErr != nil {
			t.Fatalf("create wire-order fixture: name=%s output_bytes=%d", fixture.name, len(output))
		}
	}
	if err := os.WriteFile(reverseInventoryMarker, []byte("reverse\n"), 0o600); err != nil {
		t.Fatal("arm reversed manager inventory fixture")
	}
	attentiveShellID, err := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "display-message", "-p", "-t", "zulu-shell", "#{session_id}").Output()
	if err != nil || strings.TrimSpace(string(attentiveShellID)) == "" {
		t.Fatalf("resolve attentive shell fixture: command_ok=%t id_present=%t", err == nil, strings.TrimSpace(string(attentiveShellID)) != "")
	}
	if output, commandErr := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "set-option", "-p", "-t", strings.TrimSpace(string(attentiveShellID)),
		"--", "@skid_attention", fmt.Sprint(time.Now().Unix())).CombinedOutput(); commandErr != nil {
		t.Fatalf("make lower-priority shell fixture attentive: output_bytes=%d", len(output))
	}
	statusKind := func(card map[string]any) string {
		return card["status"].(map[string]any)["kind"].(string)
	}
	deadline := time.Now().Add(tmuxConvergenceTimeout)
	var attentiveShell, runningAlpha, runningMiddle, runningOmega, shellAlpha map[string]any
	for {
		response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
		assertStatus(t, response, http.StatusOK)
		inventory = decodeObject(t, response)
		attentiveShell = findSession(t, inventory, "zulu-shell")
		runningAlpha = findSession(t, inventory, "gateway-test")
		runningMiddle = findSession(t, inventory, "laptop")
		runningOmega = findSession(t, inventory, "zulu-running")
		shellAlpha = findSession(t, inventory, "aardvark-shell")
		if attentiveShell["attention"].(bool) && statusKind(attentiveShell) == "Shell" &&
			statusKind(runningAlpha) == "Running" && statusKind(runningMiddle) == "Running" &&
			statusKind(runningOmega) == "Running" && statusKind(shellAlpha) == "Shell" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"wire-order fixtures did not converge: attentive=%t statuses=%s/%s/%s/%s/%s",
				attentiveShell["attention"].(bool),
				statusKind(attentiveShell),
				statusKind(runningAlpha),
				statusKind(runningMiddle),
				statusKind(runningOmega),
				statusKind(shellAlpha),
			)
		}
		time.Sleep(tmuxConvergencePollInterval)
	}
	wireOrder := make([]string, 0, len(inventory["sessions"].([]any)))
	for _, value := range inventory["sessions"].([]any) {
		wireOrder = append(wireOrder, value.(map[string]any)["tmuxName"].(string))
	}
	wantWireOrder := []string{"zulu-shell", "gateway-test", "laptop", "zulu-running", "aardvark-shell"}
	if !reflect.DeepEqual(wireOrder, wantWireOrder) {
		t.Fatalf("gateway wire order mismatch: got=%v want=%v", wireOrder, wantWireOrder)
	}
	if err := os.Remove(reverseInventoryMarker); err != nil {
		t.Fatal("disarm reversed manager inventory fixture")
	}
	for _, card := range []map[string]any{attentiveShell, runningOmega, shellAlpha} {
		id := card["id"].(string)
		if output, commandErr := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "kill-session", "-t", id).CombinedOutput(); commandErr != nil {
			t.Fatalf("remove exact wire-order fixture: output_bytes=%d", len(output))
		}
	}
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	findSession(t, inventory, "laptop")
	findSession(t, inventory, "gateway-test")

	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(createdID), bearer, "niels@example.test", fmt.Sprintf(`{"tmuxName":"gateway-test","identityToken":%q}`, createdToken))
	assertStatus(t, response, http.StatusNoContent)
	response.Body.Close()
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	findSession(t, inventory, "laptop")
	if got := len(inventory["sessions"].([]any)); got != 1 {
		t.Fatalf("exact kill left %d sessions, want laptop only", got)
	}

	const groupedPeerName = "laptop-group-peer"
	if output, err := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "new-session", "-d", "-t", laptopID, "-s", groupedPeerName).CombinedOutput(); err != nil {
		t.Fatalf("create ordinary grouped peer: output_bytes=%d", len(output))
	}
	sharedPanePID, err := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "display-message", "-p", "-t", laptopID, "#{pane_pid}").Output()
	if err != nil || strings.TrimSpace(string(sharedPanePID)) == "" {
		t.Fatalf("capture grouped target pane PID: command_ok=%t output_present=%t", err == nil, strings.TrimSpace(string(sharedPanePID)) != "")
	}
	response = request(t, server.Client(), http.MethodDelete, server.URL+"/v1/sessions/"+url.PathEscape(laptopID), bearer, "niels@example.test", fmt.Sprintf(`{"tmuxName":"laptop","identityToken":%q}`, laptopToken))
	assertError(t, response, http.StatusConflict, "SessionGroupedConflict")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	unchangedLaptop := findSession(t, inventory, "laptop")
	findSession(t, inventory, groupedPeerName)
	idUnchanged := unchangedLaptop["id"] == laptopID
	identityUnchanged := unchangedLaptop["identityToken"] == laptopToken
	if !idUnchanged || !identityUnchanged {
		t.Fatalf("refused grouped HTTP kill changed target identity: id_unchanged=%t identity_unchanged=%t", idUnchanged, identityUnchanged)
	}
	afterPanePID, err := isolatedTmuxCommand(tmuxPath, "-L", socketName, "-f", "/dev/null", "display-message", "-p", "-t", laptopID, "#{pane_pid}").Output()
	if err != nil || strings.TrimSpace(string(afterPanePID)) != strings.TrimSpace(string(sharedPanePID)) {
		t.Fatalf("refused grouped HTTP kill changed shared pane: command_ok=%t identity_unchanged=%t", err == nil, strings.TrimSpace(string(afterPanePID)) == strings.TrimSpace(string(sharedPanePID)))
	}

	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/pressure", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	pressureBody := decodeObject(t, response)
	history := pressureBody["history"].([]any)
	if len(history) == 0 || len(history) > pressure.HistorySampleLimit || !reflect.DeepEqual(history[len(history)-1], pressureBody["current"]) {
		t.Fatalf("pressure history/current mismatch: history_count=%d current_matches_last=%t", len(history), len(history) > 0 && reflect.DeepEqual(history[len(history)-1], pressureBody["current"]))
	}
	metrics := pressureBody["current"].(map[string]any)["metrics"].(map[string]any)
	unsupported := pressureBody["unsupported"].([]any)
	if platform.Current().Kind == platform.KindLinux {
		_, memoryAvailableExists := metrics["memoryAvailablePercent"]
		memoryPressureUnsupported := containsString(unsupported, "memoryPressure")
		if !memoryAvailableExists || !memoryPressureUnsupported {
			t.Fatalf("Linux pressure capability contract is incomplete: memory_available_present=%t memory_pressure_unsupported=%t", memoryAvailableExists, memoryPressureUnsupported)
		}
	} else {
		_, memoryPressureExists := metrics["memoryPressure"]
		unsupportedComplete := containsString(unsupported, "memoryAvailablePercent") &&
			containsString(unsupported, "cpuPsiSomeAvg60Percent") && containsString(unsupported, "memoryPsiFullAvg60Percent") &&
			containsString(unsupported, "ioPsiFullAvg60Percent")
		if !memoryPressureExists || !unsupportedComplete {
			t.Fatalf("Darwin pressure capability contract is incomplete: memory_pressure_present=%t unsupported_complete=%t", memoryPressureExists, unsupportedComplete)
		}
	}
	if _, exists := metrics["cpuPercent"]; exists {
		t.Fatal("first pressure sample encoded an unavailable CPU delta")
	}
	missing := pressureBody["current"].(map[string]any)["missing"].([]any)
	if !containsString(missing, "cpuPercent") {
		t.Fatalf("first pressure sample did not name omitted cpuPercent: missing_count=%d", len(missing))
	}
	logOutput := logs.String()
	for _, forbidden := range []struct {
		name  string
		value string
	}{
		{name: "original bearer", value: firstBearer},
		{name: "current bearer", value: bearer},
		{name: "session identity", value: createdToken},
		{name: "working directory", value: testRoot},
		{name: "objective", value: "Prove the control plane"},
		{name: "session-specific route", value: "/v1/sessions/" + createdID},
	} {
		if strings.Contains(logOutput, forbidden.value) {
			t.Fatalf("gateway log leaked forbidden %s", forbidden.name)
		}
	}
	for _, eventName := range []string{"Authentication.Rejected", "Sessions.Listed", "Session.Created", "Session.Killed", "Pressure.Sampled", "Request.Completed"} {
		if !strings.Contains(logOutput, `"event.name":"`+eventName+`"`) {
			t.Fatalf("gateway log omitted required event %s", eventName)
		}
	}
}

func gatewayTestProfile(key, label, command, codexHome string) sessions.Profile {
	return sessions.Profile{
		Key:     key,
		Label:   label,
		Command: command,
		Environment: []sessions.EnvironmentVariable{
			{Name: "CODEX_HOME", Value: codexHome},
		},
		ForegroundSignatures: []sessions.ForegroundSignature{{ExecutableBase: "sleep"}},
		Arguments:            []string{"--dangerously-bypass-approvals-and-sandbox"},
	}
}

func assertBearerResult(t *testing.T, verifier auth.FileVerifier, authorization string, want bool) {
	t.Helper()
	got, err := verifier.Verify(authorization)
	if err != nil {
		t.Fatalf("verify bearer authorization: %v", err)
	}
	if got != want {
		t.Fatalf("bearer authorization result = %t, want %t", got, want)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test filename")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func request(t *testing.T, client *http.Client, method, target, bearer, tailnetLogin, body string) *http.Response {
	t.Helper()
	machineHandle := ""
	if bearer != "" {
		machineHandle = integrationMachineText
	}
	return requestForMachine(t, client, method, target, bearer, machineHandle, tailnetLogin, body)
}

func requestForMachine(t *testing.T, client *http.Client, method, target, bearer, machineHandle, tailnetLogin, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, target, reader)
	if err != nil {
		t.Fatalf("build request: method=%s", method)
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	if machineHandle != "" {
		request.Header.Set("Skidbladnir-Machine", machineHandle)
	}
	if tailnetLogin != "" {
		request.Header.Set("Tailscale-User-Login", tailnetLogin)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("perform request: method=%s", method)
	}
	return response
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("HTTP status = %d, want %d; response_body_bytes=%d", response.StatusCode, want, len(body))
	}
}

func assertError(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	assertStatus(t, response, status)
	body := decodeObject(t, response)
	messages := map[string]string{
		"Unauthenticated":             "Authentication required.",
		"InvalidRequest":              "The request is not valid.",
		"RequestTooLarge":             "The request is too large.",
		"WorkingDirectoryUnavailable": "That directory does not exist or cannot be opened.",
		"ProfileUnknown":              "Choose an available profile.",
		"SessionNameInvalid":          "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.",
		"ObjectiveInvalid":            "Use 1–240 characters without terminal controls.",
		"SessionIdentityMismatch":     "The session changed. Refresh before killing it.",
		"MachineIdentityMismatch":     "The machine identity changed. Provisioning repair is required.",
		"SessionGroupedConflict":      "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.",
	}
	wantMessage, found := messages[code]
	if !found {
		t.Fatalf("test has no literal message for error code %s", code)
	}
	if len(body) != 2 || body["code"] != code || body["message"] != wantMessage {
		t.Fatalf(
			"error response did not match exact contract: field_count=%d code_match=%t message_match=%t",
			len(body),
			body["code"] == code,
			body["message"] == wantMessage,
		)
	}
}

const integrationMachineText = "mh-0123456789abcdef0123456789abcdef"

func integrationMachine(t *testing.T) machine.Handle {
	t.Helper()
	handle, err := machine.Parse(integrationMachineText)
	if err != nil {
		t.Fatalf("parse fixed integration machine handle: %v", err)
	}
	return handle
}

func decodeObject(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode HTTP response: status=%d", response.StatusCode)
	}
	return body
}

func findSession(t *testing.T, inventory map[string]any, name string) map[string]any {
	t.Helper()
	sessions := inventory["sessions"].([]any)
	for _, value := range sessions {
		session := value.(map[string]any)
		if session["tmuxName"] == name {
			return session
		}
	}
	t.Fatalf("expected session not found in inventory: session_count=%d", len(sessions))
	return nil
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
