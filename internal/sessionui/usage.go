package sessionui

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/charmbracelet/x/ansi"
)

type usageProfile struct {
	profile                fleetclient.ProfileUsage
	observedAt, receivedAt time.Time
}
type usagePeer struct {
	machine  fleetclient.Machine
	failure  *fleetclient.Failure
	profiles []usageProfile
}
type usageMsg struct {
	generation uint64
	result     fleetclient.Result
}

func (m *model) usageVisible() bool {
	return !m.usageStopped && !m.usageAttached && m.ctx.Err() == nil && (m.page == "" || m.page == "usage")
}
func (m *model) cancelUsage() {
	m.usageGeneration++
	if m.usageCancel != nil {
		m.usageCancel()
		m.usageCancel = nil
	}
	m.usagePending = false
}
func (m *model) refreshUsage(now time.Time, manual bool) tea.Cmd {
	if !m.usageVisible() || m.usagePending || !manual && now.Before(m.usageNext) {
		return nil
	}
	m.usageGeneration++
	ctx, cancel := context.WithCancel(m.ctx)
	m.usageCancel, m.usagePending = cancel, true
	// justify-polling: the existing browser tick reads the full native quota
	// scope every minute from attempt start; forms/attachment/quit cancel it.
	m.usageNext = now.Add(time.Minute)
	client, generation, workers := m.client, m.usageGeneration, &m.usageWorkers
	completed := make(chan usageMsg, 1)
	workers.Add(1)
	// Bubble Tea does not wait for commands. Register the worker before delivery
	// so Run can cancel and join it even if its command never runs.
	go func() {
		defer workers.Done()
		defer cancel()
		result := client.ProfileUsage(ctx, "")
		completed <- usageMsg{generation: generation, result: result}
	}()
	return func() tea.Msg { return <-completed }
}
func (m *model) acceptUsage(message usageMsg) {
	if !m.usageVisible() || message.generation != m.usageGeneration {
		return
	}
	m.usagePending, m.usageCancel = false, nil
	if !message.result.OK {
		panic("whole-fleet usage read failed before peer collection") // justify-defect: the immutable client table supplies a valid unfiltered scope.
	}
	for _, received := range message.result.Value.(fleetclient.ProfileUsageObservation).Peers {
		for index := range m.usagePeers {
			peer := &m.usagePeers[index]
			if peer.machine.Handle != received.Machine {
				continue
			}
			peer.failure = received.Error
			m.invalidateAccess(peer.machine.Label, received.Error)
			if !received.OK {
				break
			}
			profiles := make([]usageProfile, len(received.Profiles))
			for profileIndex, profile := range received.Profiles {
				value := usageProfile{profile: profile, observedAt: received.ObservedAt, receivedAt: received.ReceivedAt}
				if profile.ReadState == "unavailable" {
					for _, previous := range peer.profiles {
						if previous.profile.Profile == profile.Profile && previous.profile.Report != nil {
							value.profile.Report = previous.profile.Report
							value.observedAt, value.receivedAt = previous.observedAt, previous.receivedAt
							break
						}
					}
				}
				profiles[profileIndex] = value
			}
			peer.profiles = profiles
			break
		}
	}
}

// Full inventories own current descriptors. A change clears only that host's
// changed Codex reports and retires any whole-fleet read of the preceding table.
func (m *model) reconcileUsageProfiles(received fleetclient.Peer) bool {
	for index := range m.usagePeers {
		peer := &m.usagePeers[index]
		if peer.machine.Handle != received.Machine {
			continue
		}
		declared := []fleetclient.Profile{}
		for _, profile := range received.Profiles {
			if profile.Provider == "Codex" {
				declared = append(declared, profile)
			}
		}
		known := peer.profiles != nil
		same := known && len(peer.profiles) == len(declared)
		if same {
			for index, profile := range declared {
				if peer.profiles[index].profile.Profile != profile {
					same = false
					break
				}
			}
		}
		if same {
			return false
		}
		profiles := make([]usageProfile, len(declared))
		for index, profile := range declared {
			profiles[index] = usageProfile{profile: fleetclient.ProfileUsage{Profile: profile, ReadState: "ok"}}
			for _, previous := range peer.profiles {
				if previous.profile.Profile == profile {
					profiles[index] = previous
					break
				}
			}
		}
		peer.profiles = profiles
		if known || m.usagePending {
			m.cancelUsage()
			m.usageNext = time.Time{}
			return true
		}
		return false
	}
	return false
}

const usageStaleAfter = time.Hour

type usageText struct {
	fiveHour, sevenDay *windowText
	stale              bool
}
type windowText struct {
	remaining, resetsAt, resetsIn string
	expired                       bool
}

func usagePresentation(profile usageProfile, hostFailed bool, now time.Time) usageText {
	text := usageText{}
	report := profile.profile.Report
	if report == nil {
		return text
	}
	elapsed := max(time.Duration(0), now.Sub(profile.receivedAt))
	age := profile.observedAt.Sub(report.ReportedAt) + elapsed
	text.stale = hostFailed || profile.profile.ReadState == "unavailable" || age >= usageStaleAfter
	text.fiveHour = usageWindowText(report.FiveHour, profile.observedAt, elapsed)
	text.sevenDay = usageWindowText(report.SevenDay, profile.observedAt, elapsed)
	return text
}
func usageWindowAvailable(window *agentcontrol.UsageWindow, observedAt time.Time, elapsed time.Duration) bool {
	return window != nil && (window.ResetsAt == nil || window.ResetsAt.Sub(observedAt) > elapsed)
}
func usageWindowText(window *agentcontrol.UsageWindow, observedAt time.Time, elapsed time.Duration) *windowText {
	if window == nil {
		return nil
	}
	text := &windowText{remaining: "—", resetsAt: "—", resetsIn: "—"}
	if !usageWindowAvailable(window, observedAt, elapsed) {
		text.expired = true
		return text
	}
	text.remaining = strconv.FormatFloat(math.Floor(max(0, 100-window.UsedPercent)), 'f', 0, 64) + "%"
	if window.ResetsAt != nil {
		remaining := window.ResetsAt.Sub(observedAt) - elapsed
		seconds := int64(remaining / time.Second)
		if remaining%time.Second != 0 {
			seconds++
		}
		text.resetsAt = window.ResetsAt.UTC().Format("2006-01-02 15:04:05")
		text.resetsIn = fmt.Sprintf("%02d:%02d:%02d:%02d", seconds/(24*60*60), seconds/(60*60)%24, seconds/60%60, seconds%60)
	}
	return text
}

// A row is a presentation projection, never another retained observation.
type usageRow struct {
	profile    fleetclient.Profile
	machine    fleetclient.Machine
	native     *usageProfile
	terminal   *fleetclient.Session
	receivedAt time.Time
	text       usageText
	state      string
	failed     bool
}

func (row usageRow) windowPresentation(window *windowText) (windowText, string) {
	text := windowText{remaining: "—", resetsAt: "—", resetsIn: "—"}
	if window != nil {
		text = *window
	}
	status := row.state
	if status == "" && window != nil {
		if window.expired {
			status = "awaiting update"
		} else if row.text.stale {
			status = "stale"
		}
	}
	return text, status
}

func usageLater(receivedAt time.Time, machine string, previous time.Time, previousMachine string) bool {
	return receivedAt.After(previous) || receivedAt.Equal(previous) && machine < previousMachine
}
func (m *model) usageAccessRejected(machine string) bool {
	for _, peer := range m.peers {
		if peer.Machine == machine && peer.Error != nil {
			return peer.Error.Code == "Unauthenticated" || peer.Error.Code == "MachineIdentityMismatch"
		}
	}
	return false
}
func (m *model) nativeUsage(profile fleetclient.Profile, now time.Time) usageRow {
	row := usageRow{profile: profile, state: "checking"}
	var expired *usageProfile
	var expiredMachine fleetclient.Machine
	expiredFailed, unavailable, absent := false, false, false
	for _, peer := range m.usagePeers {
		relevant := peer.profiles == nil
		for _, value := range peer.profiles {
			relevant = relevant || value.profile.Key == profile.Key && value.profile.Provider == profile.Provider
		}
		if !relevant {
			continue
		}
		if m.usageAccessRejected(peer.machine.Handle) {
			unavailable = true
			continue
		}
		if peer.failure != nil {
			unavailable = true
		}
		for index := range peer.profiles {
			value := &peer.profiles[index]
			if value.profile.Key != profile.Key || value.profile.Provider != profile.Provider {
				continue
			}
			if value.profile.ReadState == "unavailable" {
				unavailable = true
			}
			report := value.profile.Report
			if report == nil {
				absent = absent || !value.receivedAt.IsZero()
				continue
			}
			elapsed := max(time.Duration(0), now.Sub(value.receivedAt))
			if !usageWindowAvailable(report.FiveHour, value.observedAt, elapsed) && !usageWindowAvailable(report.SevenDay, value.observedAt, elapsed) {
				absent = true
				if (report.FiveHour != nil || report.SevenDay != nil) && (expired == nil || usageLater(value.receivedAt, peer.machine.Handle, expired.receivedAt, expiredMachine.Handle)) {
					expired, expiredMachine, expiredFailed = value, peer.machine, peer.failure != nil
				}
				continue
			}
			if row.native == nil || usageLater(value.receivedAt, peer.machine.Handle, row.native.receivedAt, row.machine.Handle) {
				row.native, row.machine, row.failed = value, peer.machine, peer.failure != nil
			}
		}
	}
	if row.native != nil {
		row.text = usagePresentation(*row.native, row.failed, now)
		row.state = ""
		return row
	}
	if expired != nil {
		row.native, row.machine, row.failed = expired, expiredMachine, expiredFailed
		row.text = usagePresentation(*expired, expiredFailed, now)
		row.state = "" // The known windows render their specific reset diagnosis.
	}
	if unavailable {
		row.state = "unavailable"
	} else if expired == nil && absent {
		row.state = ""
	}
	return row
}
func usageTerminalLess(left, right fleetclient.Session) bool {
	l, _ := fleetclient.DecodeReference(left.Ref)
	r, _ := fleetclient.DecodeReference(right.Ref)
	for _, pair := range [][2]string{{l.TmuxID, r.TmuxID}, {left.ActivePaneID, right.ActivePaneID}} {
		a, b := strings.TrimLeft(pair[0][1:], "0"), strings.TrimLeft(pair[1][1:], "0")
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		if a != b {
			return a < b
		}
	}
	return false
}
func (m *model) claudeUsage(profile fleetclient.Profile, now time.Time) usageRow {
	row := usageRow{profile: profile, state: "checking"}
	unavailable, absent := false, false
	for _, peer := range m.peers {
		declared := false
		for _, local := range peer.Profiles {
			declared = declared || local.Key == profile.Key && local.Provider == profile.Provider
		}
		if !declared {
			unavailable = unavailable || peer.Profiles == nil && peer.Error != nil
			continue
		}
		if !peer.OK {
			unavailable = unavailable || peer.Error != nil
			continue
		}
		absent = true
		var candidate *fleetclient.Session
		count := 0
		for index := range peer.Sessions {
			session := &peer.Sessions[index]
			if session.Agent == nil {
				unavailable = unavailable || session.TerminalStatus.Source == sessions.SourceUnavailable
				continue
			}
			if session.Agent.Provider != "Claude" || session.Agent.Profile != profile.Key || session.Connection != nil {
				continue
			}
			unavailable = unavailable || session.TerminalStatus.Source == sessions.SourceUnavailable
			usage := session.TerminalUsage
			if usage == nil {
				continue
			}
			known := 0
			if usage.FiveHourUsedPercent != nil {
				known++
			}
			if usage.SevenDayUsedPercent != nil {
				known++
			}
			if candidate == nil || known > count || known == count && usageTerminalLess(*session, *candidate) {
				candidate, count = session, known
			}
		}
		if candidate != nil && (row.terminal == nil || usageLater(peer.ReceivedAt, peer.Machine, row.receivedAt, row.machine.Handle)) {
			row.terminal, row.machine, row.receivedAt = candidate, fleetclient.Machine{Label: peer.Label, Handle: peer.Machine}, peer.ReceivedAt
		}
	}
	if row.terminal != nil {
		row.state = ""
		row.text.stale = max(time.Duration(0), now.Sub(row.receivedAt)) >= usageStaleAfter
		if value := row.terminal.TerminalUsage.FiveHourUsedPercent; value != nil {
			row.text.fiveHour = &windowText{remaining: strconv.Itoa(100-*value) + "%", resetsAt: "—", resetsIn: "—"}
		}
		if value := row.terminal.TerminalUsage.SevenDayUsedPercent; value != nil {
			row.text.sevenDay = &windowText{remaining: strconv.Itoa(100-*value) + "%", resetsAt: "—", resetsIn: "—"}
		}
	} else if unavailable {
		row.state = "unavailable"
	} else if absent {
		row.state = ""
	}
	return row
}
func (m *model) usageRows(now time.Time) []usageRow {
	profiles := []fleetclient.Profile{}
	add := func(profile fleetclient.Profile) {
		for _, present := range profiles {
			if present.Key == profile.Key && present.Provider == profile.Provider {
				return
			}
		}
		profiles = append(profiles, profile)
	}
	for _, peer := range m.peers {
		for _, profile := range peer.Profiles {
			add(profile)
		}
	}
	for _, peer := range m.usagePeers {
		for _, profile := range peer.profiles {
			add(profile.profile.Profile)
		}
	}
	rows := make([]usageRow, len(profiles))
	for index, profile := range profiles {
		switch profile.Provider {
		case "Codex":
			rows[index] = m.nativeUsage(profile, now)
		case "Claude":
			rows[index] = m.claudeUsage(profile, now)
		default:
			panic("usage declaration has an unknown provider") // justify-defect: both owned transport decoders close the provider union.
		}
	}
	return rows
}
func (m *model) usageDeclarationState() string {
	full, completed, failed := true, false, false
	for _, peer := range m.peers {
		full = full && peer.Profiles != nil
		completed = completed || peer.Profiles != nil || peer.Error != nil
		failed = failed || peer.Error != nil
	}
	if full {
		return "no agent profiles"
	}
	for _, peer := range m.usagePeers {
		completed = completed || peer.profiles != nil || peer.failure != nil
	}
	if !completed || m.inventoryPending() && !failed {
		return "checking usage"
	}
	return "profile declarations unavailable"
}
func (m *model) usageSummary() []string {
	if !m.usageVisible() || m.page != "" {
		return nil
	}
	rows := m.usageRows(time.Now())
	if len(rows) == 0 {
		return []string{m.usageDeclarationState()}
	}
	lines := []string{}
	for _, row := range rows {
		text, status := row.windowPresentation(row.text.sevenDay)
		item := attention.SingleLine(row.profile.Key) + " 7d " + text.remaining
		if status != "" {
			item += " " + status
		}
		if last := len(lines) - 1; last >= 0 && ansi.StringWidth(lines[last])+3+ansi.StringWidth(item) <= m.width-2 {
			lines[last] += " · " + item
		} else {
			lines = append(lines, item)
		}
	}
	return lines
}
func (m *model) usageLines() []string {
	rows := m.usageRows(time.Now())
	if len(rows) == 0 {
		return []string{m.usageDeclarationState()}
	}
	lines := []string{}
	for _, row := range rows {
		for index, window := range []*windowText{row.text.sevenDay, row.text.fiveHour} {
			profile, label := row.profile.Key, "7d"
			if index == 1 {
				if window == nil {
					continue
				}
				profile, label = "", "5h"
			}
			text, status := row.windowPresentation(window)
			lines = append(lines, cell(profile, 11)+"  "+fmt.Sprintf("%-2s %5s %-15s", label, text.remaining, status)+"  "+cell(text.resetsAt, 19)+"  "+text.resetsIn)
		}
	}
	return lines
}
func (m *model) usageBody(height int) []string {
	headings := []string{bold.Styled("usage"), "", cell("profile", 11) + "  " + cell("window", 24) + "  " + cell("reset (utc)", 19) + "  in (d:hh:mm:ss)"}
	if height < len(headings) {
		return headings[:max(0, height)]
	}
	lines := m.usageLines()
	capacity := m.pageCapacity()
	offset := min(m.offset, max(0, len(lines)-capacity))
	return append(headings, lines[offset:min(len(lines), offset+capacity)]...)
}
func (m *model) usageKey(key string) {
	switch key {
	case "q", "esc":
		m.page = ""
	case "up":
		m.offset = max(0, m.offset-1)
	case "down":
		m.offset = min(max(0, len(m.usageLines())-m.pageCapacity()), m.offset+1)
	}
}
