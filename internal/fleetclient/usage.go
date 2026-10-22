package fleetclient

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type ProfileUsage struct {
	Profile
	ReadState string        `json:"readState"`
	Report    *quota.Report `json:"report,omitempty"`
}

type ProfileUsagePeer struct {
	Label      string
	Machine    string
	OK         bool
	Error      *Failure
	ObservedAt time.Time
	ReceivedAt time.Time
	Profiles   []ProfileUsage
}

// ProfileUsage reads every peer's usage, independently of inventories.
func (client *Client) ProfileUsage(parent context.Context) []ProfileUsagePeer {
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	peers := client.peerSnapshot()
	rows := make([]ProfileUsagePeer, len(peers))
	var pending sync.WaitGroup
	for index, selected := range peers {
		pending.Go(func() {
			result := client.call(ctx, selected, "profile_usage", "", nil)
			row := ProfileUsagePeer{Label: selected.Label, Machine: selected.Machine, OK: result.OK, Error: result.Error}
			if result.OK {
				row = result.Value.(ProfileUsagePeer)
			}
			rows[index] = row
		})
	}
	pending.Wait()
	return rows
}

func decodeProfileUsage(encoded []byte, target peer) (any, bool) {
	var wire *struct {
		Machine struct {
			Handle   string `json:"handle"`
			Platform string `json:"platform"`
		} `json:"machine"`
		ObservedAt string `json:"observedAt"`
		Profiles   []struct {
			Key          string        `json:"key"`
			Label        string        `json:"label"`
			Provider     string        `json:"provider"`
			AccountScope *string       `json:"accountScope,omitempty"`
			ReadState    string        `json:"readState"`
			Report       *quota.Report `json:"report,omitempty"`
		} `json:"profiles"`
	}
	if strictjson.Decode(encoded, &wire) != nil || wire == nil || wire.Machine.Handle != target.Machine || !slices.Contains([]string{"Linux", "Darwin"}, wire.Machine.Platform) || wire.Profiles == nil {
		return nil, false
	}
	stamp, err := sessions.ParseProjectionInstant(wire.ObservedAt)
	if err != nil {
		return nil, false
	}
	profiles := make([]ProfileUsage, len(wire.Profiles))
	for index, profile := range wire.Profiles {
		descriptor, valid := (hostProfile{Key: profile.Key, Label: profile.Label, Provider: profile.Provider, AccountScope: profile.AccountScope}).project()
		if !valid || descriptor.Provider != "Codex" {
			return nil, false
		}
		report := profile.Report
		if profile.ReadState != "ok" && profile.ReadState != "unavailable" || profile.ReadState == "unavailable" && report != nil {
			return nil, false
		}
		if report != nil {
			if !utcInstant(report.ReportedAt) || report.ReportedAt.After(stamp) {
				return nil, false
			}
			for _, window := range []*quota.Window{report.FiveHour, report.SevenDay} {
				if window != nil && (window.UsedPercent < 0 || window.ResetsAt != nil && !utcInstant(*window.ResetsAt)) {
					return nil, false
				}
			}
		}
		profiles[index] = ProfileUsage{Profile: descriptor, ReadState: profile.ReadState, Report: report}
	}
	return ProfileUsagePeer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: stamp, Profiles: profiles}, true
}

// utcInstant is a decoded wire instant: non-zero, with a zero utc offset.
func utcInstant(value time.Time) bool {
	return !value.IsZero() && value.Format(time.RFC3339Nano) == value.UTC().Format(time.RFC3339Nano)
}
