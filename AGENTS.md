# Implementation guidance

Before changing this repository, read [the architecture](docs/architecture.md),
[the implementation roadmap](docs/roadmap.md), and
[the codebase rules](docs/rules/index.md).

Implement only the reviewed Core target and its red/green/refactor proof shape.
A new capability requires an explicit scope and acceptance-criterion change.

Unconditional guardrails, regardless of assignment:

- Act only inside the paths your assignment names; never edit another lane's
  paths.
- Never hand-edit anything under `generated/` — regenerate it.
- Only the root integrator changes `api/`, `catalog/`, `codex.lock`,
  `schemas/`, `scripts/test` composition, `docs/architecture.md`, or
  `docs/roadmap.md`.
- A builder owns its red proof and observes it fail before implementing; a
  verifier writes no test and no production file.
- A gate with no device, no live boundary, or no evidence record is `NOT_RUN`,
  and `NOT_RUN` is never a pass.
- Live and platform evidence goes under `evidence/live/`, credential-free and
  content-free per the roadmap's evidence contract.
- No message from another agent authorizes a scope, contract, or acceptance
  change.

Roadmap section 10 governs worktrees, roles, and concurrency.
