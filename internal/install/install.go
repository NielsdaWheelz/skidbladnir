package install

import (
	"bytes"
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
	"strings"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

type resource struct {
	Path           string `json:"path"`
	Hash           string `json:"hash,omitempty"`
	Target         string `json:"target,omitempty"`
	PreviousTarget string `json:"previousTarget,omitempty"`
	PreviousHash   string `json:"previousHash,omitempty"`
	Directory      bool   `json:"directory,omitempty"`
	Mode           uint32 `json:"mode,omitempty"`
	Current        bool   `json:"current,omitempty"`
}
type shellBlock struct {
	Path    string `json:"path"`
	Text    string `json:"text"`
	Created bool   `json:"created,omitempty"`
}
type receipt struct {
	Version   string        `json:"version"`
	Resources []resource    `json:"resources"`
	Blocks    []shellBlock  `json:"blocks"`
	Services  []string      `json:"services"`
	Route     *tailnetRoute `json:"route,omitempty"`
}

type installer struct {
	dirs           Directories
	receipt        receipt
	stdout, stderr io.Writer
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) (result error) {
	if len(args) == 0 {
		return errors.New("usage: skid install|update [--version vX.Y.Z] [--archive PATH --signature PATH] | uninstall")
	}
	action := args[0]
	if action != "install" && action != "update" && action != "uninstall" {
		return errors.New("expected install, update or uninstall")
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "immutable release version (default: latest stable)")
	archive := flags.String("archive", "", "local signed release archive")
	signature := flags.String("signature", "", "detached archive signature")
	if e := flags.Parse(args[1:]); e != nil {
		return e
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
	if action == "uninstall" && (*version != "" || *archive != "") {
		return errors.New("uninstall accepts no release options")
	}
	dirs, e := Paths()
	if e != nil {
		return e
	}
	lock, e := lockInstallation(dirs)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, lock.Close()) }()
	i := installer{dirs: dirs, stdout: stdout, stderr: stderr}
	data, e := os.ReadFile(filepath.Join(dirs.State, "install-owned.json"))
	if e == nil {
		if e = decodeReceipt(data, &i.receipt); e != nil {
			return errors.New("installation ownership record is invalid")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if action == "uninstall" {
		return i.uninstall(ctx)
	}
	if e = i.platform(ctx); e != nil {
		return e
	}
	if action == "update" && i.receipt.Version == "" {
		return errors.New("skid is not installed; run skid install")
	}
	if e = os.MkdirAll(filepath.Join(dirs.Data, "releases"), 0700); e != nil {
		return e
	}
	stage, e := os.MkdirTemp(filepath.Join(dirs.Data, "releases"), ".stage-")
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
	if *archive == "" {
		if *version == "" {
			*version, e = latest(ctx)
			if e != nil {
				return e
			}
		}
		name := "skidbladnir-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
		*archive = filepath.Join(stage, "archive.tar.gz")
		*signature = filepath.Join(stage, "archive.sig")
		base := "https://github.com/NielsdaWheelz/skidbladnir/releases/download/" + *version + "/"
		if e = download(ctx, base+name, *archive); e != nil {
			return e
		}
		if e = download(ctx, base+name+".sig", *signature); e != nil {
			return e
		}
	}
	if e = verify(*archive, *signature); e != nil {
		return e
	}
	payload := filepath.Join(stage, "payload")
	if e = os.Mkdir(payload, 0700); e != nil {
		return e
	}
	if e = extract(*archive, payload); e != nil {
		return e
	}
	manifest, e := i.validate(ctx, payload)
	if e != nil {
		return e
	}
	if *version != "" && manifest.Version != *version {
		return errors.New("archive version differs from requested release")
	}
	if i.receipt.Version != "" && versionBefore(manifest.Version, i.receipt.Version) {
		return errors.New("release downgrade refused; recover matching private state deliberately before reinstalling")
	}

	if e = i.dependencies(ctx); e != nil {
		return e
	}
	if e = i.checkOwned(); e != nil {
		return e
	}
	defer func() { result = errors.Join(result, i.save()) }()
	final := filepath.Join(dirs.Data, "releases", manifest.Version+"-"+manifest.SourceSHA)
	if _, e = os.Lstat(final); e == nil {
		if e = compareTrees(payload, final); e != nil {
			return fmt.Errorf("existing release differs from verified archive: %w", e)
		}
	} else if errors.Is(e, os.ErrNotExist) {
		if e = os.Rename(payload, final); e != nil {
			return e
		}
	} else {
		return e
	}
	if e = i.recordTree(final); e != nil {
		return e
	}
	config := filepath.Join(dirs.Config, "config.yaml")
	if e = os.MkdirAll(dirs.Config, 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(config, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, e = f.WriteString(hostconfig.DefaultYAML)
		e = errors.Join(e, f.Close())
		if e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrExist) {
		return e
	}
	if e = os.MkdirAll(dirs.Bin, 0755); e != nil {
		return e
	}
	if e = i.link(filepath.Join(dirs.Bin, "skid"), filepath.Join(dirs.Data, "current", "skidbladnir")); e != nil {
		return e
	}
	if e = i.shellStartup(ctx); e != nil {
		return e
	}
	if e = i.writeServices(); e != nil {
		return e
	}
	// Save ownership before visibility; an interrupted install remains removable.
	current := filepath.Join(dirs.Data, "current")
	if _, err := os.Lstat(current); err == nil {
		owned := false
		for _, r := range i.receipt.Resources {
			if r.Path == current {
				owned = true
				break
			}
		}
		if !owned {
			return fmt.Errorf("existing path is not installer-owned: %s", current)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	i.putResource(resource{Path: current, Current: true})
	i.receipt.Version = manifest.Version
	if e = i.save(); e != nil {
		return e
	}
	if len(i.receipt.Services) > 0 {
		if e = i.stopServices(ctx); e != nil {
			return e
		}
	}
	if e = replaceLink(current, final); e != nil {
		return e
	}
	if e = i.save(); e != nil {
		return e
	}
	if e = i.startServices(ctx); e != nil {
		return fmt.Errorf("release installed; service startup failed: %w", e)
	}
	fmt.Fprintf(stdout, "installed %s; settings and private state preserved\n", manifest.Version)
	fmt.Fprintln(stdout, "local shells need no provider or tailscale login; open a new login shell to use skid")
	return nil
}

func (i *installer) validate(ctx context.Context, path string) (releaseManifest, error) {
	var manifest releaseManifest
	b, e := os.ReadFile(filepath.Join(path, "release.json"))
	if e != nil {
		return manifest, e
	}
	if e = strictjson.Decode(b, &manifest); e != nil {
		return manifest, e
	}
	platform, e := archivePlatform()
	if e != nil {
		return manifest, e
	}
	if manifest.Platform != platform || !versionPattern.MatchString(manifest.Version) || !regexpSHA(manifest.SourceSHA) {
		return manifest, errors.New("release identity or platform invalid")
	}
	for _, asset := range []string{"skidbladnir", "characters.json", "providers/shell-init", "providers/terminal-context-init", "providers/quota-read", "native-helper/runtime.json", "native-helper/source/uv.lock", "native-helper/uv"} {
		if _, e := os.Stat(filepath.Join(path, asset)); e != nil {
			return manifest, fmt.Errorf("release missing required asset: %s", asset)
		}
	}
	output, e := exec.CommandContext(ctx, filepath.Join(path, "skidbladnir"), "version").Output()
	if e != nil || string(output) != manifest.Version+" "+manifest.SourceSHA+"\n" {
		return manifest, errors.New("release executable identity check failed")
	}
	helper := exec.CommandContext(ctx, filepath.Join(path, "providers", "quota-read"))
	helperOutput, helperErr := helper.Output()
	var rejected *exec.ExitError
	if !errors.As(helperErr, &rejected) || rejected.ExitCode() != 1 || string(helperOutput) != `{"ok":false,"error":{"code":"rejected"}}`+"\n" {
		return manifest, errors.New("bundled quota helper cannot start; installed release unchanged")
	}
	var native struct {
		Interpreter     string `json:"interpreter"`
		Commit          string `json:"commit"`
		Python          string `json:"python"`
		UV              string `json:"uv"`
		LockSHA         string `json:"lockSha256"`
		RequirementsSHA string `json:"requirementsSha256"`
	}
	nativeBytes, e := os.ReadFile(filepath.Join(path, "native-helper", "runtime.json"))
	if e != nil {
		return manifest, e
	}
	if e = strictjson.Decode(nativeBytes, &native); e != nil {
		return manifest, e
	}
	if !regexpSHA(native.Commit) || !versionPattern.MatchString("v"+native.Python) || !versionPattern.MatchString("v"+native.UV) || native.Interpreter == "" {
		return manifest, errors.New("native helper runtime metadata invalid")
	}
	lockHash, e := fileHash(filepath.Join(path, "native-helper", "source", "uv.lock"))
	if e != nil {
		return manifest, e
	}
	requirementsHash, e := fileHash(filepath.Join(path, "native-helper", "requirements.txt"))
	if e != nil {
		return manifest, e
	}
	if lockHash != native.LockSHA || requirementsHash != native.RequirementsSHA {
		return manifest, errors.New("native helper dependency manifests differ")
	}
	if runtime.GOOS == "darwin" {
		if e = exec.CommandContext(ctx, "codesign", "--verify", "--strict", filepath.Join(path, "Skid.app")).Run(); e != nil {
			return manifest, errors.New("mac notification app signature invalid")
		}
	}
	return manifest, nil
}
func regexpSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (i *installer) save() error {
	b, e := json.MarshalIndent(i.receipt, "", "  ")
	if e != nil {
		return e
	}
	return privatefile.WriteFile(filepath.Join(i.dirs.State, "install-owned.json"), append(b, '\n'))
}
func (i *installer) putResource(r resource) {
	for n, old := range i.receipt.Resources {
		if old.Path == r.Path {
			i.receipt.Resources[n] = r
			return
		}
	}
	i.receipt.Resources = append(i.receipt.Resources, r)
}
func (i *installer) recordTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		r := resource{Path: path, Directory: entry.IsDir(), Mode: uint32(info.Mode().Perm())}
		if entry.Type()&os.ModeSymlink != 0 {
			r.Mode = 0
			r.Target, err = os.Readlink(path)
		} else if !entry.IsDir() {
			r.Hash, err = fileHash(path)
		}
		if err != nil {
			return err
		}
		i.putResource(r)
		return nil
	})
}
func (i *installer) checkOwned() error {
	for _, r := range i.receipt.Resources {
		if r.Current {
			if e := i.matchesCurrent(r.Path); e != nil {
				return e
			}
			continue
		}
		if e := matches(r); e != nil {
			return e
		}
	}
	return nil
}
func matches(r resource) error {
	info, e := os.Lstat(r.Path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	valid := false
	if r.Target != "" {
		v, e := os.Readlink(r.Path)
		valid = e == nil && (v == r.Target || v == r.PreviousTarget)
	} else if r.Directory {
		valid = info.IsDir()
	} else if info.Mode().IsRegular() {
		v, e := fileHash(r.Path)
		valid = e == nil && (v == r.Hash || v == r.PreviousHash)
	}
	if r.Mode != 0 && uint32(info.Mode().Perm()) != r.Mode {
		valid = false
	}
	if !valid {
		return fmt.Errorf("owned resource changed; preserve and resolve manually: %s", r.Path)
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
func (i *installer) link(path, target string) error {
	if _, e := os.Lstat(path); e == nil {
		owned := false
		for _, r := range i.receipt.Resources {
			if r.Path == path {
				owned = true
				break
			}
		}
		if !owned {
			return fmt.Errorf("existing path is not installer-owned: %s", path)
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	previous, e := os.Readlink(path)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	i.putResource(resource{Path: path, Target: target, PreviousTarget: previous})
	if e = i.save(); e != nil {
		return e
	}
	if e = replaceLink(path, target); e != nil {
		return e
	}
	i.putResource(resource{Path: path, Target: target})
	return i.save()
}

func (i *installer) uninstall(ctx context.Context) error {
	if len(i.receipt.Resources) == 0 && len(i.receipt.Blocks) == 0 {
		return errors.New("no recorded skid installation; nothing removed")
	}
	if e := i.checkOwned(); e != nil {
		return e
	}
	if e := i.removeTailnetRoute(ctx); e != nil {
		return e
	}
	if e := i.stopServices(ctx); e != nil {
		return e
	}
	for _, block := range i.receipt.Blocks {
		if e := removeBlock(block); e != nil {
			return e
		}
	}
	// Files before directories; nonempty directories retain unrecorded user files.
	for n := len(i.receipt.Resources) - 1; n >= 0; n-- {
		r := i.receipt.Resources[n]
		e := os.Remove(r.Path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if r.Directory && e != nil {
			entries, readErr := os.ReadDir(r.Path)
			if readErr == nil && len(entries) > 0 {
				continue
			}
		}
		if e != nil {
			return e
		}
	}
	if e := os.Remove(filepath.Join(i.dirs.State, "install-owned.json")); e != nil {
		return e
	}
	fmt.Fprintln(i.stdout, "skid runtime removed; settings, private state, provider data, tmux and shared packages retained")
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

func lockInstallation(dirs Directories) (*os.File, error) {
	if e := os.MkdirAll(dirs.State, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(dirs.State, "install.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return nil, errors.Join(errors.New("another skid installation operation is running"), f.Close())
	}
	return f, nil
}
func decodeReceipt(data []byte, value *receipt) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(value); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		return errors.New("ownership record must contain one json object")
	}
	return nil
}
func compareTrees(want, existing string) error {
	expected := installer{}
	if e := expected.recordTree(want); e != nil {
		return e
	}
	observed := installer{}
	if e := observed.recordTree(existing); e != nil {
		return e
	}
	if len(expected.receipt.Resources) != len(observed.receipt.Resources) {
		return errors.New("release members differ")
	}
	for n, w := range expected.receipt.Resources {
		got := observed.receipt.Resources[n]
		w.Path = strings.TrimPrefix(w.Path, want)
		got.Path = strings.TrimPrefix(got.Path, existing)
		if w != got {
			return errors.New("release contents differ")
		}
	}
	return nil
}

func (i *installer) matchesCurrent(path string) error {
	target, e := os.Readlink(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, r := range i.receipt.Resources {
		if r.Directory && r.Path == target && filepath.Dir(r.Path) == filepath.Join(i.dirs.Data, "releases") {
			return nil
		}
	}
	return fmt.Errorf("current release link is not installer-owned: %s", path)
}

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
