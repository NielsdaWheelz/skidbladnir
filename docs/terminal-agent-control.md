# terminal status and control

status: source snapshot composed into `codex/reply-notifications`; broader
qualification and deployment remain separate. this owns the hard cutover
of ordinary terminal observation and orchestration. it supersedes conflicting
session-target behavior in [native interaction](native-agent-observation.md),
[agent control](agent-control.md), and [client controls](agent-control-ux.md) at
implementation cutover. retain native integration as a separate capability.

## 1. outcome, scope and decisions

one card represents one tmux session, anchored to its current active pane.
starting, exiting, replacing or resuming an agent requires no tracking ceremony.
ordinary status, inspect/read/send/wait/stop/close use terminal evidence and input.
no native-first mode, runtime feature flag, automatic transport selection, or
fallback after a failed native request.
this applies equally to codex and claude, including registered claude sessions;
only provider-specific detector rules and interruption keys differ.

| operation | contract |
| --- | --- |
| status / inspect | sampled interpretation of the current terminal; never a task result |
| read | bounded rendered terminal text; no native-history substitution |
| send | paste and submit after a fresh heuristic check; acceptance is unconfirmed |
| text / keys | deliberate terminal input, independent of inferred state/provider |
| wait | wait for an inferred terminal state on one captured terminal target |
| `s` / stop | one interruption request; keep the terminal |
| `c` / close | one interruption request, then independently close the exact session |
| `x` / close --terminal-only | close the exact session without interruption |

all terminals support read/text/keys/stop/close, including shells, other programs,
unregistered agents and ssh/mosh. existing machine freshness/authentication and
exact-target admission still apply. a dead pane can refuse input; its session
remains closable. status uncertainty never disables stop or closure.

non-goals: task scheduling, verified task completion, signal escalation, process
tree killing, provider queues, conversation discovery, hooks publishing status,
provider forks, new daemons/stores/transports, notification redesign, human reply
viewers, naming/selector redesign, launch/startup repairs, deployment/publication.
retain current profile flags, account homes, creation and initial-agent startup;
this change removes native dependencies from ordinary observation/control, not
from the existing codex creation path. no new tracked-launch command.

## 2. observation and schemas

reuse foreground process classification and the existing five-second inventory
cadence. replace automatic native status enrichment with one bounded terminal
observation per represented session, outside the session mutation lock; retain
the two-second enrichment budget. no background lifecycle reconstruction.

```text
terminal target = {identityToken, paneId} // machine + tmuxId supplied by routing
terminal status = {state: working|blocked|idle|unknown,
                   source: terminal|unavailable}
session += {terminalStatus: terminal status}
session.conversation? = Conversation // recorded native identity only, no runtime
```

retain existing `activePaneId`, foreground `agent?`, connection and account facts;
do not introduce a second pane/provider identity or per-action capability map.
native `ConversationRuntime`, status and methods remain on explicit native
operations. ordinary inventory makes no helper status/history calls. metadata
does not establish what a terminal displays, change status, or enable controls.
define terminal status at the sessions domain owner; keep native status validation
separate. sessions never imports its higher-level observer.
preserve metadata-only claude identity projection from its validated registration
and codex's recorded identity; neither requires helper calls. inventory/create
never fabricate native methods or turns. explicit native resolution obtains its
runtime through native inspect; captured native references keep their exact turn.

use one pure provider detector in `internal/agentcontrol/detect.go`, returning
internal `{state, composer:empty|draft|unknown}` so send reuses the same parsing.
composer evidence is ephemeral and never enters inventory. port useful
rules from `c164ded:internal/agentcontrol/detect.go`; do not restore its orchestration
or native-fallback composition. inspect at most 8 kib of the current visible
screen and its last eight joined lines, never old scrollback. adapt the existing
bounded tmux capture primitive to select visible screen versus retained tail;
keep utf-8 truncation, byte limits and capture cleanup at that owner.

precedence: recognized dialog/blocker -> positive working indicator -> recognized
prompt/footer idle pattern -> unknown. identify codex/claude from fresh process
facts before applying their rules; quoted output, titles, quietness, elapsed time,
process existence and old screen content alone are not state evidence. no match,
unsupported provider, clipped/unrecognized screen or unstable foreground produces
unknown. successful ambiguity is `source:terminal`; failed capture/observation
is `source:unavailable,state:unknown`. no `done`, `failed` or `stopped` inference.

accepted user decision: idle is an inference. a recognized prompt/footer without
working/blocker evidence may produce it. it does not establish turn completion,
input admission, an empty queue or task success. qualify current provider screens;
document known false-positive cases rather than claiming authoritative state.
no synthetic certainty score, status timestamp, transition ledger or debounce.

sample exact session/pane and foreground before capture; recheck them afterward.
discard evidence on observed change. `/exit` stops applying the former agent's
rules; the next recognized agent uses its own rules. same-process `/resume` needs
no detection or association update: subsequent samples inspect the displayed
screen. ordinary shells/other programs render `terminal`, without an agent-idle
claim. remote context remains display-only: local transport terminals use generic
terminal control and unknown agent state; no gateway-to-gateway routing is added.

## 3. terminal effects and ownership

```text
client -> existing fleet routing -> target gateway
                                 -> sessions -> tmux / kernel
                                 -> agentcontrol terminal detector/policy
explicit native target           -> agentcontrol native adapter -> existing helper
```

`sessions` owns target resolution; `tmux` owns capture, input queues and deletion;
`process` owns kernel facts; `agentruntime` owns provider recognition;
`agentcontrol` owns screen interpretation and interruption-key policy. the gateway
orders interruption and terminal closure. clients neither parse screens nor
select provider-specific keys. preserve existing attachment and mutation owners.

replace the terminal methods' `AgentTarget` prerequisite with a terminal target.
do not require a recognized provider, provider home, registered id or client PID.
resolve a named/handle target once; a captured reference pins its pane. each effect
guards the server epoch/pid/start, session id and captured active-pane membership
in the same synchronous tmux command queue as the capture/input effect. reuse the
existing lifetime predicates and strict pane encoding. a changed pane rejects;
never resolve a replacement pane by name. a rename alone does not change authority.

kernel foreground samples choose detector/key behavior, not terminal authority.
revalidate known foreground facts immediately before provider-specific input;
an observed change refuses that input. there is no atomic transaction between
kernel job control, screen inspection and terminal consumption: bytes written
are only terminal delivery, never guaranteed delivery to a particular program
or conversation. do not add a process supervisor to conceal this boundary.

stop chooses escape for freshly recognized local codex, ctrl-c for local claude,
and ctrl-c for every other/unknown foreground, including ssh/mosh. send once;
never repeat until something stops, escalate signals, purge input or kill a daemon.
at a shell prompt ctrl-c may clear a draft; another program may ignore it. an
unavailable recognition sample uses the same generic ctrl-c contract, not a
second transport. no provider-native call occurs.

send requires a freshly detected local agent in working or idle state. blocked,
unknown, unavailable and non-agent terminals refuse before writing; use text/keys
for deliberate interaction. also refuse a visibly nonempty composer or an
unrecognized composer layout; never append-and-submit a detectable user draft.
this heuristic cannot eliminate concurrent edits. text deliberately retains
ordinary append/paste-and-submit semantics. send/text reuse the same unique-buffer
paste plus one submit primitive. stage that buffer before final foreground
revalidation; both paste and enter belong inside the successful tmux predicate
branch. refusal executes neither; cleanup deletes only this operation's buffer.
an observed foreground change refuses, without generic-key substitution.
no peer attribution, native admission, queue or completion
claim. preparation failures before possible input are `not_sent`; possible input
with lost confirmation is `unknown`; neither is automatically retried.

close validates its entire request and session lifetime first. attempt stop once,
then run validate -> cancel owned attachments -> bounded cleanup -> revalidate/delete.
interruption failure, stale pane, unknown write or attachment-cleanup failure
does not veto deletion of the captured session. retain the existing attachment
owners; delete no unrelated resource. changed session lifetime refuses closure;
no process/pane condition is carried into deletion.
report both effects even if one fails. terminal-only close performs that same
deletion operation directly. closure never proves background/remote work stopped.
client cancellation/gateway shutdown can prevent later phases; do not detach work
from its request owner. lost responses leave outcomes unknown, not retriable.

## 4. api and client commands

replace `/v1/sessions/{id}/agent/*` with closed terminal routes; delete the old
route reader and native/terminal method switches. retain auth, pinned machine,
64-kib envelopes, 10-second operations and 15-second client budgets. inspect uses
the two-second status budget. within close's ten-second budget, interruption gets
at most two seconds; cleanup uses at most six, reserving the final two for exact
deletion. use separate child contexts, never the expired interruption context;
calculate each cap against the absolute operation deadline, including validation
and lock waits. parent cancellation still wins. no retry finishes a partial operation.

| route | request fields beyond terminal target | result |
| --- | --- | --- |
| `POST /v1/sessions/{id}/terminal/inspect` | none | `{terminalStatus}` |
| `POST /v1/sessions/{id}/terminal/read` | `maxBytes?` | `{text,source:"terminal",scope:visible|terminal_history,truncated}` |
| `POST /v1/sessions/{id}/terminal/send` | `text` | write receipt |
| `POST /v1/sessions/{id}/terminal/text` | `text` | write receipt |
| `POST /v1/sessions/{id}/terminal/keys` | `keys` | write receipt |
| `POST /v1/sessions/{id}/terminal/stop` | none | write receipt |
| `POST /v1/sessions/{id}/terminal/close` | none | separate interruption/closure outcomes |
| `DELETE /v1/sessions/{id}` | exactly `{identityToken}`; no pane | existing `204`; client projects `{terminal:"closed"}` |

```text
write receipt = {method:"terminal", outcome:written|unknown}
close result = {interrupt:written|not_sent|unknown,
                terminal:closed|not_closed|unknown}
```

close `not_closed` means positively refused before deletion; `unknown` means
deletion may have occurred. omit neither effect and never turn unknown into
success. malformed input/auth/session validation errors before effects use the
existing error envelope; post-admission close failures use the compound result.
reuse session-not-found/identity errors; introduce `TerminalTargetChanged`,
`TerminalUnavailable`, `TerminalInputBlocked` for the corresponding terminal
boundaries. every error has existing `dispatch:not_sent|unknown` semantics.
unknown/null/duplicate keys, old pid/start/mode/method/conversation bodies reject.

read defaults to 16 kib, maximum 32 kib; alternate screen means visible capture,
otherwise bounded retained terminal tail. text is nonempty valid utf-8, <=32 kib,
without nul. keep the existing 1–16 logical-key vocabulary. content stays out of
logs, saved state and evidence. capture/parse never executes captured text.

ordinary terminal selectors/ref route inspect/read/send/text/keys/wait/stop/close
to these terminal operations. remove `--terminal` from read/stop; retain
`start --terminal` and its existing shell-creation semantics.
`--history`, `--input` and `--queue` reject on terminal targets. native targets
retain their existing flags and unavailable-capability semantics. `text` remains
distinct from guarded send. stdout/json and exit codes distinguish written from
unknown; unknown/refusal/timeouts are nonzero. no native acceptance wording.
close exits zero only for `interrupt:written,terminal:closed`; a partial result
is nonzero even if the terminal closed. terminal-only close exits zero on confirmed
closure. neither zero exit establishes that work stopped.
the captured target kind selects the expected response decoder before dispatch;
reject opposite-kind receipts/status rather than changing interpretation on return.
terminal routes return `200` with declared bodies; deletion retains `204`.

terminal reference = existing opaque machine/tmuxId/identityToken plus `paneId`;
it contains neither captured agent PID nor native conversation. native-only
references retain their existing conversation schema and contain no terminal.
reject old mixed references; do not add another codec or compatibility reader.
[naming](automatic-session-names.md) owns selector grammar/typed handles; its
terminal selectors must not implicitly become conversation selectors. this plan
supersedes its command routing table: terminal handles select info/enter/read/send/
text/keys/wait/stop/close/shell/group; conversation handles and direct native ids
select native read/send/wait/stop only. conversation handles explicitly select
native identity, never whichever terminal happens to contain it. inspection stays
an internal client operation plus the declared api; add no public inspect command.
root integrates overlapping files serially, with one grammar in the final release;
no selector redesign or compatibility alias is implemented in this slice.

wait reuses the current client loop: capture once, sample immediately, then one
request per five seconds, default idle/60 seconds, maximum one hour; every request
is bounded by the monotonic remaining deadline. terminal states are idle/blocked
only. native-only waits retain native state choices. terminal-source unknown
continues without matching; unavailable-source samples return an unavailable error.
active-pane selection change, pane destruction or session replacement ends target_changed;
foreground exit/restart/resume within the same pane remains the same terminal
target. cancellation stops the waiter only. matched means observed inferred state,
not completed work; automation must inspect its task's actual result separately.
terminal wait result is `{outcome:matched|timeout|target_changed,target:<captured-ref>,
terminalStatus?}`; timeout retains the last successful status. only matched exits
zero and requires a matching status. omit status only before any successful sample.
native wait retains its existing result schema; no new wait service.

## 5. retained native capability and adjacent work

retain native adapter methods, helper pin, provider configuration, exact-target
validators, explicit conversation endpoints/cli targeting and creation behavior.
retain creation-time association metadata for its current consumers. manual
track/untrack/forms/routes are being removed independently; never restore them.
ordinary terminal references never resolve through a recorded association.
remove native method gating from terminal cards and shortcuts, not the native
capability implementation. this is the explicit preservation exception requested
by the user; no copied old controller, dormant fallback tree or speculative
reactivation framework. a future terminal/native integration requires supported
current-selection evidence and a new reviewed capability contract.
remove the former session-native compound-close controller with its route; its
provider stop adapter remains. explicit native stop and terminal deletion remain
separate operations, never an automatic composite selected from session metadata.

[reply notifications](reply-notifications.md) owns `r`/`replies`/viewer deletion,
notification stores, colors and visit policy. do not implement those changes here.
the user approved replacing native-reply claims with terminal `ready` attention.
that revised contract follows this exact terminal and foreground evidence after
`/resume`; it never claims native work identity/finality. both implementations
must still qualify their combined lifecycle and content before release. terminal
primitives and notification client slices remain separate owners.

the naming plan owns automatic titles/handles; titles never become status evidence.
its exact-lifetime deletion change is shared with this plan: implement once at
`sessions`/`tmux`, reuse from both, and remove old name authority once. startup
repairs and native history/notification qualification retain their own issues.

## 6. content contract

assign a designer to each client slice and a content reviewer to shared cli/api
errors. good content names the actual terminal/machine, distinguishes observation
from delivery, exposes an available action, and stays legible at narrow widths.
one presentation function per client serves cards, details, terminal menus and
accessibility; never duplicate detector logic in clients.

| feature | required content / behavior |
| --- | --- |
| inferred agent state | `working`, `waiting`, `idle`, `status unknown`; details: `inferred from terminal` |
| observation failure | `status unavailable`; retain terminal actions |
| ordinary shell/program | `terminal`; retain current command/profile context when known |
| stop / combined close / immediate close | `send interrupt` / `interrupt and close terminal` / `close terminal only` |
| stop receipt | `interrupt key sent; stopping is unconfirmed.` |
| guarded send refused | `send unavailable for this screen. open the terminal or use text/keys.`; dialog: `respond to the dialog in the terminal.`; draft: `terminal contains a draft. open it before sending.` |
| unknown write | `could not confirm terminal input. inspect the terminal before trying again.` |
| partial close | `terminal closed; interruption unconfirmed.` |
| target change | `the terminal changed. refresh before trying again.` |
| inferred wait match | `observed idle (inferred).` or `observed waiting (inferred).` |

stop is a deliberate single action; close variants retain the existing confirmation
with exact captured name and terminal-host machine, not an ssh destination.
combined copy: `send one interrupt to the selected pane, then close this entire
session. closure proceeds even if interruption fails.` both closure variants:
`work shared elsewhere or running remotely may continue.` same-lifetime rename
does not retarget the confirmation. no extra confirmation cascade.
desktop s/c/x appear in agents and terminal/group views; mobile equivalents appear
on cards and terminal screens. availability never depends on agent presence or
configured profiles; retain freshness checks and explicit failure feedback.
remove routine `conversation not tracked`, tracking chips and native-method gates.
secondary details may say `recorded native conversation; may differ from terminal`;
never claim current identity or recommend manual tracking as repair.

presentation precedence: failed observation -> `status unavailable`; successful
non-agent sample -> `terminal`; recognized agent -> inferred state label. stale
retained facts use existing freshness treatment and `last observed: <label>`.
accessibility includes `inferred from terminal` and announces meaningful changes
only. the agents filter follows current local/remote process context, never a
recorded conversation. an exited agent remains in all-terminals/group views;
if its row leaves the filter, use existing nearest-row selection, never retarget
an open confirmation. preserve relative attention order blocked/idle/unknown/working.
retain typography, dimensions, palette, motion and group sorting; the notification
owner governs its colors. labels survive no-color display; no success claim.

## 7. disjoint implementation and review

| slice | exclusive paths | deliverable |
| --- | --- | --- |
| terminal owner | `internal/sessions/`, `internal/tmux/`, `internal/process/` only if required | generic targets, guarded capture/input, exact independent closure |
| observation/control owner | `internal/agentcontrol/`, `internal/agentruntime/` | pure detector, terminal policy/results; retain native implementation |
| gateway owner | `internal/gateway/`, `internal/logging/` | strict replacement routes/dtos, separate close outcomes, content-free errors |
| desktop + designer | `internal/fleetclient/`, `internal/agentcli/`, `internal/sessionui/` | target dispatch, references/wait, controls and shared content |
| android + designer | `android/app/src/main/java/dev/niels/skidbladnir/` | schemas, fresh-target controls and shared content |
| root integrator | `docs/`, `cmd/skidbladnir/` only if composition needs it; coordinated release metadata | contract/adjacent-pr integration, verification and hard cutover |
| adversarial verifier | read-only; no production/test files | challenge contracts, red-test sensitivity, green behavior and refactor |

primary existing files: `sessions/{control,types,manager,agent}.go`,
`tmux/{control,client}.go`, `agentcontrol/{service,actions,native,create}.go`,
`gateway/{agent_control,conversations,dto,gateway}.go`,
`fleetclient/{request,response,client,wait}.go`, `agentcli/run.go`,
`sessionui/{session,view,details,navigation}.go`; android `AgentControl.kt`, `GatewayClient.kt`,
`ProductModel.kt`, `SkidbladnirController.kt`, `SessionCard.kt`, `TerminalScreen.kt`.
split terminal policy from native policy within the existing owner if needed;
no generic provider framework. consolidate one capture/input validator, one
interruption-key selector and one closure operation. remove old session-native
routing, obsolete dto/mode branches and callers after the new path replaces them.

root freezes schemas and serializes shared-file work with naming/notifications.
terminal + observation owners implement against reviewed interfaces; gateway
follows, then desktop/android proceed in parallel. each slice implements its own
temporary integration/live tests. designers review content before implementation
and at rendered acceptance; verifier writes no tests. root updates architecture,
agent-control/ux, native spec, desktop-browser, codebase map, roadmap and affected
issue resolutions at cutover. do not mark issues resolved from this plan alone.

## 8. temporary red/green/refactor acceptance

follow [testing policy](rules/testing.md). write focused end-to-end tests through
real clients/gateway/isolated tmux and actual supported provider tuis. demonstrate
the intended failures on the original implementation, implement green, challenge
test sensitivity, refactor at the responsible layer, rerun affected cases, then
delete temporary tests/fixtures before commit. synthetic screen cases supplement
actual terminal checks; they do not replace them. use disposable provider homes;
no existing user-home mutation, production test seams or recreated test framework.
preserved native/creation behavior needs green retention checks, not fabricated
red failures. reproduce changed behavior against the exact recorded base revision.

| boundary | proof required |
| --- | --- |
| observation | real codex/claude working/dialog/prompt/unknown; narrow/wrapped/clipped screens, quoted lookalikes, old scrollback and capture failure; sampled source is truthful |
| lifecycle | a -> resume b -> exit -> shell -> other program -> new agent: terminal remains usable without association; old native id/status cannot affect card or controls |
| generic controls | s/c/x through desktop and mobile on shell, long-running foreground command, both providers, unknown program, ssh/mosh and dead pane; no native dependency or hidden escalation |
| exact effects | pane switch/replacement, session/server reuse and rename; no other session/socket affected; stale interruption can coexist with exact-session closure; unrelated/background work not claimed stopped |
| transport/results | real unique-buffer paste and keys; guard loss after staging submits nothing; cleanup error/timeout cannot consume deletion budget; pre-write failure vs possible write; lost responses/cancellation preserve separate outcomes; no replay |
| orchestration | send refuses dialogs/unknown and visible drafts; active-work send qualifies independently; text remains available; bounded read/utf-8/alternate screen; wait match/timeout/cancel/foreground changes/pane selection changes |
| separation/cutover | helper-unavailable terminal operations; retained native read/peer receipt/exact-turn stop/results and creation; wrong-kind response rejection; old mixed refs/routes/read-stop flags reject; start --terminal preserved |
| content | real desktop and phone narrow-layout actions, confirmations, accessibility, inferred-source disclosure and unavailable feedback; r remains the other pr's responsibility |

at each stage the independent reviewer attempts a concrete counterexample and
blocks the next stage on unresolved correctness findings. after required cases
pass, broaden only for a remaining concrete concern. remove temporary tests and
run `scripts/check verify`; engineering checks are not behavioral acceptance.
ship one coordinated gateway/cli/apk generation; no old reader, compatibility
mode, native fallback or smaller-fleet branch. rollback is the previous complete
release. native integration retained above is deliberate capability, not legacy
terminal routing. no release or installation is authorized by this plan.

tmux/live/phone/adb work requires explicit current-turn approval. use only exact
owned sessions on isolated `-L` sockets; never kill/resize/retarget user sessions.
missing providers/devices/live boundaries are `NOT_RUN`, never pass. evidence
contains case names, versions, outcomes and timing, never terminal text, prompts,
objectives, tokens or account data. record unresolved issues under `docs/issues/`.

accepted costs: inferred status can be wrong and sampling misses transitions;
idle waits do not prove completion; terminal input may be ignored or interpreted
differently; foreground changes cannot be atomic with key consumption; remote
controls use generic terminal behavior; closure may leave detached/shared work;
request cancellation can leave close incomplete; creation retains its native
dependency; coordinated schemas invalidate old refs;
retained native code needs upkeep; deleting tests forfeits automatic regression
protection. none warrants a new runtime, history store or compatibility layer.
