package agentcontrol

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// ObservationFailure is a content-free record of one failed inventory
// observation: its unavailable reason and elapsed time.
type ObservationFailure struct {
	TmuxID  string
	Reason  sessions.StatusReason
	Elapsed time.Duration
}

// Diagnostics explain one observation without terminal content. Rules name
// at most eight matched content-free rules; Capture is present only when a
// screen was captured; ElapsedMs holds only the stages that were performed.
type Diagnostics struct {
	Rules     []DiagnosticRule    `json:"rules"`
	Capture   *CaptureDiagnostics `json:"capture,omitempty"`
	ElapsedMs StageElapsed        `json:"elapsedMs"`
}

type DiagnosticRegion string

const (
	DiagnosticTop      DiagnosticRegion = "top"
	DiagnosticBottom   DiagnosticRegion = "bottom"
	DiagnosticCompound DiagnosticRegion = "compound"
)

// DiagnosticRule.ID is at most 48 ASCII characters from [a-z0-9_.-].
type DiagnosticRule struct {
	ID     string           `json:"id"`
	Region DiagnosticRegion `json:"region"`
}

type CaptureDiagnostics struct {
	Width         int  `json:"width"`
	Height        int  `json:"height"`
	Alternate     bool `json:"alternate"`
	TopClipped    bool `json:"topClipped"`
	BottomClipped bool `json:"bottomClipped"`
}

// StageElapsed values are integer milliseconds in 0..2147483647.
type StageElapsed struct {
	Resolve  *int64 `json:"resolve,omitempty"`
	Capture  *int64 `json:"capture,omitempty"`
	Classify *int64 `json:"classify,omitempty"`
}
