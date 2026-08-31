package gateway

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestSessionProjectionPublishesOnlyRequiredActivityAndOptionalAgent(t *testing.T) {
	profiles := testProfileCatalog()
	providerSession, err := agentruntime.NewProviderSessionFacts("019-runtime", "")
	if err != nil {
		t.Fatalf("construct provider session facts: %v", err)
	}
	agent := &agentruntime.AgentRuntime{
		Provider:        agentruntime.ProviderCodex,
		PID:             4312,
		Profile:         "work",
		ProviderSession: providerSession,
	}
	candidate := sessions.Session{
		TmuxID:          "$7",
		TmuxName:        "laptop-work",
		IdentityToken:   "v1-lifetime",
		Character:       catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"},
		LaunchProfile:   "work",
		Agent:           agent,
		AttachedClients: 2,
		Activity:        sessions.SessionActivityActive,
	}
	card, err := mapSession(candidate, profiles)
	if err != nil {
		t.Fatalf("project session with exact agent identity: %v", err)
	}
	fields := encodedObject(t, card)
	want := map[string]any{
		"tmuxId":        "$7",
		"tmuxName":      "laptop-work",
		"identityToken": "v1-lifetime",
		"character": map[string]any{
			"key":         "norse.durinn",
			"displayName": "Durinn",
		},
		"launchProfile": "work",
		"agent": map[string]any{
			"provider":        "Codex",
			"pid":             float64(4312),
			"profile":         "work",
			"providerSession": map[string]any{"id": "019-runtime"},
		},
		"attachedClients": float64(2),
		"activity":        "Active",
	}
	if !maps.EqualFunc(fields, want, encodedValuesEqual) {
		t.Fatalf("session wire projection = %#v, want exact closed contract %#v", fields, want)
	}

	candidate.Agent = nil
	card, err = mapSession(candidate, profiles)
	if err != nil {
		t.Fatalf("project session without optional agent identity: %v", err)
	}
	fields = encodedObject(t, card)
	if fields["activity"] != "Active" {
		t.Fatalf("agent absence changed Active activity to %#v", fields["activity"])
	}
	if _, present := fields["agent"]; present {
		t.Fatalf("absent agent identity crossed the wire: %#v", fields["agent"])
	}
	candidate.Agent = agent
	candidate.Activity = sessions.SessionActivityQuiet
	card, err = mapSession(candidate, profiles)
	if err != nil {
		t.Fatalf("project Quiet session with optional agent identity: %v", err)
	}
	fields = encodedObject(t, card)
	if fields["activity"] != "Quiet" || fields["agent"] == nil {
		t.Fatalf("agent presence changed Quiet activity or disappeared: %#v", fields)
	}
	for _, retired := range []string{"status", "signal", "signalAt", "runtime", "interaction", "attention"} {
		if _, present := fields[retired]; present {
			t.Fatalf("session wire retained retired field %q", retired)
		}
	}

	for _, invalid := range []sessions.SessionActivity{"", "ACTIVE", "Unknown"} {
		candidate.Activity = invalid
		if _, err := mapSession(candidate, profiles); err == nil {
			t.Fatalf("session projection accepted activity %q outside Active | Quiet", invalid)
		}
	}
	candidate.Activity = sessions.SessionActivityActive
	candidate.LaunchProfile = "other"
	if _, err := mapSession(candidate, profiles); err == nil {
		t.Fatal("session projection accepted a launch profile outside the emitted catalog")
	}
	candidate.LaunchProfile = ""
	invalidAgent := *agent
	invalidAgent.Provider = agentruntime.ProviderClaude
	candidate.Agent = &invalidAgent
	if _, err := mapSession(candidate, profiles); err == nil {
		t.Fatal("session projection accepted agent metadata invalid for the emitted catalog")
	}
	candidate.Agent = agent
	for _, test := range []struct {
		name   string
		mutate func(*sessions.Session)
	}{
		{name: "missing tmux id", mutate: func(value *sessions.Session) { value.TmuxID = "" }},
		{name: "missing tmux name", mutate: func(value *sessions.Session) { value.TmuxName = "" }},
		{name: "missing lifetime identity", mutate: func(value *sessions.Session) { value.IdentityToken = "" }},
		{name: "missing character key", mutate: func(value *sessions.Session) { value.Character.Key = "" }},
		{name: "missing character name", mutate: func(value *sessions.Session) { value.Character.DisplayName = "" }},
		{name: "negative client count", mutate: func(value *sessions.Session) { value.AttachedClients = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := candidate
			test.mutate(&invalid)
			if _, err := mapSession(invalid, profiles); err == nil {
				t.Fatal("session projection accepted an invalid required fact")
			}
		})
	}
}

func TestInventoryProjectionUsesItsSingleServiceObservationClock(t *testing.T) {
	observedAt := time.Date(2026, time.August, 28, 19, 14, 53, 987654321, time.UTC)
	inventory := sessions.Inventory{
		ObservedAt: observedAt,
		Sessions: []sessions.Session{{
			TmuxID:        "$12",
			TmuxName:      "observed",
			IdentityToken: "v1-observed",
			Character:     catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"},
			Activity:      sessions.SessionActivityActive,
		}},
	}
	projected, err := mapSessionsResponse(
		machineDTO{Handle: "mh-00112233445566778899aabbccddeeff", Platform: platform.KindLinux},
		inventory,
		validProfileProjectionCatalog(),
	)
	if err != nil {
		t.Fatalf("project inventory: %v", err)
	}
	envelope := encodedObject(t, projected)
	if got, want := slices.Sorted(maps.Keys(envelope)), []string{"machine", "observedAt", "profiles", "sessions"}; !slices.Equal(got, want) {
		t.Fatalf("inventory response fields = %v, want exact envelope %v", got, want)
	}
	if envelope["observedAt"] != observedAt.Format(time.RFC3339Nano) {
		t.Fatalf("inventory observedAt = %#v, want exact service clock", envelope["observedAt"])
	}
	if len(projected.Sessions) != 1 || projected.Sessions[0].Activity != "Active" {
		t.Fatalf("inventory session activity = %+v, want one Active card", projected.Sessions)
	}

	inventory.ObservedAt = time.Time{}
	if _, err := mapSessionsResponse(machineDTO{}, inventory, validProfileProjectionCatalog()); err == nil {
		t.Fatal("inventory projection accepted a zero observation clock")
	}
	inventory.ObservedAt = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := mapSessionsResponse(machineDTO{}, inventory, validProfileProjectionCatalog()); err == nil {
		t.Fatal("inventory projection accepted a clock outside canonical RFC 3339")
	}
}

func TestCreateSessionProjectionCarriesTheSameStrictSessionDTO(t *testing.T) {
	observedAt := time.Date(2026, time.August, 28, 19, 14, 53, 123456789, time.UTC)
	observed := sessions.ObservedSession{
		ObservedAt: observedAt,
		Session: sessions.Session{
			TmuxID:        "$11",
			TmuxName:      "created",
			IdentityToken: "v1-created",
			Character:     catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"},
			Activity:      sessions.SessionActivityQuiet,
		},
	}
	projected, err := mapCreateSessionResponse(observed, validProfileProjectionCatalog())
	if err != nil {
		t.Fatalf("project created session: %v", err)
	}
	fields := encodedObject(t, projected)
	if got, want := slices.Sorted(maps.Keys(fields)), []string{"observedAt", "session"}; !slices.Equal(got, want) {
		t.Fatalf("create response fields = %v, want exact envelope %v", got, want)
	}
	if fields["observedAt"] != observedAt.Format(time.RFC3339Nano) {
		t.Fatalf("create observedAt = %#v, want exact mapping clock", fields["observedAt"])
	}
	sessionFields, ok := fields["session"].(map[string]any)
	if !ok || sessionFields["activity"] != "Quiet" {
		t.Fatalf("create session does not carry the strict Quiet DTO: %#v", fields["session"])
	}
	for _, retired := range []string{"status", "signal", "signalAt", "runtime", "interaction", "attention"} {
		if _, present := sessionFields[retired]; present {
			t.Fatalf("create session retained retired field %q", retired)
		}
	}

	observed.ObservedAt = time.Time{}
	if _, err := mapCreateSessionResponse(observed, validProfileProjectionCatalog()); err == nil {
		t.Fatal("create response projection accepted a zero observation clock")
	}
	observed.ObservedAt = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := mapCreateSessionResponse(observed, validProfileProjectionCatalog()); err == nil {
		t.Fatal("create response projection accepted a clock outside canonical RFC 3339")
	}
}

func encodedObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode projected object: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode projected object: %v", err)
	}
	return fields
}

func encodedValuesEqual(left, right any) bool {
	leftEncoded, leftErr := json.Marshal(left)
	rightEncoded, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftEncoded) == string(rightEncoded)
}

func testProfileCatalog() []agentruntime.Profile {
	return []agentruntime.Profile{
		{Key: "work", Label: "Codex · Work", Provider: agentruntime.ProviderCodex},
		{Key: "claude-work", Label: "Claude · Work", Provider: agentruntime.ProviderClaude},
	}
}

func TestProfileProjectionRequiresAClosedProvider(t *testing.T) {
	profiles, err := mapProfiles(validProfileProjectionCatalog())
	if err != nil || len(profiles) != 1 || profiles[0].Provider != "Codex" {
		t.Fatalf("profile projection = (%+v, %v), want required Codex provider", profiles, err)
	}
	if _, err := mapProfiles([]agentruntime.Profile{{Key: "work", Label: "Work", Provider: agentruntime.Provider("Other")}}); err == nil {
		t.Fatal("profile projection accepted a provider outside the closed wire union")
	}
}

func TestProfileProjectionRejectsInvalidConfiguredCatalogs(t *testing.T) {
	tests := []struct {
		name      string
		catalogue func() []agentruntime.Profile
	}{
		{
			name: "duplicate profile key",
			catalogue: func() []agentruntime.Profile {
				profiles := validProfileProjectionCatalog()
				duplicate := profiles[0]
				duplicate.Label = "Codex · Personal"
				duplicate.Environment = []agentruntime.EnvironmentVariable{{Name: "CODEX_HOME", Value: "/profiles/personal"}}
				return append(profiles, duplicate)
			},
		},
		{
			name: "relative provider command",
			catalogue: func() []agentruntime.Profile {
				profiles := validProfileProjectionCatalog()
				profiles[0].Command = "codex"
				return profiles
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := mapProfiles(test.catalogue()); err == nil {
				t.Fatalf("profile projection accepted %s", test.name)
			}
		})
	}
}

func validProfileProjectionCatalog() []agentruntime.Profile {
	return []agentruntime.Profile{{
		Key:      "work",
		Label:    "Codex · Work",
		Provider: agentruntime.ProviderCodex,
		Command:  "/bin/codex",
		Environment: []agentruntime.EnvironmentVariable{
			{Name: "CODEX_HOME", Value: "/profiles/work"},
		},
		ForegroundSignatures: []agentruntime.ForegroundSignature{{ExecutableBase: "codex"}},
	}}
}

// pressureMetricUniverse is the closed `PressureMetric` union of the wire
// schema; each value is also the metric's exact JSON key under `signals`.
var pressureMetricUniverse = []pressure.Metric{
	pressure.MetricCPUPercent,
	pressure.MetricCPUPressureSomeAvg60,
	pressure.MetricDiskAvailablePercent,
	pressure.MetricInputOutputPressureFullAvg60,
	pressure.MetricLoadNormalized,
	pressure.MetricMemoryAvailablePercent,
	pressure.MetricMemoryPressure,
	pressure.MetricMemoryPressureFullAvg60,
	pressure.MetricSwapUsedPercent,
}

// declaredUnsupportedByPlatform is the capability contract: Linux never
// observes native memory pressure, and Darwin never observes the Linux memory
// percentage or any PSI signal. internal/pressure owns the declaration; this
// table owns the promise the wire schema makes about it.
var declaredUnsupportedByPlatform = map[platform.Kind][]pressure.Metric{
	platform.KindLinux: {pressure.MetricMemoryPressure},
	platform.KindDarwin: {
		pressure.MetricCPUPressureSomeAvg60,
		pressure.MetricInputOutputPressureFullAvg60,
		pressure.MetricMemoryAvailablePercent,
		pressure.MetricMemoryPressureFullAvg60,
	},
}

func TestPressureResponsePublishesRichSignalsAndCompactHistoryForTheDeclaredHostCapability(t *testing.T) {
	host := platform.Current().Kind
	monitor := pressure.NewMonitor()
	want, declared := declaredUnsupportedByPlatform[host], monitor.Unsupported()
	if !slices.Equal(declared, want) {
		t.Fatalf("%s declared unsupported capability = %v, want %v", host, declared, want)
	}
	if !slices.IsSorted(declared) || len(slices.Compact(append([]pressure.Metric(nil), declared...))) != len(declared) {
		t.Fatalf("%s declared unsupported capability is not sorted and unique: %v", host, declared)
	}

	handle, err := machine.Parse("mh-00000000000000000000000000000000")
	if err != nil {
		t.Fatalf("construct test machine identity: %v", err)
	}
	gateway := New(Config{
		Pressure: monitor,
		Pairing:  pairing.NewSlot(),
		Logger:   logging.New(io.Discard),
		Machine:  handle,
		Platform: platform.Current(),
	})
	recorder := httptest.NewRecorder()
	gateway.readPressure(recorder)
	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("pressure HTTP response = status %d content-type %q, want 200 application/json", response.StatusCode, response.Header.Get("Content-Type"))
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the observed %s pressure response: %v", host, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode the pressure response fields: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("pressure response fields = %v, want unsupported/current/history only", slices.Sorted(maps.Keys(fields)))
	}
	var currentFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["current"], &currentFields); err != nil {
		t.Fatalf("decode current pressure fields: %v", err)
	}
	if got, want := slices.Sorted(maps.Keys(currentFields)), []string{"level", "missing", "phase", "reasons", "sampledAt", "signals"}; !slices.Equal(got, want) {
		t.Fatalf("current pressure fields = %v, want %v", got, want)
	}
	var body struct {
		Unsupported []pressure.Metric `json:"unsupported"`
		Current     struct {
			Phase   string                     `json:"phase"`
			Signals map[string]json.RawMessage `json:"signals"`
			Missing []pressure.Metric          `json:"missing"`
		} `json:"current"`
		History []map[string]json.RawMessage `json:"history"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("decode the pressure response: %v", err)
	}

	if !slices.Equal(body.Unsupported, want) {
		t.Fatalf("%s response unsupported = %v, want the declared capability %v", host, body.Unsupported, want)
	}
	if body.Current.Phase != "Steady" {
		t.Fatalf("%s current phase = %q, want Steady", host, body.Current.Phase)
	}
	for _, metric := range body.Current.Missing {
		if slices.Contains(body.Unsupported, metric) {
			t.Fatalf("metric %q is reported both missing and unsupported on %s", metric, host)
		}
	}
	absent := make([]pressure.Metric, 0, len(pressureMetricUniverse))
	for _, metric := range pressureMetricUniverse {
		if _, published := body.Current.Signals[string(metric)]; !published {
			absent = append(absent, metric)
		}
	}
	if len(body.Current.Signals)+len(absent) != len(pressureMetricUniverse) {
		t.Fatalf(
			"published metric keys escape the closed union on %s: published_count=%d absent_count=%d union_count=%d",
			host, len(body.Current.Signals), len(absent), len(pressureMetricUniverse),
		)
	}
	partition := append(append([]pressure.Metric(nil), body.Current.Missing...), body.Unsupported...)
	slices.Sort(partition)
	if !slices.Equal(partition, absent) {
		t.Fatalf("%s missing + unsupported = %v, want exactly the absent metric keys %v", host, partition, absent)
	}
	for metric, encoded := range body.Current.Signals {
		var signal map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &signal); err != nil {
			t.Fatalf("decode %s signal: %v", metric, err)
		}
		if len(signal) != 2 || signal["value"] == nil || signal["state"] == nil {
			t.Fatalf("%s signal fields = %v, want value/state only", metric, slices.Sorted(maps.Keys(signal)))
		}
		var state string
		if err := json.Unmarshal(signal["state"], &state); err != nil {
			t.Fatalf("decode %s signal state: %v", metric, err)
		}
		if metric == string(pressure.MetricCPUPercent) || metric == string(pressure.MetricSwapUsedPercent) {
			if state != "Informational" {
				t.Fatalf("%s state = %q, want Informational", metric, state)
			}
		} else if state != "Normal" && state != "Warm" && state != "Hot" {
			t.Fatalf("%s state = %q, want a threshold status", metric, state)
		}
	}
	if len(body.History) != 1 || len(body.History[0]) != 2 || body.History[0]["sampledAt"] == nil || body.History[0]["level"] == nil {
		t.Fatalf("history = %v, want one compact sampledAt/level item", body.History)
	}
}

func TestClosedAPIErrorsWriteExactHTTPResponses(t *testing.T) {
	tests := []struct {
		failure apiError
		code    string
		message string
		status  int
	}{
		{errorUnauthenticated, "Unauthenticated", "Authentication required.", http.StatusUnauthorized},
		{errorInvalidRequest, "InvalidRequest", "The request is not valid.", http.StatusBadRequest},
		{errorRequestTooLarge, "RequestTooLarge", "The request is too large.", http.StatusRequestEntityTooLarge},
		{errorWorkingDirectoryInvalid, "WorkingDirectoryInvalid", "Choose a valid working directory.", http.StatusUnprocessableEntity},
		{errorWorkingDirectoryUnavailable, "WorkingDirectoryUnavailable", "That directory does not exist or cannot be opened.", http.StatusUnprocessableEntity},
		{errorProfileUnknown, "ProfileUnknown", "Choose an available profile.", http.StatusUnprocessableEntity},
		{errorSessionNameInvalid, "SessionNameInvalid", "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.", http.StatusUnprocessableEntity},
		{errorObjectiveInvalid, "ObjectiveInvalid", "Use 1–240 characters without terminal controls.", http.StatusUnprocessableEntity},
		{errorSessionNameConflict, "SessionNameConflict", "A session with that name already exists.", http.StatusConflict},
		{errorSessionNotFound, "SessionNotFound", "That session no longer exists.", http.StatusNotFound},
		{errorSessionIdentityMismatch, "SessionIdentityMismatch", "The session changed. Refresh and try again.", http.StatusConflict},
		{errorPairingInviteRejected, "PairingInviteRejected", "This fleet invite is invalid, expired, or already used.", http.StatusUnauthorized},
		{errorMachineIdentityMismatch, "MachineIdentityMismatch", "The machine identity changed. Fleet reset is required.", http.StatusConflict},
		{errorSessionGroupedConflict, "SessionGroupedConflict", "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.", http.StatusConflict},
		{errorInternal, "InternalError", "Skíðblaðnir could not complete the request.", http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeError(recorder, test.failure)
			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
			if got := response.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("error response content type = %q, want application/json", got)
			}
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if len(body) != 2 || body["code"] != test.code || body["message"] != test.message {
				t.Fatalf(
					"error response does not match exact contract: field_count=%d code_match=%t message_match=%t",
					len(body),
					body["code"] == test.code,
					body["message"] == test.message,
				)
			}
		})
	}
}

func TestSessionPathParserIsSharedAndExact(t *testing.T) {
	for _, valid := range []struct {
		path string
		id   string
	}{
		{path: "/v1/sessions/$7", id: "$7"},
		{path: "/v1/sessions/opaque", id: "opaque"},
	} {
		id, ok := parseSessionPath(valid.path)
		if !ok || id != valid.id {
			t.Fatalf("parse session path %q = (%q, %t), want (%q, true)", valid.path, id, ok, valid.id)
		}
	}
	for _, invalid := range []string{"", "/v1/sessions", "/v1/sessions/", "/v1/sessions/$7/terminal", "/v1/sessions/$7/extra"} {
		if id, ok := parseSessionPath(invalid); ok {
			t.Fatalf("invalid session path %q parsed as %q", invalid, id)
		}
	}
}

func TestRenameRequestRejectsDuplicateRequiredStringKey(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/sessions/$7",
		strings.NewReader(`{"tmuxName":"current","newTmuxName":"first","newTmuxName":"second","identityToken":"token"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	_, failure := decodeJSON[renameSessionRequest](httptest.NewRecorder(), request)
	if failure == nil || failure.Code != errorInvalidRequest.Code || failure.Status != errorInvalidRequest.Status {
		t.Fatalf("duplicate required rename key failure = %#v, want InvalidRequest", failure)
	}
}

func TestRenameRequestRejectsAlternateCaseRequiredKeys(t *testing.T) {
	for _, body := range []string{
		`{"TmuxName":"current","newTmuxName":"next","identityToken":"token"}`,
		`{"tmuxName":"current","NewTmuxName":"next","identityToken":"token"}`,
		`{"tmuxName":"current","newTmuxName":"next","IdentityToken":"token"}`,
	} {
		request := httptest.NewRequest(http.MethodPatch, "/v1/sessions/$7", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		_, failure := decodeJSON[renameSessionRequest](httptest.NewRecorder(), request)
		if failure == nil || failure.Code != errorInvalidRequest.Code || failure.Status != errorInvalidRequest.Status {
			t.Fatalf("alternate-case rename key failure = %#v, want InvalidRequest", failure)
		}
	}
}
