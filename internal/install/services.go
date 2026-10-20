package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func (i *installer) writeOwned(path, text string) error {
	if _, e := os.Lstat(path); e == nil {
		owned := false
		for _, r := range i.receipt.Resources {
			if r.Path == path {
				owned = true
				break
			}
		}
		if !owned {
			return fmt.Errorf("service path is not installer-owned: %s", path)
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	previous, e := fileHash(path)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	digest := sha256.Sum256([]byte(text))
	hash := hex.EncodeToString(digest[:])
	i.putResource(resource{Path: path, Hash: hash, PreviousHash: previous, Mode: 0644})
	if e = i.save(); e != nil {
		return e
	}
	temporary, e := os.CreateTemp(filepath.Dir(path), ".skid-service-")
	if e != nil {
		return e
	}
	temporaryPath := temporary.Name()
	_, e = temporary.WriteString(text)
	if e == nil {
		e = temporary.Chmod(0644)
	}
	if e == nil {
		e = temporary.Sync()
	}
	e = errors.Join(e, temporary.Close())
	if e == nil {
		e = os.Rename(temporaryPath, path)
	}
	if e != nil {
		return errors.Join(e, os.Remove(temporaryPath))
	}
	i.putResource(resource{Path: path, Hash: hash, Mode: 0644})
	return i.save()
}
func xmlText(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
func systemdQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `$$`).Replace(value) + `"`
}
func (i *installer) writeServices() error {
	current := filepath.Join(i.dirs.Data, "current")
	environment := map[string]string{"PATH": os.Getenv("PATH"), "XDG_CONFIG_HOME": filepath.Dir(i.dirs.Config), "XDG_DATA_HOME": filepath.Dir(i.dirs.Data), "XDG_STATE_HOME": filepath.Dir(i.dirs.State)}
	if runtime.GOOS == "linux" {
		unit := "[Unit]\nDescription=skid local host\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=" + systemdQuote(filepath.Join(current, "skidbladnir")) + " serve\nRestart=on-failure\nRestartSec=3\nKillMode=process\n"
		for _, key := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
			unit += "Environment=" + systemdQuote(key+"="+environment[key]) + "\n"
		}
		unit += "\n[Install]\nWantedBy=default.target\n"
		path := filepath.Join(i.dirs.Config, "..", "systemd", "user", "skidbladnir.service")
		if e := i.writeOwned(filepath.Clean(path), unit); e != nil {
			return e
		}
		i.receipt.Services = []string{filepath.Clean(path)}
		return nil
	}
	i.receipt.Services = nil
	for _, service := range []struct {
		name string
		args []string
	}{{"dev.niels.skidbladnir", []string{filepath.Join(current, "skidbladnir"), "serve"}}, {"dev.niels.skidbladnir.notifications", []string{"/usr/bin/open", "-g", "-W", "-a", filepath.Join(current, "Skid.app"), "--env", "XDG_STATE_HOME=" + filepath.Dir(i.dirs.State), "--env", "XDG_DATA_HOME=" + filepath.Dir(i.dirs.Data), "--env", "XDG_CONFIG_HOME=" + filepath.Dir(i.dirs.Config), "--env", "PATH=" + environment["PATH"], "--args", "--config", i.dirs.State, "--skid", filepath.Join(current, "skidbladnir")}}} {
		text := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>` + xmlText(service.name) + `</string><key>ProgramArguments</key><array>`
		for _, arg := range service.args {
			text += "<string>" + xmlText(arg) + "</string>"
		}
		text += `</array><key>RunAtLoad</key><true/><key>KeepAlive</key><true/>`
		if service.name == "dev.niels.skidbladnir" {
			text += `<key>AbandonProcessGroup</key><true/>`
		}
		text += `<key>EnvironmentVariables</key><dict>`
		for _, key := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
			text += "<key>" + key + "</key><string>" + xmlText(environment[key]) + "</string>"
		}
		text += "</dict></dict></plist>\n"
		path := filepath.Join(i.dirs.Home, "Library", "LaunchAgents", service.name+".plist")
		if e := i.writeOwned(path, text); e != nil {
			return e
		}
		i.receipt.Services = append(i.receipt.Services, path)
	}
	return nil
}
func (i *installer) startServices(ctx context.Context) error {
	if runtime.GOOS == "linux" {
		if e := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "daemon-reload"); e != nil {
			return e
		}
		if e := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "enable", "skidbladnir.service"); e != nil {
			return e
		}
		if e := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "restart", "skidbladnir.service"); e != nil {
			return e
		}
		fmt.Fprintln(i.stdout, "service registered; for startup before login, run: sudo loginctl enable-linger "+shellQuote(os.Getenv("USER")))
		return nil
	}
	app, err := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "Skid.app"))
	if err != nil {
		return err
	}
	if e := command(ctx, i.stdout, i.stderr, filepath.Join(app, "Contents", "MacOS", "skid-notifications"), "register", app); e != nil {
		return e
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	for _, path := range i.receipt.Services {
		label := strings.TrimSuffix(filepath.Base(path), ".plist")
		probe := command(ctx, io.Discard, io.Discard, "launchctl", "print", domain+"/"+label)
		if probe == nil {
			if e := command(ctx, i.stdout, i.stderr, "launchctl", "bootout", domain+"/"+label); e != nil {
				return e
			}
		} else {
			var exit *exec.ExitError
			if !errors.As(probe, &exit) || exit.ExitCode() != 113 {
				return fmt.Errorf("inspect skid service: %w", probe)
			}
		}
		if e := command(ctx, i.stdout, i.stderr, "launchctl", "bootstrap", domain, path); e != nil {
			return e
		}
	}
	return nil
}
func (i *installer) stopServices(ctx context.Context) error {
	if len(i.receipt.Services) == 0 {
		return nil
	}
	if runtime.GOOS == "linux" {
		state, e := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=LoadState", "--value", "skidbladnir.service").Output()
		if strings.TrimSpace(string(state)) == "not-found" {
			return nil
		}
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(state)) != "not-found" {
			if e = command(ctx, i.stdout, i.stderr, "systemctl", "--user", "stop", "skidbladnir.service"); e != nil {
				return e
			}
		}
		for _, path := range i.receipt.Services {
			if _, e = os.Stat(path); errors.Is(e, os.ErrNotExist) {
				continue
			} else if e != nil {
				return e
			}
			if e = command(ctx, i.stdout, i.stderr, "systemctl", "--user", "disable", filepath.Base(path)); e != nil {
				return e
			}
		}
		return nil
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	for _, path := range i.receipt.Services {
		label := strings.TrimSuffix(filepath.Base(path), ".plist")
		probe := command(ctx, io.Discard, io.Discard, "launchctl", "print", domain+"/"+label)
		if probe == nil {
			if e := command(ctx, i.stdout, i.stderr, "launchctl", "bootout", domain+"/"+label); e != nil {
				return e
			}
		} else {
			var exit *exec.ExitError
			if !errors.As(probe, &exit) || exit.ExitCode() != 113 {
				return fmt.Errorf("inspect skid service: %w", probe)
			}
		}
	}
	app, e := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "Skid.app"))
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	return command(ctx, i.stdout, i.stderr, filepath.Join(app, "Contents", "MacOS", "skid-notifications"), "stop", app)
}
