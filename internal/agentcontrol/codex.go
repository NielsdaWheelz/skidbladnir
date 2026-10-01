package agentcontrol

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// detectCodex classifies a codex 0.159.2 screen by the frozen grammar of
// research/codex.md revision 4. No rule is implemented yet: every screen is
// layout_unknown with an unknown composer.
func detectCodex(screen screen) detection {
	return detection{
		status:   outcome{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}.status(),
		composer: composerUnknown,
	}
}
