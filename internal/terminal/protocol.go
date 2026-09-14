package terminal

import (
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const (
	MaximumFrameBytes = 64 * 1024
	MinimumColumns    = 20
	MaximumColumns    = 1024
	MinimumRows       = 5
	MaximumRows       = 512
)

var (
	ErrInvalidFrame  = errors.New("invalid terminal frame")
	ErrFrameTooLarge = errors.New("terminal frame exceeds 64 KiB")
)

type ErrorCode string

const (
	ErrorInvalidRequest                   ErrorCode = "InvalidRequest"
	ErrorRequestTooLarge                  ErrorCode = "RequestTooLarge"
	ErrorReconnectRequired                ErrorCode = "ReconnectRequired"
	ErrorInternal                         ErrorCode = "InternalError"
	ErrorTerminalConfigurationUnsupported ErrorCode = "TerminalConfigurationUnsupported"
)

type ClientFrame interface {
	isClientFrame()
}

type ResizeFrame struct {
	Columns int
	Rows    int
}

func (ResizeFrame) isClientFrame() {}

type DetachFrame struct{}

func (DetachFrame) isClientFrame() {}

type ServerFrame interface {
	isServerFrame()
}

type HelloFrame struct{ AttachedClients int }
type PresenceFrame struct{ AttachedClients int }
type ErrorFrame struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (HelloFrame) isServerFrame()    {}
func (PresenceFrame) isServerFrame() {}
func (ErrorFrame) isServerFrame()    {}

func EncodeHello(attachedClients int) ([]byte, error) {
	return encodePresence("Hello", attachedClients)
}

func EncodePresence(attachedClients int) ([]byte, error) {
	return encodePresence("Presence", attachedClients)
}

func errorMessage(code ErrorCode) string {
	switch code {
	case ErrorInvalidRequest:
		return "The request is not valid."
	case ErrorRequestTooLarge:
		return "The request is too large."
	case ErrorReconnectRequired:
		return "Reconnect required."
	case ErrorInternal:
		return "Skíðblaðnir could not complete the request."
	case ErrorTerminalConfigurationUnsupported:
		return "tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on."
	default:
		return ""
	}
}

func EncodeError(code ErrorCode) ([]byte, error) {
	message := errorMessage(code)
	if message == "" {
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind  string     `json:"kind"`
		Error ErrorFrame `json:"error"`
	}{
		Kind:  "Error",
		Error: ErrorFrame{Code: code, Message: message},
	})
}

func EncodeResize(columns, rows int) ([]byte, error) {
	if columns < MinimumColumns || columns > MaximumColumns || rows < MinimumRows || rows > MaximumRows {
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind    string `json:"kind"`
		Columns int    `json:"columns"`
		Rows    int    `json:"rows"`
	}{Kind: "Resize", Columns: columns, Rows: rows})
}

func EncodeDetach() []byte { return []byte(`{"kind":"Detach"}`) }

func ParseServerText(encoded []byte) (ServerFrame, error) {
	if len(encoded) > MaximumFrameBytes {
		return nil, ErrFrameTooLarge
	}
	var envelope struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return nil, ErrInvalidFrame
	}
	switch envelope.Kind {
	case "Hello", "Presence":
		var frame struct {
			Kind            string `json:"kind"`
			AttachedClients int    `json:"attachedClients"`
		}
		if strictjson.Decode(encoded, &frame) != nil || frame.AttachedClients < 1 {
			return nil, ErrInvalidFrame
		}
		if frame.Kind == "Hello" {
			return HelloFrame{AttachedClients: frame.AttachedClients}, nil
		}
		return PresenceFrame{AttachedClients: frame.AttachedClients}, nil
	case "Error":
		var frame struct {
			Kind  string     `json:"kind"`
			Error ErrorFrame `json:"error"`
		}
		if strictjson.Decode(encoded, &frame) != nil || errorMessage(frame.Error.Code) == "" || frame.Error.Message != errorMessage(frame.Error.Code) {
			return nil, ErrInvalidFrame
		}
		return frame.Error, nil
	default:
		return nil, ErrInvalidFrame
	}
}

func ParseClientText(encoded []byte) (ClientFrame, error) {
	if len(encoded) > MaximumFrameBytes {
		return nil, ErrFrameTooLarge
	}
	var envelope struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return nil, ErrInvalidFrame
	}
	switch envelope.Kind {
	case "Resize":
		var frame struct {
			Kind    string `json:"kind"`
			Columns int    `json:"columns"`
			Rows    int    `json:"rows"`
		}
		if strictjson.Decode(encoded, &frame) != nil || frame.Kind != "Resize" ||
			frame.Columns < MinimumColumns || frame.Columns > MaximumColumns ||
			frame.Rows < MinimumRows || frame.Rows > MaximumRows {
			return nil, ErrInvalidFrame
		}
		return ResizeFrame{Columns: frame.Columns, Rows: frame.Rows}, nil
	case "Detach":
		var frame struct {
			Kind string `json:"kind"`
		}
		if strictjson.Decode(encoded, &frame) != nil || frame.Kind != "Detach" {
			return nil, ErrInvalidFrame
		}
		return DetachFrame{}, nil
	default:
		return nil, ErrInvalidFrame
	}
}

func ValidateClientBinary(contents []byte) error {
	if len(contents) > MaximumFrameBytes {
		return ErrFrameTooLarge
	}
	return nil
}

func encodePresence(kind string, attachedClients int) ([]byte, error) {
	if attachedClients < 1 {
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind            string `json:"kind"`
		AttachedClients int    `json:"attachedClients"`
	}{Kind: kind, AttachedClients: attachedClients})
}
