package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	apiPath        = "api/skidbladnir.v1.json"
	cataloguePath  = "catalog/characters.json"
	codexLockPath  = "codex.lock"
	goOutputPath   = "generated/api/go/skidbladnir.go"
	kotlinPath     = "generated/api/kotlin/SkidbladnirApi.kt"
	proofsPath     = "evidence/proof-ledger.json"
	dvergatalPath  = "evidence/sources/dvergatal.md"
	digestDomain   = "Skidbladnir.Contract.V1"
	minimumDwarfs  = 100
	maximumRecents = 8
)

var (
	characterKeyPattern = regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`)
	fieldTypePattern    = regexp.MustCompile(`^(\?|\[\])?([A-Za-z][A-Za-z0-9]*)$`)
)

type apiContract struct {
	FormatVersion  int                    `json:"formatVersion"`
	Namespace      string                 `json:"namespace"`
	ContractHeader string                 `json:"contractHeader"`
	Bounds         map[string]int64       `json:"bounds"`
	Scalars        map[string]scalarSpec  `json:"scalars"`
	Enums          map[string][]string    `json:"enums"`
	Records        map[string][]fieldSpec `json:"records"`
	Unions         map[string]unionSpec   `json:"unions"`
	Errors         map[string]int         `json:"errors"`
	Routes         []routeSpec            `json:"routes"`
	LocalCommands  []string               `json:"localCommands"`
	Proofs         []proofSpec            `json:"proofs"`
}

type scalarSpec struct {
	Base       string `json:"base"`
	Pattern    string `json:"pattern,omitempty"`
	Format     string `json:"format,omitempty"`
	MaxScalars int    `json:"maxScalars,omitempty"`
	MaxBytes   int    `json:"maxBytes,omitempty"`
	Sensitive  bool   `json:"sensitive,omitempty"`
}

type fieldSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type unionSpec struct {
	Discriminator string                     `json:"discriminator"`
	Variants      map[string]json.RawMessage `json:"variants"`
}

type routeSpec struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Auth     string `json:"auth"`
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
	Stream   string `json:"stream,omitempty"`
	Upgrade  string `json:"upgrade,omitempty"`
	Status   int    `json:"status"`
}

type proofSpec struct {
	ID          string   `json:"id"`
	Gate        string   `json:"gate"`
	RequiredFor []string `json:"requiredFor"`
}

type catalogueEntry struct {
	Key         string          `json:"key"`
	DisplayName string          `json:"displayName"`
	Tradition   string          `json:"tradition"`
	Source      catalogueSource `json:"source"`
}

type catalogueSource struct {
	Work  string `json:"work"`
	Locus string `json:"locus"`
}

type codexLock struct {
	Version         string   `json:"version"`
	Package         string   `json:"package"`
	PlatformPackage string   `json:"platformPackage"`
	BinaryPath      string   `json:"binaryPath"`
	BinarySHA256    string   `json:"binarySha256"`
	SchemaBundle    string   `json:"schemaBundle"`
	SchemaSHA256    string   `json:"schemaSha256"`
	SchemaCommand   []string `json:"schemaCommand"`
}

type proofLedger struct {
	Version int           `json:"version"`
	Results []proofResult `json:"results"`
}

type proofResult struct {
	ID       string  `json:"id"`
	Gate     string  `json:"gate"`
	Status   string  `json:"status"`
	Evidence *string `json:"evidence"`
	Reason   *string `json:"reason"`
}

func Generate(root string) error {
	apiBytes, spec, catalogue, err := loadInputs(root)
	if err != nil {
		return err
	}
	digest := contractDigest(apiBytes, catalogue)
	outputs, err := render(spec, digest)
	if err != nil {
		return err
	}
	for path, content := range outputs {
		absolute := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			return fmt.Errorf("create generated directory for %s: %w", path, err)
		}
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func Verify(root string) error {
	apiBytes, spec, catalogue, err := loadInputs(root)
	if err != nil {
		return err
	}
	if err := verifyCodexLock(root); err != nil {
		return err
	}
	if err := verifyDvergatalEvidence(root, catalogue); err != nil {
		return err
	}
	if err := verifyProofLedger(root, spec); err != nil {
		return err
	}
	if err := verifyEvidenceContent(root); err != nil {
		return err
	}
	if err := verifyLoggerBoundary(root); err != nil {
		return err
	}

	outputs, err := render(spec, contractDigest(apiBytes, catalogue))
	if err != nil {
		return err
	}
	for path, expected := range outputs {
		actual, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return fmt.Errorf("read generated file %s: %w", path, err)
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("generated file is stale: %s; run ./scripts/build generate", path)
		}
	}
	return nil
}

func Accept(root string, target string, output io.Writer) error {
	_, spec, _, err := loadInputs(root)
	if err != nil {
		return err
	}
	ledger, err := loadProofLedger(root)
	if err != nil {
		return err
	}

	results := make(map[string]proofResult, len(ledger.Results))
	for _, result := range ledger.Results {
		results[result.ID] = result
	}
	accepted := true
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	for _, proof := range spec.Proofs {
		if !contains(proof.RequiredFor, target) {
			continue
		}
		result := results[proof.ID]
		if result.Status != "PASS" {
			accepted = false
		}
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("write acceptance result: %w", err)
		}
	}
	verdict := map[string]any{"target": target, "status": "PASS"}
	if !accepted {
		verdict["status"] = "FAIL"
	}
	if err := encoder.Encode(verdict); err != nil {
		return fmt.Errorf("write acceptance verdict: %w", err)
	}
	if !accepted {
		return fmt.Errorf("%s acceptance has a non-PASS proof", target)
	}
	return nil
}

func loadInputs(root string) ([]byte, apiContract, []catalogueEntry, error) {
	apiBytes, err := os.ReadFile(filepath.Join(root, apiPath))
	if err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("read %s: %w", apiPath, err)
	}
	var spec apiContract
	if err := decodeStrict(apiBytes, &spec); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("decode %s: %w", apiPath, err)
	}
	if err := validateContract(spec); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("validate %s: %w", apiPath, err)
	}

	catalogueBytes, err := os.ReadFile(filepath.Join(root, cataloguePath))
	if err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("read %s: %w", cataloguePath, err)
	}
	var catalogue []catalogueEntry
	if err := decodeStrict(catalogueBytes, &catalogue); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("decode %s: %w", cataloguePath, err)
	}
	if err := validateCatalogue(catalogue); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("validate %s: %w", cataloguePath, err)
	}
	return apiBytes, spec, catalogue, nil
}

func validateContract(spec apiContract) error {
	if spec.FormatVersion != 1 || spec.Namespace != "Skidbladnir.V1" || spec.ContractHeader != "Skidbladnir-Contract" {
		return errors.New("contract identity is not the reviewed v1 identity")
	}
	if spec.Bounds["objectiveScalars"] != 240 || spec.Bounds["workingDirectoryBytes"] != 4096 || spec.Bounds["agentPage"] != 100 || spec.Bounds["historyPoints"] != 300 || spec.Bounds["terminalFrameBytes"] != 65536 {
		return errors.New("reviewed transport bounds changed")
	}
	if len(spec.LocalCommands) != 7 || !equalStringSets(spec.LocalCommands, []string{"PrepareNew", "ListResumableSessions", "PrepareResume", "RegisterRuntime", "CreatePairingChallenge", "ListInstallations", "RevokeInstallation"}) {
		return errors.New("local command inventory changed")
	}

	types := map[string]bool{"String": true, "Boolean": true, "Int64": true, "Float64": true, "Instant": true}
	for name, scalar := range spec.Scalars {
		if !exportedName(name) || scalar.Base != "String" {
			return fmt.Errorf("invalid scalar %s", name)
		}
		if scalar.Pattern != "" {
			if _, err := regexp.Compile(scalar.Pattern); err != nil {
				return fmt.Errorf("scalar %s pattern: %w", name, err)
			}
		}
		types[name] = true
	}
	for name, values := range spec.Enums {
		if !exportedName(name) || len(values) == 0 || duplicate(values) != "" {
			return fmt.Errorf("invalid enum %s", name)
		}
		types[name] = true
	}
	for name := range spec.Records {
		if !exportedName(name) {
			return fmt.Errorf("invalid record name %s", name)
		}
		types[name] = true
	}
	for name := range spec.Unions {
		if !exportedName(name) {
			return fmt.Errorf("invalid union name %s", name)
		}
		types[name] = true
	}
	for name, fields := range spec.Records {
		if err := validateFields(name, fields, types); err != nil {
			return err
		}
	}
	for name, union := range spec.Unions {
		if union.Discriminator == "" || len(union.Variants) == 0 {
			return fmt.Errorf("union %s has no discriminator or variants", name)
		}
		for variant, raw := range union.Variants {
			var recordName string
			if err := json.Unmarshal(raw, &recordName); err == nil {
				if !types[recordName] {
					return fmt.Errorf("union %s variant %s references unknown %s", name, variant, recordName)
				}
				continue
			}
			var fields []fieldSpec
			if err := decodeStrict(raw, &fields); err != nil {
				return fmt.Errorf("union %s variant %s: %w", name, variant, err)
			}
			if err := validateFields(name+variant, fields, types); err != nil {
				return err
			}
		}
	}
	if !equalStringSets(spec.Enums["ErrorCode"], mapKeysInt(spec.Errors)) {
		return errors.New("ErrorCode and HTTP error mapping differ")
	}
	if !equalStringSets(spec.Enums["FactKind"], mapKeysRaw(spec.Unions["Fact"].Variants)) {
		return errors.New("FactKind and Fact variants differ")
	}
	if !equalStringSets(spec.Enums["WssFrameKind"], mapKeysRaw(spec.Unions["WssFrame"].Variants)) {
		return errors.New("WssFrameKind and WssFrame variants differ")
	}
	if err := validateRoutes(spec, types); err != nil {
		return err
	}
	seenProofs := map[string]bool{}
	for _, proof := range spec.Proofs {
		if proof.ID == "" || seenProofs[proof.ID] || !contains([]string{"static", "unit", "integration", "component", "system", "platform", "live", "e2e"}, proof.Gate) || len(proof.RequiredFor) == 0 {
			return fmt.Errorf("invalid proof %q", proof.ID)
		}
		seenProofs[proof.ID] = true
	}
	return nil
}

func validateRoutes(spec apiContract, types map[string]bool) error {
	seen := map[string]bool{}
	for _, route := range spec.Routes {
		key := route.Method + " " + route.Path
		if seen[key] || !contains([]string{"GET", "POST"}, route.Method) || !strings.HasPrefix(route.Path, "/v1/") || !contains([]string{"Pairing", "Bearer"}, route.Auth) {
			return fmt.Errorf("invalid route %s", key)
		}
		seen[key] = true
		outputs := 0
		for _, name := range []string{route.Response, route.Stream, route.Upgrade} {
			if name != "" {
				outputs++
				if !types[name] {
					return fmt.Errorf("route %s references unknown output %s", key, name)
				}
			}
		}
		if outputs != 1 || (route.Request != "" && !types[route.Request]) {
			return fmt.Errorf("route %s has invalid request/output", key)
		}
	}
	return nil
}

func validateFields(owner string, fields []fieldSpec, types map[string]bool) error {
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Name == "" || seen[field.Name] || !lowerCamelName(field.Name) {
			return fmt.Errorf("%s has invalid field %q", owner, field.Name)
		}
		seen[field.Name] = true
		match := fieldTypePattern.FindStringSubmatch(field.Type)
		if match == nil || !types[match[2]] {
			return fmt.Errorf("%s.%s has unknown type %s", owner, field.Name, field.Type)
		}
	}
	return nil
}

func validateCatalogue(entries []catalogueEntry) error {
	if len(entries) < minimumDwarfs {
		return fmt.Errorf("catalogue has %d entries; need at least %d", len(entries), minimumDwarfs)
	}
	keys := map[string]bool{}
	names := map[string]bool{}
	for index, entry := range entries {
		if !characterKeyPattern.MatchString(entry.Key) || keys[entry.Key] {
			return fmt.Errorf("entry %d has invalid or duplicate key %q", index, entry.Key)
		}
		keys[entry.Key] = true
		if entry.DisplayName == "" || names[entry.DisplayName] || !norm.NFC.IsNormalString(entry.DisplayName) || utf8.RuneCountInString(entry.DisplayName) > 64 || containsForbiddenText(entry.DisplayName) {
			return fmt.Errorf("entry %s has invalid or duplicate displayName %q", entry.Key, entry.DisplayName)
		}
		names[entry.DisplayName] = true
		if !contains([]string{"OldNorse", "Tolkien", "GermanicOperatic"}, entry.Tradition) {
			return fmt.Errorf("entry %s has forbidden tradition %q", entry.Key, entry.Tradition)
		}
		if strings.TrimSpace(entry.Source.Work) == "" || strings.TrimSpace(entry.Source.Locus) == "" {
			return fmt.Errorf("entry %s has incomplete source", entry.Key)
		}
	}
	return nil
}

func verifyCodexLock(root string) error {
	bytes, err := os.ReadFile(filepath.Join(root, codexLockPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", codexLockPath, err)
	}
	var lock codexLock
	if err := decodeStrict(bytes, &lock); err != nil {
		return fmt.Errorf("decode %s: %w", codexLockPath, err)
	}
	if lock.Version == "" || lock.Package != "@openai/codex@"+lock.Version || !strings.HasSuffix(lock.PlatformPackage, "@"+lock.Version+"-linux-x64") || !filepath.IsAbs(lock.BinaryPath) {
		return errors.New("codex.lock identity is incomplete")
	}
	if err := verifyDigest(lock.BinaryPath, lock.BinarySHA256); err != nil {
		return fmt.Errorf("pinned Codex binary: %w", err)
	}
	if err := verifyDigest(filepath.Join(root, lock.SchemaBundle), lock.SchemaSHA256); err != nil {
		return fmt.Errorf("pinned Codex schema: %w", err)
	}
	if len(lock.SchemaCommand) != 5 || lock.SchemaCommand[0] != lock.BinaryPath || lock.SchemaCommand[1] != "app-server" || lock.SchemaCommand[2] != "generate-json-schema" || lock.SchemaCommand[3] != "--out" || lock.SchemaCommand[4] != filepath.Dir(lock.SchemaBundle) {
		return errors.New("codex.lock schema command is not exact")
	}
	return nil
}

func verifyDigest(path string, expected string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != expected {
		return fmt.Errorf("digest mismatch for %s", path)
	}
	return nil
}

func verifyDvergatalEvidence(root string, entries []catalogueEntry) error {
	content, err := os.ReadFile(filepath.Join(root, dvergatalPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", dvergatalPath, err)
	}
	text := string(content)
	lowerText := strings.ToLower(text)
	for _, phrase := range []string{"portrait production", "rights basis", "dedupe", "exclusion"} {
		if !strings.Contains(lowerText, phrase) {
			return fmt.Errorf("%s is missing %q", dvergatalPath, phrase)
		}
	}
	for _, entry := range entries {
		if strings.Count(text, "ENTRY|key="+entry.Key+"|") != 1 {
			return fmt.Errorf("%s must contain catalogue key %s exactly once", dvergatalPath, entry.Key)
		}
	}
	return nil
}

func verifyProofLedger(root string, spec apiContract) error {
	ledger, err := loadProofLedger(root)
	if err != nil {
		return err
	}
	proofs := make(map[string]proofSpec, len(spec.Proofs))
	for _, proof := range spec.Proofs {
		proofs[proof.ID] = proof
	}
	seen := map[string]bool{}
	for _, result := range ledger.Results {
		proof, ok := proofs[result.ID]
		if !ok || seen[result.ID] || result.Gate != proof.Gate || !contains([]string{"PASS", "FAIL", "NOT_RUN"}, result.Status) {
			return fmt.Errorf("invalid proof result %q", result.ID)
		}
		seen[result.ID] = true
		if result.Status == "PASS" && (result.Evidence == nil || *result.Evidence == "") {
			return fmt.Errorf("PASS proof %s has no evidence", result.ID)
		}
		if result.Status == "NOT_RUN" && (result.Reason == nil || *result.Reason == "") {
			return fmt.Errorf("NOT_RUN proof %s has no reason", result.ID)
		}
		if result.Evidence != nil {
			path := filepath.Clean(*result.Evidence)
			if filepath.IsAbs(path) || strings.HasPrefix(path, "..") {
				return fmt.Errorf("proof %s has unsafe evidence path", result.ID)
			}
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				return fmt.Errorf("proof %s evidence: %w", result.ID, err)
			}
		}
	}
	if len(seen) != len(proofs) {
		return errors.New("proof ledger does not cover the closed proof inventory")
	}
	return nil
}

func loadProofLedger(root string) (proofLedger, error) {
	content, err := os.ReadFile(filepath.Join(root, proofsPath))
	if err != nil {
		return proofLedger{}, fmt.Errorf("read %s: %w", proofsPath, err)
	}
	var ledger proofLedger
	if err := decodeStrict(content, &ledger); err != nil {
		return proofLedger{}, fmt.Errorf("decode %s: %w", proofsPath, err)
	}
	if ledger.Version != 1 {
		return proofLedger{}, errors.New("proof ledger version is not 1")
	}
	return ledger, nil
}

func verifyEvidenceContent(root string) error {
	return filepath.WalkDir(filepath.Join(root, "evidence/live"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(content))
		for _, forbidden := range []string{"prompt_text", "assistant_text", "tool_input", "tool_output", "transcript_path", "terminal_bytes", "account_email", "bearer", "pairing_secret", "auth_token"} {
			if strings.Contains(lower, forbidden) {
				return fmt.Errorf("forbidden evidence field %q in %s", forbidden, path)
			}
		}
		return nil
	})
}

func verifyLoggerBoundary(root string) error {
	for _, base := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"logging"+string(filepath.Separator)) {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				name, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if name == "log" || name == "log/slog" {
					return fmt.Errorf("ad-hoc logger import outside internal/logging: %s", path)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func contractDigest(apiBytes []byte, catalogue []catalogueEntry) string {
	keys := make([]string, 0, len(catalogue))
	for _, entry := range catalogue {
		keys = append(keys, entry.Key)
	}
	sort.Strings(keys)
	keyBytes := []byte(strings.Join(keys, "\n"))
	hash := sha256.New()
	hash.Write([]byte(digestDomain))
	writeDigestPart(hash, apiPath, apiBytes)
	writeDigestPart(hash, "catalog/characters.keys", keyBytes)
	return hex.EncodeToString(hash.Sum(nil))
}

func writeDigestPart(writer io.Writer, path string, content []byte) {
	writer.Write([]byte(path)) // justify-ignore-error: hash.Hash.Write cannot fail.
	writer.Write([]byte{0})    // justify-ignore-error: hash.Hash.Write cannot fail.
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(content)))
	writer.Write(length[:]) // justify-ignore-error: hash.Hash.Write cannot fail.
	writer.Write(content)   // justify-ignore-error: hash.Hash.Write cannot fail.
}

func render(spec apiContract, digest string) (map[string][]byte, error) {
	goBytes, err := renderGo(spec, digest)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		goOutputPath: goBytes,
		kotlinPath:   renderKotlin(spec, digest),
	}, nil
}

func renderGo(spec apiContract, digest string) ([]byte, error) {
	var output strings.Builder
	output.WriteString("// Code generated by ./scripts/build generate. DO NOT EDIT.\n\npackage skidbladnirv1\n\n")
	for _, name := range sortedScalarNames(spec.Scalars) {
		output.WriteString("type " + name + " string\n")
	}
	output.WriteString("\nconst ContractDigestValue ContractDigest = \"")
	output.WriteString(digest)
	output.WriteString("\"\n")
	for _, name := range sortedEnumNames(spec.Enums) {
		output.WriteString("\ntype " + name + " string\n\nconst (\n")
		for _, value := range spec.Enums[name] {
			output.WriteString("\t" + name + goIdentifier(value) + " " + name + " = " + strconv.Quote(value) + "\n")
		}
		output.WriteString(")\n")
	}
	for _, name := range sortedRecordNames(spec.Records) {
		output.WriteString("\ntype " + name + " struct {\n")
		for _, field := range spec.Records[name] {
			optional := strings.HasPrefix(field.Type, "?")
			tag := field.Name
			if optional {
				tag += ",omitempty"
			}
			output.WriteString("\t" + goIdentifier(field.Name) + " " + goType(field.Type) + " `json:\"" + tag + "\"`\n")
		}
		output.WriteString("}\n")
	}
	for _, unionName := range sortedUnionNames(spec.Unions) {
		union := spec.Unions[unionName]
		output.WriteString("\ntype " + unionName + " interface {\n\tis" + unionName + "()\n}\n")
		for _, variant := range sortedRawNames(union.Variants) {
			raw := union.Variants[variant]
			var recordName string
			if json.Unmarshal(raw, &recordName) == nil {
				output.WriteString("\nfunc (" + recordName + ") is" + unionName + "() {}\n")
				continue
			}
			var fields []fieldSpec
			if err := decodeStrict(raw, &fields); err != nil {
				return nil, err
			}
			name := variant + unionName
			output.WriteString("\ntype " + name + " struct {\n")
			output.WriteString("\t" + goIdentifier(union.Discriminator) + " " + unionDiscriminatorType(spec, unionName) + " `json:\"" + union.Discriminator + "\"`\n")
			for _, field := range fields {
				optional := strings.HasPrefix(field.Type, "?")
				tag := field.Name
				if optional {
					tag += ",omitempty"
				}
				output.WriteString("\t" + goIdentifier(field.Name) + " " + goType(field.Type) + " `json:\"" + tag + "\"`\n")
			}
			output.WriteString("}\n\nfunc (" + name + ") is" + unionName + "() {}\n")
		}
	}
	output.WriteString("\nvar ErrorHTTPStatus = map[ErrorCode]int{\n")
	for _, code := range sortedIntNames(spec.Errors) {
		output.WriteString("\tErrorCode" + goIdentifier(code) + ": " + strconv.Itoa(spec.Errors[code]) + ",\n")
	}
	output.WriteString("}\n")
	formatted, err := format.Source([]byte(output.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated Go: %w", err)
	}
	return formatted, nil
}

func renderKotlin(spec apiContract, digest string) []byte {
	recordUnions := map[string][]string{}
	recordVariant := map[string]string{}
	for unionName, union := range spec.Unions {
		for variant, raw := range union.Variants {
			var recordName string
			if json.Unmarshal(raw, &recordName) == nil {
				recordUnions[recordName] = append(recordUnions[recordName], unionName)
				recordVariant[recordName] = variant
			}
		}
	}
	var output strings.Builder
	output.WriteString("// Code generated by ./scripts/build generate. DO NOT EDIT.\npackage dev.niels.skidbladnir.generated\n\nimport kotlinx.serialization.SerialName\nimport kotlinx.serialization.Serializable\n\n")
	output.WriteString("object SkidbladnirContract { const val digest: String = \"")
	output.WriteString(digest)
	output.WriteString("\" }\n")
	for _, name := range sortedScalarNames(spec.Scalars) {
		output.WriteString("\n@Serializable\n@JvmInline\nvalue class " + name + "(val value: String)\n")
	}
	for _, name := range sortedEnumNames(spec.Enums) {
		output.WriteString("\n@Serializable\nenum class " + name + " {\n")
		for index, value := range spec.Enums[name] {
			output.WriteString("    @SerialName(" + strconv.Quote(value) + ") " + kotlinIdentifier(value))
			if index+1 != len(spec.Enums[name]) {
				output.WriteString(",")
			}
			output.WriteString("\n")
		}
		output.WriteString("}\n")
	}
	for _, name := range sortedUnionNames(spec.Unions) {
		output.WriteString("\nsealed interface " + name + "\n")
	}
	for _, name := range sortedRecordNames(spec.Records) {
		output.WriteString("\n@Serializable\ndata class " + name + "(\n")
		variant := recordVariant[name]
		for index, field := range spec.Records[name] {
			output.WriteString("    val " + field.Name + ": " + kotlinType(field.Type))
			for _, unionName := range recordUnions[name] {
				union := spec.Unions[unionName]
				if field.Name == union.Discriminator {
					output.WriteString(" = " + unionDiscriminatorType(spec, unionName) + "." + kotlinIdentifier(variant))
				}
			}
			if index+1 != len(spec.Records[name]) {
				output.WriteString(",")
			}
			output.WriteString("\n")
		}
		output.WriteString(")")
		if unions := recordUnions[name]; len(unions) != 0 {
			output.WriteString(" : " + strings.Join(unions, ", "))
		}
		output.WriteString("\n")
	}
	for _, unionName := range sortedUnionNames(spec.Unions) {
		union := spec.Unions[unionName]
		for _, variant := range sortedRawNames(union.Variants) {
			raw := union.Variants[variant]
			var recordName string
			if json.Unmarshal(raw, &recordName) == nil {
				continue
			}
			var fields []fieldSpec
			if err := decodeStrict(raw, &fields); err != nil {
				panic(err) // justify-defect: validateContract has already accepted this committed union shape.
			}
			output.WriteString("\n@Serializable\ndata class " + variant + unionName + "(\n")
			output.WriteString("    val " + union.Discriminator + ": " + unionDiscriminatorType(spec, unionName) + " = " + unionDiscriminatorType(spec, unionName) + "." + kotlinIdentifier(variant))
			if len(fields) != 0 {
				output.WriteString(",")
			}
			output.WriteString("\n")
			for index, field := range fields {
				output.WriteString("    val " + field.Name + ": " + kotlinType(field.Type))
				if index+1 != len(fields) {
					output.WriteString(",")
				}
				output.WriteString("\n")
			}
			output.WriteString(") : " + unionName + "\n")
		}
	}
	output.WriteString("\nval errorHttpStatus: Map<ErrorCode, Int> = mapOf(\n")
	for _, code := range sortedIntNames(spec.Errors) {
		output.WriteString("    ErrorCode." + kotlinIdentifier(code) + " to " + strconv.Itoa(spec.Errors[code]) + ",\n")
	}
	output.WriteString(")\n")
	return []byte(output.String())
}

func goType(name string) string {
	match := fieldTypePattern.FindStringSubmatch(name)
	prefix := match[1]
	base := match[2]
	if base == "String" || base == "Instant" {
		base = "string"
	} else if base == "Boolean" {
		base = "bool"
	} else if base == "Int64" {
		base = "int64"
	} else if base == "Float64" {
		base = "float64"
	}
	if prefix == "?" {
		return "*" + base
	}
	if prefix == "[]" {
		return "[]" + base
	}
	return base
}

func kotlinType(name string) string {
	match := fieldTypePattern.FindStringSubmatch(name)
	prefix := match[1]
	base := match[2]
	if base == "Instant" {
		base = "String"
	} else if base == "Boolean" {
		base = "Boolean"
	} else if base == "Int64" {
		base = "Long"
	} else if base == "Float64" {
		base = "Double"
	}
	if prefix == "?" {
		return base + "?"
	}
	if prefix == "[]" {
		return "List<" + base + ">"
	}
	return base
}

func unionDiscriminatorType(spec apiContract, unionName string) string {
	variants := mapKeysRaw(spec.Unions[unionName].Variants)
	for enumName, values := range spec.Enums {
		if equalStringSets(values, variants) {
			return enumName
		}
	}
	panic("union discriminator has no exact enum") // justify-defect: contract validation requires every public union to have a closed discriminator enum.
}

func decodeStrict(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func containsForbiddenText(value string) bool {
	for _, runeValue := range value {
		if unicode.IsControl(runeValue) || runeValue == '\u2028' || runeValue == '\u2029' || (runeValue >= '\u202a' && runeValue <= '\u202e') || (runeValue >= '\u2066' && runeValue <= '\u2069') {
			return true
		}
	}
	return false
}

func exportedName(value string) bool {
	return value != "" && unicode.IsUpper([]rune(value)[0]) && strings.IndexFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) == -1
}

func lowerCamelName(value string) bool {
	return value != "" && unicode.IsLower([]rune(value)[0]) && strings.IndexFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) == -1
}

func goIdentifier(value string) string {
	return identifier(value, true)
}

func kotlinIdentifier(value string) string {
	identifier := identifier(value, true)
	if identifier == "" {
		return "Value"
	}
	if unicode.IsDigit([]rune(identifier)[0]) {
		return "Value" + identifier
	}
	return identifier
}

func identifier(value string, upper bool) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for index, part := range parts {
		runes := []rune(part)
		if len(runes) != 0 && (upper || index != 0) {
			runes[0] = unicode.ToUpper(runes[0])
		}
		parts[index] = string(runes)
	}
	return strings.Join(parts, "")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func duplicate(values []string) string {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			return value
		}
		seen[value] = true
	}
	return ""
}

func equalStringSets(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	copyLeft := append([]string(nil), left...)
	copyRight := append([]string(nil), right...)
	sort.Strings(copyLeft)
	sort.Strings(copyRight)
	return slicesEqual(copyLeft, copyRight)
}

func slicesEqual(left []string, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sortedScalarNames(values map[string]scalarSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedEnumNames(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRecordNames(values map[string][]fieldSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUnionNames(values map[string]unionSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRawNames(values map[string]json.RawMessage) []string {
	keys := mapKeysRaw(values)
	sort.Strings(keys)
	return keys
}

func sortedIntNames(values map[string]int) []string {
	keys := mapKeysInt(values)
	sort.Strings(keys)
	return keys
}

func mapKeysRaw(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func mapKeysInt(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
