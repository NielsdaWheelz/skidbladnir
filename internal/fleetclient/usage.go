package fleetclient

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type ProfileUsage struct {
	Profile
	ReadState string                    `json:"readState"`
	Report    *agentcontrol.UsageReport `json:"report,omitempty"`
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

type ProfileUsageObservation struct {
	Peers []ProfileUsagePeer
}

// ProfileUsage reads only the named usage scope, independently of inventories.
func (client *Client) ProfileUsage(parent context.Context, machineScope string) Result {
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	peers := client.peerSnapshot()
	if machineScope != "" {
		selected, found := client.peerByLabel(machineScope)
		if !found {
			return Failed("machine_unknown", "not_sent")
		}
		peers = []peer{selected}
	}
	rows := make([]ProfileUsagePeer, len(peers))
	var pending sync.WaitGroup
	for index, selected := range peers {
		pending.Go(func() {
			result := client.call(ctx, selected, "profile_usage", "/v1/profile-usage", nil)
			row := ProfileUsagePeer{Label: selected.Label, Machine: selected.Machine, OK: result.OK, Error: result.Error}
			if result.OK {
				row = result.Value.(ProfileUsagePeer)
			}
			rows[index] = row
		})
	}
	pending.Wait()
	return success(ProfileUsageObservation{Peers: rows})
}

func decodeProfileUsage(encoded []byte, target peer) (any, bool) {
	var wire *struct {
		Machine struct {
			Handle   string `json:"handle"`
			Platform string `json:"platform"`
		} `json:"machine"`
		ObservedAt string `json:"observedAt"`
		Profiles   []struct {
			Key          string                    `json:"key"`
			Label        string                    `json:"label"`
			Provider     string                    `json:"provider"`
			AccountScope *string                   `json:"accountScope,omitempty"`
			ReadState    string                    `json:"readState"`
			Report       *agentcontrol.UsageReport `json:"report,omitempty"`
		} `json:"profiles"`
	}
	if strictjson.Decode(encoded, &wire) != nil || wire == nil || wire.Machine.Handle != target.Machine || !slices.Contains([]string{"Linux", "Darwin"}, wire.Machine.Platform) || wire.Profiles == nil {
		return nil, false
	}
	stamp, err := time.Parse(time.RFC3339Nano, wire.ObservedAt)
	if err != nil || stamp.IsZero() || stamp.Format(time.RFC3339Nano) != stamp.UTC().Format(time.RFC3339Nano) {
		return nil, false
	}
	profiles := make([]ProfileUsage, len(wire.Profiles))
	for index, profile := range wire.Profiles {
		descriptor, valid := (hostProfile{Key: profile.Key, Label: profile.Label, Provider: profile.Provider, AccountScope: profile.AccountScope}).project()
		if !valid || descriptor.Provider != "Codex" {
			return nil, false
		}
		if profile.ReadState != "ok" && profile.ReadState != "unavailable" || profile.ReadState == "unavailable" && profile.Report != nil || profile.Report != nil && profile.Report.ReportedAt.After(stamp) {
			return nil, false
		}
		profiles[index] = ProfileUsage{Profile: descriptor, ReadState: profile.ReadState, Report: profile.Report}
	}
	return ProfileUsagePeer{Label: target.Label, Machine: target.Machine, OK: true, ObservedAt: stamp.UTC(), Profiles: profiles}, true
}
