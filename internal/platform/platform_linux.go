//go:build linux

package platform

func current() Descriptor {
	return Descriptor{Kind: KindLinux, TmuxPath: "/usr/bin/tmux", TmuxVersion: "tmux 3.4"}
}
