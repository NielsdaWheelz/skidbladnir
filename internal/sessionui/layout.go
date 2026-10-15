package sessionui

import (
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/charmbracelet/x/ansi"
)

// tableLayout measures the whole collection; scrolling never changes columns.
// Entries describe content, while viewport decides which gaps and headings fit.
type tableLayout struct {
	width, group, name, status, provider, profile, footer int
	stacked                                               bool
	entries                                               []tableEntry
}

type tableEntry struct {
	height            int
	heading, metadata bool
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
	layout := tableLayout{width: width, stacked: width+2 < 138}
	if m.view.kind != viewNeedsInput {
		layout.stacked = width+2 < 120
	}
	for index, row := range m.rows {
		printed, _ := m.tableStatus(row)
		layout.status = max(layout.status, ansi.StringWidth(attention.SingleLine(printed)))
		assigned := m.view.kind == viewNeedsInput && !row.session.Group.IsUnassigned()
		if assigned {
			layout.group = max(layout.group, 1, ansi.StringWidth(attention.SingleLine(row.session.Group.String())))
		}
		layout.name = max(layout.name, ansi.StringWidth(attention.SingleLine(row.session.Name)))
		provider, profile, footer := m.rowMetadata(row)
		layout.provider = max(layout.provider, ansi.StringWidth(attention.SingleLine(provider)))
		layout.profile = max(layout.profile, ansi.StringWidth(attention.SingleLine(profile)))
		layout.footer = max(layout.footer, ansi.StringWidth(attention.SingleLine(footer)))
		entry := tableEntry{
			height:   2,
			heading:  m.opensGroup(index),
			metadata: assigned || provider != "" || profile != "" || footer != "",
		}
		if layout.stacked && entry.metadata {
			entry.height++
		}
		layout.entries = append(layout.entries, entry)
	}
	if layout.group > 0 {
		layout.group = 3 + min(layout.group, 16)
	}
	layout.name = min(layout.name, 28)
	layout.profile = min(layout.profile, 11)
	layout.footer = min(layout.footer, 30)
	return layout
}

// viewport admits complete records in order. Only its first heading may be
// omitted; an interior group transition includes its gap, heading and record.
func (layout tableLayout) viewport(first, height int) tableViewport {
	viewport := tableViewport{end: first}
	for index := first; index < len(layout.entries); index++ {
		entry := layout.entries[index]
		placement := tablePlacement{index: index, heading: entry.heading}
		length := entry.height
		if index > first && (layout.stacked || entry.heading) {
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

func (m *model) rowMetadata(row listedRow) (provider, profile, footer string) {
	if agent := m.current(&row).Agent; agent != nil {
		provider, profile = agent.Provider, agent.Profile
		if profile == "" {
			profile = "unknown"
		}
		footer = m.rowFooter(row)
	}
	return
}

func rowDirectory(current fleetclient.ExecutionContext, width int) string {
	prefix := ""
	switch current.Kind {
	case "local":
	case "remote":
		prefix = "remote: "
	case "remoteUnknown":
		return "remote context unknown"
	default:
		panic("invalid execution context") // justify-defect: current returns one of three closed execution contexts.
	}
	if current.CWD == "" {
		return prefix + "directory unavailable"
	}
	return prefix + tail(attention.SingleLine(current.CWD), width-len(prefix))
}

func (m *model) fitViewports() {
	if m.width < 80 || m.height < 24 || m.page != "" || m.cursor < 0 {
		return
	}
	layout := m.tableLayout(m.width - 2)
	m.top = min(max(0, m.top), m.cursor)
	height := m.height - 2 - len(m.footerLines())
	if height < layout.entries[m.cursor].height {
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

// tableLines renders the admitted plan without making further fitting choices.
func (m *model) tableLines(layout tableLayout, viewport tableViewport) []string {
	lines := []string{}
	for _, placement := range viewport.entries {
		index := placement.index
		row := m.rows[index]
		if placement.gap {
			lines = append(lines, "")
		}
		if placement.heading {
			heading := ansi.Truncate(attention.SingleLine(fleetclient.GroupHeading(row.session.Group)), layout.width-1, "…") + ":"
			lines = append(lines, faint.Styled(heading))
		}
		gutter, nameStyle := "  ", plain
		if !row.available {
			nameStyle = faint
		}
		if index == m.cursor {
			gutter, nameStyle = here.Styled("▌")+" ", nameStyle.Bold()
		}
		name := ansi.Truncate(attention.SingleLine(row.session.Name), layout.name, "…")
		printed, statusStyle := m.tableStatus(row)
		line := gutter + nameStyle.Styled(name) + strings.Repeat(" ", max(0, layout.name-ansi.StringWidth(name)))
		line += "  " + statusStyle.Styled(cell(printed, layout.status))
		provider, profile, footer := m.rowMetadata(row)
		metadata := []string{}
		if layout.group > 0 {
			membership := strings.Repeat(" ", layout.group)
			if !row.session.Group.IsUnassigned() {
				membership = "in " + cell(row.session.Group.String(), layout.group-3)
			}
			metadata = append(metadata, membership)
		}
		if layout.provider > 0 {
			metadata = append(metadata, cell(provider, layout.provider))
		}
		if layout.profile > 0 {
			metadata = append(metadata, cell(profile, layout.profile))
		}
		if layout.footer > 0 {
			metadata = append(metadata, cell(footer, layout.footer))
		}
		text := strings.Join(metadata, "  ")
		if !layout.stacked && layout.entries[index].metadata {
			line += "  " + faint.Styled(text)
		}
		lines = append(lines, line)
		if layout.stacked && layout.entries[index].metadata {
			lines = append(lines, gutter+"  "+faint.Styled(text))
		}
		directoryStyle := plain
		if !row.available {
			directoryStyle = faint
		}
		lines = append(lines, gutter+"  "+directoryStyle.Styled(rowDirectory(m.current(&row), layout.width-4)))
	}
	return lines
}
