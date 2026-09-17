# json values

## rules

- keep structured json as structured values until the boundary requires an
  encoded string or bytes. use the protocol's existing codec.
- distinguish an omitted field, a present json `null`, and a present value when
  the wire contract distinguishes them. do not change that shape mechanically.
- use native optional values for successful absence in owned code; see
  [boundaries.md](boundaries.md). when json `null` is itself a valid value, keep
  field presence separate so it cannot be confused with absence.
- decode into the owning shape at ingress. preserve the existing strict-json
  rules for duplicate, unknown, or malformed members and field-specific null
  behavior; ordinary decoding alone may not enforce them.
- compare structured json by content when equality is required, not by object
  identity. narrow to primitives when only a primitive field matters.
