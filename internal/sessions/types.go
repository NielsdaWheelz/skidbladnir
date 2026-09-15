package sessions

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Config struct {
	TmuxPath      string
	SocketName    string
	Workdir       *workdir.Service
	CataloguePath string
	Profiles      []agentruntime.Profile
}

type CreateInput struct {
	CWD              string
	Profile          string
	OptionalTmuxName string
	Objective        string
	Space            space.Label
}

type SetSpaceInput struct {
	TmuxID        string
	IdentityToken string
	Space         space.Label
}

type KillInput struct {
	TmuxID        string
	TmuxName      string
	IdentityToken string
}

type OpenTerminalInput struct {
	TmuxID        string
	IdentityToken string
	Columns       int
	Rows          int
}

type RenameInput struct {
	TmuxID        string
	TmuxName      string
	NewTmuxName   string
	IdentityToken string
}

type Session struct {
	foreground      *processinfo.Observation
	TmuxID          string
	TmuxName        string
	IdentityToken   string
	LaunchProfile   agentruntime.ProfileKey
	Agent           *agentruntime.AgentRuntime
	Objective       string
	Space           space.Label
	Character       catalog.Character
	CWD             string
	ActiveCommand   string
	AttachedClients int
}

type Inventory struct {
	ObservedAt time.Time
	Sessions   []Session
}

type ObservedSession struct {
	ObservedAt time.Time
	Session    Session
}

// ValidProjectionInstant closes observation clocks at the same four-digit UTC
// year boundary as Go's RFC3339 JSON encoder.
func ValidProjectionInstant(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	_, err := value.UTC().MarshalJSON()
	return err == nil
}

type ErrorCode string

const (
	ErrorWorkingDirectoryInvalid     ErrorCode = "WorkingDirectoryInvalid"
	ErrorWorkingDirectoryUnavailable ErrorCode = "WorkingDirectoryUnavailable"
	ErrorProfileUnknown              ErrorCode = "ProfileUnknown"
	ErrorSessionNameInvalid          ErrorCode = "SessionNameInvalid"
	ErrorObjectiveInvalid            ErrorCode = "ObjectiveInvalid"
	ErrorSessionNameConflict         ErrorCode = "SessionNameConflict"
	ErrorSessionNotFound             ErrorCode = "SessionNotFound"
	ErrorSessionIdentityMismatch     ErrorCode = "SessionIdentityMismatch"
)

type Error struct {
	Code    ErrorCode
	Message string
}

func (err *Error) Error() string { return err.Message }
