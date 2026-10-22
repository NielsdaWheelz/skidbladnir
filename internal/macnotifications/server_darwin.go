//go:build darwin && cgo

package macnotifications

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

// listenSocket admits one app per state directory: the flock is held for the
// app's lifetime and the socket is replaced only by its holder.
func listenSocket() (*net.UnixListener, *os.File, error) {
	path, err := socketPath()
	if err != nil || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return nil, nil, attention.ErrUnavailable
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, attention.ErrUnavailable
	}
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		_ = lock.Close()
		return nil, nil, errors.New("notification app is already running")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = lock.Close()
		return nil, nil, attention.ErrUnavailable
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		_ = lock.Close()
		return nil, nil, attention.ErrUnavailable
	}
	if os.Chmod(path, 0600) != nil {
		_ = listener.Close()
		_ = lock.Close()
		return nil, nil, attention.ErrUnavailable
	}
	return listener, lock, nil
}

// serve answers one `setup` line per connection with `ok` once the setup
// window is shown, or `unavailable`. It stops accepting when ctx ends.
func serve(ctx context.Context, listener *net.UnixListener) {
	var connections sync.WaitGroup
	defer connections.Wait()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		connections.Go(func() {
			defer connection.Close()
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			if connection.SetDeadline(deadline) != nil {
				return
			}
			reply := "unavailable\n"
			line, err := bufio.NewReader(io.LimitReader(connection, 64)).ReadString('\n')
			if err == nil && line == "setup\n" && bridge.call(ctx, "setup", struct{}{}, nil) == nil {
				reply = "ok\n"
			}
			_, _ = io.WriteString(connection, reply)
		})
	}
}
