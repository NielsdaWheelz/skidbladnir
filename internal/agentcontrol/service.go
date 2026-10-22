package agentcontrol

import (
	"errors"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var ErrInvalidInput = errors.New("invalid terminal input")

type Service struct {
	sessions *sessions.Manager
	mu       sync.Mutex
	reused   map[sessions.ScreenKey]detection // a3: the last inventory's settled batch detections
	last     time.Time                        // the previous inventory enrichment's start (wall and monotonic)
}

func New(manager *sessions.Manager) *Service {
	return &Service{sessions: manager}
}
