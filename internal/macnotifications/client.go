// Package macnotifications is the mac notification app: it reconciles the
// hub's notification state against the notification center, and its socket
// opens the app's setup window for `skid notifications setup`.
package macnotifications

import (
	"bufio"
	"context"
	"io"
	"net"
	"path/filepath"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

func socketPath() (string, error) {
	directory, err := attention.StateDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "notifications.sock"), nil
}

// Setup opens the running app's notification setup window.
func Setup(ctx context.Context) error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return attention.ErrUnavailable
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return attention.ErrUnavailable
	}
	if _, err := io.WriteString(conn, "setup\n"); err != nil {
		return attention.ErrUnavailable
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 64)).ReadString('\n')
	if err != nil || reply != "ok\n" {
		return attention.ErrUnavailable
	}
	return nil
}
