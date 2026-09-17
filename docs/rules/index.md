# Docs

## Role

this directory owns codebase rules. product behavior and accepted scope belong
to the [architecture](../architecture.md), [roadmap](../roadmap.md), and the
feature specifications they accept. rules do not authorize new runtime machinery
or override those contracts.

## Goals

- MECE organization: documents are mutually exclusive and collectively exhaustive.
- Concision
- Clear boundaries

## Docs

### Correctness and concurrency

- [correctness.md](correctness.md): abnormality classification and system invariants
- [concurrency.md](concurrency.md): linearization and concurrent execution
- [mutation-ordering.md](mutation-ordering.md): ordering mutations across systems and module boundaries
- [retries.md](retries.md): retry policies and exhaustion handling

### Data and types

- [boundaries.md](boundaries.md): data representation at ingress, internal, and egress edges
- [errors.md](errors.md): error and defect modeling, null classification
- [keys-and-identities.md](keys-and-identities.md): identity, authority, and canonical values
- [json-values.md](json-values.md): structured JSON values
- [tagged-unions.md](tagged-unions.md): tagged variants versus domain-record shapes
- [generated-text.md](generated-text.md): escaping and quoting at generated-text boundaries

### Runtime composition

- [effect.md](effect.md): effectful work, background tasks, and scoped values

### Code style

- [cleanliness.md](cleanliness.md): dead code, ownership, duplication, and complexity reduction
- [simplicity.md](simplicity.md): fewer code paths, no speculative surface
- [naming.md](naming.md): naming grammar for identifiers and observability
- [function-parameters.md](function-parameters.md): parameter conventions
- [control-flow.md](control-flow.md): exhaustive branching and race-safety
- [overrides.md](overrides.md): justified escape hatches and type assertions
- [conventions.md](conventions.md): named constants and protocol encodings

### Platform

- [codebase.md](codebase.md): technology ownership, repo structure, imports, and module boundaries
- [frontend.md](frontend.md): client UI state, navigation, and data boundaries
- [testing.md](testing.md): test retirement status and retained engineering checks
- [timing.md](timing.md): clocks, expiry, deadlines, and recurring work
- [polling.md](polling.md): polling rules

## Placement Rules

- Each rule lives in exactly one document.
- Put content in the narrowest document that fully owns it.
- Link to related docs instead of restating them.
- If two docs need the same text, the split is wrong.
- If a document covers multiple unrelated topics, split it.
- Small docs are fine when they keep ownership and boundaries sharp.
- Keep repo-wide rule docs flat until a topic clearly needs its own directory.
- Use subdirectories for service-owned, module-owned, or feature-owned docs when that keeps them separate from repo-wide rules.
- Avoid over-categorized hierarchies and umbrella docs with weak boundaries.

## Rule Shape

- Prefer unconditional rules.
- Do not write soft rules with words like `usually`, `generally`, or `normally`.
- State the unconditional rule or the explicit exception.
- Prefer narrowing scope or splitting a rule over adding exceptions.
- If a rule needs many exceptions, the rule or the document boundary is probably wrong.

## Ownership

This file defines the documentation system itself: purpose and placement rules. It does not own product or codebase rules beyond that.
