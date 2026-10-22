// Package api declares the gateway's json documents: the one shape the gateway encodes and go
// clients decode. A struct's field order is its key order on the wire; instants are UTC.
// Producers own the values' invariants; consumers decode tolerantly and check only what they
// branch on (docs/rules.md). pressure.Snapshot and workdir's Listing and SearchResult are
// declared by their producers; the terminal status, usage, footer, name mode and recovery
// vocabulary is still declared in sessions.
package api

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// Machine names the answering gateway; a client compares Handle with the machine it addressed.
type Machine struct {
	Handle   string `json:"handle"`
	Platform string `json:"platform"`
}

// Inventory answers GET /v1/sessions. Profiles and Sessions are never null; Sessions are
// ordered by numeric tmux id.
type Inventory struct {
	Machine    Machine   `json:"machine"`
	ObservedAt time.Time `json:"observedAt"`
	Recovery   Recovery  `json:"recovery"`
	Profiles   []Profile `json:"profiles"`
	Sessions   []Session `json:"sessions"`
}

// Recovery is the host's recovery fact with its wording of it, sessions.Recovery.Notice, for the
// phone; go clients word the fact themselves.
type Recovery struct {
	sessions.Recovery
	HostNotice *sessions.RecoveryNotice `json:"notice,omitempty"`
}

type Profile struct {
	AccountScope string `json:"accountScope,omitempty"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
}

type Session struct {
	TerminalStatus  sessions.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *sessions.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *sessions.TerminalFooter `json:"terminalFooter,omitempty"`
	TmuxID          string                   `json:"tmuxId"`
	ActivePaneID    string                   `json:"activePaneId"`
	TmuxName        string                   `json:"tmuxName"`
	NameMode        sessions.NameMode        `json:"nameMode"`
	IdentityToken   string                   `json:"identityToken"`
	Character       Character                `json:"character"`
	LaunchProfile   string                   `json:"launchProfile,omitempty"`
	Agent           *Agent                   `json:"agent,omitempty"`
	Connection      *Connection              `json:"connection,omitempty"`
	Objective       string                   `json:"objective,omitempty"`
	Group           string                   `json:"group,omitempty"`
	CWD             string                   `json:"cwd,omitempty"`
	ActiveCommand   string                   `json:"activeCommand,omitempty"`
	AttachedClients int                      `json:"attachedClients"`

	// Status is the host's copy of TerminalStatus, sessions.PresentStatus, for the phone; go
	// clients render it themselves.
	Status sessions.StatusPresentation `json:"status"`
}

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
	TerminalStatus sessions.TerminalStatus `json:"terminalStatus"`
	Diagnostics    *Diagnostics            `json:"diagnostics,omitempty"`
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
	ReadState string        `json:"readState"`
	Report    *quota.Report `json:"report,omitempty"`
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

type Target struct {
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}
