package agentcontrol

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

// ObservationFailure is a content-free record of one failed inventory
// observation: its unavailable reason and elapsed time.
type ObservationFailure struct {
	TmuxID  string
	Reason  api.StatusReason
	Elapsed time.Duration
}
