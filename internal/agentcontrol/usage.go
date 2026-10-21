package agentcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type UsageWindow struct {
	UsedPercent float64    `json:"usedPercent"`
	ResetsAt    *time.Time `json:"resetsAt,omitempty"`
}

type UsageReport struct {
	ReportedAt time.Time    `json:"reportedAt"`
	FiveHour   *UsageWindow `json:"fiveHour,omitempty"`
	SevenDay   *UsageWindow `json:"sevenDay,omitempty"`
}

type ProfileUsage struct {
	Profile   agentruntime.Profile
	ReadState string
	Report    *UsageReport
}

type UsageObservation struct {
	ObservedAt time.Time
	Profiles   []ProfileUsage
}

type usageReportJSON struct {
	ReportedAt string          `json:"reportedAt"`
	FiveHour   json.RawMessage `json:"fiveHour,omitempty"`
	SevenDay   json.RawMessage `json:"sevenDay,omitempty"`
}

// UnmarshalJSON establishes the shared normalized native helper/wire report.
func (report *UsageReport) UnmarshalJSON(encoded []byte) error {
	var wire usageReportJSON
	if err := strictjson.Decode(encoded, &wire); err != nil {
		return errors.New("invalid usage report")
	}
	accepted, err := parseUsageReport(wire)
	if err != nil {
		return err
	}
	*report = accepted
	return nil
}

func parseUsageReport(wire usageReportJSON) (UsageReport, error) {
	stamp, err := time.Parse(time.RFC3339Nano, wire.ReportedAt)
	if err != nil || stamp.Format(time.RFC3339Nano) != stamp.UTC().Format(time.RFC3339Nano) || stamp.IsZero() {
		return UsageReport{}, errors.New("invalid usage report time")
	}
	report := UsageReport{ReportedAt: stamp.UTC()}
	for _, entry := range []struct {
		encoded json.RawMessage
		target  **UsageWindow
	}{{wire.FiveHour, &report.FiveHour}, {wire.SevenDay, &report.SevenDay}} {
		if len(entry.encoded) == 0 {
			continue
		}
		var window *struct {
			UsedPercent *float64        `json:"usedPercent"`
			ResetsAt    json.RawMessage `json:"resetsAt,omitempty"`
		}
		if strictjson.Decode(entry.encoded, &window) != nil || window == nil || window.UsedPercent == nil || math.IsNaN(*window.UsedPercent) || math.IsInf(*window.UsedPercent, 0) || *window.UsedPercent < 0 {
			return UsageReport{}, errors.New("invalid usage window")
		}
		accepted := &UsageWindow{UsedPercent: *window.UsedPercent}
		if len(window.ResetsAt) != 0 {
			var encoded string
			if strictjson.Decode(window.ResetsAt, &encoded) != nil {
				return UsageReport{}, errors.New("invalid usage reset")
			}
			reset, err := time.Parse(time.RFC3339Nano, encoded)
			if err != nil || reset.Format(time.RFC3339Nano) != reset.UTC().Format(time.RFC3339Nano) || reset.IsZero() {
				return UsageReport{}, errors.New("invalid usage reset")
			}
			reset = reset.UTC()
			accepted.ResetsAt = &reset
		}
		*entry.target = accepted
	}
	return report, nil
}

// ProfileUsage owns each read's cancellation and completion. It never observes
// sessions or creates provider owners; absent/failed profiles remain in order.
func (service *Service) ProfileUsage(parent context.Context) (UsageObservation, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	revision := service.sessions.SettingsRevision()
	declared := service.sessions.Profiles()
	profiles := make([]agentruntime.Profile, 0, len(declared))
	for _, profile := range declared {
		if profile.Provider == agentruntime.ProviderCodex {
			profiles = append(profiles, profile)
		}
	}
	observed := make([]ProfileUsage, len(profiles))
	var pending sync.WaitGroup
	for index, profile := range profiles {
		pending.Go(func() {
			value := ProfileUsage{Profile: profile, ReadState: "unavailable"}
			if report := service.quota(ctx, profile); report != nil && ctx.Err() == nil {
				value.ReadState, value.Report = "ok", report
			}
			observed[index] = value
		})
	}
	pending.Wait()
	stamp := time.Now().UTC()
	for index := range observed {
		if report := observed[index].Report; report != nil && report.ReportedAt.After(stamp) {
			observed[index].ReadState, observed[index].Report = "unavailable", nil
		}
	}
	if service.sessions.SettingsRevision() != revision {
		return UsageObservation{}, errors.New("quota configuration changed")
	}
	return UsageObservation{ObservedAt: stamp, Profiles: observed}, nil
}
