# polling

## scope

recurring inventory, status, and pressure observations.

## rules

- use the existing polling owners and accepted cadence. the
  [architecture](../architecture.md#4-product-behavior),
  [agent-control status contract](../agent-control.md#identity-state-and-dispatch),
  and [desktop browser](../desktop-browser.md#2-composition-and-data-contract)
  own refresh, stale-state, and manual-verification behavior.
- polling samples changing external state. it does not create an atomic fleet
  snapshot or authorize replay of mutations and terminal input.
- keep requests bounded and coalesce overlapping work at the existing owner:
  per machine/resource on android and one scoped inventory read in the browser.
  provider status enrichment retains its existing batch and time limits.
- recurring work stops with its owning foreground, browser, or host-process
  lifecycle. late results must not update a replaced scope or generation.
- retain stale observations only as specified by the product, visibly unavailable
  and without enabling actions that require fresh inventory.
- document a new polling loop's need, cadence, and stopping condition with
  `justify-polling`. do not add push infrastructure or a new polling daemon to
  satisfy a generic preference.
- clocks and cancellation follow [timing.md](timing.md) and [effect.md](effect.md).
