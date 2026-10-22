# non-native terminal observation

status: implemented, released as `v0.11.0` and deployed. qualified on darwin,
linux and a physical phone within the limits the
[qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/terminal-agent-control-qualification.md#terminal-observation-qualification)
records ([remaining phone rows](issues/unrun-acceptance.md)),
and the [roadmap](roadmap.md#non-native-status-and-needs-input--source-implemented)
indexes every open item. the [codex](terminal-observation-codex.md) and
[claude](terminal-observation-claude.md) grammars own the frozen capability
tables, rules, rule ids and provider costs.
the user approved managed display improvements, request badges, a needs-input
filter and current-screen evidence with unknown when relevant controls are
obscured.

this owns the status facts, the observed screen regions and their limits,
provider classification, guarded-send admission and the terminal wait states;
[terminal control](terminal-agent-control.md) keeps terminal targets, the tmux
capture mechanics, effects, routes, wait's polling loop and the observation
sample's composition, and [terminal attention](reply-notifications.md) keeps the
ready machine's store and visits. host and clients cut over together for the
observation protocol. the 2026-10-01 [attention revision](reply-notifications.md)
is implemented separately; it changes client memory and visits without changing
observation facts or the host protocol. [attention qualification](reply-notifications.md#qualification)
records the new runtime evidence.

[terminal usage](terminal-usage.md) owns optional claude quota evidence from this
same captured sample. its narrow formatter grammar leaves status, composer,
reasons, rules and control admission unchanged; unsupported quota text is omission.

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
StatusPresentation = {label, tone: working | attention | muted,
  queue: none | ready | action, detail?, evidence?, queueNotice?}
```

each inventory and creation row also carries `status: StatusPresentation`, the
host's rendering of `terminalStatus` as if fresh and not ready (§6). queue
`ready` means eligible: a client keeps it only while fresh and while the hub
holds a pending ready. optional members are omitted, never null or empty.

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
| `notice` | how the latest turn ended abnormally: the provider's own interruption or error marker as the transcript's last item (or a qualified current banner); `none` otherwise |

`working + question` is valid. classify each dimension independently; conflicting
evidence makes only the affected dimension unknown. `notice=none` means no
recognized current notice. only the marker that ends the transcript counts:
historical error/interruption text above a later prompt or turn cannot set it.
a notice persists until the next turn replaces it. notices never erase
independent work/request evidence.

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

`sessions` owns these domain types/invariants. gateway serializes them; the go
client validates once at ingress and kotlin decodes the closed enums it reads. reject the
former `state` shape, unknown enum values and, in go, invalid combinations. no dual reader or native alias.
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
  each row is self-contained: it keeps its sgr and parses from the default style
  ([terminal control §2](terminal-agent-control.md#2-observation-and-schemas)).
  truncation means lost requested evidence, not an omitted middle transcript. a
  clipped upper region cannot invalidate an intact lower composer; a clipped
  required boundary invalidates that inference. do not flatten transcript and
  controls into one text tail or add an emulator.
- one row parser serves both grammars. it reads sgr as tmux's writer emits it,
  OSC 8 hyperlinks and SO/SI shifts as zero width, and tmux 3.7c's tab cells as
  spaces to the next 8-column stop (3.4 writes the blank cells instead).
  `capture-pane` runs without its `-N`, so tmux trims the space run after a row's
  last escape and no grammar requires a trailing space. any other escape makes
  only that row unparseable: present but unrecognized in both grammars, never
  clipped evidence.
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
| codex | activity with/without glyph, remapped interrupt, suffix and wrapped details; background-terminal waiting remains working; a run-state word when the user's status line shows one; idle from a settled transcript, never from the footer alone |
| codex | permission overlay including open-thread footer; legacy single/multi/free-text questions; collapsed/expanded async questions; plan implementation decision; trust/update setup; ordinary pickers and transcript views |
| claude | generation/tools/retry; background agent/task/workflow/mcp activity; default/custom multiline statusline and missing interruption hint; fullscreen and classic layouts |
| claude | permissions; questions with navigation between confirm/cancel hints; free-text/multiple questions; plan approval; mcp elicitation; trust/login/setup; model/settings/viewer and side-question overlays |
| both | current interruption/error structures when distinguishable; empty/draft/blocked/unknown composer; active work with historical request/error text as negatives |

positive activity cannot be overridden by a persistent composer/footer. codex
idle requires a settled transcript (its last stop is a turn-end separator, a
`■` notice cell or the session header), no status row, the main placeholder on a
clean band, no external editor and no goal indicator; codex hides its status
row while it streams a final answer, so a prompt as the last stop is never idle. claude idle requires a qualified complete current ready layout,
including all relevant activity/request regions; missing spinner alone is not
enough. behind menus/viewers, retain only independently visible positive facts;
do not carry the previous activity forward.

skid adds no status-line or title override to codex launches; managed and manual
sessions read the same screen. when a user's own status line shows a run-state
word, `Working`, `Thinking` and `Waiting` mean working, `Starting` starting, and
`Ready` beside a status row is a conflict. persistent user settings are untouched.
claude retains its display configuration: its supported progress setting adds no
signal under this screen-only contract. do not replace its custom statusline or
force accessibility mode.
unsupported display settings must be resolved during qualification, not silently
dropped or routed through an old detector.
in json launch arguments, `-c` and the expression are separate argv elements;
the expression contains no surrounding shell quote characters.

manual claude recognition is a prerequisite, not a footer problem.
[claude update recognition](claude-update-recognition.md) owns the replacement
contract and qualification: optional `ForegroundSignature.ExecutableDirectory` /
json `executableDirectory` names the canonical native versions directory. admission
resolves that directory once; matching compares the kernel executable's immediate
parent with it. launch configuration stays separate, so relinking the launcher
does not exclude an older surviving image. linux's deleted-image suffix is rejected
by the directory predicate; deleted-image recovery is unsupported.

codex retains `{executableBase: "codex"}`. populated signature fields remain
conjunctive; multiple signatures are alternatives; cross-provider ambiguity stays
unrecognized. missing/unresolvable directories fail admission. recognition retains
foreground process-group leader selection and never reads argv. npm or waiting-shell
launchers can therefore hide a provider; [wrapper discovery](issues/provider-wrapper-recognition.md)
is separate. no wrapper fallback, version-name matching, installation scan or new
process api. native resume identity and provider screen grammar remain independent.

## 5. controls and diagnostic api

guarded send requires a fresh local provider, activity working/idle,
interaction none and ordinary composer empty; a notice does not refuse, since
it is a transcript marker, not a surface over the composer. question editors,
menus, drafts, clipped/unknown composers and unknown interactions refuse before
writing. the composer is `empty` when the ordinary composer shows only its
placeholder, `draft` when it holds input (claude bash mode included), `blocked`
when it visibly refuses ordinary input (a request or menu replaces it, it is
disabled, a side view or an external editor holds it), and `unknown` otherwise.
`blocked` refuses as `unknown` does; it stays as diagnostic vocabulary.
a refusal names its reason: a request interaction is `dialog`; otherwise a draft
composer is `draft`; otherwise `unknown`. a failed sample (observation_timeout,
capture_failed, process_failed) refuses with the existing `TerminalUnavailable`.
retain the existing buffer staging, exact guards and dispatch receipts.
better status coverage cannot independently relax composer recognition.
deliberate text/keys, stop/close and their errors retain their existing contracts.

terminal `wait --state` accepts `working`, `idle`, `needs-input`; default idle.
idle matches idle + interaction none, whatever notice the turn ended with, so a
wait ends on an interrupted or failed turn too. working matches working
even with a question. needs-input matches permission/question/confirmation/setup/
input. remove terminal `blocked`; explicit native wait keeps its native states.
validate against the captured target kind before polling. an unknown tested
dimension never matches: working+unknown interaction matches working, and
unknown activity+question matches needs-input. unavailable and target changes
retain existing wait failure behavior.

reuse `POST /v1/sessions/{id}/terminal/inspect`:

```text
request  = {identityToken, paneId, explain?: boolean} // absent or null = false
response = {terminalStatus, diagnostics?}           // present iff explain=true
diagnostics = {rules: [id]}
```

add `skid info HANDLE --explain` for terminal handles/captured terminal references;
resolve once, inspect that target, print facts and the explanation from the same
sample. reject native targets for this flag. ordinary info/json includes the
status/reason without a second sample. observation failures after target admission
must return unavailable status plus same-sample diagnostics; target/auth/identity failures
remain errors. wait still maps unavailable to its existing failure.

rule ids are string literals of `[a-z0-9_.-]` written in the grammars, never
built from screen text: no raw fragments, titles, paths, prompts, account data
or provider ids. rules follow the grammar's decision order; a successful sample
may match none, printed `none`.
reuse content-free logging for failure stage/duration;
no new metrics service or trace store. successful partial classification explains
which dimensions matched; a classifier defect is not disguised as unavailable.

## 6. content, attention and filtering

2026-10-02 accepted design: [session views](session-views.md) supersedes this
section's former independent ui filter and ordering. its human queue includes
ready, requests and current notices.
this document retains status facts, copy/precedence and the strict request
predicate used by cli wait. the clients implement the exclusive queue view.

the content designer owns this table and reviews every rendered feature against
it. good content names an observed action, distinguishes knowledge from outcome,
fits the card's [status line](session-card.md#status-line) and remains
meaningful without colour.

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
`work continues` in existing detail space. the phone session sheet adds
`inferred from terminal` only for a recognized local agent with source terminal;
unavailable/non-agent labels make no inference claim.
rows of a host whose read failed or that is being re-read are never ready and
stay out of the filter. a failed host's rows keep their stale cached facts as
muted `last observed: <label>`, and keep that form while the host is re-read;
on the desktop that form is wider than any fresh label and the other columns
give way, an accepted exception to the fit rule above
([desktop browser §5](desktop-browser.md#5-presentation)). the rows of any other
re-read host take each client's form, and the clients need not agree:
the desktop table cell reads faint `checking` and makes no status claim, while
the selected row's facts read `checking; last observed: <label>`
([desktop browser §5](desktop-browser.md#5-presentation)); android shows
`last observed: <label>` under the machine's `REFRESHING · actions disabled`.
no repeated announcement, pulse or sound for unchanged polls. retain secondary
`notifications unavailable` independently of live status.

one go projection, `sessions.PresentStatus`, renders this copy as if fresh and
not ready: the gateway sends it as each inventory row's `status` member, and
the desktop and cli call it on the row they hold. android's
`SessionStatusContent` (card and terminal session sheet) reads `status` and adds
only staleness, ready and notification health. labels, tone and
predicates must derive from the same facts; never compare rendered strings to choose behavior. preserve status
before cwd at narrow sizes. no badge stack, new palette or approve/answer action.

the accepted [attention contract](reply-notifications.md#behavior) owns when
non-idle-to-idle observation creates ready, which gaps preserve memory, and what
dismisses attention. its implemented 2026-10-01 revision supersedes this
section's former consecutive-sample and closing-baseline rules.
this table still owns presentation priority: a current interruption/error notice
can hide a saved ready; a visit never dismisses a current request. available again
does not assert that a question was answered. no request ids or durable request
records are introduced.

`needs input` is the exclusive human attention view specified by
[session views](session-views.md#membership-and-ordering). it includes qualified
ready, explicit requests and current error/interruption notices, ordered ready
first. all/group ordering stays stable. cli `wait --state needs-input` retains
the narrower explicit-response-request predicate above. selection, restoration
and creation transitions belong to the session-views contract.

diagnostic heading: `status evidence`. show activity, interaction and notice
separately, then reason and matched rules. exact reason copy:

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

## 7. ownership and review boundaries

the [codebase map](codebase-map.md) locates each owner's code. a change names its
writer, content designer (§6 copy) and adversarial reviewer before work starts;
the reviewer never approves their own implementation. freeze the domain and
capture interfaces (§2, §3) while dependent work runs; a producer/consumer
interface change returns to the contract owner before either side changes its
assumptions. each writer creates temporary acceptance probes only inside its own
paths, a separate verifier writes nothing, and no writer edits another owner's
paths to make its own work pass. no cross-repository deployment producer changes
without inspecting and naming that assignment.

reuse target guards, kernel observations, sgr parsing, fleet routing, strict
decoders, shared status projection, existing poller and notification owner.
delete obsolete eight-line/global-truncation logic, terminal blocked mappings,
old label/sort/wait branches and duplicated capture paths. no compatibility flags,
legacy classifier, shadow production detector or redundant abstraction.

a change to this contract reconciles the docs that restate it: architecture
§4/§8, roadmap, terminal-control observation/send, reply-notifications
qualification, agent-control/ux, desktop-browser, the
dashboard-return-continuity/groups capsule contract and codebase-map. append new
qualification; do not relabel historical passes or `NOT_RUN` results.

## 8. red / green / refactor acceptance

this matrix is the acceptance contract for qualification and for every
requalification after a provider or tmux upgrade
([codex §7](terminal-observation-codex.md#7-requalification),
[claude §6](terminal-observation-claude.md#6-requalification)); each run
updates the grammar's §1 `qualified` column. isolated tmux, integration and live runs need current-turn
explicit approval; physical android/adb needs its own under `AGENTS.md`. use only
the exact sessions on the isolated socket the probes created.

for every change: designer specifies expected observations/copy; reviewer attacks
the oracle and negative cases; writer demonstrates the intended red on the
recorded baseline, implements green, then refactors. reviewer challenges the
result at the real affected boundary. rerun affected checks after refactoring;
delete all temporary tests, screens and harnesses before commit. finish with
existing engineering checks. do not recreate a retired gate or production seam.

| acceptance group | required evidence |
| --- | --- |
| provider behavior | actual stock codex/claude for applicable required §4 families; default/custom display, reduced motion, wrapped/narrow screen, permission modes, active/background/retry, single/multi/free-text/async questions and elicitation |
| capture | actual isolated tmux normal/alternate screen; >8 kib styled upper output with intact lower controls; top setup modal, tall pane, sgr/utf-8 clipping, dimension change |
| negatives | composer during active codex work; quoted old chrome; menu versus request; title-generation spinner, static claude title and inherited busy/action-required/progress across provider→shell→same provider (except §9's accepted false claims); historical errors under current work |
| recognition | managed, bare, absolute, relative and symlink launches on darwin/linux; unrelated executables, exit/replacement and foreground suspend/resume |
| full product | real gateway→cli/desktop and gateway→physical phone: work, each request label, concurrent work, stale/unavailable, filter membership/order, visits, return restoration, accessibility/large text |
| attention | use the separately accepted [attention acceptance](reply-notifications.md#acceptance); prior attention results qualify the former policy only |
| controls | empty ordinary composer send succeeds; draft/dialog/menu/unknown/clipped/changed target refuses without bytes; explicit text/keys and stop/close unchanged |
| diagnostics/cost | reason and same-sample explanation agree; logs/envelopes contain no content; 1 and 16 represented sessions on each host platform fit the existing two-second enrichment budget with no induced timeouts |

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
each grammar freezes, before its parser changes, a provider/version/configuration
capability table with required positive families, permitted unknown/none cases
and upstream-unavailable variants. notices
without a distinguishable current structure and obscured state permit unknown/none;
permissions/questions that the supported provider actually renders require positive
recognition. unavailable upstream features are not implemented claims or passes;
unexercised required cases are `NOT_RUN` blockers. qualify installed versions
first; no untested universal-version promise or runtime exact version gate.
measured supported cases must pass; ambiguity controls must read as their
capability rows state (unknown or none, except where a row admits a false claim
§9 accepts) and may not be waived by an overall accuracy score.

## 9. final state, costs and completion

one observer, two provider grammars, one public fact model and existing client
owners. hard-cut all producers/consumers together; no old/new mixed generation.
coordinate external cli consumers and generated deployment configuration before
shipping. rollback restores the previous complete release; publishing and
installing are separate operations.

explicit costs:

- conservative unknown loses some idle waits and delays ready until idle is
  recognized; the accepted attention revision preserves memory across gaps.
  five-second sampling still misses brief transitions.
- codex ([grammar §6](terminal-observation-codex.md#6-accepted-costs)):
  idle needs the main placeholder on a clean band, no status row, no external
  editor and a visible transcript terminator; goal pursuit withholds idle; status describes
  only the displayed thread, so a v1 sub-agent view reads as the main thread and
  guarded send types into the sub-agent; cue-less turn starts read idle for
  their duration and can raise ready, with no two-sample rule (grammar §6 c4
  names them: `!cmd`, queued slash commands, and the unmeasured daemon-recovery
  and plan-mode goal continuations); idle with surviving background terminals
  stays idle; narrow or short question footers read layout_unknown or a request
  of another subtype, an mcp approval whose option block holds an unparsed row
  reads input, and the mcp prompts drawn in the legacy question view read
  question; only the interruption and error `■` cells are notices; with the
  under-development `features.default_mode_request_user_input` switched on (off
  by default; the managed launch never sets it), a default-mode question left
  untouched resolves itself with empty answers 120 s after it is shown, so an
  unvisited question leaves the needs-input filter and attention while the turn
  continues on those answers, and working sampled in that continuation arms ready
  as usual (stock questions, from plan mode and the mcp prompts, block until
  answered, and a key or paste in the view stops the timer;
  [grammar §2.4](terminal-observation-codex.md#24-legacy-request_user_input-and-mcp-forms));
  idle comes from the screen without a status-line cue, so it can read idle
  briefly while mcp servers start or a restarted daemon resumes a turn; a codex
  started through npm's node launcher is a generic terminal.
- claude ([grammar §5](terminal-observation-claude.md#5-accepted-costs-and-residual-ambiguity)):
  requests and menus hide activity, so working with a request is unrepresentable;
  an interruption row or a failure block (api error, model or usage limit) ending
  the transcript is a notice; idle is lost for
  unproven pill slots, footer links, non-ordinary footers, non-work panel rows,
  usage-limit copy and colour level 0; remotely configured usage-limit copy
  without a default anchor reads idle; plugin render hooks are unsupported, and
  an `AbovePrompt` band is undetectable, so a single-character guarded send can
  reach it; vim NORMAL mode reads as an empty composer, so guarded send is
  admitted there; `← N agents` keeps interaction none; request dialogs without a
  qualified rule read unknown; screen-reader mode is unsupported and reads
  unknown, so those sessions lose status, attention, ready, guarded send and
  `start --stdin` admission.
- tall panes: one capture holds at most 256 rows, so rows `192..h−65` of a taller
  pane are clipped evidence, and idle there needs the transcript tail within the
  bottom 64 rows.
- claude recognition trusts the configured versions directory; moving that
  directory requires configuration repair. wrappers and deleted images remain
  unsupported. [update recognition](claude-update-recognition.md) supersedes the
  former accepted loss of older running images after a launcher relink.
- provider ui upgrades require requalification (each grammar's last section);
  broader capture costs more per sample; the capsule hard cut resets old dashboard
  context; deleting tests removes automatic future behavioral regression
  protection.

the accepted false idles are claude's remote usage-limit copy and three of
codex's: its cue-less turn starts (grammar §6 c4); its millisecond stale-`Ready`
window in a pane at most header + 4 columns wide (c14), which can raise ready;
and an unmanaged status line whose first item is a user value equal to a
run-state word (c12, read as that word's cue). codex's other accepted false
claims are a stale request, under a shell that prints nothing, until a
relaunched codex first draws (about 0.4 s; c13) and a wrong picker kind under a
transcript row that starts with a listed title (c11), each of which can enter
the needs-input filter; and a request read as another request subtype
(c15–c17), which misstates only the subtype (its label, reported interaction
and rule id): every client treats all request subtypes alike for wait, the
needs-input filter, attention, ready and guarded send. the accepted
send-admission exposures are codex's displayed-thread scope and claude's
`AbovePrompt` band and vim NORMAL mode. no other
cost justifies false idle, weaker input admission or silent scope expansion.

complete when the matrix passes at its stated boundaries, independent reviews
find no contract violation, obsolete paths/temporary probes are removed,
engineering checks pass and docs match the final code. close only resolved
issues. remaining blockers are recorded individually; the
[roadmap](roadmap.md#non-native-status-and-needs-input--source-implemented)
indexes them. no engineering pass can close missing live acceptance.
