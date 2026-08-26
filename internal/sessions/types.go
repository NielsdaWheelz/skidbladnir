package sessions

import "time"

type Config struct {
	TmuxPath      string
	SocketName    string
	Home          string
	CataloguePath string
	Profiles      []Profile
}

type ForegroundSignature struct {
	ExecutableBase string
	Argument0      string
	Argument1      string
}

type EnvironmentVariable struct {
	Name  string
	Value string
}

type Profile struct {
	Key                  string
	Label                string
	Command              string
	Environment          []EnvironmentVariable
	ForegroundSignatures []ForegroundSignature
	Arguments            []string
}

type CreateInput struct {
	CWD          string
	Profile      string
	OptionalName string
	Objective    string
}

type KillInput struct {
	ID            string
	DisplayedName string
	IdentityToken string
}

type Session struct {
	ID                   string
	Name                 string
	IdentityToken        string
	Profile              string
	Objective            string
	CharacterKey         string
	CharacterDisplayName string
	CWD                  string
	ActiveCommand        string
	AttachedClients      int
	Status               Status
	Attention            bool
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
