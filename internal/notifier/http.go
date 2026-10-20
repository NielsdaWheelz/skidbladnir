package notifier

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
)

func (observer *Observer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawPath != "" || request.URL.RawQuery != "" || request.URL.ForceQuery {
		writeError(writer, "InvalidRequest")
		return
	}
	// Authentication precedes writer admission. Drain may run while the fleet
	// mutation lock is held; no admitted writer can wait for that lock.
	member, _, err := observer.fleet.Authenticate(request)
	if err != nil {
		writeFleetError(writer, err)
		return
	}
	if code := auth.BindMachine(request, observer.machine.String()); code != "" {
		writeError(writer, string(code))
		return
	}

	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v1/notifications/state":
		if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
			writeError(writer, "InvalidRequest")
			return
		}
		observer.state(writer)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/notifications/events":
		if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
			writeError(writer, "InvalidRequest")
			return
		}
		observer.events(writer, request, member.DeviceID)
	case (request.Method == http.MethodGet || request.Method == http.MethodPut) && request.URL.Path == "/v1/notifications/receivers/me":
		if member.Kind != "phone" {
			writeError(writer, "Forbidden")
			return
		}
		if request.Method == http.MethodGet {
			if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
				writeError(writer, "InvalidRequest")
				return
			}
			observer.receiverState(writer, member.DeviceID)
		} else {
			observer.register(writer, request, member.DeviceID)
		}
	case request.Method == http.MethodPost && request.URL.Path == "/v1/notifications/receivers/me/confirm":
		if member.Kind != "phone" {
			writeError(writer, "Forbidden")
			return
		}
		observer.confirm(writer, request, member.DeviceID)
	default:
		writeError(writer, "InvalidRequest")
	}
}
func (observer *Observer) state(writer http.ResponseWriter) {
	observer.mutex.Lock()
	data, err := json.Marshal(observer.snapshot)
	unavailable := observer.unavailable || observer.drained
	observer.mutex.Unlock()
	if unavailable || err != nil || len(data) > attention.MaximumSnapshotBytes {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(data) // justify-ignore-error: disconnected read has no recoverable effect.
}
func (observer *Observer) events(writer http.ResponseWriter, request *http.Request, device string) {
	if _, ok := writer.(http.Flusher); !ok {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	subscriber := make(chan struct{}, 1)
	observer.mutex.Lock()
	if observer.unavailable || observer.drained {
		observer.mutex.Unlock()
		writeError(writer, "NotificationsUnavailable")
		return
	}
	observer.streams.Add(1)
	observer.subscribers[subscriber] = struct{}{}
	observer.mutex.Unlock()
	defer func() {
		observer.mutex.Lock()
		delete(observer.subscribers, subscriber)
		observer.mutex.Unlock()
		observer.streams.Done()
	}()
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(writer)
	for {
		observer.mutex.Lock()
		hint := attention.Hint{Schema: 1, Epoch: observer.file.Epoch, Revision: observer.file.Revision}
		unavailable := observer.unavailable || observer.drained
		if index := observer.receiverIndex(device); index >= 0 && observer.file.Receivers[index].Removed {
			unavailable = true
		}
		observer.mutex.Unlock()
		if unavailable {
			return
		}
		data, err := json.Marshal(hint)
		if err != nil {
			panic("encode current revision hint")
		} // justify-defect: fixed fields always encode.
		if controller.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
			return
		}
		if _, err = io.WriteString(writer, "event: revision\ndata: "+string(data)+"\n\n"); err != nil {
			return
		}
		if controller.Flush() != nil {
			return
		}
		select {
		case <-subscriber:
		case <-request.Context().Done():
			return
		}
	}
}
func writeError(writer http.ResponseWriter, code string) {
	var status int
	var message string
	switch code {
	case string(auth.AdmissionInvalid), string(auth.AdmissionUnauthenticated), string(auth.AdmissionUnavailable), string(auth.AdmissionMachineMismatch), string(auth.AdmissionTooLarge):
		status, message = auth.AdmissionCode(code).Response()
	case "NotificationsUnavailable":
		status, message = http.StatusServiceUnavailable, "notifications unavailable"
	case "Forbidden":
		status, message = http.StatusForbidden, "phone membership required"
	case "ReceiverConflict":
		status, message = http.StatusConflict, "receiver generation changed"
	default:
		panic("unhandled notification error") // justify-defect: routes supply their closed error set.
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	data, err := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, message})
	if err != nil {
		panic("encode notification error")
	} // justify-defect: string fields always encode.
	_, _ = writer.Write(data) // justify-ignore-error: client disconnect after headers cannot be repaired.
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(value) // justify-ignore-error: a disconnected response cannot be repaired.
}
func writeFleetError(writer http.ResponseWriter, err error) {
	var value *fleet.Error
	if !errors.As(err, &value) {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	if value.Status == http.StatusUnauthorized {
		writer.Header().Set("WWW-Authenticate", "Bearer")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(value.Status)
	_ = json.NewEncoder(writer).Encode(struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Routing *fleet.Envelope `json:"routing,omitempty"`
		Removal *fleet.Envelope `json:"removal,omitempty"`
	}{value.Code, value.Message, value.Routing, value.Removal}) // justify-ignore-error: a disconnected response cannot be repaired.
}
func readRegistration(writer http.ResponseWriter, request *http.Request, value any) bool {
	media, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(writer, "InvalidRequest")
		return false
	}
	if request.ContentLength > attention.MaximumRegistrationBytes {
		writeError(writer, "RequestTooLarge")
		return false
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, attention.MaximumRegistrationBytes+1))
	if err != nil || len(data) > attention.MaximumRegistrationBytes {
		writeError(writer, "RequestTooLarge")
		return false
	}
	if attention.Decode(data, value) != nil {
		writeError(writer, "InvalidRequest")
		return false
	}
	return true
}
