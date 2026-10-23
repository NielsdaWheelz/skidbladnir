package fleetclient

import (
	"context"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

type ProfileUsage struct {
	Profile
	ReadState string           `json:"readState"`
	Report    *api.QuotaReport `json:"report,omitempty"`
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
