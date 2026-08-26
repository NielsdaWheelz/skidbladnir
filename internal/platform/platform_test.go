package platform

import (
	"runtime"
	"testing"
)

func TestCurrentPinsTheAcceptancePlatform(t *testing.T) {
	got := Current()
	switch runtime.GOOS {
	case "linux":
		if got != (Descriptor{Kind: KindLinux, TmuxPath: "/usr/bin/tmux", TmuxVersion: "tmux 3.4"}) {
			t.Fatalf("Linux descriptor = %+v", got)
		}
	case "darwin":
		if got != (Descriptor{Kind: KindDarwin, TmuxPath: "/opt/homebrew/bin/tmux", TmuxVersion: "tmux 3.7b"}) {
			t.Fatalf("Darwin descriptor = %+v", got)
		}
	}
}
