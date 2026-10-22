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
	usage    *sessions.TerminalUsage
	footer   *sessions.TerminalFooter
	rules    []string
}

// composer says whether guarded send may paste into the ordinary composer. Its
// zero value is unknown, so a grammar that never establishes it refuses send;
// a known value names its rule suffix.
type composer string

const (
	composerUnknown composer = ""
	composerEmpty   composer = "empty"   // the ordinary composer shows only its placeholder
	composerDraft   composer = "draft"   // the ordinary composer holds input
	composerBlocked composer = "blocked" // the ordinary composer visibly refuses ordinary input
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
	notice           sessions.Notice // the latest turn's abnormal end; zero is none
	usage            *sessions.TerminalUsage
	footer           *sessions.TerminalFooter
	rules            []string
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
	}

	conflict, clipped, unknown := false, false, 0
	if read.activity == sessions.ActivityUnknown {
		unknown++
		conflict, clipped = read.activityCause == causeConflict, read.activityCause == causeClipped
	}
	if read.interaction == sessions.InteractionUnknown {
		unknown++
		conflict = conflict || read.interactionCause == causeConflict
		clipped = clipped || read.interactionCause == causeClipped
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
	notice := read.notice
	if notice == "" {
		notice = sessions.NoticeNone
	}
	status := sessions.TerminalStatus{Activity: read.activity, Interaction: read.interaction, Notice: notice, Source: sessions.SourceTerminal, Reason: reason}
	if !status.Valid() {
		panic("terminal detection produced an invalid status") // recognition yields only the two providers, whose grammars set closed dimension values
	}
	return detection{status: status, composer: read.composer, usage: read.usage, footer: read.footer, rules: read.rules}
}
