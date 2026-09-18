package sessionui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/charmbracelet/x/ansi"
)

const sidebarWidth = 26

func (m *model) View() tea.View {
	if m.width < 80 || m.height < 24 {
		view := tea.NewView(ansi.Truncate("resize browser to at least 80 × 24; escape cancels or quits", max(1, m.width), "…"))
		view.AltScreen = true
		return view
	}
	footer := m.footerLines()
	notices := m.noticeLines()
	height := m.mainHeight() + 1
	width := m.width - sidebarWidth - 1
	left := m.sidebarLines(height)
	right := append([]string{m.tabLine(width)}, m.mainLines(width, height-1)...)
	heading := "skid · " + m.machineHeading() + " · " + fleetclient.SpaceFilterHeading(m.spaceFilter)
	if m.refreshing {
		heading += " · refreshing…"
	}
	lines := []string{cell(heading, m.width)}
	for i := 0; i < height; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, cell(l, sidebarWidth)+"│"+cell(r, width))
	}
	lines = append(lines, notices...)
	lines = append(lines, footer...)
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}

func (m *model) footerLines() []string {
	var text string
	switch m.page {
	case "confirm":
		text = "y/enter confirms · n/escape cancels"
	case "machine-picker":
		text = "up/down/j/k choose · enter selects · escape cancels"
	case "space-edit":
		text = "left/right fill suggestion · ctrl-u unassigned\nenter/ctrl-s saves · ctrl-r refresh · escape cancels"
		if m.spaceChecking {
			text = "checking inventory · ctrl-r refresh · escape cancels"
		}
	case "create":
		text = "left/right choose machine/launch/space · tab/enter next\nenter on space creates · ctrl-u unassigned · escape cancels"
	case "output", "details":
		text = "up/down/page-up/page-down scroll · q/escape returns"
	default:
		text = "g spaces · a agents · t tabs · tab/shift-tab focus · n new · m machine\n"
		switch m.focus {
		case tabs:
			text += "left/right/h/l select"
		case spaces:
			text += "up/down/j/k select · enter tabs"
		case agents:
			text += "up/down/j/k select"
		}
		text += " · ctrl-r refresh · q/escape quit"
		row := m.selectedRow()
		if m.focus == agents && m.agentIndex() < 0 {
			text += "\nno active agent · down/up selects first/last"
		} else if m.focus != spaces && row != nil {
			if row.available {
				text += "\nenter attach · space info"
				if row.session.Agent != nil {
					text += " · r read · i interrupt · s stop"
				}
				text += " · x kill\ne edit space · T terminal here"
			} else {
				text += "\nspace info · session unavailable; remote actions disabled"
			}
		}
	}
	return wrapped(text, m.width)
}

func (m *model) noticeLines() []string {
	lines := []string{}
	// Outcomes precede host notices: unavailable labels cannot conceal uncertainty.
	if m.notice != "" {
		lines = append(lines, wrapped(m.notice, m.width)...)
	}
	for _, peer := range m.scopedPeers() {
		if !peer.OK {
			lines = append(lines, ansi.Truncate(singleLine(peer.Label), max(1, m.width-13), "…")+": unavailable")
		}
	}
	limit := max(1, m.height-len(m.footerLines())-9)
	if len(lines) > limit {
		lines = append(lines[:limit-1], "…")
	}
	return lines
}

func (m *model) mainHeight() int {
	return m.height - 2 - len(m.footerLines()) - len(m.noticeLines())
}
func (m *model) pageCapacity() int {
	height := m.mainHeight()
	if m.page == "output" {
		height -= 2
	}
	return max(1, height)
}

func capturedHeading(name, machine string, width int) string {
	return ansi.Truncate(singleLine(name), (width-4)/2, "…") + " on " + ansi.Truncate(singleLine(machine), (width-4)/2, "…")
}

func (m *model) sidebarHeights(height int) (int, int) {
	spaces := min(len(m.spaceOptions())+1, height/2)
	return spaces, height - spaces
}

func fitList(top, index, length, height int) int {
	top = min(top, max(0, length-height))
	if index >= 0 {
		if index < top {
			top = index
		}
		if index >= top+height {
			top = index - height + 1
		}
	}
	return max(0, top)
}

func (m *model) fitViewports() {
	if m.width < 80 || m.height < 24 {
		return
	}
	height := m.mainHeight() + 1
	spacesHeight, agentsHeight := m.sidebarHeights(height)
	options := m.spaceOptions()
	m.spacesTop = fitList(m.spacesTop, slices.Index(options, m.spaceFilter), len(options), spacesHeight-1)
	m.agentsTop = fitList(m.agentsTop, m.agentIndex(), len(m.agents), agentsHeight-1)
	m.tabsTop = min(m.tabsTop, max(0, len(m.rows)-1))
	if m.cursor < 0 {
		m.tabsTop = 0
		return
	}
	if m.cursor < m.tabsTop {
		m.tabsTop = m.cursor
	}
	width := m.width - sidebarWidth - 1
	for m.tabsTop < m.cursor {
		used := 9
		for index := m.tabsTop; index <= m.cursor; index++ {
			used += ansi.StringWidth(m.tabText(index, width))
		}
		if used <= width {
			break
		}
		m.tabsTop++
	}
}

func (m *model) sidebarLines(height int) []string {
	spacesHeight, agentsHeight := m.sidebarHeights(height)
	title := func(label string, focus region, top, end, total int) string {
		marker := "  "
		if m.page == "" && m.focus == focus {
			marker = "> "
		}
		text := marker + label
		if top > 0 {
			text += " ↑"
		}
		if end < total {
			text += " ↓"
		}
		return text
	}
	options := m.spaceOptions()
	end := min(len(options), m.spacesTop+spacesHeight-1)
	lines := []string{title("spaces", spaces, m.spacesTop, end, len(options))}
	for index := m.spacesTop; index < end; index++ {
		marker := "  "
		if options[index] == m.spaceFilter {
			marker = "* "
		}
		lines = append(lines, marker+fleetclient.SpaceFilterHeading(options[index]))
	}
	for len(lines) < spacesHeight {
		lines = append(lines, "")
	}
	end = min(len(m.agents), m.agentsTop+agentsHeight-1)
	lines = append(lines, title("agents", agents, m.agentsTop, end, len(m.agents)))
	selected := m.agentIndex()
	for index := m.agentsTop; index < end; index++ {
		row := m.agents[index]
		marker := "  "
		if index == selected {
			marker = "* "
		}
		state := row.session.Agent.Status.State
		if !row.available {
			state = "unavailable"
		}
		suffix := " " + state
		name := ansi.Truncate(singleLine(row.session.Name), max(1, sidebarWidth-2-ansi.StringWidth(suffix)), "…")
		lines = append(lines, marker+name+suffix)
	}
	if len(m.agents) == 0 {
		lines = append(lines, "  no agents")
	}
	return lines
}

func (m *model) tabText(index, width int) string {
	name := ansi.Truncate(singleLine(m.rows[index].session.Name), max(4, (width-9)/2-3), "…")
	if index == m.cursor {
		return "[" + name + "] "
	}
	return " " + name + "  "
}

func (m *model) tabLine(width int) string {
	marker := "  tabs "
	if m.page == "" && m.focus == tabs {
		marker = "> tabs "
	}
	if len(m.rows) == 0 {
		return marker + "(none)"
	}
	if m.tabsTop > 0 {
		marker += "<"
	} else {
		marker += " "
	}
	for index := m.tabsTop; index < len(m.rows); index++ {
		text := m.tabText(index, width)
		if ansi.StringWidth(marker)+ansi.StringWidth(text) > width-1 {
			marker += ">"
			break
		}
		marker += text
	}
	return marker
}

func (m *model) mainLines(width, height int) []string {
	var lines []string
	focusStart, focusEnd := -1, -1
	add := func(text string) { lines = append(lines, wrapped(text, width)...) }
	switch m.page {
	case "confirm":
		add(m.pending.Operation + " " + capturedHeading(m.pendingName, m.pendingLabel, width-len(m.pending.Operation)-2) + "?")
		if m.pending.Operation == "stop" {
			add("attempt agent halt, then close this session. shared work may be affected.")
		} else {
			add("close this session. work shared through another session may survive.")
		}
	case "machine-picker":
		add(m.machineHeading())
		for index, option := range m.pickerOptions() {
			marker := "  "
			if index == m.picker {
				marker = "> "
				focusStart = len(lines)
			}
			add(marker + option)
			if index == m.picker {
				focusEnd = len(lines)
			}
		}
	case "space-edit":
		add("space for " + m.pendingName + " on " + m.pendingLabel)
		if m.spaceFailure != nil {
			add(m.spaceFailure.Code + " (unknown); not repeated")
		}
		current := m.pendingRow()
		if current != nil {
			add("current: " + fleetclient.SpaceHeading(current.session.Space))
		}
		focusStart = len(lines)
		add("> space: " + spaceDraftDisplay(m.spaceDraft))
		focusEnd = len(lines)
		label, err := space.ParseDraft(m.spaceDraft)
		switch {
		case m.spaceChecking:
			add("checking inventory; escape returns")
		case err != nil:
			add(space.ErrInvalid.Error())
		case current == nil || !current.available:
			add("session unavailable; save disabled")
		case current.session.Space == label:
			add("unchanged; save disabled")
		default:
			add("enter/ctrl-s saves")
		}
		add(m.spaceSuggestions())
	case "create":
		for index, label := range []string{"machine", "launch", "name", "directory", "space"} {
			marker := "  "
			if index == m.field {
				marker = "> "
				focusStart = len(lines)
			}
			text := m.form[index]
			if index == 4 {
				text = spaceDraftDisplay(text)
			}
			add(marker + label + ": " + text)
			if index == m.field {
				focusEnd = len(lines)
			}
		}
		if !m.createAvailable() {
			add("host unavailable; create disabled")
		}
		if _, err := space.ParseDraft(m.form[4]); err != nil {
			add(space.ErrInvalid.Error())
		}
		add(m.spaceSuggestions())
	case "output", "details":
		if m.page == "output" {
			// Identity and coverage stay pinned and leave room for the snapshot.
			header := []string{capturedHeading(m.outputName, m.outputMachine, width), cell(m.outputCoverage, width)}
			body := m.detailLines()
			offset := min(m.offset, max(0, len(body)-1))
			lines = append(header, body[offset:min(len(body), offset+max(0, height-len(header)))]...)
			return lines[:min(len(lines), height)]
		}
		lines = m.detailLines()
		offset := min(m.offset, max(0, len(lines)-1))
		return lines[offset:min(len(lines), offset+height)]
	default:
		row := m.selectedRow()
		if row == nil {
			partial := false
			for _, peer := range m.scopedPeers() {
				partial = partial || !peer.OK
			}
			switch {
			case !m.scopeReady:
				add("checking inventory")
			case partial:
				add("no matching sessions in available inventory")
			default:
				add("no sessions in this view")
			}
			break
		}
		session := row.session
		lines = []string{"session: " + session.Name, "machine: " + row.label, fleetclient.SpaceHeading(session.Space), "directory: " + session.CWD}
		if session.Agent == nil {
			lines = append(lines, "agent: none (shell)")
		} else {
			agent := session.Agent
			lines = append(lines, "provider: "+agent.Provider, "profile: "+agent.Profile, "state: "+agent.Status.State+" ("+agent.Status.Source+")", "reason: "+agent.Status.Reason)
		}
		lines = append(lines, fmt.Sprintf("attached clients: %d", session.AttachedClients))
		availability := "available"
		if !row.available {
			availability = "unavailable"
		}
		lines = append(lines, "availability: "+availability)
		for i, line := range lines {
			lines[i] = cell(line, width)
		}
	}
	start := 0
	if focusEnd > height {
		start = focusEnd - height
		// A long field keeps its label and current tail visible, without a second editor.
		if focusEnd-focusStart > height {
			return append([]string{lines[focusStart]}, lines[focusEnd-height+1:focusEnd]...)
		}
	}
	return lines[start:min(len(lines), start+height)]
}

func (m *model) machineHeading() string {
	if m.machine == "" {
		return "all machines"
	}
	return "machine: " + m.machine
}
func spaceDraftDisplay(draft string) string {
	if _, err := space.ParseDraft(draft); err != nil {
		return strconv.QuoteToASCII(draft)
	}
	return draft
}
func (m *model) spaceSuggestions() string {
	options := []string{}
	for _, label := range fleetclient.ObservedSpaces(m.scopedPeers()) {
		options = append(options, fleetclient.SpaceHeading(label))
	}
	return "observed spaces: " + strings.Join(options, " · ")
}
func (m *model) detailLines() []string {
	lines := []string{}
	for _, line := range m.text {
		lines = append(lines, strings.Split(ansi.Hardwrap(singleLine(line), max(1, m.width-sidebarWidth-1), true), "\n")...)
	}
	return lines
}
func wrapped(text string, width int) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = ansi.Wrap(singleLine(line), max(1, width), "")
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}
func cell(text string, width int) string {
	text = ansi.Truncate(singleLine(text), width, "…")
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}
