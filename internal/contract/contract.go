package contract

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	apiPath           = "api/skidbladnir.v1.json"
	apiLockPath       = "api/skidbladnir.v1.lock"
	cataloguePath     = "catalog/characters.json"
	codexLockPath     = "codex.lock"
	goOutputPath      = "generated/api/go/skidbladnir.go"
	kotlinPath        = "generated/api/kotlin/SkidbladnirApi.kt"
	proofsPath        = "evidence/proof-ledger.json"
	dvergatalPath     = "evidence/sources/dvergatal.md"
	terminalLockPath  = "android/terminal.lock"
	workflowPath      = ".github/workflows/verify.yml"
	digestDomain      = "Skidbladnir.Contract.V1"
	schemaDomain      = "Skidbladnir.CodexSchemaTree.V1"
	acceptanceTimeout = 30 * time.Minute
	minimumDwarfs     = 100
	maximumRecents    = 8
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
	Version              string   `json:"version"`
	Package              string   `json:"package"`
	PlatformPackage      string   `json:"platformPackage"`
	BinaryPath           string   `json:"binaryPath"`
	BinarySHA256         string   `json:"binarySha256"`
	SchemaDirectory      string   `json:"schemaDirectory"`
	SchemaFiles          int      `json:"schemaFiles"`
	SchemaTreeSHA256     string   `json:"schemaTreeSha256"`
	SchemaBundle         string   `json:"schemaBundle"`
	SchemaSHA256         string   `json:"schemaSha256"`
	SchemaCommand        []string `json:"schemaCommand"`
	SourceRepository     string   `json:"sourceRepository"`
	SourceCommit         string   `json:"sourceCommit"`
	SourceTag            string   `json:"sourceTag"`
	HookSchemaDirectory  string   `json:"hookSchemaDirectory"`
	HookSchemaFiles      int      `json:"hookSchemaFiles"`
	HookSchemaTreeSHA256 string   `json:"hookSchemaTreeSha256"`
	HookSchemaCommand    []string `json:"hookSchemaCommand"`
}

type contractInputLock struct {
	Version               int    `json:"version"`
	APISHA256             string `json:"apiSha256"`
	CatalogueSHA256       string `json:"catalogueSha256"`
	CatalogueKeySetSHA256 string `json:"catalogueKeySetSha256"`
	CatalogueEntries      int    `json:"catalogueEntries"`
}

type schemaFile struct {
	Path   string
	SHA256 string
}

type terminalAssetLock struct {
	Package   string            `json:"package"`
	Version   string            `json:"version"`
	Source    string            `json:"source"`
	Integrity string            `json:"integrity"`
	Files     map[string]string `json:"files"`
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

type acceptanceGateResult struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
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
	if err := verifyHookArtifacts(root); err != nil {
		return err
	}
	if err := verifyTerminalAssets(root); err != nil {
		return err
	}
	if err := verifyDvergatalEvidence(root, catalogue); err != nil {
		return err
	}
	if _, err := verifyProofLedger(root, spec); err != nil {
		return err
	}
	if err := verifyEvidenceContent(root); err != nil {
		return err
	}
	if err := verifyWorkflow(root); err != nil {
		return err
	}
	if err := verifyProductLanguage(root, spec); err != nil {
		return err
	}
	if err := verifyLoggerBoundary(root); err != nil {
		return err
	}
	if err := verifyJustifications(root); err != nil {
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
	gates, err := acceptanceGates(target)
	if err != nil {
		return err
	}
	_, spec, _, err := loadInputs(root)
	if err != nil {
		return err
	}
	ledger, err := verifyProofLedger(root, spec)
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
	for _, gate := range gates {
		result := executeAcceptanceGate(root, gate)
		if result.Status != "PASS" {
			accepted = false
		}
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("write acceptance gate result: %w", err)
		}
	}
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
		return fmt.Errorf("%s acceptance has a non-PASS gate or proof", target)
	}
	return nil
}

func acceptanceGates(target string) ([]string, error) {
	switch target {
	case "core":
		return []string{"static", "unit", "integration", "component", "system", "platform", "live", "e2e"}, nil
	case "upgrade":
		return []string{"static", "unit", "integration", "component", "system", "upgrade-live"}, nil
	default:
		return nil, fmt.Errorf("unknown acceptance target %q", target)
	}
}

func executeAcceptanceGate(root, gate string) acceptanceGateResult {
	ctx, cancel := context.WithTimeout(context.Background(), acceptanceTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(root, "scripts", "test"), gate)
	command.Dir = root
	command.Env = append(os.Environ(), "SKIDBLADNIR_ACCEPTANCE=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 5 * time.Second
	var result bytes.Buffer
	command.Stdout = &result
	command.Stderr = io.Discard
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return acceptanceGateResult{Gate: gate, Status: "FAIL", Reason: "declared gate timed out"}
	}
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return acceptanceGateResult{Gate: gate, Status: "FAIL", Reason: "declared gate could not run"}
		}
		exitCode = exitError.ExitCode()
	}
	parsed, parseErr := parseAcceptanceGateResult(gate, exitCode, result.Bytes())
	if parseErr != nil {
		return acceptanceGateResult{Gate: gate, Status: "FAIL", Reason: "declared gate returned an invalid result"}
	}
	return parsed
}

func parseAcceptanceGateResult(gate string, exitCode int, content []byte) (acceptanceGateResult, error) {
	var result acceptanceGateResult
	if err := decodeStrict(bytes.TrimSpace(content), &result); err != nil {
		return acceptanceGateResult{}, err
	}
	if result.Gate != gate {
		return acceptanceGateResult{}, fmt.Errorf("gate result names %q; want %q", result.Gate, gate)
	}
	expected := "FAIL"
	if exitCode == 0 {
		expected = "PASS"
	} else if exitCode == 2 {
		expected = "NOT_RUN"
	}
	if result.Status != expected {
		return acceptanceGateResult{}, fmt.Errorf("gate %s exit %d contradicts status %s", gate, exitCode, result.Status)
	}
	if (result.Status == "PASS" && result.Reason != "") || (result.Status != "PASS" && result.Reason == "") {
		return acceptanceGateResult{}, fmt.Errorf("gate %s has invalid result reason", gate)
	}
	return result, nil
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
	lockBytes, err := os.ReadFile(filepath.Join(root, apiLockPath))
	if err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("read %s: %w", apiLockPath, err)
	}
	var lock contractInputLock
	if err := decodeStrict(lockBytes, &lock); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("decode %s: %w", apiLockPath, err)
	}
	if err := validateContractInputLock(lock, apiBytes, catalogueBytes, catalogue); err != nil {
		return nil, apiContract{}, nil, fmt.Errorf("validate %s: %w", apiLockPath, err)
	}
	return apiBytes, spec, catalogue, nil
}

func validateContractInputLock(lock contractInputLock, apiBytes []byte, catalogueBytes []byte, catalogue []catalogueEntry) error {
	if lock.Version != 1 {
		return errors.New("contract input lock version is not 1")
	}
	if lock.APISHA256 != sha256Hex(apiBytes) {
		return errors.New("API contract digest differs from reviewed lock")
	}
	if lock.CatalogueSHA256 != sha256Hex(catalogueBytes) {
		return errors.New("catalogue digest differs from reviewed lock")
	}
	if lock.CatalogueEntries != len(catalogue) {
		return errors.New("catalogue count differs from reviewed lock")
	}
	if lock.CatalogueKeySetSHA256 != sha256Hex(catalogueKeySetBytes(catalogue)) {
		return errors.New("catalogue key set differs from reviewed lock")
	}
	return nil
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
		discriminatorType, found := findUnionDiscriminatorType(spec, name)
		if !found {
			return fmt.Errorf("union %s has no exact discriminator enum", name)
		}
		for variant, raw := range union.Variants {
			var recordName string
			if err := json.Unmarshal(raw, &recordName); err == nil {
				fields, exists := spec.Records[recordName]
				if !exists {
					return fmt.Errorf("union %s variant %s references unknown %s", name, variant, recordName)
				}
				discriminatorFields := 0
				for _, field := range fields {
					if field.Name == union.Discriminator {
						discriminatorFields++
						if field.Type != discriminatorType {
							return fmt.Errorf("union %s record %s has invalid discriminator type %s", name, recordName, field.Type)
						}
					}
				}
				if discriminatorFields != 1 {
					return fmt.Errorf("union %s record %s must declare its discriminator exactly once", name, recordName)
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
			for _, field := range fields {
				if field.Name == union.Discriminator {
					return fmt.Errorf("union %s inline variant %s repeats its discriminator", name, variant)
				}
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
	if err := validateCodexLockIdentity(lock); err != nil {
		return err
	}
	if err := verifyDigest(lock.BinaryPath, lock.BinarySHA256); err != nil {
		return fmt.Errorf("pinned Codex binary: %w", err)
	}
	if err := verifyCodexSchemaGeneration(root, lock); err != nil {
		return err
	}
	if err := verifyDigest(filepath.Join(root, lock.SchemaBundle), lock.SchemaSHA256); err != nil {
		return fmt.Errorf("pinned Codex schema: %w", err)
	}
	files, err := collectSchemaFiles(filepath.Join(root, lock.SchemaDirectory))
	if err != nil {
		return fmt.Errorf("read pinned Codex schema tree: %w", err)
	}
	if len(files) != lock.SchemaFiles || schemaTreeDigest(files) != lock.SchemaTreeSHA256 {
		return errors.New("pinned Codex schema tree differs from codex.lock")
	}
	hookFiles, err := collectSchemaFiles(filepath.Join(root, lock.HookSchemaDirectory))
	if err != nil {
		return fmt.Errorf("read pinned Codex hook schema tree: %w", err)
	}
	if len(hookFiles) != lock.HookSchemaFiles || schemaTreeDigest(hookFiles) != lock.HookSchemaTreeSHA256 {
		return errors.New("pinned Codex hook schema tree differs from codex.lock")
	}
	return nil
}

func verifyCodexSchemaGeneration(root string, lock codexLock) error {
	temporaryDirectory, err := os.MkdirTemp("", "skidbladnir-codex-schema-")
	if err != nil {
		return fmt.Errorf("create temporary Codex schema directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(temporaryDirectory) // justify-ignore-error: the operating system owns cleanup of an unpublished temporary verification tree after process exit.
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, lock.BinaryPath, "app-server", "generate-json-schema", "--experimental", "--out", temporaryDirectory)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("pinned Codex schema generation timed out")
		}
		return fmt.Errorf("regenerate pinned Codex schema: %w", err)
	}

	generated, err := collectSchemaFiles(temporaryDirectory)
	if err != nil {
		return fmt.Errorf("read regenerated Codex schema tree: %w", err)
	}
	committed, err := collectSchemaFiles(filepath.Join(root, lock.SchemaDirectory))
	if err != nil {
		return fmt.Errorf("read committed Codex schema tree: %w", err)
	}
	withoutHooks := committed[:0]
	for _, file := range committed {
		if !strings.HasPrefix(file.Path, "hooks/") {
			withoutHooks = append(withoutHooks, file)
		}
	}
	if !equalSchemaFiles(generated, withoutHooks) {
		return errors.New("committed Codex schemas are not reproducible from the pinned binary")
	}
	return nil
}

func equalSchemaFiles(left, right []schemaFile) bool {
	if len(left) != len(right) {
		return false
	}
	left = append([]schemaFile(nil), left...)
	right = append([]schemaFile(nil), right...)
	sort.Slice(left, func(i, j int) bool { return left[i].Path < left[j].Path })
	sort.Slice(right, func(i, j int) bool { return right[i].Path < right[j].Path })
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateCodexLockIdentity(lock codexLock) error {
	expectedSchemaDirectory := filepath.ToSlash(filepath.Join("schemas", "codex", lock.Version))
	if lock.Version == "" || lock.Package != "@openai/codex@"+lock.Version || !strings.HasSuffix(lock.PlatformPackage, "@"+lock.Version+"-linux-x64") || !filepath.IsAbs(lock.BinaryPath) || len(lock.BinarySHA256) != sha256.Size*2 {
		return errors.New("codex.lock identity is incomplete")
	}
	if lock.SchemaDirectory != expectedSchemaDirectory || lock.SchemaFiles < 1 || len(lock.SchemaTreeSHA256) != sha256.Size*2 || filepath.ToSlash(filepath.Dir(lock.SchemaBundle)) != lock.SchemaDirectory || len(lock.SchemaSHA256) != sha256.Size*2 {
		return errors.New("codex.lock schema identity is incomplete")
	}
	if len(lock.SchemaCommand) != 6 || lock.SchemaCommand[0] != lock.BinaryPath || lock.SchemaCommand[1] != "app-server" || lock.SchemaCommand[2] != "generate-json-schema" || lock.SchemaCommand[3] != "--experimental" || lock.SchemaCommand[4] != "--out" || filepath.ToSlash(lock.SchemaCommand[5]) != lock.SchemaDirectory {
		return errors.New("codex.lock schema command is not exact")
	}
	if lock.SourceRepository != "https://github.com/openai/codex.git" || len(lock.SourceCommit) != 40 || lock.SourceTag != "rust-v"+lock.Version {
		return errors.New("codex.lock source identity is incomplete")
	}
	if lock.HookSchemaDirectory != filepath.ToSlash(filepath.Join(lock.SchemaDirectory, "hooks")) || lock.HookSchemaFiles != 7 || len(lock.HookSchemaTreeSHA256) != sha256.Size*2 {
		return errors.New("codex.lock hook schema identity is incomplete")
	}
	if len(lock.HookSchemaCommand) != 1 || lock.HookSchemaCommand[0] != "./scripts/vendor-codex-hook-schemas" {
		return errors.New("codex.lock hook schema command is not exact")
	}
	return nil
}

func collectSchemaFiles(directory string) ([]schemaFile, error) {
	var files []schemaFile
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("schema tree contains symlink %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("schema tree contains non-regular file %s", path)
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.ContainsAny(relative, "\r\n") {
			return fmt.Errorf("schema path contains a line break: %q", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, schemaFile{Path: relative, SHA256: sha256Hex(content)})
		return nil
	})
	return files, err
}

func schemaTreeDigest(files []schemaFile) string {
	ordered := append([]schemaFile(nil), files...)
	sort.Slice(ordered, func(left int, right int) bool { return ordered[left].Path < ordered[right].Path })
	hash := sha256.New()
	hash.Write([]byte(schemaDomain)) // justify-ignore-error: hash.Hash.Write cannot fail.
	for _, file := range ordered {
		hash.Write([]byte(file.SHA256)) // justify-ignore-error: hash.Hash.Write cannot fail.
		hash.Write([]byte("  "))        // justify-ignore-error: hash.Hash.Write cannot fail.
		hash.Write([]byte(file.Path))   // justify-ignore-error: hash.Hash.Write cannot fail.
		hash.Write([]byte{'\n'})        // justify-ignore-error: hash.Hash.Write cannot fail.
	}
	return hex.EncodeToString(hash.Sum(nil))
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

func verifyTerminalAssets(root string) error {
	content, err := os.ReadFile(filepath.Join(root, terminalLockPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", terminalLockPath, err)
	}
	var lock terminalAssetLock
	if err := decodeStrict(content, &lock); err != nil {
		return fmt.Errorf("decode %s: %w", terminalLockPath, err)
	}
	if lock.Package != "@xterm/xterm" || lock.Version != "6.0.0" || lock.Source != "https://registry.npmjs.org/@xterm/xterm/-/xterm-6.0.0.tgz" || !strings.HasPrefix(lock.Integrity, "sha512-") {
		return errors.New("terminal asset lock identity differs from the reviewed pin")
	}
	required := map[string]bool{
		"android/app/src/main/assets/terminal/vendor/xterm-6.0.0.js":      true,
		"android/app/src/main/assets/terminal/vendor/xterm-6.0.0.css":     true,
		"android/app/src/main/assets/terminal/vendor/xterm-6.0.0.LICENSE": true,
	}
	if len(lock.Files) != len(required) {
		return errors.New("terminal asset lock has the wrong file inventory")
	}
	for path, expectedDigest := range lock.Files {
		if !required[path] || len(expectedDigest) != sha256.Size*2 {
			return fmt.Errorf("terminal asset lock has unexpected file %s", path)
		}
		if err := verifyDigest(filepath.Join(root, filepath.FromSlash(path)), expectedDigest); err != nil {
			return fmt.Errorf("terminal asset %s: %w", path, err)
		}
	}
	return nil
}

func verifyDvergatalEvidence(root string, entries []catalogueEntry) error {
	content, err := os.ReadFile(filepath.Join(root, dvergatalPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", dvergatalPath, err)
	}
	if err := validateDvergatalEvidence(content, entries); err != nil {
		return fmt.Errorf("validate %s: %w", dvergatalPath, err)
	}
	return nil
}

func validateDvergatalEvidence(content []byte, entries []catalogueEntry) error {
	text := string(content)
	for _, heading := range []string{"## Acceptance for good content", "## Portrait production and rights basis", "## Dedupe and exclusion"} {
		if !strings.Contains(text, heading) {
			return fmt.Errorf("missing section %q", heading)
		}
	}
	recorded := make(map[string]catalogueEntry, len(entries))
	for lineNumber, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "ENTRY|") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 6 {
			return fmt.Errorf("entry line %d has %d fields; want 6", lineNumber+1, len(parts))
		}
		values := make([]string, 5)
		for index, name := range []string{"key", "displayName", "tradition", "work", "locus"} {
			prefix := name + "="
			if !strings.HasPrefix(parts[index+1], prefix) || len(parts[index+1]) == len(prefix) {
				return fmt.Errorf("entry line %d has invalid %s field", lineNumber+1, name)
			}
			values[index] = strings.TrimPrefix(parts[index+1], prefix)
		}
		entry := catalogueEntry{
			Key:         values[0],
			DisplayName: values[1],
			Tradition:   values[2],
			Source:      catalogueSource{Work: values[3], Locus: values[4]},
		}
		if _, exists := recorded[entry.Key]; exists {
			return fmt.Errorf("duplicate evidence entry %s", entry.Key)
		}
		recorded[entry.Key] = entry
	}
	if len(recorded) != len(entries) {
		return fmt.Errorf("evidence has %d entries; catalogue has %d", len(recorded), len(entries))
	}
	for _, entry := range entries {
		if recordedEntry, exists := recorded[entry.Key]; !exists || recordedEntry != entry {
			return fmt.Errorf("evidence does not exactly match catalogue entry %s", entry.Key)
		}
	}
	return nil
}

func verifyProofLedger(root string, spec apiContract) (proofLedger, error) {
	ledger, err := loadProofLedger(root)
	if err != nil {
		return proofLedger{}, err
	}
	proofs := make(map[string]proofSpec, len(spec.Proofs))
	for _, proof := range spec.Proofs {
		proofs[proof.ID] = proof
	}
	seen := map[string]bool{}
	for _, result := range ledger.Results {
		proof, ok := proofs[result.ID]
		if !ok || seen[result.ID] {
			return proofLedger{}, fmt.Errorf("invalid proof result %q", result.ID)
		}
		seen[result.ID] = true
		if err := validateProofResult(proof, result); err != nil {
			return proofLedger{}, err
		}
		if result.Evidence != nil {
			info, err := os.Lstat(filepath.Join(root, *result.Evidence))
			if err != nil {
				return proofLedger{}, fmt.Errorf("proof %s evidence: %w", result.ID, err)
			}
			if !info.Mode().IsRegular() {
				return proofLedger{}, fmt.Errorf("proof %s evidence is not a regular file", result.ID)
			}
		}
	}
	if len(seen) != len(proofs) {
		return proofLedger{}, errors.New("proof ledger does not cover the closed proof inventory")
	}
	return ledger, nil
}

func validateProofResult(proof proofSpec, result proofResult) error {
	if result.ID != proof.ID || result.Gate != proof.Gate || !contains([]string{"PASS", "FAIL", "NOT_RUN"}, result.Status) {
		return fmt.Errorf("invalid proof result %q", result.ID)
	}
	if result.Status == "PASS" && (result.Evidence == nil || *result.Evidence == "") {
		return fmt.Errorf("PASS proof %s has no evidence", result.ID)
	}
	if result.Status != "PASS" && (result.Reason == nil || *result.Reason == "") {
		return fmt.Errorf("%s proof %s has no reason", result.Status, result.ID)
	}
	if result.Evidence == nil {
		return nil
	}
	path := filepath.Clean(*result.Evidence)
	if path != *result.Evidence || filepath.IsAbs(path) || !strings.HasPrefix(path, filepath.Join("evidence", "live")+string(filepath.Separator)) {
		return fmt.Errorf("proof %s has unscanned evidence path", result.ID)
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
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("evidence entry is not a regular file: %s", path)
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

func verifyWorkflow(root string) error {
	content, err := os.ReadFile(filepath.Join(root, workflowPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", workflowPath, err)
	}
	if err := validateWorkflow(content); err != nil {
		return fmt.Errorf("validate %s: %w", workflowPath, err)
	}
	return nil
}

func validateWorkflow(content []byte) error {
	text := string(content)
	for _, forbidden := range []string{"pull_request_target:", "continue-on-error:", "|| true", "write-all"} {
		if strings.Contains(text, forbidden) {
			return fmt.Errorf("workflow contains forbidden %q", forbidden)
		}
	}
	if !strings.Contains(text, "pull_request:") || !strings.Contains(text, "permissions:\n  contents: read") {
		return errors.New("workflow lacks the reviewed trigger or read-only permissions")
	}
	if regexp.MustCompile(`(?m)^\s+[a-z][a-z-]*:\s+write\s*$`).Match(content) {
		return errors.New("workflow grants write permission")
	}
	required := []string{
		"run: ./scripts/test static",
		"run: ./scripts/build all",
		"run: ./scripts/test unit",
	}
	for _, command := range required {
		if strings.Count(text, command) != 1 {
			return fmt.Errorf("workflow must contain exactly one %q", command)
		}
	}
	for lineNumber, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if strings.HasPrefix(trimmed, "uses:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
			parts := strings.Split(value, "@")
			if len(parts) != 2 || parts[0] == "" || !regexp.MustCompile(`^[0-9a-f]{40}(?:\s+#.*)?$`).MatchString(parts[1]) {
				return fmt.Errorf("workflow line %d has an unpinned action", lineNumber+1)
			}
		}
		if strings.HasPrefix(trimmed, "run: ./scripts/test ") && !contains(required, trimmed) {
			return fmt.Errorf("workflow line %d invokes an undeclared test gate", lineNumber+1)
		}
		if strings.HasPrefix(trimmed, "run: ./scripts/build ") && trimmed != "run: ./scripts/build all" {
			return fmt.Errorf("workflow line %d invokes an undeclared build target", lineNumber+1)
		}
	}
	return nil
}

func verifyProductLanguage(root string, spec apiContract) error {
	surface := spec
	surface.Proofs = nil
	content, err := json.Marshal(surface)
	if err != nil {
		return fmt.Errorf("encode product contract: %w", err)
	}
	if err := validateProductText(string(content)); err != nil {
		return fmt.Errorf("validate %s product language: %w", apiPath, err)
	}

	return filepath.WalkDir(filepath.Join(root, "android", "app", "src", "main"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".css", ".html", ".js", ".kt", ".kts", ".xml":
		default:
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := validateProductText(string(content)); err != nil {
			return fmt.Errorf("validate product language in %s: %w", path, err)
		}
		return nil
	})
}

func validateProductText(content string) error {
	words := productWords(content)
	for index, word := range words {
		if forbiddenProductWord(word) {
			return fmt.Errorf("reserved product word %q", word)
		}
		if index+1 < len(words) && forbiddenProductPair(word, words[index+1]) {
			return fmt.Errorf("reserved product phrase %q", word+" "+words[index+1])
		}
	}
	return nil
}

func productWords(content string) []string {
	runes := []rune(norm.NFKD.String(content))
	words := make([]string, 0, len(runes)/8)
	word := make([]rune, 0, 16)
	flush := func() {
		if len(word) == 0 {
			return
		}
		words = append(words, string(word))
		word = word[:0]
	}
	for index, current := range runes {
		if unicode.Is(unicode.Mn, current) {
			continue
		}
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			flush()
			continue
		}
		if len(word) != 0 && unicode.IsUpper(current) {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextIsLower {
				flush()
			}
		}
		word = append(word, unicode.ToLower(current))
	}
	flush()
	return words
}

func forbiddenProductWord(word string) bool {
	switch word {
	case "ariadne", "gloriana", "sutradhara", "aule", "hephaestus", "prospero", "solomon", "shabti", "gwydion", "dvergatal",
		"project", "projects", "repo", "repos", "repository", "repositories", "worktree", "worktrees", "quota", "quotas",
		"provider", "providers", "transcript", "transcripts", "composer", "composers", "appserver", "workspacemode", "rawtarget", "nstack", "kstack":
		return true
	default:
		return false
	}
}

func forbiddenProductPair(first string, second string) bool {
	switch first + " " + second {
	case "app server", "app servers", "workspace mode", "workspace modes", "raw target", "raw targets", "n stack", "k stack":
		return true
	default:
		return false
	}
}

func verifyLoggerBoundary(root string) error {
	for _, base := range []string{"cmd", "internal", filepath.Join("android", "app", "src", "main")} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "vendor" || strings.HasSuffix(path, filepath.Join("internal", "logging")) {
					return filepath.SkipDir
				}
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := validateAdHocLogging(path, content); err != nil {
				return fmt.Errorf("validate logging in %s: %w", path, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func validateAdHocLogging(path string, content []byte) error {
	switch filepath.Ext(path) {
	case ".go":
		return validateGoLogging(path, content)
	case ".js":
		if regexp.MustCompile(`\bconsole\b`).Match(content) {
			return errors.New("ad-hoc JavaScript console call")
		}
	case ".kt", ".kts":
		text := string(content)
		for _, forbidden := range []string{"android.util.Log", "Log.", "println(", "System.out", "System.err"} {
			if strings.Contains(text, forbidden) {
				return fmt.Errorf("ad-hoc Kotlin log call %q", forbidden)
			}
		}
	}
	return nil
}

func validateGoLogging(path string, content []byte) error {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, content, 0)
	if err != nil {
		return err
	}
	imports := make(map[string]string, len(parsed.Imports))
	for _, imported := range parsed.Imports {
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return err
		}
		if name == "log" || name == "log/slog" {
			return fmt.Errorf("ad-hoc logger import %q", name)
		}
		alias := filepath.Base(name)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if name == "fmt" && alias == "." {
			return errors.New("dot-imported fmt bypasses the logging boundary")
		}
		imports[alias] = name
	}

	var result error
	ast.Inspect(parsed, func(node ast.Node) bool {
		if node == nil || result != nil {
			return result == nil
		}
		call, ok := node.(*ast.CallExpr) // justify-type-assertion: only call expressions can invoke an ad-hoc logging surface.
		if !ok {
			return true
		}
		switch function := call.Fun.(type) { // justify-type-assertion: the Go AST exposes call targets through its closed expression interface.
		case *ast.Ident:
			if function.Name == "print" || function.Name == "println" {
				result = fmt.Errorf("ad-hoc builtin %s call", function.Name)
			}
		case *ast.SelectorExpr:
			qualifier, ok := function.X.(*ast.Ident) // justify-type-assertion: only a package identifier can select a package-level fmt function.
			if !ok || imports[qualifier.Name] != "fmt" || !contains([]string{"Print", "Printf", "Println", "Fprint", "Fprintf", "Fprintln"}, function.Sel.Name) {
				return true
			}
			if strings.HasPrefix(function.Sel.Name, "F") && commandPath(path) && len(call.Args) != 0 && isStandardError(call.Args[0], imports) {
				return true
			}
			result = fmt.Errorf("ad-hoc fmt.%s call", function.Sel.Name)
		}
		return result == nil
	})
	return result
}

func commandPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.HasPrefix(clean, "cmd/") || strings.Contains(clean, "/cmd/")
}

func isStandardError(expression ast.Expr, imports map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr) // justify-type-assertion: os.Stderr is represented by a selector expression.
	if !ok || selector.Sel.Name != "Stderr" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident) // justify-type-assertion: os.Stderr must be qualified by the imported os package identifier.
	return ok && imports[qualifier.Name] == "os"
}

func verifyJustifications(root string) error {
	for _, base := range []string{"cmd", "internal", "android", "generated"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				name := entry.Name()
				if name == ".gradle" || name == "build" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".go" && extension != ".kt" && extension != ".kts" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := validateJustificationLines(path, content); err != nil {
				return err
			}
			if extension == ".go" {
				return validateGoJustifications(path, content)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func validateJustificationLines(path string, content []byte) error {
	extension := filepath.Ext(path)
	if extension == ".go" {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
		if err != nil {
			return err
		}
		for _, group := range parsed.Comments {
			for _, comment := range group.List {
				line := fileSet.Position(comment.Slash).Line
				if err := validateJustificationComment(path, line, comment.Text); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for index, line := range strings.Split(string(content), "\n") {
		comment := ""
		if commentIndex := strings.Index(line, "//"); commentIndex >= 0 {
			comment = line[commentIndex:]
		}
		if err := validateJustificationComment(path, index+1, comment); err != nil {
			return err
		}
		if extension == ".kt" || extension == ".kts" {
			if (strings.Contains(line, "@Suppress") || strings.Contains(line, "@OptIn") || strings.Contains(comment, "noinspection")) && !strings.Contains(comment, "justify-override:") {
				return fmt.Errorf("%s:%d override lacks justify-override", path, index+1)
			}
		}
	}
	return nil
}

func validateJustificationComment(path string, line int, comment string) error {
	known := map[string]bool{
		"justify-defect":                  true,
		"justify-ignore-error":            true,
		"justify-service-invariant-check": true,
		"justify-polling":                 true,
		"justify-retry-schedule":          true,
		"justify-override":                true,
		"justify-dead-code":               true,
		"justify-type-assertion":          true,
		"justify-base64url-over-base64":   true,
	}
	if tokenIndex := strings.Index(comment, "justify-"); tokenIndex >= 0 {
		remainder := comment[tokenIndex:]
		colon := strings.IndexByte(remainder, ':')
		if colon < 0 {
			return fmt.Errorf("%s:%d justification has no colon", path, line)
		}
		justification := remainder[:colon]
		if !known[justification] || strings.TrimSpace(remainder[colon+1:]) == "" {
			return fmt.Errorf("%s:%d has unknown or empty justification %s", path, line, justification)
		}
	}
	if strings.Contains(comment, "nolint") || strings.Contains(comment, "#nosec") || strings.Contains(comment, "lint:ignore") || strings.Contains(comment, "go:nocheckptr") {
		if !strings.Contains(comment, "justify-override:") {
			return fmt.Errorf("%s:%d suppression lacks justify-override", path, line)
		}
	}
	return nil
}

func validateGoJustifications(path string, content []byte) error {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, content, 0)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	var result error
	ast.Inspect(parsed, func(node ast.Node) bool {
		if node == nil || result != nil {
			return result == nil
		}
		line := fileSet.Position(node.Pos()).Line
		required := ""
		switch value := node.(type) { // justify-type-assertion: the Go AST exposes syntax kinds through its closed node interface.
		case *ast.TypeAssertExpr:
			required = "justify-type-assertion:"
		case *ast.CallExpr:
			if identifier, ok := value.Fun.(*ast.Ident); ok && identifier.Name == "panic" { // justify-type-assertion: only identifier calls can name the panic builtin.
				required = "justify-defect:"
			}
		case *ast.AssignStmt:
			last := value.Lhs[len(value.Lhs)-1]
			identifier, ok := last.(*ast.Ident) // justify-type-assertion: only identifier nodes can represent the blank assignment target.
			if ok && identifier.Name == "_" {
				for _, expression := range value.Rhs {
					if _, ok := expression.(*ast.CallExpr); ok { // justify-type-assertion: a discarded call result is the only assignment shape governed by the ignored-error rule.
						required = "justify-ignore-error:"
						break
					}
				}
			}
		}
		if required != "" && (line < 1 || line > len(lines) || !strings.Contains(lines[line-1], required)) {
			result = fmt.Errorf("%s:%d requires %s", path, line, required)
			return false
		}
		return true
	})
	return result
}

func contractDigest(apiBytes []byte, catalogue []catalogueEntry) string {
	hash := sha256.New()
	hash.Write([]byte(digestDomain))
	writeDigestPart(hash, apiPath, apiBytes)
	writeDigestPart(hash, "catalog/characters.keys", catalogueKeySetBytes(catalogue))
	return hex.EncodeToString(hash.Sum(nil))
}

func catalogueKeySetBytes(catalogue []catalogueEntry) []byte {
	keys := make([]string, 0, len(catalogue))
	for _, entry := range catalogue {
		keys = append(keys, entry.Key)
	}
	sort.Strings(keys)
	return []byte(strings.Join(keys, "\n"))
}

func sha256Hex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
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

type renderedUnionVariant struct {
	UnionName         string
	Variant           string
	ConcreteName      string
	Discriminator     string
	DiscriminatorType string
	Fields            []fieldSpec
}

func renderedUnionVariants(spec apiContract) ([]renderedUnionVariant, error) {
	var variants []renderedUnionVariant
	for _, unionName := range sortedUnionNames(spec.Unions) {
		union := spec.Unions[unionName]
		discriminatorType := unionDiscriminatorType(spec, unionName)
		for _, variant := range sortedRawNames(union.Variants) {
			raw := union.Variants[variant]
			concreteName := variant + unionName
			var fields []fieldSpec
			var recordName string
			if json.Unmarshal(raw, &recordName) == nil {
				concreteName = recordName
				fields = append([]fieldSpec(nil), spec.Records[recordName]...)
				fields = removeField(fields, union.Discriminator)
			} else if err := decodeStrict(raw, &fields); err != nil {
				return nil, err
			}
			variants = append(variants, renderedUnionVariant{
				UnionName:         unionName,
				Variant:           variant,
				ConcreteName:      concreteName,
				Discriminator:     union.Discriminator,
				DiscriminatorType: discriminatorType,
				Fields:            fields,
			})
		}
	}
	return variants, nil
}

func removeField(fields []fieldSpec, name string) []fieldSpec {
	result := make([]fieldSpec, 0, len(fields))
	for _, field := range fields {
		if field.Name != name {
			result = append(result, field)
		}
	}
	return result
}

func renderGo(spec apiContract, digest string) ([]byte, error) {
	variants, err := renderedUnionVariants(spec)
	if err != nil {
		return nil, err
	}
	recordVariant := map[string]renderedUnionVariant{}
	for _, variant := range variants {
		if _, exists := spec.Records[variant.ConcreteName]; exists {
			recordVariant[variant.ConcreteName] = variant
		}
	}
	var output strings.Builder
	output.WriteString("// Code generated by ./scripts/build generate. DO NOT EDIT.\n\npackage skidbladnirv1\n\n")
	output.WriteString("import (\n\t\"bytes\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n)\n\n")
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
	for _, unionName := range sortedUnionNames(spec.Unions) {
		discriminatorType := unionDiscriminatorType(spec, unionName)
		output.WriteString("\ntype " + unionName + " interface {\n\tis" + unionName + "()\n\t" + discriminatorType + "() " + discriminatorType + "\n}\n")
	}
	for _, name := range sortedRecordNames(spec.Records) {
		fields := spec.Records[name]
		if variant, exists := recordVariant[name]; exists {
			fields = variant.Fields
		}
		writeGoStruct(&output, name, fields)
	}
	for _, variant := range variants {
		if _, recordBacked := spec.Records[variant.ConcreteName]; !recordBacked {
			writeGoStruct(&output, variant.ConcreteName, variant.Fields)
		}
	}
	for _, variant := range variants {
		wireName := lowerFirst(variant.UnionName + variant.Variant + "Wire")
		output.WriteString("\nfunc (" + variant.ConcreteName + ") is" + variant.UnionName + "() {}\n")
		output.WriteString("\nfunc (" + variant.ConcreteName + ") " + variant.DiscriminatorType + "() " + variant.DiscriminatorType + " {\n")
		output.WriteString("\treturn " + variant.DiscriminatorType + goIdentifier(variant.Variant) + "\n}\n")
		output.WriteString("\ntype " + wireName + " struct {\n")
		writeGoField(&output, fieldSpec{Name: variant.Discriminator, Type: variant.DiscriminatorType})
		for _, field := range variant.Fields {
			writeGoField(&output, field)
		}
		output.WriteString("}\n")
		output.WriteString("\nfunc (value " + variant.ConcreteName + ") MarshalJSON() ([]byte, error) {\n")
		output.WriteString("\treturn json.Marshal(" + wireName + "{\n")
		output.WriteString("\t\t" + goIdentifier(variant.Discriminator) + ": " + variant.DiscriminatorType + goIdentifier(variant.Variant) + ",\n")
		for _, field := range variant.Fields {
			fieldName := goIdentifier(field.Name)
			output.WriteString("\t\t" + fieldName + ": value." + fieldName + ",\n")
		}
		output.WriteString("\t})\n}\n")
	}
	for _, unionName := range sortedUnionNames(spec.Unions) {
		union := spec.Unions[unionName]
		discriminatorType := unionDiscriminatorType(spec, unionName)
		output.WriteString("\nfunc Decode" + unionName + "(content []byte) (" + unionName + ", error) {\n")
		output.WriteString("\tvar envelope struct {\n")
		output.WriteString("\t\t" + goIdentifier(union.Discriminator) + " " + discriminatorType + " `json:\"" + union.Discriminator + "\"`\n\t}\n")
		output.WriteString("\tif err := json.Unmarshal(content, &envelope); err != nil {\n\t\treturn nil, err\n\t}\n")
		output.WriteString("\tswitch envelope." + goIdentifier(union.Discriminator) + " {\n")
		for _, variant := range variants {
			if variant.UnionName != unionName {
				continue
			}
			wireName := lowerFirst(variant.UnionName + variant.Variant + "Wire")
			output.WriteString("\tcase " + discriminatorType + goIdentifier(variant.Variant) + ":\n")
			output.WriteString("\t\tvar wire " + wireName + "\n")
			output.WriteString("\t\tif err := decodeStrict(content, &wire); err != nil {\n\t\t\treturn nil, err\n\t\t}\n")
			output.WriteString("\t\treturn " + variant.ConcreteName + "{\n")
			for _, field := range variant.Fields {
				fieldName := goIdentifier(field.Name)
				output.WriteString("\t\t\t" + fieldName + ": wire." + fieldName + ",\n")
			}
			output.WriteString("\t\t}, nil\n")
		}
		output.WriteString("\tdefault:\n\t\treturn nil, fmt.Errorf(\"unknown " + unionName + " discriminator %q\", envelope." + goIdentifier(union.Discriminator) + ")\n\t}\n}\n")
	}
	output.WriteString("\nfunc ErrorHTTPStatus(code ErrorCode) (int, bool) {\n\tswitch code {\n")
	for _, code := range sortedIntNames(spec.Errors) {
		output.WriteString("\tcase ErrorCode" + goIdentifier(code) + ":\n\t\treturn " + strconv.Itoa(spec.Errors[code]) + ", true\n")
	}
	output.WriteString("\tdefault:\n\t\treturn 0, false\n\t}\n}\n")
	output.WriteString("\nfunc decodeStrict(content []byte, target any) error {\n\tdecoder := json.NewDecoder(bytes.NewReader(content))\n\tdecoder.DisallowUnknownFields()\n\tif err := decoder.Decode(target); err != nil {\n\t\treturn err\n\t}\n\tif decoder.Decode(&struct{}{}) != io.EOF {\n\t\treturn errors.New(\"multiple JSON values\")\n\t}\n\treturn nil\n}\n")
	formatted, err := format.Source([]byte(output.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated Go: %w", err)
	}
	return formatted, nil
}

func writeGoStruct(output *strings.Builder, name string, fields []fieldSpec) {
	output.WriteString("\ntype " + name + " struct {\n")
	for _, field := range fields {
		writeGoField(output, field)
	}
	output.WriteString("}\n")
}

func writeGoField(output *strings.Builder, field fieldSpec) {
	tag := field.Name
	if strings.HasPrefix(field.Type, "?") {
		tag += ",omitempty"
	}
	output.WriteString("\t" + goIdentifier(field.Name) + " " + goType(field.Type) + " `json:\"" + tag + "\"`\n")
}

func renderKotlin(spec apiContract, digest string) []byte {
	variants, err := renderedUnionVariants(spec)
	if err != nil {
		panic(err) // justify-defect: validateContract has already accepted this committed union shape.
	}
	recordVariant := map[string]renderedUnionVariant{}
	for _, variant := range variants {
		if _, exists := spec.Records[variant.ConcreteName]; exists {
			recordVariant[variant.ConcreteName] = variant
		}
	}
	var output strings.Builder
	output.WriteString("// Code generated by ./scripts/build generate. DO NOT EDIT.\npackage dev.niels.skidbladnir.generated\n\n")
	output.WriteString("import kotlinx.serialization.ExperimentalSerializationApi\nimport kotlinx.serialization.SerialName\nimport kotlinx.serialization.Serializable\nimport kotlinx.serialization.json.JsonClassDiscriminator\n\n")
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
		output.WriteString("\n@OptIn(ExperimentalSerializationApi::class) // justify-override: kotlinx serialization requires explicit opt-in for a custom class discriminator.\n@Serializable\n@JsonClassDiscriminator(" + strconv.Quote(spec.Unions[name].Discriminator) + ")\nsealed interface " + name + "\n")
	}
	for _, name := range sortedRecordNames(spec.Records) {
		fields := spec.Records[name]
		if variant, exists := recordVariant[name]; exists {
			fields = variant.Fields
			output.WriteString("\n@Serializable\n@SerialName(" + strconv.Quote(variant.Variant) + ")\n")
			if len(fields) == 0 {
				output.WriteString("data object " + name + " : " + variant.UnionName + "\n")
			} else {
				output.WriteString("data class " + name + "(\n")
				writeKotlinFields(&output, fields)
				output.WriteString(") : " + variant.UnionName + "\n")
			}
			continue
		}
		output.WriteString("\n@Serializable\ndata class " + name + "(\n")
		writeKotlinFields(&output, fields)
		output.WriteString(")\n")
	}
	for _, variant := range variants {
		if _, recordBacked := spec.Records[variant.ConcreteName]; recordBacked {
			continue
		}
		output.WriteString("\n@Serializable\n@SerialName(" + strconv.Quote(variant.Variant) + ")\ndata class " + variant.ConcreteName + "(\n")
		writeKotlinFields(&output, variant.Fields)
		output.WriteString(") : " + variant.UnionName + "\n")
	}
	output.WriteString("\nfun errorHttpStatus(code: ErrorCode): Int = when (code) {\n")
	for _, code := range sortedIntNames(spec.Errors) {
		output.WriteString("    ErrorCode." + kotlinIdentifier(code) + " -> " + strconv.Itoa(spec.Errors[code]) + "\n")
	}
	output.WriteString("}\n")
	return []byte(output.String())
}

func writeKotlinFields(output *strings.Builder, fields []fieldSpec) {
	for index, field := range fields {
		output.WriteString("    val " + field.Name + ": " + kotlinType(field.Type))
		if index+1 != len(fields) {
			output.WriteString(",")
		}
		output.WriteString("\n")
	}
}

func lowerFirst(value string) string {
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
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
	name, found := findUnionDiscriminatorType(spec, unionName)
	if found {
		return name
	}
	panic("union discriminator has no exact enum") // justify-defect: contract validation requires every public union to have a closed discriminator enum.
}

func findUnionDiscriminatorType(spec apiContract, unionName string) (string, bool) {
	variants := mapKeysRaw(spec.Unions[unionName].Variants)
	for enumName, values := range spec.Enums {
		if equalStringSets(values, variants) {
			return enumName, true
		}
	}
	return "", false
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
