# native conversations and unread replies

2026-09-29 · stock-provider source qualified; no installed-fleet acceptance.
[qualification](native-agent-qualification.md) owns evidence. this contract
supersedes selected-view registration, the codex fork, terminal-opening
acknowledgement and native user queueing. terminal and conversation are separate
targets. providers own execution/history; tmux owns terminals.

## 1. outcome and scope

keep cards, direct tmux attachment, existing accounts, machine routing, sorting
and visual language. a card may explicitly track a native conversation. status
and unread describe that conversation even when its terminal displays another
conversation or a shell. omit blocking reasons.

- native commands capture a conversation id. switching terminal a→b never
  retargets a command addressing a.
- codex uses upstream `@openai/codex`, its owning app-server and existing helper.
  no source patch, custom package, view registry or exact-version gate.
- creation captures `thread/start`'s returned id. existing conversations require
  explicit selection by id. never infer from cwd, clocks, screens, process
  presence or the newest transcript.
- claude retains native listing, sdk history/results and exact background-worker
  stop. native input remains unavailable after failed recipient qualification;
  prompting stays deliberate terminal input.
- native reads never resume/load. unsupported, malformed or missing evidence is
  unavailable; no terminal fallback, invented idle or successful empty history.
- unread means newly observed provider-finalized text, not success. opening
  native output acknowledges only replies known at opening; later replies remain
  unread. terminal attachment acknowledges nothing.

non-goals: conversation browser/inbox, cross-device unread, counts, push, copied
history, skid scheduling/queues, approval ui, per-pane servers, new supervisors,
compatibility readers or unrelated rendering changes.

## 2. composition and stock connection

```text
client -> existing target-machine gateway -> sessions / tmux / kernel
                                         -> agentcontrol -> native helper -> provider
client -> device-local unread store
```

reuse profiles, strict envelopes, bounded native transport, terminal primitives
and refresh owners. the short-lived helper owns no provider lifetime and answers
no other client's approvals.

for codex creation, choose the terminal name with the existing creation preflight,
run configured stock `app-server daemon start` under the selected `CODEX_HOME`,
create the native thread, set its native name and prepare that exact thread with
native `thread/resume`, then launch
`codex --remote unix:// --cd CWD resume ID`. explicit remote mode does not
start a server; ordinary codex may embed one. another server never attaches to
an independently running agent. manual ordinary terminals stay unassociated
until explicitly tracked.

the helper uses the account's default unix control socket, currently
`CODEX_HOME/app-server-control/app-server-control.sock`, derived from the existing
home. remove duplicate profile endpoint configuration. npm CLI and running daemon
may have different versions; upstream management owns daemon updates. install,
read and status never replace a responding owner.

stock empty threads need preparation before terminal adoption. `thread/name/set`
records the chosen session name; paginated history stores that metadata without
materializing the tui's required rollout. creation then calls native
`thread/resume` with only that exact `threadId` and validates the returned id.
this prepares persistence without starting a turn. the name is initial, not a
synchronized alias: subsequent terminal renames remain independent. require both
steps before terminal launch; no fabricated turn, history injection or alternate
creation path. unsupported preparation fails creation with its captured id.

native creation preserves the configured profile's permission policy before any
turn starts. the private helper create input is `{name, cwd, bypassPermissions:boolean}`;
host derives the flag from configured `--yolo`. true means native
`approvalPolicy:never` and `sandbox:danger-full-access`; false inherits account
defaults. remote resume restores saved permissions; omit configured `--yolo`
from its argv because stock codex rejects that override after creation.
callers never supply permission settings. native creation accepts only empty
profile arguments or the configured single `--yolo`; other argument overrides
are unavailable before daemon start, rather than partly interpreted.

validate consumed methods/fields. incompatible upgrades disable the affected
capability with a useful `AgentUnavailable` error; no alternate server or
terminal fallback. upstream app-server is experimental: accept integration
maintenance without owning a fork.

## 3. identity and association

```text
Conversation = {provider: Codex|Claude, profileKey, historyScope, conversationId}
Binding = {conversation: Conversation}
Status = {state: working|blocked|idle|done|failed|stopped|unknown,
          source: native|unavailable}
Turn = {id, state: inProgress|completed|failed|interrupted}
Methods = {read, sendPeer, sendUser, queueUser, stop: native|unavailable}
ConversationRuntime = {binding, status, methods, turn?}
Session += {conversation?: ConversationRuntime}
```

`Agent` describes foreground process/provider presence. native state has ONE
projection, `Session.conversation`. remove agent state duplication and view
fields. `historyScope` is lowercase sha256 of `provider + NUL + realpath(home)`;
it identifies history storage, not credentials. host validates profile/provider/
scope before calls. client references contain no home, endpoint or credential.

one `@skid_conversation_b64` session option records explicitly tracked codex
Conversation. creation writes its captured id. explicit association replaces or
clears it; pane/foreground/tui navigation never does. claude's exact identity
hook can supply its current conversation projection; hooks publish no status,
history or prompt data. tracking never proves what the terminal displays.

`NAME --machine HOST` resolves its recorded conversation once. direct
`--conversation ID --profile PROFILE --machine HOST` works independently of
terminal lifetime; opaque refs retain the captured structured target. fresh named
commands may resolve a replacement association; running commands keep their
original conversation. no alias store or second account table. return-address
text is ordinary delegation metadata, never authority.

`skid inspect --ref REF --json` observes a captured conversation independently
of its terminal. after local reference/configuration admission, its outer result
is `{label, machine, target:{ref, conversation, turn?}, inspection, observedRef?}`.
`target` retains the original reference bytes, conversation and captured turn;
`inspection` is the existing native success/error envelope. native, transport
or protocol failure keeps that target and exits one. malformed references,
references without a conversation and unknown machines fail the outer envelope.
names and direct conversation arguments are not accepted by this command.

successful inspection must match the captured conversation. only then,
`observedRef` encodes that same conversation's newly sampled runtime and turn,
without terminal/process identity. it is a separate target for a later explicitly
authorized action; inspection never substitutes it into an existing write.
terminal reassociation or deletion does not affect this read. the internal native
inspection result used by the browser and other clients remains unchanged.

## 4. api and behavior

reuse gateway authentication, pinned machine identity and closed envelopes.
absent fields are omitted, not null. reject old schemas/view fields outright.

| route | contract |
| --- | --- |
| `POST /v1/conversations/inspect` | conversation → sampled ConversationRuntime; read-only |
| `POST /v1/conversations/read` | conversation, latest/history scope, maxBytes → bounded native output |
| `POST /v1/conversations/send` | conversation, peer/user input, direct/queue delivery, text → native receipt |
| `POST /v1/conversations/stop` | conversation plus captured active codex turn → exact interruption |
| `POST /v1/conversations/results` | conversation, optional cursor → finalized reply ids only |
| `PUT /v1/sessions/{id}/conversation` | exact terminal lifetime + explicit Conversation; validate without resume, record |
| `DELETE /v1/sessions/{id}/conversation` | exact terminal lifetime; clear association only |
| terminal read/text/keys/stop | existing exact terminal/process targets; explicit terminal mode |
| close | exact terminal target and optional explicit native conversation/turn or terminal halt; separate outcomes |

start reuses profile/cwd/name validation and normal generated-name selection.
gateway runs native daemon start/create/name/resume before tmux creation and
writes the association with the terminal. tmux creation rechecks name occupancy;
preflight reserves nothing. do not add callbacks or a transaction framework.
naming, preparation or tmux failure after native creation returns its captured
reference as a partial outcome;
preserve dispatch certainty, never delete history or replay creation.
private create errors may include `sessionId` once known; a failed naming or
preparation step keeps its error code and reports overall `unknown`, since creation already had
an effect. the gateway exposes that exact Conversation. a later tmux failure's
dispatch describes terminal creation separately, with the Conversation retained.

```text
Observation = {binding, status, turn?}
read = {text, source:native, scope, truncated, observation,
        outputState:partial|finalized|unknown|none, outputId?, outputTurnId?}
send = {method:native, input, delivery, outcome:accepted, turnId}
wait = {outcome:matched|timeout|target_changed, target:captured-ref,
        observation?:Observation}
close = {agent, terminal} // independent outcomes; no queue-stop promise
```

output identity belongs to the assistant output, not current work. finalized
requires §5's predicate; absent evidence is unknown. none requires a successful
read with no assistant text. output is 16 kib default/32 kib max; text ≤32 kib;
envelopes ≤64 kib. retain 2-second status, 10-second operation, 15-second client
budgets and close's separate terminal-closure budget.

codex user send uses `turn/start.input`; peer send uses standalone
`turn/start.toolOutput` namespace `skid`. preserve native sender metadata when
available, never promote peer text to user input or fabricate source authority.
direct means provider admission, not immediate execution/completion. exact native
receipts earn accepted/exit zero. experimental user queueing is intentionally
unavailable in this cutover, including `--queue`; remove queue dispatch machinery.

claude saved read/results need no live process. status needs a unique native
session match. stop captures exact background job/PID/kernel lifetime, then uses
existing native stop; ambiguous or interactive workers are unavailable. native
claude peer/user/queue input rejects unavailable/not_sent before discovery.

wait is client-only: capture once, sample immediately, then existing five-second
cadence with one request in flight. default idle/60 seconds, max one hour; cap
requests by the monotonic remaining deadline. terminal switches, closure and
reassociation never retarget it. profile/history-scope changes end it. timeout
returns last observation; cancellation stops waiter only. idle is neither job
completion nor empty queue. native unavailability is an error; only matched exits
zero.

stop interrupts captured work only: no successor chasing, terminal closure or
provider queue purge. idle/no work returns finished without dispatch. terminal
keys confirm bytes written only. compound close explicitly names a conversation
and terminal and reports their outcomes separately; native close requires no
foreground-process identity. terminal halt still requires exact process proof.
closure does not establish that another displayed thread stopped. terminal-only
close reuses exact session deletion; never kill the shared daemon or erase history.

preserve dispatch unknown after possible mutation; never replay. unsupported
capability returns AgentUnavailable/not_sent. retain malformed/stale distinctions.
references capture identity and turns, not permanent capability availability;
the host checks native methods at execution. well-formed compound close still
attempts its exact terminal effect when native halt is unavailable or stale,
returning conversation stop unconfirmed. malformed input rejects before closure.

## 5. unread recovery and acknowledgement

```text
device key = (machine, provider, historyScope, conversationId)
record = {acknowledgedIds: set<string>, unreadIds: set<string>}
results = {conversation, resultIds: string[], nextCursor?}
```

codex eligibility: native uuidv7 turn id, non-null started/completed timestamps,
completed status and final_answer text. claude groups assistant blocks by inner
message.id, requiring text and consistent end_turn. exclude tool-use, refusal,
length-stop and api-error output. finalized recovered partial text can qualify;
this does not prove success.

enumerate independently of newest/running work, ≤128 unique ids per page in
native history order. helper cursors anchor to history identity; HistoryChanged
restarts enumeration idempotently. unavailable/incomplete history is never an
empty success. first complete enumeration atomically establishes baseline;
failed baseline staging remains uninitialized. thereafter retain successful pages
even if later pages fail. restored first-seen history counts unread; no clock or
lexical id ordering.

reuse serialized local-store merges: disjoint acknowledged/unread sets, growing
union, acknowledgement wins same-id races. opening captures exact conversation
and known unread ids. first successful bounded native output presentation
acknowledges exactly that capture. failed read/presentation or precommit storage
failure acknowledges nothing; later replies remain unread. an error after atomic
storage commit can leave acknowledgement written: report unconfirmed, retain known
markers until successful observation, never claim no effect. selection, terminal Hello/attachment,
reconnect, polling and staying in an output view never acknowledge later replies.

codex uses host-recorded association, never a remembered pane guess. unassociated
codex has no native status/unread. claude retains same-pane previous-agent recovery.
card deletion ends polling; retain reply records for exact reappearance.
unavailable inventory is not deletion. use existing foreground refresh, one
result call per host/client, fair scans across represented conversations.

reuse Android's separate unread datastore and desktop's shared 0600 unread file,
stable-sidecar lock and read/merge/atomic-replace. store no text/prompts/credentials.
expose storage failure without false acknowledgement or silent reset. no new store,
migration reader or retained behavioral suite.

## 6. content contract

client designer owns content and reviews each feature. good content distinguishes
tracked conversation, foreground terminal, sampled state and outcome. reuse
existing typography/status bay/chips; no new visual system.

| feature | required copy / behavior |
| --- | --- |
| association | tracking <last 8 id characters>; full id in details; never current tui selection |
| unassociated codex | conversation not tracked; no asserted native status/unread |
| native output | view replies; conversation id visible; successful opening clears captured known replies only |
| state | working/waiting/idle/done/failed/stopped; status unavailable; no reasons |
| unread | static new reply; no count/pulse/sorting; claude recovery new reply · previous agent |
| unavailable | retain known marker; replies unavailable / unread unavailable |
| send | message accepted; peer/user kind; no completion claim |
| stop/close | stop tracked conversation / stop tracked conversation and close terminal; pending input may remain; close terminal only |
| partial close | terminal closed; conversation stop unconfirmed |
| uncertain mutation | could not confirm the request. inspect the conversation before trying again |

association/state/unread survive narrow layouts before directory detail.
accessibility announces tracked id plus state/unread once, without unchanged-poll
repetition or color-only distinctions. only fresh native working uses existing
motion. terminal attachment and native output are separate visible actions.
tracking takes an explicit id, never an inferred chooser. use suffixes because
contemporaneous uuidv7 conversations share their timestamp prefixes; shortened
labels are presentation only, never command targets.

## 7. delivery and acceptance

| slice | exclusive owner |
| --- | --- |
| installer | dev-server ordinary npm install, fork/build deletion, account env, helper install |
| adapter | llm-calling stock create/read/send/interrupt/results and native Claude capture |
| host | runtime/session/agentcontrol/gateway/hostconfig; conversation routes, recorded association |
| cli/desktop | fleetclient/agentcli/sessionui; names/direct ids, wait, native output, local acknowledgement |
| android | gateway/models/controller/cards; explicit association, native output, acknowledgement |
| root | architecture/spec/roadmap, pins/manifests, integration, final evidence |

use temporary meaningful integration/live red–green–refactor tests; review then
delete before commit. engineering checks are not behavioral acceptance. tmux uses
owned isolated -L sockets; phone/adb/fleet need their approval. logs/evidence
contain no output, prompts, token or account data.

acceptance: stock npm install/repeat/upgrade; same owner for tui/helper; created-id
capture, native name and empty-thread tui adoption before its first terminal input;
explicit existing association; a→b→a switches/terminal exit never
retarget a; read never resumes; exact-turn interruption cannot cancel successor;
unsupported methods/fields/queue fail without fallback; CLI/daemon version skew
handled by consumed capabilities; native-output captured-only acknowledgement,
failed open, later reply, concurrent local merges, offline/restart recovery,
unassociated terminal; truthful separate close outcomes; Claude saved history and
background stop preserved; coordinated schemas/pins and checks after test deletion.

hard-cut viewId/revision/expectedView, selected-view registry, fork/source/build,
duplicate native state and terminal-opening acknowledgement. retain explicit
terminal primitives. immutable v0.10.4 is historical fork-contract output; ship
a new coordinated release, never mutate artifacts. rollback uses prior complete
release. no installed-fleet/phone pass without actual boundary.

accepted costs: manual existing-thread association; weaker automatic terminal
association; experimental method/socket drift may disable capabilities; independent
daemon updates may interrupt work; native creation excludes other profile-argument
overrides; possible empty thread after terminal-create
failure; sampled wait latency; unavailable user queueing; bounded output can omit
older text; baseline/restore/deletion limits above. no fork, copied history or
skid lifecycle supervisor.
