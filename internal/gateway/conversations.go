package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type conversationRequest struct {
	Conversation *agentruntime.Conversation `json:"conversation"`
	Turn         *agentruntime.Turn         `json:"turn"`
	Scope        stringField                `json:"scope"`
	Cursor       stringField                `json:"cursor"`
	Input        stringField                `json:"input"`
	Delivery     stringField                `json:"delivery"`
	Text         stringField                `json:"text"`
	MaxBytes     *int                       `json:"maxBytes"`
}

func (input *conversationRequest) UnmarshalJSON(encoded []byte) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	for _, value := range members {
		if bytes.Equal(value, []byte("null")) {
			return errors.New("conversation request member cannot be null")
		}
	}
	type wire conversationRequest
	var decoded wire
	if err := strictjson.Decode(encoded, &decoded); err != nil {
		return err
	}
	*input = conversationRequest(decoded)
	return nil
}

func (input conversationRequest) valid(operation string) bool {
	if input.Conversation == nil || !input.Conversation.Valid() ||
		input.Turn != nil && (!input.Turn.Valid() || input.Turn.State != "inProgress" || input.Conversation.Provider != agentruntime.ProviderCodex) {
		return false
	}
	switch operation {
	case "inspect":
		return input.Turn == nil && !input.Scope.present && !input.Cursor.present && !input.Input.present && !input.Delivery.present && !input.Text.present && input.MaxBytes == nil
	case "read":
		return input.Turn == nil && (input.Scope.value == "latest" || input.Scope.value == "history") && !input.Cursor.present && !input.Input.present && !input.Delivery.present && !input.Text.present && (input.MaxBytes == nil || *input.MaxBytes > 0 && *input.MaxBytes <= 32768)
	case "send":
		return input.Turn == nil && !input.Scope.present && !input.Cursor.present && (input.Input.value == "peer" || input.Input.value == "user") && (input.Delivery.value == "direct" || input.Delivery.value == "queue" && input.Input.value == "user") && input.Text.present && input.MaxBytes == nil
	case "stop":
		return !input.Scope.present && !input.Cursor.present && !input.Input.present && !input.Delivery.present && !input.Text.present && input.MaxBytes == nil
	case "results":
		return input.Turn == nil && !input.Scope.present && (!input.Cursor.present || input.Cursor.value != "") && !input.Input.present && !input.Delivery.present && !input.Text.present && input.MaxBytes == nil
	default:
		return false
	}
}

func (gateway *Gateway) conversationOperation(writer http.ResponseWriter, request *http.Request) {
	operation := strings.TrimPrefix(request.URL.Path, "/v1/conversations/")
	input, failure := decodeJSON[conversationRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.valid(operation) {
		writeError(writer, errorAgentInputInvalid)
		return
	}
	ctx := request.Context()
	var result any
	var err error
	switch operation {
	case "inspect":
		result, err = gateway.agents.Inspect(ctx, *input.Conversation)
	case "read":
		maxBytes := 0
		if input.MaxBytes != nil {
			maxBytes = *input.MaxBytes
		}
		read, failure := gateway.agents.Read(ctx, *input.Conversation, input.Scope.value, maxBytes)
		if failure != nil {
			writeAgentError(writer, failure)
			return
		}
		writeAgentRead(writer, read)
		return
	case "send":
		result, err = gateway.agents.Send(ctx, *input.Conversation, input.Text.value, input.Input.value, input.Delivery.value)
	case "stop":
		result, err = gateway.agents.Stop(ctx, *input.Conversation, input.Turn)
	case "results":
		result, err = gateway.agents.Results(ctx, *input.Conversation, input.Cursor.value)
	}
	if err != nil {
		writeAgentError(writer, err)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > int(MaximumBodyBytes) {
		writeError(writer, errorInternal)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

// The caller holds terminalLifecycle; no foreground provider identity is required.
func (gateway *Gateway) closeSessionTerminal(ctx context.Context, id, identityToken string) error {
	input, err := gateway.sessions.SessionKillInput(ctx, id, identityToken)
	if err != nil {
		return err
	}
	if err := gateway.closeLiveTerminals(ctx, id); err != nil {
		return err
	}
	return gateway.sessions.Kill(ctx, input)
}
