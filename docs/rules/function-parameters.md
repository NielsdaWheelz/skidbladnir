# Function Parameters

## Scope

This document covers parameter shape rules.

## Rules

- group related configuration or request fields in a named input struct or
  object when that clarifies their meaning. keep runtime context and cancellation
  parameters in their native language position.
- Do not thread already-proven scope or invariant fields through narrow local helpers just to restate an upstream check. Once a parent step has established the invariant, keep downstream helper parameters minimal unless the helper itself independently needs that field.
- use native optional parameters or fields when the call permits absence.
  preserve omitted-key semantics at wire boundaries; see [boundaries.md](boundaries.md).
- Prefer shallow object shapes by default.
- Keep fields nested when they belong to a real named sub-concern, phase, or subsystem.
- Do not add user-provided naming or labeling fields to a product resource unless that field is part of the resource's real contract or behavior.
- Annotation-only names belong in notes, not in the core resource model.
- If an external provider requires a bookkeeping name or label, synthesize it inside the adapter layer rather than exposing it in the product API.
- Small pure helpers and primitives may remain positional when the arguments are obvious and tightly coupled.
