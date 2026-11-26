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
	groupColumn = iota
	profileColumn
	modelColumn
	placeColumn
	columnCount

	rowNameStart = 4 // lip, sign, branch, blank
	unknownPlace = "remote context unknown"
	unknownPath  = "directory unavailable"
)

type tableColumns struct {
	name  int
	width [columnCount]int
	start [columnCount]int // screen column, or zero for a lane that did not fit
}

type tablePlacement struct {
	index, line, heading int
}

type frame struct {
	width, rowWidth, body, top      int
	strip, blank, caption, hintLine bool
	columns                         tableColumns
	rows                            []tablePlacement
	foldAt                          int
	fold                            []line
	notices, band                   []line
	rule, hints                     line
}

func (f frame) bandLimit() int { return (f.body - len(f.fold)) / 2 }

// chrome assigns room before laying out any action or key hint.
func (m *model) chrome() frame {
	f := frame{width: m.width, rowWidth: m.width, body: m.height, top: m.top}
	if m.width >= 4 {
		f.width -= 2
		f.rowWidth--
	}
	f.strip = m.height >= 3 && m.width >= 6
	f.blank = m.height >= 12 && f.strip
	f.caption = m.height >= 2
	f.hintLine = m.height >= 8
	for _, drawn := range [...]bool{f.strip, f.blank, f.caption, f.hintLine} {
		if drawn {
			f.body--
		}
	}
	if m.fold.loss != "" && f.body > 0 {
		// A disabled fold occupies even the last body line.
		f.fold = m.foldLines(max(1, f.body-1), f.rowWidth)
	}
	if limit := f.body - len(f.fold) - 2; limit > 0 && f.width > 0 {
		f.notices = m.noticeLines(f.width)
		f.notices = f.notices[:min(limit, len(f.notices))]
		f.body -= len(f.notices)
	}
	if m.band {
		f.band = m.usageBand(f.bandLimit(), f.width)
		f.body -= len(f.band)
	}
	return f
}

func (m *model) frame() frame {
	f := m.chrome()
	reserved := len(f.fold)
	tableHeight := f.body - reserved
	f.columns = m.measureColumns(max(0, f.rowWidth-rowNameStart))
	if m.cursor >= 0 && tableHeight > 0 {
		f.top = min(max(0, f.top, m.cursor-tableHeight+1), m.cursor)
		headings := m.tableHeadings()
		for {
			f.rows = tableViewport(f.top, tableHeight, headings, m.height >= 24)
			if f.rows[len(f.rows)-1].index >= m.cursor {
				break
			}
			f.top++
		}
	}
	if reserved != 0 {
		f.foldAt = tableHeight
	} else if m.fold.kind != foldNone && (m.fold.foot() || m.cursor >= 0) {
		lines := m.foldLines(m.fold.capacity(f.body), f.rowWidth)
		if m.fold.foot() {
			f.composeFoot(lines)
		} else {
			f.composeInline(m.cursor, lines)
		}
	}
	if f.hintLine {
		f.hints = m.keyHints(f.width)
	}
	if !f.caption {
		return f
	}
	room := f.width
	if f.width >= 12 {
		room -= 5
	}
	var caption line
	switch {
	case m.notice != "" && len(f.notices) == 0:
		role := inkRole
		if m.noticeFailure {
			role = emberRole
		}
		caption = line{{text: ansi.Truncate(attention.SingleLine(m.notice), room, "…"), role: role}}
	case m.fold.kind == foldTerminal:
		caption = line{{text: ansi.Truncate("new terminal on "+attention.SingleLine(m.fold.request.Machine), room, "…")}}
	case m.fold.kind == foldDraft && !m.fold.seeded:
		caption = line{{text: ansi.Truncate("new session on "+attention.SingleLine(m.form.machine), room, "…")}}
	case m.cursor >= 0:
		caption = m.tableCaption(f, len(f.rows) < len(m.rows), room)
	}
	if f.width < 12 {
		f.rule = caption
		return f
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
	if m.view.kind != viewAll || m.width < 14 {
		return headings
	}
	for index, row := range m.rows {
		headings[index] = index == 0 || m.tree[m.rows[index-1].Session.Ref.SessionRef()].heading != m.tree[row.Session.Ref.SessionRef()].heading
	}
	return headings
}

// tableViewport puts each heading on the line before the first session. The
// selected row's line is an anchor for inline folds, independent of their size.
func tableViewport(first, height int, headings []bool, gap bool) []tablePlacement {
	var rows []tablePlacement
	y := 0
	for index := first; index < len(headings); index++ {
		heading := -1
		rowLine := y
		if headings[index] && (index != first || height > 1) {
			rowLine++
			if index != first && gap {
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

// measureColumns measures retained fleet inventory and configured profiles. Room
// is what remains after the spine and margin. An admitted place takes the rest.
func (m *model) measureColumns(room int) tableColumns {
	columns := tableColumns{width: [columnCount]int{profileColumn: len("unknown"), modelColumn: 20, placeColumn: 24}}
	target := 0
	for _, peer := range m.peers {
		for _, profile := range peer.Profiles {
			width := ansi.StringWidth(attention.SingleLine(profile.Key))
			columns.width[profileColumn] = max(columns.width[profileColumn], width)
		}
		for _, session := range peer.Sessions {
			node := m.tree[session.Ref.SessionRef()]
			target = max(target, node.depth*2+ansi.StringWidth(attention.SingleLine(node.name())))
			width := ansi.StringWidth(attention.SingleLine(fleetview.GroupHeading(session.Group)))
			columns.width[groupColumn] = max(columns.width[groupColumn], width)
		}
	}
	target = min(target, 40)
	columns.width[groupColumn] = min(columns.width[groupColumn], 16)
	columns.width[profileColumn] = min(columns.width[profileColumn], 16)

	first := profileColumn
	if m.view.kind == viewNeedsInput {
		first = groupColumn
	}
	used := 0
	for column := first; column < columnCount; column++ {
		keep := min(target, 32)
		if column == placeColumn {
			keep = target
		}
		if room-used-2-columns.width[column] < keep {
			break
		}
		columns.start[column] = used + 2
		used += 2 + columns.width[column]
	}
	columns.name = max(0, room)
	if used > 0 {
		columns.name = min(target, room-used)
	}
	for column, start := range columns.start {
		if start != 0 {
			columns.start[column] = rowNameStart + columns.name + start
		}
	}
	if start := columns.start[placeColumn]; start != 0 {
		columns.width[placeColumn] = rowNameStart + room - start
	}
	return columns
}

func (m *model) rowCells(row fleetclient.Row) [placeColumn]string {
	var cells [placeColumn]string
	if !m.rowAttachable(row) {
		return cells
	}
	cells[groupColumn] = attention.SingleLine(fleetview.GroupHeading(row.Session.Group))
	if agent := m.current(&row).Agent; agent != nil {
		cells[profileColumn] = attention.SingleLine(agent.Profile)
		if cells[profileColumn] == "" {
			cells[profileColumn] = "unknown"
		}
		cells[modelColumn] = attention.SingleLine(m.rowFooter(row))
	}
	return cells
}

// place returns a host prefix only when the work runs off this machine. An
// unresolved context has neither a host nor a directory.
func (m *model) place(row fleetclient.Row) (host, directory string) {
	current := m.current(&row)
	if current.Kind == "remoteUnknown" {
		return "", ""
	}
	if current.Machine != m.client.Self() {
		host = ansi.Truncate(attention.SingleLine(current.Label), 12, "…") + ":"
	}
	if current.CWD == "" {
		return host, unknownPath
	}
	cwd := strings.TrimRight(attention.SingleLine(current.CWD), "/")
	return host, cwd[strings.LastIndex(cwd, "/")+1:] + "/"
}

// fitPlace keeps the host and the directory's tail, or drops the place when
// there is no room for both.
func fitPlace(host, directory string, width int) string {
	if directory == "" {
		directory = unknownPlace
	}
	if ansi.StringWidth(host+directory) <= width {
		return host + directory
	}
	room := width - ansi.StringWidth(host) - 1
	if room < 1 {
		return ""
	}
	return host + "…" + tail(directory, room)
}

// tail drops whole graphemes from the head until what remains fits.
func tail(text string, cells int) string {
	for remove := ansi.StringWidth(text) - cells; remove > 0; {
		cluster, width := ansi.FirstGraphemeCluster(text, ansi.GraphemeWidth)
		text = text[len(cluster):]
		remove -= width
	}
	return text
}

func (node *treeNode) name() string {
	name := node.row.Session.Name
	if node.row.Session.Parent != nil {
		name += " · " + node.row.Label
		if node.context != "" {
			name += " · " + node.context
		}
	}
	return name
}

func cutName(name, mode string, room int) string {
	if ansi.StringWidth(name) <= room {
		return name
	}
	if mode == "automatic" || room < 3 {
		return ansi.Truncate(name, room, "…")
	}
	return ansi.Truncate(name, room/2, "") + "…" + tail(name, (room-1)/2)
}

func (m *model) tableLines(f frame) []line {
	lines := make([]line, f.body)
	if m.width == 0 {
		return lines
	}
	if m.width >= 4 {
		for i := range lines {
			lines[i] = line{{text: " "}}
		}
	}
	for _, placement := range f.rows {
		index := placement.index
		row := m.rows[index]
		node := m.tree[row.Session.Ref.SessionRef()]
		if placement.heading >= 0 {
			heading := ansi.Truncate(attention.SingleLine(fleetview.GroupHeading(node.heading)), f.width, "…")
			role := inkRole
			if node.heading.IsUnassigned() {
				role = quietRole
			}
			lines[placement.heading] = line{{text: " "}, {text: heading, role: role, bold: true}}
		}
		selected := index == m.cursor
		status := m.statusView(row)
		sign := signSpan(status.Sign)
		if status.WorkContinues {
			sign.under, sign.underRole = ansi.UnderlineDouble, frostRole
		}
		if m.width == 1 {
			lines[placement.line] = line{sign}
			continue
		}
		lip := span{text: " "}
		if selected {
			lip = span{text: "▌", role: goldRole}
		}
		finished := line{lip, sign}
		if m.width >= 3 {
			branch := " "
			if m.view.kind != viewNeedsInput && len(node.children) > 0 {
				branch = "▸"
				if m.expanded[row.Session.Ref.SessionRef()] {
					branch = "▾"
				}
			}
			finished = append(finished, span{text: branch, role: mutedRole})
		}
		if m.width >= 8 {
			indent := min(node.depth*2, max(0, f.columns.name-3))
			name := span{
				text: strings.Repeat(" ", indent) + cutName(attention.SingleLine(node.name()), row.Session.NameMode, f.columns.name-indent),
				role: inkRole,
				bold: selected || m.queued(row),
			}
			if !m.rowAttachable(row) {
				name.role = quietRole
			}
			finished = append(finished, span{text: " "}, name)
			used := rowNameStart + ansi.StringWidth(name.text)
			cells := m.rowCells(row)
			for column, start := range f.columns.start {
				if start == 0 {
					continue
				}
				var text string
				if column == placeColumn {
					host, directory := m.place(row)
					text = fitPlace(host, directory, f.columns.width[column])
				} else {
					text = ansi.Truncate(cells[column], f.columns.width[column], "…")
				}
				if text != "" {
					finished = append(finished, span{text: strings.Repeat(" ", max(0, start-used))}, span{text: text, role: quietRole})
					used = start + ansi.StringWidth(text)
				}
			}
		}
		if selected && m.width >= 4 {
			finished = finished.filled(m.ground.raise, f.rowWidth)
		}
		lines[placement.line] = finished
	}
	return lines
}

func (m *model) tableCaption(f frame, hidden bool, room int) line {
	row := m.selectedRow()
	view := m.statusView(*row)
	name := attention.SingleLine(row.Session.Name)
	caption := line{
		{text: name},
		{text: " on " + attention.SingleLine(row.Label)},
		{text: "   "},
		{text: attention.SingleLine(view.Label), role: signSpan(view.Sign).role, italic: !m.rowAttachable(*row)},
	}
	if m.rowChecking(*row) {
		caption = append(caption, span{text: " · checking", role: mutedRole, italic: true})
	}
	if m.guest {
		note := "left the queue"
		if m.view.kind == viewGroup {
			note = "left this group"
		}
		caption = append(caption, span{text: " · " + note, role: mutedRole, italic: true})
	}
	if detail := attention.SingleLine(view.Detail); detail != "" {
		caption = append(caption, span{text: " · " + detail, role: mutedRole})
	}

	statusEnd := len(caption)
	for column, fact := range m.rowCells(*row) {
		if f.columns.start[column] == 0 && (column != groupColumn || m.view.kind == viewNeedsInput) && fact != "" && fact != "—" {
			caption = append(caption, span{text: "   " + fact, role: quietRole})
		}
	}
	placeAt := -1
	var host, directory string
	prefix := "   "
	if f.columns.start[placeColumn] == 0 {
		host, directory = m.place(*row)
		if directory != "" {
			prefix += "in "
		}
		placeAt = len(caption)
		place := host + directory
		if directory == "" {
			place = unknownPlace
		}
		caption = append(caption, span{text: prefix + place, role: quietRole})
	}
	if hidden {
		caption = append(caption, span{text: fmt.Sprintf("   %d of %d", m.cursor+1, len(m.rows)), role: quietRole})
	}

	// The least important facts are at the tail. Shorten place once before
	// giving up the fact, retaining the host and at least 12 cells with "in ".
	width := caption.width()
	for width > room && len(caption) > statusEnd {
		last := len(caption) - 1
		width -= ansi.StringWidth(caption[last].text)
		if last == placeAt && room-width-3 >= 12 {
			if place := fitPlace(host, directory, room-width-len(prefix)); place != "" {
				caption[last].text = prefix + place
				return caption
			}
		}
		caption = caption[:last]
	}
	if width <= room {
		return caption
	}
	width -= ansi.StringWidth(caption[1].text)
	caption[1].text = ""
	if width <= room {
		return caption
	}
	if nameRoom := room - width + ansi.StringWidth(name); nameRoom >= 6 {
		caption[0].text = ansi.Truncate(name, nameRoom, "…")
		return caption
	}

	status := caption[3:]
	for i := range status {
		if width := ansi.StringWidth(status[i].text); width <= room {
			room -= width
			continue
		}
		status[i].text = ansi.Truncate(status[i].text, room, "…")
		return status[:i+1]
	}
	return status
}
