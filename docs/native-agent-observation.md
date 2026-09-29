# native agent interaction and unread replies

2026-09-28 · implemented and isolated-source qualified; no fleet deployment.
[qualification](native-agent-qualification.md) records evidence and limits.
[research](agent-status-research.md) owns supporting evidence. this document owns
the contract: native observation/messaging, exact codex association, explicit
controls and device-local unread. it supersedes conflicting terminal-only,
fallback, no-unread, no-native-queue, peer-exclusion, compound-stop and retained-test
proposals for this work. other product rules stand.

## 1. outcome and scope

keep terminal cards, direct tmux attachment, existing accounts, three-host routing
and current sorting. each card independently communicates execution state and
whether this device has an unacknowledged reply. omit blocking reasons.

- status/history/results use native provider interfaces only. unsupported,
  unbound, timed-out or malformed observations are unavailable, never screen
  inference, empty history or idle. explicit terminal reads/text/keys remain
  separate operations; a failed native operation never selects them automatically.
- codex: patch the native tui/app-server association; reuse the pinned helper for
  status/history, attributed messages, user input, qualified native queueing and
  exact-turn interruption. embedded/unpatched or
  remotely proxied tuis have no native binding in this release.
- claude: retain native listing, sdk history/results and exactly matched background
  stop. peer qualification failed on 2.1.284: a held message can enter a successor
  conversation after `/clear`. native peer/user/queue input is unavailable and
  rejected before dispatch. prompting remains deliberate terminal text/keys.
  no execution sdk attachment or supervisor-channel takeover.
- unread means a newly observed, provider-finalized text reply, not success or
  all work finished. one boolean marker coalesces replies. opening acknowledges
  only the replies known at activation; later replies remain unread.

public operations are `start` (create), `read`, `send` (write), `wait`, `stop`
(current work only) and `close` (halt plus terminal closure). keep existing
terminal escape hatches. presentation and cli share the backend; neither's
delivery depends on completing the other's client work.

non-goals: conversation inventory/inbox, cross-device sync, counts, push,
skid-owned scheduling/queues, offline command replay, permission-dialog ui, history replication, status
hooks, per-pane servers, new supervisors, compatibility readers or automatic
mutation retries. no changes to unrelated terminal rendering, grouping or visual identity.

## 2. owners and composition

```text
clients -> existing per-host gateway -> sessions / kernel / tmux
                                   -> agentcontrol -> short-lived native helper
                                                      -> owning provider
clients -> local unread store (identities and acknowledgement only)
```

tmux owns terminal lifetimes; the tui owns selected conversation; providers own
execution/history/queues; `llm-calling` owns native protocols; clients own unread.
collect terminal identity under the existing manager lock, release it before
provider calls, then revalidate. observations are sampled, not a fleet transaction.

reuse profile environments, kernel start identities, opaque references, strict
json/error envelopes, native transport, terminal primitives and refresh owners.
deployment adds the actual owning unix endpoint to existing codex profile rows;
no second account table. helpers disconnect without changing provider lifetime,
resuming threads or answering another client's approval requests.

reuse `NAME --machine HOST` and the existing inventory resolver. resolve once per
invocation, then dispatch the exact reference; `--ref` remains available. separate
named commands may target a replacement occupant. no aliases or addressing store.
cross-machine calls go through the target's existing gateway to its local adapter;
no provider cloud transport or new listener. reciprocal client configuration is
an installation responsibility. return addresses are ordinary message text, e.g.
`reply using: skid send coordinator --machine macbook --stdin`; use a captured
`--ref` instead when a reply must fail after target replacement.

## 3. codex selected-view contract

add to the pinned provider protocol; no new service or durable registry:

```text
Process = {pid: positive-int, startIdentity: string}
View = {viewId: uuid, revision: uint64, threadId: string|null}
tui/view/update({process: Process, threadId: string|null}) -> View
tui/view/read({process: Process}) -> {view: View|null}
turn/start, turn/interrupt, thread/queue/add add expectedView: {viewId, revision}
```

registration is owner-only on the tui's direct local unix connection. verify its
unix peer pid and kernel lifetime (`SO_PEERCRED` on linux, `LOCAL_PEERPID` on
darwin; extend `codex-rs/uds`, never substitute peer uid). start identity uses existing skid encoding:
linux start ticks; darwin start seconds × 1,000,000 + microseconds. remote/proxy
process ids never establish local identity. the server assigns a new `viewId`
per connection, including reconnect; each update advances revision, even a→b→a.
disconnect removes the record. observers cannot update it.

the tui registers null. BEFORE committing any selection change, invalidate to
null and await acknowledgement; then switch; publish the committed thread when
widget and selected conversation agree. include cached navigation, new/clear,
resume/fork, child/parent and overview. temporary voice/background routing is not
selection. failed switching republishes the old view with a new revision.
failed invalidation leaves the old selection in place. closing a local socket
is not proof of server invalidation: reconnect-null acknowledgement must revoke
the prior record for that process; the old connection cannot revive it.

guarded control admission and view updates use one short server critical section.
require matching view id/revision, non-null selection and equal request thread id;
capture the thread, release the lock, then invoke the existing native operation.
an admitted command can execute after a later switch but cannot target the new
thread. queued work belongs to the admitted conversation through later switches.
retain exact-turn checks for interrupt. never hold the registry lock
across execution/network waits or persist `expectedView` in model input/config.
skid always supplies the guard; rejection never retries unguarded. independent
provider clients keep their own existing protocol semantics.

status/read enrichment reads the view before and after native observation and
discards mismatches; revalidate the foreground tuple too. this detects observed
changes without claiming atomicity across tmux, provider execution and pixels.
saved-result recovery below is a conversation read, not a current-view assertion.

## 4. shared data and capabilities

extend the existing agent projection/reference; no parallel target registry.
`?` means omitted when absent; reject null except where explicitly shown.

```text
Conversation = {provider: Codex|Claude, profileKey, historyScope, conversationId}
Binding = {conversation: Conversation, view?: {viewId, revision}}
Status = {state: working|blocked|idle|done|failed|stopped|unknown,
          source: native|unavailable}
Turn = {id, state: inProgress|completed|failed|interrupted}
Session += {activePaneId: string}  // present for shells too
Agent += {binding?: Binding, turn?: Turn,
          status: Status,
          methods: {read: native|unavailable,
                    sendPeer: native|unavailable, sendUser: native|unavailable,
                    queueUser: native|unavailable,
                    stop: native|terminal|unavailable}}
```

retain existing machine/session lifetime/pane/pid/start fields. native action
references additionally carry the exact binding and expected turn where needed;
never resolve a stale reference into the latest thread. codex bindings require
a view; claude bindings require its existing exact pid/session match. ambiguous
background-attachment clients are unbound, not joined to workers by name/cwd.
validate an agent's pane id against `Session.activePaneId`. explicit terminal
read/text/keys and attachment are independent of native method availability.
advertise each send capability only after its exact native path is qualified.
codex requires a loaded selected thread explicitly accepting direct input.
claude native input is unavailable: exact process/socket binding cannot preserve
conversation targeting through native holding and approval. absent capability
is unavailable; do not keep the failed socket adapter as dormant code.

`historyScope` is the lowercase sha256 of `provider + NUL + realpath(historyHome)`,
computed by the host from existing configuration. it identifies storage, not
credentials or login identity. clients copy it; home/profile changes cannot
reuse another store's acknowledgement. paths/endpoints never enter client refs.

native runtime active maps to working; pending approval/input maps to blocked;
idle maps to idle; system error maps to failed; not-loaded maps to unknown with
unavailable control. claude background terminal outcomes may map to done/failed/
stopped. a previous completed codex turn does not make an idle conversation done.
execution/input availability does not depend on saved-history readiness. a fresh
codex thread can accept native input before its history is materialized; keep
its native status and send capabilities while read/results remain unavailable.
turn outcome and unread remain separate. native failures never erase cached
unread. propagate the full status through `ExecutionAgent`, not a duplicate state
field. ssh/mosh display-only contexts retain unknown and no local-native controls.

## 5. api and dispatch

reuse `POST /v1/sessions/{tmuxId}/agent/{operation}` and its error envelope.
process-bound operations retain the existing exact process target. `start` reuses
the existing create route/profile contract; creation promises no input readiness.
retire `mode:auto`; read requires `mode:native|terminal` on the wire, with native
the cli default and `--terminal` an explicit choice. native failure is returned.

| operation | added input / semantics | result |
| --- | --- | --- |
| read | native binding + `scope:latest|history`; latest assistant output by default in cli, `--history` selects bounded history; `maxBytes` retains 16 kib default/32 kib max; terminal uses capture | existing text/source/scope/truncated plus native observation below |
| send | binding + `{input:peer|user,delivery:direct|queue,text}` | native send result below |
| text | text; explicit terminal paste+submit, no readiness inference | `{method:terminal,outcome:written|unknown}` |
| keys | existing bounded logical keys | existing terminal write result |
| stop | required `method:native|terminal`; native requires binding and exact codex active turn or claude background worker; terminal uses declared provider key; retain session | native `interrupted|stopped|finished|unknown`; terminal `written|unknown` |
| close | captured target/binding/turn and declared halt method; halt then exact terminal closure | agent and terminal outcomes reported separately |
| results | session lifetime + conversation + optional cursor; no pid requirement | result metadata page below |

```text
Observation = {binding, status, turn?}
native read += {observation: Observation,
                outputState: partial|finalized|unknown|none, outputId?, outputTurnId?}
send result = {method:native, input, delivery,
               outcome:accepted,
               turnId?, queueItemId?, clientMessageId?}
wait result = {outcome:matched|timeout|target_changed,
               target:captured-ref, observation?:Observation}
```

read keeps output identity separate from the currently active turn; old output
alongside active work cannot look like that work's result. `finalized` uses §6's
predicate, `partial` requires native in-progress evidence, and absent evidence is
`unknown`. `none` requires a successful read with no assistant text. history
scope uses the existing native reader and output bound; summary fields describe
its latest assistant output, not the whole history. reject `--history --terminal`.

cli: `send NAME [--input peer|user] [--queue] TEXT|--stdin`. default input is peer;
`--queue` requires explicit `--input user`. api fields are required; reject
peer+queue before dispatch. `direct` names a native operation, not immediate
execution. retain the 32 kib text limit. no automatic permission answers, resume
or retry. use three capability keys from §4, not a generic native-send promise.

- codex user input uses `turn/start.input`; peer input uses attributed
  `turn/start.toolOutput` under namespace `skid`. use the owning app-server;
  do not invoke the ephemeral `codex_tui` adapter, impersonate its namespace,
  resume the target or register background execution. submission can start or
  join active work; report the native turn id without inventing a subtype.
- claude native input returns unavailable/not_sent before provider discovery or
  process lookup. exact recipient qualification failed, so no peer socket path,
  admission setting, internal priority mode or descendant credentials remain.
  explicit terminal input is a separate requested operation.
- codex queue uses experimental `thread/queue/add`, with the same view guard.
  it accepts user input only. a receipt earns `accepted` plus native queue-item/
  client-message ids; execution may already have begun. neither id is an
  idempotency key. uncertain delivery is never replayed. no queue editor, purge
  command, durable receipt ledger or skid scheduling state.

use genuine native sender metadata when available; otherwise identify the
integration as `skid`. never fabricate a native source conversation or promote
peer text into user input. supplied `from:` / `reply using:` lines are ordinary
attribution/routing text, not permission or identity proof. native send requires
a native receipt: direct input returns its turn id; queue input returns queue-item
and client-message ids. accepted is the sole successful send outcome and earns
exit 0. uncertainty uses the existing error envelope with `dispatch:unknown`;
never replay it or add a callback listener to manufacture acknowledgement.
terminal writing alone remains `written`, with agent state unconfirmed.

`wait NAME [--state idle|blocked|done|failed|stopped] [--timeout DURATION]` is
client-only: default idle/60 seconds, maximum one hour. resolve once, sample
immediately, then use existing five-second inventory/status cadence with one
request in flight. `justify-polling`: this foreground command watches one target;
match, timeout, cancellation, unavailability or identity/view change ends it.
cap every request by the remaining monotonic deadline. return the last admitted
observation on timeout; return no replacement observation on `target_changed`.
native unavailability is an error. only matched exits 0; cancellation stops the
waiter, never the agent. idle may match immediately and proves neither a submitted
request's completion nor an empty queue. use `read` for output; no host wait
route, job join, provider idle callback or background worker.

`stop` interrupts only captured current work; clients select the advertised
method before dispatch. no key substitute after native failure and no successor
turn chasing. fresh native idle with no active work returns `finished` without
dispatch. stop does not explicitly purge provider input. codex interruption retains
its persistent user queue and pauses normal queue
draining for that interruption event; this is not a durable queue pause.
claude background stop reports `stopped` only after the native terminal outcome
for that exact worker is confirmed; an already-finished target reports `finished`.
terminal key delivery alone never confirms cancellation.

`close` moves the old compound stop under its clearer name: attempt the selected
halt, then close the exact terminal. native background stop may terminate a
proven claude worker; an ordinary terminal-owned claude process is confirmed
stopped only on observed exit. unconfirmed halt plus successful closure remains
unconfirmed/closed. detected replacement process/view aborts closure; after final
validation, a same-process view switch can still precede it. terminal closure is
separately authorized, not atomic with native halt. never kill the shared daemon,
purge pending work, archive/delete history or promise queued work cannot run later.
codex's persistent user queue survives terminal closure. claude's pending peer
messages are not guaranteed to survive receiver exit, including background stop;
skid neither copies them nor promises their recovery.
absent halt capability disables compound close. `close --terminal-only` reuses
the exact session deletion primitive without requesting halt; plain shell close
uses that primitive too. retire the `interrupt`/`kill` cli spellings and
`/agent/interrupt` route after callers move. retain `DELETE /v1/sessions/{id}` for
shell/terminal-only close. no aliases or compound `stop` behavior.

preserve unknown after possible dispatch and no replay. map unsupported native
capability to a closed `AgentUnavailable` error with `dispatch:not_sent`; keep
existing invalid/stale error distinctions. helper startup loss during a mutation
cannot become not-sent without evidence. use the existing 2-second status budget,
10-second operation budget and 15-second client timeout; reserve close's existing
2 seconds for closure. encoded requests/replies stay within 64 kib.

reuse helper inspect/read/send/interrupt/stop primitives for the new public
meanings; add only results and native peer/queue dispatch. do not rename provider
methods merely to match product verbs. classify wait/results as observations,
never mutations. extend
the existing typed envelope/targets with endpoint, process/view and conversation
fields; batch inspect per profile. no alternate helper or provider implementation.

## 6. result recovery and local acknowledgement

```text
results request = {identityToken, conversation: Conversation, cursor?: string}
results reply = {conversation: Conversation, resultIds: string[], nextCursor?: string}
device key = (machine, provider, historyScope, conversationId)
local record = {acknowledgedIds: set<string>, unreadIds: set<string>}
association = (machine, tmuxId, identityToken, paneId) -> Conversation
store = {schema: 1, conversations: [{key, ...local record}],
         associations: [{machine, tmuxId, identityToken, paneId, conversation}]}
```

the history route validates the surviving tmux session lifetime and configured
profile/provider/scope. the supplied conversation is routing data, not proof of
current selection or new authority. use the same authenticated host access; no
caller paths/endpoints. query retained history without resuming, even after the
agent exits. no result text or prompts enter result metadata/storage.

helper eligibility is explicit:

- codex: uuidv7 turn id, non-null native `startedAt`/`completedAt`, completed
  status and `final_answer` text, qualified against the pinned history backend.
  coalesce under turn id; timestamps are evidence, never ordering keys. exclude
  inferred legacy completion, synthetic ids and turns lacking that evidence.
- claude: group native assistant blocks by inner `message.id`; require text and
  consistent `end_turn`. row uuid is not reply identity. exclude tool-use,
  refusals, length stops and api errors. native stream recovery can finalize
  partial text: this is a reply, not proof of success.

enumerate qualifying replies independently of the newest/running turn. pages
contain at most 128 unique ids, in provider history order. cursor ownership stays
in the helper: anchor to native history identity, never a bare mutable offset.
invalidated history returns `HistoryChanged`; restart enumeration idempotently.
unavailable/incomplete history is never a successful empty page. qualify native
paging and identity on the pinned versions; do not add raw-transcript or
latest-turn-only fallback. deadlines bound client waiting, not provider work.
after initialization retain ids from successful pages even if a later page fails;
before initialization discard failed/invalidated baseline staging and restart.

persist one schema-1 local store: conversation records plus terminal associations.
absent record means no baseline. after the first complete enumeration, atomically
seed the ids observed during that enumeration as acknowledged; partial/failed
baselines stay uninitialized. this is not an atomic history snapshot: first seen
later means unread, even for older restored history. later
pages add ids to unread except those already acknowledged. the sets are disjoint;
their union only grows. opening captures the displayed conversation and known
unread set; after that attempt's first successful terminal `Hello`, move exactly
that set to acknowledged. failed attachment acknowledges nothing. selection,
read/info, reconnect and remaining attached do not acknowledge later replies.

apply updates through the store's serialized merge, not stale file replacement.
acknowledgement wins for the same id; uncaptured ids stay unread. if another local
process already established baseline, a concurrent baseline attempt becomes an
ordinary observation. rewind never removes membership; fork gets a new baseline;
two tuis displaying one conversation share the device's record. never order ids
lexically or by clocks. late responses cannot update a replacement row/scope.

each fresh exact binding replaces that card's association. after exit to a shell
in the SAME pane/session lifetime, recover the remembered conversation and label
its badge as a previous-agent reply. a pane switch or live unbound provider hides
that association; a new exact binding replaces it. deleting the session ends
active recovery; keep conversation records for exact reappearance. unavailable
inventory is not deletion. no orphan cards or background polling. use existing
foreground refresh, one result read in flight per host per client; rotate across
represented conversations/pages fairly and resume scans on return.

android: separate `UnreadStore.kt` datastore using its existing dependency.
desktop: `$XDG_STATE_HOME/skidbladnir/unread.json` (default
`~/.local/state/skidbladnir/unread.json`), shared by local browser processes,
mode 0600, locked read–merge–atomic-replace;
lock a stable sidecar, not the replaced inode. expose storage failure without
clearing metadata or claiming acknowledgement saved. no silent corruption reset,
migration reader, credential-store reuse or navigation-capsule reuse. retain ids
until machine removal or explicit local-state reset; do not prune by age/count.

## 7. content contract

the design owner defines these fields/copy before implementation and reviews both
clients. good content separates activity, unread and action outcome; it never
claims success from attention, colors or transport delivery. reuse existing
status bay, angular chip, data typography and spacing; no new visual system.

| feature | visible content / acceptance |
| --- | --- |
| status | `working`, `waiting` (blocked), `idle`, `done`, `failed`, `stopped`; unknown is `status unavailable`; initial inventory uses existing `checking`; shell is `terminal` |
| unread | static `new reply`; shell recovery `new reply · previous agent`; no count, pulse, button or sorting change |
| unavailable replies/store | retain known marker; muted `replies unavailable` / `unread unavailable` in existing availability area |
| stale control | `the session changed. refresh and try again.` |
| action unavailable / accepted | `this action is unavailable for this session.` / `message accepted.`; acceptance never means completion |
| peer / user / queue | name the selected input kind in action output; `queued input accepted; it may already be running.` requires native receipt; no promise of pending state |
| wait | `observed: <state>` / `wait timed out.` / `the session changed; wait ended.`; no task-complete claim from idle |
| uncertain mutation | `could not confirm the request. check the terminal before trying again.` |
| terminal delivery | `keys sent; agent state not confirmed.` / `text sent; agent state not confirmed.` |
| stop / close | labels `stop current work` / `stop work and close terminal`; `pending input may remain`; saved provider history is retained; explicit bypass says `close terminal only` |
| partial close | `terminal closed; agent stop unconfirmed.`; replacement: `terminal left open because the session changed.` |

remove reasons and inferred markers from cards, footers and details. keep protocol
source for diagnostics. android announces state plus `new reply, unread on this
device`; shell recovery says `terminal. new reply from the previous agent,
unread on this device.` retain stale qualification. no duplicate
child announcements or repeated unchanged-poll announcements. desktop uses bold
text, preserving the gold selection cursor. state/unread survive narrow layouts
before directory/profile detail. only fresh native working may use existing
activity motion; provider failure stops it even when the gateway is reachable.
no color-only distinctions or new animations.

## 8. non-overlapping work

each builder owns its task-added temporary tests in the same slice. reviewers
write neither code nor tests. the design/content reviewer owns §7 and evaluates
each feature; client builders consume that contract. root freezes interfaces
before parallel implementation and integrates cross-slice changes.

| slice | exclusive paths / output | depends on |
| --- | --- | --- |
| a · provider | isolated pinned codex checkout: `tui`, `app-server`, protocol, `uds` and existing process primitives; selected-view admission including queue add | contract |
| b · native adapter | `llm-calling/src/provider_runtime/agent_runtime/{codex_app_server,codex_control,claude_control,native_control_cli}.py` and matching temporary tests; native reads, peer/user sends, queue add, interruption and qualification | a schema |
| c · host | skid `internal/{agentruntime,hostconfig,sessions,agentcontrol,gateway,logging}/`; exact binding, capabilities, closed dto/routes, results reads, stop/close composition | b schema |
| d · desktop/cli | `internal/{fleetclient,agentcli,sessionui,terminalclient}/`; names/ref resolution, commands/wait, delegation examples, local store and Hello acknowledgement | c schema, design |
| e · android | gateway/model/controller/terminal-connection/session-card files and new unread store under `android/app/src/main/java/dev/niels/skidbladnir/` | c schema, design |
| f · installation | dev-server codex build/install/runtime assets and skid helper/config installation, excluding root-owned manifests | a–c |
| root | docs, entry-point wiring, all pins/manifests/lockfiles, `scripts/check` composition and temporary cross-system journey | all |

f reuses existing provider homes/endpoints and publishes no new per-pane daemon.
the marked codex tui starts its provider-owned app-server on demand; that
daemon can remain after the tui exits and consumes memory per used profile.
install the merged helper source directly and codex from immutable source plus
its checksummed patch. generation identity includes exact source and patch
digests. native codex artifacts are
traversable by existing shared clients; provider account data stays private.
the fixed native socket root accepts owned 0700 or explicit client-group 0710;
only individually granted socket inodes become 0660. clients resolve the native
rendezvous alias to its existing physical listener before connecting; configured
account paths may exceed unix socket pathname limits. native sandbox exclusion
of that root remains unchanged, and startup locks remain 0600. claude's qualified
version is reconciled without changing user update policy; drift requires
requalification, and missing/malformed native interfaces remain unavailable.
claude native input stays unavailable under this contract.
root owns `deployment/native-control/`, dev-server `assets/codex/native-source.json`,
`assets/skidbladnir/*.json` and `assets/skid-provider/native-control.json`.
file changes outside a slice require reassignment before editing.

## 9. delivery and acceptance

first qualify the provider extension, message/queue admission and history identity on real boundaries;
then implement helper/host and clients against the frozen contract. each step:
write a temporary end-to-end/integration/live test, demonstrate the intended red
failure, implement, get green, adversarially review semantics, refactor under
the codebase rules, rerun the affected test, then delete task-added tests/fixtures/
instrumentation. preserve pre-existing upstream tests. no new test framework,
production-only test seams or recreated retired gates.

| acceptance | required evidence |
| --- | --- |
| identity | two tuis on one daemon; cached/child/new/resume selection; a→b→a; process/server restart; no wrong-thread status or action |
| ordering | delayed invalidation cannot commit a switch; old view rejected after invalidation; admitted send/queue remains on captured thread; read/switch race discarded |
| native behavior | real working/waiting/idle, finalized reply, failure, peer/user submit and exact-turn interrupt; no read resumes, competing approvals or provider lifetime changes |
| messaging | cross-machine native reads/replies and qualified codex peer/user input through existing gateways; exact recipient during switches; claude send unavailable before dispatch, deliberate terminal prompting stays separate; lost-response uncertainty, no invented sender authority |
| queue | supported user input starts only when native thread permits; unsupported peer queue rejected before dispatch; native receipt/restart and uncertain add without replay; no fabricated idempotency |
| lifecycle/wait | stop retains terminal; close reports halt/closure separately and preserves saved history; codex queue survives, claude pending delivery is not promised across exit; interruption does not chase successors; wait handles already-idle, timeout, cancellation, unavailable and target-change without retargeting |
| recovery | baseline; client offline through reply a and active turn b; process exit to same-pane shell; gateway/client restart; recover a once without seeing transition |
| acknowledgement | b arrives during opening a; failed attach; duplicate tui; concurrent desktop processes; rewind/fork; unavailable history/store; no accidental acknowledgement |
| disappearance | unbound replacement/pane switch hides old badge; deleted session creates no orphan row; exact reappearance preserves acknowledged membership |
| presentation | both clients, narrow viewport and accessibility; working plus unread; previous-agent result; no reasons, inference markers or false success |
| cutover | all hosts/profiles, native helper/clients/provider pins agree; shared-provider/herdr coexistence; old schemas/modes rejected; engineering checks pass after test deletion |

use isolated `-L` tmux sessions and exact owned cleanup. tmux/live and phone/adb
execution require current-turn approval under `AGENTS.md`; this plan authorizes
none of those runs now. absent boundary is `NOT_RUN`, never pass. logs contain
only identifiers/counts/outcomes needed for diagnosis, never prompts, terminal
bytes, transcripts, account paths, credentials or raw provider errors.

hard-cut in one coordinated release: remove `agentcontrol/detect.go`, terminal
status capture, auto-send/read fallback, helper not-loaded→terminal substitution,
blocking-reason dto/presentation, old CLI modes and duplicate state projection.
replace public interrupt with stop and compound stop with close; remove kill in
favor of close's terminal-only choice. update help, browser/phone labels and callers
together; no alias, retired agent route or native-to-terminal substitution survives.
remove only task-obsoleted code; leave explicit terminal primitives. root reconciles
architecture, agent-control/ux, identity projection, desktop, deployment and roadmap
docs; closes issues only on actual acceptance. no mixed-version reader/feature
flag. rollback uses the prior complete release, not parallel legacy paths.

accepted costs: provider patch/build maintenance and connection-dependent codex
selection; switching waits for native invalidation acknowledgement and stops if
that acknowledgement fails; unavailable native capability on unsupported modes/history; initial
baseline suppresses existing replies; restored unseen history counts as newly
observed; deleted-before-recovery replies cannot be recovered; a deleted terminal
has no badge; finalized partial replies can qualify; refusals, length stops and
api-error outputs do not set unread and may leave interactive claude idle;
id sets grow with history; baseline can suppress replies arriving during its
scan; full recovery can be expensive and span several refresh cycles;
test deletion leaves no new retained regression
protection. claude native peer input is unavailable because held native messages
can cross conversations; prompting requires deliberate terminal input. native
queue support differs by input kind/provider, is experimental in codex and has
no exactly-once guarantee. codex pending user input survives stop/close; claude
pending peer input can be lost at receiver exit. separate named commands
may address replacements; state waits add sampling latency and are not job joins.
close's terminal closure and native halt remain independent, potentially partial
effects, including closure after a later same-process view switch. these are
limits of the chosen contract, not hidden implementation gaps.
