# session views and needs input

accepted 2026-10-02; source implemented in `session-views`. phone and desktop expose one
selected view: `needs input`, `all`, then groups. the phone removes machine
filtering and uses one horizontally scrolling view strip. `needs input` collects
agents ready for the operator's next action: ready first, then requests and
current error/interruption notices. quiet idle stays out. all/group views keep
their stable order.

this document owns the replacement collection/navigation contract and its
implementation plan. it supersedes the independent needs-input toggle, desktop
agents view, phone machine filter and their dependent restoration/pressure
placement clauses in the linked specifications. [the roadmap](roadmap.md)
tracks delivery; [qualification](#qualification) records the implemented source
and its verified boundaries.

[terminal observation](terminal-observation.md) retains status facts, copy,
presentation precedence and the strict request predicate used by cli wait.
[terminal attention](reply-notifications.md) retains ready memory, identities,
visits and persistence. [groups](groups.md) retains label identity and membership;
[desktop browser](desktop-browser.md) retains table/actions/attachment;
[dashboard continuity](dashboard-return-continuity.md) retains semantic viewport
restoration. [session card](session-card.md) and
[machine pressure](machine-pressure-rail.md) retain their components except for
the explicit presentation changes below. [searchable group entry](group-entry.md)
remains an independent accepted change to creation/editing.

## outcome and boundaries

the first view answers: what is ready for my input or needs my intervention?
it combines pending availability attention with currently observed requests and
abnormal-stop notices. it is neither the complete agent roster nor a claim that
work succeeded. `ready` means observed non-idle followed by idle, unacknowledged
on this device. quiet idle includes both acknowledged ready and first-observed
idle, where no preceding activity was seen.

the change stays in existing client state and presentation owners. no new host
api, wire status enum, detector rule, provider integration, poller, dependency,
group registry, navigation framework or notification store is required. the cli's
`wait --state needs-input` continues to mean an explicit response request;
human collection membership never becomes control admission.

recognized error/interruption notices are covered. disappearance of the agent
process into its shell is not: existing attention resets on foreground exit,
and process absence does not establish whether the exit was unexpected. the
[exit-attention gap](issues/agent-exit-attention.md) needs separate scope and
acceptance. no crash detector or retained incident history is implied here.

## membership and ordering

derive the queue category beside each client's existing typed status projection.
apply the following first-match rules to the same admitted facts used to draw
the row. all eligible observations require fresh inventory, source terminal and
a recognized local foreground agent; remote descriptive agents are excluded.

| condition | queue category and order |
| --- | --- |
| host stale/checking/unavailable, unavailable status, no local agent, or remote execution context | excluded |
| permission, question, confirmation, setup or input interaction | action, second tier, regardless of activity |
| current error or interruption notice | action, second tier, regardless of activity |
| menu interaction without a current notice | excluded |
| qualified visible ready under the existing attention contract | ready, first tier |
| everything else, including quiet idle, starting/working alone and unknown without a positive request/notice | excluded |

a working or unknown-activity agent with a recognized question qualifies. a
current request or notice outranks stored ready in classification: an error
displayed over a pending ready belongs in the second tier. a working agent with
a qualifying request/notice retains the existing `work continues` detail. do not
compare rendered strings, colours or raw stored-ready bits to decide membership.
when a menu takes primary display precedence over an independently recognized
notice, the notice still qualifies. show `error shown` or `interruption shown`
in the queue's existing status detail and speech so the inclusion is explained;
keep the primary `menu open` label and any work-continues fact. menu alone never
qualifies. this contextual detail introduces no new observation or badge.

sort the whole needs-input collection by tier, then the client's existing stable
session order: desktop configured peer then numeric tmux id; phone
case-folded/exact machine label, machine handle, then numeric tmux id. no sort by
name, inferred completion time or notification age. requests, errors and
interruptions share a tier. no idle tier exists.

`all` retains named group headings in the existing group comparator order, then
unassigned, with the existing session order within each group. a group view
retains that group's existing order. both include ordinary terminals, working
and unknown agents, and explicitly stale retained rows. the cli's inventory
ordering is unchanged.

failed-host notices remain outside every view. a failed attention admission hides
that machine's ready without hiding fresh requests/notices. its failed outcome
remains attached while that inventory is admitted; only its matching success can
repair it. inventory/visit invalidation or a modeled non-live outcome replaces
the sample and its outcome. another machine's no-op store transaction cannot
repair the still-admitted failure. the existing global
operation failure also hides ready while current. successfully matched machines
can show ready when that global operation failure has cleared. expose
`notifications unavailable` outside the filtered collection while either failure
remains, including when no cards survive. derive this disclosure from the existing
failure owner and admitted outcomes, with no second failure state machine.
while initial inventory/restoration is unresolved, keep the existing checking
presentation. once settled, an empty queue reads
`no sessions currently need input in this view`; uncertainty notices remain
visible alongside it. an empty view is not proof that every agent is healthy.

## appearance and removal

| inclusion reason | how that reason ends |
| --- | --- |
| ready | actual terminal output presentation acknowledges it, or a fresh non-idle observation replaces it under the existing attention transition rule |
| response request | a fresh observation no longer shows the request |
| error or interruption | a fresh observation no longer shows the current notice |

recompute the union after any reason ends: removing one reason does not remove
a session that still qualifies for another. fresh uncertainty may hide a row
without resolving its work or consuming saved attention. confirmed session or
foreground replacement cannot inherit the former identity's ready; classify
the replacement's own facts normally.

selecting a row, opening info, changing views or attempting attachment does not
acknowledge ready. failed entry before output presentation leaves it pending.
opening a terminal never answers a request or clears its current error notice.
an inspected error can therefore remain until the provider replaces its notice,
usually during subsequent work. resumed activity alone does not prove the
notice cleared. no manual dismiss control or notice-acknowledgement record is
added. stale snapshots cannot clear a request or acknowledge a notice.

the existing attention owner's gap, visit, revision-fence and restart rules
remain authoritative. ready is device-local; phone and desktop may legitimately
show different ready rows. direct tmux visits outside skid do not acknowledge it.

## selected views and continuity

one closed selected-view value replaces the independent collection dimensions:

`needs input | all | named group | unassigned`

desktop keeps its separate machine scope, including the visible narrowed-scope
label and `m` picker. phone collection scope is always the accepted fleet.
view changes are local projections; they never discard inventory or narrow
refresh to hosts previously observed in the selected group.

the strip order is `needs input`, `all`, observed named groups in the existing
label order, then unassigned when observed. retain the selected group even when
empty or unresolved after restoration. group choices come from complete retained
inventory in scope, not needs-input results. use typed identities, never tab
indices or displayed label text. exact label equality remains. draw named-group
tabs, headings and details with bare labels; truncate long tab labels directly.
named `all`, `needs input` and `unassigned` can read like the built-in views,
but their typed identities remain distinct.
speech identifies named group tabs as groups and retains their complete labels.

fresh launch remains `all`, all machines, at the top. initial selection and first
tab position are separate. selecting the active view is a no-op. desktop view
changes preserve the selected lifetime if present, otherwise select the first
row or none and bring it into view; refresh preserves a surviving lifetime and
otherwise clamps its former index. phone view changes use the existing keyed
grid/clamping behavior, with no per-view history. live reordering cannot retarget
a captured action, editor, confirmation or terminal attachment.

terminal return retains the view and semantic viewport. a consumed ready may
have disappeared, so existing removed-anchor fallback applies. metadata edits
retain the view, even when the edited session leaves it. confirmed creation
leaves needs input to reveal the returned session in its actual named/unassigned
group; a matching all/group view remains selected. failed or uncertain creation
does not change the view or replay the request. unresolved restored named groups
still require explicit group selection before creation.

## phone presentation and machine scope removal

replace the machine chips, group-selector sheet trigger and needs-input toggle
with one horizontally scrolling single-selection strip. keep the current theme,
clear selection, 48dp targets and full spoken names/selected semantics. selectors
remain outside pull-to-refresh. labels stay on one line, with bounded visual
ellipsis for labels wider than the available strip; speech retains the full
label. reveal the selected tab after restoration or programmatic selection using
the existing minimal bring-into-view behavior. ordinary polls must not reset the
strip while the user scrolls. exact horizontal offset is not persisted.

needs input uses the existing 300dp-minimum adaptive card grid, globally ordered
by tier. group headings cannot precede ready-first order. in this view only, add one quiet
`group: <label>` line after the card's where line, or `unassigned`; use the
existing data typography, one visual line with ellipsis and the full group once
in the existing body speech node. add no separate action or badge. this costs
one card line in exchange for group context without interleaving priority tiers.
all/group views retain their headings and existing card geometry. every phone
card now shows its machine, including in a named group.

remove machine scope from `DashboardEntryState`, collection projection,
restoration readiness, pull verification, forge gating/defaults and recovery
copy. remove controller writes that select a machine during source-forge entry,
draft recovery or terminal access loss. access loss still returns to the
dashboard, cancels pending restoration, resets the viewport to top and exposes
the affected machine's notice, while preserving the selected view. actions and
drafts remain bound to their exact machines. standalone forge requires an
explicit machine; source creation keeps its source machine. availability of any
eligible machine enables standalone creation.

manual verification snapshots every live fleet inventory poller and retains the
existing post-request completion fence. pressure is not refreshed by this
gesture. saved restoration waits for each fleet machine's existing retained,
current or modeled non-live outcome before resolving its one anchor. one failed
host does not prevent another host's actions.

for a restored needs-input view, a fresh inventory is insufficient until its
corresponding attention result is admitted or storage failure is modeled.
settle the queue projection and anchor from those matched outcomes, using the
existing read-sequence and completion callbacks. admit inventory and its attention
projection together; retain the preceding admitted sample while its successor
joins the store. do not consume restoration or show a settled empty queue in that
intermediate state. completion callbacks may settle a pull only for their current
credential generation and poll runtime; an old runtime still releases its own lane.
this adds no poll or store and does not make failed hosts wait for recovery.

replace the top-bar `reconnect fleet` button with `machines`, opening one small
machine sheet. it lists the accepted machines with their existing pressure
rails and exceptional access/pressure state, and includes `reconnect fleet`.
the machine list scrolls within the available sheet height, including at large text.
tapping a rail opens that machine's existing pressure details; details back
returns to the machine list, then back returns to the unchanged dashboard.
`dismiss`, outside tap and swipe dismissal close the disclosure from either
page. reconnect closes it before opening the existing fleet flow. use one
transient `machines | details(machine handle)` selection, absent when closed, in
`DashboardMain` and one modal sheet, reusing the current rail/details content.
extract the existing details body from its modal wrapper only as needed; do not
stack modal surfaces or add navigation history. no request is triggered by
disclosure; existing polling supplies observations. no machine selection filters cards or
prefills forge. exceptional machine notices stay visible on the dashboard even
while the sheet is closed. pressure metrics, history, semantics and action
admission retain their current owners.

## saved navigation

advance the dashboard task capsule from schema 3 to schema 4. it carries only
the selected-view discriminant, a group fingerprint when named, and the existing
typed session/heading anchor, fallback index and pixel offset. reuse the current
domain-separated fingerprints and semantic restoration algorithm. remove machine
scope and the independent needs-input boolean; no raw label, terminal token,
inventory, priority tier or notification state enters the capsule.

the exact primitive bundle uses the existing registry key
`dev.niels.skidbladnir.dashboard-entry`:

| key | value |
| --- | --- |
| `version` | integer `4` |
| `viewKind` | `needsInput`, `all`, `unassigned`, or `named` |
| `viewLabelSha256` | only named; 64 lowercase hex |
| `anchorKind` | `none`, `session`, `space`, or `unassigned` |
| `anchorSha256` | only session/space; 64 lowercase hex |
| `fallbackIndex`, `offsetPx` | nonnegative integer rendered index/pixel offset; both zero without anchor |

extra keys, wrong primitive types and inconsistent current-version variants are
trusted-state defects. `space` and the retained `space:` item-key prefix mean a
group; neither changes group identity.

preserve unresolved named-group restoration and save-again behavior while
inventory is pending. use one exact-version reader/writer. unsupported older
capsules reset to all/top; malformed current-version state follows the existing
trusted-state defect policy. this loses one navigation position on upgrade and
does not clear pairings, terminal text size or the independent v2 attention store.

## desktop interaction

replace the agents view with needs input and retire its independent `f` toggle.
`f` selects needs input through the ordinary view-selection path; repeated `f`
is a no-op. retire `a`, whose agents view disappears. left/right and h/l keep
their clamped stepping order. `m`, creation, details, captured controls and
fullscreen attachment retain their existing contracts. update footer and cli
browser help together; remove the old filter mark and agents empty-state copy.

the first view keeps the current flat table and group column. the longer fixed
label consumes five more terminal cells; retain whole system labels, the current
tab, existing overflow markers and the 80×24 minimum. other group tabs may hide
sooner. modal pages retain their input ownership. the independent
[group-entry plan](group-entry.md) owns form acceptance/submission keys.

## implementation plan

the implementation follows these existing ownership boundaries. these are code
ownership boundaries, not requirements for new modules or separate agents.

1. **typed collection projection.** extend the existing status presentation in
   `internal/fleetclient/status.go` and android `ProductModel.kt` with the small
   derived queue category needed by membership/order. renderers and collection
   projections consume the same classification. preserve the strict request
   predicate and cli wait behavior; remove a phone-only filter helper if its
   final caller disappears. add no wire rank or shared cross-language framework.
2. **desktop model and ordering.** update
   `internal/sessionui/{session,navigation,view,notifications}.go` and browser
   help in `internal/agentcli/run.go`. replace agents/filter state with one view,
   retain captured lifetimes and group creation defaults. currently inventory
   calls `rebuild` before `observeNotifications`: derive rows after the admitted
   observation/store outcome so order and labels use the same snapshot. attachment
   return must rederive after installing its acknowledgement snapshot, before
   awaiting a new inventory. cover notification failure/recovery and creation
   completion through these existing paths; keep the store reducer unchanged.
3. **phone model and projection.** update `DashboardEntryState.kt`, `Groups.kt`,
   `ProductModel.kt` and `SkidbladnirController.kt` under
   `android/app/src/main/java/dev/niels/skidbladnir/`. replace the three selection
   dimensions, remove all hidden scope writes, project the ranked flat queue,
   preserve group/all projections and update capsule/refresh/recovery ownership.
   use the same admitted `NotificationPresentation` as the card; publish queue
   membership and labels together. settle restored queue anchors after the
   matching attention completion, including failure. expose notification failure
   even with no rows.
4. **phone presentation.** update `DashboardScreen.kt`, `SessionCard.kt` and the
   narrow visibility/disclosure wiring in `PressurePresentation.kt` and
   `MachinePressureRail.kt`. replace the controls, always show card machine,
   provide queue group context and route machines/pressure/reconnect through one
   transient disclosure. remove unused scope-dependent helpers and imports;
   `MainActivity.kt` wiring changes only if its existing calls require them.
5. **integrate and verify.** reconcile the predecessor clauses referenced above
   plus affected design-language, refresh, creation and codebase-map references
   with the implemented state. preserve independent group-entry/card/chrome work.
   exercise the acceptance below, remove temporary checks under
   [testing policy](rules/testing.md), run `scripts/check verify`, and update the
   issue/status with actual source-attributed results. release, installation and
   deployment are outside this plan.

## acceptance

| boundary | required evidence |
| --- | --- |
| membership and rank in both clients | ready included first; all request kinds and both notices included second; request with working/unknown activity; notice with working; menu with notice included with visible/spoken reason; ready hidden by notice ranks second; menu alone, quiet idle, plain working/unknown, remote and stale/checking rows excluded; stable ties |
| attention and removal | observed work → idle enters; successful output presentation consumes existing ready and removes the row when no other/newer qualifying reason remains; failed entry does not consume it; opening leaves requests/notices; notice clearing removes its reason while another reason can retain the row; a new observed work → idle re-enters; foreground replacement cannot inherit ready |
| publication and uncertainty | first admitted ready appears in the right place on that update, not one poll later; acknowledgement rederives membership before the next inventory completes without erasing newer attention; storage failure hides ready but preserves requests/notices and visible failure copy even with an empty queue; late pre-visit observations cannot restore consumed ready |
| selection and collection | exact selection/anchors survive reorder; removal uses existing fallback; group labels appearing/disappearing never select a different view; selected empty and unresolved groups persist; all/group include terminals and retain their old order; captured editors/actions keep their target |
| phone scope and creation | all machines participate in every view and pull; notices survive empty filtering; access loss, source-forge, draft recovery, confirmed creation and cancellation cannot restore hidden machine scope; standalone creation requires a host and source creation retains its host |
| phone return and task state | detach/back and task recreation preserve view/anchor without an all/top flash; group/all heading anchors and flat-queue session anchors restore; delayed attention completion cannot prematurely consume a ready-row anchor or produce a false settled empty view; modeled storage failure allows restoration to finish; old capsule resets once; pending save-again and unresolved labels retain their meaning; selected tab is revealed |
| phone interaction | overflowing groups, long/unicode/selector-like labels, 2× text, landscape, spoken names/selection, manual strip scrolling during polls, flat priority order and group context; every machine's pressure details and reconnect remain reachable without changing collection selection; details back returns to list, explicit/gesture dismissal closes the sheet, reconnect closes it before navigation |
| desktop interaction | f and arrows select the documented views; a has no obsolete action; 80×24 with long machine/group labels keeps system/current tabs and overflow markers usable; return, creation and captured controls retain exact targets |

use a small set of temporary production-projection/model probes and real client
journeys, not a second collection implementation. establish the intended
baseline failures for changed behavior. actual attention/attachment and phone
interaction need their real boundaries; static checks and synthetic renderings
do not replace them. do not recreate retired behavioral harnesses or modify
`scripts/check` composition. the detector is unchanged, so its fixture corpus
does not need new cases for a client filter.

tmux and phone execution require explicit current-turn approval under
`AGENTS.md`. unavailable or unexecuted boundaries remain `NOT_RUN`. keep evidence
content-free and credential-free. recording the plan supplies no runtime pass.

## accepted costs

- needs input is a human attention view; cli needs-input wait remains the
  narrower explicit-request condition. copy/help must preserve that distinction.
- ready-first prioritizes reviewing available work over unblocking agents.
  readiness remains inferred and device-local, not proof of task completion.
- errors/interruption notices persist after inspection until their current
  screen evidence clears. deliberate interruption may also qualify; the detector
  cannot infer operator intent. no once-per-incident guarantee is made.
- unknown may conceal an unrecognized request; all/groups remain the inspection
  route. failed machines and notification storage remain separately visible.
- exclusive views remove requests-within-group and phone machine-only browsing.
  desktop machine scope remains. standalone phone creation loses machine prefill.
- the flat phone queue gains one group-context line per card; ordinary grouped
  views retain their density. tie order can differ across clients by existing
  contract, while tiers and membership rules agree.
- a menu's independent notice must remain visible. in needs input, its detail
  begins on the next line and wraps. this costs a status line even at default
  text size; all/group status geometry remains unchanged.
- horizontal scrolling hides some groups. longer tui fixed copy hides other
  tabs sooner; bare named-group tabs can read like system views. no tab search,
  counts, badges or per-view history is added.
- desktop `f` becomes direct view selection and `a` is retired; existing filter
  toggle and jump-to-first-agent muscle memory changes.
- pressure moves into a disclosure and reconnect becomes one step further from
  the dashboard. both remain available without recreating machine filtering.
- old saved navigation resets once. new restoration waits for fleet outcomes
  and retains existing sampling/visit limitations; no inventory is persisted.
- phone inventory publication waits for its serialized attention write, including
  modeled failure. this trades a small publication delay for one coherent sample
  and retains preceding admitted ready rows during ordinary polls.
- every machine update derives its notification projection at the existing
  installation owner. this adds bounded local projection work on existing
  updates, with no new polling or state owner.
- failed attention admissions stay unavailable per machine while that sample
  remains admitted, until its matching success. invalidation replaces the sample,
  rather than retaining historical failure. the global operation failure briefly
  hides all ready. after a successful store operation clears that gate, other
  successfully matched machines can show ready; the fleet disclosure may therefore
  coexist with ready rows.
  this is matched-sample availability, not a filesystem-health monitor.
- process-exit notification and inspected-error dismissal are separate future
  capabilities. this plan does not close those observation/acknowledgement gaps.

## qualification

2026-10-02, isolated `session-views` worktree, implementation `7de45db`, based
on `13f9ebc`. temporary
checks exercise production projections and owners, not another collection
implementation. original-source probes fail on missing ready/notice membership,
independent selection, obsolete desktop keys and ambiguous named tabs. observation
before rebuild and attachment rederive probes also reject their corresponding
mutants.

host evidence passes on darwin through an isolated tmux socket, authored native
dummy provider frames, the production process observer/detector, a real tls
gateway, desktop model and tty/websocket attachment. it covers first-update ready,
failed-entry retention, output consumption before another inventory, request
retention and work-to-idle reentry. temporary model/projection probes cover categories,
stable peer/id ties, selection/reorder/removal, pinned confirmation/editor targets,
creation completion/reveal, notification-store failure/recovery and the 80×24
strip/rows. visible menu-notice reasons pass in selected and unselected rows;
targeted ordering/target-ownership mutants fail their intended assertions. this is neither stock or
cloud-provider qualification nor a full interactive metadata-editing journey.

physical-phone evidence passes on the sm-s906w, android 16/api 36, in a separate
application id with controlled loopback tls/wss peers. the actual gateway client,
encrypted pairing store, attention datastore, controller, main activity, compose
dashboard and webview output/acknowledgement paths run unchanged. the installed
fleet application and credentials are untouched.

the dashboard journey covers default all, fleet-wide inventory/pull, ready-first
membership and group speech, actual output consumption, failed entry, request
retention, notice clearing, reentry, detach and activity recreation, selected
group restoration/landscape, explicit standalone host selection/cancellation,
and machines/details/back/reconnect. qr capture is blocked at its external
activity boundary; reconnect navigation and cancellation use their real owner.
no production session creation, qr redemption or installed-fleet deployment is
claimed.

five controller-boundary probes use real tls replies and the actual serialized
datastore. delayed matched attention preserves a restored queue anchor and prior
admitted ready. old callbacks cannot finish a replacement generation's held pull.
real failed atomic writes preserve requests, settle restoration and expose failure
copy even with an empty queue; another peer's no-op cannot repair that failed
sample, and matching recovery restores ready/removes failure copy. definitive
creation rejection cannot revive ready before the matching inventory/attention
result. after real webview output, detach immediately hides another same-machine
ready row while its visit-end transaction is held; the matching read restores
only the unvisited ready. each failure mode reproduced before its repair.

physical model/layout probes cover all request kinds, both notices, uncertainty,
remote/stale exclusion, stable queue/all/group ordering, schema-4 saved-registry
round trips and unresolved save-again, one-time older-version reset and confirmed
creation reveal. at 320dp/2× and in landscape, tabs preserve full spoken labels,
one selection, targets and manual scrolling through polls. measured glyph bounds
and card-only captures prove menu/notice/work detail remains visible. ordinary
card height is retained. real outside taps and swipes close machines and details;
hardware back and explicit dismissal retain their separate meanings.

the actual access-loss and draft-recovery helpers preserve fleet scope and the
selected view; access loss resets the restored viewport, and recovery retains
the draft's explicit host/group. source-forge host binding is reviewed at its
existing owner; no live creation is claimed.

the combined instrumentation runner stalled in espresso's idle loop after its
four controller probes passed. the dashboard journey and model/layout probes
then passed in separate instrumentation processes on the same compiled source.
this evidence does not qualify stock/cloud providers, process-kill restoration
or unrelated camera/pairing behavior.

final `scripts/check verify` passes with scope `all`: host checks/build and
android lint/debug assembly. the five controller probes, dashboard journey and
fourteen model/layout probes pass in separate instrumentation processes on the
final source. all temporary behavioral tests, dependencies, test application
configuration and trust fixtures are removed before commit. the two phone test
packages and temporary baseline worktree are removed; the ordinary debug apk
contains no temporary trust assets. no production app installation or fleet
deployment was performed.

2026-10-02 merge integration: `main` at `6b4865e` adds the independently qualified
[searchable group entry](group-entry.md#qualification). conflict resolution keeps
that implementation and exclusive views; attention, controller and task-restoration
owners are unchanged. full-scope `scripts/check verify` passes on the combined
source. a temporary desktop model/client probe passes with the race detector for
needs input, an empty named view with literal prefill, and all. suggestions come
from retained scoped inventory even when the queue is empty; group acceptance
dispatches nothing; the separate create action sends one request through the real
fleet client to a test-owned https endpoint. repeated in-flight submission is
suppressed. the modeled creation reply reveals the returned group when needed,
selects its exact lifetime and schedules attachment; attachment is not executed.
the probe and listener are removed. this integration step runs no tmux or device
journey; the live/device evidence above retains its pre-integration source boundary.
