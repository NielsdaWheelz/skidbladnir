package sessionui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/charmbracelet/x/ansi"
)

type screen struct {
	ref     fleetclient.Reference
	lines   []string
	at      time.Time
	failure string
	asked   bool
}

type screenMsg struct {
	generation uint64
	result     fleetclient.Result
	at         time.Time
}

func (m *model) forgetScreen() {
	m.screenLane.fence()
	m.screen = screen{}
}

func (m *model) askScreen() tea.Cmd {
	ref := m.selectedRow().Session.Ref.Reference
	if m.screen.ref != ref {
		m.forgetScreen()
		m.screen.ref = ref
	}
	m.screen.asked = true
	return m.readScreen()
}

func (m *model) readScreen() tea.Cmd {
	row := m.selectedRow()
	if !m.screen.asked || m.screenLane.flight != 0 || !m.observing() ||
		m.fold.kind != foldOpen || row == nil || !row.Available || row.Session.Ref.Reference != m.screen.ref {
		return nil
	}
	m.screen.asked = false
	client := m.client
	request := fleetclient.Request{Operation: "read", Ref: m.screen.ref, MaxBytes: 4096}
	return m.screenLane.read(m.ctx, false, func(ctx context.Context, generation uint64) tea.Msg {
		result := client.Execute(ctx, request)
		return screenMsg{generation, result, time.Now()}
	})
}

func (m *model) acceptScreen(message screenMsg) tea.Cmd {
	landed, current := m.screenLane.land(message.generation)
	if !landed {
		return nil
	}
	if current {
		result := message.result
		switch {
		case accessRejected(result.Error):
			m.invalidateAccess(m.screen.ref.Machine, result.Error)
		case !result.OK:
			m.screen.failure = fleetview.ErrorMessage(*result.Error, fleetclient.Request{Operation: "read", Ref: m.screen.ref})
		default:
			m.screen.lines, m.screen.at, m.screen.failure = nil, message.at, ""
			for raw := range strings.SplitSeq(result.Value.(api.ReadResult).Text, "\n") {
				text := attention.SingleLine(raw)
				if strings.ContainsFunc(text, func(r rune) bool {
					return !unicode.IsSpace(r) && !unicode.Is(unicode.Dash, r) && r != '_' && (r < 0x2500 || r > 0x257f)
				}) {
					m.screen.lines = append(m.screen.lines, text)
				}
			}
		}
	}
	return m.readScreen()
}

func (m *model) screenLines(row *fleetclient.Row, capacity, width int) []line {
	if capacity <= 0 {
		return nil
	}
	s := m.screen
	var hostFailure *fleetclient.Failure
	if index := m.peerIndex(row.Machine); index >= 0 {
		hostFailure = m.peers[index].Error
	}
	const prefix = "screen of "
	suffix := ": available"
	showSample := !s.at.IsZero() && (s.failure == "" || hostFailure != nil)
	switch {
	case m.screenLane.flight != 0 && m.screenLane.flight == m.screenLane.generation && (hostFailure == nil || !showSample):
		suffix = ": reading"
		showSample = false
	case showSample:
		age, elapsed := "just now", time.Since(s.at)
		if elapsed >= time.Minute {
			age = fmt.Sprintf("%dm ago", elapsed/time.Minute)
		} else if elapsed >= 5*time.Second {
			age = fmt.Sprintf("%ds ago", elapsed/time.Second)
		}
		suffix = ", read " + age
		if hostFailure != nil {
			before, after := ", as read "+age+"; ", " is unavailable"
			host := ansi.Truncate(attention.SingleLine(row.Label), width-4-ansi.StringWidth(prefix+before+after), "…")
			suffix = before + host + after
		}
	case accessRejected(hostFailure):
		suffix = ": " + fleetview.ErrorMessage(*hostFailure, fleetclient.Request{Operation: "read"})
	case s.failure != "":
		suffix = ": " + s.failure
	case !row.Available && m.rowAttachable(*row):
		suffix = ": checking"
	case !row.Available:
		suffix = ": " + m.rowUnavailability(m.rowForReference(row.Session.Ref.Reference))
	}
	suffix = attention.SingleLine(suffix)
	name := ansi.Truncate(attention.SingleLine(row.Session.Name), max(1, width-3-ansi.StringWidth(prefix+suffix)), "…")
	header := wrapped(prefix+name+suffix, width-3)
	lines := []line{rule(ansi.Truncate(header[0], width-3, "…"), "", width)}
	for _, continuation := range header[1:min(capacity, len(header))] {
		lines = append(lines, line{{text: continuation}})
	}
	if showSample {
		for _, text := range s.lines[max(0, len(s.lines)-capacity+len(lines)):] {
			lines = append(lines, line{{text: "  " + ansi.Truncate(text, width-2, "")}})
		}
	}
	return lines
}
