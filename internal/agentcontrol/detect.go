package agentcontrol

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// detection is one classified screen: its successful status, the ordinary
// composer's state for guarded send, and the content-free rules that decided
// them, in grammar order.
type detection struct {
	status   sessions.TerminalStatus
	composer composer
	rules    []DiagnosticRule
}

// composer says whether guarded send may paste into the ordinary composer.
type composer string

const (
	composerEmpty   composer = "empty"   // the ordinary composer shows only its placeholder
	composerDraft   composer = "draft"   // the ordinary composer holds input
	composerBlocked composer = "blocked" // the ordinary composer visibly refuses ordinary input
	composerUnknown composer = "unknown"
)

// detect classifies one observation of a recognized provider's screen. It is
// pure: it parses the observation once and dispatches to that provider's
// grammar. A classifier defect panics; it never reads as unavailable.
func detect(provider agentruntime.Provider, observation tmuxclient.PaneObservation) detection {
	parsed := parseScreen(observation)
	var detected detection
	switch provider {
	case agentruntime.ProviderCodex:
		detected = detectCodex(parsed)
	case agentruntime.ProviderClaude:
		detected = detectClaude(parsed)
	default:
		panic("terminal detection for an unknown provider") // justify-defect: recognition yields only the closed providers.
	}
	switch detected.composer {
	case composerEmpty, composerDraft, composerBlocked, composerUnknown:
	default:
		panic("terminal detection without a composer state") // justify-defect: grammars return the closed composer states.
	}
	if len(detected.rules) > maxDiagnosticRules {
		detected.rules = detected.rules[:maxDiagnosticRules]
	}
	return detected
}

// unknownCause says why a dimension stayed unknown.
type unknownCause uint8

const (
	causeUnrecognized unknownCause = iota // no rule classified it
	causeClipped                          // a row its rule needed was absent
	causeConflict                         // its decisive evidence disagreed
)

// outcome is one successful sample's dimensions before a reason is chosen.
// A cause says why its dimension is unknown and stays causeUnrecognized when
// the dimension is classified. The notice is independent.
type outcome struct {
	activity         sessions.Activity
	activityCause    unknownCause
	interaction      sessions.Interaction
	interactionCause unknownCause
	notice           sessions.Notice
}

// status chooses the reason with spec section 2 precedence: among the unknown
// dimensions a conflict, then clipping; otherwise the classified count.
func (outcome outcome) status() sessions.TerminalStatus {
	conflict, clipped, unknown := false, false, 0
	for _, dimension := range [...]struct {
		unknown bool
		cause   unknownCause
	}{
		{outcome.activity == sessions.ActivityUnknown, outcome.activityCause},
		{outcome.interaction == sessions.InteractionUnknown, outcome.interactionCause},
	} {
		if !dimension.unknown {
			if dimension.cause != causeUnrecognized {
				panic("classified terminal dimension with an unknown cause") // justify-defect: grammars give a cause only to an unknown dimension.
			}
			continue
		}
		unknown++
		switch dimension.cause {
		case causeUnrecognized:
		case causeClipped:
			clipped = true
		case causeConflict:
			conflict = true
		default:
			panic("unknown terminal dimension cause") // justify-defect: the causes are closed.
		}
	}
	reason := sessions.ReasonRecognized
	switch {
	case conflict:
		reason = sessions.ReasonEvidenceConflict
	case clipped:
		reason = sessions.ReasonEvidenceClipped
	case unknown == 1:
		reason = sessions.ReasonPartial
	case unknown == 2:
		reason = sessions.ReasonLayoutUnknown
	}
	status := sessions.TerminalStatus{Activity: outcome.activity, Interaction: outcome.interaction, Notice: outcome.notice, Source: sessions.SourceTerminal, Reason: reason}
	if !status.Valid() {
		panic("terminal detection produced an invalid status") // justify-defect: grammars set closed dimension values.
	}
	return status
}
