# Public Fleet Distribution And Connect

Status: accepted contract with source implemented and routine proofs green;
external release, host/tmux, and phone gates remain `NOT_RUN`.
[architecture.md](architecture.md) owns the resulting product contract and
[roadmap.md](roadmap.md) owns delivery order; this document owns the slice's
detailed capability and work split.

Normative code and proof rules remain [docs/rules](rules/index.md), especially
[testing.md](rules/testing.md). Do not create a second testing standard.

## 1. Decision

Ship one public GitHub Release and one fixed personal fleet:

- Android 16/API 36 phones install `skidbladnir-android.apk` from GitHub.
- Two trusted users pair separately on the same tailnet with one fresh QR each.
- `dev-server` pins that release and converges Devbox, MacBook, and Arch.
- Each gateway is a machine-local, auto-started service over Tailscale Serve.
- A MacBook operator command displays one transient fleet QR per phone.
- `Connect` scans that QR, redeems all three one-use invitations, and commits
  the exact fleet atomically.
- After initial host convergence and Tailscale login, no process is started by
  hand. tmux remains the database; the phone remains a tmux client.

This is a hard cut. Delete external ADB provisioning, two-machine assumptions,
manual bearer entry, repo-owned host installers, headerless inventory pairing,
and platform-derived profile configuration. Add no compatibility path.

One unanswered acceptance input remains: name the second API 36 phone before
its physical gate. Until then that gate is `NOT_RUN`, never inferred from the
S22+ result.

## 2. Goals And Boundary

Goals:

1. A new trusted user installs one obvious APK, signs into Tailscale once,
   taps `Connect`, scans one QR, and sees all three machines.
2. `dev-server converge` owns repeatable installation, configuration,
   autostart, Serve publication, and diagnosis on all three hosts.
3. Releases are signed, attributable, checksummed, version-locked, and easy to
   update without introducing an app store or hosted control plane.
4. Setup and repair are create-only or identity-preserving, fail closed, and
   never leave a partial fleet.

Non-goals:

- Play Store, Play Internal Testing, auto-update, or background APK install.
- Embedded Tailscale, Headscale, VPN control, tailnet login automation, public
  ingress, relay, coordinator, discovery, or a fleet database.
- Machine add/remove/rename, arbitrary fleet size, arbitrary remote commands,
  multi-tailnet tenancy, roles, invitations between users, or account recovery.
- Degoogled Android support in this slice; Google Code Scanner is the accepted
  no-camera-permission 80/20 scanner boundary.
- Independent phone revocation. Both phones share each host's bearer; rotating
  one host bearer intentionally requires both phones to reconnect.
- General installer, plugin runtime, profile registry, migration layer,
  provenance ledger, or backward-compatible QR/API versions.

## 3. Final State And Ownership

```text
GitHub Release (APK + 2 host bundles + checksums + signer fingerprint)
       |                                  |
       | user installs                    | dev-server pins + converges
       v                                  v
Android phone -- Tailscale tailnet --> Devbox gateway --> local tmux
       |                             --> MacBook gateway -> local tmux
       |                             --> Arch gateway ----> local tmux
       |
       +-- scan one QR <-- dev-server `./skidbladnir invite`
                             | local fixed command
                             + SSH fixed command to Devbox
                             + SSH fixed command to Arch
```

| Owner | Owns | Must not own |
|---|---|---|
| Skíðblaðnir | Android product, gateway/API, strict host-config parser, pairing protocol, release artifacts, protocol tests | Fleet topology, host service convergence, durable host secrets |
| `dev-server` | Exact three hosts, pinned release/checksums, host configs, services, Tailscale Serve desired state, doctor, fleet-invite aggregation | App behavior, API semantics, release signing key, copied gateway source |
| Each host | One immutable machine handle and one mode-0600 bearer; local tmux truth | Other hosts' credentials or runtime state |
| Phone | One encrypted fixed fleet and ephemeral scan/redeem state | Host administration, Tailscale credentials, terminal content outside the terminal boundary |

There is no runtime dependency on GitHub, `dev-server`, the MacBook operator,
or another gateway after pairing.

## 4. Capability Contract

### 4.1 Release

One immutable public release tag `v<semver>` publishes exactly five owned
assets (in addition to GitHub's automatic source archives):

- `skidbladnir-android.apk`
- `skidbladnir-linux-amd64.tar.gz` for Devbox and Arch
- `skidbladnir-darwin-arm64.tar.gz` for MacBook
- `SHA256SUMS`
- `android-signing-cert.sha256`

`versionName` equals the tag without `v`; `versionCode` increases strictly.
`skidbladnir version` reports the tag and exact source SHA. Host archives contain
the one binary plus the immutable catalogue; `dev-server` owns service/config
assets.

`scripts/release v<semver>` is local and fail closed. It requires a clean
`origin/main`, fresh successful GitHub `verify` for the exact SHA, the existing
external Android signing key, exact signer verification, cross-builds, and
checksum verification. It creates a GitHub **draft** only. Publication is one
manual review action; published assets are never replaced. No signing secret is
stored in GitHub. A separate read-only post-publication gate requires the final
release to be non-draft, non-prerelease, immutable, exact-SHA, exact-five-assets,
and byte-valid after a fresh public download. That gate also compares the
canonical tag, SHA, and all five downloaded-asset digests byte-for-byte with
the committed `dev-server` release pin; partial pins cannot pass.

Update order is hosts first, verify, then phone. Mixed versions are not
supported. A failed host update is repinned before the phone advances; after a
phone update, recovery is a forward release because ordinary APK downgrade is
not supported.

### 4.2 Host convergence

`dev-server` pins one release tag and every asset digest. Convergence:

1. installs the latest stable tmux, Tailscale, the pinned Skíðblaðnir release
   bundle, and `qrencode` where needed;
2. creates a machine handle and bearer only when absent, preserving both on
   every reinstall;
3. renders one strict host config and the existing content-free Codex lifecycle
   assets;
4. installs a user systemd service with lingering on Linux or a LaunchAgent on
   macOS;
5. owns only its dedicated Tailscale Serve `:8443/v1` mapping to
   `127.0.0.1:7341/v1`, removes only its retired root handler, and does not
   reset unrelated Serve state or expose loopback-only `/healthz`;
6. starts/restarts only the gateway service when its owned artifact or config
   changes; and
7. reports stable `PASS|WARN|FAIL <key> <message>` doctor facts for artifact
   version/digest, config, secret modes, service, loopback health, Serve, tmux,
   and Tailscale login.

Tailscale authentication is a one-time human boundary per host and phone.
Convergence may install and diagnose Tailscale; it must not manufacture login
state or claim that an installed client is connected.

The gateway and `status-hook` require `--host-config=PATH`. There are no host
defaults. JSON is decoded strictly with unknown and null members rejected:

```json
{
  "platform": "Linux",
  "tmux": {"path": "/usr/bin/tmux", "testedVersion": "tmux 3.4"},
  "codexNodeEntrypoint": "/home/niels/.local/bin/codex",
  "profiles": [
    {
      "key": "personal",
      "label": "Codex · Personal",
      "command": "/home/niels/bin/codex-personal",
      "environment": [{"name": "CODEX_HOME", "value": "/home/niels/.codex-personal"}],
      "foregroundSignatures": [{"executableBase": "codex"}],
      "arguments": ["--dangerously-bypass-approvals-and-sandbox"]
    }
  ]
}
```

All paths are rendered absolute by `dev-server`; no interpolation occurs in the
gateway. Platform is exactly `Linux|Darwin`; runtime mismatch or a
missing/broken/noncanonical configured tmux prevents startup. `testedVersion`
records the last acceptance target for an advisory doctor warning; a different
canonical installed version does not block convergence, gateway startup, or the
lifecycle adapter. Profiles reuse `sessions.Profile` validation. Every
host config declares exactly `personal`, `work`, `work2`, `claude-personal`, and
`claude-work`, matching the existing `dev-server` shortcuts. Platform adapters
retain only native observation/process/pressure behavior; they no longer choose
paths, runtime versions, commands, or profiles.

### 4.3 Machine pairing API

Each gateway holds one in-memory pairing slot. It is not tmux state and does
not survive gateway restart. Creating a slot replaces any prior slot. The raw
token is returned once; memory retains only a domain-separated SHA-256 verifier,
expiry, machine handle, and current-bearer verifier. TTL is exactly five minutes.

`PairingInviteToken` is 32 random bytes encoded as strict unpadded base64url
(43 characters). It is authority, never identity.

```http
POST /v1/pairing-invites
Authorization: Bearer <current-host-bearer>
Skidbladnir-Machine: <machine-handle>
Content-Length: 0

201
{"pairingInviteToken":"<token>","expiresAt":"<RFC3339Nano>","machine":{"handle":"<handle>","platform":"Linux"}}
```

```http
POST /v1/pairings
Authorization: Skidbladnir-Invite <pairing-invite-token>
Skidbladnir-Machine: <machine-handle>
Content-Length: 0

200
{"machine":{"handle":"<handle>","platform":"Linux"},"bearer":"<43-character-bearer>"}
```

Both routes reject query strings, bodies, duplicate headers, unknown methods,
and noncanonical values. Create uses ordinary bearer authentication. Redeem
does not accept a durable bearer; it validates the invitation and machine
header, then consumes the slot atomically before returning. On redeem, expired,
replaced, used, malformed, wrong-machine, restarted, or
bearer-rotation-invalidated tokens all return the same non-oracular
`PairingInviteRejected`. `MachineIdentityMismatch` is reserved for ordinary
authenticated routes and invitation creation:

| Code | HTTP | Frozen message |
|---|---:|---|
| `PairingInviteRejected` | 401 | `This fleet invite is invalid, expired, or already used.` |
| `MachineIdentityMismatch` | 409 | `The machine identity changed. Fleet reset is required.` |
| `InvalidRequest` | 400 | `The request is not valid.` |
| `InternalError` | 500 | `Skíðblaðnir could not complete the request.` |

All other `/v1` routes require exactly one normal bearer and the matching
machine header. Delete the headerless `GET /v1/sessions` exception. Logs contain
route/error/status/duration only: never token, bearer, origin, QR payload,
terminal bytes, prompts, objectives, or account data.

`skidbladnir pairing-invite create` reads the host's default credential files,
calls the loopback API, and writes the successful response as strict JSON to
stdout. It never prints the durable bearer. Errors are credential-free and go
to stderr.

### 4.4 Fleet invitation

From the MacBook, `dev-server` exposes `./skidbladnir invite`. It maps the fixed
closed host enum `Local|DevServer|Arch` to fixed local/SSH invocations; no config
field may contain a command. Origins come from one mode-0600, gitignored
operator manifest. Bearers never leave their host.

The command requests all three invitations, awaits every result, validates the
fixed labels/origins/handles, and emits no QR unless all succeed. It renders the
payload to `qrencode` through stdin, never an argument or file. It persists
nothing and prints no machine secret outside the QR. A failed/uncertain attempt
requires a new command and new QR; there is no retry or partial reuse.

The QR is UTF-8 JSON, at most 4096 bytes, with no null or unknown members:

```json
{
  "kind": "skidbladnir.fleet-invite.v1",
  "machines": [
    {"label": "Arch", "origin": "https://arch.<tailnet>:8443/", "machineHandle": "mh-<32-lowercase-hex>", "pairingInviteToken": "<token>"},
    {"label": "Devbox", "origin": "https://devbox.<tailnet>:8443/", "machineHandle": "mh-<32-lowercase-hex>", "pairingInviteToken": "<token>"},
    {"label": "MacBook", "origin": "https://macbook.<tailnet>:8443/", "machineHandle": "mh-<32-lowercase-hex>", "pairingInviteToken": "<token>"}
  ]
}
```

The array is exactly three entries in that order. Labels are exact and unique;
origins are canonical HTTPS origins with no userinfo/query/fragment and unique
host:port; handles and tokens are canonical and unique. The `kind` is a strict
discriminant, not a compatibility switch: future formats hard-cut this value.
A new QR is required for each phone.

### 4.5 Android connect and repair

An empty valid store enters `FleetConnect`; it never enters an empty dashboard.
The primary `Connect` action invokes Google Code Scanner without camera
permission. If Tailscale is unavailable, the UI offers `Open Tailscale` (or
`Install Tailscale` when absent) and resumes when the user returns. No supported
Android contract lets Skíðblaðnir perform or verify Tailscale login, so the UI
must state that boundary instead of simulating one-button VPN control.

The scanner result is parsed once into an owned `FleetInvite`. The app redeems
all three entries concurrently, awaits every non-idempotent result, verifies
that every returned handle matches, and retains successful bearers in memory
only. It commits exactly once only after all three succeed. On any network,
protocol, cancellation, process-death, or storage failure, no partial fleet is
readable and the user creates a new invitation. There is no automatic redeem
retry.

`MachineStore.installFixedFleet` reuses the existing AES-256-GCM and canonical
machine validation. It accepts only an entirely empty preference boundary,
seals all three values before mutation, performs one synchronous commit, then
reads back the exact collection. Any existing valid, partial, malformed, or
quarantined collection refuses installation.

If a target commit is confirmed but rollback cannot be confirmed, the app
deletes and verifies absence of its fleet-only Keystore key before process
quarantine; a restart therefore cannot resurrect the target or prior encrypted
snapshot.

Replace raw `Update bearer` with `Reconnect fleet`. Reconnect uses the same QR
and redeem path, but its single commit may change bearers only when the QR's
labels, origins, and handles exactly equal the complete readable installed
fleet. It cannot repair quarantine or identity replacement. Those remain
explicit app-data reset/reinstall operations outside the app.

## 5. Content Contracts

Each feature begins with its named designer freezing content and its rubric in
the red proof. Content changes after green require that owner and proof again.

| Feature / designer | Required schema | “Good” means |
|---|---|---|
| GitHub release / release-content designer | version, source SHA, signer fingerprint, asset/checksum table, install steps, host-before-phone update order, known limits | exact, terse, copyable, no unsupported compatibility or CI claim |
| Converge + doctor / operator-content designer | `PASS|WARN|FAIL`, stable key, host/component fact, one next action | distinguishes installed/running/connected; names exact failing boundary; leaks no secret or terminal content |
| Fleet QR / security-content designer | title, five-minute/single-use warning, QR, phone action, regeneration action | one task per screen, no durable bearer, no token in logs/files/argv, never calls the QR permanent |
| Android connect / product-accessibility designer | ready, external-Tailscale, scanning, connecting, success, failure, reconnect states; visible and spoken labels | literal `Connect`; truthful boundary; one recovery action; focus order and touch targets remain accessible |
| API / API-security content designer | exhaustive code/status/frozen-message table | typed, stable, actionable where safe, non-oracular, no raw downstream error |

Frozen core product copy:

- Title: `Connect your fleet`
- Body: `Sign in to Tailscale, then scan a fresh fleet invite from your MacBook.`
- Primary: `Connect`
- External action: `Open Tailscale`
- Progress: `Connecting to 3 machines…`
- Failure: `Couldn’t connect the whole fleet. Nothing was saved. Create and scan a new fleet invite.`
- Repair: `Reconnect fleet`
- CLI: `Fleet invite ready. It expires in 5 minutes and works once. On the phone, open Skíðblaðnir and tap Connect.`

## 6. Hard-Cut Removal And Reuse

Delete, do not deprecate:

- `MachineProvisioningInstrumentedTest.kt`, `scripts/test provision`, its ADB
  staging/env contract, and all release provisioning entrypoints;
- `provisionFixedCollection`, the no-machine dashboard copy, and manual bearer
  text entry;
- `scripts/install-devbox`, `scripts/install-macbook`, `deploy/systemd`,
  `deploy/launchd`, and duplicated deployment assets after `dev-server` owns
  them;
- `scripts/test devbox|macbook` publication ownership after equivalent
  `dev-server` gates exist;
- Devbox/MacBook-size constants and the headerless inventory exception; and
- `gatewayProfiles` plus host-specific tmux/Codex fields in `internal/platform`.

Reuse and centralize:

- `sessions.Profile` and its validation for deployment-supplied profiles;
- `machine.Handle`, canonical origins, `GatewayBearer`, strict JSON helpers,
  API error mapping, and controller machine-isolation primitives;
- `MachineStore` sealing/quarantine and bearer-rotation rules;
- the current content-free, process-lifetime Codex status adapter without
  parsing payloads or adding Claude/thread hooks; and
- one auth credential-file reader for verify, canonical read, and
  domain-separated digest instead of re-reading bearer files in pairing code.

## 7. Non-Overlapping Implementation Edges

Every builder writes its own behavioral red, observes the intended failure,
implements only its paths, then refactors green. A verifier is read-only. Root
alone edits architecture, roadmap, catalogue, composition, and integration
seams. Work may be parallel only where rows do not overlap.

| Edge / role | Owned paths | Deliverable |
|---|---|---|
| 0. Contract / root | `docs/architecture.md`, `docs/roadmap.md`, this spec | Accept three-host/two-phone topology and the gates before production work |
| 1. Host config / Go builder | `internal/hostconfig/**`, `internal/platform/**`, their tests | Strict config, native platform check; no CLI edit |
| 2. Pairing / Go builder | `internal/pairing/**`, `internal/auth/**`, `internal/gateway/**`, their tests | One-slot invitation and API; no CLI edit |
| 3. Android data / Android builder | `FleetInvite.kt`, `GatewayClient.kt`, `MachineStore.kt`, focused unit/storage tests | Strict schema, redeem, atomic install/reconnect; no UI/controller edit |
| 4. Android experience / UI builder + content designer | `FleetConnectScreen.kt`, scanner adapter, `SkidbladnirController.kt`, `DashboardScreen.kt`, `MainActivity.kt`, focused component tests | Connect/reconnect states and final copy; no storage edit |
| 5. Release / release builder + content designer | `scripts/build`, `scripts/release`, signing/version checks, release-focused tests | Exact artifacts and draft release; no workflow/composition edit |
| 6. Host runtime / `dev-server` builder + operator designer | `assets/skidbladnir/**`, `lib/skidbladnir.sh`, package lists, `workstation`, Ansible role/playbook, focused tests | Three strict configs, packages, service/Serve convergence, doctor |
| 7. Fleet invite / `dev-server` builder + security designer | top-level `skidbladnir`, invite library/tests, README section | Fixed three-host aggregation and stdin-only QR |
| 8. Integration / root | `cmd/skidbladnir/**`, Gradle dependency/version seam, `scripts/test`, deletions, cross-repo pin | Compose frozen APIs; remove old paths; no new behavior |
| 9. Verification / verifier | read-only | Fresh exact-SHA evidence and honest `NOT_RUN` gates |

Edges sharing `cmd/skidbladnir`, Gradle, packages, or test composition are
serialized through edge 8. No agent message may widen scope or acceptance.

## 8. Red/Green Proof Shape

One proof owns each boundary; lower-layer fakes do not claim the next boundary.
Routine verification remains hermetic and invokes no tmux, SSH, Tailscale,
GitHub mutation, ADB, or physical device.

| Boundary | Red first / green fact | Gate |
|---|---|---|
| Host config | Linux hardcoding cannot represent Arch/all profiles; strict Devbox/MacBook/Arch fixtures parse, wrong platform/version/unknown fields fail | routine |
| Pairing concurrency | two simultaneous redeems can both win; exactly one wins, while expiry/replacement/restart/bearer rotation reject uniformly | routine gateway service test, no tmux |
| Fleet schema | malformed/extra/duplicate/variable fleets enter redeem; only the exact canonical three-machine payload reaches it | Android unit |
| Android persistence | a partial or pre-existing store can be overwritten; install/reconnect is one exact encrypted commit or no readable change | Android instrumentation |
| Connect content/state | fresh install shows legacy administration; it shows the frozen Connect flow and failure saves nothing | Android component |
| Release | an unsigned/mis-versioned/unlisted asset can stage or a draft can be mistaken for distribution; exact public repo, clean-main SHA/hosted verify, signer, tag/SHA, two bundles, checksums, then immutable public exact-five download are mandatory | pre-publication release and post-publication read-only gates; `NOT_RUN` without their evidence |
| `dev-server` rendering | pin/config/service drift passes; fixtures fail on drift and doctor keys are stable and credential-free | `dev-server` routine |
| Native tmux composition | fixture behavior is mistaken for tmux behavior; configured binary performs existing inventory/create/attach/kill semantics on one isolated `-L` socket | separately approved integration |
| Host convergence | an install changes identity/session lifetime or needs a manual daemon; reinstall preserves handle/bearer/tmux lifetime and service survives login/reboot | separately approved live gate on each host |
| Physical product | injected scan/network is mistaken for product; signed release update preserves the fleet before reconnect, real scanner pairs, process recreation preserves routing, all hosts render, and one-host outage/recovery stays isolated while lifetime digests remain unchanged | separately approved S22+ product gate with bounded inventory-reconciliation capability for gateway-owned character metadata and stale phone shadows |
| Second user | first phone or ADB install substitutes for distribution proof; named second API 36 phone downloads the public GitHub APK in its browser, installs an exact digest/signer match, and pairs with a new QR | separately approved second-phone gate; otherwise `NOT_RUN` |

Apply the 80/20 rule: exhaustive pure tests at local validation/concurrency
boundaries; one real proof at storage, gateway, host, scanner, and release
ownership boundaries. Do not multiply end-to-end permutations already owned by
lower tests. Live gates preserve production tmux sessions and never kill,
resize, rename, or retarget anything they did not create on an isolated socket.

## 9. Acceptance Criteria

The capability is complete only when:

1. A public immutable release exposes the exact five assets; APK install/update
   preserves the signer and app data, and host artifacts report the same
   tag/SHA.
2. `dev-server` converges and diagnoses the pinned gateway on Devbox, MacBook,
   and Arch without changing an existing handle, bearer, or tmux lifetime.
3. A fresh phone needs only APK install, one-time Tailscale sign-in, `Connect`,
   and one fresh QR; no ADB, laptop-side secret copying, or manual process start
   is involved.
4. Every QR is exact-three, five-minute, one-use-per-host, nonpersistent, and
   new per phone. Concurrent double redemption yields one winner.
5. Android stores all three encrypted credentials atomically or stores none;
   existing/quarantined data is never overwritten. Reconnect changes bearers
   only for the exact installed identities.
6. Dashboard, terminal routing, create, and kill remain bound to the selected
   immutable machine. One gateway outage cannot block or retarget another.
7. All five declared Codex/Claude profiles are supplied from each host config;
   tmux remains the sole session/lifetime truth.
8. No coordinator, public ingress, secret in source/release/logs/argv, legacy
   provisioning path, default host profile, compatibility fallback, or partial
   retry remains.
9. Routine and required boundary proofs are green at the exact source SHA.
   Every unapproved/unavailable live or physical gate is reported `NOT_RUN`.

## 10. Expected File Surface

Skíðblaðnir additions/changes are limited to:

- `docs/architecture.md`, `docs/roadmap.md`, this spec;
- `internal/hostconfig/**`, `internal/pairing/**`, focused `auth`, `gateway`,
  `platform`, and `cmd/skidbladnir` composition;
- Android `FleetInvite`, scanner/connect UI, client/store/controller seams, and
  their focused tests;
- Android version/scanner dependency, `scripts/build`, `scripts/release`,
  signing/release checks, and root test composition; and
- deletion of the legacy provisioning/install/deploy surfaces named above.

`dev-server` additions/changes are limited to:

- `assets/skidbladnir/**`, `lib/skidbladnir.sh`, package lists, `workstation`,
  one Ansible role/playbook inclusion, and their tests;
- top-level `skidbladnir` operator command, one gitignored mode-0600 fleet
  manifest, and a concise README runbook; and
- a pinned release tag plus exact per-asset digests, never copied source or
  committed credentials; and
- deletion of the retired Skíðblaðnir interception branch from
  `assets/routers/ai-router` plus its now-dead focused assertions. The general
  AI router remains unchanged outside that hard-cut removal.

Anything else is a new reviewed capability, not implementation discretion.
