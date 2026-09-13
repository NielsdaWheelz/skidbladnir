package agentcontrol

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

var (
	ErrBlocked      = errors.New("agent requires deliberate terminal input")
	ErrInvalidInput = errors.New("invalid agent input")
)

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
	Text      string `json:"text"`
	Source    string `json:"source"`
	Scope     string `json:"scope"`
	Truncated bool   `json:"truncated"`
}

type WriteResult struct {
	Method  string `json:"method"`
	Outcome string `json:"outcome"`
}

type StopResult struct {
	Agent    string `json:"agent"`
	Terminal string `json:"terminal"`
	Reason   string `json:"reason,omitempty"`
}

func (service *Service) Enrich(parent context.Context, inventory *sessions.Inventory) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	groups := make(map[agentruntime.ProfileKey][]int)
	nativeResults := make([]*nativeInspection, len(inventory.Sessions))
	terminalResults := make([]agentruntime.Status, len(inventory.Sessions))
	var work sync.WaitGroup
	for i, session := range inventory.Sessions {
		if session.Agent == nil {
			continue
		}
		if profile, _, ok := service.nativeIdentity(session); ok {
			groups[profile.Key] = append(groups[profile.Key], i)
		}
		work.Add(1)
		go func(index int, session sessions.Session) {
			defer work.Done()
			capture, err := service.sessions.CaptureAgent(ctx, sessions.TargetOf(session), 8192)
			if err == nil {
				terminalResults[index] = Detect(session.Agent.Provider, capture.Text)
			}
		}(i, session)
	}
	for key, indices := range groups {
		work.Add(1)
		go func(key agentruntime.ProfileKey, indices []int) {
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
				var inspected nativeInspection
				if !result.OK || strictjson.Decode(result.Result, &inspected) != nil || !validInspection(inspected) {
					continue
				}
				nativeResults[indices[i]] = &inspected
			}
		}(key, indices)
	}
	work.Wait()
	for i, session := range inventory.Sessions {
		if session.Agent == nil {
			continue
		}
		if inspected := nativeResults[i]; inspected != nil {
			session.Agent.Status = inspected.Status
			session.Agent.Methods = inspected.Methods
		} else if terminalResults[i].Valid() {
			session.Agent.Status = terminalResults[i]
		}
	}
}

func validInspection(value nativeInspection) bool {
	return value.Status.Valid() && value.Status.Source == "native" && value.Methods.Valid() &&
		value.Methods.Send == "terminal" && value.Methods.Interrupt == "terminal"
}

func (service *Service) nativeIdentity(session sessions.Session) (agentruntime.Profile, nativeTarget, bool) {
	agent := session.Agent
	if agent == nil || agent.Provider != agentruntime.ProviderClaude {
		return agentruntime.Profile{}, nativeTarget{}, false
	}
	profile, found := service.sessions.Profile(agent.Profile)
	if !found {
		return agentruntime.Profile{}, nativeTarget{}, false
	}
	target := nativeTarget{PID: int(agent.PID)}
	if agent.ProviderSession != nil {
		target.SessionID = agent.ProviderSession.ID()
	}
	return profile, target, true
}

func (service *Service) inspect(ctx context.Context, session sessions.Session) (agentruntime.Profile, nativeTarget, nativeInspection, bool) {
	profile, target, available := service.nativeIdentity(session)
	if !available {
		return profile, target, nativeInspection{}, false
	}
	inspectionContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var results []nativeEnvelope
	if service.native(inspectionContext, profile, "inspect", []nativeTarget{target}, nil, &results) != nil || len(results) != 1 || !results[0].OK {
		return profile, target, nativeInspection{}, false
	}
	var inspected nativeInspection
	if strictjson.Decode(results[0].Result, &inspected) != nil || !validInspection(inspected) {
		return profile, target, nativeInspection{}, false
	}
	if inspected.SessionID != "" {
		target.SessionID = inspected.SessionID
	}
	return profile, target, inspected, true
}

func (service *Service) Read(parent context.Context, target sessions.AgentTarget, mode string, maxBytes int) (ReadResult, error) {
	if maxBytes == 0 {
		maxBytes = 16384
	}
	if maxBytes < 1 || maxBytes > 32768 || mode != "" && mode != "auto" && mode != "terminal" {
		return ReadResult{}, ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	session, err := service.sessions.ResolveAgent(ctx, target)
	if err != nil {
		return ReadResult{}, err
	}
	if mode != "terminal" {
		nativeContext, cancelNative := context.WithDeadline(ctx, time.Now().Add(8*time.Second))
		defer cancelNative()
		profile, native, inspected, ok := service.inspect(nativeContext, session)
		if ok && inspected.Methods.Read == "native" {
			var result ReadResult
			if _, err := service.sessions.ResolveAgent(nativeContext, target); err != nil {
				return ReadResult{}, err
			}
			if service.native(nativeContext, profile, "read", []nativeTarget{native}, struct {
				MaxBytes int `json:"maxBytes"`
			}{maxBytes}, &result) == nil && validRead(result) {
				boundRead(&result, maxBytes)
				return result, nil
			}
		}
	}
	capture, err := service.sessions.CaptureAgent(ctx, target, maxBytes)
	if err != nil {
		return ReadResult{}, err
	}
	scope := "terminal_history"
	if capture.Alternate {
		scope = "visible"
	}
	return ReadResult{Text: capture.Text, Source: "terminal", Scope: scope, Truncated: capture.Truncated}, nil
}

func validRead(result ReadResult) bool {
	if result.Source != "native" || !utf8.ValidString(result.Text) {
		return false
	}
	return result.Scope == "recent_messages" || result.Scope == "latest_turn"
}

func boundRead(result *ReadResult, maxBytes int) {
	if len(result.Text) <= maxBytes {
		return
	}
	result.Text = result.Text[len(result.Text)-maxBytes:]
	for len(result.Text) > 0 && !utf8.RuneStart(result.Text[0]) {
		result.Text = result.Text[1:]
	}
	result.Truncated = true
}
