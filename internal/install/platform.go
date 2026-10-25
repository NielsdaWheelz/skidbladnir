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

func platform(ctx context.Context) error {
	if host := runtime.GOOS + "-" + runtime.GOARCH; host != "darwin-arm64" && host != "linux-amd64" {
		return errors.New("supported hosts: macos 26+ arm64, ubuntu 24.04+ amd64, arch amd64 with systemd")
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
			// pacman reads its confirmation from stdin, which under curl | bash is
			// the spent script, so it asks on the terminal instead.
			args := append([]string{"pacman", "-S", "--needed"}, missing...)
			tty, e := os.Open("/dev/tty")
			if e != nil {
				return errors.New("pacman needs a terminal to confirm; run: sudo " + strings.Join(args, " ") + ", then rerun skid install")
			}
			pacman := exec.CommandContext(ctx, "sudo", args...)
			pacman.Stdin, pacman.Stdout, pacman.Stderr = tty, i.stdout, i.stderr
			if e = errors.Join(pacman.Run(), tty.Close()); e != nil {
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
