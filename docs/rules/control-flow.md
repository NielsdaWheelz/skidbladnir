# Control Flow

## Scope

This document covers exhaustive branching and race-safety rules.

## Exhaustiveness

- cover every case in a finite domain. use native compile-time exhaustiveness
  where available, such as kotlin `when` over a sealed type or enum.
- where the language cannot prove coverage, such as go switches over named
  string constants, handle unmatched values explicitly. reject unsupported
  external input; report impossible owned values as defects instead of returning
  plausible default output.
- do not add code generation or a matching framework solely to impose
  compile-time exhaustiveness on a language that does not provide it.
- This applies to errors as well. Do not erase finite error channels with catch-all handlers that discard or collapse distinct errors.
- Prefer tag-specific or variant-specific error handlers for finite error sets.

## Races

- Do not race work that performs a destructive or non-idempotent operation unless losing the result is acceptable.
- Race primitives usually discard or cancel losing results.
- If the losing task performed an irreversible side effect, the side effect may
  be committed while the result is lost.
- When concurrent work needs to coordinate around destructive operations, route
  signals through a single serialization point.
