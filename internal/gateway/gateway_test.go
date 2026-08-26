package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

type stubSessionManager struct {
	created sessions.Session
}

func (stub stubSessionManager) Profiles() []sessions.Profile { return nil }

func (stub stubSessionManager) List(context.Context) ([]sessions.Session, error) {
	return nil, nil
}

func (stub stubSessionManager) Create(context.Context, sessions.CreateInput) (sessions.Session, error) {
	return stub.created, nil
}

func (stub stubSessionManager) ValidateKill(context.Context, sessions.KillInput) error { return nil }

func (stub stubSessionManager) Kill(context.Context, sessions.KillInput) error { return nil }

func (stub stubSessionManager) ValidateTerminal(context.Context, string, string) error { return nil }

func (stub stubSessionManager) OpenTerminal(context.Context, string, string) (*sessions.TerminalAttachment, error) {
	return nil, nil
}

// A create whose post-create tmux readback degraded returns a card with the
// profile fact absent; the response and its log line must still complete
// instead of treating the degraded card as a logging defect.
func TestCreateSessionWithDegradedReadbackRespondsCreated(t *testing.T) {
	degraded := sessions.Session{
		ID:            "$7",
		TmuxName:      "laptop-work",
		IdentityToken: "v1-0123456789abcdef0123456789abcdef.1234.5678.7",
		Character:     catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"},
		Status: sessions.Status{
			Kind:     sessions.StatusUnknown,
			Signal:   sessions.StatusSignalPollFailure,
			SignalAt: time.Now().UTC(),
		},
	}
	var logs bytes.Buffer
	gateway := New(Config{Sessions: stubSessionManager{created: degraded}, Logger: logging.New(&logs)})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/v1/sessions",
		strings.NewReader(`{"cwd":"/home/niels","profile":"personal"}`))
	request.Header.Set("Content-Type", "application/json")
	gateway.createSession(recorder, request)

	if recorder.Code != 201 {
		t.Fatalf("degraded create readback answered %d, want 201", recorder.Code)
	}
	var card sessionDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &card); err != nil {
		t.Fatalf("create response is not a session card: %v", err)
	}
	if card.ID != "$7" || card.TmuxName != "laptop-work" || card.Character.Key != "norse.durinn" ||
		card.Profile != "" || card.Status.Kind != "Unknown" {
		t.Fatalf("degraded card misreported facts: %+v", card)
	}
	if !strings.Contains(logs.String(), "personal") {
		t.Fatalf("create log line lost the requested profile: %s", logs.String())
	}
}
