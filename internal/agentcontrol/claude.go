package agentcontrol

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// detectClaude classifies a claude 2.1.286 screen by the frozen grammar of
// research/claude.md. No rule is implemented yet: every screen is
// layout_unknown with an unknown composer.
func detectClaude(screen screen) detection {
	return detection{
		status:   outcome{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}.status(),
		composer: composerUnknown,
	}
}
