# Machine pressure rail

Status: the typographic rail source is implemented; its pure and real-Compose
reds, focused unit, signed same-version S22+ component, routine verification,
and approved hands-on visual glance are green. The All-filter density follow-up
is source implemented with its pure red/green and routine verification green;
its signed same-version S22+ component proof is green at 360dp/1× and
320dp/2×, including exact-machine disclosure and one-Back dismissal. The
complete 54-test release-bound platform gate is green. Product and current
hands-on acceptance remain `NOT_RUN`.

This is the implementation contract for one hard-cut presentation refactor.
It is subordinate to [`architecture.md`](architecture.md),
[`roadmap.md`](roadmap.md), [`design-language.md`](design-language.md), and
[`rules/testing.md`](rules/testing.md). Before Android work, the root integrator
must reconcile those documents with this approved target. No parallel
`testing-standards.md` exists or should be created.

## Outcome

Replace the five status-coloured metric gems with one flat typographic metric
row. Preserve the machine verdict, whole-rail disclosure, details sheet, and
history renderer. The rail becomes quieter and materially shorter without
changing pressure meaning or any external capability.

```text
MACBOOK  RECOVERING FROM HOT · LOAD
CPU 34% i   MEM WARNING W   SWAP NO DATA ?   LOAD 1.3 W   DISK 61% N
[ existing pressure history band ]
```

The row is presentation only. It is not a database, event ledger, threshold
owner, alert, or second pressure model.

### All-filter density follow-up

The reviewed 2026-08-28 follow-up supersedes only the rail-placement clauses
below: `All` renders no pressure rail, while an explicit machine filter renders
exactly that machine's rail and details disclosure. Compact exceptional machine
notices remain in `All`; pressure polling, accepted snapshots, presentation,
history, disclosure, and action admission remain unchanged.

The details surface deliberately has no partially expanded resting state. That
forgoes an intermediate sheet height so one system Back consistently dismisses
the modal details and returns focus to the originating rail.

## Scope and closed decisions

Own only:

- collapsed-rail header colouring, metric structure, precision, spacing, and
  height;
- the rail's pure presentation schema and focused Android proofs;
- removal of gem-only production/test/documentation paths; and
- canonical documentation reconciliation before implementation.

No blocking question remains. These decisions are closed:

- Retain values; labels alone are insufficient evidence.
- Do not map raw values onto a continuous colour ramp.
- Labels are neutral. Normal and informational measurements are quiet. Gold and
  Ember identify host-evaluated `Warm` and `Hot` exceptions.
- Retain literal `i | N | W | H | ?` marks so colour is redundant.
- Keep one non-wrapping row in `CPU`, `MEM`, `SWAP`, `LOAD`, `DISK` order.
- Keep the outer rail as the single disclosure surface and interaction target.
- Do not edit or redraw `PressureHistoryBand`.
- Keep the details sheet and its fuller evidence unchanged.

Changing one of these decisions reopens scope and acceptance review.

## Goals and rules

- Answer “which machine warrants inspection?” before showing diagnostic detail.
- Separate measurement from judgment: the value is an observation; the
  host-emitted state is the judgment.
- Spend colour on active reading and exceptions, not metric identity or resting
  reassurance.
- Preserve stable order, honest absence, platform capability differences, and
  detail on demand.
- Reuse `RaisedSurface`, `Bone`, `Muted`, `Gold`, `Ember`,
  `NidavellirType.Data`, `AngularIndication`, and the existing rail/sheet.
- Add no token, dependency, setting, breakpoint, animation, or generic metric
  component.
- Keep visible text at least 11sp, small-text contrast at least 4.5:1, and the
  rail target at least 48dp.

## Target behaviour

### Structure

With an accepted pressure snapshot, one rail has exactly three strata:

1. one-line machine/verdict/cause/freshness header;
2. one horizontally scrollable, non-wrapping typographic metric row; and
3. the existing 16dp pressure-history band.

`Reading` and `Unavailable` have no accepted snapshot and therefore render only
the same at-least-48dp disclosure surface and header. They do not fabricate a
metric row or history band.

At 360dp width, font scale 1.0, fresh data, and no adjacent notice, rail height
is 68-76dp. At larger font scales the rail grows rather than clipping. The
metric row scrolls; it never wraps, reorders, shrinks text, or omits supported
evidence.

Keep 12dp horizontal and 6dp vertical rail padding, 4dp between strata, and
12dp between metric groups. The metric row has no artificial minimum height.
Dashboard-owned outer padding remains unchanged.

### Header

The visible header is `<machine> <status>`. Render it as one ellipsized Data-face
line with two spans:

- machine: `Bone`, bold, always neutral;
- status: medium weight; Muted for `NORMAL`, `UNKNOWN`, stale, and unavailable;
  Gold for reading, warm, and recovering-from-warm; Ember for hot and
  recovering-from-hot.

Keep the current copy and precedence exactly:

```text
<MACHINE> NORMAL
<MACHINE> WARM · <CAUSE>[ +N]
<MACHINE> HOT · <CAUSE>[ +N]
<MACHINE> RECOVERING FROM <WARM|HOT> · <CAUSE>[ +N]
<MACHINE> PRESSURE STALE · LAST <LEVEL>
<MACHINE> READING
<MACHINE> PRESSURE UNAVAILABLE
<MACHINE> UNKNOWN · <METRIC> NO DATA[ +N]
```

Truncation may shorten visible copy; every untruncated header fact remains in
the rail's merged accessibility summary. The spoken summary may use equivalent
prose and need not repeat the visible header byte-for-byte.

### Metric row

Each metric is one unbroken Data-face text group:

```text
<LABEL> <VALUE> <MARK>
```

- Label: Muted, medium weight.
- `Informational` / `Normal`: medium-weight value Bone; medium-weight `i` / `N`
  mark Muted.
- `Warm`: medium-weight value and `W` Gold.
- `Hot`: medium-weight value and `H` Ember.
- Missing supported sample: medium-weight `NO DATA ?`, entirely Muted.
- No fill, border, shape, nested surface, separator glyph, pulse, or nested
  action.

Primary order is `CPU`, `MEM`, `SWAP`, `LOAD`, `DISK`. `MEM` is Linux RAM
available or Darwin native memory pressure. A missing supported primary signal
retains its slot; an unsupported signal has no slot. Linux PSI remains
summary/detail-only and never gains a collapsed slot.

Collapsed precision is deliberate semantic zoom:

- numeric percentages: nearest whole percent;
- normalized load: one decimal, with trailing `.0` removed;
- native memory pressure: existing uppercase category; and
- details sheet: existing precision and full directional labels unchanged.

CPU and swap remain `Informational` at every value. Android must never infer
health, interpolate colour, or convert them to `Normal`, `Warm`, or `Hot`.

### History and disclosure

`PressureHistoryBand` is a frozen owner. Its source and observable rendering
remain unchanged: 16dp height, 5dp top padding, sample width/gap behavior,
ordering, colours, and level heights (`Normal .25`, `Warm .58`, `Hot 1.0`,
`Unknown .42`). It has no visible/spoken title and no duplicate in the sheet.

The whole rail remains one `Role.Button` with action label
`Show <machine> pressure details`. Descendants add no focus or click targets.
The merged summary retains machine, verdict/phase, freshness, causes, every
visible metric value/mark/full state, detail-only missing evidence, and
compressed history without relying on colour.

The existing machine-bound sheet remains local to the accepted snapshot. Open,
dismiss, Back, focus return, machine removal, full rows, state words, reasons,
and `NO DATA` behaviour are unchanged. Disclosure performs no request, retry,
poll, navigation, or mutation.

## Architecture and capability contract

```text
host sampler/classifier
  -> strict gateway GET /v1/pressure
  -> Android decoder/domain PressureState
  -> PressurePresentation
       -> collapsed PressureRailContent -> MachinePressureRail -> Dashboard
       `-> PressureDetailsContent       -> MachinePressureDetailsSheet
```

Ownership is fixed:

| Boundary | Owner | Contract |
| --- | --- | --- |
| Sampling, thresholds, hysteresis, recovery, reasons | Host pressure package | Sole pressure-meaning owner; unchanged |
| Wire projection and strict decoding | Gateway/client boundary | Existing single `GET /v1/pressure` shape; unchanged |
| Text, precision, order, marks, accents, accessibility summary | `PressurePresentation.kt` | Pure, exhaustive, no I/O |
| Rail geometry, spans, scrolling, semantics, disclosure | `MachinePressureRail.kt` | Real Compose presentation |
| Placement and selected machine | `DashboardScreen.kt` | No rail in `All`; exactly the selected machine's rail under an explicit filter |
| History geometry | `PressureHistoryBand` | Frozen source and pixel contract |

The external API remains exactly one strict shape:
`{ unsupported, current, history }`. Existing signal states are
`Informational | Normal | Warm | Hot`; supported absence remains the existing
missing variant. Add no endpoint, field, version, nullable fallback, tolerant
decoder, feature negotiation, or legacy payload path.

Hard-cut the internal presentation schema to:

```text
PressureRailContent(
  header: PressureRailHeaderContent,
  metrics: List<PressureRailMetricContent>,
  historySummary,
  accessibilitySummary,
  actionLabel,
)
PressureRailHeaderContent(machineLabel, statusText, accent)
PressureRailMetricContent(metric, shortLabel, value, stateMark, stateWord, accent)
PressureRailAccent = None | Gold | Ember | Muted
```

`None` is the quiet informational/normal path; it never resolves to Frost or
Moss in the collapsed rail. The details model keeps its current full-state
colour roles. `MachinePressureRail` and `MachinePressureDetailsSheet` keep their
current composable signatures. Do not add a view model, controller state, DTO,
cache, service, or reusable telemetry framework.

## Hard cut and cleanup

Delete in the same implementation change:

- `PressureGemContent`, `PressureRailContent.gems`, `primaryGems`, and
  `gemContent`;
- gem `Surface`/border/fill/chip/padding/min-height rendering;
- `pressure-gems-*` and `pressure-gem-*` selectors and gem-named assertions;
- old 84-92dp acceptance, comments, imports, and documentation; and
- any helper made single-use or dead by the cut.

Keep no alias, overload, deprecated name, old/new branch, fallback layout,
feature flag, compatibility test, or hidden density preference. Rename all
surviving concepts to `metric`/`metrics`. Do not generalize one-consumer text,
accent, row, or telemetry primitives.

## Files and non-overlapping work

The internal presentation schema and its sole Compose consumer change
atomically. Splitting them into sequential green slices would require either a
broken Kotlin compilation unit or a forbidden compatibility path.

| Order | Owner | Exclusive paths | Proof |
| --- | --- | --- | --- |
| 1 | Root integrator, docs only | `docs/machine-pressure-rail.md`, pressure wording in `docs/architecture.md`, `docs/roadmap.md`, `docs/design-language.md`, `docs/chrome-tokens.md` | Canonical target agreement; no runtime claim |
| 2 | Android pressure builder | `android/app/src/main/java/dev/niels/skidbladnir/PressurePresentation.kt`, `android/app/src/main/java/dev/niels/skidbladnir/MachinePressureRail.kt`, `android/app/src/test/java/dev/niels/skidbladnir/PressurePresentationTest.kt`, pressure test in `android/app/src/androidTest/java/dev/niels/skidbladnir/MultiMachineUiInstrumentedTest.kt` | Owns and observes the compile-complete pure and real-Compose reds before the atomic production hard cut |
| 3 | Verifier, read-only | none | Diff/contract/gate review; writes no test or production file |

For the original typographic-row hard cut, no owner changes
`DashboardScreen.kt`, `Theme.kt`, `ProductModel.kt`, `GatewayClient.kt`,
controller/polling code, Go code, build files, `scripts/test`, catalogue files,
tmux code, terminal code, or another owner's tests. The reviewed density
follow-up explicitly reopens only `DashboardScreen.kt`, the pure visibility
rule in `ProductModel.kt`, its existing multi-machine JVM and Compose proofs,
the product-gate confirmation copy in `scripts/test`, and canonical placement
prose. It does not reopen pressure presentation, polling, protocol, gateway,
tmux, terminal, or device-mutation code.

## Red / green / refactor and 80/20 proof

Follow [`rules/testing.md`](rules/testing.md). Establish a green routine
baseline first. Each builder writes and observes its own behavioral red before
editing production in its slice.

One proof per ownership boundary:

1. **Pure presentation unit proof.** Replace, do not duplicate, existing gem
   assertions. One Linux mixed-state fixture and one Darwin capability fixture
   prove exact header parts/accents, stable metric order, collapsed precision,
   `i/N/W/H/?`, quiet versus exception accents, supported-missing versus
   unsupported, detail precision, and colour-independent accessibility copy.
   No I/O or mocks.
2. **Real Compose component proof.** Adapt the existing pressure-rail journey;
   do not add a parallel screenshot suite. Prove one button/no nested actions,
   exact visible metric groups, horizontal access to `DISK` at 320dp/large text,
   68-76dp default height, unclipped header/history, neutral empty space around
   glyphs instead of coloured metric containers, and unchanged details/focus
   behaviour. Query visible text/role first; retain tags only where pixel or
   scroll geometry has no semantic query.
3. **Frozen history proof.** Retain the existing 16dp pixel test byte-for-byte
   in intent: top padding, heights, colours, ordering, and gaps. Do not create a
   second history test.
4. **Filter-placement proof.** One pure rule proves that `All` suppresses the
   rail and an explicit machine filter admits it. The existing Compose journey,
   rather than a parallel test, proves zero rails in `All`, exactly the selected
   machine's rail after filtering, and the unchanged local details interaction.

Red must fail on an observable target assertion, not an unresolved symbol.
Green makes the minimum production change. Refactor then deletes gem names,
orphaned imports/selectors/tests, duplicated formatting, and compatibility
residue; rerun the focused proofs after cleanup.

Verification shape:

- focused `PressurePresentationTest` red/green;
- one separately approved focused instrumentation workflow containing the
  Compose red/green and exact signed-app restoration checks;
- `scripts/test verify` after confirming its composition still executes no tmux
  and no integration/live/platform/ADB boundary; tagged compile checks are
  allowed; and
- one approved-device hands-on glance covering normal, mixed warm/hot, missing,
  Darwin memory, 320dp/large text, horizontal scroll, and greyscale distinction.

The official release-bound `scripts/test platform` gate requires a clean source
SHA matching the installed release pin and is not an uncommitted-red runner.
Do not misreport the focused workflow as that gate. The workflow compares the
encrypted pairing digest immediately before and after instrumentation while the
signed test build owns read access, then proves final test-package absence and
restoration of the captured APK's exact digest, signer, and version. It does not
claim to read the non-debuggable release's private preferences after restore.
No integration, live, or second-phone proof is warranted because their owned
behavior is unchanged. The product journey's confirmation copy consumes the
new placement but does not substitute for the focused component proof. The
complete 54-test release-bound platform gate is green. Product and current
hands-on acceptance remain `NOT_RUN`; `NOT_RUN` is never a pass.

## Acceptance criteria

- An accepted snapshot has one neutral machine identity, one status with
  dynamic-state colour, one flat metric row, and the unchanged history band;
  Reading/Unavailable states are honest header-only disclosures.
- `All` renders no pressure rail. An explicit machine filter renders exactly
  that machine's rail; switching back to `All` removes it. Compact exceptional
  machine notices remain visible in `All`.
- No metric pill, fill, border, nested surface/action, Frost informational wash,
  or Moss normal wash remains.
- A fresh default rail is 68-76dp at 360dp/font scale 1.0; 320dp and large text
  remain usable without clipping or hidden supported evidence.
- Metric order, values, precision, state marks, missing/unsupported distinction,
  and Darwin/Linux capability substitution match this contract.
- Only host-emitted signal state controls Gold/Ember metric accents; Reading
  may use Gold in the header. Android contains no pressure threshold or
  raw-value colour interpolation.
- Rail semantics communicate the same facts without colour and disclosure stays
  machine-bound, local, non-mutating, and focus-correct.
- `PressureHistoryBand` and its pixel proof are unchanged.
- The tree contains no gem production/type/test vocabulary, duplicate proof,
  dead code, compatibility path, or stale canonical pressure prose.
- Exact-head focused and routine results are reported separately from approved
  physical/device evidence and from every `NOT_RUN` gate.

## Non-goals and final state

No new metrics, threshold, sampling cadence, history duration, alert, push,
sparkline, gauge, heat ramp, anomaly model, prediction, recommendation, AI copy,
metric reordering, configurable layout, localization system, analytics, haptic,
animation, external API/wire-schema change, polling change, or details-sheet
redesign.

Final state: one host-owned pressure model and wire format feed one pure Android
presentation owner. An explicit machine filter renders that machine's quiet
typographic pressure rail, frozen history band, and local details sheet; `All`
renders none of those rails. The gem model and every legacy path are absent.
