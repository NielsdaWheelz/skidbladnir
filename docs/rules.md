# rules

these rules bind implementation. product behavior, scope and acceptance belong to the
[architecture](architecture.md), the [roadmap](roadmap.md) and the specs they accept;
rules authorize no new runtime machinery. this document owns decode strictness,
defensive checks, draft canonicalization and `justify-*` tokens; a spec or code that
conflicts with it on those is superseded and is corrected when its code next changes.
skid has one owner, three hosts and one phone, with live repair: build the simplest
complete design for that, and state every trade-off a change makes.

## trust boundaries

validate once, where data enters skid, into owned types. ingress failures are
expected and modeled; downstream code carries the owned value. there are three
ingresses:

- human input: `config.yaml` and cli arguments. `config.yaml` rejects unknown keys so
  typos fail loudly; an invalid edit leaves accepted settings. a draft typed into a
  client enters at the gateway request that carries it (see drafts).
- os, tmux, provider and third-party output: tmux formats and captures, os reads,
  provider screens and daemons, tailscale, fcm. its adapter parses it into owned
  types and models what it cannot accept.
- network requests at the gateway: authenticate, bound, then decode the body strictly
  into its owned request type. rejection is a typed http error.

everything else is same-system data. host, desktop and phone ship from one release
and trust each other's outputs:

- producers own invariants. consumers re-check no ranges, formats, identities or
  cross-field consistency, and call no `Valid()` on decoded peer data.
- consumers decode tolerantly and only what they read: unknown keys and unread fields
  are ignored. a required read field that is missing or mistyped, or a read enum value
  the consumer does not know, fails the decode of that response.
- an undecodable peer response is a lost response: observations show that peer
  unavailable, and a mutation that may have been dispatched is `unknown`. it is never
  a crash or panic.
- a skid-owned file or stored state that fails to read is an error its owner handles,
  never a panic.
- the accepted cost: a producer bug renders wrong data instead of crashing a client.
  repair the producer.

## copy and drafts

- whoever produces an outcome renders its copy. clients display host error and status
  copy and keep no parallel copy tables; they render copy only for their own
  outcomes, such as a lost connection.
- a draft bound for the host (a label, name or path typed into a form or passed as a
  cli argument) is sent as typed. the host canonicalizes and validates it, returning
  the canonical value or a typed error with copy; clients do not normalize drafts.
  pre-submit affordances a spec names, such as a disabled save or no search on blank
  input, stay client ux; the host's verdict is authoritative.

## errors and panics

- model expected conditions as values or typed errors where they are detected:
  invalid input, missing or replaced sessions, process exit, unavailable peers,
  uncertain delivery. callers branch on named errors, never message text.
- anything else is a defect: an unmodeled failure that propagates and becomes the
  content-free `InternalError` at the gateway edge. a defect calls for a code or
  operational fix, never a ui state, retry branch or persisted status.
- panic only for a programmer error in pure code, such as an impossible case of an
  owned closed set; never on io, decoding, external state or peer data. kotlin `when`
  over sealed types is exhaustive; a go switch rejects an unmatched value at ingress
  and panics on one in pure code.
- add no check that restates what the compiler, a constructor or the caller already
  guarantees, such as a nil-dependency assertion. no `justify-*` tokens; existing ones
  go with the guards they decorate. comment why only where the reason is not obvious.
- discard an error only when no caller could act on it, such as a write to a
  disconnected client. never hide failure behind fallback output, placeholder values
  or best-effort persisted state. map an external failure to a domain error only when
  its contract names that exact condition.
- absence uses native optionals (go pointers or an ok flag, kotlin nullables), or an
  explicit variant when absent, unavailable and failed differ. an omitted key stays
  distinct from `null` where the wire contract distinguishes them.

## identities and exact targets

- machine handle, tmux id, session lifetime token (`identityToken`), pane id and
  process start identity are distinct; display names never substitute for them.
  references identify and route; they do not authorize. add no signatures, sealing,
  alias registries or private uuids around them.
- parsing proves shape, not existence. an action keeps the exact target it captured;
  a refresh never retargets pending input or a confirmed mutation. the mutation owner
  reobserves tmux or the process and refuses a target whose live identity no longer
  matches. inventory observes tmux; it does not claim a session.
- bearer credentials are random, compared in constant time and kept out of logs,
  references and identity fields; `identityToken` is not one.

## mutations, delivery and retries

- clients dispatch each session mutation or terminal input once. after possible
  dispatch, a timeout or lost response is `unknown`, never a retry; preserve
  `not_sent`, `unknown` and partial results as
  [agent control](agent-control.md#operations-and-composition) specifies.
  reconnecting never authorizes resending bytes or repeating a mutation.
- retry only observations that cannot duplicate an effect, in one bounded owner that
  states its delay, limit and stop. never stack retry loops across layers.
- publish visibility only after the support it promises exists; withdraw it before
  releasing that support. keep the exact identities cleanup needs before removing
  their source; cleanup never resolves a replacement by name.
- make partial outcomes explicit; no placeholder lifecycle state implying atomicity.

## concurrency and lifetimes

- gateway requests run concurrently. each gateway owns its tmux connection and state;
  there is no cross-host transaction or coordinator.
- keep a target check and its guarded mutation together at the authoritative
  boundary. a local mutex does not serialize tmux clients or external processes:
  claim no atomicity across systems and no defense against a hostile same-user
  process. hold no lock across provider calls or client interaction, and never race
  a destructive or non-idempotent operation whose result could be lost.
- whoever starts concurrent work owns its cancellation, completion and errors: no
  detached goroutines or coroutines, and no value outlives the resource it depends
  on. shutdown cancels a stream's workers, unblocks their io and waits for them.

## timing and polling

- use the clock that owns the fact; never compare host clocks or substitute receipt
  time. transmitted instants carry a zone; local durations use a monotonic clock and
  native duration types. a caller's deadline and a child's timeout never extend each
  other.
- polling samples changing state: no atomic fleet snapshot, and never a replay of a
  mutation or input. use the existing owners and cadence, bounded and coalesced per
  machine. recurring work stops with its owner; late results never update a replaced
  scope or generation.
- add no polling daemon or push infrastructure for a generic preference. a new loop
  states its need, cadence and stop condition beside it.

## code shape

- density is the goal. delete dead code, impossible branches, speculative options and
  anything reachable only from tests. releases are hard cutovers: no legacy paths,
  fallbacks, shims, dual decoders or migration code, and nothing named for a finished
  era.
- one concern, one owner, persisted state included. modules are deep: small explicit
  interfaces, hidden internals, unexported by default. derive state; do not copy it.
  edge adapters (http handlers, cli commands, ui transport) parse, translate and call
  the owner; they hold no business rules.
- control flow is linear and explicit. inline anything used once unless it hides real
  incidental complexity. no hollow wrappers or caches over indirection; collapse only
  large or dangerous duplication. a field needs a current consumer.
- a name says what its value is or what success establishes: `ensure` converges;
  create and delete do not absorb an existing or missing target; parsing is not
  resolving a live target. wire and provider values keep their spelling. log events
  have stable names, with dynamic data in values.
- keep values structured until the boundary that encodes them, with its existing
  codec. renderers and quoters escape; they do not validate.
- go and kotlin own application logic; another language appears only where its
  runtime owns the work (the terminal page's javascript, macos objective-c, python
  tooling). shell orchestrates processes. a lint
  suppression sits on its exact line with a reason. generated files are committed
  with one owning command, and `scripts/check` fails when they are stale.

## testing

behavioral tests and harnesses were retired on 2026-09-17, superseding the former
test tiers, red/green workflow and gates (unit, integration, provider-live, live,
platform, product, second-phone, full). no behavioral test command or gate remains;
[the gap](issues/test-system-reset.md) is recorded and temporary tests do not close it.

- a change proves itself with temporary tests at the affected integration or live
  boundary, run against the original and changed code; a bug fix shows its failure
  first. review the test's sensitivity, then delete it before committing. rebuild no
  harness and add no production seam. tests assert observable outcomes through the
  public surface, never structure, styling or wiring.
- one exception, at the owner's request (2026-10-01): the detector's authored provider
  frames stay as fixtures in `internal/agentcontrol/testdata/frames/` (dummy content
  captured through real tmux by its `capture.py`), with one pure replay,
  `internal/agentcontrol/frames_test.go`, run on demand by `go test
  ./internal/agentcontrol`. no gate runs it. rerun it when the classifier or a
  provider grammar changes; extend the corpus instead of writing a frame harness.
- deletions of unreachable code or docs are proved by caller, build and link checks.
- builds and static checks do not prove product behavior. historical results apply
  only to their recorded source; removed, unavailable or unexecuted checks are never
  passes. tmux and phone operations need current-turn approval under
  [AGENTS.md](../AGENTS.md).

commands:

- `scripts/check {static|verify} [all|host|android|none]`: static runs formatting,
  whitespace and shell lint, plus go vet and module verification for
  host and terminal-page syntax and android lint for android (default `all`). verify
  adds the scoped go build and android debug assembly.
- ornament geometry and the launcher icon are frozen source with no drift gate. the
  catalogue has no pre-release check: the gateway loader rejects invalid or duplicate
  keys and names at startup; [dvergatal](dvergatal.md) owns admission rules.
- terminal vendor files are verbatim upstream npm and release bytes with no build
  step and no retained check; git tracks them, and
  [touch scroll](terminal-touch-scroll.md) records their sources, integrity and
  re-verify recipe.
- `scripts/release TAG` builds and signs every artifact from a fresh clone of exact
  canonical main, runs `scripts/check-release` on the result, then drafts a release;
  publication is separate. `scripts/check-release [--reproduce] [--source DIR]
  RELEASE_DIRECTORY TAG SOURCE_SHA` verifies artifacts against the public certificate
  and clean exact source; only `--reproduce`, run from that checkout, compares bytes,
  rebuilding with its own `scripts/build-host-binary` and `scripts/build-mac-icon`.
  `scripts/check published-release TAG SOURCE_SHA` reads a published release's
  source, assets, pin and hosted verify run.
- hosted ci runs `verify` on every pull request and main push, scoped by the changed
  paths in `.github/workflows/verify.yml`. a scoped pass proves only the checks it ran.

## docs

- docs state the current contract and are edited in place. dated evidence,
  qualification logs and superseded plans belong in pull requests and git history.
  update the docs that describe code you change in the same change.
- record each unresolved issue in `docs/issues/<short-name>.md`: problem, impact,
  evidence and what proves it resolved. delete it once resolved.
