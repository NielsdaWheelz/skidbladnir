// Package api declares the gateway's json documents and the vocabulary they carry:
// terminal status, displayed usage and footer, name mode, recovery and account quota, with
// the host's copy of status and recovery. It is the one shape the gateway encodes and go
// clients decode, and it imports no internal package. A struct's field order is its key
// order on the wire; instants are UTC. Producers own the values' invariants; consumers
// decode tolerantly and check only what they branch on (docs/rules.md). pressure.Snapshot
// and workdir's Listing and SearchResult are declared by their producers.
package api

import "time"

// Machine names the answering gateway; a client compares Handle with the machine it addressed.
type Machine struct {
	Handle   string `json:"handle"`
	Platform string `json:"platform"`
}

// Inventory answers GET /v1/sessions. Profiles and Sessions are never null; Sessions are
// ordered by numeric tmux id.
type Inventory struct {
	Machine    Machine           `json:"machine"`
	ObservedAt time.Time         `json:"observedAt"`
	Recovery   InventoryRecovery `json:"recovery"`
	Profiles   []Profile         `json:"profiles"`
	Sessions   []Session         `json:"sessions"`
}

// InventoryRecovery is the host's recovery fact with its wording of it, Recovery.Notice, for
// the phone; go clients word the fact themselves.
type InventoryRecovery struct {
	Recovery
	HostNotice *RecoveryNotice `json:"notice,omitempty"`
}

type Profile struct {
	AccountScope string `json:"accountScope,omitempty"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
}

type Session struct {
	TerminalStatus  TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *TerminalFooter `json:"terminalFooter,omitempty"`
	TmuxID          string          `json:"tmuxId"`
	ActivePaneID    string          `json:"activePaneId"`
	TmuxName        string          `json:"tmuxName"`
	NameMode        NameMode        `json:"nameMode"`
	IdentityToken   string          `json:"identityToken"`
	Character       Character       `json:"character"`
	LaunchProfile   string          `json:"launchProfile,omitempty"`
	Agent           *Agent          `json:"agent,omitempty"`
	Connection      *Connection     `json:"connection,omitempty"`
	Objective       string          `json:"objective,omitempty"`
	Group           string          `json:"group,omitempty"`
	CWD             string          `json:"cwd,omitempty"`
	ActiveCommand   string          `json:"activeCommand,omitempty"`
	AttachedClients int             `json:"attachedClients"`

	// Status is the host's copy of TerminalStatus, PresentStatus, for the phone; go clients
	// render it themselves.
	Status StatusPresentation `json:"status"`
}

type NameMode string

const (
	NameAutomatic NameMode = "automatic"
	NameManual    NameMode = "manual"
)

type Character struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type Agent struct {
	PaneID        string `json:"paneId"`
	StartIdentity string `json:"startIdentity"`
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	Profile       string `json:"profile,omitempty"`
	Label         string `json:"label,omitempty"` // Profile's configured label
}

type Connection struct {
	Transport string `json:"transport"`
	ID        string `json:"id,omitempty"`
}

// ObservedSession answers session and shell creation.
type ObservedSession struct {
	ObservedAt time.Time `json:"observedAt"`
	Session    Session   `json:"session"`
}

// Inspect answers terminal/inspect. Diagnostics are present only when explained.
type Inspect struct {
	TerminalStatus TerminalStatus `json:"terminalStatus"`
	Diagnostics    *Diagnostics   `json:"diagnostics,omitempty"`
}

// Diagnostics name the grammar rules that decided one inspect sample, in decision order; Rules is
// never null on the wire.
type Diagnostics struct {
	Rules []string `json:"rules"`
}

type ReadResult struct {
	Text      string `json:"text"`
	Source    string `json:"source"`
	Scope     string `json:"scope"`
	Truncated bool   `json:"truncated"`
}

type WriteResult struct {
	Method  string `json:"method"`
	Outcome string `json:"outcome"`
}

type CloseResult struct {
	Interrupt string `json:"interrupt"`
	Terminal  string `json:"terminal"`
}

type TerminalContext struct {
	ObservedAt time.Time     `json:"observedAt"`
	CWD        string        `json:"cwd,omitempty"`
	Agent      *ContextAgent `json:"agent,omitempty"`
	Connection *Connection   `json:"connection,omitempty"`
}

type ContextAgent struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile,omitempty"`
	Label    string `json:"label,omitempty"` // Profile's configured label
}

// ProfileUsageReport answers GET /v1/profile-usage. Profiles is never null.
type ProfileUsageReport struct {
	Machine    Machine        `json:"machine"`
	ObservedAt time.Time      `json:"observedAt"`
	Profiles   []ProfileUsage `json:"profiles"`
}

type ProfileUsage struct {
	Profile
	ReadState string       `json:"readState"`
	Report    *QuotaReport `json:"report,omitempty"`
}

// QuotaWindow and QuotaReport are one codex profile's account quota as internal/quota reads it.
type QuotaWindow struct {
	UsedPercent float64    `json:"usedPercent"`
	ResetsAt    *time.Time `json:"resetsAt,omitempty"`
}

type QuotaReport struct {
	ReportedAt time.Time    `json:"reportedAt"`
	FiveHour   *QuotaWindow `json:"fiveHour,omitempty"`
	SevenDay   *QuotaWindow `json:"sevenDay,omitempty"`
}

// Error is every gateway error envelope. Dispatch not_sent: no effect happened; unknown: an
// effect may have happened, never replay; absent: the client infers. Target names a created
// terminal whose creation follow-up failed.
type Error struct {
	Target   *Target `json:"target,omitempty"`
	Dispatch string  `json:"dispatch,omitempty"`
	Code     string  `json:"code"`
	Message  string  `json:"message"`
}

// TerminalInputBlocked messages name why a guarded send was refused. The gateway
// sends them; skid start's readiness check refuses locally with the same words.
const (
	InputBlockedDraft    = "terminal contains a draft. open it before sending."
	InputBlockedDialog   = "respond to the dialog in the terminal."
	InputBlockedNotReady = "the requested agent is not ready for its initial prompt. inspect the terminal before sending."
)

type Target struct {
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}
