# terminal reply notifications

status: implementation plan; production unchanged. this owns the coordinated
replacement of historical unread tracking and human reply viewing. at cutover it
supersedes conflicting notification/presentation rules in [native interaction](native-agent-observation.md),
[agent control](agent-control.md), [desktop browser](desktop-browser.md), and
[design language](design-language.md). other conversation/control contracts stand.

## outcome and limits

humans use the terminal. `new reply` means a current reply worth returning for,
not unread history. both codex and claude follow the same policy:

| event | effect |
| --- | --- |
| newly observed eligible reply from the newest work | green `new reply` |
| successful deliberate terminal entry | clear the captured conversation's notice |
| remaining in that terminal visit | consume replies observed for that conversation |
| leaving the visit | consume the first qualified post-visit snapshot before ordinary notifications resume |
| fresh native working observation | clear the obsolete notice; blue `working` |
| newer work observed without a reply | clear the older work's notice |
| idle without a notice | grey `idle` |
| unavailable/stale observation | do not invent work or silently clear saved state |

entry clearing is product policy, not proof the tracked conversation was read.
capture its exact identity; terminal navigation never retargets native commands
or the visit. failed entry clears nothing. only the captured conversation is
consumed during that visit; a different tracked target requires a new visit.

remove human reply viewers and acknowledgement actions. retain orchestration
`read`, including its latest/history/terminal modes and errors. retain creation
association, stop/close, groups, sorting, terminal input, accounts and history.
manual linking is removed independently and must not be reintroduced by this plan.

non-goals: chat/inbox, counts, push, cross-device or cross-window focus sync,
screen/prompt parsing, status hooks, provider forks, new supervisors, databases,
copied history, exact human-read evidence, terminal replacement, naming work,
deployment/publication, and unrelated association/control repairs.

## prerequisite: current work, not latest historical text

the existing paginated result-id feed cannot distinguish an old reply from a
successor whose output has not persisted. replacing only the client reset is
insufficient. **qualify this boundary before implementing client behavior.** the
[native-boundary issue](issues/reply-notification-native-boundary.md) tracks it.

replace the existing results operation; do not add an alternate route:

```text
POST /v1/conversations/results
request = {conversation: Conversation, previousHead?: {workId: string, resultId?: string}}
response = {conversation: Conversation, head?: {workId: string, resultId?: string}}
```

reuse authentication, machine/profile/history-scope validation, strict envelopes,
64-kib transport bound, 10-second operation and 15-second client budgets. ids are
opaque native identities: 1–128 printable ascii characters (`0x21..0x7e`), matching
the existing android native-id validator; align the go boundary. omission is absence;
null, unknown keys, old `cursor` input and old result-page output reject.

- `head` is the newest admitted native work at the source observation point,
  including unfinished, failed and interrupted work. validate ancestry and select
  the head from one coherent native view; response delivery is not that boundary.
  never search backward for the newest work having text.
- `resultId` exists only for an eligible finalized text reply of that exact work.
  preserve native finality predicates; completion/error/permission outcomes remain
  separate. absence of a result never exposes an earlier work's result.
- no `head` means positively established empty history. missing, lagging,
  incomplete or ambiguous evidence is `AgentUnavailable`, not empty success.
- `previousHead.workId`, when supplied, must be the same head or its ancestor in
  native work order. a finalized result identity is immutable for that work;
  missing evidence is unavailable, never `result -> absent -> result`. removal,
  rewind or genuine rewriting of supplied history returns the existing
  `HistoryChanged`. client obtains a new baseline; no id/clock ordering.
- provider history owns order and completeness. gateway validates and projects;
  it stores no notification state. reads never resume or mutate execution.

codex: use newest native turn, including turns without output; `workId = turn.id`.
inspect only that turn's complete items. eligible `final_answer` uses its turn id
as `resultId`. qualify admission visibility and completion/history races.

claude: require a provider-owned work boundary covering prompts, peer input and
autonomous continuations; finalized assistant blocks still group by inner
`message.id`. **the pinned listing/sdk does not establish this boundary.** a
`user` row may be a tool result or compact summary, and the sdk loses completeness
metadata. qualify a supported provider/sdk interface at its owner; do not invent
a turn id from message type, timestamps, prose or a second transcript parser.
see [history completeness](issues/claude-history-completeness.md).

gate: demonstrate unseen reply a, admission of successor b, delayed history,
and b's completion. after b is admitted the operation must never return a as
the head. demonstrate tool/compaction boundaries and history rewind as well.
if a provider cannot supply this contract, stop at this prerequisite and report
the missing capability. permanent unavailability is not completion of this
feature; no weaker silent fallback or additional runtime is authorized.

## composition and client state

```text
existing inventory -> native execution status -> client model
existing results cadence -> gateway -> helper -> current native work
terminal presentation/lifecycle -> client visit -> serialized local store
local store result + fresh status -> one client presentation projection
```

retain the five-second foreground cadence, fair conversation scheduling and one
in-flight result request per host. keep result observation outside inventory;
terminal entry/exit adds only the specified local writes and settlement read.

reuse the existing conversation key and claude same-pane previous-agent
association. visits additionally capture the exact terminal lifetime and current
client attempt/generation. another agent's working state cannot clear a previous
agent's notice. existing unassociated-codex and remote-context limits remain.

replace the unread record with a bounded record:

```text
key = (machine, provider, historyScope, conversationId)
head = {workId, resultId?}
record = {key, revision: nonnegative int64, head?, pendingResultId?, baselinePending: boolean}
store = {schema: 2, conversations: record[], associations: existing-association[]}
```

record absence means uninitialized. first qualified snapshot establishes the
baseline without a notification. an entry/working event before initialization
creates `{revision:1,baselinePending:true}` with no head/pending result. absence
is distinct from revision zero: a request dispatched against absence cannot
commit over that new record. an absent head alone cannot prove an initialized
empty baseline. `pendingResultId`, if present, equals
`head.resultId`. retained `head` prevents rediscovering a cleared result. there
are no historical acknowledged/unread sets or copied text. retain records for
exact reappearance; remove them with the configured machine, as today.

desktop reuses its 0600 file, stable-sidecar lock and read/merge/atomic-replace;
android reuses its separate datastore and serialized updates. keep persistence
logic at those owners, with one reducer per language. no cross-language framework.

every network observation captures the record revision before dispatch; its
merge checks that revision under the store's existing serialization. changed
revision means discard and use the next existing cadence. accepted working,
entry and visit-boundary events advance revision even when pending is empty;
unchanged ordinary snapshots do not. other state changes also advance it; never
wrap. callbacks also check exact target, client generation and request ownership.
never hold a store lock across
network work. this guards stale responses; the native head contract separately
guards fresh rediscovery of old history.

reducer rules:

1. process `baselinePending` before ordinary comparison: consume a qualified
   snapshot, store its head, clear pending and the flag. ordinary snapshot:
   unchanged head/result preserves state; newer head clears
   an older pending reply; a newly eligible result becomes the sole pending reply.
2. fresh `source:native,state:working`: clear the pending reply at that request's
   captured revision, retaining the known head. no observed idle edge is required.
   never consume a later result solely because a cached status still says working.
3. consumption: retain the observed head, remove pending, publish the committed
   store result immediately. no inventory request mediates local feedback.
4. `HistoryChanged`: atomically advance revision, clear head/pending and set
   `baselinePending`. omit `previousHead` until the qualified baseline commits;
   never delete/recreate the record and reuse its revision. unavailable is distinct
   and preserves committed state; malformed stores never silently reset.
5. failures after atomic commit remain unconfirmed. reread through the existing
   owner; never claim that no effect occurred or overwrite with a stale snapshot.

## one terminal visit

reuse existing attachment attempts and foreground lifecycle; no active-visit
registry. implement one desktop visit wrapper used by browser entry, creation
auto-entry and `skid enter`.

1. capture exact target/conversation; invalidate older notification requests.
2. activate once on first successfully presented terminal output: desktop after
   completely writing a nonempty payload; android after existing `OutputApplied`
   for the current page/attempt. `Hello` alone is insufficient. clear pending via
   the store. notification storage failure never disconnects a working terminal.
3. while active, suppress its notification; android consumes observed heads using
   existing polling. desktop adds no polling during blocking `tea.Exec`.
4. successful visits end on detach, transport loss or android backgrounding.
   invalidate older callbacks; atomically clear pending, advance revision and
   persist `baselinePending:true`. while foreground, request one fresh head after
   that commit. browser/dashboard return does not wait on the host. backgrounding
   defers the read until foreground resumes; orderly shutdown waits for the
   serialized exit write before cancelling its store scope.
5. settlement consumes that snapshot's result, stores its head and clears
   `baselinePending`. incomplete work is not retired: its later result may notify.
   failed settlement retains the flag; retry only this read on the existing
   foreground cadence. ordinary result notification stays suppressed meanwhile.

standalone `skid enter` awaits one bounded settlement attempt before exit; it has
no returning browser cadence. failure preserves the flag, reports `reply
notifications unavailable`, and leaves recovery to the next foreground client.
no detached task or new loop. notification failure is distinct from the terminal
outcome, including errors after a successful visit.

the user selected the successful source observation as the semantic closing
boundary. a reply arriving between detach and that observation
is consumed too; outages can extend that interval. this is the selected
sample-based visit policy, not a claim of exact output-time coverage.
already-produced but not yet source-visible output cannot be treated as consumed
without the prerequisite's completeness evidence.

committed state survives restart. a crash before the end flag commits cannot
prove whole-visit consumption: unseen replies can notify again. a committed
settlement flag resumes read-only settlement after restart; it never reattaches.
other browser processes share committed consumption but do not synchronize active
focus. attachment outside skid supplies no local visit event.

## content and final surfaces

the designer assigned to each client slice owns its copy, projection and review.
good content states current attention, supplies an obvious terminal-entry action,
and never asks the user to maintain read receipts. one shared projection supplies
each client's card/table, selected details, terminal header and accessibility.

| condition | literal / tone |
| --- | --- |
| fresh working | `working` / frost blue (`#78A9C6`; desktop bright blue) |
| idle or done with pending reply | `new reply` / moss green (`#76B082`; bright green) |
| idle without pending reply | `idle` / muted grey (`#AAA69D`; desktop faint/default) |
| blocked / failed | `waiting` / `failed`; existing ember |
| done / stopped without pending reply | `done` / `stopped`; muted |
| native state unavailable | `status unavailable`; existing muted/stale treatment |
| baseline/settlement outstanding / failed | secondary `checking replies` / `reply notifications unavailable`; muted |
| saved claude reply after exit in same pane | `new reply · previous agent`; green when fresh |

working and failure/waiting take primary precedence; baseline/error copy is one
muted secondary notice. suppress reply presentation during baseline/settlement.
never display working plus an old reply. live unknown/stale state does not acquire
a green claim of freshness. exception: a freshly qualified same-pane previous-agent
reply may be green without live agent status; stale inventory or result failure
still disables fresh styling. preserve committed reply metadata for recovery.
keep labels under `NO_COLOR`, no counts/pulse/new art, and no unchanged-poll announcements. status
and notification survive narrow layouts before directory detail. retain current
sorting; changing color does not change priority or imply successful work.

delete browser `r`, `replyPresentation`, `outputPresentedMsg`, `outputAck`, viewer
reference/capture helpers, cli `replies` parsing/help/execution, mobile reply
actions/sheet variant/controller callbacks and viewer-only native-read models,
encoders and decoders. manual tracking is already removed; delete the remaining
reply-only sheet. keep gateway/helper/cli `read` and its native output contract.
delete old notification pagination, baseline-staging and unread-set paths after replacement;
search callers before deleting shared validation/transport primitives.

## disjoint delivery and hard cutover

| slice | exclusive implementation paths | deliverable |
| --- | --- | --- |
| provider | llm-calling `src/provider_runtime/agent_runtime/{codex_control,claude_control,native_control_cli}.py`, directly owned result types | prerequisite proof, new head operation; preserve machine read |
| host | `internal/agentcontrol/`, `internal/gateway/conversations.go` | strict replacement request/result validation and existing helper dispatch |
| desktop + designer | `internal/fleetclient/`, `internal/sessionui/`, `internal/agentcli/`, `internal/terminalclient/` | store/reducer/visit reuse, source transport, viewer deletion, content |
| android + designer | `android/app/src/main/java/dev/niels/skidbladnir/` | equivalent reducer/visit, existing output-applied signal, viewer deletion, shared content |
| root integrator | `docs/`, `deployment/native-control/pin.json`, release/helper manifests if required | contract integration, qualified pin, acceptance and coordinated cutover |
| adversarial verifier | read-only; no implementation/test files | review every contract, red test, final change and evidence |

android primary files: `UnreadStore.kt`, `SkidbladnirController.kt`,
`AgentControl.kt`, `GatewayClient.kt`, `ProductModel.kt`, `Theme.kt`,
`SessionCard.kt`, `DashboardScreen.kt`, `TerminalScreen.kt`,
`LockedTerminalWebView.kt`, `ConversationSheet.kt`, `MainActivity.kt`.
rename unread-owned modules to notification names within their slices. no two
writers share a file; root serializes integration with the separate naming plan.

sequence: freeze/review contract -> provider qualification -> host -> desktop and
android in parallel -> integrated live/content review -> refactor -> hard cutover.
implementers write temporary tests; the verifier challenges them but writes none.
no schema codegen, legacy reader, compatibility flag or alternate provider path.

ship helper/gateway/clients as one generation. use `notifications.json` in the
existing desktop state directory and android datastore directory; initialize
schema 2 without opening or migrating `unread.json`. the old file is inert disk
data, never a runtime fallback; no legacy reader or in-app cleanup subsystem.
never touch pairings, provider data or other preferences. wrong/corrupt schemas
reject, never silently reset. rollback restores the previous complete release,
which does not read the new cache. cache continuity is not promised across
rollback; old unused bytes are an explicit small cost of this clean cutover.
initial/cutover baselines intentionally drop old notices.

root updates architecture, native interaction, agent-control/ux, desktop-browser,
design-language, roadmap and the affected issues at implementation cutover. this
plan does not resolve the [workflow](issues/reply-acknowledgement-workflow.md) or
[viewer projection](issues/desktop-reply-acknowledgement-refresh.md) issues by itself.

## temporary acceptance and review

follow [testing policy](rules/testing.md): implementer writes focused temporary
integration/live tests, proves red on original behavior, implements green,
reviews/refactors at the responsible owner, reruns affected checks, then deletes
the tests before commit. no production test seams or recreated retired harness.

| boundary | required proof |
| --- | --- |
| native helper, both providers | newest output-less work excludes old replies; unseen a/new b/delayed history; completion races; tool/compact/error cases; rewind/empty/unavailable distinctions |
| actual browser/cli -> gateway -> isolated tmux/provider | enter and auto-enter clear after presentation; failed entry preserves; whole visit consumed on return; later work finishing after closing observation notifies |
| actual android path | same lifecycle, output-applied boundary, background/reconnect, dashboard return and color/copy at narrow width |
| local state through real persistence | stale inventory/results cannot relight; concurrent clients cannot undo clearing; working clears without idle edge; next reply survives; failures/restart follow declared costs |
| removal and content | `r`/`replies`/human viewer absent; machine `read` unchanged; no acknowledgement/unread copy; blue/green/grey plus exception labels and `NO_COLOR` |

at each stage, reviewer tries the counterexample before approving the stage's
evidence. no transition proceeds on an unresolved correctness finding. once
required behavior passes, broaden testing only for a concrete remaining risk.
after temporary-test deletion run `scripts/check verify`; its engineering checks
do not prove behavior. missing device/provider/live boundaries are `NOT_RUN`.
tmux uses only owned isolated `-L` sockets; live/device operations need explicit
current-turn approval. evidence remains content/credential-free. this plan runs
no tests and authorizes no deployment or session mutation.

explicit costs: old reminders expire on resumed work, even autonomous work;
only the newest relevant reply survives offline; visit-settlement loss window;
possible repeat notice after a pre-settlement crash; per-device consumption;
first-baseline/history reset (including possible later repeat notices after a
rewind); inert old cache bytes; provider capability maintenance/unavailability;
no retained behavioral regression suite. no other implicit tradeoff is permitted.
