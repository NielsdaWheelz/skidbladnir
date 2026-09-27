# terminal continuity

status: shipped in immutable `v0.10.2` from source
`492058f6433d106645779f09cc13dc3900b625fc`. matching gateways run on
macbook, arch, and devbox. isolated mac ssh/mosh and physical-phone journeys
have partial live acceptance; remaining boundaries are recorded in the
[remote](issues/terminal-continuity-remote-qualification.md),
[darwin](issues/terminal-continuity-darwin-qualification.md), and
[phone](issues/terminal-continuity-phone-qualification.md) issues.

## 1. scope and decisions

| decision | contract | consequence |
| --- | --- | --- |
| remote entry | ordinary `ssh` / `mosh`, with scoped integration on paired hosts | no new user command, transport replacement or remote tmux enrollment |
| reopening remote work | reopen the same original terminal | observations change; attachment routing and original host dependency do not |
| codex identity | configured personal/work/work2 home label only | no model/version, conversation or authenticated-account claim |
| combined stop | retain, labelled `stop agent and close terminal` | preserve native claude halt plus terminal closure; `/exit` remains provider input |

this document owns the implementation delta to [architecture](architecture.md),
[shells](shells.md), [identity](agent-identity-projection.md),
[agent control](agent-control.md), [desktop](desktop-browser.md) and
[directory selection](working-directory-chooser.md). root updates their affected
clauses at cutover; do not maintain contradictory final contracts. retain
[codebase rules](rules/index.md) and [temporary-test policy](rules/testing.md).

goals: persistent terminals, immediate desktop creation, touch-friendly mobile
creation, current cwd/provider/home and ordinary ssh/mosh context tracking.
non-goals: new multiplexer/supervisor/database, generic command execution api,
provider history/model/account inspection, automatic session migration, arbitrary
host enrollment, new provider lifecycle hooks, shell-language evaluation, visual
redesign. no remote attach/control api, forced remote persistence or recovery claim.

## 2. final behaviour and ownership

- one dwarf remains one tmux session. name, portrait, group and exact reference
  survive cwd and agent changes. the current active pane remains the card anchor;
  enumerating every pane is outside this change.
- every new terminal starts the configured interactive login shell. an initial
  agent is one foreground child. `/exit`, normal agent exit and agent failure
  return to that same shell. shell `exit` and deliberate `exec` retain ordinary
  shell semantics. detach leaves the terminal running.
- tui `n` creates at home on the visible target machine and immediately attaches;
  browser-only `N` opens existing launch options. `T` retains exact-source new-terminal-here.
  target = explicit machine filter, otherwise configured default; never choose
  the first reachable peer. inherit the selected named group, as today.
- mobile keeps the forge's machine, launch/account, directory, optional name,
  objective and group controls. creation attaches the confirmed terminal. menu
  choices specify initial state; subsequent terminal commands change observations.
- reuse five-second inventory sampling. unavailable cwd is absent; launch cwd
  never substitutes. no recognized agent means `terminal`, not proof of a shell.
  remove launch-profile fallback from the current-account display; retain the
  historical fact only in existing desktop/cli details.
- tmux owns lifetime; the shell owns job control; providers own execution/history;
  `process` owns native observation; `agentruntime` interprets provider/home facts;
  `sessions` composes them; clients own presentation and cross-host composition.

## 3. capability contracts

### creation and shell startup

keep existing strict requests and responses:

```text
POST /v1/sessions
  {kind:"terminal", cwd, optionalTmuxName?, objective?, group?}
  {kind:"agent", profile, cwd, optionalTmuxName?, objective?, group?}
POST /v1/sessions/{tmuxId}/shell
  {identityToken}
201 {observedAt, session}
```

`kind` describes the initial action, never a permanent session type. both paths
use the existing creation queue and `terminal-exec` directory/shell boundary.
`201` establishes creation, not shell/provider readiness. preserve exact targets,
strict decoding, partial-create outcomes, navigation fences and no mutation replay.
allow omitted generated names through the desktop client; the gateway already
owns name allocation. do not create a second name allocator.

initial-agent composition:

```text
create -> sessions -> tmux -> terminal-exec -> interactive login shell
                                             -> initial agent child -> same shell
```

derive one private envelope from the validated profile:
`{command, arguments, environment}`. preserve configured environment; use
`LaunchArguments` for flags/claude name. base64url-encode json, at most 16 kib
decoded; validate the bound before creation. no generated shell code.
carry the original helper path, requested cwd and intended shell pid separately
through scoped startup variables. reuse `agent-exec` for final child environment
cleanup and provider exec. work-account environment must not leak into the shell.

`terminal-exec` records its own pid immediately before shell exec. bash/zsh
integration installs the callback during startup; it runs after all startup,
before the first prompt. qualify the final effective bash login file and zsh
`.zlogin`, including prompt-hook changes. every new shell consumes its one-shot
cwd action; the agent action is optional. only the intended shell consumes them.
capture/unset startup data and remove the callback before changing cwd or invoking
the helper as a normal foreground child. preserve user prompt callbacks.
re-sourcing, startup-created children, subshells and agent return cannot replay it.
failed directory entry skips the agent and leaves a usable shell with an explicit
error; provider failure also leaves the shell usable, including under `errexit`.
missing integration is a deployment qualification failure, never permission to
use direct agent-as-pane launch. prove real job control before accepting this
mechanism; no guessed prompt delays or injected keystrokes.

### observed codex home

keep the existing agent wire schema and native-method fields. after classification
identifies codex, `sessions/agent.go` calls:

```go
ObserveForegroundEnvironment(terminalPID PID, expected Observation) (map[string]string, error)
```

retain a closed allowlist: `CODEX_HOME`, `HOME`, `SKIDBLADNIR_CONNECTION`; the last
key serves transport observation below. preserve missing versus empty. native APIs
return an environment block: require eof within 1 mib, reading one extra byte to
detect overflow; discard other entries, reject incomplete records and duplicate
requested keys, never log the buffer. revalidate foreground,
tty, pid/start and executable/argv around the read; exec can preserve pid/start.
stable-process read failure omits profile; a distinct foreground-mismatch error
discards/reobserves the whole agent, never retains stale provider identity.

reuse `MatchProfileEnvironment`: explicit home must exactly match a configured
absolute home. empty/relative/unlisted is unknown. an absent home may use observed
absolute `HOME/.codex` only after a complete successful read. never use the
gateway's home, launch metadata, ancestors, provider files or terminal text.
qualify linux and darwin separately. retain claude registration unchanged.
managed native commands and the exact configured stock node launcher are
supported. other wrappers remain unknown unless separately proven. retain the
profile catalogue: it also supplies foreground signatures. this narrowly
supersedes the current ban on codex home projection and other-process
environment observation; it adds no codex hook.

### directory search

ordinary shell `z` uses existing shell setup. optional launch directory entry
accepts existing literal paths or `z <words>`; the explicit prefix opens ranked
results, never launches the top match. words are literal whitespace-separated
terms; quotes, variables and substitutions have no shell meaning. parse this
prefix before path validation; leading whitespace is invalid; bare `z` requests
words; `/path/z foo` stays literal. mobile also
offers `search visited directories` in its existing chooser. both entrances use
one search transition per client. selection fills cwd.

```text
POST /v1/directory-searches {terms: string[]}
200 {directories: string[], omitted: boolean}
```

terms: 1–8 nonempty terms, at most 256 utf-8 bytes combined, no control characters.
`workdir` invokes configured absolute `zoxidePath` with argv
`query --list -- <terms...>` under the service user's existing zoxide environment.
deployment aligns `_ZO_DATA_DIR`/platform data-directory environment with the
user's shell; the gateway never sources shell configuration. no separate history
or crawler.
deadline 2 seconds; stdout cap 64 kib; return at most 64 valid existing searchable
paths within 32 kib, in zoxide order; `omitted` indicates dropped/truncated entries.
reuse existing path validation and validate selection again at creation.
no matches is a successful empty list; missing executable/timeout/failure is typed
`DirectorySearchUnavailable`; excess stdout is `DirectorySearchTooLarge`.
use strict existing machine/auth/error boundaries. machine change invalidates
pending search; late results cannot overwrite a new draft.

host config adds required `zoxidePath` as absolute path or explicit null (disabled).
null produces unavailable, with existing browse/exact-path actions still usable.
query may perform zoxide's normal database bookkeeping; amend the chooser's
read-only claim specifically for this operation. no shell expression endpoint.

### controls and remote work

interrupt retains its existing terminal-key delivery semantics; it does not
prove cancellation. provider `/exit` is ordinary terminal input. close-terminal
retains exact-session deletion and confirmation. no generic agent-exit api.
retain the current `stop` route, result and cli action with explicit destructive
copy. never relabel it as exit. it does not halt remote work merely because a
local transport closes.

### ordinary ssh/mosh context

this is observation only. source machine/ref, attachment, input, close and machine
grouping remain source-owned. remote agent facts never enter source `Session.agent`.
remote controls and remote new-terminal-here are excluded; while connected, label
the existing creation action `new terminal on <owner>`.

1. scoped bash/zsh functions invoke a private helper that mints a fresh 128-bit
   lowercase-hex connection id, exports `SKIDBLADNIR_CONNECTION`, then execs the
   native ssh/mosh with unchanged argv. do not overwrite existing user aliases or
   functions; qualify actual command resolution. absolute/custom bypasses remain
   usable with unknown context. deployment renders absolute native command paths
   into this integration; no recursive function lookup. mosh's internal ssh inherits
   the same id. recognize final `mosh-client`, not just its transient launcher.
   install only inside skid shells, inbound tracked connections or their scoped
   interactive descendants; unrelated shells keep their existing commands.
2. deployment forwards only this variable through OpenSSH `SendEnv` and server
   `AcceptEnv`, or the corresponding Tailscale SSH `acceptEnv` policy. do not reuse
   locale variables or broaden authentication. this is an explicit deployment
   prerequisite; no tailnet-policy automation is added.
3. remote interactive integration consumes/unsets the inbound marker and registers
   its controlling-session leader. verify the invoking shell belongs to that
   leader's tty/session; nested shells do not register again.
   new `terminal-context-init` handles only registration/transport functions;
   do not import provider aliases/home overrides into remote shells. consume the id
   only in an interactive shell with a controlling tty; noninteractive bootstrap
   leaves it intact for mosh-server. retain private `SKIDBLADNIR_TERMINAL_CONTEXT=1`
   for scoped transport functions in nested interactive shells. preserve native
   argv, status, job control, remote commands and exit; never create remote tmux.
4. a private registration is `{connectionId, bootId, pid, startIdentity, tty}`,
   at most 1 kib, atomically stored under
   `$HOME/.cache/skidbladnir/terminal-contexts/<connectionId>.json` (directory 0700,
   files 0600). helper and gateway share one path/codec owner. records hold no cwd,
   provider content, timestamps or credentials. at most 256 live registrations;
   prune dead records during ordinary register/read work. records survive gateway
   restart but supply no authority without current kernel validation. boot identity
   prevents accepting pid/start/tty reuse after reboot. no heartbeat or daemon.
   this is a narrow exception to tmux-only runtime metadata: transient connection
   association, never session lifetime, status or history. amend architecture §1/§4.
5. after native transport classification, source inventory uses the same bounded
   environment reader for its marker. remote gateway requires matching boot,
   pid/start, nonzero tty and leader `SessionID == PID`; foreground session/tty
   must match that root. bracket the whole cwd/environment/provider sample with
   revalidation; instability discards it. invalid root returns unavailable; stable
   missing cwd/home only omits that field. linux `/proc/.../cwd` and darwin
   `PROC_PIDVNODEPATHINFO` remain in `process`.

```text
source session adds connection?: {transport:"ssh"|"mosh", id?:<32 hex>}
GET /v1/terminal-contexts/{connectionId}
200 {observedAt, cwd?, agent?:{provider, profile?}, connection?:{transport,id?}}
404 TerminalContextUnavailable
```

reuse pinned gateway authentication, strict decoding and existing path bounds;
response cap 64 kib. source and remote `agent`/`connection` are mutually exclusive.
a transport has no execution cwd; missing marker means unknown context. remote `agent`
is descriptive: no pid, pane, status, methods, history or mutation target. codex
home matching is the same as local; raw-tty claude profile stays unknown without
its existing tmux-bound registration. unknown remote status stays unknown.

clients resolve a marker across paired gateways and accept one fresh live match
within the budget. offline peers do not invalidate a positive; multiple positives
are ambiguous. this provides sampled description, never control authority.
no hostname/ssh-config parser or gateway-to-gateway calls. follow
onward markers for nested hops, with a maximum of eight and cycle detection;
missing, ambiguous or exhausted resolution is `remote context
unknown`. never present an intermediate transport cwd as the final location.
use existing refresh work, one two-second total context budget per refresh,
coalescing identical lookups. late results require the same source session,
current sampled connection and refresh generation; no persisted client context.

each client derives one execution-context projection keyed by the original session
reference: `local {machine,cwd?,agent?}`, `remote {machine,cwd?,agent?}` or
`remoteUnknown`. cards, agent filters/counts and details consume that projection.
resolved remote cwd replaces displayed source cwd; unknown remote context hides
source cwd/provider. resolve remote profile keys through the destination catalogue,
even when the source catalogue is empty. attachment, availability and action
eligibility still use source facts; a reachable destination cannot replace an
unavailable source.

agent exit leaves the remote shell context; connection exit/suspension removes
it when the source foreground changes. `fg` and nested-hop exit rederive context.
metadata failure cannot block native connections. noninteractive commands, missing
forwarding/integration, unpaired hosts and custom transports remain unknown.
recognized inner tmux/screen returns context unavailable; no screen/title scraping.
qualify stock ssh/mosh bootstrap, reconnect and forwarding on the actual fleet.

mechanism references: [ssh forwarding](https://man.openbsd.org/ssh_config#SendEnv),
[server acceptance](https://man.openbsd.org/sshd_config#AcceptEnv),
[tailscale policy](https://tailscale.com/docs/reference/syntax/policy-file#acceptenv),
[mosh bootstrap](https://github.com/mobile-shell/mosh/blob/master/scripts/mosh.pl).

## 4. client configuration and content

desktop private configuration becomes `{peers:[...], defaultMachine:<handle>}`;
the required handle must name a peer. fleet provisioning writes each desktop's
own known fleet handle as its default. mobile's explicit machine picker remains.
re-render configuration at cutover; no legacy reader or implicit default.

one content designer owns the following feature copy and acceptance before each
builder starts. good content names the target/effect, separates observation from
certainty, fits existing chrome and works through accessibility without colour.
configured profile labels and current design tokens remain authoritative.

| feature | content contract and designer's quality criterion |
| --- | --- |
| desktop creation | `n terminal on <host>`; `opening terminal on <host>…`; `<host> unavailable`. destination visible before acting; `N` options and `T` here are discoverable |
| mobile creation | preserve forge and `create on <host>`; selected directory/account visible; no required terminal typing; existing 48dp targets and traversal |
| identity/cwd | configured profile label; `<provider> · profile unknown`; `terminal` means no recognized agent in resolved context. existing desktop/cli details: `agent: not detected`, `directory unavailable`, `started with: <profile>`. abbreviate cwd visually only; no repeated polling announcements |
| search | `search visited directories`, `searching…`, `no matching directories`; unavailable: `directory search unavailable on <host>`; overflow: `too many results; narrow your search`; omitted: `some directories are not shown`; malformed: `enter 1–8 search words`. show full ranked paths and host; selection only edits draft |
| controls | local-agent controls: `interrupt agent`, `interrupt key sent`, `interrupt outcome unknown`, `stop agent and close terminal`. terminal control: `close <name> on <host>?`, `close terminal` / `cancel`; remote input remains ordinary terminal input |
| uncertain creation | `creation outcome unknown. check the session list before creating another.` preserve current attachment/draft; never imply a retry is harmless |
| remote | `running on <host>`; `terminal on <owner>`; `remote context unknown`; `new terminal on <owner>`. show destination beside remote facts, even in owner-filtered views; spoken context includes both hosts/full cwd; close names owner. no remote agent controls; missing cwd alone means `directory unavailable` |

## 5. exclusive work boundaries

root freezes contracts before parallel edits. directory paths below are relative
to the repo; phone paths are under `android/app/src/main/java/dev/niels/skidbladnir`.
no two implementation owners edit one file. designers/reviewers propose changes;
only the assigned builder writes them. each owner covers its module across local
and remote features; remote work introduces no overlapping slice.

| owner | exclusive files / responsibility |
| --- | --- |
| a: terminal lifecycle/context | `cmd/skidbladnir/{terminal_exec,agent_exec}.go`, new private `cmd/skidbladnir/terminal_context.go`, `internal/sessions/{manager,types}.go`, `internal/tmux/client.go`, `internal/runtimeenv/`, new `internal/agentruntime/launch.go`, new `internal/terminalcontext/`, `deployment/providers/{shell-init,provider-command,terminal-context-init}`; shell startup, connection registration and composition |
| b: native observation/identity | `internal/process/`, `internal/agentruntime/profile.go`, `internal/sessions/agent.go`; native cwd/environment, provider/home matching; reject newly reserved startup/connection variables in configured provider env |
| c: directories | `internal/workdir/`; bounded zoxide query and existing path contracts |
| d: desktop | `internal/{fleetclient,agentcli,sessionui,terminalclient}/`; request/response/config admission, creation, search, content |
| e: phone | `ForgeSheet.kt`, `WorkingDirectoryPicker.kt`, `WorkingDirectoryPickerScreen.kt`, `ProductModel.kt`, `SkidbladnirController.kt`, `GatewayClient.kt`, `SessionCard.kt`, `TerminalScreen.kt`, `AgentControl.kt`, `DashboardScreen.kt`, `Chrome.kt`, `SessionRename.kt`; retained forge, search and current-context composition/content |
| root: composition | `cmd/skidbladnir/main.go`, `internal/{gateway,hostconfig,agentcontrol,logging}/`, `deployment/providers/host-config.json`, `scripts/fleet`, docs; shared schemas, configuration, controls and final composition |
| f: installer | separate `dev-server` worktree: `lib/{skidbladnir,skid-provider,gateway-runtime}.sh`, `assets/{skidbladnir,skid-provider,dotfiles}/` exact installer assets; rendered paths, startup files and eleven-file receipt producer. ssh marker forwarding remains an external deployment prerequisite |

reuse existing create-here, path validation, exact references, mutation fences,
foreground matching, profile matching, provider argv construction and attachment.
delete direct agent-as-pane creation, mandatory desktop-form dispatch and
current-profile launch fallback. retain advanced form and agent exec boundary
where still called. remove code only after its last caller is removed; no unrelated
hook cleanup or general framework extraction. deployment startup installation
remains owned by `dev-server` and its separate installer slice; producer and
fleet receipt consumer must agree before cutover.

## 6. delivery and adversarial acceptance

sequence: review contracts/content -> native/launch/directory services -> gateway
composition -> clients -> whole journey. owners may work concurrently only against
agreed interfaces. root alone updates architecture/roadmap and check
composition; no new permanent behavioral gate or catalogue/artwork work.

for **each** feature: designer defines good content within §4; independent
reviewer challenges requirement, ownership, failure behaviour and test sensitivity;
builder writes temporary end-to-end integration/live proof and demonstrates the
specific baseline red; implement to green; review code and evidence; refactor
within rules; rerun affected proof; reviewer checks final diff and omissions;
delete temporary tests/harnesses before commit; run `scripts/check verify`.
working invariants are preservation checks, not fabricated baseline failures.
verifiers write no production file or test. retain only content-free results and
limitations in review records. unresolved defects get one `docs/issues/<name>.md`
each and block their acceptance; delete resolved records.

| proof boundary | acceptance |
| --- | --- |
| gateway -> isolated tmux -> real bash/zsh pty | terminal and initial-agent create share shell lifetime; agent exit/failure leaves same shell; cwd/argv survive special characters; ctrl-z/fg, input and resize work; repeated/startup-child sourcing launches once |
| process -> inventory -> both client projections | personal/work/work2 reflect exact native process; provider swaps clear stale fields; launch profile never wins; absent/empty/unlisted/inaccessible homes stay honest; `HOME` before the read cap with `CODEX_HOME` beyond it stays unknown; background agents do not become foreground; stale actions reject replacement |
| client -> gateway -> real host zoxide/path admission | ranked results use that host's existing database; literal paths remain literal; selection only edits cwd; no matches, disabled/missing tool, bounds, removed directory and host-switch races behave as specified |
| real desktop attachment | `n` creates/attaches without form; target/default and offline failure are correct; `N` retains options; `T` samples source cwd/group; pending duplicate submissions are suppressed, uncertain creation is never replayed, deliberate retries may create another terminal |
| actual phone -> gateway -> provider | forge remains usable without terminal navigation; agent launch -> `/exit` -> shell -> another provider preserves same dwarf; search, copy, close confirmation and post-create attachment are usable |
| real ssh/mosh -> remote tty -> both clients | actual command resolution/options preserved; distinct simultaneous connections/nested hops; remote cd/z and provider swaps; ctrl-z/fg/hop exit; gateway restart during a busy agent; `/exit` retains shell; detach/reopen retains original attachment; source loss, no integration and inner tmux show honest limits; no remote action retargeting |

remote adversarial cases: stale/reused registrations, wrong tty, late results after
replacement, ambiguous matches, unrelated offline peer, reconnect, oversized
environment, and source-empty/destination-populated profile catalogues.

qualify native observation on linux and darwin, and actual fleet bash/zsh startup
paths. deterministic helper programs establish integration mechanics; stock
providers and actual phone establish their own live boundaries. use only
test-owned isolated `-L` tmux resources. tmux/live and phone/adb work require
explicit current-turn approval under `AGENTS.md`; this document runs no tests.
unexecuted boundaries are `NOT_RUN`, never passes. no credentials, terminal bytes,
prompts, objectives, account data or environment buffers in retained evidence.

hard cutover: one coordinated source/client/config release, no compatibility
reader, duplicate launch path, retry fallback or automatic terminal conversion.
existing running terminals are not migrated; the shell-lifetime guarantee applies
to new creation. rollback restores the matching release/config set. release,
installation and activation are separate from this implementation plan.

accepted costs: shell-specific startup and deployment qualification; required
default-machine/zoxide configuration; zoxide bookkeeping; bounded native environment
reads; remote shell integration, forwarding and transient registrations; sampled
context; home labels without authenticated identity; remote display without remote
controls/status or persistence; unsupported wrappers/inner multiplexers stay unknown.
temporary proofs leave no retained behavioral regression suite.
