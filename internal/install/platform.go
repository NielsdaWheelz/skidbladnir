package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

func (i *installer) platform(ctx context.Context) error {
	if _, e := archivePlatform(); e != nil {
		return e
	}
	if runtime.GOOS == "darwin" {
		b, e := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
		if e != nil {
			return e
		}
		major, e := strconv.Atoi(strings.Split(strings.TrimSpace(string(b)), ".")[0])
		if e != nil || major < 26 {
			return errors.New("macos 26 or later is required")
		}
		return nil
	}
	values, e := linuxRelease()
	if e != nil {
		return e
	}
	switch linuxFamily(values) {
	case "ubuntu":
		if values["VERSION_ID"] < "24.04" {
			return errors.New("ubuntu 24.04 or later is required")
		}
	case "arch":
	default:
		return errors.New("linux installation supports ubuntu 24.04+ and arch with systemd")
	}
	if _, e = exec.LookPath("systemctl"); e != nil {
		return errors.New("systemd user services are required")
	}
	return nil
}
func linuxRelease() (map[string]string, error) {
	b, e := os.ReadFile("/etc/os-release")
	if e != nil {
		return nil, e
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = strings.Trim(value, "\"'")
		}
	}
	return values, nil
}

func linuxFamily(values map[string]string) string {
	family := values["ID"]
	if family != "ubuntu" && slices.Contains(strings.Fields(values["ID_LIKE"]), "arch") {
		family = "arch"
	}
	return family
}

// TailscalePath prefers the user's CLI and also supports the official mac app,
// whose installer does not promise a separate command on PATH.
func TailscalePath() (string, error) {
	if path, e := exec.LookPath("tailscale"); e == nil {
		return path, nil
	}
	if runtime.GOOS == "darwin" {
		if path, e := exec.LookPath("/Applications/Tailscale.app/Contents/MacOS/Tailscale"); e == nil {
			return path, nil
		}
	}
	return "", errors.New("tailscale is unavailable; run skid install")
}

func (i *installer) dependencies(ctx context.Context) error {
	missing := []string{}
	for _, name := range []string{"tmux", "tailscale"} {
		var e error
		if name == "tailscale" {
			_, e = TailscalePath()
		} else {
			_, e = exec.LookPath(name)
		}
		if e != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	fmt.Fprintln(i.stdout, "installing missing shared dependencies; native package tools may request administrator permission")
	if runtime.GOOS == "darwin" {
		if _, e := exec.LookPath("brew"); e != nil {
			return errors.New("homebrew is required to install tmux; install it from https://brew.sh, then rerun skid install")
		}
		for _, name := range missing {
			if name == "tmux" {
				if e := command(ctx, i.stdout, i.stderr, "brew", "install", "tmux"); e != nil {
					return e
				}
			} else {
				if e := command(ctx, i.stdout, i.stderr, "brew", "install", "--cask", "tailscale-app"); e != nil {
					return e
				}
				fmt.Fprintln(i.stdout, "tailscale installed; approve its system extension and sign in when remote access is needed")
			}
		}
	} else {
		values, e := linuxRelease()
		if e != nil {
			return e
		}
		if linuxFamily(values) == "arch" {
			args := append([]string{"pacman", "-S", "--needed"}, missing...)
			if e = command(ctx, i.stdout, i.stderr, "sudo", args...); e != nil {
				return e
			}
		} else {
			for _, name := range missing {
				if name == "tailscale" {
					// The official installer configures the distribution-specific signed apt repository.
					scratch, e := os.MkdirTemp("", "skid-tailscale-")
					if e != nil {
						return e
					}
					path := filepath.Join(scratch, "install.sh")
					e = download(ctx, "https://tailscale.com/install.sh", path)
					if e == nil {
						e = command(ctx, i.stdout, i.stderr, "sh", path)
					}
					removeErr := os.RemoveAll(scratch)
					if e = errors.Join(e, removeErr); e != nil {
						return e
					}
				} else {
					if e = command(ctx, i.stdout, i.stderr, "sudo", "apt-get", "update"); e != nil {
						return e
					}
					if e = command(ctx, i.stdout, i.stderr, "sudo", "apt-get", "install", "-y", "tmux"); e != nil {
						return e
					}
				}
			}
		}
	}
	if _, e := exec.LookPath("tmux"); e != nil {
		return errors.New("tmux installed but absent from PATH; reopen your login shell and rerun skid install")
	}
	return nil
}

func (i *installer) shellStartup(ctx context.Context) error {
	home := i.dirs.Home
	// zsh itself resolves ZDOTDIR after the user's existing zshenv. No startup files are replaced.
	zdir := os.Getenv("ZDOTDIR")
	if zdir == "" {
		zdir = home
	}
	if zsh, e := exec.LookPath("zsh"); e == nil {
		cmd := exec.CommandContext(ctx, zsh, "-c", `printf '\000%s\000' "${ZDOTDIR:-$HOME}"`)
		output, e := cmd.Output()
		if e != nil {
			return errors.New("cannot resolve zsh startup directory")
		}
		parts := strings.Split(string(output), "\x00")
		if len(parts) != 3 || !filepath.IsAbs(parts[1]) {
			return errors.New("zsh startup directory is not absolute")
		}
		zdir = parts[1]
	}
	paths := []string{filepath.Join(home, ".bashrc")}
	login := filepath.Join(home, ".bash_profile")
	for _, candidate := range []string{".bash_profile", ".bash_login", ".profile"} {
		path := filepath.Join(home, candidate)
		if _, e := os.Stat(path); e == nil {
			login = path
			break
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	paths = append(paths, login, filepath.Join(zdir, ".zshrc"), filepath.Join(zdir, ".zlogin"))
	source := filepath.Join(i.dirs.Data, "current", "providers", "shell-init")
	text := "\n# >>> skidbladnir managed shell >>>\nif [ \"${SKIDBLADNIR_SHELL:-}\" = 1 ] && [ -r " + shellQuote(source) + " ]; then\n  . " + shellQuote(source) + "\nfi\n# <<< skidbladnir managed shell <<<\n"
	type startupBlock struct {
		path, text string
		first      bool
	}
	blocks := make([]startupBlock, 0, len(paths)+1)
	for _, path := range paths {
		blocks = append(blocks, startupBlock{path: path, text: text})
	}
	// The documented p10k cache preamble can redirect the first command's tty.
	// Disable it before user startup code, only in the exact shell skid created.
	promptText := `
# >>> skidbladnir managed zsh prompt >>>
if [[ "${SKIDBLADNIR_SHELL:-}" == 1 && "${SKIDBLADNIR_STARTUP_PID:-}" == "$$" && "${ZSH_SUBSHELL:-0}" == 0 ]]; then
  typeset -g POWERLEVEL9K_INSTANT_PROMPT=off
fi
# <<< skidbladnir managed zsh prompt <<<
`
	blocks = append(blocks, startupBlock{path: filepath.Join(zdir, ".zshrc"), text: promptText, first: true})
	for _, entry := range blocks {
		path, text := entry.path, entry.text
		marker := shellMarker(text)
		found := false
		createdBefore := false
		for _, block := range i.receipt.Blocks {
			if block.Path != path {
				continue
			}
			createdBefore = createdBefore || block.Created
			if shellMarker(block.Text) != marker {
				continue
			}
			if block.Text != text {
				return errors.New("shell integration changed; remove recorded installation before reinstalling")
			}
			found = true
		}
		b, e := os.ReadFile(path)
		created := errors.Is(e, os.ErrNotExist) || createdBefore
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if found && strings.Count(string(b), text) == 1 {
			atBoundary := strings.HasSuffix(string(b), text)
			if entry.first {
				atBoundary = strings.HasPrefix(string(b), text)
			}
			if atBoundary {
				continue
			}
			b = []byte(strings.Replace(string(b), text, "", 1))
		}
		if strings.Contains(string(b), marker) {
			return fmt.Errorf("unrecorded or modified shell integration: %s", path)
		}
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		if !found {
			i.receipt.Blocks = append(i.receipt.Blocks, shellBlock{Path: path, Text: text, Created: created})
		}
		if e = i.save(); e != nil {
			return e
		}
		updated := append(b, []byte(text)...)
		if entry.first {
			updated = append([]byte(text), b...)
		}
		if e = writeShellFile(path, updated); e != nil {
			return e
		}
	}
	return nil
}
func removeBlock(block shellBlock) error {
	b, e := os.ReadFile(block.Path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	count := strings.Count(string(b), block.Text)
	if count == 0 && !strings.Contains(string(b), shellMarker(block.Text)) {
		return nil
	}
	if count != 1 {
		return fmt.Errorf("shell integration changed; preserve and resolve manually: %s", block.Path)
	}
	updated := strings.Replace(string(b), block.Text, "", 1)
	if block.Created && updated == "" {
		info, e := os.Lstat(block.Path)
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			return os.Remove(block.Path)
		}
	}
	return writeShellFile(block.Path, []byte(updated))
}

// Resolve the existing link graph, then replace its target atomically. Shell
// startup content may contain unrelated secrets and must never be truncated.
func writeShellFile(path string, data []byte) error {
	mode := os.FileMode(0644)
	if info, e := os.Stat(path); e == nil {
		mode = info.Mode().Perm()
		resolved, e := filepath.EvalSymlinks(path)
		if e != nil {
			return e
		}
		path = resolved
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".skid-shell-")
	if e != nil {
		return e
	}
	temporary := f.Name()
	_, e = f.Write(data)
	if e == nil {
		e = f.Chmod(mode)
	}
	if e == nil {
		e = f.Sync()
	}
	e = errors.Join(e, f.Close())
	if e == nil {
		e = os.Rename(temporary, path)
	}
	if e != nil {
		return errors.Join(e, os.Remove(temporary))
	}
	return nil
}

func shellMarker(text string) string {
	return strings.SplitN(strings.TrimPrefix(text, "\n"), "\n", 2)[0]
}
