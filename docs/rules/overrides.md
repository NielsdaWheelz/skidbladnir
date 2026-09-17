# Overrides

## Scope

justified compiler and tooling overrides, unsafe casts, and native type narrowing.

## Overrides

- every compiler, type-checker, linter, formatter, or static-analysis override
  must have a local justification in the tool's supported comment form.
- The justification must explain why the override is needed and what invariant
  keeps it safe.
- Prefer a small documented override at the exact line over a broad file-level
  or project-level suppression.

## Type Assertions

- use native checked narrowing when the input may have another shape: go type
  switches or comma-ok assertions, kotlin `is` or `as?`, and explicit js checks.
- an unchecked assertion needs a local producer guarantee. explain nonobvious
  assumptions beside the assertion; ordinary checked narrowing needs no special
  justification token or wrapper.
- unsafe casts require a justification explaining why they are needed, the
  invariant that makes them safe, and why a safer typing approach is not feasible.
