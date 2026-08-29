package sessions

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
)

type Config struct {
	TmuxPath      string
	SocketName    string
	Home          string
	CataloguePath string
	Profiles      []agentruntime.Profile
}

type CreateInput struct {
	CWD              string
	Profile          string
	OptionalTmuxName string
	Objective        string
}

type KillInput struct {
	TmuxID        string
	TmuxName      string
	IdentityToken string
}

type RenameInput struct {
	TmuxID        string
	TmuxName      string
	NewTmuxName   string
	IdentityToken string
}

type Session struct {
	TmuxID          string
	TmuxName        string
	IdentityToken   string
	LaunchProfile   agentruntime.ProfileKey
	Agent           *agentruntime.AgentRuntime
	Objective       string
	Character       catalog.Character
	CWD             string
	ActiveCommand   string
	AttachedClients int
	Status          Status
	Attention       bool
}

type Status struct {
	Kind     StatusKind
	Signal   StatusSignal
	SignalAt time.Time
}

type StatusKind string

const (
	StatusWorking StatusKind = "WORKING"
	StatusRunning StatusKind = "RUNNING"
	StatusIdle    StatusKind = "IDLE"
	StatusShell   StatusKind = "SHELL"
	StatusUnknown StatusKind = "UNKNOWN"
)

type StatusSignal string

const (
	StatusSignalLifecycle   StatusSignal = "Lifecycle"
	StatusSignalProcess     StatusSignal = "Process"
	StatusSignalPollFailure StatusSignal = "PollFailure"
)

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
	ErrorSessionGroupedConflict      ErrorCode = "SessionGroupedConflict"
)

type Error struct {
	Code    ErrorCode
	Message string
}

func (err *Error) Error() string { return err.Message }
