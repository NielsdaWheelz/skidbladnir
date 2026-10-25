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
	"install": `usage: skid install [--version vX.Y.Z] [--archive PATH --signature PATH]

install a signed skid release on this computer: the latest stable release, the
--version given, or a local --archive with its detached --signature. missing
tmux and tailscale are installed with homebrew (required on macos), apt and
tailscale's installer, or pacman. skid links ~/.local/bin/skid, adds its marked
blocks to your shell startup files, which put ~/.local/bin on PATH, and starts
its services. settings and private state are kept; a release older than the
installed one is refused.
`,
	"update": `usage: skid update [--version vX.Y.Z] [--archive PATH --signature PATH]

replace the installed release with the latest stable release or the one given,
as skid install does. tmux sessions and providers keep running.
`,
	"uninstall": `usage: skid uninstall

turn off tailscale serve on https port 8443, stop skid's services, and remove
~/.local/bin/skid, the installed releases, the service files and skid's shell
blocks. settings, private state, provider data, tmux and shared packages remain.
`,
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
	if action != "uninstall" {
		version = flags.String("version", "", "")
		archive = flags.String("archive", "", "")
		signature = flags.String("signature", "", "")
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
	defer func() { result = errors.Join(result, lock.Close()) }()
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
	releases := filepath.Join(dirs.Data, "releases")
	if err = os.MkdirAll(releases, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(releases, ".stage-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
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

	// Nothing above changed a skid-owned path. The writes below leave running
	// services undisturbed, so they precede stop: a refused startup file fails
	// before any downtime.
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
	for _, s := range i.services() {
		if err = os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
			return err
		}
		if err = writeAtomic(s.path, []byte(s.text), 0644); err != nil {
			return err
		}
	}
	// Stop precedes any release move: mac finds the running app through
	// current's old physical bundle, which a move aside would hide.
	if err = i.stopServices(ctx); err != nil {
		return err
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
	if err = i.startServices(ctx); err != nil {
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
	fmt.Fprintf(stdout, "installed %s; settings and private state preserved\n", manifest.Version)
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
	output, err := exec.CommandContext(ctx, filepath.Join(payload, "skidbladnir"), "version").Output()
	if err != nil || string(output) != manifest.Version+" "+manifest.SourceSHA+"\n" {
		return manifest, errors.New("release executable identity check failed")
	}
	if runtime.GOOS == "darwin" {
		if err = exec.CommandContext(ctx, "codesign", "--verify", "--strict", filepath.Join(payload, "Skid.app")).Run(); err != nil {
			return manifest, errors.New("mac notification app signature invalid")
		}
	}
	return manifest, nil
}

// uninstall is idempotent: on a host with nothing installed it changes nothing
// but the tailscale route and still succeeds.
func (i *installer) uninstall(ctx context.Context) error {
	// zsh runs the user's .zshenv and can fail, so it runs before any side effect.
	zdir, err := zshDirectory(ctx, i.dirs.Home)
	if err != nil {
		return err
	}
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
	if err = i.stopServices(ctx); err != nil {
		return err
	}
	if err = removeShellBlocks(i.dirs.Home, zdir); err != nil {
		return err
	}
	paths := []string{filepath.Join(i.dirs.Bin, "skid"), filepath.Join(i.dirs.Data, "current")}
	for _, s := range i.services() {
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
	fmt.Fprintln(i.stdout, "skid runtime removed; settings, private state, provider data, tmux and shared packages retained")
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
