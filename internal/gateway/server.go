package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 9 * time.Second
)

func ListenAndServe(ctx context.Context, address string, gateway *Gateway) (result error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on gateway loopback: %w", err)
	}
	defer func() {
		// justify-ignore-error: HTTP shutdown already closes the owned listener;
		// net.ErrClosed confirms that release, while other close failures remain.
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			result = errors.Join(result, fmt.Errorf("close gateway listener: %w", err))
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopRecovery := gateway.sessions.StartRecovery(ctx, func(change sessions.RecoveryChange) {
		attrs := []any{"skidbladnir.recovery.at", change.At.UTC().Format(time.RFC3339Nano), "skidbladnir.recovery.state", string(change.Recovery.State)}
		if savedAt := change.Recovery.SavedAt; savedAt != nil {
			attrs = append(attrs, "skidbladnir.recovery.saved_at", savedAt.UTC().Format(time.RFC3339Nano))
		}
		if failure := change.Recovery.Failure; failure != nil {
			attrs = append(attrs, "skidbladnir.recovery.failure_at", failure.At.UTC().Format(time.RFC3339Nano), "skidbladnir.recovery.phase", string(failure.Phase), "skidbladnir.recovery.cause", string(failure.Cause))
		}
		if change.Operation != "" {
			attrs = append(attrs, "skidbladnir.recovery.operation", change.Operation)
		}
		if change.Errno != 0 {
			attrs = append(attrs, "skidbladnir.recovery.errno", uint64(change.Errno)) // slog writes an Errno as its text
		}
		gateway.logger.Info("Workspace.RecoveryChanged", attrs...)
	})
	defer func() {
		if err := stopRecovery(); err != nil {
			result = errors.Join(result, fmt.Errorf("close workspace recovery: %w", err))
		}
	}()
	server := &http.Server{
		Handler:           gateway,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    int(MaximumBodyBytes),
	}
	serveResult := make(chan error, 1)
	gateway.logger.Info("Gateway.Started")
	go func() {
		serveResult <- server.Serve(listener)
	}()
	var serveErr error
	serveFinished := false
	select {
	case serveErr = <-serveResult:
		serveFinished = true
	case <-ctx.Done():
	}
	cancel()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := gateway.closeLiveConnections(shutdownContext); err != nil {
		result = errors.Join(result, fmt.Errorf("close live connections: %w", err))
	}
	if err := server.Shutdown(shutdownContext); err != nil {
		result = errors.Join(result, fmt.Errorf("shut down gateway HTTP: %w", err))
		if err := server.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close gateway HTTP: %w", err))
		}
	}
	if !serveFinished {
		serveErr = <-serveResult
	}
	// justify-ignore-error: ErrServerClosed is HTTP's confirmed normal shutdown.
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		result = errors.Join(result, fmt.Errorf("serve gateway HTTP: %w", serveErr))
	}
	return result
}
