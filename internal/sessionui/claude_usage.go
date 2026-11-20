package sessionui

import (
	"fmt"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// The fleet client resolves the active account and remembers its independent
// readings across hosts. The browser only projects them at the current time.
func (m *model) claudeUsageRow(profile fleetclient.Profile, now time.Time) usageRow {
	row := usageRow{profile: profile, state: "checking"}
	for _, known := range m.claudeUsage {
		if known.Key != profile.Key {
			continue
		}
		row.state = ""
		if known.ReadState == "unavailable" {
			row.state = "unavailable"
		} else if record := known.Claude; record != nil {
			row.text = usageText{
				fiveHour:      claudeWindowText(record.FiveHour, 5*time.Hour, now),
				sevenDay:      claudeWindowText(record.SevenDay, 7*24*time.Hour, now),
				fableSevenDay: claudeWindowText(record.FableSevenDay, 7*24*time.Hour, now),
			}
		}
		return row
	}
	for index, peer := range m.usagePeers {
		for _, value := range peer.profiles {
			if value.profile.Profile != profile {
				continue
			}
			switch {
			case peer.failure != nil || accessRejected(m.peers[index].Error) || value.profile.ReadState == "unavailable":
				row.state = "unavailable"
			case !value.receivedAt.IsZero():
				row.state = ""
				return row
			}
		}
	}
	return row
}

func claudeWindowText(window api.ClaudeWindow, span time.Duration, now time.Time) *windowText {
	if window.Usage == nil && window.Reset == nil {
		return nil
	}
	left := -1
	if window.Usage != nil {
		left = 100 - int(window.Usage.UsedPercent)
	}
	if window.Reset == nil {
		return usageWindowText(left, span, 0)
	}
	text := usageWindowText(left, span, window.Reset.ResetsAt.Sub(now))
	if ago := now.Sub(window.Reset.ResetsAt); ago >= 0 {
		// Keep elapsed dates legible in the seven-cell reset column.
		switch {
		case ago < time.Minute:
			text.resetTime = "<1m ago"
		case ago < time.Hour:
			text.resetTime = fmt.Sprintf("%dm ago", ago/time.Minute)
		case ago < 24*time.Hour:
			text.resetTime = fmt.Sprintf("%dh ago", ago/time.Hour)
		case ago < 7*24*time.Hour:
			text.resetTime = fmt.Sprintf("%dd ago", ago/(24*time.Hour))
		case ago < 365*24*time.Hour:
			text.resetTime = fmt.Sprintf("%dw ago", ago/(7*24*time.Hour))
		default:
			text.resetTime = window.Reset.ResetsAt.Format("2006-01")
		}
	}
	return text
}
