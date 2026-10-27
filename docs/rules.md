# Rules

These rules bind implementation. Product behavior, scope and acceptance belong to the
[architecture](architecture.md) and the specs it links; rules authorize no new runtime
machinery. This document owns decode strictness, defensive checks and draft
canonicalization; code that conflicts with it is corrected when it next changes.
skid has one owner, three hosts and one phone, with live repair: build the simplest
complete design for that, and state every trade-off a change makes.

## Trust boundaries

Validate once, where data enters skid, into owned types. Ingress failures are
expected and modeled; downstream code carries the owned value. There are three
ingresses:

- Human input: `config.yaml` and CLI arguments. `config.yaml` rejects unknown keys so
  typos fail loudly; an invalid edit leaves accepted settings. A draft typed into a
  client enters at the gateway request that carries it (see drafts).
- OS, tmux, provider and third-party output: tmux formats and captures, OS reads,
  provider screens and daemons, Tailscale, FCM. Its adapter parses it into owned
  types and models what it cannot accept.
- Network requests at the gateway: authenticate, bound, then decode the body strictly
  into its owned request type. Rejection is a typed HTTP error.

Everything else is same-system data. Host, desktop and phone ship from one release
and trust each other's outputs:

- Producers own invariants. Consumers re-check no ranges, formats, identities or
  cross-field consistency, and call no `Valid()` on decoded peer data.
- Consumers decode tolerantly and only what they read: unknown keys and unread fields
  are ignored. A required read field that is missing or mistyped, or a read enum value
  the consumer does not know, fails the decode of that response.
- Go declares each gateway JSON document once, for the gateway and Go clients alike:
  `internal/api` holds the inventory, session, terminal, profile-usage and error
  documents and the vocabulary they carry; pressure, workdir, notification and
  WebSocket documents stay with their producers. Go clients share those types with the
  gateway and decode same-release documents with plain `encoding/json`: a missing key
  is its zero value, not a failed decode. An unknown value of an enum a Go client
  branches on or republishes (status, name mode, recovery, provider, write and close
  outcomes, read state) fails the decode; one it only prints (read source and scope,
  write method, transport) passes through. An unknown error code is still the
  gateway's answer: the failure keeps the code and the body's dispatch, else the
  call's default (`not_sent` for a read, `unknown` for a sent write); an empty code or
  an unknown dispatch fails the decode.
- An undecodable peer response is a lost response: observations show that peer
  unavailable, and a mutation that may have been dispatched is `unknown`. It is never
  a crash or panic.
- A skid-owned file or stored state that fails to read is an error its owner handles,
  never a panic.
- The accepted cost: a producer bug renders wrong data instead of crashing a client.
  Repair the producer.

## Copy and drafts

- Whoever produces an outcome renders its copy. Clients display host error and status
  copy and keep no parallel copy tables; they render copy only for their own
  outcomes, such as a lost connection.
- A draft bound for the host (a label, name or path typed into a form or passed as a
  CLI argument) is sent as typed. The host canonicalizes and validates it, returning
  the canonical value or a typed error with copy; clients do not normalize drafts.
  Pre-submit affordances a spec names, such as a disabled save or no search on blank
  input, stay client UX; the host's verdict is authoritative.

## Errors and panics

- Model expected conditions as values or typed errors where they are detected:
  invalid input, missing or replaced sessions, process exit, unavailable peers,
  uncertain delivery. Callers branch on named errors, never message text.
- Anything else is a defect: an unmodeled failure that propagates and becomes the
  content-free `InternalError` at the gateway edge. A defect calls for a code or
  operational fix, never a UI state, retry branch or persisted status.
- Panic only for a programmer error in pure code, such as an impossible case of an
  owned closed set; never on IO, decoding, external state or peer data. Kotlin `when`
  over sealed types is exhaustive; a Go switch rejects an unmatched value at ingress
  and panics on one in pure code.
- Add no check that restates what the compiler, a constructor or the caller already
  guarantees, such as a nil-dependency assertion. No `justify-*` tokens; existing ones
  go with the guards they decorate. Comment why only where the reason is not obvious.
- Discard an error only when no caller could act on it, such as a write to a
  disconnected client. Never hide failure behind fallback output, placeholder values
  or best-effort persisted state. Map an external failure to a domain error only when
  its contract names that exact condition.
- Absence uses native optionals (Go pointers or an ok flag, Kotlin nullables), or an
  explicit variant when absent, unavailable and failed differ. An omitted key stays
  distinct from `null` where the wire contract distinguishes them.

## Identities and exact targets

- Machine handle, tmux ID, session lifetime token (`identityToken`), pane ID and
  process start identity are distinct; display names never substitute for them.
  References identify and route; they do not authorize. Add no signatures, sealing,
  alias registries or private UUIDs around them.
- Parsing proves shape, not existence. An action keeps the exact target it captured;
  a refresh never retargets pending input or a confirmed mutation. The mutation owner
  reobserves tmux or the process and refuses a target whose live identity no longer
  matches. Inventory observes tmux; it does not claim a session.
- Bearer credentials are random, compared in constant time and kept out of logs,
  references and identity fields; `identityToken` is not one.

## Mutations, delivery and retries

- Clients dispatch each session mutation or terminal input once. After possible
  dispatch, a timeout or lost response is `unknown`, never a retry; preserve
  `not_sent`, `unknown` and partial results as the
  [architecture](architecture.md#gateway-wire-invariants) specifies.
  Reconnecting never authorizes resending bytes or repeating a mutation.
- Retry only observations that cannot duplicate an effect, in one bounded owner that
  states its delay, limit and stop. Never stack retry loops across layers.
- Publish visibility only after the support it promises exists; withdraw it before
  releasing that support. Keep the exact identities cleanup needs before removing
  their source; cleanup never resolves a replacement by name.
- Make partial outcomes explicit; no placeholder lifecycle state implying atomicity.

## Concurrency and lifetimes

- Gateway requests run concurrently. Each gateway owns its tmux connection and state;
  there is no cross-host transaction or coordinator.
- Keep a target check and its guarded mutation together at the authoritative
  boundary. A local mutex does not serialize tmux clients or external processes:
  claim no atomicity across systems and no defense against a hostile same-user
  process. Hold no lock across provider calls or client interaction, and never race
  a destructive or non-idempotent operation whose result could be lost.
- Whoever starts concurrent work owns its cancellation, completion and errors: no
  detached goroutines or coroutines, and no value outlives the resource it depends
  on. Shutdown cancels a stream's workers, unblocks their IO and waits for them.

## Timing and polling

- Use the clock that owns the fact; never compare host clocks or substitute receipt
  time. Transmitted instants carry a zone; local durations use a monotonic clock and
  native duration types. A caller's deadline and a child's timeout never extend each
  other.
- Polling samples changing state: no atomic fleet snapshot, and never a replay of a
  mutation or input. Use the existing owners and cadence, bounded and coalesced per
  machine. Recurring work stops with its owner; late results never update a replaced
  scope or generation.
- Add no polling daemon or push infrastructure for a generic preference. A new loop
  states its need, cadence and stop condition beside it.

## Code shape

- Density is the goal. Delete dead code, impossible branches, speculative options and
  anything reachable only from tests. Releases are hard cutovers: no legacy paths,
  fallbacks, shims, dual decoders or migration code, and nothing named for a finished
  era.
- One concern, one owner, persisted state included. Modules are deep: small explicit
  interfaces, hidden internals, unexported by default. Derive state; do not copy it.
  Edge adapters (HTTP handlers, CLI commands, UI transport) parse, translate and call
  the owner; they hold no business rules.
- Control flow is linear and explicit. Inline anything used once unless it hides real
  incidental complexity. No hollow wrappers or caches over indirection; collapse only
  large or dangerous duplication. A field needs a current consumer.
- A name says what its value is or what success establishes: `ensure` converges;
  create and delete do not absorb an existing or missing target; parsing is not
  resolving a live target. Wire and provider values keep their spelling. Log events
  have stable names, with dynamic data in values.
- Keep values structured until the boundary that encodes them, with its existing
  codec. Renderers and quoters escape; they do not validate.
- Go and Kotlin own application logic; another language appears only where its
  runtime owns the work (the terminal page's JavaScript, macOS Objective-C, Python
  tooling). Shell orchestrates processes. A lint
  suppression sits on its exact line with a reason. Generated files are committed
  with one owning command, and `scripts/check` fails when they are stale.

## Testing

There is no behavioral test command, suite or gate; [the gap](issues/test-system-reset.md)
is recorded, and temporary tests do not close it.

- A change proves itself with temporary tests at the affected integration or live
  boundary, run against the original and changed code; a bug fix shows its failure
  first. Review the test's sensitivity, then delete it before committing. Rebuild no
  harness and add no production seam. Tests assert observable outcomes through the
  public surface, never structure, styling or wiring.
- One exception, at the owner's request: the detector's authored provider
  frames stay as fixtures in `internal/agentcontrol/testdata/frames/` (dummy content
  captured through real tmux by its `capture.py`), with one pure replay,
  `internal/agentcontrol/frames_test.go`, run on demand by `go test
  ./internal/agentcontrol`. No gate runs it. Rerun it when the classifier or a
  provider grammar changes; extend the corpus instead of writing a frame harness.
- Deletions of unreachable code or docs are proved by caller, build and link checks.
- Builds and static checks do not prove product behavior. Historical results apply
  only to their recorded source; removed, unavailable or unexecuted checks are never
  passes. Isolated runs need no approval; anything that reaches live state
  needs current-turn approval under [AGENTS.md](../AGENTS.md).

Commands:

- `scripts/check {static|verify} [all|host|android|none]`: static runs formatting,
  whitespace and shell lint, plus go vet and module verification for
  host and terminal-page syntax and Android lint for Android (default `all`). verify
  adds the scoped go build and Android debug assembly.
- Ornament geometry and the launcher icon are frozen source with no drift gate. The
  catalogue has no pre-release check: the gateway loader rejects invalid or duplicate
  keys at startup; the [design language](design-language.md#18-dvergatal-curation)
  owns admission rules.
- Terminal vendor files are verbatim upstream npm and release bytes with no build
  step and no retained check; git tracks them, and the
  [architecture](architecture.md#page-and-files) records their sources, integrity and
  re-verify recipe.
- `scripts/release TAG`, `scripts/check-release [--reproduce] [--source DIR]
  RELEASE_DIRECTORY TAG SOURCE_SHA` and `scripts/check published-release TAG
  SOURCE_SHA` build, verify and re-verify releases ([release](architecture.md#release)).
  Only `--reproduce`, run from the release's source checkout, compares bytes.
- Hosted CI runs `verify` on every pull request and main push, scoped by the changed
  paths in `.github/workflows/verify.yml`. A scoped pass proves only the checks it ran.

## Docs

- [Architecture](architecture.md) is the one contract: concepts, invariants,
  interfaces, owners, error semantics, topology and trust. Specs exist only where an
  invariant is not evident from the code: the observation grammars, workspace
  recovery, the tmux exec budget and the design language. The
  [codebase map](codebase-map.md) locates code; the [roadmap](roadmap.md) holds what
  is next and what is deferred.
- Docs state the current contract and are edited in place, in the change that
  alters the behavior. No history, amendment chains, dated slices or per-feature
  delta docs: dated evidence, qualification logs and superseded plans belong in pull
  requests and git history. A fact belongs in a doc only when the code does not show
  it plainly.
- Record each unresolved issue in `docs/issues/<short-name>.md`: problem, impact,
  evidence and what proves it resolved. Delete it once resolved.
