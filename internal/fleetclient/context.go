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
	result := ExecutionContext{Kind: "local", Machine: peer.Machine, Label: peer.Label, CWD: session.CWD}
	if session.Agent != nil {
		result.Agent = &ExecutionAgent{
			Provider: session.Agent.Provider,
			Profile:  session.Agent.Profile,
			Label:    profileLabel(peer.Profiles, session.Agent.Profile),
			State:    session.Agent.Status.State,
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

// resolveContexts enriches display only. source refs and action fields remain untouched.
func (client *Client) resolveContexts(parent context.Context, peers []Peer) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	var cache sync.Map
	var catalogues sync.Map
	unavailable := make(map[string]bool)
	for _, peer := range peers {
		if peer.OK {
			catalogues.Store(peer.Machine, peer.Profiles)
		} else {
			unavailable[peer.Machine] = true
		}
	}
	var pending sync.WaitGroup
	for peerIndex := range peers {
		if !peers[peerIndex].OK {
			continue
		}
		for sessionIndex := range peers[peerIndex].Sessions {
			session := &peers[peerIndex].Sessions[sessionIndex]
			if session.Connection == nil {
				continue
			}
			pending.Go(func() {
				session.Execution = &ExecutionContext{Kind: "remoteUnknown"}
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
					entry, present := catalogues.Load(match.peer.Machine)
					if !present {
						var profiles []Profile
						listed := client.call(ctx, match.peer, "list", "/v1/sessions", nil)
						if listed.OK {
							profiles = listed.Value.(Peer).Profiles
						}
						entry, _ = catalogues.LoadOrStore(match.peer.Machine, profiles)
					}
					profiles := entry.([]Profile)
					execution := &ExecutionContext{Kind: "remote", Machine: match.peer.Machine, Label: match.peer.Label, CWD: match.context.CWD}
					if match.context.Agent != nil {
						execution.Agent = &ExecutionAgent{Provider: match.context.Agent.Provider, Profile: match.context.Agent.Profile, Label: profileLabel(profiles, match.context.Agent.Profile), State: "unknown"}
					}
					session.Execution = execution
					return
				}
			})
		}
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
	results := make([]resolvedContext, len(client.peers))
	var pending sync.WaitGroup
	for index, target := range client.peers {
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
