package sessionui

import (
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/charmbracelet/x/ansi"
)

// tableLayout measures names across the whole collection; scrolling never
// changes their width. viewport decides which gaps and headings fit.
type tableLayout struct {
	width, name int
	headings    []bool
}

type tablePlacement struct {
	index        int
	gap, heading bool
}

type tableViewport struct {
	entries []tablePlacement
	end     int
}

func (m *model) tableLayout(width int) tableLayout {
	layout := tableLayout{width: width, headings: make([]bool, len(m.rows))}
	for index, row := range m.rows {
		layout.name = max(layout.name, ansi.StringWidth(attention.SingleLine(row.Session.Name)))
		layout.headings[index] = m.opensGroup(index)
	}
	layout.name = min(layout.name, 28)
	return layout
}

// viewport admits complete records in order. Only its first heading may be
// omitted; an interior group transition includes its gap, heading and record.
func (layout tableLayout) viewport(first, height int) tableViewport {
	viewport := tableViewport{end: first}
	for index := first; index < len(layout.headings); index++ {
		placement := tablePlacement{index: index, heading: layout.headings[index]}
		length := 1
		if index > first && placement.heading {
			placement.gap = true
			length++
		}
		if placement.heading {
			if index == first && length+1 > height {
				placement.heading = false
			} else {
				length++
			}
		}
		if length > height {
			break
		}
		viewport.entries = append(viewport.entries, placement)
		viewport.end = index + 1
		height -= length
	}
	return viewport
}

func (m *model) rowMetadata(row fleetclient.Row) (provider, profile, footer string) {
	if agent := m.current(&row).Agent; agent != nil {
		provider, profile = agent.Provider, agent.Profile
		if profile == "" {
			profile = "unknown"
		}
		footer = m.rowFooter(row)
	}
	return
}

func rowDirectory(current fleetclient.ExecutionContext, width int) (string, bool) {
	machine := ansi.Truncate(attention.SingleLine(current.Label), 16, "…")
	switch current.Kind {
	case "local":
	case "remote":
		machine += " (remote)"
	case "remoteUnknown":
		text := "remote context unknown"
		if ansi.StringWidth(text) > width {
			return "", false
		}
		return text, true
	default:
		panic("invalid execution context") // justify-defect: current returns one of three closed execution contexts.
	}
	prefix := machine + " · "
	if current.CWD == "" {
		text := prefix + "directory unavailable"
		if ansi.StringWidth(text) > width {
			return "", false
		}
		return text, true
	}
	path := attention.SingleLine(current.CWD)
	// Keep two final components without resolving or normalizing the path.
	start := len(path)
	for range 2 {
		start = strings.LastIndex(strings.TrimRight(path[:start], "/"), "/") + 1
	}
	if strings.Trim(path[:start], "/") != "" {
		path = "…/" + path[start:]
	}
	suffixWidth := width - ansi.StringWidth(prefix)
	if suffixWidth < 1 || suffixWidth < min(8, ansi.StringWidth(path)) {
		return "", false
	}
	return prefix + tail(path, suffixWidth), true
}

// the browser lays out nothing below 80 × 24.
func (m *model) tooSmall() bool { return m.width < 80 || m.height < 24 }

func (m *model) fitViewports() {
	if m.tooSmall() || m.page != "" || m.cursor < 0 {
		return
	}
	layout := m.tableLayout(m.width - 2)
	m.top = min(max(0, m.top), m.cursor)
	height := m.height - 2 - len(m.footerLines())
	if height < 1 {
		m.top = m.cursor
		return
	}
	if layout.viewport(0, height).end == len(m.rows) {
		m.top = 0
		return
	}
	end := layout.viewport(m.top, height).end
	anchor := m.cursor
	if end == len(m.rows) {
		anchor = end - 1
	} else if m.cursor < end {
		return
	}
	// Choose the earliest top whose actual forward plan includes the anchor;
	// group boundaries and the optional first heading need no backward credits.
	for first := 0; first <= m.cursor; first++ {
		if layout.viewport(first, height).end > anchor {
			m.top = first
			return
		}
	}
}

// tableLines packs each admitted row without changing the viewport.
func (m *model) tableLines(layout tableLayout, viewport tableViewport) []string {
	lines := []string{}
	for _, placement := range viewport.entries {
		index := placement.index
		row := m.rows[index]
		if placement.gap {
			lines = append(lines, "")
		}
		if placement.heading {
			heading := ansi.Truncate(attention.SingleLine(fleetclient.GroupHeading(row.Session.Group)), layout.width-1, "…") + ":"
			lines = append(lines, faint.Styled(heading))
		}
		gutter, nameStyle := "  ", plain
		if !row.Available {
			nameStyle = faint
		}
		if index == m.cursor {
			gutter, nameStyle = here.Styled("▌")+" ", nameStyle.Bold()
		}
		name := attention.SingleLine(row.Session.Name)
		name = ansi.Truncate(name, layout.name, "…")
		printed, statusStyle := m.tableStatus(row)
		printed = attention.SingleLine(printed)
		line := gutter + nameStyle.Styled(name) + strings.Repeat(" ", max(0, layout.name-ansi.StringWidth(name)))
		line += "  " + statusStyle.Styled(printed)
		used := 2 + layout.name + 2 + ansi.StringWidth(printed)
		provider, profile, footer := m.rowMetadata(row)
		metadata := []string{}
		if m.view.kind == viewNeedsInput && !row.Session.Group.IsUnassigned() {
			metadata = append(metadata, "in "+ansi.Truncate(attention.SingleLine(row.Session.Group.String()), 16, "…"))
		}
		for _, text := range []string{provider, profile, footer} {
			if text != "" {
				metadata = append(metadata, attention.SingleLine(text))
			}
		}
		completeWidth := used
		for _, text := range metadata {
			completeWidth += 2 + ansi.StringWidth(text)
		}
		directory, fits := rowDirectory(m.current(&row), layout.width-completeWidth-2)
		limit := layout.width
		if !fits {
			limit -= 3
		}
		for _, text := range metadata {
			next := used + 2 + ansi.StringWidth(text)
			if next > limit {
				break
			}
			line += "  " + faint.Styled(text)
			used = next
		}
		if fits {
			line += strings.Repeat(" ", layout.width-used-ansi.StringWidth(directory)) + faint.Styled(directory)
		} else {
			line += strings.Repeat(" ", layout.width-used-3) + faint.Styled("  …")
		}
		lines = append(lines, line)
	}
	return lines
}
