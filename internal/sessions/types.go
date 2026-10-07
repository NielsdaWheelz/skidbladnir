package sessions

import (
	"errors"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Config struct {
	TmuxPath       string
	SocketName     string
	Workdir        *workdir.Service
	CataloguePath  string
	Profiles       []agentruntime.Profile
	Machine        machine.Handle
	CheckpointPath string
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
	Model            string
	Effort           string
	preparedName     string
	OptionalTmuxName string
	Objective        string
	Group            group.Label
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

type NameMode string

const (
	NameAutomatic NameMode = "automatic"
	NameManual    NameMode = "manual"
)

type Naming struct {
	Mode NameMode
	Name string
}
type RenameInput struct {
	TmuxID         string
	IdentityToken  string
	ExpectedNaming Naming
	Naming         Naming
}

type Session struct {
	panePID          processinfo.PID
	foreground       *processinfo.Observation
	foregroundFailed bool
	TmuxID           string
	ActivePaneID     string
	TmuxName         string
	NameMode         NameMode
	IdentityToken    string
	LaunchProfile    agentruntime.ProfileKey
	Agent            *agentruntime.AgentRuntime
	Conversation     *agentruntime.Conversation
	TerminalStatus   TerminalStatus // zero (invalid) until agentcontrol.Enrich observes the session
	Connection       *Connection
	Objective        string
	Group            group.Label
	Character        catalog.Character
	CWD              string
	ActiveCommand    string
	AttachedClients  int
}

// ForegroundFailed reports that this sample could not identify the pane's
// foreground process. Agent and Connection are then unknown, not absent.
func (session Session) ForegroundFailed() bool { return session.foregroundFailed }

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
	Recovery   Recovery
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

// ParseProjectionInstant admits exactly the writer's UTC RFC3339Nano spelling.
// time.Parse alone normalizes offsets and noncanonical fractional seconds.
func ParseProjectionInstant(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || !ValidProjectionInstant(value) || raw != value.UTC().Format(time.RFC3339Nano) {
		return time.Time{}, errors.New("invalid projection instant")
	}
	return value, nil
}

type ErrorCode string

const (
	ErrorWorkingDirectoryInvalid     ErrorCode = "WorkingDirectoryInvalid"
	ErrorWorkingDirectoryUnavailable ErrorCode = "WorkingDirectoryUnavailable"
	ErrorProfileUnknown              ErrorCode = "ProfileUnknown"
	ErrorSessionNameInvalid          ErrorCode = "SessionNameInvalid"
	ErrorObjectiveInvalid            ErrorCode = "ObjectiveInvalid"
	ErrorSessionNameChanged          ErrorCode = "SessionNameChanged"
	ErrorSessionNameConflict         ErrorCode = "SessionNameConflict"
	ErrorSessionNotFound             ErrorCode = "SessionNotFound"
	ErrorSessionIdentityMismatch     ErrorCode = "SessionIdentityMismatch"
)

type Error struct {
	Code    ErrorCode
	Message string
}

func (err *Error) Error() string { return err.Message }
