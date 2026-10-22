package fleetclient

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
)

// NotificationForeground is the exact agent lifetime attention binds to.
func NotificationForeground(session Session) *attention.Foreground {
	if session.Agent == nil || session.Connection != nil {
		return nil
	}
	return &attention.Foreground{Provider: session.Agent.Provider, PID: session.Agent.PID, StartIdentity: session.Agent.StartIdentity}
}

// notificationHub is the hub's route; fleet.json always lists the hub.
func (client *Client) notificationHub() fleet.Target {
	_ = client.Refresh() // an unreadable fleet.json keeps the admitted routes
	client.mu.RLock()
	hub := client.hub
	client.mu.RUnlock()
	target, _ := client.peerByMachine(hub)
	return target.Target
}

// NotificationState reads the hub's verdicts. The hub ships in the same
// release and owns their invariants, so nothing here re-checks them.
func (client *Client) NotificationState(ctx context.Context) (attention.State, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var state attention.State
	if fleet.Call(ctx, client.notificationHub(), http.MethodGet, "/v1/notifications/state", nil, &state) != nil {
		return attention.State{}, attention.ErrUnavailable
	}
	return state, nil
}

// NotificationEvents holds one hint stream open and calls wake for every
// event; reconnecting belongs to the caller. It returns when the stream ends.
func (client *Client) NotificationEvents(ctx context.Context, wake func()) error {
	request, err := fleet.NewRequest(ctx, client.notificationHub(), http.MethodGet, "/v1/notifications/events", nil)
	if err != nil {
		return attention.ErrUnavailable
	}
	streaming := *client.http
	streaming.Timeout = 0
	response, err := streaming.Do(request)
	if err != nil {
		return attention.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return attention.ErrUnavailable
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 512), 512)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data:") {
			wake()
		}
	}
	return attention.ErrUnavailable
}

// NotificationHandled sends the owner's acknowledgement to the hub.
func (client *Client) NotificationHandled(ctx context.Context, handled attention.Handled) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if fleet.Call(ctx, client.notificationHub(), http.MethodPost, "/v1/notifications/handled", handled, nil) != nil {
		return attention.ErrUnavailable
	}
	return nil
}
