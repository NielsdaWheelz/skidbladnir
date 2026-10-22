package install

import (
	"context"
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
	"time"
)

type service struct{ path, text string }

func xmlText(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
func systemdQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `$$`).Replace(value) + `"`
}

// services are this host's fixed skid-owned service definitions: one systemd user
// unit on linux, the gateway and notification launch agents on mac.
func (i *installer) services() []service {
	current := filepath.Join(i.dirs.Data, "current")
	environment := map[string]string{"PATH": os.Getenv("PATH"), "XDG_CONFIG_HOME": filepath.Dir(i.dirs.Config), "XDG_DATA_HOME": filepath.Dir(i.dirs.Data), "XDG_STATE_HOME": filepath.Dir(i.dirs.State)}
	if runtime.GOOS == "linux" {
		unit := "[Unit]\nDescription=skid local host\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=" + systemdQuote(filepath.Join(current, "skidbladnir")) + " serve\nRestart=on-failure\nRestartSec=3\nKillMode=process\n"
		for _, key := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
			unit += "Environment=" + systemdQuote(key+"="+environment[key]) + "\n"
		}
		unit += "\n[Install]\nWantedBy=default.target\n"
		return []service{{filepath.Join(filepath.Dir(i.dirs.Config), "systemd", "user", "skidbladnir.service"), unit}}
	}
	var services []service
	for _, agent := range []struct {
		name string
		args []string
	}{{"dev.niels.skidbladnir", []string{filepath.Join(current, "skidbladnir"), "serve"}}, {"dev.niels.skidbladnir.notifications", []string{"/usr/bin/open", "-g", "-W", "-a", filepath.Join(current, "Skid.app"), "--env", "XDG_STATE_HOME=" + filepath.Dir(i.dirs.State), "--env", "XDG_DATA_HOME=" + filepath.Dir(i.dirs.Data), "--env", "XDG_CONFIG_HOME=" + filepath.Dir(i.dirs.Config), "--env", "PATH=" + environment["PATH"], "--args", "--config", i.dirs.State, "--skid", filepath.Join(current, "skidbladnir")}}} {
		text := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>` + xmlText(agent.name) + `</string><key>ProgramArguments</key><array>`
		for _, arg := range agent.args {
			text += "<string>" + xmlText(arg) + "</string>"
		}
		text += `</array><key>RunAtLoad</key><true/><key>KeepAlive</key><true/>`
		if agent.name == "dev.niels.skidbladnir" {
			// Without Interactive, launchd runs serve, tmux and every agent at priority 20.
			text += `<key>AbandonProcessGroup</key><true/><key>ProcessType</key><string>Interactive</string>`
		}
		text += `<key>EnvironmentVariables</key><dict>`
		for _, key := range []string{"PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
			text += "<key>" + key + "</key><string>" + xmlText(environment[key]) + "</string>"
		}
		text += "</dict></dict></plist>\n"
		services = append(services, service{filepath.Join(i.dirs.Home, "Library", "LaunchAgents", agent.name+".plist"), text})
	}
	return services
}

// stopServices stops whichever skid services are loaded; on a fresh host every
// probe finds nothing. On mac the running app is found through current's
// physical bundle, so no release tree or current may move before this.
func (i *installer) stopServices(ctx context.Context) error {
	if runtime.GOOS == "linux" {
		state, err := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=LoadState", "--value", "skidbladnir.service").Output()
		if strings.TrimSpace(string(state)) == "not-found" {
			return nil
		}
		if err != nil {
			return err
		}
		if err = command(ctx, i.stdout, i.stderr, "systemctl", "--user", "stop", "skidbladnir.service"); err != nil {
			return err
		}
		unit := i.services()[0].path
		if _, err = os.Stat(unit); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		return command(ctx, i.stdout, i.stderr, "systemctl", "--user", "disable", filepath.Base(unit))
	}
	for _, s := range i.services() {
		target := "gui/" + strconv.Itoa(os.Getuid()) + "/" + strings.TrimSuffix(filepath.Base(s.path), ".plist")
		probe := command(ctx, io.Discard, io.Discard, "launchctl", "print", target)
		var exit *exec.ExitError
		if probe == nil {
			if err := command(ctx, i.stdout, i.stderr, "launchctl", "bootout", target); err != nil {
				return err
			}
		} else if !errors.As(probe, &exit) || exit.ExitCode() != 113 {
			return fmt.Errorf("inspect skid service: %w", probe)
		}
	}
	app, err := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "Skid.app"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return command(ctx, i.stdout, i.stderr, filepath.Join(app, "Contents", "MacOS", "skid-notifications"), "stop", app)
}

func (i *installer) startServices(ctx context.Context) error {
	if runtime.GOOS == "linux" {
		if err := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		if err := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "enable", "skidbladnir.service"); err != nil {
			return err
		}
		if err := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "restart", "skidbladnir.service"); err != nil {
			return err
		}
		fmt.Fprintln(i.stdout, "service registered; for startup before login, run: sudo loginctl enable-linger "+shellQuote(os.Getenv("USER")))
		return nil
	}
	app, err := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "Skid.app"))
	if err != nil {
		return err
	}
	if err = command(ctx, i.stdout, i.stderr, filepath.Join(app, "Contents", "MacOS", "skid-notifications"), "register", app); err != nil {
		return err
	}
	// The previous serve, which ran the same command, may still be exiting after
	// bootout; only a serve absent from this table is the one bootstrap starts.
	// A failed read is returned after bootstrap, so the check never leaves the
	// jobs unloaded.
	serve := filepath.Join(i.dirs.Data, "current", "skidbladnir") + " serve"
	serves := func() (map[int]int, error) {
		table, err := exec.CommandContext(ctx, "ps", "-xww", "-o", "pid=,ppid=,pri=,command=").Output()
		if err != nil {
			return nil, fmt.Errorf("list processes: %w", err)
		}
		return launchdChildren(table, serve)
	}
	previous, listErr := serves()
	domain := "gui/" + strconv.Itoa(os.Getuid())
	// stopServices booted every agent out under the same install lock.
	for _, s := range i.services() {
		if err = command(ctx, i.stdout, i.stderr, "launchctl", "bootstrap", domain, s.path); err != nil {
			return err
		}
	}
	if listErr != nil {
		return listErr
	}
	// Every agent inherits serve's priority through tmux. Poll every 100 ms, up
	// to 5 s, until the new serve appears; RunAtLoad spawns it at bootstrap.
	for attempt := 0; attempt < 50; attempt++ {
		time.Sleep(100 * time.Millisecond)
		current, err := serves()
		if err != nil {
			return err
		}
		for pid, priority := range current {
			if _, old := previous[pid]; old {
				continue
			}
			if priority < 31 {
				return fmt.Errorf("skid serve (pid %d) runs at scheduling priority %d, below interactive 31: check that `launchctl print %s/dev.niels.skidbladnir` reports an interactive spawn type, then reinstall", pid, priority, domain)
			}
			fmt.Fprintf(i.stdout, "skid serve (pid %d) runs at scheduling priority %d\n", pid, priority)
			return nil
		}
	}
	return fmt.Errorf("skid serve did not appear within 5 s of bootstrap, so its priority is unchecked: inspect `launchctl print %s/dev.niels.skidbladnir`", domain)
}

// launchdChildren maps each process whose parent is launchd (pid 1) and whose
// argv is exactly command to its scheduling priority. table is `ps -xww -o
// pid=,ppid=,pri=,command=` output, whose command column joins argv with spaces.
func launchdChildren(table []byte, command string) (map[int]int, error) {
	processes := map[int]int{}
	for _, line := range strings.Split(string(table), "\n") {
		pid, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
		parent, rest, _ := strings.Cut(strings.TrimSpace(rest), " ")
		priority, rest, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if parent != "1" || strings.TrimSpace(rest) != command {
			continue
		}
		id, idErr := strconv.Atoi(pid)
		level, levelErr := strconv.Atoi(priority)
		if idErr != nil || levelErr != nil {
			return nil, fmt.Errorf("unreadable process table row for %s", command)
		}
		processes[id] = level
	}
	return processes, nil
}
