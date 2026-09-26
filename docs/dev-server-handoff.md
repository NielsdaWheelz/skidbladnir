# skidbladnir deployment contract

2026-09-25 restoration source in `/Users/nnandal/Documents/code/skid-v1`, based on
`927c55412eec7fa129a3325fb8f5ebf8051b6ea0`. publication belongs to the root
operator; live installation waits for host namespace handback. this file's
containing commit owns the source contract. dev-server is the sole installation owner. this file
is the contract for its separation agent; the other product is herdr-mobile.

owner correction: isolation belongs only to original-skid launches and its
marked terminals. herdr and ordinary shells retain their existing provider
binaries, account commands, homes and histories. the private herdr-home plan
is withdrawn. preserve `.codex`, `.codex-work`, `.codex-work2`, `.claude`,
`.claude-work`, their native herdr integrations, and existing user state.
only skid's five homes below start fresh; no provider relocation is required.

## release and namespace

original github repository id: `1386409483`; reclaimed name:
`NielsdaWheelz/skidbladnir`. read-only github verification on 2026-09-25 confirms
the other repository, `1342599607`, is now `NielsdaWheelz/herdr-mobile`.
`scripts/check-release-repository immutable` passes: the original public
repository has immutable releases enabled. neither github name transfer nor
that setting remains a blocker. no original-product release or tag exists yet.
the local checkout remains `skid-v1`. live namespace handback still belongs to
the root operator; the github rename does not authorize installing into occupied
skid paths.

published release-pin reference: **unavailable**. the stale `v0.6.0` pin was
removed because its artifacts belong to the other repository. do not recover
it from history for installation or rollback. after publication this repository
will add `release-pin.json` containing the exact version, source sha, and five
asset digests; update this section with its immutable source reference then.
planned release: `v0.9.0`, android `0.9.0` / `9000`, subject to a fresh installed
version observation and tag availability. package: `dev.niels.skidbladnir`.
the local signing configuration passed `scripts/check-android-signing --config-only`;
the installed phone signer/version observation remains pending. retained
signing certificate sha-256:
`7b2ba254e9d3cb18044b723fe124dec87b727ab56e817860bb48e056eddc47bf`.

before publication require the reclaimed name's numeric id, immutable releases,
and a successful `verify.yml` push run for the exact release source. the source
tools enforce these through `scripts/check-release-repository`.
`scripts/check-release-source v0.9.0` returns the clean `origin/main` source sha
and its successful hosted run id once the pushed commit passes. obtain that
fresh result before the root operator prepares the draft; publication does
not wait for live host handback. no signer or state from herdr-mobile is an
input to skid installation or verification.

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
  names unique, no other provider's home, `HERDR_*`, `SKIDBLADNIR_SHELL`, or
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

all homes are private regular directories beneath
`HOME/.local/share/skidbladnir/providers/`, created `0700`. initialize empty and
authenticate/trust normally. no account-tree, socket, cache, or credential copy.

| forge key | provider home basename | native argv after executable |
| --- | --- | --- |
| `personal` | `codex-personal` | `--yolo` |
| `work` | `codex-work` | `--yolo` |
| `work2` | `codex-work2` | `--yolo` |
| `claude-work` | `claude-work` | `--name <tmuxName> --dangerously-skip-permissions --plugin-dir HOME/.local/share/skidbladnir/claude-agent-identity` |

codex rows set only their `CODEX_HOME`; claude sets `CLAUDE_CONFIG_DIR`.
`claude-personal` is a fifth home for manually typed commands, never a forge
profile. its unconfigured runtime profile remains unknown and uses terminal
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
dev-server records its acknowledgment and admission correction in local commit
`94931a13c045fa9c9b294080149524501ff30b68`,
[runbook](/Users/nnandal/Documents/code/dev-server/docs/gateway-separation-runbook.md#generation-receipt-contract)
and [validation](/Users/nnandal/Documents/code/dev-server/docs/gateway-separation-validation.md).
that deployment commit is not yet pushed. its stale six-file note is removed.
the revised dev-server runbook and issue record retain this acknowledgment
and close the directory-mode/digest-suffix discrepancy. withdrawing private
herdr homes changes neither the ten-file sequence nor its digest or mode rules.
a disposable fixture
used dev-server's actual `gateway_runtime_identity` and
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

new skid terminals set `SKIDBLADNIR_SHELL=1` and both private personal home defaults
before the configured login shell. at the END of that shell's ordinary startup,
source the installed `shell-init` when that marker is `1`. it removes any
`HERDR_*` introduced during startup and replaces shared aliases with product-local
functions. zsh requires the end of `.zshrc` and,
if it changes provider setup, the later `.zlogin`; bash requires the actual
login file (`.bash_profile`, `.bash_login`, or `.profile`) and `.bashrc` for
interactive subshells. do not assume bash login reads `.bashrc`. current fleet
shell support is bash/zsh; configure and qualify its actual startup path.

within marked skid terminals, bare `codex`/`claude` select the private personal
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

render [`agent-hooks.json`](../deployment/providers/agent-hooks.json) into each
of the three private codex homes as `hooks.json` (0600). `@HOOK_COMMAND@` is the
shell-quoted command `HOME/.local/bin/skidbladnir agent-hook
--host-config=HOME/.local/share/skidbladnir/current/host-config.json Codex SessionStart`,
serialized as a json string. stage the supplied
[`claude-agent-identity`](../deployment/providers/claude-agent-identity) tree at
the generation's `providers/claude-agent-identity`, with its public path supplied
by the stable symlink above. its `bin/agent-hook` is 0755, json files 0644.
both manual claude commands and the forge explicitly load it.

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

the gateway sets the selected private `CLAUDE_CONFIG_DIR` before import and
sets `SKIDBLADNIR_CLAUDE_COMMAND` to that profile's exact native executable.
the installed launcher checks its shim and puts its venv bin first in `PATH`.
the pinned adapter executes bare `claude agents --json --all` and
`claude stop <id>`; the shim execs the selected absolute command without account
flags or home changes. missing native/shim fails without searching shared
wrappers. the sdk reader consults the same home. output is capped at 64 kib; inspect
shares the two-second enrichment budget, controls the ten-second operation
budget. helper termination does not cancel a provider turn.

new internal environment contract: `SKIDBLADNIR_SHELL` is optional, absent
outside product terminals, and exactly `1` to activate shell startup; callers
do not use it to select an account. `SKIDBLADNIR_CLAUDE_COMMAND` is required only
for native helper dispatch, has no default, is set by the gateway from validated
profile config, and is removed before exec of claude. inherited values are
cleared at gateway/forge/terminal boundaries. `HERDR_*` is never a skid input.
provider home variables retain their upstream meaning with the explicit values
above; no home is inferred from cwd.

## qualification and remaining work

source/no-auth qualification so far: pinned helper frozen installation, selected
absolute-command/private-home subprocess routing, missing-native/missing-shim
no-fallback checks, and native claude `2.1.282` command availability passed on
macbook; no authenticated sessions were touched. runtime exec environment and
validator valid/invalid/usage probes passed. 28 disposable bash/zsh command
selection cases preserved ordinary unmarked aliases, selected every private
account, and preserved quoted arguments; both shells cleared context after
startup. both also left genuine herdr panes unchanged with an inherited skid
marker. codex `0.157.0` reported hooks enabled in an empty disposable home;
hook execution itself remains unqualified. release identity checks reject the
former wrong repository; the reclaimed name now has the required numeric id.
complete engineering checks passed with `scripts/check verify` (go build/vet, android
lint/debug build, dependency/generated-asset and shell checks). a disposable
generation probe demonstrated the former verifier's omissions and confirmed
that altered shell assets, helper launcher and plugin files are rejected under
the ten-file receipt contract above. the actual gateway helper subprocess
received the selected private home/native command with foreign context absent.
temporary probes were removed. source preparation is not fleet acceptance.

`NOT_RUN`: three-host forge/manual-provider authentication and hook isolation;
native live status/history/stop; concurrent products; independent restart,
reinstall, rollback, disposable removal; phone version/signer observation and
both apps' pairing/launch/attach/control. live host gates require handback of
occupied paths, installation, and applicable current-turn tmux/device approval.
the phone version/signer preflight needs current-turn device approval and does
not depend on host handback. no live user
tmux, device, release publication, or service mutation occurred here.

phone transition belongs to the root operator: install/pair distinct
`dev.niels.herdr.mobile` first, force-stop and clear ONLY obsolete
`dev.niels.skidbladnir` data, install the increasing-code signed skid apk, pair
fresh. no downgrade or stored-data compatibility assumption.

costs accepted: new authentication/trust setup for five private homes; one small
exec boundary per agent launch; explicit bash/zsh startup integration; private
helper download/disk per revision; one private claude exec shim; native cli
requalification on provider upgrades. these costs belong to original skid;
ordinary/herdr account state is retained. shared project hooks still need
integration qualification. tmux ownership is unchanged. open evidence belongs in
[release](issues/restoration-release.md),
[native-control](issues/restoration-native-control.md) and
[runtime](issues/restoration-runtime.md) issue records. the receipt admission
correction and contract acknowledgment are complete; neither asserts live acceptance.
