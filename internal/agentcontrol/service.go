package agentcontrol

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
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

// Enrich sets each session's TerminalStatus from one focused observation of
// the identity List or creation captured, never resolving it again. Every
// session is observed concurrently under one shared two-second deadline. It
// returns the unavailable observations, each timed from Enrich's start, for
// content-free logging.
func (service *Service) Enrich(parent context.Context, observed []sessions.Session) []ObservationFailure {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	elapsed := make([]time.Duration, len(observed))
	defects := make([]string, len(observed))
	var work sync.WaitGroup
	for index := range observed {
		work.Go(func() {
			// A panic on a worker goroutine would end the gateway; the defect is
			// raised again below on the caller's goroutine, where net/http confines
			// it to the one request, as it does for inspect and send.
			defer func() {
				if defect := recover(); defect != nil {
					defects[index] = fmt.Sprintf("%v\n\n%s", defect, debug.Stack())
				}
			}()
			status, _, _, err := service.sample(ctx, observed[index])
			if errors.Is(err, sessions.ErrTerminalTargetChanged) {
				// justify-ignore-error: inventory has no target to reject, so a lifetime or pane change during capture is a failed capture; explicit operations return it.
				status = sessions.UnknownStatus(sessions.ReasonCaptureFailed)
			}
			observed[index].TerminalStatus = status
			elapsed[index] = time.Since(startedAt)
		})
	}
	work.Wait()
	for _, defect := range defects {
		if defect != "" {
			panic(defect) // justify-defect: a worker's classifier or observation defect, with the worker's stack.
		}
	}
	var failures []ObservationFailure
	for index, session := range observed {
		if session.TerminalStatus.Source == sessions.SourceUnavailable {
			failures = append(failures, ObservationFailure{TmuxID: session.TmuxID, Reason: session.TerminalStatus.Reason, Elapsed: elapsed[index]})
		}
	}
	return failures
}

func (service *Service) conversationTarget(conversation agentruntime.Conversation) (agentruntime.Profile, nativeTarget, error) {
	profile, found := service.sessions.Profile(conversation.ProfileKey)
	if !found || profile.Provider != conversation.Provider {
		return profile, nativeTarget{}, sessions.ErrAgentTargetStale
	}
	scope, err := agentruntime.HistoryScope(profile)
	if err != nil {
		return profile, nativeTarget{}, &UnavailableError{Dispatch: "not_sent"}
	}
	if scope != conversation.HistoryScope {
		return profile, nativeTarget{}, sessions.ErrAgentTargetStale
	}
	return profile, nativeTarget{SessionID: conversation.ConversationID}, nil
}

func (service *Service) observation(profile agentruntime.Profile, inspected nativeInspection) (agentruntime.Observation, error) {
	if !inspected.Status.Valid() || !inspected.Methods.Valid() || inspected.SessionID == "" || inspected.Turn != nil && !inspected.Turn.Valid() {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	scope, err := agentruntime.HistoryScope(profile)
	if err != nil {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	binding := agentruntime.Binding{Conversation: agentruntime.Conversation{Provider: profile.Provider, ProfileKey: profile.Key, HistoryScope: scope, ConversationID: inspected.SessionID}}
	if !binding.Valid() {
		return agentruntime.Observation{}, &UnavailableError{Dispatch: "not_sent"}
	}
	return agentruntime.Observation{Binding: binding, Status: inspected.Status, Turn: inspected.Turn}, nil
}

func (service *Service) Inspect(parent context.Context, conversation agentruntime.Conversation) (agentruntime.ConversationRuntime, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	_, _, inspected, observation, err := service.bound(ctx, conversation)
	if err != nil {
		return agentruntime.ConversationRuntime{}, err
	}
	return agentruntime.ConversationRuntime{Binding: observation.Binding, Status: observation.Status, Methods: inspected.Methods, Turn: observation.Turn}, nil
}

func (service *Service) bound(ctx context.Context, conversation agentruntime.Conversation) (agentruntime.Profile, nativeTarget, nativeInspection, agentruntime.Observation, error) {
	profile, target, err := service.conversationTarget(conversation)
	if err != nil {
		return profile, target, nativeInspection{}, agentruntime.Observation{}, err
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
	if err == nil && observation.Binding.Conversation != conversation {
		err = sessions.ErrAgentTargetStale
	}
	return profile, target, inspected, observation, err
}

func (service *Service) Read(parent context.Context, conversation agentruntime.Conversation, scope string, maxBytes int) (ReadResult, error) {
	if maxBytes == 0 {
		maxBytes = 16384
	}
	if maxBytes < 1 || maxBytes > 32768 || scope != "latest" && scope != "history" {
		return ReadResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	profile, native, inspected, before, err := service.bound(ctx, conversation)
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

func (service *Service) Results(parent context.Context, conversation agentruntime.Conversation, cursor string) (ResultsResult, error) {
	if !conversation.Valid() || len(cursor) > 4096 || !utf8.ValidString(cursor) {
		return ResultsResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	profile, _, err := service.conversationTarget(conversation)
	if err != nil {
		return ResultsResult{}, err
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
	if page.ResultIDs == nil || len(page.ResultIDs) > 128 || len(page.NextCursor) > 4096 || page.NextCursor != "" && page.NextCursor == cursor {
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
	return ResultsResult{Conversation: conversation, ResultIDs: page.ResultIDs, NextCursor: page.NextCursor}, nil
}
