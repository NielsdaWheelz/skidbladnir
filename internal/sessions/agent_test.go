package sessions

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	processinfo "github.com/NielsdaWheelz/skidbladnir/internal/process"
	"testing"
)

func TestAgentIdentityProjectionRemainsOptionalAndProcessBound(t *testing.T) {
	profiles := []agentruntime.Profile{{
		Key:      "work",
		Label:    "Codex · Work",
		Provider: agentruntime.ProviderCodex,
		ForegroundSignatures: []agentruntime.ForegroundSignature{{
			ExecutableBase: "codex",
		}},
	}}
	observed := processinfo.Observation{
		PID:           4312,
		StartIdentity: "991827",
		Executable:    "/opt/skid/bin/codex",
	}
	registration, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider:      agentruntime.ProviderCodex,
		PID:           observed.PID,
		StartIdentity: observed.StartIdentity,
	}, "work", "thr_123")
	if err != nil {
		t.Fatalf("encode exact runtime registration: %v", err)
	}

	agent := deriveAgent(profiles, observed, registration)
	if agent == nil || agent.Provider != agentruntime.ProviderCodex || agent.PID != observed.PID ||
		agent.Profile != "work" || agent.ProviderSession == nil || agent.ProviderSession.ID() != "thr_123" {
		t.Fatalf("exact process-bound agent identity = %+v", agent)
	}

	stale, err := agentruntime.EncodeRegistration(agentruntime.Foreground{
		Provider: agentruntime.ProviderCodex, PID: observed.PID, StartIdentity: "991826",
	}, "work", "thr_stale")
	if err != nil {
		t.Fatalf("encode stale runtime registration: %v", err)
	}
	agent = deriveAgent(profiles, observed, stale)
	if agent == nil || agent.PID != observed.PID || agent.Profile != "" || agent.ProviderSession != nil {
		t.Fatalf("stale registration crossed into current agent identity: %+v", agent)
	}

	if agent := deriveAgent(profiles, processinfo.Observation{
		PID: 4313, StartIdentity: "991828", Executable: "/bin/zsh",
	}, registration); agent != nil {
		t.Fatalf("unsupported foreground projected an agent: %+v", agent)
	}
	if agent := deriveAgent(profiles, processinfo.Observation{}, registration); agent != nil {
		t.Fatalf("failed process observation projected an agent: %+v", agent)
	}
}
