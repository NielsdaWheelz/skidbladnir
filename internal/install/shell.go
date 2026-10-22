package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// skid owns two marker-delimited startup blocks. dev-server's dotfiles apply
// requires the prompt block at byte 0 of .zshrc and the shell block at its end,
// each exactly once.

func (i *installer) addShellBlocks(ctx context.Context) error {
	home := i.dirs.Home
	zdir, err := zshDirectory(ctx, home)
	if err != nil {
		return err
	}
	login := filepath.Join(home, ".bash_profile")
	for _, candidate := range []string{".bash_profile", ".bash_login", ".profile"} {
		path := filepath.Join(home, candidate)
		if _, err := os.Stat(path); err == nil {
			login = path
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	source := filepath.Join(i.dirs.Data, "current", "providers", "shell-init")
	shell := "\n# >>> skidbladnir managed shell >>>\nif [ \"${SKIDBLADNIR_SHELL:-}\" = 1 ] && [ -r " + shellQuote(source) + " ]; then\n  . " + shellQuote(source) + "\nfi\n# <<< skidbladnir managed shell <<<\n"
	// The documented p10k cache preamble can redirect the first command's tty.
	// Disable it before user startup code, only in the exact shell skid created.
	prompt := `
# >>> skidbladnir managed zsh prompt >>>
if [[ "${SKIDBLADNIR_SHELL:-}" == 1 && "${SKIDBLADNIR_STARTUP_PID:-}" == "$$" && "${ZSH_SUBSHELL:-0}" == 0 ]]; then
  typeset -g POWERLEVEL9K_INSTANT_PROMPT=off
fi
# <<< skidbladnir managed zsh prompt <<<
`
	for _, path := range []string{filepath.Join(home, ".bashrc"), login, filepath.Join(zdir, ".zlogin")} {
		if err := rewriteStartup(path, "", shell); err != nil {
			return err
		}
	}
	// Last, so a startup file linked to .zshrc ends with the prompt block too.
	return rewriteStartup(filepath.Join(zdir, ".zshrc"), prompt, shell)
}

// Install records nothing, so removal strips every file install could have chosen
// under the zsh directory resolved now; blocks under an earlier ZDOTDIR stay.
func removeShellBlocks(home, zdir string) error {
	for _, path := range []string{
		filepath.Join(zdir, ".zshrc"), filepath.Join(zdir, ".zlogin"),
		filepath.Join(home, ".bashrc"), filepath.Join(home, ".bash_profile"), filepath.Join(home, ".bash_login"), filepath.Join(home, ".profile"),
		filepath.Join(home, ".zshrc"), filepath.Join(home, ".zlogin"),
	} {
		if err := rewriteStartup(path, "", ""); err != nil {
			return err
		}
	}
	return nil
}

// zsh itself resolves ZDOTDIR after the user's .zshenv.
func zshDirectory(ctx context.Context, home string) (string, error) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		if zdir := os.Getenv("ZDOTDIR"); zdir != "" {
			return zdir, nil
		}
		return home, nil
	}
	output, err := exec.CommandContext(ctx, zsh, "-c", `printf '\000%s\000' "${ZDOTDIR:-$HOME}"`).Output()
	if err != nil {
		return "", errors.New("cannot resolve zsh startup directory")
	}
	parts := strings.Split(string(output), "\x00")
	if len(parts) != 3 || !filepath.IsAbs(parts[1]) {
		return "", errors.New("zsh startup directory is not absolute")
	}
	return parts[1], nil
}

// rewriteStartup removes skid's blocks from one startup file and wraps what is
// left in prefix and suffix. It writes only changed bytes, through symlinks,
// keeping the mode; a regular file left empty is deleted. Startup files may hold
// secrets: replace them atomically, never truncate.
func rewriteStartup(path, prefix, suffix string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(data)
	for _, name := range []string{"zsh prompt", "shell"} {
		begin, end := "# >>> skidbladnir managed "+name+" >>>", "# <<< skidbladnir managed "+name+" <<<"
		b, e := strings.Index(text, begin), strings.Index(text, end)
		if b < 0 && e < 0 {
			continue
		}
		if strings.Count(text, begin) != 1 || strings.Count(text, end) != 1 || e < b {
			return fmt.Errorf("skid shell block is incomplete or duplicated: %s", path)
		}
		start, stop := b, e+len(end)
		if start > 0 && text[start-1] == '\n' {
			start--
		}
		if stop < len(text) && text[stop] == '\n' {
			stop++
		}
		head, tail := text[:start], text[stop:]
		// The block's leading newline can be the only one between two user lines.
		if head != "" && tail != "" && !strings.HasSuffix(head, "\n") {
			head += "\n"
		}
		text = head + tail
	}
	text = prefix + text + suffix
	if text == string(data) {
		return nil
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
		if text == "" {
			if link, err := os.Lstat(path); err != nil {
				return err
			} else if link.Mode().IsRegular() {
				return os.Remove(path)
			}
		}
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	} else {
		return err
	}
	return writeAtomic(path, []byte(text), mode)
}
