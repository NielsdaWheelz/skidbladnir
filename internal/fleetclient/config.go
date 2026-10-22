package fleetclient

import (
	"context"
	"errors"
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

var errConfiguration = errors.New("fleet state is invalid or unavailable; check skid status")
var errOutputLimit = errors.New("output limit")

type peer struct{ Label, Origin, Machine, Bearer string }

// Open reads installed identity and verified fleet state from the state directory.
func Open(directory string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.Proxy = nil
	client := &Client{directory: directory, http: &http.Client{Transport: transport, Timeout: Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if err := client.Refresh(); err != nil {
		return nil, err
	}
	return client, nil
}

// Refresh adopts the host service's durable snapshot; it never writes host state.
func (client *Client) Refresh() error {
	client.refreshMutex.Lock()
	defer client.refreshMutex.Unlock()
	identity, err := fleet.LoadIdentity(client.directory)
	if err != nil || identity.Schema != 1 {
		return errConfiguration
	}
	if _, err := machine.Parse(identity.Machine); err != nil {
		return errConfiguration
	}
	if _, err := auth.ParseCredential(identity.LocalBearer); err != nil {
		return errConfiguration
	}
	settings, _, err := fleet.LoadResolved(client.directory)
	if err != nil {
		return errConfiguration
	}
	peers := []peer{}
	hub, defaultMachine := "", identity.Machine
	var public fleet.Public
	var envelope fleet.Envelope
	if association := identity.ActiveFleet; association != nil {
		key, err := fleet.ParsePublicKey(association.PublicKey)
		if err != nil || fleet.Fingerprint(key) != association.FleetID {
			return errConfiguration
		}
		if _, err := auth.ParseCredential(association.Bearer); err != nil {
			return errConfiguration
		}
		state, err := fleet.LoadHostState(client.directory)
		if err != nil {
			return errConfiguration
		}
		public, err = fleet.VerifyPublic(key, state.Public)
		if err != nil {
			return errConfiguration
		}
		envelope = state.Public
		client.mu.RLock()
		previous, previousEnvelope := client.public, client.envelope
		client.mu.RUnlock()
		if previous.FleetID == public.FleetID && (public.AuthorityEpoch < previous.AuthorityEpoch || public.StateRevision < previous.StateRevision || public.ConfigRevision < previous.ConfigRevision || public.AuthorityEpoch == previous.AuthorityEpoch && public.StateRevision == previous.StateRevision && envelope != previousEnvelope) {
			return errConfiguration
		}
		hub = public.ActiveHub
		foundSelf := false
		for _, target := range public.Machines {
			origin, bearer := target.Origin, association.Bearer
			if target.Machine == identity.Machine {
				origin, bearer, foundSelf = "http://127.0.0.1:7341", identity.LocalBearer, true
			}
			peers = append(peers, peer{target.Name, origin, target.Machine, bearer})
			if target.Name == settings.DefaultMachine {
				defaultMachine = target.Machine
			}
		}
		if !foundSelf {
			return errConfiguration
		}
	} else {
		// Detached installations retain launch settings and route only to themselves.
		label := identity.Name
		peers = append(peers, peer{label, "http://127.0.0.1:7341", identity.Machine, identity.LocalBearer})
		hub = identity.Machine
	}
	client.mu.Lock()
	client.peers, client.defaultMachine, client.hub = peers, defaultMachine, hub
	client.identity, client.public, client.envelope = identity, public, envelope
	client.mu.Unlock()
	return nil
}

// Apply requests application of the hub's human configuration without uploading a cache.
func (client *Client) Apply(ctx context.Context) error {
	client.mu.RLock()
	identity := client.identity
	client.mu.RUnlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:7341/v1/local/apply", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+identity.LocalBearer)
	request.Header.Set("Skidbladnir-Machine", identity.Machine)
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errConfiguration
	}
	return nil
}
func (client *Client) peerSnapshot() []peer {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return append([]peer{}, client.peers...)
}

type Machine struct{ Label, Handle string }

func (client *Client) Machines() []Machine {
	peers := client.peerSnapshot()
	result := make([]Machine, 0, len(peers))
	for _, target := range peers {
		result = append(result, Machine{target.Label, target.Machine})
	}
	return result
}
func (client *Client) DefaultMachine() Machine {
	client.mu.RLock()
	defer client.mu.RUnlock()
	for _, target := range client.peers {
		if target.Machine == client.defaultMachine {
			return Machine{target.Label, target.Machine}
		}
	}
	panic("validated default machine is absent") // justify-defect: Refresh always includes the installed machine.
}
