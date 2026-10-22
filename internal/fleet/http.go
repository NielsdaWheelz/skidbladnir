package fleet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const maximumRequestBytes = 64 << 10

func (s *Service) Handler() http.Handler { return http.HandlerFunc(s.serveHTTP) }
func (s *Service) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawPath != "" || r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeError(w, invalidRequest())
		return
	}
	if r.Method == "GET" {
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
	}
	if code := auth.BindMachine(r, s.machine.String()); code != "" {
		status, message := code.Response()
		writeError(w, &Error{Code: string(code), Message: message, Status: status})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/local/") {
		s.serveLocal(w, r)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/v1/fleet/join" {
		var request joinRequest
		if err := readBody(r, &request); err != nil {
			writeError(w, err)
			return
		}
		reply, err := s.join(request, time.Now())
		if err != nil {
			writeError(w, err)
			return
		}
		if request.Kind == "phone" {
			reply = reply.public()
		}
		writeJSON(w, 201, reply)
		return
	}
	member, err := s.Authenticate(r)
	if err != nil {
		writeError(w, err)
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/fleet":
		s.mu.Lock()
		current := s.doc.Fleet.State
		s.mu.Unlock()
		if member.Kind != "computer" {
			current = current.public()
		}
		writeJSON(w, 200, current)
	case r.Method == "GET" && r.URL.Path == "/v1/fleet/events":
		s.events(w, r)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/apply":
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.apply()
		respond(w, status, err)
	case r.Method == "POST" && r.URL.Path == "/v1/fleet/invites":
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
		invitation, err := s.invite(time.Now())
		respond(w, invitation, err)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/fleet/members/"):
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
		device := strings.TrimPrefix(r.URL.Path, "/v1/fleet/members/")
		if !ValidDeviceID(device) {
			writeError(w, invalidRequest())
			return
		}
		status, err := s.remove(device)
		respond(w, status, err)
	case r.Method == "PUT" && r.URL.Path == "/v1/fleet/self":
		var report SelfReport
		if err := readBody(r, &report); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.report(member, report)
		respond(w, status, err)
	default:
		http.NotFound(w, r)
	}
}

// serveLocal admits only the installed local credential: the cli on this host.
func (s *Service) serveLocal(w http.ResponseWriter, r *http.Request) {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 || !s.local.Verify(headers[0]) {
		writeError(w, &Error{Code: "Unauthenticated", Message: "installed local credential required", Status: 401})
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/local/status":
		s.mu.Lock()
		status := s.status()
		s.mu.Unlock()
		writeJSON(w, 200, status)
	case r.Method == "PUT" && r.URL.Path == "/v1/local/network":
		var input struct {
			Origin string `json:"origin"`
		}
		if err := readBody(r, &input); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.setOrigin(r.Context(), input.Origin)
		respond(w, status, err)
	case r.Method == "POST" && r.URL.Path == "/v1/local/join":
		var input struct {
			Invitation Invitation `json:"invitation"`
			Name       string     `json:"name"`
		}
		if err := readBody(r, &input); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.enroll(r.Context(), input.Invitation, input.Name)
		respond(w, status, err)
	case r.Method == "POST" && r.URL.Path == "/v1/local/apply":
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.localApply(r.Context())
		respond(w, status, err)
	case r.Method == "POST" && r.URL.Path == "/v1/local/recover":
		if err := emptyBody(r); err != nil {
			writeError(w, err)
			return
		}
		status, err := s.recover()
		respond(w, status, err)
	default:
		http.NotFound(w, r)
	}
}

// events streams revision hints, one per state revision. It authenticates again
// on every wake, so a removed caller's stream ends at the next commit.
func (s *Service) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, errors.New("streaming unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	var sent uint64 // revisions start at 1
	for {
		s.mu.Lock()
		_, err := s.authenticate(r)
		revision, changed := s.doc.Fleet.State.StateRevision, s.changed
		s.mu.Unlock()
		if err != nil {
			return
		}
		if revision != sent {
			if _, err := fmt.Fprintf(w, "event: revision\ndata: {\"stateRevision\":%d}\n\n", revision); err != nil {
				return
			}
			flusher.Flush()
			sent = revision
		}
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

// readBody decodes one gateway request body strictly: it is network ingress.
func readBody(r *http.Request, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.Header.Get("Content-Encoding") != "" {
		return invalidRequest()
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maximumRequestBytes+1))
	if err != nil {
		return invalidRequest()
	}
	if len(data) > maximumRequestBytes {
		return &Error{Code: "RequestTooLarge", Message: "the request is too large", Status: 413}
	}
	if strictjson.Decode(data, target) != nil {
		return invalidRequest()
	}
	return nil
}
func emptyBody(r *http.Request) error {
	if r.Header.Get("Content-Type") != "" || r.Header.Get("Content-Encoding") != "" || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		return invalidRequest()
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(data) != 0 {
		return invalidRequest()
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value) // a disconnected client cannot be answered
}
func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, value)
}

// writeError answers {code,message}; an untyped error is an internal failure.
func writeError(w http.ResponseWriter, err error) {
	var typed *Error
	if !errors.As(err, &typed) {
		typed = &Error{Code: "InternalError", Message: "skid could not complete the request", Status: 500}
	}
	writeJSON(w, typed.Status, typed)
}
