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

func ListenAndServe(ctx context.Context, address string, gateway *Gateway) error {
	if err := ValidateListenAddress(address); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on gateway loopback: %w", err)
	}
	server := &http.Server{
		Handler:           gateway,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    int(MaximumBodyBytes),
	}
	serveResult := make(chan error, 1)
	gateway.log(logging.NewGatewayStarted())
	go func() {
		serveResult <- server.Serve(listener)
	}()
	select {
	case err := <-serveResult:
		closeContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if closeErr := gateway.CloseLiveTerminals(closeContext); closeErr != nil {
			return fmt.Errorf("close live terminals after HTTP server exit: %w", closeErr)
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve gateway HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := gateway.CloseLiveTerminals(shutdownContext); err != nil {
			return fmt.Errorf("close live terminals: %w", err)
		}
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down gateway HTTP: %w", err)
		}
		if err := <-serveResult; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve gateway HTTP: %w", err)
		}
		return nil
	}
}
