package sessionui

import (
	"math"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

type claudeQuota struct {
	fiveHour, sevenDay claudeQuotaWindow
}

type claudeBinding struct {
	machine, scope string
}

func (b claudeBinding) declared(scopes map[string]string) bool {
	return b.scope != "" && scopes[b.machine] == b.scope
}

// deadline is measured on the client's sleep-inclusive clock. instant is the
// provider's calendar time; it lets several hosts agree without comparing host clocks.
type claudeDate struct {
	instant  *time.Time
	deadline time.Duration
}

type claudeReset struct {
	claudeDate
	source claudeBinding
}

type claudeReading struct {
	source    claudeBinding
	received  time.Duration
	sessionID string
	remaining int
	date      claudeDate
}

func (r claudeReading) later(other claudeReading) bool {
	if r.received != other.received {
		return r.received > other.received
	}
	if r.source.machine != other.source.machine {
		return r.source.machine < other.source.machine
	}
	return fleetclient.CompareSessionIDs(r.sessionID, other.sessionID) < 0
}

func (r claudeReading) prefer(other claudeReading) bool {
	switch {
	case r.remaining != other.remaining:
		return r.remaining < other.remaining
	case (r.source.scope == "") != (other.source.scope == ""):
		return r.source.scope != ""
	case (r.date.instant == nil) != (other.date.instant == nil):
		return r.date.instant != nil
	default:
		return r.later(other)
	}
}

type claudeQuotaWindow struct {
	amount         claudeReading
	reset          claudeReset
	expiredInstant time.Time
	cutoff         time.Duration
	visible        bool
	showOwnDate    bool
	expiredVisible bool
}

// Among equal dates, keep a real reporter for binding the result.
type claudeDates struct {
	reporter *claudeReading
	conflict bool
}

func (d *claudeDates) include(r *claudeReading) {
	if d.reporter != nil && !r.date.instant.Equal(*d.reporter.date.instant) {
		d.conflict = true
	}
	if d.reporter == nil || r.later(*d.reporter) {
		d.reporter = r
	}
}

// All panes for one window are reduced together. A date can have a different
// owner and receipt from the selected amount.
func (w claudeQuotaWindow) reconcile(scopes map[string]string, readings []claudeReading, now time.Duration) claudeQuotaWindow {
	// Expire before applying binding withdrawals, including on the first update after sleep.
	if w.reset.instant != nil && now >= w.reset.deadline {
		w.expiredInstant, w.cutoff = *w.reset.instant, w.reset.deadline
		w.reset = claudeReset{}
	}
	if !w.reset.source.declared(scopes) {
		w.reset = claudeReset{}
	}
	if !w.amount.source.declared(scopes) || w.amount.received < w.cutoff || w.expired(w.amount.date, now) {
		w.amount = claudeReading{}
	}
	w.visible, w.expiredVisible = false, false

	var scoped, unscoped claudeDates
	for i := range readings {
		r := &readings[i]
		if w.expired(r.date, now) {
			w.expiredVisible = true
			continue
		}
		if r.date.instant != nil {
			if r.source.scope != "" {
				scoped.include(r)
			} else {
				unscoped.include(r)
			}
		}
		if r.received < w.cutoff {
			w.expiredVisible = true
		} else if !w.visible || r.prefer(w.amount) {
			w.amount, w.visible = *r, true
		}
	}
	if r := scoped.reporter; r != nil && !scoped.conflict && (w.reset.instant == nil || !r.date.instant.Equal(*w.reset.instant)) {
		w.reset = claudeReset{claudeDate: r.date, source: r.source}
	}
	w.showOwnDate = w.reset.instant == nil && !scoped.conflict && !unscoped.conflict && w.visible && w.amount.source.scope == ""
	return w
}

func (w claudeQuotaWindow) expired(date claudeDate, now time.Duration) bool {
	return date.instant != nil && (date.deadline <= now || w.cutoff != 0 && !date.instant.After(w.expiredInstant))
}

type claudeSamples struct {
	scopes             map[string]string // latest declaration on each authorized host
	fiveHour, sevenDay []claudeReading
}

func claudeObservation(window *api.QuotaWindow, peer fleetclient.Peer, scope, sessionID string) claudeReading {
	source := claudeBinding{machine: peer.Machine, scope: scope}
	r := claudeReading{source: source, received: peer.ReceivedBoot, sessionID: sessionID, remaining: 100 - int(window.UsedPercent)}
	if at := window.ResetsAt; at != nil {
		// A printable distant date can exceed the duration that fits after receipt.
		delta := min(at.Sub(peer.ObservedAt), time.Duration(math.MaxInt64)-peer.ReceivedBoot)
		r.date = claudeDate{instant: at, deadline: peer.ReceivedBoot + delta}
	}
	return r
}

// Reconcile only after a whole update so simultaneous withdrawals see the same previous winners.
func (m *model) reconcileClaudeQuota() {
	profiles := make(map[string]*claudeSamples)
	for index, peer := range m.peers {
		if accessRejected(peer.Error) {
			continue
		}
		for _, profile := range peer.Profiles {
			if profile.Provider == "Claude" {
				if profiles[profile.Key] == nil {
					profiles[profile.Key] = &claudeSamples{scopes: make(map[string]string)}
				}
				profiles[profile.Key].scopes[peer.Machine] = profile.AccountScope
			}
		}
		if peer.Error != nil || m.inventoryLanes[index].needsReadAfterVisit || peer.ReceivedBoot == 0 || m.bootNow == 0 || m.bootNow-peer.ReceivedBoot >= usageStaleAfter {
			continue
		}
		for _, session := range peer.Sessions {
			agent, usage := session.Agent, session.TerminalUsage
			if agent == nil || agent.Provider != "Claude" || session.Connection != nil || usage == nil {
				continue
			}
			samples := profiles[agent.Profile]
			if samples == nil {
				continue
			}
			scope, declared := samples.scopes[peer.Machine]
			if !declared {
				continue
			}
			if usage.FiveHour != nil {
				samples.fiveHour = append(samples.fiveHour, claudeObservation(usage.FiveHour, peer, scope, session.Ref.TmuxID))
			}
			if usage.SevenDay != nil {
				samples.sevenDay = append(samples.sevenDay, claudeObservation(usage.SevenDay, peer, scope, session.Ref.TmuxID))
			}
		}
	}
	next := make(map[string]claudeQuota, len(profiles))
	for key, samples := range profiles {
		quota := m.claudeUsage[key]
		quota.fiveHour = quota.fiveHour.reconcile(samples.scopes, samples.fiveHour, m.bootNow)
		quota.sevenDay = quota.sevenDay.reconcile(samples.scopes, samples.sevenDay, m.bootNow)
		next[key] = quota
	}
	m.claudeUsage = next
}
