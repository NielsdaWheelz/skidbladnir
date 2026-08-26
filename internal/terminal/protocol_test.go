package terminal_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminal"
)

func TestServerTextFramesHaveClosedWireShapes(t *testing.T) {
	tests := []struct {
		name   string
		encode func() ([]byte, error)
		want   string
	}{
		{
			name: "hello",
			encode: func() ([]byte, error) {
				return terminal.EncodeHello(2, terminal.GeometryConstrained)
			},
			want: `{"kind":"Hello","attachedClients":2,"geometry":"Constrained"}`,
		},
		{
			name: "presence",
			encode: func() ([]byte, error) {
				return terminal.EncodePresence(1, terminal.GeometryOwner)
			},
			want: `{"kind":"Presence","attachedClients":1,"geometry":"Owner"}`,
		},
		{
			name: "reconnect required",
			encode: func() ([]byte, error) {
				return terminal.EncodeError(terminal.ErrorReconnectRequired)
			},
			want: `{"kind":"Error","error":{"code":"ReconnectRequired","message":"Reconnect required."}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.encode()
			if err != nil {
				t.Fatalf("encode terminal frame: %v", err)
			}
			if string(encoded) != test.want {
				t.Fatalf("unexpected terminal frame\nwant: %s\n got: %s", test.want, encoded)
			}
		})
	}
}

func TestClientTextFramesDecodeOnlyResizeAndDetach(t *testing.T) {
	resize, err := terminal.ParseClientText([]byte(`{"kind":"Resize","columns":120,"rows":40}`))
	if err != nil {
		t.Fatalf("decode Resize: %v", err)
	}
	if resize != (terminal.ResizeFrame{Columns: 120, Rows: 40}) {
		t.Fatalf("unexpected Resize frame: %#v", resize)
	}

	detach, err := terminal.ParseClientText([]byte(`{"kind":"Detach"}`))
	if err != nil {
		t.Fatalf("decode Detach: %v", err)
	}
	if detach != (terminal.DetachFrame{}) {
		t.Fatalf("unexpected Detach frame: %#v", detach)
	}
}

func TestClientTextFramesRejectEveryOtherShape(t *testing.T) {
	invalid := []string{
		`{"kind":"Hello","attachedClients":1,"geometry":"Owner"}`,
		`{"kind":"Resize","columns":19,"rows":40}`,
		`{"kind":"Resize","columns":120,"rows":121}`,
		`{"kind":"Resize","columns":120,"rows":40,"extra":true}`,
		`{"kind":"Detach","extra":true}`,
		`{"kind":"Unknown"}`,
		`{"kind":"Detach"} {"kind":"Detach"}`,
		`null`,
	}

	for _, encoded := range invalid {
		_, err := terminal.ParseClientText([]byte(encoded))
		if !errors.Is(err, terminal.ErrInvalidFrame) {
			t.Errorf("expected invalid-frame error for %s; got %v", encoded, err)
		}
	}
}

func TestTerminalFrameBoundIsExact(t *testing.T) {
	if err := terminal.ValidateClientBinary(make([]byte, terminal.MaximumFrameBytes)); err != nil {
		t.Fatalf("accept maximum binary frame: %v", err)
	}
	if err := terminal.ValidateClientBinary(make([]byte, terminal.MaximumFrameBytes+1)); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected oversized binary-frame error; got %v", err)
	}

	oversizedText := bytes.Repeat([]byte{'x'}, terminal.MaximumFrameBytes+1)
	if _, err := terminal.ParseClientText(oversizedText); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatalf("expected oversized text-frame error; got %v", err)
	}
}

func TestPresenceRejectsImpossibleContent(t *testing.T) {
	if _, err := terminal.EncodeHello(0, terminal.GeometryOwner); !errors.Is(err, terminal.ErrInvalidFrame) {
		t.Fatalf("expected zero-client Hello to fail; got %v", err)
	}
	if _, err := terminal.EncodePresence(1, terminal.Geometry("Other")); !errors.Is(err, terminal.ErrInvalidFrame) {
		t.Fatalf("expected unknown geometry to fail; got %v", err)
	}
}
