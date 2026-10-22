# skíðblaðnir

use codex, claude code, and ordinary shell sessions from your desktop or android
phone. skid brings enrolled machines' tmux sessions into one view. detach and the
work keeps running; either device can return to the same live terminal.

## install

[releases](https://github.com/NielsdaWheelz/skidbladnir/releases) contain the
bootstrap `install`, signed host archives, and matching android apk. download and
inspect `install`, then run:

```sh
bash install
```

the installer owns skid's services, shell integration and updates. it installs
missing tmux/tailscale dependencies through native package tools; privilege and
network login can require your action. mac requires macos 26+ arm64 and homebrew.
linux supports ubuntu 24.04+ and arch amd64 with systemd. android requires 13+ and
google play services. use host and phone artifacts from the same release.

run `skid` for the desktop browser. empty profiles support ordinary shells;
provider tools and accounts are optional. the terminal needs at least 80 × 24 cells.

## configure

edit `${XDG_CONFIG_HOME:-~/.config}/skidbladnir/config.yaml` on your hub:

```yaml
version: 1
profiles:
  - name: codex
    command: codex
    provider: codex
  - name: work
    command: claude-work
    provider: claude
# hub: always-on-computer
# terminal: ghostty
```

commands run exactly once in the configured bash/zsh after its normal startup.
aliases, functions, environment and provider settings remain effective. skid adds
no provider flags and starts no provider daemon. optional `provider` and `home`
metadata enable attributable account observation; commands without them still run.
see the [configuration contract](docs/generalization-plan.md#2-configuration-and-durable-state)
for sparse machine overrides. opening skid on any enrolled computer asks the hub
to apply pending edits; `skid apply` does so without opening the browser. invalid
edits retain the accepted settings.

## connect devices

1. for remote/mobile access, connect tailscale on both devices and enable tailnet https.
2. install the matching skid apk.
3. run `skid connect` on an installed computer; scan its single-use five-minute qr.

session management needs no firebase setup. android background notifications are
an [optional integration](docs/android-push-setup.md): you own its firebase project
and private sending credentials; your hub sends directly to google. mac
notifications work independently. skid operates no notification backend.

another computer
joins by installing skid and pasting the invitation into `skid join --name NAME`.
new enrolled machines appear automatically. `skid status` reports membership and
configuration application; `skid remove DEVICE_ID` revokes a device.

one machine is the hub for configuration, membership and notification observation.
set `hub` to an enrolled machine name to request a planned move. cached direct
terminal access survives hub loss; edits and notifications wait for a hub.
`skid recover` explicitly creates a new fleet and requires fresh enrollment.

skid manages tmux only where skid is installed. manual ssh/mosh from an ordinary
local terminal remains available for other machines.

## use

press enter to attach; `N` creates an agent and `n` creates a shell. `ctrl-]`, then
`d`, detaches to the browser. the phone opens the same sessions and supports its
usual typing, pasting, dictation, scrolling and copy controls. the latest active
client determines terminal size.

```sh
skid list
skid list --json
skid --help
skid update
skid uninstall
```

ordinary controls use tmux and exact terminal identities. read returns rendered
terminal text; guarded send checks the current screen. stop sends an interrupt.
receipts prove neither provider acceptance nor completion. quota prefers a fresh
account report from an already-running native daemon, then
attributable terminal observations. [agent control](docs/agent-control.md) describes
the boundaries. readiness is a cue to inspect, not proof of successful work.

uninstall removes only recorded skid runtime resources. settings, private state,
provider history, tmux sessions and shared dependencies remain.

## development

build with the go toolchain family in [go.mod](go.mod); mac also needs a native c
toolchain. android uses jdk 17, platform 36 and build-tools 36.1.0.

```sh
go build -o ./skid ./cmd/skidbladnir
scripts/check verify
```

[architecture](docs/architecture.md), [codebase map](docs/codebase-map.md),
[rules](docs/rules.md), [roadmap](docs/roadmap.md), and
[open issues](docs/issues) describe ownership and remaining work. engineering
checks and builds do not establish live behavior or release/deployment acceptance.
