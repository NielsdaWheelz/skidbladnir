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

var ErrInvalidFrame = errors.New("invalid terminal frame")

type ErrorCode string

const (
	ErrorInvalidRequest                   ErrorCode = "InvalidRequest"
	ErrorRequestTooLarge                  ErrorCode = "RequestTooLarge"
	ErrorReconnectRequired                ErrorCode = "ReconnectRequired"
	ErrorInternal                         ErrorCode = "InternalError"
	ErrorTerminalConfigurationUnsupported ErrorCode = "TerminalConfigurationUnsupported"
)

var errorMessages = map[ErrorCode]string{
	ErrorInvalidRequest:                   "The request is not valid.",
	ErrorRequestTooLarge:                  "The request is too large.",
	ErrorReconnectRequired:                "Reconnect required.",
	ErrorInternal:                         "Skíðblaðnir could not complete the request.",
	ErrorTerminalConfigurationUnsupported: "tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on.",
}

// ClientFrame is a Detach, or a Resize to Columns x Rows.
type ClientFrame struct {
	Detach        bool
	Columns, Rows int
}

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

func ValidSize(columns, rows int) bool {
	return columns >= MinimumColumns && columns <= MaximumColumns && rows >= MinimumRows && rows <= MaximumRows
}

func EncodeHello(frame HelloFrame) []byte {
	payload, _ := json.Marshal(struct {
		Kind            string      `json:"kind"`
		AttachedClients int         `json:"attachedClients"`
		PaneID          string      `json:"paneId"`
		Foreground      *Foreground `json:"foreground,omitempty"`
	}{"Hello", frame.AttachedClients, frame.PaneID, frame.Foreground})
	return payload
}

func EncodePresence(attachedClients int) []byte {
	payload, _ := json.Marshal(struct {
		Kind            string `json:"kind"`
		AttachedClients int    `json:"attachedClients"`
	}{"Presence", attachedClients})
	return payload
}

func EncodeError(code ErrorCode) []byte {
	payload, _ := json.Marshal(struct {
		Kind  string     `json:"kind"`
		Error ErrorFrame `json:"error"`
	}{"Error", ErrorFrame{Code: code, Message: errorMessages[code]}})
	return payload
}

func EncodeResize(columns, rows int) []byte {
	payload, _ := json.Marshal(struct {
		Kind    string `json:"kind"`
		Columns int    `json:"columns"`
		Rows    int    `json:"rows"`
	}{"Resize", columns, rows})
	return payload
}

func EncodeDetach() []byte { return []byte(`{"kind":"Detach"}`) }

// ParseClientText strictly decodes gateway ingress: an exact Resize within the
// geometry bounds, or an exact Detach. Callers cap the frame size.
func ParseClientText(encoded []byte) (ClientFrame, error) {
	var resize struct {
		Kind    string `json:"kind"`
		Columns int    `json:"columns"`
		Rows    int    `json:"rows"`
	}
	if strictjson.Decode(encoded, &resize) == nil && resize.Kind == "Resize" && ValidSize(resize.Columns, resize.Rows) {
		return ClientFrame{Columns: resize.Columns, Rows: resize.Rows}, nil
	}
	var detach struct {
		Kind string `json:"kind"`
	}
	if strictjson.Decode(encoded, &detach) == nil && detach.Kind == "Detach" {
		return ClientFrame{Detach: true}, nil
	}
	return ClientFrame{}, ErrInvalidFrame
}

// ParseServerText strictly decodes one gateway frame for the desktop client. An
// Error's message is the gateway's copy and is shown as sent.
func ParseServerText(encoded []byte) (ServerFrame, error) {
	var envelope struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(encoded, &envelope) != nil {
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
		if strictjson.Decode(encoded, &frame) != nil || frame.AttachedClients < 1 || !validPaneID(frame.PaneID) {
			return nil, ErrInvalidFrame
		}
		hello := HelloFrame{AttachedClients: frame.AttachedClients, PaneID: frame.PaneID}
		if frame.Foreground != nil { // present, including null, which fails validForeground
			hello.Foreground = &Foreground{}
			if strictjson.Decode(frame.Foreground, hello.Foreground) != nil || !validForeground(*hello.Foreground) {
				return nil, ErrInvalidFrame
			}
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
		if strictjson.Decode(encoded, &frame) != nil || errorMessages[frame.Error.Code] == "" {
			return nil, ErrInvalidFrame
		}
		return frame.Error, nil
	default:
		return nil, ErrInvalidFrame
	}
}

func validPaneID(paneID string) bool {
	if len(paneID) < 2 || paneID[0] != '%' {
		return false
	}
	for _, character := range paneID[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validForeground(foreground Foreground) bool {
	start, err := strconv.ParseUint(foreground.StartIdentity, 10, 64)
	return (foreground.Provider == "Codex" || foreground.Provider == "Claude") && foreground.PID > 0 &&
		err == nil && start > 0 && strconv.FormatUint(start, 10) == foreground.StartIdentity
}
