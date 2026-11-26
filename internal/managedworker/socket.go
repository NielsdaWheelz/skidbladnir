// Package managedworker speaks the private, one-request worker socket protocol.
package managedworker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

type Request struct {
	Op    string `json:"op"`
	JobID string `json:"job_id,omitempty"`
	Brief string `json:"brief,omitempty"`
}

type Response struct {
	State       string `json:"state"`
	Reason      string `json:"reason"`
	Draft       bool   `json:"draft"`
	Interaction string `json:"interaction"`
}

// Call opens a fresh connection and bounds its response. Cancellation closes
// only this request, never the worker or its turn.
func Call(ctx context.Context, path string, request Request) (Response, error) {
	dialer := net.Dialer{Timeout: time.Second}
	connection, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return Response{}, err
	}
	defer connection.Close()
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	// Marshal once, checking the encoded bound before the worker receives any bytes.
	encoded, err := json.Marshal(request)
	if err != nil {
		return Response{}, err
	}
	if len(encoded)+1 > 6*api.JobPageSize+1024 {
		return Response{}, io.ErrShortBuffer
	}
	encoded = append(encoded, '\n')
	if _, err := connection.Write(encoded); err != nil {
		return Response{}, err
	}
	var result Response
	err = json.NewDecoder(io.LimitReader(connection, 1024)).Decode(&result)
	return result, err
}
