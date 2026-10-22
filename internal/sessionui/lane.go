package sessionui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// lane serializes one stream of reads. At most one read is in flight; a fence
// makes every earlier read stale (its reply only frees the lane).
type lane struct {
	generation uint64             // advanced by each read and each fence
	flight     uint64             // generation of the read in flight; 0 when idle
	cancel     context.CancelFunc // the flight's
	// trailing owes one more read: a refresh asked for while the lane was
	// busy, or one its owner wants once the lane frees (context after an
	// inventory with remote rows, usage after a fence). A read clears it.
	trailing bool
}

// read claims an idle lane and returns a command running fetch under its own
// cancellable context; on a busy lane it records trailing and returns nil.
// fetch runs on another goroutine: it must capture no model state.
func (l *lane) read(parent context.Context, trailing bool, fetch func(ctx context.Context, generation uint64) tea.Msg) tea.Cmd {
	if l.flight != 0 {
		l.trailing = l.trailing || trailing
		return nil
	}
	l.generation++
	l.flight, l.trailing = l.generation, false
	ctx, cancel := context.WithCancel(parent)
	l.cancel = cancel
	generation := l.generation
	return func() tea.Msg {
		defer cancel()
		return fetch(ctx, generation)
	}
}

// land frees the lane for its own flight's reply; current means no fence came after.
func (l *lane) land(generation uint64) (landed, current bool) {
	if generation != l.flight {
		return false, false
	}
	l.flight = 0
	return true, generation == l.generation
}

// fence cancels the flight and makes it stale; the lane stays busy until it lands.
func (l *lane) fence() {
	l.generation++
	if l.flight != 0 {
		l.cancel()
	}
}
