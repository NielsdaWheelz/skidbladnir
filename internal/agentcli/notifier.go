package agentcli

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
)

func runNotifications(ctx context.Context, args []string, _, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "setup" {
		fmt.Fprintln(stderr, "usage: skid notifications setup")
		return 2
	}
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
