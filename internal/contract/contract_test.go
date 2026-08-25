package contract

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestFrozenInputsRejectAnyMutation(t *testing.T) {
	apiBytes := []byte(`{"formatVersion":1}`)
	catalogueBytes := []byte(`[{"key":"norse.ai"}]`)
	lock := contractInputLock{
		Version:               1,
		APISHA256:             sha256Hex(apiBytes),
		CatalogueSHA256:       sha256Hex(catalogueBytes),
		CatalogueKeySetSHA256: sha256Hex([]byte("norse.ai")),
		CatalogueEntries:      1,
	}
	entries := []catalogueEntry{{Key: "norse.ai"}}
	if err := validateContractInputLock(lock, apiBytes, catalogueBytes, entries); err != nil {
		t.Fatalf("reviewed inputs rejected: %v", err)
	}

	for name, changed := range map[string]func() error{
		"api bytes": func() error {
			return validateContractInputLock(lock, append(apiBytes, '\n'), catalogueBytes, entries)
		},
		"catalogue bytes": func() error {
			return validateContractInputLock(lock, apiBytes, append(catalogueBytes, '\n'), entries)
		},
		"catalogue key set": func() error {
			return validateContractInputLock(lock, apiBytes, catalogueBytes, []catalogueEntry{{Key: "norse.changed"}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := changed(); err == nil {
				t.Fatalf("frozen contract accepted mutated %s", name)
			}
		})
	}
}

func TestSchemaTreeDigestCoversEveryPathAndByte(t *testing.T) {
	files := []schemaFile{
		{Path: "v2/ThreadReadParams.json", SHA256: sha256Hex([]byte("read"))},
		{Path: "v2/ThreadStartParams.json", SHA256: sha256Hex([]byte("start"))},
	}
	baseline := schemaTreeDigest(files)
	reordered := []schemaFile{files[1], files[0]}
	if schemaTreeDigest(reordered) != baseline {
		t.Fatal("schema tree digest depends on directory walk order")
	}
	changedPath := append([]schemaFile(nil), files...)
	changedPath[0].Path = "v2/ThreadReadResponse.json"
	if schemaTreeDigest(changedPath) == baseline {
		t.Fatal("schema path mutation did not change tree digest")
	}
	changedBytes := append([]schemaFile(nil), files...)
	changedBytes[0].SHA256 = sha256Hex([]byte("changed"))
	if schemaTreeDigest(changedBytes) == baseline {
		t.Fatal("schema byte mutation did not change tree digest")
	}
}

func TestDvergatalEvidenceExactlyMatchesCatalogue(t *testing.T) {
	entry := catalogueEntry{
		Key:         "norse.ai",
		DisplayName: "Ái",
		Tradition:   "OldNorse",
		Source:      catalogueSource{Work: "Vǫluspá", Locus: "st. 11"},
	}
	valid := strings.Join([]string{
		"# Dvergatal curation and provenance",
		"## Acceptance for good content",
		"## Portrait production and rights basis",
		"## Dedupe and exclusion decisions",
		"ENTRY|key=norse.ai|displayName=Ái|tradition=OldNorse|work=Vǫluspá|locus=st. 11",
	}, "\n")
	if err := validateDvergatalEvidence([]byte(valid), []catalogueEntry{entry}); err != nil {
		t.Fatalf("matching evidence rejected: %v", err)
	}

	for name, evidence := range map[string]string{
		"mismatched field": strings.Replace(valid, "displayName=Ái", "displayName=Ai", 1),
		"duplicate row":    valid + "\n" + strings.Split(valid, "\n")[4],
		"malformed row":    strings.Replace(valid, "|locus=st. 11", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateDvergatalEvidence([]byte(evidence), []catalogueEntry{entry}); err == nil {
				t.Fatalf("curation evidence accepted %s", name)
			}
		})
	}
}

func TestGeneratedUnionsOwnTheirWireDiscriminators(t *testing.T) {
	variant, err := jsonFields([]fieldSpec{{Name: "sequence", Type: "Int64"}})
	if err != nil {
		t.Fatal(err)
	}
	spec := apiContract{
		Enums:   map[string][]string{"FactKind": {"AgentStarted"}},
		Records: map[string][]fieldSpec{},
		Unions: map[string]unionSpec{
			"Fact": {Discriminator: "kind", Variants: map[string]json.RawMessage{"AgentStarted": variant}},
		},
		Errors: map[string]int{},
	}
	goBytes, err := renderGo(spec, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	kotlin := string(renderKotlin(spec, strings.Repeat("a", 64)))
	goSource := string(goBytes)
	for _, required := range []string{"func DecodeFact", "FactKind() FactKind", `json:"kind"`} {
		if !strings.Contains(goSource, required) {
			t.Fatalf("generated Go union lacks %q", required)
		}
	}
	if strings.Contains(goSource, "type AgentStartedFact struct {\n\tKind FactKind") {
		t.Fatal("generated Go exposes a forgeable Fact discriminator field")
	}
	for _, required := range []string{`@JsonClassDiscriminator("kind")`, `@SerialName("AgentStarted")`} {
		if !strings.Contains(kotlin, required) {
			t.Fatalf("generated Kotlin union lacks %q", required)
		}
	}
	if strings.Contains(kotlin, "val kind: FactKind") {
		t.Fatal("generated Kotlin exposes a forgeable Fact discriminator field")
	}
}

func TestJustificationGrammarRejectsMissingOrEmptyReasons(t *testing.T) {
	valid := `@Suppress("UNCHECKED_CAST") // justify-override: library boundary exposes erased generic type.`
	if err := validateJustificationLines("example.kt", []byte(valid)); err != nil {
		t.Fatalf("justified override rejected: %v", err)
	}
	for name, content := range map[string]string{
		"missing token": `@Suppress("UNCHECKED_CAST")`,
		"empty reason":  `@Suppress("UNCHECKED_CAST") // justify-override:`,
		"unknown token": `val x = 1 // justify-magic: compiler says so`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateJustificationLines("example.kt", []byte(content)); err == nil {
				t.Fatalf("justification grammar accepted %s", name)
			}
		})
	}
}

func jsonFields(fields []fieldSpec) ([]byte, error) {
	return json.Marshal(fields)
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
