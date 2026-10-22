package notifier

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"slices"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// ServeHTTP serves /v1/notifications/* behind the gateway's admission (bearer,
// machine, path, query and body cap); it authenticates again only to learn the
// member's kind, which the gateway does not pass on.
func (observer *Observer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	member, err := observer.fleet.Authenticate(request)
	if err != nil {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeError(writer, http.StatusUnauthorized, "Unauthenticated", "authentication required")
		return
	}
	bodiless := request.ContentLength == 0 && len(request.TransferEncoding) == 0
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v1/notifications/state" && bodiless:
		observer.state(writer, member)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/notifications/events" && bodiless:
		observer.events(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v1/notifications/handled":
		observer.handled(writer, request)
	case request.Method == http.MethodPut && request.URL.Path == "/v1/notifications/receivers/me":
		if member.Kind != "phone" {
			writeError(writer, http.StatusForbidden, "Forbidden", "phone membership required")
			return
		}
		observer.register(writer, request, member.DeviceID)
	default:
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "the request is not valid")
	}
}

// state serves the projection; a phone also reads its own push delivery.
func (observer *Observer) state(writer http.ResponseWriter, member fleet.Member) {
	response := struct {
		attention.State
		Receiver *receiverStatus `json:"receiver,omitempty"`
	}{}
	observer.mutex.Lock()
	unavailable := observer.unavailable
	response.State = observer.served
	if member.Kind == "phone" {
		status := observer.status(member.DeviceID)
		response.Receiver = &status
	}
	observer.mutex.Unlock()
	if unavailable {
		WriteUnavailable(writer)
		return
	}
	data, _ := json.Marshal(response) // plain fields always encode
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(data) // a disconnected client cannot be answered
}

// events hints one client: "changed" at connect and on every projection
// change, and a keepalive comment every minute so a dead stream is noticed.
func (observer *Observer) events(writer http.ResponseWriter, request *http.Request) {
	stream := make(chan struct{}, 1)
	stream <- struct{}{}
	observer.mutex.Lock()
	unavailable := observer.unavailable
	if !unavailable {
		observer.streams[stream] = struct{}{}
	}
	observer.mutex.Unlock()
	if unavailable {
		WriteUnavailable(writer)
		return
	}
	defer func() {
		observer.mutex.Lock()
		delete(observer.streams, stream)
		observer.mutex.Unlock()
	}()
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(writer)
	keepalive := time.NewTicker(time.Minute)
	defer keepalive.Stop()
	for {
		line := ": keepalive\n\n"
		select {
		case <-stream:
			line = "data: changed\n\n"
		case <-keepalive.C:
		case <-request.Context().Done():
			return
		}
		if controller.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
			return
		}
		if _, err := io.WriteString(writer, line); err != nil || controller.Flush() != nil {
			return
		}
	}
}

// handled records the owner's acknowledgement. A field applies only while it
// names the session's current episode or ready generation; a stale, superseded
// or unknown one is a no-op, so the hub's equality is the identity fence.
func (observer *Observer) handled(writer http.ResponseWriter, request *http.Request) {
	var input attention.Handled
	if !readBody(writer, request, &input) {
		return
	}
	if input.Slot == "" || input.Episode < 0 || input.ReadyGeneration < 0 {
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "the request is not valid")
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		WriteUnavailable(writer)
		return
	}
	index := slices.IndexFunc(observer.file.Records, func(stored record) bool { return slotID(stored.Key) == input.Slot })
	if index >= 0 {
		next := observer.file.Records[index]
		if input.Episode != 0 && input.Episode == next.AttentionEpisode {
			next.HandledEpisode = input.Episode
		}
		if input.ReadyGeneration != 0 && input.ReadyGeneration == next.ReadyGeneration {
			next.AckedReadyGeneration = input.ReadyGeneration
		}
		if next != observer.file.Records[index] {
			file := observer.file
			file.Records = slices.Clone(file.Records)
			file.Records[index] = next
			if observer.save(file) != nil {
				WriteUnavailable(writer)
				return
			}
			observer.publish(false)
		}
	}
	writer.WriteHeader(http.StatusNoContent)
}

// readBody strictly decodes one bounded json request body into value.
func readBody(writer http.ResponseWriter, request *http.Request, value any) bool {
	media, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "the request is not valid")
		return false
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, 4097))
	switch {
	case err == nil && len(data) > 4096:
		writeError(writer, http.StatusRequestEntityTooLarge, "RequestTooLarge", "the request is too large")
		return false
	case err != nil || strictjson.Decode(data, value) != nil:
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "the request is not valid")
		return false
	}
	return true
}

// WriteUnavailable answers 503 for notification state that failed to load or
// save; the hub answers it until the process restarts.
func WriteUnavailable(writer http.ResponseWriter) {
	writeError(writer, http.StatusServiceUnavailable, "NotificationsUnavailable", "notification state unavailable; repair its state and restart the skid service")
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	data, _ := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, message}) // plain fields always encode
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(data) // a disconnected client cannot be answered
}
