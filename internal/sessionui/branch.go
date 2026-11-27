package sessionui

import (
	"context"
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
)

// A dialog owns one read-only proposal and at most one confirmation of that proposal.
type branchDialog struct {
	request     fleetclient.Request
	name, label string
	plan        *fleetclient.ClosePlan
	report      *fleetclient.CloseReport
	failure     string
	offset      int
	cancel      context.CancelFunc
}

type branchPreviewMsg struct {
	dialog  *branchDialog
	plan    fleetclient.ClosePlan
	failure *fleetclient.Failure
}

type branchClosedMsg struct {
	dialog *branchDialog
	report fleetclient.CloseReport
}

func (m *model) openClose() tea.Cmd {
	row := m.selectedRow()
	ctx, cancel := context.WithCancel(m.ctx)
	d := &branchDialog{request: fleetclient.Request{Operation: "close", Ref: row.Session.Ref.Reference}, name: row.Session.Name, label: row.Label, cancel: cancel}
	m.fold = fold{kind: foldBranch, branch: d}
	m.inform("")
	client := m.client
	return func() tea.Msg {
		defer cancel()
		plan, failure := client.PreviewClose(ctx, d.request)
		return branchPreviewMsg{d, plan, failure}
	}
}

func (m *model) onPreview(msg branchPreviewMsg) {
	if m.fold.branch != msg.dialog {
		return
	}
	if msg.failure != nil {
		msg.dialog.failure = fleetview.ErrorMessage(*msg.failure, msg.dialog.request)
		return
	}
	root := msg.plan.Targets[len(msg.plan.Targets)-1]
	msg.dialog.name, msg.dialog.label = root.Name, root.Label
	msg.dialog.plan = &msg.plan
}

func (d *branchDialog) text(width int) (header, rows []string) {
	subject := d.name + " on " + d.label
	switch {
	case d.report != nil:
		header = wrapped("close results for "+subject, width)
		if d.report.Failure != nil {
			rows = append(rows, wrapped(fleetview.ErrorMessage(*d.report.Failure, d.request), width)...)
		}
		if d.report.CleanupFailure != nil {
			rows = append(rows, wrapped("former parent registration could not be cleared.", width)...)
		}
		for _, fence := range d.report.FenceFailures {
			rows = append(rows, wrapped(fence.Name+" on "+fence.Label+" · close fence release unconfirmed", width)...)
		}
		for _, target := range d.report.Targets {
			outcome := target.Terminal
			if !d.plan.TerminalOnly {
				outcome += " · interrupt " + target.Interrupt
			}
			rows = append(rows, wrapped(target.Name+" on "+target.Label+" · "+outcome, width)...)
		}
	case d.failure != "":
		header = wrapped("cannot close "+subject, width)
		rows = wrapped(d.failure, width)
	case d.plan == nil:
		header = wrapped("checking what would close with "+subject+"…", width)
	default:
		header = wrapped(fleetview.CloseQuestion(d.plan.Targets, d.plan.TerminalOnly), width)
		effect := "the listed sessions close leaves first. shared or remote work may continue."
		if !d.plan.TerminalOnly {
			effect = "send one interrupt to each selected pane, then close its session after confirmed delivery. shared or remote work may continue."
		}
		header = append(header, wrapped(effect, width)...)
		for _, target := range d.plan.Targets {
			rows = append(rows, wrapped(target.Name+" on "+target.Label, width)...)
		}
	}
	return
}

func (d *branchDialog) page(capacity, width int) (header, rows []string, room int, scroll bool) {
	header, rows = d.text(width)
	room = max(0, capacity-len(header))
	scroll = len(rows) > room && room >= 2
	if scroll {
		room--
	}
	return
}

func (d *branchDialog) lines(capacity, width int) []line {
	header, rows, room, scroll := d.page(capacity, width)
	var lines []line
	for _, s := range header {
		lines = append(lines, line{{text: s, role: emberRole, bold: true}})
	}
	at := min(d.offset, max(0, len(rows)-room))
	for _, s := range rows[at:min(len(rows), at+room)] {
		part := span{text: s}
		if d.plan != nil && d.report == nil {
			part.preview = d
		}
		lines = append(lines, line{part})
	}
	if scroll {
		lines = append(lines, line{{text: fmt.Sprintf("lines %d–%d of %d · ↑↓ scroll", at+1, at+room, len(rows)), role: quietRole}})
	}
	return lines
}

func (m *model) scrollBranch(delta int) {
	d := m.fold.branch
	f := m.chrome()
	_, rows, room, _ := d.page(m.fold.capacity(f.body), max(0, f.rowWidth-rowNameStart))
	if room > 0 {
		last := max(0, len(rows)-room)
		d.offset = min(max(0, min(d.offset, last)+delta), last)
	}
}

func (m *model) branchKeys() []key {
	d := m.fold.branch
	dismiss := "cancel"
	if d.report != nil || d.failure != "" {
		dismiss = "dismiss"
	}
	keys := []key{
		{keys: []string{"esc", "n", "q"}, show: "escape", label: dismiss, keep: true, act: func(m *model) tea.Cmd { d.cancel(); m.fold = fold{}; return nil }},
		{keys: []string{"up", "k"}, show: "↑↓", label: "scroll", rank: 2, act: func(m *model) tea.Cmd { m.scrollBranch(-1); return nil }},
		{keys: []string{"down", "j"}, act: func(m *model) tea.Cmd { m.scrollBranch(1); return nil }},
	}
	if d.plan != nil && d.report == nil {
		keys = append(keys, key{keys: []string{"enter", "y"}, show: "enter", label: "confirm close", rank: 1,
			can: func(m *model) string {
				f := m.chrome()
				width := f.rowWidth - rowNameStart
				header, _ := d.text(width)
				if width < 8 || len(header)+1 > m.fold.capacity(f.body) {
					return "window too small to confirm this close."
				}
				return ""
			},
			act: func(m *model) tea.Cmd {
				m.busy = true
				client, ctx, plan := m.client, m.ctx, *d.plan
				return func() tea.Msg { return branchClosedMsg{d, client.ConfirmClose(ctx, plan)} }
			},
		})
	}
	return keys
}

func (m *model) onBranchClosed(msg branchClosedMsg) tea.Cmd {
	m.busy = false
	d := msg.dialog
	d.report = &msg.report
	d.offset = 0
	// A confirmed closure supersedes reads already in flight even across a fleet rejoin.
	affected := map[int]bool{}
	for _, result := range msg.report.Targets {
		index := m.peerIndex(result.Ref.Machine)
		if index < 0 {
			continue
		}
		affected[index] = true
		if result.Terminal == "closed" {
			peer := &m.peers[index]
			peer.Sessions = slices.DeleteFunc(peer.Sessions, func(s fleetclient.Session) bool { return s.Ref.SessionEqual(result.Ref) })
		}
	}
	m.rebuild()
	var reads []tea.Cmd
	for index := range affected {
		reads = append(reads, m.refreshMutation(index))
	}
	return tea.Batch(reads...)
}
