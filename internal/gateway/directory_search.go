package gateway

import (
	"context"
	"encoding/json"
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
	input, failure := decodeJSON[directorySearchRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
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
	encoded, err := json.Marshal(directorySearchResponse{Directories: result.Directories, Omitted: result.Omitted})
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	if len(encoded)+1 > int(MaximumBodyBytes) {
		writeError(writer, errorDirectorySearchTooLarge)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(append(encoded, '\n')) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}
