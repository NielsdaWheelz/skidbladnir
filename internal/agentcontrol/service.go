package agentcontrol

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var (
	ErrInvalidInput   = errors.New("invalid agent input")
	ErrHistoryChanged = errors.New("native history changed")
)

type UnavailableError struct{ Dispatch string }

func (err *UnavailableError) Error() string { return "native agent action unavailable" }

type nativeStaleError struct{ Dispatch string }

func (err *nativeStaleError) Error() string         { return "native agent target changed" }
func (err *nativeStaleError) Unwrap() error         { return sessions.ErrAgentTargetStale }
func (err *nativeStaleError) DispatchState() string { return err.Dispatch }

type Service struct {
	sessions   *sessions.Manager
	nativePath string
}

func New(manager *sessions.Manager, nativeControlPath string) (*Service, error) {
	if manager == nil || !filepath.IsAbs(nativeControlPath) {
		return nil, errors.New("agent control requires sessions and an absolute native command")
	}
	return &Service{sessions: manager, nativePath: nativeControlPath}, nil
}

type ReadResult struct {
	Text         string                    `json:"text"`
	Source       string                    `json:"source"`
	Scope        string                    `json:"scope"`
	Truncated    bool                      `json:"truncated"`
	Observation  *agentruntime.Observation `json:"observation,omitempty"`
	OutputState  string                    `json:"outputState,omitempty"`
	OutputID     string                    `json:"outputId,omitempty"`
	OutputTurnID string                    `json:"outputTurnId,omitempty"`
}

type WriteResult struct {
	Method  string `json:"method"`
	Outcome string `json:"outcome"`
}

type SendResult struct {
	Method          string `json:"method"`
	Input           string `json:"input"`
	Delivery        string `json:"delivery"`
	Outcome         string `json:"outcome"`
	TurnID          string `json:"turnId,omitempty"`
	QueueItemID     string `json:"queueItemId,omitempty"`
	ClientMessageID string `json:"clientMessageId,omitempty"`
}

type CloseResult struct {
	Agent    string `json:"agent"`
	Terminal string `json:"terminal"`
	Reason   string `json:"reason,omitempty"`
}

type ResultsResult struct {
	Conversation agentruntime.Conversation `json:"conversation"`
	ResultIDs    []string                  `json:"resultIds"`
	NextCursor   string                    `json:"nextCursor,omitempty"`
}

func (service *Service) Enrich(parent context.Context, inventory *sessions.Inventory) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	groups := make(map[agentruntime.ProfileKey][]int)
	for index, session := range inventory.Sessions {
		if session.Agent == nil {
			continue
		}
		session.Agent.Status = agentruntime.Status{State: "unknown", Source: "unavailable"}
		session.Agent.Methods = agentruntime.UnavailableMethods(session.Agent.Provider)
		session.Agent.Binding = nil
		session.Agent.Turn = nil
		if profile, _, ok := service.nativeIdentity(session); ok {
			groups[profile.Key] = append(groups[profile.Key], index)
		}
	}
	var work sync.WaitGroup
	for key, indices := range groups {
		work.Add(1)
		go func() {
			defer work.Done()
			profile, _ := service.sessions.Profile(key)
			targets := make([]nativeTarget, len(indices))
			for i, index := range indices {
				_, targets[i], _ = service.nativeIdentity(inventory.Sessions[index])
			}
			var results []nativeEnvelope
			if service.native(ctx, profile, "inspect", targets, nil, &results) != nil || len(results) != len(indices) {
				return
			}
			for i, result := range results {
				session := inventory.Sessions[indices[i]]
				var inspected nativeInspection
				if decodeNativeEnvelope(result, &inspected, &UnavailableError{Dispatch: "not_sent"}) != nil {
					continue
				}
				observation, err := service.observation(profile, inspected)
				if err != nil {
					continue
				}
				if _, err := service.sessions.ResolveAgent(ctx, sessions.TargetOf(session)); err != nil {
					continue
				}
				session.Agent.Status = observation.Status
				session.Agent.Methods = inspected.Methods
				session.Agent.Binding = &observation.Binding
				session.Agent.Turn = observation.Turn
			}
		}()
	}
	work.Wait()
}

func (service *Service) nativeIdentity(session sessions.Session) (agentruntime.Profile, nativeTarget, bool) {
	agent := session.Agent
	if agent == nil || session.ActivePaneID != agent.PaneID {
		return agentruntime.Profile{}, nativeTarget{}, false
	}
	profile, found := service.sessions.Profile(agent.Profile)
	if !found || profile.Provider != agent.Provider || profile.Provider == agentruntime.ProviderCodex && profile.Endpoint == "" {
		return profile, nativeTarget{}, false
	}
	target := nativeTarget{PID: int(agent.PID), StartIdentity: string(agent.StartIdentity)}
	if agent.ProviderSession != nil {
		target.SessionID = agent.ProviderSession.ID()
	}
	return profile, target, true
}

func historyScope(profile agentruntime.Profile) (string, error) {
	name := "CODEX_HOME"
	if profile.Provider == agentruntime.ProviderClaude {
		name = "CLAUDE_CONFIG_DIR"
	}
	for _, entry := range profile.Environment {
		if entry.Name != name {
			continue
		}
		home, err := filepath.EvalSymlinks(entry.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", sha256.Sum256([]byte(string(profile.Provider)+"\x00"+home))), nil
	}
	return "", errors.New("profile omits native history home")
}

func (service *Service) observation(profile agentruntime.Profile, inspected nativeInspection) (agentruntime.Observation, error) {
	if !inspected.Status.Valid() || !inspected.Methods.Valid() || inspected.SessionID == "" || inspected.Turn != nil && !inspected.Turn.Valid() {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	scope, err := historyScope(profile)
	if err != nil {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	binding := agentruntime.Binding{Conversation: agentruntime.Conversation{Provider: profile.Provider, ProfileKey: profile.Key, HistoryScope: scope, ConversationID: inspected.SessionID}, View: inspected.View}
	if !binding.Valid() {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	return agentruntime.Observation{Binding: binding, Status: inspected.Status, Turn: inspected.Turn}, nil
}

func (service *Service) inspect(ctx context.Context, session sessions.Session) (agentruntime.Profile, nativeTarget, nativeInspection, agentruntime.Observation, error) {
	profile, target, available := service.nativeIdentity(session)
	if !available {
		return profile, target, nativeInspection{}, agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	inspectContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var results []nativeEnvelope
	if err := service.native(inspectContext, profile, "inspect", []nativeTarget{target}, nil, &results); err != nil {
		return profile, target, nativeInspection{}, agentruntime.Observation{}, err
	}
	if len(results) != 1 {
		return profile, target, nativeInspection{}, agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	var inspected nativeInspection
	if err := decodeNativeEnvelope(results[0], &inspected, &UnavailableError{Dispatch: "not_sent"}); err != nil {
		return profile, target, inspected, agentruntime.Observation{}, err
	}
	observation, err := service.observation(profile, inspected)
	target.SessionID = inspected.SessionID
	target.View = inspected.View
	return profile, target, inspected, observation, err
}

func (service *Service) bound(ctx context.Context, target sessions.AgentTarget) (agentruntime.Profile, nativeTarget, nativeInspection, agentruntime.Observation, error) {
	if target.Binding == nil || !target.Binding.Valid() {
		return agentruntime.Profile{}, nativeTarget{}, nativeInspection{}, agentruntime.Observation{}, ErrInvalidInput
	}
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return agentruntime.Profile{}, nativeTarget{}, nativeInspection{}, agentruntime.Observation{}, err
	}
	profile, native, inspected, observation, err := service.inspect(ctx, session)
	if err == nil && !target.Binding.Equal(observation.Binding) {
		err = sessions.ErrAgentTargetStale
	}
	if err == nil {
		_, err = service.sessions.ResolveAgent(ctx, target)
	}
	return profile, native, inspected, observation, err
}

func (service *Service) Read(parent context.Context, target sessions.AgentTarget, mode, scope string, maxBytes int) (ReadResult, error) {
	if maxBytes == 0 {
		maxBytes = 16384
	}
	if maxBytes < 1 || maxBytes > 32768 || mode != "native" && mode != "terminal" || mode == "native" && scope != "latest" && scope != "history" || mode == "terminal" && scope != "" {
		return ReadResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if mode == "terminal" {
		capture, err := service.sessions.CaptureAgent(ctx, target, maxBytes)
		if err != nil {
			return ReadResult{}, err
		}
		captureScope := "terminal_history"
		if capture.Alternate {
			captureScope = "visible"
		}
		return ReadResult{Text: capture.Text, Source: "terminal", Scope: captureScope, Truncated: capture.Truncated}, nil
	}
	profile, native, inspected, before, err := service.bound(ctx, target)
	if err != nil {
		return ReadResult{}, err
	}
	if inspected.Methods.Read != "native" {
		return ReadResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	var read struct {
		Text         *string          `json:"text"`
		Source       string           `json:"source"`
		Scope        string           `json:"scope"`
		Truncated    *bool            `json:"truncated"`
		OutputState  string           `json:"outputState"`
		OutputID     string           `json:"outputId,omitempty"`
		OutputTurnID string           `json:"outputTurnId,omitempty"`
		Inspection   nativeInspection `json:"inspection"`
	}
	if err := service.native(ctx, profile, "read", []nativeTarget{native}, struct {
		Scope    string `json:"scope"`
		MaxBytes int    `json:"maxBytes"`
	}{scope, maxBytes}, &read); err != nil {
		return ReadResult{}, err
	}
	after, err := service.observation(profile, read.Inspection)
	if err != nil {
		return ReadResult{}, err
	}
	if !before.Binding.Equal(after.Binding) {
		return ReadResult{}, sessions.ErrAgentTargetStale
	}
	if _, err := service.sessions.ResolveAgent(ctx, target); err != nil {
		return ReadResult{}, err
	}
	if read.Text == nil || read.Truncated == nil || read.Source != "native" || read.Scope != scope || !utf8.ValidString(*read.Text) {
		return ReadResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	switch read.OutputState {
	case "partial", "finalized", "unknown", "none":
	default:
		return ReadResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	if read.OutputState == "finalized" && read.OutputID == "" || read.OutputState == "none" && (scope == "latest" && *read.Text != "" || read.OutputID != "" || read.OutputTurnID != "") {
		return ReadResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	result := ReadResult{Text: *read.Text, Source: read.Source, Scope: read.Scope, Truncated: *read.Truncated, Observation: &after, OutputState: read.OutputState, OutputID: read.OutputID, OutputTurnID: read.OutputTurnID}
	if len(result.Text) > maxBytes {
		result.Text = result.Text[len(result.Text)-maxBytes:]
		for len(result.Text) > 0 && !utf8.RuneStart(result.Text[0]) {
			result.Text = result.Text[1:]
		}
		result.Truncated = true
	}
	return result, nil
}

func (service *Service) Results(parent context.Context, tmuxID, identityToken string, conversation agentruntime.Conversation, cursor string) (ResultsResult, error) {
	if !conversation.Valid() || len(cursor) > 32768 || !utf8.ValidString(cursor) {
		return ResultsResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := service.sessions.ResolveSession(ctx, tmuxID, identityToken); err != nil {
		return ResultsResult{}, err
	}
	profile, found := service.sessions.Profile(conversation.ProfileKey)
	if !found || profile.Provider != conversation.Provider {
		return ResultsResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	scope, err := historyScope(profile)
	if err != nil || scope != conversation.HistoryScope {
		return ResultsResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	var page struct {
		ResultIDs  []string `json:"resultIds"`
		NextCursor string   `json:"nextCursor,omitempty"`
	}
	var input any
	if cursor != "" {
		input = struct {
			Cursor string `json:"cursor"`
		}{cursor}
	}
	if err := service.native(ctx, profile, "results", []nativeTarget{{SessionID: conversation.ConversationID}}, input, &page); err != nil {
		return ResultsResult{}, err
	}
	if page.ResultIDs == nil || len(page.ResultIDs) > 128 || len(page.NextCursor) > 32768 || page.NextCursor != "" && page.NextCursor == cursor {
		return ResultsResult{}, &UnavailableError{Dispatch: "not_sent"}
	}
	seen := make(map[string]struct{}, len(page.ResultIDs))
	for _, id := range page.ResultIDs {
		if id == "" || len(id) > 128 || !utf8.ValidString(id) {
			return ResultsResult{}, &UnavailableError{Dispatch: "not_sent"}
		}
		if _, found := seen[id]; found {
			return ResultsResult{}, &UnavailableError{Dispatch: "not_sent"}
		}
		seen[id] = struct{}{}
	}
	if err := service.sessions.ResolveSession(ctx, tmuxID, identityToken); err != nil {
		return ResultsResult{}, err
	}
	return ResultsResult{Conversation: conversation, ResultIDs: page.ResultIDs, NextCursor: page.NextCursor}, nil
}
