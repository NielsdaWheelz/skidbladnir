package fleetclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type ProviderSession struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}
type Agent struct {
	PID             int64            `json:"pid"`
	StartIdentity   string           `json:"startIdentity"`
	Provider        string           `json:"provider"`
	Profile         string           `json:"profile,omitempty"`
	ProviderSession *ProviderSession `json:"providerSession,omitempty"`
}
type Connection struct {
	Transport string `json:"transport"`
	ID        string `json:"id,omitempty"`
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
	TerminalStatus     TerminalStatus             `json:"terminalStatus"`
	ActivePaneID       string                     `json:"activePaneId"`
	Name               string                     `json:"name"`
	NameMode           string                     `json:"nameMode"`
	TerminalHandle     string                     `json:"terminalHandle"`
	ConversationHandle string                     `json:"conversationHandle,omitempty"`
	Ref                string                     `json:"ref"`
	CWD                string                     `json:"cwd,omitempty"`
	ActiveCommand      string                     `json:"activeCommand,omitempty"`
	LaunchProfile      string                     `json:"launchProfile,omitempty"`
	AttachedClients    int                        `json:"attachedClients"`
	Agent              *Agent                     `json:"agent,omitempty"`
	Connection         *Connection                `json:"connection,omitempty"`
	Execution          *ExecutionContext          `json:"execution,omitempty"`
	Group              group.Label                `json:"-"`
	Conversation       *agentruntime.Conversation `json:"conversation,omitempty"`
}

type groupField struct{ label group.Label }

func (value groupField) IsZero() bool                 { return value.label.IsUnassigned() }
func (value groupField) MarshalJSON() ([]byte, error) { return json.Marshal(value.label.String()) }
func (value *groupField) UnmarshalJSON(encoded []byte) error {
	var text *string
	if strictjson.Decode(encoded, &text) != nil || text == nil || *text == "" {
		return errors.New("invalid group response")
	}
	label, err := group.Parse(*text)
	if err != nil {
		return err
	}
	value.label = label
	return nil
}
func (s Session) MarshalJSON() ([]byte, error) {
	type sessionJSON Session
	return json.Marshal(struct {
		sessionJSON
		Group groupField `json:"group,omitzero"`
	}{sessionJSON(s), groupField{s.Group}})
}

type Profile struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider"`
	HistoryScope string `json:"historyScope,omitempty"`
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

// InspectedReference separates captured action identity from current observation.
type InspectedReference struct {
	Label   string `json:"label"`
	Machine string `json:"machine"`
	Target  struct {
		Ref          string                    `json:"ref"`
		Conversation agentruntime.Conversation `json:"conversation"`
		Turn         *agentruntime.Turn        `json:"turn,omitempty"`
	} `json:"target"`
	Inspection  Result `json:"inspection"`
	ObservedRef string `json:"observedRef,omitempty"`
}
type ReadResult struct {
	Observation  *agentruntime.Observation `json:"observation,omitempty"`
	OutputState  string                    `json:"outputState,omitempty"`
	OutputID     string                    `json:"outputId,omitempty"`
	OutputTurnID string                    `json:"outputTurnId,omitempty"`
	Text         string                    `json:"text"`
	Source       string                    `json:"source"`
	Scope        string                    `json:"scope"`
	Truncated    bool                      `json:"truncated"`
}
type WriteResult struct {
	Method  string `json:"method"`
	Outcome string `json:"outcome"`
}
type CloseResult struct {
	Interrupt string `json:"interrupt"`
	Terminal  string `json:"terminal"`
}
type TerminalCloseResult struct {
	Terminal string `json:"terminal"`
}
type GroupResult struct {
	Group string `json:"group"`
}

type DirectorySearchResult struct {
	Directories []string `json:"directories"`
	Omitted     bool     `json:"omitted"`
}

type hostAgent struct {
	Provider        string           `json:"provider"`
	Profile         string           `json:"profile,omitempty"`
	ProviderSession *ProviderSession `json:"providerSession,omitempty"`
	PID             int              `json:"pid"`
	PaneID          string           `json:"paneId"`
	StartIdentity   string           `json:"startIdentity"`
}
type hostSession struct {
	TerminalStatus *TerminalStatus `json:"terminalStatus"`
	ActivePaneID   string          `json:"activePaneId"`
	TmuxID         string          `json:"tmuxId"`
	TmuxName       string          `json:"tmuxName"`
	NameMode       string          `json:"nameMode"`
	IdentityToken  string          `json:"identityToken"`
	Character      struct {
		Key         string `json:"key"`
		DisplayName string `json:"displayName"`
	} `json:"character"`
	LaunchProfile   string                     `json:"launchProfile,omitempty"`
	Agent           *hostAgent                 `json:"agent,omitempty"`
	Connection      *Connection                `json:"connection,omitempty"`
	Objective       string                     `json:"objective,omitempty"`
	CWD             string                     `json:"cwd,omitempty"`
	ActiveCommand   string                     `json:"activeCommand,omitempty"`
	AttachedClients *int                       `json:"attachedClients"`
	Group           groupField                 `json:"group,omitzero"`
	Conversation    *agentruntime.Conversation `json:"conversation,omitempty"`
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
	ref := Reference{Machine: machine, TmuxID: s.TmuxID, IdentityToken: s.IdentityToken, PaneID: s.ActivePaneID}
	row := Session{TerminalStatus: *s.TerminalStatus, ActivePaneID: s.ActivePaneID, Group: s.Group.label, Name: s.TmuxName, NameMode: s.NameMode, TerminalHandle: terminalHandle(ref), CWD: s.CWD, ActiveCommand: s.ActiveCommand, LaunchProfile: s.LaunchProfile, AttachedClients: *s.AttachedClients, Connection: s.Connection, Conversation: s.Conversation}
	if s.Agent != nil {
		row.Agent = &Agent{Provider: s.Agent.Provider, Profile: s.Agent.Profile, ProviderSession: s.Agent.ProviderSession, PID: int64(s.Agent.PID), StartIdentity: s.Agent.StartIdentity}
	}
	row.Ref = ref.Encode()
	if s.Conversation != nil {
		row.ConversationHandle = conversationHandle(machine, *s.Conversation)
	}
	return row
}

func decodeResponse(operation string, encoded []byte, target peer) (any, bool) {
	if !nonNullJSON(encoded) {
		return nil, false
	}
	switch operation {
	case "terminal_context":
		var value *RemoteContext
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Agent != nil && value.Connection != nil {
			return nil, false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return nil, false
		}
		if value.CWD != "" && (len(value.CWD) > 4096 || !filepath.IsAbs(value.CWD) || !utf8.ValidString(value.CWD) || strings.IndexFunc(value.CWD, unicode.IsControl) >= 0) {
			return nil, false
		}
		if value.Connection != nil && (value.CWD != "" || !validConnection(*value.Connection)) {
			return nil, false
		}
		if value.Agent != nil && value.Agent.Provider != "Codex" && value.Agent.Provider != "Claude" {
			return nil, false
		}
		return *value, true
	case "directory_search":
		var value *struct {
			Directories []string `json:"directories"`
			Omitted     *bool    `json:"omitted"`
		}
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Directories == nil || value.Omitted == nil || len(value.Directories) > 64 {
			return nil, false
		}
		bytes := 0
		for _, directory := range value.Directories {
			bytes += len(directory)
			if directory == "" || len(directory) > 4096 || bytes > 32*1024 || !filepath.IsAbs(directory) || !utf8.ValidString(directory) || strings.IndexFunc(directory, unicode.IsControl) >= 0 {
				return nil, false
			}
		}
		return DirectorySearchResult{Directories: value.Directories, Omitted: *value.Omitted}, true
	case "list":
		var value *hostInventory
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Machine.Handle != target.Machine || !slices.Contains([]string{"Linux", "Darwin"}, value.Machine.Platform) || value.Profiles == nil || value.Sessions == nil {
			return nil, false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return nil, false
		}
		for _, p := range value.Profiles {
			if p.Key == "" || p.Label == "" || !slices.Contains([]string{"Codex", "Claude"}, p.Provider) || p.HistoryScope != "" && !(agentruntime.Conversation{Provider: agentruntime.Provider(p.Provider), ProfileKey: agentruntime.ProfileKey(p.Key), HistoryScope: p.HistoryScope, ConversationID: "validation"}).Valid() {
				return nil, false
			}
		}
		observed := Peer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: value.ObservedAt, Profiles: value.Profiles, Sessions: make([]Session, 0, len(value.Sessions))}
		for _, s := range value.Sessions {
			if !validSession(s) {
				return nil, false
			}
			observed.Sessions = append(observed.Sessions, s.project(target.Machine))
		}
		slices.SortStableFunc(observed.Sessions, compareSessions)
		return observed, true
	case "start", "shell":
		var value *hostObservedSession
		if strictjson.Decode(encoded, &value) != nil || value == nil || !validSession(value.Session) {
			return nil, false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return nil, false
		}
		return ObservedSession{Label: target.Label, Machine: target.Machine, ObservedAt: value.ObservedAt, Session: value.Session.project(target.Machine)}, true
	case "terminal_read":
		var value *struct {
			Text      *string `json:"text"`
			Source    string  `json:"source"`
			Scope     string  `json:"scope"`
			Truncated *bool   `json:"truncated"`
		}
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Text == nil || value.Truncated == nil || len(*value.Text) > 32768 || !utf8.ValidString(*value.Text) || value.Source != "terminal" || !slices.Contains([]string{"visible", "terminal_history"}, value.Scope) {
			return nil, false
		}
		return ReadResult{Text: *value.Text, Source: value.Source, Scope: value.Scope, Truncated: *value.Truncated}, true
	case "read":
		var value *ReadResult
		if strictjson.Decode(encoded, &value) != nil || value == nil {
			return nil, false
		}
		var fields map[string]json.RawMessage
		if strictjson.Decode(encoded, &fields) != nil || fields["text"] == nil || fields["truncated"] == nil || len(value.Text) > 32768 {
			return nil, false
		}
		if value.Source != "native" {
			return nil, false
		}
		if value.Scope != "latest" && value.Scope != "history" || value.Observation == nil || !value.Observation.Binding.Valid() || !value.Observation.Status.Valid() || value.Observation.Turn != nil && !value.Observation.Turn.Valid() || !slices.Contains([]string{"partial", "finalized", "unknown", "none"}, value.OutputState) {
			return nil, false
		}
		if value.OutputState == "finalized" && value.OutputID == "" || value.OutputState == "none" && (value.Scope == "latest" && value.Text != "" || value.OutputID != "" || value.OutputTurnID != "") {
			return nil, false
		}
		return *value, true
	case "send":
		var value *SendResult
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Method != "native" || !slices.Contains([]string{"peer", "user"}, value.Input) || value.Delivery != "direct" || value.Outcome != "accepted" {
			return nil, false
		}
		if value.TurnID == "" {
			return nil, false
		}
		return *value, true
	case "terminal_send", "terminal_keys", "terminal_text", "terminal_stop", "stop":
		var value *WriteResult
		if strictjson.Decode(encoded, &value) != nil || value == nil {
			return nil, false
		}
		if operation != "stop" {
			if value.Method != "terminal" {
				return nil, false
			}
			if value.Outcome != "written" && value.Outcome != "unknown" {
				return nil, false
			}
		} else if value.Method != "native" || !slices.Contains([]string{"interrupted", "stopped", "finished", "unknown"}, value.Outcome) {
			return nil, false
		}
		return *value, true
	case "close":
		var value *CloseResult
		if strictjson.Decode(encoded, &value) != nil || value == nil || !slices.Contains([]string{"written", "not_sent", "unknown"}, value.Interrupt) || !slices.Contains([]string{"closed", "not_closed", "unknown"}, value.Terminal) {
			return nil, false
		}
		return *value, true
	case "terminal_inspect":
		var value *TerminalInspectResult
		if strictjson.Decode(encoded, &value) != nil || value == nil || !validTerminalStatus(value.TerminalStatus) {
			return nil, false
		}
		return *value, true
	case "inspect":
		var value *agentruntime.ConversationRuntime
		if strictjson.Decode(encoded, &value) != nil || value == nil || !validConversationRuntime(*value) {
			return nil, false
		}
		return *value, true
	case "results":
		var value *ResultsResult
		if strictjson.Decode(encoded, &value) != nil || value == nil || !value.Conversation.Valid() || value.ResultIDs == nil || len(value.ResultIDs) > 128 {
			return nil, false
		}
		seen := make(map[string]bool, len(value.ResultIDs))
		for _, id := range value.ResultIDs {
			if !validNativeID(id) || seen[id] {
				return nil, false
			}
			seen[id] = true
		}
		return *value, true
	default:
		return nil, false
	}

}

func validSession(s hostSession) bool {
	if s.NameMode != "automatic" && s.NameMode != "manual" {
		return false
	}
	if !tmuxAddress(s.ActivePaneID, '%') || !tmuxAddress(s.TmuxID, '$') || s.TmuxName == "" || s.IdentityToken == "" || s.AttachedClients == nil || *s.AttachedClients < 0 || s.TerminalStatus == nil || !validTerminalStatus(*s.TerminalStatus) {
		return false
	}
	if s.Connection != nil && (s.Agent != nil || s.CWD != "" || !validConnection(*s.Connection)) {
		return false
	}
	if s.Conversation != nil && !s.Conversation.Valid() {
		return false
	}
	a := s.Agent
	return a == nil || slices.Contains([]string{"Codex", "Claude"}, a.Provider) && a.PID > 0 && a.PaneID == s.ActivePaneID && a.StartIdentity != ""

}

func validConnection(connection Connection) bool {
	if connection.Transport != "ssh" && connection.Transport != "mosh" {
		return false
	}
	return connection.ID == "" || validConnectionID(connection.ID)
}

// Creation and membership errors carry required dispatch evidence. Malformed
// evidence never becomes a definite rejection through control-route inference.
func decodeMutationFailure(operation string, encoded []byte, status int) *Failure {
	var value struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Dispatch string `json:"dispatch"`
	}
	if !nonNullJSON(encoded) || strictjson.Decode(encoded, &value) != nil || value.Dispatch != "not_sent" && value.Dispatch != "unknown" {
		return nil
	}
	wantStatus, wantMessage := 0, ""
	switch value.Code {
	case "Unauthenticated":
		wantStatus, wantMessage = http.StatusUnauthorized, "Authentication required."
	case "MachineIdentityMismatch":
		wantStatus, wantMessage = http.StatusConflict, "The machine identity changed. Fleet reset is required."
	case "InvalidRequest":
		wantStatus, wantMessage = http.StatusBadRequest, "The request is not valid."
	case "RequestTooLarge":
		wantStatus, wantMessage = http.StatusRequestEntityTooLarge, "The request is too large."
	case "GroupInvalid":
		wantStatus, wantMessage = http.StatusUnprocessableEntity, group.ErrInvalid.Error()
	case "SessionNotFound":
		wantStatus, wantMessage = http.StatusNotFound, "That session no longer exists."
	case "SessionIdentityMismatch":
		wantStatus, wantMessage = http.StatusConflict, "The session changed. Refresh and try again."
	case "WorkingDirectoryInvalid", "WorkingDirectoryUnavailable", "ProfileUnknown", "SessionNameInvalid", "SessionNameConflict", "ObjectiveInvalid":
		if operation == "group" {
			return nil
		}
		wantStatus = http.StatusUnprocessableEntity
		switch value.Code {
		case "WorkingDirectoryInvalid":
			wantMessage = "Choose a valid working directory."
		case "WorkingDirectoryUnavailable":
			wantMessage = "That directory does not exist or cannot be opened."
		case "ProfileUnknown":
			wantMessage = "Choose an available profile."
		case "SessionNameInvalid":
			wantMessage = "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number."
		case "SessionNameConflict":
			wantStatus, wantMessage = http.StatusConflict, "another session on this machine uses that name."
		case "ObjectiveInvalid":
			wantMessage = "Use 1–240 characters without terminal controls."
		}
	case "AgentUnavailable", "AgentTargetStale":
		if operation != "start" {
			return nil
		}
		wantStatus = http.StatusConflict
		if value.Code == "AgentUnavailable" {
			wantMessage = "this action is unavailable for this session."
		} else {
			wantMessage = "the session changed. refresh and try again."
		}
	case "InternalError":
		wantStatus, wantMessage = http.StatusInternalServerError, "Skíðblaðnir could not complete the request."
	default:
		return nil
	}
	if status != wantStatus || value.Message != wantMessage || value.Code != "InternalError" && value.Code != "AgentUnavailable" && value.Code != "AgentTargetStale" && value.Dispatch != "not_sent" {
		return nil
	}
	return &Failure{Code: value.Code, Dispatch: value.Dispatch}
}

type SendResult struct {
	Method   string `json:"method"`
	Input    string `json:"input"`
	Delivery string `json:"delivery"`
	Outcome  string `json:"outcome"`
	TurnID   string `json:"turnId,omitempty"`
}
type ResultsResult struct {
	Conversation agentruntime.Conversation `json:"conversation"`
	ResultIDs    []string                  `json:"resultIds"`
	NextCursor   string                    `json:"nextCursor,omitempty"`
}
type WaitResult struct {
	Outcome        string                    `json:"outcome"`
	Target         string                    `json:"target"`
	Observation    *agentruntime.Observation `json:"observation,omitempty"`
	TerminalStatus *TerminalStatus           `json:"terminalStatus,omitempty"`
}

func StatusText(status agentruntime.Status) string {
	if status.Source == "unavailable" || status.State == "unknown" {
		return "status unavailable"
	}
	if status.State == "blocked" {
		return "waiting"
	}
	return status.State
}

// All fleet and local notification schemas distinguish omission from null.
func nonNullJSON(encoded []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil || token == nil {
			return false
		}
	}
}

func validConversationRuntime(value agentruntime.ConversationRuntime) bool {
	return value.Binding.Valid() && value.Status.Valid() && value.Methods.Valid() && (value.Turn == nil || value.Turn.Valid())
}

type TerminalStatus struct {
	State  string `json:"state"`
	Source string `json:"source"`
}

type TerminalInspectResult struct {
	TerminalStatus TerminalStatus `json:"terminalStatus"`
}

func validTerminalStatus(value TerminalStatus) bool {
	return (value.Source == "terminal" && slices.Contains([]string{"working", "blocked", "idle", "unknown"}, value.State)) || value.Source == "unavailable" && value.State == "unknown"
}

func SessionStatus(session Session) string {
	if session.TerminalStatus.Source == "unavailable" {
		return "status unavailable"
	}
	if session.Agent == nil {
		return "terminal"
	}
	return TerminalStatusText(session.TerminalStatus)
}

func TerminalStatusText(status TerminalStatus) string {
	if status.Source == "unavailable" {
		return "status unavailable"
	}
	switch status.State {
	case "blocked":
		return "waiting"
	case "unknown":
		return "status unknown"
	default:
		return status.State
	}
}
