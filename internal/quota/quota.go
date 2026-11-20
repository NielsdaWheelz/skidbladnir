// Package quota reads account limits from the existing Codex daemon and Claude cache.
package quota

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/claudeusage"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/coder/websocket"
)

// Usage carries the observation for one configured provider profile.
type Usage struct {
	Profile agentruntime.Profile
	Report  *api.QuotaReport
	Claude  *api.ClaudeUsage
}

type Observation struct {
	ObservedAt          time.Time
	Profiles            []Usage
	OtherClaudeAccounts []api.ClaudeUsage
	HistoryUnavailable  bool
}

// Observe reads local profiles concurrently. The Claude-only route never contacts a
// Codex daemon. Both routes remember the local Claude samples before replying.
func Observe(parent context.Context, manager *sessions.Manager, stateDir string, claudeOnly bool) (Observation, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	revision := manager.SettingsRevision()
	var profiles []Usage
	for _, profile := range manager.Profiles() {
		if profile.Provider == agentruntime.ProviderClaude || !claudeOnly && profile.Provider == agentruntime.ProviderCodex {
			profiles = append(profiles, Usage{Profile: profile})
		}
	}
	var pending sync.WaitGroup
	for index := range profiles {
		pending.Go(func() {
			usage := &profiles[index]
			switch usage.Profile.Provider {
			case agentruntime.ProviderCodex:
				usage.Report = read(ctx, agentruntime.CodexEndpoint(usage.Profile))
			case agentruntime.ProviderClaude:
				reading := ReadClaudeProfile(usage.Profile.Home)
				if reading.Account != "" {
					usage.Claude = &reading
				}
			}
		})
	}
	pending.Wait()
	observedAt := time.Now().UTC()
	historyUnavailable := false
	represented := make(map[string]bool)
	for index := range profiles {
		usage := &profiles[index]
		if usage.Claude != nil {
			represented[usage.Claude.Account] = true
			saved, err := claudeusage.Remember(stateDir, *usage.Claude)
			if err != nil {
				historyUnavailable = true
			}
			usage.Claude = &saved
		}
		// only a wall-clock step dates a report after the sample; clients refuse the whole host for one.
		if report := usage.Report; report != nil && report.ReportedAt.After(observedAt) {
			usage.Report = nil
		}
	}
	var other []api.ClaudeUsage
	if claudeOnly {
		var err error
		other, err = claudeusage.Others(stateDir, represented)
		historyUnavailable = historyUnavailable || err != nil
	}
	if manager.SettingsRevision() != revision {
		return Observation{}, errors.New("quota configuration changed")
	}
	return Observation{ObservedAt: observedAt, Profiles: profiles, OtherClaudeAccounts: other, HistoryUnavailable: historyUnavailable}, nil
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
