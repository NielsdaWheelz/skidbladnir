package agentcontrol

import "github.com/NielsdaWheelz/skidbladnir/internal/sessions"

// detectCodex reads a codex 0.159.2 screen by the frozen grammar of
// research/codex.md revision 4. No rule is implemented yet: both dimensions
// stay unrecognized and the composer unknown, so every screen is layout_unknown.
func detectCodex(screen screen) reading {
	return reading{activity: sessions.ActivityUnknown, interaction: sessions.InteractionUnknown, notice: sessions.NoticeNone}
}
