// Package fleetclient owns direct peer routing, name resolution, and exact references.
package fleetclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/space"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/coder/websocket"
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
	OK         bool            `json:"ok"`
	Value      json.RawMessage `json:"result,omitempty"`
	Error      *Failure        `json:"error,omitempty"`
	Candidates []string        `json:"-"`
}

func Failed(code, dispatch string) Result {
	return Result{Error: &Failure{Code: code, Dispatch: dispatch}}
}
func success(value any) Result {
	encoded, _ := json.Marshal(value)
	return Result{OK: true, Value: encoded}
}

type Client struct {
	peers []peer
	http  *http.Client
}

// Execute never retries a write, including after a lost acknowledgement.
func (client *Client) Execute(ctx context.Context, request Request) Result {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !request.Valid() {
		return Failed("invalid_input", "not_sent")
	}
	var result Result
	switch request.Operation {
	case "list":
		result = client.list(ctx, request.Machine)
		if result.OK && request.SpaceFilter.Kind() != space.FilterAll {
			var value Inventory
			if json.Unmarshal(result.Value, &value) != nil {
				return Failed("protocol_error", "not_sent")
			}
			for index := range value.Peers {
				peer := &value.Peers[index]
				if !peer.OK {
					continue
				}
				rows := make([]Session, 0, len(peer.Sessions))
				for _, row := range peer.Sessions {
					if request.SpaceFilter.Matches(row.Space) {
						rows = append(rows, row)
					}
				}
				peer.Sessions = rows
			}
			result = success(value)
		}
	case "start":
		selected, ok := client.peerByLabel(request.Machine)
		if !ok {
			return Failed("machine_unknown", "not_sent")
		}
		cwd := request.CWD
		if cwd == "" {
			cwd = "~"
		}
		body, _ := json.Marshal(struct {
			Kind    LaunchKind `json:"kind"`
			CWD     string     `json:"cwd"`
			Profile string     `json:"profile,omitempty"`
			Name    string     `json:"optionalTmuxName"`
			Space   string     `json:"space,omitempty"`
		}{request.Kind, cwd, request.Profile, request.Name, request.Space.String()})
		if len(body) > MaximumInputBytes {
			return Failed("input_limit", "not_sent")
		}
		result = client.call(ctx, selected, "start", "/v1/sessions", body)
		result = creationResult(selected, result)
	default:
		ref, observed, failure := client.resolve(ctx, request)
		if failure != nil {
			return *failure
		}
		selected, ok := client.peerByMachine(ref.Machine)
		if !ok {
			return Failed("machine_unknown", "not_sent")
		}
		if request.Operation == "info" {
			result = success(observed)
			break
		}
		if request.Operation == "enter" {
			return Failed("invalid_input", "not_sent")
		}
		body := map[string]any{"identityToken": ref.IdentityToken}
		path := "/v1/sessions/" + ref.TmuxID
		if request.Operation == "shell" {
			path += "/shell"
		} else if request.Operation == "space" {
			body["space"] = request.Space.String()
			path += "/space"
		} else if request.Operation == "kill" {
			body["tmuxName"] = observed.Session.Name
		} else {
			if ref.Agent == nil {
				return Failed("agent_unavailable", "not_sent")
			}
			body["paneId"] = ref.Agent.PaneID
			body["pid"] = ref.Agent.PID
			body["startIdentity"] = ref.Agent.StartIdentity
			path += "/agent/" + request.Operation
			switch request.Operation {
			case "read", "send":
				mode := request.Mode
				if mode == "" {
					mode = "auto"
				}
				body["mode"] = mode
				if request.Operation == "read" {
					maximum := request.MaxBytes
					if maximum == 0 {
						maximum = 16384
					}
					body["maxBytes"] = maximum
				} else {
					body["text"] = request.Text
				}
			case "keys":
				body["keys"] = request.Keys
			}
		}
		encoded, _ := json.Marshal(body)
		if len(encoded) > MaximumInputBytes {
			return Failed("input_limit", "not_sent")
		}
		result = client.call(ctx, selected, request.Operation, path, encoded)
		if request.Operation == "shell" {
			result = creationResult(selected, result)
		}
		if result.OK && request.Operation == "space" {
			result = success(struct {
				Space string `json:"space"`
			}{request.Space.String()})
		}
	}
	if _, err := result.Encode(request.Operation); err != nil {
		dispatch := "not_sent"
		if request.Operation == "start" || request.Operation == "shell" || request.Operation == "send" || request.Operation == "keys" || request.Operation == "interrupt" || request.Operation == "stop" || request.Operation == "kill" || request.Operation == "space" {
			dispatch = "unknown"
		}
		return Failed("output_limit", dispatch)
	}
	return result
}

func creationResult(selected peer, result Result) Result {
	if !result.OK {
		return result
	}
	var created hostObservedSession
	if json.Unmarshal(result.Value, &created) != nil {
		return Failed("protocol_error", "unknown")
	}
	return success(ObservedSession{Label: selected.Label, Machine: selected.Machine, ObservedAt: created.ObservedAt, Session: created.Session.project(selected.Machine)})
}

func (client *Client) list(ctx context.Context, label string) Result {
	peers := client.peers
	if label != "" {
		selected, ok := client.peerByLabel(label)
		if !ok {
			return Failed("machine_unknown", "not_sent")
		}
		peers = []peer{selected}
	}
	rows := make([]Peer, len(peers))
	var pending sync.WaitGroup
	for index, selected := range peers {
		pending.Go(func() {
			result := client.call(ctx, selected, "list", "/v1/sessions", nil)
			row := Peer{Label: selected.Label, Machine: selected.Machine, OK: result.OK, Error: result.Error}
			if result.OK {
				var observed hostInventory
				if json.Unmarshal(result.Value, &observed) != nil {
					row.OK = false
					row.Error = &Failure{Code: "protocol_error", Dispatch: "not_sent"}
				} else {
					row.ObservedAt = observed.ObservedAt
					row.Profiles = observed.Profiles
					row.Sessions = make([]Session, 0, len(observed.Sessions))
					for _, session := range observed.Sessions {
						row.Sessions = append(row.Sessions, session.project(selected.Machine))
					}
				}
			}
			rows[index] = row
		})
	}
	pending.Wait()
	result := Inventory{Peers: rows}
	for _, row := range rows {
		if !row.OK {
			result.Partial = true
		}
	}
	encoded := success(result)
	if _, err := encoded.Encode("list"); err != nil {
		return Failed("output_limit", "not_sent")
	}
	return encoded
}

func (client *Client) resolve(ctx context.Context, request Request) (Reference, ObservedSession, *Result) {
	fail := func(code string) (Reference, ObservedSession, *Result) {
		result := Failed(code, "not_sent")
		return Reference{}, ObservedSession{}, &result
	}
	var ref Reference
	label := request.Machine
	if request.Ref != "" {
		var err error
		ref, err = DecodeReference(request.Ref)
		if err != nil {
			return fail("invalid_input")
		}
		selected, ok := client.peerByMachine(ref.Machine)
		if !ok {
			return fail("machine_unknown")
		}
		if request.Operation != "info" && request.Operation != "kill" {
			return ref, ObservedSession{}, nil
		}
		label = selected.Label
	}
	result := client.list(ctx, label)
	if !result.OK {
		return Reference{}, ObservedSession{}, &result
	}
	var listed Inventory
	if json.Unmarshal(result.Value, &listed) != nil {
		return fail("protocol_error")
	}
	if listed.Partial {
		return fail("inventory_incomplete")
	}
	matches := make([]ObservedSession, 0)
	for _, peer := range listed.Peers {
		for _, row := range peer.Sessions {
			current, err := DecodeReference(row.Ref)
			if err != nil {
				return fail("protocol_error")
			}
			if request.Ref != "" {
				if !current.SessionEqual(ref) {
					continue
				}
			} else if row.Name != request.Name {
				continue
			}
			matches = append(matches, ObservedSession{Label: peer.Label, Machine: peer.Machine, ObservedAt: peer.ObservedAt, Session: row})
		}
	}
	if len(matches) == 0 {
		if request.Ref != "" {
			return fail("SessionIdentityMismatch")
		}
		return fail("name_not_found")
	}
	if len(matches) > 1 {
		result := Failed("name_ambiguous", "not_sent")
		for _, match := range matches {
			result.Candidates = append(result.Candidates, match.Label+" / "+match.Session.Name)
		}
		return Reference{}, ObservedSession{}, &result
	}
	observed := matches[0]
	if request.Ref == "" {
		ref, _ = DecodeReference(observed.Session.Ref)
	}
	return ref, observed, nil
}

// OpenTerminal shares the configured transport and identity binding with control calls.
func (client *Client) OpenTerminal(ctx context.Context, request Request) (*websocket.Conn, *Failure) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	request.Operation = "enter"
	if !request.Valid() {
		return nil, &Failure{Code: "invalid_input", Dispatch: "not_sent"}
	}
	ref, _, failure := client.resolve(ctx, request)
	if failure != nil {
		return nil, failure.Error
	}
	selected, ok := client.peerByMachine(ref.Machine)
	if !ok {
		return nil, &Failure{Code: "machine_unknown", Dispatch: "not_sent"}
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+selected.Bearer)
	headers.Set("Skidbladnir-Machine", selected.Machine)
	headers.Set("Skidbladnir-Session-Identity", ref.IdentityToken)
	connection, response, err := websocket.Dial(ctx, strings.Replace(selected.Origin, "https://", "wss://", 1)+"/v1/sessions/"+ref.TmuxID+"/terminal", &websocket.DialOptions{HTTPClient: client.http, HTTPHeader: headers, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		if response != nil && response.Body != nil {
			defer response.Body.Close()
			encoded, readErr := io.ReadAll(io.LimitReader(response.Body, MaximumControlBytes+1))
			mediaType, _, typeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
			if readErr == nil && len(encoded) <= MaximumControlBytes && typeErr == nil && mediaType == "application/json" {
				if refusal := decodeFailure(encoded, "not_sent"); refusal != nil {
					return nil, refusal
				}
			}
		}
		return nil, &Failure{Code: "terminal_unavailable", Dispatch: "not_sent"}
	}
	return connection, nil
}

func (client *Client) peerByLabel(label string) (peer, bool) {
	for _, p := range client.peers {
		if p.Label == label {
			return p, true
		}
	}
	return peer{}, false
}
func (client *Client) peerByMachine(machine string) (peer, bool) {
	for _, p := range client.peers {
		if p.Machine == machine {
			return p, true
		}
	}
	return peer{}, false
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
		if operation == "space" {
			method = http.MethodPut
		}
		if operation == "kill" {
			method = http.MethodDelete
		}
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
	if (operation == "kill" || operation == "space") && response.StatusCode == http.StatusNoContent {
		if len(encoded) != 0 {
			return Failed("protocol_error", dispatch)
		}
		if operation == "space" {
			return success(struct{}{})
		}
		return success(struct {
			Terminal string `json:"terminal"`
		}{"closed"})
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return Failed("protocol_error", dispatch)
	}
	expected := http.StatusOK
	if operation == "start" || operation == "shell" {
		expected = http.StatusCreated
	}
	if operation == "kill" || operation == "space" {
		expected = http.StatusNoContent
	}
	if response.StatusCode != expected {
		var failure *Failure
		if operation == "space" || operation == "start" || operation == "shell" {
			failure = decodeMutationFailure(operation, encoded, response.StatusCode)
		} else {
			failure = decodeFailure(encoded, dispatch)
		}
		if failure == nil {
			return Failed("protocol_error", dispatch)
		}
		return Result{Error: failure}
	}
	if !validResponse(operation, encoded, target.Machine) {
		return Failed("protocol_error", dispatch)
	}
	var compact bytes.Buffer
	if json.Compact(&compact, encoded) != nil {
		return Failed("protocol_error", dispatch)
	}
	return Result{OK: true, Value: compact.Bytes()}
}

func decodeFailure(encoded []byte, dispatch string) *Failure {
	var value *struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Dispatch string `json:"dispatch,omitempty"`
	}
	if strictjson.Decode(encoded, &value) != nil || value == nil || value.Code == "" {
		return nil
	}
	if value.Dispatch == "not_sent" || knownRejection(value.Code) {
		dispatch = "not_sent"
	}
	return &Failure{Code: value.Code, Dispatch: dispatch}
}

func knownRejection(code string) bool {
	switch code {
	case "Unauthenticated", "MachineIdentityMismatch", "InvalidRequest", "RequestTooLarge", "WorkingDirectoryInvalid", "WorkingDirectoryUnavailable", "ProfileUnknown", "SessionNameInvalid", "ObjectiveInvalid", "SpaceInvalid", "SessionNameConflict", "SessionNotFound", "SessionIdentityMismatch":
		return true
	default:
		return false
	}
}

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

func (result Result) ExitCode(operation string) int {
	if !result.OK {
		return 1
	}
	switch operation {
	case "list":
		var value Inventory
		if json.Unmarshal(result.Value, &value) != nil || value.Partial {
			return 1
		}
	case "send", "keys", "interrupt":
		var value WriteResult
		if json.Unmarshal(result.Value, &value) != nil || value.Outcome == "unknown" {
			return 1
		}
	case "stop":
		var value StopResult
		if json.Unmarshal(result.Value, &value) != nil || value.Agent == "unconfirmed" || value.Terminal != "closed" {
			return 1
		}
	case "kill":
		var value struct {
			Terminal string `json:"terminal"`
		}
		if json.Unmarshal(result.Value, &value) != nil || value.Terminal != "closed" {
			return 1
		}
	}
	return 0
}
