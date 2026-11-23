package sessionui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
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
	peers      []fleetclient.ProfileUsagePeer
	claude     []fleetclient.ProfileUsage
}

// Explicit reads coalesce to one successor while a read holds the lane.
func (m *model) refreshUsage(manual bool) tea.Cmd {
	now := time.Now()
	if !m.observing() || !manual && !m.usageLane.trailing && now.Before(m.usageNext) {
		return nil
	}
	client := m.client
	command := m.usageLane.read(m.ctx, manual, func(ctx context.Context, generation uint64) tea.Msg {
		peers := client.ProfileUsage(ctx)
		return usageMsg{generation: generation, peers: peers, claude: client.KnownClaudeUsage(peers...)}
	})
	if command != nil {
		// The existing browser tick reads the full native quota scope every
		// minute from attempt start while the browser owns the screen.
		m.usageNext = now.Add(time.Minute)
	}
	return command
}
func (m *model) acceptUsage(message usageMsg) tea.Cmd {
	landed, current := m.usageLane.land(message.generation)
	if !landed {
		return nil
	}
	if !current || !m.observing() {
		if m.usageLane.trailing { // a fence retired this read and asked for its successor
			return m.refreshUsage(false)
		}
		return nil
	}
	m.claudeUsage = message.claude
	for _, received := range message.peers {
		index := m.peerIndex(received.Machine)
		if index < 0 {
			continue
		}
		peer := &m.usagePeers[index]
		peer.failure = received.Error
		m.invalidateAccess(received.Machine, received.Error)
		if !received.OK {
			continue
		}
		profiles := make([]usageProfile, len(received.Profiles))
		for profileIndex, profile := range received.Profiles {
			value := usageProfile{profile: profile, observedAt: received.ObservedAt, receivedAt: received.ReceivedAt}
			if profile.Provider == "Codex" && profile.ReadState == "unavailable" {
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
	}
	if m.usageLane.trailing {
		return m.refreshUsage(false)
	}
	return nil
}

// Full inventories own current descriptors. A change clears only that host's
// changed native reports and retires any whole-fleet read of the preceding table.
func (m *model) reconcileUsageProfiles(index int, received fleetclient.Peer) bool {
	peer := &m.usagePeers[index]
	known := peer.profiles != nil
	changed := false
	profiles := make([]usageProfile, 0, len(received.Profiles))
	for _, profile := range received.Profiles {
		if profile.Provider != "Codex" && profile.Provider != "Claude" {
			continue
		}
		found := slices.IndexFunc(peer.profiles, func(old usageProfile) bool { return old.profile.Profile == profile })
		if found >= 0 {
			profiles = append(profiles, peer.profiles[found])
		} else {
			profiles = append(profiles, usageProfile{profile: fleetclient.ProfileUsage{Profile: profile, ReadState: "ok"}})
			changed = true
		}
	}
	changed = changed || len(peer.profiles) != len(profiles)
	peer.profiles = profiles
	if known && !changed {
		return false
	}
	if known || m.usageLane.flight != 0 {
		m.usageLane.fence()
		m.usageLane.trailing = true
		m.usageNext = time.Time{}
		return true
	}
	return false
}

const usageStaleAfter = time.Hour

// usageRow is one profile at m.now: the one contract between provider
// selection and drawing.
type usageRow struct {
	profile           fleetclient.Profile
	state             string       // "", "checking", "unavailable"; set only when no window is
	week, fable, five *quotaWindow // nil: no such window
}

// quotaWindow is a window as the band draws it.
type quotaWindow struct {
	left     int            // printed percent left; -1 unknown
	estimate bool           // read from a terminal: printed with ~
	until    *time.Duration // to its reset; ≤ 0 passed; nil: none
	faded    bool
}

func (w quotaWindow) percent() string {
	switch {
	case w.left < 0:
		return "—"
	case w.estimate:
		return "~" + strconv.Itoa(w.left) + "%"
	default:
		return strconv.Itoa(w.left) + "%"
	}
}

func (row usageRow) windows() (some, unfaded bool) {
	for _, window := range []*quotaWindow{row.week, row.fable, row.five} {
		if window != nil {
			some, unfaded = true, unfaded || !window.faded
		}
	}
	return some, unfaded
}

func (row usageRow) weeklyResetSource() *quotaWindow {
	if row.week != nil && row.week.until != nil {
		return row.week
	}
	if row.fable != nil && row.fable.until != nil {
		return row.fable
	}
	return nil
}

// A used percent can exceed 100; left saturates at 0.
func percentLeft(used float64) int {
	return 100 - int(min(100, used))
}
func usageWindowAvailable(window *api.QuotaWindow, observedAt time.Time, elapsed time.Duration) bool {
	return window != nil && (window.ResetsAt == nil || window.ResetsAt.Sub(observedAt) > elapsed)
}
func usageLater(receivedAt time.Time, machine string, previous time.Time, previousMachine string) bool {
	return receivedAt.After(previous) || receivedAt.Equal(previous) && machine < previousMachine
}

// nativeUsage projects one native report across hosts: one with a window still
// ahead, fresh before stale, then the later receipt; else the latest whose
// windows have all passed. A host whose access was rejected offers none.
func (m *model) nativeUsage(profile fleetclient.Profile) usageRow {
	var chosen, expired *usageProfile
	var chosenMachine, expiredMachine string
	chosenStale, unavailable, absent := false, false, false
	for index, peer := range m.usagePeers {
		relevant := peer.profiles == nil
		for _, value := range peer.profiles {
			relevant = relevant || value.profile.Key == profile.Key && value.profile.Provider == profile.Provider
		}
		if !relevant {
			continue
		}
		if accessRejected(m.peers[index].Error) {
			unavailable = true
			continue
		}
		failed := peer.failure != nil
		unavailable = unavailable || failed
		for index := range peer.profiles {
			value := &peer.profiles[index]
			if value.profile.Key != profile.Key || value.profile.Provider != profile.Provider {
				continue
			}
			unavailable = unavailable || value.profile.ReadState == "unavailable"
			report := value.profile.Report
			if report == nil {
				absent = absent || !value.receivedAt.IsZero()
				continue
			}
			elapsed := max(0, m.now.Sub(value.receivedAt))
			if !usageWindowAvailable(report.FiveHour, value.observedAt, elapsed) && !usageWindowAvailable(report.SevenDay, value.observedAt, elapsed) {
				absent = true
				if (report.FiveHour != nil || report.SevenDay != nil) && (expired == nil || usageLater(value.receivedAt, peer.machine.Handle, expired.receivedAt, expiredMachine)) {
					expired, expiredMachine = value, peer.machine.Handle
				}
				continue
			}
			stale := failed || value.profile.ReadState == "unavailable" || value.observedAt.Sub(report.ReportedAt)+elapsed >= usageStaleAfter
			if chosen == nil || !stale && chosenStale || stale == chosenStale && usageLater(value.receivedAt, peer.machine.Handle, chosen.receivedAt, chosenMachine) {
				chosen, chosenMachine, chosenStale = value, peer.machine.Handle, stale
			}
		}
	}
	if chosen == nil {
		chosen = expired
	}
	switch {
	case chosen != nil:
		elapsed := max(0, m.now.Sub(chosen.receivedAt))
		window := func(quota *api.QuotaWindow) *quotaWindow {
			if quota == nil {
				return nil
			}
			projected := &quotaWindow{left: percentLeft(quota.UsedPercent), faded: chosenStale}
			if quota.ResetsAt != nil {
				until := quota.ResetsAt.Sub(chosen.observedAt.Add(elapsed))
				projected.until, projected.faded = &until, chosenStale || until <= 0
			}
			return projected
		}
		return usageRow{profile: profile, week: window(chosen.profile.Report.SevenDay), five: window(chosen.profile.Report.FiveHour)}
	case unavailable:
		return usageRow{profile: profile, state: "unavailable"}
	case absent:
		return usageRow{profile: profile}
	default:
		return usageRow{profile: profile, state: "checking"}
	}
}
func (m *model) codexTerminalUsage(profile fleetclient.Profile) usageRow {
	var terminal *fleetclient.Session
	var machine string
	var receivedAt time.Time
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
		if peer.Error != nil {
			unavailable = true
			continue
		}
		absent = true
		var candidate *fleetclient.Session
		count := 0
		for index := range peer.Sessions {
			session := &peer.Sessions[index]
			if session.Agent == nil {
				unavailable = unavailable || session.TerminalStatus.Source == api.SourceUnavailable
				continue
			}
			if session.Agent.Provider != profile.Provider || session.Agent.Profile != profile.Key || session.Connection != nil {
				continue
			}
			unavailable = unavailable || session.TerminalStatus.Source == api.SourceUnavailable
			usage := session.TerminalUsage
			if usage == nil {
				continue
			}
			known := 0
			if usage.FiveHour != nil {
				known++
			}
			if usage.SevenDay != nil {
				known++
			}
			if candidate == nil || known > count || known == count && fleetclient.CompareSessions(*session, *candidate) < 0 {
				candidate, count = session, known
			}
		}
		if candidate != nil && (terminal == nil || usageLater(peer.ReceivedAt, peer.Machine, receivedAt, machine)) {
			terminal, machine, receivedAt = candidate, peer.Machine, peer.ReceivedAt
		}
	}
	switch {
	case terminal != nil:
		faded := m.now.Sub(receivedAt) >= usageStaleAfter
		window := func(quota *api.QuotaWindow) *quotaWindow {
			if quota == nil {
				return nil
			}
			return &quotaWindow{left: percentLeft(quota.UsedPercent), estimate: true, faded: faded}
		}
		return usageRow{profile: profile, week: window(terminal.TerminalUsage.SevenDay), five: window(terminal.TerminalUsage.FiveHour)}
	case unavailable:
		return usageRow{profile: profile, state: "unavailable"}
	case absent:
		return usageRow{profile: profile}
	default:
		return usageRow{profile: profile, state: "checking"}
	}
}

// Only claude can read fable.
func (m *model) usageRows() (rows []usageRow, fableColumn bool) {
	profiles := []fleetclient.Profile{}
	add := func(profile fleetclient.Profile) {
		for _, present := range profiles {
			if present.Key == profile.Key {
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
	for _, known := range m.claudeUsage {
		add(known.Profile)
	}
	rows = make([]usageRow, len(profiles))
	for index, profile := range profiles {
		switch profile.Provider {
		case "Codex":
			native, terminal := m.nativeUsage(profile), m.codexTerminalUsage(profile)
			nativeSome, nativeUnfaded := native.windows()
			terminalSome, terminalUnfaded := terminal.windows()
			switch {
			case nativeUnfaded:
				rows[index] = native
			case terminalUnfaded:
				rows[index] = terminal
			case nativeSome:
				rows[index] = native
			case terminalSome:
				rows[index] = terminal
			default:
				rows[index] = native
			}
		case "Claude":
			rows[index] = m.claudeUsageRow(profile)
			fableColumn = true
		default:
			rows[index] = usageRow{profile: profile}
		}
	}
	return rows, fableColumn
}
func (m *model) usageDeclarationState() string {
	full, completed, failed, pending := true, false, false, false
	for index, peer := range m.peers {
		lane := m.inventoryLanes[index]
		pending = pending || lane.flight != 0 || lane.writing || lane.trailing
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
	if !completed || pending && !failed {
		return "checking usage"
	}
	return "profile declarations unavailable"
}
func (m *model) selectedProfile() string {
	if row := m.selectedRow(); row != nil {
		if agent := m.current(row).Agent; agent != nil {
			return agent.Profile
		}
	}
	return ""
}

func countdown(until time.Duration, now time.Time) string {
	if until > 0 {
		switch minutes := int(until / time.Minute); {
		case minutes >= 24*60:
			return fmt.Sprintf("%dd %dh", minutes/(24*60), minutes/60%24)
		case minutes >= 60:
			return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
		case minutes > 0:
			return strconv.Itoa(minutes) + "m"
		default:
			return "<1m"
		}
	}
	switch {
	case until > -time.Minute:
		return "<1m ago"
	case until > -time.Hour:
		return fmt.Sprintf("%dm ago", -until/time.Minute)
	case until > -24*time.Hour:
		return fmt.Sprintf("%dh ago", -until/time.Hour)
	case until > -7*24*time.Hour:
		return fmt.Sprintf("%dd ago", -until/(24*time.Hour))
	case until > -365*24*time.Hour:
		return fmt.Sprintf("%dw ago", -until/(7*24*time.Hour))
	default:
		return now.Add(until).UTC().Format("2006-01") // in UTC, as resets are reported
	}
}

func (m *model) weeklyQuota(room int) line {
	if room <= 0 {
		return nil
	}
	rows, _ := m.usageRows()
	if len(rows) == 0 {
		if state := m.usageDeclarationState(); ansi.StringWidth(state) <= room {
			return line{{text: state}}
		}
		return nil
	}
	selectedProfile := m.selectedProfile()
	all := line{{text: "7d quota left"}}
	var selected line
	for _, row := range rows {
		name, amount := span{text: attention.SingleLine(row.profile.Key)}, span{text: "—", role: quietRole}
		if row.week != nil {
			amount = percentCell(row.week, 0)
			if row.week.faded {
				name.role = quietRole
			}
		}
		item := line{name, {text: " "}, amount}
		one := line{name, {text: " 7d ", role: name.role}, amount}
		switch {
		case row.state != "":
			item = append(item, span{text: " "}, span{text: row.state, role: mutedRole, italic: true})
			one = item
		case row.week != nil && row.week.left >= 0:
			one = append(one, span{text: " left", role: name.role})
		}
		all = append(append(all, span{text: "  "}), item...)
		if row.profile.Key == selectedProfile {
			selected = one
		}
	}
	if all.width() <= room {
		return all
	}
	if selected != nil && selected.width() <= room {
		return selected
	}
	return nil
}

// The band is one grid of these widths, 78 cells at every terminal width;
// without a claude profile the fable column and its gap are absent.
const (
	usageName      = 11
	usageGauge     = 24
	usagePercent   = 5
	usageFable     = 7
	usageCountdown = 7
)

func (m *model) usageBand(limit int) []line {
	if limit < 3 {
		return nil
	}
	rows, fableColumn := m.usageRows()
	head := line{{text: cell("usage", usageName) + "  ", role: quietRole, bold: true}}
	if len(rows) == 0 {
		return []line{append(head, span{text: m.usageDeclarationState(), role: mutedRole, italic: true})}
	}
	cut := 0
	if len(rows)+1 > limit {
		cut = len(rows) - (limit - 2)
		rows = rows[:limit-2]
	}
	heavy := "━"
	if m.ground.raise == nil {
		heavy = "═"
	}
	// Titles align as their columns' cells do; a sample is one cell.
	head = append(head, span{text: cell("", usageGauge)}, span{text: " "},
		span{text: fmt.Sprintf("%*s", usagePercent-len(" 7d"), heavy)}, span{text: " 7d", role: quietRole}, span{text: "  "})
	if fableColumn {
		head = append(head, span{text: fmt.Sprintf("%*s", usageFable-len(" fable"), "🬋"), role: goldRole}, span{text: " fable", role: quietRole}, span{text: "  "})
	}
	head = append(head, span{text: "┴"}, span{text: cell(" reset", usageCountdown-1), role: quietRole}, span{text: "   "},
		span{text: fmt.Sprintf("%*s", usagePercent, "5h"), role: quietRole}, span{text: "  "}, span{text: cell("reset", usageCountdown), role: quietRole})
	lines := []line{head}
	selected := m.selectedProfile()
	frame := -1
	if !m.reveal.IsZero() {
		frame = int(m.now.Sub(m.reveal) / revealFrame)
	}
	for _, row := range rows {
		name := span{text: cell(row.profile.Key, usageName) + "  ", bold: row.profile.Key == selected}
		switch some, unfaded := row.windows(); {
		case row.state != "":
			lines = append(lines, line{name, {text: row.state, role: mutedRole, italic: true}})
		case !some:
			lines = append(lines, line{name, {text: "—", role: quietRole}})
		default:
			if !unfaded {
				name.role = quietRole
			}
			entry := append(line{name}, m.weekGauge(row, frame)...)
			entry = append(entry, span{text: " "}, percentCell(row.week, usagePercent), span{text: "  "})
			if fableColumn {
				entry = append(entry, percentCell(row.fable, usageFable), span{text: "  "})
			}
			entry = append(entry, m.countdownCell(row.weeklyResetSource(), row.week), span{text: "   "},
				percentCell(row.five, usagePercent), span{text: "  "}, m.countdownCell(row.five, row.five))
			lines = append(lines, entry)
		}
	}
	if cut > 0 {
		lines = append(lines, line{{text: fmt.Sprintf("… %d more profiles in a taller window", cut), role: quietRole}})
	}
	return lines
}

func percentCell(window *quotaWindow, width int) span {
	if window == nil {
		return span{text: cell("", width)}
	}
	role := inkRole
	if window.faded || window.left < 0 {
		role = quietRole
	}
	return span{text: fmt.Sprintf("%*s", width, window.percent()), role: role}
}

// source may be fable's window; only the period's general window can ink it.
func (m *model) countdownCell(source, general *quotaWindow) span {
	text, role := "", quietRole
	if source != nil && source.until != nil {
		text = countdown(*source.until, m.now)
		if general != nil && general.left == 0 && !general.faded && !source.faded {
			role = inkRole
		}
	}
	return span{text: cell(text, usageCountdown), role: role}
}

// Empty and full are reserved for exactly 0 and 100 percent.
func scaled(left, cells int) int {
	n := (left*cells + 50) / 100
	if left > 0 && left < 100 {
		n = min(max(n, 1), cells-1)
	}
	return n
}

func (m *model) weekGauge(row usageRow, frame int) line {
	heavy, heavyStem, scale := "━", "┷", 2*usageGauge
	if m.ground.raise == nil {
		heavy, heavyStem, scale = "═", "╧", usageGauge
	}
	known := row.week != nil && row.week.left >= 0
	halves := 0
	if known {
		halves = scaled(row.week.left, scale) * 2 * usageGauge / scale
	}
	touched := (halves + 1) / 2
	fableCells := 0
	if row.fable != nil && row.fable.left >= 0 {
		fableCells = scaled(row.fable.left, usageGauge/2)
	}
	stem := -1
	if reset := row.weeklyResetSource(); reset != nil && *reset.until > 0 {
		// For until > 0, (until − 1) / cell is ⌈until / cell⌉ − 1.
		stem = min(int((*reset.until-1)/(7*24*time.Hour/usageGauge)), usageGauge-1)
	}
	chomper := -1
	if known && frame >= 0 {
		chomper = usageGauge - 1 - frame/2
	}
	generalRole, fableRole := inkRole, goldRole
	if row.week != nil && row.week.faded {
		generalRole = quietRole
	}
	if row.fable != nil && row.fable.faded {
		fableRole = quietRole
	}
	var gauge line
	for index := range usageGauge {
		glyph, stemGlyph, role := "─", "┴", quietRole
		switch {
		case !known:
			glyph = " "
		case index < halves/2:
			glyph, stemGlyph, role = heavy, heavyStem, generalRole
		case index < touched:
			glyph, stemGlyph, role = "╾", "┵", generalRole
		}
		if index < fableCells { // thick where general quota can fund fable's, dashed beyond
			glyph, role = "╌", fableRole
			if index < touched || !known {
				glyph = "🬋"
			}
		}
		switch {
		case index == chomper && index >= touched:
			glyph, role = "◕", goldRole
			if frame%2 == 1 && index > touched {
				glyph = "●"
			}
		case index == stem:
			glyph, role = stemGlyph, generalRole
		case index >= touched && index < chomper:
			glyph, role = "╍", quietRole
		}
		if last := len(gauge) - 1; last >= 0 && gauge[last].role == role {
			gauge[last].text += glyph
		} else {
			gauge = append(gauge, span{text: glyph, role: role})
		}
	}
	return gauge
}
