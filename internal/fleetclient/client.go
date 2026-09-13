// Package fleetclient routes the shared agent operations directly to configured peers.
package fleetclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const (
	MaximumInputBytes     = 64 * 1024
	MaximumControlBytes   = 64 * 1024
	MaximumInventoryBytes = 1024 * 1024
	Timeout               = 15 * time.Second
)

type Failure struct {
	Code     string `json:"code"`
	Dispatch string `json:"dispatch"`
}

type Result struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"result,omitempty"`
	Error *Failure        `json:"error,omitempty"`
}

func Failed(code, dispatch string) Result {
	return Result{Error: &Failure{Code: code, Dispatch: dispatch}}
}

type Client struct {
	peers []peer
	http  *http.Client
}

// Execute accepts the same structured inputs as the command-line interface.
// A failed write is never retried, including after a lost acknowledgement.
func (client *Client) Execute(ctx context.Context, operation string, encoded []byte) Result {
	if len(encoded) > MaximumInputBytes {
		return Failed("input_limit", "not_sent")
	}
	if operation == "list" {
		var input *struct {
			Machine string `json:"machine,omitempty"`
		}
		if strictjson.Decode(encoded, &input) != nil || input == nil {
			return Failed("invalid_input", "not_sent")
		}
		var members map[string]json.RawMessage
		if strictjson.Decode(encoded, &members) != nil || members["machine"] != nil && input.Machine == "" {
			return Failed("invalid_input", "not_sent")
		}
		peers := client.peers
		if input.Machine != "" {
			peers = nil
			for _, candidate := range client.peers {
				if candidate.Label == input.Machine {
					peers = []peer{candidate}
					break
				}
			}
			if len(peers) == 0 {
				return Failed("machine_unknown", "not_sent")
			}
		}
		type observation struct {
			Label   string          `json:"label"`
			Machine string          `json:"machine"`
			OK      bool            `json:"ok"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   *Failure        `json:"error,omitempty"`
		}
		rows := make([]observation, len(peers))
		var pending sync.WaitGroup
		for index, target := range peers {
			pending.Go(func() {
				result := client.call(ctx, target, "list", "/v1/sessions", nil)
				rows[index] = observation{target.Label, target.Machine, result.OK, result.Value, result.Error}
			})
		}
		pending.Wait()
		value, _ := json.Marshal(struct {
			Peers []observation `json:"peers"`
		}{rows}) // json.RawMessage values were decoded at the HTTP boundary.
		result := Result{OK: true, Value: value}
		if _, err := result.Encode(operation); err != nil {
			return Failed("output_limit", "not_sent")
		}
		return result
	}
	selected, path, body, failure := client.prepare(operation, encoded)
	if failure != nil {
		return Result{Error: failure}
	}
	return client.call(ctx, selected, operation, path, body)
}

func (client *Client) call(ctx context.Context, target peer, operation, path string, body []byte) Result {
	dispatch := "not_sent"
	writes := operation != "list" && operation != "read"
	if ctx.Err() != nil {
		return Failed("unavailable", dispatch)
	}
	method := http.MethodPost
	var reader io.Reader
	if operation == "list" {
		method = http.MethodGet
	} else {
		// No GetBody: net/http cannot replay a possibly delivered mutation.
		reader = io.NopCloser(bytes.NewReader(body))
	}
	request, err := http.NewRequestWithContext(ctx, method, target.Origin+path, reader)
	if err != nil {
		return Failed("invalid_input", dispatch)
	}
	request.Header.Set("Authorization", "Bearer "+target.Bearer)
	request.Header.Set("Skidbladnir-Machine", target.Machine)
	request.Header.Set("Accept", "application/json")
	if reader != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if writes {
		dispatch = "unknown"
	}
	response, err := client.http.Do(request)
	if err != nil {
		return Failed("unavailable", dispatch)
	}
	defer response.Body.Close()
	limit := MaximumControlBytes
	if operation == "list" {
		limit = MaximumInventoryBytes
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, int64(limit+1)))
	if err != nil {
		return Failed("unavailable", dispatch)
	}
	if len(encoded) > limit {
		return Failed("output_limit", dispatch)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return Failed("protocol_error", dispatch)
	}
	expectedStatus := http.StatusOK
	if operation == "start" {
		expectedStatus = http.StatusCreated
	}
	if response.StatusCode != expectedStatus {
		var failure *struct {
			Code     string `json:"code"`
			Message  string `json:"message"`
			Dispatch string `json:"dispatch,omitempty"`
		}
		if strictjson.Decode(encoded, &failure) != nil || failure == nil || failure.Code == "" {
			return Failed("protocol_error", dispatch)
		}
		if failure.Dispatch == "not_sent" || knownRejection(failure.Code) {
			dispatch = "not_sent"
		}
		return Failed(failure.Code, dispatch)
	}
	if !validResponse(operation, encoded, target.Machine) {
		return Failed("protocol_error", dispatch)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, encoded); err != nil {
		return Failed("protocol_error", dispatch)
	}
	return Result{OK: true, Value: compact.Bytes()}
}

func knownRejection(code string) bool {
	switch code {
	case "Unauthenticated", "MachineIdentityMismatch", "InvalidRequest", "RequestTooLarge",
		"WorkingDirectoryInvalid", "WorkingDirectoryUnavailable", "ProfileUnknown",
		"SessionNameInvalid", "ObjectiveInvalid", "SessionNameConflict", "SessionNotFound",
		"SessionIdentityMismatch", "SessionGroupedConflict":
		return true
	default:
		return false
	}
}

// Encode bounds the complete envelope, including the aggregate fleet result.
func (result Result) Encode(operation string) ([]byte, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	limit := MaximumControlBytes
	if operation == "list" {
		limit = MaximumInventoryBytes
	}
	if len(encoded)+1 > limit {
		return nil, errOutputLimit
	}
	return append(encoded, '\n'), nil
}
