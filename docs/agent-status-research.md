# provider status and unread results: research

2026-09-28. research findings and recommendations, not an implementation spec.
no production code, provider settings, sessions or runtime owners changed.
three parallel source reviews covered herdr/codex, claude/comparators, and
skid/identity/unread integration. official documentation and content-free native
read probes supplement the source review.

the decisions below are historical. [terminal control](terminal-agent-control.md)
now owns ordinary observation and interaction; [terminal attention](reply-notifications.md)
replaces unread results and human acknowledgement. [native conversations](native-agent-observation.md)
retain explicit machine capabilities. those specifications supersede this research's
native-only surfaces and unread recommendations.

## settled product decisions

- omit blocking reasons. use native state or explicit unknown; retire terminal
  inference and automatic transport fallbacks from agent surfaces.
- native controls must preserve exact identity and the operation's meaning,
  including the distinction between user input and peer messages.
- unread results belong to each device. there is no cross-device acknowledgement
  synchronization.
- opening acknowledges only the results already known at that moment. later
  results remain unread, including results arriving while the terminal is open.
  opening is an acknowledgement policy, not proof the final answer was visible.
- qualify fragile integration with temporary focused integration/live tests;
  delete them after verification, without rebuilding retired infrastructure.

## the three facts

execution state answers whether the provider is working or waiting. input
readiness answers what an input operation will do now. unread answers whether
this device has acknowledged a particular result. none determines the others.
a working agent may have an unread earlier result; an idle agent may have no
new result; an available composer may steer or queue rather than begin a turn.

the fourth necessary fact is identity: which native conversation the selected
terminal actually represents. stronger state attached to the wrong conversation
is worse than honest uncertainty. the tui owns its selected conversation, the
provider owns execution/history, tmux owns terminal/process identity, and the
client owns acknowledgement. these are the useful boundaries.

## what to take from herdr

inspected installed `0.9.1`, source
[`065ef9d6`](https://github.com/herdrdev/herdr/tree/065ef9d6a531c49fb8bee7e818ef837065b21ee9),
and subsequent master `fff6c820`. claude/codex integrations register session
identity through session-start hooks; their status is inferred from process and
terminal evidence. full lifecycle hook support for other providers does not make
claude/codex status native.

herdr combines foreground-process recognition, bottom-screen rules and terminal
title/progress signals. its installed release has an optimistic known-agent idle
fallback. the subsequent
[codex correction](https://github.com/herdrdev/herdr/pull/4563) removes static
chrome as evidence of idle and separates startup readiness from turn state.
[issue 4507](https://github.com/herdrdev/herdr/issues/4507) reports false idle
during active work; its reported measurements are upstream evidence, not our
local failure rate.

the useful borrowing is the separation of detection from attention, explicit
unknown, startup readiness, and regression cases. herdr's client presentation
uses endpoint boot identity and a state-change sequence for acknowledgements,
with an initial baseline and independent clients. its sequence describes its
own observations; copying it cannot recover a provider turn missed between two
skid polls. see
[client presentation source](https://github.com/herdrdev/herdr/blob/065ef9d6a531c49fb8bee7e818ef837065b21ee9/src/client/shell/endpoint_agent_state.rs).

literal reuse is available under the repository's
[apache-2.0 license](https://github.com/herdrdev/herdr/blob/065ef9d6a531c49fb8bee7e818ef837065b21ee9/LICENSE),
with its license/attribution/modification requirements. do not import herdr's
generalized detection manifest, hook arbitration and lifecycle machinery merely
to reuse a few useful rules.

## codex: native capability is real

installed cli: `0.157.1`; inspected source:
[`36650394c5b38c2990ccf2a3457165ca3e9d9726`](https://github.com/openai/codex/tree/36650394c5b38c2990ccf2a3457165ca3e9d9726).
the running daemon and available `codex_tui` host may have different versions.

| surface | useful capability | limitation |
| --- | --- | --- |
| existing shared app-server | native status, history, start, steer and interrupt | connect to the server that owns execution; a new server is not an observer of the old server |
| embedded tui server | native internal operations | no external connection; some launch options select this mode |
| `codex app-server proxy` | byte relay to existing daemon | unix transport still needs websocket upgrade/framing; not a jsonl conversion |
| `codex_tui` | list/read/wait and task messaging | ephemeral tui-hosted adapter, no current-pane query; bounded/pruned responses |
| `codex exec --json` / execution sdk | structured events for its own execution | not passive attachment to an arbitrary existing tui |
| `codex queue` / `thread/queue/*` | provider-owned persistent user-input queue | experimental; no peer/tool-output attribution; interruption retains pending input |

[official app-server documentation](https://learn.chatgpt.com/docs/app-server)
describes non-resuming reads, statuses, history and turn controls. it labels the
app-server command and websocket transport experimental and unsupported for
production workloads. this is a real maintenance tradeoff for a one-user
prototype: pinned qualification and explicit unsupported behavior, not a claim
of provider stability.

source details that materially affect integration:

- `thread/read` does not load or subscribe. `notLoaded` means absent from that
  server's memory; it is not completion or proof of exit. statuses are idle,
  active, system error and not loaded; active flags describe pending approval or
  user input. idle alone does not distinguish success, failure or interruption.
- `thread/status/changed` broadcasts to initialized connections without resuming
  a thread. there is no initial replay guarantee; reconcile with snapshots.
  transient per-refresh reads fit the existing helper before a new event service
  is justified. events can reduce latency but cannot replace recovery reads.
- `thread/turns/list` and `thread/items/list` expose provider-projected ids and
  outcomes. legacy history can synthesize `rollout-<index>` turn ids and default
  implicit turns to completed; inactive historical in-progress turns can become
  interrupted. those projections are not uniformly recorded completion events.
  qualify stable result identity before using them for unread. item paging may
  also be unsupported by a backing store.
- pagination bounds the response; older rollout-backed reads can reconstruct the
  entire history internally. the helper deadline bounds client waiting, not the
  provider's reconstruction work or its cancellation.
- metadata-only `thread/read` still includes a preview. project required fields
  immediately; do not log the raw reply. no copied transcript store is needed.
- `thread.id` identifies the conversation. `thread.sessionId` identifies the
  session tree shared by related threads and is insufficient as the unread key.
- `thread/resume` can load work and change client configuration. it is not a
  harmless subscription. do not resume merely to observe.
- `turn/start` can start or steer. explicit `turn/steer` carries an expected turn
  id; `turn/interrupt` names and validates a turn. preserve these distinctions.
  a lost response after possible dispatch remains unknown; do not resend blindly.

source anchors:
[thread schemas](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/thread.rs),
[thread identity](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/thread_data.rs),
[status producer](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/thread_status.rs),
[broadcast](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/outgoing_message.rs),
[turn controls](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/turn_processor.rs).
legacy projection details are in
[history reconstruction](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/thread_history.rs)
and the
[history request processor](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/thread_processor.rs).

the available `codex_tui` list/read/wait probes succeeded. an immediate repeat
with the prior cursor reported unchanged, timed out and produced no wake-up.
these probes recorded only shapes, counts, booleans and allowed state values.
they establish native observation through this tool surface, not gateway
connectivity or an exactly-once completion journal.

`wait_threads` cursors encode a status/history snapshot, not a durable sequence.
its completion wake can include a failed or interrupted terminal turn.
`send_message_to_thread` resumes the target and submits delegated tool output;
it is not ordinary human input. its 1,000-byte prompt limit and ephemeral bearer
endpoint favor implementing qualified native semantics in the existing helper
over adopting this task adapter as skid's transport. direct app-server input
must preserve the same peer attribution when it implements peer send. see
[tool implementation](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/tui/src/dynamic_tools.rs)
and [hosting](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/tui/src/dynamic_tools_mcp.rs).

the experimental `thread/queue/*` api accepts `UserInput`; its service rejects
non-user `TurnInput`. queueing delegated text as user input would change its
authority. qualify queue for user input only until native peer attribution is
supported. provider storage owns persistence; skid needs no queue service.
interruption leaves pending entries intact and skips that idle-triggered drain;
this is not a durable queue cancellation. failure can continue draining. dispatch
starts a turn before deleting its queue entry, and enqueue generates a fresh
entry id, so neither atomic exactly-once dispatch nor safe blind retry is
established. stopping work does not promise to
discard queued input. see the
[experimental methods](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/common.rs),
[queue service](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/ext/queue/src/service.rs),
[queue storage](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/state/src/runtime/queued_items.rs)
and [upstream lifecycle tests](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/ext/queue/tests/queue_service.rs).

## codex: the missing current-view association

no stock interface was found that maps a foreground tui process to its currently
displayed native thread. this is a source-review finding, not proof that no future
or undocumented interface could ever exist.

| candidate | why it does not establish current selection |
| --- | --- |
| known launch/resume id | cached navigation, new threads, forks and child selection change the view |
| session-start hook | background starts need not become visible; cached switches need no hook; daemon hooks lack tui ancestry |
| one daemon per pane | that daemon can still host multiple selectable threads |
| embedded/no-daemon mode | restores process origin, not one-thread-per-tui semantics |
| configured thread-id title | this version truncates the uuid to 29 characters plus ellipsis; cached title has no process-lifetime binding |
| full thread-id footer | can clip or retain old screen content; still a presentation observation |
| cwd, name, latest transcript, thread tree root | identifies neither the selected thread nor its current client |

the tui owns selection; `current_displayed_thread_id()` is a candidate integration
point, not by itself proof of committed visible selection. a narrow upstream
capability could publish committed tui selection on the existing
app-server connection, scoped to that connection's lifetime, with enough process
identity to match the kernel-observed foreground tui. disconnect removes the
association. no status hooks, transcript interception, lifecycle database or
separate execution supervisor are implied.

that proposal covers shared-daemon tuis. embedded mode still needs an externally
readable interface; absent one, native operations remain unavailable. a
same-process selection can change during an asynchronous read; a selection
revision and revalidation, or an explicit sampled-association contract, must
address that race. connection
lifetime alone does not freeze selection or make cross-system control atomic.

publication must follow actual visible selection, not every temporary routing
assignment: voice/background paths temporarily swap routing fields. this needs
source-level qualification, including two tuis sharing a daemon. see
[view getter](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/tui/src/app/thread_routing.rs),
[selection owner](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/tui/src/app/session_lifecycle.rs),
[title rendering](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/tui/src/chatwidget/status_surfaces.rs).

recommendation: preserve terminal cards and repair association at its provider
owner. cost: an upstream change or a maintained provider patch until accepted.
native conversation rows are the stock-provider alternative, but change the
product's primary object and cannot promise attachment to the correct existing
tui. silently replacing terminal cards with conversation rows is not justified.
title-prefix matching is inference, so it cannot satisfy the native-only plan.

## claude: use saved replies, distinguish peer input

installed provider: `2.1.284`, build commit
`2b8ce618c24de26410e4bdfc4e1d592accd61f61`; sdk: `0.2.130`.
public docs establish
[session listing](https://code.claude.com/docs/en/agent-view#manage-sessions-from-the-shell),
[saved-message reads](https://code.claude.com/docs/en/agent-sdk/python#get_session_messages)
and [remote control](https://code.claude.com/docs/en/remote-control).
[cross-session messaging](https://code.claude.com/docs/en/cross-session-messaging)
also documents peer delivery, idle notifications and posting from scripts/hooks.
the capability is public; installed implementation details below qualify the
external adapter where the public docs do not specify its complete protocol.
no provider code is copied here.

| surface | capability | consequential limit |
| --- | --- | --- |
| `claude agents --json --all` | sampled interactive/background state and identity | no completion revision; public projection omits internal socket/process-start fields |
| sdk saved-message reader | stable message uuid, native history, raw inner message | no execution attachment; parses saved history before slicing; drops some outer envelope flags |
| documented cross-session inbox | peer text reaches existing tui between tool calls; idle recipients start a turn | preserves peer authority; can hold/refuse; external discovery/wire/receipts need qualification |
| documented idle notification | one-shot notification when the recipient finishes with nothing queued | optional, ephemeral, no recovery replay; external callback transport needs qualification |
| background worker rendezvous | native human replies and worker controls | new connection replaces the supervisor connection; unsuitable as an independent observer |
| remote control | concurrent access to existing runtime | provider-cloud bridge, not a discovered public local rpc |
| execution sdk | result/interrupt for its managed subprocess | resume/query creates or owns execution; not a passive connection to an arbitrary live tui |

public background attach/logs/stop commands do not imply equivalent controls for
ordinary interactive tuis. a background attachment client's pid also differs
from its worker's pid; do not weaken exact matching to join them by name or cwd.

### saved finalized replies

the native recorder withholds in-flight assistant messages whose `stop_reason`
is null. the sdk preserves the finalized message's uuid and inner stop reason.
a saved assistant text message with explicit `end_turn` is therefore useful
evidence of a newly available finalized reply. this is deliberately narrower
than every possible final outcome: refusals, token-limit stops and api errors
need separate mapping. `tool_use`, commentary alone, and an arbitrary latest
assistant message are insufficient.

this has a narrower meaning than successful agent completion: a stop hook may
continue work, children may remain active, and stream-recovery code can finalize
partial output with synthesized `end_turn`. unread should mean a new finalized
reply is available; execution/outcome remains a separate observation. accepting
provider-finalized partial replies is a stated reliability limit.

bounded content-free local sampling found 403 assistant rows across 16 main
transcripts (6.18 mb): 29 `end_turn`, 368 `tool_use`, six `stop_sequence`, no nulls.
all sampled `stop_sequence` rows were synthetic api errors. sampled provider
versions were `2.1.261`–`2.1.283`; static recorder analysis was `2.1.284`. this
supports the mechanism without qualifying every future version or error path.
the sdk drops outer `isApiErrorMessage`/`isVirtual` flags, so do not broaden the
candidate to stop-sequence messages through that reader.

one logical response can produce several saved content-block rows with different
outer uuids and a shared inner `message.id`. a stable row uuid is not necessarily
one distinct reply. use native conversation identity plus inner `message.id` as
the logical reply key. sdk `0.2.130` preserves that inner id. the source's block
normalization and recovery paths preserve it and finalize sibling stop reasons
together. use conversation-chain order, not lexical id order.

the same sample's 403 rows represented 135 response ids; 29 `end_turn` rows
represented 20 replies. 107 response groups had multiple outer uuids. no missing
inner ids or contradictory grouped stop reasons appeared. missing identity or
inconsistent grouped outcomes remain unqualified; do not substitute hashes,
timestamps or row uuids. a group qualifies for the proposed narrow policy when
it contains assistant text and its final outcome is consistently `end_turn`.

the helper currently discards this identity. expose native result metadata through
that owner rather than add a transcript parser or completion-hook ledger.
source anchors within the installed build: interactive recorder `kge.record`,
in-flight filtering `rIt`, and the sdk's `SessionMessage`/`get_session_messages`.
these internal symbol names are version-specific evidence, not integration names.

### documented messaging, qualified adapter

the [public contract](https://code.claude.com/docs/en/cross-session-messaging)
provides `ListAgents`/`SendMessage` and script access to a local inbox. messages
arrive between tool calls without interrupting a running tool. they retain peer
authority: no permission consent or slash-command execution. inbound policy can
deliver, hold or refuse. `notify_when_idle` is one-shot, can expire or be refused,
and is not a durable completion record or replacement for history reconciliation.

the [environment contract](https://code.claude.com/docs/en/env-vars) exposes each
session's `CLAUDE_CODE_MESSAGING_SOCKET` and `CLAUDE_CODE_MESSAGING_TOKEN` to its
hooks/commands; the socket is available before `SessionStart`. `/status` also
shows the address. unix sockets are restricted to the same os user. authentication
is optional there and mandatory on windows; the exported token authenticates a
session's own scripts. child credentials can change admission: an external peer
must not impersonate a descendant. no new hook runtime or messaging service
follows from these surfaces.

public docs do not specify the complete external message schema, standalone
discovery api or positive enqueue receipt. the inspected registry's
`messagingSocketPath` plus session/pid/process-start identity provides a pinned
discovery candidate; public `agents --json` omits socket/process-start fields.
never derive an address from pid or route by a display name alone. the inspected
newline-json receiver checks an optional expected session id and queues
peer-origin input. its internal `now` priority can abort work, unlike documented
normal delivery; it is not a qualified standalone interrupt capability.

socket-write success has no direct accepted/enqueued reply in the inspected
receiver. optional callbacks report some admission outcomes; neither transport
success nor an idle notice proves execution success. qualify exact discovery,
peer credentials, active/idle delivery, hold/refusal and uncertain receipts in a
version-pinned helper adapter. expose peer send distinctly from user input and
report unconfirmed delivery honestly; do not blindly resend. this admits the
documented capability without inventing a stronger acknowledgement contract.
reject background rendezvous as an observer because it replaces the supervisor's
connection.

installed-source anchors: `S9o`/`Izo` bind the inbox, `en` reads frames, `Pwt`
applies admission, and `Pe` verifies peer process/path binding. these names are
version-specific evidence, not a stable api.

### hooks

the [hook contract](https://code.claude.com/docs/en/hooks#common-input-fields)
includes session identity and optional prompt correlation. prompt id lasts until
the next prompt, not one unique completion. stop events omit a final-message uuid,
may precede persistence, can be followed by continued work, and do not cover user
interruption. hooks would add an observation store that is not needed for the
proposed saved-reply unread policy and does not establish that the agent finished.
no new lifecycle hook is recommended.

## unread implications

use stable provider result identity and history reconciliation, not observed
working-to-idle transitions, silence, terminal bytes or a wall-clock timestamp.
retain the provider conversation identity after the foreground process disappears
long enough to recover its final result. a conversation first discovered after
all its work completed has no prior device baseline; suppressing old history is
an intentional first-observation policy, not recovered unread history.

the device key needs machine, provider/account identity and native conversation
identity. pid and tmux lifetime are observation targets, not durable result keys.
profile identity must remain distinguishable if its underlying provider home is
changed; a mutable display label alone is insufficient.

opening captures the known result boundary. if result b arrives while the
acknowledgement for result a is being saved, b stays unread. acknowledgements
cannot move backward; unavailable observations cannot clear them. do not order
opaque ids or snapshot cursors lexically. history paging/branch changes and
out-of-order responses need explicit handling in the implementation plan.

the plan selects a boolean unread marker that coalesces multiple unseen results.
the tradeoff is no result count or event-log ui. persist only
the metadata needed for baseline, result comparison and acknowledgements in
each client's ordinary local store. no host-side cross-device read state.

android pauses polling in the background; desktop polling stops while inside
full terminal attachment. provider history recovery therefore matters even for
one device. `OutputApplied` only means terminal bytes were parsed; it does not
prove a final answer was seen and is not needed for the chosen open policy.

## existing implementation owners

the pinned `llm-calling` helper is
`ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f`, recorded in
[the deployment pin](../deployment/native-control/helper.json). it already contains an external
unix-websocket `CodexControl` and native inspect/read/send/interrupt/stop dispatch.
closing that client disconnects; it does not own or stop the daemon. native
reads use metadata and paged history, not `thread/resume`. its observe-only
request policy leaves approvals for the native tui; retain that behavior.

this is reusable capability, not a config-only feature switch. skid's
`internal/agentcontrol/native.go` lacks endpoint/turn-id fields; its strict
inspection decoder lacks turn id; `service.go` intentionally restricts native
identity to claude and accepts only terminal send/interrupt capability values.
the helper's current latest-turn projection also cannot recover completed turn a
when newer turn b is already running. completion metadata must remain separately
queryable.

the current reader takes one item page, up to 50 items, from the latest turn.
`thread/items/list` can refuse a read for an unsupported backing store; the
adapter returns unavailable history. qualify actual installed histories before
promising this capability. the existing boundary fixtures cover no-resume reads,
disconnect-only cleanup, approval noninterference, exact-turn interruption,
paging refusal and uncertain mutations; they were inspected, not rerun here.

immutable source:
[codex control](https://github.com/NielsdaWheelz/llm-calling/blob/ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f/src/provider_runtime/agent_runtime/codex_control.py),
[transport](https://github.com/NielsdaWheelz/llm-calling/blob/ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f/src/provider_runtime/agent_runtime/codex_app_server.py),
[native command](https://github.com/NielsdaWheelz/llm-calling/blob/ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f/src/provider_runtime/agent_runtime/native_control_cli.py).

| owner | responsibility for later work |
| --- | --- |
| provider tui | exact current-view association, if terminal cards use native codex |
| existing `llm-calling` helper | protocol/version handling, native state and finalized-result projection, bounded recovery reads |
| skid `internal/agentcontrol` | exact target validation and inventory enrichment outside the session lock |
| `internal/fleetclient` / `internal/sessionui` | preserve table-row status source; remove reasons already shown in local footer/details; desktop-local acknowledgement and projection |
| android model/controller/local store | device-local baseline and acknowledgement; open captures only known results |
| existing deployment owner | account-to-owning-daemon endpoint configuration, qualification and any deliberately maintained provider patch; reuse the current account table |

reuse android's existing datastore dependency in a separate small metadata store.
desktop can use a small atomic file with serialized updates across local browser
processes. this adds local persistence and concurrency handling; it buys restart
recovery. account credentials and navigation return capsules are unrelated stores.
no shared acknowledgement service or new database is justified.

the desktop defect is specifically table-row source loss. local details and the
selected-row footer already restore source directly from `Session.Agent`; they
also show reasons. source review corrected the earlier broader claim that all
desktop presentation lost evidence. the later change should fix the shared
projection and remove the unwanted reasons without duplicating display rules.

## comparative lessons and expert disagreement

| source / lens | useful lesson | what we decline and why |
| --- | --- | --- |
| herdr | separate attention from execution; unknown and startup readiness are useful | optimistic idle and observer-local transitions cannot prove provider completion |
| [cmux notification store](https://github.com/manaflow-ai/cmux/blob/main/Sources/TerminalNotificationStore.swift) | centralize acknowledgement and notification projection | no notification center, focus suppression rules or remote badge synchronization needed here |
| [cmux client focus](https://github.com/manaflow-ai/cmux/blob/main/cmux-tui/spec/commands.md) | selection is explicit client-owned data | guessing focus from execution history loses this distinction |
| [acp prompt turns](https://agentclientprotocol.com/protocol/v1/prompt-turn) | native turn boundaries and stop reasons distinguish lifecycle from streamed content | a protocol adapter does not retroactively attach to an arbitrary stock tui |
| [claude-squad issue 266](https://github.com/smtg-ai/claude-squad/issues/266) | startup input can disappear before the composer is ready | elapsed delay is not a readiness contract; this is an upstream user report, not our reproduction |

the systems reviewer asks who owns each fact and whether it survives missed
polls. the provider reviewer asks which server owns the thread and whether a
read changes execution. the ux reviewer asks whether a badge gives a useful
next action; they reject blocking-reason clutter and require an explicit
acknowledgement policy. the maintenance reviewer asks which dependency changes
can silently manufacture certainty.

the real disagreement is over provider-native integration. it removes broad
screen heuristics but introduces version maintenance. a documented feature can
still require pinned external adapter details. use it where its contract is
understood and testable; undocumented does not mean impossible, and native does
not mean stable. the other disagreement is a provider patch
versus stock-provider product constraints. preserve identity correctness and
state the cost rather than disguising the tradeoff as a parser improvement.

## qualification limits

- successful: local source/doc audit and read-only `codex_tui` metadata/history/
  wait probes. no claims about message content or account state are recorded.
- unqualified: direct external daemon transport. the sandbox denied the socket
  connection before handshake. this is an environment restriction, not a
  demonstrated provider failure. proxy is a websocket byte bridge, so a future
  probe must use the correct framing.
- `NOT_RUN`: tmux mutation/attachment, live sends, interrupts, provider completion
  scenarios, fleet rollout and phone checks. no existing user session was
  prompted, stopped, resized or retargeted.
- temporary focused tests should protect exact association, native state/outcome mapping,
  missed completion across reconnect/restart, initial baseline, acknowledgement
  races, failure versus successful result, history lag/branching, unavailable
  observations, peer admission/attribution, queue pause/recovery and uncertain
  sends. provider upgrades need a small live check
  at the affected boundary, not an exhaustive fleet permutation matrix.

the [implementation plan](native-agent-observation.md) owns the superseding
scope and acceptance criteria. this research does not label unperformed live
work as passed.
