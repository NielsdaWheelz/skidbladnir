# terminal attention notifications

status: implemented in source. the 2026-09-30 behavioral qualification on
`codex/reply-notifications` covers the earlier working/blocked/idle machine; the
2026-10-01 qualification at the end of this document covers the
arming/clearing/ready machine below on the darwin desktop. its phone and linux
runs are `NOT_RUN`. deployment is outside this change.
replaces the former native-work/head prerequisite and human unread/viewer rules.
[terminal observation](terminal-observation.md) owns status facts;
[terminal control](terminal-agent-control.md) owns exact targeting.
native history supplies no notifications; no helper/provider/pin upgrade.

## behavior

humans enter terminals. `ready` means this client observed the same foreground
agent working and subsequently idle. it proves neither a new reply, success,
completion, admission nor an empty queue. codex and claude use one policy.

| event | effect |
| --- | --- |
| consecutive arming -> ready outside a visit | green `ready` |
| arming sample | clear pending; arm predecessor |
| other clearing sample | clear pending; disarm |
| ready without an armed predecessor | pending unchanged; disarm |
| any other sample; stale/offline | disarm; retain pending, hide green |
| startup/foreground recovery first sample | quiet baseline; preserve matching pending; an arming sample arms |
| positive foreground replacement/exit | clear former pending and predecessor |
| first successfully presented output on entry | immediately clear exact terminal pending |
| active terminal visit | suppress pending and predecessors |
| successful visit ends | clear; persist outstanding closing baseline |
| first qualified post-visit sample | clear closing flag; ready stays quiet; an arming sample arms |
| entry fails before any output presentation | clear nothing |

only a fresh successful terminal-source sample of a local foreground agent is
classified. arming is working + interaction none + notice none. clearing is any
permission/question/confirmation/setup/input or menu interaction, any notice, or
activity starting or working; every arming sample also clears. ready is idle +
interaction none + notice none. a qualified sample is clearing or ready.
everything else only disarms: unknown breaks continuity even after successful
capture, and a request clears even when activity is unknown. never bridge
`working -> unknown -> idle`. cached status cannot mutate notices.
positive shell/non-agent observation may settle a closing boundary; absent
identity on unavailable observation does not prove agent exit.

identity = machine + tmuxId + identityToken + paneId. continuity additionally
requires provider + pid + startIdentity. reuse existing facts; names, titles,
profile and native conversation never select notices. same-process /resume stays
the same terminal. remote transports/shells have no agent-ready notice.

explicit costs: inference can be wrong; cancellation/navigation may appear ready
without new text; work between polls and offline completions may be missed; missed
renewed work may leave old pending until entry or a clearing sample.
the first qualified post-visit observation is the selected closing boundary and
can consume readiness just after detach; outages extend it. concurrent clients
may conservatively lose transitions. focus is per client/device. old unread
notices are dropped at cutover; old files stay inert. temporary tests are removed,
leaving no retained behavioral regression coverage.

non-goals: proven native reply/turn semantics, inbox/counts, human read receipts,
cross-device focus, new polls/hooks/supervisors/event ledgers, provider forks,
naming/startup/control redesign, deployment and unrelated native-history repair.
retain explicit machine native read/control under terminal control's preservation
exception; native failure never selects terminal operations.

## owner, schemas and interfaces

```text
existing inventory/status -> serialized client notification owner
existing output/attachment lifecycle -> same owner
committed snapshot + fresh status + existing visit -> one client projection

terminalKey = {machine, tmuxId, identityToken, paneId}
foreground = {provider, pid, startIdentity}
record = {key: terminalKey, revision: nonnegative int64,
          foreground?: foreground, pending: boolean, baselinePending: boolean}
store = {schema: 1, terminals: record[]}
client predecessor? = {key, foreground, revision}
expected record = absent | present(revision)

Read() -> snapshot
Observe(current inventory samples, captured expected records,
        predecessors, mode: ordinary|baseline|visiting)
    -> committed snapshot + next predecessors
Presented(key) -> committed snapshot
EndVisit(key) -> committed snapshot
```

these are internal semantic interfaces, not a new wire api. implementation may
combine the two consumption methods behind one private reducer. no persisted
working state, conversation/result ids, history, associations or active-visit map.
predecessors die on startup, foreground loss, uncertain observation and conflicts.

replace unread ownership in place. desktop retains stable-sidecar flock and
read/merge/atomic replace; android retains datastore serialization. batch an
inventory's updates into one write. accepted observations advance each record's
revision, including unchanged samples, to invalidate competing older samples.
explicit cost: one serialized cache write per accepted inventory batch on the
existing five-second cadence. never wrap revisions; absence differs from zero.

read the current store immediately before each network dispatch; capture expected
record revisions from that read, not the cached UI snapshot. retain a predecessor
only if its revision matches; never rebase it. mode is per terminal sample: only
the exact presented visit is visiting; all other terminals continue normally.
captured predecessors must still survive in local continuity when merging;
an intervening disarm cannot be undone by an in-flight response.
android's existing per-machine inventory lane finishes after notification commit
and predecessor/snapshot publication, or handled failure/disarm. its trailing
request cannot compete with unfinished local acceptance. every rejected or
invalidated path releases the lane; backgrounding admits no obsolete trailing read.
capture expected record revision before network dispatch; compare under store
serialization. mismatch discards that sample's notification processing and
predecessor, without replay/rebasing. ready sets pending only when its predecessor
was armed for this exact foreground at the previous revision; otherwise pending is
unchanged. baseline preserves matching saved pending but cannot create it; an
arming sample still arms. visiting clears pending and never arms. clearing clears.
other samples preserve pending and disarm. replacement clears and starts quietly.
Presented and EndVisit advance the revision, so a predecessor armed before them
can never make a ready pending: a ready first post-visit sample stays quiet, and
only an arming sample after the visit can lead to a notifying ready. the first
clearing or ready sample, or a positive exit, clears baselinePending;
unknown/unavailable/stale leave it set. no outcome depends on the flag
([issue](issues/notification-baseline-pending.md)).

Presented clears and advances revision even if already clear. EndVisit also sets
baselinePending. call only once on actual presentation and only end a presented
visit. publish the committed snapshot immediately. never lock across network
work. callers discard callbacks outside their existing attempt/generation.

use new `notifications.json`; never open/migrate `unread.json`. validate unique
keys, identities, revision range and strict schema using existing primitives.
corruption/wrong schema is visible failure, never reset. notification storage
failure never disconnects a working terminal. after uncertain atomic commit,
reread through the owner before merging. successful authoritative inventory
retires absent exact session lifetimes; unavailable hosts retain records. an
unselected pane is not proven deleted: retain its revision record and clear its
pending/baseline continuity on positively observed selection change, never
delete/recreate it. remove unconfigured machines.
rollback restores the previous whole release without cache-continuity guarantees.

## one terminal visit

reuse existing attempt/page generation and foreground lifecycle. no persistent
visit registry, detached worker or additional polling. one desktop wrapper serves
browser entry, creation auto-entry and `skid enter`.

1. capture exact terminal; invalidate older callbacks.
2. activate after first real output presentation: fully written nonempty desktop
   payload; android's existing OutputApplied for current page/attempt. hello alone
   is insufficient. consume and immediately publish the committed snapshot.
3. suppress that terminal throughout the visit. android's existing polling uses
   visiting mode; desktop adds no polling during blocking tea.Exec.
4. on detach, loss or backgrounding, persist EndVisit and invalidate old callbacks.
   dashboard return does not wait for host; existing foreground inventory provides
   closing baseline, from a request dispatched AFTER EndVisit commits. a request
   begun during the visit cannot settle it. background defers observation until
   resume. orderly shutdown
   finishes serialized exit write before cancelling its owner.
5. standalone enter makes one bounded existing info observation after a successful
   visit, matching its captured pane as well as exact session lifetime. failure
   retains closing flag for the next foreground client; no loop.

failed entry preserves pending. terminal outcome and notification failure remain
distinct. restart retains pending/closing flags but begins sampling quietly.
a crash before exit persistence cannot prove whole-visit consumption. another
client may notify during this client's visit: focus synchronization is excluded.
direct tmux attachment outside skid produces no local entry event.

## content and removal

each client writer is also its designer. good content names current attention,
makes entry obvious and claims no unseen text. one projection serves cards/table,
details, terminal header and accessibility.

| fresh local condition | literal / tone |
| --- | --- |
| idle + interaction none + notice none + pending outside visit/closing baseline | `ready` / moss green #76B082 |
| idle + interaction none + notice none without pending | `idle` / muted grey #AAA69D |
| any other status | [observation §6](terminal-observation.md#6-content-attention-and-filtering) label and tone; working is frost blue #78A9C6 |
| stale | existing last-observed treatment; no green |
| shell/remote | existing `terminal`; no ready |

ready also requires stored foreground equals the fresh inventory foreground;
new inventory cannot briefly render the former agent's pending before the store
update commits. visiting clears even on unknown samples.
details/accessibility disclose `inferred from terminal`, including ready.
primary labels are exclusive. storage failure is one muted secondary
`notifications unavailable`. labels survive NO_COLOR; no counts/pulse,
unchanged-poll announcements or sorting changes. preserve status before cwd in
narrow layouts.

delete desktop r/replies, human viewer/ack callbacks, history scan/store and
previous-agent notification projection; android reply action/sheet/controller
and viewer-only read/result models. preserve explicit native machine read and
other native capabilities; search callers before removing shared primitives.
no unread/acknowledgement copy, legacy reader or notification fallback remains.

## disjoint delivery and verification

| writer | exclusive implementation paths |
| --- | --- |
| desktop + designer | internal/fleetclient/, internal/sessionui/, internal/agentcli/, internal/terminalclient/ |
| android + designer | android/app/src/main/java/dev/niels/skidbladnir/ |
| root | docs/; prerequisite integration outside writer paths |
| adversarial reviewer | read-only; no production/test writes |

root serializes dependency integration; never edits the other worktree.
reuse its terminal-status primitives, not another detector. no host notification
api. interface-preserving implementation changes must not affect consumers.

workflow: adversarial contract review -> temporary integration/live red tests ->
both clients -> adversarial review/refactor -> affected green checks -> remove
temporary tests -> scripts/check verify -> commit. engineering checks are not
behavioral acceptance. no production test seams or retired harnesses.

prove both providers/clients: working->idle ready; working, requests, menus and
notices clear;
unknown/stale/outage breaks continuity; successful entry clears only after
presentation; failed entry preserves; whole visit/first closing sample quiet;
working closing baseline's later idle notifies; replacement cannot inherit;
delayed callbacks and concurrent desktop writes cannot undo consumption;
restart quietly preserves pending; corruption/failure visible; viewer absent;
native machine read unchanged; narrow/accessibility/colors and NO_COLOR correct.

tmux tests use exact owned sessions on isolated -L sockets. tmux/live/phone
operations require current-turn explicit approval under AGENTS.md. absent live
or device boundaries are NOT_RUN, never passes. evidence contains no terminal
bytes, prompts, objectives, provider ids, account data or credentials. remove
resolved issues only with evidence; native history limitations remain separate.

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
another session and provider that read ready; codex's unanswered question, which resolves itself after 120 s,
returns to idle, never ready. pending ready → notice → idle is `NOT_RUN`: no
grammar emits a current notice. stale rows stay out of ready: the live run passed
through the old faint cell, and the projected stale cell was proven by a temporary
desktop-model probe at `5c59996`. both clients' stores and projections passed
temporary model-level probes (go: 15 sequences and visits; android: robolectric);
physical-phone and linux attention runs are `NOT_RUN`
([phone](issues/terminal-observation-phone-acceptance.md),
[linux](issues/terminal-observation-linux-coverage.md)).
