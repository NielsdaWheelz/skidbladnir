package sessions

import (
	"bytes"
	"testing"
)

func TestPhoneShadowNameIsClosedAndDeterministic(t *testing.T) {
	name, err := shadowNameFromEntropy(bytes.NewReader([]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}))
	if err != nil {
		t.Fatalf("mint phone shadow name: %v", err)
	}
	const want = "skid-phone-00112233445566778899aabbccddeeff"
	if name != want {
		t.Fatalf("phone shadow name = %q, want %q", name, want)
	}
	if _, err := shadowNameFromEntropy(bytes.NewReader(make([]byte, 15))); err == nil {
		t.Fatal("short entropy unexpectedly minted a phone shadow name")
	}
}
