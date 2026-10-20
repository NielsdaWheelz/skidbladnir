package agentcli

import (
	"context"
	"fmt"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
	"io"
	"runtime"
)

func runNotifications(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "setup" && args[0] != "reset" {
		fmt.Fprintln(stderr, "usage: skid notifications {setup|reset}")
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
		_, err = store.Reset(ctx)
	}
	if err != nil {
		fmt.Fprintln(stderr, "notification memory reset failed")
		return 1
	}
	fmt.Fprintln(stdout, "notification memory reset")
	return 0
}
