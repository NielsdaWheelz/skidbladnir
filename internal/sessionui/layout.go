package sessionui

import (
	"fmt"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/charmbracelet/x/ansi"
)

const (
	nameColumn = iota
	groupColumn
	providerColumn
	profileColumn
	modelColumn
	machineColumn
	pathColumn
	columnCount

	rowNameStart = 5
	unknownPlace = "remote context unknown"
	unknownPath  = "directory unavailable"
)

type tableColumns struct {
	width [columnCount]int
	start [columnCount]int
	last  int
}

type tablePlacement struct {
	index, line, heading int
}

type frame struct {
	width, body, top int
	columns          tableColumns
	rows             []tablePlacement
	foldAt           int
	fold             []line
	notices, band    []line
	rule, hints      line
}

func (m *model) frame() frame {
	f := frame{width: m.width - 2, top: m.top}
	width := f.width - 3
	f.body = m.height - 4
	if m.fold.loss != "" {
		f.fold = m.foldLines(f.body-1, width)
	}
	reserved := len(f.fold)
	f.notices, f.hints = m.noticeLines(f.width), m.keyHints(f.width)
	// Leave room for the disabled prompt and the selected row or empty state.
	f.notices = f.notices[:min(len(f.notices), f.body-reserved-2)]
	f.body -= len(f.notices)
	if m.band {
		f.band = m.usageBand((f.body - reserved) / 2)
		f.body -= len(f.band)
	}
	tableHeight := f.body - reserved
	f.columns = m.measureColumns(f.width)
	if m.cursor >= 0 {
		f.top = min(max(0, f.top, m.cursor-tableHeight+1), m.cursor)
		headings := m.tableHeadings()
		for {
			f.rows = tableViewport(f.top, tableHeight, headings)
			if f.rows[len(f.rows)-1].index >= m.cursor {
				break
			}
			f.top++
		}
	}
	if reserved != 0 {
		f.foldAt = tableHeight
	} else if m.fold.foot() {
		f.composeFoot(m.foldLines(f.body*11/20, width))
	} else if m.fold.kind != foldNone && m.cursor >= 0 {
		f.composeInline(m.cursor, m.foldLines(max(1, min(16, (f.body-1)/2)), width))
	}
	var caption line
	switch {
	case m.fold.kind == foldTerminal:
		caption = line{{text: "new terminal on " + ansi.Truncate(attention.SingleLine(m.fold.request.Machine), f.width-21, "…")}}
	case m.fold.kind == foldDraft && !m.fold.seeded:
		caption = line{{text: "new session on " + ansi.Truncate(attention.SingleLine(m.form.machine), f.width-20, "…")}}
	case m.cursor >= 0:
		caption = m.tableCaption(f, len(f.rows) < len(m.rows))
	}
	var quota line
	if len(f.band) == 0 {
		quota = m.weeklyQuota(f.width - caption.width() - 6)
	}
	f.rule = rule(caption, quota, f.width)
	return f
}

func (m *model) tableHeadings() []bool {
	headings := make([]bool, len(m.rows))
	if m.view.kind != viewAll {
		return headings
	}
	for index, row := range m.rows {
		headings[index] = index == 0 || m.rows[index-1].Session.Group != row.Session.Group
	}
	return headings
}

// tableViewport puts each heading on the line before the first session. The
// selected row's line is an anchor for inline folds, independent of their size.
func tableViewport(first, height int, headings []bool) []tablePlacement {
	var rows []tablePlacement
	y := 0
	for index := first; index < len(headings); index++ {
		heading := -1
		rowLine := y
		if headings[index] && (index != first || height > 1) {
			rowLine++
			if index != first {
				rowLine++
			}
			heading = rowLine - 1
		}
		if rowLine >= height {
			break
		}
		rows = append(rows, tablePlacement{index: index, line: rowLine, heading: heading})
		y = rowLine + 1
	}
	return rows
}

func (m *model) measureColumns(width int) tableColumns {
	layout := tableColumns{}
	unknown := false
	for _, peer := range m.peers {
		for _, session := range peer.Sessions {
			row := peer.Row(session)
			values := m.rowCells(row)
			for column := nameColumn; column < machineColumn; column++ {
				layout.width[column] = max(layout.width[column], ansi.StringWidth(values[column]))
			}
			machine, _ := executionPlace(m.current(&row))
			if machine == unknownPlace {
				unknown = true
			} else {
				layout.width[machineColumn] = max(layout.width[machineColumn], ansi.StringWidth(machine))
			}
		}
	}
	layout.width[nameColumn] = min(28, max(1, layout.width[nameColumn]))
	layout.width[groupColumn] = min(16, layout.width[groupColumn])
	used := rowNameStart
	for column := nameColumn; column < machineColumn; column++ {
		cells := layout.width[column]
		if cells == 0 {
			continue
		}
		if column != nameColumn {
			used += 2
		}
		if used+cells > width {
			return layout
		}
		layout.start[column] = used
		layout.last = column
		used += cells
	}
	place := layout.width[machineColumn] + 2 + 8
	if unknown {
		place = max(place, len(unknownPlace))
	}
	if used+2+place <= width {
		layout.last = pathColumn
		layout.start[machineColumn] = used + 2
		layout.start[pathColumn] = layout.start[machineColumn] + layout.width[machineColumn] + 2
		layout.width[pathColumn] = width - layout.start[pathColumn]
	}
	return layout
}

func (m *model) rowCells(row fleetclient.Row) [machineColumn]string {
	cells := [machineColumn]string{nameColumn: row.Session.Name}
	if m.view.kind == viewNeedsInput {
		cells[groupColumn] = fleetview.GroupHeading(row.Session.Group)
	}
	if agent := m.current(&row).Agent; agent != nil {
		cells[providerColumn], cells[profileColumn], cells[modelColumn] = strings.ToLower(agent.Provider), agent.Profile, m.rowFooter(row)
		if cells[profileColumn] == "" {
			cells[profileColumn] = "unknown"
		}
	}
	for i, text := range cells {
		cells[i] = attention.SingleLine(text)
	}
	return cells
}

func executionPlace(current fleetclient.ExecutionContext) (machine, path string) {
	label := attention.SingleLine(current.Label)
	switch current.Kind {
	case "local":
		machine = ansi.Truncate(label, 16, "…")
	case "remote":
		machine = ansi.Truncate(label, 7, "…") + " (remote)"
	case "remoteUnknown":
		return unknownPlace, ""
	default:
		panic("invalid execution context")
	}
	path = attention.SingleLine(current.CWD)
	if path == "" {
		path = unknownPath
	}
	return machine, path
}

func shortPath(path string, width int) string {
	if path == unknownPath {
		return ansi.Truncate(path, width, "")
	}
	end := strings.TrimRight(path, "/")
	last := strings.LastIndex(end, "/") + 1
	first := strings.LastIndex(strings.TrimRight(end[:last], "/"), "/") + 1
	two := path
	if strings.Trim(path[:first], "/") != "" {
		two = "…/" + path[first:]
	}
	if ansi.StringWidth(two) <= width {
		return two
	}
	return "…/" + ansi.Truncate(path[last:], width-2, "")
}

// the browser lays out nothing below 80 × 24.
func (m *model) tooSmall() bool { return m.width < 80 || m.height < 24 }

func (m *model) tableLines(f frame) []line {
	lines := make([]line, f.body)
	for _, placement := range f.rows {
		index := placement.index
		row := m.rows[index]
		if placement.heading >= 0 {
			heading := ansi.Truncate(attention.SingleLine(fleetview.GroupHeading(row.Session.Group)), f.width, "…")
			role := inkRole
			if row.Session.Group.IsUnassigned() {
				role = quietRole
			}
			lines[placement.heading] = line{{text: heading, role: role, bold: true}}
		}
		selected := index == m.cursor
		status := m.statusView(row)
		sign := signSpan(status.Sign)
		if status.WorkContinues {
			sign.under, sign.underRole = ansi.UnderlineDouble, frostRole
		}
		stale := !m.rowAttachable(row)
		cells := m.rowCells(row)
		lip := span{text: " "}
		if selected {
			lip = span{text: "▌", role: goldRole}
		}
		finished := line{lip, {text: " "}, sign, {text: "  "}}
		used := rowNameStart
		for column := nameColumn; column <= min(f.columns.last, modelColumn); column++ {
			text := cells[column]
			if f.columns.width[column] == 0 || text == "" || column > nameColumn && stale {
				continue
			}
			item := span{text: text, role: quietRole}
			switch column {
			case nameColumn:
				item.text = ansi.Truncate(text, f.columns.width[column], "…")
				item.bold = selected || fleetview.QueueRank(status.Sign) >= 0
				if !stale {
					item.role = inkRole
				}
			case groupColumn:
				item.text = ansi.Truncate(text, f.columns.width[column], "…")
			}
			start := f.columns.start[column]
			finished = append(finished, span{text: strings.Repeat(" ", start-used)}, item)
			used = start + ansi.StringWidth(item.text)
		}
		if start := f.columns.start[machineColumn]; f.columns.last == pathColumn && start >= used+2 {
			machine, path := executionPlace(m.current(&row))
			machineWidth := ansi.StringWidth(machine)
			if path == "" && machineWidth <= f.width-start || path != "" && machineWidth <= f.columns.width[machineColumn] {
				finished = append(finished, span{text: strings.Repeat(" ", start-used)}, span{text: machine, role: quietRole})
				if path != "" {
					finished = append(finished, span{text: strings.Repeat(" ", f.columns.start[pathColumn]-start-machineWidth)},
						span{text: shortPath(path, f.columns.width[pathColumn]), role: quietRole})
				}
			}
		}
		if selected {
			finished = finished.filled(m.ground.raise, f.width)
		}
		lines[placement.line] = finished
	}
	return lines
}

func (m *model) tableCaption(f frame, hidden bool) line {
	row := m.selectedRow()
	view := m.statusView(*row)
	label := span{text: view.Label, role: signSpan(view.Sign).role, italic: !m.rowAttachable(*row)}
	status := line{label}
	if m.rowChecking(*row) {
		status = append(status, span{text: " · checking", role: mutedRole, italic: true})
	}
	if m.guest {
		note := "left the queue"
		if m.view.kind == viewGroup {
			note = "left this group"
		}
		status = append(status, span{text: " · " + note, role: mutedRole, italic: true})
	}
	width := f.width - 5
	base := target(row.Session.Name, row.Label, width-3-status.width())
	caption := append(line{{text: base}, {text: "   "}}, status...)
	used := caption.width()
	if detail := " · " + view.Detail; view.Detail != "" && used+ansi.StringWidth(detail) <= width {
		caption = append(caption, span{text: detail, role: mutedRole})
		used += ansi.StringWidth(detail)
	}
	if f.columns.last != pathColumn {
		context := m.current(row)
		machine, path := executionPlace(context)
		place := machine
		room := width - used - 3
		if path != "" {
			place = ""
			if context.Kind == "remote" {
				place = machine + "  "
			}
			room -= ansi.StringWidth(place)
			if room >= 8 || ansi.StringWidth(path) <= room {
				place += shortPath(path, room)
			}
		}
		if place = strings.TrimSpace(place); place != "" && used+3+ansi.StringWidth(place) <= width {
			caption = append(caption, span{text: "   " + place, role: quietRole})
			used += 3 + ansi.StringWidth(place)
		}
	}
	if hidden {
		position := fmt.Sprintf("   %d of %d", m.cursor+1, len(m.rows))
		if used+len(position) <= width {
			caption = append(caption, span{text: position, role: quietRole})
		}
	}
	return caption
}
