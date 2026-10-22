package agentcontrol

import (
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var ErrInvalidInput = errors.New("invalid terminal input")

type Service struct {
	sessions *sessions.Manager
}

func New(manager *sessions.Manager) *Service {
	return &Service{sessions: manager}
}

type ReadResult struct {
	Text      string `json:"text"`
	Source    string `json:"source"`
	Scope     string `json:"scope"`
	Truncated bool   `json:"truncated"`
}
type WriteResult struct {
	Method  string `json:"method"`
	Outcome string `json:"outcome"`
}
