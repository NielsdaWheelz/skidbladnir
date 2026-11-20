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
	Claude    *api.ClaudeUsage `json:"claude,omitempty"`
}

type ProfileUsagePeer struct {
	Label               string
	Machine             string
	OK                  bool
	Error               *Failure
	ObservedAt          time.Time
	ReceivedAt          time.Time
	Profiles            []ProfileUsage
	OtherClaudeAccounts []api.ClaudeUsage
}

// ProfileUsage reads every peer's usage, independently of inventories.
func (client *Client) ProfileUsage(parent context.Context) []ProfileUsagePeer {
	return client.readUsage(parent, "profile_usage")
}

// RunClaudeUsage keeps local history current while both displays are closed.
// It pulls every enrolled gateway at startup and once a minute until shutdown.
func (client *Client) RunClaudeUsage(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for ctx.Err() == nil {
		client.readUsage(ctx, "claude_usage")
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (client *Client) readUsage(parent context.Context, operation string) []ProfileUsagePeer {
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	peers := client.peerSnapshot()
	rows := make([]ProfileUsagePeer, len(peers))
	var pending sync.WaitGroup
	for index, selected := range peers {
		pending.Go(func() {
			result := client.call(ctx, selected, operation, "", nil)
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
