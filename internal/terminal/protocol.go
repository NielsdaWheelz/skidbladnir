package terminal

import (
	"encoding/json"
	"errors"
	"strconv"

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

type HelloFrame struct {
	AttachedClients int
	PaneID          string
	Foreground      *Foreground
}

type Foreground struct {
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}
type PresenceFrame struct{ AttachedClients int }
type ErrorFrame struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (HelloFrame) isServerFrame()    {}
func (PresenceFrame) isServerFrame() {}
func (ErrorFrame) isServerFrame()    {}

func EncodeHello(frame HelloFrame) ([]byte, error) {
	if !validHello(frame) {
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind            string      `json:"kind"`
		AttachedClients int         `json:"attachedClients"`
		PaneID          string      `json:"paneId"`
		Foreground      *Foreground `json:"foreground,omitempty"`
	}{"Hello", frame.AttachedClients, frame.PaneID, frame.Foreground})
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
	case "Hello":
		var frame struct {
			Kind            string          `json:"kind"`
			AttachedClients int             `json:"attachedClients"`
			PaneID          string          `json:"paneId"`
			Foreground      json.RawMessage `json:"foreground"`
		}
		if strictjson.Decode(encoded, &frame) != nil {
			return nil, ErrInvalidFrame
		}
		hello := HelloFrame{AttachedClients: frame.AttachedClients, PaneID: frame.PaneID}
		if len(frame.Foreground) > 0 {
			var foreground Foreground
			if strictjson.Decode(frame.Foreground, &foreground) != nil {
				return nil, ErrInvalidFrame
			}
			hello.Foreground = &foreground
		}
		if !validHello(hello) {
			return nil, ErrInvalidFrame
		}
		return hello, nil
	case "Presence":
		var frame struct {
			Kind            string `json:"kind"`
			AttachedClients int    `json:"attachedClients"`
		}
		if strictjson.Decode(encoded, &frame) != nil || frame.AttachedClients < 1 {
			return nil, ErrInvalidFrame
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

func validHello(frame HelloFrame) bool {
	if frame.AttachedClients < 1 || len(frame.PaneID) < 2 || frame.PaneID[0] != '%' {
		return false
	}
	for _, character := range frame.PaneID[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return frame.Foreground == nil || validForeground(*frame.Foreground)
}

func validForeground(foreground Foreground) bool {
	start, err := strconv.ParseUint(foreground.StartIdentity, 10, 64)
	return (foreground.Provider == "Codex" || foreground.Provider == "Claude") && foreground.PID > 0 &&
		err == nil && start > 0 && strconv.FormatUint(start, 10) == foreground.StartIdentity
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
