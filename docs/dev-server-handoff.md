# skidbladnir deployment contract

2026-09-25 restoration source in `/Users/nnandal/Documents/code/skid-v1`, based on
`927c55412eec7fa129a3325fb8f5ebf8051b6ea0`. publication belongs to the root
operator; live installation waits for host namespace handback. this file's
containing commit owns the source contract. dev-server is the sole installation owner. this file
is the contract for its separation agent; the other product is herdr-mobile.

provider continuity correction: all products use the existing provider accounts.
preserve `.codex`, `.codex-work`, `.codex-work2`, native personal claude's default
state, and `.claude-work`, including authentication, configuration, history,
memories, trust and native herdr/user integrations. no provider-home provisioning,
relocation or copy is part of skid installation. gateway credentials and helper
dependencies remain skid-owned. command overrides remain scoped to skid launches
and marked terminals; ordinary/herdr account selection is unchanged.

## release and namespace

original github repository id: `1386409483`; reclaimed name:
`NielsdaWheelz/skidbladnir`. the other repository, `1342599607`, is
`NielsdaWheelz/herdr-mobile`. github names and immutable-release settings are
verified; neither remains a blocker. the local checkout remains `skid-v1`.
live original-skid installation still waits for the root operator's namespace
handback.

published release: [v0.9.0](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.9.0),
immutable and final, from `580e0992d1ee0d7334cefc6561e7f55a5836baf5`.
android `0.9.0` / `9000`; package `dev.niels.skidbladnir`.
published upstream pin: [release-pin.json at 50a688b](https://github.com/NielsdaWheelz/skidbladnir/blob/50a688bf92080428bcb9543b1654c28eff81213f/release-pin.json),
containing the exact source and all five published asset digests.
deployment host pin: [release-pin.json at ce6b96b](https://github.com/NielsdaWheelz/dev-server/blob/ce6b96bc0aacd6ab23506a38cd4e014061c3f103/assets/skidbladnir/release-pin.json).
read-only verification confirms that the immutable release metadata matches
all five upstream digests and both deployment archive urls/digests. the stale
`v0.6.0` artifacts belong to the other repository and remain excluded from
installation and rollback.

the root operator's fresh usb preflight before publication observed the old
`dev.niels.skidbladnir` package at `0.8.0` / `8000`; build-tools `36.1.0`
`apksigner` verified its retained signing certificate sha-256:
`7b2ba254e9d3cb18044b723fe124dec87b727ab56e817860bb48e056eddc47bf`.
the operator reports the signed draft reviewed, all five published digests
verified, and `scripts/check published-release v0.9.0
580e0992d1ee0d7334cefc6561e7f55a5836baf5` passed using the android-studio jdk.
these publication prerequisites are closed; they do not qualify live cutover.

the released source passed [hosted verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36212743764)
and `scripts/check-release-source v0.9.0` before publication. that preflight
checks clean main, repository identity, immutable releases, tag availability,
and the exact hosted run; use an unused increasing tag for a future release.
publication did not depend on live host handback. no signer or state from
herdr-mobile is an input to skid installation or verification.

## owned installation and runtime

retain binary `skidbladnir` and optional `skid` cli, linux user unit
`skidbladnir.service`, macos label `dev.niels.skidbladnir`, and private roots
`~/.config/skidbladnir`, `~/.local/share/skidbladnir`,
`~/.local/state/skidbladnir`. bind `127.0.0.1:7341`; tailscale serve owns only
https `8443` `/v1`. upstream herdr, its socket/service, and herdr-mobile's
`7342` / `8444` belong to the other product. remove herdr service dependencies
from skid's restored unit. gateway stop must not kill tmux workers; preserve
the historical linux `KillMode=process` arrangement. never reset tailscale
serve or modify the default tmux server's environment.

after explicit handback, mint fresh bearer and machine-handle files using the
existing skid commands; mode `0600`. generate a fresh private
`~/.config/skidbladnir/client.json` through skid's existing fleet provisioning.
do not copy, link, preserve, or roll back to herdr-era credentials/generations.
ordinary later skid reinstalls preserve these new skid identities and workers.
removal is an explicit allowlist of skid files/services and its own serve
mapping; no provider-binary, tailscale, herdr, shared-home, or jarvis removal.

gateway argv remains:

```text
skidbladnir gateway --listen=127.0.0.1:7341 --bearer-file=HOME/.config/skidbladnir/bearer --catalogue-path=HOME/.local/share/skidbladnir/current/characters.json --machine-handle-file=HOME/.config/skidbladnir/machine-handle --host-config=HOME/.local/share/skidbladnir/current/host-config.json
```

## host config and validator

render [`deployment/providers/host-config.json`](../deployment/providers/host-config.json)
to the generation's `host-config.json`. replace tokens in decoded json string
values, then serialize json; do not shell-interpolate json. substitutions:

| token | value |
| --- | --- |
| `@ROOT@` | deployment user's absolute home |
| `@PLATFORM@` | `Linux` or `Darwin` |
| `@TMUX@` | absolute installed tmux executable; historically `/usr/bin/tmux` on linux, `/opt/homebrew/bin/tmux` on macbook |
| `@TMUX_VERSION@` | observed canonical `tmux ...` version, advisory |
| `@CODEX@` | absolute native codex executable from the shared managed installation; no account wrapper or router |
| `@CLAUDE@` | absolute native claude executable or its managed symlink; historically `HOME/.local/bin/claude` |

codex's npm command may itself be a javascript launcher: select its packaged
native `codex` executable, not `HOME/bin/codex` or an account wrapper. resolving
that platform binary is the shared provider installer's responsibility. inspect
the native claude symlink target/executable; a same-name account wrapper is invalid.

exact schema (all named members required unless marked optional):

- root: `platform`, `tmux:{path,testedVersion}`, `nativeControlPath`, `profiles`.
- `profiles`: either `[]` for terminal-only operation or exactly the four rows
  below in that order. the restoration uses all four.
- each row: `key`, `label`, `provider`, `command`, `environment`,
  `foregroundSignatures`, `arguments`.
- environment entry: `{name,value}` strings. exactly one absolute provider home;
  names unique, no other provider's home, `HERDR_*`, `SKIDBLADNIR_SHELL`, `SKIDBLADNIR_AGENT`, or
  `SKIDBLADNIR_CLAUDE_COMMAND`. provider homes unique
  within each provider. other explicitly configured environment values retain
  their existing meaning.
- signature: optional `executableBase`, `argument0`, `argument1` strings;
  executable base or absolute argument zero required. use the template's native
  signatures. overlapping signatures across providers are invalid.
- arguments: string array. claude forbids configured `-n`, `--name`, or
  `--name=...`; skid supplies the generated tmux name itself.

unknown/duplicate/null members, wrong types, relative required paths, wrong
runtime platform, or invalid profile order are rejected. config must be one
regular non-symlink file, at most 64 kib. validate before switching a generation:

```sh
"$staged_binary" validate-host-config --host-config="$staged_host_config"
```

this source candidate adds that command. it performs the same strict config
load as gateway startup against the current os and returns `0` only on valid
configuration; diagnostics are content-free. it does not invoke tmux, start a
provider, inspect credentials, or prove executable/dependency availability.
the installer separately checks the declared executables and helper pins.

## profiles and shell defaults

select the existing account directories below. skid does not create account
trees, replace instructions/settings, reset trust, or copy credentials/history.
the user's existing provider setup owns those files and its normal login flow.

| forge key | provider home relative to `HOME` | native argv after executable |
| --- | --- | --- |
| `personal` | `.codex` | `--yolo` |
| `work` | `.codex-work` | `--yolo` |
| `work2` | `.codex-work2` | `--yolo` |
| `claude-work` | `.claude-work` | `--name <tmuxName> --dangerously-skip-permissions --plugin-dir HOME/.local/share/skidbladnir/claude-agent-identity` |

codex rows set only their `CODEX_HOME`; claude sets `CLAUDE_CONFIG_DIR`.
manual `claude`/`claude-personal` leave `CLAUDE_CONFIG_DIR` unset to retain native
default behavior, including `~/.claude.json`; setting it to `~/.claude` is not
assumed equivalent. personal claude is never a forge profile. its unconfigured
runtime profile remains unknown and uses terminal
observation/control; it does not add a fifth native-helper account. permission
bypass is preserved; provider trust/setup may still prompt.

install [`provider-command`](../deployment/providers/provider-command) at
`current/providers/provider-command` (0755), rendering `@ROOT_SHELL@`,
`@CODEX_SHELL@`, and `@CLAUDE_SHELL@` as shell-quoted literals from the same
deployment inputs as the json config. install
[`shell-init`](../deployment/providers/shell-init) at `current/providers/shell-init`
(0644). this is a closed five-account launcher, not a shared provider router.

the original app adopts dev-server's ten-file
[generation receipt contract](/Users/nnandal/Documents/code/dev-server/docs/gateway-separation-runbook.md:275).
this supersedes the earlier six-file proposal. `scripts/fleet` validates regular
non-symlink files and these exact modes, in this exact hash order:

| order | relative filename | mode |
| --- | --- | --- |
| 1 | `skidbladnir` | `0755` |
| 2 | `characters.json` | `0644` |
| 3 | `release.json` | `0644` |
| 4 | `host-config.json` | `0600` |
| 5 | `providers/native-control` | `0755` |
| 6 | `providers/provider-command` | `0755` |
| 7 | `providers/shell-init` | `0644` |
| 8 | `providers/claude-agent-identity/.claude-plugin/plugin.json` | `0644` |
| 9 | `providers/claude-agent-identity/hooks/hooks.json` | `0644` |
| 10 | `providers/claude-agent-identity/bin/agent-hook` | `0755` |

for each file, encode `relative filename + 0x00 + lowercase hex sha256(file) +
0x0a`. filenames are the literal ascii strings above; there are no spaces,
leading `./`, mode bytes, or extra record separators. concatenate all ten
records in that order and compute their sha-256, expressed as 64 lowercase hex
digits. modes are validated separately, not hashed. generation directories are
`0700`; their basename is `VERSION-DIGEST`. the active receipt
`~/.local/state/dev-server/active/skid.runtime.sha256` is a regular non-symlink
file, owned by the deployment user, mode `0600`, containing `DIGEST` and exactly
one final newline. fleet verification requires computed digest, directory suffix,
and receipt to agree.

the helper launcher and plugin are generation files. public paths are stable
relative symlinks: `~/.local/bin/skidbladnir-provider-runtime-control` points to
`../share/skidbladnir/current/providers/native-control`;
`~/.local/share/skidbladnir/claude-agent-identity` points to
`current/providers/claude-agent-identity`. changing `current` selects gateway,
helper launcher, shell commands, and plugin together. the launcher's separately
pinned helper environment retains its own preactivation integrity checks.
cost: four more hashed files and retained private helper environments, with no
independent dependency switch during rollback. published archive contents and
checksums are unaffected.

contract agreement, 2026-09-25: both owners acknowledge this ten-file contract.
dev-server introduced its acknowledgment and admission correction in
`94931a13c045fa9c9b294080149524501ff30b68`, retained in pushed source
`1296309c8350930befeb62e02479a8a6cf2a819c`:
[runbook](https://github.com/NielsdaWheelz/dev-server/blob/1296309c8350930befeb62e02479a8a6cf2a819c/docs/gateway-separation-runbook.md#generation-receipt-contract)
and [validation](https://github.com/NielsdaWheelz/dev-server/blob/1296309c8350930befeb62e02479a8a6cf2a819c/docs/gateway-separation-validation.md).
its owner confirmed agreement with this corrected handoff. the stale six-file
note is removed and the directory-mode/digest-suffix discrepancy is closed.
using existing provider accounts changes neither the ten-file sequence nor its
digest or mode rules. dev-server subsequently implemented and acknowledged
the existing-account, claude-only hook and skid-owned shell contract in pushed
source `ae70f2b78ad9b02a7dbccfd79d53be5bcfd11b50`. its
[validation](https://github.com/NielsdaWheelz/dev-server/blob/ae70f2b78ad9b02a7dbccfd79d53be5bcfd11b50/docs/gateway-separation-validation.md)
records installer and shell evidence; no source contract discrepancy remains.

a disposable fixture used dev-server's actual `gateway_runtime_identity` and
`gateway_restore_runtime` with this fleet verifier. the producer matched the
specified byte encoding; an intact receipt passed, alterations to each of the
four added files failed, and wrong modes on all ten files failed. generation
mode, receipt mode/final newline, and digest suffix checks also passed.

the rollback fixture started with a candidate `current` and a verified prior
receipt/unit pair. real filesystem restore operations selected the prior
gateway, helper launcher, all plugin files, and unit together. fleet verification
then passed. both a distinct `previous` and no `previous` were exercised;
failure to stop the candidate returned `4` without moving pointers or files.
the prior receipts and an unrelated herdr-mobile sentinel were preserved.
supervisor and health boundaries used stand-ins: this proves filesystem/control
flow, not live supervisor recovery or preserved workers/attachments. no service,
tmux, device, or other repository file was changed; temporary probes were removed.

the deployment admission gap is resolved: `gateway_generation_owned` now
requires `0700` and the computed digest suffix. both owners reproduced the
former failures and verified corrected intact/wrong-mode/wrong-suffix cases
against the actual fleet verifier. our independent rerun also rejected each
of the four added-file alterations. dev-server verified that invalid prior
generations fail admission and an intact prior restores the runtime/unit pair
and helper/plugin selection. contract agreement is complete; native rollback on macbook,
devbox, and arch remains `NOT_RUN`; after the approved live window, record that
the restored receipt/pair, `current`, public helper/plugin links and unrelated
workers/attachments all match the intended prior deployment.

new skid terminals set `SKIDBLADNIR_SHELL=1`, select `CODEX_HOME=HOME/.codex`, and
clear inherited `CLAUDE_CONFIG_DIR` before the configured login shell.
at the END of that shell's ordinary startup,
source the installed `shell-init` when that marker is `1`. it removes any
`HERDR_*` introduced during startup and replaces shared aliases with product-local
functions. zsh requires the end of `.zshrc` and,
if it changes provider setup, the later `.zlogin`; bash requires the actual
login file (`.bash_profile`, `.bash_login`, or `.profile`) and `.bashrc` for
interactive subshells. do not assume bash login reads `.bashrc`. current fleet
shell support is bash/zsh; configure and qualify its actual startup path.
skid apply owns startup validation and edits. shared provider installation must
not edit or validate startup files for skid, or fail because skid is absent.
the fully managed zshrc may retain its optional source line, guarded by the
skid shell marker and absence of herdr context before any file access. this
small static dependency preserves skid support when ordinary dotfile maintenance
replaces that whole managed file. no general extension renderer is introduced.
preserve startup-file symlinks and user content; resolve a missing or unsupported
skid startup path within skid setup, without replacing dotfiles. shell-init is
idempotent: source it after each relevant startup file, without a once-only flag
that would allow later login files to override skid's functions or defaults.

within marked skid terminals, bare `codex`/`claude` select the existing personal
homes; `codex-personal`, `codex-work`, `codex-work2`, `claude-personal`,
`claude-work` select those exact homes. explicit account selection wins over
inherited home variables. functions
call the absolute installed launcher, so startup PATH and shared work wrappers
cannot redirect them. no changes apply to ordinary unmarked shells or existing
tmux sessions. a real herdr pane (`HERDR_ENV=1`) makes this integration a no-op
even if it inherited skid's shell marker; upstream herdr owns that pane's setup.
ordinary/herdr commands retain their existing account selection, provider homes,
histories, trust and integrations. do not install global account replacements,
provider-home exports, or a private herdr-home layout for skid's restoration.
arbitrary absolute commands remain deliberate user overrides.

gateway, tmux client startup, native helper, and the new pane's one-shot exec
boundary strip inherited `HERDR_*`. the latter boundary is required because an
already running tmux server can carry its own environment. it never changes
that server or unrelated sessions. provider-command also strips that context
and the shell marker before exec. both forge and manual paths clear the opposite
provider-home variable before selecting their own.

## hooks

skid installs no codex hooks. codex projection already ignores registrations and
uses the foreground process and terminal controls. the unused codex writer and
template are retired; existing `hooks.json` remains untouched by skid setup.
no hook merger is needed. stage the supplied
[`claude-agent-identity`](../deployment/providers/claude-agent-identity) tree at
the generation's `providers/claude-agent-identity`, with its public path supplied
by the stable symlink above. its `bin/agent-hook` is 0755, json files 0644.
both scoped manual claude commands and the forge explicitly load it. the plugin
does not edit shared claude settings. `agent-hook` accepts only `Claude SessionStart`.
after argv validation it returns without reading input or host config unless
`SKIDBLADNIR_AGENT=1` and `HERDR_ENV` is not `1`. the marker is set only at skid's
provider exec boundaries; it does not replace exact pane/tty/pid/start checks.

these hooks register process-lifetime identity only. preserve normal provider
project instructions/trust and native herdr integrations in existing accounts.
the separation integrator may remove only proven obsolete skid registrations
and scripts, matched to the exact historical command or file contents. preserve
unrelated entries in the same hook group, user settings, histories, trust,
plugins and jarvis cognition; an unknown reference stays for its owner to resolve.

qualify actual hook interaction at `cwd=$HOME` and a shared project, including
inline and enabled plugin sources. check skid forge/marked-terminal launches
and ordinary/herdr bare/account launches: each must keep its intended account
and only publish identity to its owning runtime. exercise herdr entry from a
marked skid terminal too. trace any wrong-runtime hook execution to its
registration or launch boundary and fix it there. do not relocate providers,
remove native herdr integrations, add a shared dispatcher, or suppress project
settings to mask an interaction. this live integration evidence remains `NOT_RUN`.

## native helper

consume [`deployment/native-control/pin.json`](../deployment/native-control/pin.json)
and [the installation contract](restoration-native-control.md):

- llm-calling `ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f`;
  uv.lock sha-256 `7566d8859aead7cfa6ae9477f5ea2406d00c860335cbe954cbb320ea330783f5`.
- private uv `0.11.28`, python `3.12.13`, frozen `claude-agent-sdk==0.2.130`;
  `uv sync --python 3.12.13 --frozen --extra claude-sdk --no-dev`.
- install the helper environment beneath `~/.local/share/skidbladnir/` and expose
  its entry point as `~/.local/bin/skidbladnir-provider-runtime-control`.
  stage/verify before switching; retain only skid-owned rollback revisions.
- install [`native-control/claude`](../deployment/native-control/claude) as the
  frozen environment's `bin/claude` (0755). render
  [`the helper launcher`](../deployment/native-control/skidbladnir-provider-runtime-control)
  with `@ENVIRONMENT_SHELL@` equal to the shell-quoted absolute frozen venv path;
  install it as the generation's `providers/native-control` (0755), reached
  through the stable public symlink above. this generation contract supersedes
  the earlier standalone launcher installation. verify the shim,
  launcher, entry point, source/lock and installed versions before activation.

one json request on stdin, one json envelope on stdout, then exit. skid calls
only `inspect`, `read`, `stop`, always with `provider:"Claude"`, selected
`profileKey`, `targets:[{sessionId?,pid?}]`, and optional operation `input`.
success is `{ok:true,result:...}`; errors use
`{ok:false,error:{code,dispatch}}`. [agent control](agent-control.md#output-and-provider-command)
owns the complete result and bounded-read semantics. there is no native send,
daemon, provider lifetime ownership, or copied history store.

the gateway sets the selected existing account's `CLAUDE_CONFIG_DIR` before import and
sets `SKIDBLADNIR_CLAUDE_COMMAND` to that profile's exact native executable.
the installed launcher checks its shim and puts its venv bin first in `PATH`.
the pinned adapter executes bare `claude agents --json --all` and
`claude stop <id>`; the shim execs the selected absolute command without account
flags or home changes. missing native/shim fails without searching shared
wrappers. the sdk reader consults the same home. output is capped at 64 kib; inspect
shares the two-second enrichment budget, controls the ten-second operation
budget. helper termination does not cancel a provider turn.

internal environment contract: `SKIDBLADNIR_SHELL` is optional, absent
outside product terminals, and exactly `1` to activate shell startup; callers
do not use it to select an account. `SKIDBLADNIR_AGENT=1` identifies a skid
provider launch, set by `agent-exec` or `provider-command` only after inherited
values are cleared. gateway, terminal and helper boundaries remove inherited
copies; marked shells do not set it. `SKIDBLADNIR_CLAUDE_COMMAND` is required only
for native helper dispatch, has no default, is set by the gateway from validated
profile config, and is removed before exec of claude. inherited values are
cleared at gateway/forge/terminal boundaries. `HERDR_*` is never a skid input.
provider home variables retain their upstream meaning with the explicit values
above; no home is inferred from cwd.

## qualification and remaining work

source/no-auth qualification for the provider continuity correction: disposable
fake-provider probes first demonstrated the former five private-home redirects,
then verified all existing account selections, personal claude's unset home,
original flags, quoted arguments and launch marker/environment cleanup. bash
and zsh probes covered marked skid shells, ordinary shells and herdr panes.
claude hook probes with open stdin returned silently outside skid and inside
herdr before reading input/config; marked malformed input reached admission.
codex hook invocation returned usage failure. a separate codex projection probe
confirmed identical results with absent, matching and stale registrations.
all temporary probes were removed; none invoked tmux or authenticated providers.
review and `scripts/check verify` passed for this correction: go build/vet,
android lint/debug build, dependencies, generated assets and shell checks.
implementation `3bd0ae468c8feefc1e2eac5ee72085d23ec445e2` and shell-contract
clarification `0bb7e2a521aade3dadec5795fc5c4a726b566c5c` passed exact-source
hosted runs [36211959613](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36211959613)
and [36212232289](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36212232289).

both owners reviewed the six byte-identical provider/config/plugin templates.
dev-server's real render and provider installer, with helper installation
stubbed, preserved all 21 disposable account sentinels and their modes, created
no private accounts or codex hooks, passed the real app config validator, and
produced the agreed generation digest. the former shared installer failed on a
valid symlinked startup file; the correction passed without changing that file.
skid-owned shell setup preserved links, unrelated contents and modes across
repeat installs; bash/zsh login and interactive fixtures covered late overrides
and ordinary/herdr guards. actual provider hook loading remains a live boundary.

prior candidate `f967ac307873f0d326ea5483d74f6683c8a4c7c0` passed complete
engineering checks and exact-source [hosted verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36210567657).
its pinned helper frozen installation, selected absolute-command/disposable-home
routing, missing-native/missing-shim no-fallback checks and native claude
`2.1.282` command availability passed on macbook without authenticated sessions.
those helper checks prove dispatch, not installed-account behavior. its
validator and generation receipt probes also passed; the receipt alteration
and rollback evidence above remains applicable because that code is unchanged.
the published source and exact hosted check are recorded above. source and
release verification are not fleet acceptance.

`NOT_RUN`: three-host forge/manual-provider authentication and hook isolation;
native live status/history/stop; concurrent products; independent restart,
reinstall, rollback, disposable removal; both apps' complete
pairing/launch/attach/control acceptance. live host gates require handback of
occupied paths, installation, and applicable current-turn tmux/device approval.
the root operator owns live progress and evidence. this source work performed
no live tmux, device or service mutation; publication and phone preflight were
completed separately by the root operator as recorded above.

phone transition belongs to the root operator: install/pair distinct
`dev.niels.herdr.mobile` first, force-stop and clear ONLY obsolete
`dev.niels.skidbladnir` data, install the increasing-code signed skid apk, pair
fresh. no downgrade or stored-data compatibility assumption.

costs accepted: shared provider configuration, history and memories across apps;
changing provider account state in either app is visible in the other. one small
exec boundary per agent launch; explicit bash/zsh startup integration; private
helper download/disk per revision; one private claude exec shim; native cli
requalification on provider upgrades. no separate provider login is introduced.
ordinary/herdr account state is retained. shared project hooks still need
integration qualification. tmux ownership is unchanged. open evidence belongs in
[native-control](issues/restoration-native-control.md) and
[runtime](issues/restoration-runtime.md) issue records. the receipt admission
correction, provider continuity and shell installer agreement are complete in
the pushed sources above. these source checks do not assert live acceptance.
