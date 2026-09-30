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

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
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
	defaultMachine string
	http           *http.Client
}

// InspectReference retains the locally admitted target even when native inspection
// fails. Execute's internal inspect result remains the provider runtime envelope.
func (client *Client) InspectReference(ctx context.Context, encoded string) Result {
	ref, err := DecodeReference(encoded)
	if err != nil || ref.Conversation == nil {
		return Failed("invalid_input", "not_sent")
	}
	selected, found := client.peerByMachine(ref.Machine)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	value := InspectedReference{Label: selected.Label, Machine: selected.Machine}
	value.Target.Ref = encoded
	value.Target.Conversation = ref.Conversation.Binding.Conversation
	value.Target.Turn = ref.Conversation.Turn
	value.Inspection = client.Execute(ctx, Request{Operation: "inspect", Ref: encoded})
	if value.Inspection.OK {
		runtime := value.Inspection.Value.(agentruntime.ConversationRuntime)
		value.ObservedRef = (Reference{Machine: ref.Machine, Conversation: &runtime}).Encode()
	}
	return success(value)
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
			Name    string     `json:"optionalTmuxName,omitempty"`
			Group   string     `json:"group,omitempty"`
		}{request.Kind, cwd, request.Profile, request.Name, request.Group.String()})
		if len(body) > MaximumInputBytes {
			return Failed("input_limit", "not_sent")
		}
		result = client.call(ctx, selected, "start", "/v1/sessions", body)
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
			result = success(observed)
			break
		}
		if request.Operation == "enter" {
			return Failed("invalid_input", "not_sent")
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
		case operation == "close" && request.TerminalOnly:
			operation = "terminal_close"
		case ref.Conversation != nil:
			runtime := ref.Conversation
			body = map[string]any{"conversation": runtime.Binding.Conversation}
			path = "/v1/conversations/" + operation
			switch operation {
			case "read":
				scope := request.Scope
				if scope == "" {
					scope = "latest"
				}
				body["scope"] = scope
				if request.MaxBytes != 0 {
					body["maxBytes"] = request.MaxBytes
				}
			case "send":
				input, delivery := request.Input, request.Delivery
				if input == "" {
					input = "peer"
				}
				if delivery == "" {
					delivery = "direct"
				}
				if delivery == "queue" {
					return Failed("AgentUnavailable", "not_sent")
				}
				body["input"], body["delivery"], body["text"] = input, delivery, request.Text
			case "stop":
				if runtime.Turn != nil && runtime.Turn.State == "inProgress" {
					body["turn"] = runtime.Turn
				}
			}
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
		if result.OK && ref.Conversation != nil {
			switch request.Operation {
			case "read":
				value := result.Value.(ReadResult)
				scope := request.Scope
				if scope == "" {
					scope = "latest"
				}
				if value.Observation.Binding.Conversation != ref.Conversation.Binding.Conversation || value.Scope != scope {
					return Failed("protocol_error", "not_sent")
				}
			case "inspect":
				if result.Value.(agentruntime.ConversationRuntime).Binding.Conversation != ref.Conversation.Binding.Conversation {
					return Failed("protocol_error", "not_sent")
				}
			}
		}
		if result.OK && request.Operation == "group" {
			result = success(GroupResult{Group: request.Group.String()})
		}
	}
	if _, err := result.Encode(request.Operation); err != nil {
		dispatch := "not_sent"
		if request.Operation == "start" || request.Operation == "shell" || request.Operation == "send" || request.Operation == "keys" || request.Operation == "text" || request.Operation == "stop" || request.Operation == "close" || request.Operation == "group" {
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
	if request.ConversationID != "" {
		selected, found := client.peerByLabel(request.Machine)
		if !found {
			return fail("machine_unknown")
		}
		conversation, failure := client.conversationByProfile(ctx, selected, request.Profile, request.ConversationID)
		if failure != nil {
			return Reference{}, ObservedSession{}, failure
		}
		ref, failure := client.inspectConversation(ctx, selected, conversation)
		return ref, ObservedSession{}, failure
	}
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
			} else if strings.HasPrefix(request.Handle, "c-") {
				if row.Conversation == nil {
					continue
				}
				conversation := *row.Conversation
				tuple, _ := json.Marshal([]string{peer.Machine, string(conversation.Provider), string(conversation.ProfileKey), conversation.HistoryScope, conversation.ConversationID})
				if seen[string(tuple)] {
					continue
				}
				seen[string(tuple)] = true
				if conversationHandle(peer.Machine, conversation) != request.Handle {
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
	if strings.HasPrefix(request.Handle, "c-") {
		selected, _ := client.peerByMachine(observed.Machine)
		ref, failure := client.inspectConversation(ctx, selected, *observed.Session.Conversation)
		return ref, ObservedSession{}, failure
	}
	if request.Ref == "" {
		ref, _ = DecodeReference(observed.Session.Ref)
	}
	return ref, observed, nil
}

func (client *Client) inspectConversation(ctx context.Context, selected peer, conversation agentruntime.Conversation) (Reference, *Result) {
	encoded, _ := json.Marshal(map[string]any{"conversation": conversation})
	inspected := client.call(ctx, selected, "inspect", "/v1/conversations/inspect", encoded)
	if !inspected.OK {
		return Reference{}, &inspected
	}
	runtime := inspected.Value.(agentruntime.ConversationRuntime)
	if runtime.Binding.Conversation != conversation {
		failure := Failed("protocol_error", "not_sent")
		return Reference{}, &failure
	}
	return Reference{Machine: selected.Machine, Conversation: &runtime}, nil
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
	writes := operation != "list" && operation != "read" && operation != "terminal_read" && operation != "terminal_inspect" && operation != "directory_search" && operation != "terminal_context" && operation != "results" && operation != "inspect"
	if ctx.Err() != nil {
		return Failed("unavailable", dispatch)
	}
	method := http.MethodPost
	var reader io.Reader
	if operation == "list" || operation == "terminal_context" {
		method = http.MethodGet
	} else {
		if operation == "group" {
			method = http.MethodPut
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
	if err != nil {
		return Failed("unavailable", dispatch)
	}
	if len(encoded) > limit {
		return Failed("output_limit", dispatch)
	}
	if (operation == "terminal_close" || operation == "group") && response.StatusCode == http.StatusNoContent {
		if len(encoded) != 0 {
			return Failed("protocol_error", dispatch)
		}
		if operation == "group" {
			return success(GroupResult{})
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
	if operation == "terminal_close" || operation == "group" {
		expected = http.StatusNoContent
	}
	if response.StatusCode != expected {
		var failure *Failure
		if operation == "group" || operation == "start" || operation == "shell" {
			failure = decodeMutationFailure(operation, encoded, response.StatusCode)
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
		case "respond to the dialog in the terminal.", "terminal contains a draft. open it before sending.":
			failure.terminalInputMessage = value.Message
		}
	}
	return failure
}

func knownRejection(code string) bool {
	switch code {
	case "Unauthenticated", "MachineIdentityMismatch", "InvalidRequest", "RequestTooLarge", "WorkingDirectoryInvalid", "WorkingDirectoryUnavailable", "ProfileUnknown", "SessionNameInvalid", "ObjectiveInvalid", "GroupInvalid", "SessionNameConflict", "SessionNotFound", "SessionIdentityMismatch", "TerminalTargetChanged", "TerminalUnavailable", "TerminalInputBlocked":
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
	if value, ok := result.Value.(InspectedReference); ok && !value.Inspection.OK {
		return 1
	}
	switch operation {
	case "list":
		if result.Value.(Inventory).Partial {
			return 1
		}
	case "send", "text", "keys", "stop":
		value, write := result.Value.(WriteResult)
		if !write {
			break
		}
		if value.Outcome == "unknown" {
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

func (client *Client) conversationByProfile(ctx context.Context, target peer, key, id string) (agentruntime.Conversation, *Result) {
	listed := client.call(ctx, target, "list", "/v1/sessions", nil)
	if !listed.OK {
		return agentruntime.Conversation{}, &listed
	}
	for _, profile := range listed.Value.(Peer).Profiles {
		if profile.Key == key {
			conversation := agentruntime.Conversation{Provider: agentruntime.Provider(profile.Provider), ProfileKey: agentruntime.ProfileKey(key), HistoryScope: profile.HistoryScope, ConversationID: id}
			if conversation.Valid() {
				return conversation, nil
			}
			break
		}
	}
	failed := Failed("AgentUnavailable", "not_sent")
	return agentruntime.Conversation{}, &failed
}
