package fleetclient

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type Status struct {
	State  string `json:"state"`
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}
type Methods struct {
	Read      string `json:"read"`
	Send      string `json:"send"`
	Interrupt string `json:"interrupt"`
}
type ProviderSession struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}
type Agent struct {
	Provider        string           `json:"provider"`
	Profile         string           `json:"profile,omitempty"`
	ProviderSession *ProviderSession `json:"providerSession,omitempty"`
	Status          Status           `json:"status"`
	Methods         Methods          `json:"methods"`
}
type Session struct {
	Name            string `json:"name"`
	Ref             string `json:"ref"`
	CWD             string `json:"cwd,omitempty"`
	ActiveCommand   string `json:"activeCommand,omitempty"`
	LaunchProfile   string `json:"launchProfile,omitempty"`
	AttachedClients int    `json:"attachedClients"`
	Agent           *Agent `json:"agent,omitempty"`
}
type Profile struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
}
type Peer struct {
	Label      string    `json:"label"`
	Machine    string    `json:"machine"`
	OK         bool      `json:"ok"`
	ObservedAt string    `json:"observedAt,omitempty"`
	Profiles   []Profile `json:"profiles,omitempty"`
	Sessions   []Session `json:"sessions,omitempty"`
	Error      *Failure  `json:"error,omitempty"`
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
		Label      string    `json:"label"`
		Machine    string    `json:"machine"`
		OK         bool      `json:"ok"`
		ObservedAt string    `json:"observedAt"`
		Profiles   []Profile `json:"profiles"`
		Sessions   []Session `json:"sessions"`
	}{p.Label, p.Machine, true, p.ObservedAt, p.Profiles, p.Sessions})
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
	TurnID  string `json:"turnId,omitempty"`
}
type StopResult struct {
	Agent    string `json:"agent"`
	Terminal string `json:"terminal"`
	Reason   string `json:"reason,omitempty"`
}

type hostAgent struct {
	Provider        string           `json:"provider"`
	Profile         string           `json:"profile,omitempty"`
	ProviderSession *ProviderSession `json:"providerSession,omitempty"`
	Status          Status           `json:"status"`
	Methods         Methods          `json:"methods"`
	PID             int              `json:"pid"`
	PaneID          string           `json:"paneId"`
	StartIdentity   string           `json:"startIdentity"`
}
type hostSession struct {
	TmuxID        string `json:"tmuxId"`
	TmuxName      string `json:"tmuxName"`
	IdentityToken string `json:"identityToken"`
	Character     struct {
		Key         string `json:"key"`
		DisplayName string `json:"displayName"`
	} `json:"character"`
	LaunchProfile   string     `json:"launchProfile,omitempty"`
	Agent           *hostAgent `json:"agent,omitempty"`
	Objective       string     `json:"objective,omitempty"`
	CWD             string     `json:"cwd,omitempty"`
	ActiveCommand   string     `json:"activeCommand,omitempty"`
	AttachedClients *int       `json:"attachedClients"`
}
type hostInventory struct {
	Machine struct {
		Handle   string `json:"handle"`
		Platform string `json:"platform"`
	} `json:"machine"`
	ObservedAt string        `json:"observedAt"`
	Profiles   []Profile     `json:"profiles"`
	Sessions   []hostSession `json:"sessions"`
}
type hostObservedSession struct {
	ObservedAt string      `json:"observedAt"`
	Session    hostSession `json:"session"`
}

func (s hostSession) project(machine string) Session {
	ref := Reference{Machine: machine, TmuxID: s.TmuxID, IdentityToken: s.IdentityToken}
	row := Session{Name: s.TmuxName, CWD: s.CWD, ActiveCommand: s.ActiveCommand, LaunchProfile: s.LaunchProfile, AttachedClients: *s.AttachedClients}
	if s.Agent != nil {
		ref.Agent = &ProcessReference{PaneID: s.Agent.PaneID, PID: s.Agent.PID, StartIdentity: s.Agent.StartIdentity}
		row.Agent = &Agent{Provider: s.Agent.Provider, Profile: s.Agent.Profile, ProviderSession: s.Agent.ProviderSession, Status: s.Agent.Status, Methods: s.Agent.Methods}
	}
	row.Ref = ref.Encode()
	return row
}

func validResponse(operation string, encoded []byte, machine string) bool {
	switch operation {
	case "list":
		var value *hostInventory
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Machine.Handle != machine || !slices.Contains([]string{"Linux", "Darwin"}, value.Machine.Platform) || value.Profiles == nil || value.Sessions == nil {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return false
		}
		for _, p := range value.Profiles {
			if p.Key == "" || p.Label == "" || !slices.Contains([]string{"Codex", "Claude"}, p.Provider) {
				return false
			}
		}
		for _, s := range value.Sessions {
			if !validSession(s) {
				return false
			}
		}
		return true
	case "start":
		var value *hostObservedSession
		if strictjson.Decode(encoded, &value) != nil || value == nil || !validSession(value.Session) {
			return false
		}
		_, err := time.Parse(time.RFC3339Nano, value.ObservedAt)
		return err == nil
	case "read":
		var value *struct {
			Text      *string `json:"text"`
			Source    string  `json:"source"`
			Scope     string  `json:"scope"`
			Truncated *bool   `json:"truncated"`
		}
		return strictjson.Decode(encoded, &value) == nil && value != nil && value.Text != nil && value.Truncated != nil && slices.Contains([]string{"native", "terminal"}, value.Source) && slices.Contains([]string{"recent_messages", "latest_turn", "terminal_history", "visible"}, value.Scope)
	case "send", "keys", "interrupt":
		var value *WriteResult
		return strictjson.Decode(encoded, &value) == nil && value != nil && slices.Contains([]string{"native", "terminal"}, value.Method) && slices.Contains([]string{"accepted", "written", "interrupted", "finished", "unknown"}, value.Outcome)
	case "stop":
		var value *StopResult
		return strictjson.Decode(encoded, &value) == nil && value != nil && slices.Contains([]string{"stopped", "interrupted", "idle", "unconfirmed"}, value.Agent) && slices.Contains([]string{"closed", "unconfirmed"}, value.Terminal) && slices.Contains([]string{"", "stale", "unavailable"}, value.Reason)
	default:
		return false
	}
}

func validSession(s hostSession) bool {
	if !tmuxAddress(s.TmuxID, '$') || s.TmuxName == "" || s.IdentityToken == "" || s.AttachedClients == nil || *s.AttachedClients < 0 {
		return false
	}
	a := s.Agent
	if a == nil {
		return true
	}
	return slices.Contains([]string{"Codex", "Claude"}, a.Provider) && a.PID > 0 && tmuxAddress(a.PaneID, '%') && a.StartIdentity != "" && slices.Contains([]string{"working", "blocked", "idle", "done", "failed", "stopped", "unknown"}, a.Status.State) && slices.Contains([]string{"native", "terminal", "unavailable"}, a.Status.Source) && slices.Contains([]string{"", "permission", "input", "dialog", "provider_unavailable", "unrecognized"}, a.Status.Reason) && slices.Contains([]string{"native", "terminal", "unavailable"}, a.Methods.Read) && slices.Contains([]string{"native", "terminal", "unavailable"}, a.Methods.Send) && slices.Contains([]string{"native", "terminal", "unavailable"}, a.Methods.Interrupt)
}
