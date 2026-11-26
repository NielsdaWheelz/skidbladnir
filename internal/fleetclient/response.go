package fleetclient

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type Agent struct {
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
	Provider      string `json:"provider"`
	Profile       string `json:"profile,omitempty"`
	Label         string `json:"-"` // the host's configured label for Profile
}
type ExecutionAgent struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile,omitempty"`
	Label    string `json:"label,omitempty"`
}
type ExecutionContext struct {
	Kind    string          `json:"kind"`
	Machine string          `json:"machine,omitempty"`
	Label   string          `json:"label,omitempty"`
	CWD     string          `json:"cwd,omitempty"`
	Agent   *ExecutionAgent `json:"agent,omitempty"`
}
type Session struct {
	TerminalStatus  api.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *api.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *api.TerminalFooter `json:"terminalFooter,omitempty"`
	ActivePaneID    string              `json:"activePaneId"`
	Name            string              `json:"name"`
	NameMode        string              `json:"nameMode"`
	TerminalHandle  string              `json:"terminalHandle"`
	Ref             Ref                 `json:"ref"`
	CWD             string              `json:"cwd,omitempty"`
	ActiveCommand   string              `json:"activeCommand,omitempty"`
	Parent          *api.SessionRef     `json:"parent,omitempty"`
	Previous        []api.SessionRef    `json:"-"`
	Promoted        bool                `json:"-"`
	LaunchProfile   string              `json:"launchProfile,omitempty"`
	AttachedClients int                 `json:"attachedClients"`
	Agent           *Agent              `json:"agent,omitempty"`
	Connection      *api.Connection     `json:"connection,omitempty"`
	Execution       *ExecutionContext   `json:"execution,omitempty"`
	Group           group.Label         `json:"-"`
}

func (s Session) MarshalJSON() ([]byte, error) {
	type plain Session
	return json.Marshal(struct {
		plain
		Group string `json:"group,omitempty"`
	}{plain(s), s.Group.String()})
}

type Profile struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
	AccountScope string `json:"accountScope,omitempty"`
}

type Peer struct {
	Label      string        `json:"label"`
	Machine    string        `json:"machine"`
	OK         bool          `json:"ok"`
	ObservedAt time.Time     `json:"observedAt,omitempty"`
	ReceivedAt time.Time     `json:"-"`
	Profiles   []Profile     `json:"profiles,omitempty"`
	Sessions   []Session     `json:"sessions,omitempty"`
	Recovery   *api.Recovery `json:"recovery,omitempty"`
	Error      *Failure      `json:"error,omitempty"`
}

// MarshalJSON preserves required empty arrays for a successfully observed peer.
func (p Peer) MarshalJSON() ([]byte, error) {
	if !p.OK {
		return json.Marshal(struct {
			Label   string   `json:"label"`
			Machine string   `json:"machine"`
			OK      bool     `json:"ok"`
			Error   *Failure `json:"error"`
		}{p.Label, p.Machine, false, p.Error})
	}
	return json.Marshal(struct {
		Label      string        `json:"label"`
		Machine    string        `json:"machine"`
		OK         bool          `json:"ok"`
		ObservedAt time.Time     `json:"observedAt"`
		Profiles   []Profile     `json:"profiles"`
		Sessions   []Session     `json:"sessions"`
		Recovery   *api.Recovery `json:"recovery"`
	}{p.Label, p.Machine, true, p.ObservedAt, p.Profiles, p.Sessions, p.Recovery})
}

type Inventory struct {
	Partial bool   `json:"partial"`
	Peers   []Peer `json:"peers"`
}
type ObservedSession struct {
	Label      string  `json:"label"`
	Machine    string  `json:"machine"`
	ObservedAt string  `json:"observedAt"`
	Session    Session `json:"session"`
	// Diagnostics explain Session.TerminalStatus; only info --explain has them.
	Diagnostics *api.Diagnostics `json:"diagnostics,omitempty"`
}

// StartResult keeps creation and optional terminal input as independent facts.
type StartResult struct {
	Label                string           `json:"label"`
	Machine              string           `json:"machine"`
	Creation             string           `json:"creation"`
	Prompt               string           `json:"prompt"`
	ReplyAddressIncluded *bool            `json:"replyAddressIncluded,omitempty"`
	Target               Ref              `json:"target,omitzero"`
	Handle               string           `json:"handle,omitempty"`
	Terminal             *ObservedSession `json:"terminal,omitempty"`
	Failure              *Failure         `json:"failure,omitempty"`
	AdmissionFailure     *Failure         `json:"admissionFailure,omitempty"`
}

type GroupResult struct {
	Group string `json:"group"`
}

type WaitResult struct {
	Outcome        string              `json:"outcome"`
	Target         Ref                 `json:"target"`
	TerminalStatus *api.TerminalStatus `json:"terminalStatus,omitempty"`
}

// clientProfile admits a profile whose provider the client knows.
func clientProfile(wire api.Profile) (Profile, bool) {
	return Profile{Key: wire.Key, Label: wire.Label, Provider: wire.Provider, AccountScope: wire.AccountScope},
		wire.Provider == "" || wire.Provider == "Codex" || wire.Provider == "Claude"
}

// clientSession projects one gateway session. It keeps only what the client depends on: ids that
// form a reference, the enums it branches on, the group label, and the agent provider, which the
// hub republishes in notification foregrounds to the phone's closed enum (NotificationState.kt).
// The host's status copy is the phone's; go clients render api.PresentStatus.
func clientSession(wire api.Session, machine string) (Session, bool) {
	ref := Reference{Machine: machine, TmuxID: wire.TmuxID, IdentityToken: wire.IdentityToken, PaneID: wire.ActivePaneID}
	label, err := group.Parse(wire.Group)
	if !ref.valid() || err != nil || !wire.TerminalStatus.Known() || wire.NameMode != api.NameAutomatic && wire.NameMode != api.NameManual {
		return Session{}, false
	}
	row := Session{TerminalStatus: wire.TerminalStatus, TerminalUsage: wire.TerminalUsage, TerminalFooter: wire.TerminalFooter, ActivePaneID: wire.ActivePaneID, Name: wire.TmuxName, NameMode: string(wire.NameMode), TerminalHandle: ref.Handle(), Ref: Ref{ref}, CWD: wire.CWD, ActiveCommand: wire.ActiveCommand, LaunchProfile: wire.LaunchProfile, AttachedClients: wire.AttachedClients, Connection: wire.Connection, Group: label, Parent: wire.Parent, Previous: wire.Previous, Promoted: wire.Promoted}
	if agent := wire.Agent; agent != nil {
		if agent.Provider != "Codex" && agent.Provider != "Claude" {
			return Session{}, false
		}
		row.Agent = &Agent{Provider: agent.Provider, Profile: agent.Profile, Label: agent.Label, PID: agent.PID, StartIdentity: agent.StartIdentity}
	}
	return row, true
}

// decodeResponse reads one same-release gateway document with plain encoding/json: null is
// absent, a missing key is zero and an unknown key is ignored. It checks only what the client
// branches on: the answering machine, ids that form a reference, and the enums it switches over.
func decodeResponse(operation string, encoded []byte, target peer) (any, bool) {
	switch operation {
	case "list":
		var value api.Inventory
		if json.Unmarshal(encoded, &value) != nil || value.Machine.Handle != target.Machine || !value.Recovery.Known() {
			return nil, false
		}
		observed := Peer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: value.ObservedAt, Profiles: make([]Profile, len(value.Profiles)), Sessions: make([]Session, len(value.Sessions)), Recovery: &value.Recovery.Recovery}
		for index, wire := range value.Profiles {
			profile, ok := clientProfile(wire)
			if !ok {
				return nil, false
			}
			observed.Profiles[index] = profile
		}
		for index, wire := range value.Sessions {
			row, ok := clientSession(wire, target.Machine)
			if !ok {
				return nil, false
			}
			observed.Sessions[index] = row
		}
		slices.SortStableFunc(observed.Sessions, CompareSessions)
		return observed, true
	case "start", "shell":
		var value api.ObservedSession
		if json.Unmarshal(encoded, &value) != nil {
			return nil, false
		}
		row, ok := clientSession(value.Session, target.Machine)
		if !ok {
			return nil, false
		}
		return ObservedSession{Label: target.Label, Machine: target.Machine, ObservedAt: value.ObservedAt.Format(time.RFC3339Nano), Session: row}, true
	case "profile_usage", "claude_usage":
		var value api.ProfileUsageReport
		if json.Unmarshal(encoded, &value) != nil || value.Machine.Handle != target.Machine {
			return nil, false
		}
		observed := ProfileUsagePeer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: value.ObservedAt, Profiles: make([]ProfileUsage, len(value.Profiles)), ClaudeAccounts: value.ClaudeAccounts}
		for index, wire := range value.Profiles {
			profile, ok := clientProfile(wire.Profile)
			signedOut := profile.Provider == "Claude" && wire.ReadState == "no_login"
			if !ok || wire.ReadState != "ok" && wire.ReadState != "unavailable" && !signedOut {
				return nil, false
			}
			observed.Profiles[index] = ProfileUsage{Profile: profile, ReadState: wire.ReadState, Report: wire.Report, ClaudeAccount: wire.ClaudeAccount}
		}
		return observed, true
	case "terminal_context":
		var value api.TerminalContext
		if json.Unmarshal(encoded, &value) != nil {
			return nil, false
		}
		return value, true
	case "directory_search":
		var value workdir.SearchResult
		if json.Unmarshal(encoded, &value) != nil {
			return nil, false
		}
		return value, true
	case "job_submit":
		var wire *api.JobReceipt
		if json.Unmarshal(encoded, &wire) != nil || wire == nil || wire.State != api.JobAccepted && !wire.State.Terminal() {
			return nil, false
		}
		return projectJob(*wire, target.Machine), true
	case "job_result":
		var wire *api.JobResult
		if json.Unmarshal(encoded, &wire) != nil || wire == nil || !wire.State.Terminal() {
			return nil, false
		}
		return jobResult{projectJob(wire.JobReceipt, target.Machine), wire.Length, wire.SHA256, wire.Offset, wire.Page}, true
	case "branch_inspect":
		var value api.BranchNode
		if json.Unmarshal(encoded, &value) != nil {
			return nil, false
		}
		return value, true
	case "branch_reserve":
		var value api.BranchReservation
		if json.Unmarshal(encoded, &value) != nil || value.ID == "" {
			return nil, false
		}
		return value, true
	case "terminal_read":
		var value api.ReadResult
		if json.Unmarshal(encoded, &value) != nil {
			return nil, false
		}
		return value, true
	case "terminal_address":
		var value api.PeerAddress
		if json.Unmarshal(encoded, &value) != nil || value.Sender == "" || value.Digest == "" {
			return nil, false
		}
		return value, true
	case "terminal_send", "terminal_keys", "terminal_text", "terminal_stop", "peer_reply":
		var value api.WriteResult
		if json.Unmarshal(encoded, &value) != nil || value.Outcome != "written" && value.Outcome != "unknown" {
			return nil, false
		}
		return value, true
	case "close":
		var value api.CloseResult
		if json.Unmarshal(encoded, &value) != nil || !slices.Contains([]string{"written", "not_sent", "unknown"}, value.Interrupt) || !slices.Contains([]string{"closed", "not_closed", "unknown"}, value.Terminal) {
			return nil, false
		}
		return value, true
	case "terminal_inspect", "terminal_explain":
		var value api.Inspect
		if json.Unmarshal(encoded, &value) != nil || !value.TerminalStatus.Known() {
			return nil, false
		}
		return value, true
	}
	return nil, false
}
