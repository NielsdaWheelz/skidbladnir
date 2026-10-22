// Package fleetclient owns direct peer routing, handle resolution, and exact references.
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
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
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
	terminalInputMessage string
	Code                 string `json:"code"`
	Dispatch             string `json:"dispatch"`
	Target               string `json:"target,omitempty"`
}
type Result struct {
	OK bool `json:"ok"`
	// Values are decoded at the owning transport boundary.
	Value any      `json:"result,omitempty"`
	Error *Failure `json:"error,omitempty"`
}

func Failed(code, dispatch string) Result {
	return Result{Error: &Failure{Code: code, Dispatch: dispatch}}
}
func success(value any) Result {
	return Result{OK: true, Value: value}
}

type Client struct {
	peers          []peer
	defaultMachine Machine
	hub            string
	self           fleet.Target
	http           *http.Client
	mu             sync.RWMutex
	directory      string
}

// Execute never retries a write, including after a lost acknowledgement.
func (client *Client) Execute(ctx context.Context, request Request) Result {
	if request.Operation == "wait" {
		return client.wait(ctx, request)
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if !request.Valid() {
		return Failed("invalid_input", "not_sent")
	}
	var result Result
	switch request.Operation {
	case "list":
		result = client.list(ctx, request.Machine)
		if result.OK && request.GroupFilter.Kind() != group.FilterAll {
			value := result.Value.(Inventory)
			for index := range value.Peers {
				peer := &value.Peers[index]
				if !peer.OK {
					continue
				}
				rows := make([]Session, 0, len(peer.Sessions))
				for _, row := range peer.Sessions {
					if request.GroupFilter.Matches(row.Group) {
						rows = append(rows, row)
					}
				}
				peer.Sessions = rows
			}
			result = success(value)
		}
		if result.OK {
			value := result.Value.(Inventory)
			client.resolveContexts(ctx, value.Peers)
			result = success(value)
		}
		return result
	case "start":
		return client.start(ctx, request)
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
			client.resolveObserved(ctx, &observed)
			if request.Explain {
				// The explained status replaces the inventory's, so the printed
				// status and its evidence come from one sample. The inspect
				// response carries no clock, so observedAt becomes the client
				// time it arrived.
				encoded, _ := json.Marshal(map[string]any{"identityToken": ref.IdentityToken, "paneId": ref.PaneID, "explain": true})
				inspected := client.call(ctx, selected, "terminal_explain", "/v1/sessions/"+ref.TmuxID+"/terminal/inspect", encoded)
				if !inspected.OK {
					return inspected
				}
				value := inspected.Value.(TerminalInspectResult)
				observed.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
				observed.Session.TerminalStatus, observed.Diagnostics = value.TerminalStatus, value.Diagnostics
				observed.Session.TerminalUsage = nil
				observed.Session.TerminalFooter = nil
			}
			result = success(observed)
			break
		}
		body := map[string]any{"identityToken": ref.IdentityToken}
		path := "/v1/sessions/" + ref.TmuxID
		operation := request.Operation
		switch {
		case operation == "shell":
			path += "/shell"
		case operation == "group":
			body["group"] = request.Group.String()
			path += "/group"
		case operation == "rename":
			body["expectedNaming"], body["naming"] = request.ExpectedNaming, request.Naming
		case operation == "close" && request.TerminalOnly:
			operation = "terminal_close"
		default:
			body["paneId"] = ref.PaneID
			path += "/terminal/" + operation
			if operation != "close" {
				operation = "terminal_" + operation
			}
			switch request.Operation {
			case "read":
				if request.MaxBytes != 0 {
					body["maxBytes"] = request.MaxBytes
				}
			case "send", "text":
				body["text"] = request.Text
			case "keys":
				body["keys"] = request.Keys
			}
		}
		encoded, _ := json.Marshal(body)
		if len(encoded) > MaximumInputBytes {
			return Failed("input_limit", "not_sent")
		}
		result = client.call(ctx, selected, operation, path, encoded)
		if result.OK && request.Operation == "group" {
			result = success(GroupResult{Group: request.Group.String()})
		}
	}
	if _, err := result.Encode(request.Operation); err != nil {
		dispatch := "not_sent"
		if request.Operation == "shell" || request.Operation == "send" || request.Operation == "keys" || request.Operation == "text" || request.Operation == "stop" || request.Operation == "close" || request.Operation == "group" || request.Operation == "rename" {
			dispatch = "unknown"
		}
		return Failed("output_limit", dispatch)
	}
	return result
}

func (client *Client) SearchDirectories(ctx context.Context, label string, terms []string) Result {
	target, ok := client.peerByLabel(label)
	if !ok {
		return Failed("machine_unknown", "not_sent")
	}
	if len(terms) == 0 || len(terms) > 8 {
		return Failed("invalid_input", "not_sent")
	}
	bytes := 0
	for _, term := range terms {
		if term == "" || !utf8.ValidString(term) || strings.IndexFunc(term, func(character rune) bool { return unicode.IsControl(character) || unicode.IsSpace(character) }) >= 0 {
			return Failed("invalid_input", "not_sent")
		}
		bytes += len(term)
	}
	if bytes > 256 {
		return Failed("invalid_input", "not_sent")
	}
	body, err := json.Marshal(struct {
		Terms []string `json:"terms"`
	}{terms})
	if err != nil {
		return Failed("invalid_input", "not_sent")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.call(ctx, target, "directory_search", "/v1/directory-searches", body)
}

// InventoryMachine reads one gateway directly, without fleet context enrichment.
func (client *Client) InventoryMachine(ctx context.Context, handle string) Result {
	target, found := client.peerByMachine(handle)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	return client.call(ctx, target, "list", "/v1/sessions", nil)
}

func (client *Client) list(ctx context.Context, label string) Result {
	peers := client.peerSnapshot()
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
			result := client.InventoryMachine(ctx, selected.Machine)
			row := Peer{Label: selected.Label, Machine: selected.Machine, OK: result.OK, Error: result.Error}
			if result.OK {
				row = result.Value.(Peer)
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
	reply := success(result)
	if _, err := reply.Encode("list"); err != nil {
		return Failed("output_limit", "not_sent")
	}
	return reply
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
		if request.Operation != "info" {
			return ref, ObservedSession{}, nil
		}
		label = selected.Label
	}
	result := client.list(ctx, label)
	if !result.OK {
		return Reference{}, ObservedSession{}, &result
	}
	listed := result.Value.(Inventory)
	if listed.Partial {
		return fail("inventory_incomplete")
	}
	matches := make([]ObservedSession, 0)
	seen := map[string]bool{}
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
			} else {
				key := Reference{Machine: current.Machine, TmuxID: current.TmuxID, IdentityToken: current.IdentityToken}.Encode()
				if seen[key] {
					continue
				}
				seen[key] = true
				if terminalHandle(current) != request.Handle {
					continue
				}
			}
			if request.Operation == "info" && row.Connection == nil {
				current := row.Current(peer)
				row.Execution = &current
			}
			matches = append(matches, ObservedSession{Label: peer.Label, Machine: peer.Machine, ObservedAt: peer.ObservedAt, Session: row})
		}
	}
	if len(matches) == 0 {
		if request.Ref != "" {
			return fail("SessionIdentityMismatch")
		}
		return fail("handle_not_found")
	}
	if len(matches) > 1 {
		return fail("handle_ambiguous")
	}
	observed := matches[0]
	if request.Ref == "" {
		ref, _ = DecodeReference(observed.Session.Ref)
	}
	return ref, observed, nil
}

// ResolveTerminal resolves a handle once; an exact captured reference needs no inventory.
func (client *Client) ResolveTerminal(ctx context.Context, request Request) (Reference, *Failure) {
	request.Operation = "enter"
	if !request.Valid() {
		return Reference{}, &Failure{Code: "invalid_input", Dispatch: "not_sent"}
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	ref, _, failure := client.resolve(ctx, request)
	if failure != nil {
		return Reference{}, failure.Error
	}
	return ref, nil
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
	for _, p := range client.peerSnapshot() {
		if p.Label == label {
			return p, true
		}
	}
	return peer{}, false
}
func (client *Client) peerByMachine(machine string) (peer, bool) {
	for _, p := range client.peerSnapshot() {
		if p.Machine == machine {
			return p, true
		}
	}
	return peer{}, false
}

func (client *Client) call(ctx context.Context, target peer, operation, path string, body []byte) Result {
	dispatch := "not_sent"
	if target.Origin == "" {
		return Failed("unavailable", dispatch)
	}
	writes := operation != "list" && operation != "profile_usage" && operation != "terminal_read" && operation != "terminal_inspect" && operation != "terminal_explain" && operation != "directory_search" && operation != "terminal_context"
	if ctx.Err() != nil {
		return Failed("unavailable", dispatch)
	}
	method := http.MethodPost
	var reader io.Reader
	if operation == "list" || operation == "profile_usage" || operation == "terminal_context" {
		method = http.MethodGet
	} else {
		if operation == "group" {
			method = http.MethodPut
		}
		if operation == "rename" {
			method = http.MethodPatch
		}
		if operation == "terminal_close" {
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
	receivedAt := time.Now()
	if err != nil {
		return Failed("unavailable", dispatch)
	}
	if len(encoded) > limit {
		return Failed("output_limit", dispatch)
	}
	if (operation == "terminal_close" || operation == "group" || operation == "rename") && response.StatusCode == http.StatusNoContent {
		if len(encoded) != 0 {
			return Failed("protocol_error", dispatch)
		}
		if operation == "group" {
			return success(GroupResult{})
		}
		if operation == "rename" {
			return success(nil)
		}
		return success(TerminalCloseResult{Terminal: "closed"})
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return Failed("protocol_error", dispatch)
	}
	expected := http.StatusOK
	if operation == "start" || operation == "shell" {
		expected = http.StatusCreated
	}
	if operation == "terminal_close" || operation == "group" || operation == "rename" {
		expected = http.StatusNoContent
	}
	if response.StatusCode != expected {
		var failure *Failure
		if operation == "group" || operation == "start" || operation == "shell" {
			failure = decodeMutationFailure(operation, encoded, response.StatusCode, target.Machine)
		} else {
			failure = decodeFailure(encoded, dispatch)
		}
		if failure == nil {
			return Failed("protocol_error", dispatch)
		}
		return Result{Error: failure}
	}
	value, ok := decodeResponse(operation, encoded, target)
	if !ok {
		return Failed("protocol_error", dispatch)
	}
	if operation == "list" {
		observed := value.(Peer)
		observed.ReceivedAt = receivedAt
		value = observed
	} else if operation == "profile_usage" {
		observed := value.(ProfileUsagePeer)
		observed.ReceivedAt = receivedAt
		value = observed
	}
	return success(value)
}

func decodeFailure(encoded []byte, dispatch string) *Failure {
	var value *struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Dispatch string `json:"dispatch,omitempty"`
	}
	if !nonNullJSON(encoded) || strictjson.Decode(encoded, &value) != nil || value == nil || value.Code == "" {
		return nil
	}
	if value.Dispatch == "not_sent" || value.Dispatch == "unknown" {
		dispatch = value.Dispatch
	} else if value.Dispatch != "" {
		return nil
	} else if knownRejection(value.Code) {
		dispatch = "not_sent"
	}
	failure := &Failure{Code: value.Code, Dispatch: dispatch}
	if value.Code == "TerminalInputBlocked" {
		switch value.Message {
		case "send unavailable for this screen. open the terminal or use text/keys.",
			"respond to the dialog in the terminal.",
			"terminal contains a draft. open it before sending.",
			"the requested agent is not ready for its initial prompt. inspect the terminal before sending.":
			failure.terminalInputMessage = value.Message
		default:
			return nil
		}
	}
	return failure
}

func knownRejection(code string) bool {
	switch code {
	case "Unauthenticated", "MachineIdentityMismatch", "InvalidRequest", "RequestTooLarge", "WorkingDirectoryInvalid", "WorkingDirectoryUnavailable", "ProfileUnknown", "SessionNameInvalid", "ObjectiveInvalid", "GroupInvalid", "SessionNameConflict", "SessionNameChanged", "SessionNotFound", "SessionIdentityMismatch", "TerminalTargetChanged", "TerminalUnavailable", "TerminalInputBlocked":
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
	case "start":
		value := result.Value.(StartResult)
		if value.Creation != "created" || value.Prompt != "not_requested" && value.Prompt != "written" || value.Failure != nil {
			return 1
		}
	case "list":
		if result.Value.(Inventory).Partial {
			return 1
		}
	case "send", "text", "keys", "stop":
		if result.Value.(WriteResult).Outcome == "unknown" {
			return 1
		}
	case "close":
		if value, ok := result.Value.(TerminalCloseResult); ok {
			if value.Terminal != "closed" {
				return 1
			}
			break
		}
		value := result.Value.(CloseResult)
		if value.Interrupt != "written" || value.Terminal != "closed" {
			return 1
		}
	case "wait":
		if result.Value.(WaitResult).Outcome != "matched" {
			return 1
		}
	}
	return 0
}
