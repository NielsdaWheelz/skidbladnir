package sessions

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSpaceMetadataReaderAcceptsOnlyCanonicalEncoding(t *testing.T) {
	for _, text := range []string{"alpha", "é", strings.Repeat("😀", 64)} {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(text))
		if decodeSpaceMetadata(encoded).String() != text {
			t.Fatal("canonical named metadata was omitted or changed")
		}
	}
	for index, encoded := range []string{"", "YQ==", "YR", "YQ\n", "YQ\r\n", "!", strings.Repeat("a", 343), base64.RawURLEncoding.EncodeToString([]byte("e\u0301")), base64.RawURLEncoding.EncodeToString([]byte(" a")), base64.RawURLEncoding.EncodeToString([]byte{0xff})} {
		if !decodeSpaceMetadata(encoded).IsUnassigned() {
			t.Fatalf("malformed optional metadata was accepted: case=%d", index)
		}
	}
}
