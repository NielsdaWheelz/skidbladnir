package fleet

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// Run owns configuration synchronization and pending authority placement. It uses
// the existing revision stream and bounded reconnect backoff, never inventory
// polling or an expiry on disconnected cached trust. Composition owns its context.
func (s *Service) Run(ctx context.Context) error {
	s.mu.Lock()
	initialHub := s.identity.ActiveFleet != nil && s.public.ActiveHub == s.options.Machine && s.authority != nil && (s.authority.Transfer == nil || s.authority.Transfer.Phase == "active" || s.authority.Transfer.Phase == "aborted")
	s.mu.Unlock()
	if initialHub {
		_, e := s.Apply()
		if e != nil && ctx.Err() != nil {
			return ctx.Err()
		}
	}
	delay := time.Second
	for ctx.Err() == nil {
		s.mu.Lock()
		changed := s.changed
		enrolled := s.identity.ActiveFleet != nil
		selfHub := enrolled && s.public.ActiveHub == s.options.Machine
		target := ""
		if selfHub && s.authority != nil {
			if t := s.authority.Transfer; t != nil && (t.Phase == "preparing" || t.Phase == "fenced") {
				target = t.Target
			} else {
				for _, m := range s.public.Machines {
					if strings.EqualFold(m.Name, s.private.Settings.Hub) && m.Machine != s.options.Machine {
						target = m.Machine
					}
				}
			}
		}
		s.mu.Unlock()
		if !enrolled {
			select {
			case <-ctx.Done():
				return nil
			case <-changed:
				continue
			}
		}
		if selfHub && target == "" {
			select {
			case <-ctx.Done():
				return nil
			case <-changed:
				continue
			}
		}
		var e error
		if selfHub {
			e = s.TransferHub(ctx, target)
		} else {
			e = s.Refresh(ctx)
			if e == nil {
				e = s.streamHints(ctx)
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		if e == nil {
			delay = time.Second
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
	return nil
}
func (s *Service) streamHints(ctx context.Context) error {
	s.mu.Lock()
	a := s.identity.ActiveFleet
	if a == nil {
		s.mu.Unlock()
		return nil
	}
	association := *a
	localChanged := s.changed
	hub := Machine{}
	for _, m := range s.public.Machines {
		if m.Machine == s.public.ActiveHub {
			hub = m
		}
	}
	s.mu.Unlock()
	if hub.Origin == "" {
		return errors.New("hub has no reachable origin")
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-localChanged:
			cancel()
		case <-streamCtx.Done():
		}
	}()
	request, e := http.NewRequestWithContext(streamCtx, "GET", hub.Origin+"/v1/fleet/events", nil)
	if e != nil {
		return e
	}
	request.Header.Set("Authorization", "Bearer "+association.Bearer)
	request.Header.Set("Skidbladnir-Machine", hub.Machine)
	client := s.httpClient()
	client.Timeout = 0
	response, e := client.Do(request)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("fleet revision stream unavailable")
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var hint struct {
			AuthorityEpoch uint64 `json:"authorityEpoch"`
			StateRevision  uint64 `json:"stateRevision"`
		}
		if e = strictjson.Decode([]byte(strings.TrimPrefix(line, "data: ")), &hint); e != nil {
			return e
		}
		s.mu.Lock()
		changed := hint.AuthorityEpoch > s.public.AuthorityEpoch || hint.StateRevision > s.public.StateRevision
		s.mu.Unlock()
		if !changed {
			continue
		}
		if e = s.Refresh(ctx); e != nil {
			return e
		}
		s.mu.Lock()
		differentHub := s.public.ActiveHub != hub.Machine
		s.mu.Unlock()
		if differentHub {
			return nil
		}
	}
	if streamCtx.Err() != nil && ctx.Err() == nil {
		return nil
	}
	if e = scanner.Err(); e != nil {
		return e
	}
	return errors.New("fleet revision stream closed")
}
