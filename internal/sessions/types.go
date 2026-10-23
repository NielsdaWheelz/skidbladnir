package sessions

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Config struct {
	TmuxPath       string
	ShellPath      string // a hostconfig.Resolve shell; sessions trusts it
	SocketName     string
	Workdir        *workdir.Service
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

type Naming struct {
	Mode api.NameMode
	Name string
}
type RenameInput struct {
	TmuxID         string
	IdentityToken  string
	ExpectedNaming Naming
	Naming         Naming
}

type Session struct {
	screen           tmuxclient.ScreenTarget // the selected pane's target, pid and screen as the row read them
	activity         int64                   // the row's window_activity (unix seconds)
	foreground       *processinfo.Observation
	foregroundFailed bool
	TmuxID           string
	ActivePaneID     string
	TmuxName         string
	NameMode         api.NameMode
	IdentityToken    string
	LaunchProfile    agentruntime.ProfileKey
	Agent            *agentruntime.Agent
	TerminalStatus   api.TerminalStatus // zero (invalid) until agentcontrol observes the session
	TerminalUsage    *api.TerminalUsage
	TerminalFooter   *api.TerminalFooter
	Connection       *Connection
	Objective        string
	Group            group.Label
	Character        catalog.Entry
	CWD              string
	ActiveCommand    string
	AttachedClients  int
}

// ForegroundFailed reports that this sample could not identify the pane's
// foreground process. Agent and Connection are then unknown, not absent.
func (session Session) ForegroundFailed() bool { return session.foregroundFailed }

// ScreenKey is an agent session's screen as its inventory row and foreground
// sample proved it (tmux-exec-budget.md a3): lifetime, session, selected pane,
// pane pid, size, alternate screen, the whole foreground sample, its provider
// and the window's last output. Equal keys with no output after a capture
// prove the capture still shows the screen.
type ScreenKey struct {
	screen     tmuxclient.ScreenTarget
	foreground processinfo.Observation
	provider   agentruntime.Provider
	activity   int64
}

// Activity is the window's last output, at tmux's one-second resolution.
func (key ScreenKey) Activity() time.Time { return time.Unix(key.activity, 0) }

// ScreenKey is defined only for a session with an Agent.
func (session Session) ScreenKey() ScreenKey {
	return ScreenKey{screen: session.screen, foreground: *session.foreground, provider: session.Agent.Provider, activity: session.activity}
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
	Profiles   []agentruntime.Profile // the one settings snapshot every LaunchProfile and Agent.Profile names
	Recovery   api.Recovery
	Server     tmuxclient.ServerIdentity // the one admitted lifetime of every row; zero with no rows
}

type ObservedSession struct {
	ObservedAt time.Time
	Session    Session
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
