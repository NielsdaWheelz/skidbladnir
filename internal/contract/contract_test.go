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

func TestCodexLockIdentityRequiresPinnedHookSchemas(t *testing.T) {
	valid := codexLock{
		Version:              "0.149.1",
		Package:              "@openai/codex@0.149.1",
		PlatformPackage:      "@openai/codex@0.149.1-linux-x64",
		BinaryPath:           "/opt/codex/0.149.1/codex",
		BinarySHA256:         strings.Repeat("c", 64),
		SchemaDirectory:      "schemas/codex/0.149.1",
		SchemaFiles:          298,
		SchemaTreeSHA256:     strings.Repeat("a", 64),
		SchemaBundle:         "schemas/codex/0.149.1/codex_app_server_protocol.v2.schemas.json",
		SchemaSHA256:         strings.Repeat("d", 64),
		SchemaCommand:        []string{"/opt/codex/0.149.1/codex", "app-server", "generate-json-schema", "--out", "schemas/codex/0.149.1"},
		SourceRepository:     "https://github.com/openai/codex.git",
		SourceCommit:         "ff29a44391deccde0aba0f8390337d7f3c319ea4",
		SourceTag:            "rust-v0.149.1",
		HookSchemaDirectory:  "schemas/codex/0.149.1/hooks",
		HookSchemaFiles:      7,
		HookSchemaTreeSHA256: strings.Repeat("b", 64),
		HookSchemaCommand:    []string{"./scripts/vendor-codex-hook-schemas"},
	}
	if err := validateCodexLockIdentity(valid); err != nil {
		t.Fatalf("complete pin identity rejected: %v", err)
	}

	for name, mutate := range map[string]func(*codexLock){
		"source commit": func(lock *codexLock) { lock.SourceCommit = "" },
		"source tag":    func(lock *codexLock) { lock.SourceTag = "rust-v0.149.0" },
		"hook directory": func(lock *codexLock) {
			lock.HookSchemaDirectory = "schemas/codex/0.149.1/other"
		},
		"hook count":   func(lock *codexLock) { lock.HookSchemaFiles = 6 },
		"hook digest":  func(lock *codexLock) { lock.HookSchemaTreeSHA256 = "" },
		"hook command": func(lock *codexLock) { lock.HookSchemaCommand = nil },
	} {
		t.Run(name, func(t *testing.T) {
			changed := valid
			mutate(&changed)
			if err := validateCodexLockIdentity(changed); err == nil {
				t.Fatalf("pin identity accepted invalid %s", name)
			}
		})
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

func TestAcceptanceGateInventoriesAreExact(t *testing.T) {
	tests := map[string]string{
		"core":    "static unit integration component system platform live e2e",
		"upgrade": "static unit integration component system upgrade-live",
	}
	for target, want := range tests {
		gates, err := acceptanceGates(target)
		if err != nil {
			t.Fatalf("%s gate inventory: %v", target, err)
		}
		if got := strings.Join(gates, " "); got != want {
			t.Fatalf("%s gates = %q, want %q", target, got, want)
		}
	}
	if _, err := acceptanceGates("unknown"); err == nil {
		t.Fatal("unknown acceptance target has a gate inventory")
	}
}

func TestAcceptanceGateResultRequiresExitAndRecordAgreement(t *testing.T) {
	valid := []struct {
		exit    int
		content string
	}{
		{exit: 0, content: `{"gate":"live","status":"PASS"}`},
		{exit: 1, content: `{"gate":"live","status":"FAIL","reason":"probe failed"}`},
		{exit: 2, content: `{"gate":"live","status":"NOT_RUN","reason":"profile unavailable"}`},
	}
	for _, test := range valid {
		if _, err := parseAcceptanceGateResult("live", test.exit, []byte(test.content)); err != nil {
			t.Fatalf("valid exit %d record rejected: %v", test.exit, err)
		}
	}
	for name, test := range map[string]struct {
		exit    int
		content string
	}{
		"wrong gate":      {exit: 0, content: `{"gate":"unit","status":"PASS"}`},
		"status mismatch": {exit: 2, content: `{"gate":"live","status":"FAIL","reason":"failed"}`},
		"missing reason":  {exit: 2, content: `{"gate":"live","status":"NOT_RUN"}`},
		"pass reason":     {exit: 0, content: `{"gate":"live","status":"PASS","reason":"unexpected"}`},
		"unknown field":   {exit: 0, content: `{"gate":"live","status":"PASS","raw":"secret"}`},
		"trailing value":  {exit: 0, content: `{"gate":"live","status":"PASS"} {}`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAcceptanceGateResult("live", test.exit, []byte(test.content)); err == nil {
				t.Fatalf("accepted contradictory gate result: %s", test.content)
			}
		})
	}
}

func TestProofResultsRequireClosedStatusShapesAndScannedEvidencePaths(t *testing.T) {
	proof := proofSpec{ID: "broker-appserver", Gate: "live", RequiredFor: []string{"core"}}
	evidence := "evidence/live/broker-appserver.json"
	reason := "live probe failed"
	for name, result := range map[string]proofResult{
		"pass without evidence": {ID: proof.ID, Gate: proof.Gate, Status: "PASS"},
		"fail without reason":   {ID: proof.ID, Gate: proof.Gate, Status: "FAIL"},
		"outside evidence tree": {ID: proof.ID, Gate: proof.Gate, Status: "PASS", Evidence: pointer("docs/architecture.md")},
		"noncanonical path":     {ID: proof.ID, Gate: proof.Gate, Status: "PASS", Evidence: pointer("evidence/live/../proof.json")},
		"unknown status":        {ID: proof.ID, Gate: proof.Gate, Status: "SKIPPED", Reason: &reason},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProofResult(proof, result); err == nil {
				t.Fatalf("accepted invalid proof result: %#v", result)
			}
		})
	}
	if err := validateProofResult(proof, proofResult{ID: proof.ID, Gate: proof.Gate, Status: "PASS", Evidence: &evidence}); err != nil {
		t.Fatalf("valid PASS proof rejected: %v", err)
	}
	if err := validateProofResult(proof, proofResult{ID: proof.ID, Gate: proof.Gate, Status: "FAIL", Reason: &reason}); err != nil {
		t.Fatalf("valid FAIL proof rejected: %v", err)
	}
}

func TestWorkflowRequiresImmutableActionsAndTheP0CommandInventory(t *testing.T) {
	valid := strings.Join([]string{
		"name: verify",
		"on:",
		"  pull_request:",
		"permissions:",
		"  contents: read",
		"steps:",
		"  - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
		"  - run: ./scripts/test static",
		"  - run: ./scripts/build all",
		"  - run: ./scripts/test unit",
	}, "\n")
	if err := validateWorkflow([]byte(valid)); err != nil {
		t.Fatalf("reviewed workflow rejected: %v", err)
	}
	for name, content := range map[string]string{
		"floating action":      strings.Replace(valid, "@3d3c42e5aac5ba805825da76410c181273ba90b1", "@v7", 1),
		"missing command":      strings.Replace(valid, "  - run: ./scripts/test unit", "", 1),
		"privileged trigger":   strings.Replace(valid, "pull_request:", "pull_request_target:", 1),
		"ignored failure":      valid + "\n  - run: ./scripts/test static || true",
		"elevated permissions": strings.Replace(valid, "contents: read", "contents: write", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateWorkflow([]byte(content)); err == nil {
				t.Fatalf("workflow accepted %s", name)
			}
		})
	}
}

func TestProductLanguageRejectsReservedNamesAndDeferredConcepts(t *testing.T) {
	if err := validateProductText("AgentCard StartAgent Skíðblaðnir The Forge Agents"); err != nil {
		t.Fatalf("Core product language rejected: %v", err)
	}
	for _, content := range []string{
		"WorktreePicker",
		"repository_id",
		"WorkspaceMode",
		"WorkspaceModes",
		"App Server",
		"AppServers",
		"rawTargets",
		"Dvergatal",
		"Ariadne",
	} {
		if err := validateProductText(content); err == nil {
			t.Fatalf("reserved product language accepted %q", content)
		}
	}
}

func TestAdHocLoggingIsRejectedOutsideTheClosedLogger(t *testing.T) {
	for path, content := range map[string]string{
		"internal/service.go": `package service
import "fmt"
func run() { fmt.Println("state") }`,
		"internal/dot.go": `package service
import . "fmt"
func run() { Println("state") }`,
		"android/Main.kt":     `fun run() { android.util.Log.i("state", "working") }`,
		"android/terminal.js": `globalThis.console["log"]("working")`,
	} {
		if err := validateAdHocLogging(path, []byte(content)); err == nil {
			t.Fatalf("ad-hoc logger accepted in %s", path)
		}
	}
	if err := validateAdHocLogging("cmd/tool/main.go", []byte(`package main
import (
    "fmt"
    "os"
)
func main() { fmt.Fprintln(os.Stderr, "typed boundary error") }`)); err != nil {
		t.Fatalf("CLI stderr boundary rejected: %v", err)
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

func pointer(value string) *string { return &value }
