# No retained behavioral suite

Problem: behavioral suites and harnesses were retired on 2026-09-17 at the
owner's direction; a change proves itself with temporary tests deleted before
commit ([testing](../rules.md#testing)). The one retained test is the
classifier's frame replay, `internal/agentcontrol/frames_test.go`, run on demand.

Impact: `scripts/check verify` runs whitespace, gofmt, shellcheck, go vet, module
verification, go build, `node --check` on the terminal page and Android lint and
assemble (`scripts/check:48-83`). It catches invalid source, broken builds and
release-integrity failures, not session targeting, input delivery, pairing,
terminal interaction or client behavior. Pure logic that is intricate and
deterministic has no regression protection either, and a reader who looks for
tests finds one file across about 36,000 lines of source.

## Pure-core tests

Proposed 2026-10-25 during the publication campaign and deferred by the owner.
Table-driven tests over pure functions whose behavior is intricate, deterministic
and easy to break silently. They are not the retired suite: no harness, no tmux,
no network.

Candidates:

- the screen classifier: the frame replay already checks every authored
  observation (466) against its expected status; it protects only when it runs;
- the seal formula in `Ornament.kt`: golden seals for a few catalogue keys;
- session lifetimes and identity tokens (`internal/sessions/identity.go`):
  round trips and rejections;
- `internal/strictjson`: unknown fields, duplicate keys, trailing data;
- the lifetime guard: the condition text built in `internal/tmux/client.go`
  (`sessionLifetimeConditions`), and how `internal/tmux/guard.go` maps each
  output to applied, refused and unknown;
- `group.ParseDraft`: NFC canonicalization and rejected characters;
- `internal/hostconfig`: accepted and rejected configuration documents.

Tests that run only on demand protect nothing, so the proposal includes running
them in `scripts/check verify`. That is a gate, which the retirement removed; the
owner decides.

Cost: maintained fixtures, and a one-line amendment to AGENTS.md and
[testing](../rules.md#testing) separating pure-function tests from the retired
behavioral suite.

Resolved when: the owner chooses and keeps a small executable suite for the
important behavioral guarantees, with measured runtime and demonstrated failure
sensitivity, or accepts the gap, which then lives in rules.md alone. The
pure-core part resolves when those tests land with demonstrated failure
sensitivity, or the owner rejects them here. Missing execution is never a pass.
