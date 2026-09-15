package space

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestLabelsCloseCanonicalTextAndHumanDrafts(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		valid bool
	}{
		{"unassigned", "", true}, {"ordinary", "alpha  beta", true},
		{"selector text", "unassigned", true}, {"canonical", "é", true},
		{"decomposed", "e\u0301", false}, {"maximum ascii", strings.Repeat("a", 64), true},
		{"excess ascii", strings.Repeat("a", 65), false},
		{"maximum supplementary", strings.Repeat("😀", 64), true},
		{"excess supplementary", strings.Repeat("😀", 65), false},
		{"invalid utf8", "\xff", false}, {"leading space", " a", false}, {"trailing space", "a ", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			label, err := Parse(test.text)
			if (err == nil) != test.valid {
				t.Fatal("canonical acceptance differs from contract")
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatal("validation has an unknown error")
			}
			if err == nil && label.String() != test.text {
				t.Fatal("canonical reader changed accepted text")
			}
		})
	}
	for _, value := range []rune{0, 0x1f, 0x7f, 0x9f, 0x061c, 0x200e, 0x200f, 0x2028, 0x2029, 0x202a, 0x202e, 0x2066, 0x2069, 0xa0, 0x1680, 0x2000, 0x200a, 0x202f, 0x205f, 0x3000} {
		if _, err := Parse("a" + string(value) + "b"); err == nil {
			t.Fatalf("display-unsafe scalar accepted: u+%04x", value)
		}
	}
	draft, err := ParseDraft("e\u0301")
	canonical, _ := Parse("é")
	if err != nil || draft != canonical {
		t.Fatal("human draft did not normalize to canonical membership")
	}
	if _, err := ParseDraft("\xff"); err == nil {
		t.Fatal("invalid utf8 draft was normalized")
	}
}

func TestLabelOrderAndFilterUseExactMembership(t *testing.T) {
	texts := []string{"", "é", "beta", "alpha", "Alpha", "all spaces", "𐀀", "\ue000"}
	labels := make([]Label, len(texts))
	for index, value := range texts {
		labels[index], _ = Parse(value)
	}
	slices.SortFunc(labels, Compare)
	want := []string{"all spaces", "Alpha", "alpha", "beta", "é", "\ue000", "𐀀", ""}
	for index, label := range labels {
		if label.String() != want[index] {
			t.Fatalf("label order mismatch at index %d", index)
		}
		if !(Filter{}).Matches(label) {
			t.Fatal("all filter omitted membership")
		}
		if UnassignedFilter().Matches(label) != label.IsUnassigned() {
			t.Fatal("unassigned filter confused named membership")
		}
	}
	named, _ := Parse("alpha")
	filter, err := NamedFilter(named)
	if err != nil || filter.Kind() != FilterNamed || filter.Label() != named || !filter.Matches(named) {
		t.Fatal("named filter lost membership")
	}
	other, _ := Parse("Alpha")
	if filter.Matches(other) || filter.Matches(Label{}) {
		t.Fatal("named filter matched distinct membership")
	}
	if _, err := NamedFilter(Label{}); err == nil {
		t.Fatal("named filter accepted unassigned")
	}
}

func TestLabelsUsePlainNFCWithoutStreamSafeInsertion(t *testing.T) {
	if norm.Version != "15.0.0" {
		t.Fatal("space normalization requires Unicode 15 tables in both clients")
	}
	for _, test := range []struct{ name, draft, canonical string }{
		{"long canonical run", "x" + strings.Repeat("\u035c", 31), "x" + strings.Repeat("\u035c", 31)},
		{"composition beyond stream boundary", "x" + strings.Repeat("\u035c", 31) + "\u0307", "\u1e8b" + strings.Repeat("\u035c", 31)},
		{"explicit joiner stays a boundary", "x" + strings.Repeat("\u035c", 31) + "\u034f\u0307", "x" + strings.Repeat("\u035c", 31) + "\u034f\u0307"},
		{"hangul composition", "\u1100\u1161\u11a8", "\uac01"},
		{"excluded composition", "\u0958", "\u0915\u093c"},
		{"blocked composition", "A\u0305\u0301", "A\u0305\u0301"},
		{"leading nonstarters", strings.Repeat("\u035c", 31) + "\u0307", "\u0307" + strings.Repeat("\u035c", 31)},
		{"equal classes stay ordered", "q\u0305\u0301", "q\u0305\u0301"},
	} {
		t.Run(test.name, func(t *testing.T) {
			label, err := Parse(test.canonical)
			if err != nil || label.String() != test.canonical {
				t.Fatal("plain canonical label was rejected or changed")
			}
			draft, err := ParseDraft(test.draft)
			if err != nil || draft.String() != test.canonical {
				t.Fatal("human normalization changed plain NFC semantics")
			}
			if test.draft != test.canonical {
				if _, err := Parse(test.draft); err == nil {
					t.Fatal("canonical wire reader normalized input")
				}
			}
		})
	}
}
