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

// ParseServerText reads one gateway frame for the desktop client: tolerant, as rules.md asks of a
// same-release document. Only the kind is checked; the client branches on it. An Error's message
// is the gateway's copy and is shown as sent.
func ParseServerText(encoded []byte) (ServerFrame, error) {
	var frame struct {
		Kind            string      `json:"kind"`
		AttachedClients int         `json:"attachedClients"`
		PaneID          string      `json:"paneId"`
		Foreground      *Foreground `json:"foreground"`
		Error           ErrorFrame  `json:"error"`
	}
	if json.Unmarshal(encoded, &frame) != nil {
		return nil, ErrInvalidFrame
	}
	switch frame.Kind {
	case "Hello":
		return HelloFrame{AttachedClients: frame.AttachedClients, PaneID: frame.PaneID, Foreground: frame.Foreground}, nil
	case "Presence":
		return PresenceFrame{AttachedClients: frame.AttachedClients}, nil
	case "Error":
		return frame.Error, nil
	}
	return nil, ErrInvalidFrame
}
