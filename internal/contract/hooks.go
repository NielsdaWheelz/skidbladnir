package contract

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	hooksConfigPath           = "deploy/codex/hooks.json"
	hooksLockPath             = "deploy/codex/hooks.lock.json"
	reviewedHelperPath        = "/home/niels/.local/bin/skidbladnir-hook"
	reviewedCodexBinaryPath   = "/home/niels/.local/lib/node_modules/@openai/codex/node_modules/@openai/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex"
	reviewedHookCommand       = reviewedHelperPath + " --pinned-codex " + reviewedCodexBinaryPath + " --socket /run/user/1000/skidbladnir/hook.sock --gap-directory /home/niels/.local/state/skidbladnir/hook-gaps"
	helperBuildTimeout        = 60 * time.Second
	reviewedHooksLockVersion  = "0.149.1"
	reviewedHooksPathFilename = "hooks.json"
)

type hooksLock struct {
	CodexVersion      string            `json:"codexVersion"`
	HooksPathFilename string            `json:"hooksPathFilename"`
	HooksSHA256       string            `json:"hooksSha256"`
	Helper            hookHelperLock    `json:"helper"`
	Profiles          []hookProfileLock `json:"profiles"`
}

type hookHelperLock struct {
	Path   string        `json:"path"`
	SHA256 string        `json:"sha256"`
	Build  hookBuildLock `json:"build"`
}

type hookBuildLock struct {
	GoVersion   string   `json:"goVersion"`
	Package     string   `json:"package"`
	Flags       []string `json:"flags"`
	Environment []string `json:"environment"`
}

type hookProfileLock struct {
	Name       string            `json:"name"`
	TargetPath string            `json:"targetPath"`
	Source     string            `json:"source"`
	TrustState map[string]string `json:"trustState"`
}

type reviewedProfile struct {
	Name       string
	TargetPath string
}

type reviewedHookEvent struct {
	ConfigName string
	Key        string
	Timeout    int
}

var reviewedProfiles = []reviewedProfile{
	{Name: "personal", TargetPath: "/home/niels/.codex-personal/hooks.json"},
	{Name: "work", TargetPath: "/home/niels/.codex-work/hooks.json"},
	{Name: "work2", TargetPath: "/home/niels/.codex-work2/hooks.json"},
}

var reviewedHookEvents = []reviewedHookEvent{
	{ConfigName: "SessionStart", Key: "session_start", Timeout: 5},
	{ConfigName: "UserPromptSubmit", Key: "user_prompt_submit", Timeout: 5},
	{ConfigName: "PostToolUse", Key: "post_tool_use", Timeout: 5},
	{ConfigName: "SubagentStart", Key: "subagent_start", Timeout: 5},
	{ConfigName: "SubagentStop", Key: "subagent_stop", Timeout: 5},
	{ConfigName: "Stop", Key: "stop", Timeout: 5},
	{ConfigName: "SessionEnd", Key: "session_end", Timeout: 1},
}

var reviewedHelperBuildEnvironment = []string{
	"CGO_ENABLED=0",
	"GOARCH=amd64",
	"GOENV=off",
	"GOEXPERIMENT=none",
	"GOFLAGS=",
	"GOOS=linux",
	"GOTOOLCHAIN=local",
	"GOAMD64=v1",
}

type hooksConfig struct {
	Hooks map[string][]hookConfigGroup `json:"hooks"`
}

type hookConfigGroup struct {
	Hooks []hookConfigHandler `json:"hooks"`
}

type hookConfigHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout *int   `json:"timeout"`
	Async   *bool  `json:"async"`
}

func verifyHookArtifacts(root string) error {
	lockBytes, err := os.ReadFile(filepath.Join(root, hooksLockPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", hooksLockPath, err)
	}
	if err := rejectDuplicateJSONKeys(lockBytes); err != nil {
		return fmt.Errorf("decode %s: %w", hooksLockPath, err)
	}
	var lock hooksLock
	if err := decodeStrict(lockBytes, &lock); err != nil {
		return fmt.Errorf("decode %s: %w", hooksLockPath, err)
	}
	hooksBytes, err := os.ReadFile(filepath.Join(root, hooksConfigPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", hooksConfigPath, err)
	}
	codexBytes, err := os.ReadFile(filepath.Join(root, codexLockPath))
	if err != nil {
		return fmt.Errorf("read %s for hook verification: %w", codexLockPath, err)
	}
	var codex codexLock
	if err := decodeStrict(codexBytes, &codex); err != nil {
		return fmt.Errorf("decode %s for hook verification: %w", codexLockPath, err)
	}
	if err := validateHookArtifacts(lock, codex, hooksBytes); err != nil {
		return err
	}
	return verifyHelperBuild(root, lock.Helper)
}

func validateHookArtifacts(lock hooksLock, codex codexLock, hooksBytes []byte) error {
	if lock.CodexVersion != reviewedHooksLockVersion || lock.CodexVersion != codex.Version || codex.BinaryPath != reviewedCodexBinaryPath {
		return errors.New("hook lock Codex identity is not exact")
	}
	if lock.HooksPathFilename != reviewedHooksPathFilename || !validSHA256(lock.HooksSHA256) || lock.HooksSHA256 != sha256Hex(hooksBytes) {
		return errors.New("hook config digest is not exact")
	}
	if lock.Helper.Path != reviewedHelperPath || !validSHA256(lock.Helper.SHA256) {
		return errors.New("hook helper identity is not exact")
	}
	if lock.Helper.Build.GoVersion != "go1.26.7" || lock.Helper.Build.Package != "./cmd/skidbladnir-hook" || !equalStrings(lock.Helper.Build.Flags, []string{"-trimpath", "-buildvcs=false"}) || !equalStrings(lock.Helper.Build.Environment, reviewedHelperBuildEnvironment) {
		return errors.New("hook helper build identity is not exact")
	}
	if err := validateHookProfiles(lock.Profiles); err != nil {
		return err
	}
	if err := validateHooksConfig(hooksBytes, reviewedHookCommand); err != nil {
		return fmt.Errorf("validate %s: %w", hooksConfigPath, err)
	}
	return nil
}

func validateHookProfiles(profiles []hookProfileLock) error {
	if len(profiles) != len(reviewedProfiles) {
		return errors.New("reviewed hook profile set is incomplete")
	}
	trustedByEvent := make(map[string]string, len(reviewedHookEvents))
	for index, expected := range reviewedProfiles {
		profile := profiles[index]
		if profile.Name != expected.Name || profile.TargetPath != expected.TargetPath || profile.Source != "user" || len(profile.TrustState) != len(reviewedHookEvents) {
			return errors.New("reviewed hook profile identity is not exact")
		}
		for _, event := range reviewedHookEvents {
			key := profile.TargetPath + ":" + event.Key + ":0:0"
			digest, ok := profile.TrustState[key]
			if !ok || !validNormalizedHookHash(digest) {
				return errors.New("reviewed hook trust set is incomplete")
			}
			if first, exists := trustedByEvent[event.Key]; exists && first != digest {
				return errors.New("reviewed hook trust digest differs by profile")
			}
			trustedByEvent[event.Key] = digest
		}
	}
	return nil
}

func validateHooksConfig(content []byte, expectedCommand string) error {
	if err := rejectDuplicateJSONKeys(content); err != nil {
		return err
	}
	var config hooksConfig
	if err := decodeStrict(content, &config); err != nil {
		return err
	}
	if len(config.Hooks) != len(reviewedHookEvents) {
		return errors.New("hook config event set is not exact")
	}
	for _, event := range reviewedHookEvents {
		groups, ok := config.Hooks[event.ConfigName]
		if !ok || len(groups) != 1 || len(groups[0].Hooks) != 1 {
			return fmt.Errorf("hook config event %s is not singular", event.ConfigName)
		}
		handler := groups[0].Hooks[0]
		if handler.Type != "command" || handler.Command != expectedCommand || handler.Timeout == nil || *handler.Timeout != event.Timeout || handler.Async == nil || *handler.Async {
			return fmt.Errorf("hook config event %s differs from the reviewed command", event.ConfigName)
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("hook artifact contains multiple JSON values")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, structured := token.(json.Delim) // justify-type-assertion: json.Decoder.Token exposes JSON container delimiters through this documented token type.
	if !structured {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string) // justify-type-assertion: json.Decoder.Token returns object keys as strings by its JSON contract.
			if !ok || keys[key] {
				return errors.New("hook artifact contains a duplicate or invalid object key")
			}
			keys[key] = true
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("hook artifact contains an invalid object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("hook artifact contains an invalid array")
		}
	default:
		return errors.New("hook artifact contains an unexpected delimiter")
	}
	return nil
}

func verifyHelperBuild(root string, helper hookHelperLock) error {
	if runtime.Version() != helper.Build.GoVersion || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return errors.New("hook helper build host differs from the reviewed toolchain")
	}
	directory, err := os.MkdirTemp("", "skidbladnir-hook-build-")
	if err != nil {
		return fmt.Errorf("create temporary hook helper directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(directory) // justify-ignore-error: the operating system owns cleanup of an unpublished temporary verification build after process exit.
	}()
	output := filepath.Join(directory, "skidbladnir-hook")
	arguments := append([]string{"build"}, helper.Build.Flags...)
	arguments = append(arguments, "-o", output, helper.Build.Package)
	ctx, cancel := context.WithTimeout(context.Background(), helperBuildTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir = root
	command.Env = helperBuildEnvironment(os.Environ(), helper.Build.Environment)
	command.Stdout = io.Discard
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("hook helper build timed out")
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return errors.New("hook helper build is not reproducible")
		}
		return fmt.Errorf("hook helper build is not reproducible: %s", message)
	}
	if err := verifyDigest(output, helper.SHA256); err != nil {
		return fmt.Errorf("rebuilt hook helper: %w", err)
	}
	return nil
}

func helperBuildEnvironment(ambient, reviewed []string) []string {
	keys := make(map[string]bool, len(reviewed))
	for _, entry := range reviewed {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			keys[key] = true
		}
	}
	environment := make([]string, 0, len(ambient)+len(reviewed))
	for _, entry := range ambient {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || keys[key] {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, reviewed...)
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validNormalizedHookHash(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validSHA256(strings.TrimPrefix(value, "sha256:"))
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
