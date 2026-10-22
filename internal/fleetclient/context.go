package fleetclient

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

func (session Session) Current(peer Peer) ExecutionContext {
	if session.Execution != nil {
		return *session.Execution
	}
	if session.Connection != nil {
		return ExecutionContext{Kind: "remoteUnknown"}
	}
	result := ExecutionContext{Kind: "local", Machine: peer.Machine, Label: peer.Label, CWD: session.CWD}
	if session.Agent != nil {
		result.Agent = &ExecutionAgent{Provider: session.Agent.Provider, Profile: session.Agent.Profile, Label: session.Agent.Label}
	}
	return result
}

// EnrichContexts returns source with display context on each remote row: where
// its connection chain ends (≤8 hops, cycle stop, 2 s), else remoteUnknown.
// peers supplies the peers not to query (a failed read). A failed source is
// returned as is. Each connection id is asked once per pass. It never changes
// its arguments.
func (client *Client) EnrichContexts(parent context.Context, source Peer, peers []Peer) Peer {
	if !source.OK {
		return source
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	var lookups sync.Map // connection id -> func() (peer, *api.TerminalContext)
	skip := make(map[string]bool, len(peers))
	for _, peer := range peers {
		skip[peer.Machine] = peer.Error != nil && !peer.OK
	}
	source.Sessions = slices.Clone(source.Sessions)
	var pending sync.WaitGroup
	for index, session := range source.Sessions {
		if session.Connection == nil {
			continue
		}
		pending.Go(func() {
			execution := ExecutionContext{Kind: "remoteUnknown"}
			defer func() { source.Sessions[index].Execution = &execution }()
			seen := make(map[string]bool)
			id := session.Connection.ID
			for depth := 0; depth < 8 && id != ""; depth++ {
				if seen[id] || ctx.Err() != nil {
					return
				}
				seen[id] = true
				key := id
				lookup, _ := lookups.LoadOrStore(key, sync.OnceValues(func() (peer, *api.TerminalContext) { return client.lookupContext(ctx, key, skip) }))
				target, remote := lookup.(func() (peer, *api.TerminalContext))()
				if remote == nil {
					return
				}
				if remote.Connection != nil {
					id = remote.Connection.ID
					continue
				}
				execution = ExecutionContext{Kind: "remote", Machine: target.Machine, Label: target.Label, CWD: remote.CWD}
				if agent := remote.Agent; agent != nil {
					label := agent.Label
					if label == "" {
						label = agent.Profile
					}
					execution.Agent = &ExecutionAgent{Provider: agent.Provider, Profile: agent.Profile, Label: label}
				}
				return
			}
		})
	}
	pending.Wait()
	return source
}

// lookupContext asks every peer not skipped for one connection id. Exactly one
// answering peer is a match; none or several is not found (nil).
func (client *Client) lookupContext(ctx context.Context, id string, skip map[string]bool) (peer, *api.TerminalContext) {
	peers := client.peerSnapshot()
	answers := make([]*api.TerminalContext, len(peers))
	var pending sync.WaitGroup
	for index, target := range peers {
		if skip[target.Machine] {
			continue
		}
		pending.Go(func() {
			result := client.call(ctx, target, "terminal_context", id, nil)
			if result.OK {
				answer := result.Value.(api.TerminalContext)
				answers[index] = &answer
			}
		})
	}
	pending.Wait()
	match := -1
	for index, answer := range answers {
		if answer == nil {
			continue
		}
		if match != -1 {
			return peer{}, nil
		}
		match = index
	}
	if match == -1 {
		return peer{}, nil
	}
	return peers[match], answers[match]
}
