package sessionui

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
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
		// Older gateways declare Claude in inventory but omit it from native
		// usage. Its declaration still has a completed, unavailable reading.
		for _, declared := range m.peers[index].Profiles {
			if declared.Provider == "Claude" && !slices.ContainsFunc(profiles, func(value usageProfile) bool {
				return value.profile.Key == declared.Key
			}) {
				profiles = append(profiles, usageProfile{profile: fleetclient.ProfileUsage{Profile: declared, ReadState: "unavailable"}})
			}
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

type usageText struct {
	fiveHour, sevenDay, fableSevenDay *windowText
	stale                             bool
}

// left is the printed remainder and share the part of the window's time still
// to run. Each is -1 when it has no mark. A past reset keeps its text.
type windowText struct {
	remaining, resetTime string
	left                 int
	share                float64
	expired              bool
}

func unknownWindowText() windowText { return windowText{remaining: "—", left: -1, share: -1} }

// resetWhen includes the direction when a date is shown in running text.
func (w windowText) resetWhen() string {
	if w.share >= 0 {
		return "in " + w.resetTime
	}
	return w.resetTime
}

func terminalWindowText(left int, span, until time.Duration) *windowText {
	text := usageWindowText(left, span, until)
	if left >= 0 {
		text.remaining = "~" + text.remaining
	}
	return text
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
	text.fiveHour = nativeWindowText(report.FiveHour, 5*time.Hour, profile.observedAt, elapsed)
	text.sevenDay = nativeWindowText(report.SevenDay, 7*24*time.Hour, profile.observedAt, elapsed)
	return text
}
func usageWindowAvailable(window *api.QuotaWindow, observedAt time.Time, elapsed time.Duration) bool {
	return window != nil && (window.ResetsAt == nil || window.ResetsAt.Sub(observedAt) > elapsed)
}
func nativeWindowText(window *api.QuotaWindow, span time.Duration, observedAt time.Time, elapsed time.Duration) *windowText {
	if window == nil {
		return nil
	}
	if !usageWindowAvailable(window, observedAt, elapsed) {
		text := unknownWindowText()
		text.expired = true
		return &text
	}
	var until time.Duration
	if window.ResetsAt != nil {
		until = window.ResetsAt.Sub(observedAt) - elapsed
	}
	return usageWindowText(int(max(0, 100-window.UsedPercent)), span, until)
}

func usageWindowText(left int, span, until time.Duration) *windowText {
	text := unknownWindowText()
	if left >= 0 {
		text.left = left
		text.remaining = strconv.Itoa(left) + "%"
	}
	if until > 0 {
		text.share = float64(until) / float64(span)
		// Whole minutes still to run, in the two largest units.
		switch minutes := int(until / time.Minute); {
		case minutes >= 24*60:
			text.resetTime = fmt.Sprintf("%dd %dh", minutes/(24*60), minutes/60%24)
		case minutes >= 60:
			text.resetTime = fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
		case minutes > 0:
			text.resetTime = strconv.Itoa(minutes) + "m"
		default:
			text.resetTime = "<1m"
		}
	}
	return &text
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
	switch {
	case row.state != "" || window == nil:
		return unknownWindowText(), row.state
	case window.expired:
		return unknownWindowText(), "awaiting update"
	case row.text.stale:
		return unknownWindowText(), "stale"
	}
	return *window, ""
}

func usageLater(receivedAt time.Time, machine string, previous time.Time, previousMachine string) bool {
	return receivedAt.After(previous) || receivedAt.Equal(previous) && machine < previousMachine
}
func (m *model) nativeUsage(profile fleetclient.Profile, now time.Time) usageRow {
	row := usageRow{profile: profile, state: "checking"}
	var expired *usageProfile
	var expiredMachine fleetclient.Machine
	expiredFailed, unavailable, absent := false, false, false
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
			fresh := !usagePresentation(*value, peer.failure != nil, now).stale
			previousFresh := row.native != nil && !usagePresentation(*row.native, row.failed, now).stale
			if row.native == nil || fresh && !previousFresh || fresh == previousFresh && usageLater(value.receivedAt, peer.machine.Handle, row.native.receivedAt, row.machine.Handle) {
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
func (m *model) codexTerminalUsage(profile fleetclient.Profile, now time.Time) usageRow {
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
		if candidate != nil && (row.terminal == nil || usageLater(peer.ReceivedAt, peer.Machine, row.receivedAt, row.machine.Handle)) {
			row.terminal, row.machine, row.receivedAt = candidate, fleetclient.Machine{Label: peer.Label, Handle: peer.Machine}, peer.ReceivedAt
		}
	}
	if row.terminal != nil {
		row.state = ""
		row.text.stale = max(time.Duration(0), now.Sub(row.receivedAt)) >= usageStaleAfter
		if value := row.terminal.TerminalUsage.FiveHour; value != nil {
			row.text.fiveHour = terminalWindowText(100-int(value.UsedPercent), 5*time.Hour, 0)
		}
		if value := row.terminal.TerminalUsage.SevenDay; value != nil {
			row.text.sevenDay = terminalWindowText(100-int(value.UsedPercent), 7*24*time.Hour, 0)
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
	rows := make([]usageRow, len(profiles))
	for index, profile := range profiles {
		switch profile.Provider {
		case "Codex":
			native := m.nativeUsage(profile, now)
			terminal := m.codexTerminalUsage(profile, now)
			if native.native != nil && !native.text.stale && (native.text.fiveHour != nil && !native.text.fiveHour.expired || native.text.sevenDay != nil && !native.text.sevenDay.expired) {
				rows[index] = native
			} else if terminal.terminal != nil && !terminal.text.stale {
				rows[index] = terminal
			} else if native.native != nil {
				rows[index] = native
			} else if terminal.terminal != nil {
				rows[index] = terminal
			} else {
				rows[index] = native
			}
		case "Claude":
			rows[index] = m.claudeUsageRow(profile, now)
		default:
			rows[index] = usageRow{profile: profile}
		}
	}
	return rows
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

func (m *model) weeklyQuota(room int) string {
	if room <= 0 {
		return ""
	}
	rows := m.usageRows(m.now)
	if len(rows) == 0 {
		state := m.usageDeclarationState()
		if ansi.StringWidth(state) <= room {
			return state
		}
		return ""
	}
	selectedProfile := m.selectedProfile()
	var items []string
	selected := ""
	for _, row := range rows {
		text, status := row.windowPresentation(row.text.sevenDay)
		profile := attention.SingleLine(row.profile.Key)
		item := profile + " " + text.remaining
		if status != "" {
			item += " " + status
		}
		one := profile + " 7d " + text.remaining
		switch {
		case status != "":
			one = item
		case text.left >= 0 || row.profile.Provider != "Claude":
			one += " left"
		}
		items = append(items, item)
		if row.profile.Key == selectedProfile {
			selected = one
		}
	}
	all := "7d quota left  " + strings.Join(items, "  ")
	if ansi.StringWidth(all) <= room {
		return all
	}
	if ansi.StringWidth(selected) <= room {
		return selected
	}
	return ""
}

// Fixed slot widths leave the full band 78 cells wide at the minimum terminal size.
const (
	usageName  = 11
	usageGauge = 16
	usageSlot  = usageGauge + 1 + 5 + 2 + 7
)

func (m *model) usageBand(limit int) []line {
	if limit < 3 {
		return nil
	}
	rows := m.usageRows(m.now)
	head := line{{text: cell("usage", usageName+2), role: quietRole, bold: true}}
	if len(rows) == 0 {
		return []line{append(head, span{text: m.usageDeclarationState(), role: mutedRole, italic: true})}
	}
	cut := 0
	if len(rows)+1 > limit {
		cut = len(rows) - (limit - 2)
		rows = rows[:limit-2]
	}
	lines := []line{head}
	selected := m.selectedProfile()
	frame := -1
	if !m.reveal.IsZero() {
		frame = int(m.now.Sub(m.reveal) / revealFrame)
	}
	five, heavy, timed := "", "", false
	showFableKey := false
	extra := limit - 1 - len(rows)
	for _, row := range rows {
		name := span{text: cell(row.profile.Key, usageName) + "  ", bold: row.profile.Key == selected}
		fableLeft := -1
		detail := ""
		if fable := row.text.fableSevenDay; fable != nil {
			fableLeft = fable.left
			showFableKey = showFableKey || fableLeft >= 0
			detail = "fable 7d " + fable.remaining
			if fable.resetTime != "" {
				detail += "  " + fable.resetWhen()
			}
		}
		entry := append(line{name}, m.usageWindow(row, row.text.sevenDay, true, fableLeft, frame)...)
		entry = append(entry, span{text: "   "})
		if detail != "" && row.text.fiveHour == nil && ansi.StringWidth(detail) <= usageSlot {
			entry = append(entry, span{text: detail})
			detail = ""
		} else {
			entry = append(entry, m.usageWindow(row, row.text.fiveHour, false, -1, frame)...)
		}
		if detail != "" && usageName+2+2*usageSlot+3+3+ansi.StringWidth(detail) <= m.width-2 {
			entry = append(entry, span{text: "   " + detail})
			detail = ""
		}
		lines = append(lines, entry)
		if row.text.fiveHour != nil {
			five = "5h"
		}
		if detail != "" && cut == 0 && extra > 0 {
			lines = append(lines, line{{text: "  " + detail}})
			extra--
		}
	}
	for _, entry := range lines[1:] {
		// A stem can replace the only heavy mark. Key the marks that survived
		// in the drawn slots, excluding the operator's profile name.
		for _, part := range entry[1:] {
			for _, mark := range part.text {
				switch mark {
				case '━', '╾':
					heavy = "━"
				case '═':
					heavy = "═"
				case '┷', '┵', '┴', '╧', '╋', '┫':
					timed = true
				}
			}
		}
	}
	if cut > 0 {
		lines = append(lines, line{{text: fmt.Sprintf("… %d more profiles in a taller window", cut), role: quietRole}})
	}
	var legend line
	if heavy != "" {
		legend = line{{text: heavy}, {text: " quota left", role: quietRole}}
	}
	if timed {
		legend = append(legend, span{text: "  ┴"}, span{text: " time left", role: quietRole})
	}
	room := usageSlot
	for _, part := range legend {
		room -= ansi.StringWidth(part.text)
	}
	if showFableKey {
		head = append(head, span{text: "7d  ", role: quietRole}, span{text: "▬", role: goldRole}, span{text: cell(" fable (½ scale)", usageSlot+3-5), role: quietRole})
	} else {
		head = append(head, span{text: cell("7d", usageSlot+3), role: quietRole})
	}
	head = append(head, span{text: cell(five, room), role: quietRole})
	lines[0] = append(head, legend...)
	return lines
}

func (m *model) usageWindow(row usageRow, window *windowText, weekly bool, fableLeft, frame int) line {
	text, word := row.windowPresentation(window)
	if text.left < 0 && fableLeft < 0 {
		if !weekly && (window == nil || row.state != "") {
			word = ""
		} else if text.resetTime != "" {
			return line{{text: cell("—  reset "+text.resetWhen(), usageSlot), role: quietRole}}
		} else if word == "" {
			return line{{text: cell("—", usageSlot), role: quietRole}}
		}
		return line{{text: cell(word, usageSlot), role: mutedRole, italic: true}}
	}

	gauge := m.quotaGauge(text.left, text.share, fableLeft, frame)
	return append(gauge, span{text: fmt.Sprintf(" %5s", text.remaining)}, span{text: "  " + cell(text.resetTime, 7), role: quietRole})
}

func (m *model) quotaGauge(left int, share float64, fableLeft, frame int) line {
	// Round to the nearest half cell (whole on plain), reserving empty and
	// full for exactly 0 and 100 percent.
	scale := 2 * usageGauge
	heavy, heavyStem := "━", "┷"
	if m.ground.raise == nil {
		scale, heavy, heavyStem = usageGauge, "═", "╧"
	}
	units := (max(0, left)*scale + 50) / 100
	if left > 0 && left < 100 {
		units = min(max(units, 1), scale-1)
	}
	halves := units * (2 * usageGauge / scale)
	// Fable has its own zero and full values on the first half of this track.
	fableCells := (max(0, fableLeft)*(usageGauge/2) + 50) / 100
	if fableLeft > 0 && fableLeft < 100 {
		fableCells = min(max(1, fableCells), usageGauge/2-1)
	}
	stem := -1
	if share >= 0 {
		stem = min(max(0, int(math.Ceil(share*usageGauge))-1), usageGauge-1)
	}
	firstSpent, chomper := (halves+1)/2, -1
	if frame >= 0 {
		chomper = usageGauge - 1 - frame/2
	}
	var gauge line
	for index := range usageGauge {
		glyph, stemGlyph, ink := "─", "┴", quietRole
		if index < halves/2 {
			glyph, stemGlyph, ink = heavy, heavyStem, inkRole
		} else if index < firstSpent {
			glyph, stemGlyph, ink = "╾", "┵", inkRole
		}
		// Inside the thicker overlay the pale cap marks a covered general
		// endpoint, and the pale cross marks the general reset stem.
		covered := index < fableCells
		endpoint := firstSpent > 0 && firstSpent < fableCells && index == firstSpent-1
		switch {
		case covered && index == stem && endpoint:
			glyph, ink = "┫", inkRole
		case covered && index == stem:
			glyph, ink = "╋", inkRole
		case covered && endpoint:
			glyph, ink = "▕", inkRole
		case covered:
			glyph, ink = "▬", goldRole
		case left >= 0 && index == chomper && index >= firstSpent:
			glyph, ink = "◕", goldRole
			if frame%2 == 1 && index > firstSpent {
				glyph = "●"
			}
		case index == stem:
			glyph, ink = stemGlyph, inkRole
		case left < 0:
			glyph = " "
		case index >= firstSpent && index < chomper:
			glyph = "╍"
		}
		if covered && index == stem && fableCells <= 2 {
			ink = goldRole // the stem and cap can occupy the entire short overlay
		}
		if last := len(gauge) - 1; last >= 0 && gauge[last].role == ink {
			gauge[last].text += glyph
		} else {
			gauge = append(gauge, span{text: glyph, role: ink})
		}
	}
	return gauge
}
