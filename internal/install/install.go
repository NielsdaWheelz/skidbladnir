package install

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

type installer struct {
	dirs           Directories
	stdout, stderr io.Writer
}

// release.json is signed, same-release data, so it is decoded tolerantly.
type releaseManifest struct {
	Platform  string `json:"platform"`
	SourceSHA string `json:"sourceSha"`
	Version   string `json:"version"`
}

func readManifest(path string) (releaseManifest, error) {
	var manifest releaseManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}

var help = map[string]string{
	"install": `usage: skid install [--version vX.Y.Z] [--archive PATH --signature PATH] [--restart-terminals]

install a signed skid release on this computer: the latest stable release, the
--version given, or a local --archive with its detached --signature. missing
tmux and tailscale are installed with homebrew (required on macos), apt and
tailscale's installer, or pacman. skid links ~/.local/bin/skid, adds its marked
blocks to your shell startup files, which put ~/.local/bin on PATH, and starts
its services. settings and private state are kept; a release older than the
installed one is refused. terminal runtime replacement requires closed tmux
sessions, unless --restart-terminals explicitly selects the default server for
checkpointed restart. finish terminal work and pause session creation first.
the restart requires a fresh complete checkpoint and a confirmed resume target
for every recognized agent. shells are rebuilt; running processes are stopped.
`,
	"update": `usage: skid update [--version vX.Y.Z] [--archive PATH --signature PATH] [--restart-terminals]

replace the installed release with the latest stable release or the one given,
as skid install does. gateway-only updates leave terminals running. terminal
runtime replacement requires closed sessions or explicit --restart-terminals,
which checkpoints and stops the selected default server before recovery.
finish work and pause session creation; provider resume requests do not prove
successful resume, and shell process state is not preserved.
`,
	"uninstall": `usage: skid uninstall

require closed tmux sessions and an empty checkpoint, turn off tailscale serve
on https port 8443, stop skid's services, and remove
~/.local/bin/skid, the installed releases, the service files and skid's shell
blocks. settings, private state, provider data and shared packages remain.
`,
}

// A cleanup failure is a failure even if the operation also requires action.
func cleanupError(result, cleanup error, operation string) error {
	if cleanup == nil {
		return result
	}
	if result != nil {
		return fmt.Errorf("%v; %s: %w", result, operation, cleanup)
	}
	return fmt.Errorf("%s: %w", operation, cleanup)
}

// Run installs, updates or uninstalls skid. Every path it writes is fixed and
// skid's by name: releases/*, current, bin/skid, the service files and two marked
// shell blocks. Install overwrites them, uninstall deletes them, and nothing else
// is recorded.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) (result error) {
	action := args[0]
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	// The caller reports parse errors; -h and --help print help below.
	flags.SetOutput(io.Discard)
	version, archive, signature := new(string), new(string), new(string)
	restartTerminals := new(bool)
	if action != "uninstall" {
		version = flags.String("version", "", "")
		archive = flags.String("archive", "", "")
		signature = flags.String("signature", "", "")
		restartTerminals = flags.Bool("restart-terminals", false, "")
	}
	err := flags.Parse(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, err = io.WriteString(stdout, help[action])
		return err
	}
	if err != nil {
		return fmt.Errorf("%w; see skid %s --help", err, action)
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected installation argument")
	}
	if (*archive == "") != (*signature == "") {
		return errors.New("--archive and --signature must be supplied together")
	}
	if *version != "" && !versionPattern.MatchString(*version) {
		return errors.New("version must be vMAJOR.MINOR.PATCH")
	}
	dirs, err := Paths()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dirs.State, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dirs.State, "install.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() { result = cleanupError(result, lock.Close(), "installation lock close failed") }()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another skid installation operation is running")
	}
	i := installer{dirs: dirs, stdout: stdout, stderr: stderr}
	if action == "uninstall" {
		return i.uninstall(ctx)
	}
	if err = platform(ctx); err != nil {
		return err
	}
	current := filepath.Join(dirs.Data, "current")
	installed, err := readManifest(filepath.Join(current, "release.json"))
	if errors.Is(err, os.ErrNotExist) {
		if action == "update" {
			return errors.New("skid is not installed; run skid install")
		}
	} else if err != nil {
		return fmt.Errorf("installed release unreadable: %w", err)
	}
	// The ordinary path refuses foreign servers; the explicit restart selects
	// one exact default lifetime and never silently retargets during installation.
	var selection *restartSelection
	if _, lookupErr := exec.LookPath("tmux"); lookupErr == nil {
		if *restartTerminals {
			chosen, selectionErr := inspectRestartSelection(ctx)
			if selectionErr != nil {
				return selectionErr
			}
			selection = &chosen
		} else {
			if _, _, err = i.inspectTerminal(ctx, false); err != nil {
				return err
			}
		}
	}
	releases := filepath.Join(dirs.Data, "releases")
	if err = os.MkdirAll(releases, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(releases, ".stage-")
	if err != nil {
		return err
	}
	defer func() { result = cleanupError(result, os.RemoveAll(stage), "installation staging cleanup failed") }()
	if *archive == "" {
		if *version == "" {
			if *version, err = latest(ctx); err != nil {
				return err
			}
		}
		name := "skidbladnir-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
		*archive = filepath.Join(stage, "archive.tar.gz")
		*signature = filepath.Join(stage, "archive.sig")
		base := "https://github.com/NielsdaWheelz/skidbladnir/releases/download/" + *version + "/"
		if err = download(ctx, base+name, *archive); err != nil {
			return err
		}
		if err = download(ctx, base+name+".sig", *signature); err != nil {
			return err
		}
	}
	if err = verify(*archive, *signature); err != nil {
		return err
	}
	payload := filepath.Join(stage, "payload")
	if err = os.Mkdir(payload, 0700); err != nil {
		return err
	}
	if err = extract(*archive, payload); err != nil {
		return err
	}
	manifest, err := validate(ctx, payload)
	if err != nil {
		return err
	}
	if *version != "" && manifest.Version != *version {
		return errors.New("archive version differs from requested release")
	}
	// An empty installed version (nothing installed) is never newer.
	if versionBefore(manifest.Version, installed.Version) {
		return errors.New("release downgrade refused; recover matching private state deliberately before reinstalling")
	}
	if err = i.dependencies(ctx); err != nil {
		return err
	}
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	terminal := i.terminalService(tmuxPath)
	runtimeChange, err := i.runtimeChanged(ctx, payload, terminal)
	if err != nil {
		return err
	}
	var before tmux.ServerObservation
	var present bool
	var restartConfig restartSettings
	if *restartTerminals {
		chosen, selectionErr := inspectRestartSelection(ctx)
		if selectionErr != nil {
			return selectionErr
		}
		if selection != nil && (chosen.present != selection.present || chosen.present && (chosen.observed.Identity != selection.observed.Identity || chosen.process.StartIdentity != selection.process.StartIdentity)) {
			return errors.New("the selected terminal server changed during release preparation")
		}
		selection = &chosen
		before, present = chosen.observed, chosen.present
		restartConfig, err = i.restartConfiguration()
		if err != nil {
			return err
		}
	} else {
		before, present, err = i.inspectTerminal(ctx, runtimeChange)
		if err != nil {
			return err
		}
	}
	if !runtimeChange && !*restartTerminals {
		if err = i.waitTerminal(ctx); err != nil {
			return err
		}
	}
	completed := ""
	defer func() {
		if result != nil && completed != "" {
			result = fmt.Errorf("%s; installation incomplete: %w", completed, result)
		}
	}()
	if err = i.stopGateway(ctx); err != nil {
		return fmt.Errorf("gateway shutdown failed: %w", err)
	}
	completed = "gateway stopped"
	var restartCheckpoint *sessions.RuntimeRestart
	defer func() {
		if restartCheckpoint != nil {
			result = cleanupError(result, restartCheckpoint.Close(), "checkpoint writer release failed")
		}
	}()
	if runtimeChange || *restartTerminals {
		if *restartTerminals {
			capture, cancel := context.WithTimeout(ctx, 10*time.Second)
			restartCheckpoint, err = sessions.PrepareRuntimeRestart(capture, restartConfig.config, before, present)
			cancel()
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "validated terminal restart: %d sessions, %d panes, %d agent resume targets (%d inferred); shell process state is not preserved\n", restartCheckpoint.Sessions, restartCheckpoint.Panes, restartCheckpoint.AgentPanes, restartCheckpoint.InferredAgentPanes)
			if err = i.stopRestartSelection(ctx, terminal, *selection, restartCheckpoint, restartConfig); err != nil {
				return err
			}
			completed = "gateway and checkpointed terminal source stopped"
		} else {
			// The stopped gateway cannot repopulate the checkpoint after this read.
			after, stillPresent, inspectErr := i.inspectTerminal(ctx, true)
			if inspectErr != nil {
				return inspectErr
			}
			if stillPresent != present || present && after.Identity != before.Identity {
				return errors.New("the drained tmux server lifetime changed; " + drainInstructions)
			}
			if err = i.stopTerminal(ctx, terminal, after, stillPresent); err != nil {
				return fmt.Errorf("terminal service shutdown failed: %w", err)
			}
			completed = "gateway and drained terminal service stopped"
		}
		if runtimeChange {
			if err = i.replaceRuntime(payload, stage); err != nil {
				return fmt.Errorf("terminal runtime replacement failed: %w", err)
			}
			if err = i.writeService(terminal); err != nil {
				return fmt.Errorf("terminal service definition failed: %w", err)
			}
		} else if err = os.RemoveAll(filepath.Join(payload, "terminal-runtime")); err != nil {
			return err
		}
		if err = i.startService(ctx, terminal); err != nil {
			return fmt.Errorf("terminal service startup failed: %w", err)
		}
		if err = i.waitTerminal(ctx); err != nil {
			return fmt.Errorf("terminal service readiness failed: %w", err)
		}
		completed = "terminal service replaced and ready; gateway stopped"
	} else if err = os.RemoveAll(filepath.Join(payload, "terminal-runtime")); err != nil {
		return err
	}

	if err = os.MkdirAll(dirs.Config, 0700); err != nil {
		return err
	}
	config, err := os.OpenFile(filepath.Join(dirs.Config, "config.yaml"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		_, err = config.WriteString(hostconfig.DefaultYAML)
		if err = errors.Join(err, config.Close()); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	if err = os.MkdirAll(dirs.Bin, 0755); err != nil {
		return err
	}
	if err = replaceLink(filepath.Join(dirs.Bin, "skid"), filepath.Join(current, "skidbladnir")); err != nil {
		return err
	}
	if err = i.addShellBlocks(ctx); err != nil {
		return err
	}
	for _, s := range i.gatewayServices() {
		if err = i.writeService(s); err != nil {
			return err
		}
	}
	final := filepath.Join(releases, manifest.Version+"-"+manifest.SourceSHA)
	replaced := filepath.Join(stage, "replaced")
	if err = os.Rename(final, replaced); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = os.Rename(payload, final); err != nil {
		// Put back the same-version tree current names; the stage cleanup would delete it.
		if restore := os.Rename(replaced, final); !errors.Is(restore, os.ErrNotExist) {
			err = errors.Join(err, restore)
		}
		return err
	}
	if err = replaceLink(current, final); err != nil {
		return err
	}
	completed = "gateway release installed; terminal service ready"
	if restartCheckpoint != nil {
		if err = restartCheckpoint.Close(); err != nil {
			return fmt.Errorf("checkpoint writer release failed: %w", err)
		}
		restartCheckpoint = nil
	}
	if err = i.startGateway(ctx, manifest.Version); err != nil {
		return fmt.Errorf("release installed; service startup failed: %w", err)
	}
	// Only a release that started supersedes the others; a failed start keeps them.
	entries, err := os.ReadDir(releases)
	if err != nil {
		return fmt.Errorf("release installed; pruning failed: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(final) {
			continue
		}
		if err = os.RemoveAll(filepath.Join(releases, entry.Name())); err != nil {
			return fmt.Errorf("release installed; pruning failed: %w", err)
		}
	}
	if *restartTerminals {
		fmt.Fprintf(stdout, "installed %s; terminal service ready; gateway ready; checkpoint recovery requested\n", manifest.Version)
	} else if runtimeChange {
		fmt.Fprintf(stdout, "installed %s; terminal service replaced and ready; gateway ready\n", manifest.Version)
	} else {
		fmt.Fprintf(stdout, "updated gateway to %s; terminal service unchanged; gateway ready\n", manifest.Version)
	}
	fmt.Fprintln(stdout, "local shells need no provider or tailscale login")
	// The installer's PATH is the invoking shell's.
	if !slices.Contains(filepath.SplitList(os.Getenv("PATH")), dirs.Bin) {
		fmt.Fprintln(stdout, `new login and interactive shells find skid; in this one, run: export PATH="$HOME/.local/bin:$PATH"`)
	}
	return nil
}

// validate admits a verified payload for this host. The signature already
// proves the contents; this proves the host can run them.
func validate(ctx context.Context, payload string) (releaseManifest, error) {
	manifest, err := readManifest(filepath.Join(payload, "release.json"))
	if err != nil {
		return manifest, err
	}
	if manifest.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return manifest, errors.New("release is for another platform")
	}
	if runtime.GOOS == "darwin" {
		for _, artifact := range []struct{ path, id string }{
			{filepath.Join(payload, "skidbladnir"), "dev.niels.skidbladnir.gateway"},
			{filepath.Join(payload, "Skid.app"), "dev.niels.skidbladnir"},
			{filepath.Join(payload, "terminal-runtime", "SkidTerminals.app"), "dev.niels.skidbladnir.terminals"},
		} {
			if _, err = verifyMacSignature(ctx, artifact.path, artifact.id); err != nil {
				return manifest, err
			}
		}
	}
	output, err := exec.CommandContext(ctx, filepath.Join(payload, "skidbladnir"), "version").Output()
	if err != nil || string(output) != manifest.Version+" "+manifest.SourceSHA+"\n" {
		return manifest, errors.New("release executable identity check failed")
	}
	return manifest, nil
}

func (i *installer) uninstall(ctx context.Context) (result error) {
	completed := ""
	defer func() {
		if result != nil && completed != "" {
			result = fmt.Errorf("%s; removal incomplete: %w", completed, result)
		}
	}()
	paths := []string{filepath.Join(i.dirs.Data, "current"), filepath.Join(i.dirs.Data, "releases"), filepath.Join(i.dirs.Data, "terminal-runtime"), filepath.Join(i.dirs.Bin, "skid")}
	for _, s := range append(i.gatewayServices(), i.terminalService("")) {
		paths = append(paths, s.path)
	}
	installed := false
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			installed = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if !installed {
		fmt.Fprintln(i.stdout, "no skid installation files found; nothing removed")
		return nil
	}
	// No route, service or file changes precede the drain and checkpoint proof.
	before, present, err := i.inspectTerminal(ctx, true)
	if err != nil {
		return err
	}
	// zsh runs the user's .zshenv and can fail, so it runs before any side effect.
	zdir, err := zshDirectory(ctx, i.dirs.Home)
	if err != nil {
		return err
	}
	if err = i.stopGateway(ctx); err != nil {
		return fmt.Errorf("gateway shutdown failed; removal incomplete: %w", err)
	}
	completed = "gateway stopped"
	after, stillPresent, err := i.inspectTerminal(ctx, true)
	if err != nil {
		return fmt.Errorf("gateway stopped; removal refused: %w", err)
	}
	if stillPresent != present || present && after.Identity != before.Identity {
		return errors.New("gateway stopped; the drained tmux server lifetime changed; " + drainInstructions)
	}
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	terminal := i.terminalService(tmuxPath)
	if err = i.stopTerminal(ctx, terminal, after, stillPresent); err != nil {
		return fmt.Errorf("gateway stopped; terminal service shutdown failed; removal incomplete: %w", err)
	}
	completed = "gateway and drained terminal service stopped"
	// skid connect points tailscale serve's https:8443 at the gateway. A host that
	// never connected, or whose tailscale is down, warns and continues.
	if tailscale, err := TailscalePath(); err == nil {
		routeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = command(routeCtx, io.Discard, io.Discard, tailscale, "serve", "--https=8443", "off")
		cancel()
		if err != nil {
			fmt.Fprintf(i.stderr, "tailscale serve --https=8443 off failed (%v); if a route remains, run it by hand\n", err)
		}
	}
	if err = removeShellBlocks(i.dirs.Home, zdir); err != nil {
		return err
	}
	paths = []string{filepath.Join(i.dirs.Bin, "skid"), filepath.Join(i.dirs.Data, "current")}
	services := append(i.gatewayServices(), terminal)
	for _, s := range services {
		if runtime.GOOS == "linux" {
			if _, err = os.Stat(s.path); err == nil {
				if err = command(ctx, i.stdout, i.stderr, "systemctl", "--user", "disable", s.name); err != nil {
					return fmt.Errorf("services stopped; disabling %s failed: %w", s.name, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		paths = append(paths, s.path)
	}
	for _, path := range paths {
		if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err = os.RemoveAll(filepath.Join(i.dirs.Data, "releases")); err != nil {
		return err
	}
	if err = os.RemoveAll(filepath.Join(i.dirs.Data, "terminal-runtime")); err != nil {
		return err
	}
	if runtime.GOOS == "linux" {
		if err = command(ctx, i.stdout, i.stderr, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
	}
	fmt.Fprintln(i.stdout, "gateway and drained terminal service removed; settings, private state, provider data and shared packages retained")
	return nil
}

// writeAtomic replaces path in one rename, so readers see the old or the new
// bytes, never a truncated file.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".skid-")
	if err != nil {
		return err
	}
	_, err = temporary.Write(data)
	if err == nil {
		err = temporary.Chmod(mode)
	}
	if err == nil {
		err = temporary.Sync()
	}
	err = errors.Join(err, temporary.Close())
	if err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		return errors.Join(err, os.Remove(temporary.Name()))
	}
	return nil
}

func replaceLink(path, target string) error {
	temporary, e := os.CreateTemp(filepath.Dir(path), ".skid-link-")
	if e != nil {
		return e
	}
	name := temporary.Name()
	if e = errors.Join(temporary.Close(), os.Remove(name)); e != nil {
		return e
	}
	if e = os.Symlink(target, name); e != nil {
		return e
	}
	if e = os.Rename(name, path); e != nil {
		return errors.Join(e, os.Remove(name))
	}
	return nil
}

func command(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func versionBefore(candidate, installed string) bool {
	a, b := strings.Split(strings.TrimPrefix(candidate, "v"), "."), strings.Split(strings.TrimPrefix(installed, "v"), ".")
	if len(a) != 3 || len(b) != 3 {
		return false
	}
	for n := range a {
		if len(a[n]) != len(b[n]) {
			return len(a[n]) < len(b[n])
		}
		if a[n] != b[n] {
			return a[n] < b[n]
		}
	}
	return false
}
