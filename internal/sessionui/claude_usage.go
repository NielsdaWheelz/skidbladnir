package sessionui

import "time"

type claudeQuota struct {
	fiveHour, sevenDay claudeQuotaWindow
}

type claudeQuotaWindow struct {
	receivedAt            time.Time // zero until a pane supplies this window
	machine, accountScope string
	remaining             int
	stale                 bool
}

// A remembered reading needs its original host's still-declared account binding.
// Another host may already have supplied a visible reading for this window.
func (w *claudeQuotaWindow) retain(previous claudeQuotaWindow, machine, scope string) {
	if w.receivedAt.IsZero() && scope != "" && previous.machine == machine && previous.accountScope == scope {
		*w = previous
		w.stale = true
	}
}

func (w *claudeQuotaWindow) observe(used *int, reading claudeQuotaWindow) {
	if used == nil {
		return
	}
	reading.remaining = 100 - *used
	var wins bool
	switch {
	case w.receivedAt.IsZero() || w.stale:
		wins = true
	case reading.remaining != w.remaining:
		wins = reading.remaining < w.remaining
	case (reading.accountScope == "") != (w.accountScope == ""):
		wins = reading.accountScope != ""
	default:
		wins = usageLater(reading.receivedAt, reading.machine, w.receivedAt, w.machine)
	}
	if wins {
		*w = reading
	}
}

// Reconcile after the entire browser update. Withdrawing several peers together
// must retain the last winner the browser selected before the withdrawal.
func (m *model) reconcileClaudeQuota() {
	next := make(map[string]claudeQuota, len(m.claudeUsage))
	for index, peer := range m.peers {
		if accessRejected(peer.Error) {
			continue
		}
		scopes := make(map[string]string)
		for _, profile := range peer.Profiles {
			if profile.Provider != "Claude" {
				continue
			}
			scopes[profile.Key] = profile.AccountScope
			previous, quota := m.claudeUsage[profile.Key], next[profile.Key]
			quota.fiveHour.retain(previous.fiveHour, peer.Machine, profile.AccountScope)
			quota.sevenDay.retain(previous.sevenDay, peer.Machine, profile.AccountScope)
			next[profile.Key] = quota
		}
		if peer.Error != nil || m.inventoryLanes[index].needsReadAfterVisit || m.now.Sub(peer.ReceivedAt) >= usageStaleAfter {
			continue
		}
		for _, session := range peer.Sessions {
			agent, usage := session.Agent, session.TerminalUsage
			if agent == nil || agent.Provider != "Claude" || session.Connection != nil || usage == nil {
				continue
			}
			scope, declared := scopes[agent.Profile]
			if !declared {
				continue
			}
			reading := claudeQuotaWindow{machine: peer.Machine, accountScope: scope, receivedAt: peer.ReceivedAt}
			quota := next[agent.Profile]
			quota.fiveHour.observe(usage.FiveHourUsedPercent, reading)
			quota.sevenDay.observe(usage.SevenDayUsedPercent, reading)
			next[agent.Profile] = quota
		}
	}
	m.claudeUsage = next
}
