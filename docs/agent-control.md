# agent control

2026-09-12 · accepted v1 implementation target.
product scope follows the user's reviewed decisions; this document fixes the
engineering contract. the existing jarvis → native codex → tmux → phone
flow is manually confirmed by the user. that is baseline evidence, not evidence
for the new cross-provider controls.

## outcome and scope

from any configured linux/darwin computer, phone, or agent: find an agent on a
peer, read its output/status, send input, create another session, interrupt work,
and stop it. jarvis uses the same controls. coordinator is a prompt, never a role,
session type, ownership relationship, or permission class.

- discovery remains ordinary tmux sessions, including manually created sessions.
  one agent per session: the active pane in its current window. no enrollment or
  mandatory launcher. other panes/windows and agents outside tmux are excluded.
- every configured computer exposes the same local host interface and can call
  every peer directly. no gateway proxy, devbox hub, or tailscale discovery.
- acceptance hosts: macbook, arch, devbox; existing configured codex/claude
  profiles. the cli accepts explicit additional peers. phone enrollment remains
  the existing three hosts; a fourth-host installer/product flow is deferred.
- phone: existing collection and stock terminal, new status labels and interrupt/
  stop actions. existing terminal input handles conversation and dialogs.
- no search, unread/attention system, persistent placement, transcript/chat ui,
  task schema, handoff generator, worktree manager, scheduler, offline queue,
  agent ownership rules, new directory policy, new permission framework, history
  replication, lifecycle database, or generalized hook/plugin framework.

## ownership and composition

```text
human / codex / claude / jarvis
  -> skidbladnir agent <operation> -> configured peer's https gateway
phone ---------------------------> configured peer's https gateway
                                     -> sessions / tmux / process observation
                                     -> agentcontrol
                                          -> native-control command -> provider runtime
                                          -> tmux text / keys / screen
```

1. **skid owns discovery and terminal operations.** extend existing session,
   process, identity, cwd, profile, authentication, and attachment primitives.
   dependency direction: `sessions` → `tmux/process/agentruntime`;
   `agentcontrol` → `sessions` + native helper; `gateway` → both.
   `agentcontrol` enriches inventory after the session lock is released;
   `sessions` never imports it. handlers only decode/map/call.
2. **llm-calling owns native provider protocols.** install one `provider-runtime-control`
   command: one json request on stdin, one json result on stdout, then exit.
   no daemon or supervisor. it imports the existing codex control client and a
   narrow claude status/history reader. prompts never appear in command arguments.
3. **dev-server owns installation.** add the command's absolute path to host
   configuration as `nativeControlPath`; add generated `nativeEndpoint` to codex profile rows,
   sourced from the existing shared-service mapping. reuse profile environments;
   no second authored profile/account table.
   set a selected profile's environment before starting/importing its helper.
   claude personal uses the native default: its row omits `CLAUDE_CONFIG_DIR`,
   its explicit `claude-personal` wrapper unsets any inherited value, and the
   helper clears inherited provider roots. setting that variable to `~/.claude`
   changes native settings/credential lookup. an empty value is not the default;
   no credential or onboarding-state migration is performed.
4. **the cli owns fleet routing.** one reusable go client calls the same gateway
   locally/remotely. jarvis invokes this installed cli with structured stdin and
   consumes json; it does not gain another http/provider/tmux implementation.
5. **providers retain history and execution.** tmux retains terminal inventory.
   helpers/connections never own worker lifetime. gateway restart re-lists tmux.

baseline trap: jarvis currently pins provider-runtime at
`69d41d38a3d290e7ae3bde9b57556dda41e1b2f1`; its `codex_control.py` exists in that
git object but not in the inspected sibling working tree (`4ddced3`). build from
the working pinned api in an isolated checkout; reconcile repository baselines
before updating dependency pins. do not overwrite unrelated work or reconstruct
that api from the older checkout.

## identity, state, and dispatch

reuse the existing machine handle and tmux session lifetime token. add the
observed pane id and foreground process start identity to the existing agent
projection. a full operation target is:

```text
target = { machine, tmuxId, identityToken, paneId, pid, startIdentity }
status = { state, source, reason? }
state  = working | blocked | idle | done | failed | stopped | unknown
source = native | terminal | unavailable
reason = permission | input | dialog | provider_unavailable | unrecognized
```

existing provider/profile/native-session fields remain optional identity facts.
the caller echoes the returned target; the host resolves provider credentials and
native identity itself. names are display labels. never match native sessions by
cwd, guessed conversation name, or caller-supplied account path.
the target is assembled from existing inventory fields: `machine.handle`,
`session.{tmuxId,identityToken}`, and
`session.agent.{paneId,pid,startIdentity}`. no duplicate target object is stored.

revalidate the tmux lifetime, pane, and foreground process before a command. if
the current pane/process changed, return stale; do not target its replacement.
reuse existing session mutation checks. no leases, ownership graph, durable
receipts, or protection against a hostile same-user process.

state selection is independent of send/read selection:

- codex: native active flags distinguish work from approval/user-input waits;
  use known latest native turn outcomes for done/failed/stopped. idle alone means
  ready, never successful work.
  `notLoaded` retains native history but selects terminal send/interrupt and
  terminal-derived status: the configured server does not own its execution.
- claude: `claude agents --json --all`, once per selected profile; match observed
  pid and available native session id. map busy/waiting/idle and background state.
- otherwise: a small checked-in detector per provider examines the live bottom
  screen/title. recognize explicit working/blocker/idle chrome; unmatched means
  unknown. start with explicit rules, no configurable rule language, smoothing
  state machine, remote manifest updater, or hook lifecycle tracking.
- done is native turn completion, never herdr-style unread state. terminal
  detectors report idle instead. unknown and host-unreachable remain distinct.

collect tmux identity under the existing manager lock, then release it before
provider calls. batch native status once per profile per refresh; a slow/failed
profile degrades its own observations. retain the existing five-second foreground
phone polling and stale-machine behavior; no new poll daemon. status is a sampled
observation, not an atomic fleet snapshot.

bound status enrichment to two seconds per refresh, concurrent across profiles;
other control operations to ten seconds. stop reserves two seconds for closing.
client timeout is fifteen seconds, matching the phone. slow work returns partial
or unknown; cancelling a helper never implies cancelling its provider turn.

choose a method before a write. unsupported/unavailable native capability can
select terminal control before dispatch. an attempted native write with an
uncertain result must never trigger a second terminal write.

## capability and api contract

keep existing routes, auth, machine header, strict decoding, and error envelope.
extend `GET /v1/sessions` agent rows with `paneId`, `startIdentity`, `status`, and
`methods: {read, send, interrupt}`; each method is `native | terminal | unavailable`.
the existing response observation time and stale handling apply. method fields
are observations, not promises that survive process replacement.

`POST /v1/sessions` remains creation. `objective` remains card metadata; it is
not an initial prompt. creation returns a terminal, not provider readiness.
poll for an addressable agent; terminal automatic send also requires recognized
idle/working chrome. native send uses native admission. no combined transaction.

new route: `POST /v1/sessions/{tmuxId}/agent/{operation}`. body contains the
target's `identityToken`, `paneId`, `pid`, `startIdentity`, plus operation inputs:

| operation | additional input | success/result |
|---|---|---|
| read | `mode: auto \| terminal`, `maxBytes` (default 16384, max 32768) | `{text, source, scope, truncated}` |
| send | `text`, `mode: auto \| terminal` (default auto) | `{method, outcome, turnId?}` |
| keys | `keys`: 1–16 logical keys | `{method: terminal, outcome}` |
| interrupt | none | `{method, outcome, turnId?}` |
| stop | none | `{agent, terminal}` as defined below |

read source is native/terminal; scope is `recent_messages | latest_turn |
terminal_history | visible`. a bounded/partial source is explicit even when
the returned text itself needed no truncation. no pagination/session store in v1.

write outcome is `accepted | written | interrupted | finished | unknown`.
accepted means native acceptance; written means terminal input delivery only.
reuse native turn identifiers when returned. invalid input, missing/stale targets,
blocked automatic submission, and unavailable methods use explicit api failures
before dispatch. after a possible effect preserve unknown or partial outcomes.
transport loss after dispatch is unknown, never a successful empty result or an
automatic retry. reuse the current 64-kib request bound; cap send text at 32 kib
and encoded control responses at 64 kib. trim read text to fit, marking truncation.

terminal send uses an argv-safe tmux input primitive: a unique buffer per send,
loaded through stdin, bracketed paste when enabled, then submit and delete only
that buffer. do not use
shell interpolation, the user's unnamed clipboard, or a second terminal attachment.
keys are `enter, escape, ctrl-c, up, down, left, right, tab, backspace, page-up,
page-down`; translate in one tmux owner. dialogs use read(mode=terminal), keys,
or send(mode=terminal) for deliberate text replies. automatic send rejects a
known dialog or unclassified terminal; explicit terminal send can address either.
peer agents may answer dialogs, including permissions, under existing host-user
authority. there is no additional human-only worker approval policy.

interrupt: codex uses one exact native turn cancellation when available;
otherwise send the provider's interrupt key and report written. never claim that
a sent key proves cancellation. no implicit wait, retry, or queued follow-up.

stop: validate the full target; attempt one provider-specific halt, then close
the exact tmux session using existing kill. codex interrupts its observed turn;
a matched claude
background job uses native stop; ordinary claude ends with its terminal.
return `{agent: stopped | interrupted | idle | unconfirmed,
terminal: closed | unconfirmed, reason?: stale | unavailable}`. ordinary claude
reports stopped only after native confirmation or observed foreground-process exit.
the exit shortcut additionally requires this operation's native observation to
match an ordinary interactive claude process; an attachment client's exit does
not establish that its background job stopped.
final close revalidates session/pane lifetime, allowing the halted process to
have exited/returned to its shell, but refusing a replacement live agent.
resolve the current tmux name on the host for existing kill. preserve both step
outcomes if closing fails. without native halt confirmation, closing alone leaves
agent unconfirmed. an already absent original session counts as closed. never kill
the shared provider server, chase successor turns, delete history, promise
descendant cleanup, or fence future input. existing terminal-only delete remains
available and retains its narrower meaning.

## output and provider command

native command request:

```text
{operation, provider, profileKey, endpoint?, targets, input?}
native target = {sessionId?, pid?, turnId?}
```

operations: `inspect/read/send/interrupt/stop`. codex requires endpoint and native
session id; claude forbids endpoint and inspect can match pid. inspect batches
one profile; other operations require one target. operation inputs reuse the
public fields, excluding mode. stdout uses the cli result/error envelope: inspect
returns input-ordered per-target status/error; read/send/interrupt return their
public result; stop returns only native halt outcome. gateway adds terminal
closure. errors distinguish unsupported, unavailable, stale, rejected, unknown;
an entire helper failure degrades that profile. no fleet bearer or skid import.
for codex interrupt/stop, absent `turnId` means the host observed no turn; a
subsequent turn is stale, never newly selected for cancellation. claude inspect
may return internal `terminalOwnsAgent: true` only for an interactive row matched
to the exact supplied pid. this transient fact is not added to public inventory.

- **codex:** reuse pinned `CodexControl.list/read/prompt/interrupt` and its
  observe-only transport. existing read retrieves the latest answer with bounded
  coverage, not arbitrary history. first reuse it; extend the same module to
  collect bounded recent text items if its api permits. no duplicate json-rpc in go.
- **claude:** use the sdk's persisted-message reader, with the profile environment
  set before import. qualify it against two separate profile roots. do not weaken
  the existing isolated-cognition adapter or invoke resume/query to read. native
  send is not a blocker: terminal paste/keys are the v1 implementation.
  the sdk currently loads a saved file before slicing messages: output and
  elapsed time are bounded, but transient helper memory scales with transcript
  size. retain the public reader rather than copying its transcript parser.
- **terminal:** capture available scrollback plus screen as a bounded tail.
  fullscreen history may be absent. bounded application scrolling is an optional
  follow-up, not a release gate; report visible-only coverage honestly.

reads never copy histories to another store. native history can lag live output;
terminal capture can be incomplete. missing native identity uses terminal methods.
keep SessionStart content-free and identity-only. logs/evidence contain no prompts,
terminal output, transcripts, credentials, or raw provider errors.

## clients, configuration, and presentation

`skidbladnir agent list|start|read|send|keys|interrupt|stop` accepts one json object
on stdin and prints one json result. list accepts optional `machine`; omitted
means all configured peers. start takes `machine, profile, cwd, name?`; other
operations take `target` plus the inputs above. list returns independent per-peer
results, including unavailable peers, rather than failing the entire fleet.
no wait command in v1: agents can list/read again. human terminal opening continues
through ordinary tmux/ssh and skid's existing attachment path.
inventory is bounded separately from control replies: the aggregate cli list
allows at most 1 mib of encoded output; exceeding it returns `output_limit`,
never an incomplete list presented as complete. other cli results retain 64 kib.
each peer's cli inventory response has the same 1-mib bound. the phone retains
its existing 64-kib response bound.

private client configuration:

```json
{"peers":[{"label":"arch","origin":"https://host:8443",
"machine":"mh-...","bearer":"..."}]}
```

labels are unique; handles/origins use existing validation. profiles come from
the host. add one explicit client config path option, not environment-driven
routing. deployment/operator code writes mode-0600 copies for each trusted user;
jarvis gets its own readable configuration under its service configuration owner.
reuse existing pairing/bearer provisioning, with no provider credentials in the
client file. extending the peer list is explicit configuration. bearer rotation
requires refreshing copies; no synchronization daemon.
provisioning is sequential: a failed copy can leave a partial update. fix the
transport and rerun; there is no distributed configuration transaction.

machine selectors for list/start are configured labels; target.machine is the
canonical handle. reject unknown selectors before dispatch. cli name maps to the
existing creation name field. stdout is exactly `{ok:true,result}` or
`{ok:false,error:{code,dispatch:not_sent|unknown}}`; exit 0/1 respectively.

jarvis replaces worker-facing `codex.*` tools with `agent.*`, retaining its action
recording. invoke the absolute cli directly with `--client-config <path> agent
<operation>` and json stdin; bound stdout and suppress content-bearing stderr.
timeout/child loss after possible dispatch is unknown; killing the client does
not cancel remote work. no shell or automatic write retries.
no hardcoded devbox, provider restriction, coordinator exclusion, or worker
work-root policy. preserve unrelated jarvis cognition, kernel, and six-table
durability and existing host cwd validation. cognition without a tmux session is
outside this inventory naturally.
retain only a read-only decoder for existing durable codex uncertainty results;
removing old execution does not erase canonical action history. no legacy tool
or dispatch path remains.
jarvis projects inventory into a closed tool schema containing peer outcomes,
profiles, session labels/cwd, full targets, provider, status, and methods. it
omits phone presentation metadata; the cli still returns the full host inventory.

android replaces active/quiet labels with semantic status, marking inferred
status unobtrusively. use machine/name/id ordering; no urgency sorting, search,
or saved placement. shell-only sessions remain visible as terminals. add interrupt
and stop to existing actions; stop reuses existing destructive confirmation.
read/send/keys remain available to tools; phone interaction uses its native
terminal. preserve attachment, keyboard, selection, sizing, pairing, and polling.

## delivery boundaries

each row owns its production paths and matching tests exclusively. new paths are
proposals; do not build abstractions beyond the named operations. root wires
shared entry points, updates pins/spec precedence, and integrates. builders own
one behavior-focused failing proof before implementation; reviewers write no code.

| slice | exclusive paths / responsibility | dependency |
|---|---|---|
| a · native operations | llm-calling `src/provider_runtime/agent_runtime/{codex_control,codex_app_server,native_control_cli}.py`, new claude reader, matching tests | pinned baseline |
| b · host | skid `internal/{agentcontrol,sessions,tmux,agentruntime,gateway,logging,hostconfig}/`, matching tests; api/state/terminal dispatch | a contract; terminal work independent |
| c · common cli | skid new `internal/{fleetclient,agentcli}/`, matching tests | b wire contract |
| d · jarvis | jarvis `src/jarvis/` worker tool/control composition, settings and callers, matching tests/qualification scripts; preserve unrelated cognition | c |
| e · phone | android `GatewayClient`, `ProductModel`, `SkidbladnirController`, `SessionCard`, `TerminalScreen`, related action files/tests | b |
| f · deployment | dev-server skid/codex installer, assets, ansible tasks and tests; skid `scripts/fleet`, `scripts/fleet-test`; exclude root-owned manifests below | a–d |
| root · integration | skid `cmd/skidbladnir/main.go`, docs, `scripts/test`; all changed `pyproject.toml`/lockfiles, sibling specs/adrs, profile/release manifests | all |

root-owned dev-server manifests are `assets/codex/profiles.json` and
`assets/skidbladnir/{host-config-*.json,release-pin.json}`; slice f excludes them.
other slices hand root changes for root-owned files; they never edit those paths.

first establish one codex and claude control path through host+cli; then jarvis,
phone, all-host deployment, and retirement. no broad refactor or cleanup slice.
ship gateway, phone, cli and the pinned helper together under existing release
rules; add no old/new contract compatibility branch.

fleet release agreement compares version, source commit and platform. runtime
integrity separately compares installed files with the installer's active digest;
the generation suffix is that runtime digest, not the downloaded archive digest.
the installer continues to verify the archive against the published release pin.

the helper installation uses a private uv 0.11.28 bootstrap, python 3.12.13,
and its pinned source's frozen `claude-sdk` environment. this adds an initial
download and retained per-revision disk space. jarvis's unchanged cognition
dependency remains on its existing qualified revision.
consolidate only replaced paths:

- remove old active/quiet wire projection, activity-only sorting/labels/tests,
  and activity sampling/required timestamp validation after the status cutover.
- replace jarvis's dedicated worker launcher/native routing. retain/extract
  `CodexHostConfig` cognition fields; its whole class is not dead.
- remove the dedicated jarvis launcher socket/services and launcher-only logic
  in dev-server only after their last caller is removed. retain shared codex
  supervision, account wrappers, process observation, and identity hooks.
- update tests that explicitly forbid semantic status. do not resurrect retired
  hook/lifecycle machinery to satisfy historical tests.

required coupled rollout: drain old non-terminal jarvis worker actions and stop
jarvis; publish/pin and install gateway/cli/helper/phone together; run
`scripts/fleet provision-clients`; activate the new jarvis with its schema-3
cognition config and explicit cli/config paths; then retire the old installed
launcher. this requires a brief jarvis outage. removing source assets alone does
not retire installed units. as the existing dev-server deployment principal:

```sh
sudo systemctl disable --now jarvis-codex-launcher.socket
# after confirming no old launcher request remains active:
sudo rm -f /etc/systemd/system/jarvis-codex-launcher.socket \
  /etc/systemd/system/jarvis-codex-launcher@.service
sudo systemctl daemon-reload
sudo rmdir /run/jarvis-codex-launcher
```

run these only during the approved cutover; shared codex services keep running.
the installed cli/jarvis/phone journey needs explicit approval for newly created,
named test sessions on default tmux servers: the isolated `-L` gate alone does
not authorize it. cover all fifteen host/profile cases across the six directed
routes; repeat detailed state/dialog/interrupt/stop cases only on representative
codex and claude sessions. finish with macbook↔arch during a devbox gateway
outage and phone controls on the same targets. use existing release, fleet,
integration and platform commands, preserving their boundary-specific opt-ins.

## acceptance and rules

| gate | observable result |
|---|---|
| discovery/addressing | manually launched codex/claude appears without enrollment; changed pane/process yields stale, never controls the replacement; one agent per session |
| native/terminal status | working, input/approval wait, idle/completion, interruption and unavailable observations map correctly; unrecognized screen is unknown; quoted approval prose alone is not blocked |
| interaction | an agent lists/reads/sends/keys/starts/interrupts/stops another; coordinator sessions are ordinary targets; multiline input arrives intact once |
| output | native fixture/history retrieves text older than the viewport; unavailable native reader falls back with honest coverage; encoded output is bounded; no resume or copied store |
| fleet | from each acceptance computer control both other hosts; both providers and all existing profiles covered; macbook↔arch works with devbox unavailable; extra cli peer passes config/client tests |
| jarvis | jarvis controls a codex and claude session on remote hosts; either session can itself use the common cli to manage another; no new task/role model |
| failure/stop | lost send response is unknown without replay; terminal fallback is never a second write; stop reports native halt and terminal closure separately; unrelated sessions survive |
| phone | three-host enrollment retained; statuses refresh through existing polling; attach/input/detach and interrupt/stop reach the same native session |
| installation | all three hosts get the same runtime interface; existing pairings/accounts/tmux lifetimes survive installation; obsolete launcher has no remaining callers |

use routine hermetic tests for mappings, bounded history/input and fake transport
outcomes; isolated real tmux for terminal behavior; a small approved live journey
for native providers/fleet/phone. never fabricate a live pass from a fixture.
tmux/integration/live and adb/platform gates still require current-turn user
approval; absent approval/device/boundary is `NOT_RUN`. no live gates run by this
document change. no stress, security-hardening, or exhaustive permutation suite.
cover each directed host pair and configured profile; do not multiply every
state/operation test across that entire matrix.

accepted costs: provider/version maintenance; helper startup/handshake latency;
five-second sampled status; inferred fallback state; partial history; shared
human/agent input without an exclusive writer; no offline execution/recovery;
manual credential redistribution; terminal closure does not prove every detached
process stopped. bounded reads have no general older-history paging in v1.

## precedence and source anchors

this accepted v1 delta replaces only the opaque-agent/no-provider-read/no-control
and active/quiet-only provisions of skid's architecture, plus jarvis's devbox-only,
codex-only, human-only worker-control provisions. root must apply these specific
supersessions in sibling specs/adrs during their implementation; unrelated rules
remain. no new generalized safety or lifecycle framework is authorized.

source anchors: [skid architecture](architecture.md),
[current jarvis integration](jarvis-codex-control.md),
[claude native state](https://code.claude.com/docs/en/agent-view),
[claude history reader](https://code.claude.com/docs/en/agent-sdk/python#get_session_messages),
[herdr terminal automation](https://herdr.dev/docs/agent-automation/),
[tmux input/capture primitives](https://man.openbsd.org/tmux).
