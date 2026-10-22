package fleetclient

import (
	"context"
	"errors"
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
)

var errConfiguration = errors.New("fleet state is invalid or unavailable; check skid status")
var errOutputLimit = errors.New("output limit")

type peer struct {
	Label string
	fleet.Target
}

// Open reads this installation's fleet from its state directory.
func Open(directory string) (*Client, error) {
	local, err := fleet.ReadLocal(directory)
	if err != nil {
		return nil, errConfiguration
	}
	return New(directory, local), nil
}

// New routes by local without reading the state directory; Refresh reads it.
func New(directory string, local fleet.Local) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.Proxy = nil
	client := &Client{directory: directory, http: &http.Client{Transport: transport, Timeout: Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	client.Adopt(local)
	return client
}

// Adopt replaces every route with one snapshot of this installation's fleet.
func (client *Client) Adopt(local fleet.Local) {
	peers := make([]peer, 0, len(local.Machines))
	defaultMachine := Machine{local.Name, local.Machine}
	for _, m := range local.Machines {
		peers = append(peers, peer{m.Name, local.Target(m.Machine)})
		if m.Name == local.Settings.DefaultMachine {
			defaultMachine = Machine{m.Name, m.Machine}
		}
	}
	client.mu.Lock()
	client.peers, client.defaultMachine, client.hub, client.self = peers, defaultMachine, local.Hub, local.Target(local.Machine)
	client.mu.Unlock()
}

// Refresh adopts the installed service's current fleet file; it never writes host state.
func (client *Client) Refresh() error {
	local, err := fleet.ReadLocal(client.directory)
	if err != nil {
		return errConfiguration
	}
	client.Adopt(local)
	return nil
}

// Apply asks the installed service to apply the hub's human configuration.
func (client *Client) Apply(ctx context.Context) error {
	client.mu.RLock()
	self := client.self
	client.mu.RUnlock()
	return fleet.Call(ctx, self, http.MethodPost, "/v1/local/apply", nil, nil)
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
	return client.defaultMachine
}
