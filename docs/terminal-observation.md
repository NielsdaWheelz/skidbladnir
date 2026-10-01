# non-native terminal observation

status: implemented in source on `terminal-observation` (baseline `564d32f`);
not deployed. qualified on darwin and linux within the limits the
[qualification](terminal-agent-control-qualification.md#terminal-observation-qualification)
records; physical-phone acceptance is [pending](issues/terminal-observation-phone-acceptance.md),
and the remaining required families are recorded as issues there. the
[codex](terminal-observation-codex.md) and [claude](terminal-observation-claude.md)
grammars own the frozen capability tables, rules, rule ids and provider costs.
the user approved managed display improvements, request badges, a needs-input
filter and current-screen evidence with unknown when relevant controls are
obscured.

this owns observation, status schemas and send admission; [terminal
control](terminal-agent-control.md) keeps terminal authority, explicit native
operations and attachment ownership, and [terminal attention](reply-notifications.md)
keeps the ready machine's store and visits. host and clients cut over together.

## 1. requirements and limits

the product answers two questions independently: **is work visible? does this
terminal offer a response that needs attention?** a private third question asks
whether guarded send can safely use the ordinary composer. none is task success.

- recognize supported codex/claude working, starting, idle, permission, question,
  review, setup and menu layouts, including concurrent work and questions.
- distinguish successful ambiguity from observation failure; explain both
  without exposing terminal content. no quiet-time or process-presence idle.
- apply the same observation contract to managed and ordinary manual launches;
  no conversation id, native helper, provider hook or transcript is required.
- preserve exact session/pane/foreground guards, drafts, existing five-second
  polling, two-second enrichment deadline, cancellation and machine freshness.
- ordinary requests remain visible after a visit until fresh observation changes
  them. existing device-local `ready` notices keep their separate visit semantics.

non-goals: authoritative completion/cancellation/crash detection; unseen or
between-poll requests; task/subagent history; request payloads/counts; approval
buttons; push, inboxes, native-signal repair, remote execution inference, provider
forks, manifests/updaters, new daemons, persisted status or retained test suites.
shell return proves provider absence, not why it exited. obscured execution may
remain unknown. no implementation may claim more through its labels.

## 2. capability and wire contract

keep `session.terminalStatus`; replace its old shape everywhere. fields below
are required, non-null and closed enums. explicit native `Status` is unchanged.

```text
TerminalStatus = {
  activity: starting | working | idle | unknown,
  interaction: none | permission | question | confirmation | setup | input | menu | unknown,
  notice: none | interrupted | error,
  source: terminal | unavailable,
  reason: recognized | partial | layout_unknown | evidence_clipped | evidence_conflict |
          provider_unrecognized | remote_context | foreground_changed |
          observation_timeout | capture_failed | process_failed
}
Detection = {status: TerminalStatus, composer: empty | draft | blocked | unknown}
```

| field/value | exact meaning |
| --- | --- |
| `working` | current provider activity: generation, tools, retry or active background work; a surviving process/task count alone is insufficient |
| `starting` | recognized current startup activity; never a startup grace period |
| `idle` | qualified provider ready-state evidence; no completion or success claim |
| `none` interaction | complete recognized interaction region has no visible response request/menu; hidden requests remain unknowable |
| `input` | positive human-request evidence with no recognized subtype |
| `unknown` interaction | insufficient evidence to classify the current interaction surface |
| `confirmation` | provider-requested review/decision, such as plan implementation; not a generic selected item |
| `menu` | user navigation, such as model/settings/resume selection; not needs-input attention |
| `notice` | qualified current provider banner/modal/status notice; never transcript prose or a task outcome |

`working + question` is valid. classify each dimension independently; conflicting
evidence makes only the affected dimension unknown. `notice=none` means no
recognized current notice. historical error/interruption text cannot set it.
if no current notice structure can be qualified, emit none and make no notice
claim. notices never erase independent work/request evidence.

`unavailable` requires unknown activity/interaction and notice none, with a
failure reason. successful unknown uses `source=terminal`. changed foreground
within an otherwise valid target gives unknown/unknown/none with
`foreground_changed`; changed terminal authority rejects explicit operations.
failure reasons are exclusively observation_timeout/capture_failed/process_failed;
all other reasons require source terminal. choose timeout before the failed stage.
for a successful sample, choose foreground_changed, remote_context or
provider_unrecognized first when applicable; otherwise conflict, clipping, then
recognized/partial/layout_unknown. conflict/clipping must affect a required
inference, not unrelated transcript. recognized means both activity and
interaction classified; partial means one; layout_unknown means neither.
no failed stage may be converted to successful ambiguity.

`sessions` owns these domain types/invariants. gateway serializes them;
go/kotlin clients validate once at ingress. reject the former `state` shape,
unknown enum values and invalid combinations. no dual reader or native alias.
inventory freshness remains the only status clock; no confidence percentage,
status expiry, transition timestamp or inference history.

## 3. composition and observation boundary

```text
existing inventory / terminal inspect / guarded send
  -> sessions: exact target + focused foreground validation
  -> tmux: bounded current-screen regions
  -> agentcontrol: pure provider classifier + composer interpretation
  -> gateway -> fleetclient / android: projection, attention, filters
```

replace the boolean `visible` status-capture path with a purpose-specific
`ObservePane` primitive beside the existing public terminal read. public read
keeps its plain-text/history contract. the observation result is private:

```text
PaneObservation = {
  width, height, alternate,
  regions: [{kind: top | bottom, firstRow, rows: string[], clipped}],
  // rows preserve physical positions and bounded sgr styling
}
```

- capture the bottom 64 physical visible rows, capped at 64 kib including sgr.
  for taller panes the top region is every row above the bottom region up to row
  191 (`0..min(h−65, 191)`), capped at 16 kib and read top-down, so a byte cap
  keeps row 0. tmux refuses a client command over 16 kib, which bounds one
  guarded per-row capture to 256 rows: rows `192..h−65` of a taller pane are not
  captured. never inspect scrollback. select the region BEFORE applying its byte
  limit.
- regions are contiguous unless a byte cap or that row bound dropped rows; there
  is no separate gap. a dropped row that a required inference reads is clipped
  evidence (`evidence_clipped`); a dropped row no required read reaches costs
  nothing.
- preserve complete utf-8 rows and style state; `capture-pane -e` without `-J`.
  rows retain sgr; establish style at each retained region's start. truncation
  means lost requested evidence, not an omitted middle transcript. a clipped upper region cannot invalidate
  an intact lower composer; a clipped required boundary invalidates that inference.
  do not flatten transcript and controls into one text tail or add an emulator.
- one row parser serves both grammars. it reads sgr as tmux's writer emits it,
  OSC 8 hyperlinks and SO/SI shifts as zero width, and tmux 3.7c's tab cells as
  spaces to the next 8-column stop (3.4 writes the blank cells instead). tmux
  trims the space run after a row's last escape, so no grammar requires a
  trailing space. any other escape makes only that row unparseable: present but
  unrecognized in both grammars, never clipped evidence.
- dimension/alternate-screen changes during capture invalidate the sample.
  reuse before/after exact target and `process.SameObservation` validation.
  this is a bounded sample, not an atomic snapshot of provider execution.
- inventory passes its existing captured identity into focused observation; no
  full session-discovery or enrichment pass runs per sample. explicit controls
  resolve fresh. no capture under the metadata mutation lock; no additional
  poller, retry loop or deadline extension.
- classify only a freshly recognized local provider. retain ordinary shell and
  ssh/mosh control; their agent status is unknown, not inferred remote state.

the row/byte limits are named internal constants, not new configuration knobs.
raise them only if the required live layouts demonstrate a real need; keep a
bounded envelope and remeasure the existing deadline.

exclude title/progress classification and emission watchers/caches. tmux retains values across process
replacement; matching a provider composer does not establish their freshness.
do not collect redundant metadata solely to claim parity with herdr. the accepted
cost is unknown behind obscuring views even where herdr would guess from titles.

## 4. provider adapters and managed displays

keep one pure entry point in `agentcontrol/detect.go`; separate cohesive codex
and claude parsers where that makes their grammars readable. the
[codex](terminal-observation-codex.md) and [claude](terminal-observation-claude.md)
grammar documents own each provider's rules. share sgr/physical-row
parsing and result construction. no provider rule registry, score engine or
generic regex manifest. each rule names a complete current region, required
controls, optional variants, conflicting evidence and a content-free rule id.

locate current modal/question/composer boundaries before interpreting their
contents. a numbered list, spinner glyph, word `working`, quoted footer or empty
composer alone is insufficient. layout knowledge must tolerate supported wrapping,
optional animation and remapped keys without treating arbitrary text as controls.

| provider | required families and decisive distinctions |
| --- | --- |
| codex | activity with/without glyph, remapped interrupt, suffix and wrapped details; background-terminal waiting remains working; configured run-state row; default footer is not idle evidence |
| codex | permission overlay including open-thread footer; legacy single/multi/free-text questions; collapsed/expanded async questions; plan implementation decision; trust/update setup; ordinary pickers and transcript views |
| claude | generation/tools/retry; background agent/task/workflow/mcp activity; default/custom multiline statusline and missing interruption hint; ordinary and screen-reader layouts |
| claude | permissions; questions with navigation between confirm/cancel hints; free-text/multiple questions; plan approval; mcp elicitation; trust/login/setup; model/settings/viewer and side-question overlays |
| both | current interruption/error structures when distinguishable; empty/draft/blocked/unknown composer; active work with historical request/error text as negatives |

positive activity cannot be overridden by a persistent composer/footer. codex
idle requires its qualified explicit ready cue; unmanaged ambiguous composers
remain unknown. claude idle requires a qualified complete current ready layout,
including all relevant activity/request regions; missing spinner alone is not
enough. behind menus/viewers, retain only independently visible positive facts;
do not carry the previous activity forward.

managed codex profiles add this supported launch-scoped override:

```text
-c 'tui.status_line=["run-state","model-with-reasoning","current-dir","thread-name"]'
```

use canonical `run-state`, not its `status` alias. qualify forwarding through the
existing remote-new launcher and actual rendering. `Waiting` for a background
terminal maps to working. preserve title settings: adding run-state to titles
would churn automatic session names. persistent user settings are untouched;
existing/manual sessions use the same classifier with the cues they expose.
claude retains its display configuration: its supported progress setting adds no
signal under this screen-only contract. do not replace its custom statusline or
force accessibility mode.
unsupported display settings must be resolved during qualification, not silently
dropped or routed through an old detector.
in json launch arguments, `-c` and the expression are separate argv elements;
the expression contains no surrounding shell quote characters.

manual claude recognition is a prerequisite, not a footer problem. it uses
optional `ForegroundSignature.ExecutablePath` / json `executablePath`. admission
requires the configured path to be absolute and clean and to resolve through
symlinks to an executable regular file, and keeps the configured spelling.
matching resolves it again at each comparison and compares the result with the
kernel's `Observation.Executable`; a failed resolution matches nothing. claude
relinks its command on every auto-update, so a relink reaches the next launch
without a configuration refresh, and a session still running the previous image
is unrecognized. claude's signature is `{executablePath: "@CLAUDE@"}`; codex's is
`{executableBase: "codex"}`. populated signature fields remain conjunctive;
multiple signatures are alternatives; cross-provider ambiguity stays unrecognized.
missing/unresolvable configured paths fail configuration admission; script wrappers
do not match the native process; a deleted or replaced image matches nothing.
recognition never reads argv: npm's node launcher leads the foreground group of a
codex started through it, so that codex is a generic terminal, while managed
launches and `codex` typed in a skid shell run the native executable. no argv
fallback, version-basename matching, installation scan or new process api.
qualify bare/relative/absolute/symlink launches and provider upgrade/reload on
darwin/linux. native resume identity is separate.

## 5. controls and diagnostic api

guarded send requires a fresh local provider, activity working/idle,
interaction none, notice none and ordinary composer empty. question editors,
menus, drafts, clipped/unknown composers and unknown interactions refuse before
writing. the composer is `empty` when the ordinary composer shows only its
placeholder, `draft` when it holds input (claude bash mode included), `blocked`
when it visibly refuses ordinary input (a request or menu replaces it, it is
disabled, a side view or an external editor holds it), and `unknown` otherwise.
a refusal names its reason: a request interaction is `dialog`; otherwise a draft
composer is `draft`; otherwise `unknown`. a failed sample (observation_timeout,
capture_failed, process_failed) refuses with the existing `TerminalUnavailable`.
retain the existing buffer staging, exact guards and dispatch receipts.
better status coverage cannot independently relax composer recognition.
deliberate text/keys, stop/close and their errors retain their existing contracts.

terminal `wait --state` accepts `working`, `idle`, `needs-input`; default idle.
idle matches idle + interaction none + notice none. working matches working
even with a question. needs-input matches permission/question/confirmation/setup/
input. remove terminal `blocked`; explicit native wait keeps its native states.
validate against the captured target kind before polling. an unknown tested
dimension never matches: working+unknown interaction matches working, and
unknown activity+question matches needs-input. unavailable and target changes
retain existing wait failure behavior.

reuse `POST /v1/sessions/{id}/terminal/inspect`:

```text
request  = {identityToken, paneId, explain?: boolean} // absent = false; no null
response = {terminalStatus, diagnostics?}           // present iff explain=true
diagnostics = {
  rules: [{id, region: top | bottom | compound}],
  capture: {width, height, alternate, topClipped, bottomClipped}?,
  elapsedMs: {resolve?, capture?, classify?}
}
```

add `skid info HANDLE --explain` for terminal handles/captured terminal references;
resolve once, inspect that target, print facts and the explanation from the same
sample. reject native targets for this flag. ordinary info/json includes the
status/reason without a second sample. observation failures after target admission
must return unavailable status plus same-sample diagnostics; target/auth/identity failures
remain errors. wait still maps unavailable to its existing failure.

diagnostics have at most eight rule entries, ids of at most 48 ascii
`[a-z0-9_.-]` characters, no raw fragments, titles, paths, prompts, account data
or provider ids. rules follow the grammar's decision order, capped at eight; a
successful sample may match none. a rule's region holds its decisive rows,
`compound` when they span both regions. `agentcontrol.Diagnostics.Valid` owns
these bounds; clients call it once at ingress. omit unavailable capture
dimensions and unperformed stages; elapsed values are integer milliseconds in
`0..2147483647`, not task duration. render missing measurements as
`not collected`, no matched rules as `none`.
reuse content-free logging for failure stage/duration;
no new metrics service or trace store. successful partial classification explains
which dimensions matched; a classifier defect is not disguised as unavailable.

## 6. content, attention and filtering

the content designer owns this table and reviews every rendered feature against
it. good content names an observed action, distinguishes knowledge from outcome,
fits the existing status bay and remains meaningful without colour.

| condition | exact primary copy | tone |
| --- | --- | --- |
| starting / working | `starting` / `working` | existing blue |
| permission / question | `needs permission` / `needs answer` | existing ember |
| setup / confirmation / input | `needs setup` / `needs review` / `needs input` | ember |
| menu | `menu open` | muted |
| qualified interrupted / error notice | `interruption shown` / `error shown` | muted / ember |
| qualified existing ready notice | `ready` | existing green |
| idle / unknown / unavailable | `idle` / `status unknown` / `status unavailable` | muted |
| ordinary non-agent or remote terminal | `terminal` | muted |

priority: stale/unavailable treatment, request, menu, current notice, starting/working,
ready, idle/unknown. when request/menu/notice takes precedence over working, add
`work continues` in existing detail space. accessibility adds
`; [work continues;] inferred from terminal` only for a recognized local agent
with source terminal; unavailable/non-agent labels make no inference claim.
stale cached facts use muted `last observed: <label>`, are never ready and never
enter the filter. on desktop the rows of a host whose read failed keep that stale
cell, the selected row's facts begin `unavailable; ` and the host keeps its
notice; the rows of a host being re-read (a pending scoped read, or the re-read
after a metadata change) make no status claim: their cell reads faint `checking`
and the selected row's facts begin `checking; `.
no repeated announcement, pulse or sound for unchanged polls. retain secondary
`notifications unavailable` independently of live status.

reuse android `SessionStatusContent` for card/header and one equivalent go
projection for desktop/cli. labels, tone and predicates must derive from the
same facts; never compare rendered strings to choose behavior. preserve status
before cwd at narrow sizes. no badge stack, new palette or approve/answer action.

ready arms only on fresh working + interaction none + notice none, and appears
only when the next qualified observation is idle with those same restrictions.
fresh requests, menus, starting, notices and working clear existing pending ready
and disarm its predecessor; qualifying working then arms a new predecessor.
unknown/stale/unavailable only disarm and hide, preserving existing pending ready.
positive requests clear it even when activity is unknown. no observation gap
can bridge working to idle.
retain existing pending-store, revision, identity and visit/closing-baseline
semantics. visit consumes ready, never a current request. request→idle produces
idle, not ready; disappearance never says answered. no request ids, durable
request records, acknowledgement state or notification-store migration.

`needs input` is an independent boolean filter after existing machine/group/view
selection. its predicate is fresh + source terminal + interaction in
permission/question/confirmation/setup/input, independent of activity, notice or
reason. an unknown activity with a recognized question remains included.
menus, notices and ready alone do not qualify. desktop uses `f` and visible help;
android uses a labelled filter chip in the existing filter controls. empty copy:
`no sessions currently need input in this view`. suppress empty group headings;
keep existing unavailable-machine notices outside the filter.

preserve relative order from the chosen view; the filter adds no sorting.
desktop agents view retains its existing attention order, replacing blocked's
rank with request/menu, then idle, unknown, starting/working. group/phone order
stays stable. reuse `rebuildForFilter`: retain the exact selected row if present,
otherwise select the first visible row, or none when empty. use existing viewport
reset/clamping. refresh retains surviving keys; actions never reuse a removed
row's target.

retain the filter through terminal visits and existing dashboard restoration.
confirmed creation clears it on both clients, then cancels saved restoration and
resets the viewport: dashboard membership follows the next inventory sample, and
the creation response usually observes the instant before the provider draws, so a
kept filter could hide the new session.
extend the existing android task capsule to schema 3 with required
`needsInputOnly: boolean`; default false for a new task. hard-cut older capsules
using existing invalid-capsule handling, no migration. this resets the old task's
dashboard position once; pairings and notification storage are unaffected.
desktop keeps it in its existing in-memory view state.

diagnostic heading: `status evidence`. show activity, interaction and notice
separately, then reason, matched rules, capture and timing. exact reason copy:

| reason | copy |
| --- | --- |
| recognized / partial | `terminal controls recognized` / `some terminal controls were not recognized` |
| layout_unknown / evidence_clipped | `terminal layout not recognized` / `required screen region clipped` |
| evidence_conflict | `current screen signals conflict` |
| provider_unrecognized | `foreground program not recognized as a local agent` |
| remote_context | `remote shell; agent status unknown` |
| foreground_changed | `foreground changed during observation` |
| observation_timeout | `terminal observation timed out` |
| capture_failed / process_failed | `terminal screen could not be captured` / `foreground process could not be identified` |

unknown/unavailable detail may append `open the terminal to inspect`. no raw
error interpolation; partial never becomes `partially working`.

## 7. exclusive work and review boundaries

freeze the domain/capture interfaces before parallel implementation. root owns
contracts and assignment changes. each writer creates its temporary acceptance
probes only inside its assigned paths; a separate verifier writes nothing.
no agent may edit another slice to make its own work pass.

| slice / writer | exclusive production paths | designer's content deliverable |
| --- | --- | --- |
| a: observation boundary | `internal/tmux/`, `internal/sessions/`, `internal/process/`, `internal/agentruntime/{runtime,profile}.go` | explicit capture/recognition failure vocabulary; complete-region examples |
| b: classifier/policy | `internal/agentcontrol/` | provider screen-family grammar, rule ids, state/composer explanations |
| c: protocol/configuration | `internal/gateway/`, `internal/hostconfig/`, `internal/logging/`, `deployment/providers/host-config.json` | bounded diagnostics and reviewed managed footer composition |
| d: go clients | `internal/fleetclient/`, `internal/agentcli/`, `internal/sessionui/` | exact badge/detail/help/filter/empty-state copy from §6 |
| e: phone | `android/app/src/main/java/dev/niels/skidbladnir/` | same copy in card/header/filter, narrow layout and accessibility |
| root: integration | `docs/`, root-owned composition files if needed; temporary cross-system probes in one assigned scratch directory | coherent contract and source-attributed qualification |

each slice has a writer, content designer and adversarial reviewer; roles may
reuse people across sequential slices, but the reviewer does not approve their
own implementation. use the available three worker slots in dependency order:
a+b+c, then d+e+independent verification. producer/consumer interface changes
return to root before either side changes its assumptions. no cross-repository
deployment producer changes without inspecting and naming that assignment.

reuse target guards, kernel observations, sgr parsing, fleet routing, strict
decoders, shared status projection, existing poller and notification owner.
delete obsolete eight-line/global-truncation logic, terminal blocked mappings,
old label/sort/wait branches and duplicated capture paths. no compatibility flags,
legacy classifier, shadow production detector or redundant abstraction.

root reconciles architecture §4/§8, roadmap, terminal-control observation/send,
reply-notifications qualification, agent-control/ux, desktop-browser,
dashboard-return-continuity/groups capsule contract and codebase-map. append new
qualification; do not relabel historical passes or `NOT_RUN` results.

## 8. red / green / refactor acceptance

no behavioral tests or runtime operations in this specification change. later,
current-turn explicit approval is required for isolated tmux/integration/live;
physical android/adb requires its own explicit approval under `AGENTS.md`.
use only the exact sessions on the isolated socket created by the probes.

for every slice: designer specifies expected observations/copy; reviewer attacks
the oracle and negative cases; writer demonstrates the intended red on the
recorded baseline, implements green, then refactors. reviewer challenges the
result at the real affected boundary. rerun affected checks after refactoring;
delete all temporary tests, screens and harnesses before commit. finish with
existing engineering checks. do not recreate a retired gate or production seam.

| acceptance group | required evidence |
| --- | --- |
| provider behavior | actual stock codex/claude for applicable required §4 families; default/custom display, reduced motion, wrapped/narrow screen, permission modes, active/background/retry, single/multi/free-text/async questions and elicitation |
| capture | actual isolated tmux normal/alternate screen; >8 kib styled upper output with intact lower controls; top setup modal, tall pane, sgr/utf-8 clipping, dimension change |
| negatives | composer during active codex work; quoted old chrome; menu versus request; title-generation spinner, static claude title and inherited busy/action-required/progress across provider→shell→same provider; historical errors under current work |
| recognition | managed, bare, absolute, relative and symlink launches on darwin/linux; unrelated executables, exit/replacement and foreground suspend/resume |
| full product | real gateway→cli/desktop and gateway→physical phone: work, each request label, concurrent work, stale/unavailable, filter membership/order, visits, return restoration, accessibility/large text |
| attention | work→idle ready; work→question→idle not ready; pending ready→request/notice→idle stays idle; visit does not dismiss unanswered request; unknown/outage cannot bridge readiness; delayed response cannot restore consumed ready |
| controls | empty ordinary composer send succeeds; draft/dialog/menu/unknown/clipped/changed target refuses without bytes; explicit text/keys and stop/close unchanged |
| diagnostics/cost | reason and same-sample explanation agree; logs/envelopes contain no content; 1 and 16 represented sessions on each host platform fit the existing two-second enrichment budget with no induced timeouts; compare per-stage timing before/after |

16 is a qualification workload, not a session limit; include the actual peak
concurrency too if larger. investigate cost at its owner before changing budgets.

use independent oracles: controlled provider responses/tool gates or a human
observing the actual provider, not skid's own status. a stock tui using a local
deterministic endpoint proves tui/protocol behavior, not production service
reliability. synthetic tty frames through real tmux prove capture/parser
mechanics, not provider support. include live ordinary-provider smoke separately.
new-schema tests failing only to compile are not behavioral reds; query baseline
observable behavior with baseline-compatible probes. challenge each regression's
sensitivity using its paired negative case.

record exact baseline/candidate revisions, provider/tmux versions, platform,
dimensions, scenario ids, expected/observed categories and timing. no terminal
bytes, prompts or account data in logs/evidence, including temporary live captures.
synthetic test screens use authored dummy content; do not save real session screens.
before parsers are written,
freeze a provider/version/configuration capability table with required positive
families, permitted unknown/none cases and upstream-unavailable variants. notices
without a distinguishable current structure and obscured state permit unknown/none;
permissions/questions that the supported provider actually renders require positive
recognition. unavailable upstream features are not implemented claims or passes;
unexercised required cases are `NOT_RUN` blockers. qualify installed versions
first; no untested universal-version promise or runtime exact version gate.
measured supported cases must pass; ambiguity controls must remain
unknown and may not be waived by an overall accuracy score.

## 9. final state, costs and completion

one observer, two provider grammars, one public fact model and existing client
owners. hard-cut all producers/consumers together; no old/new mixed generation.
coordinate external cli consumers and generated deployment configuration before
shipping. rollback restores the previous complete release; publishing/installing
is a later operation, not part of writing this plan.

explicit costs:

- conservative unknown loses some idle waits and ready notices; five-second
  sampling misses brief transitions.
- codex ([grammar §6](terminal-observation-codex.md#6-accepted-costs-and-residual-ambiguity)):
  idle needs `Ready`, the main placeholder on a clean band, no external editor and
  a visible transcript terminator; goal pursuit withholds idle; status describes
  only the displayed thread; cue-less turn starts (`!cmd`, queued slash commands)
  read idle for their few tens of milliseconds, with no two-sample rule; codex
  emits no notices; managed launches replace the launch's statusline layout; a
  codex started through npm's node launcher is a generic terminal.
- claude ([grammar §5](terminal-observation-claude.md#5-accepted-costs-and-residual-ambiguity)):
  requests and menus hide activity, so working with a request is unrepresentable;
  interrupted or failed turns read idle and can raise ready; idle is lost for
  unproven pill slots, footer links, non-ordinary footers, non-work panel rows,
  usage-limit copy, colour level 0 and screen-reader sessions without a completion
  neighbour; remotely configured usage-limit copy without a default anchor reads
  idle; plugin render hooks are unsupported; `← N agents` keeps interaction none;
  request dialogs without a qualified rule read unknown.
- tall panes: one capture holds at most 256 rows, so rows `192..h−65` of a taller
  pane are clipped evidence, and idle there needs the transcript tail within the
  bottom 64 rows.
- after a claude update relinks the configured path, a session still running the
  previous image is unrecognized (a generic terminal) until relaunched; new
  launches are recognized without a configuration refresh.
- provider ui upgrades require requalification (each grammar's last section);
  broader capture costs more per sample; the capsule hard cut resets old dashboard
  context; deleting tests removes automatic future behavioral regression
  protection.

codex's cue-less windows and claude's remote usage-limit copy are the accepted
exceptions where idle can be false; no other cost justifies false idle, weaker
input admission or silent scope expansion.

complete when the matrix passes at its stated boundaries, independent reviews
find no contract violation, obsolete paths/temporary probes are removed,
engineering checks pass and docs match the final code. close only resolved
issues. remaining blockers are recorded individually; the
[qualification](terminal-agent-control-qualification.md#terminal-observation-qualification)
lists them. no engineering pass can close missing live acceptance.
