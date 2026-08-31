# Automatic dwarf identity

Status: implemented, 2026-08-26; routine verification and isolated real-tmux
acceptance green (see [`roadmap.md`](roadmap.md)). The final behavior is cut
into [`architecture.md`](architecture.md); this document remains the
historical scope and red/green plan.

Historical boundary: the accepted 2026-08-31
[`terminal-activity.md`](terminal-activity.md) hard cut supersedes this plan's
gateway-owned product ordering and status-era names. Android owns the one
global freshness/activity order; the gateway derives the flat activity fact
and otherwise orders only by tmux name/id. Conflicting examples below are
historical, not active contracts.

Normative rules: [`rules/index.md`](rules/index.md), especially
[`rules/testing.md`](rules/testing.md). Architecture guardrails remain binding.

## Goal

Every visible ordinary tmux session has one persistent Dvergatal character,
regardless of whether the laptop or Skíðblaðnir created it. Dwarf identity is
independent of the tmux session name. Assignment is automatic and has no UI.

```text
tmux server lifetime + $session_id  exact ephemeral identity
tmux session_name                   operational name
@skid_character                     persistent dwarf character key
catalogue[key]                      display name + procedural portrait
```

The dwarf belongs to the tmux-session lifetime. It survives gateway/app
restart, tmux rename, and foreground-process replacement; it disappears with
the session. It is not an agent, process, conversation, objective, profile, or
origin identity.

## Capability contract

`Manager.List` is an inventory query with one declared lazy-normalization
effect: after phone-shadow reconciliation, it assigns a valid
`@skid_character` to each visible session whose current value is absent or
invalid. A successful inventory contains a valid character for every returned
session. It never fabricates a response-only character.

- Preserve every valid existing character.
- Exclude every session still classified as a phone shadow.
- A reclaimed last-link shadow becomes ordinary and is assigned in that list.
- Persist only the catalogue key. Derive all presentation from the catalogue.
- Never read terminal bytes, infer agenthood, or infer other metadata.
- Never rename, resize, retarget, attach, detach, signal, or kill a session.
- Serialize with the existing `Manager.mutations` lock.
- Linearize each assignment in one tmux client queue guarded by exact server
  epoch/PID/start time, `$session_id`, expected raw option value, and absence of
  the phone-shadow marker. The tmux name is not an assignment predicate.
- On a lost conditional race, re-read once: accept a now-valid value, omit a
  vanished session, or make one guarded attempt against the newly observed raw
  value. An unresolved extant session is an internal failure. Do not return a
  neutral card or retry forever.
- Never log the raw prior option value.

This is the sole automatic assignment path. Do not add a worker, timer, hook,
startup migration, Android cache, sidecar, or mutation endpoint.

## Allocation and naming

Character allocation and tmux-name allocation are separate functions.

### Character

Count valid characters on visible ordinary sessions. Choose among the
least-used catalogue entries; existing externally assigned duplicates remain
untouched. Break ties with a stable pseudorandom score:

```text
score = SHA-256(seed + NUL + character.key)
winner = lexicographically greatest score
```

- Detected-session seed: server epoch + NUL + `$session_id`.
- Create seed: the already minted random server-epoch candidate.
- Normalize a sorted worklist of unassigned session ids and increment usage
  only after each committed assignment. This worklist does not reorder the
  manager's returned inventory; the gateway alone owns wire-grid ordering.

The policy is content-free, random-looking, deterministic for a pending
assignment, balanced across the live inventory, and dependency-free. Once
written, the key—not the algorithm—is authoritative.

### tmux name

`POST /v1/sessions` keeps an explicitly supplied tmux name. Otherwise it uses
the smallest free positive suffix:

```text
skidbladnir-<profile>-<N>
```

Examples: `skidbladnir-work-1`, `skidbladnir-claude-work-2`. Existing `ga-*`
sessions are not renamed; their names are now ordinary operator-owned tmux
names. New code contains no dwarf-derived tmux-name path.

## Composition

```text
GET /v1/sessions
  -> sessions.Manager.List
  -> reconcile owned phone shadows
  -> scan visible session ids/options
  -> tmux exact conditional character assignment
  -> inspect persisted character before fallible pane facts
  -> gateway required DTO
  -> Android required model
  -> existing procedural portrait

POST /v1/sessions
  -> validate request
  -> choose tmux name and character independently
  -> existing single create command queue
  -> inspect and return the persisted session
```

Ownership stays where it is:

- `catalog`: validates catalogue keys/names and resolves a key.
- `tmux.Client`: owns the exact conditional tmux command and format escaping.
- `sessions.Manager`: owns allocation, normalization, create, and inventory.
- `gateway`: maps the required domain value and owns the sole wire-grid sort;
  it owns no assignment policy.
- Android: decodes and renders; it owns no fallback or assignment state.

Do not add a package, service, registry, generic metadata setter, or duplicated
inventory scan when a narrow extraction from the existing scan suffices.

## Hard-cut schemas

The five routes remain the complete API. No route is added.

```text
SessionCard {
  id: string
  tmuxName: string
  identityToken: string
  character: { key: string, displayName: string } // required
  profile?: string
  objective?: string
  cwd?: string
  activeCommand?: string
  attachedClients: integer
  attention: boolean
  status: Status
}

POST /v1/sessions {
  cwd: string
  profile: string
  optionalTmuxName?: string
  objective?: string
}

DELETE /v1/sessions/{id} {
  tmuxName: string
  identityToken: string
}
```

Hard rename domain and wire fields in one cut:

- `Session.Name` -> `Session.TmuxName`
- `CreateInput.OptionalName` -> `CreateInput.OptionalTmuxName`
- `KillInput.DisplayedName` -> `KillInput.TmuxName`
- JSON `name` -> `tmuxName`
- JSON `optionalName` -> `optionalTmuxName`

Do not accept aliases, dual payloads, nullable `character`, or omitted
`character`. Keep existing session-name validation/error codes; tmux itself
calls this value a session name. Objective remains create-time-only.

Make the domain character one required value, preferably
`catalog.Character`, instead of parallel `CharacterKey` and
`CharacterDisplayName` strings. The catalogue owns that invariant. The API
contains no image/color fields; Android derives them from `character.key`.

Android adds no action, sheet, picker, rename, or objective editor. Keep the
current card hierarchy and portrait. Remove the neutral portrait and nullable
character branches because a successful same-system payload cannot contain
them. Rename the existing Forge label to `tmux name (optional)`.

## Hard-cut cleanup

Delete, do not deprecate:

- `catalog.Character.NameSuffix` and suffix length/uniqueness checks.
- Suffix-as-runtime-identity checks in `scripts/check-catalogue`.
- `generatedIdentity` and all `ga-<dwarf>[-N]` generation.
- Split `CharacterKey`/`CharacterDisplayName` state and incomplete-pair guards.
- Nullable/omitted API and Android character paths.
- `NeutralSessionPortrait` and `tmux session` character fallback copy.
- Any manager-level grid sort or `statusRank`; retain the gateway's one
  `listSessions` sort and `sessionPriority` as the sole wire-grid-order owner.
- Old DTO names, compatibility decoders, fixtures, comments, and tests.
- Helpers made single-use or dead by the cutover.

Keep `catalog/characters.json` and its provenance inventory unchanged. Do not
rename already-running tmux sessions. `scripts/test` composition does not
change.

## Acceptance criteria

1. First successful inventory of a laptop-created visible session persists and
   returns one valid character.
2. Missing and invalid character options converge; valid options never change.
3. Repeated inventory, gateway reconstruction, tmux rename, and process
   replacement preserve the character.
4. Character selection is independent of tmux naming; custom names and
   generated `skidbladnir-<profile>-<N>` names receive the same allocation
   policy.
5. Automatic allocation chooses only a least-used character and balances
   multiple unassigned sessions in one inventory.
6. A concurrent valid assignment is preserved. Server restart, session
   disappearance, or id mismatch cannot write to another session.
7. Assignment changes only `@skid_character`; name, pane/window ids, pane PID,
   group topology, clients, selection, geometry, and other options are
   unchanged.
8. Phone shadows are neither returned nor assigned; a shadow made ordinary is
   assigned when first returned.
9. Every successful session/create response uses `tmuxName` and a required
   character; Android rejects any other same-system shape.
10. No character-management UI or API exists, and active source/docs contain
    no dwarf-derived tmux-name implementation.

## Red / green / refactor

Each builder writes its owned behavioral proof and observes it fail before
touching production files.

### Red

- Catalogue unit: suffix identity is no longer part of the runtime character.
- Sessions pure unit: selection is least-used, stable for a seed, and balanced
  across sequential assignments; generated names use the new namespace.
- Real-tmux integration: a laptop session without/with invalid metadata is
  returned with a persisted stable character while all non-character facts are
  byte-for-byte unchanged; concurrent valid assignment wins.
- Gateway integration: inventory/create expose required `character` and
  `tmuxName`; create and kill use the hard-cut request fields.
- Android contract: required character and hard-cut names decode/encode; the
  model has no neutral character state.

Do not write tests whose only purpose is preserving rejection of a dead legacy
payload. Strict current-shape decoders and the absence of compatibility code
are sufficient.

### Green

Implement only enough to pass the owned red proofs. Reuse the existing
mutation lock, server identity, tmux format-literal escaping, catalogue lookup,
create queue, inventory scan, DTO mapping, and portrait renderer.

### Refactor

- Collapse name/character collection into one narrow session-scan owner used by
  List and Create; do not keep parallel tmux enumerators with the same facts.
- Keep allocation pure and tmux mutation effectful.
- Move character observation before fallible pane inspection so `UNKNOWN`
  cards still satisfy the required-character contract.
- Keep the sorted normalization worklist separate from returned inventory;
  retain exactly one presentation sort at the gateway wire boundary.
- Remove all hard-cut dead symbols and duplicated validation.
- Run targeted searches for `NameSuffix`, `generatedIdentity`, `ga-`,
  `OptionalName`, `CharacterKey`, `CharacterDisplayName`, and
  `NeutralSessionPortrait`; removed symbols must have no active hits and `ga-`
  may remain only in explicit no-rename/historical prose.

## Non-overlapping work

| Owner | Paths | Proof |
| --- | --- | --- |
| Root contract/catalog | `docs/architecture.md`, `docs/roadmap.md`, `internal/catalog/catalog.go`, `internal/catalog/catalog_test.go`, `scripts/check-catalogue` | catalogue unit + checker |
| Session/tmux builder | `internal/sessions/manager.go`, `internal/sessions/types.go`, `internal/sessions/validation.go`, their owned tests, `internal/tmux/client.go`, `internal/tmux/client_test.go`, `tests/integration/sessions_test.go`, compile-only adjustments in `tests/integration/terminal_test.go` | pure allocation + one isolated real-tmux journey |
| Gateway builder | `internal/gateway/dto.go`, `internal/gateway/dto_test.go`, `internal/gateway/gateway.go`, `tests/integration/gateway_test.go`, `internal/logging/logger_test.go` | one authenticated HTTP journey over the real session manager |
| Android builder | `ProductModel.kt`, `DashboardScreen.kt`, `TerminalScreen.kt`, `SkidbladnirController.kt`, and `ProductContractTest.kt` under the existing Android package paths | one protocol/model contract proof |
| Root integrator | cross-slice search, canonical docs, gate composition only if required | combined gates; no new test |
| Verifier | read-only review of accepted behavior and evidence | writes no production or test file |

The contract/catalog cut lands first. Session/tmux establishes the domain.
Gateway and Android may then proceed in parallel. Only the root integrator
changes canonical scope docs or test-gate composition.

## 80/20 verification

1. Baseline and final `scripts/test verify`: static, build, pure unit only.
2. Run each new pure/Android contract test red, then green.
3. Run the existing isolated `integration` gate once red and once after the
   combined green, only with explicit user approval in that implementation
   turn.
4. No new terminal, device, provider, deployment, or live behavior exists;
   `platform` and `live` are not acceptance gates for this slice and remain
   `NOT_RUN` unless separately approved and requested.
5. Record ordinary command output only. Add no proof ledger, evidence schema,
   generated contract, or compatibility matrix.

## Non-goals

- Renaming any existing session or adding post-create rename.
- Editing objectives or any other session metadata.
- User-selected characters, custom images, colors, tags, or assignment UI.
- Persisting a persona beyond the tmux-session/server lifetime.
- Agent/conversation identity, origin, provenance, transcript parsing, or
  profile/objective inference.
- Background reconciliation, offline queues, retries with durable state,
  SQLite/JSON state, hooks, notifications, or orchestration.
- Migration of `ga-*` names, backward-compatible API fields, fallbacks, or
  dual old/new behavior.
