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
				return terminal.EncodeHello(2)
			},
			want: `{"kind":"Hello","attachedClients":2}`,
		},
		{
			name: "presence",
			encode: func() ([]byte, error) {
				return terminal.EncodePresence(1)
			},
			want: `{"kind":"Presence","attachedClients":1}`,
		},
		{
			name: "reconnect required",
			encode: func() ([]byte, error) {
				return terminal.EncodeError(terminal.ErrorReconnectRequired)
			},
			want: `{"kind":"Error","error":{"code":"ReconnectRequired","message":"Reconnect required."}}`,
		},
		{
			name: "unsupported tmux configuration",
			encode: func() ([]byte, error) {
				return terminal.EncodeError(terminal.ErrorCode("TerminalConfigurationUnsupported"))
			},
			want: `{"kind":"Error","error":{"code":"TerminalConfigurationUnsupported","message":"tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on."}}`,
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

func TestDesktopResizeUsesMeasuredGeometry(t *testing.T) {
	frame, err := terminal.ParseClientText([]byte(`{"kind":"Resize","columns":1024,"rows":512}`))
	if err != nil || frame != (terminal.ResizeFrame{Columns: 1024, Rows: 512}) {
		t.Fatalf("desktop geometry was rejected or changed: frame=%v err=%v", frame, err)
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
		`{"kind":"Hello","attachedClients":1}`,
		`{"kind":"Resize","columns":19,"rows":40}`,
		`{"kind":"Resize","columns":120,"rows":513}`,
		`{"kind":"Resize","columns":1025,"rows":40}`,
		`{"kind":"Resize","columns":120,"rows":40,"extra":true}`,
		`{"kind":"Detach","extra":true}`,
		`{"kind":"Detach","Kind":"Detach"}`,
		`{"kind":"Detach","kind":"Detach"}`,
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

func TestPresenceRejectsAnImpossibleClientCount(t *testing.T) {
	if _, err := terminal.EncodeHello(0); !errors.Is(err, terminal.ErrInvalidFrame) {
		t.Fatalf("expected zero-client Hello to fail; got %v", err)
	}
}

func TestDesktopClientUsesTheSameWireProtocol(t *testing.T) {
	for _, encode := range []func() ([]byte, error){
		func() ([]byte, error) { return terminal.EncodeHello(2) },
		func() ([]byte, error) { return terminal.EncodePresence(1) },
		func() ([]byte, error) { return terminal.EncodeError(terminal.ErrorReconnectRequired) },
	} {
		encoded, err := encode()
		if err != nil {
			t.Fatal(err)
		}
		frame, err := terminal.ParseServerText(encoded)
		if err != nil {
			t.Fatalf("desktop could not decode host output: %v", err)
		}
		switch value := frame.(type) {
		case terminal.HelloFrame:
			if value.AttachedClients != 2 {
				t.Fatal("hello lost presence")
			}
		case terminal.PresenceFrame:
			if value.AttachedClients != 1 {
				t.Fatal("presence count changed")
			}
		case terminal.ErrorFrame:
			if value.Code != terminal.ErrorReconnectRequired || value.Message != "Reconnect required." {
				t.Fatal("terminal error changed")
			}
		default:
			t.Fatalf("unexpected server frame %T", frame)
		}
	}
	encoded, err := terminal.EncodeResize(320, 150)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := terminal.ParseClientText(encoded)
	if err != nil || frame != (terminal.ResizeFrame{Columns: 320, Rows: 150}) {
		t.Fatalf("host could not decode desktop resize: %v", err)
	}
	if frame, err := terminal.ParseClientText(terminal.EncodeDetach()); err != nil || frame != (terminal.DetachFrame{}) {
		t.Fatalf("host could not decode desktop detach: %v", err)
	}
	if _, err := terminal.EncodeResize(19, 5); !errors.Is(err, terminal.ErrInvalidFrame) {
		t.Fatal("desktop encoded an unsupported viewport")
	}
}

func TestDesktopRejectsMalformedServerFrames(t *testing.T) {
	for _, encoded := range []string{
		`{"kind":"Hello","attachedClients":0}`,
		`{"kind":"Presence","attachedClients":1,"extra":true}`,
		`{"kind":"Presence","AttachedClients":1}`,
		`{"kind":"Presence","attachedClients":1,"attachedClients":2}`,
		`{"kind":"Error","error":{"code":"SessionNotFound","message":"missing"}}`,
		`{"kind":"Error","error":{"code":"ReconnectRequired","message":"other"}}`,
		`{"kind":"Error","error":{"code":"ReconnectRequired","message":"Reconnect required.","extra":true}}`,
		`{"kind":"Detach"}`,
		`{"kind":"Hello","attachedClients":1} {}`,
		`null`,
	} {
		if _, err := terminal.ParseServerText([]byte(encoded)); !errors.Is(err, terminal.ErrInvalidFrame) {
			t.Errorf("desktop accepted malformed server frame: %s", encoded)
		}
	}
	if _, err := terminal.ParseServerText(bytes.Repeat([]byte{' '}, terminal.MaximumFrameBytes+1)); !errors.Is(err, terminal.ErrFrameTooLarge) {
		t.Fatal("desktop accepted an oversized server frame")
	}
}
