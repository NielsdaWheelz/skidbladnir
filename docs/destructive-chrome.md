# Design delta D6: destructive and notice chrome

Status: implemented and verified 2026-08-27 on `nidavellir/destructive-chrome`.
Routine verification green — `compileDebugKotlin`,
`compileDebugAndroidTestKotlin`, `lintDebug`, and 45 JVM unit tests, 0
failures. The instrumented suite is green on the physical S22+: `OK (35
tests)`, debug-signed over adb. Both rendered proofs pass in isolation, and
the cleft proof was mutation-checked — flattening `Cleft` to a symmetric 4dp
chip fails it with `expected:<1> but was:<0>`, so it is not vacuous.
Adversarial review found and fixed four defects before that run: a dropped
`Role.Button`, two sub-floor contrast values, and an `EmptyState` severity
contradiction (F13–F15 below).
The two hands-on S22+ judgements below stay `NOT_RUN` — they need eyes on the
panel, not a runner. Note the signed `deviceDebug` platform gate remains
MacBook-owned, and this run replaced the phone's pinned-signed install with a
devbox debug-signed one, so `scripts/test platform` will refuse until a signed
build is reinstalled from the MacBook.

[`architecture.md`](architecture.md) owns product behavior and acceptance —
including product language and the visibly-distinct detach/kill guarantee;
[`design-language.md`](design-language.md) owns the visual values (§5 color,
§6 shape, §12 motion, §15 the forbidden list); this document owns the
severity model, the shared chrome primitives, and the delivery plan. Testing
standard is [`rules/testing.md`](rules/testing.md).

Numbered D6 because a Forge-seal delta already claims D5 in flight on its own
branch (its spec is not a file here, so it is named rather than linked).
**Sequencing: independent of both in-flight deltas.** The pull-to-refresh
delta deletes the header `Refresh` button and the Forge seal moves the create
affordance out of the header; neither is a notice or destructive surface, so
the only interaction is textual conflict in `DashboardScreen.kt`. Any order
works.

## Outcome

Ember stops meaning five things. One owned severity type routes every
failure, degradation, and armed-recovery surface; staleness moves to Muted;
the kill control becomes the only asymmetric shape in the product. Three
hand-rolled banner constructions collapse into one panel and two
hand-rolled kill buttons collapse into one control.

No wire, gateway, controller, tmux, or transport change. No copy change.

## The defect

Ember carries five unrelated meanings today: destructive control, failed
attempt, stale data, host load, corrupt record. Job three is the disease —
in normal two-machine operation one host is routinely stale, so Ember is the
dashboard's resting state and no longer reads as an alarm. The file already
contradicts itself: stale *pressure* renders Muted (`DashboardScreen.kt`
`pressureStateColor`) while stale *inventory* renders Ember (the `STALE`
chip and the card's `STALE · actions disabled` line).

The structural cause: `machineStateMessage` switches on the seven-variant
`MachineAvailability`, but `machineStateMessageColor` switches on the
three-variant `machine.access`. One derivation, two switches, different
granularity — and the coarse one inverts severity, painting "pressure is
STALE, sessions remain current" in the alarm color while "identity changed,
provisioning repair is required" gets the calm armed accent.

## Goals and rules

- **One owner for severity.** Every notice, error, degraded marker, and
  destructive control reads its color from `noticeToneColor`. After this
  delta the bare `Ember` token survives in exactly two files: `Theme.kt`
  (its definition and `noticeToneColor`) and `DashboardScreen.kt`
  (`pressureColor` alone, where design language §5 assigns Ember to `HOT` —
  host load is a separate axis with its own owner).
  `MaterialTheme.colorScheme.error` is never read by app code (the scheme
  slot stays for M3-internal use such as text-field error state).
- **Staleness is absence, not failure.** Missing knowledge renders Muted,
  matching `UNKNOWN → Muted` (design language §5) and the honesty law
  (§1.4): absence is displayed, not alarmed. Trust events — auth required,
  identity changed — are the loud ones.
- **Geometry carries the destructive signal, not a picture.** No icon, no
  glyph, no rune, no figure. §15's ban on axe/hammer/helm clip-art and §7's
  "no ornament near destructive surfaces" are both upheld unchanged; the
  signal is a deeper cut in the existing `CutCornerShape` grammar.
- **Copy stays per-surface; tone gets one owner.** Four functions already
  switch on `MachineAvailability` to produce genuinely different prose for
  different surfaces (banner, wait copy, choice-label suffix, Forge
  disablement). They are not merged — merging them would flatten real
  content differences. Only the *tone* derivation is consolidated.
- **Product language is untouched.** `Dwarves` is deliberate product
  language (`architecture.md` §Product language, PR #10) and is not
  reopened. This delta changes zero strings. `architecture.md`'s rule that
  destructive language "stays literal and names the tmux session when
  relevant" is what `killActionLabel` below exists to satisfy.
- Behavior is otherwise untouched: every controller call, semantics
  property, traversal order, touch target, dialog flow, and testTag not
  named below is byte-identical.

## Capability contract

### Tokens (`Theme.kt`)

```text
NidavellirShapes.Cleft = CutCornerShape(
    topStart = 14.dp, topEnd = 4.dp, bottomEnd = 4.dp, bottomStart = 4.dp)
  // The only asymmetric shape in the product. Chip facet is 4dp; the cleft
  // corner is 14dp — material cleaved off. Kill controls only.
  // Named "Cleft", not "Struck": the seal vocabulary reserves
  // struck/unstruck for minted-vs-blank, and one word cannot mean both.

internal enum class NoticeTone { Failure, Degraded, Armed }

internal fun noticeToneColor(tone: NoticeTone): Color = when (tone) {
    Failure  -> Ember    // an attempt failed, trust broke, or you are ending something
    Degraded -> Muted    // knowledge is absent or old; nothing is broken
    Armed    -> Gold     // a recovery is waiting on you
}
```

### Derivations (`ProductModel.kt`)

`MachineAvailability`, `machineAvailability`, and `machineStateTag` move
here from `DashboardScreen.kt` — they are pure product derivations and the
Compose file is not their owner. `machineStateMessage` and
`machineStateMessageColor` are replaced by one function returning both:

```text
internal data class MachineNotice(val message: String, val tone: NoticeTone)

internal fun machineNotice(machine: MachineState): MachineNotice?
  // ONE exhaustive `when` over MachineAvailability. Messages verbatim from
  // today's machineStateMessage — no copy change.
  Ready + fresh pressure          -> null
  Ready + stale/unavailable press -> Degraded
  Reading | Refreshing            -> Degraded
  Stale | Unavailable             -> Degraded
  AuthRequired | IdentityChanged  -> Failure
```

`Unavailable` is Degraded deliberately: `architecture.md` treats one host
outage as normal federated operation, and the message still names it
literally. Only the alarm color is withdrawn.

Destructive copy gains one owner so the button description and the dialog
title cannot drift. Both strings are byte-identical to today's:

```text
internal fun killActionLabel(label, target)       = "Kill <tmuxName> on <machine>"
internal fun killConfirmationTitle(label, target) = killActionLabel(...) + "?"
```

### Chrome primitives (new file `Chrome.kt`)

```text
@Composable NoticePanel(
    tone: NoticeTone, body: String,
    title: String? = null, actions: (@Composable RowScope.() -> Unit)? = null)
  // Surface: tone color at 12% fill, 1dp tone hairline, NidavellirShapes.Card,
  // 12dp padding. `title` (when present) is Bone titleSmall/SemiBold; `body`
  // is bodyMedium in the tone color. Both optional params have a real
  // consumer today (rules/simplicity).
  // NO `modifier` param, deliberately: all three consumers applied the same
  // fillMaxWidth().padding(horizontal 16, vertical 4), so the panel owns that
  // geometry — it was the duplication being removed — and rules/simplicity
  // forbids a parameter no call site varies. EmptyState and
  // UnreadableMachineStrip already set that precedent in this codebase.

@Composable KillButton(
    machineLabel: MachineLabel, target: AgentTarget,
    enabled: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier)
  // Surface: Ember at 12% fill, 1dp Ember hairline, NidavellirShapes.Cleft,
  // label "Kill" in the body face (it is a control, not a machine fact —
  // design language §9), Ember content, minimumInteractiveComponentSize()
  // on the INNER box so the cut background fills the node rather than being
  // centred inside a larger touch target.
  // 12% and not the status chip's 18%: Ember on an 18% Ember fill over a card
  // measures 4.43:1, under §14's 4.5 floor and below the 5.62:1 of the plain
  // TextButton it replaces. 12% holds 4.85:1.
  // role = Role.Button — the TextButton carried it, a hand-built Surface does
  // not, and without it the app's only destructive control announces to
  // TalkBack as a generic view rather than a button.
  // contentDescription = killActionLabel(...) so a grid of kill buttons is
  // distinguishable to a screen reader; today they all speak a bare "Kill".
  // Disabled: the hairline is dropped (the non-opacity cue §12 requires) and
  // the label goes to Bone at 38%, NOT a dimmed Ember — Ember at 38% over an
  // Ember-tinted ground is 1.82:1, where Bone holds 3.21:1, matching the
  // legibility the TextButton had. The hue change is a second non-opacity cue.
  // Press: AngularIndication(Cleft).
```

### `AngularIndication` is generalized

`AngularIndication` is a singleton hardcoding `NidavellirShapes.Card`, and
its own comment states that a second consumer with another shape reopens
`chrome-tokens.md`. This delta reopens it: the platform ripple is circular
and circular ripples are forbidden (§15), so the kill control has no legal
alternative.

```text
internal data class AngularIndication(val shape: Shape) : IndicationNodeFactory
  // Structural equals/hashCode replace the identity-based pair, so
  // Modifier.clickable(indication = …) does not churn across recompositions.
  // The node draws the inset copy of its own `shape`, not the Card outline.
```

Consumers: `AgentCard` passes `NidavellirShapes.Card`; `KillButton` passes
`NidavellirShapes.Cleft`. `chrome-tokens.md`'s card-only note is superseded
and edited by this delta.

### `MachinePressureStrip` signature

`supportingMessage` and `supportingMessageColor` become one nullable
`supporting: MachineNotice?` — a message without a tone and a tone without
a message are both illegal states and stop being representable. Its default
(`= Ember`) is deleted: production always passed it explicitly and the
default existed only for a test call site (rules/simplicity).
`inventoryStale` also loses its test-only default. The three modifier
params keep theirs per Compose convention.

## Surface disposition

| Site | Today | After |
| --- | --- | --- |
| `DashboardScreen` notice banner | `Surface(error 16%)` + Text | `NoticePanel(Failure, body)` |
| `DashboardScreen` forge-recovery banner | `Surface(Gold 16%)` + Column + 2 buttons | `NoticePanel(Armed, body, actions)` |
| `UnreadableMachineStrip` | `Surface(error 16%)` + title + body | `NoticePanel(Failure, title, body)` |
| `MachinePressureStrip` supporting text | `supportingMessageColor` | `noticeToneColor(supporting.tone)` |
| `MachinePressureStrip` `STALE` marker | Ember | Degraded |
| Card `STALE/REFRESHING · actions disabled` | Ember | `availabilityTone(...)` — see F1 |
| Card kill | `TextButton(Ember)` | `KillButton` |
| Terminal kill | `TextButton(Ember)` | `KillButton` |
| Kill confirm button | `Button(Ember, Card shape)` | `shape = Cleft` |
| Forge unavailable copy | Ember | `noticeToneColor(availabilityTone(...))` |
| Empty state, unreadable credentials | Muted | `NoticeTone.Failure` — see F13 |
| Empty state, inventory wait copy | Muted | `availabilityTone(...)`, Failure dominating — see F13 |
| Forge error text | `colorScheme.error` | `noticeToneColor(Failure)` |
| Reconnect panel message | Ember | `noticeToneColor(Failure)` |
| Bearer-repair error | `colorScheme.error` | `noticeToneColor(Failure)` |
| `pressureColor(Hot)` | Ember | **unchanged** — design language §5 assigns it |
| `terminalPresenceColor(ReconnectRequired)` | Ember | **unchanged** — a failure state, not staleness |

The kill dialog gains no decoration; §7 holds. The Forge's error text stays
plain body copy rather than a nested `NoticePanel` (§13) — only its color
owner changes.

## Content design

This delta ships no new strings, so content work is a review, not an
authoring pass. Each feature gets a content designer who first defines the
quality bar within the schema, then audits the existing set against it.
Rubrics ship as a checked-in addendum to this section. Findings that
require string changes are logged, not applied — copy changes are a
separate delta under `architecture.md`'s product-language ownership.

| Feature | Schema | Owner | Deliverable |
| --- | --- | --- | --- |
| Machine notices | `MachineNotice.message`, one per `MachineAvailability`, across banner / wait copy / choice-label suffix / Forge disablement | Content designer A | Rubric for a good machine notice; audit of the four existing sets for contradiction and drift |
| Destructive copy | `killActionLabel`, `killConfirmationTitle`, the button word, the three dialog bodies | Content designer B | Rubric for destructive copy; confirmation the shipped strings meet it, including the new screen-reader description |
| Degraded markers | The `STALE` / `REFRESHING` mono-caps tokens and the `· actions disabled` suffix | Content designer A | Rubric for degraded markers; ruling on whether shouty caps still earn their place once the color is Muted |

Hard constraints none of them may move: literal register, no themed nouns
in lifecycle/error/destructive copy, every string names its machine, and no
string may imply a state tmux did not report.

### Addendum: rulings and findings log

Both reviews ran. Rubrics are in full at `content-designer-a.md` and
`content-designer-b.md` in the implementing session's scratch; the rulings and
the findings that survived are recorded here, because a finding that lives
only in a transcript is a finding that will be rediscovered.

**Ruling — degraded markers keep their caps; prose loses them.** The caps were
never the alarm; Ember was. `NidavellirType.Data` caps is this product's
register for a machine-emitted state token — the same face prints `READING`,
`NORMAL`, `HOT` — so caps means "token, scan me", not "be alarmed". Withdrawing
Ember demotes `STALE` and `REFRESHING` into exactly that register, beside
`NORMAL`, which is this delta's thesis expressed in type. What the colour
change *does* orphan is shouted `STALE` inside a sentence: in prose caps is
emphasis, emphasis is a tone claim, and the tone is now Muted, so the sentence
whispers while one word screams.

**Ruling — WCAG 2.5.3 passes in its recommended form.** The visible label is
`Kill`; the accessible name is `Kill ga-durinn on MacBook`, which does not
merely *contain* the visible label but *begins* with it. "Tap Kill" still
matches; a grid of them triggers the ordinary numbered-disambiguation flow,
which is a multi-match, not a mismatch failure.

**Ruling — the word survives; ship no picture.** `Kill` is `tmux kill-session`'s
own verb and §4 demands the literal register. Every softer synonym either lies
about severity or bleeds into detach's semantic field, eroding the very
guarantee `Cleft` exists to carry. An icon is triply barred: the app ships
none, §15 bans the obvious candidates, and an icon-only control would need an
invented accessible name — reinstating the exact label/name gap this delta
closes. An icon *beside* the word is ornament on a destructive surface. The
result is the right redundancy: the same signal in two independent channels,
lexical and geometric, one of which survives greyscale and the other of which
survives screen-reader linearization.

**Findings.** One was promoted into this delta because it is a tone, not a
string; the rest are logged for a copy delta under `architecture.md`'s
product-language ownership and are deliberately **not** applied here.

| # | Sev | Finding | Disposition |
| --- | --- | --- | --- |
| F1 | High | `AgentCard` picks its marker from `!machine.canMutate`, whose else-branch prints `STALE · actions disabled`. But `canMutate` is false for `AuthRequired`/`IdentityChanged` on a **Fresh** inventory, so a machine whose credentials broke claims its sessions are stale — a state tmux never reported. Ember hid this; painting it Degraded would have made a trust failure calm, inverting the delta's own thesis. | **Tone half FIXED here** via `availabilityTone`, proved in `MultiMachineContractTest`. The wrong *word* is a string and stays logged. |
| F3 | High | ` · RE-PAIR` was the only imperative in a vocabulary of state tokens, and named a remedy that no longer exists. | Resolved as ` · IDENTITY CHANGED`; sibling surfaces require fleet reset outside the app |
| F4 | High | `$label needs an updated bearer.` leaks mechanism jargon into user copy; the other three surfaces call this state "authentication required". | Logged |
| F2 | High | `Prior sessions are STALE` attributes to tmux something it never said — the read failed; the sessions may be fine. Our knowledge aged, not the host's sessions. | Logged |
| Fb | Med | The kill dialog contradicts itself within one glance: the resting body says "the **confirmed** tmux lifetime", the pending body says "the **exact** tmux lifetime". After the user has confirmed, "confirmed" is the truthful anchor. | Logged |
| F5 | Med | `its sessions may be out of date` hedges a probability tmux did not report, and register-inverts severity: the trust failure hedges softly while mere staleness shouts. | Logged |
| F10 | Med | `DEVBOX UNAVAILABLE` on the pressure strip reads as a machine outage when only the pressure read failed and sessions remain current; adjacency implies the wrong subject. | Logged → `PRESSURE UNAVAILABLE` |
| F13 | High | `EmptyState` hardcoded its body to Muted, so the inventory wait copy and the unreadable-credentials state rendered every severity as Degraded. With one machine's bearer expired and no cached inventory, the strip said *"Devbox: authentication required"* in Ember while the empty state directly beneath said the same thing in Muted — one screen, one fact, two severities. | **FIXED here**: `EmptyState` takes a tone, `dashboardInventoryWaitCopy` returns a `MachineNotice` whose tone is Failure if any machine's is. |
| F14 | High | `KillButton` dropped `Role.Button`. The `TextButton`s it replaces got the role from M3; a hand-built `Surface` does not, so the app's only destructive control announced to TalkBack as a generic view and reported `android.view.View` instead of `android.widget.Button` to Accessibility Scanner. | **FIXED here** and pinned by an instrumented assertion. |
| F15 | High | The kill chip shipped an 18% fill (the status-chip register), which puts the Ember label at 4.43:1 over a card — under §14's 4.5 floor, and below the 5.62:1 of the `TextButton` it replaces. Its disabled label, Ember at 38% over an Ember-tinted ground, measured 1.82:1. | **FIXED here**: fill back to the specified 12% (4.85:1); disabled label to Bone at 38% (3.21:1, matching the old control). |
| F7, F11, F12 | Low | Marker vocabulary leaked into prose with three different subjects for one state; three grammatical forms of one consequence clause; telegraphese amid sentence-form neighbours. | Logged |

## Hard cut

- `machineStateMessage` and `machineStateMessageColor` are deleted, not
  wrapped. No function switching on `machine.access` for presentation
  remains.
- The three hand-rolled banner `Surface` constructions are deleted at their
  call sites. No second banner path exists.
- The two hand-rolled kill `TextButton`s are deleted. `KillButton` is the
  only destructive control.
- `AngularIndication`'s singleton form is replaced, not kept alongside the
  parameterized one.
- No tone flag, no old/new toggle, no compatibility alias.

## Work split

Boundaries are file-exclusive. D depends on A, B, and C.

| Slice | Owner | Paths | Owned proof |
| --- | --- | --- | --- |
| A. Tokens | Compose UI builder | `Theme.kt`, `ThemeTest.kt` | Red 1 |
| B. Derivations | Product-model builder | `ProductModel.kt`, `MultiMachineContractTest.kt`, `ProductContractTest.kt` | Red 2, 3 |
| C. Primitives | Compose UI builder | `Chrome.kt` (new), `AngularIndication.kt`, `DashboardChromeInstrumentedTest.kt` | Red 4, 5 |
| D. Adoption | Compose UI builder | `DashboardScreen.kt`, `TerminalScreen.kt`, `MainActivity.kt`, `MultiMachineUiInstrumentedTest.kt` | Red 6 |
| E. Docs | Root integrator | `design-language.md` (§5 tone contract, §17 delta list), `chrome-tokens.md` (supersede the card-only note), `roadmap.md` (D6 section + status row) | Review only |
| F. Verification | Read-only verifier | none | Review only |

No `scripts/test`, gateway, Go, asset, or `architecture.md` change. No new
static gate: tone containment is a type-level consequence of the two JVM
proofs below, which is strictly stronger than a grep allowlist.

## Red / green / refactor

Red (each observed failing first). One proof per boundary; the single
rendered proof covers the one thing only rendering can show.

1. **JVM** (`ThemeTest`): `noticeToneColor` is injective across all three
   tones, and `Degraded` is not `Ember`. Fails today — no such function.
2. **JVM** (`MultiMachineContractTest`): `machineNotice` is exhaustive over
   `MachineAvailability`; every non-trust degradation is `Degraded` and both
   trust events are `Failure`. Fails today — the color switch reads
   `access` and returns Ember for a `Ready` machine with stale pressure.
3. **JVM** (`ProductContractTest`): `killConfirmationTitle` is
   `killActionLabel` plus `?` for a representative target, and both still
   equal today's literal strings. Fails today — one owner does not exist.
4. **Instrumented** (`DashboardChromeInstrumentedTest`, pixel): the kill
   control is asymmetric — its top-start corner region reads the background
   while its bottom-start corner region reads the Ember fill. This is the
   `architecture.md` "detach and kill are visibly different" guarantee made
   observable, and it survives greyscale. Fails today — the control is a
   bare text button with no shape at all. (Pixel capture under a paused
   `mainClock` needs one `advanceTimeByFrame()` first.)
5. **Instrumented** (same suite): a disabled `KillButton` drops its hairline
   and exposes no click action; an enabled one exposes one and speaks
   `killActionLabel` rather than a bare "Kill".
6. **Instrumented** (`MultiMachineUiInstrumentedTest`): each `NoticePanel`
   consumer — gateway notice, unreadable pairing, forge recovery — still
   renders its literal copy and its recovery actions after the collapse.
   Fails today — the panel does not exist.

Green: implement only enough to satisfy those six plus the disposition
table. Routine `scripts/test verify` stays green and device-free.

Refactor: fold any remaining color literal in the three screen files into
`noticeToneColor`/`pressureColor`; delete symbols left unreferenced by the
move to `ProductModel.kt`; add no abstraction with only a speculative
second consumer.

## Acceptance and gates

Routine verification, plus one separately approved platform pass — the
existing instrumented suites re-run green, plus reds 4–6. One hands-on S22+
look, on the same pass:

- The kill chip reads as *cleaved* at arm's length, not as a rendering
  defect or a clipped label.
- A stale machine no longer reads as an error at a glance; an auth-required
  machine still does.
- Muted degraded text stays comfortable on Ink at night brightness (design
  language §18 records the WCAG-near-black caveat).

No integration or live gate. This delta touches no external boundary.

## Non-goals

Icons of any kind — the app ships none and gains none here. Any string
change, including the `Dwarves` product language. Seals (D3), the Forge
seal (D5), ornament (D4), the terminal ANSI table (D1), the forge-heat ramp
and any continuous meter, light theme, dynamic color, M3-internal ripple
unification beyond `AngularIndication`'s two consumers, merging the four
`MachineAvailability` copy functions, `MachinePressureStrip`'s remaining
modifier defaults, pull-to-refresh, push (v0.5), and any wire, gateway, or
tmux work.
