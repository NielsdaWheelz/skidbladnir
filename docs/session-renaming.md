# Tmux session renaming

Status: **accepted implementation contract; source implemented**. This document
owns the phone-initiated tmux-session rename capability, delivery boundaries,
and red/green proof shape.
[`architecture.md`](architecture.md) owns final
behavior and acceptance; [`roadmap.md`](roadmap.md) owns ordering;
[`rules/testing.md`](rules/testing.md) is the testing standard. Do not create a
parallel `testing-standards.md`.

## Outcome and final state

From an active Terminal, the operator can rename the exact ordinary tmux
session being viewed. The tmux session is renamed in place. Its machine,
server lifetime, `$session_id`, phone attachment, panes, windows, processes,
clients, geometry, character, objective, options, and provider facts do not
change. Rename writes no activity fact; the next inventory independently
re-derives activity from tmux, so elapsed time or concurrent output may change
the projected value.

Tmux remains the database. The phone stores no alias, rename history, or second
name. Claude/Codex provider names remain independent and are never synchronized.

## Scope and closed decisions

- Add one terminal-header rename affordance and one focused rename sheet.
- Add one strict authenticated mutation:
  `PATCH /v1/sessions/{tmuxId}`.
- Target by pinned machine, server-lifetime token, `$session_id`, and expected
  current tmux name. The desired name is never authority.
- Keep the existing name grammar: 1–64 ASCII letters, numbers, underscores, or
  hyphens, beginning alphanumeric.
- Success is bodyless `204`, followed by one mandatory authoritative inventory
  read. Do not return or synthesize a card from the mutation.
- Do not retry a rename. Transport or unmodeled failure after dispatch is
  outcome-unknown and resolves only through inventory.
- An unchanged desired name is not a transition: the UI disables submission;
  a direct request returns `409 SessionNameConflict` without mutation.
- Rename one member of a tmux group without a Kill-style last-link guard.
- Keep the active phone terminal attached and retarget its presentation only
  after inventory proves the same machine/id/token under the observed name.
- Dashboard rename, Unicode/spaces, push eventing, and provider rename are not
  part of this slice.

Trade-offs are explicit:

| Choice | Cost accepted | Rejected alternative |
| --- | --- | --- |
| Terminal-first | Rename requires opening the session | Crowding the reviewed 170dp card or introducing a generic action menu for one action |
| `PATCH` + expected-name CAS | A committed request is not safely replayable | Action RPC, last-writer-wins, or using the mutable name as identity |
| `204` + reread | One read of latency before canonical UI state | A response DTO that can fail inspection after tmux already committed |
| ASCII grammar | Less expressive labels | Expanding quoting, ambiguity, accessibility, and protocol surface in this slice |
| Existing polling | External laptop rename may take up to one poll | A persistent tmux control-mode observer and reconnect/resnapshot subsystem |
| Strict same-name conflict | Callers cannot treat rename as convergent | Reporting success when no transition occurred |
| Transient detached-operation fence | One content-free handle-to-fence entry survives until authoritative inventory lands | Losing outcome-unknown convergence on Detach or persisting names/rename history |

No implementation question remains open.

## Product and capability contract

Terminal header:

```text
[Detach] [machine · current-tmux-name] [Kill]
         [Rename · presence]
```

The middle identity block is the visible Rename control. It keeps the exact
machine/name line and adds literal `Rename` before the existing presence line;
no icon, tooltip, long-press, overflow menu, ornament, or dependency is added.
It has `Role.Button`, a 48dp minimum target, existing angular pressed
indication, and at least 8dp separation from Detach and Kill. Its spoken label
is `Rename <tmuxName> on <machine>` and includes current presence as state.
It is enabled only when existing terminal actions are admissible.

Opening it resets one-shot terminal modifiers and opens a DeepSurface,
top-cut `ModalBottomSheet`; it sends no terminal bytes and does not detach.
The sheet contains:

- title `Rename tmux session`;
- context `<current tmux name> on <machine>`;
- single-line field `Tmux name`, prefilled with the exact current name;
- helper `1–64 letters, numbers, underscores, or hyphens`;
- inline literal error text;
- `Cancel` and `Rename` actions.

Disable autocorrect, capitalization, and smart punctuation. IME Done submits.
Draft and server error survive a definite rejection. Rename is disabled while
unchanged, locally invalid, stale, or pending. A submitting sheet cannot issue
a second mutation. No confirmation, success toast, haptic, ambient animation,
or undo exists; the changed header is the success result.

Android mirrors the server grammar only to admit the form; the server remains
authoritative. Add no contract codegen or compatibility normalization.

## Domain and API design

Domain input:

```go
type RenameInput struct {
    TmuxID        string // path-derived local $session_id
    TmuxName      string // exact expected current selector
    NewTmuxName   string // desired selector
    IdentityToken string // exact server/session lifetime
}
```

Wire contract:

```http
PATCH /v1/sessions/{tmuxId}
Authorization: Bearer <credential>
Skidbladnir-Machine: <pinned handle>
Content-Type: application/json

{
  "tmuxName": "expected-current-name",
  "newTmuxName": "desired-name",
  "identityToken": "server-lifetime-token"
}
```

- The body has exactly these three case-sensitive, non-duplicate required
  string keys. Unknown, alternate-case, duplicate, missing, `null`,
  wrong-typed, trailing, compressed, encoded, or oversized input uses the
  existing strict request failures.
- Empty expected name/token is `InvalidRequest`. Empty or invalid desired name
  is `SessionNameInvalid`.
- Authenticate and bind the machine before session lookup or tmux invocation.
- Success is `204 No Content` with an empty body.
- Reuse `SessionNameInvalid`, `SessionNameConflict`, `SessionNotFound`, and
  `SessionIdentityMismatch`. Hard-cut the latter's public message to
  `The session changed. Refresh and try again.` everywhere.
- Add `PATCH` to the closed request-method logger. The ordinary request event
  records method, route, status, duration, and error code only. Add no rename
  event and log no old/new name, token, body, provider fact, or terminal fact.
- There is no `POST .../rename`, old field name, alias, version branch,
  compatibility reader, or response schema.

## Tmux mutation and concurrency

`Manager.Rename` is one unreplayable single mutation under the existing
`Manager.mutations` lock. It reuses one shared session-mutation identity owner
for Rename and Kill:

```text
server epoch + server PID + server start time
+ exact $session_id
+ exact expected session_name
```

The manager validates the desired name, token, current identity, and ordinary
non-phone-shadow target. One tmux client command then queues:

```text
if exact lifetime/id/expected-name/ordinary-session
  -> rename-session -t '$session_id' 'new-name'
  -> success marker
else
  -> identity-mismatch marker
```

Rules:

- Target only by `$session_id`; never by current or desired name.
- Escape expected values through the existing tmux format-literal owner. The
  desired name is a validated command token; never invoke a shell.
- Keep predicate and rename in one tmux command queue. No Go preflight may
  authorize a later unguarded rename.
- Let tmux's native session-name index linearize destination uniqueness. On a
  failed rename, authoritative source/destination rereads classify conflict,
  stale identity, disappearance, or defect; stderr text is not a protocol.
- Two renames from one expected name yield at most one success. Two sessions
  racing for one destination yield at most one success. Neither loser mutates.
- A server restart, id reuse, external rename, source disappearance, internal
  shadow, or invalid token cannot reach the rename branch.
- The reserved phone-shadow namespace stays two-factor: when either expected
  or desired name has the reserved shape, the conditional queue also requires
  that the internal shadow marker is absent. A marker-only ordinary session
  cannot be promoted into reconciliation-owned shadow state by Rename.
- Before returning a definitive destination conflict after failure, reread the
  source id/name/token/server/marker after the destination observation. A
  concurrent source change is identity mismatch, never a false conflict.
- Group topology is not an eligibility predicate: session names belong to
  individual group links and rename changes no shared pane/window.
- Same-name rejection is evaluated against the exact current identity and
  mutates nothing.

## Intra-system composition and Android state

```text
Terminal Rename control
  -> terminal-local RenameState(target, draft, phase, error)
  -> existing per-machine InventoryOperationLane.submitMutation
  -> mark inventory Superseded at the reserved fence
  -> GatewayClient PATCH (retryOnConnectionFailure remains false)
  -> gateway auth + strict DTO
  -> Manager lock + one conditional tmux rename
  -> 204 or typed/unknown failure
  -> existing awaited GET /v1/sessions lane
  -> authoritative same-id/token session
  -> replace Terminal.machine + Terminal.target; preserve attempt/WSS/page
```

Use a closed rename phase (`Editing | Sending | Reconciling`), not parallel
booleans that permit impossible state. Keep this terminal-local; do not add a
generic mutation/recovery framework.

- `Sending`: sheet is visible, fields and dismissal are locked, while the
  existing attachment and terminal output continue unchanged.
- `204`: enter `Reconciling` and require a later inventory read; do not
  optimistically change the name.
- Invalid/conflict: return to `Editing` with the draft and inline error; because
  tmux did not mutate, clear the reserved fence through the existing owner.
- Not-found/identity-mismatch: no mutation, but require inventory before any
  further action because the captured target is stale.
- Transport/InternalError: outcome unknown; enter `Reconciling`, show
  `Rename outcome unknown. Checking tmux.`, never resend, and require inventory.
- During reconciliation the sheet may be dismissed after the HTTP call ends;
  the terminal-local reconciliation record survives until inventory resolves
  or the terminal exits.
- If inventory finds the same id/token at the desired name, replace the whole
  session from inventory, close rename UI, and retain the existing attachment,
  attempt, WebView, modifiers-off state, and terminal geometry.
- If the same id/token has another name, adopt that authoritative target. If
  the sheet remains open, preserve the desired draft and show a stale-edit
  error. Never overwrite the later writer.
- If id/token is absent or replaced, discard rename state and use the existing
  reconnect-required path. Do not close a healthy attachment merely because
  only the name changed.
- Detach while a dispatched rename is unresolved does not cancel or replay the
  request. A transient content-free handle-to-mutation-fence entry survives the
  terminal-local state; the controller-lifetime per-machine operation lane
  orders the authoritative read. Dashboard inventory eventually shows tmux
  truth. No name, target, draft, response, or history is copied into that owner.

## Reuse, hard cut, and cleanup

- Replace `validateOptionalTmuxName` with one required `validateTmuxName`;
  Create calls it only when its optional value is present, Rename always calls
  it. One regex and one error literal remain.
- Extract the shared lifetime/id/name tmux predicate from Kill; Kill adds its
  existing group guard, Rename adds its ordinary-session guard. Delete the old
  kill-only helper name; keep Kill behavior unchanged.
- Extract one exact session-path parser for PATCH and DELETE.
- Add one bodyless-response client helper and move Kill and Rename to it.
- Reuse the manager mutation lock, identity-token parser, format escaping,
  strict JSON decoder, typed error mapper, per-machine Android mutation fence,
  awaited inventory read, terminal action admission, sheet shape, colors,
  keyboard rules, and accessibility floor.
- Do not generalize session metadata, mutation commands, sheets, buttons,
  recovery, or eventing beyond these present second consumers.
- Delete every superseded kill-specific public identity-error literal and test
  assertion in the same hard cut. Keep no legacy API or compatibility branch.

## Red / green / refactor and 80/20 verification

Before production edits, run a green routine baseline. Each builder then owns
and observes its behavioral red; a compile failure is not the red.

### Red

1. **Go HTTP/tmux:** one authenticated isolated-socket journey sends PATCH to
   the current server and observes the unsupported behavior. It then owns the
   final API and tmux assertions: success + GET truth, invalid input, same-name,
   collision, stale name/token, disappearance, two races, and live attachment
   survival. Assert preserved id/token, pane/window/PID/options/group/client
   facts through supported tmux/session surfaces. Do not duplicate every field
   at every layer.
2. **Android state/client:** one JVM proof owns exact PATCH encoding, bodyless
   success, definitive versus outcome-unknown classification, mutation fencing,
   mandatory reread, no retry, same-lifetime target replacement, and retained
   terminal attempt/connection.
3. **Compose:** one separately approved, reversible S22+ instrumentation
   workflow runs one real-runtime terminal component proof and first observes
   no Rename affordance. Final assertions cover visible/spoken action, 48dp
   and 8dp geometry, prefilled field, unchanged/invalid/pending admission,
   inline error, IME Done, and Detach/Kill/name/presence retention. Query by
   text/role/label; add no test-only production seam. The workflow installs a
   same-version signed test build without clearing data, proves the encrypted
   fleet unchanged, removes the test package, and restores the captured APK's
   exact digest, signer, and version on every exit path.

### Green

Implement only the contracts above in ownership order. Do not weaken a red,
mock an internal boundary, or claim an unrun tmux/device proof.

### Refactor

Delete superseded helpers/literals and perform only the named second-consumer
extractions. `git diff --check`, residue searches for old literals/routes, and
all focused tests must be clean before routine verification.

### Gates

- Routine `scripts/test verify`: static/build/pure regressions.
- The same separately approved isolated `integration` invocation once on
  Linux and once on Darwin: the real tmux + authenticated HTTP proof, each on
  its own `-L` socket.
- The separately approved focused S22+ workflow above, plus one hands-on
  portrait/large-text/Gboard glance confirming readable header, modal focus,
  and uninterrupted terminal attachment. This is focused component evidence,
  not the release-bound `platform` gate.
- Platform, provider-live, deployment, live-host, release, published-release,
  product, and second-phone gates remain `NOT_RUN`: their boundaries are
  unchanged.

This is the 80/20 boundary. More unit permutations, a full product journey,
screenshots, performance tests, or event-latency proofs add cost without owning
a new risk in this slice.

## Acceptance criteria

1. A fresh exact target renames once and the next inventory exposes only the
   new tmux name with unchanged machine/id/token and unchanged session facts.
2. The active terminal stays connected; its title retargets only after the
   authoritative read, and later Kill/reconnect uses the new target.
3. Invalid, unchanged, conflicting, stale, missing, restarted-server, and
   concurrent-loser requests mutate no session.
4. Unknown transport outcome is never retried and converges through inventory.
5. Provider session id/name, character, objective, panes, processes, group
   topology, clients, selection, and geometry do not change. Rename authors no
   activity state; the next inventory remains authoritative.
6. The UI is literal, discoverable, keyboard-safe, TalkBack-addressable, and
   usable at the supported narrow width and large font scale.
7. Logs and evidence contain no request body, names, token, provider fact,
   terminal bytes, prompt, objective, credential, or account data.
8. Only the new strict route/schema exists; no alias, fallback, flag, old
   literal, duplicate state owner, or compatibility path remains.
9. Routine verification and every approved external gate are reported against
   the exact tested SHA; every unapproved gate remains `NOT_RUN`.

## Work split and files

Work is sequential at the named seams. No two builders edit one path.

| Order | Owner | Exclusive paths | Owned proof |
| --- | --- | --- | --- |
| 0 | Root integrator | `docs/session-renaming.md`, `docs/architecture.md`, `docs/roadmap.md` | Scope and acceptance only |
| 1 | Go rename builder | `internal/sessions/{types.go,validation.go,manager.go}`, `internal/tmux/{client.go,attachment.go,client_test.go}`, `internal/gateway/{gateway.go,dto.go,dto_test.go}`, `internal/logging/{logger.go,logger_test.go}`, `internal/sessions/validation_test.go`, `tests/integration/{sessions_test.go,gateway_test.go}` | Go HTTP/tmux red and focused greens |
| 2 | Android rename builder | `android/app/src/main/java/dev/niels/skidbladnir/{ProductModel.kt,GatewayClient.kt,SkidbladnirController.kt,TerminalScreen.kt}`, new `android/app/src/main/java/dev/niels/skidbladnir/SessionRename.kt`, `android/app/src/test/java/dev/niels/skidbladnir/{ProductContractTest.kt,MultiMachineContractTest.kt}`, `android/app/src/androidTest/java/dev/niels/skidbladnir/TerminalChromeInstrumentedTest.kt` | One observed Compose red, then Android contract/reconciliation and real-Compose greens |
| 3 | Read-only verifier | none | Diff/rules/residue/gate review; writes no test or production file |

Only the root integrator changes canonical docs. No owner changes `catalog/`,
`scripts/test`, hooks, host/deployment files, terminal protocol/WebView/key
deck, dashboard/card/Forge files, pressure, pairing, storage, build files, or
another owner's tests.

## Non-goals

Dashboard rename; machine/provider/conversation/dwarf/objective/profile rename;
aliases; previous-name lookup; undo/history/audit ledger; automatic or AI names;
transcript, prompt, terminal, git, cwd, or provider parsing; Unicode/spaces;
tags, groups, favorites, search, resurrection, archive, persistence, offline
queue, idempotency key, durable receipt, retry, push/control mode/hooks,
analytics, generic metadata PATCH, generic action sheet/button/mutation
framework, terminal reconnect redesign, attachment recreation, new dependency,
release/deployment/publication work, or changes to Kill/Detach semantics.
