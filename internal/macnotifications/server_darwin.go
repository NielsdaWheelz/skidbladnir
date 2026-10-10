//go:build darwin && cgo

package macnotifications

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

type presentedArgs struct {
	VisitID string               `json:"visitId"`
	Token   attention.ReadyToken `json:"token"`
}

func listenSocket() (*net.UnixListener, *os.File, error) {
	path, err := SocketPath()
	if err != nil || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return nil, nil, ErrUnavailable
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		_ = lock.Close() // justify-ignore-error: failed admission owns no store or socket.
		return nil, nil, errors.New("notification app is already running")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = lock.Close() // justify-ignore-error: the socket removal failure is primary.
		return nil, nil, ErrUnavailable
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		_ = lock.Close() // justify-ignore-error: the failed bind owns no listener.
		return nil, nil, ErrUnavailable
	}
	if os.Chmod(path, 0600) != nil {
		_ = listener.Close() // justify-ignore-error: failed admission does not publish an owner.
		_ = lock.Close()     // justify-ignore-error: releasing failed owner admission cannot repair chmod.
		return nil, nil, ErrUnavailable
	}
	return listener, lock, nil
}

func (owner *owner) serve(listener *net.UnixListener) {
	var next atomic.Uint64
	var connections sync.WaitGroup
	defer connections.Wait()
	stop := context.AfterFunc(owner.ctx, func() {
		_ = listener.Close() // justify-ignore-error: cancellation owns listener teardown.
	})
	defer stop()
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		id := next.Add(1)
		connections.Go(func() { owner.serveConnection(connection, id) })
	}
}

func (owner *owner) serveConnection(connection *net.UnixConn, id uint64) {
	defer connection.Close()                                                  // justify-ignore-error: disconnect retires all connection-owned visits.
	defer func() { _, _ = owner.request(owner.ctx, "disconnect", id, nil) }() // justify-ignore-error: process shutdown already retires ephemeral visits.
	stop := context.AfterFunc(owner.ctx, func() {
		_ = connection.Close() // justify-ignore-error: app cancellation closes every live attachment rpc.
	})
	defer stop()
	reader := bufio.NewReader(connection)
	for {
		line, err := readLine(reader, 4096)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(owner.ctx, operationTimeout)
		var request struct {
			Op   string          `json:"op"`
			Args json.RawMessage `json:"args"`
		}
		var value any
		valid := attention.Decode(line, &request) == nil && len(request.Args) != 0
		if valid {
			switch request.Op {
			case "read":
				var args struct{}
				valid = attention.Decode(request.Args, &args) == nil
			case "setup":
				var args *struct{}
				valid = attention.Decode(request.Args, &args) == nil && args != nil
			case "register":
				var wire struct {
					Key         attention.Key         `json:"key"`
					Foreground  *attention.Foreground `json:"foreground,omitempty"`
					ReadyToken  *attention.ReadyToken `json:"readyToken,omitempty"`
					Presented   *bool                 `json:"presented"`
					LaunchNonce string                `json:"launchNonce,omitempty"`
					Surface     *SurfaceAssociation   `json:"surface,omitempty"`
				}
				valid = attention.Decode(request.Args, &wire) == nil && wire.Presented != nil
				args := RegisterArgs{Key: wire.Key, Foreground: wire.Foreground, ReadyToken: wire.ReadyToken, LaunchNonce: wire.LaunchNonce, Surface: wire.Surface}
				if valid {
					args.Presented = *wire.Presented
				}
				valid = valid && args.Key.Valid() && (args.Foreground == nil || args.Foreground.Valid()) && (args.ReadyToken == nil || validToken(*args.ReadyToken)) && (args.LaunchNonce == "" || attention.EpochValid(args.LaunchNonce)) && (args.Surface == nil || validSurface(*args.Surface)) && (args.Surface == nil || args.LaunchNonce == "" || args.Surface.Nonce == args.LaunchNonce)
				value = args
			case "presented":
				var wire struct {
					VisitID string                `json:"visitId"`
					Token   *attention.ReadyToken `json:"token"`
				}
				valid = attention.Decode(request.Args, &wire) == nil && wire.Token != nil && attention.EpochValid(wire.VisitID)
				args := presentedArgs{VisitID: wire.VisitID}
				if valid {
					args.Token = *wire.Token
				}
				valid = valid && validToken(args.Token)
				value = args
			case "end":
				var args struct {
					VisitID string `json:"visitId"`
				}
				valid = attention.Decode(request.Args, &args) == nil && attention.EpochValid(args.VisitID)
				value = args.VisitID
			default:
				valid = false
			}
		}
		var result any
		if valid {
			if request.Op == "setup" {
				err = bridge.call(ctx, "setup", struct{}{}, nil)
				result = struct{}{}
			} else {
				result, err = owner.request(ctx, request.Op, id, value)
			}
		} else {
			err = errors.New("invalid")
		}
		cancel()
		reply := map[string]any{"ok": true, "value": result}
		if err != nil {
			code := "unavailable"
			if !valid {
				code = "invalid"
			}
			reply = map[string]any{"ok": false, "error": code}
		}
		encoded, encodeErr := json.Marshal(reply)
		if encodeErr != nil || len(encoded)+1 > attention.MaximumSnapshotBytes || connection.SetWriteDeadline(time.Now().Add(operationTimeout)) != nil {
			return
		}
		if _, err := connection.Write(append(encoded, '\n')); err != nil {
			return
		}
	}
}

func validToken(token attention.ReadyToken) bool {
	return token.Generation >= 0 && token.Generation < math.MaxInt64 && (attention.EpochValid(token.Epoch) || token.Epoch == "" && token.Generation == 0)
}
