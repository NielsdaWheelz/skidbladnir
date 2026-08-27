//go:build linux

package platform

func current() Descriptor {
	return Descriptor{Kind: KindLinux}
}
