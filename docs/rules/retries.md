# retries

## scope

when an operation may be attempted again and which owner decides.

## mutation and delivery

- clients dispatch each session mutation or terminal-input action once. a
  timeout or lost response after possible dispatch must not trigger an automatic
  retry.
- preserve `not_sent`, `unknown`, and partial results as specified by the
  [architecture](../architecture.md), [agent control](../agent-control.md), and
  [client contract](../agent-control-ux.md). uncertainty is an observable result,
  not an instruction to reconcile or repeat the effect.
- reconnecting a transport does not authorize resending terminal bytes or
  repeating a mutation.

## retryable observations

- retry only when the operation contract permits another attempt and repeating
  it cannot duplicate an effect. polling is a new observation, not recovery of
  an earlier mutation.
- keep retries with the operation that owns them. make the retryable conditions,
  delay, cancellation, and stopping condition explicit in that owner.
- bound server-side retries. continuous client connection or polling work must
  stop with its owning lifecycle; see [effect.md](effect.md) and [polling.md](polling.md).
- exhaustion preserves the operation's failure classification. do not convert
  an expected unavailable result into a defect merely because attempts ran out,
  or hide an invariant failure behind an unavailable result.
- retry the smallest permitted unit. do not stack independent retry loops at
  multiple layers. see [timing.md](timing.md) for clocks and bounds.
- in-memory changes are not rolled back when an attempt fails. create
  attempt-local state within the attempt, or update shared state only when the
  owning operation can establish the corresponding fact.
