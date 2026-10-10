package notifier

import (
	"encoding/json"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
)

func (observer *Observer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawPath != "" || request.URL.RawQuery != "" || request.URL.ForceQuery {
		writeError(writer, "InvalidRequest")
		return
	}
	if _, code := auth.Authenticate(observer.bearer, request); code != "" {
		if code == auth.AdmissionUnauthenticated {
			writer.Header().Set("WWW-Authenticate", "Bearer")
		}
		writeError(writer, string(code))
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
		observer.events(writer, request)
	case request.Method == http.MethodPut && request.URL.Path == "/v1/notifications/receivers/android":
		observer.register(writer, request)
	default:
		writeError(writer, "InvalidRequest")
	}
}
func (observer *Observer) state(writer http.ResponseWriter) {
	observer.mutex.Lock()
	data, err := json.Marshal(observer.snapshot)
	unavailable := observer.unavailable
	observer.mutex.Unlock()
	if unavailable || err != nil || len(data) > attention.MaximumSnapshotBytes {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(data) // justify-ignore-error: disconnected read has no recoverable effect.
}
func (observer *Observer) events(writer http.ResponseWriter, request *http.Request) {
	if _, ok := writer.(http.Flusher); !ok {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	subscriber := make(chan struct{}, 1)
	observer.mutex.Lock()
	if observer.unavailable {
		observer.mutex.Unlock()
		writeError(writer, "NotificationsUnavailable")
		return
	}
	observer.subscribers[subscriber] = struct{}{}
	observer.mutex.Unlock()
	defer func() { observer.mutex.Lock(); delete(observer.subscribers, subscriber); observer.mutex.Unlock() }()
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(writer)
	for {
		observer.mutex.Lock()
		hint := attention.Hint{Schema: 1, Epoch: observer.file.Epoch, Revision: observer.file.Revision}
		unavailable := observer.unavailable
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
func (observer *Observer) register(writer http.ResponseWriter, request *http.Request) {
	preconditions := request.Header.Values("If-Match")
	if len(preconditions) == 0 {
		writeError(writer, "PreconditionRequired")
		return
	}
	if len(preconditions) != 1 || len(preconditions[0]) < 2 || preconditions[0][0] != '"' || preconditions[0][len(preconditions[0])-1] != '"' {
		writeError(writer, "InvalidRequest")
		return
	}
	tag := preconditions[0][1 : len(preconditions[0])-1]
	epoch, _, found := strings.Cut(tag, ":")
	if _, valid := attention.ReceiverRevision(epoch, tag); !found || !attention.EpochValid(epoch) || !valid {
		writeError(writer, "InvalidRequest")
		return
	}
	media, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(writer, "InvalidRequest")
		return
	}
	if request.ContentLength > attention.MaximumRegistrationBytes {
		writeError(writer, "RequestTooLarge")
		return
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, attention.MaximumRegistrationBytes+1))
	if err != nil || len(data) > attention.MaximumRegistrationBytes {
		writeError(writer, "RequestTooLarge")
		return
	}
	var subscription attention.Subscription
	if attention.Decode(data, &subscription) != nil || !subscription.Valid(observer.origin) {
		writeError(writer, "InvalidRequest")
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if observer.unavailable {
		writeError(writer, "NotificationsUnavailable")
		return
	}
	if installed := observer.file.AndroidSubscription; installed != nil && *installed == subscription {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if tag != observer.snapshot.ReceiverTag {
		writeError(writer, "PreconditionFailed")
		return
	}
	if observer.file.Revision >= math.MaxInt64-1 || observer.file.ReceiverRevision >= math.MaxInt64-1 {
		observer.unavailable = true
		writeError(writer, "NotificationsUnavailable")
		return
	}
	file := observer.file
	file.Revision++
	file.ReceiverRevision++
	file.AndroidSubscription = &subscription
	encoded, err := json.Marshal(file)
	if err != nil || len(encoded) > attention.MaximumSnapshotBytes || attention.WriteFile(observer.path, encoded) != nil {
		observer.unavailable = true
		writeError(writer, "NotificationsUnavailable")
		return
	}
	observer.file = file
	observer.snapshot.Revision = file.Revision
	observer.snapshot.ReceiverTag = file.Epoch + ":" + strconv.FormatInt(file.ReceiverRevision, 10)
	observer.signal()
	writer.WriteHeader(http.StatusNoContent)
}
func writeError(writer http.ResponseWriter, code string) {
	var status int
	var message string
	switch code {
	case string(auth.AdmissionInvalid), string(auth.AdmissionUnauthenticated), string(auth.AdmissionUnavailable), string(auth.AdmissionMachineMismatch), string(auth.AdmissionTooLarge):
		status, message = auth.AdmissionCode(code).Response()
	case "NotificationsUnavailable":
		status, message = http.StatusServiceUnavailable, "notifications unavailable"
	case "PreconditionRequired":
		status, message = http.StatusPreconditionRequired, "receiver precondition required"
	case "PreconditionFailed":
		status, message = http.StatusPreconditionFailed, "receiver precondition changed"
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
