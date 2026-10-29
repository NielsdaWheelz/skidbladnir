package install

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalservice"
)

type service struct{ name, path, text string }

func xmlText(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
func systemdQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`).Replace(value) + `"`
}
func (i *installer) environment() [][2]string {
	locale := os.Getenv("LANG")
	if locale == "" {
		locale = "en_US.UTF-8"
		if runtime.GOOS == "linux" {
			locale = "C.UTF-8"
		}
	}
	return [][2]string{{"HOME", i.dirs.Home}, {"PATH", os.Getenv("PATH")}, {"LANG", locale}, {"XDG_CONFIG_HOME", filepath.Dir(i.dirs.Config)}, {"XDG_DATA_HOME", filepath.Dir(i.dirs.Data)}, {"XDG_STATE_HOME", filepath.Dir(i.dirs.State)}}
}
func (i *installer) unit(name string, args []string, terminal bool) service {
	text := "[Unit]\nDescription=skid "
	if terminal {
		text += "terminals\n\n[Service]\nType=simple\nRestart=always\nRestartPreventExitStatus=78\n"
	} else {
		text += "gateway\nAfter=network.target\n\n[Service]\nType=simple\nRestart=on-failure\n"
	}
	text += "RestartSec=3\nExecStart="
	for index, arg := range args {
		if index != 0 {
			text += " "
		}
		text += systemdQuote(strings.ReplaceAll(arg, "$", "$$"))
	}
	text += "\n"
	for _, entry := range i.environment() {
		text += "Environment=" + systemdQuote(entry[0]+"="+entry[1]) + "\n"
	}
	text += "\n[Install]\nWantedBy=default.target\n"
	return service{name, filepath.Join(filepath.Dir(i.dirs.Config), "systemd", "user", name), text}
}
func (i *installer) agent(name string, args []string, terminal bool) service {
	text := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>` + xmlText(name) + `</string><key>ProgramArguments</key><array>`
	for _, arg := range args {
		text += "<string>" + xmlText(arg) + "</string>"
	}
	text += `</array><key>RunAtLoad</key><true/>`
	if terminal {
		text += `<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict><key>ProcessType</key><string>Interactive</string><key>AssociatedBundleIdentifiers</key><string>` + terminalservice.DarwinLabel + `</string>`
	} else {
		text += `<key>KeepAlive</key><true/>`
		if name == "dev.niels.skidbladnir" {
			text += `<key>ProcessType</key><string>Interactive</string>`
		}
	}
	text += `<key>EnvironmentVariables</key><dict>`
	for _, entry := range i.environment() {
		text += "<key>" + entry[0] + "</key><string>" + xmlText(entry[1]) + "</string>"
	}
	text += "</dict></dict></plist>\n"
	return service{name, filepath.Join(i.dirs.Home, "Library", "LaunchAgents", name+".plist"), text}
}
func (i *installer) terminalService(tmuxPath string) service {
	if runtime.GOOS == "linux" {
		return i.unit(terminalservice.LinuxUnit, []string{tmuxPath, "-D", "-c", "exit 78"}, true)
	}
	return i.agent(terminalservice.DarwinLabel, []string{filepath.Join(i.dirs.Data, "terminal-runtime", "SkidTerminals.app", "Contents", "MacOS", "skid-terminals"), tmuxPath}, true)
}
func (i *installer) gatewayServices() []service {
	current := filepath.Join(i.dirs.Data, "current")
	if runtime.GOOS == "linux" {
		return []service{i.unit("skidbladnir.service", []string{filepath.Join(current, "skidbladnir"), "serve"}, false)}
	}
	args := []string{"/usr/bin/open", "-g", "-W", "-a", filepath.Join(current, "Skid.app")}
	for _, entry := range i.environment() {
		args = append(args, "--env", entry[0]+"="+entry[1])
	}
	args = append(args, "--args", "--config", i.dirs.State, "--skid", filepath.Join(current, "skidbladnir"))
	return []service{i.agent("dev.niels.skidbladnir", []string{filepath.Join(current, "skidbladnir"), "serve"}, false), i.agent("dev.niels.skidbladnir.notifications", args, false)}
}
func (i *installer) writeService(s service) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return writeAtomic(s.path, []byte(s.text), 0644)
}
func (i *installer) stopService(ctx context.Context, s service) error {
	if runtime.GOOS == "linux" {
		state, err := exec.CommandContext(ctx, "systemctl", "--user", "show", "--property=LoadState", "--value", s.name).Output()
		if strings.TrimSpace(string(state)) == "not-found" {
			return nil
		}
		if err != nil {
			return err
		}
		return command(ctx, i.stdout, i.stderr, "systemctl", "--user", "stop", s.name)
	}
	target := "gui/" + strconv.Itoa(os.Getuid()) + "/" + s.name
	err := command(ctx, io.Discard, io.Discard, "launchctl", "print", target)
	if err == nil {
		return command(ctx, i.stdout, i.stderr, "launchctl", "bootout", target)
	}
	var failure *exec.ExitError
	if errors.As(err, &failure) && failure.ExitCode() == 113 {
		return nil
	}
	return fmt.Errorf("inspect %s: %w", s.name, err)
}
func (i *installer) startService(ctx context.Context, s service) error {
	if runtime.GOOS == "linux" {
		if err := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		if err := command(ctx, i.stdout, i.stderr, "systemctl", "--user", "enable", s.name); err != nil {
			return err
		}
		return command(ctx, i.stdout, i.stderr, "systemctl", "--user", "start", s.name)
	}
	return command(ctx, i.stdout, i.stderr, "launchctl", "bootstrap", "gui/"+strconv.Itoa(os.Getuid()), s.path)
}
func (i *installer) stopGateway(ctx context.Context) error {
	previous, running, err := terminalservice.GatewayProcess(ctx)
	if err != nil {
		return err
	}
	for _, s := range i.gatewayServices() {
		if err := i.stopService(ctx, s); err != nil {
			return err
		}
	}
	if running {
		if err = waitStopped(ctx, previous); err != nil {
			return err
		}
	}
	if runtime.GOOS != "darwin" {
		return nil
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

func waitStopped(ctx context.Context, previous ...process.Observation) error {
	stopped, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		remaining := false
		for _, expected := range previous {
			after, err := process.Observe(expected.PID)
			if errors.Is(err, process.ErrProcessAbsent) || err == nil && after.StartIdentity != expected.StartIdentity {
				continue
			}
			if err != nil {
				return err
			}
			remaining = true
		}
		if !remaining {
			return nil
		}
		select {
		case <-stopped.Done():
			return errors.New("a stopped service process did not exit within 5 s")
		case <-ticker.C:
		}
	}
}
func (i *installer) startGateway(ctx context.Context, version string) error {
	// Installation excludes concurrent manual gateway launches. Prove that an
	// unmanaged old listener cannot answer readiness for the new native job.
	listener, err := net.Listen("tcp", fleet.LocalAddress)
	if err != nil {
		return errors.New("the local gateway address is already in use; stop the conflicting listener, then rerun the installer")
	}
	if err = listener.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		app, err := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "Skid.app"))
		if err != nil {
			return err
		}
		if err = command(ctx, i.stdout, i.stderr, filepath.Join(app, "Contents", "MacOS", "skid-notifications"), "register", app); err != nil {
			return err
		}
	}
	for _, s := range i.gatewayServices() {
		if err := i.startService(ctx, s); err != nil {
			return err
		}
	}
	ready, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		before, running, observeErr := terminalservice.GatewayProcess(ready)
		executable, pathErr := filepath.EvalSymlinks(filepath.Join(i.dirs.Data, "current", "skidbladnir"))
		local, localErr := fleet.ReadLocal(i.dirs.State)
		if observeErr == nil && running && pathErr == nil && before.Executable == executable && localErr == nil {
			var status fleet.Status
			if localErr = fleet.Call(ready, local.Target(local.Machine), "GET", "/v1/local/status", nil, &status); localErr == nil && status.Version == version {
				after, stillRunning, err := terminalservice.GatewayProcess(ready)
				if err != nil || !stillRunning || after != before {
					return errors.New("the started gateway lifetime changed during readiness")
				}
				return nil
			}
		}
		select {
		case <-ready.Done():
			return errors.New("the installed gateway did not become ready within 5 s")
		case <-ticker.C:
		}
	}
}
