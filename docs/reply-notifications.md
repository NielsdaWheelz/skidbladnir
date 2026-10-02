# terminal attention notifications

status: implemented and behaviorally qualified 2026-10-01 in
`feature/ready-transition-memory`, from `2e2e9a1`. this specification replaces
consecutive working-to-idle detection with remembered non-idle-to-idle attention.
[current qualification](#qualification) records exact source and boundaries;
[historical qualification](#historical-qualification) describes former policies.
deployment remains separate.

[terminal observation](terminal-observation.md) owns current status facts and
presentation priority; [terminal control](terminal-agent-control.md) owns exact
targeting. this document owns notification memory, visits, storage, and the
implementation plan. codex and claude follow the same rule on both clients.

## behavior

`ready` means this device observed the same foreground agent non-idle and later
idle, and has not dismissed that attention. it means available again; it proves
neither successful work, a new answer, an answered request nor an empty queue.
unknown and unavailable observations do not erase a known earlier observation.

classify each fresh, successful terminal-source observation of a local agent
from its typed facts, in this order:

| observation class | predicate |
| --- | --- |
| non-idle | interaction is permission, question, confirmation, setup, input or menu; or activity is starting or working |
| idle | activity is idle and interaction is none |
| gap | everything else, including idle with unknown interaction |

an unknown activity with a recognized question is non-idle; working with unknown
interaction is non-idle. neither positive fact is erased by uncertainty in the
other dimension. notice is independent: an error or interruption does not arm or
clear attention by itself. its display priority remains with observation §6.
stale inventory, failed polls and unavailable status are gaps; they supply no
transition. confirmed identity changes are handled before classification below.

one persisted attention state belongs to each exact terminal and foreground:

- `quiet`: no transition awaiting notification and no pending ready.
- `armed`: non-idle was observed; a subsequent idle should raise ready.
- `ready`: that transition was observed and remains unacknowledged.

the initial state is quiet. the complete transition rule is:

| event | quiet | armed | ready |
| --- | --- | --- | --- |
| non-idle observation | armed | armed | armed |
| idle observation outside a visit | quiet | ready | ready |
| idle observation during this client's presented visit | quiet | quiet | quiet |
| gap, backgrounding or restart | quiet | armed | ready |
| first actual output presentation on entry | quiet | armed | quiet |
| end of a presented visit | quiet | armed | ready |
| confirmed identity reset | quiet | quiet | quiet |

non-idle dismisses an old ready and arms the next transition. actual presentation
dismisses existing ready, not unfinished activity. an idle observed during a
visit is acknowledged there. ending a visit is only an ordering boundary; it
never consumes a later ready recorded by another desktop client.

first-ever idle stays quiet; first non-idle arms immediately. there is no minimum
duration, expiry, debounce, consecutive-sample requirement or post-visit quiet
sample. startup, a menu closing and a request being declined or auto-resolved can
all lead to ready. these are intended consequences of available-again semantics.

## identity and lifetime

```text
terminalKey = {machine, tmuxId, identityToken, paneId}
foreground = {provider, pid, startIdentity}
```

keep the existing exact identity boundaries. a new process, reused pid with a
different start identity, changed pane selection or changed terminal lifetime
cannot inherit attention. reset the former identity, then classify the new
agent's same admitted sample: non-idle arms; idle stays quiet. positive local
shell/non-agent or remote-transport observation clears the former local agent.
a failed observation or missing identity during a failure proves no exit; retain
stored identity and state, and require an exact match before presenting ready.

names, titles, group, profile and native conversation do not identify attention.
rename preserves it. same-process conversation navigation keeps the same identity;
its positively observed menu participates in the transition rule. remote agents
are not resolved into local attention targets.

successful authoritative inventory retires absent session lifetimes. failed hosts
retain records. a positively deselected pane retains a quiet record with an
advanced revision and no foreground, so a late sample cannot recreate its former
attention. remove records for unconfigured machines. identity/lifetime cleanup
uses the existing inventory scope; a single-session read cannot retire others.

## owner, schemas and interfaces

```text
existing inventory/status -> serialized device-local notification owner
existing presentation/attachment lifecycle -> same owner
committed attention + fresh status + local visit -> current presentation

record = {key: terminalKey, revision: nonnegative int64,
          foreground?: foreground, state: quiet | armed | ready}
store = {schema: 2, terminals: record[]}
expected record = absent | present(revision)

read() -> snapshot
observe(samples, expected records, local visit context) -> committed snapshot
observeSession(sample, expected records, local visit context) -> committed snapshot
presented(key) -> committed snapshot
endVisit(key) -> committed snapshot
```

these are internal semantic operations, not a new wire api. use one owner per
client: desktop `internal/fleetclient/notifications.go`, android
`NotificationStore.kt`. remove the working-predecessor maps and their plumbing,
`pending`, `baselinePending`, and the arming/clearing split. the state enum is the
only notification memory; do not also persist the last status or a second flag.
armed and ready require a foreground identity; quiet may omit it. validate unique
keys, closed fields/enums, identities, and revision range at store ingress.

persist state through app backgrounding, process recreation, host outages and
client restart. this enables a remembered non-idle before a gap to notify on the
first fresh idle after recovery. it does not reconstruct unobserved activity.

retain desktop stable-sidecar flock and read/merge/atomic replace, android
serialized datastore updates, and one commit per inventory batch on the existing
cadence. every admitted observation advances its record revision, including gaps
and unchanged state. revision is ordering metadata, not an activity timestamp.
never wrap it; an absent record differs from revision zero.

read the store immediately before network dispatch and capture expected revisions.
compare under store serialization. mismatches discard that sample's notification
processing without clearing existing durable state, rebasing or replay. selection
samples compare the selected pane and every current or expected sibling pane
before changing any of them; a missing sibling record is also a mismatch. retain
caller generation/attempt and read-sequence admission; rejected callbacks cannot
publish old snapshots over newer acknowledgement. no lock spans network work.
a scoped read says nothing about another host's records.

successful browser/phone creation responses are scoped observations too. capture
their expected revisions before dispatch, admit the existing action callback,
then commit the returned terminal sample before auto-entry and its first output
acknowledgement. a creation response cannot retire unrelated sessions. notification
storage failure stays secondary; it does not prevent the created terminal from
opening. ordinary cli create/list/info/wait do not acquire notification memory.
standalone `enter` and phone reattach admit their fresh target preflight through
the same scoped owner, with expected revisions captured before dispatch. resolver
inventories do not turn unrelated rows into admitted notification observations.
explicit phone reattach pins the same session lifetime and captures its returned
selected pane for the new visit; observation and output acknowledgement use that
same key. the post-visit standalone read never substitutes a changed pane.

both presented and end-visit operations advance the exact record's revision even
when its state is unchanged. first presentation rejects pre-entry observations;
end-visit rejects in-flight visiting observations. capture visit context with the
request and validate the current lifecycle before acceptance. android's per-machine
inventory lane ends only after commit and publication or handled failure. publish
only committed notification state; current freshness and foreground independently
gate its visibility.

phone notification/pressure publication never refreshes terminal facts from an
unchanged cached inventory snapshot. admitted scoped entry samples invalidate the
machine's cached-ready qualification; its normal fresh inventory restores that
qualification. even a scoped idle can commit ready while green waits for that
full-inventory qualification if no output has acknowledged it. the existing
read-sequence gate owns this boundary; no extra poll is introduced.

## cache cutover and failure

use `notifications-v2.json` with schema 2 in each client's existing notification
storage directory, and the corresponding new desktop sidecar lock. start empty
when that file is absent. do not read, migrate or delete `notifications.json` or
`unread.json`; old files remain inert to the new release. the cutover deliberately
drops old ready notices once. subsequent v2 restarts preserve quiet/armed/ready.
old and new binaries cannot overwrite each other's cache formats.

malformed or unsupported v2 data is a visible notification failure, never a reset
or fallback. storage failure does not disconnect a terminal or turn cached status
into fresh evidence. after an uncertain commit, reread through the existing owner
before another merge; a failed write cannot promise durable acknowledgement.
keep the existing secondary `notifications unavailable` treatment.

rollback restores the previous whole release and its separate old cache; no
notification continuity across downgrade is promised. deployment and publication
remain separate from this implementation. there is no host schema, provider
configuration or native-helper change.

## one terminal visit

reuse current attempt/page generation and foreground lifecycle. no persistent
visit registry, cross-client focus synchronization or extra poller is introduced.
one desktop wrapper serves browser entry, creation auto-entry and `skid enter`.

1. capture the exact terminal and invalidate older callbacks. merely requesting
   entry does not clear or disarm notification memory.
2. activate after first actual output presentation: a fully written nonempty
   desktop payload, or android's current-page/current-attempt output-applied
   event. hello or attachment success alone is insufficient. commit presented,
   immediately publish, and suppress ready for that local visit.
3. android's existing observations during the visit use the transition table:
   non-idle arms, idle consumes to quiet, gaps preserve. requests remain visible.
   desktop polling stays paused during its blocking attachment.
4. on detach, connection loss or backgrounding, end the presented visit and
   invalidate its callbacks. commit the revision fence without changing state.
   schedule the post-visit read only after that commit; do not delay dashboard
   navigation on host access. backgrounded clients wait for normal resume.
   orderly shutdown finishes the serialized exit write before cancelling its owner.
5. standalone enter retains its single bounded existing info read after a
   presented visit. require the captured session lifetime and pane to match and
   apply the ordinary transition table. failure preserves state; no closing flag
   or retry loop remains.

failed entry before presentation does not acknowledge armed or ready. independently
admitted status observations still follow the transition table. opening while
armed does not consume the next idle: `armed -> open -> detach -> idle` is ready,
unless an admitted idle during the visit already consumed it. first presentation
is the acknowledgement event; successful entry is not a promise about what the
human read or whether a task is complete.

accepted sampling limit: desktop cannot distinguish completion during its paused
visit from completion just after departure. if it was armed and the first fresh
post-visit observation is idle, it raises ready in both cases. conversely, work
that starts and ends entirely during that unobserved visit cannot arm it. android
can distinguish only transitions its existing polling actually observes. no
terminal text is parsed to manufacture the missing history.

another desktop process may observe while this one is visiting. its committed
state is shared through the device-local store; active focus is not shared.
end-visit preserves that state. another device remains entirely independent.
direct tmux attachment outside skid emits no local acknowledgement event.

## content and removal

retain one status projection per client for cards/table, detail, terminal header
and accessibility. show green `ready` only with committed state ready, an exact
matching local foreground, fresh idle + interaction none, and no local visit.
requests, menus and current error/interruption notices keep observation §6's
priority. a notice can hide ready without consuming it. unknown, unavailable,
stale and checking presentation hide green and preserve attention memory.

non-idle always shows its current status and sets state armed. first idle with
quiet state remains muted idle. retain the inferred-from-terminal disclosure,
exclusive labels, existing colors, needs-input predicate, sort order and secondary
notification-storage failure treatment. unchanged polls produce no new sound,
pulse, announcement or count. labels remain meaningful without color.

ordinary cli list/info/wait continue reporting current terminal facts; they do
not start writing device-local attention. `skid enter` continues using the shared
desktop visit owner. explicit native reads/controls never supply notifications.
no classifier smoothing, provider hooks, title inference, native history, timers,
new background execution, host notification api or alternate detector is added.

## disjoint delivery and verification

the following plan is completed. [current qualification](#qualification) records
the behavioral evidence; historical results qualify only their recorded sources.

| step | owner and paths | work and completion condition |
| --- | --- | --- |
| 1. contract review | root; this document and linked summaries | review the transition table, partial observations, visit races, cache reset and sampling limits; keep one policy owner |
| 2. desktop | `internal/fleetclient/notifications.go`, `internal/sessionui/notifications.go`, `internal/sessionui/session.go`, `internal/terminalclient/terminal.go`, `internal/agentcli/run.go`; projection callers only if needed | implement v2 enum/store and ordinary/visiting reduction; remove predecessor maps; update presentation/end-visit and standalone observation signatures; preserve revisions, scoped reads and existing locking |
| 3. android | `android/app/src/main/java/dev/niels/skidbladnir/NotificationStore.kt`, `SkidbladnirController.kt`; `ProductModel.kt` only if projection changes are necessary | implement the same v2 contract; remove predecessor plumbing and lifecycle erasure; retain datastore serialization, inventory lane, visit/generation gates and immediate committed publication |
| 4. review and acceptance | writers own temporary probes in their assigned paths; root coordinates acceptance; independent reviewer is read-only | exercise actual owners and boundaries below; review races and both implementations against the same table; repair only demonstrated gaps |
| 5. finish | root; affected docs and issue records | remove temporary probes, run required engineering checks, record exact source and boundary evidence, close only resolved issues; release/install are separate work |

root serializes integration. writers use disjoint assigned paths; a verifier
writes neither production nor tests. do not build a cross-language state-machine
framework, contract generator, retained test harness or production test seam.
[testing policy](rules/testing.md) governs temporary behavioral tests and their
removal before commit. demonstrate representative old-source failures before
implementing; then verify the changed behavior at its actual owner/boundary.
run `scripts/check verify` for engineering checks after implementation. classifier
fixtures remain useful independent evidence but do not test notification memory.

### acceptance

run the same transition cases through both real notification owners. arrows may
include repeated observations; identity remains fixed unless the row says otherwise.

| case | required result |
| --- | --- |
| first idle; unknown -> idle with no positive history | quiet; plain idle |
| working -> idle | ready |
| working -> one/many unknown, unavailable or failed polls -> idle | ready |
| idle -> unknown -> idle | quiet |
| ready -> unknown/outage -> idle | ready returns; no green during the gap |
| starting -> idle; each request kind -> idle; menu -> idle | ready |
| unknown activity + question -> idle; working + unknown interaction -> idle | ready |
| idle + unknown interaction between non-idle and idle | gap preserves armed; later qualified idle is ready |
| ready -> working/request/menu/starting | armed and current non-idle label; later idle is a new ready |
| working -> idle with error/interruption notice | state ready; notice label takes precedence |
| ready -> open -> idle | quiet; no re-notification from repeated idle |
| ready or armed -> entry fails before presentation | no acknowledgement; any independently admitted status follows the ordinary table |
| armed -> open -> detach -> idle | ready |
| android armed -> open -> admitted idle during visit -> detach -> idle | quiet |
| unknown during visit; end of visit without fresh idle | armed preserved; no blind acknowledgement |
| armed or ready -> background/process restart -> same agent idle | ready; first-ever idle still quiet |
| replacement/exit, reused pid, pane switch, terminal recreation | former attention cannot transfer; new non-idle arms and new idle is quiet |
| rename, group change, another host's failure or scoped read | matching state retained |
| creation returns non-idle -> auto-entry presents -> detach -> first fresh idle | ready; unrelated sessions retained; rejected creation callbacks cannot arm |
| standalone enter preflight returns non-idle -> presents -> detach -> idle | ready, even with no prior inventory history |
| phone reattach preflight returns non-idle -> presents -> detach -> idle | ready; unrelated records retained; obsolete attempt cannot arm |
| delayed pre-entry/pre-exit inventory and obsolete page/generation callback | cannot mutate or republish state past the visit fence |
| two desktop clients commit competing samples or one acknowledges | stale merge discarded; winner retained; next fresh sample resumes from durable state |
| another desktop client creates ready during a visit | end-visit preserves it; local focus is not synchronized |
| v1 cache present; v2 absent; subsequent v2 restart | deliberate one-time quiet baseline, then durable v2 state; v1 untouched |
| malformed v2 or read/write/uncertain-commit failure | visible notification failure; terminal usable; no reset, fallback or false committed result |

verify inventory -> store -> presentation composition on both clients, desktop
browser and standalone entry, and android background/resume/process recreation
and real output acknowledgement. replay the default-footer codex work -> streaming
unknown -> settled idle sequence through attention. use both providers for the
real terminal journey and record the actual host/device boundary. verify a visible
ready, uncertainty hiding/restoration, request/notice priority and a consumed ready
without losing the inferred accessibility label or no-color meaning.

live tmux/provider and physical-phone operations require explicit current-turn
approval under `AGENTS.md`; this specification does not authorize them. tmux
probes use only exact test-created sessions on isolated `-L` sockets. unavailable
boundaries remain `NOT_RUN`. evidence is content-free and credential-free.
the ready-memory and redundant closing-field issues are resolved. affected
phone/linux/request records retain their separate gaps and the new evidence below.
keep unresolved gaps as individual issue records; a documentation edit cannot
close them.

## accepted costs

- terminal inference can be wrong. no grace period compensates for false idle;
  notices and requests remain observations, not authoritative outcomes.
- all activity between observations may be missed. an armed client can notify
  after a long outage but cannot count or reconstruct intervening turns.
- menus, startup, declined requests and auto-resolved questions can produce ready
  without new work or new text. this is the approved meaning of available again.
- desktop visit timing and unsynchronized client focus have the limits described
  above. a rejected concurrent sample may delay or miss a brief transition.
  android current facts can appear before their notification merge commits;
  immediate entry can reject that pending sample. a visible working label alone
  therefore does not promise durable armed state.
- a scoped phone entry sample can briefly hide that machine's cached ready until
  normal inventory refresh; it preserves durable attention and prevents an older
  idle from overriding the newer sample.
- the v2 cutover resets old attention once. failed writes cannot promise
  persistence, and a crash before acknowledgement commit can leave ready pending.
- temporary behavioral probes are removed under current policy; this change adds
  no retained notification regression suite.

## qualification

2026-10-01 runtime source: `a856a6d1d60ab0aa59f61f56c49fbab76871db6e`,
from `2e2e9a113e515f1cda6186a673b2e1767d9bbc9f`. the six changed runtime
files are identical to the final tested worktree. writers used disjoint desktop
and android paths; independent adversarial review covered the contract, original
failures, each repair, refactoring, test sensitivity and final acceptance.

original-source integration probes demonstrated the failure before implementation:
desktop 22 cases, 16 failed and 6 passed; android 5 owner tests, 4 failed and 1
passed. later counterexamples also went red before repair: competing/missing pane
fences, creation and standalone preflight omissions, android acknowledgement
publication with a blocked store, cached ready after a failed write, phone reattach
sample loss, cached old-pane replacement, and same-pane unknown overwritten by
notification publication or unpresented departure.

| boundary | final result and scope |
| --- | --- |
| darwin 25.4.0, go 1.26.0, tmux 3.7c | PASS: 87 leaf cases: 61 actual disk-owner/cache/identity/concurrency/process cases, 6 real websocket output-write cases, 9 browser model/admission/projection cases, 9 actual isolated tmux/gateway/tty/browser/standalone/ordinary-cli cases, and 2 stock-provider journeys |
| linux/arm64, go 1.26.0, unprivileged local container | PASS: 76 owner/stream/browser-model cases; installed linux providers, tmux and fleet gateway journeys remain outside this run |
| android owner, robolectric with actual datastore and serializer | PASS: 11 tests, including both providers' complete transition/identity matrix, gaps and unchanged revisions, absent-versus-zero expectations, scoped preservation, pane races, independent datastore reopen, strict v2 decoding and projection |
| physical sm-s906w, api 36, isolated `.readyprobe` app | PASS: 11 phases: full journey, creation, shell creation, reattach, changed-pane reattach, same-pane gap/held departure, write failure, armed process, armed restoration, ready restoration and corruption |

root reran the final desktop suite; its subprocess-helper test intentionally
skips in the parent process and executes in the actual child. independent
processes verified flock/revision competition, durable armed restoration,
acknowledgement protection and recovery after a committed write whose result was
lost. the real websocket cases distinguish hello/empty output, failed or partial
writes, and exactly one acknowledgement after a completely written nonempty
payload. actual cli list/info/wait left attention untouched; ordinary cli creation's
no-write branch was source-reviewed.

stock codex `0.159.2` and claude `2.1.287` ran against disposable local scripted
model endpoints with dummy homes and credentials. their actual native processes,
tmux screens, gateway, desktop owner/projection and tty output proved work →
settled idle → ready, then output acknowledgement and quiet post-visit idle.
codex additionally proved streaming unknown with its stock default footer;
claude's streaming stage did not assert unknown. request and notice matrices
use typed/authored fixtures; these client results make no claim about cloud
responses or every provider dialog family.

phone phases exercised actual activity/controller, keystore, datastore, TLS/WSS,
Compose and WebView output-applied acknowledgement with scripted phone-local
peers. creation/shell responses supplied the unique working observation;
subsequent inventory was unknown, real output preserved armed, then departure
and fresh idle raised ready. reattach used its unique preflight sample; changed
pane observation and presentation shared one key. controlled held responses and
store writes proved lane completion and obsolete-callback rejection. unknown
reattach plus another host's publication and held post-departure read kept green
hidden; fresh idle restored the preserved ready. rendered accessibility nodes
retained ready's inferred disclosure, and request/error/interruption priority
passed. the armed-visit controls waited for committed armed rather than only a
visible working label.

physical process phases used distinct pids `13582 -> 13817 -> 13943`, with an
explicit force-stop and no data clearing between them: armed restored as ready,
and ready survived another process restart. orderly controller close committed
the exit fence. malformed bytes stayed unchanged with usable terminal transport;
an actual private-directory write failure stayed secondary, hid cached green,
and recovered preserved attention after normal storage access resumed. inert v1
and unread files stayed byte-identical.

the tested app apk sha256 is
`e4e00fec5d4b851ae039ca7c33e44519d493fde1f18a79e1a4282456062282ff`;
its test apk is
`9126dc9a2cfaeff06179f50b4c8d6f549a2a1b1a8ee95f545f03130a5c964545`.
installed bytes independently matched both digests. the tested android source
sha256 values are `NotificationStore.kt`:
`6de94015d8b56e033f268e850efe2ebc93929c686db1462d723c8557ac3d4456`, and
`SkidbladnirController.kt`:
`a03c1a4fea1fab52972a5bf1cd73830cc5288797a67d8acccf33174d1a2bc685`.
temporary app-id/trust/dependency configuration and all probe sources were removed
before commit; build and debug manifest were restored exactly. both isolated
packages were uninstalled. task-owned tmux sessions, provider helpers, peers,
containers and stale sockets were cleaned; production apps, pairings, provider
accounts and terminals were untouched.

`scripts/check verify` passed on the final runtime source after probe removal:
formatting, syntax, lint, dependency/catalogue/assets, go vet/build and android
lint/debug assembly. it supplies engineering evidence, separately from the
behavioral results above. documentation links and whitespace were checked after
the final evidence update.

limits: filesystem syscall failure specifically after atomic rename was
source-reviewed, not injected; actual committed-write/lost-result recovery was
exercised. installed-fleet deployment, stock-provider/cloud phone journeys,
spoken talkback and the full naming/control composition are not supplied by
these isolated runs. their existing issue records remain open. no retained
notification regression suite or production test seam was added.

## historical qualification

these results qualify the policies implemented at their recorded sources. their
old unknown/request/closing-baseline outcomes are intentionally superseded above;
they are retained as evidence, not current acceptance criteria.

2026-09-30 qualification: temporary original-source and counterexample REDs
became GREEN after adversarial review. desktop exercised actual gateway/tmux/tty
entry and browser projection; both providers' native machine-read route passed
gateway/helper/decoder checks using protocol fixtures. stock codex 0.159.2 and
claude 2.1.284 used disposable configuration and a local model backend. codex
fullscreen/inline layouts passed. android's isolated test app exercised the
actual controller, datastore, TLS/WSS peer boundary and xterm
presentation on the connected phone: full journey 58, fresh-process restart 2,
corruption 4, held-response continuity 3, rendered labels/colors/bounds 11;
serializer/datastore/projection integration passed 60 assertions. rendering used
refreshed native accessibility nodes and actual pixels, not event-delivery proof.
production accounts, app pairings and existing terminals were untouched. this
qualifies terminal inference and client behavior, not cloud/native completion or
installed-fleet deployment. temporary tests and harnesses are removed by policy.
`scripts/check verify` passed after removal; it supplies engineering checks only.

main integration preserves automatic naming, typed handles, browser strip/Escape,
provider-owned names and launch descriptor ordering. the focused isolated-tmux
host composition and TLS/HTTP desktop selector/browser/attention checks pass;
independent reviews found no runtime merge blocker. the cli example’s wrong
native-handle assumption produced RED assertions, then passed using terminal
handles. the additional merged phone probe compiled but was skipped by explicit
user direction after adb found no device: [NOT_RUN](issues/reply-notifications-phone-composition.md),
not a pass or an invalidation of earlier source-attributed live evidence.

2026-10-01 arming/clearing/ready qualification, on the
[terminal observation](terminal-observation.md) source (`4e1b737`, affected rows
re-run on `82c0589`): darwin 25.4.0, tmux 3.7c, codex 0.159.2 and claude 2.1.286
against scripted local endpoints, the real gateway and the desktop's real
notification store on its five-second poll. work → idle reads ready for both
providers, including claude with production arguments; work → question → idle,
pending ready → request → idle and starting → idle read idle; a visit keeps an
unanswered request; unknown, an outage and a foreground change cannot bridge
working to idle; an outage preserves pending ready; a delayed idle response
arriving after a visit cannot restore the consumed ready, against a control on
another session and provider that read ready. with codex's off-by-default
`features.default_mode_request_user_input` enabled, the unanswered default-mode
question resolved itself with empty answers after 120 s and returned to idle
without ready; the scripted endpoint ended each continuation at once, so this
run sampled no working after the question, and with a real model a continuation
sampled working arms ready as usual
([observation §9](terminal-observation.md#9-final-state-costs-and-completion)).
pending ready → notice → idle is `NOT_RUN`: no grammar emits a current notice.
the live never-ready check on stale rows ran only on the old faint `unavailable`
cell, which cannot show ready, so it proved nothing. the projected stale cell
(`5c59996`) and the checking cell (`131c1df`) are proven by temporary
desktop-model probes; their live run is `NOT_RUN`
([stale and checking cells](issues/terminal-observation-stale-live.md)).
both clients' stores and projections passed temporary model-level
probes (go: 15 sequences and visits; android: robolectric). on the physical
phone, work → idle raised ready and a visit consumed it, and a visit kept an
unanswered request; its delayed-response row and linux attention runs are `NOT_RUN`
([phone](issues/terminal-observation-phone-acceptance.md),
[linux](issues/terminal-observation-linux-coverage.md)).
