# correctness

## scope

failure classification and repository-wide correctness invariants.

## failures

- model expected conditions explicitly: invalid requests, missing or replaced
  sessions, process exit, unavailable peers, and uncertain delivery are part of
  the current product contracts.
- a broken internal invariant is a defect. preserve evidence and repair its
  cause; do not hide it behind plausible output or silent normalization.
- classify dependency failures by the owning operation's contract, not by how
  long a dependency has been unavailable. see [errors.md](errors.md) and
  [retries.md](retries.md).

## invariants

- concurrent execution must preserve the operation's promised ordering and
  target identity. see [concurrency.md](concurrency.md).
- a successful parse proves a value's shape, not the continued existence of the
  session, process, or path it identified. reobserve changing external state at
  the boundary responsible for using it.
- inventory is an observation of tmux, not a durable claim on a session. handle
  later disappearance and replacement through the existing stale-target rules.
- after dispatch, loss of confirmation does not prove that no effect occurred.
  preserve the [agent-control outcomes](../agent-control.md#identity-state-and-dispatch)
  and the [architecture's mutation contracts](../architecture.md).
- keep cross-boundary ordering explicit; see [mutation-ordering.md](mutation-ordering.md).
- use types to enforce local invariants where they make the code simpler. parse
  external data at ingress; see [boundaries.md](boundaries.md).
