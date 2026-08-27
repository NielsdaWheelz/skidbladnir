//go:build darwin

package platform

func current() Descriptor {
	return Descriptor{Kind: KindDarwin}
}
