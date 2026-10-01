package agentcontrol

import (
	"math"
	"strings"
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

// maxDiagnosticRules bounds the rules one observation reports.
const maxDiagnosticRules = 8

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

// Valid owns the diagnostics bounds: at most eight rules with content-free
// ids and a known region, positive captured dimensions, and stage times that
// fit the wire's integer milliseconds.
func (diagnostics Diagnostics) Valid() bool {
	if len(diagnostics.Rules) > maxDiagnosticRules {
		return false
	}
	for _, rule := range diagnostics.Rules {
		if rule.ID == "" || len(rule.ID) > 48 || strings.Trim(rule.ID, "abcdefghijklmnopqrstuvwxyz0123456789_.-") != "" {
			return false
		}
		switch rule.Region {
		case DiagnosticTop, DiagnosticBottom, DiagnosticCompound:
		default:
			return false
		}
	}
	if capture := diagnostics.Capture; capture != nil && (capture.Width < 1 || capture.Height < 1) {
		return false
	}
	for _, elapsed := range []*int64{diagnostics.ElapsedMs.Resolve, diagnostics.ElapsedMs.Capture, diagnostics.ElapsedMs.Classify} {
		if elapsed != nil && (*elapsed < 0 || *elapsed > math.MaxInt32) {
			return false
		}
	}
	return true
}
