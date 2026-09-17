# keys and identities

## scope

identity, authority, and canonical values. the [architecture](../architecture.md)
and [agent-control reference contract](../agent-control-ux.md#selection-identity-and-results)
own the wire formats and lifetime semantics.

## identity

- use the domain's existing identity and name it precisely. machine handles,
  local tmux ids, session lifetime tokens, pane ids, and process start identities
  describe different things; display names are not substitutes for them.
- machine identity is the immutable installation handle. label, origin, bearer,
  and platform are separate facts.
- a session reference routes to a configured machine and carries the observed
  session lifetime; an agent reference additionally carries process identity.
  keep references opaque to callers and use the owning encoder and decoder.
- references identify targets; they do not authorize actions. do not add
  signatures, sealing, alias registries, or private uuid identities around the
  existing protocol.
- parsing an identity does not prove that its target is still present. the
  mutation owner must enforce the required live identity checks.
- meaningful keys and structured targets should keep their structure until a
  boundary needs the canonical string form. give them a named type when that
  distinguishes concepts or enforces useful local invariants.
- a spec describes a value's terms rather than a separate identity. do not
  invent an id for something whose structured value already identifies it.

## credentials

- bearer credentials and pairing invitation tokens are authority. generate
  random credential material; do not derive authority from entity identity.
  the existing `identityToken` field is session lifetime identity, not a bearer.
- the auth owner loads and verifies the gateway's bearer file. pairing owns its
  one-use, expiring in-memory invitation and its verifier. follow those
  lifecycles; do not introduce credential tables or a separate lookup system.
- preserve constant-time credential comparison and keep credentials out of
  logs, references, and identity fields.
- expiry, replacement, and rotation are explicit product behavior, not naming
  conventions. [architecture](../architecture.md#7-security) owns the security
  boundary.

## names and validated values

- prefer the most specific honest domain name and use it consistently across
  boundaries. suffixes such as id, key, handle, token, and ref do not create a
  new storage or authorization model.
- validate locally knowable malformedness in the owning parser. use native
  named types or wrappers when they carry a useful guarantee; do not introduce
  a parallel schema or branding framework.
- a parser accepts a representation. a resolver observes or locates its target.
  keep those responsibilities distinct.
- validate canonical wire values at ingress; do not silently normalize trusted
  values later. see [boundaries.md](boundaries.md).
