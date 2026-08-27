package gateway

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestSessionProjectionUsesTmuxNameAndRequiredCharacterWithoutWideningStatus(t *testing.T) {
	observedAt := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	character := catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"}
	card, err := mapSession(sessions.Session{
		ID:              "$7",
		TmuxName:        "laptop-work",
		IdentityToken:   "v1-lifetime",
		Character:       character,
		Attention:       true,
		AttachedClients: 2,
		Status: sessions.Status{
			Kind:     sessions.StatusRunning,
			Signal:   sessions.StatusSignalProcess,
			SignalAt: observedAt,
		},
	}, observedAt)
	if err != nil {
		t.Fatalf("project running session: %v", err)
	}
	if card.TmuxName != "laptop-work" || card.Character != (characterDTO{Key: character.Key, DisplayName: character.DisplayName}) {
		t.Fatalf("session identity projection = %+v, want tmux name and required character", card)
	}
	if !card.Attention || card.Status.Kind != "Running" || card.Status.Signal != "Process" {
		t.Fatalf("attention replaced or widened status: %+v", card)
	}
	payload, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("encode projected card: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode projected card: %v", err)
	}
	if fields["tmuxName"] != "laptop-work" || fields["character"] == nil {
		t.Fatalf("wire projection omitted required identity fields: %s", payload)
	}
	if _, exists := fields["name"]; exists {
		t.Fatalf("wire projection retained the retired name field: %s", payload)
	}
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
		{errorSessionIdentityMismatch, "SessionIdentityMismatch", "The session changed. Refresh before killing it.", http.StatusConflict},
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
