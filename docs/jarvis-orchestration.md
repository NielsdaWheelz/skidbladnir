# jarvis orchestration changes for skid

owner-reviewed local spec and implementation plan, extracted and adversarially reviewed
2026-10-02. jarvis needs configurable worker launch, an optional initial prompt,
and honest evidence for asynchronous observation. the launch extension fits
skid's existing owners. identifying the result of a particular worker request
does not follow from adding launch flags or waiting for idle.

this document separates source requirements, owner decisions and accepted contracts.
material owner preferences are settled. source written during investigation was
accepted for retention. the owner then authorized full implementation, temporary
integration/live red/green/refactor checks and adversarial review.
[architecture §8](architecture.md#8-upgrade-ladder) records the design scope,
not permission for more code or deployment. source presence and engineering
checks do not fulfill acceptance. the acceptance table records each exercised
boundary; production installation and activation remain separate cutover work.

## source and review baseline

the source is jarvis's [active implementation plan](https://github.com/NielsdaWheelz/jarvis/blob/cc4873fabce7f0c3f0b024b3ab13242a4569b04f/docs/implementation-plan.md),
committed at `cc4873fabce7f0c3f0b024b3ab13242a4569b04f`. its document date is
2026-10-01. the relevant passages are [o1–o2, lines 109–126](https://github.com/NielsdaWheelz/jarvis/blob/cc4873fabce7f0c3f0b024b3ab13242a4569b04f/docs/implementation-plan.md#L109-L126),
[o8, lines 248–274](https://github.com/NielsdaWheelz/jarvis/blob/cc4873fabce7f0c3f0b024b3ab13242a4569b04f/docs/implementation-plan.md#L248-L274)
and [o9, lines 276–297](https://github.com/NielsdaWheelz/jarvis/blob/cc4873fabce7f0c3f0b024b3ab13242a4569b04f/docs/implementation-plan.md#L276-L297).
the source explicitly says the roadmap alone does not authorize normative changes.
jarvis's `SPEC.md` and adr 0064 now record the coordinated implementation contract.

the initial code review used skid `79679b41824b92cb80a7a055be7d46853c606fa1` and
jarvis `cc4873fabce7f0c3f0b024b3ab13242a4569b04f`, plus skid's current local
architecture and roadmap drafts. installed cli/configuration were inspected
privately; qualification uses current source builds and isolated fixtures. the
source's historical instruction to inspect `../skid-v1` is stale here: that path
does not exist. its recorded v0.10.7 installation is not a current release claim.

## owner decisions

2026-10-02: jarvis, or the owner when explicitly instructed,
decides whether to reuse a session, send more input, wait or launch another.
there is no host-enforced one-open-request rule, session reservation or mandatory
workflow sequence. tools provide clear effects and evidence; the agent owns
orchestration and interpretation.

both codex and claude are in scope in this release, including launch and input to
existing sessions. prompts, replies, partial results and blockers remain ordinary
freeform conversation. jarvis observes the selected target and interprets which
text matters and whether the brief is satisfied.

ordinary conversation is sufficient. this supersedes the interview's initial
exact-submission-attribution and explicit-return choices. there is no mandatory
worker reply tool, request id, task packet, semantic output schema, worker ledger
or exact-turn result extension. observation retains its source, target, scope
and limits; mechanical status never certifies task completion.

bounded reads are sufficient. expose truncation, unavailable text and read scope;
jarvis may ask for concise reports or further input. complete ordinary replies
and full transcript retrieval for both providers are not a release requirement.

worker observations wake jarvis for integration, not necessarily the owner.
jarvis judges when to notify: useful outcomes, material blockers and requests for
owner input should surface; individual replies can remain internal while work
continues. jarvis's notification contract must permit that distinction rather
than mandate a visible notice for every intermediate wait resolution.

model/effort values supplied at start override the provider's native account
defaults. omitted fields retain those native defaults. no additional skid
profile-default layer is added; jarvis chooses explicit launch values when useful.

agent ergonomics are in the same orchestration pr scope. discovery, targeting and
all controls use concise model-facing results and existing short typed handles.
jarvis's host retains full captured references for effects, approvals and durable
waits. the model never has to copy an opaque ref, machine uuid or native id.
the ordinary skid cli also uses compact defaults; its structured host output
retains full evidence. no alias registry or new targeting identity is added.

after the original owner turn closes, worker events permit reads, integration and
notification only. further worker input or fresh wait registration requires new
current owner authority. the owner explicitly keeps this slice narrow; autonomous
follow-through belongs to o6/o9. agent discretion over methods does not bypass the
existing write gate. no material owner preference remains open for this slice.

## extracted requirements

| source | requirement | responsible owner |
| --- | --- | --- |
| o1 | start on an explicit machine/account profile and cwd, with optional provider-compatible raw model and effort | skid |
| o1 | source proposes explicit values over compatible profile defaults, then provider-home defaults; owner simplifies this to explicit overrides then native defaults; reject unsupported pairs without substitution or shell interpolation | skid and dev-server |
| o1 | optional freeform initial prompt; skid handles readiness and submission; ordinary start without a prompt remains available | skid |
| o1 | distinguish creation, input submission and completed work; retain the captured terminal ref after input failure/uncertainty; ambiguous creation never causes relaunch | skid, then jarvis receipt persistence |
| o2 | publish/pin/install the immutable launch release and configuration; verify the intended artifact on every peer; preserve jarvis's separate cognition | dev-server |
| o8 | register/cancel `agent.wait`; return `watching` immediately; observe/read the captured target outside main's mutex; persist one outcome and one deduplicated action-resolution event | jarvis |
| o8 | restart resumes observation without resending input; bound total timeout/retries; chunk skid's one-hour waits without model polling; preserve freeform prose and evidence metadata | jarvis |
| o8 | source asks to distinguish fast completion from stale idle; owner accepts disclosed observation limits instead. retain timeout, target change, interruption and unavailable evidence; idle never proves the new request completed | jarvis consumes skid/provider evidence |
| o9 | owner and coordinators can inspect/interact directly; coordination models and fanout remain prompt choices; no descendant registry, child budgets, worker accounting or subtree cancellation | jarvis; skid retains ordinary controls |

jarvis resolves convenience choices to a configured machine/profile before calling
skid; start's tool input names both explicitly. provider convenience selection
must resolve a compatible account profile; the source does not require a new skid
provider-only selector. its preferred
`gpt-6-astra`/`xhigh` coordinator and `gpt-6-sol`/`xhigh` specialist selections
are jarvis prompt defaults, not host-wide defaults or skid capability classes.

o8 source work can proceed independently; activation needs o2's paired cli
installation and working shared cognition. o6 is needed only for broader event
write authority, and o7 is not a dependency. jarvis's owning spec
must state the accepted observation limits rather than promise exact attribution
that skid does not provide. o3–o7's cognition, callback transport, authority and
work storage remain outside this skid plan. o9's
six-hour continuation and durable stop remain jarvis/kernel responsibilities.
skid gets no durable watcher, scheduler, result store or execution supervisor.

### subsystem boundaries

| owner | responsibility and interface |
| --- | --- |
| agentruntime | validate/lower literal options; profile selects provider/account; provider owns omitted defaults |
| sessions / gateway | create once, retain complete lifetime/pane capture, fresh initial guard and exact paste/enter effect |
| fleetclient / cli | bounded create/observe/input composition, selectors and projections; no durable state |
| jarvis adapter | closed wire decoding, concise results, pre-admission capture and original-ref execution |
| jarvis action / service | receipt/outcome persistence, atomic event/cancel, restart observer outside main's mutex |
| dev-server | paired immutable installation/configuration; preserve shared provider ownership |

reuse current profile validation, capture, guarded input, native resolver/ref codec,
fleet transport, action recorder, scheduler and event dedup. the same skid mechanisms
serve jarvis→skid and skid→skid; jarvis additionally owns its existing authority,
durability and wake handling. no provider-only selector, generic shell bridge,
phone launch-option ui or notification service.

## current contract and invariants

[terminal control](terminal-agent-control.md) and [native interaction](native-agent-observation.md)
remain distinct capabilities. terminal references capture machine, session
lifetime and pane. they deliberately survive provider exit/restart/resume in
that pane. native references capture account/history scope/conversation; existing
wait observes conversation state, not the result of a captured turn.

new codex terminals have no recorded native conversation. claude registration
can supply conversation identity, but native claude input is unavailable.
neither provider gains native identity, input or task correlation merely because
start accepts a prompt. no newest-thread, timestamp, cwd or screen inference may
manufacture a native target.

terminal input receipts prove bytes written, with provider acceptance unconfirmed.
native accepted input proves admission, with task success unconfirmed. a rendered
read is bounded screen/tail text; preserve its source, scope and truncation.
neither native finality nor zero exit proves the work met its brief.

creation and input each dispatch once. later failure preserves known facts;
uncertainty never authorizes replay, replacement targeting or cleanup of user
work. a lost response can also lose the ref; skid's absence of durable command
receipts is an explicit limit, not a reason to add a recovery database.

## agent use and targeting

### discovery and selectors

baseline skid `list` and `info` already return `terminalHandle` (`t-` plus 16
lowercase hex characters) and an optional `conversationHandle` (`c-` plus 16).
their json also contains full refs and transport metadata; baseline human `info`/`start`
print full refs. baseline jarvis tool schemas accept `ref` and expose
the wire rows directly. this pr changes those agent-facing projections, not the
underlying target identities.

jarvis tool targets use `{machine, handle}` with the configured short machine
label and an existing typed handle. list/info/start and later results return that
same target shape. names remain mutable display labels, never selectors. expose
machine/group filtering through the existing cli options; empty and partial
inventories remain explicit, including unavailable hosts. do not clip away
unavailable peers or silently omit rows to produce a tidy list.

terminal handles identify session lifetimes and remain stable across rename and
active-pane changes. a handle resolves the current pane once for each new call;
the full terminal ref pins the captured lifetime and pane. native handles identify
provider/account/history-scope/conversation, not one turn. preserve this distinction
rather than claiming the short handle itself freezes every execution detail.

skid owns resolution and ref encoding. jarvis's host captures the full target
before effect admission/approval or durable wait registration, records it in the
existing action state, then executes/preflights with that original `--ref`.
restart and approval delays retain it; they never resolve the handle again to
follow a replacement pane, session or conversation. preserve each operation's
native semantics: read/send/wait address the conversation; codex stop retains the
captured turn and refuses a successor. claude interactive native stop is unavailable;
its existing background helper captures the job at dispatch, not jarvis admission.
delayed claude background-job pinning is outside this slice. a wait may observe later turns in the
same conversation. ordinary read/info calls resolve afresh. there is no independent
handle-to-ref database, ref parser or provider-id table.

terminal capture uses `info HANDLE --machine HOST --json`. for explicit native
targets, extend the existing `inspect` command to accept a `c-` handle and machine
scope, reusing skid's current native resolver and captured-inspection envelope;
retain `inspect --ref`. this narrowly supplies native capture without making
jarvis construct refs or derive ids. it adds no provider method, new native
association or fallback. unavailable native capture refuses honestly.

canonical model arguments are validated, default-completed
`model_dump(mode="json")`, then stored/hashed unchanged. capture is separate.
native gate context retains the captured profile and an optional current matching
terminal name, using machine-scoped discovery and the same full conversation
tuple. missing, ambiguous or reassociated terminals supply no name. explicit
native-target authority survives terminal loss; names never substitute a ref.
this context is captured before admission and survives delayed execution/recovery.

### concise results

ordinary cli output and jarvis tool results expose facts useful to the calling
agent. keep skid's full closed `--json` transport result for host consumers and
explicit inspection; jarvis decodes it strictly before projecting a smaller
model result. do not add another cli output mode or silently redact wire evidence.

| operation | concise result |
| --- | --- |
| list | machine, name, short handles, provider/profile, group, cwd and observed status; partial/unavailable hosts |
| info / inspect | selected short target and readable metadata/status; ordinary output omits opaque refs, machine uuids, process ids, account paths and native ids |
| start | new short target when captured, creation outcome, prompt delivery outcome and any inspectable failure |
| read | selected short target and bounded prose, with source/scope/truncation or unavailable output |
| send / text / keys | selected short target, written/accepted/not-sent/unknown outcome as applicable, and actionable refusal reason; no prompt echo |
| wait | short target, matched/timeout/target-changed/cancelled/unavailable outcome and observed state/output limits; no task-completion assertion |
| stop / close | short target and separate interruption/terminal-closure facts, including partial or unknown outcomes |

put detector details under the existing `info --explain`; keep full ids/refs in
structured host output. default creation/attachment guidance uses a short handle
and machine label. update `--help` examples and tool descriptions consistently.
an agent may start, inspect, send or wait in any useful order; no enforced
list→info→send ceremony is added. preserve `read`'s text-only stdout; concise target
and limit metadata goes to stderr or the structured result. keep stdin input
literal, without asking an agent to escape prompt text into shell arguments.

known partial facts, typed failure codes, dispatch uncertainty and nonzero-exit
semantics survive the smaller projection. unknown creation without a captured ref
retains the requested machine and evidence, with no invented handle. errors state
the known target or requested machine, what failed and the useful next inspection;
they never invite blind mutation replay. preserve
terminal/native distinctions, guarded send versus deliberate text/keys, and stop
versus closure. compact output must not hide behavior the agent needs to judge.

### content design

the content designer owns wording/examples within existing schemas; source owners
apply them in their files. good content answers: which target, what happened,
what remains unknown, what useful observation comes next. no new prose analyser,
transcript logging or content subsystem.

content examples; tests judge their facts, not exact wording:

- partial launch: `macbook t-0123456789abcdef: created; prompt not_sent; initial agent not ready; inspect this terminal.`
- unknown creation: `devbox: creation unknown; no captured target; not replayed.`
- bounded read: prose on stdout; target, terminal/visible scope and truncation on stderr.
- match with failed read: `arch t-0123456789abcdef: idle matched; text unavailable.` never claim task completion.
- partial inventory: retain every failed peer and its reason alongside available workers; never render it as an empty fleet.
- owner notice: integrate useful outcomes/blockers/questions; an unmixed intermediate wait event may remain internal.

judge fixtures for truthful information and usefulness, not exact words.
historical launch profile is not observed runtime profile. retain material partial
facts and read limits even in concise results.

## launch contract

these engineering choices implement the owner decisions above; they are not
additional requirements attributed to jarvis's source. the wire schema below is
accepted for coordinated consumers; qualification issues remain explicit.

### options and defaults

extend the existing `start [name] --machine ... --profile ...` with optional
`--model`, `--effort` and `--stdin` for initial prompt text. retain the existing
name/cwd/group contracts. terminal-only creation rejects agent options and an
initial prompt. use stdin rather than adding positional prompt syntax alongside
the existing positional name.

`agentruntime` owns provider validation and argument construction. explicit
per-field values are passed to the provider; omission preserves native account
defaults. dev-server continues to own that account configuration. do not add
`launchDefaults` or put competing model/effort defaults in skid's profile argument
bag. source o1's extra profile-default layer needs amendment. when only one field
is supplied, the provider's effective model/effort pair still determines support;
do not silently clear, translate or substitute the other field.

raw means provider values, not arbitrary argv, configuration expressions, homes,
permission flags or commands. each value is nonempty valid utf-8, at most 256
bytes, with no whitespace or controls; reject rather than trim. lower through
fixed provider argument positions and preserve the literal value. qualify the
installed providers' lowering in [launch options](issues/jarvis-launch-options.md).
locally known provider incompatibility rejects before launch; a syntactically
admissible model later rejected by the account remains a launch failure, without fallback.
do not create a duplicate model catalogue to promise account availability.

research 2026-10-02: [codex's configuration documentation](https://learn.chatgpt.com/docs/config-file/config-advanced)
describes `--model` and a fixed `--config` key with toml-encoded values;
[claude's cli reference](https://code.claude.com/docs/en/cli-reference) describes
`--model` and `--effort`. implemented lowering uses one `--model=VALUE` argument
for either provider, one `--effort=VALUE` for claude, and a fixed `--config` argument
with toml-encoded `model_reasoning_effort` for codex. values beginning with `-`
remain values. these mappings preserve provider-specific effort semantics;
darwin stock remote launch and effective selection are qualified; exact fleet
installation remains separate.

the prompt uses the existing terminal text bound: nonempty valid utf-8, at most
32 kib, no nul. preserve quotes, newlines and trailing text without trimming.
the serialized terminal request also must fit the existing 64 kib body bound.
raw admission is necessary, not a guarantee of encoded fit. reject minimum
send-body encoding, including known `{text,initialProfile}` and mandatory identity
members at their minimum valid sizes, that cannot fit BEFORE creation. check the
exact captured body again before input. metadata-dependent overflow may return created/not_sent
with target. never truncate, invent a metadata reserve or enlarge the global
bound. both preflights are implemented; the escaping-heavy case rejects before
the actual creation boundary.
keep it in transient stdin/request memory, outside argv, environment, launch
envelopes, tmux metadata, logs and qualification evidence. provider history
remains provider-owned.

### readiness and input

use bounded composition in `fleetclient`: create once, retain the returned
terminal ref, observe that original target, then use the existing guarded input
boundary. the gateway owns fresh screen/process checks and the exact paste/enter
effect. never wait while holding the session mutation lock.

initial-prompt readiness requires a fresh, available sample of the requested
provider/profile in the captured terminal, activity idle, no interaction or
notice, and an empty ordinary composer. enforce these conditions at the gateway's
initial-prompt input guard, alongside its exact sampled foreground check. ordinary
`send` continues to admit an empty composer while working; this start-only guard
does not restrict jarvis's later steering choices. qualify it in
[launch readiness](issues/jarvis-launch-readiness.md).

the route offers terminal delivery only. fresh checks cannot atomically guarantee
that the launched program consumes the subsequent input; retain terminal
delivery's stated race limitation and avoid provider-admission wording.

reuse bounded observations; perform one input attempt after readiness. unknown
layout may be observed again within the deadline. a dialog, retained draft,
unavailable observation, changed target or expired budget returns an inspectable
partial launch. never answer trust/permission dialogs automatically, switch to
deliberate text/keys or use native failure to select terminal delivery.

retain the existing 15-second total skid start budget and jarvis's 20-second child
budget. creation consumes that same budget; remaining time governs readiness and
input. cold startup may return a created terminal with prompt not sent. qualify
that path rather than silently adding time. increase these paired bounds only
if qualification demonstrates a concrete need. no phase restarts the clock.

### launch evidence

the ordinary cli envelope is `{ok:true,result:StartResult}` for a valid compound
attempt, including a partial attempt with nonzero exit. locally invalid requests
retain the ordinary `{ok:false,error}` envelope. `StartResult` is:

```text
label: configured machine label
machine: pinned machine identity
creation: not_sent | created | unknown
prompt: not_requested | not_sent | written | unknown
target?: exact opaque captured terminal ref
handle?: existing short terminal handle for target
terminal?: complete observed session {label,machine,observedAt,session}
failure?: {code,dispatch:not_sent|unknown}
```

`target` and `handle` appear together if and only if `creation:created`.
`not_sent`/`unknown` creation has no target/handle/terminal and permits only
`not_requested` or `not_sent` prompt. nested failure must not duplicate target.
wire optionals are omitted, not null; unknown/additive members reject. when
`terminal` is present, its label/machine/ref/handle match that capture. `created`
requires captured creation evidence; it does not assert the terminal is still alive.
`written` carries terminal delivery semantics, never native `accepted` wording.
input requires `creation:created` and the captured original terminal ref. unknown
creation supplies no invented target. ordinary no-prompt start returns `not_requested`.

confirmed creation plus failed/unknown input retains the same terminal ref and
returns a nonzero exit. complete no-prompt creation, or created plus written
input, earns zero to this contract only when no failure is present. known partial
facts belong in the structured result even when exit is nonzero. jarvis persists them
before settling the action. the exact outer envelope and all consumers change
together; no legacy reader or alternate decoder is added.

jarvis's model-facing failure uses `dispatch:sent` when creation or another
compound effect is known to have occurred, even if the action failed overall.
it preserves the independent phase facts. the skid wire's dispatch vocabulary remains
`not_sent|unknown`; a definite ordinary refusal stays `not_sent`, and unknown
effects remain uncertain without replay.

the gateway's successful `201` observation remains unchanged. creation errors
retain `code,message,dispatch` and may include `target:{tmuxId,identityToken,paneId}`
only for `InternalError/unknown` after the complete existing capture;
fleetclient encodes that target. it is absent, never null, before capture and
forbidden on non-creation errors. this target can survive a failure that prevents
a full observation. the strict android
creation-error consumer accepts and retains the same optional target. it is not
a new phone launch surface or an instruction to replay creation.

preserve a complete exact terminal ref as soon as existing creation validation
has captured its session lifetime and pane; later projection/readiness/input
failure cannot discard it. this includes the narrow host repair in
[launch receipts](issues/jarvis-launch-receipts.md). before that capture, do not
invent a usable ref from a session id or partial identity. no ref recovery is
promised after a lost create response; add no durable skid receipt store.

## observation contract for jarvis

ordinary conversation is accepted: start/send remain freeform; jarvis chooses
when to read/wait, receives honest target/status/text evidence and interprets its
relevance. asynchronous wait uses jarvis's existing action durability to wake
main while other conversation remains serviceable. no mandatory worker return
tool or host-owned task lifecycle is introduced.

`agent.wait` registers the selected original target, requested observable state
and finite overall timeout; jarvis chooses those values. registration returns
`watching` immediately. match, deadline expiry, target loss/change, cancellation
or unavailable evidence settles one wait action and delivers one deduplicated
host event. a subsequent read retains its own scope/truncation or failure; it is
not an atomic snapshot of the matched state. no automatic repeat wait follows a
settled registration; jarvis can register another when useful.

| stored field / lifecycle | invariant |
| --- | --- |
| registration_receipt | immutable `{action_id,target,state,deadline,recorded_at,arguments_digest,status:watching}`; digest uses canonical default-completed arguments |
| registration | queued; tool returns immediately; replay always returns original watching receipt |
| wait_outcome | initially null; separate `{target,outcome,recorded_at,cancellation_action_id?,status?,terminalStatus?,output?,failure?}` |
| matched/timeout/target_changed/unavailable | lifecycle succeeded means observation settled, never successful work |
| cancelled | original lifecycle cancelled; cancelling action reports cancelled/already_settled |
| outcome/event | one atomic commit with source deduplication |
| cancellation | original outcome/event and cancel receipt commit together; first committed outcome wins |

wait defaults: idle, 300 seconds, 16384 bytes; timeout 1–86400 seconds, text
1–32768 bytes. terminal states idle/working/needs-input; native states
idle/blocked/done/failed/stopped. wait/cancel use existing local redispatchable
recovery: two lifetime executor entries and zero external mutation attempts.
external writes retain billed-once, one lifetime entry. conclusive staged receipts
settle without executing again.

watchers use 15-second chunks under 20-second child fences, retaining original
ref/deadline across restart. startup closes stranded original inputs without
replaying them and materializes missing deduplicated events; recovery renders
from its freshly locked row. pause/cognitive quarantine suspend reads while
deadlines continue. shutdown cancels read children and joins database work before
closure. none of these rules implements durable work-stop.

only a batch entirely of `agent_wait_event_v1` observations with no owner row may
settle silently. mixed owner input, other action events and scheduled wakes retain
visible terminal/fallback rules. once the original owner turn closes, worker
events confer reads/integration/notification only, not fresh writes or waits.

terminal sends for both providers use explicit terminal targets, not native
input followed by fallback; native claude input remains unavailable. native
controls keep their independent contract when jarvis explicitly selects them.

keep skid wait synchronous, bounded and client-owned: capture once, sample
immediately, then the existing five-second cadence, at most one hour per call.
jarvis owns longer elapsed time, retry policy, cancellation, persistence and
event deduplication. restarting/chunking uses the original ref. cancellation
stops observation only. durable work-stop belongs to o7/o9, outside this slice.
neither timeout nor cancellation proves the worker
stopped; another wait is jarvis's decision, not automatic task continuation.

a terminal wait reports a state observation about a session/pane. it cannot
distinguish an old idle screen from completion before the first poll, or detect
a provider replacement within that same pane. a working-to-idle transition can
be useful evidence but loses fast work and still proves no task success.

fast completion before waiting can produce an immediate state match and available
text; jarvis evaluates that evidence. after intervening input, latest text may
concern the later input. neither observation is advertised as the exact answer
to the earlier brief. the provider owns history; jarvis may inspect available
native history when it has a valid native target, without inferring one.
no new native identity or turn-result retrieval is selected for this release.

## adversarial review

baseline gaps have source repairs and isolated live qualification; installed
artifact/cutover work stays explicit:
[cli contract](issues/jarvis-cli-contract.md),
[options](issues/jarvis-launch-options.md),
[readiness](issues/jarvis-launch-readiness.md),
[capture receipts](issues/jarvis-launch-receipts.md).

current audit: independent source reviewers plus root, followed by isolated live
qualification. these findings were reproduced, corrected and reviewed:

| priority | resolved finding | correction and evidence |
| --- | --- | --- |
| p1 | omitted uncertainty evidence raises KeyError in event/fallback | explicit optional projection; real postgres recovery publishes one event, zero replay |
| p2 | conclusive staged receipts become uncertain at restart/cancel/deadline | one receipt classifier for execution and settlement; process-crash/postgres checks preserve facts without entry |
| p2 | native name/profile consent context is incomplete | current matching full conversation tuple; real cognitive gate permits grounded/explicit targets and denies absent/conflicting context |
| medium | predictable encoded overflow creates a terminal | minimum encoding preflight before creation; exact-body preflight before input; live boundary observes zero create/input calls |
| low | android accepts creation-only target on other terminal errors | exact route-member exclusion, including null; 67 shared-decoder cases |
| low | host/consumers disagree on readiness reasons | closed legal reasons and compact refusal; real retained-draft refusal |
| low | jarvis accepts a capture with non-created launch state | capture iff created and exact nested terminal match; strict adapter cases |
| medium | submillisecond positive waits round to zero cli timeout | six-decimal datetime-precision duration; four red/green cases use the real public parser |
| low | preflight drops an observed dialog's useful instruction | existing private refusal wording; eight compact/JSON public-cli cases, zero input attempts; wire remains code/dispatch |
| medium | generic native-stop wording promises delayed claude job pinning | state captured codex-turn versus claude dispatch-time background scope; preserve existing provider contracts, add no job identity |

one suspected archive defect was withdrawn after inspecting the actual v1–v6
stored format; no compatibility wrapper was added. native accepted-send receipts
remain unstaged: a crash before generic settlement is uncertain, never replayed.
that is an existing evidence limit, not a promise of exact worker handback.
scope questions are resolved. installed-fleet and external delivery qualification
remain distinct from source correctness and local live acceptance.

## implementation sequence and acceptance

current coding assignments; every file has one writer. root integrates contracts
and reviews handoffs. shared-file work is serial, never competing edits.

| owner | exclusive files | handoff |
| --- | --- | --- |
| root / content designer | this spec; skid architecture/roadmap/feature docs/issues; jarvis SPEC/adr/active plan/operations/agent guidance/issues | accepted schemas, concise examples, dependencies/acceptance; root alone owns catalog/check composition if needed |
| skid host | agentruntime launch.go/profile.go; sessions manager.go/types.go/validation.go; gateway dto.go/gateway.go/terminal_control.go; agentcontrol terminal.go; android GatewayClient.kt | literal lowering, captured create result, initial guard, strict shared error consumers |
| skid cli/client | agentcli run.go; fleetclient client.go/content.go/handles.go/request.go/response.go/start.go; sessionui session.go | short resolution/output, budget preflight, one-shot compound result; consumes host contract |
| jarvis adapter | src/jarvis/agent_tools.py, agent_control.py | closed wire/model separation, capture context, common receipt classification, original-ref effects/observer |
| jarvis durability | src/jarvis/actions.py, write_dispatch.py, checkpoints.py, service.py, cli.py | staged-receipt recovery, immutable waits, atomic events/cancel, scheduler/shutdown; consumes adapter types/classification |
| jarvis authority | src/jarvis/write_policy.py, write_gate.py, definitions.py | readable captured gate context, tool prompts, narrow authority and wait-only notice exception |
| release/install dependency | no source files in this correction slice; existing release/install workflows belong to root/dev-server | paired immutable generation and exact peer qualification; separate authorization |

paths are relative to the named repository; skid go paths are under `internal/`,
android under `android/app/src/main/java/dev/niels/skidbladnir/`. source owners
apply designer wording in their own files. no new dependency work is planned.
review interface handoffs before parallel coding. the owner's implementation
request authorizes source changes and temporary integration/live checks;
production release and activation remain separate.

temporary red/green/refactor acceptance; do not recreate retired suites,
gates or a general harness:

| journey | real boundary and observable acceptance | meaningful red / status |
| --- | --- | --- |
| discovery/targeting/content | public cli and jarvis projections; filters, empty/partial peers, t/c capture, rename/pane change, short model ids, wrong-kind/null/additive/contradictory rejection, legal error reasons | strict adapter/client checks pass; real native capture/name/profile/read, rename/ref stability, group filters, pane change, compact refusal and partial inventory retaining an unavailable peer pass; installed paired fleet `NOT_RUN` |
| both-provider launch | isolated tmux and stock codex/claude; explicit/model-only/effort-only/omitted options, inherited owner/account/plugin, literal stdin, cold readiness and no-prompt start | darwin: eight omitted/explicit combinations with actual account environment/permissions/plugin argv; native UI/status verifies effective selection; linux: both omitted and explicit model/effort launches, literal input and normal reads/waits pass |
| refusal/lost acknowledgements | actual create/input boundary with controlled reply loss; dialogs/drafts, expiry, post-capture failure, no receipt, complete/partial close; one effect and retained facts | six darwin live cases pass, including zero-call encoded overflow, lost create/input replies and retained draft; exact post-capture failure covered at transport boundary |
| immutable delayed control | actual captured action/approval/restart with pane/association/codex-turn change; original ref, matching native consent context, no codex successor stop; claude's dispatch-time background scope disclosed | real pane-change refusal/partial close, real native capture/grounding, actual cognitive consent, strict adapter capture and real postgres restart pass; installed fleet `NOT_RUN` |
| durable observation/recovery | actual postgres transactions/process restart at registration/staging/outcome/event commits; cancel races, first commit wins, original watching replay, one event, zero worker replay | 39 postgres/process-crash cases pass; baseline fails 19 of the original 27; all four windows below pass |
| responsive conversation/notice | actual service/cognition/notification delivery; register, handle intervening owner input, integrate later status/text limits; isolated waits may be internal, mixed/owner/scheduled stay visible; post-turn reads only | eleven actual service/cli/postgres checks pass, including blocked-commit shutdown; actual main/gate/recaller registers and integrates a later wait event silently, original capture/receipt unchanged, zero extra effects; external Discord delivery `NOT_RUN` |

crash/race schedule; all four windows passed on real postgres with real child
process exit/restart, including both commit sides and cancellation orderings:

| window | required durable facts after recovery |
| --- | --- |
| before/after registration commit | no receipt before commit; if committed, replay one original receipt and resume original ref/deadline; eventually one outcome/event |
| after worker receipt staging, before generic settlement | preserve receipt; classify complete/definite partial/refusal without execution, unknown as uncertainty; one resolution |
| before/after atomic outcome-event commit | both facts commit or neither; restart produces one original registration, one terminal outcome and one source-deduplicated event |
| concurrent cancel/match | first committed original outcome wins; one original event and one cancel receipt, cancelled/already_settled as applicable |

every window adds zero worker effects. local pre-commit registration may re-enter
only within its existing two-entry bound; receipt replay never becomes new work.

write a small temporary check at each actual boundary. observe failure against the
relevant baseline/current defect BEFORE changing source. existing simulated probes
are diagnostics, not retrospective live red/green. baseline comparison uses an
isolated checkout/socket and synthetic content. after green, refactor for the
existing [rules](rules/index.md), adversarially review, rerun affected checks, then
delete temporary checks before commit. retain content-free boundary/status/count/
artifact evidence, never prompts, terminal bytes, account data or credentials.
a missing device/boundary is `NOT_RUN`.

[testing policy](rules/testing.md) owns verification. tmux/integration/live and
phone/adb need explicit current-turn approval. engineering checks do not prove
behavior. the owner's current implementation request explicitly includes
integration/live checks; phone/adb work is outside this change.

## hard cutover and tradeoffs

one paired cli/gateway/consumer generation; no legacy decoder, negotiation or
fallback. drain pending old actions/turns/events/delivery under old code, pause
and stop cleanly, stage exact artifacts/configuration, validate current contracts,
then activate/resume separately. jarvis v7 treats finalized v1–v6 worker records
as opaque; unfinished retired rows block activation. no new table/schema migration.
reuse jarvis's existing operations runbook.

paired code/config/artifact rollback is straightforward BEFORE new receipts.
after v7 writes canonical receipts, older readers are unqualified: pause, retain
data and forward-repair. whole-state restore requires demonstrated consistency
with new data and external effects; code rollback alone is not data rollback.
hard cutover deliberately forgoes rolling old/new compatibility.

accepted costs/limits: one capture subprocess per addressed write; optional
matching-name discovery for native consent within the existing call budget; one
bounded read child per active wait; existing action/event storage. bounded reads
and lost create replies limit knowledge. terminal checks retain a consumption
race; the 15-second clock can leave cold launches unsent. raw/encoded bounds can
refuse escaping-heavy prompts. claude background native stop captures its job at
dispatch; only codex retains a turn captured before approval. ordinary conversation leaves semantic attribution
to the agent. broader follow-through waits for o6/o9. none is hidden by a registry,
new lifecycle or replay.

## source verification

pr integration uses skid `566a20d849d88a618c239bf7cd8a2bfea470a6cd`
with its merged profile-usage and directory/group-entry features. independent
source reviews and the full engineering check pass after integration. the live
evidence below predates that merge; those journeys were not rerun.

final engineering checks passed after temporary test removal: skid
`scripts/check verify all` passed go formatting,
dependency/vet/build, shell/python/catalog/assets and android lint/debug assembly;
jarvis `scripts/verify` passed format/lint/types/docs/package/install/import/cli
and indexed-package audit. its targeted urllib3 2.7.0→2.8.0 lock update fixed the
reported advisories; three pinned git libraries remain audit skips.

completed temporary qualification: 16 public cli/HTTPS cases under the race detector,
plus eight preflight content cases/16 calls with zero input;
67 actual android shared-decoder cases; 50 jarvis adapter cases; 23 authority and
checkpoint diagnostics; 39 real postgres/process-crash cases; four actual native
cognitive gate decisions; isolated darwin gateway/cli/stock-provider launches and
six actual refusal/lost-acknowledgement cases. eight actual darwin process/account
checks verify omissions/overrides and preserved environment/permissions/plugin.
real native capture/read, rename-safe handles, group filters, compact info and
partial inventory retaining an unavailable peer also pass.
linux x86_64 stock codex 0.160.0 and claude 2.1.288 pass ten journeys/58 authenticated
http calls, four creations and two exact literal submissions. reds reproduce the corrected defects;
additional cases exercise strict contracts. temporary tests, harnesses, binaries,
copied configuration and credentials were deleted after review. owned isolated
tmux sessions, gateways, relays and postgres fixtures were closed; account-managed
provider daemons and history were retained. engineering checks remain the
repository's normal composition; no retired behavioral gate is restored.

service qualification uses actual public `cli.run_service`, six-table postgres,
stock skid read/wait children, action/message/checkpoint stores and fresh service
process restart. ten controlled-cognition cases plus one real blocked-postgres
commit prove coexistence, deadlines, cancellation/event ordering and joined
shutdown. a separate actual main/gate/recaller scenario uses the installed personal
shared socket and code-qualified model, seven durable model decisions and the
existing dispatcher/recorders: one owner-authorized read/wait, later silent event
integration, zero new effects. its public tool plan is tightened to agent reads
and wait; empty-store embedding and Discord delivery remain controlled. no full
connector or production-notification qualification is inferred.

the cli live fixture uses the real public `agentcli.Run`, a verified fixture ca and
an isolated TLS forwarder into the current stock gateway. it proves current source
composition, not a published binary or the production TLS route. postgres fixtures
use the actual six-table metadata and pgvector. provider history/account daemons
remain provider-owned. no phone/adb or production activation is claimed.
the linux daemon ensure path ran normally; preexisting versus newly started
daemon cannot be established from the initial symlink-blind presence check.
account daemons were never stopped. a fresh-directory trust refusal was preserved;
successful input used each account's existing trusted directory, without changing
trust/permissions or automatically answering a dialog.
