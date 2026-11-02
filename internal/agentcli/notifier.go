package agentcli

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
)

func runNotifications(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	help, _ := Help("notifications")
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		if _, err := io.WriteString(stdout, help); err != nil {
			return 1
		}
		return 0
	}
	if len(args) != 1 || args[0] != "setup" {
		io.WriteString(stderr, help)
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
