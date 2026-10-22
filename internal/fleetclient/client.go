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

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
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
	Target               Ref    `json:"target,omitzero"`
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

// Execute runs one operation once. It never retries a write, including after a
// lost acknowledgement.
func (client *Client) Execute(ctx context.Context, request Request) Result {
	if request.Operation == "wait" {
		return client.wait(ctx, request)
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	switch request.Operation {
	case "list":
		return client.list(ctx, request.Machine, request.GroupFilter)
	case "start":
		return client.start(ctx, request)
	}
	resolved, failure := client.resolve(ctx, request)
	if failure != nil {
		return Result{Error: failure}
	}
	if request.Operation == "info" {
		return client.info(ctx, resolved, request.Explain)
	}
	ref := resolved.ref
	body := map[string]any{"identityToken": ref.IdentityToken}
	operation := request.Operation
	switch {
	case operation == "group":
		body["group"] = request.Group.String()
	case operation == "rename":
		body["expectedNaming"], body["naming"] = request.ExpectedNaming, request.Naming
	case operation == "shell":
	case operation == "close" && request.TerminalOnly:
		operation = "terminal_close"
	default: // read, send, text, keys, stop, close: the selected pane's terminal
		body["paneId"] = ref.PaneID
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
	result := client.call(ctx, resolved.peer, operation, ref.TmuxID, encoded)
	switch {
	case !result.OK:
	case operation == "group":
		result = success(GroupResult{Group: request.Group.String()})
	case operation == "terminal_close":
		result = success(TerminalCloseResult{Terminal: "closed"})
	}
	return result
}

func (client *Client) SearchDirectories(ctx context.Context, label string, terms []string) Result {
	target, ok := client.peerByLabel(label)
	if !ok {
		return Failed("machine_unknown", "not_sent")
	}
	body, _ := json.Marshal(struct {
		Terms []string `json:"terms"`
	}{terms})
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.call(ctx, target, "directory_search", "", body)
}

// InventoryMachine reads one gateway directly, without fleet context enrichment.
func (client *Client) InventoryMachine(ctx context.Context, handle string) Result {
	target, found := client.peerByMachine(handle)
	if !found {
		return Failed("machine_unknown", "not_sent")
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	return client.call(ctx, target, "list", "", nil)
}

// inventory reads every peer, or only the labelled one, concurrently. Its
// envelope, before any filter or enrichment, is bounded like a list's.
func (client *Client) inventory(ctx context.Context, label string) Result {
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

// list is the inventory filtered by group, with each remote row's display
// context: a complete listing waits for every source's enrichment.
func (client *Client) list(ctx context.Context, label string, filter group.Filter) Result {
	result := client.inventory(ctx, label)
	if !result.OK {
		return result
	}
	value := result.Value.(Inventory)
	for index := range value.Peers {
		peer := &value.Peers[index]
		if !peer.OK {
			continue
		}
		rows := make([]Session, 0, len(peer.Sessions))
		for _, row := range peer.Sessions {
			if filter.Matches(row.Group) {
				rows = append(rows, row)
			}
		}
		peer.Sessions = rows
	}
	// value.Peers stays read-only while the passes run; each writes only its own slot.
	enriched := make([]Peer, len(value.Peers))
	var pending sync.WaitGroup
	for index, peer := range value.Peers {
		pending.Go(func() { enriched[index] = client.EnrichContexts(ctx, peer, value.Peers) })
	}
	pending.Wait()
	value.Peers = enriched
	return success(value)
}

// resolved is one target: its route, the exact reference, and for a handle or
// info its inventory row.
type resolved struct {
	peer     peer
	ref      Reference
	observed ObservedSession
}

// resolve finds a request's target in one inventory of its scope. An exact
// reference needs no inventory except for info, which reports the row.
func (client *Client) resolve(ctx context.Context, request Request) (resolved, *Failure) {
	ref, label := request.Ref, request.Machine
	if !ref.IsZero() {
		selected, ok := client.peerByMachine(ref.Machine)
		if !ok {
			return resolved{}, Failed("machine_unknown", "not_sent").Error
		}
		if request.Operation != "info" {
			return resolved{peer: selected, ref: ref}, nil
		}
		label = selected.Label
	}
	result := client.inventory(ctx, label)
	if !result.OK {
		return resolved{}, result.Error
	}
	listed := result.Value.(Inventory)
	if listed.Partial {
		return resolved{}, Failed("inventory_incomplete", "not_sent").Error
	}
	matches := []ObservedSession{}
	for _, peer := range listed.Peers {
		for _, row := range peer.Sessions {
			if ref.IsZero() && row.TerminalHandle != request.Handle || !ref.IsZero() && !row.Ref.SessionEqual(ref) {
				continue
			}
			if request.Operation == "info" && row.Connection == nil {
				current := row.Current(peer)
				row.Execution = &current
			}
			matches = append(matches, ObservedSession{Label: peer.Label, Machine: peer.Machine, ObservedAt: peer.ObservedAt, Session: row})
		}
	}
	switch {
	case len(matches) == 0 && !ref.IsZero():
		return resolved{}, Failed("SessionIdentityMismatch", "not_sent").Error
	case len(matches) == 0:
		return resolved{}, Failed("handle_not_found", "not_sent").Error
	case len(matches) > 1:
		return resolved{}, Failed("handle_ambiguous", "not_sent").Error
	}
	observed := matches[0]
	if ref.IsZero() {
		ref = observed.Session.Ref.Reference
	}
	selected, ok := client.peerByMachine(ref.Machine)
	if !ok {
		return resolved{}, Failed("machine_unknown", "not_sent").Error
	}
	return resolved{selected, ref, observed}, nil
}

// info is the resolved row with its display context; explain replaces its
// status with one fresh sample and that sample's evidence.
func (client *Client) info(ctx context.Context, resolved resolved, explain bool) Result {
	observed := resolved.observed
	source := Peer{Label: observed.Label, Machine: observed.Machine, OK: true, Sessions: []Session{observed.Session}}
	observed.Session = client.EnrichContexts(ctx, source, []Peer{source}).Sessions[0]
	if explain {
		// The explained status replaces the inventory's, so the printed
		// status and its evidence come from one sample. The inspect
		// response carries no clock, so observedAt becomes the client
		// time it arrived.
		ref := resolved.ref
		encoded, _ := json.Marshal(map[string]any{"identityToken": ref.IdentityToken, "paneId": ref.PaneID, "explain": true})
		inspected := client.call(ctx, resolved.peer, "terminal_explain", ref.TmuxID, encoded)
		if !inspected.OK {
			return inspected
		}
		value := inspected.Value.(api.Inspect)
		observed.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		observed.Session.TerminalStatus, observed.Diagnostics = value.TerminalStatus, value.Diagnostics
		observed.Session.TerminalUsage = nil
		observed.Session.TerminalFooter = nil
	}
	return success(observed)
}

// ResolveTerminal resolves a handle once; an exact captured reference needs no inventory.
func (client *Client) ResolveTerminal(ctx context.Context, request Request) (Reference, *Failure) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	resolved, failure := client.resolve(ctx, request)
	return resolved.ref, failure
}

// OpenTerminal shares the configured transport and identity binding with control calls.
func (client *Client) OpenTerminal(ctx context.Context, ref Reference) (*websocket.Conn, *Failure) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	selected, ok := client.peerByMachine(ref.Machine)
	if !ok {
		return nil, Failed("machine_unknown", "not_sent").Error
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
				if refusal := decodeFailure("enter", encoded, "not_sent", selected.Machine); refusal != nil {
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

// routes is every gateway call. {id} is the session's tmux id or a connection id.
// A write may take effect once sent: after transmission its failures default to
// dispatch unknown, and nothing ever replays it.
var routes = map[string]struct {
	method, path string
	write        bool
	status       int
}{
	"list":             {http.MethodGet, "/v1/sessions", false, http.StatusOK},
	"profile_usage":    {http.MethodGet, "/v1/profile-usage", false, http.StatusOK},
	"terminal_context": {http.MethodGet, "/v1/terminal-contexts/{id}", false, http.StatusOK},
	"directory_search": {http.MethodPost, "/v1/directory-searches", false, http.StatusOK},
	"terminal_inspect": {http.MethodPost, "/v1/sessions/{id}/terminal/inspect", false, http.StatusOK},
	"terminal_explain": {http.MethodPost, "/v1/sessions/{id}/terminal/inspect", false, http.StatusOK},
	"terminal_read":    {http.MethodPost, "/v1/sessions/{id}/terminal/read", false, http.StatusOK},
	"terminal_send":    {http.MethodPost, "/v1/sessions/{id}/terminal/send", true, http.StatusOK},
	"terminal_text":    {http.MethodPost, "/v1/sessions/{id}/terminal/text", true, http.StatusOK},
	"terminal_keys":    {http.MethodPost, "/v1/sessions/{id}/terminal/keys", true, http.StatusOK},
	"terminal_stop":    {http.MethodPost, "/v1/sessions/{id}/terminal/stop", true, http.StatusOK},
	"close":            {http.MethodPost, "/v1/sessions/{id}/terminal/close", true, http.StatusOK},
	"terminal_close":   {http.MethodDelete, "/v1/sessions/{id}", true, http.StatusNoContent},
	"rename":           {http.MethodPatch, "/v1/sessions/{id}", true, http.StatusNoContent},
	"group":            {http.MethodPut, "/v1/sessions/{id}/group", true, http.StatusNoContent},
	"shell":            {http.MethodPost, "/v1/sessions/{id}/shell", true, http.StatusCreated},
	"start":            {http.MethodPost, "/v1/sessions", true, http.StatusCreated},
}

func (client *Client) call(ctx context.Context, target peer, operation, id string, body []byte) Result {
	route, ok := routes[operation]
	if !ok {
		panic("unknown gateway operation " + operation) // justify-defect: operations are package literals.
	}
	dispatch := "not_sent"
	if target.Origin == "" || ctx.Err() != nil {
		return Failed("unavailable", dispatch)
	}
	var reader io.Reader
	if route.method != http.MethodGet {
		// No GetBody: net/http cannot replay a possibly delivered mutation.
		reader = io.NopCloser(bytes.NewReader(body))
	}
	request, err := http.NewRequestWithContext(ctx, route.method, target.Origin+strings.Replace(route.path, "{id}", id, 1), reader)
	if err != nil {
		return Failed("invalid_input", dispatch)
	}
	request.Header.Set("Authorization", "Bearer "+target.Bearer)
	request.Header.Set("Skidbladnir-Machine", target.Machine)
	request.Header.Set("Accept", "application/json")
	if reader != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if route.write {
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
	if response.StatusCode == http.StatusNoContent && route.status == http.StatusNoContent {
		if len(encoded) != 0 {
			return Failed("protocol_error", dispatch)
		}
		return success(nil)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return Failed("protocol_error", dispatch)
	}
	if response.StatusCode != route.status {
		if failure := decodeFailure(operation, encoded, dispatch, target.Machine); failure != nil {
			return Result{Error: failure}
		}
		return Failed("protocol_error", dispatch)
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

// decodeFailure reads one gateway error envelope. dispatch is the call's default: the body's own
// dispatch wins, else a known rejection was not sent. Only creation carries a created target.
func decodeFailure(operation string, encoded []byte, dispatch, machine string) *Failure {
	var envelope api.Error
	if json.Unmarshal(encoded, &envelope) != nil || envelope.Code == "" {
		return nil
	}
	switch {
	case envelope.Dispatch == "not_sent" || envelope.Dispatch == "unknown":
		dispatch = envelope.Dispatch
	case envelope.Dispatch != "":
		return nil
	case knownRejection(envelope.Code):
		dispatch = "not_sent"
	}
	failure := &Failure{Code: envelope.Code, Dispatch: dispatch}
	if envelope.Code == "TerminalInputBlocked" {
		failure.terminalInputMessage = envelope.Message
	}
	if target := envelope.Target; target != nil && (operation == "start" || operation == "shell") {
		ref := Reference{Machine: machine, TmuxID: target.TmuxID, IdentityToken: target.IdentityToken, PaneID: target.PaneID}
		if !ref.valid() {
			return nil
		}
		failure.Target = Ref{ref}
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
		if result.Value.(api.WriteResult).Outcome == "unknown" {
			return 1
		}
	case "close":
		if _, ok := result.Value.(TerminalCloseResult); ok {
			return 0 // Execute builds it only after a confirmed deletion
		}
		value := result.Value.(api.CloseResult)
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
