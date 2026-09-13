package fleetclient

import (
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type session struct {
	TmuxID        string `json:"tmuxId"`
	TmuxName      string `json:"tmuxName"`
	IdentityToken string `json:"identityToken"`
	Character     struct {
		Key         string `json:"key"`
		DisplayName string `json:"displayName"`
	} `json:"character"`
	LaunchProfile string `json:"launchProfile,omitempty"`
	Agent         *struct {
		Provider        string `json:"provider"`
		PID             int    `json:"pid"`
		PaneID          string `json:"paneId"`
		StartIdentity   string `json:"startIdentity"`
		Profile         string `json:"profile,omitempty"`
		ProviderSession *struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"providerSession,omitempty"`
		Status struct {
			State  string `json:"state"`
			Source string `json:"source"`
			Reason string `json:"reason,omitempty"`
		} `json:"status"`
		Methods struct {
			Read      string `json:"read"`
			Send      string `json:"send"`
			Interrupt string `json:"interrupt"`
		} `json:"methods"`
	} `json:"agent,omitempty"`
	Objective       string `json:"objective,omitempty"`
	CWD             string `json:"cwd,omitempty"`
	ActiveCommand   string `json:"activeCommand,omitempty"`
	AttachedClients int    `json:"attachedClients"`
}

func validResponse(operation string, encoded []byte, machine string) bool {
	switch operation {
	case "list":
		var value *struct {
			Machine struct {
				Handle   string `json:"handle"`
				Platform string `json:"platform"`
			} `json:"machine"`
			ObservedAt string `json:"observedAt"`
			Profiles   []struct {
				Key      string `json:"key"`
				Label    string `json:"label"`
				Provider string `json:"provider"`
			} `json:"profiles"`
			Sessions []session `json:"sessions"`
		}
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Machine.Handle != machine || !slices.Contains([]string{"Linux", "Darwin"}, value.Machine.Platform) || value.Profiles == nil || value.Sessions == nil {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return false
		}
		for _, profile := range value.Profiles {
			if profile.Key == "" || profile.Label == "" || !slices.Contains([]string{"Codex", "Claude"}, profile.Provider) {
				return false
			}
		}
		for _, current := range value.Sessions {
			if !validSession(current) {
				return false
			}
		}
		return true
	case "start":
		var value *struct {
			ObservedAt string  `json:"observedAt"`
			Session    session `json:"session"`
		}
		return strictjson.Decode(encoded, &value) == nil && value != nil && value.ObservedAt != "" && validSession(value.Session)
	case "read":
		var value *struct {
			Text      *string `json:"text"`
			Source    string  `json:"source"`
			Scope     string  `json:"scope"`
			Truncated *bool   `json:"truncated"`
		}
		return strictjson.Decode(encoded, &value) == nil && value != nil && value.Text != nil && value.Truncated != nil && slices.Contains([]string{"native", "terminal"}, value.Source) && slices.Contains([]string{"recent_messages", "latest_turn", "terminal_history", "visible"}, value.Scope)
	case "send", "keys", "interrupt":
		var value *struct {
			Method  string `json:"method"`
			Outcome string `json:"outcome"`
			TurnID  string `json:"turnId,omitempty"`
		}
		return strictjson.Decode(encoded, &value) == nil && value != nil && slices.Contains([]string{"native", "terminal"}, value.Method) && slices.Contains([]string{"accepted", "written", "interrupted", "finished", "unknown"}, value.Outcome)
	case "stop":
		var value *struct {
			Agent    string `json:"agent"`
			Terminal string `json:"terminal"`
			Reason   string `json:"reason,omitempty"`
		}
		return strictjson.Decode(encoded, &value) == nil && value != nil && slices.Contains([]string{"stopped", "interrupted", "idle", "unconfirmed"}, value.Agent) && slices.Contains([]string{"closed", "unconfirmed"}, value.Terminal) && slices.Contains([]string{"", "stale", "unavailable"}, value.Reason)
	default:
		return false
	}
}

func validSession(value session) bool {
	if !tmuxAddress(value.TmuxID, '$') || value.TmuxName == "" || value.IdentityToken == "" || value.AttachedClients < 0 {
		return false
	}
	agent := value.Agent
	if agent == nil {
		return true
	}
	return slices.Contains([]string{"Codex", "Claude"}, agent.Provider) && agent.PID > 0 && tmuxAddress(agent.PaneID, '%') && agent.StartIdentity != "" &&
		slices.Contains([]string{"working", "blocked", "idle", "done", "failed", "stopped", "unknown"}, agent.Status.State) &&
		slices.Contains([]string{"native", "terminal", "unavailable"}, agent.Status.Source) &&
		slices.Contains([]string{"", "permission", "input", "dialog", "provider_unavailable", "unrecognized"}, agent.Status.Reason) &&
		slices.Contains([]string{"native", "terminal", "unavailable"}, agent.Methods.Read) &&
		slices.Contains([]string{"native", "terminal", "unavailable"}, agent.Methods.Send) &&
		slices.Contains([]string{"native", "terminal", "unavailable"}, agent.Methods.Interrupt)
}
