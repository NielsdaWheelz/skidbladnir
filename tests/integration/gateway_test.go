//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/coder/websocket"
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
	if _, known := snapshot.Current.CPUPercent.Value(); known && snapshot.Current.CPUPercent.Status != pressure.SignalStatusInformational {
		t.Fatalf("host CPU signal status=%s, want Informational", snapshot.Current.CPUPercent.Status)
	}
	if platform.Current().Kind == platform.KindLinux {
		if _, known := snapshot.Current.MemoryAvailablePercent.Value(); !known || !isThresholdSignalStatus(snapshot.Current.MemoryAvailablePercent.Status) {
			t.Fatalf("Linux host memory availability signal invalid: value_known=%t status=%s", known, snapshot.Current.MemoryAvailablePercent.Status)
		}
	}
	if platform.Current().Kind == platform.KindDarwin {
		if _, known := snapshot.Current.MemoryPressure.Value(); !known || !isThresholdSignalStatus(snapshot.Current.MemoryPressure.Status) {
			t.Fatalf("Darwin host memory pressure signal invalid: value_known=%t status=%s", known, snapshot.Current.MemoryPressure.Status)
		}
	}
	if _, known := snapshot.Current.DiskAvailablePercent.Value(); !known || !isThresholdSignalStatus(snapshot.Current.DiskAvailablePercent.Status) {
		t.Fatalf("host statfs signal invalid: value_known=%t status=%s", known, snapshot.Current.DiskAvailablePercent.Status)
	}
}

func TestAuthenticatedGatewayRenamesExactSessionInPlace(t *testing.T) {
	t.Parallel()

	fixture := newSessionFixture(t)
	ctx := context.Background()
	const (
		sourceName       = "rename-source"
		firstName        = "rename-current"
		collisionName    = "rename-existing"
		rivalName        = "rename-rival"
		disappearingName = "rename-disappearing"
	)

	createdTarget, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD:              fixture.project,
		Profile:          "claude-work",
		OptionalTmuxName: sourceName,
	})
	if err != nil {
		t.Fatal("create rename target")
	}
	target := fixture.waitForAgent(t, ctx, sourceName)
	if target.Agent.Provider != agentruntime.ProviderClaude || target.Agent.ProviderSession == nil ||
		target.Agent.ProviderSession.Name() != sourceName {
		t.Fatal("rename target did not expose its initial provider-owned name")
	}
	if createdTarget.Session.TmuxID != target.TmuxID || createdTarget.Session.IdentityToken != target.IdentityToken {
		t.Fatal("rename target changed identity while its provider runtime converged")
	}
	if _, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD: fixture.project, Profile: "personal", OptionalTmuxName: collisionName,
	}); err != nil {
		t.Fatal("create rename collision fixture")
	}

	bearerPath := filepath.Join(fixture.root, "rename-bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint rename gateway bearer")
	}
	var logs bytes.Buffer
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: fixture.manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(&logs),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)

	connection := dialTerminal(
		t,
		server.Client(),
		"ws"+strings.TrimPrefix(server.URL, "http")+"/v1/sessions/"+url.PathEscape(target.TmuxID)+"/terminal",
		bearer,
		target.IdentityToken,
	)
	requireTerminalPresence(t, connection, "Hello", 1, "Owner")
	terminalReader := startRenameTerminalReader(t, connection)

	response := request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventoryBefore := decodeObject(t, response)
	cardBefore := findSession(t, inventoryBefore, sourceName)
	tmuxBefore := renameInvariantSnapshot(t, fixture, target.TmuxID)
	renameLogsStart := logs.Len()

	response = request(
		t,
		server.Client(),
		http.MethodPatch,
		server.URL+"/v1/sessions/"+url.PathEscape(target.TmuxID),
		bearer,
		"",
		renameBody(t, sourceName, firstName, target.IdentityToken),
	)
	assertBodylessStatus(t, response, http.StatusNoContent)

	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventoryAfter := decodeObject(t, response)
	cardAfter := findSession(t, inventoryAfter, firstName)
	assertSessionCardPreservesRenameInvariants(t, cardBefore, cardAfter, firstName)
	if providerSessionName(cardAfter) != sourceName {
		t.Fatal("tmux rename synchronized the independent provider-owned name")
	}
	if renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore {
		t.Fatal("tmux rename changed session lifetime, topology, process, client, option, selection, or geometry facts")
	}
	terminalReader.requireResponsive(t, connection)

	staleToken := target.IdentityToken
	if staleToken[len(staleToken)-1] == '0' {
		staleToken = staleToken[:len(staleToken)-1] + "1"
	} else {
		staleToken = staleToken[:len(staleToken)-1] + "0"
	}
	rejected := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "empty expected name", body: renameBody(t, "", "rename-unused", target.IdentityToken), status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "empty identity token", body: renameBody(t, firstName, "rename-unused", ""), status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "missing desired name", body: fmt.Sprintf(`{"tmuxName":%q,"identityToken":%q}`, firstName, target.IdentityToken), status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "null desired name", body: fmt.Sprintf(`{"tmuxName":%q,"newTmuxName":null,"identityToken":%q}`, firstName, target.IdentityToken), status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "wrong-typed desired name", body: fmt.Sprintf(`{"tmuxName":%q,"newTmuxName":7,"identityToken":%q}`, firstName, target.IdentityToken), status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "trailing desired-name request", body: renameBody(t, firstName, "rename-unused", target.IdentityToken) + ` {}`, status: http.StatusBadRequest, code: "InvalidRequest"},
		{name: "empty desired name", body: renameBody(t, firstName, "", target.IdentityToken), status: http.StatusUnprocessableEntity, code: "SessionNameInvalid"},
		{name: "invalid desired name", body: renameBody(t, firstName, "unsafe:name", target.IdentityToken), status: http.StatusUnprocessableEntity, code: "SessionNameInvalid"},
		{name: "same name", body: renameBody(t, firstName, firstName, target.IdentityToken), status: http.StatusConflict, code: "SessionNameConflict"},
		{name: "stale identity precedes same name", body: renameBody(t, firstName, firstName, staleToken), status: http.StatusConflict, code: "SessionIdentityMismatch"},
		{name: "destination collision", body: renameBody(t, firstName, collisionName, target.IdentityToken), status: http.StatusConflict, code: "SessionNameConflict"},
		{name: "stale expected name", body: renameBody(t, sourceName, "rename-unused", target.IdentityToken), status: http.StatusConflict, code: "SessionIdentityMismatch"},
		{name: "stale lifetime token", body: renameBody(t, firstName, "rename-unused", staleToken), status: http.StatusConflict, code: "SessionIdentityMismatch"},
		{name: "unknown field", body: fmt.Sprintf(`{"tmuxName":%q,"newTmuxName":"rename-unused","identityToken":%q,"unknown":true}`, firstName, target.IdentityToken), status: http.StatusBadRequest, code: "InvalidRequest"},
	}
	for _, test := range rejected {
		t.Run(test.name, func(t *testing.T) {
			response := request(
				t,
				server.Client(),
				http.MethodPatch,
				server.URL+"/v1/sessions/"+url.PathEscape(target.TmuxID),
				bearer,
				"",
				test.body,
			)
			assertError(t, response, test.status, test.code)
		})
	}
	if renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore {
		t.Fatal("a rejected rename mutated the exact target")
	}

	createdClassificationSource, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD: fixture.project, Profile: "personal", OptionalTmuxName: "rename-classify-source",
	})
	if err != nil {
		t.Fatal("create rename classification source")
	}
	createdClassificationDestination, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD: fixture.project, Profile: "personal", OptionalTmuxName: "rename-classify-occupied",
	})
	if err != nil {
		t.Fatal("create rename classification destination")
	}
	classificationSource := createdClassificationSource.Session
	classificationDestination := createdClassificationDestination.Session
	classificationIDs := []string{classificationSource.TmuxID, classificationDestination.TmuxID}
	t.Cleanup(func() {
		if output, unsetErr := isolatedTmuxCommand(
			tmuxPath, "-L", fixture.socket, "-f", "/dev/null", "set-hook", "-gu", "after-list-sessions",
		).CombinedOutput(); unsetErr != nil {
			t.Errorf("unset exact rename classification hook: output_bytes=%d", len(output))
		}
		for _, id := range classificationIDs {
			_, _ = isolatedTmuxCommand(
				tmuxPath, "-L", fixture.socket, "-f", "/dev/null", "kill-session", "-t", id,
			).CombinedOutput()
		}
	})
	const classificationExternalName = "rename-classify-external"
	classificationHook := "set-hook -gu after-list-sessions ; rename-session -t '" +
		classificationSource.TmuxID + "' '" + classificationExternalName + "'"
	fixture.tmux(t, "set-hook", "-g", "after-list-sessions", classificationHook)
	response = request(
		t,
		server.Client(),
		http.MethodPatch,
		server.URL+"/v1/sessions/"+url.PathEscape(classificationSource.TmuxID),
		bearer,
		"",
		renameBody(
			t,
			classificationSource.TmuxName,
			classificationDestination.TmuxName,
			classificationSource.IdentityToken,
		),
	)
	assertError(t, response, http.StatusConflict, "SessionIdentityMismatch")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	classificationInventory := decodeObject(t, response)
	classificationCard := findSession(t, classificationInventory, classificationExternalName)
	if classificationCard["tmuxId"] != classificationSource.TmuxID ||
		classificationCard["identityToken"] != classificationSource.IdentityToken {
		t.Fatal("rename classification race did not preserve the externally renamed source identity")
	}

	createdDisappearing, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD: fixture.project, Profile: "personal", OptionalTmuxName: disappearingName,
	})
	if err != nil {
		t.Fatal("create disappearing rename fixture")
	}
	disappearing := createdDisappearing.Session
	fixture.tmux(t, "kill-session", "-t", disappearing.TmuxID)
	response = request(
		t,
		server.Client(),
		http.MethodPatch,
		server.URL+"/v1/sessions/"+url.PathEscape(disappearing.TmuxID),
		bearer,
		"",
		renameBody(t, disappearingName, "rename-missing-target", disappearing.IdentityToken),
	)
	assertError(t, response, http.StatusNotFound, "SessionNotFound")
	if renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore {
		t.Fatal("a missing rename target caused collateral mutation")
	}

	const (
		firstRaceName  = "rename-first-writer"
		secondRaceName = "rename-second-writer"
	)
	sameSourceRace := runConcurrentRenameRequests(t, server.Client(), bearer, []renameHTTPRequest{
		{target: server.URL + "/v1/sessions/" + url.PathEscape(target.TmuxID), body: renameBody(t, firstName, firstRaceName, target.IdentityToken)},
		{target: server.URL + "/v1/sessions/" + url.PathEscape(target.TmuxID), body: renameBody(t, firstName, secondRaceName, target.IdentityToken)},
	})
	assertConcurrentRenameResults(t, sameSourceRace, "SessionIdentityMismatch", "The session changed. Refresh and try again.")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventoryAfter = decodeObject(t, response)
	mainAfterFirstRace := findSessionID(t, inventoryAfter, target.TmuxID)
	mainRaceName := mainAfterFirstRace["tmuxName"].(string)
	if mainRaceName != firstRaceName && mainRaceName != secondRaceName {
		t.Fatal("same-source rename race published neither requested winner")
	}
	assertSessionCardPreservesRenameInvariants(t, cardAfter, mainAfterFirstRace, mainRaceName)
	if renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore {
		t.Fatal("same-source rename race changed non-name target facts")
	}

	createdRival, err := fixture.manager.Create(ctx, sessions.CreateInput{
		CWD: fixture.project, Profile: "personal", OptionalTmuxName: rivalName,
	})
	if err != nil {
		t.Fatal("create destination-race rival")
	}
	rival := fixture.waitForAgent(t, ctx, rivalName)
	if createdRival.Session.TmuxID != rival.TmuxID || createdRival.Session.IdentityToken != rival.IdentityToken {
		t.Fatal("rename rival changed identity while its provider runtime converged")
	}
	const destinationName = "rename-one-destination"
	rivalBefore := renameInvariantSnapshot(t, fixture, rival.TmuxID)
	destinationRace := runConcurrentRenameRequests(t, server.Client(), bearer, []renameHTTPRequest{
		{target: server.URL + "/v1/sessions/" + url.PathEscape(target.TmuxID), body: renameBody(t, mainRaceName, destinationName, target.IdentityToken)},
		{target: server.URL + "/v1/sessions/" + url.PathEscape(rival.TmuxID), body: renameBody(t, rivalName, destinationName, rival.IdentityToken)},
	})
	assertConcurrentRenameResults(t, destinationRace, "SessionNameConflict", "A session with that name already exists.")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventoryAfter = decodeObject(t, response)
	mainAfterDestinationRace := findSessionID(t, inventoryAfter, target.TmuxID)
	rivalAfterDestinationRace := findSessionID(t, inventoryAfter, rival.TmuxID)
	destinationWinners := 0
	if mainAfterDestinationRace["tmuxName"] == destinationName {
		destinationWinners++
	}
	if rivalAfterDestinationRace["tmuxName"] == destinationName {
		destinationWinners++
	}
	if destinationWinners != 1 {
		t.Fatalf("destination rename race winner count=%d, want 1", destinationWinners)
	}
	mainAfterDestinationName := mainAfterDestinationRace["tmuxName"].(string)
	assertSessionCardPreservesRenameInvariants(t, mainAfterFirstRace, mainAfterDestinationRace, mainAfterDestinationName)
	if renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore || renameInvariantSnapshot(t, fixture, rival.TmuxID) != rivalBefore {
		t.Fatal("destination rename race changed non-name facts")
	}

	rivalCurrentName := rivalAfterDestinationRace["tmuxName"].(string)
	if err := fixture.manager.Kill(ctx, sessions.KillInput{
		TmuxID: rival.TmuxID, TmuxName: rivalCurrentName, IdentityToken: rival.IdentityToken,
	}); err != nil {
		t.Fatal("remove destination-race rival")
	}
	response = request(
		t,
		server.Client(),
		http.MethodPatch,
		server.URL+"/v1/sessions/"+url.PathEscape(target.TmuxID),
		bearer,
		"",
		renameBody(t, mainAfterDestinationName, sourceName, target.IdentityToken),
	)
	assertBodylessStatus(t, response, http.StatusNoContent)
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	finalInventory := decodeObject(t, response)
	finalCard := findSession(t, finalInventory, sourceName)
	assertSessionCardPreservesRenameInvariants(t, cardBefore, finalCard, sourceName)
	if providerSessionName(finalCard) != sourceName || renameInvariantSnapshot(t, fixture, target.TmuxID) != tmuxBefore {
		t.Fatal("rename restoration changed provider or non-name tmux facts")
	}
	terminalReader.requireResponsive(t, connection)

	assertContentFreePatchRequestLogs(t, logs.String()[renameLogsStart:])

	assertGatewayRenameRejectsRestartedLifetime(t)
}

func assertGatewayRenameRejectsRestartedLifetime(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(home, "project")
	profileHome := filepath.Join(home, "profile")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal("create restart project fixture")
	}
	if err := os.Mkdir(profileHome, 0o700); err != nil {
		t.Fatal("create restart profile fixture")
	}
	cataloguePath := filepath.Join(root, "catalogue.json")
	if err := os.WriteFile(cataloguePath, []byte(`[{"key":"norse.durinn","displayName":"Durinn"}]`), 0o600); err != nil {
		t.Fatal("write restart catalogue fixture")
	}
	socket := randomTmuxSocketName(t, "skid-rename-restart")
	socketPath := namedTmuxSocketPath(socket)
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    socket,
		Home:          home,
		CataloguePath: cataloguePath,
		Profiles: []agentruntime.Profile{{
			Key:                  "personal",
			Label:                "Personal",
			Provider:             agentruntime.ProviderCodex,
			Command:              sleepPath,
			Environment:          []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: profileHome}},
			ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
			Arguments:            []string{"300"},
		}},
	})
	if err != nil {
		t.Fatal("construct restart rename manager")
	}
	ctx := context.Background()
	var cleanup *sessions.Session
	t.Cleanup(func() {
		if cleanup == nil {
			return
		}
		listed, listErr := manager.List(ctx)
		if listErr != nil {
			t.Error("list restart rename cleanup target")
			return
		}
		for _, candidate := range listed.Sessions {
			if candidate.TmuxID != cleanup.TmuxID {
				continue
			}
			if killErr := manager.Kill(ctx, sessions.KillInput{
				TmuxID: candidate.TmuxID, TmuxName: candidate.TmuxName, IdentityToken: candidate.IdentityToken,
			}); killErr != nil {
				t.Error("kill restart rename cleanup target")
			}
			return
		}
	})

	const recycledName = "rename-restarted"
	createdFirst, err := manager.Create(ctx, sessions.CreateInput{
		CWD: project, Profile: "personal", OptionalTmuxName: recycledName,
	})
	if err != nil {
		t.Fatal("create pre-restart rename target")
	}
	first := createdFirst.Session
	cleanup = &first
	firstServer := captureTestTmuxServer(t, tmuxPath, socketPath)

	bearerPath := filepath.Join(root, "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatal("mint restart gateway bearer")
	}
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)

	if err := manager.Kill(ctx, sessions.KillInput{
		TmuxID: first.TmuxID, TmuxName: first.TmuxName, IdentityToken: first.IdentityToken,
	}); err != nil {
		t.Fatal("end pre-restart exact lifetime")
	}
	cleanup = nil
	deadline := time.Now().Add(tmuxCleanupTimeout)
	for processStartIdentity(firstServer.pid) == firstServer.kernelStartTime {
		if time.Now().After(deadline) {
			t.Fatal("pre-restart tmux server did not exit")
		}
		time.Sleep(tmuxConvergencePollInterval)
	}

	createdSecond, err := manager.Create(ctx, sessions.CreateInput{
		CWD: project, Profile: "personal", OptionalTmuxName: recycledName,
	})
	if err != nil {
		t.Fatal("create recycled rename target")
	}
	second := createdSecond.Session
	cleanup = &second
	if second.TmuxID != first.TmuxID || second.TmuxName != first.TmuxName || second.IdentityToken == first.IdentityToken {
		t.Fatalf(
			"restart fixture did not recycle local identity under a new server lifetime: id_match=%t name_match=%t token_changed=%t",
			second.TmuxID == first.TmuxID,
			second.TmuxName == first.TmuxName,
			second.IdentityToken != first.IdentityToken,
		)
	}
	restartFixture := sessionFixture{manager: manager, socket: socket}
	second = restartFixture.waitForAgent(t, ctx, recycledName)
	cleanup = &second
	before := renameInvariantSnapshot(t, restartFixture, second.TmuxID)

	response := request(
		t,
		server.Client(),
		http.MethodPatch,
		server.URL+"/v1/sessions/"+url.PathEscape(second.TmuxID),
		bearer,
		"",
		renameBody(t, recycledName, "rename-stale-server", first.IdentityToken),
	)
	assertError(t, response, http.StatusConflict, "SessionIdentityMismatch")
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	current := findSession(t, inventory, recycledName)
	if current["tmuxId"] != second.TmuxID || current["identityToken"] != second.IdentityToken ||
		renameInvariantSnapshot(t, restartFixture, second.TmuxID) != before {
		t.Fatal("pre-restart rename token changed the recycled session")
	}
}

func TestAuthenticatedEmptyInventoryDoesNotStartAnIsolatedTmuxServer(t *testing.T) {
	testRoot := t.TempDir()
	socketName := randomTmuxSocketName(t, "skid-empty-inventory")
	socketPath := namedTmuxSocketPath(socketName)
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("isolated tmux socket exists before empty inventory: absent=%t", os.IsNotExist(err))
	}
	manager, err := sessions.New(sessions.Config{
		TmuxPath:      tmuxPath,
		SocketName:    socketName,
		Home:          testRoot,
		CataloguePath: filepath.Join(repositoryRoot(t), "catalog", "characters.json"),
		Profiles: []agentruntime.Profile{
			gatewayTestProfile("personal", "Codex · Personal", "/bin/true", filepath.Join(testRoot, "codex-personal")),
		},
	})
	if err != nil {
		t.Fatalf("create empty-inventory session manager: %v", err)
	}
	bearerPath := filepath.Join(testRoot, "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint empty-inventory bearer: %v", err)
	}
	server := httptest.NewServer(gateway.New(gateway.Config{
		Sessions: manager,
		Pressure: pressure.NewMonitor(),
		Bearer:   auth.FileVerifier{Path: bearerPath},
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)

	response := request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	for _, field := range []string{"machine", "observedAt", "profiles", "sessions"} {
		if _, present := inventory[field]; !present {
			t.Fatalf("empty inventory omitted required field %q", field)
		}
	}
	if len(inventory) != 4 {
		t.Fatalf("empty inventory fields = %v, want exact clock-bearing envelope", slices.Sorted(maps.Keys(inventory)))
	}
	machineWire, machinePresent := inventory["machine"].(map[string]any)
	wantMachine := map[string]any{
		"handle":   integrationMachineText,
		"platform": string(platform.Current().Kind),
	}
	if !machinePresent || !reflect.DeepEqual(machineWire, wantMachine) {
		t.Fatalf("empty inventory machine = %#v, want %#v", inventory["machine"], wantMachine)
	}
	profilesWire, profilesPresent := inventory["profiles"].([]any)
	wantProfiles := []any{
		map[string]any{
			"key":      "personal",
			"label":    "Codex · Personal",
			"provider": "Codex",
		},
	}
	if !profilesPresent || !reflect.DeepEqual(profilesWire, wantProfiles) {
		t.Fatalf("empty inventory profiles = %#v, want %#v", inventory["profiles"], wantProfiles)
	}
	sessionsWire, sessionsPresent := inventory["sessions"].([]any)
	if !sessionsPresent || sessionsWire == nil || len(sessionsWire) != 0 {
		t.Fatalf("empty inventory sessions = %#v, want non-null empty array", inventory["sessions"])
	}
	observedAt, observedAtPresent := inventory["observedAt"].(string)
	parsedObservedAt, parseErr := time.Parse(time.RFC3339Nano, observedAt)
	if !observedAtPresent || parseErr != nil || parsedObservedAt.IsZero() ||
		parsedObservedAt.UTC().Format(time.RFC3339Nano) != observedAt {
		t.Fatalf(
			"empty inventory observedAt is not one canonical nonzero UTC clock: present=%t parse_ok=%t",
			observedAtPresent,
			parseErr == nil,
		)
	}
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("authenticated empty inventory started an isolated tmux server: absent=%t", os.IsNotExist(err))
	}
}

func TestAuthenticatedGatewayControlsRealTmuxAndExposesHostPressure(t *testing.T) {
	testRoot := t.TempDir()
	socketName := randomTmuxSocketName(t, "skid-gateway")
	socketPath := namedTmuxSocketPath(socketName)
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("isolated tmux socket unexpectedly exists before test: absent=%t", os.IsNotExist(err))
	}
	codexAgentCommand := filepath.Join(testRoot, "codex-agent-command")
	if err := os.WriteFile(codexAgentCommand, []byte("#!/bin/sh\nexec /bin/sleep 300\n"), 0o700); err != nil {
		t.Fatalf("write test Codex command: %v", err)
	}
	claudeAgentCommand := filepath.Join(testRoot, "claude-agent-command")
	if err := os.WriteFile(claudeAgentCommand, []byte("#!/bin/sh\nwhile IFS= read -r line; do\n  :\ndone\n"), 0o700); err != nil {
		t.Fatalf("write test Claude command: %v", err)
	}
	for _, home := range []string{"personal", "work", "work2", "claude-work"} {
		if err := os.Mkdir(filepath.Join(testRoot, home), 0o700); err != nil {
			t.Fatalf("create %s profile home: %v", home, err)
		}
	}
	tmuxWrapper := filepath.Join(testRoot, "gateway-tmux")
	reverseInventoryMarker := filepath.Join(testRoot, "reverse-inventory")
	tmuxScript := fmt.Sprintf(`#!/bin/sh
set -eu
tmux_real=%s
reverse_inventory_marker=%s
for argument in "$@"; do
  if [ "$argument" = list-sessions ] && [ -e "$reverse_inventory_marker" ]; then
    output=$("$tmux_real" "$@") || exit $?
    if [ -n "$output" ]; then
      printf '%%s\n' "$output" | /usr/bin/awk '{ lines[NR] = $0 } END { for (line = NR; line > 0; line--) print lines[line] }'
    fi
    exit 0
  fi
done
exec "$tmux_real" "$@"
`, shellQuote(tmuxPath), shellQuote(reverseInventoryMarker))
	if err := os.WriteFile(tmuxWrapper, []byte(tmuxScript), 0o700); err != nil {
		t.Fatal("write gateway tmux wrapper")
	}
	managerConfig := sessions.Config{
		TmuxPath:      tmuxWrapper,
		SocketName:    socketName,
		Home:          testRoot,
		CataloguePath: filepath.Join(repositoryRoot(t), "catalog", "characters.json"),
		Profiles: []agentruntime.Profile{
			gatewayTestProfile("personal", "Codex · Personal", codexAgentCommand, filepath.Join(testRoot, "personal")),
			gatewayTestProfile("work", "Codex · Work", codexAgentCommand, filepath.Join(testRoot, "work")),
			gatewayTestProfile("work2", "Codex · Work 2", codexAgentCommand, filepath.Join(testRoot, "work2")),
			{
				Key:      "claude-work",
				Label:    "Claude · Work",
				Provider: agentruntime.ProviderClaude,
				Command:  claudeAgentCommand,
				Environment: []agentruntime.EnvironmentVariable{
					{Name: "CLAUDE_CONFIG_DIR", Value: filepath.Join(testRoot, "claude-work")},
				},
				ForegroundSignatures: []agentruntime.ForegroundSignature{{Argument0: "/bin/sh", Argument1: claudeAgentCommand}},
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
		Pairing:  pairing.NewSlot(),
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
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "", "")
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
	assertError(t, response, http.StatusBadRequest, "InvalidRequest")

	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory := decodeObject(t, response)
	profiles := inventory["profiles"].([]any)
	gotProfiles := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		fields := profile.(map[string]any)
		gotProfiles = append(gotProfiles, fields["key"].(string)+"="+fields["label"].(string)+":"+fields["provider"].(string))
	}
	wantProfiles := []string{"personal=Codex · Personal:Codex", "work=Codex · Work:Codex", "work2=Codex · Work 2:Codex", "claude-work=Claude · Work:Claude"}
	if !reflect.DeepEqual(gotProfiles, wantProfiles) {
		t.Fatalf("advertised profile contract mismatch: got=%v want=%v", gotProfiles, wantProfiles)
	}
	laptop := findSession(t, inventory, "laptop")
	laptopID := laptop["tmuxId"].(string)
	laptopToken := laptop["identityToken"].(string)
	laptopCard := decodeSessionCard(t, laptop)
	for _, absent := range []string{"launchProfile", "objective"} {
		if _, exists := laptop[absent]; exists {
			t.Fatalf("laptop-created session guessed %s metadata", absent)
		}
	}
	laptopAgent := laptopCard.Agent
	if laptopAgent == nil || laptopAgent.Provider != "Codex" || laptopAgent.PID <= 0 ||
		laptopAgent.Profile != "" || laptopAgent.ProviderSession != nil {
		t.Fatalf("unhooked laptop agent projection is not exact and presence-only: agent=%+v", laptopAgent)
	}
	for _, retired := range []string{"id", "name", "profile"} {
		if _, exists := laptop[retired]; exists {
			t.Fatalf("laptop session retained retired field %q", retired)
		}
	}
	laptopCharacter := laptop["character"].(map[string]any)
	if laptopCharacter["key"] == "" || laptopCharacter["displayName"] == "" {
		t.Fatal("laptop-created session omitted its required normalized character")
	}
	laptopPID := laptopAgent.PID
	registration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider:      agentruntime.ProviderCodex,
		PID:           processinfo.PID(laptopPID),
		StartIdentity: processStartIdentity(laptopPID),
	}, "work", "gateway-http-session")
	if err != nil {
		t.Fatalf("encode exact laptop registration: %v", err)
	}
	laptopPaneOutput, err := isolatedTmuxCommand(
		tmuxPath, "-L", socketName, "-f", "/dev/null", "display-message", "-p", "-t", laptopID, "#{pane_id}",
	).Output()
	if err != nil {
		t.Fatal("resolve exact test-owned laptop pane")
	}
	laptopPane := strings.TrimSpace(string(laptopPaneOutput))
	if laptopPane == "" {
		t.Fatal("exact test-owned laptop pane id is empty")
	}
	if output, commandErr := isolatedTmuxCommand(
		tmuxPath, "-L", socketName, "-f", "/dev/null", "set-option", "-p", "-t", laptopPane, "--", agentruntime.PaneOption, registration,
	).CombinedOutput(); commandErr != nil {
		t.Fatalf("install exact test-owned laptop registration: output_bytes=%d", len(output))
	}
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	registeredLaptop := findSession(t, inventory, "laptop")
	registeredCard := decodeSessionCard(t, registeredLaptop)
	registeredAgent := registeredCard.Agent
	if registeredAgent == nil || registeredAgent.Provider != "Codex" || registeredAgent.PID != laptopPID ||
		registeredAgent.Profile != "work" || registeredAgent.ProviderSession == nil ||
		registeredAgent.ProviderSession.ID != "gateway-http-session" {
		t.Fatalf("registered laptop agent projection is incomplete: agent=%+v", registeredAgent)
	}
	for _, absent := range []string{"id", "name", "profile", "launchProfile", "objective"} {
		if _, exists := registeredLaptop[absent]; exists {
			t.Fatalf("registered laptop session emitted absent or retired field %q", absent)
		}
	}
	if registeredAgent.ProviderSession.Name != "" {
		t.Fatal("registered Codex laptop session emitted a provider name")
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
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(&logs),
		Machine:  integrationMachine(t),
		Platform: platform.Current(),
	}))
	t.Cleanup(server.Close)
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	reconstructedLaptop := findSession(t, inventory, "laptop")
	decodeSessionCard(t, reconstructedLaptop)
	if reconstructedLaptop["tmuxId"] != laptopID || reconstructedLaptop["identityToken"] != laptopToken ||
		!reflect.DeepEqual(reconstructedLaptop["character"], laptopCharacter) {
		t.Fatalf(
			"gateway reconstruction changed persisted laptop identity: id_match=%t token_match=%t character_match=%t",
			reconstructedLaptop["tmuxId"] == laptopID,
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
	created := decodeCreateSessionResponse(t, response)
	if created["tmuxName"] != "gateway-test" || created["launchProfile"] != "claude-work" || created["objective"] != "Prove the control plane" {
		t.Fatal("created card did not preserve the requested name, profile, and objective")
	}
	createdCard := decodeSessionCard(t, created)
	if createdCard.Activity != "Active" {
		t.Fatalf("newly created current window activity = %q, want Active", createdCard.Activity)
	}
	if createdAgent := createdCard.Agent; createdAgent != nil {
		if createdAgent.Provider != "Claude" || createdAgent.PID <= 0 ||
			createdAgent.Profile != "" || createdAgent.ProviderSession == nil ||
			createdAgent.ProviderSession.Name != "gateway-test" || createdAgent.ProviderSession.ID != "" {
			t.Fatalf("immediate managed Claude agent projection is invalid: agent=%+v", createdAgent)
		}
	}
	character := created["character"].(map[string]any)
	if character["key"] == "" || character["displayName"] == "" {
		t.Fatal("created card omitted character metadata")
	}
	createdID := created["tmuxId"].(string)
	createdToken := created["identityToken"].(string)
	if len(createdToken) < 4 {
		t.Fatal("created card omitted its lifetime identity token")
	}
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
		{name: "zulu-agent", command: sleepPath, arguments: []string{"300"}},
		{name: "aardvark-no-agent", command: "/usr/bin/tail", arguments: []string{"-f", "/dev/null"}},
		{name: "zulu-no-agent", command: "/usr/bin/tail", arguments: []string{"-f", "/dev/null"}},
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
	response = request(t, server.Client(), http.MethodGet, server.URL+"/v1/sessions", bearer, "niels@example.test", "")
	assertStatus(t, response, http.StatusOK)
	inventory = decodeObject(t, response)
	zuluNoAgent := findSession(t, inventory, "zulu-no-agent")
	findSession(t, inventory, "gateway-test")
	findSession(t, inventory, "laptop")
	zuluAgent := findSession(t, inventory, "zulu-agent")
	aardvarkNoAgent := findSession(t, inventory, "aardvark-no-agent")
	for _, card := range inventory["sessions"].([]any) {
		decodeSessionCard(t, card.(map[string]any))
	}
	wireOrder := make([]string, 0, len(inventory["sessions"].([]any)))
	for _, value := range inventory["sessions"].([]any) {
		wireOrder = append(wireOrder, value.(map[string]any)["tmuxName"].(string))
	}
	wantWireOrder := []string{"aardvark-no-agent", "gateway-test", "laptop", "zulu-agent", "zulu-no-agent"}
	if !reflect.DeepEqual(wireOrder, wantWireOrder) {
		t.Fatalf("gateway wire order mismatch: got=%v want=%v", wireOrder, wantWireOrder)
	}
	if err := os.Remove(reverseInventoryMarker); err != nil {
		t.Fatal("disarm reversed manager inventory fixture")
	}
	for _, card := range []map[string]any{zuluNoAgent, zuluAgent, aardvarkNoAgent} {
		id := card["tmuxId"].(string)
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
	idUnchanged := unchangedLaptop["tmuxId"] == laptopID
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
	pressureBody := decodePressureResponse(t, response)
	if pressureBody.Current.SampledAt.IsZero() || !isAggregatePressureStatus(pressureBody.Current.Level) {
		t.Fatalf("pressure current verdict invalid: sampled_at_present=%t level=%s", !pressureBody.Current.SampledAt.IsZero(), pressureBody.Current.Level)
	}
	if pressureBody.Current.Phase != pressure.PhaseSteady && pressureBody.Current.Phase != pressure.PhaseRecovering {
		t.Fatalf("pressure current phase=%s, want Steady or Recovering", pressureBody.Current.Phase)
	}
	if len(pressureBody.History) == 0 || len(pressureBody.History) > pressure.HistorySampleLimit {
		t.Fatalf("pressure history count=%d, want 1..%d", len(pressureBody.History), pressure.HistorySampleLimit)
	}
	latestHistory := pressureBody.History[len(pressureBody.History)-1]
	if latestHistory.SampledAt != pressureBody.Current.SampledAt || latestHistory.Level != pressureBody.Current.Level {
		t.Fatalf(
			"pressure compact history/current mismatch: sampled_at_match=%t level_match=%t",
			latestHistory.SampledAt == pressureBody.Current.SampledAt,
			latestHistory.Level == pressureBody.Current.Level,
		)
	}
	for index, item := range pressureBody.History {
		if item.SampledAt.IsZero() || !isAggregatePressureStatus(item.Level) {
			t.Fatalf("pressure history item %d invalid: sampled_at_present=%t level=%s", index, !item.SampledAt.IsZero(), item.Level)
		}
		if index > 0 && !pressureBody.History[index-1].SampledAt.Before(item.SampledAt) {
			t.Fatalf("pressure history is not strictly chronological at item %d", index)
		}
	}
	if pressureBody.Current.Reasons == nil {
		t.Fatal("pressure current omitted its reasons array")
	}

	wantUnsupported := []pressure.Metric{pressure.MetricMemoryPressure}
	if platform.Current().Kind == platform.KindLinux {
		assertMeasuredPressureSignal(t, pressureBody.Current.Signals, pressure.MetricMemoryAvailablePercent)
	} else {
		wantUnsupported = []pressure.Metric{
			pressure.MetricCPUPressureSomeAvg60,
			pressure.MetricInputOutputPressureFullAvg60,
			pressure.MetricMemoryAvailablePercent,
			pressure.MetricMemoryPressureFullAvg60,
		}
		assertMeasuredPressureSignal(t, pressureBody.Current.Signals, pressure.MetricMemoryPressure)
	}
	if !reflect.DeepEqual(pressureBody.Unsupported, wantUnsupported) {
		t.Fatalf("%s pressure unsupported=%v, want exact capability set %v", platform.Current().Kind, pressureBody.Unsupported, wantUnsupported)
	}
	if _, exists := pressureBody.Current.Signals[pressure.MetricCPUPercent]; exists {
		t.Fatal("first pressure sample encoded an unavailable CPU delta")
	}
	if !slices.Contains(pressureBody.Current.Missing, pressure.MetricCPUPercent) {
		t.Fatalf("first pressure sample did not name omitted cpuPercent: missing_count=%d", len(pressureBody.Current.Missing))
	}
	assertPressureCapabilityPartition(t, pressureBody)
	for metric, signal := range pressureBody.Current.Signals {
		assertPressureSignalValueAndState(t, metric, signal)
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

func gatewayTestProfile(key, label, command, codexHome string) agentruntime.Profile {
	return agentruntime.Profile{
		Key:      agentruntime.ProfileKey(key),
		Label:    label,
		Provider: agentruntime.ProviderCodex,
		Command:  command,
		Environment: []agentruntime.EnvironmentVariable{
			{Name: "CODEX_HOME", Value: codexHome},
		},
		ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "sleep"}},
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

type renameHTTPRequest struct {
	target string
	body   string
}

type renameHTTPResult struct {
	status int
	body   []byte
	err    error
}

type renameTerminalReader struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func startRenameTerminalReader(t *testing.T, connection *websocket.Conn) renameTerminalReader {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			messageType, payload, err := connection.Read(ctx)
			if err != nil {
				return
			}
			if messageType != websocket.MessageText {
				continue
			}
			var control struct {
				Kind string `json:"kind"`
			}
			if json.Unmarshal(payload, &control) == nil && control.Kind == "Error" {
				return
			}
		}
	}()
	reader := renameTerminalReader{cancel: cancel, done: done}
	t.Cleanup(func() {
		reader.cancel()
		select {
		case <-reader.done:
		case <-time.After(terminalIntegrationTimeout):
			t.Error("rename terminal reader did not stop within its test-owned deadline")
		}
	})
	return reader
}

func (reader renameTerminalReader) requireResponsive(t *testing.T, connection *websocket.Conn) {
	t.Helper()
	select {
	case <-reader.done:
		t.Fatal("rename terminal reader ended before the survival probe")
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	err := connection.Ping(ctx)
	cancel()
	if err != nil {
		t.Fatal("rename terminal did not answer the bounded survival probe")
	}
	select {
	case <-reader.done:
		t.Fatal("rename terminal reader ended during the survival probe")
	default:
	}
}

func renameBody(t *testing.T, tmuxName, newTmuxName, identityToken string) string {
	t.Helper()
	body, err := json.Marshal(struct {
		TmuxName      string `json:"tmuxName"`
		NewTmuxName   string `json:"newTmuxName"`
		IdentityToken string `json:"identityToken"`
	}{
		TmuxName: tmuxName, NewTmuxName: newTmuxName, IdentityToken: identityToken,
	})
	if err != nil {
		t.Fatal("encode rename request")
	}
	return string(body)
}

func runConcurrentRenameRequests(t *testing.T, client *http.Client, bearer string, requests []renameHTTPRequest) []renameHTTPResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), terminalIntegrationTimeout)
	defer cancel()
	start := make(chan struct{})
	results := make(chan renameHTTPResult, len(requests))
	for _, request := range requests {
		request := request
		go func() {
			<-start
			httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPatch, request.target, bytes.NewBufferString(request.body))
			if err != nil {
				results <- renameHTTPResult{err: err}
				return
			}
			httpRequest.Header.Set("Authorization", "Bearer "+bearer)
			httpRequest.Header.Set("Skidbladnir-Machine", integrationMachineText)
			httpRequest.Header.Set("Content-Type", "application/json")
			response, err := client.Do(httpRequest)
			if err != nil {
				results <- renameHTTPResult{err: err}
				return
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil {
				err = readErr
			} else if closeErr != nil {
				err = closeErr
			}
			results <- renameHTTPResult{status: response.StatusCode, body: body, err: err}
		}()
	}
	close(start)
	observed := make([]renameHTTPResult, 0, len(requests))
	for range requests {
		select {
		case result := <-results:
			observed = append(observed, result)
		case <-ctx.Done():
			t.Fatal("concurrent rename requests did not finish within the test-owned deadline")
		}
	}
	return observed
}

func assertContentFreePatchRequestLogs(t *testing.T, encoded string) {
	t.Helper()
	allowedKeys := map[string]struct{}{
		"event.name":                {},
		"http.request.method":       {},
		"http.route":                {},
		"http.response.status_code": {},
		"skidbladnir.duration.ms":   {},
		"skidbladnir.error.code":    {},
	}
	expectedErrors := map[int]map[string]struct{}{
		http.StatusBadRequest: {
			string(logging.ErrorInvalidRequest): {},
		},
		http.StatusNotFound: {
			string(logging.ErrorSessionNotFound): {},
		},
		http.StatusConflict: {
			string(logging.ErrorSessionNameConflict):     {},
			string(logging.ErrorSessionIdentityMismatch): {},
		},
		http.StatusUnprocessableEntity: {
			string(logging.ErrorSessionNameInvalid): {},
		},
	}
	patchCount := 0
	for _, line := range strings.Split(encoded, "\n") {
		if line == "" {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatal("rename flow emitted a non-JSON structured log line")
		}
		eventName, ok := fields["event.name"].(string)
		if !ok {
			t.Fatal("rename flow emitted a structured log without a closed event name")
		}
		switch eventName {
		case "Sessions.Listed":
			continue
		case "Request.Completed":
		default:
			t.Fatal("rename flow emitted an event outside request completion and authoritative inventory")
		}
		method, ok := fields["http.request.method"].(string)
		if !ok || method != http.MethodPatch {
			continue
		}
		patchCount++
		for key := range fields {
			if _, allowed := allowedKeys[key]; !allowed {
				t.Fatal("PATCH request completion log exposed a field outside the exact allowed set")
			}
		}
		if fields["event.name"] != "Request.Completed" || fields["http.request.method"] != http.MethodPatch ||
			fields["http.route"] != string(logging.RouteSession) {
			t.Fatal("PATCH request completion log did not use the exact event, method, and route")
		}
		statusNumber, ok := fields["http.response.status_code"].(float64)
		status := int(statusNumber)
		if !ok || statusNumber != float64(status) {
			t.Fatal("PATCH request completion log did not encode an integer HTTP status")
		}
		duration, ok := fields["skidbladnir.duration.ms"].(float64)
		if !ok || duration < 0 || duration != float64(int64(duration)) {
			t.Fatal("PATCH request completion log did not encode a nonnegative integer duration")
		}
		errorValue, hasError := fields["skidbladnir.error.code"]
		if status == http.StatusNoContent {
			if hasError || len(fields) != 5 {
				t.Fatal("successful PATCH request completion log contained an error or an extra field")
			}
			continue
		}
		expectedCodes, knownStatus := expectedErrors[status]
		errorCode, stringError := errorValue.(string)
		if !knownStatus || !hasError || !stringError || len(fields) != 6 {
			t.Fatal("rejected PATCH request completion log did not contain the exact closed fields")
		}
		if _, expected := expectedCodes[errorCode]; !expected {
			t.Fatal("rejected PATCH request completion log used an error outside its closed status mapping")
		}
	}
	if patchCount == 0 {
		t.Fatal("rename flow emitted no PATCH request completion logs")
	}
}

func assertConcurrentRenameResults(t *testing.T, results []renameHTTPResult, loserCode, loserMessage string) {
	t.Helper()
	if len(results) != 2 {
		t.Fatalf("concurrent rename result count=%d, want 2", len(results))
	}
	successes := 0
	losers := 0
	for _, result := range results {
		if result.err != nil {
			t.Fatal("concurrent rename request did not return an HTTP result")
		}
		switch result.status {
		case http.StatusNoContent:
			successes++
			if len(result.body) != 0 {
				t.Fatalf("successful concurrent rename returned %d body bytes, want 0", len(result.body))
			}
		case http.StatusConflict:
			losers++
			var body map[string]any
			if err := json.Unmarshal(result.body, &body); err != nil {
				t.Fatal("decode concurrent rename rejection")
			}
			if len(body) != 2 || body["code"] != loserCode || body["message"] != loserMessage {
				t.Fatal("concurrent rename loser did not expose the exact typed rejection")
			}
		default:
			t.Fatalf("concurrent rename status=%d, want 204 or 409; response_body_bytes=%d", result.status, len(result.body))
		}
	}
	if successes != 1 || losers != 1 {
		t.Fatalf("concurrent rename outcomes: successes=%d losers=%d, want 1/1", successes, losers)
	}
}

func assertBodylessStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal("read bodyless HTTP response")
	}
	if response.StatusCode != want || len(body) != 0 {
		t.Fatalf("HTTP bodyless result: status=%d want=%d response_body_bytes=%d", response.StatusCode, want, len(body))
	}
}

func assertSessionCardPreservesRenameInvariants(t *testing.T, before, after map[string]any, wantName string) {
	t.Helper()
	if after["tmuxName"] != wantName {
		t.Fatal("authoritative inventory did not expose the expected tmux name")
	}
	for _, snapshot := range []struct {
		label string
		card  map[string]any
	}{{label: "before", card: before}, {label: "after", card: after}} {
		if snapshot.card["activity"] != "Active" && snapshot.card["activity"] != "Quiet" {
			t.Fatalf("%s rename card has invalid fresh activity %#v", snapshot.label, snapshot.card["activity"])
		}
	}
	beforeFacts := make(map[string]any, len(before)-1)
	afterFacts := make(map[string]any, len(after)-1)
	for key, value := range before {
		if key != "tmuxName" && key != "activity" {
			beforeFacts[key] = value
		}
	}
	for key, value := range after {
		if key != "tmuxName" && key != "activity" {
			afterFacts[key] = value
		}
	}
	if !reflect.DeepEqual(beforeFacts, afterFacts) {
		t.Fatal("authoritative inventory changed a rename-stable session fact")
	}
}

func providerSessionName(card map[string]any) string {
	agent, ok := card["agent"].(map[string]any)
	if !ok {
		return ""
	}
	providerSession, ok := agent["providerSession"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := providerSession["name"].(string)
	return name
}

func renameInvariantSnapshot(t *testing.T, fixture sessionFixture, tmuxID string) string {
	t.Helper()
	session := fixture.tmux(t, "display-message", "-p", "-t", tmuxID,
		"#{session_id}|#{session_attached}|#{session_group_size}|#{session_group_attached}|#{session_width}|#{session_height}|#{window_id}|#{pane_id}|#{pane_pid}|#{@skid_internal}|#{@skid_profile}|#{@skid_character}|#{@skid_objective_b64}")
	windows := strings.Split(fixture.tmux(t, "list-windows", "-t", tmuxID, "-F",
		"#{window_id}|#{window_index}|#{window_active}|#{window_name}|#{window_panes}|#{window_width}|#{window_height}|#{window_layout}|#{window_active_clients}"), "\n")
	panes := strings.Split(fixture.tmux(t, "list-panes", "-s", "-t", tmuxID, "-F",
		"#{window_id}|#{pane_id}|#{pane_index}|#{pane_active}|#{pane_pid}|#{pane_width}|#{pane_height}|#{pane_current_path}|#{pane_current_command}"), "\n")
	group := fixture.tmux(t, "display-message", "-p", "-t", tmuxID, "#{session_group}")
	groupMembers := make([]string, 0)
	memberSet := make(map[string]struct{})
	for _, line := range strings.Split(fixture.tmux(t, "list-sessions", "-F", "#{session_id}|#{session_group}"), "\n") {
		fields := strings.SplitN(line, "|", 2)
		if len(fields) == 2 && fields[1] == group {
			groupMembers = append(groupMembers, fields[0])
			memberSet[fields[0]] = struct{}{}
		}
	}
	clients := make([]string, 0)
	for _, line := range strings.Split(fixture.tmux(t, "list-clients", "-F",
		"#{session_id}|#{client_name}|#{client_width}|#{client_height}|#{client_flags}"), "\n") {
		fields := strings.SplitN(line, "|", 2)
		if len(fields) == 2 {
			if _, belongs := memberSet[fields[0]]; belongs {
				clients = append(clients, line)
			}
		}
	}
	sessionOptions := strings.Split(fixture.tmux(t, "show-options", "-t", tmuxID), "\n")
	windowOptions := make([]string, 0)
	for _, window := range windows {
		fields := strings.SplitN(window, "|", 2)
		if len(fields) != 2 {
			t.Fatal("malformed rename window snapshot")
		}
		for _, option := range strings.Split(fixture.tmux(t, "show-options", "-w", "-t", fields[0]), "\n") {
			if option != "" {
				windowOptions = append(windowOptions, fields[0]+"|"+option)
			}
		}
	}
	paneOptions := make([]string, 0)
	for _, pane := range panes {
		fields := strings.SplitN(pane, "|", 3)
		if len(fields) != 3 {
			t.Fatal("malformed rename pane snapshot")
		}
		for _, option := range strings.Split(fixture.tmux(t, "show-options", "-p", "-t", fields[1]), "\n") {
			if option != "" {
				paneOptions = append(paneOptions, fields[1]+"|"+option)
			}
		}
	}
	for _, values := range [][]string{windows, panes, groupMembers, clients, sessionOptions, windowOptions, paneOptions} {
		slices.Sort(values)
	}
	return strings.Join([]string{
		session,
		strings.Join(windows, "\n"),
		strings.Join(panes, "\n"),
		strings.Join(groupMembers, "\n"),
		strings.Join(clients, "\n"),
		strings.Join(sessionOptions, "\n"),
		strings.Join(windowOptions, "\n"),
		strings.Join(paneOptions, "\n"),
	}, "\n")
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
		"SessionNameConflict":         "A session with that name already exists.",
		"SessionNotFound":             "That session no longer exists.",
		"ObjectiveInvalid":            "Use 1–240 characters without terminal controls.",
		"SessionIdentityMismatch":     "The session changed. Refresh and try again.",
		"MachineIdentityMismatch":     "The machine identity changed. Fleet reset is required.",
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

type sessionAgentResponse struct {
	Provider        string                   `json:"provider"`
	PID             int                      `json:"pid"`
	Profile         string                   `json:"profile,omitempty"`
	ProviderSession *providerSessionResponse `json:"providerSession,omitempty"`
}

type providerSessionResponse struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type sessionCharacterResponse struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type sessionCardResponse struct {
	TmuxID          string                   `json:"tmuxId"`
	TmuxName        string                   `json:"tmuxName"`
	IdentityToken   string                   `json:"identityToken"`
	Character       sessionCharacterResponse `json:"character"`
	LaunchProfile   string                   `json:"launchProfile,omitempty"`
	Agent           *sessionAgentResponse    `json:"agent,omitempty"`
	Objective       string                   `json:"objective,omitempty"`
	CWD             string                   `json:"cwd,omitempty"`
	ActiveCommand   string                   `json:"activeCommand,omitempty"`
	AttachedClients int                      `json:"attachedClients"`
	Activity        string                   `json:"activity"`
}

type createSessionResponse struct {
	ObservedAt string         `json:"observedAt"`
	Session    map[string]any `json:"session"`
}

func decodeCreateSessionResponse(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	fields := decodeObject(t, response)
	var decoded createSessionResponse
	decodeStrictValue(t, fields, &decoded)
	observedAt, err := time.Parse(time.RFC3339Nano, decoded.ObservedAt)
	canonicalObservedAt := err == nil && !observedAt.IsZero() &&
		decoded.ObservedAt == observedAt.UTC().Format(time.RFC3339Nano)
	if len(fields) != 2 || !canonicalObservedAt || decoded.Session == nil {
		t.Fatalf(
			"create response is not the exact observedAt/session envelope: field_count=%d observed_at_canonical=%t session_present=%t",
			len(fields),
			canonicalObservedAt,
			decoded.Session != nil,
		)
	}
	decodeSessionCard(t, decoded.Session)
	return decoded.Session
}

func decodeSessionCard(t *testing.T, card map[string]any) sessionCardResponse {
	t.Helper()
	var decoded sessionCardResponse
	decodeStrictValue(t, card, &decoded)
	for _, required := range []string{"tmuxId", "tmuxName", "identityToken", "character", "attachedClients", "activity"} {
		if _, exists := card[required]; !exists {
			t.Fatalf("session wire payload omitted required field %q", required)
		}
	}
	if decoded.TmuxID == "" || decoded.TmuxName == "" || decoded.IdentityToken == "" ||
		decoded.Character.Key == "" || decoded.Character.DisplayName == "" || decoded.AttachedClients < 0 {
		t.Fatalf("session wire payload has incomplete required identity or character: %+v", decoded)
	}
	if decoded.Activity != "Active" && decoded.Activity != "Quiet" {
		t.Fatalf("session wire payload has invalid activity %q", decoded.Activity)
	}
	for _, optional := range []string{"launchProfile", "objective", "cwd", "activeCommand"} {
		if value, exists := card[optional]; exists && value == "" {
			t.Fatalf("session wire payload emitted empty optional field %q", optional)
		}
	}
	agentValue, hasAgent := card["agent"]
	if decoded.Agent == nil {
		if hasAgent {
			t.Fatalf("session wire payload emitted a null or invalid optional agent: %#v", agentValue)
		}
		return decoded
	}
	agentFields, ok := agentValue.(map[string]any)
	if !ok || (decoded.Agent.Provider != "Codex" && decoded.Agent.Provider != "Claude") || decoded.Agent.PID <= 0 {
		t.Fatalf("session wire payload has invalid optional agent: %#v", agentValue)
	}
	if decoded.Agent.Profile == "" {
		if _, exists := agentFields["profile"]; exists {
			t.Fatal("agent emitted an empty profile field")
		}
	}
	providerSessionValue, hasProviderSession := agentFields["providerSession"]
	if decoded.Agent.ProviderSession == nil {
		if hasProviderSession {
			t.Fatal("agent emitted an empty providerSession field")
		}
		return decoded
	}
	providerSessionFields, ok := providerSessionValue.(map[string]any)
	if !ok || (decoded.Agent.ProviderSession.ID == "" && decoded.Agent.ProviderSession.Name == "") {
		t.Fatalf("agent providerSession is not a non-empty object: %#v", providerSessionValue)
	}
	if decoded.Agent.ProviderSession.ID == "" {
		if _, exists := providerSessionFields["id"]; exists {
			t.Fatal("providerSession emitted an empty id field")
		}
	}
	if decoded.Agent.ProviderSession.Name == "" {
		if _, exists := providerSessionFields["name"]; exists {
			t.Fatal("providerSession emitted an empty name field")
		}
	}
	return decoded
}

func decodeStrictValue(t *testing.T, value any, destination any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode strict wire value: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("decode strict wire value: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("strict wire value contained trailing JSON: %v", err)
	}
}

type pressureResponse struct {
	Unsupported []pressure.Metric       `json:"unsupported"`
	Current     pressureCurrentResponse `json:"current"`
	History     []pressureHistoryItem   `json:"history"`
}

type pressureCurrentResponse struct {
	SampledAt time.Time                                  `json:"sampledAt"`
	Level     pressure.Status                            `json:"level"`
	Phase     pressure.Phase                             `json:"phase"`
	Reasons   []pressure.Reason                          `json:"reasons"`
	Signals   map[pressure.Metric]pressureSignalResponse `json:"signals"`
	Missing   []pressure.Metric                          `json:"missing"`
}

type pressureSignalResponse struct {
	Value json.RawMessage       `json:"value"`
	State pressure.SignalStatus `json:"state"`
}

type pressureHistoryItem struct {
	SampledAt time.Time       `json:"sampledAt"`
	Level     pressure.Status `json:"level"`
}

func decodePressureResponse(t *testing.T, response *http.Response) pressureResponse {
	t.Helper()
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	var body pressureResponse
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("decode strict pressure HTTP response: status=%d error=%v", response.StatusCode, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("pressure HTTP response contained trailing JSON: error=%v", err)
	}
	return body
}

func assertPressureCapabilityPartition(t *testing.T, body pressureResponse) {
	t.Helper()
	universe := []pressure.Metric{
		pressure.MetricCPUPercent,
		pressure.MetricLoadNormalized,
		pressure.MetricMemoryAvailablePercent,
		pressure.MetricSwapUsedPercent,
		pressure.MetricDiskAvailablePercent,
		pressure.MetricCPUPressureSomeAvg60,
		pressure.MetricMemoryPressureFullAvg60,
		pressure.MetricInputOutputPressureFullAvg60,
		pressure.MetricMemoryPressure,
	}
	known := make(map[pressure.Metric]struct{}, len(universe))
	for _, metric := range universe {
		known[metric] = struct{}{}
	}
	for _, part := range []struct {
		name    string
		metrics []pressure.Metric
	}{
		{name: "missing", metrics: body.Current.Missing},
		{name: "unsupported", metrics: body.Unsupported},
	} {
		unique := slices.Compact(append([]pressure.Metric(nil), part.metrics...))
		if !slices.IsSorted(part.metrics) || len(unique) != len(part.metrics) {
			t.Fatalf("pressure %s is not sorted and unique: %v", part.name, part.metrics)
		}
	}
	membership := make(map[pressure.Metric]string, len(universe))
	for metric := range body.Current.Signals {
		if _, exists := known[metric]; !exists {
			t.Fatalf("pressure signals published unknown metric %q", metric)
		}
		membership[metric] = "signals"
	}
	for _, part := range []struct {
		name    string
		metrics []pressure.Metric
	}{
		{name: "missing", metrics: body.Current.Missing},
		{name: "unsupported", metrics: body.Unsupported},
	} {
		for _, metric := range part.metrics {
			if _, exists := known[metric]; !exists {
				t.Fatalf("pressure %s published unknown metric %q", part.name, metric)
			}
			if previous, duplicate := membership[metric]; duplicate {
				t.Fatalf("pressure metric %q appears in both %s and %s", metric, previous, part.name)
			}
			membership[metric] = part.name
		}
	}
	for _, metric := range universe {
		if _, exists := membership[metric]; !exists {
			t.Fatalf("pressure metric %q is absent from signals, missing, and unsupported", metric)
		}
	}
}

func assertMeasuredPressureSignal(t *testing.T, signals map[pressure.Metric]pressureSignalResponse, metric pressure.Metric) {
	t.Helper()
	if _, exists := signals[metric]; !exists {
		t.Fatalf("%s pressure capability omitted required measured signal %q", platform.Current().Kind, metric)
	}
}

func assertPressureSignalValueAndState(t *testing.T, metric pressure.Metric, signal pressureSignalResponse) {
	t.Helper()
	if len(signal.Value) == 0 || bytes.Equal(signal.Value, []byte("null")) {
		t.Fatalf("pressure signal %q has no measured value", metric)
	}
	if metric == pressure.MetricCPUPercent || metric == pressure.MetricSwapUsedPercent {
		if signal.State != pressure.SignalStatusInformational {
			t.Fatalf("pressure signal %q state=%s, want Informational", metric, signal.State)
		}
	} else if !isThresholdSignalStatus(signal.State) {
		t.Fatalf("pressure signal %q state=%s, want Normal, Warm, or Hot", metric, signal.State)
	}
}

func isThresholdSignalStatus(status pressure.SignalStatus) bool {
	return status == pressure.SignalStatusNormal || status == pressure.SignalStatusWarm || status == pressure.SignalStatusHot
}

func isAggregatePressureStatus(status pressure.Status) bool {
	return status == pressure.StatusNormal || status == pressure.StatusWarm || status == pressure.StatusHot || status == pressure.StatusUnknown
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

func findSessionID(t *testing.T, inventory map[string]any, tmuxID string) map[string]any {
	t.Helper()
	sessions := inventory["sessions"].([]any)
	for _, value := range sessions {
		session := value.(map[string]any)
		if session["tmuxId"] == tmuxID {
			return session
		}
	}
	t.Fatalf("expected session id not found in inventory: session_count=%d", len(sessions))
	return nil
}
