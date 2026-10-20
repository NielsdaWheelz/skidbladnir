package fleetclient

import (
	"context"
	"sync"
	"time"
)

type RemoteContext struct {
	ObservedAt string       `json:"observedAt"`
	CWD        string       `json:"cwd,omitempty"`
	Agent      *RemoteAgent `json:"agent,omitempty"`
	Connection *Connection  `json:"connection,omitempty"`
}

type RemoteAgent struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile,omitempty"`
}

func validConnectionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, digit := range []byte(id) {
		if digit < '0' || digit > '9' {
			if digit < 'a' || digit > 'f' {
				return false
			}
		}
	}
	return true
}

func (session Session) Current(peer Peer) ExecutionContext {
	if session.Execution != nil {
		return *session.Execution
	}
	if session.Connection != nil {
		return ExecutionContext{Kind: "remoteUnknown"}
	}
	result := ExecutionContext{Kind: "local", Machine: peer.Machine, Label: peer.Label, CWD: session.CWD}
	if session.Agent != nil {
		result.Agent = &ExecutionAgent{
			Provider: session.Agent.Provider,
			Profile:  session.Agent.Profile,
			Label:    profileLabel(peer.Profiles, session.Agent.Profile),
		}
	}
	return result
}

func profileLabel(profiles []Profile, key string) string {
	for _, profile := range profiles {
		if profile.Key == key {
			return profile.Label
		}
	}
	return ""
}

// ContextObservation changes display context only, for the exact source capture.
type ContextObservation struct {
	SourceRef    string
	ConnectionID string
	Execution    ExecutionContext
}

// EnrichContexts returns immutable overlays without changing the source inventory.
func (client *Client) EnrichContexts(parent context.Context, source Peer, catalogues []Peer) []ContextObservation {
	if !source.OK {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	var cache sync.Map
	profiles := make(map[string][]Profile, len(catalogues))
	unavailable := make(map[string]bool)
	for _, catalogue := range catalogues {
		profiles[catalogue.Machine] = catalogue.Profiles
		unavailable[catalogue.Machine] = catalogue.Error != nil && !catalogue.OK
	}
	rows := []Session{}
	for _, session := range source.Sessions {
		if session.Connection != nil {
			rows = append(rows, session)
		}
	}
	observations := make([]ContextObservation, len(rows))
	var pending sync.WaitGroup
	for index, session := range rows {
		pending.Go(func() {
			observation := ContextObservation{SourceRef: session.Ref, ConnectionID: session.Connection.ID, Execution: ExecutionContext{Kind: "remoteUnknown"}}
			defer func() { observations[index] = observation }()
			seen := make(map[string]bool)
			id := session.Connection.ID
			for depth := 0; depth < 8 && id != ""; depth++ {
				if seen[id] || ctx.Err() != nil {
					return
				}
				seen[id] = true
				entry, _ := cache.LoadOrStore(id, &contextLookup{})
				lookup := entry.(*contextLookup)
				lookup.once.Do(func() { lookup.result = client.lookupContext(ctx, id, unavailable) })
				match := lookup.result
				if !match.found {
					return
				}
				if match.context.Connection != nil {
					id = match.context.Connection.ID
					continue
				}
				observation.Execution = ExecutionContext{Kind: "remote", Machine: match.peer.Machine, Label: match.peer.Label, CWD: match.context.CWD}
				if agent := match.context.Agent; agent != nil {
					label := profileLabel(profiles[match.peer.Machine], agent.Profile)
					if label == "" {
						label = agent.Profile
					}
					observation.Execution.Agent = &ExecutionAgent{Provider: agent.Provider, Profile: agent.Profile, Label: label}
				}
				return
			}
		})
	}
	pending.Wait()
	return observations
}

// Complete CLI listings wait for each source's optional display pass.
func (client *Client) resolveContexts(ctx context.Context, peers []Peer) {
	var pending sync.WaitGroup
	limit := make(chan struct{}, 3)
	for index, source := range peers {
		pending.Go(func() {
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-limit }()
			for _, observation := range client.EnrichContexts(ctx, source, peers) {
				for sessionIndex := range peers[index].Sessions {
					session := &peers[index].Sessions[sessionIndex]
					if session.Ref == observation.SourceRef && session.Connection != nil && session.Connection.ID == observation.ConnectionID {
						execution := observation.Execution
						session.Execution = &execution
					}
				}
			}
		})
	}
	pending.Wait()
}

type contextLookup struct {
	once   sync.Once
	result resolvedContext
}

type resolvedContext struct {
	peer    peer
	context RemoteContext
	found   bool
}

func (client *Client) lookupContext(ctx context.Context, id string, unavailable map[string]bool) resolvedContext {
	peers := client.peerSnapshot()
	results := make([]resolvedContext, len(peers))
	var pending sync.WaitGroup
	for index, target := range peers {
		if unavailable[target.Machine] {
			continue
		}
		pending.Go(func() {
			result := client.call(ctx, target, "terminal_context", "/v1/terminal-contexts/"+id, nil)
			if result.OK {
				results[index] = resolvedContext{peer: target, context: result.Value.(RemoteContext), found: true}
			}
		})
	}
	pending.Wait()
	match := resolvedContext{}
	for _, result := range results {
		if !result.found {
			continue
		}
		if match.found {
			return resolvedContext{}
		}
		match = result
	}
	return match
}

func (client *Client) resolveObserved(ctx context.Context, observed *ObservedSession) {
	peer, ok := client.peerByMachine(observed.Machine)
	if !ok {
		return
	}
	rows := []Peer{{Label: peer.Label, Machine: peer.Machine, OK: true, Sessions: []Session{observed.Session}}}
	client.resolveContexts(ctx, rows)
	observed.Session = rows[0].Sessions[0]
}
