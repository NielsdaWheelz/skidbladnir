package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

func TestSessionProjectionKeepsAttentionIndependentFromLifecycleStatus(t *testing.T) {
	observedAt := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	card, err := mapSession(sessions.Session{
		ID:              "$7",
		Name:            "ga-durinn",
		IdentityToken:   "v1-lifetime",
		Attention:       true,
		AttachedClients: 2,
		Status: sessions.Status{
			Kind:     sessions.StatusRunning,
			Signal:   sessions.StatusSignalProcess,
			SignalAt: observedAt,
		},
	}, observedAt)
	if err != nil {
		t.Fatal("project running session")
	}
	if !card.Attention || card.Status.Kind != "Running" || card.Status.Signal != "Process" {
		t.Fatalf("attention replaced or widened status: attention=%t kind=%s signal=%s", card.Attention, card.Status.Kind, card.Status.Signal)
	}
}

func TestPressureProjectionPartitionsEveryMetricByPlatform(t *testing.T) {
	allMetrics := []pressure.Metric{
		pressure.MetricCPUPercent,
		pressure.MetricCPUPressureSomeAvg60,
		pressure.MetricDiskAvailablePercent,
		pressure.MetricInputOutputPressureFullAvg60,
		pressure.MetricMemoryAvailablePercent,
		pressure.MetricMemoryPressure,
		pressure.MetricMemoryPressureFullAvg60,
		pressure.MetricLoadNormalized,
		pressure.MetricSwapUsedPercent,
	}
	slices.Sort(allMetrics)
	for _, testCase := range []struct {
		name        string
		unsupported []pressure.Metric
	}{
		{name: "Linux", unsupported: []pressure.Metric{pressure.MetricMemoryPressure}},
		{name: "Darwin", unsupported: []pressure.Metric{
			pressure.MetricCPUPressureSomeAvg60,
			pressure.MetricInputOutputPressureFullAvg60,
			pressure.MetricMemoryAvailablePercent,
			pressure.MetricMemoryPressureFullAvg60,
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			unsupported, unsupportedSet, err := mapUnsupportedMetrics(testCase.unsupported)
			if err != nil {
				t.Fatal("map unsupported metrics")
			}
			unique := slices.Compact(append([]pressure.Metric(nil), unsupported...))
			if !slices.IsSorted(unsupported) || len(unique) != len(unsupported) {
				t.Fatal("unsupported metrics are not sorted and unique")
			}
			mapped, err := mapHostSample(pressure.Sample{
				ObservedAt: time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC),
				Status:     pressure.StatusUnknown,
			}, unsupportedSet)
			if err != nil {
				t.Fatal("map absent sample")
			}
			partition := append(append([]pressure.Metric(nil), mapped.Missing...), unsupported...)
			slices.Sort(partition)
			if !slices.Equal(partition, allMetrics) {
				t.Fatalf("missing + unsupported = %v, want complete metric universe %v", partition, allMetrics)
			}
			for _, metric := range mapped.Missing {
				if _, found := unsupportedSet[metric]; found {
					t.Fatalf("metric %q is both missing and unsupported", metric)
				}
			}
		})
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
			if response.Header.Get("Content-Type") != "application/json" {
				t.Fatal("error response content type is not application/json")
			}
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal("decode error response")
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
