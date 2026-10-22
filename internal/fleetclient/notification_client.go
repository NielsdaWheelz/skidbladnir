package fleetclient

import (
	"bufio"
	"context"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
)

// notificationRequest targets the hub, which fleet.json always lists.
func (client *Client) notificationRequest(ctx context.Context, path string) (*http.Request, error) {
	_ = client.Refresh() // justify-ignore-error: preserve the admitted cache while its durable replacement is unavailable.
	client.mu.RLock()
	hub := client.hub
	client.mu.RUnlock()
	target, _ := client.peerByMachine(hub)
	request, err := fleet.NewRequest(ctx, target.Target, http.MethodGet, "/v1/notifications/"+path, nil)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	return request, nil
}
func (client *Client) NotificationState(ctx context.Context) (attention.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := client.notificationRequest(ctx, "state")
	if err != nil {
		return attention.Snapshot{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return attention.Snapshot{}, attention.ErrUnavailable
	}
	defer response.Body.Close()
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || response.StatusCode != http.StatusOK {
		return attention.Snapshot{}, attention.ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, attention.MaximumSnapshotBytes+1))
	var snapshot attention.Snapshot
	if err != nil || len(data) > attention.MaximumSnapshotBytes || attention.Decode(data, &snapshot) != nil || !snapshot.Valid() {
		return attention.Snapshot{}, attention.ErrUnavailable
	}
	if len(snapshot.Machines) != len(client.Machines()) {
		return attention.Snapshot{}, attention.ErrUnavailable
	}
	for _, host := range snapshot.Machines {
		if _, found := client.peerByMachine(host.Machine); !found {
			return attention.Snapshot{}, attention.ErrUnavailable
		}
	}
	return snapshot, nil
}

// NotificationEvents owns one cancellable stream attempt; reconnect belongs to the app.
func (client *Client) NotificationEvents(ctx context.Context, consume func(attention.Hint) error) error {
	request, err := client.notificationRequest(ctx, "events")
	if err != nil {
		return err
	}
	streaming := *client.http
	streaming.Timeout = 0
	response, err := streaming.Do(request)
	if err != nil {
		return attention.ErrUnavailable
	}
	defer response.Body.Close()
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "text/event-stream" || response.StatusCode != http.StatusOK {
		return attention.ErrUnavailable
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 512), 512)
	event, data := "", ""
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if event != "revision" || data == "" || len(data) > attention.MaximumHintBytes {
				return attention.ErrUnavailable
			}
			var hint attention.Hint
			if attention.Decode([]byte(data), &hint) != nil || !hint.Valid() {
				return attention.ErrUnavailable
			}
			if err := consume(hint); err != nil {
				return err
			}
			event, data = "", ""
			continue
		}
		if value, found := strings.CutPrefix(line, "event: "); found && event == "" {
			event = value
			continue
		}
		if value, found := strings.CutPrefix(line, "data: "); found && data == "" {
			data = value
			continue
		}
		return attention.ErrUnavailable
	}
	return attention.ErrUnavailable
}
