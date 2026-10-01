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

// composer says whether guarded send may paste into the ordinary composer. Its
// zero value is unknown, so a grammar that never establishes it refuses send.
type composer uint8

const (
	composerUnknown composer = iota
	composerEmpty            // the ordinary composer shows only its placeholder
	composerDraft            // the ordinary composer holds input
	composerBlocked          // the ordinary composer visibly refuses ordinary input
)

// reading is what one grammar read on a screen: each dimension's value, or the
// cause it stayed unknown; the composer; and the rules that decided them, in
// grammar order. A grammar returns a reading, never a status, so only detect
// chooses the reason and every detection is a valid successful terminal status.
type reading struct {
	activity         sessions.Activity
	activityCause    unknownCause // given only when activity is unknown
	interaction      sessions.Interaction
	interactionCause unknownCause // given only when interaction is unknown
	composer         composer
	rules            []DiagnosticRule
}

// unknownReading is a reading that classified neither dimension, for cause.
func unknownReading(cause unknownCause) reading {
	return reading{activity: sessions.ActivityUnknown, activityCause: cause, interaction: sessions.InteractionUnknown, interactionCause: cause}
}

// unknownCause says why a dimension stayed unknown.
type unknownCause uint8

const (
	causeUnrecognized unknownCause = iota // no rule classified it
	causeClipped                          // a row its rule needed was absent
	causeConflict                         // its decisive evidence disagreed
)

// detect classifies one observation of a recognized provider's screen. It is
// pure: it parses the observation once, lets that provider's grammar read it,
// and chooses the reason with spec section 2 precedence: among the unknown
// dimensions a conflict, then clipping; otherwise the classified count. A
// classifier defect panics; it never reads as unavailable.
func detect(provider agentruntime.Provider, observation tmuxclient.PaneObservation) detection {
	parsed := parseScreen(observation)
	var read reading
	switch provider {
	case agentruntime.ProviderCodex:
		read = detectCodex(parsed)
	case agentruntime.ProviderClaude:
		read = detectClaude(parsed)
	default:
		panic("terminal detection for an unknown provider") // justify-defect: recognition yields only the closed providers.
	}

	conflict, clipped, unknown := false, false, 0
	for _, dimension := range [...]struct {
		unknown bool
		cause   unknownCause
	}{
		{read.activity == sessions.ActivityUnknown, read.activityCause},
		{read.interaction == sessions.InteractionUnknown, read.interactionCause},
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
	// No grammar qualifies a current notice structure: codex draws none
	// (research/codex.md 2.9), and claude's interruption and error rows are
	// transcript (research/claude.md 2.8). Spec section 2 then claims none.
	status := sessions.TerminalStatus{Activity: read.activity, Interaction: read.interaction, Notice: sessions.NoticeNone, Source: sessions.SourceTerminal, Reason: reason}
	if !status.Valid() {
		panic("terminal detection produced an invalid status") // justify-defect: grammars set closed dimension values.
	}
	return detection{status: status, composer: read.composer, rules: read.rules[:min(len(read.rules), maxDiagnosticRules)]}
}
