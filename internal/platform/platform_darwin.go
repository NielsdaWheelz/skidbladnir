//go:build darwin

package platform

func current() Descriptor {
	return Descriptor{Kind: KindDarwin, TmuxPath: "/opt/homebrew/bin/tmux", TmuxVersion: "tmux 3.7b"}
}
