# Install skid on a computer

> This procedure is written from the code. Nobody has yet followed it on a fresh
> macOS or Linux computer ([unwalked install](issues/fresh-install-walk.md)). If a step fails with a message that
> [troubleshooting](#troubleshooting) does not list, please open an issue.

This page installs skid on macOS and Linux computers and joins them into one
fleet. The Android app has its own page: [phone](phone.md).

## Requirements

| Platform | Requirement |
| --- | --- |
| macOS | macOS 26 or later on Apple silicon, with [Homebrew](https://brew.sh) |
| Linux | amd64 with systemd user services, running Ubuntu 24.04 or later or an Arch-family distribution (`ID=arch`, or `arch` in `ID_LIKE` in `/etc/os-release`). Ubuntu derivatives and Debian are refused |
| Every computer | curl; bash or zsh as your login shell, or a `shell:` set in [configuration](#configuration) |
| Phone | Android 13 or later with Google Play services ([phone](phone.md)) |

The installer installs tmux and Tailscale when they are missing: through Homebrew
(`tmux` and the `tailscale-app` cask), through apt and Tailscale's own install
script on Ubuntu, or through pacman on Arch. It never installs Codex, Claude Code
or any other provider, nor zoxide, SSH or mosh. Providers are optional; ordinary
shells work without them.

Using skid on one computer needs no account and no network. Reaching a computer
from your phone or from another computer needs a Tailscale account, with every
device signed in to the same tailnet. The installer and `skid connect` check the
rest and name each fix; neither changes your tailnet settings or host policy for
you.

| Prerequisite | Checked by | Fix it names |
| --- | --- | --- |
| Tailscale signed in | `skid connect` | sign in to Tailscale |
| MagicDNS and HTTPS certificates on for the tailnet | `skid connect` | turn them on at <https://login.tailscale.com/admin/dns> |
| Linux: your user is Tailscale's operator (unless you run skid as root) | `skid connect` | `sudo tailscale set --operator=USER` |
| Linux: `tailscaled` running (pacman installs it without starting it) | installer | `sudo systemctl enable --now tailscaled` |
| Linux: linger on, so logging out does not stop skid's gateway and every session | installer | `sudo loginctl enable-linger UID` |

Remote access uses Tailscale Serve's HTTPS port 8443 and only Tailscale's
`*.ts.net` names. `skid connect` replaces whatever port 8443 served before.

## Download and inspect the installer

1. Download the bootstrap installer from the latest release:

   ```sh
   curl -fsSLO https://github.com/NielsdaWheelz/skidbladnir/releases/latest/download/install
   ```

2. Read it (`less install`). It is a short bash script that:
   - refuses any host but macOS 26+ on arm64 and Linux on x86_64;
   - needs curl and an OpenSSL that supports Ed25519 `pkeyutl -rawin`. macOS's
     system OpenSSL does not, so on macOS it runs `brew install openssl@3`;
   - asks GitHub's API for the latest release tag, unless you name a version;
   - downloads `skidbladnir-<platform>.tar.gz` and its `.sig` from that release;
   - verifies the signature with the release public key written at the top of the
     script, and stops with `release signature invalid; nothing installed` if it
     fails;
   - extracts the `skidbladnir` binary and runs its `install`, which verifies the
     archive again with the same key compiled into the binary, checks the
     platform, and only then changes anything.

3. Optionally, check the installer itself. From v0.23.0, every release has a
   `SHA256SUMS` signed by the same release key. With an explicit tag, so that
   every file comes from one release:

   ```sh
   tag=vX.Y.Z
   base=https://github.com/NielsdaWheelz/skidbladnir/releases/download/$tag
   curl -fsSL -O "$base/install" -O "$base/SHA256SUMS" -O "$base/SHA256SUMS.sig"
   curl -fsSLo release-public-key.pem \
     "https://raw.githubusercontent.com/NielsdaWheelz/skidbladnir/$tag/internal/install/release-public-key.pem"
   openssl dgst -sha256 -binary -out SHA256SUMS.digest SHA256SUMS
   openssl pkeyutl -verify -rawin -pubin -inkey release-public-key.pem \
     -in SHA256SUMS.digest -sigfile SHA256SUMS.sig
   sha256sum -c --ignore-missing SHA256SUMS    # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
   ```

   Expect `Signature Verified Successfully` and `install: OK`. The key in
   `release-public-key.pem` is the one at the top of `install`. On macOS, use
   Homebrew's OpenSSL (`"$(brew --prefix openssl@3)/bin/openssl"`) if `openssl`
   is the system one.

## Run the installer

1. Run it, for the latest release or a named one:

   ```sh
   bash install
   bash install vX.Y.Z
   ```

   If you accept the script unread, this does the same; add `-s -- vX.Y.Z` after
   `bash` for a named release:

   ```sh
   curl -fsSL https://github.com/NielsdaWheelz/skidbladnir/releases/latest/download/install | bash
   ```

2. Answer the prompts. Package tools may ask for your password. On macOS, approve
   Tailscale's system extension when macOS asks. On Arch, pacman asks for
   confirmation on the terminal.

3. Read the last lines. A first install usually ends with:

   ```text
   installed vX.Y.Z; settings and private state preserved
   local shells need no provider or tailscale login
   new login and interactive shells find skid; in this one, run: export PATH="$HOME/.local/bin:$PATH"
   ```

   The third line appears only when the shell you ran the installer from lacks
   `~/.local/bin`; new shells need nothing. Before these lines, Linux prints a
   line for linger or `tailscaled` if either is off, with the command to run, and
   macOS reports the scheduling priority of skid's service.

4. Run `skid list`, or `skid` for the session browser in a terminal of at least
   80 × 24 cells.

## What the installer changes

The paths are fixed, and the installer keeps no record of what it wrote.
`XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_STATE_HOME` default to `~/.config`,
`~/.local/share` and `~/.local/state`.

| Path | What |
| --- | --- |
| `~/.local/bin/skid` | Link to the current release's binary |
| `$XDG_DATA_HOME/skidbladnir/releases/VERSION-SHA` | The installed release; `current` links to it. Older releases are deleted once the new one has started |
| `$XDG_CONFIG_HOME/skidbladnir/config.yaml` | Your [configuration](#configuration), written once with no profiles and never rewritten |
| `$XDG_CONFIG_HOME/skidbladnir/machine-handle` | This computer's identity, written once on the first start |
| `$XDG_STATE_HOME/skidbladnir/` | Private state: the fleet, notification state, the workspace checkpoint; on macOS also `serve.log` |
| Linux: `$XDG_CONFIG_HOME/systemd/user/skidbladnir.service` | The user service that runs the gateway |
| macOS: `~/Library/LaunchAgents/dev.niels.skidbladnir.plist` and `dev.niels.skidbladnir.notifications.plist` | The gateway and the notification app |
| Shell startup files | The [shell block](#the-shell-block) |

Private files are mode `0600` in `0700` directories. The gateway listens only on
`127.0.0.1:7341`; nothing is reachable from another device until you run
`skid connect`. Its first start makes this computer the hub of a new fleet of
one, named after its hostname without `.local`.

On macOS, the release includes `Skid.app`, which posts skid's desktop
notifications. It is signed with the project's own certificate, whose SHA-256
each release's notes list, not with an Apple Developer ID, and it is not
notarized. The release signature covers it, and the installer also checks its
code signature. The installer downloads it with curl, which does not quarantine
files, so Gatekeeper does not ask about it.

Reinstalling, updating and uninstalling leave tmux, your sessions and the
programs running in them alone.

### The shell block

The installer appends this block, with your own home in the path, to `.bashrc`;
to the first of `.bash_profile`, `.bash_login` and `.profile` that exists (else
it creates `.bash_profile`); and to `.zlogin` and `.zshrc` in zsh's `ZDOTDIR`
(your home by default). It creates the files that do not exist.

```sh
# >>> skidbladnir managed shell >>>
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin${PATH:+:$PATH}" ;; esac
if [ "${SKIDBLADNIR_SHELL:-}" = 1 ] && [ -r '/home/you/.local/share/skidbladnir/current/providers/shell-init' ]; then
  . '/home/you/.local/share/skidbladnir/current/providers/shell-init'
fi
# <<< skidbladnir managed shell <<<
```

The first line puts `~/.local/bin` on `PATH` only when it is absent. The rest
acts only inside terminals skid starts. `.zshrc` also begins with a short
`managed zsh prompt` block, which turns off powerlevel10k's instant prompt only
in the shell skid starts. Every install rewrites both blocks, so edits inside the
markers are lost; a damaged or duplicated block stops the installation. Other
shells, such as fish, get no block: put `~/.local/bin` on their `PATH` yourself.

## Connect the first computer

Choose one computer as the hub; the others will join its fleet. The hub holds
the fleet's membership and configuration, and terminals never pass through it
([topology and trust](architecture.md#3-topology-and-trust)). On the hub:

1. Sign in to Tailscale: the Tailscale app on macOS, `sudo tailscale up` on
   Linux.
2. Turn on MagicDNS and HTTPS certificates at
   <https://login.tailscale.com/admin/dns>.
3. On Linux, make your user Tailscale's operator:
   `sudo tailscale set --operator=$USER`.
4. Run:

   ```sh
   skid connect
   ```

`skid connect` checks that the running service is the installed release and
that Tailscale is ready, naming each missing fix. It then runs
`tailscale serve --bg --https=8443 http://127.0.0.1:7341`, replacing any earlier
handler on port 8443, and records this computer's address, such as
`https://laptop.example.ts.net:8443`. Last, it prints an invitation: a QR code
when the output is a terminal, then one `skid://connect/...` line. The invitation
works once and expires in five minutes; run `skid connect` again for another.

To pair your phone, scan the QR code: see [phone](phone.md#pair).

On macOS, `skid notifications setup` sets up desktop notifications.

## Add another computer

1. Install skid on the new computer, sign in to the same tailnet, and on Linux
   make your user Tailscale's operator.
2. On any computer already in the fleet, run `skid connect` and copy its
   `skid://connect/...` line.
3. On the new computer, run `skid join` and paste the line when it asks:

   ```text
   $ skid join
   paste the invitation from skid connect:
   ```

   If the fleet already has a computer of the same name (case does not matter),
   choose another: `skid join --name NAME`.
4. Wait a few seconds while the new computer's service restarts as a member.
5. On the new computer, run `skid connect` once. This points its port 8443 at its
   gateway, which your phone and your other computers need to reach it. Ignore
   the invitation it prints.
6. On the hub, run `skid status`. It lists every device; each computer should
   show its release, its `https://...ts.net:8443` address and `applied`. Run it
   on the hub: a member lists the other computers as `pending_offline`
   ([issue](issues/member-status-placeholder.md)).

Joining replaces the new computer's own fleet of one. Its tmux sessions and
identity survive. Its `config.yaml` stays on disk but is unused while it is a
member, so copy any profiles you want into the hub's. A phone paired to its
former fleet loses access to it; to reach the new fleet, clear Skíðblaðnir's
storage on the phone and pair it again ([phone](phone.md#pair)).

Every device in a fleet can open a shell on every computer in it. To revoke one,
run `skid remove DEVICE_ID` on a computer; `skid status` lists device IDs. The
hub cannot be removed. [Fleet operations](architecture.md#fleet-operations)
covers moving or replacing the hub.

## Configuration

The hub's `$XDG_CONFIG_HOME/skidbladnir/config.yaml` configures the whole fleet:
agent profiles, the shell, the macOS terminal for notification clicks, the
default machine and optional Android push. Each field is defined in
[architecture §5](architecture.md#5-configuration).

1. Edit the file on the hub.
2. Run `skid apply` on any computer. It prints why an invalid file is refused,
   and the settings already accepted stay in force. Opening `skid` also applies
   pending edits, but silently.

If your login shell is neither bash nor zsh, set `shell:` to the absolute path of
an installed bash or zsh. Until a computer joins a fleet, its own `config.yaml`
is the one that applies: set it there and restart its gateway (see
[troubleshooting](#troubleshooting)).

## Update

Nothing updates itself. For each computer, then the phone:

1. Run `skid update`, or `skid update --version vX.Y.Z`. Alternatively, download
   the new `install` and run `bash install` as above.
2. Update the phone to the same release ([phone](phone.md#update)).

`skid update` runs the installed release's installer, so changes to the
installer itself take effect from the following update; running the new release's
`install` applies them at once. An older release than the installed one is
refused. tmux sessions and their programs keep running while skid's services
restart; an open `skid` session browser keeps its old binary until you reopen it.
After the hub updates, `skid status` lists it at its old version until you run
`skid connect` on it ([issue](issues/hub-self-report-after-update.md)).

## Uninstall

1. If the computer is a member of a fleet, remove it first: on any computer in
   the fleet, find its device ID with `skid status` and run
   `skid remove DEVICE_ID`.
2. Run:

   ```sh
   skid uninstall
   ```

   It turns off Tailscale Serve on HTTPS port 8443
   (`tailscale serve --https=8443 off`), whatever that port served, and warns if
   it cannot. It stops skid's services and removes `~/.local/bin/skid`, the
   installed releases, the service files and the shell blocks. It keeps your
   configuration, private state, provider data, tmux sessions, tmux and
   Tailscale.
3. Optionally, delete what it keeps. This forgets the computer's identity and
   fleet; if it was the hub, every other device must join a new fleet:

   ```sh
   rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/skidbladnir" \
     "${XDG_STATE_HOME:-$HOME/.local/state}/skidbladnir" \
     "${XDG_DATA_HOME:-$HOME/.local/share}/skidbladnir"
   ```

## Troubleshooting

Most errors print as `skid COMMAND: message`. Where a message names a command,
run it.

Logs: on Linux, `journalctl --user -u skidbladnir`. On macOS, the event log is
`$XDG_STATE_HOME/skidbladnir/serve.log` and `launchctl print
gui/$(id -u)/dev.niels.skidbladnir` shows whether the gateway runs; the reason a
macOS gateway exits is not recorded ([issue](issues/macos-serve-stderr-discarded.md)).

To restart the gateway: `systemctl --user restart skidbladnir.service` on
Linux, `launchctl kickstart -k gui/$(id -u)/dev.niels.skidbladnir` on macOS.

### Installing

| Message | Fix |
| --- | --- |
| `macos 26 or later is required`; `supported hosts: ...`; `ubuntu 24.04 or later is required`; `linux installation supports ubuntu 24.04+ and arch with systemd`; `systemd user services are required` | This computer is not a supported platform |
| `curl is required; install it with the native package manager` | Install curl, then run the installer again |
| `openssl with ed25519 pkeyutl -rawin support is required; install openssl 3 or later` | Install OpenSSL 3 or later. On macOS, install Homebrew; the installer then installs `openssl@3` itself |
| `release version must be vMAJOR.MINOR.PATCH` | Name the release by its tag, such as `bash install v0.23.0` |
| `curl: (22) ...` or another curl error | GitHub did not serve the release, or the named release does not exist. Check the tag and try again later. Naming the release skips the lookup of the latest one, which GitHub rate-limits |
| `release lookup failed: http N`; `release download failed: http N` (from `skid update`) | GitHub did not serve the release. Try again later, or name the version |
| `homebrew is required to install tmux; ...` | Install Homebrew from <https://brew.sh> and run the installer again |
| `pacman needs a terminal to confirm; run: sudo pacman -S --needed ...` | Run the printed command, then the installer again |
| `tmux installed but absent from PATH; ...` | Open a new login shell and run the installer again |
| `release signature invalid; ...` | The download does not match the release key. Nothing changed. Download again; do not work around it |
| `release downgrade refused; ...` | The release is older than the installed one. Downgrading is a deliberate rollback: see [fleet operations](architecture.md#fleet-operations) |
| `another skid installation operation is running` | Wait for the other install, update or uninstall to finish |
| `skid shell block is incomplete or duplicated: FILE` | Edit FILE so it holds one complete block between the markers, or none, then run the installer again |
| `release installed; service startup failed: ...` | Read the logs, fix the cause, and run the installer again. Two macOS causes follow |
| macOS: `...failed: skid serve did not appear within 5 s of bootstrap, ...` | The gateway did not start, or exited at once. Run the `launchctl print` command the message names, and read the logs |
| macOS: `...failed: skid serve (pid N) runs at scheduling priority N, below interactive 31: ...` | Follow the message's check, then reinstall |

### After installing

| Symptom or message | Fix |
| --- | --- |
| `command not found: skid` | Open a new terminal, or run the `export PATH=...` line the installer printed. Shells other than bash and zsh need `~/.local/bin` on their `PATH` |
| `skid is not installed; run skid install` | The gateway has never started successfully. Read the logs |
| `skid service unavailable; check skid status and network access` | The local gateway, or the hub it needed, did not answer. Read the logs; on a member, check that the hub is online |
| `skid service: local settings invalid; fix config.yaml and restart skid` (Linux log) | Fix `config.yaml`; a login shell that is not bash or zsh needs `shell:`. Restart the gateway |
| `linger is off, ...` | Run the printed `sudo loginctl enable-linger UID` if the computer must keep sessions after you log out |
| `tailscaled is not running, ...` | Run `sudo systemctl enable --now tailscaled` |

### Connecting and joining

| Message | Fix |
| --- | --- |
| `installed service differs from this skid release; ...` | Restart the gateway |
| `tailscale is unavailable; run skid install` | Tailscale is not installed; run the installer again |
| `tailscale unavailable; open tailscale and sign in` | Start Tailscale and sign in; Tailscale's own message follows |
| `tailscale is not connected; sign in to tailscale first` | Sign in to Tailscale and connect |
| `tailscale is not ready to serve skid:` with a list | Do each listed fix, then run `skid connect` again |
| `tailscale serve failed:` with Tailscale's text | Follow Tailscale's text |
| `hub has no remote address; ...` | Run `skid connect` on the hub first |
| `FleetConflict: too many pending invitations; wait five minutes` | Wait for unused invitations to expire |
| `invalid invitation; run skid connect again` | The line was incomplete or has expired. Copy a fresh one |
| `PairingInviteRejected: this fleet invite is invalid, expired, or already used` | Run `skid connect` for a fresh invitation |
| `FleetConflict: machine name already enrolled; choose an explicit new name` | Another computer in the fleet has this name: join with `--name NAME`. If that entry is this computer from before its identity was deleted, remove the stale entry instead, as for `origin already enrolled` |
| `FleetConflict: this computer is the hub` | This computer already leads that fleet; it cannot join it |
| `FleetConflict: origin already enrolled` (join); `FleetConflict: origin already belongs to another computer` (connect) | The fleet holds a stale entry with this computer's Tailscale address, usually this computer from before its identity was deleted. On a computer in the fleet, find the old device ID with `skid status`, run `skid remove DEVICE_ID`, then join or connect again |
