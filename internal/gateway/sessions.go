package gateway

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// Request members decode null as absent. A pointer marks a member whose
// absence answers differently from its empty value.
type createSessionRequest struct {
	Kind             sessions.LaunchKind `json:"kind"`
	CWD              *string             `json:"cwd"`
	Profile          *string             `json:"profile"`
	OptionalTmuxName *string             `json:"optionalTmuxName"`
	Objective        *string             `json:"objective"`
	Group            *string             `json:"group"`
}

// identityRequest addresses a session lifetime: shell creation and deletion.
type identityRequest struct {
	IdentityToken string `json:"identityToken"`
}

type namingRequest struct {
	Mode string  `json:"mode"`
	Name *string `json:"name"`
}

// valid reports whether the naming has its mode's shape: automatic has no name, manual has one.
func (field namingRequest) valid() bool {
	return field.Mode == "automatic" && field.Name == nil || field.Mode == "manual" && field.Name != nil
}

func (field namingRequest) naming() sessions.Naming {
	return sessions.Naming{Mode: sessions.NameMode(field.Mode), Name: valueOf(field.Name)}
}

type renameSessionRequest struct {
	ExpectedNaming *namingRequest `json:"expectedNaming"`
	Naming         *namingRequest `json:"naming"`
	IdentityToken  string         `json:"identityToken"`
}

type setSessionGroupRequest struct {
	IdentityToken string  `json:"identityToken"`
	Group         *string `json:"group"`
}

func (gateway *Gateway) listSessions(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	inventory, failures, err := gateway.agents.Inventory(ctx)
	for _, failure := range failures {
		gateway.logObservationFailure(failure.TmuxID, failure.Reason, failure.Elapsed)
	}
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	// inventory.Profiles is the one settings snapshot every launch profile and agent profile in it names.
	document := api.Inventory{
		Machine:    gateway.apiMachine(),
		ObservedAt: inventory.ObservedAt.UTC(),
		Recovery:   api.Recovery{Recovery: inventory.Recovery, HostNotice: inventory.Recovery.Notice()},
		Profiles:   make([]api.Profile, len(inventory.Profiles)),
		Sessions:   make([]api.Session, len(inventory.Sessions)),
	}
	for index, profile := range inventory.Profiles {
		document.Profiles[index] = apiProfile(profile)
	}
	for index, session := range inventory.Sessions {
		document.Sessions[index] = apiSession(session, inventory.Profiles)
	}
	// The five-second budget covers enrichment and projection: a late inventory fails whole.
	if ctx.Err() != nil {
		writeError(writer, errorInternal)
		return
	}
	sort.Slice(document.Sessions, func(left, right int) bool {
		leftID, _ := strconv.ParseUint(strings.TrimPrefix(document.Sessions[left].TmuxID, "$"), 10, 64)
		rightID, _ := strconv.ParseUint(strings.TrimPrefix(document.Sessions[right].TmuxID, "$"), 10, 64)
		if leftID != rightID {
			return leftID < rightID
		}
		return document.Sessions[left].TmuxID < document.Sessions[right].TmuxID
	})
	writeJSON(writer, http.StatusOK, document, nil)
}

func (gateway *Gateway) createSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, ok := decodeJSON[createSessionRequest](writer, request)
	if !ok {
		return
	}
	if input.CWD == nil {
		writeError(writer, errorInvalidRequest)
		return
	}
	switch input.Kind {
	case sessions.LaunchAgent:
		if input.Profile == nil {
			writeError(writer, errorInvalidRequest)
			return
		}
	case sessions.LaunchTerminal:
		if input.Profile != nil {
			writeError(writer, errorInvalidRequest)
			return
		}
	default:
		writeError(writer, errorInvalidRequest)
		return
	}
	// Present but empty is invalid; absent means none.
	if input.OptionalTmuxName != nil && *input.OptionalTmuxName == "" {
		writeError(writer, errorSessionNameInvalid)
		return
	}
	if input.Objective != nil && *input.Objective == "" {
		writeError(writer, errorObjectiveInvalid)
		return
	}
	label, err := group.ParseDraft(valueOf(input.Group))
	if err != nil || input.Group != nil && label.IsUnassigned() {
		writeError(writer, errorGroupInvalid)
		return
	}
	created, err := gateway.sessions.Create(request.Context(), sessions.CreateInput{
		Kind:             input.Kind,
		CWD:              *input.CWD,
		Profile:          valueOf(input.Profile),
		OptionalTmuxName: valueOf(input.OptionalTmuxName),
		Objective:        valueOf(input.Objective),
		Group:            label,
	})
	gateway.completeCreation(request.Context(), writer, created, err, startedAt)
}

func (gateway *Gateway) createShell(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, ok := decodeJSON[identityRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	created, err := gateway.sessions.CreateShell(request.Context(), sessions.ShellInput{TmuxID: request.PathValue("tmuxId"), IdentityToken: input.IdentityToken})
	gateway.completeCreation(request.Context(), writer, created, err, startedAt)
}

func (gateway *Gateway) completeCreation(ctx context.Context, writer http.ResponseWriter, created sessions.ObservedSession, err error, startedAt time.Time) {
	if errors.Is(err, sessions.ErrCreateDispatchUnknown) {
		failure := errorInternal
		failure.Dispatch = "unknown"
		failure.Target = capturedCreationTarget(created)
		writeError(writer, failure)
		return
	}
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	// Session.Created times creation alone; the status observation below is response latency.
	elapsed := time.Since(startedAt)
	observing := time.Now()
	created.Session = gateway.agents.EnrichCreated(ctx, created.Session)
	if status := created.Session.TerminalStatus; status.Source == sessions.SourceUnavailable {
		gateway.logObservationFailure(created.Session.TmuxID, status.Reason, time.Since(observing))
	}
	attrs := []any{"skidbladnir.duration.ms", elapsed.Milliseconds(), "skidbladnir.session.tmux_id", created.Session.TmuxID}
	if created.Session.LaunchProfile != "" {
		attrs = append(attrs, "skidbladnir.session.launch_profile", string(created.Session.LaunchProfile))
	}
	gateway.logger.Info("Session.Created", attrs...)
	writeJSON(writer, http.StatusCreated, api.ObservedSession{ObservedAt: created.ObservedAt.UTC(), Session: apiSession(created.Session, gateway.sessions.Profiles())}, nil)
}

func capturedCreationTarget(created sessions.ObservedSession) *api.Target {
	// Manager only retains a partial observation once it has validated the
	// complete target.
	if created.Session.TmuxID == "" || created.Session.IdentityToken == "" || created.Session.ActivePaneID == "" {
		return nil
	}
	return &api.Target{TmuxID: created.Session.TmuxID, IdentityToken: created.Session.IdentityToken, PaneID: created.Session.ActivePaneID}
}

func (gateway *Gateway) killSession(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	tmuxID := request.PathValue("tmuxId")
	input, ok := decodeJSON[identityRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	if err := gateway.sessions.ResolveSession(ctx, tmuxID, input.IdentityToken); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	if err := gateway.closeSession(ctx, tmuxID, input.IdentityToken); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) renameSession(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[renameSessionRequest](writer, request)
	if !ok {
		return
	}
	if input.ExpectedNaming == nil || input.Naming == nil || !input.ExpectedNaming.valid() || !input.Naming.valid() || input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	err := gateway.sessions.Rename(request.Context(), sessions.RenameInput{TmuxID: request.PathValue("tmuxId"), IdentityToken: input.IdentityToken, ExpectedNaming: input.ExpectedNaming.naming(), Naming: input.Naming.naming()})
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) setSessionGroup(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("tmuxId")
	if !tmux.ValidID(id, '$') {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, ok := decodeJSON[setSessionGroupRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" || input.Group == nil {
		writeError(writer, errorInvalidRequest)
		return
	}
	label, err := group.ParseDraft(*input.Group)
	if err != nil {
		writeError(writer, errorGroupInvalid)
		return
	}
	err = gateway.sessions.SetGroup(request.Context(), sessions.SetGroupInput{
		TmuxID: id, IdentityToken: input.IdentityToken, Group: label,
	})
	if errors.Is(err, sessions.ErrGroupDispatchUnknown) {
		failure := errorInternal
		failure.Dispatch = "unknown"
		writeError(writer, failure)
		return
	}
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func apiProfile(profile agentruntime.Profile) api.Profile {
	wire := api.Profile{Key: string(profile.Key), Label: profile.Label, Provider: profile.Provider.String()}
	// An absent account home omits this profile's quota scope without hiding its terminal launch.
	if scope, err := agentruntime.AccountScope(profile); err == nil {
		wire.AccountScope = scope
	}
	return wire
}

// apiSession maps one observed session; profiles labels its agent's profile.
func apiSession(session sessions.Session, profiles []agentruntime.Profile) api.Session {
	wire := api.Session{
		TerminalStatus:  session.TerminalStatus,
		TerminalUsage:   session.TerminalUsage,
		TerminalFooter:  session.TerminalFooter,
		TmuxID:          session.TmuxID,
		ActivePaneID:    session.ActivePaneID,
		TmuxName:        session.TmuxName,
		NameMode:        session.NameMode,
		IdentityToken:   session.IdentityToken,
		Character:       api.Character{Key: session.Character.Key, DisplayName: session.Character.DisplayName},
		LaunchProfile:   string(session.LaunchProfile),
		Objective:       session.Objective,
		Group:           session.Group.String(),
		CWD:             session.CWD,
		ActiveCommand:   session.ActiveCommand,
		AttachedClients: session.AttachedClients,
		Status:          sessions.PresentStatus(session.TerminalStatus, session.Agent != nil && session.Connection == nil),
	}
	if agent := session.Agent; agent != nil {
		wire.Agent = &api.Agent{PaneID: session.ActivePaneID, StartIdentity: string(agent.StartIdentity), Provider: agent.Provider.String(), PID: int64(agent.PID), Profile: string(agent.Profile), Label: profileLabel(profiles, agent.Profile)}
	}
	if connection := session.Connection; connection != nil {
		wire.Connection = &api.Connection{Transport: connection.Transport, ID: connection.ID}
	}
	return wire
}

// profileLabel is key's label in profiles; empty when key is empty or not
// in this snapshot.
func profileLabel(profiles []agentruntime.Profile, key agentruntime.ProfileKey) string {
	for _, profile := range profiles {
		if profile.Key == key {
			return profile.Label
		}
	}
	return ""
}
