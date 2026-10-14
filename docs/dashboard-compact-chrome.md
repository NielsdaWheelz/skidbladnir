# compact dashboard chrome

accepted and implemented 2026-10-14. executed qualification is recorded below.

## scope and outcome

give the dwarf collection more room by removing routine administration and
redundant summary text from the resting dashboard. retain the existing visual
language, capabilities and state owners. this document owns the replacement
header, view-tab presentation and dashboard notification-action placement.
[session views](session-views.md) retains membership/order and machine disclosure;
[dashboard continuity](dashboard-return-continuity.md) retains restoration;
[native notifications](native-notifications.md) retains health/setup/transport.

final order: header; view strip; existing body composition. all notices retain
their current owners/positions, including workspace-recovery notices inside
the grid with their pull exclusion. forge remains overlaid.
healthy chrome targets `48dp + 48dp`, before system insets and the existing
`12dp` collection gutter. the estimated saving is `64dp`; qualification below
records the measured footprint.
large text and actual notices may increase height.

## target behavior and content

the content/interaction designer owns this table and the presentation rules
below for both features. literals are closed content, not a new runtime schema.

| slot | exact content | effect |
| --- | --- | --- |
| title | `Dwarves` | existing display style and silent `24dp` mark |
| overflow | spoken `dashboard options` | existing three-stud mark; button semantics |
| menu row 1 | `machines` | dismiss popup, then open existing machine sheet |
| menu row 2 | `notification settings` | dismiss popup, then launch existing channel-settings intent |
| unhealthy notice action | `set up notifications` | existing setup callback/dialog |
| built-in views | `needs input`, `all`, `unassigned` | existing typed selections |
| named view | exact authored group label | speech: `group: <complete label>` |
| unresolved named view | `previously selected group` | speech: `group: previously selected group`; retain selection |

good content names the destination or current fact once. no counters, badges,
menu-row icons, health suffixes, new status claims or themed instructions.
a named group `all` still speaks `group: all`; it never becomes the built-in
view. long unicode names ellipsize visually and retain complete speech.

**header/menu:** one `48dp`-minimum row, existing `16dp` side insets and `8dp`
gaps. retain the weighted, single-line title with visual ellipsis; one text
node still speaks the complete `Dwarves`, while the mark is silent. remove the
summary without relocating it. the trailing trigger is a real `48dp` square,
with its studs centered at `24dp`. both menu entries are always enabled,
including when no machine is actionable. opening/dismissing changes no
collection selection, viewport or controller navigation and initiates no
request; existing polling continues normally.
use native popup dismissal/focus behavior, the existing shadowless raised
surface, cut corners and angular press indication.

**view strip:** preserve one horizontally scrolling, single-selection strip,
`16dp` outer insets, `8dp` gaps and nonoverlapping targets of at least
`48dp × 48dp`. center the smaller visible field within each target:

| property | selected | inactive |
| --- | --- | --- |
| field | `32dp` minimum height; existing `4dp` chip cuts | same geometry |
| surface/border | `RaisedSurface`; `1dp Gold` | transparent; no border |
| text | `Gold` | `Bone` |
| typography/padding | native body `labelLarge` (existing `14sp`); `8dp` horizontal, `6dp` vertical | identical |

use the same weight in both states so selection does not change width. these
heights are minima: allow text to grow the field and target. preserve one-line
ellipsis and current width bound. the outer target owns `selectable`,
`Role.Tab`, complete speech, selected state, bring-into-view and angular press
indication. its inner field fills that width, centers vertically, owns only
paint/text/padding, and clears its semantics; it is not independently clickable.
do not shrink targets or rely on overlapping invisible hit expansion. apply
these styles locally.

**notifications:** `Ready` renders no dashboard notice or settings-button row.
every other health retains its existing message, `NoticeTone.Degraded` and
only `set up notifications`. remove the duplicate settings action and its
unnecessary action `Column`. keep `notificationHealthMessage`, the setup dialog
and its `open notification settings` action unchanged. the existing
[enrollment issue](issues/notification-enrollment-ux.md) remains unresolved.

## contracts, composition and internal api

- `DashboardEntryState` remains the sole view/grid/restoration owner. preserve
  typed keys, ordering, empty/unresolved selected groups, active-view no-op,
  return anchors and existing removed-anchor clamping. schema stays `4`.
- retain selected-tab reveal on selection, resolved label or viewport geometry;
  ordinary polls must not reset manual strip scrolling. exact strip offset
  remains unpersisted. preserve the existing collection-only refresh boundary.
- `DashboardMain` retains transient `machines | details(handle)` selection and
  the existing settings callback. machines/details/back/dismiss/reconnect keep
  their current behavior. disclosure neither filters cards nor prefills forge.
- the menu owns only a remembered expanded boolean; no saved state. dismissal
  precedes destination invocation. settings still uses
  `NotificationOwner.settingsIntent()`; `MainActivity.onResume` retains refresh.
  global actions never inherit session mutation fencing.

reuse the existing menu mechanism through neutral renames, not another popup:

```kotlin
internal class MenuAction(
    val label: String,
    val enabled: Boolean,
    val destructive: Boolean,
    val perform: () -> Unit,
)
// composable signatures; existing behavior and defaults retained
ActionMenuButton(spokenName: String, actions: List<MenuAction>, markCenterY: Dp,
                 modifier: Modifier = Modifier)
ActionRows(actions: List<MenuAction>, dismiss: () -> Unit)
DashboardTopBar(onMachines: () -> Unit, onNotificationSettings: () -> Unit)
```

rename `SessionActions.kt` to `ActionMenu.kt`, `SessionAction` to `MenuAction`,
`SessionActionsButton` to `ActionMenuButton`, `SessionActionRows` to `ActionRows`,
and the private item accordingly. retain `terminalLifetimeActions` as the
session-specific list factory. preserve session action order, captured targets,
live enablement, destructive separator/tone, spoken/custom actions and existing
card mark alignment. the header supplies two enabled, non-destructive actions.

no wire api, persisted schema, lifecycle, controller, notification-owner,
gateway, polling or dependency change. the only generalized concept is the
existing presentation-only menu, now with a second actual caller.

## files, ownership and hard cut

phone paths below are relative to
`android/app/src/main/java/dev/niels/skidbladnir/`.

| owner | exclusive paths/responsibility | dependency |
| --- | --- | --- |
| content/interaction designer | this document's content/presentation review; no production edits | settle content before builders |
| menu builder | `SessionActions.kt` → `ActionMenu.kt`, `SessionCard.kt`, `TerminalScreen.kt`; neutral rename and unchanged session behavior | frozen api above |
| dashboard builder | `DashboardScreen.kt`; header, view strip, notice action; one temporary `android/app/src/androidTest/java/dev/niels/skidbladnir/CompactDashboardProbe.kt` | baseline red before edits; new menu api for green |
| root integrator | docs; temporary changes to `android/app/build.gradle.kts`, `android/app/src/debug/AndroidManifest.xml`; temporary `android/app/src/debug/java/dev/niels/skidbladnir/CompactDashboardProbeActivity.kt` | integrate builders; remove temporary material and restore existing build/manifest content |
| independent verifier | reads diffs/evidence only; writes no tests or production files | challenge each stage before proceeding |

builders may work in parallel after baseline evidence and api agreement; they
never edit one another's paths. run shared build/device operations sequentially.
use a disposable worktree for temporary verification.

delete `dashboardSummary`, its call/parameter/text, the old direct machines
button, healthy settings-button branch and unhealthy duplicate settings action.
remove dead imports/comments and every old menu symbol; retain no aliases,
feature flag, parallel renderer or compatibility path. existing restoration
clamping is intentional behavior, not part of this deletion.

root aligns architecture/roadmap, session views, native notifications, design
language and the current pull-to-refresh composition with this accepted target.
at implementation, update mechanical owner/symbol references in
`docs/codebase-map.md`, `docs/session-card.md`, `docs/terminal-chrome.md`.
historical evidence stays historical. do not edit unrelated issue records.

## red, green, refactor and acceptance

follow [testing policy](rules/testing.md). there is no retained android runner;
temporary instrumentation dependencies/configuration are permitted only for
this proof and must be deleted. no revived gate, general harness, source-text
acceptance test, production test tag, health setter or transport seam.

1. **red:** author two or three focused probes against the unchanged production
   `DashboardMain`/session-card boundary, production theme, real entry state,
   typed dummy machine inventory and real controller without calling `start()`.
   use a disposable application-id suffix retaining `SkidNotificationApplication`
   and the target debug activity above, not the test apk's application. create/
   close the controller on the main thread using target context. its fresh
   notification owner naturally supplies `SetupRequired`; do not mutate private
   health. demonstrate missing overflow, duplicate settings action and overlarge
   header before implementation. retained behavior checks may pass.
2. **green:** implement only the owned presentation changes. rerun the same
   screen-boundary probes. menu symbol renames must not require rewriting the
   baseline assertions. perform the actual native journey below when authorized.
3. **refactor:** independently review ownership, callback capture, semantics,
   deletion and test sensitivity. rerun affected probes after meaningful edits.
   delete probes/activity/fixtures and restore build/manifest changes, including
   runner/dependencies and application-id suffix; then run
   `scripts/check verify android`. builds prove
   engineering correctness only. do not commit temporary tests or evidence data.

| acceptance | required evidence |
| --- | --- |
| compact header and notice | rendered actual dashboard: title/mark, usable overflow, no summary or direct machines/settings buttons; unhealthy diagnostic and one setup action remain |
| menu | exactly two rows in order; dismiss/outside/back; machines opens current sheet; settings intent dispatches once to existing app/channel; popup absent after compose idles |
| shared menu safety | actual session-card menu with inert callbacks: enabled action fires once, disabled action never fires; existing session speech/order/targets retained |
| selection and layout | actual view strip at `320dp`/`360dp`, `1×`/`2×` text and landscape; minimum targets, no overlap/clipping; long unicode and selector-like group labels; empty/unresolved selection and full spoken names |
| continuity | machine disclosure closes with the same view/grid anchor/offset; selected-tab reveal works; inventory updates preserve manual strip browsing |
| native end-to-end | actual app: menu → machines → details → back/dismiss; reconnect entry without submitting; menu → actual channel settings → return; view and surviving semantic anchor remain |
| healthy physical layout | actually enrolled notification `Ready`: two resting rows, about `96dp` at default text before insets/gutter; no settings row; larger text may grow |

measure the default header from the first tab's semantic top relative to the
dashboard root minus its applied system-bar inset: `48dp`, within one pixel;
baseline is `64dp`. target bounds prove hit size, not painted-field size.
inspect the synthetic chrome render for `32dp` field, colors and typography;
no screenshot golden or production tag. review dismiss-before-perform ordering;
popup nodes need not disappear synchronously inside the callback.

intercepting the settings intent proves dispatch, not the native journey or
resume refresh. a synthetic inventory/unstarted controller proves presentation,
not gateway, reconnect or terminal behavior. healthy device acceptance requires
working enrollment; do not fabricate `Ready` or expand this change to repair
transport. inspect remaining health mappings by review; exercise reachable
states without damaging the user's notification setup. terminal-return logic
is unchanged: retain entry-state proof and review its callers. an owner-operated
ordinary terminal-return observation may qualify that existing journey; otherwise
record it `NOT_RUN`. do not build a gateway/fleet harness for this chrome change.

adb/phone, tmux and integration/live operations require explicit approval in
their execution turn under `AGENTS.md`; this plan grants none. any terminal
fixture uses only its own isolated `-L` socket. preserve installed pairings,
notification memory and user work. evidence contains only dummy labels,
geometry and outcomes, never terminal bytes or credentials. unavailable checks
are `NOT_RUN`, never passes; record remaining qualification in `docs/issues/`
if implementation finishes with unresolved acceptance.

## qualification

2026-10-14, from the chrome prerequisite worktree (`016acd0`), with independent
adversarial review:

- **red:** unchanged actual dashboard failed on its `64dp` header, missing
  overflow and duplicate settings action; the installed original also failed
  the overflow check before navigation.
- **screen boundary:** all three temporary probes passed. actual dashboard,
  entry state and session-card rendering covered typed/long/unresolved groups,
  selection/reveal, polling versus manual strip browsing, semantic anchor/offset,
  menu order/back/outside dismissal, single settings dispatch and live action
  fencing. synthetic renders confirmed the `32dp` field, colors and typography.
- **physical api 36 phone:** enrolled healthy chrome measured `48dp + 48dp`.
  `320dp`/`360dp` at `1×`/`2×` text and `780dp` landscape at `2×` passed;
  lower-edge pointer taps confirmed real touch coverage. native selection was
  checked on the semantic owner, separately from description-node geometry.
- **native journeys:** machines/details/back/dismiss, reconnect entry/cancel
  without submission, and actual channel settings/return passed with `all`
  selection and grid geometry retained. exact semantic anchor/offset was proved
  at the screen boundary; the owner also confirmed ordinary terminal return
  retained the same view and scroll position.
- **retirement:** temporary tests/activity, runner, suffix and dependencies were
  removed; `scripts/check verify android` passed afterward. the original phone
  apk and display settings were restored; disposable test apps were removed.

the `96dp` footprint is measured; the total `64dp` saving remains an estimate.
agents issued no tmux commands and changed no enrollment. no behavioral
harness was retained and no production release was made.

## exclusions, costs and references

no new settings page, group mode/picker, counters/badges, density preference,
collapsing header, animation, card redesign, global typography/theme change,
notification enrollment repair, backend work, production deployment or release.
an explicitly approved test installation belongs only to verification.

accepted costs: machines/settings gain a tap and lose immediate discoverability;
permission repair loses its direct banner shortcut; population overview is
removed; groups may remain offscreen horizontally; proportional labels lose
some terminal texture; `48dp` targets limit height reduction; large text/failure
notices can remain tall. shared-menu renames touch existing consumers without
changing their behavior. deleting temporary tests leaves no retained regression
protection. these costs buy more room for work with unchanged authority.

platform references: [touch targets](https://developer.android.com/develop/ui/compose/accessibility/api-defaults),
[native compose menus](https://developer.android.com/develop/ui/compose/components/menu),
[maximum text scaling](https://developer.android.com/about/versions/14/features#non-linear-font-scaling).
