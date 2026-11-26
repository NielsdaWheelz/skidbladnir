package gateway

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/jobs"
)

var errorJobOutputTooLarge = apiError{api.Error{Code: "JobOutputTooLarge", Message: "The result page exceeds the response limit."}, http.StatusUnprocessableEntity}

func (gateway *Gateway) submitJob(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	spec, ok := decodeJSON[api.JobRequest](writer, request)
	if !ok {
		return
	}
	if err := jobs.Validate(id, spec); err != nil {
		writeJobError(writer, err)
		return
	}
	receipt, err := gateway.jobs.Submit(request.Context(), id, spec)
	if err != nil {
		// A failed durable write can follow reservation and possibly dispatch.
		// A conflicting reservation is proved before this client's dispatch.
		if !errors.Is(err, jobs.ErrConflict) {
			writer.(*trackedResponseWriter).dispatch = "unknown"
		}
		writeJobError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, receipt, &errorJobOutputTooLarge)
}

func (gateway *Gateway) resultJob(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if _, ok := api.JobIDBytes(id); !ok || request.ContentLength != 0 {
		writeJobError(writer, jobs.ErrInvalid)
		return
	}
	var offset int
	if query := request.URL.RawQuery; query != "" {
		text, found := strings.CutPrefix(query, "offset=")
		n, err := strconv.Atoi(text)
		if !found || err != nil || strconv.Itoa(n) != text || n < 0 || n > api.JobAnswerLimit || n%api.JobPageSize != 0 {
			writeJobError(writer, jobs.ErrInvalid)
			return
		}
		offset = n
	}
	ctx, cancel := context.WithTimeout(request.Context(), 25*time.Second)
	defer cancel()
	connection := gateway.register(request.Context().Value(deviceContextKey{}).(string), "", "", cancel)
	if connection == nil {
		writeError(writer, errorUnauthenticated)
		return
	}
	defer gateway.unregister(connection, nil)
	result, err := gateway.jobs.Result(ctx, id, offset)
	if err != nil {
		writeJobError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, result, &errorJobOutputTooLarge)
}

func (gateway *Gateway) ackJob(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if _, ok := api.JobIDBytes(id); !ok || request.ContentLength != 0 {
		writeJobError(writer, jobs.ErrInvalid)
		return
	}
	if err := gateway.jobs.Ack(id); err != nil {
		writeJobError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func writeJobError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, jobs.ErrInvalid):
		writeError(writer, errorInvalidRequest)
	case errors.Is(err, jobs.ErrConflict):
		writeError(writer, apiError{api.Error{Code: "JobConflict", Message: "That job id already belongs to another request."}, http.StatusConflict})
	case errors.Is(err, jobs.ErrNotFound):
		writeError(writer, apiError{api.Error{Code: "JobNotFound", Message: "This machine has no reservation for that job."}, http.StatusNotFound})
	case errors.Is(err, jobs.ErrPending):
		writeError(writer, apiError{api.Error{Code: "JobPending", Message: "That job has no saved outcome yet."}, http.StatusRequestTimeout})
	case errors.Is(err, jobs.ErrAcknowledged):
		writeError(writer, apiError{api.Error{Code: "JobAcknowledged", Message: "The requester has already acknowledged that result."}, http.StatusGone})
	default:
		writeError(writer, errorInternal)
	}
}
