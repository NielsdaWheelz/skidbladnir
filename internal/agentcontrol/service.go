package agentcontrol

import (
	"errors"
	"path/filepath"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var ErrInvalidInput = errors.New("invalid terminal input")

type Service struct {
	sessions      *sessions.Manager
	quotaReadPath string
}

func New(manager *sessions.Manager, quotaReadPath string) (*Service, error) {
	if manager == nil || !filepath.IsAbs(quotaReadPath) {
		return nil, errors.New("agent control requires sessions and an absolute quota command")
	}
	return &Service{sessions: manager, quotaReadPath: quotaReadPath}, nil
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
