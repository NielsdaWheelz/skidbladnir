package sessionui

import (
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/charmbracelet/x/ansi"
)

// tableLayout measures the whole current collection. Spans include an optional
// heading and a complete entry; top always names a session, never a text line.
type tableLayout struct {
	entryHeight, group, name, status, provider, profile, footer int
	spans                                                       []tableSpan
}

type tableSpan struct{ start, first, end int }

func (m *model) tableLayout(width int) tableLayout {
	layout := tableLayout{entryHeight: 2}
	if m.width >= 135 || (m.width >= 120 && m.view.kind != viewNeedsInput) {
		layout.entryHeight = 1
	}
	line := 0
	for index, row := range m.rows {
		span := tableSpan{start: line, first: line}
		if m.opensGroup(index) {
			span.first++
		}
		span.end = span.first + layout.entryHeight
		layout.spans = append(layout.spans, span)
		line = span.end

		printed, _ := m.tableStatus(row)
		layout.status = max(layout.status, ansi.StringWidth(attention.SingleLine(printed)))
		if m.view.kind == viewNeedsInput && !row.session.Group.IsUnassigned() {
			layout.group = max(layout.group, 1, ansi.StringWidth(attention.SingleLine(row.session.Group.String())))
		}
		current := m.current(&row)
		nameWidth := ansi.StringWidth(attention.SingleLine(row.session.Name))
		if current.Kind == "remote" {
			nameWidth += 4 + min(12, ansi.StringWidth(attention.SingleLine(current.Label)))
		}
		layout.name = max(layout.name, nameWidth)
		provider, profile, footer := m.rowMetadata(row)
		layout.provider = max(layout.provider, ansi.StringWidth(attention.SingleLine(provider)))
		layout.profile = max(layout.profile, ansi.StringWidth(attention.SingleLine(profile)))
		layout.footer = max(layout.footer, ansi.StringWidth(attention.SingleLine(footer)))
		if current.Kind == "remoteUnknown" {
			layout.provider, layout.profile = max(layout.provider, 6), max(layout.profile, 6)
		}
	}
	layout.group = min(layout.group, 16)
	layout.name = min(layout.name, 28)
	layout.footer = min(layout.footer, 30)
	if m.view.kind == viewNeedsInput && layout.entryHeight == 2 {
		room := width - 2 - layout.status - 2
		if layout.group > 0 {
			room -= layout.group + 2
		}
		layout.name = min(layout.name, room)
	}
	return layout
}

// entryLines omits a heading when only its complete entry fits. A continuation
// or a heading without its entry never enters the viewport.
func (layout tableLayout) entryLines(index, remaining int) int {
	if remaining < layout.entryHeight {
		return 0
	}
	span := layout.spans[index]
	return min(remaining, span.end-span.start)
}

func (layout tableLayout) end(first, height int) int {
	end := first
	for end < len(layout.spans) {
		length := layout.entryLines(end, height)
		if length == 0 {
			break
		}
		height -= length
		end++
	}
	return end
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

func (m *model) fitViewports() {
	if m.width < 80 || m.height < 24 || m.page != "" || m.cursor < 0 {
		return
	}
	layout := m.tableLayout(m.width - 2)
	height := m.height - 2 - len(m.footerLines())
	m.top = min(max(0, m.top), m.cursor)
	if height < layout.entryHeight {
		return
	}
	if layout.end(0, height) == len(m.rows) {
		m.top = 0
		return
	}
	if m.cursor < layout.end(m.top, height) {
		return
	}
	// Pack backward from the selected session. Its heading may be omitted at
	// the bottom, exactly as entryLines permits while rendering forward.
	limit := height + layout.spans[m.cursor].first - layout.spans[m.cursor].start
	length := 0
	for index := m.cursor; index >= 0; index-- {
		span := layout.spans[index]
		length += span.end - span.start
		if length > limit {
			break
		}
		m.top = index
	}
}

// tableLines renders the whole measured collection. The viewport selects
// complete spans afterward; cursor and scrolling never change column widths.
func (m *model) tableLines(width int) []string {
	layout := m.tableLayout(width)
	lines := []string{}
	for index, row := range m.rows {
		if m.opensGroup(index) {
			lines = append(lines, "  "+faint.Styled(ansi.Truncate(attention.SingleLine(fleetclient.GroupHeading(row.session.Group)), width-2, "…")))
		}
		gutter, nameStyle := "  ", plain
		if !row.available {
			nameStyle = faint
		}
		if index == m.cursor {
			gutter, nameStyle = here.Styled("▌")+" ", nameStyle.Bold()
		}
		current := m.current(&row)
		name, qualifier := attention.SingleLine(row.session.Name), ""
		if current.Kind == "remote" {
			budget := min(12, layout.name-4-min(12, ansi.StringWidth(name)))
			qualifier = " on " + ansi.Truncate(attention.SingleLine(current.Label), budget, "…")
		}
		name = ansi.Truncate(name, layout.name-ansi.StringWidth(qualifier), "…")
		line, axis := gutter, 2
		if layout.group > 0 {
			line += faint.Styled(cell(row.session.Group.String(), layout.group)) + "  "
			axis += layout.group + 2
		}
		printed, statusStyle := m.tableStatus(row)
		line += nameStyle.Styled(name) + faint.Styled(qualifier) + strings.Repeat(" ", max(0, layout.name-ansi.StringWidth(name)-ansi.StringWidth(qualifier)))
		line += "  " + statusStyle.Styled(cell(printed, layout.status))
		provider, profile, footer := m.rowMetadata(row)
		metadata := []string{}
		if current.Kind == "remoteUnknown" {
			metadata = append(metadata, cell("remote unknown", layout.provider+2+layout.profile))
		} else {
			if layout.provider > 0 {
				metadata = append(metadata, cell(provider, layout.provider))
			}
			if layout.profile > 0 {
				metadata = append(metadata, cell(profile, layout.profile))
			}
		}
		if layout.footer > 0 {
			metadata = append(metadata, cell(footer, layout.footer))
		}
		if layout.entryHeight == 1 {
			if len(metadata) > 0 {
				line += "  " + faint.Styled(strings.Join(metadata, "  "))
			}
			lines = append(lines, line)
		} else {
			lines = append(lines, line, strings.Repeat(" ", axis)+faint.Styled(strings.Join(metadata, "  ")))
		}
	}
	return lines
}
