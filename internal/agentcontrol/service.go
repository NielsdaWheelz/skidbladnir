package agentcontrol

import (
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var ErrInvalidInput = errors.New("invalid terminal input")

type Service struct {
	sessions      *sessions.Manager
	quotaReadPath string
}

func New(manager *sessions.Manager, quotaReadPath string) *Service {
	return &Service{sessions: manager, quotaReadPath: quotaReadPath}
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
