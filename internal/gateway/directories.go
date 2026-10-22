package gateway

import (
	"context"
	"errors"
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type directoryListingRequest struct {
	Directory string `json:"directory"`
}

type directorySearchRequest struct {
	Terms []string `json:"terms"`
}

func (gateway *Gateway) listDirectory(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[directoryListingRequest](writer, request)
	if !ok {
		return
	}
	listing, err := gateway.workdir.List(request.Context(), input.Directory)
	if err != nil {
		writeWorkdirFailure(writer, err, errorDirectoryListingUnavailable, errorDirectoryListingTooLarge)
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		Machine api.Machine `json:"machine"`
		workdir.Listing
	}{gateway.apiMachine(), listing}, &errorDirectoryListingTooLarge)
}

func (gateway *Gateway) searchDirectories(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[directorySearchRequest](writer, request)
	if !ok {
		return
	}
	result, err := gateway.workdir.Search(request.Context(), input.Terms)
	if err != nil {
		writeWorkdirFailure(writer, err, errorDirectorySearchUnavailable, errorDirectorySearchTooLarge)
		return
	}
	writeJSON(writer, http.StatusOK, result, &errorDirectorySearchTooLarge)
}

// writeWorkdirFailure answers a List or Search failure. A canceled request answers
// nothing: the client is gone and no Request.Completed line is logged.
func writeWorkdirFailure(writer http.ResponseWriter, err error, unavailable, tooLarge apiError) {
	switch code, _ := workdir.ErrorCodeOf(err); {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
	case code == workdir.Invalid:
		writeError(writer, errorInvalidRequest)
	case code == workdir.Unavailable:
		writeError(writer, unavailable)
	case code == workdir.TooLarge:
		writeError(writer, tooLarge)
	default:
		writeError(writer, errorInternal) // a contract defect answers content-free
	}
}
