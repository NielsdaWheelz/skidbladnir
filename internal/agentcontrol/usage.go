package agentcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
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
	Source    string
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

// UnmarshalJSON establishes the shared normalized helper/file/wire report.
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
	profiles := service.sessions.Profiles()
	observed := make([]ProfileUsage, len(profiles))
	failures := make([]error, len(profiles))
	var pending sync.WaitGroup
	for index, profile := range profiles {
		pending.Go(func() {
			value := ProfileUsage{Profile: profile, ReadState: "unavailable"}
			var report *UsageReport
			var err error
			switch profile.Provider {
			case agentruntime.ProviderCodex:
				value.Source = "native"
				var result UsageReport
				err = service.native(ctx, profile, "usage", nil, nil, &result)
				if err == nil {
					report = &result
				}
			case agentruntime.ProviderClaude:
				value.Source = "statusline"
				report, err = readClaudeUsage(profile)
			default:
				panic("usage profile has an unknown provider") // justify-defect: manager construction closes the profile provider union.
			}
			var unavailable *UnavailableError
			if err != nil && !errors.As(err, &unavailable) {
				failures[index] = err
			}
			if err == nil && ctx.Err() == nil {
				value.ReadState, value.Report = "ok", report
			}
			observed[index] = value
		})
	}
	pending.Wait()
	stamp := time.Now().UTC()
	for index, err := range failures {
		if err != nil {
			return UsageObservation{}, err
		}
		if report := observed[index].Report; report != nil && report.ReportedAt.After(stamp) {
			observed[index].ReadState, observed[index].Report = "unavailable", nil
		}
	}
	return UsageObservation{ObservedAt: stamp, Profiles: observed}, nil
}

func readClaudeUsage(profile agentruntime.Profile) (*UsageReport, error) {
	home := ""
	for _, entry := range profile.Environment {
		if entry.Name == "CLAUDE_CONFIG_DIR" {
			home = entry.Value
			break
		}
	}
	file, err := os.Open(filepath.Join(home, "skidbladnir-usage.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &UnavailableError{Dispatch: "not_sent"}
	}
	defer file.Close()
	encoded, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(encoded) > 4096 {
		return nil, &UnavailableError{Dispatch: "not_sent"}
	}
	// Read the version before accepting the single v1 shape. Incompatible
	// snapshots are unavailable. justify-defect: malformed publications violate
	// our single owned writer's contract and retain the gateway's internal error.
	var fields map[string]json.RawMessage
	if strictjson.Decode(encoded, &fields) != nil || fields == nil || len(fields["schemaVersion"]) == 0 {
		return nil, errors.New("invalid statusline usage publication")
	}
	var schema *int
	if strictjson.Decode(fields["schemaVersion"], &schema) != nil || schema == nil {
		return nil, errors.New("invalid statusline usage publication version")
	}
	if *schema != 1 {
		return nil, &UnavailableError{Dispatch: "not_sent"}
	}
	var wire struct {
		SchemaVersion int             `json:"schemaVersion"`
		ReportedAt    string          `json:"reportedAt"`
		FiveHour      json.RawMessage `json:"fiveHour,omitempty"`
		SevenDay      json.RawMessage `json:"sevenDay,omitempty"`
	}
	if strictjson.Decode(encoded, &wire) != nil {
		return nil, errors.New("invalid statusline usage publication")
	}
	report, err := parseUsageReport(usageReportJSON{ReportedAt: wire.ReportedAt, FiveHour: wire.FiveHour, SevenDay: wire.SevenDay})
	if err != nil {
		return nil, err
	}
	return &report, nil
}
