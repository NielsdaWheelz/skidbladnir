package sessionui

import (
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// The fleet client resolves the active account and remembers its independent
// readings across hosts. The browser only projects them at the current time.
func (m *model) claudeUsageRow(profile fleetclient.Profile) usageRow {
	for _, known := range m.claudeUsage {
		if known.Key != profile.Key {
			continue
		}
		if known.Claude == nil {
			return usageRow{profile: profile, state: "unavailable"}
		}
		return usageRow{
			profile: profile,
			week:    claudeWindow(known.Claude.SevenDay, m.now),
			fable:   claudeWindow(known.Claude.FableSevenDay, m.now),
			five:    claudeWindow(known.Claude.FiveHour, m.now),
		}
	}
	state := "checking"
	for index, peer := range m.usagePeers {
		for _, value := range peer.profiles {
			if value.profile.Profile != profile {
				continue
			}
			switch {
			case peer.failure != nil || accessRejected(m.peers[index].Error) || value.profile.ReadState == "unavailable":
				state = "unavailable"
			case !value.receivedAt.IsZero():
				return usageRow{profile: profile}
			}
		}
	}
	return usageRow{profile: profile, state: state}
}

// A claude window fades only once its reset has passed, never by age.
func claudeWindow(window api.ClaudeWindow, now time.Time) *quotaWindow {
	if window.Usage == nil && window.Reset == nil {
		return nil
	}
	projected := &quotaWindow{left: -1}
	if window.Usage != nil {
		projected.left = percentLeft(window.Usage.UsedPercent)
	}
	if window.Reset != nil {
		until := window.Reset.ResetsAt.Sub(now)
		projected.until, projected.faded = &until, until <= 0
	}
	return projected
}
