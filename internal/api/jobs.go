package api

import (
	"encoding/hex"
	"strings"
)

const (
	JobAnswerLimit = 16 * 1024 * 1024
	JobPageSize    = 32 * 1024
)

type JobRequest struct {
	Profile string  `json:"profile,omitempty"`
	CWD     string  `json:"cwd,omitempty"`
	Brief   string  `json:"brief"`
	Model   *string `json:"model,omitempty"`
	Effort  *string `json:"effort,omitempty"`
	After   string  `json:"after,omitempty"`
}

type JobTerminal struct {
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

type JobState string

const (
	JobAccepted    JobState = "accepted"
	JobAnswered    JobState = "answered"
	JobFailed      JobState = "failed"
	JobInterrupted JobState = "interrupted"
	JobUnknown     JobState = "unknown"
)

func (state JobState) Terminal() bool {
	return state == JobAnswered || state == JobFailed || state == JobInterrupted || state == JobUnknown
}

type JobReceipt struct {
	JobID    string       `json:"job_id"`
	State    JobState     `json:"state"`
	Reason   string       `json:"reason,omitempty"`
	Terminal *JobTerminal `json:"terminal,omitempty"`
}

type JobResult struct {
	JobReceipt
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
	Offset int    `json:"offset"`
	Page   []byte `json:"page"`
}

// JobIDBytes accepts the canonical lowercase UUID spelling used by the action owner.
func JobIDBytes(value string) ([]byte, bool) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || strings.ToLower(value) != value {
		return nil, false
	}
	data, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return data, err == nil && len(data) == 16
}

type PeerForeground struct {
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

type PeerAddress struct {
	Sender string `json:"sender"`
	Digest string `json:"digest"`
}
