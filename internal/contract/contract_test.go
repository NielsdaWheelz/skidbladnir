package contract

import (
	"fmt"
	"testing"
)

func TestCatalogueRejectsInvalidContentAtItsBoundary(t *testing.T) {
	valid := catalogueFixture()
	if err := validateCatalogue(valid); err != nil {
		t.Fatalf("valid catalogue rejected: %v", err)
	}

	tests := []struct {
		name   string
		change func([]catalogueEntry)
	}{
		{
			name: "duplicate key",
			change: func(entries []catalogueEntry) {
				entries[1].Key = entries[0].Key
			},
		},
		{
			name: "duplicate display name",
			change: func(entries []catalogueEntry) {
				entries[1].DisplayName = entries[0].DisplayName
			},
		},
		{
			name: "forbidden tradition",
			change: func(entries []catalogueEntry) {
				entries[0].Tradition = "ModernMedia"
			},
		},
		{
			name: "non canonical unicode",
			change: func(entries []catalogueEntry) {
				entries[0].DisplayName = "A\u0301i"
			},
		},
		{
			name: "empty citation locus",
			change: func(entries []catalogueEntry) {
				entries[0].Source.Locus = ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := catalogueFixture()
			test.change(entries)
			if err := validateCatalogue(entries); err == nil {
				t.Fatalf("catalogue accepted %s", test.name)
			}
		})
	}
}

func TestStrictDecoderRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	type value struct {
		Known string `json:"known"`
	}
	for _, input := range []string{
		`{"known":"yes","unknown":"no"}`,
		`{"known":"yes"} {"known":"twice"}`,
	} {
		var decoded value
		if err := decodeStrict([]byte(input), &decoded); err == nil {
			t.Fatalf("strict decoder accepted %s", input)
		}
	}
}

func TestContractDigestUsesSortedCatalogueKeySet(t *testing.T) {
	left := catalogueFixture()
	right := append([]catalogueEntry(nil), left...)
	right[0], right[1] = right[1], right[0]
	if contractDigest([]byte("api"), left) != contractDigest([]byte("api"), right) {
		t.Fatal("catalogue ordering changed the key-set contract digest")
	}
	right[0].Key = "norse.changed"
	if contractDigest([]byte("api"), left) == contractDigest([]byte("api"), right) {
		t.Fatal("catalogue key change did not change the contract digest")
	}
}

func catalogueFixture() []catalogueEntry {
	entries := make([]catalogueEntry, minimumDwarfs)
	for index := range entries {
		entries[index] = catalogueEntry{
			Key:         fmt.Sprintf("norse.dwarf-%03d", index),
			DisplayName: fmt.Sprintf("Dwarf %03d", index),
			Tradition:   "OldNorse",
			Source:      catalogueSource{Work: "Vǫluspá", Locus: "st. 10"},
		}
	}
	return entries
}
