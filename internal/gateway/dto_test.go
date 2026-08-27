package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
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
// schema; each value is also the metric's exact JSON key under `metrics`.
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

func TestPressureResponsePartitionsEveryMetricOfTheDeclaredHostCapability(t *testing.T) {
	host := platform.Current().Kind
	monitor := pressure.NewMonitor()
	want, declared := declaredUnsupportedByPlatform[host], monitor.Unsupported()
	if !slices.Equal(declared, want) {
		t.Fatalf("%s declared unsupported capability = %v, want %v", host, declared, want)
	}
	if !slices.IsSorted(declared) || len(slices.Compact(append([]pressure.Metric(nil), declared...))) != len(declared) {
		t.Fatalf("%s declared unsupported capability is not sorted and unique: %v", host, declared)
	}

	unsupported, unsupportedSet := mapUnsupportedMetrics(monitor.Unsupported())
	snapshot := monitor.Snapshot()
	current, err := mapHostSample(snapshot.Current, unsupportedSet)
	if err != nil {
		t.Fatalf("project the observed %s pressure sample: %v", host, err)
	}
	payload, err := json.Marshal(pressureResponseDTO{Unsupported: unsupported, Current: current, History: []hostSampleDTO{current}})
	if err != nil {
		t.Fatalf("encode the pressure response: %v", err)
	}
	var body struct {
		Unsupported []pressure.Metric `json:"unsupported"`
		Current     struct {
			Metrics map[string]json.RawMessage `json:"metrics"`
			Missing []pressure.Metric          `json:"missing"`
		} `json:"current"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("decode the pressure response: %v", err)
	}

	if !slices.Equal(body.Unsupported, want) {
		t.Fatalf("%s response unsupported = %v, want the declared capability %v", host, body.Unsupported, want)
	}
	for _, metric := range body.Current.Missing {
		if slices.Contains(body.Unsupported, metric) {
			t.Fatalf("metric %q is reported both missing and unsupported on %s", metric, host)
		}
	}
	absent := make([]pressure.Metric, 0, len(pressureMetricUniverse))
	for _, metric := range pressureMetricUniverse {
		if _, published := body.Current.Metrics[string(metric)]; !published {
			absent = append(absent, metric)
		}
	}
	if len(body.Current.Metrics)+len(absent) != len(pressureMetricUniverse) {
		t.Fatalf(
			"published metric keys escape the closed union on %s: published_count=%d absent_count=%d union_count=%d",
			host, len(body.Current.Metrics), len(absent), len(pressureMetricUniverse),
		)
	}
	partition := append(append([]pressure.Metric(nil), body.Current.Missing...), body.Unsupported...)
	slices.Sort(partition)
	if !slices.Equal(partition, absent) {
		t.Fatalf("%s missing + unsupported = %v, want exactly the absent metric keys %v", host, partition, absent)
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
		{errorMachineIdentityMismatch, "MachineIdentityMismatch", "The machine identity changed. Provisioning repair is required.", http.StatusConflict},
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
