# mutation ordering

## scope

ordering changes across ownership and observation boundaries.

## visibility and support state

- identify what makes a resource observable and what that observation promises.
- when visibility promises that support resources exist, establish that support
  before publication and remove visibility before releasing required support.
- order steps by those dependencies, not by whether state is local or external.
  a boundary can exist inside one process.
- retain the exact identities needed by later cleanup before removing their
  source. cleanup must not resolve a replacement resource by name.
- make intermediate states and possible partial outcomes explicit. do not add
  placeholder lifecycle state or recovery machinery to imply atomicity the
  operation does not have.
- the [architecture](../architecture.md), [shell creation](../shells.md), and
  [agent-control contract](../agent-control.md) own publication, detach, stop,
  and deletion order. a generic ordering rule does not change those sequences.

## ownership

- the caller orders its own changes relative to calls into another module. the
  called module owns its internal sequence and external resources.
- use that module's public operation; do not reach into its private state to
  force ordering.
- await a step when the next step depends on its completion. background work
  must have an explicit owner and lifetime; see [effect.md](effect.md).
- after a possible external effect, preserve uncertainty or partial completion
  required by the operation contract. do not replay an earlier mutation to
  finish a later step; see [retries.md](retries.md).
