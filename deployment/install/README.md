# standalone installation

download the reviewed `install` script from an immutable github release, inspect
it, then run `bash install`. it resolves the latest stable tag once, verifies the
host archive against the embedded ed25519 public key, then invokes that archive's
installer. `bash install vMAJOR.MINOR.PATCH` selects one exact release. the script
is the initial trust anchor; downloading a replacement key beside an archive
would not authenticate the archive.

installed commands:

```sh
skid install
skid update
skid update --version vMAJOR.MINOR.PATCH
skid install --archive /absolute/release.tar.gz --signature /absolute/release.tar.gz.sig
skid uninstall
```

install and update download one signed immutable release, or verify the supplied
local archive. repeated installation converges only when an existing release's
complete immutable tree matches the signed archive. staging validates the host
binary, packaged assets and managed helper before replacing `current`. the
installer stops and restarts skid services at activation; it never restarts tmux
or providers. failed validation leaves the active release untouched. a service
startup failure reports that the release is installed and needs service repair.
the installer refuses a release older than its recorded installation. matching-state
recovery remains an explicit operator action.

supported hosts are macos 26+ arm64, ubuntu 24.04+ amd64 and current arch-family
amd64 with systemd. arch-family means `ID=arch` or an exact `arch` token in
`ID_LIKE`; admission and pacman dependency installation use the same decision.
ubuntu still requires its own `ID=ubuntu` and minimum version. mac binaries
explicitly encode deployment target 26.0; the release
checker inspects both. the ubuntu qualification boundary is 24.04/glibc 2.39,
not an assertion about every linux distribution. android's release check requires
minimum api 33 and target api 36; physical older-version acceptance is separate.

missing tmux/tailscale dependencies use native package mechanisms: homebrew and
the official tailscale mac app cask on mac; apt plus tailscale's official signed
repository installer on ubuntu; pacman on arch. package installation may request
administrator permission. homebrew itself is an explicit mac prerequisite when
missing packages need it. the bootstrap uses openssl 3+ for ed25519 verification;
macos's system libressl is insufficient, so it installs homebrew openssl when
available. tailscale login and mac system-extension permission are user actions,
required only for remote access. linux startup before login additionally needs
`sudo loginctl enable-linger USER`; the installer prints the exact command.

provider installation, accounts, settings, history and daemon lifetime remain
external. the default yaml has no profiles. local shell use needs no provider,
network login or runtime download. zoxide, ssh and mosh are optional user tools.

the signed archive contains the provider-runtime source, uv, and managed
python pinned in [`native-runtime.json`](../../internal/install/native-runtime.json),
the source's original `uv.lock`, and installed distributions admitted by frozen export
with base dependencies and required wheel hashes. `providers/quota-read` imports
only the bounded account-quota entrypoint; no optional claude SDK, conversation
launcher, identity plugin, cli or provider daemon is installed. `runtime.json`
records commit, uv, python, interpreter, lock and requirements hashes.
runtime launches the bundled interpreter directly, with
isolated imports and bytecode writes disabled; startup never downloads packages.

config, data, state and cache follow their XDG directories, falling back to
`~/.config`, `~/.local/share`, `~/.local/state` and `~/.cache`. the public executable
is `~/.local/bin/skid`. installation adds guarded source blocks after existing
bash/zsh startup content; it respects bash's selected login file and zsh's
resolved `ZDOTDIR`, preserving symlink targets and existing file permissions.
for the exact initial skid zsh process, a separate managed preamble disables
powerlevel10k's instant prompt before the user's cache preamble can redirect its
terminal. ordinary and nested shells keep their normal instant prompt. the
fully initialized theme still draws the prompt; only the early cached prompt is
skipped. a later user theme setting may override the parameter. skid changes no
provider configuration or global environment for this compatibility rule.

uninstall checks recorded ownership before acting. it removes recorded skid
services, exact links, release files and exact shell blocks. a changed owned
resource stops removal with its path. additional user files remain. a recorded
tailscale root proxy is removed only if the exact https:8443 configuration still
matches and no additional routes share that port; unrelated serve routes remain.
settings, private state, provider data, tmux and shared packages remain. there is
no purge option.

# release signing

`scripts/release` signs the sha256 bytes of each host archive with ed25519 and ships
detached `.tar.gz.sig` files beside the bootstrap installer. set
`SKIDBLADNIR_RELEASE_SIGNING_KEY` to the private PKCS8 key path; the local default is
`~/.config/skidbladnir/release-signing.pem`. the release refuses before building unless
the key's derived public key equals `internal/install/release-public-key.pem`. private
material is never included in release assets. the release checker verifies each
signature and requires the bootstrap's literal public key to match this canonical pin.

checksums detect corruption; signatures establish possession of the pinned
release key. they are different claims. signing keys and the initial bootstrap
must be distributed through a trusted channel.
