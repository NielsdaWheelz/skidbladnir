// Package quota acquires and reads host-local account limits.
package quota

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/claudeusage"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/coder/websocket"
)

const (
	readTimeout          = 5 * time.Second
	claudeFailureBackoff = 5 * time.Minute
)

// Usage carries the observation for one configured provider profile.
type Usage struct {
	Profile agentruntime.Profile
	Report  *api.QuotaReport
	Claude  ClaudeSample
}

type Observation struct {
	ObservedAt         time.Time
	Profiles           []Usage
	ClaudeAccounts     []api.ClaudeUsage
	HistoryUnavailable bool
}

// Observer owns the gateway's quota reads and any native work they start.
type Observer struct {
	manager  *sessions.Manager
	stateDir string
	logger   *slog.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	blocked  atomic.Bool // a detected dispatch could not be blocked on disk

	mu      sync.Mutex
	pending sync.WaitGroup
	next    map[string]time.Time // zero means an attempt is running
}

func NewObserver(parent context.Context, manager *sessions.Manager, stateDir string, logger *slog.Logger) *Observer {
	ctx, cancel := context.WithCancel(parent)
	return &Observer{manager: manager, stateDir: stateDir, logger: logger, ctx: ctx, cancel: cancel, next: make(map[string]time.Time)}
}

func (observer *Observer) Close() {
	observer.mu.Lock()
	observer.cancel()
	observer.mu.Unlock()
	observer.pending.Wait()
}

// Observe reads Codex profiles concurrently. Passive reads do no native work and
// also publish retained accounts with no bound profile.
func (observer *Observer) Observe(parent context.Context, passive bool) (Observation, error) {
	ctx, cancel := context.WithTimeout(parent, readTimeout)
	defer cancel()
	revision := observer.manager.SettingsRevision()
	observed := Observation{Profiles: []Usage{}, ClaudeAccounts: []api.ClaudeUsage{}}
	for _, profile := range observer.manager.Profiles() {
		if profile.Provider == agentruntime.ProviderClaude || !passive && profile.Provider == agentruntime.ProviderCodex {
			observed.Profiles = append(observed.Profiles, Usage{Profile: profile})
		}
	}
	var pending sync.WaitGroup
	for index := range observed.Profiles {
		usage := &observed.Profiles[index]
		if usage.Profile.Provider == agentruntime.ProviderCodex {
			pending.Go(func() {
				usage.Report = read(ctx, agentruntime.CodexEndpoint(usage.Profile))
			})
		}
	}
	pending.Wait()
	// Date Claude bindings when they are read. A full read waiting on Codex
	// must not give an earlier login a later timestamp than a passive sign-out.
	observer.mu.Lock()
	for index := range observed.Profiles {
		usage := &observed.Profiles[index]
		if usage.Profile.Provider == agentruntime.ProviderClaude {
			usage.Claude = ReadClaudeProfile(usage.Profile)
		}
	}
	observed.ObservedAt = time.Now().UTC()
	observer.mu.Unlock()
	positions := make(map[string]int)
	for index := range observed.Profiles {
		usage := &observed.Profiles[index]
		if reading := usage.Claude.Reading; reading.Account != "" {
			if position, ok := positions[reading.Account]; ok {
				observed.ClaudeAccounts[position].Merge(reading)
			} else {
				positions[reading.Account] = len(observed.ClaudeAccounts)
				observed.ClaudeAccounts = append(observed.ClaudeAccounts, reading)
			}
		}
		// only a wall-clock step dates a report after the sample; clients refuse the whole host for one.
		if report := usage.Report; report != nil && report.ReportedAt.After(observed.ObservedAt) {
			usage.Report = nil
		}
	}
	for index, reading := range observed.ClaudeAccounts {
		saved, err := claudeusage.Remember(ctx, observer.stateDir, reading)
		observed.HistoryUnavailable = observed.HistoryUnavailable || err != nil
		observed.ClaudeAccounts[index] = saved
	}
	if passive {
		represented := make(map[string]bool, len(positions))
		for account := range positions {
			represented[account] = true
		}
		other, err := claudeusage.Others(observer.stateDir, represented)
		observed.HistoryUnavailable = observed.HistoryUnavailable || err != nil
		observed.ClaudeAccounts = append(observed.ClaudeAccounts, other...)
	}
	if observer.manager.SettingsRevision() != revision {
		return Observation{}, errors.New("quota configuration changed")
	}
	if !passive {
		observer.demand(observed, revision)
	}
	return observed, nil
}

func (observer *Observer) demand(observed Observation, revision uint64) {
	configs := make(map[string]string)
	ambiguous := make(map[string]bool)
	for _, usage := range observed.Profiles {
		profile := usage.Profile
		if profile.Provider == agentruntime.ProviderClaude {
			if previous, seen := configs[profile.Home]; seen && previous != profile.ClaudeConfigDir {
				ambiguous[profile.Home] = true
			}
			configs[profile.Home] = profile.ClaudeConfigDir
		}
	}
	fresh := make(map[string]bool)
	byAccount := make(map[string][]Usage)
	for _, usage := range observed.Profiles {
		account := usage.Claude.Reading.Account
		if account == "" {
			continue
		}
		fetchedAt := usage.Claude.FetchedAt
		if age := observed.ObservedAt.Sub(fetchedAt); age >= 0 && age < time.Minute {
			fresh[account] = true
		}
		if !ambiguous[usage.Profile.Home] {
			byAccount[account] = append(byAccount[account], usage)
		}
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.ctx.Err() != nil || observer.blocked.Load() {
		return
	}
	now := time.Now()
	for account, candidates := range byAccount {
		next, seen := observer.next[account]
		if fresh[account] || seen && (next.IsZero() || now.Before(next)) {
			continue
		}
		observer.next[account] = time.Time{}
		observer.pending.Go(func() { observer.refresh(account, candidates, revision) })
	}
}

func (observer *Observer) refresh(account string, candidates []Usage, revision uint64) {
	var next time.Time
	defer func() {
		observer.mu.Lock()
		defer observer.mu.Unlock()
		if next.IsZero() {
			delete(observer.next, account)
		} else {
			observer.next[account] = next
		}
	}()
	current := func() bool {
		return observer.ctx.Err() == nil && !observer.blocked.Load() && observer.manager.SettingsRevision() == revision
	}
	authFailed := false
	for _, usage := range candidates {
		if !current() {
			return
		}
		started, fresh, err := refreshClaude(observer.ctx, observer.stateDir, usage.Profile, usage.Claude, current)
		if errors.Is(err, errClaudeBlock) {
			observer.blocked.Store(true)
		}
		if err != nil && observer.ctx.Err() == nil {
			observer.logger.Warn("Quota.ClaudeRefreshFailed", "reason", err.Error())
		}
		if started.IsZero() {
			if fresh {
				return // a matching native write made this account fresh during preflight
			}
			authFailed = authFailed || errors.Is(err, errClaudeAuth)
			continue
		}
		if err != nil || !current() {
			next = time.Now().Add(claudeFailureBackoff)
			return
		}
		next = started.Add(time.Minute)
		// The ordinary snapshot reads the current login and publishes its cache.
		return
	}
	if authFailed {
		next = time.Now().Add(claudeFailureBackoff)
	}
}

// read makes one observe-only exchange with the daemon at endpoint: initialize,
// initialized, account/rateLimits/read. Any failure leaves the report nil.
func read(ctx context.Context, endpoint string) *api.QuotaReport {
	// a throwaway transport must not pool the connection of a refused handshake.
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", endpoint)
	}}
	connection, _, err := websocket.Dial(ctx, "ws://localhost/", &websocket.DialOptions{HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		return nil
	}
	// no close handshake: a read or write whose ctx ends closes the connection too.
	defer connection.CloseNow()
	connection.SetReadLimit(4 << 20)
	send := func(message string) bool { return connection.Write(ctx, websocket.MessageText, []byte(message)) == nil }
	if !send(`{"id":1,"method":"initialize","params":{"capabilities":{"experimentalApi":false},"clientInfo":{"name":"skidbladnir","title":"skidbladnir","version":"1"}}}`) {
		return nil
	}
	if _, ok := await(ctx, connection, "1"); !ok || !send(`{"method":"initialized"}`) || !send(`{"id":2,"method":"account/rateLimits/read","params":{}}`) {
		return nil
	}
	result, ok := await(ctx, connection, "2")
	if !ok {
		return nil
	}
	// reportedAt carries microseconds on every platform, as the wire always has.
	return normalize(result, time.Now().UTC().Truncate(time.Microsecond))
}

// await returns the result of response id. Server notifications and requests
// are skipped and never answered.
func await(ctx context.Context, connection *websocket.Conn, id string) (json.RawMessage, bool) {
	for {
		kind, frame, err := connection.Read(ctx)
		var message map[string]json.RawMessage
		// strictjson refuses invalid utf-8 and duplicate keys at any depth.
		if err != nil || kind != websocket.MessageText || strictjson.Decode(frame, &message) != nil {
			return nil, false
		}
		if _, fromServer := message["method"]; !fromServer {
			result, answered := message["result"]
			return result, answered && message["error"] == nil && string(message["id"]) == id
		}
	}
}

// normalize maps the default snapshot's 300- and 10080-minute windows. Maps keep
// member names exact, and every number must be a JSON integer token that
// strconv.ParseInt accepts: 42.0, "42", true and integers beyond int64 are malformed.
func normalize(result json.RawMessage, reportedAt time.Time) *api.QuotaReport {
	var response, snapshot map[string]json.RawMessage
	if json.Unmarshal(result, &response) != nil || json.Unmarshal(response["rateLimits"], &snapshot) != nil || snapshot == nil {
		return nil
	}
	report := &api.QuotaReport{ReportedAt: reportedAt}
	for _, position := range []string{"primary", "secondary"} {
		if absent(snapshot[position]) {
			continue
		}
		var window map[string]json.RawMessage
		if json.Unmarshal(snapshot[position], &window) != nil {
			return nil
		}
		if absent(window["windowDurationMins"]) {
			continue
		}
		duration, err := strconv.ParseInt(string(window["windowDurationMins"]), 10, 64)
		var slot **api.QuotaWindow
		switch {
		case err != nil:
			return nil
		case duration == 300:
			slot = &report.FiveHour
		case duration == 10080:
			slot = &report.SevenDay
		default:
			continue // an unknown window is skipped unread
		}
		used, err := strconv.ParseInt(string(window["usedPercent"]), 10, 64)
		if err != nil || used < 0 || *slot != nil {
			return nil
		}
		*slot = &api.QuotaWindow{UsedPercent: float64(used)}
		if !absent(window["resetsAt"]) {
			seconds, err := strconv.ParseInt(string(window["resetsAt"]), 10, 64)
			// years 1 to 9999, what time.MarshalJSON can encode, excluding the zero instant clients refuse.
			if err != nil || seconds <= -62135596800 || seconds > 253402300799 {
				return nil
			}
			reset := time.Unix(seconds, 0).UTC()
			(*slot).ResetsAt = &reset
		}
	}
	return report
}

// absent: the protocol uses a missing member and null interchangeably.
func absent(raw json.RawMessage) bool { return raw == nil || string(raw) == "null" }
