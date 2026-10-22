package gateway

import (
	"context"
	"errors"
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type directorySearchRequest struct {
	Terms []string `json:"terms"`
}

type directorySearchResponse struct {
	Directories []string `json:"directories"`
	Omitted     bool     `json:"omitted"`
}

func (gateway *Gateway) searchDirectories(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[directorySearchRequest](writer, request)
	if !ok {
		return
	}
	result, err := gateway.workdir.Search(request.Context(), input.Terms)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		switch code, classified := workdir.ErrorCodeOf(err); {
		case !classified:
			writeError(writer, errorInternal)
		case code == workdir.Invalid:
			writeError(writer, errorInvalidRequest)
		case code == workdir.Unavailable:
			writeError(writer, errorDirectorySearchUnavailable)
		case code == workdir.TooLarge:
			writeError(writer, errorDirectorySearchTooLarge)
		default:
			writeError(writer, errorInternal)
		}
		return
	}
	writeJSON(writer, http.StatusOK, directorySearchResponse{Directories: result.Directories, Omitted: result.Omitted}, &errorDirectorySearchTooLarge)
}
