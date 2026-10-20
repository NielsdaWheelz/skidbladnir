package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

func (s *Service) Handler() http.Handler { return http.HandlerFunc(s.serveHTTP) }
func (s *Service) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawPath != "" || r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeHTTPError(w, invalidRequest())
		return
	}
	if r.Method == "GET" {
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
	}
	if code := auth.BindMachine(r, s.options.Machine); code != "" {
		status, message := code.Response()
		writeHTTPError(w, &Error{Code: string(code), Message: message, Status: status})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/local/") {
		s.serveLocal(w, r)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/v1/fleet/join" {
		var request JoinRequest
		if e := readBody(r, &request); e != nil {
			writeHTTPError(w, e)
			return
		}
		state, e := s.Join(request, time.Now())
		if e != nil {
			writeHTTPError(w, e)
			return
		}
		if request.Kind == "phone" {
			writeJSON(w, 201, state.Public)
		} else {
			writeJSON(w, 201, state)
		}
		return
	}
	member, _, e := s.Authenticate(r)
	if e != nil {
		writeHTTPError(w, e)
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/fleet":
		state, e := s.Snapshot()
		respond(w, state.Public, e)
	case r.Method == "GET" && r.URL.Path == "/v1/fleet/host-state":
		if member.Kind != "computer" {
			writeHTTPError(w, &Error{Code: "Forbidden", Message: "computer membership required", Status: 403})
			return
		}
		state, e := s.Snapshot()
		respond(w, state, e)
	case r.Method == "GET" && r.URL.Path == "/v1/fleet/events":
		s.events(w, r)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/apply":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.ApplyContext(r.Context())
		respond(w, status, e)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/invites":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		invite, e := s.Invite(time.Now())
		respond(w, invite, e)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/fleet/members/"):
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/fleet/members/")
		if !ValidDeviceID(id) {
			writeHTTPError(w, invalidRequest())
			return
		}
		status, e := s.Remove(id)
		respond(w, status, e)
	case r.Method == "PUT" && r.URL.Path == "/v1/fleet/self":
		var report SelfReport
		if e := readBody(r, &report); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.Report(member, report)
		respond(w, status, e)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/handoff/prepare":
		var bundle TransferBundle
		if e := readBody(r, &bundle); e != nil {
			writeHTTPError(w, e)
			return
		}
		transfer, e := s.Stage(member, bundle)
		respond(w, transfer, e)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/handoff/activate":
		var activation Envelope
		if e := readBody(r, &activation); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.ActivateTransfer(member, activation)
		respond(w, status, e)

	case r.Method == "POST" && r.URL.Path == "/v1/fleet/handoff/abort":
		var abort Envelope
		if e := readBody(r, &abort); e != nil {
			writeHTTPError(w, e)
			return
		}
		e := s.AbortStaged(member, abort)
		respond(w, struct{}{}, e)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/handoff/status":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		transfer, e := s.TransferStatus()
		if transfer != nil && member.Machine != transfer.Source && member.Machine != transfer.Target {
			writeHTTPError(w, &Error{Code: "Forbidden", Message: "designated transfer member required", Status: 403})
			return
		}
		respond(w, transfer, e)
	default:
		http.NotFound(w, r)
	}
}
func (s *Service) serveLocal(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.admissionUncertain {
		s.mu.Unlock()
		writeHTTPError(w, &Error{Code: "ControlUnavailable", Message: "identity write uncertain; restart to reconcile durable state", Status: 503})
		return
	}
	credential, e := auth.ParseCredential(s.identity.LocalBearer)
	s.mu.Unlock()
	headers := r.Header.Values("Authorization")
	if e != nil || len(headers) != 1 || !credential.Verify(headers[0]) {
		writeHTTPError(w, &Error{Code: "Unauthenticated", Message: "installed local credential required", Status: 401})
		return
	}
	switch {

	case r.Method == "PUT" && r.URL.Path == "/v1/local/network":
		var input struct {
			Origin string `json:"origin"`
		}
		if e := readBody(r, &input); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.SetOrigin(r.Context(), input.Origin)
		respond(w, status, e)
	case r.Method == "GET" && r.URL.Path == "/v1/local/status":
		writeJSON(w, 200, s.Status())
	case r.Method == "POST" && r.URL.Path == "/v1/local/join":
		var input struct {
			Invitation Invitation `json:"invitation"`
			Name       string     `json:"name"`
		}
		if e := readBody(r, &input); e != nil {
			writeHTTPError(w, e)
			return
		}
		e := s.Enroll(r.Context(), input.Invitation, input.Name)
		respond(w, s.Status(), e)

	case r.Method == "POST" && r.URL.Path == "/v1/local/refresh":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		e := s.Refresh(r.Context())
		respond(w, s.Status(), e)
	case r.Method == "POST" && r.URL.Path == "/v1/local/apply":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.ApplyContext(r.Context())
		respond(w, status, e)
	case r.Method == "POST" && r.URL.Path == "/v1/local/recover":
		if e := emptyBody(r); e != nil {
			writeHTTPError(w, e)
			return
		}
		status, e := s.Recover()
		respond(w, status, e)
	default:
		http.NotFound(w, r)
	}
}
func (s *Service) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHTTPError(w, errors.New("streaming unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	for {
		s.mu.Lock()
		if _, _, e := s.authenticate(r); e != nil {
			s.mu.Unlock()
			return
		}
		p := s.public
		changed := s.changed
		s.mu.Unlock()
		data, _ := json.Marshal(struct {
			AuthorityEpoch uint64 `json:"authorityEpoch"`
			StateRevision  uint64 `json:"stateRevision"`
		}{p.AuthorityEpoch, p.StateRevision})
		if _, e := fmt.Fprintf(w, "event: revision\ndata: %s\n\n", data); e != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-changed:
		}
	}
}
func invalidRequest() error {
	return &Error{Code: "InvalidRequest", Message: "the request is not valid", Status: 400}
}
func readBody(r *http.Request, target any) error {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" || r.Header.Get("Content-Encoding") != "" {
		return invalidRequest()
	}
	limit := hostconfig.MaximumBytes
	if strings.HasPrefix(r.URL.Path, "/v1/fleet/handoff/") {
		limit = MaximumStateBytes * 3
	}
	data, e := io.ReadAll(io.LimitReader(r.Body, int64(limit+1)))
	if e != nil {
		return invalidRequest()
	}
	if len(data) > limit {
		return &Error{Code: "RequestTooLarge", Message: "the request is too large", Status: 413}
	}
	if e = strictjson.Decode(data, target); e != nil {
		return invalidRequest()
	}
	return nil
}
func emptyBody(r *http.Request) error {
	if r.Header.Get("Content-Type") != "" || r.Header.Get("Content-Encoding") != "" || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		return invalidRequest()
	}
	data, e := io.ReadAll(io.LimitReader(r.Body, 1))
	if e != nil || len(data) != 0 {
		return invalidRequest()
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func respond(w http.ResponseWriter, value any, e error) {
	if e != nil {
		writeHTTPError(w, e)
		return
	}
	writeJSON(w, 200, value)
}
func writeHTTPError(w http.ResponseWriter, e error) {
	var typed *Error
	if !errors.As(e, &typed) {
		typed = &Error{Code: "InternalError", Message: "skid could not complete the request", Status: 500}
	}
	payload := struct {
		Code    string    `json:"code"`
		Message string    `json:"message"`
		Routing *Envelope `json:"routing,omitempty"`
		Removal *Envelope `json:"removal,omitempty"`
	}{typed.Code, typed.Message, typed.Routing, typed.Removal}
	writeJSON(w, typed.Status, payload)
}
func (s *Service) httpClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	if s.options.HTTPClient != nil {
		copy := *s.options.HTTPClient
		client = &copy
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if client.Timeout <= 0 || client.Timeout > 15*time.Second {
		client.Timeout = 15 * time.Second
	}
	return client
}
func (s *Service) request(ctx context.Context, origin, machine, bearer, method, path string, body any, result any) error {
	var encoded []byte
	var e error
	if body != nil {
		encoded, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	request, e := http.NewRequestWithContext(ctx, method, origin+path, bytes.NewReader(encoded))
	if e != nil {
		return e
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set("Skidbladnir-Machine", machine)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, e := s.httpClient().Do(request)
	if e != nil {
		return e
	}
	data, e := io.ReadAll(io.LimitReader(response.Body, MaximumStateBytes*3+1))
	e = errors.Join(e, response.Body.Close())
	if e != nil {
		return e
	}
	if len(data) > MaximumStateBytes*3 {
		return errors.New("fleet response too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var wire struct {
			Code    string    `json:"code"`
			Message string    `json:"message"`
			Routing *Envelope `json:"routing,omitempty"`
			Removal *Envelope `json:"removal,omitempty"`
		}
		if strictjson.Decode(data, &wire) != nil {
			return errors.New("invalid fleet error response")
		}
		return &Error{wire.Code, wire.Message, response.StatusCode, wire.Routing, wire.Removal}
	}
	return strictjson.Decode(data, result)
}

// RequestHub uses only a verified cached origin and the current device identity.
// It never follows an unsigned redirect or automatically repeats a mutation.
func (s *Service) RequestHub(ctx context.Context, method, path string, body any, result any) error {
	s.mu.Lock()
	if e := s.admissionError(); e != nil {
		s.mu.Unlock()
		return e
	}
	association := s.identity.ActiveFleet
	if association == nil {
		s.mu.Unlock()
		return conflict("local mode; join a fleet or recover")
	}
	hub := Machine{}
	for _, m := range s.public.Machines {
		if m.Machine == s.public.ActiveHub {
			hub = m
		}
	}
	bearer := association.Bearer
	s.mu.Unlock()
	if hub.Origin == "" {
		return &Error{Code: "HubUnavailable", Message: "hub offline; reconnect its tailscale network", Status: 503}
	}
	return s.request(ctx, hub.Origin, hub.Machine, bearer, method, path, body, result)
}
func (s *Service) Enroll(ctx context.Context, invitation Invitation, name string) error {
	s.mu.Lock()
	admissionErr := s.admissionError()
	s.mu.Unlock()
	if admissionErr != nil {
		return admissionErr
	}
	identity := s.Identity()
	if identity.ActiveFleet != nil && identity.ActiveFleet.FleetID == invitation.FleetID {
		if e := ValidateInvitation(invitation, time.Now()); e != nil {
			return e
		}
		if identity.ActiveFleet.PublicKey != invitation.PublicKey {
			return errors.New("same-fleet invitation key mismatch")
		}
		var state HostState
		if e := s.request(ctx, invitation.Hub.Origin, invitation.Hub.Machine, identity.ActiveFleet.Bearer, "GET", "/v1/fleet/host-state", nil, &state); e != nil {
			return e
		}
		return s.Receive(state)
	}
	request, e := s.StageJoin(invitation, name)
	if e != nil {
		return e
	}
	s.mu.Lock()
	pending := *s.identity.Pending
	s.mu.Unlock()
	// First recover a possibly committed response using only the staged credential.
	var state HostState
	e = s.request(ctx, invitation.Hub.Origin, invitation.Hub.Machine, pending.Association.Bearer, "GET", "/v1/fleet/host-state", nil, &state)
	if e == nil {
		return s.ConfirmJoin(state)
	}
	var typed *Error
	if !errors.As(e, &typed) || typed.Status != 401 {
		return e
	}
	e = s.request(ctx, invitation.Hub.Origin, invitation.Hub.Machine, "", "POST", "/v1/fleet/join", request, &state)
	if e != nil {
		return e
	}
	return s.ConfirmJoin(state)
}
