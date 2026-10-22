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

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type Agent struct {
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
	Provider      string `json:"provider"`
	Profile       string `json:"profile,omitempty"`
	Label         string `json:"-"` // the host's configured label for Profile
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
	TerminalStatus  sessions.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *sessions.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *sessions.TerminalFooter `json:"terminalFooter,omitempty"`
	ActivePaneID    string                   `json:"activePaneId"`
	Name            string                   `json:"name"`
	NameMode        string                   `json:"nameMode"`
	TerminalHandle  string                   `json:"terminalHandle"`
	Ref             Ref                      `json:"ref"`
	CWD             string                   `json:"cwd,omitempty"`
	ActiveCommand   string                   `json:"activeCommand,omitempty"`
	LaunchProfile   string                   `json:"launchProfile,omitempty"`
	AttachedClients int                      `json:"attachedClients"`
	Agent           *Agent                   `json:"agent,omitempty"`
	Connection      *Connection              `json:"connection,omitempty"`
	Execution       *ExecutionContext        `json:"execution,omitempty"`
	Group           group.Label              `json:"-"`
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
	Provider     string `json:"provider,omitempty"`
	AccountScope string `json:"accountScope,omitempty"`
}

type hostProfile struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Provider     string  `json:"provider,omitempty"`
	AccountScope *string `json:"accountScope,omitempty"`
}

func (wire hostProfile) project() (Profile, bool) {
	if wire.Key == "" || wire.Label == "" || !slices.Contains([]string{"", "Codex", "Claude"}, wire.Provider) {
		return Profile{}, false
	}
	profile := Profile{Key: wire.Key, Label: wire.Label, Provider: wire.Provider}
	if wire.AccountScope != nil {
		if len(*wire.AccountScope) != 64 || strings.Trim(*wire.AccountScope, "0123456789abcdef") != "" {
			return Profile{}, false
		}
		profile.AccountScope = *wire.AccountScope
	}
	return profile, true
}

// Recovery contains one strictly admitted host fact. Clients never persist it.
type Recovery struct{ fact sessions.Recovery }

type recoveryJSON struct {
	State   sessions.RecoveryState `json:"state"`
	SavedAt *string                `json:"savedAt,omitempty"`
	Failure *recoveryFailureJSON   `json:"failure,omitempty"`

	// Notice is decoded and ignored: the phone's copy. Go clients render
	// sessions.Recovery.Notice, and MarshalJSON never sets it.
	Notice *sessions.RecoveryNotice `json:"notice,omitempty"`
}

type recoveryFailureJSON struct {
	At    string                 `json:"at"`
	Phase sessions.RecoveryPhase `json:"phase"`
	Cause sessions.RecoveryCause `json:"cause"`
}

func (r *Recovery) UnmarshalJSON(encoded []byte) error {
	var value *recoveryJSON
	if !nonNullJSON(encoded) || strictjson.Decode(encoded, &value) != nil || value == nil {
		return errors.New("invalid recovery response")
	}
	fact := sessions.Recovery{State: value.State}
	if value.SavedAt != nil {
		instant, err := sessions.ParseProjectionInstant(*value.SavedAt)
		if err != nil {
			return errors.New("invalid recovery response")
		}
		fact.SavedAt = &instant
	}
	if value.Failure != nil {
		at, err := sessions.ParseProjectionInstant(value.Failure.At)
		if err != nil {
			return errors.New("invalid recovery response")
		}
		fact.Failure = &sessions.RecoveryFailure{At: at, Phase: value.Failure.Phase, Cause: value.Failure.Cause}
	}
	if !fact.Valid() {
		return errors.New("invalid recovery response")
	}
	*r = Recovery{fact: fact}
	return nil
}

func (r Recovery) MarshalJSON() ([]byte, error) {
	switch r.fact.State {
	case sessions.RecoveryTracking, sessions.RecoveryRecovered, sessions.RecoveryDelayed, sessions.RecoveryBroken:
	default:
		panic("invalid recovery state") // justify-defect: Recovery's private fact is admitted by UnmarshalJSON.
	}
	value := recoveryJSON{State: r.fact.State}
	if r.fact.SavedAt != nil {
		savedAt := r.fact.SavedAt.UTC().Format(time.RFC3339Nano)
		value.SavedAt = &savedAt
	}
	if failure := r.fact.Failure; failure != nil {
		value.Failure = &recoveryFailureJSON{At: failure.At.UTC().Format(time.RFC3339Nano), Phase: failure.Phase, Cause: failure.Cause}
	}
	return json.Marshal(value)
}

type Peer struct {
	Label      string    `json:"label"`
	Machine    string    `json:"machine"`
	OK         bool      `json:"ok"`
	ObservedAt string    `json:"observedAt,omitempty"`
	ReceivedAt time.Time `json:"-"`
	Profiles   []Profile `json:"profiles,omitempty"`
	Sessions   []Session `json:"sessions,omitempty"`
	Recovery   *Recovery `json:"recovery,omitempty"`
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
		Recovery   *Recovery `json:"recovery"`
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
	Diagnostics *agentcontrol.Diagnostics `json:"diagnostics,omitempty"`
}

// StartResult keeps creation and optional terminal input as independent facts.
type StartResult struct {
	Label    string           `json:"label"`
	Machine  string           `json:"machine"`
	Creation string           `json:"creation"`
	Prompt   string           `json:"prompt"`
	Target   Ref              `json:"target,omitzero"`
	Handle   string           `json:"handle,omitempty"`
	Terminal *ObservedSession `json:"terminal,omitempty"`
	Failure  *Failure         `json:"failure,omitempty"`
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
	Provider      string `json:"provider"`
	Profile       string `json:"profile,omitempty"`
	PID           int    `json:"pid"`
	PaneID        string `json:"paneId"`
	StartIdentity string `json:"startIdentity"`
	Label         string `json:"label,omitempty"`
}
type hostSession struct {
	TerminalStatus sessions.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage  *sessions.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter *sessions.TerminalFooter `json:"terminalFooter,omitempty"`
	ActivePaneID   string                   `json:"activePaneId"`
	TmuxID         string                   `json:"tmuxId"`
	TmuxName       string                   `json:"tmuxName"`
	NameMode       string                   `json:"nameMode"`
	IdentityToken  string                   `json:"identityToken"`
	Character      struct {
		Key         string `json:"key"`
		DisplayName string `json:"displayName"`
	} `json:"character"`
	LaunchProfile   string      `json:"launchProfile,omitempty"`
	Agent           *hostAgent  `json:"agent,omitempty"`
	Connection      *Connection `json:"connection,omitempty"`
	Objective       string      `json:"objective,omitempty"`
	CWD             string      `json:"cwd,omitempty"`
	ActiveCommand   string      `json:"activeCommand,omitempty"`
	AttachedClients *int        `json:"attachedClients"`
	Group           groupField  `json:"group,omitzero"`

	// Status is decoded and ignored: the phone's copy. Go clients render
	// sessions.PresentStatus over TerminalStatus.
	Status sessions.StatusPresentation `json:"status"`
}
type hostInventory struct {
	Machine struct {
		Handle   string `json:"handle"`
		Platform string `json:"platform"`
	} `json:"machine"`
	ObservedAt string        `json:"observedAt"`
	Profiles   []hostProfile `json:"profiles"`
	Sessions   []hostSession `json:"sessions"`
	Recovery   *Recovery     `json:"recovery"`
}
type hostObservedSession struct {
	ObservedAt string      `json:"observedAt"`
	Session    hostSession `json:"session"`
}

func (s hostSession) project(machine string) Session {
	ref := Reference{Machine: machine, TmuxID: s.TmuxID, IdentityToken: s.IdentityToken, PaneID: s.ActivePaneID}
	row := Session{TerminalStatus: s.TerminalStatus, TerminalUsage: s.TerminalUsage, TerminalFooter: s.TerminalFooter, ActivePaneID: s.ActivePaneID, Group: s.Group.label, Name: s.TmuxName, NameMode: s.NameMode, TerminalHandle: ref.Handle(), Ref: Ref{ref}, CWD: s.CWD, ActiveCommand: s.ActiveCommand, LaunchProfile: s.LaunchProfile, AttachedClients: *s.AttachedClients, Connection: s.Connection}
	if s.Agent != nil {
		row.Agent = &Agent{Provider: s.Agent.Provider, Profile: s.Agent.Profile, Label: s.Agent.Label, PID: int64(s.Agent.PID), StartIdentity: s.Agent.StartIdentity}
	}
	return row
}

func decodeResponse(operation string, encoded []byte, target peer) (any, bool) {
	if !nonNullJSON(encoded) {
		return nil, false
	}
	switch operation {
	case "profile_usage":
		return decodeProfileUsage(encoded, target)
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
		if strictjson.Decode(encoded, &value) != nil || value == nil || value.Machine.Handle != target.Machine || !slices.Contains([]string{"Linux", "Darwin"}, value.Machine.Platform) || value.Profiles == nil || value.Sessions == nil || value.Recovery == nil {
			return nil, false
		}
		if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
			return nil, false
		}
		profiles := make([]Profile, len(value.Profiles))
		for index, wire := range value.Profiles {
			profile, valid := wire.project()
			if !valid {
				return nil, false
			}
			profiles[index] = profile
		}
		observed := Peer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: value.ObservedAt, Profiles: profiles, Sessions: make([]Session, 0, len(value.Sessions)), Recovery: value.Recovery}
		for _, s := range value.Sessions {
			if !validSession(s) {
				return nil, false
			}
			observed.Sessions = append(observed.Sessions, s.project(target.Machine))
		}
		slices.SortStableFunc(observed.Sessions, CompareSessions)
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
	case "terminal_send", "terminal_keys", "terminal_text", "terminal_stop":
		var value *WriteResult
		if strictjson.Decode(encoded, &value) != nil || value == nil {
			return nil, false
		}
		if value.Method != "terminal" || value.Outcome != "written" && value.Outcome != "unknown" {
			return nil, false
		}
		return *value, true
	case "close":
		var value *CloseResult
		if strictjson.Decode(encoded, &value) != nil || value == nil || !slices.Contains([]string{"written", "not_sent", "unknown"}, value.Interrupt) || !slices.Contains([]string{"closed", "not_closed", "unknown"}, value.Terminal) {
			return nil, false
		}
		return *value, true
	case "terminal_inspect", "terminal_explain":
		// Pointers expose omitted members; nonNullJSON has already rejected null.
		var value *struct {
			TerminalStatus sessions.TerminalStatus   `json:"terminalStatus"`
			Diagnostics    *agentcontrol.Diagnostics `json:"diagnostics"`
		}
		if strictjson.Decode(encoded, &value) != nil || value == nil || !value.TerminalStatus.Valid() || (value.Diagnostics != nil) != (operation == "terminal_explain") || value.Diagnostics != nil && value.Diagnostics.Rules == nil {
			return nil, false
		}
		return TerminalInspectResult{TerminalStatus: value.TerminalStatus, Diagnostics: value.Diagnostics}, true
	default:
		return nil, false
	}

}

func validSession(s hostSession) bool {
	if s.NameMode != "automatic" && s.NameMode != "manual" {
		return false
	}
	if !tmuxAddress(s.ActivePaneID, '%') || !tmuxAddress(s.TmuxID, '$') || s.TmuxName == "" || s.IdentityToken == "" || s.AttachedClients == nil || *s.AttachedClients < 0 || !s.TerminalStatus.Valid() {
		return false
	}
	if s.Connection != nil && (s.Agent != nil || s.CWD != "" || !validConnection(*s.Connection)) {
		return false
	}
	a := s.Agent
	if s.TerminalUsage != nil && (a == nil || !slices.Contains([]string{"Codex", "Claude"}, a.Provider) || s.Connection != nil || !s.TerminalUsage.ValidFor(s.TerminalStatus)) {
		return false
	}
	if s.TerminalFooter != nil && (a == nil || s.Connection != nil || !s.TerminalFooter.ValidFor(s.TerminalStatus)) {
		return false
	}
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
func decodeMutationFailure(operation string, encoded []byte, status int, machine string) *Failure {
	var value struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Dispatch string `json:"dispatch"`
		Target   *struct {
			TmuxID        string `json:"tmuxId"`
			IdentityToken string `json:"identityToken"`
			PaneID        string `json:"paneId"`
		} `json:"target,omitempty"`
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
	case "InternalError":
		wantStatus, wantMessage = http.StatusInternalServerError, "Skíðblaðnir could not complete the request."
	default:
		return nil
	}
	if status != wantStatus || value.Message != wantMessage || value.Code != "InternalError" && value.Dispatch != "not_sent" {
		return nil
	}
	if value.Target != nil && (operation != "start" && operation != "shell" || value.Code != "InternalError" || value.Dispatch != "unknown" || !tmuxAddress(value.Target.TmuxID, '$') || value.Target.IdentityToken == "" || !tmuxAddress(value.Target.PaneID, '%')) {
		return nil
	}
	failure := &Failure{Code: value.Code, Dispatch: value.Dispatch}
	if value.Target != nil {
		failure.Target = Ref{Reference{Machine: machine, TmuxID: value.Target.TmuxID, IdentityToken: value.Target.IdentityToken, PaneID: value.Target.PaneID}}
	}
	return failure
}

type WaitResult struct {
	Outcome        string                   `json:"outcome"`
	Target         Ref                      `json:"target"`
	TerminalStatus *sessions.TerminalStatus `json:"terminalStatus,omitempty"`
}

// Fleet schemas distinguish omission from null.
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

// TerminalInspectResult is one decoded inspect sample; Diagnostics are present
// exactly when explain was requested.
type TerminalInspectResult struct {
	TerminalStatus sessions.TerminalStatus
	Diagnostics    *agentcontrol.Diagnostics
}
