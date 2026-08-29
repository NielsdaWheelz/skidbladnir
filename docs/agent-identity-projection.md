# Agent address and runtime identity projection

Status: **accepted implementation contract; active**. The 2026-08-28 scope and
acceptance change is incorporated into [`architecture.md`](architecture.md)
and sequenced by [`roadmap.md`](roadmap.md). This file owns the detailed
non-overlapping implementation boundary.

Normative rules: [`rules/index.md`](rules/index.md), especially
[`rules/testing.md`](rules/testing.md). The requested testing standard is that
file; do not create a parallel `testing-standards.md`.

## Outcome

For the current pane of every visible ordinary tmux session, inventory exposes:

- exact machine-bound tmux address facts: tmux id, tmux name, and existing
  server-lifetime identity token;
- an optional exact foreground-agent projection: `Codex | Claude`, PID,
  profile when proven, provider session id when registered, and provider
  session name when observable;
- the same behavior for gateway-managed launches and laptop launches inside
  the observed tmux server.

Unknown facts are omitted. They are never guessed, read from transcripts, or
recovered from a second registry. This slice adds no message transport, prompt
tool, queue, wake, resume, rename, or agent-to-agent action.

## Terms and address

```text
machineHandle + identityToken   exact current tmux-session lifetime target
tmuxId                          local tmux $session_id
tmuxName                        mutable human selector within one machine
agent PID + kernel start id     exact foreground process lifetime
provider session id             provider-owned conversation/session identity
provider session name           optional provider-owned human name
```

`SessionAddress` is the existing client-composed machine target plus the
hard-renamed tmux fields; it is not another persisted or wire string. Human
notation is `<machine>/<tmuxName>`, but only a fresh inventory target is safe
for mutation. Provider ids are facts, not Skíðblaðnir handles or authority.

One card still represents one tmux session and its current pane. Non-current
windows/panes do not become independent inventory rows.

## Architecture

Tmux remains the database. Add one pane option:

```text
@skid_agent_runtime =
  v1:<pid>:<kernel-start-id>:<Codex|Claude>:<profile-key|->:<session-id-b64url>
```

- The option is written only by a provider `SessionStart` hook.
- PID/start id/provider must match the current exact foreground runtime before
  any registered field is returned. A stale, malformed, nested, wrong-pane, or
  wrong-provider value is ignored, not repaired.
- Profile is the unique configured profile whose provider and provider-home
  environment value match inside the hook process. `-` means unproven.
- Provider session id is bounded, safe opaque text from the documented hook
  field, base64url-encoded without padding. The option contains no name,
  prompt, cwd, model, transcript path, token, credential, or socket.
- Inventory is read-only for this capability. It never clears or rewrites a
  stale registration. Pane lifetime removes it naturally; a later valid
  `SessionStart` replaces it.

Replace the Codex-only `status-hook` with one closed `agent-hook` command:

```text
skidbladnir agent-hook --host-config=PATH Codex SessionStart
skidbladnir agent-hook --host-config=PATH Codex UserPromptSubmit
skidbladnir agent-hook --host-config=PATH Codex Stop
skidbladnir agent-hook --host-config=PATH Claude SessionStart
```

No other provider/event pair is valid. Codex keeps its existing lifecycle
effects. Only `SessionStart` parses bounded content-free identity input;
prompt-bearing events are drained exactly as today. Claude gains identity
registration only, not lifecycle or attention semantics.

```text
provider hook
  -> validate closed provider/event and bounded SessionStart input
  -> resolve exact inherited pane plus provider-runtime ancestry/foreground origin
  -> match zero-or-one configured profile from inherited allowlisted env
  -> one tmux command writes @skid_agent_runtime
  -> Codex event may also write existing @skid_lifecycle

GET /v1/sessions
  -> existing tmux scan and active-pane anchor
  -> one ObserveForeground call
  -> classify exact provider/PID from configured signatures
  -> accept matching @skid_agent_runtime fields
  -> derive Claude name from the live CLI argv when explicit
  -> derive status from the same observation
  -> gateway DTO -> Android model/card context
```

The shared foreground observation is the single owner for status and runtime
identity. Do not add a second process scan, `/proc/*/environ` reader, polling
worker, sidecar, SQLite/JSON database, provider registry reader, or App Server.

## Source policy

| Fact | Source | Rule |
| --- | --- | --- |
| tmux id/name/token | Existing tmux scan | Required for every returned session |
| provider/PID/start id | Existing native foreground observer | Provider/PID returned only on one exact configured match; start id stays internal |
| launch profile | Existing session `@skid_profile` | Rename its API meaning to `launchProfile`; never claim it is the current runtime profile |
| runtime profile | Matching live registration | Match the hook's own provider-home variable; omit on zero matches |
| provider session id | Provider `SessionStart.session_id` | Omit without a valid live registration |
| provider session name | Exact live provider argv | Claude `-n`, `--name value`, or `--name=value`; otherwise omit |

Managed Claude launches insert `--name <tmuxName>` before configured arguments
after rejecting a configured `-n`/`--name` conflict, so their initial provider
name maps exactly to tmux.
Codex has no in-scope startup/read surface for its task name; Codex name is
therefore absent. Laptop Claude names appear only when the live invocation
contains the documented name flag. Laptop profiles/ids appear only when that
profile's installed hook runs.

## Host configuration

Hard-cut every profile row to require:

```json
{
  "key": "work",
  "label": "Codex · Work",
  "provider": "Codex",
  "command": "/absolute/launcher",
  "environment": [{"name": "CODEX_HOME", "value": "/absolute/home"}],
  "foregroundSignatures": [{"executableBase": "codex"}],
  "arguments": []
}
```

`provider` is exactly `Codex | Claude`. Every Codex profile declares exactly
one `CODEX_HOME`; every Claude profile declares exactly one
`CLAUDE_CONFIG_DIR`; values are absolute and unique within that provider.
Identical foreground-signature rows cannot be shared across providers. Because
the version-stable Codex basename and Claude argv[0] predicates can overlap in
the abstract, any concrete process matching more than one provider is
unclassified. Provider-home values are compared only inside the hook, never
returned or logged. The public profile choice DTO also gains required
`provider`; labels are presentation, never parsed.

Deployment replaces every Codex profile's hook file atomically with the
CLI/config cut. Codex's changed hook digest still requires its native
review/approval. The existing Claude profile router adds one deployment-owned
local plugin directory whose native `SessionStart` hook merges with user hooks;
deployment does not overwrite Claude settings. Neither path edits or bypasses
a provider trust store. A direct raw-Claude-binary launch bypasses that plugin
and remains honestly unregistered.

## Domain and API contract

Use one shared `internal/agentruntime` owner for provider/profile validation,
foreground classification, registration encoding/acceptance, and
provider-specific argv rules. Provider-specific branches stay inside that
module.

```text
AgentRuntime {
  provider: Codex | Claude
  pid: positive integer
  profile?: ProfileKey
  providerSession?: ProviderSessionFacts // at least id or name
}

ProviderSessionFacts {
  id?: 1..128 visible ASCII bytes
  name?: 1..128 NFC Unicode scalars without controls or bidi formatting
}
```

The domain representation must make an empty `ProviderSessionFacts` invalid.
JSON uses omitted keys, never `null`. `agent` is absent when no supported exact
foreground runtime is proven.

No route is added. Hard-cut the existing inventory and create response:

```json
{
  "machine": {"handle": "mh-...", "platform": "Linux"},
  "observedAt": "...",
  "profiles": [
    {"key": "work", "label": "Codex · Work", "provider": "Codex"}
  ],
  "sessions": [{
    "tmuxId": "$1",
    "tmuxName": "ga-worker",
    "identityToken": "v1-...",
    "launchProfile": "work",
    "agent": {
      "provider": "Codex",
      "pid": 1234,
      "profile": "work",
      "providerSession": {"id": "019..."}
    },
    "character": {"key": "norse.durinn", "displayName": "Durinn"},
    "attachedClients": 0,
    "attention": false,
    "status": {"kind": "Running", "signal": "Process", "signalAt": "..."}
  }]
}
```

Hard renames in the same release:

- domain/wire `id` -> `tmuxId`;
- domain/wire `profile` -> `launchProfile`;
- Android `AgentSession` -> `TmuxSession`, `AgentTarget` -> `SessionTarget`,
  `VisibleAgent` -> `VisibleSession`, and `AgentCard` -> `SessionCard`;
  `agent` thereafter means only `AgentRuntime`;
- Android `agent-*` test tags -> `session-*`.

The route `/v1/sessions/{tmuxId}` remains. Do not accept old keys, old hook
commands, providerless profile rows, aliases, nullable fields, dual option
formats, or compatibility decoders.

## Product behavior

Tmux name remains the card's primary identity. The existing footer becomes:

```text
Known runtime profile:  machine? · configured profile label
Unknown runtime profile: machine? · provider · profile unknown
No agent:                machine? · launch profile label, else profile unknown
```

When an agent exists but runtime profile is absent, render
`<provider> · profile unknown`; do not substitute launch profile. Provider
session id/name and PID remain available to the typed product model and API,
but do not add a card row, sheet, badge, copy action, or raw-id UI in this
slice. Existing height, layout, status, attention, actions, and accessibility
structure remain unchanged.

## Capability rules

- Managed and laptop launches use the same observation/registration path.
- Provider registration never changes status truth: missing hooks still yield
  `RUNNING`, not a guessed lifecycle.
- A process replacement immediately loses stale runtime profile/id; provider,
  PID, and an observable Claude name may still come from the new live process.
- A tmux rename changes only the human selector. Exact target identity and
  provider session facts remain unchanged for their valid lifetimes.
- Provider session ids and names are not unique keys. Resume, fork, or grouped
  tmux views may repeat them; no decoder, index, or future-address assumption
  may require uniqueness.
- Hook absence, provider name absence, raw direct launch, or unconfigured
  profile is normal successful absence. An identical cross-provider signature
  is a host-config defect rejected at startup; a concrete process matching
  distinct provider predicates is omitted fail-closed.
- Never read process environments from another process. The hook may compare
  only its own inherited, config-declared keys.
- Provider command hooks may execute in a provider-owned tty-less subprocess
  session. Accept only when an exact supported runtime ancestor is the inherited
  tmux pane's foreground process lifetime; the helper transport is never the
  identity origin.
- Never log hook input, provider id/name, profile-home values, argv, prompts,
  objectives, transcript paths, or terminal bytes.

## Hard-cut cleanup

Delete, do not deprecate:

- `internal/statushook`, the `status-hook` CLI, its usage text, tests, and
  deployment invocation;
- session/profile/id names made ambiguous by the new runtime model;
- manager-local agent-signature matching and hook-local duplicated Codex
  origin logic after `internal/agentruntime` owns them;
- old API fixtures/decoders, old Android types/selectors, providerless host
  config fixtures, and compile-only compatibility residue;
- any new abstraction left with only one speculative consumer.

Retain `@skid_profile` as the tmux-session launch fact, existing
`@skid_lifecycle`, the native process observer, tmux option readers, strict JSON
boundaries, and the current card shell. Do not migrate or rename running tmux
sessions.

## Acceptance criteria

1. Every returned row has exact `tmuxId`, `tmuxName`, and identity token; the
   Android-composed target remains machine-bound and detects a stale rename.
2. A managed Codex and managed Claude launch expose the exact provider/PID;
   shipped hooks add exact runtime profile and provider session id.
3. Managed Claude additionally exposes provider name equal to its tmux name;
   managed Codex honestly omits provider name.
4. A laptop Codex/Claude launch inside visible tmux gets the same fields when
   its configured hook runs; a raw/unhooked launch returns only facts proven by
   process observation.
5. Explicit laptop Claude name flags map exactly. Unnamed/resumed Claude and
   all Codex names remain absent without provider lookup.
6. Registration from another pane, nested runtime, process lifetime, provider,
   or malformed input contributes no profile/id to inventory.
7. Process replacement and tmux/server restart cannot inherit stale agent
   identity. Tmux rename preserves valid provider identity.
8. Runtime profile never falls back to `@skid_profile`; launch and runtime
   profile remain separately and honestly named.
9. One foreground observation supplies both status and agent projection; no
   duplicate scan or contradictory provider/status result exists.
10. Gateway and Android accept only the new schema; no old field, command, or
    providerless config path remains active.
11. Card context names provider/runtime profile without adding geometry or
    displaying raw ids/PID. Existing open/detach/kill behavior is unchanged.
12. No communication endpoint/tool, provider API, transcript/config-store
    parser, background worker, or non-tmux process inventory exists.

## Red / green / refactor

Each builder writes its owned behavioral proof and observes an assertion red
before production edits. A rename-caused compile failure is not the red proof.

### Red

1. **Runtime unit:** closed provider/profile validation, exact classification,
   registration round-trip/lifetime rejection, and Claude name/launch argv.
2. **Hook/host boundary:** unit proof owns required provider config, the closed
   provider/event matrix, and bounded SessionStart parsing; one isolated
   hook-to-pane case proves only the exact content-free tmux publication.
3. **Session real-tmux journey:** test-owned live observations plus directly
   installed registration fixtures prove inventory mapping, stale rejection,
   rename/process replacement, and no unrelated tmux mutation.
4. **Gateway HTTP journey:** authenticated inventory/create expose only the
   hard-cut DTO and all legal absence combinations.
5. **Android contract/component:** strict decode of provider id/name presence,
   machine-bound address, session terminology, and exact footer content for
   agent/profile absence. Move existing coverage; do not duplicate it.
6. **Deployment topology:** deployment tests prove the exact installed host
   configuration, Codex hooks, and Claude plugin/router topology without
   launching a provider.
7. **Provider live:** one separately approved integration proof uses the
   production session manager plus the fixed installed commands and hooks for
   one managed and one laptop launch across the Linux/Darwin sample matrix.

### Green

Implement only those proofs. Reuse the process observer, exact tty/session
ancestry guard, tmux option access, profile table, strict decoders, existing
inventory endpoint, and card footer.

### Refactor

- Observe foreground once and pass the typed result to status/runtime derivation.
- Move profile/provider primitives and provider branches to one runtime owner.
- Collapse Codex lifecycle plus identity publication into one tmux queue on
  `SessionStart`.
- Search for `status-hook`, `statushook`, `AgentSession`, `AgentTarget`,
  `VisibleAgent`, `AgentCard`, JSON `"id"`, and flat session `profile`; active
  legacy hits must be zero outside historical prose.

## Non-overlapping ownership

| Owner | Paths | Proof |
| --- | --- | --- |
| Root contract | this file, then accepted edits to `docs/architecture.md` and `docs/roadmap.md` | scope/acceptance review only |
| Runtime/session builder | new `internal/agentruntime/**`, `internal/sessions/{types,validation,status,manager}*`, `tests/integration/sessions_test.go` | runtime unit plus one consuming real-tmux journey |
| Hook/config builder | `internal/hostconfig/**`, new `internal/agenthook/**`, deleted `internal/statushook/**`, `cmd/skidbladnir/main*`, new `tests/integration/agent_hook_test.go` | hook/host unit plus one publishing real-tmux case |
| Gateway builder | `internal/gateway/{dto,gateway}*`, `tests/integration/gateway_test.go`, mechanical wire-key consumers including `tests/integration/multi_machine_test.go`, and logging-test fixtures | one authenticated HTTP journey |
| Android builder | `ProductModel.kt`, `GatewayClient.kt`, `SessionCard.kt`, `DashboardScreen.kt`, `SkidbladnirController.kt`, `TerminalConnection.kt`, `Chrome.kt`, direct hard-rename consumers `ForgeSeal.kt`, `MainActivity.kt`, `TerminalScreen.kt`, and exact symbol consumers in Android tests | one contract proof plus one focused card component proof |
| Deployment builder | deployment-owned host configs, Codex hooks, new Claude hook, and install/doctor tests in their owning repository | exact installed config and hook/plugin byte topology |
| Provider-live builder | `tests/integration/provider_live_test.go` | approved fixed-install provider sample reusing the integration tmux/process safety boundary |
| Root integrator | cross-slice hard-cut search; canonical docs; `scripts/test` only if composition must change | combined gates; writes no new proof |
| Verifier | read-only exact-diff and evidence review | writes no production/test file |

Order: contract acceptance -> runtime/session core -> hook/config. Gateway and
Android then proceed in parallel; deployment follows the final CLI/config
contract. No two builders edit one path.

## 80/20 verification

1. Baseline/final `scripts/test verify`: static, build, and unit gates only.
2. Run each focused owner proof red, then green; no duplicate cross-layer cases.
3. Run the isolated `integration` gate once for red and once for combined green,
   only with explicit approval in that implementation turn.
4. Run one focused Android platform proof only with explicit device approval;
   no hands-on product gate is required because geometry/actions do not change.
5. Run the separately approved `provider-live` matrix against one clean exact
   released checkout and its fixed installed candidate/config/catalogue:
   Linux managed Codex + laptop Claude; Darwin managed Claude + laptop Codex.
   Deployment inspection covers every profile and host. This samples both
   providers, launch origins, and OS observers without an eight-case Cartesian
   gate.
6. Any unapproved integration, platform, deployment, or live boundary is
   `NOT_RUN`, never pass. Add no proof ledger or compatibility matrix.

## Decisions and explicit trade-offs

| Decision | Cost accepted |
| --- | --- |
| No provider APIs | Codex task name and some resumed Claude names are unavailable |
| No cross-process environment reads | Laptop profile requires its installed hook; raw launches may stay unknown |
| One current pane per tmux session | Agents in non-current panes are not independently inventoried |
| Auto-name managed Claude only | Codex cannot promise tmux/provider name equality in this scope |
| Keep raw ids/PID off the compact card | Full mapping is API/model data; no new inspection UI exists yet |
| Hard-cut schema/CLI/type names | Gateway, Android, deployment, and fixtures must release atomically |
| Ignore stale registration without cleanup | A harmless pane option may remain until pane death, avoiding inventory mutation |
| Accept a provider-launched tty-less hook helper | Exact runtime ancestry plus pane foreground binding carries origin; the same host UID remains trusted |
| Permit distinct but abstractly overlapping provider predicates | Version-stable signatures remain deployable; any concrete ambiguous match is omitted fail-closed |
| Load Claude's hook as a local router plugin | User settings remain untouched; direct raw-binary launches cannot register runtime profile or session id |
| Four-case live sample | The full provider x launch-origin x OS Cartesian product is not executed |
| Reuse the integration safety harness for provider-live | Deployment owns installed bytes; Skidbladnir owns exact tmux/process lifetime and inventory behavior, avoiding a second shell safety implementation |

## Non-goals

Prompt sending, communication bridges, Codex queue/App Server, Claude inbox or
socket use, arbitrary CLI control, scheduling, wake/resume/rename, message
receipts, agent discovery outside tmux, non-current-pane enumeration, provider
config/transcript database parsing, name synchronization after in-TUI rename,
thread history, provenance, durable orchestration, new routes, new card detail
UI, or compatibility/migration machinery.

External contract references: [OpenAI Codex hooks](https://learn.chatgpt.com/docs/hooks),
[Claude Code hooks](https://code.claude.com/docs/en/hooks), and
[Claude Code sessions](https://code.claude.com/docs/en/sessions).
