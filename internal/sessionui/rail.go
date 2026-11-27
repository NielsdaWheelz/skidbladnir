package sessionui

import (
	"errors"
	"fmt"
	"image/color"
	"slices"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/compositor"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
)

type rail struct {
	client    *compositor.Client
	state     compositor.State
	visit     *visit
	moving    bool
	mutations sync.WaitGroup
	err       error
}

type visit struct {
	ref            fleetclient.Reference
	id, name, host string
}

func (v *visit) subject() string { return v.name + " on " + v.host }

type compositorWake struct{}
type movementResult struct {
	attempt *visit
	err     error
}

func (m *model) waitForCompositor() tea.Cmd {
	ctx, woken := m.ctx, m.rail.client.Woken()
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case <-woken:
			return compositorWake{}
		}
	}
}

// Only a key handoff can be in flight. Read and discard any queued rail input
// until its result has been reconciled against the compositor.
func (m *model) moveKeys(change func() movementResult) tea.Cmd {
	m.pressedPointer = pointerTarget{}
	m.rail.moving = true
	done := make(chan movementResult, 1)
	m.rail.mutations.Go(func() { done <- change() })
	return func() tea.Msg { return <-done }
}

func (m *model) finishMovement(result movementResult) tea.Cmd {
	m.rail.moving = false
	returns := m.rail.state.Returns
	command := m.readRail(result.attempt)
	if result.err != nil && m.rail.err == nil && m.rail.visit != nil && returns == m.rail.state.Returns {
		m.fail("could not return the keys to " + m.rail.visit.subject() + ".")
	}
	return command
}

func (m *model) enter(ref fleetclient.Reference) tea.Cmd {
	row := m.rowForReference(ref)
	if row == nil || row.Session.Ref.Reference != ref {
		m.fail("session no longer listed at that pane; nothing was sent.")
		return nil
	}
	m.fold, m.reveal = fold{}, time.Time{}
	m.revealGeneration++
	m.clearDirectorySearch()
	candidate := visit{
		ref: ref, name: attention.SingleLine(row.Session.Name), host: attention.SingleLine(row.Label),
	}
	client := m.rail.client
	return m.moveKeys(func() movementResult {
		// The returned id is the attempted start, even if tmux reported an error.
		candidate.id, _ = client.Visit(ref.Encode())
		return movementResult{attempt: &candidate}
	})
}

func (m *model) stageFits() bool {
	s := m.rail.state
	return terminal.ValidSize(compositor.StageSize(s.Columns, s.Rows))
}

// The command result is not a layout or input verdict. An acknowledged or
// uncertain start is adopted only when the compositor reports its exact id.
func (m *model) readRail(attempt *visit) tea.Cmd {
	state, err := m.rail.client.State()
	if err != nil {
		m.rail.err = err
		return tea.Quit
	}
	returned := state.Returns != m.rail.state.Returns
	if old := m.rail.state; state.Columns != old.Columns || state.Rows != old.Rows ||
		state.Live != old.Live || state.StageKeys != old.StageKeys || state.Visit != old.Visit || returned {
		m.pressedPointer = pointerTarget{}
	}
	m.rail.state = state
	m.width, m.height = state.Columns, state.Rows
	if state.Live {
		m.width = compositor.RailColumns
	}
	if attempt != nil && state.Visit == attempt.id {
		m.rail.visit = attempt
	}
	current := m.rail.visit
	if current != nil && state.Visit != current.id {
		m.rail.err = errors.New("the compositor reported an unrequested visit")
		return tea.Quit
	}
	if returned && current != nil {
		m.fold = fold{}
		m.inform("")
		m.selectVisit(current.ref)
	}
	if current == nil || state.Live {
		if attempt != nil && state.Visit != attempt.id {
			m.fail("attach failed for " + attempt.subject() + ".")
		}
		return nil
	}
	m.rail.visit = nil
	m.fold = fold{}
	subject := current.subject()
	switch state.End {
	case "rail":
		m.inform("detached from " + subject + ".")
	case "narrow":
		m.inform(fmt.Sprintf("detached from %s; the window was under %d columns.", subject, compositor.SplitColumns))
	case "exit":
		outcome, err := m.rail.client.Outcome(current.id)
		if err != nil && outcome == "" {
			m.fail(subject + ": attachment ended; its outcome could not be read; work may continue.")
			break
		}
		if outcome == "" {
			outcome = "attachment ended; no cause was recorded; work may continue"
		}
		if state.ExitStatus == compositor.StageEnded {
			m.inform(subject + ": " + outcome)
		} else {
			m.fail(subject + ": " + outcome)
		}
	}
	if attempt != nil && state.Visit != attempt.id {
		m.fail(m.notice + " attach failed for " + attempt.subject() + ".")
	}
	return m.refresh(true)
}

const discardedRailInput = "the rail has the keys; nothing was sent."

func (m *model) browserInput(message tea.Msg, painted pointerHit) tea.Cmd {
	if m.rail.moving {
		return nil
	}
	var ended, action tea.Cmd
	if m.rail.visit != nil {
		ended = m.readRail(nil)
		if m.rail.err != nil || m.rail.visit != nil && m.rail.state.StageKeys {
			return ended
		}
	}
	switch message := message.(type) {
	case tea.KeyPressMsg:
		if m.rail.visit != nil {
			action = m.railKey(message.String())
		} else {
			action = m.onKey(message)
		}
	case tea.PasteMsg:
		if m.rail.visit != nil {
			m.fail(discardedRailInput)
		} else {
			action = m.onPaste(message)
		}
	case tea.MouseMsg:
		action = m.pointerInput(message, painted)
	}
	return tea.Batch(ended, action)
}

func (m *model) railKey(pressed string) tea.Cmd {
	current := m.rail.visit
	if m.notice == discardedRailInput {
		m.inform("")
	}
	for _, entry := range movementKeys {
		if slices.Contains(entry.keys, pressed) {
			return entry.act(m)
		}
	}
	switch pressed {
	case "tab", "shift+tab":
		direction := 1
		if pressed == "shift+tab" {
			direction = -1
		}
		if refusal := queuedGuard(direction)(m); refusal != "" {
			m.fail(refusal)
		} else {
			return queuedMove(direction)(m)
		}
	case "enter":
		return m.attachSelected()
	case "ctrl+]":
		return m.returnStageKeys(current.id)
	case "ctrl+d":
		err := m.rail.client.Table(current.id)
		command := m.readRail(nil)
		if err != nil && m.rail.visit != nil {
			m.fail("could not detach from " + current.subject() + ".")
		}
		return command
	case "?":
		if m.fold.kind == foldKeys {
			m.fold = fold{}
		} else {
			m.fold = fold{kind: foldKeys}
		}
	case "esc", "q":
		if m.fold.kind == foldKeys {
			m.fold = fold{}
		} else {
			m.fail(discardedRailInput)
		}
	case "ctrl+r":
		return m.refreshManually()
	default:
		m.fail(discardedRailInput)
	}
	return nil
}

func (m *model) returnStageKeys(visitID string) tea.Cmd {
	client := m.rail.client
	return m.moveKeys(func() movementResult {
		return movementResult{err: client.Keys(visitID, true)}
	})
}

func (m *model) settleGround(answered bool) tea.Cmd {
	m.ground.settle(answered)
	gold, quiet, dim := "brightyellow", "", m.ground.raise != nil
	if dim {
		hex := func(c color.Color) string {
			rgb := color.NRGBAModel.Convert(c).(color.NRGBA)
			return fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B)
		}
		gold = hex(m.ground.pigments[goldRole])
		quiet = hex(m.ground.pigments[quietRole])
	}
	if err := m.rail.client.Paint(gold, quiet, dim); err != nil {
		m.rail.err = err
		return tea.Quit
	}
	return nil
}

func (m *model) railHints() line {
	if m.rail.state.StageKeys {
		return appendHint(nil, key{show: "ctrl-]", label: "browse"})
	}
	back := appendHint(nil, key{show: "ctrl-]", label: "back"})
	if m.fold.kind == foldKeys {
		return appendHint(back, key{show: "escape", label: "close"})
	}
	return appendHint(back, key{show: "ctrl-d", label: "detach"})
}

func (m *model) railKeyMap() []line {
	lines := []line{
		{{text: "rail keys", bold: true}},
		{{text: "ctrl-] back"}},
		{{text: "enter attach"}},
		{{text: "ctrl-d detach"}},
	}
	for _, entry := range movementKeys {
		if entry.show != "" {
			lines = append(lines, line{{text: entry.show + " " + entry.label}})
		}
	}
	return append(lines,
		line{{text: "tab next needing input"}},
		line{{text: "shift-tab previous"}},
		line{{text: "ctrl-r refresh"}},
		line{{text: "escape close"}},
		line{{text: "▌", role: goldRole}, {text: " rail has keys", role: quietRole}},
		line{{text: "▏", role: quietRole}, {text: " session has keys", role: quietRole}},
		line{{text: "▐ shown at right", role: quietRole}},
		line{{text: "other keys do nothing"}},
	)
}
