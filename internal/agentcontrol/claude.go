package agentcontrol

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// detectClaude reads a claude 2.1.286 screen by the frozen grammar of
// research/claude.md. No rule is implemented yet: both dimensions stay
// unrecognized and the composer unknown, so every screen is layout_unknown.
func detectClaude(screen screen) reading {
	return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}
}
