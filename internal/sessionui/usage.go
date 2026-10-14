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

type usageText struct {
	fiveHour, sevenDay, fiveReset, sevenReset string
	age                                       string
	stale                                     bool
}

func usagePresentation(profile usageProfile, hostFailed bool, now time.Time) usageText {
	text := usageText{fiveHour: "—", sevenDay: "—", fiveReset: "reset unknown", sevenReset: "reset unknown"}
	report := profile.profile.Report
	if report == nil {
		return text
	}
	elapsed := max(time.Duration(0), now.Sub(profile.receivedAt))
	age := profile.observedAt.Sub(report.ReportedAt) + elapsed
	text.age = "report age " + age.Truncate(time.Second).String()
	text.stale = hostFailed || profile.profile.ReadState == "unavailable" || age >= 2*time.Minute
	text.fiveHour, text.fiveReset = usageWindowText(report.FiveHour, profile.observedAt, elapsed)
	text.sevenDay, text.sevenReset = usageWindowText(report.SevenDay, profile.observedAt, elapsed)
	return text
}
func usageWindowAvailable(window *agentcontrol.UsageWindow, observedAt time.Time, elapsed time.Duration) bool {
	return window != nil && (window.ResetsAt == nil || window.ResetsAt.Sub(observedAt) > elapsed)
}
func usageWindowText(window *agentcontrol.UsageWindow, observedAt time.Time, elapsed time.Duration) (string, string) {
	if window == nil {
		return "—", "reset unknown"
	}
	if !usageWindowAvailable(window, observedAt, elapsed) {
		return "—", "awaiting report after reset"
	}
	reset := "reset unknown"
	if window.ResetsAt != nil {
		remaining := window.ResetsAt.Sub(observedAt) - elapsed
		reset = "resets " + window.ResetsAt.UTC().Format("2006-01-02 15:04:05") + " utc · in " + remaining.Round(time.Second).String()
	}
	return strconv.FormatFloat(math.Floor(max(0, 100-window.UsedPercent)), 'f', 0, 64) + "%", reset
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
	diagnostic bool
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
	row := usageRow{profile: profile, text: usageText{fiveHour: "—", sevenDay: "—"}, state: "checking usage"}
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
		row.native, row.machine, row.failed, row.diagnostic = expired, expiredMachine, expiredFailed, true
		row.text = usagePresentation(*expired, expiredFailed, now)
		row.state = "" // The known windows render their specific reset diagnosis.
	}
	if unavailable {
		row.state = "native quota unavailable"
	} else if expired == nil && absent {
		row.state = "no reported quota"
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
func (m *model) claudeUsage(profile fleetclient.Profile) usageRow {
	row := usageRow{profile: profile, text: usageText{fiveHour: "—", sevenDay: "—"}, state: "checking usage"}
	conflict, unavailable, absent := false, false, false
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
		var five, seven *int
		hostConflict, count := false, 0
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
				hostConflict = hostConflict || five != nil && *five != *usage.FiveHourUsedPercent
				five = usage.FiveHourUsedPercent
			}
			if usage.SevenDayUsedPercent != nil {
				known++
				hostConflict = hostConflict || seven != nil && *seven != *usage.SevenDayUsedPercent
				seven = usage.SevenDayUsedPercent
			}
			if candidate == nil || known > count || known == count && usageTerminalLess(*session, *candidate) {
				candidate, count = session, known
			}
		}
		if hostConflict {
			conflict = true
			continue
		}
		if candidate != nil && (row.terminal == nil || usageLater(peer.ReceivedAt, peer.Machine, row.receivedAt, row.machine.Handle)) {
			row.terminal, row.machine, row.receivedAt = candidate, fleetclient.Machine{Label: peer.Label, Handle: peer.Machine}, peer.ReceivedAt
		}
	}
	if row.terminal != nil {
		if value := row.terminal.TerminalUsage.FiveHourUsedPercent; value != nil {
			row.text.fiveHour = strconv.Itoa(*value) + "%"
		}
		if value := row.terminal.TerminalUsage.SevenDayUsedPercent; value != nil {
			row.text.sevenDay = strconv.Itoa(*value) + "%"
		}
	} else if conflict {
		row.state = "conflicting terminal displays"
	} else if unavailable {
		row.state = "terminal observation unavailable"
	} else if absent {
		row.state = "usage not detected for this profile"
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
			rows[index] = m.claudeUsage(profile)
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
	if !completed || m.refreshing && !failed {
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
	codex, claude := []string{}, []string{}
	for _, row := range rows {
		if row.profile.Provider == "Codex" {
			codex = append(codex, row.profile.Key+" "+row.text.sevenDay)
		} else {
			claude = append(claude, "used · "+row.profile.Key+": 5h "+row.text.fiveHour+" · 7d "+row.text.sevenDay)
		}
	}
	lines := []string{}
	if len(codex) > 0 {
		lines = append(lines, "remaining · codex 7d: "+strings.Join(codex, " · "))
	}
	lines = append(lines, claude...)
	for index, line := range lines {
		lines[index] = ansi.Truncate(line, m.width-2, "…")
	}
	return lines
}
func (m *model) usageLines() []string {
	now, width := time.Now(), m.width-2
	rows := m.usageRows(now)
	if len(rows) == 0 {
		return []string{m.usageDeclarationState()}
	}
	lines := []string{}
	for _, row := range rows {
		if row.profile.Provider == "Claude" {
			lines = append(lines, wrapped(fmt.Sprintf("%s  5h used %s  7d used %s", row.profile.Key, row.text.fiveHour, row.text.sevenDay), width)...)
			if row.terminal == nil {
				lines = append(lines, row.state)
			} else {
				age := max(time.Duration(0), now.Sub(row.receivedAt)).Truncate(time.Second)
				lines = append(lines, wrapped("source "+attention.SingleLine(row.machine.Label)+" · terminal display · received "+age.String()+" ago", width)...)
				lines = append(lines, wrapped("terminal "+attention.SingleLine(row.terminal.Name)+" · pane "+row.terminal.ActivePaneID, width)...)
				lines = append(lines, "provider freshness and resets unknown")
			}
		} else {
			heading := fmt.Sprintf("%s  7d remaining %s", row.profile.Key, row.text.sevenDay)
			if row.native != nil && row.native.profile.Report.FiveHour != nil {
				heading = fmt.Sprintf("%s  5h remaining %s  7d remaining %s", row.profile.Key, row.text.fiveHour, row.text.sevenDay)
			}
			if row.text.stale {
				heading += " · stale"
			}
			lines = append(lines, wrapped(heading, width)...)
			if row.native == nil {
				lines = append(lines, row.state)
			} else {
				if row.diagnostic && row.state != "" {
					lines = append(lines, row.state)
				}
				prefix := "source "
				if row.diagnostic {
					prefix = "last report from "
				}
				source := prefix + attention.SingleLine(row.machine.Label) + " · native · " + row.text.age
				if row.failed || row.native.profile.ReadState == "unavailable" {
					source += " · unavailable; showing last report"
				}
				lines = append(lines, wrapped(source, width)...)
				resets := []string{}
				if row.native.profile.Report.FiveHour != nil {
					resets = append(resets, "5h "+row.text.fiveReset)
				}
				if row.native.profile.Report.SevenDay != nil {
					resets = append(resets, "7d "+row.text.sevenReset)
				}
				lines = append(lines, wrapped(strings.Join(resets, " · "), width)...)
			}
		}
		lines = append(lines, "")
	}
	return lines
}
func (m *model) usageBody(height int) []string {
	lines := m.usageLines()
	offset := min(m.offset, max(0, len(lines)-(height-2)))
	return append([]string{bold.Styled("usage · all machines"), ""}, lines[offset:min(len(lines), offset+max(0, height-2))]...)
}
func (m *model) usageKey(key string) {
	switch key {
	case "q", "esc":
		m.page = ""
	case "up", "k":
		m.offset = max(0, m.offset-1)
	case "down", "j":
		m.offset = min(max(0, len(m.usageLines())-m.pageCapacity()), m.offset+1)
	case "pgup":
		m.offset = max(0, m.offset-m.pageCapacity())
	case "pgdown":
		m.offset = min(max(0, len(m.usageLines())-m.pageCapacity()), m.offset+m.pageCapacity())
	}
}
