// Package macnotifications owns the local mac notification app's socket contract.
// Native operations stay in the app; clients read state, register visits, and open setup.
package macnotifications

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

const operationTimeout = 3 * time.Second

var ErrUnavailable = errors.New("notifications unavailable")

type View struct {
	DeviceSnapshot          attention.DeviceSnapshot `json:"deviceSnapshot"`
	ProducerAvailable       bool                     `json:"producerAvailable"`
	CurrentProducerSnapshot *attention.Snapshot      `json:"currentProducerSnapshot,omitempty"`
}

func (view *View) UnmarshalJSON(data []byte) error {
	var wire struct {
		DeviceSnapshot          *attention.DeviceSnapshot `json:"deviceSnapshot"`
		ProducerAvailable       *bool                     `json:"producerAvailable"`
		CurrentProducerSnapshot *attention.Snapshot       `json:"currentProducerSnapshot,omitempty"`
	}
	if attention.Decode(data, &wire) != nil || wire.DeviceSnapshot == nil || wire.ProducerAvailable == nil || !wire.DeviceSnapshot.Valid() || wire.CurrentProducerSnapshot != nil && !wire.CurrentProducerSnapshot.Valid() {
		return ErrUnavailable
	}
	if *wire.ProducerAvailable && (wire.CurrentProducerSnapshot == nil || wire.DeviceSnapshot.ObserverEpoch != wire.CurrentProducerSnapshot.Epoch || wire.DeviceSnapshot.AdmittedRevision != wire.CurrentProducerSnapshot.Revision) {
		return ErrUnavailable
	}
	*view = View{*wire.DeviceSnapshot, *wire.ProducerAvailable, wire.CurrentProducerSnapshot}
	return nil
}

// SurfaceAssociation is the creator's ephemeral receipt. A live attachment
// retains it across app reconnects; it is never an attention fact on disk.
type SurfaceAssociation struct {
	Nonce string `json:"nonce"`
	ID    string `json:"id"`
}

func validSurface(surface SurfaceAssociation) bool {
	return attention.EpochValid(surface.Nonce) && len(surface.ID) > 0 && len(surface.ID) <= 128 && utf8.ValidString(surface.ID) && strings.IndexFunc(surface.ID, unicode.IsControl) < 0
}

type RegisterArgs struct {
	Key         attention.Key         `json:"key"`
	Foreground  *attention.Foreground `json:"foreground,omitempty"`
	ReadyToken  *attention.ReadyToken `json:"readyToken,omitempty"`
	Presented   bool                  `json:"presented"`
	LaunchNonce string                `json:"launchNonce,omitempty"`
	Surface     *SurfaceAssociation   `json:"surface,omitempty"`
}

type registration struct {
	VisitID string              `json:"visitId"`
	Surface *SurfaceAssociation `json:"surface,omitempty"`
}

// Visit owns one connection and one registered attachment. Disconnecting ends
// that visit, even when the terminal cannot send an explicit end operation.
type Visit struct {
	operation chan struct{}
	presented atomic.Bool
	conn      net.Conn
	id        string
	args      RegisterArgs
	replies   chan []byte
	done      chan struct{}
	closed    bool
}

func SocketPath() (string, error) {
	directory, err := attention.StateDirectory()
	if err != nil {
		return "", ErrUnavailable
	}
	return filepath.Join(directory, "notifications.sock"), nil
}

func Read(ctx context.Context) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		return View{}, err
	}
	defer conn.Close() // justify-ignore-error: a read has no connection-owned visit.
	var view View
	if err := call(ctx, conn, bufio.NewReader(conn), "read", struct{}{}, &view); err != nil {
		return View{}, err
	}
	return view, nil
}

// Setup opens the already-running mac app's notification setup window.
func Setup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close() // justify-ignore-error: setup owns no connection-bound visit.
	var result *struct{}
	if err := call(ctx, conn, bufio.NewReader(conn), "setup", struct{}{}, &result); err != nil || result == nil {
		return ErrUnavailable
	}
	return nil
}

func Register(ctx context.Context, args RegisterArgs) (*Visit, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	var result registration
	if err := call(ctx, conn, reader, "register", args, &result); err != nil || !attention.EpochValid(result.VisitID) || result.Surface != nil && !validSurface(*result.Surface) {
		_ = conn.Close() // justify-ignore-error: registration failed; disconnect retires it.
		return nil, ErrUnavailable
	}
	visit := &Visit{args: args, operation: make(chan struct{}, 1)}
	visit.presented.Store(args.Presented)
	visit.connected(conn, reader, result)
	return visit, nil
}

func (visit *Visit) Presented(ctx context.Context, token attention.ReadyToken) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	// Actual output remains a fact even if the app disconnected before its
	// acknowledgement or serialization deadline. Reconnect never re-acks it.
	visit.presented.Store(true)
	if err := visit.acquire(ctx); err != nil {
		return err
	}
	defer visit.release()
	if visit.closed || visit.conn == nil {
		return ErrUnavailable
	}
	return visit.call(ctx, "presented", struct {
		VisitID string               `json:"visitId"`
		Token   attention.ReadyToken `json:"token"`
	}{visit.id, token})
}

func (visit *Visit) Disconnected() <-chan struct{} {
	visit.operation <- struct{}{}
	defer visit.release()
	return visit.done
}

// Reconnect is scoped by the terminal caller's lifetime and retry policy.
// It reconstructs the live association, never a presentation or ready token.
func (visit *Visit) Reconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := visit.acquire(ctx); err != nil {
		return err
	}
	defer visit.release()
	if visit.closed {
		return ErrUnavailable
	}
	if visit.conn != nil {
		_ = visit.conn.Close() // justify-ignore-error: a replacement connection ends its own prior visit.
		<-visit.done
	}
	conn, err := dial(ctx)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	args := visit.args
	args.Presented, args.ReadyToken = visit.presented.Load(), nil
	var result registration
	if err := call(ctx, conn, reader, "register", args, &result); err != nil || !attention.EpochValid(result.VisitID) || result.Surface != nil && !validSurface(*result.Surface) {
		_ = conn.Close() // justify-ignore-error: failed reconstruction retires its connection-owned visit.
		return ErrUnavailable
	}
	if args.Presented != visit.presented.Load() {
		_ = conn.Close() // justify-ignore-error: output during registration requires a fresh presented reconstruction, never replayed entry ack.
		return ErrUnavailable
	}
	visit.connected(conn, reader, result)
	return nil
}

func (visit *Visit) connected(conn net.Conn, reader *bufio.Reader, result registration) {
	visit.conn, visit.id = conn, result.VisitID
	visit.args.Surface = result.Surface
	if result.Surface != nil {
		visit.args.LaunchNonce = ""
	}
	visit.replies, visit.done = make(chan []byte, 1), make(chan struct{})
	replies, done := visit.replies, visit.done
	_ = conn.SetReadDeadline(time.Time{}) // justify-ignore-error: failure is observed by the connection reader.
	go func() {
		defer close(done)
		defer conn.Close() // justify-ignore-error: loss of a socket ends that connection's visit.
		for {
			line, err := readLine(reader, 1<<20)
			if err != nil {
				return
			}
			select {
			case replies <- line:
			default:
				return
			}
		}
	}()
}

func (visit *Visit) call(ctx context.Context, op string, args any) error {
	deadline, _ := ctx.Deadline()
	if ctx.Err() != nil || visit.conn.SetWriteDeadline(deadline) != nil {
		return ErrUnavailable
	}
	data, err := encodeRequest(op, args)
	if err != nil {
		return err
	}
	if _, err := visit.conn.Write(data); err != nil {
		_ = visit.conn.Close() // justify-ignore-error: an uncertain rpc cannot leave a reply for the next operation.
		return ErrUnavailable
	}
	select {
	case line := <-visit.replies:
		err := decodeReply(line, nil)
		if err != nil {
			_ = visit.conn.Close() // justify-ignore-error: an invalid or failed rpc ends its connection-owned visit.
		}
		return err
	case <-visit.done:
		return ErrUnavailable
	case <-ctx.Done():
		_ = visit.conn.Close() // justify-ignore-error: discard uncertain rpc state by ending its connection.
		return ErrUnavailable
	}
}

func (visit *Visit) End(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := visit.acquire(ctx); err != nil {
		return err
	}
	defer visit.release()
	if visit.closed || visit.conn == nil {
		return nil
	}
	err := visit.call(ctx, "end", struct {
		VisitID string `json:"visitId"`
	}{visit.id})
	closeErr := visit.conn.Close()
	<-visit.done
	visit.conn, visit.closed = nil, true
	if err != nil || closeErr != nil {
		return ErrUnavailable
	}
	return nil
}

func (visit *Visit) Close() error {
	visit.operation <- struct{}{}
	defer visit.release()
	if visit.closed || visit.conn == nil {
		return nil
	}
	err := visit.conn.Close()
	<-visit.done
	visit.conn, visit.closed = nil, true
	return err
}

func (visit *Visit) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ErrUnavailable
	case visit.operation <- struct{}{}:
		if ctx.Err() != nil {
			visit.release()
			return ErrUnavailable
		}
		return nil
	}
}

func (visit *Visit) release() { <-visit.operation }

func dial(ctx context.Context) (net.Conn, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, ErrUnavailable
	}
	return conn, nil
}

func call(ctx context.Context, conn net.Conn, reader *bufio.Reader, op string, args, result any) error {
	deadline := time.Now().Add(operationTimeout)
	if bound, ok := ctx.Deadline(); ok && bound.Before(deadline) {
		deadline = bound
	}
	if ctx.Err() != nil || conn.SetDeadline(deadline) != nil {
		return ErrUnavailable
	}
	encoded, err := encodeRequest(op, args)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() }) // justify-ignore-error: cancellation owns this bounded rpc's connection.
	defer stop()
	if _, err := conn.Write(encoded); err != nil {
		return ErrUnavailable
	}
	line, err := readLine(reader, 1<<20)
	if err != nil {
		return ErrUnavailable
	}
	return decodeReply(line, result)
}

func encodeRequest(op string, args any) ([]byte, error) {
	encoded, err := json.Marshal(struct {
		Op   string `json:"op"`
		Args any    `json:"args"`
	}{op, args})
	if err != nil || len(encoded)+1 > 4096 {
		return nil, ErrUnavailable
	}
	return append(encoded, '\n'), nil
}

func decodeReply(line []byte, result any) error {
	var reply struct {
		OK    bool            `json:"ok"`
		Value json.RawMessage `json:"value,omitempty"`
		Error string          `json:"error,omitempty"`
	}
	if attention.Decode(line, &reply) != nil || !reply.OK || reply.Error != "" || len(reply.Value) == 0 {
		return ErrUnavailable
	}
	if result != nil && strictjson.Decode(reply.Value, result) != nil {
		return ErrUnavailable
	}
	return nil
}

func readLine(reader *bufio.Reader, maximum int) ([]byte, error) {
	var result []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(result)+len(part) > maximum {
			return nil, ErrUnavailable
		}
		result = append(result, part...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		return result[:len(result)-1], nil
	}
}
