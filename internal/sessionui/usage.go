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
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/charmbracelet/x/ansi"
)

const usageMachineLabel = "devbox"

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
	machine    string
	result     fleetclient.Result
}

func (m *model) usageScope() (string, bool) {
	if m.usageStopped || m.usageAttached || m.ctx.Err() != nil {
		return "", false
	}
	switch m.page {
	case "", "usage":
		_, configured := m.client.MachineByLabel(usageMachineLabel)
		return usageMachineLabel, configured
	default:
		return "", false
	}
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
	machine, visible := m.usageScope()
	if !visible || m.usagePending || !manual && now.Before(m.usageNext) {
		return nil
	}
	m.usageGeneration++
	ctx, cancel := context.WithCancel(m.ctx)
	m.usageCancel, m.usagePending = cancel, true
	// justify-polling: the existing browser tick reads visible quota scope every
	// minute from attempt start; scope changes/attachment/quit cancel its work.
	m.usageNext = now.Add(time.Minute)
	client, generation, workers := m.client, m.usageGeneration, &m.usageWorkers
	completed := make(chan usageMsg, 1)
	workers.Add(1)
	// Bubble Tea does not wait for command goroutines. Start the actual worker
	// here, so Run can cancel and observe it even if its delivery command never runs.
	go func() {
		defer workers.Done()
		defer cancel()
		result := client.ProfileUsage(ctx, machine)
		completed <- usageMsg{generation: generation, machine: machine, result: result}
	}()
	return func() tea.Msg { return <-completed }
}

func (m *model) acceptUsage(message usageMsg) {
	machine, visible := m.usageScope()
	if !visible || message.generation != m.usageGeneration || message.machine != machine {
		return
	}
	m.usagePending, m.usageCancel = false, nil
	if !message.result.OK {
		panic("usage read selected an unknown configured machine") // justify-defect: browser scopes come only from the client's immutable validated machine table.
	}
	for _, received := range message.result.Value.(fleetclient.ProfileUsageObservation).Peers {
		for index := range m.usagePeers {
			peer := &m.usagePeers[index]
			if peer.machine.Handle != received.Machine {
				continue
			}
			peer.failure = received.Error
			if !received.OK {
				break
			}
			profiles := make([]usageProfile, len(received.Profiles))
			for profileIndex, profile := range received.Profiles {
				value := usageProfile{profile: profile, observedAt: received.ObservedAt, receivedAt: received.ReceivedAt}
				if profile.ReadState == "unavailable" {
					for _, previous := range peer.profiles {
						if previous.profile.Profile == profile.Profile {
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

// Inventory can reveal a changed descriptor before the next quota read. Clear
// only changed profiles and retire a read of that visible machine's old table.
func (m *model) reconcileUsageProfiles(received fleetclient.Peer) bool {
	for index := range m.usagePeers {
		peer := &m.usagePeers[index]
		if peer.machine.Handle != received.Machine {
			continue
		}
		known := peer.profiles != nil
		same := peer.profiles != nil && len(peer.profiles) == len(received.Profiles)
		if same {
			for index, profile := range received.Profiles {
				if peer.profiles[index].profile.Profile != profile {
					same = false
					break
				}
			}
		}
		if same {
			return false
		}
		profiles := make([]usageProfile, len(received.Profiles))
		for index, profile := range received.Profiles {
			source := "native"
			if profile.Provider == "Claude" {
				source = "statusline"
			}
			profiles[index] = usageProfile{profile: fleetclient.ProfileUsage{Profile: profile, Source: source, ReadState: "ok"}}
			for _, previous := range peer.profiles {
				if previous.profile.Profile == profile {
					profiles[index] = previous
					break
				}
			}
		}
		peer.profiles = profiles
		scope, visible := m.usageScope()
		if known && visible && scope == peer.machine.Label {
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
	source, age                               string
	stale                                     bool
}

func usagePresentation(profile usageProfile, hostFailed bool, now time.Time) usageText {
	text := usageText{fiveHour: "—", sevenDay: "—", fiveReset: "reset unknown", sevenReset: "reset unknown", source: "native", age: "no report"}
	if profile.profile.Source == "statusline" {
		text.source = "last reported"
	}
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

func usageWindowText(window *agentcontrol.UsageWindow, observedAt time.Time, elapsed time.Duration) (string, string) {
	if window == nil {
		return "—", "reset unknown"
	}
	reset := "reset unknown"
	if window.ResetsAt != nil {
		remaining := window.ResetsAt.Sub(observedAt) - elapsed
		if remaining <= 0 {
			return "—", "awaiting report after reset"
		}
		reset = "resets " + window.ResetsAt.UTC().Format("2006-01-02 15:04:05") + " utc · in " + remaining.Round(time.Second).String()
	}
	return strconv.FormatFloat(math.Floor(max(0, 100-window.UsedPercent)), 'f', 0, 64) + "%", reset
}

func (m *model) usageSummary() []string {
	if m.page != "" {
		return nil
	}
	machine, visible := m.usageScope()
	if !visible {
		if machine == usageMachineLabel {
			return []string{"remaining unavailable"}
		}
		return nil
	}
	width := m.width - 2
	heading := "remaining · "
	for _, peer := range m.usagePeers {
		if peer.machine.Label != machine {
			continue
		}
		if peer.profiles == nil {
			state := "checking"
			if peer.failure != nil {
				state = "unavailable"
			}
			return []string{heading + state}
		}
		if len(peer.profiles) == 0 {
			return []string{heading + "no agent profiles"}
		}
		codex, claude := []string{}, []string{}
		now := time.Now()
		for _, profile := range peer.profiles {
			text := usagePresentation(profile, peer.failure != nil, now)
			if profile.profile.Provider == "Codex" {
				codex = append(codex, profile.profile.Key+" "+text.sevenDay)
			} else {
				claude = append(claude, profile.profile.Key+": 5h "+text.fiveHour+" · 7d "+text.sevenDay)
			}
		}
		lines := []string{}
		if len(codex) > 0 {
			lines = append(lines, heading+"codex 7d: "+strings.Join(codex, " · "))
		}
		for _, item := range claude {
			prefix := strings.Repeat(" ", ansi.StringWidth(heading))
			if len(lines) == 0 {
				prefix = heading
			}
			lines = append(lines, prefix+item)
		}
		for index, line := range lines {
			lines[index] = ansi.Truncate(line, width, "…")
		}
		return lines
	}
	panic("usage summary has no configured machine") // justify-defect: visible usage scope and retained peers select the same configured devbox.
}

func (m *model) usageLines() []string {
	if _, configured := m.client.MachineByLabel(usageMachineLabel); !configured {
		return []string{usageMachineLabel + " is not configured"}
	}
	lines := []string{}
	now, width := time.Now(), m.width-2
	for _, peer := range m.usagePeers {
		if peer.machine.Label != usageMachineLabel {
			continue
		}
		lines = append(lines, bold.Styled(peer.machine.Label))
		if peer.failure != nil {
			lines = append(lines, wrapped("unavailable ("+singleLine(peer.failure.Code)+"); showing last reports", width)...)
		}
		if peer.profiles == nil {
			state := "checking profile usage"
			if peer.failure != nil {
				state = "no report; profile usage unavailable"
			}
			lines = append(lines, state, "")
			continue
		}
		if len(peer.profiles) == 0 {
			lines = append(lines, "no agent profiles", "")
			continue
		}
		for _, profile := range peer.profiles {
			text := usagePresentation(profile, peer.failure != nil, now)
			remaining := fmt.Sprintf("%s  7d remaining %s", profile.profile.Key, text.sevenDay)
			resets := "7d " + text.sevenReset
			if profile.profile.Provider == "Claude" || profile.profile.Report != nil && profile.profile.Report.FiveHour != nil {
				remaining = fmt.Sprintf("%s  5h remaining %s  7d remaining %s", profile.profile.Key, text.fiveHour, text.sevenDay)
				resets = "5h " + text.fiveReset + "  ·  " + resets
			}
			if text.stale {
				remaining += " · stale"
			}
			lines = append(lines, wrapped(remaining, width)...)
			source := text.source + " · " + text.age
			if peer.failure != nil || profile.profile.ReadState == "unavailable" {
				source += " · unavailable"
			}
			lines = append(lines, wrapped(source, width)...)
			lines = append(lines, wrapped(resets, width)...)
		}
		lines = append(lines, "")
	}
	return lines
}

func (m *model) usageBody(height int) []string {
	lines := m.usageLines()
	offset := min(m.offset, max(0, len(lines)-(height-2)))
	return append([]string{bold.Styled("usage · remaining"), ""}, lines[offset:min(len(lines), offset+max(0, height-2))]...)
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
