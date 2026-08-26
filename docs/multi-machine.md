# Multi-machine v0 change specification

Status: implementation complete; external acceptance pending.

This is the temporary reviewed contract for adding Niels's MacBook beside the
existing devbox. The current [architecture](architecture.md) remains true for
the shipped product. This document owns only the intentional delta below.
After every named acceptance gate passes, the root integrator folds the final
behavior into `architecture.md`, updates `roadmap.md`, and deletes this file.
`NOT_RUN` blocks that fold.

No production change is authorized outside this scope. Repository rules,
especially [testing](rules/testing.md), remain normative.

## 1. Goal and final state

One Android app directly controls two independent Skíðblaðnir gateways over
the tailnet:

```text
                     Android
                    /       \
       HTTPS/WSS :8443       HTTPS/WSS :8443
                  /           \
       Devbox gateway         MacBook gateway
       systemd + tmux         launchd + tmux
```

- `Agents` is one collection spanning Devbox and MacBook.
- Every card, action, pressure reading, error, and terminal names its machine.
- The Forge requires a machine target, then offers only that machine's
  profiles.
- Each gateway observes and mutates only its local default tmux server.
- Either machine may fail without blocking, hiding, or mutating the other.
- Restart still means "list local tmux again." Android holds pairing
  configuration and an in-memory last snapshot, not lifecycle truth.

Success is boring federation: two copies of the existing host capability and
one client that composes their results. There is no fleet control plane.

## 2. Scope

### In scope

- Exactly two acceptance hosts: the existing Linux devbox and this MacBook.
- A collection-shaped client model; adding a third explicit pairing later must
  not require a schema redesign.
- Independent origin, bearer, installation identity, polling, pressure,
  lifecycle, and failure state per machine.
- macOS launch/install, process observation, lifecycle/attention, pressure,
  create/list/attach/detach/kill parity.
- One hard API/client cutover and removal of the single-host path.

### Non-goals

- Automatic discovery, central registry, coordinator, proxy, relay, tsnet, or
  Tailscale Service load balancing.
- Cross-machine move, broadcast, scheduling, failover, wake, or migration.
- Shared credentials, delegated auth, multi-user access, or public ingress.
- Durable inventory cache, SQLite, synchronization, conflict resolution, or
  globally unique tmux names.
- Thread identity, transcript inspection, payload hooks, adoption, provenance,
  or any retired machinery from the architecture reset.
- Tabs/table modes, fleet-wide pressure aggregation, push, remote host setup,
  arbitrary machines, arbitrary profiles, or arbitrary tmux servers.
- Tailnet policy automation, Tailnet Lock, device posture, and capability
  grants. They may harden a later version; they do not enter this slice.
- Backward compatibility, migration, fallback parsers, dual protocols, or a
  retained one-host UI branch.

## 3. Fixed decisions

| Concern | Decision |
| --- | --- |
| Topology | Android federates two direct gateways; gateways never know each other |
| Machine identity | Random immutable installation handle, independent of label, DNS, Tailscale node, and bearer |
| Human identity | Mutable phone-local label, initially `Devbox` or `MacBook` |
| Network | One pinned HTTPS origin per machine; Tailscale Serve `:8443`; Funnel forbidden |
| Auth | One independently minted bearer per machine; never copied or shared |
| Runtime truth | Each machine's local tmux server only |
| Session identity | `(machineHandle, tmuxServerLifetime, sessionID, name)` |
| Polling | Existing five-second cadence, scheduled independently per machine |
| Cache | Last successful snapshot in memory only; stale data is non-mutating |
| Terminal | At most one active phone terminal, bound to one exact machine/session target |
| macOS service | Per-user LaunchAgent; available only while the user is logged in and the Mac is awake |
| Cutover | Gateway and APK move in lockstep; old/new combinations are unsupported |

Machine is an identity and failure namespace, not decorative metadata. Session
IDs, names, identity-token bytes, profile keys, and dwarf keys may collide
across machines and must remain distinct.

## 4. Capability contract

### 4.1 Host capability

Each installed gateway provides the existing five routes against exactly one
local tmux server:

| Route | Capability |
| --- | --- |
| `GET /v1/sessions` | Machine envelope, observation time, local profiles, local sessions |
| `POST /v1/sessions` | Create one allowlisted local session |
| `GET /v1/sessions/{id}/terminal` | Attach WSS to one exact local lifetime |
| `DELETE /v1/sessions/{id}` | Kill one exact confirmed local lifetime |
| `GET /v1/pressure` | Current local pressure and 15-minute local window |

All existing validation, exact-kill, grouped-shadow, bounded-terminal,
attention, and content-free lifecycle guarantees remain per machine.

### 4.2 Machine identity

`MachineHandle` is `mh-` followed by 32 lowercase hexadecimal digits. It is
128 random bits, opaque, non-secret, and never derived from mutable platform
facts.

- `skidbladnir machine init --file=PATH` atomically creates it only when
  absent; an existing valid value is returned unchanged.
- Installers own `~/.config/skidbladnir/machine-handle` with mode `0600`.
- `gateway --machine-handle-file=PATH` fails closed on missing or malformed
  content. It never mints, repairs, or substitutes an identity.
- Reinstall preserves the file. Intentional deletion creates a new machine and
  requires remove/re-pair on Android.
- The handle may appear in protocol diagnostics. Origins, bearers, terminal
  bytes, cwd, objectives, prompts, and account data never enter logs.

### 4.3 Request binding

- Bearer authentication remains mandatory on every `/v1` request and runs
  before any machine-identity check or disclosure.
- Paired requests also send `Skidbladnir-Machine: <MachineHandle>`.
- Only the initial authenticated `GET /v1/sessions` used for pairing may omit
  that header.
- A supplied mismatch returns `409 MachineIdentityMismatch` before any
  mutation, WSS upgrade, or tmux command.
- POST, DELETE, pressure, and terminal always require the header.
- Terminal also retains `Skidbladnir-Session-Identity`.
- A missing required or wrong handle returns
  `{"code":"MachineIdentityMismatch","message":"The machine identity changed. Pair this machine again."}`.
  Multiple machine headers are `400 InvalidRequest`. Never return the actual
  handle in an error.

The machine header prevents a changed DNS/origin from turning a valid local
session target into an action on another host. It is identity binding, not a
second credential.

## 5. Wire schema: hard cut

Unknown keys and enum values remain defects in this same-system closed schema.
There is no protocol version branch or compatibility state.

`GET /v1/sessions` becomes:

```json
{
  "machine": {
    "handle": "mh-0123456789abcdef0123456789abcdef",
    "platform": "Linux"
  },
  "observedAt": "2026-08-26T12:00:00Z",
  "profiles": [],
  "sessions": []
}
```

- `platform` is the closed union `Linux | Darwin`.
- Existing profile and session DTOs do not gain machine fields. Android wraps
  them in the machine envelope; the gateway cannot mislabel local facts.
- Existing POST/DELETE bodies and terminal frames stay unchanged.
- `MachineIdentityMismatch` is added to the closed HTTP error union.

`GET /v1/pressure` becomes:

```text
PressureResponse(unsupported: PressureMetric[], current, history)
PressureMetrics(..., memoryPressure?: Normal | Warning | Critical)
```

- `unsupported` is sorted, unique, and constant for one running gateway.
- Add `metrics.memoryPressure` with the closed values
  `Normal | Warning | Critical` and add `memoryPressure` to `PressureMetric`.
- Linux reports `memoryPressure` as unsupported.
- Darwin reports `memoryAvailablePercent` and the three PSI metrics as
  unsupported; it reports the native current system memory-pressure state.
- A sample's `missing` contains only supported metrics whose observation
  failed. For every sample, absent metric keys equal `missing + unsupported`.
- Disk available and normalized load are required classification inputs on
  both platforms. Linux additionally requires memory available and its three
  PSI signals. Darwin instead requires native memory pressure: warning maps to
  `Warm`, critical maps to `Hot`. CPU and swap remain display-only.
- A failed required supported signal yields `Unknown`; an unsupported signal
  does not. Thresholds and de-escalation remain otherwise unchanged.

## 6. Android product model and behavior

The product model is explicit and collection-shaped:

```text
PairedMachine(handle, label, origin)
AgentTarget(machineHandle, AgentSession)
ForgeDraft(machineHandle, cwd, profile, optionalName, objective)
InventorySnapshot(inventory, receivedAtMonotonic)
InventoryState = Reading | Fresh(snapshot) | Stale(snapshot, cause)
               | Unreachable(cause)
PressureState = Reading | Fresh(response) | Stale(response, cause)
              | Unavailable(cause)
MachineAccess = Ready | AuthRequired | IdentityChanged
MachineState(access, inventoryState, pressureState)
```

Rules:

- Pairing accepts a label, HTTPS `:8443` origin, and bearer; it performs the
  headerless authenticated inventory read and pins the returned handle.
- Origin must have a hostname and no user-info, path, query, or fragment.
- Handles, origins, and labels are unique. A label may be renamed. An origin is
  immutable; remove and add the pairing to change it.
- Bearer rotation re-authenticates against the same pinned handle.
- Pairing metadata uses app-private preferences. Each bearer reuses the
  existing Android Keystore AES-GCM primitive with a fresh nonce and AAD bound
  to its handle/origin; bearer material is absent from the product model.
  Session snapshots and Forge drafts do not survive process death.
- `BearerStore` and the singular hard-coded origin are deleted. No old
  preference reader or migration exists; cutover requires clearing app data or
  uninstalling the old APK.

### Agents

- Default view is one grid with `All` plus one chip per paired-machine label
  (`Devbox`, `MacBook` initially).
- Every card always shows a machine pill. Stable sort is: attention, status,
  machine label, session name, local tmux ID.
- Duplicate local IDs/names never collapse; Compose keys use the full target.
- Each machine has its own pressure strip, freshness, and inline failure.
  There is no synthetic fleet pressure.
- A failed poll preserves only that machine's last in-memory snapshot as
  a literal `STALE` state (color is secondary). Attach, kill, and create are
  disabled for stale/unreachable inventory, auth, and identity change.
- Inventory and pressure publish independently; unavailable pressure never
  disables actions against fresh inventory.
- The scheduled poll is the only read retry. Mutations and terminal input are
  never replayed or retried.
- Each machine owns its polling work. Requests may overlap across machines,
  never within the same resource/machine; ticks coalesce instead of queueing.
  A slow host cannot delay the other.
- Freshness uses host `observedAt - signalAt`, then advances with the phone's
  monotonic elapsed time. Host clocks are never compared to each other.

### Forge

- Machine is the first required field and remains visible.
- Opening Forge from a machine filter may preselect that explicit filter.
  Otherwise no machine is inferred from cwd, profile, or prior network state.
- Changing machine clears cwd/profile, preserves name/objective, and reloads
  only the selected machine's declared profiles.
- Submission and CTA name the target: `Create on MacBook`.
- Validation and errors remain local to the selected machine.

### Terminal and destructive actions

- Terminal header always shows machine label and session name.
- One terminal connection owns one `AgentTarget`; reconnect re-reads that
  machine before opening a new WSS and never replays bytes.
- Kill confirmation says `Kill <name> on <machine>?` and sends the selected
  machine handle, local ID, name, and local identity token to that origin.
- Identity change closes an active terminal and disables every mutation for
  that pairing until remove/re-pair.

## 7. Host composition

### Shared substrate

- Add one narrow `internal/platform` closed descriptor for `Linux | Darwin`
  and the exact tmux executable pin. It is not a generic environment/provider
  framework.
- Extract one `internal/process` observer used by both session status and the
  content-free status hook. It returns typed PID, parent, process group,
  foreground group, executable, argv, and kernel start identity.
- Linux keeps `/proc` behind Linux build files. Delete the duplicated `/proc`
  ancestry/argv parsing from `sessions` and `statushook` after the extraction.
- Darwin uses native kernel process information and exact argv/start identity;
  no `ps` text parsing, guessed basename, Linux file fallback, or permanent
  `Unknown` exemption. Use the pane tty foreground group plus Darwin
  `KERN_PROC`, `KERN_PROCARGS2`, and `proc_pidinfo`/`proc_pidpath` facts to
  recover PID, parent/group, executable, argv, and start timestamp.
- Keep one evaluator and DTO mapper for pressure. Split only metric collection
  and declared capability by platform build files.
- Gateway configuration receives exact tmux path, home, profile table, bearer
  path, catalogue path, and machine-handle path. Do not add a generic
  environment/provider framework.

### Devbox

- Preserve `/usr/bin/tmux` and proven tmux `3.4`.
- Preserve systemd user service, profile commands, hooks, and current Linux
  pressure semantics.
- Installer additionally initializes/publishes the machine handle and new
  hard-cut gateway.

### MacBook

- Acceptance baseline is macOS `26.4.1` build `25E253`, Darwin `25.4.0`,
  `arm64`. OS drift requires renewed Darwin boundary proof before the installer
  is republished; it does not add a runtime compatibility branch.
- Pin `/opt/homebrew/bin/tmux` and exact version `tmux 3.7b`; acceptance must
  re-prove the path/version before publication.
- Pin the installed Tailscale CLI at
  `/Applications/Tailscale.app/Contents/MacOS/Tailscale`, version `1.102.1`.
  Publish only `/v1` on HTTPS `:8443` to `127.0.0.1:7341`; installer
  verification reads `serve status --json` and rejects Funnel/public state.
  Any path/version drift stops installation for review.
- Profiles are `/Users/nnandal/bin/codex-{personal,work,work2}` with matching
  `/Users/nnandal/.codex-{personal,work,work2}` homes and exact foreground
  Node path `/Users/nnandal/.local/bin/codex`.
- Install a per-user `dev.niels.skidbladnir` LaunchAgent with `RunAtLoad`,
  restart-on-failure, numeric loopback listen, explicit `HOME`/`PATH`, and
  `TMUX`, `TMUX_PANE`, `TMUX_TMPDIR` removed.
- Publish separate exact MacBook hook/notify assets; do not make Linux shell
  assets branch on the host at runtime.
- Collect with Darwin APIs, not command output: processor-tick deltas for CPU,
  one-minute load divided by logical CPUs, current system memory pressure,
  `vm.swapusage` for swap, and `statfs("/")` for disk. The memory signal uses
  macOS's native normal/warning/critical semantics, not an invented Linux
  percentage; it samples current state rather than inferring it from the last
  notification. Unknown native values are missing. PSI is explicitly
  unsupported. See Apple's [system pressure API](https://developer.apple.com/documentation/dispatch/dispatch_source_type_memorypressure)
  and [XNU pressure semantics](https://github.com/apple-oss-distributions/xnu/blob/main/doc/vm/memorystatus_notify.md).
- Sleep, logout, Tailscale loss, or LaunchAgent absence is ordinary
  machine-local unreachability. Skíðblaðnir does not wake the Mac.

Every gateway and test process that may invoke tmux must drop inherited
`TMUX`, `TMUX_PANE`, and `TMUX_TMPDIR`. Any approved tmux test owns a unique
explicit `-L` or `-S` socket and uses that same selector for cleanup. It never
kills, resizes, or retargets a user server.

## 8. Files and non-overlapping ownership

Each builder writes its own red proof, observes it fail, implements only its
slice, then refactors green. A verifier changes no production or test file.

| Slice | Sole paths | Observable red |
| --- | --- | --- |
| A. Protocol/gateway | `cmd/skidbladnir/main.go`, `internal/machine/**`, `internal/gateway/**`, `internal/logging/**` | Gateway lacks a stable envelope or accepts a mismatched machine on a mutation/WSS |
| B. Host semantics | `internal/platform/**`, `internal/process/**`, `internal/sessions/**`, `internal/statushook/**`, `internal/pressure/**` | Darwin cannot prove exact process lifetime/status or honest pressure; duplicate Linux observers remain |
| C. Host publication | `scripts/build`, `scripts/install-devbox`, `scripts/install-macbook`, `deploy/**` | Repeatable Mac install/restart or identity-preserving reinstall fails |
| D. Android federation | `android/**` | Equal local sessions collapse, wrong-origin mutation is possible, or one host failure blocks the other |
| E. Root integration | `tests/**`, `scripts/test`, `docs/**`, `catalog/**` | The real two-host journey or hard-cut cleanup is not composed/proven |

No slice edits another row. Only E changes `scripts/test`, architecture,
roadmap, or catalogue. Cross-slice needs go to the root integrator; they do not
authorize shared ownership.

Expected hard-cut file changes include:

- add `internal/machine/**`, `internal/platform/**`, `internal/process/**`,
  Darwin/Linux build files, `scripts/install-macbook`, and
  `deploy/launchd/**`;
- replace `BearerStore.kt` with `MachineStore.kt` and machine-scoped client,
  controller, model, Forge, dashboard, and terminal composition;
- split exact devbox/MacBook hook and notify assets;
- remove superseded platform-specific parsers and every singular-origin/
  singular-bearer path once green.

Do not create a machine repository, coordinator service, compatibility DTO,
schema generator, migration framework, or second product-state owner.

## 9. Verification: 80/20 shape

Use the fewest proofs that cross the most risk. Tests assert public behavior,
not helper calls; no internal mocks or fake client runtimes.

| Ownership boundary | One primary proof |
| --- | --- |
| Pure contracts | Fast table tests for handle/origin validation, strict DTOs, pressure capability classification, federation reducer, routing, and stable sort |
| Gateway/API | Approved real gateway + isolated tmux proof: pairing envelope plus machine-bound read/create/kill/terminal rejection before mutation |
| Host/tmux | One approved isolated-socket integration exercised on Linux and Darwin: list/create/status/attention/attach/detach/exact kill |
| Mac publication | One approved live LaunchAgent proof: install, restart, gateway re-list, bearer isolation, identity-preserving reinstall |
| Android platform | Approved S22+ instrumentation: encrypted collection persistence, pairing/repair/rotation/removal state, lifecycle reconciliation, and visible stale-action admission |
| Product | One approved physical S22+ read-only production journey: two-host federation/routing plus machine-local outage and recovery, without mutating a production tmux lifetime |

The composed real-boundary proof provides most confidence. Approved isolated
Linux/Darwin host and API gates own create, attach, input, detach, exact kill,
and identity/auth rejection against test-created tmux lifetimes. Approved S22+
instrumentation owns Android persistence and lifecycle behavior. The physical
production journey owns federation, exact routing, and machine-local
outage/recovery, and remains read-only with respect to production tmux. Together
they cover the acceptance list below. Unit tests cover only closed decision
logic. Do not duplicate the full matrix at every tier.

Red/green order:

1. Each builder establishes the named failing public behavior in its owned
   path and records the red command/output in the change handoff.
2. Implement the smallest green path. No compatibility branch.
3. Refactor duplicated process/platform/client composition while green.
4. Run routine static/unit/Android compile gates; they never invoke tmux or a
   device.
5. With explicit same-turn approval, run isolated Linux/Darwin tmux gates,
   live installs, then the physical-device gate. Missing hardware/boundary is
   `NOT_RUN`, never pass.

## 10. Acceptance criteria

The slice is accepted only when all are true:

- Devbox and MacBook sessions render together; identical local IDs, names,
  tokens, and dwarf keys remain separate and route to the correct origin.
- Machine is visible on every card, pressure state, terminal, Forge submission,
  error, and kill confirmation.
- MacBook Forge uses only MacBook profiles/paths; Devbox uses only Devbox
  profiles/paths. Unknown profile and invalid cwd mutate nothing.
- Disconnecting either host leaves the other fresh and fully usable. Only the
  failed host's prior in-memory cards become stale and non-mutating.
- A changed origin/handle cannot list as fresh, create, attach, send input, or
  kill under the pinned pairing.
- Bearer failure/rotation on one host has no effect on the other. Neither
  bearer reaches WebView/JavaScript, logs, or the other gateway.
- Terminal handoff preserves the laptop view and local process; detach leaves
  work alive; reconnect never replays input.
- Kill destroys only the exact confirmed machine-local lifetime and fails
  closed on stale identity or ordinary grouped siblings.
- Gateway/app/LaunchAgent restart re-lists tmux without recovery state. Mac
  reinstall preserves its handle; deliberate handle deletion requires re-pair.
- Mac status and attention match the current exact foreground Codex lifetime;
  stale PID/lifecycle data cannot transfer to a replacement process.
- Linux pressure behavior is unchanged. Darwin pressure is useful without
  pretending PSI exists; missing and unsupported are disjoint and exhaustive.
- Routine gates, both approved host gates, live Mac publication, and approved
  S22+ two-host journey are green. No required gate is `NOT_RUN`.
- `rg` finds no old singular origin/store, old sessions envelope, Linux-only
  process owner outside build-specific files, compatibility reader, fallback,
  or dead deployment asset.
- The root integrator folds the accepted behavior into architecture/roadmap
  and deletes this temporary spec in the same final change.

## 11. Cutover

1. Build one revision for both gateways and Android.
2. Install and verify Devbox, then MacBook, including unique origin, bearer,
   handle, exact tmux pin, and local inventory. Do not expose Funnel.
3. Clear old Android app data or uninstall the old APK; install the new APK.
4. Pair Devbox and MacBook independently and run the two-host acceptance.
5. Fold this contract into architecture only after every gate is green.

There is no mixed-version service window. If either host cannot move, stop the
cutover; do not restore a legacy parser, one-host branch, shared bearer, or
fallback route.
