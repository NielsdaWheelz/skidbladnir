package agentcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/gateway"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
	"github.com/NielsdaWheelz/skidbladnir/internal/notifier"
)

type notificationServiceConfig struct {
	ClientConfig                string `json:"clientConfig"`
	BearerFile                  string `json:"bearerFile"`
	MachineHandleFile           string `json:"machineHandleFile"`
	Listen                      string `json:"listen"`
	NtfyPublisherCredentialFile string `json:"ntfyPublisherCredentialFile"`
}

func runNotifications(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "observer" {
		return RunNotificationObserver(ctx, args[1:], stderr)
	}
	if len(args) != 1 || args[0] != "setup" && args[0] != "reset" {
		fmt.Fprintln(stderr, "usage: skid notifications {observer --config FILE|setup|reset}")
		return 2
	}
	if args[0] == "setup" {
		if runtime.GOOS != "darwin" {
			fmt.Fprintln(stderr, "notification setup is available only on macos")
			return 2
		}
		if macnotifications.Setup(ctx) != nil {
			fmt.Fprintln(stderr, "notification setup unavailable")
			return 1
		}
		return 0
	}
	if runtime.GOOS == "darwin" {
		fmt.Fprintln(stderr, "reset notification memory through skid notifications setup")
		return 2
	}
	store, err := fleetclient.DefaultNotificationStore()
	if err == nil {
		_, err = store.Reset()
	}
	if err != nil {
		fmt.Fprintln(stderr, "notification memory reset failed")
		return 1
	}
	fmt.Fprintln(stdout, "notification memory reset")
	return 0
}

// RunNotificationObserver is the explicit background service entrypoint.
func RunNotificationObserver(ctx context.Context, args []string, stderr io.Writer) (result int) {
	if len(args) != 2 || args[0] != "--config" || !filepath.IsAbs(args[1]) {
		fmt.Fprintln(stderr, "usage: skid notifications observer --config FILE")
		return 2
	}
	config, err := loadNotificationServiceConfig(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "notification service configuration unavailable")
		return 1
	}
	client, err := fleetclient.Open(config.ClientConfig)
	if err != nil {
		fmt.Fprintln(stderr, "notification client configuration unavailable")
		return 1
	}
	handle, err := machine.Load(config.MachineHandleFile)
	if err != nil {
		fmt.Fprintln(stderr, "notification machine identity unavailable")
		return 1
	}
	transport, configured := client.NotificationConfig()
	if !configured || transport.ObserverMachine != handle.String() || len(client.Machines()) != 3 {
		fmt.Fprintln(stderr, "notification observer is not the configured fleet host")
		return 1
	}
	publisher, err := notifier.NewPublisher(transport.NtfyOrigin, config.NtfyPublisherCredentialFile)
	if err != nil {
		fmt.Fprintln(stderr, "notification publisher credential unavailable")
		return 1
	}
	stateDirectory, err := attention.StateDirectory()
	if err != nil {
		fmt.Fprintln(stderr, "notification state unavailable")
		return 1
	}
	observer, err := notifier.New(notifier.Config{Client: client, Machine: handle, Bearer: auth.FileVerifier{Path: config.BearerFile}, StatePath: filepath.Join(stateDirectory, "notification-observer-v1.json"), Publisher: publisher})
	if err != nil {
		fmt.Fprintln(stderr, "notification observer state unavailable")
		return 1
	}
	defer func() {
		if observer.Close() != nil {
			fmt.Fprintln(stderr, "notification observer state unavailable")
			result = 1
		}
	}()
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		fmt.Fprintln(stderr, "notification observer listener unavailable")
		return 1
	}
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{Handler: observer, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 16 * 1024}
	done := make(chan error, 2)
	go func() { done <- observer.Run(serviceCtx) }()
	go func() { done <- server.Serve(listener) }()
	completed := 0
	select {
	case err = <-done:
		completed = 1
	case <-ctx.Done():
	}
	cancel()
	closeErr := server.Close()
	for completed < 2 {
		next := <-done
		completed++
		if err == nil && next != nil && !errors.Is(next, http.ErrServerClosed) {
			err = next
		}
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) || closeErr != nil {
		fmt.Fprintln(stderr, "notification observer unavailable")
		return 1
	}
	return 0
}
func loadNotificationServiceConfig(path string) (notificationServiceConfig, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return notificationServiceConfig{}, attention.ErrUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return notificationServiceConfig{}, attention.ErrUnavailable
	}
	data, readErr := io.ReadAll(io.LimitReader(file, fleetclient.MaximumInputBytes+1))
	closeErr := file.Close()
	var config notificationServiceConfig
	if readErr != nil || closeErr != nil || len(data) > fleetclient.MaximumInputBytes || attention.Decode(data, &config) != nil {
		return notificationServiceConfig{}, attention.ErrUnavailable
	}
	for _, file := range []string{config.ClientConfig, config.BearerFile, config.MachineHandleFile, config.NtfyPublisherCredentialFile} {
		if !filepath.IsAbs(file) {
			return notificationServiceConfig{}, attention.ErrUnavailable
		}
	}
	if gateway.ValidateListenAddress(config.Listen) != nil {
		return notificationServiceConfig{}, attention.ErrUnavailable
	}
	return config, nil
}
