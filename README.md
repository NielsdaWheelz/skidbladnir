# skíðblaðnir

use codex, claude code, and ordinary shell sessions from your desktop or android
phone. skíðblaðnir (`skid`) brings your machines' tmux sessions into one view and
lets you open the same live terminal on either device. detach and the work keeps
running.

this is a personal project built for one user on one tailscale network. the
current phone app requires the author's three-host fleet: arch, devbox, and
macbook. using it with a different fleet requires adaptation.

## a typical workflow

with the fleet installed and paired:

1. run `skid` on your desktop to open the session browser. select a session and
   press enter, or press `N` to choose a machine, directory, and agent profile
   for a new session. lowercase `n` opens a plain shell.
2. work in the agent's normal terminal interface. press `ctrl-]`, then `d`, to
   detach back to the browser.
3. open skíðblaðnir on your phone and tap the same session. continue typing,
   pasting, or dictating through gboard; scroll and copy terminal text as needed.
4. detach on the phone and return to the session from your desktop.

both devices share the process, screen, input draft, and window/pane navigation.
terminal sizing follows the latest active client, so attaching from the phone
can resize the desktop view.

the desktop browser and phone dashboard can filter by machine, group, and whether
a session needs input. they show which agent is present and infer its activity
and requests for input from the terminal. a `ready` indication is a cue to look,
not proof that a task succeeded.

the cli also supports terminal reads, input, waits, interrupts, and closure, with
structured output for automation:

```sh
skid list
skid list --json
skid --help
```

## getting started

[releases](https://github.com/NielsdaWheelz/skidbladnir/releases) contain a signed
android apk and host bundles for linux amd64 and macos arm64. the phone requires
android 16 or later and google play services for qr pairing. use host and phone
artifacts from the same release; mixed versions are unsupported.

host installation belongs to
[dev-server](https://github.com/NielsdaWheelz/dev-server). it installs the gateway,
tmux and provider dependencies, services, shell integration, and tailscale serve
configuration. provider accounts must already be configured. this repository's
[deployment contract](docs/dev-server-handoff.md#owned-installation-and-runtime)
describes that setup; there is no standalone installer or verified fresh-install
walkthrough here yet.

on an installed fleet, `skid` reads the private client configuration at
`~/.config/skidbladnir/client.json`. the terminal browser needs an interactive
terminal of at least 80 × 24 cells. to connect the phone:

1. install the matching release apk and connect tailscale to the same network.
2. from this checkout on a configured desktop, run `scripts/fleet invite`.
   it requires `curl`, `jq`, and `qrencode`, and access to all three gateways.
3. use the app's connect action to scan the qr code. the invitation is single-use
   and expires after five minutes.

## how it works

each host runs a small go gateway in front of its local tmux server. desktop and
phone clients contact those gateways directly over tailscale. each gateway has
its own identity and bearer credential; there is no central coordinator.

tmux owns terminal sessions and processes. codex and claude own their execution
and conversation history. skid lists and attaches to those terminals, so losing
a client connection leaves the host session running. provider credentials and
history stay on their host.

ordinary controls use the terminal. native conversation reads and controls are
a separate capability with provider-specific limits; see
[agent control](docs/agent-control.md). stopping sends an interrupt and does not
prove that background or remote work has ended. the configured agent profiles
run with provider permission prompts bypassed and are trusted as the host user.

## build the cli

requires linux or macos and the go toolchain family declared in [go.mod](go.mod)
(currently 1.26.x). macos also requires a native c toolchain.

```sh
git clone https://github.com/NielsdaWheelz/skidbladnir.git
cd skidbladnir
go build -o ./skid ./cmd/skidbladnir
./skid --help
```

this builds the cli and prints its command reference. running it against hosts
still requires the configured gateways and private client file described above.

`scripts/build` builds the go packages and android debug app. the android build
also requires jdk 17, android sdk platform 36, and build-tools 36.0.0; set
`ANDROID_HOME` to your sdk and `JAVA_HOME` to your jdk. debug builds use a
different signing identity from the published app.

## documentation and status

| question | document |
| --- | --- |
| what owns each part, and what are the constraints? | [architecture](docs/architecture.md) |
| what do terminal and native agent controls mean? | [agent control](docs/agent-control.md) |
| how are hosts configured and installed? | [deployment contract](docs/dev-server-handoff.md#owned-installation-and-runtime) |
| where is the implementation, and how should it change? | [codebase map](docs/codebase-map.md), [codebase rules](docs/rules/index.md) |
| what remains unfinished? | [roadmap](docs/roadmap.md), [open issues](docs/issues) |
| what checks exist, and what do they establish? | [testing policy](docs/rules/testing.md) |

runtime and usability acceptance remain incomplete. `scripts/check verify` runs
engineering checks and builds; the retained detector replay test runs separately.
release publication, installation, and live verification are separate facts.

the repository currently has no project license file. bundled dependencies carry
their own licenses.
