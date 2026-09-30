package sessions

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Config struct {
	TmuxPath      string
	SocketName    string
	Workdir       *workdir.Service
	CataloguePath string
	Profiles      []agentruntime.Profile
}

type LaunchKind string

const (
	LaunchAgent    LaunchKind = "agent"
	LaunchTerminal LaunchKind = "terminal"
)

type CreateInput struct {
	Kind             LaunchKind
	CWD              string
	Profile          string
	OptionalTmuxName string
	Objective        string
	Group            group.Label
	Conversation     *agentruntime.Conversation
}

type ShellInput struct {
	TmuxID        string
	IdentityToken string
}

type SetGroupInput struct {
	TmuxID        string
	IdentityToken string
	Group         group.Label
}

type KillInput struct {
	TmuxID        string
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
	ActivePaneID    string
	TmuxName        string
	IdentityToken   string
	LaunchProfile   agentruntime.ProfileKey
	Agent           *agentruntime.AgentRuntime
	Conversation    *agentruntime.Conversation
	TerminalStatus  TerminalStatus
	Connection      *Connection
	Objective       string
	Group           group.Label
	Character       catalog.Character
	CWD             string
	ActiveCommand   string
	AttachedClients int
}

type TerminalStatus struct {
	State  string `json:"state"`
	Source string `json:"source"`
}

func (status TerminalStatus) Valid() bool {
	switch status.State {
	case "working", "blocked", "idle", "unknown":
	default:
		return false
	}
	return status.Source == "terminal" || status.Source == "unavailable" && status.State == "unknown"
}

type Connection struct {
	Transport string
	ID        string
}

type RemoteAgent struct {
	Provider agentruntime.Provider
	Profile  agentruntime.ProfileKey
}

type TerminalContext struct {
	ObservedAt time.Time
	CWD        string
	Agent      *RemoteAgent
	Connection *Connection
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
