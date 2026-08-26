package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	MaximumFrameBytes = 64 * 1024
	MinimumColumns    = 20
	MaximumColumns    = 240
	MinimumRows       = 5
	MaximumRows       = 120
)

var (
	ErrInvalidFrame  = errors.New("invalid terminal frame")
	ErrFrameTooLarge = errors.New("terminal frame exceeds 64 KiB")
)

type Geometry string

const (
	GeometryOwner       Geometry = "Owner"
	GeometryConstrained Geometry = "Constrained"
)

type ErrorCode string

const (
	ErrorInvalidRequest    ErrorCode = "InvalidRequest"
	ErrorRequestTooLarge   ErrorCode = "RequestTooLarge"
	ErrorReconnectRequired ErrorCode = "ReconnectRequired"
	ErrorInternal          ErrorCode = "InternalError"
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

func EncodeHello(attachedClients int, geometry Geometry) ([]byte, error) {
	return encodePresence("Hello", attachedClients, geometry)
}

func EncodePresence(attachedClients int, geometry Geometry) ([]byte, error) {
	return encodePresence("Presence", attachedClients, geometry)
}

func EncodeError(code ErrorCode) ([]byte, error) {
	message := ""
	switch code {
	case ErrorInvalidRequest:
		message = "The request is not valid."
	case ErrorRequestTooLarge:
		message = "The request is too large."
	case ErrorReconnectRequired:
		message = "Reconnect required."
	case ErrorInternal:
		message = "Skíðblaðnir could not complete the request."
	default:
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Error struct {
			Code    ErrorCode `json:"code"`
			Message string    `json:"message"`
		} `json:"error"`
	}{
		Kind: "Error",
		Error: struct {
			Code    ErrorCode `json:"code"`
			Message string    `json:"message"`
		}{Code: code, Message: message},
	})
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
		if !decodeExact(encoded, &frame) || frame.Kind != "Resize" ||
			frame.Columns < MinimumColumns || frame.Columns > MaximumColumns ||
			frame.Rows < MinimumRows || frame.Rows > MaximumRows {
			return nil, ErrInvalidFrame
		}
		return ResizeFrame{Columns: frame.Columns, Rows: frame.Rows}, nil
	case "Detach":
		var frame struct {
			Kind string `json:"kind"`
		}
		if !decodeExact(encoded, &frame) || frame.Kind != "Detach" {
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

func encodePresence(kind string, attachedClients int, geometry Geometry) ([]byte, error) {
	if attachedClients < 1 || (geometry != GeometryOwner && geometry != GeometryConstrained) {
		return nil, ErrInvalidFrame
	}
	return json.Marshal(struct {
		Kind            string   `json:"kind"`
		AttachedClients int      `json:"attachedClients"`
		Geometry        Geometry `json:"geometry"`
	}{Kind: kind, AttachedClients: attachedClients, Geometry: geometry})
}

func decodeExact(encoded []byte, destination any) bool {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}
