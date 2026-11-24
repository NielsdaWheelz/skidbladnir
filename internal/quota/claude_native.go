package quota

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/claudeusage"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

// Each binary completed the builtin against an existing login, without model
// usage or a saved session, and advanced the same account's native cache.
var qualifiedClaudeBuilds = map[string]string{
	"darwin/arm64/2.1.331": "4874591dc39bff1188921b82cbeb8a44e3a33b0f92c888b3d43639b9344101f7",
	"linux/amd64/2.1.325":  "0dc4f4a9a165fc60746d7ae8489712e5fc352f1e7ad4eb22e7141bd0812eb0cf",
}

var (
	errClaudeAuth      = errors.New("claude quota login unverified")
	errClaudeFailed    = errors.New("claude quota command failed")
	errClaudeUnchanged = errors.New("claude quota cache unchanged")
	errClaudeDispatch  = errors.New("claude quota unexpected dispatch")
	errClaudeBlock     = errors.New("claude quota dispatch block unavailable")
)

func qualifiedClaude(home string) (target, digest string) {
	target, err := filepath.EvalSymlinks(filepath.Join(home, ".local", "bin", "claude"))
	if err != nil {
		return "", ""
	}
	versions, err := filepath.EvalSymlinks(filepath.Join(home, ".local", "share", "claude", "versions"))
	if err != nil || filepath.Dir(target) != versions {
		return "", ""
	}
	digest = qualifiedClaudeBuilds[runtime.GOOS+"/"+runtime.GOARCH+"/"+filepath.Base(target)]
	if digest == "" {
		return "", ""
	}
	file, err := os.OpenFile(target, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", ""
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil || hex.EncodeToString(hash.Sum(nil)) != digest {
		return "", ""
	}
	return target, digest
}

// Neither safe mode nor restricted mode suppresses executable managed settings.
// Unknown or unreadable policy keeps this account cache-only.
func claudePolicyAbsent(dir, username string) bool {
	admin := "/etc/claude-code"
	paths := []string{filepath.Join(dir, "remote-settings.json"), filepath.Join(dir, "state", "hybrid-settings.json")}
	if runtime.GOOS == "darwin" {
		admin = "/Library/Application Support/ClaudeCode"
		const prefs = "/Library/Managed Preferences"
		const domain = "com.anthropic.claudecode.plist"
		paths = append(paths, filepath.Join(prefs, domain), filepath.Join(prefs, username, domain))
	}
	paths = append(paths, filepath.Join(admin, "managed-settings.json"), filepath.Join(admin, "managed-mcp.json"))
	for _, p := range paths {
		if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	entries, err := os.ReadDir(filepath.Join(admin, "managed-settings.d"))
	if err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") && !strings.HasPrefix(entry.Name(), ".") {
			return false
		}
	}
	return true
}

func (s ClaudeSample) sameLogin(other ClaudeSample) bool {
	return s.ReadState == "ok" && other.ReadState == "ok" && s.Reading.Account == other.Reading.Account && s.organization == other.organization && s.path == other.path
}

// A fresh sample satisfies demand. A zero start otherwise permits the next home.
func refreshClaude(parent context.Context, stateDir string, profile agentruntime.Profile, expected ClaudeSample, current func() bool) (started time.Time, fresh bool, err error) {
	home, err := os.UserHomeDir()
	if err != nil || !expected.personal || expected.organization == "" {
		return started, false, nil
	}
	info, err := os.Stat(profile.Home)
	if err != nil || !info.IsDir() {
		return started, false, nil
	}
	owner, err := user.Current()
	if err != nil || !claudePolicyAbsent(profile.Home, owner.Username) {
		return started, false, nil
	}
	target, digest := qualifiedClaude(home)
	if target == "" {
		return started, false, nil
	}
	block := filepath.Join(claudeusage.Dir(stateDir), "native-block", digest)
	if _, err := os.Lstat(block); !errors.Is(err, os.ErrNotExist) {
		return started, false, nil
	}
	env := []string{"HOME=" + home, "USER=" + owner.Username, "PATH=/usr/bin:/bin", "DISABLE_AUTOUPDATER=1"}
	if profile.ClaudeConfigDir != "" {
		env = append(env, "CLAUDE_CONFIG_DIR="+profile.ClaudeConfigDir)
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	before := ReadClaudeProfile(profile)
	if !expected.sameLogin(before) || !before.personal || !current() {
		return started, false, nil
	}
	auth, err := claudeCommand(ctx, target, env, "--safe-mode", "--restricted", "auth", "status")
	var login struct {
		LoggedIn         bool      `json:"loggedIn"`
		AuthMethod       string    `json:"authMethod"`
		APIProvider      string    `json:"apiProvider"`
		SubscriptionType string    `json:"subscriptionType"`
		ConfigDirectory  string    `json:"configDirectory"`
		OrgID            string    `json:"orgId"`
		LoginExpiresAt   time.Time `json:"loginExpiresAt"`
	}
	dir := profile.ClaudeConfigDir
	if dir == "" {
		dir = filepath.Join(home, ".claude")
	}
	if err != nil || json.Unmarshal(auth, &login) != nil ||
		!login.LoggedIn || login.AuthMethod != "claude.ai" || login.APIProvider != "firstParty" ||
		(login.SubscriptionType != "max" && login.SubscriptionType != "pro") ||
		login.ConfigDirectory != dir || login.OrgID != expected.organization || !login.LoginExpiresAt.After(time.Now()) {
		return started, false, errClaudeAuth
	}
	before = ReadClaudeProfile(profile)
	if !expected.sameLogin(before) || !before.personal || !claudePolicyAbsent(profile.Home, owner.Username) || !current() || ctx.Err() != nil {
		return started, false, nil
	}
	if _, err := os.Lstat(block); !errors.Is(err, os.ErrNotExist) {
		return started, false, nil
	}
	if time.Since(before.FetchedAt) < time.Minute {
		return started, true, nil
	}
	started = time.Now()
	out, runErr := claudeCommand(ctx, target, env, "--safe-mode", "--restricted", "--no-session-persistence", "--tools", "", "--output-format", "json", "-p", "/usage")
	resultErr := checkClaudeResult(out)
	if resultErr == errClaudeDispatch {
		if err := privatefile.WriteFile(block, []byte("unexpected dispatch\n")); err != nil {
			return started, false, errClaudeBlock
		}
	}
	if resultErr != nil {
		return started, false, resultErr
	}
	if runErr != nil {
		return started, false, errClaudeFailed
	}
	after := ReadClaudeProfile(profile)
	if !before.sameLogin(after) || !after.FetchedAt.After(before.FetchedAt) || !current() {
		return started, false, errClaudeUnchanged
	}
	return started, true, nil
}

// Inspect only dispatch and accounting fields. All user-facing output is discarded.
func checkClaudeResult(out []byte) error {
	var result struct {
		Type, Subtype string
		IsError       *bool                      `json:"is_error"`
		Command       string                     `json:"local_command"`
		Cost          *float64                   `json:"total_cost_usd"`
		API           *int64                     `json:"duration_api_ms"`
		Models        map[string]json.RawMessage `json:"modelUsage"`
		Usage         struct {
			Input       *int64           `json:"input_tokens"`
			Output      *int64           `json:"output_tokens"`
			CacheRead   *int64           `json:"cache_read_input_tokens"`
			CacheWrite  *int64           `json:"cache_creation_input_tokens"`
			ServerTools map[string]int64 `json:"server_tool_use"`
		}
		Denials []json.RawMessage `json:"permission_denials"`
	}
	if json.Unmarshal(out, &result) != nil || result.Type != "result" {
		return errClaudeFailed
	}
	used := len(result.Models) > 0 || len(result.Denials) > 0 || (result.Cost != nil && *result.Cost != 0) || (result.API != nil && *result.API != 0)
	complete := result.Cost != nil && result.API != nil && result.Models != nil && result.Denials != nil
	for _, n := range []*int64{result.Usage.Input, result.Usage.Output, result.Usage.CacheRead, result.Usage.CacheWrite} {
		if n == nil {
			complete = false
		} else if *n != 0 {
			used = true
		}
	}
	for _, count := range result.Usage.ServerTools {
		if count != 0 {
			used = true
		}
	}
	if used || (result.Subtype == "success" && result.Command != "usage") {
		return errClaudeDispatch
	}
	if result.Subtype != "success" || result.IsError == nil || *result.IsError || !complete {
		return errClaudeFailed
	}
	return nil
}

type claudeOutput struct{ buf bytes.Buffer }

func (out *claudeOutput) Write(p []byte) (int, error) {
	if len(p) > (1<<20)-out.buf.Len() {
		return 0, errClaudeFailed
	}
	return out.buf.Write(p)
}

// A descendant can hold stdout open or outlive a successfully exited leader.
// WaitDelay bounds the copy. Group cleanup also covers descendants after Wait.
func claudeCommand(ctx context.Context, target string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, target, args...)
	cmd.Dir = "/"
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 100 * time.Millisecond
	var out claudeOutput
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return nil, errClaudeFailed
	}
	waitErr := cmd.Wait()
	stopErr := cmd.Cancel()
	if waitErr != nil || stopErr != nil && !errors.Is(stopErr, os.ErrProcessDone) {
		return out.buf.Bytes(), errClaudeFailed
	}
	return out.buf.Bytes(), nil
}
