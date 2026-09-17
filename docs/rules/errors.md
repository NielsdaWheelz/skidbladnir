# Errors

## Scope

error and defect modeling, absence classification, and runtime invariant checks.

## Errors and Defects

- Construct an error type only in the code that detects the condition it represents.
- When branching on an error, consume the original error type completely and replace it with distinct branch-specific types.
- Use errors for expected, modelable failures.
- Use defects for broken invariants, impossible states, internal corruption,
  schema or code mismatch, and similar "should never happen" conditions.
- classify dependency failure by the operation contract. the current product
  models unavailable peers/providers and stale targets explicitly; duration or
  retry exhaustion alone does not make those outcomes defects.
- do not invent fallback output or convert an invariant failure into a modeled
  unavailable result merely to continue.
- Do not classify provider failures into domain errors by coarse HTTP class or transport shape alone.
- Only map a provider response to a modeled domain error when the provider contract or our adapter normalization identifies that exact condition.
- preserve unexpected provider failures as errors for investigation; do not
  guess a domain outcome from an unrecognized response.
- Do not soften a required follow-up dependency observation because an earlier irreversible external side effect already succeeded.
- If the provider contract says a follow-up fact should exist after that side effect, missing or inconsistent follow-up data is a defect after the applicable retry/classification path.
- If the provider contract says the follow-up fact is expected to be absent sometimes, model that absence explicitly at the adapter boundary.
- preserve the owning result and dispatch classification when attempts end;
  see [retries.md](retries.md).
- Handle errors as deeply as possible and propagate them upward only when needed.
- Defects are not normal application control flow.
- Do not convert defects into UI states, retryable business branches, persisted domain status fields, or other product-facing recovery paths.
- Do not mask unexpected internal failures with synthetic fallback output, placeholder summaries, or "best effort" persisted state just to keep a workflow moving.
- Observing a defect in production should trigger a code or operational change.
- Any intentional defect classification must include `justify-defect`.
- Any branch that discards an error must first narrow it to a named or tagged error and include `justify-ignore-error`.

## absence and null

- classify absence where its meaning becomes known. use native optional values
  when absence is a successful result, an error when it is an expected failure,
  and a defect when it violates an invariant.
- do not make callers guess whether an empty value means absent, unavailable,
  invalid, or failed. use an explicit result variant when those states differ.
- native nullable domain fields are valid. preserve field-specific wire
  omission and null semantics; see [boundaries.md](boundaries.md).
- loss of confirmation after dispatch is not absence or proof of failure.
  preserve the unknown or partial outcome specified by
  [agent control](../agent-control.md#capability-and-api-contract).

## Service Invariants

- Represent parameter validity in types, validated wrappers, and parsed canonical values.
- If malformedness is knowable locally, validate once into an owned type,
  validated wrapper, or parsed canonical value and carry that value through the
  system.
- Do not hide local representability checks inside render, quote, or encode helpers.
- Renderers, quoters, and encoders should assume already-owned local types and only perform boundary-specific escaping or formatting.
- Do not use runtime service-boundary guards for parameter validity that should
  be encoded in types, validated wrappers, or parsed canonical values.
- runtime checks enforce remaining local invariants and reobserve mutable
  external state where required by the operation contract.
- checks for internal invariants not represented in types must include
  `justify-service-invariant-check` explaining that choice. their violation is a
  defect. this does not turn expected external changes, such as a replaced
  session or exited process, into defects.
