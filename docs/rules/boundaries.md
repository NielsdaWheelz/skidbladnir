# Boundaries

## Scope

This document covers how values should change shape when they cross a boundary
between owned code and external systems, between modules, or between trust
levels.

## Goal

boundary conversion establishes the value's local shape and meaning. downstream
code uses that representation without repeating the same shape checks. a parsed
identity does not guarantee that a session, process, or path still exists; the
owner must reobserve mutable external state when its operation requires it.

## Boundary Contracts

- boundary adapters accept only the states their contract models and reject
  malformed or unsupported external input explicitly.
- an impossible owned state after successful decoding is a defect. do not
  confuse it with bad external input, which is an expected boundary failure.
- Do not add fallback parsing, placeholder owned values, persisted invalid state, or other recovery paths for inputs we do not expect to receive.
- If an unexpected input state needs product behavior, first make it an explicit modeled adapter output or domain state, then handle that modeled value.

## Untrusted Input

Untrusted input comes from outside the system: API requests, webhooks, third-party APIs, files, user input, or generated output.

- Parse, validate, and narrow to a specific typed form where untrusted data enters. Failures are typed errors — bad external input is expected.
- The result of parsing is the narrow internal representation: a validated
  value, domain object, or internal identifier. That representation flows
  through all downstream code.
- do not repeat shape validation on an unchanged value whose type already
  establishes that guarantee. live existence, lifetime, authorization, and
  filesystem checks are separate operation requirements.
- Parse only the single expected shape. Do not add branches for alternative shapes "just in case" — those are dead code that hides bugs.
- see [keys-and-identities.md](keys-and-identities.md) for identity and authority
  boundaries and [errors.md](errors.md) for absence classification.

## Human Draft Input

Human-facing drafts are the ingress for typing convenience.

- Keep draft state as text and normalize presentation affordances in explicit draft-formatting helpers such as `formatXDraft`.
- Draft formatting may strip punctuation, insert display separators, or add obvious typing defaults before submission.
- Submit handlers parse the draft into the owned canonical type before crossing
  an API boundary.
- API schemas and backend parsers for values with draft formatting accept only
  the canonical wire shape. Do not duplicate client convenience normalization at
  the backend boundary.
- If another client needs the same convenience behavior, put an equivalent
  draft-formatting step at that client boundary instead of widening the backend
  parser.

## Generated Output

Generated output from an LLM or other model is untrusted until the owning ingress helper decodes it and accepts it into owned types.

- Validate objective local invariants at the generated-output ingress: schema
  shape, enum membership, identifiers, handles, and renderable asset names.
- Do not normalize valid generated prose to satisfy style preferences. Encode style preferences in the prompt and preserve accepted prose as authored.
- After accepted generated output is stored or returned through our own typed transport, treat it as trusted same-system data.
- Use `sanitize` only for helpers that intentionally remove or rewrite unsafe content. Use `accept`, `parse`, or a domain-specific conversion name for helpers that validate or brand without rewriting.

## Trusted Data

trusted values are values whose shape an owning module has established.
external state remains independently mutable even when we wrote it earlier.

- If trusted data does not match the expected shape, that is a defect — something we wrote or stored is wrong.
- do not silently normalize malformed owned data. repair the writer or decoder
  that failed to preserve its contract.
- Do not add redundant validation across layers. Tighten the source or the persisted representation instead.
- references can outlive their targets. classify a missing target according to
  its lifetime contract; a stale tmux or process observation is an expected
  outcome, not corruption. see [agent control](../agent-control.md).

## Same-System Transport

Responses across owned transport boundaries are same-system data after the
transport decoder has accepted them.

- Decode the wire shape at the client boundary, then treat the decoded value as owned typed data.
- preserve the meaning of optional fields when passing decoded data through
  models and views; do not replace absent facts with fabricated values.
- UI components should not re-validate domain invariants that the server write path or API schema is responsible for preserving.
- If decoded same-system data violates an owned type guarantee later, treat that as a defect and fix the source or schema.

## internal representation

- keep meaningful values in their owned representation until a consumer needs
  an encoded form. add a named type when it carries a useful distinction or
  invariant, not just to wrap every primitive.
- use native representations for successful absence: go pointers or a value
  plus a presence flag, kotlin nullable types, and explicit js missing/null
  semantics. use a tagged variant when absence has multiple distinct meanings.
- do not add an optional-value framework around those representations.
- distinguish successful absence, an expected failure, and a broken invariant
  at the responsible owner; see [errors.md](errors.md).
- an omitted wire key and an explicit `null` can have different meanings. keep
  that distinction at decoding and encoding boundaries; do not mechanically
  substitute one for the other. see [json-values.md](json-values.md).

## outgoing conversion

- encode at the boundary that consumes the encoded form, using its existing
  codec and exact null/omission contract.
- do not carry pre-encoded strings through owned logic when the structured value
  is still needed.
