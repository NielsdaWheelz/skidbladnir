# Frontend

## Scope

ui state, navigation, and data boundaries in the android app, terminal ui, and
embedded terminal page.

## state

- represent local empty state with the language's native optional value; use
  an explicit variant when loading, failure, empty, and ready are different
  states. do not introduce a second absence framework for decoded domain data.
- preserve the meaning of optional facts when moving them from decoded payloads
  into models and views. missing agent identity is not a loading placeholder.
- empty draft text is valid input state. outside draft text, use empty strings
  or numbers as special values only when the owning contract explicitly gives
  them that meaning.
- booleans represent yes/no facts, not a missing current value.
- decode browser and transport values at their boundary; use the resulting
  native types in owned code. see [boundaries.md](boundaries.md).
- prefer derived state over duplicated state.

## Variants

- Keep expected control-flow variants explicit.
- For enum casing and exhaustiveness, follow [naming.md](naming.md) and [control-flow.md](control-flow.md).
- Omission is the default. Do not add `"Default"`-style variants unless they represent real logic distinct from absence.
- Optional variant fields represent omission, not a default variant.
- Unexpected UI invariants should fail loudly.

## Boundaries

- Map domain and API errors to UI messages in one helper near the screen
  boundary. Name these helpers `*ErrorMessage` and match exhaustively on
  structured error variants.
- External strings keep external spelling.
- Product-facing operational names use the current product brand unless the boundary explicitly requires another spelling.

## navigation and data

- keep navigation and selection state in the existing client owner. android's
  controller and dashboard entry own phone navigation; the terminal ui model
  owns its current filters, selection, viewport, and pending actions.
- the [architecture](../architecture.md#4-product-behavior) and
  [desktop browser](../desktop-browser.md) own retention and restoration rules.
  do not introduce url routing, navigation history, or a query framework around
  these owners.
- bind loading and callbacks to their owner's lifecycle and selected scope.
  discard results that no longer belong to the current request or generation.
- preserve the exact target captured for an action. a refresh may update visible
  observations, but must not retarget pending input or a confirmed mutation.
- the embedded terminal page owns terminal presentation within its documented
  native bridge and attachment lifetime; it is not a separate application router.
- use explicit action handlers for mutations and the existing observation loop
  for refresh. see [polling.md](polling.md) and [effect.md](effect.md).
