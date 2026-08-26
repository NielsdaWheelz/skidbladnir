package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		t.Fatalf("project running session: %v", err)
	}
	if !card.Attention || card.Status.Kind != "Running" || card.Status.Signal != "Process" {
		t.Fatalf("attention replaced or widened status: %+v", card)
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
				t.Fatalf("content type = %q, want application/json", response.Header.Get("Content-Type"))
			}
			var body map[string]string
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if len(body) != 2 || body["code"] != test.code || body["message"] != test.message {
				t.Fatalf("body = %#v, want exact code and message", body)
			}
		})
	}
}
