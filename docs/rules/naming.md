# naming

## scope

names for values, operations, and observability.

## values

- use names that describe the domain and current owner.
- preserve the casing and spelling owned by each wire or provider contract.
  launch kinds and agent-control states use lowercase; existing provider names
  and error codes retain their specified spelling. do not rename wire values
  to satisfy a global enum convention.
- follow the language's existing identifier conventions within each package.

## operations

- choose a verb that states what success establishes: observing, validating,
  creating, changing, or deleting are different contracts.
- `ensure...` means convergence on the named condition; an already-satisfied
  condition succeeds. implement that behavior explicitly in its owner.
- do not give an ordinary create or delete operation convergent semantics by
  silently swallowing an already-existing or missing target. use its specified
  result and error contract.
- distinguish parsing a representation from resolving or observing a live
  resource; see [keys-and-identities.md](keys-and-identities.md).

## observability

- use stable event names and field keys from the owning logging contract.
- dynamic data belongs in values, not keys. do not derive keys from user input,
  provider data, counters, or entity identities.
- preserve content-free, credential-free logging. no prompts, terminal bytes,
  objectives, tokens, or account data.
