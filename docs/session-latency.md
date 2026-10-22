# session inventory and attachment latency

2026-10-20: merged and deployed in v0.18.0; source behavior was qualified on
`codex/session-latency` in an isolated worktree.
[qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/session-latency-qualification.md) records evidence and limits.
this cutover follows the [architecture](architecture.md),
[rules](rules.md) and [temporary-test policy](rules.md#testing). v4 reset
identity and bounded isolated-tmux/real-phone acceptance are explicitly approved;
[v0.18.0 installed qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/v0.18.0-installed-qualification.md) separately
records the coordinated rollout and its unperformed production-entry boundary.

diagnosis: serial native work, fleet publication barriers and terminal entry
dependencies; [the evidence](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/session-latency-qualification.md#diagnosis-and-red)
records their correction. a preexisting canceled-input dependency race is
[explicitly deferred](issues/bubbletea-canceled-input-race.md).

## target behavior and scope

| operation | required behavior |
| --- | --- |
| inventory | capture native facts in batches; normalize only where needed; preserve complete valid machine inventory |
| desktop refresh | publish each machine independently, without waiting for notifications or remote display context |
| android refresh | publish base inventory and complete its read fence before optional remote context |
| desktop captured terminal entry | validate/open that lifetime directly; no inventory, display enrichment or observer fetch |
| first actual output | render immediately; acknowledge only an exact generation captured before presentation |
| slow optional owner | preserve uncertain ready; report secondary notification failure; keep terminal input/output usable |

scope: gateway, desktop, android codecs/dataflow and existing notification owners.
no runtime replacement, cache, request singleflight, permanent control connection,
new daemon, event framework, http route, account/provider change,
input replay or visual redesign. phone interaction and visual redesign are
[separate](issues/phone-interaction-redesign.md) [issues](issues/visual-redesign.md).
the [cwd framing concern](issues/tmux-cwd-framing-race.md) is a separate fix;
do not copy that duplicated length/value encoding into inventory.

## authority and invariants

- machine, server lifetime, session, selected pane and foreground process are
  distinct identities. names/row positions never target controls.
- retain tmux-side predicates, conservative uncertain effects and actual
  post-effect reads. a gateway gate cannot serialize independent tmux clients.
- ordinary inventory calls no provider status/history API. malformed optional
  metadata stays absent; required-read failure never becomes empty success.
- only existing owners mutate state. workers receive immutable inputs and return
  values. cancellation, teardown and supersession retire their work.
- captured refs retain their lifetime. fresh short handles still require complete
  scoped inventory; incomplete scope refuses. no early first-match resolution.
- readiness remains the existing direct/producer/device join. add no reducer.

## native inventory and admission

the adapter owns framing/scope; sessions owns domain decoding, catalogue/profile
validation and projection. keep the http inventory DTO. freeze these private types:

```go
type InventoryCapture struct { Server ServerIdentity; Rows []InventoryRow }
type InventoryRow struct {
    ID, Name, PaneID string
    PanePID, AttachedClients int
    Title, CWD, Command string
    LocalSession SessionMetadata
    PaneRegistration string
}
type SessionMetadata struct {
    Character, AutoName, Group, Profile, Objective, Conversation string
}
func (Client) ReadInventory(context.Context, ServerIdentity) (InventoryCapture, error)
```

expected server is already admitted. ids, nonempty name, pane id and nonnegative
pid/count are required. title/cwd/command may be empty. metadata is exact admitted
token bytes, otherwise empty. after admission, a missing/replaced server returns
`WorkspaceSourceChanged`; the same surviving server becoming empty is valid.
malformed transcript returns `WorkspaceProtocolInvalid`, overflow `WorkspaceLimit`;
child/context failures retain their causes. initial absent/empty admission stays
outside and succeeds. additions appear next sample; disappearing rows use existing
reconciliation, never a topology failure solely because a session was added.

after existing epoch admission, use two native batches: builtin session/pane
facts, then local metadata for those discovered ids. reuse `source-file -`,
literal command construction, bounded output and native identity predicates from
`tmux/recovery.go`; do not capture the recovery graph or inherit its session cap.
bracket each batch with the expected server identity. retain `-u`, `-N`, context
cancellation and the existing 100 ms child wait allowance.

builtin records have a tag, fixed ordered fields and a final newline. encode
each field once with `#{q:field}` and an unescaped `|` terminator; decode the byte
stream before recognizing record boundaries. backslash consumes the next literal
byte; only unescaped pipe terminates a field. raw embedded newlines are data.
reject malformed framing, duplicate ids, invalid required fields or changed
server identity. use the existing 8 mib capture ceiling per transcript; overflow
fails the request, never truncates inventory. retain the existing wire limit.

read session-local `@skid_character`, `@skid_auto_name_b64`, `@skid_space_b64`,
`@skid_profile`, `@skid_objective_b64`, `@skid_conversation_b64`, and selected-pane
`@skid_agent_runtime` through `show-options -q`, with the correct local scope,
without `-A` or `-v`. admit existing canonical unquoted tokens; malformed optional
values are absent. never substitute inherited format lookup. missing/invalid
character metadata takes the existing exact raw-read/guarded-repair path.
explicit rename/reset retains its exact marker read. add no general shell parser.

sessions applies existing character/naming policy to the captured observation.
untouched rows need no rescan. writes and predicate losses require an actual
affected-row reread; never synthesize a renamed card. preserve title/pane/name/
marker guards, collision handling, and partial rename's conservative manual mode.
reuse this scanner for checkpoint normalization. release manager ownership before
kernel/process and screen enrichment; project the captured registration without
another per-row tmux read.

remove manager locks from `ValidateTerminal`, `OpenTerminal` and `PreflightCreate`:
they read immutable configuration/native facts, reserve nothing, and have exact
checks at dispatch. retain gateway `terminalLifecycle` registration/cancel/join/
close ordering and tmux's final attach predicate. inventory cannot block attachment
through the manager gate. replace actual mutation/recovery ownership with one
capacity-one context-aware manager gate. check cancellation before waiting and after acquiring;
an expired waiter releases ownership, returns its context error and starts no
native work. preserve independent native-guarded terminal observations. recovery
shutdown cancels/joins its loop, then uses uncanceled final cleanup ownership to
release its writer lock. do not introduce another scheduler or lock hierarchy.

## independent publication

reuse one raw single-machine fleetclient primitive preserving typed failures;
separate it from display-context enrichment. the complete cli list/resolver
continues aggregating that primitive.

the desktop model owns these private lanes; use existing resource owners on phone:

```text
inventory[machine] = {generation, flight?, trailingRead, admittedPeer}
context[machine]   = {sourceGeneration, flight?}
notifications     = {generation, flight?, trailingRead, admittedView?, failed}
```

one inventory request per machine. initial load and five-second ticks start idle
lanes; ordinary ticks skip occupied lanes. manual refresh starts idle lanes and
coalesces one trailing read behind occupied lanes. matching completion releases
its own flight; obsolete completion cannot alter admitted state. quit cancels and
joins owned workers; buffered completion delivery cannot strand them.

publish successful base rows immediately. failure retains that host's previous
rows/time with actions/ready disabled; another host's success cannot clear it.
preserve configured host order, selected session lifetime, view, drafts and
captured actions through partial arrivals. derive initial settlement per peer:
an early empty peer cannot establish an empty fleet.

mutations supersede only the affected peer's pre-write inventory/context and
require a post-write read. only that read can settle its metadata editor. entry
supersedes/cancels browser inventory, notification and context flights, retaining
admitted rows. launch no browser reads while the terminal owns the screen. return
starts fresh peer and notification reads independently; old completions cannot
look fresh or undo acknowledgement. ready stays unqualified until the new
notification outcome.

each successful admitted source inventory starts one optional context pass: two seconds
total, at most one per source and three overall. cancel an obsolete pass and
coalesce its latest successor without overlap. deduplicate ids within each pass;
use retained destination catalogues, otherwise the existing profile-key label.
never list sessions solely for a label. apply overlays only when source generation,
session lifetime and connection id still match; never replace a current peer with
an enriched old peer. remote rows start as `remoteUnknown`, hiding source cwd/
provider. failed peers retire overlays and never enrich stale rows. overlays
cannot change source identity, availability, status or actions.

android retains credential/runtime/read-sequence and mutation fences. publish base
inventory with cleared remote overlays, complete awaited inventory/pull-to-refresh,
then enrich. notification refresh remains independent. overlays cannot complete a
mutation fence or resurrect a replaced credential/runtime/session.

## terminal and notification contracts

desktop browser, creation and explicit-ref entry pass captured lifetime directly
to the existing websocket. cli handle entry resolves once. keep authentication,
pinned machine, session header, first resize, exact native attach predicate and
owned-client cleanup. no guessed or fabricated phone session model.

the strict greeting becomes:

```json
{"kind":"Hello","attachedClients":1,"paneId":"%3","foreground":{"provider":"Codex","pid":123,"startIdentity":"456"}}
```

`kind`, positive `attachedClients` and native `paneId` are required. optional
`foreground` is the exact `{provider:Codex|Claude,pid>0,startIdentity}` triple:
positive signed-64-bit pid and canonical positive decimal uint64 start identity.
omit it on uncertainty. reject nulls, duplicates, unknown fields and the
old count-only greeting. `Presence`, errors, client frames and binary payloads
retain their schemas. the greeting establishes no readiness or permanent pane pin.

extend the existing exact owned-client query with selected pane id/pid. sample
kernel foreground using existing observation/classification primitives, without
registration, profile environment, screen capture or `enrichSession`. freeze the
order: initial owned-client row, foreground observation a, existing final
pre-greeting row, foreground observation b. include foreground only when pane/root
are unchanged and `processinfo.SameObservation(a,b)` holds; classify that sample.
otherwise send the final row's pane and omit foreground, without retry.
required client/lifetime failure still
refuses entry. optional foreground failure never refuses a valid terminal.

use distinct `startupCtx` and `lifetimeCtx` in `sessions.OpenTerminal` and
`tmux.StartAttachment`: admission, identity and readiness use startup; the owned
tmux process uses lifetime. canceling successful startup must not kill that child.

prepare one immutable seed by existing session slot before connection:
`ReadySeed{resetId,candidate?}`, where candidate is
`{key,foreground,token:{epoch,generation>0}}`. a successful cached lookup with no candidate
is ordinary absence, not failure. browser/phone seed from admitted in-memory
device state; standalone desktop starts cached local preparation concurrently
with dialing. never fetch producer state to prepare or delay output for it.

mac adds `prepare {machine,tmuxId,identityToken} -> {resetId,candidate?}` using the existing
slot fields; candidate key/foreground/token use existing exact attention shapes.
it reads the existing serialized owner's admitted memory, performs no fetch/write,
creates no visit and waits for no launch surface. retain the existing socket
envelope, bounds and three-second operation budget. `register` removes
`readyToken`; it retains exact key/foreground, presented flag and surface/nonce,
plus the frozen optional reset id when reconstructing an already-presented visit.
`presented {visitId,resetId?,token:{epoch,generation}}` is the sole entry token input.
missing reset id permits only the zero token and no ready consumption. no
candidate/lease registry is added.

retain greeting identity; immediately before the first nonempty payload write,
freeze a matching candidate already available; a reply arriving during/after that
write cannot qualify entry.
freeze the seed's reset id even when its candidate is absent. only a completely
successful nonempty presentation submits that frozen reset id/token.
no token uses the existing zero token. greeting, queued/empty output and failed/
partial write acknowledge nothing. never replace the frozen generation later.

one visit-owned worker orders registration, presentation, reconnect and end,
replacing existing scattered orchestration. register on greeting with
`presented=false`, independently of preparation. completed output queues its frozen
token; a slow initial registration may finish later and submit that token once.
distinguish `outputWritten` from `ackAttempted`; set the latter before rpc and
never replay an uncertain attempt. the pump never waits for notification rpc/storage. owner
checks connection/visit, idempotence, exact key/foreground, reset id, epoch and
captured generation. bind the supplied reset id to the presented visit; entry
acknowledgement and later live-visit ready consumption require the current device
reset id. ordinary admissions/acks preserve it. unknown/stale reset identity
preserves ready; never substitute today's identity. physical focus rules remain.
reconnect retains this frozen identity and actual
presented state and never replays entry acknowledgement. every entry cleanup
invokes local `abandon {launchNonce} -> {}` once when a launch nonce exists,
idempotently retiring any unconsumed association, including failure before greeting
or cancellation before registration. late creator completion cannot recreate it;
this operation opens/closes no terminal or window. detach cancels preparation,
retires pending registration and closes/joins its exact connection/worker; a late
success cannot leak a visit. retain existing bounded cleanup and focus rules.

linux uses the existing local notification store, without a mac socket or new
owner. store read/admit/present/reset/update methods accept context: nonblocking flock,
retrying only `EWOULDBLOCK/EAGAIN` with a 25 ms cancellable wait and remaining
caller budget, checked again after acquisition. preserve the same sidecar lock
through read/merge/atomic replacement and retain uncertain effects. local filesystem
i/o itself has no promised hard wall limit; cancellation cannot undo a possible
replacement. moving an unbounded flock into a goroutine is insufficient.
android uses its existing nonblocking owner: snapshot one candidate before
connection, qualify on greeting, then enqueue acknowledgement after actual page
presentation. greeting qualification and subsequent page output use the same
existing main queue in receive order, fenced by attempt/owner/reset identity.

freeze these cross-slice interfaces before consumers start; no compatibility
overloads. returned captures/seeds are immutable; native/attention types do not
leak backward into terminal wire types.

| owner | interface |
| --- | --- |
| h → g | `OpenTerminal(lifetimeCtx,startupCtx,input) (*TerminalAttachment,error)`; tmux startup uses the same two-context order; attachment exposes final `{attachedClients,paneId,foreground?}` |
| g → clients | `HelloFrame{AttachedClients,PaneID,Foreground?}`; `EncodeHello(HelloFrame)`; wire foreground has exactly provider/pid/startIdentity |
| d → clients/observer | `InventoryMachine(ctx,machineHandle) Result` returns raw `Peer` or typed failure; `EnrichContexts(ctx,source Peer,catalogues []Peer) []ContextObservation`; overlay is `{sourceRef,connectionID,execution}` |
| d → n | `Read/Admit/Reset/Update(ctx,...)`; `Presented(ctx,resetId,key,foreground,token)` checks reset id inside the locked update; no background-context wrappers |
| n → d | `Run(ctx,client,encodedRef,seed,input,output) Result`; `Result{Err,NotificationErr}`; optional `*attention.ReadySeed{ResetID,Candidate *ReadyCandidate}` |
| n cached read | `Prepare(ctx,slot) (attention.ReadySeed,error)`; valid reset id plus nil candidate means successful empty preparation |
| a receive order | `onHello(attachedClients,paneId,foreground?)` precedes following binary page application on the existing main queue |

nil seed means unknown: start cached preparation concurrently. a nonnil seed with
valid reset id and no candidate is known absence. only an admitted device view may seed preparation. return
terminal result after tty restoration and owned cleanup; notification error stays
secondary and never determines terminal command exit. remove the old callback
and synchronous post-detach notification read; the browser's independent return
read owns fresh notification display.

phone card entry remains direct. retain notification-click and reconnect inventory
lookups, plus existing created-terminal post-write verification: they populate the
rich session model used by pane controls. greeting identity qualifies attention
only; never replace its pane inside old cwd/agent/status/footer metadata. the
[phone target/model split](issues/phone-entry-metadata-latency.md) is deferred.

## accepted reset contract

producer epoch/generation cannot fence a local reset followed by re-admission of
that same producer state. the approved contract above adds required `resetId`
to the existing go device snapshot: schema 4, canonical random 32-lowercase-hex,
created under its sidecar lock and rotated only by explicit reset/new memory.
ordinary admissions/acks and helper restarts preserve it. use one strict
`notifications-v4.json` store; no v3 reader, migration or fallback. android keeps
its existing reset identity. owner-instance/config counters do not replace this
domain fact; existing fetch-generation fences retain their separate purpose.
initializing an absent store is a durable mutation even for `Read`: increment
local revision once and atomically persist the reset id before returning a usable
snapshot/seed. persistence failure returns no seed; malformed existing memory
fails rather than silently initializing a new domain.

the owner approved v4 and its one-time go notification-memory reset at
coordinated cutover, potentially re-alerting current ready. sessions, pairings
and provider data remain unchanged. no v3 migration or conservative revision
fencing remains. this authorizes the contract and disposable qualification;
it does not install or reset the running notification owner.

## budgets and diagnostics

| owner | budget |
| --- | --- |
| gateway inventory work | 5 s including admission, normalization, foreground and terminal enrichment |
| terminal status enrichment | existing shared 2 s, also capped by remaining inventory budget |
| pre-upgrade lifetime validation | 5 s child of http request |
| post-resize startup through greeting | 5 s child of attachment lifetime; includes admission/readiness |
| display context | 2 s per source, at most three passes |
| optional notification operations | existing 3 s; never on terminal pump |

retain existing 15 s client, initial-resize, recovery, liveness and cleanup owners.
required inventory timeout/failure returns existing `InternalError`/500 without a
partial success; optional screen's own timeout preserves valid rows with existing
unavailable status only while the whole inventory budget remains. required
projection and the whole five-second deadline still fail the host refresh. never extend an outer deadline or mint fresh `observedAt`
after failed required validation.

`Manager.List(ctx)` returns `(Inventory,ListDiagnostics,error)`, including diagnostics
on failure. sessions owns admission/capture/normalize/foreground durations; gateway
adds terminal/projection/total. replace `Sessions.Listed` with one `Sessions.Observed`
event: outcome is `success|failed|canceled`; phase is `complete`, `admission`,
`capture`, `normalize`, `foreground`, `terminal` or `projection`; include total ms,
performed-stage ms and successful count
only. deadline expiry is `failed`; parent cancellation is `canceled`; success uses
`complete`. optional duration fields distinguish performed zero from unperformed.
reuse the typed logger; no diagnostics enter the http DTO. no raw errors, names, paths,
option values, identities, terminal/provider content, tracing endpoint or metrics
service. temporary qualification alone counts native child processes.

## content and tradeoffs

each feature has a read-only content designer: inventory, publication/context,
and terminal entry (including android). each defines fixtures/copy before coding
and reviews actual rendering after green and refactor. good content identifies
known facts, responsible host and consequences, without new chrome or color-only
meaning. preserve existing widths, wrapping, accessibility and design tokens.

| state | content contract |
| --- | --- |
| unresolved initial peer/post-write check | `<host>: checking inventory`; existing faint treatment |
| failed peer with retained rows | `<host> unavailable (<code>); showing its last observation`; rows say `last observed: <status>` |
| failed peer without retained rows | `<host> unavailable (<code>)` |
| empty before initial settlement | `checking inventory` |
| settled empty needs-input | `no sessions currently need input in this view`; keep uncertainty notices outside collection |
| other empty view with failed peers | `no matching sessions in available inventory` |
| complete empty view | `no sessions in this view` |
| remote context pending | `remote context unknown`; hide source cwd/provider |
| known context without cwd | `directory unavailable` |
| notification preparation/acknowledgement failure | existing secondary `notifications unavailable`; no modal/toast and no terminal-loop write |

background reads retain admitted status rather than flashing `checking` every
five seconds. fresh direct requests/errors remain visible when ready cannot be
qualified. optional terminal-status failure preserves the valid host/row and
ordinary attachment; required normalization failure fails that host's inventory.
while raw terminal ownership is active, collect notification failures without
writing that tty. cli prints the secondary notice after restoration; browser uses
its secondary state after return. do not expose gate/parser details in ordinary ui.

accepted costs: a native transcript parser;
independent sample times; up to three context passes with cross-source duplicate
lookups; temporarily unknown context; ready possibly remaining after an uncertain
visit; a coordinated strict greeting/local-socket cutover; required inventory
failing at five seconds rather than waiting through the client budget; temporary
tests leaving no retained regression protection; v4's one-time go notification-
memory reset may re-alert current ready at coordinated cutover.

## exclusive slices and workflow

each production file has one owner; shared changes return to root.

| slice | exclusive production paths |
| --- | --- |
| h native/runtime | `internal/sessions/**`, `internal/tmux/**` |
| g transport/contracts | `internal/gateway/**`, `internal/terminal/**`, `internal/logging/**` |
| d discovery/browser | `internal/fleetclient/**`, `internal/sessionui/**`, `internal/agentcli/**`, `internal/notifier/observer.go` (raw-read callsite only) |
| n desktop notification visit | `internal/terminalclient/**`, `internal/macnotifications/**`, `internal/attention/**` |
| a android | `SkidbladnirController.kt`, `TerminalConnection.kt`, `NotificationOwner.kt`, `ProductModel.kt` (new shared observation type only) |
| root | this spec, active contract/roadmap reconciliation, issue records, temporary integration/live test files and final evidence |

android paths are relative to `android/app/src/main/java/dev/niels/skidbladnir/`.
root reserves named temporary test files before builders start; production globs
exclude them. verifiers/designers write no production or tests. reviewers challenge
contract and test sensitivity before coding, implementation before green, refactor
after green, and evidence/test retirement before closure.

integration order: h and g first; d, n and a consume those contracts in bounded
parallel waves; root verifies the combined journey. use
existing capture, predicates, status projection, coalescers, store, visit and ui
helpers. remove unused per-field inventory reads, unconditional untouched rescans,
global browser barrier/flags, entry `info`, producer reads on entry, synchronous
presentation callback, duplicated register token and old greeting decoder paths.
retain helpers with independent callers. no compatibility dispatch or fallback.

## acceptance and cutover

root authors disposable end-to-end tests against real boundaries and demonstrates
the intended red on baseline before implementation. no synthetic timing/output
can establish native acceptance. real forwarding counters may count closed
operation categories, never arguments/content. fixtures use dummy content.

| acceptance | required boundary and proof |
| --- | --- |
| a1 batch equivalence | real isolated darwin/linux tmux 3.4/3.7c/3.8; ordinary/automatic/grouped/agent/remote rows, unicode/newline/pipe/backslash/format text, malformed local metadata, inherited marker and empty server |
| a2 normalization/identity | external rename/title/pane/collision/character changes, disappearance, server replacement and interrupted writes preserve exact guards/actual outcomes |
| a3 cancellation/startup | queued canceled requests start no child; stalled native reads respect deadline plus existing wait allowance; startup timeout cleans only owned client; successful client survives startup-context cancellation |
| a4 independent ui | real browser under pty with controlled TLS peers: healthy rows/status advance during another peer's 15 s stall or delayed notifications; selection/empty states and post-write/post-detach fences hold |
| a5 optional overlays | desktop and phone base publication precedes delayed context; obsolete overlay cannot change source/credentials/fences; repeated staggered updates eventually enrich healthy sources |
| a6 entry/acknowledgement | desktop browser/cli/create entry issues no inventory/context/producer read; greeting/current pane qualifies candidate; registration/surface/rpc stalls never block output/input, with bounded detach cleanup; empty/partial output, late candidate, newer generation, reset, replacement, reconnect and pre-greeting launch failure cannot misacknowledge/replay/leak; independent first store readers share the persisted reset id, and failed initialization supplies no seed |
| a7 coordinated codecs/phone | old/null/unknown/malformed greeting rejected; process greeting before page output; missing foreground permits output without entry acknowledgement; direct card entry, retained reconnect/notification-click/post-create admissions, rotation/background/return and focus/reset fences preserve behavior |

performance target: forty measured warm requests/entries after five warmups, on
documented quiet reference hosts, sixteen unchanged sessions: p95 complete local
http inventory and captured terminal first output below 500 ms. measure cold
startup separately; include representative codex/claude observation in a separate
provider run. unchanged native metadata capture uses bounded batches rather than
per-field/per-session children. report 64 sessions, four readers and checkpoint
overlap separately; required deadlines/failure isolation still hold under load.
report host load, stage timings and child categories; never relax fixtures to pass.

run meaningful red/green, adversarial review, refactor, rerun affected acceptance,
then delete temporary tests/harnesses and run `scripts/check verify all`. engineering
checks do not substitute for behavioral acceptance. tmux/live and phone/adb still
require explicit approval in their execution turn; absent boundary is `NOT_RUN`.
implementation and bounded isolated-tmux/real-phone acceptance are authorized;
production installation and deployment remain separate.
build combined interface changes after integration; add no compatibility stubs to
make an intermediate slice compile.

at implementation cutover, reconcile the affected clauses in architecture,
desktop browser, terminal continuity, native notifications, agent-control UX and
polling rules. stage one matching gateway/desktop/mac-app/android generation;
reject old schemas, add no negotiation. after the phone advances, repair uses a
coordinated forward release; independent host rollback and apk downgrade are unsupported.
installation/publication requires its own authorized step and installed boundary
verification. close linked latency issues only with their real acceptance evidence.
