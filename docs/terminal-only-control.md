# terminal controls, ordinary launches, account quota

status: source merged and released in v0.20.0 on 2026-10-21.
[isolated qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/terminal-only-control-qualification.md) records its actual
results; [live cells](issues/terminal-only-live-qualification.md) remain open; [installed qualification](v0.20.0-installed-qualification.md)
owns the separately authorized fleet rollout.
this is one cross-repository change, not a rollback to an old skid release.
amends [architecture](architecture.md), [terminal control](terminal-agent-control.md),
[usage selection](terminal-usage.md) and [generalization](generalization-plan.md)
only where specified below. existing unrelated contracts remain authoritative.

## decision and boundary

skid manages terminals and observes account quota. remove its provider-native
conversation reading, results, input, interruption, waits, association and
identity detection for both codex and claude. jarvis worker tools use terminals
only. keep foreground provider/account detection, screen classification, usage,
notifications, immutable terminal targets and the ordinary shell launcher.

| capability | final owner and contract |
| --- | --- |
| execution, accounts, saved history | upstream providers and the user's normal wrappers/settings |
| launching | initialized destination bash/zsh evaluates the configured command once, unchanged |
| read/input/interrupt/wait/close | existing terminal implementations; no provider-native dispatch |
| status, displayed model/effort, notifications | current foreground process and captured screen; unknown remains unknown |
| codex quota | eligible fresh native account report, otherwise eligible fresh terminal observation, otherwise unknown |
| claude quota | fresh attributable terminal observation, otherwise unknown; no new native claude source |
| shared memory and jarvis cognition | their existing independent libraries/services; outside this removal |

native quota fallback is an intended acquisition policy, not a legacy path.
no conversation fallback, compatibility decoder, tombstone endpoint or replacement
registration mechanism remains. no new daemon, registry, database or settings ui.

## 1. ordinary personal launches

retain the four yaml profile keys, labels, order, `provider`/`home`, hub, terminal,
machine overrides and unrelated private settings. command values become exactly:

```yaml
personal: codex
work: codex-work
work2: codex-work2
claude-work: claude-work
```

this is a key-to-command table, not a replacement yaml schema. keep the existing
ordered `profiles: [{name, label, command, provider?, home?}]` structure.
generic skid commands remain arbitrary user-defined shell text: no whitelist,
rewriting, injected flags or interpretation of account wrappers.

normal aliases supply permission defaults. existing wrappers retain account/home
selection, memory credentials and codex installation-provenance cleanup. bare
`codex` preserves an inherited `CODEX_HOME`; named work wrappers force their
normal homes. observation metadata never overrides execution. a mismatching or
ambiguous account is unknown, not forced to match the launch label.

dev-server still copies devbox's `${XDG_CONFIG_HOME:-~/.config}/dev-server/skidbladnir.yaml`,
when present, whole into `${XDG_CONFIG_HOME:-~/.config}/skidbladnir/config.yaml`
and invokes `skid apply`. changing only the template or generated hub file is
insufficient. remove permission/socket/plugin additions from the
template AND the private desired file. do not move them into aliases, wrappers,
provider settings or another launcher to simulate the removed skid integration.
ordinary provider daemon auto-start/settings remain the provider's behavior.

## 2. terminal capability and schema contract

reuse existing routes and implementations under `/v1/sessions/…/terminal/…`,
session creation/closure and inventory. remove `/v1/conversations/*`; those paths
receive the ordinary unknown-route response. remove `agent-hook` and `inspect`.

| surface | exact change |
| --- | --- |
| cli selectors | only `t-` plus 16 lowercase hex digits, or a captured terminal `--ref`; names remain labels |
| retired cli syntax | reject `c-…`, `--conversation`, `--history`, `--input`, `--queue`, native `inspect` and conversation-bearing refs before effects |
| `--profile` | retained for creation, never as a conversation selector |
| reference | `{machine, tmuxId, identityToken, paneId}`, all required; same terminal encoding/hash and lifetime guards |
| session/agent | remove `conversation`, `conversationHandle`, `providerSession` including name/id; keep provider, pid, start identity and observed profile |
| read | no source/scope input selector; retain max-bytes (default 16384, range 1–32768) and bounded rendered text; `{text, source:"terminal", scope:"terminal_history"|"visible", truncated}` |
| send/text/keys/stop | `{method:"terminal", outcome:"written"|"unknown"}`; guarded send and explicit text/keys retain their distinct admission |
| close | retain separate `interrupt:written|not_sent|unknown` and `terminal:closed|not_closed|unknown` facts |
| wait | states `idle`, `working`, `needs-input`; retain deadline/cancel and `{target, outcome:matched|timeout|target_changed, terminalStatus?}`; matched requires terminal status |
| native-only facts | remove conversation/turn bindings, method availability, accepted input, finalized output ids/state, results cursors and native wait observations |

all changed wire/model objects stay closed; reject obsolete fields and explicit
null rather than accepting/ignoring them. preserve unrelated existing fields,
envelope formats, terminal read truncation, body limits and error semantics.
delete conversation-only `AgentUnavailable`, `AgentTargetStale`, `AgentInputInvalid`
and `HistoryChanged` wire/logging/android variants. retain terminal errors,
uncertain-write dispatch and the terminal primitive `ErrInvalidInput`.
retain `--json`, literal `--stdin`, `--terminal-only`, `info --explain`, terminal
refs, and initial-prompt readiness checks. idle and written prove neither task
completion nor provider acceptance; ctrl-c/closure do not prove cancellation.

remove all readers/writers/decoders for `@skid_agent_runtime` and
`@skid_conversation_b64`. old tmux options may remain inert. do not sweep them,
restart sessions or retain a reader to migrate them. retain tmux/pane-title
naming, remote-context handling and notification identity/acknowledgement.

remove hook-only ancestry/registration/name parsers after caller checks. preserve
foreground/environment observation and account matching. relocate shared terminal
read/write records and bounded serialization rather than deleting them with the
native handlers. android's generic strict-json helpers currently in
`AgentControl.kt` remain available to notification persistence.

## 3. quota boundary and packaging

keep `GET /v1/profile-usage`, codex-only and account-scoped. no conversation ids,
thread loading, provider start/restart/update, prompting or history access.
reuse the existing five-second host budget, cancellation, 64-kib bounds and
normalized reports. no new poller, cache or freshness setting.

rename `HistoryScope`/`historyScope` to `AccountScope`/`accountScope` in profile
descriptors and validators. retain the current sha256 of provider plus nul plus
canonical account home. this identifies local configuration scope, not a verified
authenticated account. validate its 64 lowercase hex digits directly. retain
existing omission when scope cannot be established. descriptor changes withdraw
old quota; no dummy conversation object, account registry or cross-host scope join.

add `provider_runtime.agent_runtime.quota_cli` in llm-calling; call the existing
`CodexControl.usage` and its observe-only transport. one closed stdin request:

```text
{profileKey: existing CredentialRef key, endpoint: absolute unix:// socket address}
success: {ok:true, result:{reportedAt, fiveHour?:{usedPercent,resetsAt?}, sevenDay?:{usedPercent,resetsAt?}}}
failure: {ok:false, error:{code:"rejected"|"unavailable"}}
```

request/output are at most 64 kib; consume one json document through eof. reject
duplicate keys, unknown fields, null, non-finite constants, trailing documents,
malformed/oversized input and nul/non-absolute endpoints before connection.
`profileKey` uses existing `CredentialRef` validation, `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`.
derive endpoint from the selected profile home on its host; no tilde/shell
expansion in the helper. isolate its account environment as the current helper
does; never rewrite the ordinary launched provider's environment.
`rejected` exits 1; bounded connection/protocol/timeout failure returns
`unavailable` and exits 1; success exits 0. no operation/provider/targets/input
union, dispatch/replay claim, account data or provider stderr in output. retain
existing normalized numeric/date/duration validation: missing stays missing,
values above 100 remain valid, supported windows are 300/10080 minutes.
subprocess cancellation closes its connection and joins owned work under the
existing host deadline; preserve bounded subprocess termination/cleanup.

skid installs `providers/quota-read`, importing only this entrypoint. rename its
internal `NativeControlPath` to `QuotaReadPath`; user yaml has no such path.
replace the old multi-operation launcher and update release admission/probes.
drop `--extra claude-sdk`, SDK imports, bundled-binary deletion and the SDK
exclusion manifest entry from skid packaging. keep current managed python/uv,
the frozen source/lock/requirements checks and the provider-runtime base package.
the runtime manifest retains `commit`, `uv`, `python`, `interpreter`,
`lockSha256`, `requirementsSha256`; delete its `excluded` field and old import probe.
update the exact dependency commit after the quota entrypoint is merged; do not
invent a pin or upgrade unrelated dependencies.

fresh means existing admission/identity/reset-expiry rules and the existing
one-hour stale threshold. a native report needs at least one recognized unexpired
usable window; successful empty reports permit terminal fallback. select one
whole report per profile: fresh native first, otherwise fresh attributable
terminal, otherwise no numeric quota. claude has only the terminal choice.
do not splice windows, sum accounts, use launch labels as account proof or let
stale native values mask fresh terminal evidence. retain unknown/staleness
diagnostics, polling cadence, receipt-time/host-clock math and claude candidate selection.
this removes stale numeric reports as a third source; freshness limits are
conditional on the observation, not a promise about provider backend freshness.

## 4. jarvis composition

keep the eleven existing worker tools and their authority/read/action/wait owners.
`AgentTarget.handle` accepts only `^t-[0-9a-f]{16}$`. remove the `native` property,
optional conversation target, conversation wire types and native dispatch branches.
`agent.read` takes `{target,maxBytes?}`; remove its `source` selector. read results
use the terminal schema above. control receipts have terminal method/outcomes;
`agent.wait` accepts terminal states only. normal `agent.info` returns terminal
facts. remove native capture/turn/output/result fields, not immutable terminal
capture, consent context, no-replay rules or asynchronous wait/cancel semantics.

bump `AGENT_IMPLEMENTATION_REVISION` from worker v8 to v9 through the existing
binding/fingerprint/cutover mechanism. finalized retired records remain opaque;
unfinished retired actions/waits/required events must settle under their owning
release before activation. no new decoder, journal, migration or privilege bridge.
fix the existing load order in `actions.py`: classify a known retired worker
revision from its common immutable envelope BEFORE decoding `agent_target` with
the current schema. retain common digest/effect/lineage/state/time validation;
finalized retired nested target/arguments/results remain opaque stored values,
untouched and never executable. unfinished retired rows block activation. v9
rows receive full current validation; an unknown/malformed revision is not an
archive escape. include v8 in retirement. no old target codec or data rewrite.
jarvis's separate native cognition, memory archive codecs and conversation ids
are not skid worker features and remain intact. revise pending worker-search
plans to promise available terminal history only; do not silently route them to
provider history or shared memory.

## 5. exclusive implementation assignments

one integrator owns cross-repository sequencing and shared documents. paths in a
row belong only to that engineer; hand interface requests to its owner. use
isolated worktrees against the then-current merged source; current local checkouts
contain unrelated work. each engineer owns disposable tests for their boundary.

| owner | exclusive production files / responsibility |
| --- | --- |
| q: provider adapter | llm-calling `src/provider_runtime/agent_runtime/quota_cli.py`; reuse shared transport/usage, no public conversation API deletion |
| h: skid host | `cmd/skidbladnir/{main,standalone}.go`; `internal/{agenthook,agentruntime,agentcontrol,sessions,tmux,process,gateway,hostconfig,logging}/`; remove conversation/hook routes/types, preserve terminal guards, implement quota adapter call |
| c: skid clients | `internal/{agentcli,fleetclient,sessionui,terminalclient,macnotifications}/`; android `app/src/main/java/dev/niels/skidbladnir/`; terminal-only models/control/copy, account-scope rename and quota selection |
| j: jarvis workers | jarvis `src/jarvis/{agent_tools,agent_control,actions,write_policy,write_dispatch,cli}.py`; worker bindings/revision/closed schemas and opaque retired-load ordering, no cognition refactor |
| d: personal deployment | dev-server `assets/skidbladnir/config.yaml`, `lib/ai-tools.sh`, `assets/claude/agent-identity/`, `ansible/roles/workspace_assets/tasks/main.yml`; plain commands and plugin publication/staging deletion |
| integrator | skid `deployment/`, `internal/install/`, `scripts/{check,build-host-archive,check-release}`, release/pin composition; dev-server pin and private desired yaml; current docs/AGENTS/spec/roadmap updates in each repo |

delete skid `internal/agenthook/`, `internal/agentruntime/control.go`,
`internal/sessions/conversation.go`, `internal/gateway/conversations.go` and
`deployment/providers/claude-agent-identity/`. trim native-only portions of
`agentcontrol/{service,actions,native}.go` and `gateway/native_response.go`;
keep their live terminal primitives under their existing responsible owner.
delete dev-server's entire identity asset tree, including hidden plugin metadata.
update current architecture/control/help/contracts and obsolete issues; preserve
historical qualification as evidence, not current promises. update jarvis's
authoritative `SPEC.md`, architecture, worker contract and single plan; add the
next available adr superseding only worker-native portions of accepted adrs.

launch designer owns yaml/examples/readme/runbook drafts. control/quota designer
owns cli/help, table/details, android and usage-disclosure drafts. these are
content-only assignments; engineers own production edits. each designer defines
and reviews good content within the schemas before implementation: exact
command/path/owner, observable effect, scope and uncertainty;
no native-conversation rows, examples, badge or removal warning remains. use:

> skid runs each configured command once after normal shell startup. aliases,
> functions, environment and provider settings remain effective.

> controls address the selected terminal. read returns rendered terminal text;
> send checks the current screen before writing. stop sends an interrupt.

retain distinct close outcomes, quota source/estimate/freshness and existing
unknown-status explanations. remove conversation columns/details/phone indicator;
do not redesign ordinary rows, navigation or notification presentation.

## 6. proof and sequencing

1. interface/content review: designer drafts; independent adversarial reviewer
   challenges every capability deletion, shared caller, schema and claim before
   engineers write tests. resolve findings in this document; no silent scope change.
2. temporary red tests at actual integration boundaries; record baseline source
   and expected failures for changed behavior. preservation checks may already
   pass. do not make a test fail artificially or claim fixtures as live providers.
3. q first; h/c/d/j consume the fixed contracts above in parallel without shared
   file edits. run green, review code AND test sensitivity, then refactor against
   repository rules and rerun affected proofs. independent adversarial review at
   each handoff; author self-review is insufficient.
4. integrator composes exact pins/artifacts; reviewer checks the complete path,
   absence of retired dispatch and preserved authority. required engineering
   checks pass. delete temporary tests/controllers/dependencies after proof and
   reviewed refactor; keep concise content-free results/source ids in qualification.
   existing independently authorized regression fixtures/checks remain.
5. installed cutover requires separate authorization. the owner authorized the
   coordinated release/deployment on 2026-10-21 under section 7; the original
   implementation task assigned no installed service/configuration mutation.

| acceptance | temporary proof |
| --- | --- |
| ordinary launch | actual gateway → isolated tmux → initialized bash/zsh → decoder → command; aliases/functions/PATH/cwd/quotes/env, once-only execution, return/failure/exec; four exact command values, inherited personal home, account/memory selection without exposing credentials |
| complete removal | baseline exposes conversation routes/syntax/output facts; candidate removes them. test obsolete inputs for rejection before effects and output absence through production serialization separately; no helper/terminal substitution. planted old tmux options are inert and existing session/PID/handle remains unchanged |
| real terminal flow | disposable stock codex and claude on darwin/linux: create with literal stdin, attach/detach, bounded read, empty/draft/dialog send admission, text/keys, wait/cancel, interrupt/close, status/model/usage/notification and pane-title naming; preserve draft and uncertain-write no-retry behavior |
| quota | real account read against already-running daemon; actual client/ui over signed fixture gateway/frame evidence proves fresh-native preference, unavailable/stale/empty-native fallback, no-fresh-source unknown, missing/clipped/ambiguous display, same-key home invalidation and no window splice; no stopping shared daemons to simulate failure |
| quota-only boundary | subprocess + real unix/websocket fixture server: normalization, duplicate/extra/null/non-finite/trailing/oversized input, invalid key/endpoint, absent socket, bad protocol, timeout and cancellation while connected; rejected requests cause no connection; package runs without claude SDK |
| jarvis | real cli + signed temporary fleet/gateway + actual postgres action/read/wait path: t-target discovery/capture/control; removed native fields/targets reject; pane/lifetime changes refuse; unknown writes never replay. finalized v8 terminal/native records in succeeded/failed/uncertain/cancelled states load opaquely through ordinary ledger paths; stored values/digests stay untouched and never gain authority. unfinished v8 blocks; malformed v9 fails |
| deployment | actual isolated apply twice, private override present/absent: four commands stay exact; unrelated private/fcm settings survive; no plugin staging/publication returns; signed host archives/install admission and matching android decoder pass |

use bounded fixtures for failures; live claims require actual provider/host/device
evidence. don't alter user sessions, resize/retarget shared panes, stop daemons,
send external messages or use the phone without explicit execution authorization.
mark unavailable boundaries `NOT_RUN`, never pass. resolve distinct discovered
defects in their owning module; record blocked/out-of-scope issues, don't absorb
unrelated repairs. documentation-only work needs link/consistency checks.

## 7. installed hard cut and tradeoffs

2026-10-21 owner clarification: deploy skid while jarvis remains disabled and
stopped. record its live-state access boundary in jarvis's existing
`docs/issues/skid-installed-state-access.md`; this change assigns no repair or
worker activation. unfinished retired worker authority must settle before that
later activation through its existing checks, without replay or ledger rewriting.
stage matching active skid hosts/android and the dev-server pin. update only four
command values in devbox's private desired yaml, preserving other content/mode.
use normal signed config application; verify every host accepted it. no re-pairing,
fleet reset, provider/account migration or generic release controller.

2026-10-21 owner clarification: preserve running claude processes during this cut.
the verified owned plugin registers only a five-second `SessionStart` identity
hook, with no tool/prompt/stop hooks or blocking failure policy. a cached hook
may briefly wait or warn after removal; [upstream documents its failures as
non-blocking](https://code.claude.com/docs/en/hooks#exit-code-2-behavior-per-event).
this does not justify interrupting work or retaining a shim/native dispatcher.
ordinary tmux shells and unrelated providers stay.
remove only verified deployment-owned files, then empty directories, from
`~/.local/share/dev-server/claude-agent-identity` and staged
`~/.local/share/dev-server/assets/claude/agent-identity`. unexpected content blocks
that cleanup, not a blind recursive delete. remove no immutable skid release
members by hand: the new signed release excludes its hook and old launcher;
the installer deletes superseded releases. no permanent cleanup scanner.

old native CLI callers/captured conversation refs become invalid at the cut;
they are never converted to terminal targets. old inert tmux options need no
migration. running codex processes may retain `--remote`; new launches use normal
commands. provider histories/auth/memory and ordinary terminal refs survive.
post-cutover failure is ordinary explicit repair, not restoration of legacy APIs.

accepted costs: saved-history/finalized-result/native-turn control disappears;
bare personal account follows ambient environment; absent fresh quota shows
unknown; compatible active hosts/phone/worker consumers must cut over together;
affected claude processes may retain a non-blocking stale-hook warning; deleting
tests gives no retained automatic regression protection. native quota still
carries managed python/uv and provider-runtime base dependencies. keep one existing protocol owner rather
than duplicate it in go or redesign shared packaging. public provider-runtime
conversation/cognitive/archive APIs used outside skid are not legacy skid paths
and are not removed. no public-data/history deletion or paid-model campaign.
