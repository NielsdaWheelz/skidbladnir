package agentcontrol

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// Reuse (tmux-exec-budget.md a3) trusts a batch detection while its session's
// screen key holds and its capture started after the key's window activity
// settled: activity has one-second resolution, so output after such a capture
// stamps another second and changes the key. stepTolerance is the largest
// wall/monotonic disagreement reuse survives; settleMargin adds it as slack
// for a step too small to detect.
const (
	stepTolerance = 100 * time.Millisecond
	settleMargin  = time.Second + stepTolerance
)

// Inventory lists one admitted lifetime's sessions and sets their terminal
// facts under one shared two-second deadline, never resolving them again:
// screenless statuses and reused detections first, then one capture batch,
// then today's per-pane observation for each pane the batch could not read.
// err is List's, or the lifetime did not hold from the inventory read to the
// end of enrichment. The failures are the unavailable observations, for
// content-free logging.
func (service *Service) Inventory(parent context.Context) (sessions.Inventory, []ObservationFailure, error) {
	inventory, err := service.sessions.List(parent)
	if err != nil || len(inventory.Sessions) == 0 {
		return inventory, nil, err // no row names a lifetime to prove
	}
	observed := inventory.Sessions
	startedAt := time.Now() // before the batch exec: no capture starts earlier
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	reads := make([]detection, len(observed))
	elapsed := make([]time.Duration, len(observed))
	settled := make(map[sessions.ScreenKey]detection) // this round's reusable detections
	var pending []int

	service.mu.Lock()
	if stepped(service.last, startedAt) {
		clear(service.reused)
	}
	service.last = startedAt
	for index, session := range observed {
		if read, ok := unobserved(session); ok {
			reads[index] = read
			continue
		}
		key := session.ScreenKey()
		if read, ok := service.reused[key]; ok {
			reads[index], settled[key] = read, read
			continue
		}
		pending = append(pending, index)
	}
	service.mu.Unlock()

	// The batch always runs: its bracket proves the lifetime, alone when
	// nothing needs capture.
	targets := make([]sessions.Session, len(pending))
	for position, index := range pending {
		targets[position] = observed[index]
	}
	captures, batchErr := service.sessions.CapturePanes(ctx, inventory.Server, targets)
	var fallback []int
	switch {
	case errors.Is(batchErr, tmuxclient.WorkspaceSourceChanged):
		return sessions.Inventory{}, nil, batchErr
	case batchErr != nil:
		// No bracketed transcript: every pane falls back, and the lifetime is
		// proved after the captures. After a deadline overrun the fallbacks
		// have no time left, so every pending pane reads observation_timeout.
		fallback = pending
	default:
		for position, index := range pending {
			switch capture := captures[position]; {
			case capture.Err == nil:
				reads[index] = detect(observed[index].Agent.Provider, capture.Observation)
				if key := observed[index].ScreenKey(); !startedAt.Before(key.Activity().Add(settleMargin)) {
					settled[key] = reads[index]
				}
			case errors.Is(capture.Err, sessions.ErrTerminalScreenChanged):
				fallback = append(fallback, index)
			default:
				reads[index] = failedDetection(ctx, capture.Err)
			}
		}
	}
	batched := time.Since(startedAt)
	for _, index := range pending {
		elapsed[index] = batched
	}

	// A fallback reads a fresh screen that need not match the key, so it is
	// never reused. A panic on a worker goroutine would end the gateway; it is
	// raised again on the caller's goroutine, where net/http confines it to the
	// one request.
	defects := make([]string, len(observed))
	var work sync.WaitGroup
	for _, index := range fallback {
		work.Go(func() {
			defer func() {
				if defect := recover(); defect != nil {
					defects[index] = fmt.Sprintf("%v\n\n%s", defect, debug.Stack())
				}
			}()
			read, err := service.sample(ctx, observed[index])
			if err != nil {
				// Enrichment has no target to reject: a lifetime or pane change
				// during capture is a failed capture.
				read = detection{status: sessions.UnknownStatus(sessions.ReasonCaptureFailed)}
			}
			reads[index], elapsed[index] = read, time.Since(startedAt)
		})
	}
	work.Wait()
	for _, defect := range defects {
		if defect != "" {
			panic(defect) // a worker's classifier or observation defect, with the worker's stack
		}
	}
	if batchErr != nil {
		if err := service.sessions.RequireServer(parent, inventory.Server); err != nil {
			return sessions.Inventory{}, nil, err
		}
	}

	// Reuse keeps exactly this round's settled detections, dropping panes the
	// inventory no longer lists. A clock step during the round voids every
	// capture time, this round's included.
	service.mu.Lock()
	service.reused = settled
	if stepped(startedAt, time.Now()) {
		clear(service.reused)
	}
	service.mu.Unlock()

	var failures []ObservationFailure
	for index, read := range reads {
		observed[index].TerminalStatus, observed[index].TerminalUsage, observed[index].TerminalFooter = read.status, read.usage, read.footer
		if read.status.Source == sessions.SourceUnavailable {
			failures = append(failures, ObservationFailure{TmuxID: observed[index].TmuxID, Reason: read.status.Reason, Elapsed: elapsed[index]})
		}
	}
	return inventory, failures, nil
}

// EnrichCreated sets the created session's terminal facts from one per-pane
// observation under a two-second deadline. A target change during it reads as
// capture_failed: it never fails creation. Creation leaves reuse alone: its
// pane printed within the second, so no capture of it could settle.
func (service *Service) EnrichCreated(parent context.Context, created sessions.Session) sessions.Session {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	read, err := service.sample(ctx, created)
	if err != nil {
		read = detection{status: sessions.UnknownStatus(sessions.ReasonCaptureFailed)}
	}
	created.TerminalStatus, created.TerminalUsage, created.TerminalFooter = read.status, read.usage, read.footer
	return created
}

// stepped reports a wall-clock step between from and to: Round(0) strips the
// monotonic reading, so the first difference is the wall delta and the second
// the monotonic one. A zero from has no monotonic reading and never steps.
func stepped(from, to time.Time) bool {
	drift := to.Round(0).Sub(from.Round(0)) - to.Sub(from)
	return drift > stepTolerance || drift < -stepTolerance
}
