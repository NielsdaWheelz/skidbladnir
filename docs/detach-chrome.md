# Detach chrome

Status: implemented; the focused red was observed on the physical S22+, routine
verification, the 47-test S22+ platform gate, and the hands-on header glance
are green.

[`architecture.md`](architecture.md) owns lifecycle behavior and acceptance;
[`design-language.md`](design-language.md) owns visual identity; this document
owns the Android detach-control content, component contract, delivery boundary,
and red/green proof shape. [`roadmap.md`](roadmap.md) owns ordering and
[`rules/testing.md`](rules/testing.md) owns test standards.

## Outcome and scope

Replace the terminal header's stock long `TextButton` with one purpose-built
Niðavellir `DetachButton`. Its entire visible and spoken content is `Detach`.
It remains the top-leading, always-available phone-detach action; Android Back
remains equivalent. Kill remains top-trailing, confirmed, Ember, and `Cleft`.

This is a hard-cut Android chrome change. No controller, connection, WebSocket,
gateway, tmux, terminal-input, state, storage, or wire contract changes.

## Goals and rules

- Read as a quiet, reversible lifecycle action and as native Skíðblaðnir chrome.
- Free header width while keeping machine and tmux-session identity visible.
- Stay visibly distinct from Kill in colour and greyscale.
- Reuse `DeepSurface`, `Gold`, `NidavellirShapes.Chip`,
  `AngularIndication`, and `minimumInteractiveComponentSize()`.
- Keep one component, one label, one callback, and one code path.
- Preserve at least a `48dp` target and `8dp` separation from adjacent actions.

Content designer contract:

| Field | Required value | Good content |
| --- | --- | --- |
| Visible and spoken label | `Detach` | Literal, sentence-case action verb; complete without help text |

The visible `Text` supplies accessibility content. Add no custom
`contentDescription`, tooltip, subtitle, destination, lifetime promise, icon,
ornament, confirmation, snackbar, or haptic. This table is the content schema;
do not create a runtime content type, label helper, or string parameter for one
literal.

## Structure and composition

```text
TerminalScreen
  -> DetachButton(onClick = controller::detachToAgents)
       -> existing controller -> existing TerminalConnection.detach
  -> machine + tmux-session identity
  -> existing KillButton
```

`DetachButton` belongs in `Chrome.kt` beside `KillButton`: both are owned
terminal chrome, but they remain separate primitives. A generic action-button
abstraction would need shape, tone, enablement, semantics, and border switches
for only two semantically opposed consumers; do not create it.

The call site owns placement and the controller callback. The component owns
presentation, target size, button role, and pressed indication. No new system,
subsystem, schema, state, service, or transport API exists.

## Capability contract

```kotlin
@Composable
internal fun DetachButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
)
```

- `Surface`: `DeepSurface`; `NidavellirShapes.Chip`; `1dp` Gold-at-40% hairline.
- Content: `Detach`, Gold, `labelLarge`, system UI body face, `12dp` horizontal
  padding; never Data or Display type.
- Interaction: `clickable(role = Role.Button, indication =
  AngularIndication(NidavellirShapes.Chip))`; no disabled state.
- Geometry: symmetric four-corner chip; Kill alone remains asymmetric `Cleft`.
- API: no `enabled`, label, icon, shape, colour, content slot, or behavior flag.

## Hard cut and cleanup

Delete in the same change:

- the stock terminal-header `TextButton` and orphaned import;
- `terminalDetachActionLabel()`;
- its pure literal-string unit test;
- the unused `terminal-detach` test tag;
- every visible `Detach · session keeps running` source/test assertion.

Retain the key-deck proof that `Detach` is absent from terminal-input controls.
Do not keep an old/new component, alias, fallback, feature flag, compatibility
copy, or alternate in-app detach affordance. Android Back remains the named
platform exception.

## Red / green / refactor

Red first, owned by the Compose builder: after a green routine baseline, one
real-Compose proof renders the actual `TerminalScreen` in its non-networked
`Verifying` state and fails against current source. It asserts the user-visible
`Detach` label, machine/session header text, `Role.Button`, a click action,
minimum target, symmetric cut geometry, and absence of the retired lifetime
copy. Query by text/role; add no test tag or internal mock. Running this
instrumented red requires separately approved S22+ access in that future turn.

Green: implement only the component, adopt it in the existing header, and
delete the retired path. Existing controller, protocol, Back, reconnect, and
Kill behavior remain byte-identical in intent.

Refactor: remove orphaned symbols/imports/tests. Reuse existing tokens and
indication directly; extract nothing else without a present second consumer.

## Acceptance and 80/20 verification

1. The header shows exactly one top `Detach` control and no lifetime subtitle,
   mark, glyph, or old label.
2. Detach is symmetric, neutral/Gold, and unmistakable from Ember/Cleft Kill in
   greyscale; Kill still confirms.
3. The control is at least `48dp`, has `Role.Button`, speaks `Detach`, and has
   no overlapping target.
4. Machine and tmux-session identity remain readable in the portrait header.
5. Tapping Detach and Android Back retain the existing phone-only detach
   lifecycle; source work survives and no input replays.

Proof shape: one focused Compose proof for the changed UI ownership boundary;
routine `scripts/test verify` for static/build/unit regression; one separately
approved S22+ platform pass plus a hands-on header glance for rendered geometry,
spacing, legibility, and Detach/Kill distinction. Existing real-tmux tests own
unchanged lifecycle behavior and are not duplicated or rerun for this visual
slice. Integration, live, and product gates remain `NOT_RUN` unless separately
approved in their own turn.

## File ownership and work split

| Slice | Owner | Exclusive paths | Proof |
| --- | --- | --- | --- |
| Contract/content | Product + content designer, root integrator | `docs/detach-chrome.md`, detach wording in `docs/architecture.md`, `docs/roadmap.md`, `docs/design-language.md`, `docs/terminal-key-deck.md`, `docs/hlidskjalf-mark.md` | Review; exact `Detach` schema approved before code |
| Chrome + adoption | Compose builder | `android/app/src/main/java/dev/niels/skidbladnir/Chrome.kt`, `android/app/src/main/java/dev/niels/skidbladnir/TerminalScreen.kt`, `android/app/src/test/java/dev/niels/skidbladnir/ProductContractTest.kt`, new `android/app/src/androidTest/java/dev/niels/skidbladnir/TerminalChromeInstrumentedTest.kt` | Owns and observes the focused red before production edits |
| Verification | Read-only verifier | none | Reviews diff, routine result, and honestly reports external gates |

No two writers share a file. Only the root integrator changes canonical docs;
the verifier writes neither tests nor production files.

## Non-goals and final state

No `x`, icon, Hlíðskjálf mark, `Back to Dwarves`, supporting copy, custom
accessibility prose, added ambient/content animation beyond the required
pressed-state indication, haptic, confirmation, undo, gesture, menu, setting,
localization framework, analytics, new dependency, controller rename, protocol
change, gateway/tmux work, or generalized button system.

Final state: `DetachButton` is the sole top detach control and `Detach` is its
sole user-facing content. The long label, helper, unit test, test tag, and stock
button path do not exist.
