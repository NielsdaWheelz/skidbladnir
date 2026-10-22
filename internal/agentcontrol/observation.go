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

// Diagnostics explain one inspect sample without terminal content: the ids of
// the grammar rules that decided it, in decision order.
type Diagnostics struct {
	Rules []string `json:"rules"`
}
