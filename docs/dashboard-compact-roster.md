# compact dashboard roster

accepted, implemented and roster acceptance verified 2026-10-14.
[verification](#verification) records the phone/source evidence and remaining
real-session qualification; no production installation.

## scope and decisions

show more distinguishable sessions through compact cards and group boundaries.
this document owns the replacement card composition, heading policy and vertical
collection spacing. [compact chrome](dashboard-compact-chrome.md) owns the header,
tabs, notification actions and shared menu; integrate it before roster code so
the two assignments never edit the same files concurrently.

accepted preferences: readable status and complete dwarf names may grow a card;
recognition uses crest plus names, not pairwise recognition of all 101 crests;
later verification records content-free installed-dashboard geometry before
comparing synthetic old/new layouts. device access remains separately authorized.

the nominal common card is `78dp`, versus the earlier `94dp`: a `48dp` upper block,
`2dp` gap, `16sp` footer line and `12dp` vertical padding at default text scale.
with `4dp` card gaps instead of `8dp`, nominal row pitch falls from `102dp` to
`82dp`. these are design budgets, not measurements or fixed heights. chrome's
separate height saving must not be credited to this change.

## card content and layout

the card content/interaction designer owns the following content contract.
good content preserves observed facts, makes work/status dominant, and keeps the
dwarf signature secondary. no invented abbreviations, icons replacing facts,
badges, counters or new status copy. external labels retain their exact spelling.

```text
[36dp crest] session-name                         ...
             literal status · detail   dwarf-name
profile · machine   …/directory
```

retain the existing surface, cut corners, gold lip, angular press and type sizes.
use `8dp` horizontal and `6dp` vertical body padding. the upper block is at least
`48dp` high: top-aligned `36dp` crest, `8dp` gap, weighted text column, and `48dp`
end reservation for the overflow. only this block reserves overflow width.
the body and top-end overflow remain sibling accessibility nodes; moving the
button inside the body's cleared semantics is forbidden. its mark stays aligned
with the first name line; its actual target remains `48dp × 48dp`.

| content | presentation and overflow |
| --- | --- |
| work name | existing exact tmux name, bold bone data `titleSmall`; at most two lines, then ellipsis |
| status and dwarf | below the name with `2dp` gap; native `FlowRow`, `8dp` horizontal and `2dp` vertical gaps; status first, dwarf second |
| status | existing `sessionStatusContent`, tone, fixed `12dp` facet and motion rules; primary label wraps without ellipsis or line cap; ordinary detail stays one line with ellipsis and complete speech; needs-input queue detail wraps completely |
| dwarf | exact `character.displayName`, display face at `labelMedium` size, muted; wraps without ellipsis or line cap |
| conditional facts | availability, notification failure, objective, in that order below the upper block; full width, `2dp` gaps; warnings wrap without ellipsis or line cap; existing copy/tone and objective two-line limit |
| context | full-width footer after a `2dp` gap; existing data `labelSmall`, muted; fitting policy below |
| needs-input group | after context, existing `group: <label>` or `unassigned`, one visual line with ellipsis and complete speech |

ordinary detail can move the dwarf to another line. text growth may move following
cards; do not reserve blank lines, shrink fonts or animate layout to conceal it.
this replaces the former fixed-height status invariant. the ordinary short-name,
no-detail/no-notice/no-objective local card remains the three-row case.

the footer is a native `FlowRow` with `8dp` horizontal and `2dp` vertical gaps:
one exact profile/host cluster, followed by an optional directory item. use
middots only inside the cluster, so a wrapped path has no stranded separator.
both flow containers fill their available width; their direct items are
unweighted and content-sized, never independently filling the line.

| execution context | profile/host cluster | directory item |
| --- | --- | --- |
| local | `[resolved profile · ]<terminal machine>` | existing abbreviated cwd when present |
| known remote | `[resolved remote profile · ]running on <execution machine> · terminal on <terminal machine>` | abbreviated remote cwd, or literal `directory unavailable` |
| unknown remote | `remote context unknown · terminal on <terminal machine>` | omitted |

brackets denote omission, never literal text. reuse `sessionProfileLabel` and
`remoteAgentLabel`; unproven profile remains `<provider> · profile unknown`.
a plain terminal has no profile. never substitute its launch profile. the cluster
wraps fully without ellipsis. the path uses `abbreviatedDirectory`, moves whole
to the next line when necessary, and uses one-line start-ellipsis only when wider
than a complete footer line. `directory unavailable` wraps fully. accepting a
second line avoids a tiny, useless path fragment. full cwd is still spoken once.

`DwarfPortrait(character)` becomes the single `36dp` renderer, including its
existing debug gallery caller. retain `sealSpec`, traits, palette and geometry.
retain the bone initial as decorative lettering: fixed `12dp` glyph size,
`16dp` line height and `6dp` inset, independent of font scale; convert those
dimensions locally, never override density for the card. its full neighboring
name scales normally. no optional size, old-size branch or catalogue redesign.

## collection content and continuity

the collection content/interaction designer owns this table. good grouping
names scope once, preserves useful boundaries and never conceals sessions.

| view | headings and context | order |
| --- | --- | --- |
| `all` | one full-span heading per nonempty group, including unassigned | existing group comparator and session order |
| named group or unassigned | no repeated heading; selected tab names scope | existing session order |
| `needs input` | flat; retain each card's group account | existing queue priority and tie order |

all-view headings use data `labelMedium`, muted, no vertical padding, one visual
line with ellipsis, complete spoken label and heading semantics. the grid's
vertical spacing is `4dp` between every rendered item, including recovery
notices; horizontal spacing stays `8dp`. retain the `300dp` adaptive columns,
`12dp` side/top insets, `84dp` trailing forge clearance and pull behavior.
the forge clearance is trailing content padding, not a viewport exclusion.

omit selected-view headings in `dashboardItems`, not by rendering empty items.
retain all-view heading keys, session lifetime keys and the common projection
used for rendering/restoration. `DashboardEntryState` and schema `4` stay intact.
existing missing-anchor clamping handles a removed heading. saved offsets may
land approximately after height changes; no migration, old-layout restoration,
reset-on-poll or per-view history is added.

## contracts and composition

```text
existing inventories and observation/notification projections
  -> dashboardItems(machines, view): List<DashboardItem>
  -> existing lazy grid -> SessionCard(existing inputs and captured callbacks)
                       -> existing entry-state anchors
```

`SessionCard(...)`, `DwarfPortrait(character)` and `dashboardItems(...)` retain
their signatures. reuse native rows/columns/flow rows; no custom measurement
engine, layout model or presentation cache. consume chrome's `MenuAction` and
`ActionMenuButton`; never restore the retired session-menu aliases.

one body tap opens its captured terminal; the menu/custom actions keep their
order, enablement, exact targets and confirmations. stale cards retain last
observations, literal fences and disabled actions. full spoken facts and the
two-node body/overflow structure remain. the layout never reclassifies status,
infers remote activity or retargets control from displayed labels.

the shared `sessionStatusContent.accessibilityLabel` also includes ordinary local
command detail once; the terminal header receives the same correction. classification
and interfaces stay intact. no wire api, dto, persisted schema, controller, poller,
gateway, provider or notification-owner change. existing typed inputs already contain every fact.

## ownership and implementation sequence

bare production filenames below are relative to
`android/app/src/main/java/dev/niels/skidbladnir/`; other paths are repo-relative.

| owner | exclusive files and work |
| --- | --- |
| card designer | this document's card content/overflow decisions and synthetic visual review; no production edits |
| collection designer | this document's grouping/spacing decisions and whole-screen review; no production edits |
| card builder | `SessionCard.kt`; `ProductModel.kt` only for complete ordinary-detail speech at `sessionStatusContent`; `android/app/src/debug/java/dev/niels/skidbladnir/SealGalleryActivity.kt` only to align its existing gallery description with the shared renderer |
| collection builder | `Groups.kt`, `DashboardScreen.kt`; projection, heading style and vertical gaps only |
| root integrator | owning docs; temporary `android/app/src/androidTest/java/dev/niels/skidbladnir/CompactRosterProbe.kt`, `android/app/src/debug/java/dev/niels/skidbladnir/CompactRosterProbeActivity.kt`, `android/app/src/debug/AndroidManifest.xml` and `android/app/build.gradle.kts`; align only that build file's catalogue-asset comment, remove all temporary verification changes |
| independent verifier | read-only review of contracts, diff, renders and evidence; writes no tests or production files |

after chrome integration and baseline red, the two builders may work in parallel.
root alone runs shared builds/device checks. root aligns architecture, roadmap,
session card, session views, groups, refresh boundary, design language and dwarf
seals; update the existing `issues/session-card-hands-on.md`, not a duplicate.

hard cut: replace the old composition in place. delete separate who/where rows,
the full-card overflow gutter, obsolete fixed-height assumptions and redundant
selected-group heading emission. remove dead helpers/imports only after caller
search. keep one renderer, no density setting, compatibility aliases or old/new
branch. preserve runtime missing-anchor handling; it is not a legacy reader.

## temporary red green refactor and acceptance

follow [testing policy](rules/testing.md) and chrome's disposable instrumentation
setup, without retaining its probes or recreating retired gates. three focused
probes cover geometry/grouping, overflow/content/semantics, and interaction/
continuity. use production theme and typed dummy inventory. call the existing
`DashboardDwarfCollection` with real entry state, `entry::restoreOnce` and inert
captured action callbacks for collection geometry/interaction. separately render
`DashboardMain` with a real unstarted controller for actual chrome/forge placement;
its controller-bound menu callbacks are not inert and must not execute synthetic
mutations. no copied renderer, new seam, production test tags, private-state
mutation or gateway harness.

1. **red:** after execution-turn approval, record installed version, viewport,
   font scale, content-free bounds/counts only. separately run the unchanged
   source roster after chrome integration with fixed dummy inventory and fixed
   chrome. demonstrate failure of the compact-card budget, selected-heading
   omission and spacing. installed appearance is not source-red evidence.
2. **green:** implement the two owned slices and rerun the same assertions.
   inspect synthetic pixels as well as semantics: the old card already speaks
   full names while visually truncating them. designers review content and
   hierarchy; verifier challenges fit, real-boundary coverage and false passes.
3. **refactor:** remove duplication/dead paths; rerun affected probes after
   meaningful changes. verifier checks exact targets, semantics and cleanup.
   delete probes, activity, fixtures, runner/dependencies and temporary manifest/
   build changes; retain the build file's catalogue-asset comment correction. run
   `scripts/check verify android`. record concise outcomes and limitations.

| acceptance | required evidence |
| --- | --- |
| density | built-in `all`, scroll at top, fixed `384×640dp` collection viewport, `1×` text, 12 local short-name sessions in one named group, `working` without detail/objective/notices: common card at most `80dp`, `4dp` vertical gap, and at least one more complete visible card than source baseline; identical chrome/window/font settings; record heights/counts |
| heading policy | actual `all` retains nonempty full-span headings; named/unassigned omit them; needs input stays flat; relative session order and exact keys preserved; heading line nominally `16dp` at default text |
| visible facts | actual synthetic renders at `320dp`/`384dp` widths and `1×`/`2×` text, plus `832×384dp` landscape: long tmux/dwarf/profile/host names, unicode groups, plain terminals, unknown profiles, remote/unknown contexts, warnings/objectives and queue reasons; no clipped primary status, dwarf, host or required warning; no overlap |
| touch and speech | body/overflow remain two reachable nodes; overflow target at least `48dp` square and clear of painted text/neighboring controls; overlapping body/overflow bounds are intentional; full status/dwarf/profile/cwd and applicable group spoken once; enabled action fires once, disabled action never fires; menu taps never invoke body, body taps outside it never open menu |
| continuity and final row | selection, inventory changes, surviving/removed anchors through real entry state; return preserves a surviving semantic anchor; last row can scroll clear of actual forge seal; wide layouts retain full-span group boundaries |
| physical use | on the actual phone, the same synthetic old/new comparison with identical view/inventory/chrome/window/font settings gains a complete ordinary card and shows readable crest/name pairs; inspect long/large-text cases, menu placement and talkback; full-dashboard count includes actual chrome and forge occlusion, reported separately from roster-only density |

freeze dummy fixtures before red: short names such as `task-01`, dwarf `Durinn`,
configured `Codex · Personal`, machine `macbook`, cwd `/workspace/code/project`;
include `Alberich of Nibelheim` among overflow cases. do not shorten fixtures after
a failure. count complete cards only; no partial or obscured card counts as a win.
a callback probe proves target delivery, not terminal attachment. retain existing
attachment ownership; an owner-operated terminal-return observation is separate
live evidence and may not be inferred from the synthetic probe.

adb/phone, tmux and integration/live operations require explicit approval in
their execution turn under `AGENTS.md`; this plan grants none. use a disposable
test package; preserve installed pairings, notification memory and user work.
never save real dashboard screenshots/hierarchy dumps, terminal bytes or account
data; synthetic images and numeric geometry are sufficient. unavailable checks
are `NOT_RUN`, never passes; maintain the existing qualification issue until
required evidence exists. no release or production installation is implied.

## costs and exclusions

accepted costs: smaller crests lose detail; the decorative initial does not grow
with font settings; wrapping moves neighboring cards and reduces density for
long/remote/exceptional content; paths may take a second row; tighter spacing and
smaller headings weaken section separation; selected groups lose a redundant
heading navigation stop; saved offsets can shift once. deleting temporary tests
leaves no retained behavioral regression protection. chrome's costs remain in
its own specification.

no header/tab/settings redesign here, collapsed or sticky groups, group counters,
two-column portrait mode, font shrinking, new animation, seal traits/catalogue
changes, global typography, new dependencies, transport/enrollment fixes,
backend work, new state/schema, release or deployment.

platform constraints: [native flow layout](https://developer.android.com/develop/ui/compose/layouts/flow),
[touch targets](https://developer.android.com/develop/ui/compose/accessibility/api-defaults),
[scalable text](https://support.google.com/accessibility/android/answer/12159181?hl=en).

## verification

2026-10-14, branch `codex/compact-dashboard-roster`, base `8551cd3` plus the frozen
compact-chrome source. the separate chrome worktree was left untouched; its
presentation source stayed identical throughout the comparison.

installed baseline: v0.14.0, 1080×2340px, 480dpi override, 1× text; the current live
view exposed two 112dp cards with 8dp gaps. only bounds/counts/version were read;
no real screenshots, hierarchy files or session/account content were retained.
this contextual sample is not the density comparison.

| identical synthetic fixture | old roster | compact roster |
| --- | --- | --- |
| 384×640dp collection, common card | 94.58dp | 78.22dp |
| vertical gap | 8.18dp | 3.91dp |
| all-view heading | 32dp | 16dp |
| complete collection cards | 5 | 7 |
| native 360dp dashboard, complete cards clear of forge | 3 | 5 |

the phone comparison used the same chrome, `SetupRequired`, window, font scale and
inventory. it makes no healthy-notification or chrome-height claim. requested
synthetic bounds were fitted inside the physical window; counts exclude clipped
or forge-obscured rows.

three temporary on-phone integration probes pass: geometry/grouping; 45 context/
width/font render cases; touch routing, live fences, exact surviving-anchor/offset
restoration, missing-anchor clamp and final row above the actual forge. the old
source fails all five density/heading assertions and ordinary-command speech;
its preservation/continuity probe passes. designers and an independent verifier
reviewed source, probe sensitivity and rendered normal/large-text/wide boundaries.

the owner confirmed physical talkback speech, body-to-overflow navigation and
crest/full-name readability. the extra automatic native-event recorder is
inconclusive: the concurrent chrome probe took the phone foreground. it is not
reported as an automatic traversal pass. original accessibility settings were
restored. all temporary probes, activity, fixtures, runner/dependencies, manifest and
application-id changes are removed; only the gallery comment correction remains.
both disposable packages were uninstalled. a clean rebuild passes
`scripts/check verify android`; the final app has its production application id,
no probe activity and no instrumentation runner. the independent final review
found no blocking findings. no behavioral test suite is retained.

terminal attachment/owner-operated return and destructive confirmation journeys
are unchanged and were not exercised against real sessions. the existing
[card qualification issue](issues/session-card-hands-on.md) retains those limits.
