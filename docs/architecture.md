# Ariadne v1: product and architecture

Status: implementation target. Audience: implementers and reviewers.
Normative rules: [`docs/rules`](rules/index.md), especially
[`testing.md`](rules/testing.md). There is no separate `testing-standards.md`;
`docs/rules/testing.md` is the sole testing authority.

## 1. Outcome

Ariadne is Niels's private Android command deck for Codex subscription agents on
one remote devbox. From a Galaxy S22+, Niels can start an agent, watch it work,
type or dictate to it, steer or interrupt it, and archive or reopen it.

The phone is the cockpit, not the runtime. An agent is a durable conversation,
not a repo, directory, worktree, terminal, or process. Ariadne tracks agents;
Codex owns their work.

Rules of the design:

- Freedom is fixed runtime policy, not configuration surface.
- The dashboard reports facts; it does not schedule or constrain work.
- One owner per fact and one canonical path per operation.
- Ambiguity is visible recovery, never a guessed result or automatic redispatch.
- Add a runtime abstraction only when a second runtime actually arrives.

No design question blocks the core. Later live gates require the devbox's
verified `codex-personal` version/login, Tailnet DNS+ACL, Firebase credentials,
and the phone's actual ADB serial/reachability. Remote ADB explicitly gates only
the two-slot release capability and may require a later product decision.

## 2. Fixed v1 contract

| Concern | Decision |
|---|---|
| User/device | Niels; Samsung Galaxy S22+ `SM-S906W`; Android 16 |
| Host/network | One Hetzner VPS; control/data over Tailscale TLS; no public listener |
| Runtime | `codex-personal` App Server only; its `CODEX_HOME` owns Codex auth/state |
| Execution | `/home/niels/src`; approval `never`; sandbox `dangerFullAccess` |
| Workspace | Shared-live only; no workspace/project/repo/worktree model |
| Capacity | No product agent-count cap or host-pressure launch gate |
| Client | Kotlin/Compose; min/compile/target SDK 36 (Android 16) |
| Host | Go gateway, SQLite, systemd user service |
| Delivery | HTTPS mutations, SSE facts/deltas, opaque FCM wake |

Operational bounds remain finite: payloads, streams, buffers, retention, retries,
and timeouts. They protect the service; they never impose an agent quota.

### Goals

1. Match publicly observable Kache behavior: dense `ga-*` cards,
   `WORKING`/`IDLE`, objective/activity, voice, attention buzzes, concurrent
   agents, recovery, host pressure, and two safe app slots.
2. Survive phone death, network loss, gateway/App Server restart, and request
   replay without losing committed identity or duplicating work silently.
3. Deliver one reliable real-phone/devbox path before adding platform machinery.

### Non-goals

- Project enrollment; path tracking; workspace isolation; worktree, git, PR,
  diff, merge, or one-writer-per-repo behavior. Agents may use worktrees when
  instructed through Codex; Ariadne stores nothing about them.
- Agent quotas, scheduling, autonomous assignment, per-agent cgroups, or shared
  memory/tool systems.
- Generic provider/runtime registries, direct provider APIs, CLI/PTY parsing,
  SDK fallback, `llm-calling`, MCP, or a custom agent runtime.
- Multi-user/team/public product, web/desktop clients, public ingress, phone SSH,
  shell endpoints, or raw App Server access.
- Permanent delete. “Close” means archive.
- V1: image input, arbitrary thread import, stable nested-subagent hierarchy,
  literal tmux attach, off-host backup, and disk-loss recovery.

## 3. Evidence and council decisions

Kache's private implementation is unavailable. We copy only public behavior, not
guessed internals. Supplied evidence shows the
[agent grid](https://x.com/yacineMTB/status/2090580615049408711),
[two app variants](https://x.com/yacineMTB/status/2090436678519312650), and a
[recovery inventory](https://x.com/yacineMTB/status/2090443641697288552) that
matches named `ga-*` owners to surviving tmux sessions, restores an exact Codex
conversation when possible, and otherwise restarts from a handoff.

| Question | Resolution | Reason |
|---|---|---|
| App Server, CLI, SDK? | App Server only | It owns rich-client auth, history, threads/turns, and streamed events. |
| App Server transport? | One supervised stdio child | Default, private, simpler than a remote raw socket. |
| Native or wrapped web? | Native Compose | Small UI; dictation, FCM, Keystore, lifecycle, and two APKs are first-class. |
| WebSocket or SSE? | HTTP + SSE | Commands are request/response; SSE gives one-way events and cursor resume. |
| Truth? | SQLite owns Ariadne; App Server owns execution/transcript | Neither mirrors the other wholesale. |
| Provider abstraction? | None in v1 | A one-implementation interface is speculative. |
| Recovery fallback? | None | Unknown outcome becomes explicit `RecoveryRequired`. |

The [Codex App Server documentation](https://learn.chatgpt.com/docs/app-server)
positions App Server as the rich-client surface and marks its command/remote
WebSocket transport experimental for production. This private prototype accepts
that maturity risk through version pinning, generated schemas, stdio, and
fail-closed compatibility—not a fallback runtime.

## 4. Architecture and ownership

```text
Galaxy slot A/B -- HTTPS + SSE over Tailscale --> Go gateway/supervisor
       ^                    |                         |-- Ariadne SQLite
       |                    |                         |-- /proc sampler
       +---- opaque FCM ----+                         |-- FCM/ADB coordinator
                                                      `-- JSONL stdio
                                                           |
                                              codex-personal app-server
                                                           |
                                                   /home/niels/src
```

Systemd owns the gateway; the gateway owns one App Server child. A phone/SSE
connection never owns a turn. Unexpected child death fails readiness and exits
the composition; systemd restarts the same gateway+child path and startup
recovery reconciles it.

Readiness requires valid config, migrated SQLite, a compatible generated
protocol, initialized App Server, and completed reconciliation. A required
dependency failure never becomes an empty dashboard.

| Owner | Owns | Must not own |
|---|---|---|
| Android | Screens, drafts, dictation, cache, installation bearer, deep links | Agent truth, Codex refs, cwd/policy, ambiguous retry |
| Gateway API | Auth, strict HTTP DTOs, protocol gate, SSE delivery | Business state, App Server JSON, operation lifetime |
| Agent/operation | Agent lifecycle, validation, idempotency, per-agent serialization | Transcript, paths, transport |
| Codex adapter | Child/process protocol, request correlation, normalized events | Product policy, HTTP, persistence schema |
| Recovery/event | Exact reconciliation, handoff, lineage, ordered facts, attention | Guessing, redispatch, raw transcript/tool archive |
| Metrics | One `/proc` sampler, retention, aggregation | Admission or per-agent attribution |
| Device | Pairing, FCM wake, signed A/B deployment | Provider secrets, arbitrary APK/ADB commands |
| SQLite | Ariadne facts, auth verifiers, device/update records | Codex history/auth, raw secrets, policy |

Transports remain thin. Every background task is supervised and has explicit
startup, failure, and shutdown behavior.

## 5. Domain and persistence

### Identity and state

- Private `AgentId`, `OperationId`, `EventId`: UUIDv7.
- Visible `AgentHandle`: sealed random `ga-<base32>`; never derived from objective
  or storage id.
- Other outward identities: opaque `OperationHandle`, `RecoveryHandle`,
  `HandoffHandle`, `AttentionHandle`, `CandidateHandle`, `EventCursor`.
- Internal only: `CodexThreadRef`, `CodexTurnRef`.
- `ClientOperationKey`: Android-generated UUID, reused only for an exact replay.

No mutable status column exists. `AgentState` is projected:

`Working | Idle | RecoveryRequired | Archived`

`Completed | Interrupted | Failed` is a separate last-turn outcome. `Working`
requires authoritative active-turn evidence; absence of events never implies
idle. Send while working is rejected; Steer/Interrupt require the exact active
turn. Archive while working is rejected. Different agents run concurrently.

Internal operation state:

`Pending -> Dispatching -> Applied | Rejected | OutcomeUnknown`

- Claiming `Dispatching` is exclusive runner ownership, not proof of an external
  effect. Never hold SQLite open across App Server, FCM, or ADB.
- Within the 90-day idempotency horizon, same `ClientOperationKey`+digest returns
  the same operation; same key with a different digest returns
  `OperationConflict`.
- Safe `Pending` work may resume. Unprovable dispatched work becomes internal
  `OutcomeUnknown`, is never auto-issued, and surfaces only as a public
  `RecoveryRequired` case.
- A start operation owns its candidate handle/objective until `thread/start`
  returns a confirmed thread. One transaction then publishes agent+binding. A
  primary agent is never visible half-created.

### SQLite schema

Use WAL, foreign keys, UTC timestamps, explicit PKs, forward-only migrations,
short transactions, and one bounded `busy` retry owner. No triggers, cascades,
business `CHECK`s, upsert-dependent semantics, or generic metadata bags.

| Table | Required contract |
|---|---|
| `agents` | PK, unique handle, objective, model, effort, created/archived times; no path/repo/worktree/provider/status |
| `agent_thread_bindings` | PK, agent FK, unique Codex ref, ordinal, reason, bound/retired times; repository enforces one open binding |
| `operations` | PK/handle, unique client key, optional agent FK, closed kind/state, versioned request, digest, typed result/error, phase times |
| `events` | monotonic sequence, PK, optional agent/operation FKs, closed kind/versioned payload, causation, time; append-only |
| `recovery_cases` | PK/handle, agent or start-operation subject, lost/replacement bindings, closed reason, open/resolved times |
| `recovery_candidates` | PK/handle, recovery FK, exact internal Codex ref, observed status/time, expiry; user must select this handle |
| `handoffs` | PK/handle, agent/binding/last-turn refs, objective, last user/agent text, normalized activity, time; deterministic, append-only |
| `attention_items` | PK/handle, agent or recovery subject, unique source event, closed kind, created/notified/acknowledged times |
| `host_samples` | sampled time PK; CPU/load/memory/swap/disk/CPU-memory-IO PSI/active turns; seven-day retention |
| `pairing_challenges` | PK, slot, high-entropy secret hash, expiry, consumed time |
| `device_installations` | PK, A/B slot/application id, bearer verifier, encrypted FCM token, paired/seen/revoked times |
| `app_releases` | PK, slot/version/protocol digest/APK hash/signing-key id, closed deployment state and phase times |
| `schema_migrations` | migration PK and applied time |

JSON columns are versioned closed unions decoded before use. SQLite owns these
facts. `$XDG_STATE_HOME/ariadne/todo.txt` is a generated recovery projection and
is never read as authority. App Server owns transcript, turn/item history,
runtime state, and Codex auth. Ariadne stores only normalized activity and short
handoff excerpts, never raw tool output or a second transcript.

One bounded cleanup keeps host samples 7 days, events and acknowledged attention
30 days, terminal operations 90 days, and the latest 20 handoffs/releases. Open
recovery and anything it references is retained. Cleanup order respects FKs; no
cascade or trigger owns lifecycle.

## 6. Android API v1

All routes use strict, bounded `/v1` JSON over the Tailnet TLS name. Every route
requires an installation bearer token except `/v1/pair`, which requires an
expiring one-use secret. Reject unknown fields/unions/handles, trailing JSON,
oversized bodies, and caller-supplied cwd/policy/provider.

| Method/path | Contract |
|---|---|
| `POST /v1/pair` | Consume one-use slot challenge; return bearer once |
| `GET /v1/bootstrap` | Protocol/models, host/slot snapshot, active operations, open recoveries, event cursor |
| `GET /v1/agents?archived=&cursor=&limit=` | Stable order: attention, working, recency, handle; `limit<=100`; opaque next cursor |
| `POST /v1/agents` | `StartAgent(clientKey, objective, model, effort, text)` |
| `GET /v1/agents/{handle}` | Agent/activity projection |
| `GET /v1/agents/{handle}/transcript` | Fresh redacted `thread/read(includeTurns=true)` projection |
| `POST /v1/agents/{handle}/turns` | Send only when idle |
| `POST /v1/agents/{handle}/steers` | Steer exact active turn; never creates a turn |
| `POST /v1/agents/{handle}/interruptions` | Interrupt exact active turn |
| `POST /v1/agents/{handle}/archive` | Reversible close; reject while working |
| `POST /v1/agents/{handle}/reopen` | Unarchive exact binding |
| `POST /v1/recoveries/{handle}/resolution` | Exactly `AdoptCandidate | RestoreHandoff | StartReplacement` |
| `GET /v1/operations/{handle}` | Durable public operation projection; never exposes `OutcomeUnknown` |
| `GET /v1/events` | SSE; `Last-Event-ID` cursor |
| `GET /v1/host/samples?window=&resolution=` | Current + at most 300 historical points |
| `POST /v1/attention/{handle}/acknowledgement` | Idempotent acknowledgement |
| `PUT /v1/device/installation` | Rotate FCM registration for paired slot |
| `GET /v1/app-update` | Active/staged/last deployment |
| `GET /healthz`, `GET /readyz` | Process health; full dependency readiness |

Mutations include `ClientOperationKey`, return `202`+`OperationHandle`, and finish
through events. Public operation state is
`Accepted | Running | Applied | Rejected | RecoveryRequired`; internal
`OutcomeUnknown` never crosses the API. Closed expected errors:

`Unauthenticated | PairingInvalid | ProtocolMismatch | InvalidRequest |
AgentNotFound | AgentArchived | AgentWorking | AgentNotWorking |
OperationConflict | ModelUnsupported | ThreadMissing |
RecoveryChoiceInvalid | UpdateRequired`

Unknown App Server shape, impossible state, corrupt storage, required dependency
failure, or retry exhaustion is a defect with a correlation handle—not an empty
or successful response.

SSE carries durable cursor-bearing `Fact`s and ephemeral item `Delta`s. On every
reconnect Android refreshes active transcripts, then resumes facts after its
cursor. An expired cursor yields `ResyncRequired` and a fresh snapshot. Deltas
are keyed by native turn/item handles; duplicates cannot duplicate UI content.
FCM carries only installation handle+latest cursor.

`ariadne pair --slot A|B` is local-only: write a ten-minute challenge hash and
print the raw secret once. `/v1/pair` consumes it transactionally.

## 7. Runtime, recovery, and projections

### Codex adapter

- Run the pinned `codex-personal app-server` over default stdio; initialize with
  client `ariadne`, `experimentalApi=false`.
- Commit version-matched output from
  `codex-personal app-server generate-json-schema`; static verification rejects
  version/schema drift.
- Use only `model/list`, `thread/start|resume|read|list|name/set|archive|unarchive`
  and `turn/start|steer|interrupt`, plus documented events.
- Supply `/home/niels/src`, `never`, and `dangerFullAccess` on every
  `thread/start` and `turn/start`; reject caller overrides.
- Resume only exact stored bindings. Steer only an exact active turn returned by
  Ariadne `turn/start` with the recorded policy fingerprint. A recovery-adopted
  candidate must be idle and cannot be steered until Ariadne starts its next
  fixed-policy turn.
- Model/effort choices come from `model/list`; no hard-coded catalogue.
- Never expose delete, shell/process/filesystem APIs, experimental hierarchy or
  dynamic tools, raw approvals, alternate CLI/SDK/socket/provider, or legacy
  decoder. Incompatibility prevents readiness.

Transcript projection permits user text, agent text, turn boundary/outcome, and
normalized activity kind/title/status/time. It omits reasoning, protocol data,
tool arguments/results, stdout/stderr, patches, usage, and provider metadata.

### Recovery

At startup: migrate; initialize App Server; list active+archived `appServer`
threads; match only exact stored refs; observe matches; open one recovery case
for every missing/contradictory binding or unknown start; regenerate `todo.txt`.
Never resume work merely because a candidate exists.

Write a deterministic handoff after terminal turns, before archive, and before
app deployment. Explicit resolution may adopt an exact idle candidate, restore
from a selected handoff into one replacement thread, or start a replacement for
an unknown start. Record lineage. Never bind by time/title/cwd/content heuristic
and never resend a prompt automatically.

### Activity and attention

Normalize App Server events to closed activity facts; do not persist raw output.
Create durable attention for `TurnCompleted`, `TurnFailed`, `RecoveryRequired`,
and `UpdateFailed`. Acknowledgement, not notification delivery, clears it.

## 8. Android, pressure, update, security

### Product surface

- Pin API 36 per the official [Android 16 SDK setup](https://developer.android.com/about/versions/16/setup-sdk);
  no lower-version compatibility layer.
- Ink-black, high-contrast, luminous-thread visual language; dense, minimal
  chrome; deterministic character avatars.
- Adaptive virtualized two-column S22+ grid; one column for narrow/large-font
  layouts. Cards show handle, `WORKING|IDLE|exception`, objective, activity,
  time, model/effort, attention. Text/icon accompanies color.
- Top strip: CPU, memory, swap, load, disk, `OK|WARM|HOT`, one-hour sparkline.
  Pressure never disables Start. No project/path/worktree picker.
- Detail: redacted transcript, activity timeline, composer, reviewed push-to-talk
  dictation, explicit Send/Steer, Interrupt, Close, Reopen, recovery actions.
- Offline preserves drafts but queues no mutation; resync precedes re-enable.
- Use Compose, coroutines/Flow, OkHttp HTTPS/SSE, AndroidX saved state,
  `SpeechRecognizer`, FCM, Keystore, and one manual app graph. No Room or DI
  framework. One root `AriadneScaffold` owns all system-bar/IME insets.
- Dictation only, no TTS. Android's selected speech service may process audio
  locally or in its cloud; Ariadne never receives/retains audio. Typed input is
  the only path when recognition is unavailable.
- Support TalkBack, font scaling, IME, edge-to-edge, process death, back, both
  navigation modes, notification permission, doze, and Tailnet loss.

### Host pressure

`justify-polling`: one supervised `/proc` sample every five seconds because the
kernel exposes sampled files, not a durable push stream. The owner starts/stops
one bounded schedule. Keep seven raw days; aggregate queries to 300 points.
No Grafana/Prometheus, alerts, per-agent attribution, or throttling.

### Attention/FCM

FCM is best-effort opaque wake only; stale/duplicate/out-of-order delivery causes
cursor resync. Use one high-importance vibrating `Agent attention` channel,
redacted lock-screen text, and an explicit package/component `PendingIntent` to
the exact agent or recovery. Android settings remain authoritative.

### Two-slot update

One app module, two flavors: `dev.niels.ariadne.a` and `.b`. Each pairs once.
An agent may invoke `scripts/deploy-android`; it builds/signs only the inactive
slot and records version, protocol digest, SHA-256, and signing-key id.

Before ADB install, verify detached manifest signature, artifact hash,
application id, monotonic version, protocol digest, and APK certificate digest.
V1 trusts one configured certificate; key rotation requires planned reinstall+
re-pair of both slots.

Launch inactive, await authenticated heartbeat, then atomically mark it active.
Only active may mutate or receive FCM. Mark old slot standby, stop its
notifications/mutations/SSE, but retain it for explicit reactivation. Failure
before commit leaves old active; this is the deployment transaction, not a
runtime fallback. The updater accepts no URL/package/shell/device override and
targets only the configured S22+ serial. If ADB is unreachable over the Tailnet,
this release gate is blocked; do not switch install mechanisms silently.
Android's current [developer-verification FAQ](https://developer.android.com/developer-verification/guides/faq)
explicitly preserves unverified ADB installs; keep its 2027 global rollout as a
release-watch item, not a second installer.

### Security/operations

- Gateway binds loopback; Tailscale Serve supplies private TLS. Tailnet ACL plus
  per-installation high-entropy bearer is required; Android stores it in Keystore
  and SQLite stores only its verifier.
- Codex credentials stay in `codex-personal` `CODEX_HOME`. Firebase,
  FCM-encryption, and APK-signing secrets arrive as systemd credentials outside
  the repo. Never log or transmit them.
- Redact auth, prompts/messages, FCM tokens, and tool output from logs; retain
  correlation handles, state transitions, timings, and error tags.
- `YOLO` is permanently visible. It removes Codex approvals, not phone auth,
  idempotency, archive confirmation, or update integrity.
- Graceful shutdown: stop ingress; drive accepted operations to a durable phase;
  close SSE; terminate child; close SQLite.

## 9. Target files and work split

```text
api/ariadne.v1.json                 # authored wire schema
codex.version                       # exact required codex-personal version
schemas/codex/<pinned-version>/     # generated App Server schema
cmd/ariadne/main.go                 # composition + local pair command
internal/{agent,operation,event,storage,recovery}/
internal/{appserver,gateway,metrics,device,supervisor}/
migrations/
android/app/src/{main,slotA,slotB}/
e2e/
scripts/{build,test,deploy-android}
deploy/systemd/ariadne.service
docs/{architecture.md,rules/}
go.mod
android/gradlew
```

Go/Go modules are the primary runtime/package manager. `scripts/build` is the
sole repo build/generate command; `scripts/test` is the sole test set. Gradle
owns only the Android target and is called by those wrappers. Use standard Go
HTTP, explicit composition, and one pure-Go SQLite driver; no Makefile, Taskfile,
ORM, web framework, DI container, job queue, or alternate migration/test tool.

Lane 0 is serial; lanes 1–5 follow; lane 6 composes. A lane edits only its paths.

| Lane | Owns | Deterministic first red |
|---|---|---|
| 0 Contract/build | `api/`, generation command, wrappers, build roots | Command inventory/schema drift fails |
| 1 Domain/storage | domain/storage/recovery, migrations | Real SQLite replay+restart yields one intent; missing binding opens recovery |
| 2 Codex adapter | `internal/appserver`, generated Codex files | External protocol fixture proves exact policy and rejects unknown shape |
| 3 Gateway/host | gateway/metrics/supervisor | Real HTTP+SSE+SQLite proves auth, strict decode, cursor recovery, metrics |
| 4 Android | main/A/B Android sources | Real-runtime component/platform proof: grid, dictation, draft recreation, navigation/accessibility; no page/API flow |
| 5 Device | device owner, deploy script | FCM/ADB boundary fixtures prove opaque wake and exact inactive artifact |
| 6 Composition | `cmd`, `e2e`, systemd | Controlled external App Server proves process readiness/lifecycle; live/device are later gates |

Lane 0 owns the schema-generation command; Lane 2 owns generated Codex files.
Shared contract changes return to Lane 0—never patch consumers independently.

## 10. Red/green/refactor and acceptance

For every lane: write a behavior-level acceptance failure (**red**), implement
only enough (**green**), then remove duplication, internal mocks, speculative
options, broad catches, and test seams with behavior unchanged (**refactor**).

Use real SQLite and all repo-owned services. Mock only external App Server, FCM,
and ADB boundaries. E2E uses the real stack and seeds through supported APIs.
Integration asserts through public API/service surfaces; only schema tests inspect
tables. Device/provider unavailable means `NOT_RUN`, never pass.

Canonical future commands:

```text
./scripts/test static             # format/lint/type/dependency/generated drift
./scripts/test unit               # pure; no I/O
./scripts/test integration        # real SQLite/services
./scripts/test component          # real Compose/client runtime
./scripts/test standard           # unit + integration + component
./scripts/test e2e                # one local real-stack command
./scripts/test platform           # connected Android instrumentation
./scripts/test platform-verify    # Android static/build; no device
./scripts/test live               # real codex-personal/Hetzner/Tailscale
./scripts/test release            # signed A/B artifact/manifest
./scripts/test verify             # static + builds + standard + platform-verify
./scripts/test full               # verify + e2e + live
```

`platform` and `release` remain separate prerequisite-dependent release gates.

### One authoritative proof per boundary

| Boundary | Proof |
|---|---|
| Android -> user | Instrumentation/component: paged grid, editable dictation, process-death draft, explicit error, deep link/back/insets |
| Gateway API -> domain | Real HTTP+SSE/SQLite: strict JSON/auth, override rejection, failed readiness/protocol mismatch, cursor-gap resync, delta dedupe |
| Operation -> SQLite | Real service: request replay/crash yields one outcome; different agents progress while one agent serializes Send/Steer/Interrupt |
| Codex adapter -> App Server | One acceptance suite: fixture proves exact policy/unknown-shape failure; live mode proves pinned initialize/start/read/resume/archive |
| App Server event -> UI | Focused golden E2E: start one harmless agent, receive answer, sever client, reconnect to the same transcript |
| Kernel -> metrics UI | Real `/proc` sample persists, aggregates, renders current/history |
| Event -> FCM -> phone | Physical phone receives one redacted wake and exact-agent intent |
| Release -> inactive slot | Physical S22+: install, launch, health-switch while old remains active until commit |
| Restart -> recovery | Kill gateway/App Server mid-turn; exact binding resumes or opens recovery; no redispatch |

The 80/20 gate is `verify` plus the focused golden E2E. Live Codex, physical FCM,
wireless-ADB, signed release, and reboot/recovery are explicit release proofs.

### V1 acceptance

- Real S22+ pages through at least 15 agents with no configured count cap;
  different agents work concurrently and UI stays responsive.
- Every start/turn proves `codex-personal`, `/home/niels/src`, `never`, and
  `dangerFullAccess`; callers cannot override them.
- Dashboard/detail show authoritative state, objective/activity, redacted
  transcript, model/effort, attention, and current/history host pressure.
- Typed/reviewed dictated text starts, sends, steers, interrupts. Send and Steer
  cannot be confused. Close requires idle, archives, and Reopen restores.
- Process death, Tailnet loss, cursor gap, and duplicate delivery resync without
  losing draft or duplicating mutation/content.
- Restart preserves exact bindings. Missing/unknown results show recovery;
  explicit adopt/handoff/replacement records lineage and never guesses/resends.
- Attention produces redacted wake and exact-agent intent. HOT never blocks Start.
- Exact signed inactive APK installs and switches only after health; old remains
  active until commit.
- App Server absence/incompatibility fails readiness. No runtime/provider/auth/
  transport/empty-state fallback exists.
- Schema/API/UI residue contains no project, path, repo, workspace, worktree,
  quota, generic provider, or delete surface.
- Relevant gates pass; missing live/device prerequisites remain `NOT_RUN`.

## 11. Deferred v1.1+

- Literal `ga-*` tmux inventory/attach and text-handoff recovery parity.
- Images; arbitrary thread adoption; stable non-experimental subagent hierarchy.
- A second runtime. Then add one explicit discriminator+adapter by hard cutover,
  with no provider fallback; evaluate `llm-calling` against that concrete need.
- Encrypted off-host backup and full devbox-loss restore.

Anything else needs a new acceptance criterion and explicit scope decision.
