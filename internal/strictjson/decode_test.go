package strictjson

import (
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousDocuments(t *testing.T) {
	type document struct {
		Name string `json:"name"`
	}
	tests := []string{
		`{"name":"one","\u006eame":"two"}`,
		`{"Name":"one"}`,
		`{"name":"one","Name":"two"}`,
		`{"name":"one"}{"name":"two"}`,
		`{"name":"one","extra":true}`,
		`{"name":"\ud800"}`,
		`{"name":"\udc00"}`,
		`{"name":"\ud800x\udc00"}`,
		string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}),
		strings.Repeat("[", maximumNestingDepth+2) + `null` + strings.Repeat("]", maximumNestingDepth+2),
	}
	for _, encoded := range tests {
		var target document
		if err := Decode([]byte(encoded), &target); err == nil {
			t.Fatalf("Decode(%q) accepted an ambiguous document", encoded)
		}
	}
}

func TestDecodeAcceptsOneStrictDocument(t *testing.T) {
	var target struct {
		Name string `json:"name"`
	}
	if err := Decode([]byte(`{"name":"Skíðblaðnir"}`), &target); err != nil {
		t.Fatal(err)
	}
	if target.Name != "Skíðblaðnir" {
		t.Fatalf("name = %q", target.Name)
	}
}

func TestDecodePreservesValidSurrogatePairsAndLiteralReplacementCharacters(t *testing.T) {
	for _, encoded := range []string{
		`{"name":"\ud83d\ude00"}`,
		`{"name":"�"}`,
	} {
		var target struct {
			Name string `json:"name"`
		}
		if err := Decode([]byte(encoded), &target); err != nil {
			t.Fatalf("Decode(%q): %v", encoded, err)
		}
	}
}
