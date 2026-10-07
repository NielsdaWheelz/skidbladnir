package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 9 * time.Second
)

func ValidateListenAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("gateway listen address must be a numeric loopback address")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return errors.New("gateway listen port must be between 1 and 65535")
	}
	return nil
}

func ListenAndServe(ctx context.Context, address string, gateway *Gateway) (result error) {
	if err := ValidateListenAddress(address); err != nil {
		return err
	}
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
	stopRecovery := gateway.sessions.StartRecovery(ctx)
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
	gateway.log(logging.NewGatewayStarted())
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
	if err := gateway.CloseLiveTerminals(shutdownContext); err != nil {
		result = errors.Join(result, fmt.Errorf("close live terminals: %w", err))
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
