# conventions

## named constants

- extract a value into a named constant when the name conveys information beyond
  what the usage site already says.
- keep a value inline when it is inherently part of the expression.

## encodings

- use the canonical encoding owned by the protocol. machine handles, session
  references, credentials, and terminal bytes have different contracts.
- reuse the existing encoder and decoder for that boundary. do not substitute
  a preferred alphabet or add encoding layers for consistency alone.
- encoded data is not authenticated merely because callers treat it as opaque.
  see [keys-and-identities.md](keys-and-identities.md).
