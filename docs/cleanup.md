# cleanup campaign

working tracker for the 2026-10-21 code-reduction campaign: map, decisions, plan, findings and progress.
the coordinator updates it with each merged pr; it is deleted when the campaign ends.
[architecture](architecture.md) stays the system contract; this file only records the campaign.

## target

baseline is origin/main `12b1516` (v0.20.0): about 94.2k lines.

| part | lines | mapper ideal |
| --- | --- | --- |
| go (host, desktop) | 34,792 | ~14k with cuts |
| kotlin (android) | 15,766 | ~7k with cuts |
| markdown docs | 32,759 | ~3k |
| xterm fork patch | 5,444 | 0 (stock xterm) |
| scripts and deployment | 3,154 | ~1.5k |
| terminal page js | 1,192 | ~1k |
| total | ~94.2k | ~23k (about 1/4) |

- with no product cuts the slice ideals sum to about 27k lines of code (45%).
- 1/5 of code (~12k) would also cut grammar precision, workspace recovery, the directory picker and pressure detail; those stay.
- static analysis finds 11 unreachable functions: the bloat is live complexity, not dead code.
- root cause: docs/rules mandated same-system distrust (216 `justify-*` comments, 94 defensive panics, 297 kotlin `require`/`check`).

## progress

measured over every tracked non-binary file (fixtures, json and docs included), so the base differs from the mapper total above.

| point | lines | change |
| --- | --- | --- |
| baseline `12b1516` | 101,192 | |
| after wave 1 `9e7da6e` | 91,926 | -9,266 (-9.2%) |
| after #129-#139 `623280a` | 87,753 | -13,439 (-13.3%) |
| wave 2 done `a00bd7a` | 85,115 | -16,077 (-15.9%) |
| after wave 3 `c4399a3` | 79,543 | -21,649 (-21.4%) |
| wave 4 phase 2 `6782385` | 73,125 | -28,067 (-27.7%) |
| wave 4 done `defbf3f` | 70,855 | -30,337 (-30.0%) |

## decisions

owner-approved product cuts (2026-10-21):

| area | cut | approx. lines | accepted cost |
| --- | --- | --- | --- |
| fleet | fixed hub; delete live hub transfer and relay | -700 | moving the hub = `skid recover` plus re-enrolment |
| fleet | no ed25519 signatures; plain hub json over tailnet tls plus bearer | -400 | no forgery protection while the hub is down; a removed phone shows an auth failure |
| install | fixed paths, no per-file receipt, prune old releases | -380 | hand-edited skid-owned files are overwritten |
| config | drop `machines:` overrides | -90 | dev-server yaml changes in step |
| notifications | hub-owned attention, cross-device acknowledgement | ~-4,900 | acknowledging on any device clears everywhere |
| notifications | drop exact-foreground banner suppression | -650 | a session you are watching can banner |
| quota | go codex quota client replaces the python helper | -110 code, -146 MB per release | skid owns the app-server protocol |
| accessibility | drop talkback and screen-reader support | ~-530 | screen-reader claude sessions read unknown |
| cli | `info --explain` prints reason and rule ids only | -310 | no region, capture or timing detail |
| terminal | stock xterm.js replaces the patched fork | ~-5,800 | physical-phone acceptance stays `NOT_RUN` |

also accepted 2026-10-21:

- the [tmux exec budget](tmux-exec-budget.md) option a and its priority change, including a3's amendments to architecture's agent-observation paragraph and session latency's no-cache scope.
- the mac serve event log goes to a bounded file, `~/.local/state/skidbladnir/serve.log`, rotating at 8 MiB with one previous file; linux keeps the journal.
- w2.7's amendment to a1 (the global probe reads `-s`, `-g` and `-g -w`), its window/pane-local residual and its status changes (an absent id answers 404, an id on an epochless server 409).
- the lockstep [cutover release](issues/release-cutover.md) is v0.21.0 (`2c7cbe0`), tagged after wave 2; the wave-3 prs merged after it.
- the coordinator cuts v0.20.1 from main once w1.7 and w1.8 merge; w2.1 and w2.2 (hard cutovers) merge only after that tag. dev-server's pin bump and the macbook maintenance window stay with the owner.

kept (owner, 2026-10-22): xterm `screenReaderMode` stays on. it also selects xterm's keydown and `insertText` input path; with it off, stock xterm lost the first Gboard letter in 4 of 72 emulator runs against 0 of 87 with it on.

decided 2026-10-22:

- merge #144 now (owner): it merged as `3ef8c2e`. its install is a second lockstep window after v0.21.0, in [attention cutover](issues/attention-cutover.md).
- android-core's verify-before-attach cut stands (owner), reversing the 2026-10-20 deferral: reconnect uses the last poll's session and a created terminal attaches on the create response. only the [notification-click read](issues/phone-entry-metadata-latency.md) remains.
- drop the phone's icu group-label mirror (owner, W4.11): the host's 422 `GroupInvalid` is the only check, and the phone loses instant inline validity.
- [rules](rules.md#trust-boundaries) amended in #157 (coordinator): go clients decode same-release documents with plain `encoding/json`, a missing key is its zero value, and an unknown error code is still the gateway's answer.

kept: runtime firebase configuration, the tui `N` create form, the usage selector, workspace restore verification, classifier precision, the identity-token format.

process:

- one finding cluster per pr, branch `cleanup/<slug>`, merged to main only; no release, re-pin or deploy without a separate request.
- temporary boundary tests run against the original and the rewrite, then are deleted before commit ([testing](rules.md#testing)).
- tests use isolated `tmux -L` sockets and loopback gateways with temporary state; never the production gateway, the owner's tmux server or other hosts. every tmux or integration run needs the owner's approval in the current turn, per pr.
- android end-to-end tests use the emulator only, never the phone.

## open questions

- [ ] release audit: delete `release-pin.json` and the published-release gate (-214)? the release ritual and dev-server pins cite them.
- [ ] phone controller: keep state across an app switch instead of resetting to booting (-120)?
- [ ] phone dashboard: restore scroll by index, not card identity, after process death (-200)?
- [ ] forge: drop the forced refresh after an uncertain create (-110)? a second tap could double-create.
- [ ] phone saved state: drop the sha-256 label fingerprints that keep labels out of the saved-state bundle (-150)?
- [ ] housekeeping: close draft pr #102 and remove the ten merged `fix/*` worktrees? (local main's drafts were triaged by #160 at the owner's request; the checkout matches main, the originals are on the local `backup/local-docs-2026-10-22` and `backup/stash-2026-10-02` branches.)

## slices

mapper estimates; lines include each slice's own docs.

| slice | paths | lines | ideal | core problem |
| --- | --- | --- | --- | --- |
| ops-docs | scripts, .github, evidence, docs | 34,855 | 6,800 | ~45 feature specs amend each other; 22 historical docs; 11 release scripts re-verify the same facts |
| android-terminal-notify | Terminal*.kt, Notification*.kt, assets/terminal, xterm patch | 11,414 | 3,000 | 82% of the 5.4k xterm patch is toolchain lockfile; one-apk page protocol re-validated on both sides |
| desktop-clients | internal/{fleetclient,agentcli,sessionui,terminalclient} | 7,710 | 4,000 | mirrors and re-validates the gateway wire; tui update has 159 branches |
| android-core | SkidbladnirController, ProductModel, GatewayClient, stores | 7,696 | 3,700 | 2,438-line controller with hand-rolled concurrency; host drift crashes the phone |
| host-fleet | cmd/skidbladnir, internal/{fleet,install,hostconfig,fcmpush,machine,auth} | 6,944 | 3,100 | crash-safe distributed protocol (signed state, epochs, 5-phase hub transfer) for a hub that never moves |
| sessions-tmux | internal/{sessions,tmux,group,catalog,terminalcontext} | 6,021 | 3,100 | three parallel tmux read paths; self-validating checkpoints; error-type classifier |
| gateway | internal/{gateway,terminal,strictjson,logging,workdir,pressure} | 5,260 | 3,000 | three hand-written route matchers; 460-line typed logging algebra; dtos mirrored and re-validated |
| agent-observation | internal/{agentcontrol,agentruntime,process} | 5,003 | 3,850 | grammars mostly earned; bloat is diagnostics threading, legacy argv, mirrored enums |
| android-ui | dashboard, cards, sheets, pickers, theme, seals | 4,778 | 1,900 | six hand-rolled sheets; parallel narration layer; 6k lines of ui delta docs |
| host-notifications | internal/{attention,notifier,macnotifications}, macos | 4,547 | 2,450 | two device reducers, epoch-reset ritual, foreground-visit machinery, stringly-typed mac actor |

cross-cutting: the session row is modelled four times, the error envelope is pinned in nine tables, an atomic private-file write is reimplemented about nine times, and enum-to-prose tables are duplicated in go and kotlin.

## prs

waves run in parallel on disjoint paths and merge in order. status: planned, running, review, merged, partial, dropped. estimates are the mappers' and overlap where findings share code; merged rows record measured net lines.

| pr | findings | est. lines | status | merged as |
| --- | --- | --- | --- | --- |
| W1.1 rules-trust-boundaries | 3 | -1,025 | merged | #121, -910 |
| W1.2 docs-history-purge | 3 | -5,987 | merged | #122, -6,003 |
| W1.3 xterm-patch-trim | 2 | -5,010 | merged | #123, -565 |
| W1.4 go-dead-surface | 9 | -430 | merged | #124, -498 |
| W1.5 android-dead-surface | 7 | -295 | merged | #125, -302 |
| W1.6 ops-ledgers | 5 | -1,392 | merged | #126, -1,339 |
| W1.7 launchd-interactive (exec budget priority) | 0 | small | merged | #131, +71; shipped in v0.20.1 |
| W1.8 tty-probe-noise | 0 | 0 | merged | #129, -24; shipped in v0.20.1 |
| R1 release v0.20.1 | | | merged | v0.20.1 at `05511dc`, pinned by #132 |
| W2.1 fleet-fixed-hub | 11 | -1,880 | merged | #137, -2383 |
| W2.2 install-fixed-paths | 4 | -745 | merged | #138, -497 |
| W2.3 config-one-shape | 5 | -307 | merged | #142, -293 |
| W2.4 gateway-http | 8 | -620 | merged | #141, -688 |
| W2.5 logging-slog + bounded mac serve log | 2 | -470 | merged | #146, -460 |
| W2.6 host-resources + startup-composition rest | 5 | -390 | merged | #147, -715 |
| W2.7 sessions-read-path + exec budget a1 | 8 | -745 | merged | #139, -183 |
| W2.8 recovery-simplify | 8 | -575 | merged | #148, -494 |
| W3.1 hub-attention | 22 | -4,875 | merged | #144, -3,809 |
| W3.2 quota-go-client | 3 | -165 | merged | #145, -160 |
| W3.3 talkback-removal | 3 | -340 | merged | #135, -580 |
| W3.4 observation-cuts | 3 | -540 | merged | #133, -610 |
| W3.5 stock-xterm | 1 | ~-1,300 after w1.3 | merged | #143, -5,432 |
| W4.1 tolerant-reader | 10 | -1,950 | partial | kotlin decoders in #154, go in #157, counted there; terminal frames and the page protocol move to W4.12, W4.13 |
| W4.2 go-wire-package | 4 | -540 | merged | #157, -600 |
| W4.3 host-rendered-copy | 9 | -930 | merged | #152, -535 |
| W4.4 desktop-core | 8 | -650 | merged | #155, -191 |
| W4.5 android-controller | 13 | -1,945 | merged | #154, -1,851 |
| W4.6 android-ui-structure | 9 | -943 | merged | #158, -1,017 |
| W4.7 terminal-page | 6 | -420 | merged | #159, -633 |
| W4.8 observation-refactors + exec budget a2, a3 | 9 | -384 | merged | #156, -20 |
| W4.9 private-file-primitive | 1 | -150 | merged | #151, -33 |
| W4.10 release-pipeline | 2 | -872 | merged | #136, -389 |
| W4.11 group-label-mirror | 1 | -120 | planned |  |
| W4.12 terminal-connection-decode | 1 | -50 | planned |  |
| W4.13 page-protocol-validation | 1 | -250 | planned |  |
| W4.14 api-vocabulary | 1 | -10 | planned |  |
| W4.15 fleetclient-split | 1 | -40 | planned |  |
| W5 docs-consolidation | 11 | ~-20,000 (mapper docs estimates overlap) | planned |  |

## spec items

non-mapper work folded into the waves.

| item | source | pr |
| --- | --- | --- |
| a1 inventory in one exec | [exec budget](tmux-exec-budget.md#a1-inventory-in-one-exec) | W2.7 |
| a2 all captures in one exec | [exec budget](tmux-exec-budget.md#a2-all-captures-in-one-exec) | W4.8, after W3.4 |
| a3 activity-gated reuse, plus the architecture and session-latency amendments | [exec budget](tmux-exec-budget.md#a3-activity-gated-reuse) | W4.8, after W3.4 |
| `ProcessType=Interactive` and the install priority check | [exec budget](tmux-exec-budget.md#priority-macbook-capacity-d7) | W1.7 |
| zsh tty probe noise | terminal-context-tty-noise issue, resolved by #129 | W1.8 |
| bounded mac serve log | [codebase map](codebase-map.md) | W2.5 |
| inline `DashboardScreen.onOpenTerminal`, always `controller::openTerminal` | #158 | W4.11 |
| delete `Json.kt`'s `ProtocolDecodeException` residue and `ProductModel.parseApiErrorCode`; the terminal upgrade shows the gateway's message, not the phone copy table | #154 | W4.12 |

## findings

every mapper finding, by pr. est. is the mapper's estimate; class is refactor, deletion or product cut.

| id | finding | est. | class | pr |
| --- | --- | --- | --- | --- |
| `cross:validation-policy-root` | The rulebook mandates same-system distrust; it is the generator of most defensive code in every slice | -100 | refactor | W1.1 |
| `ops-docs:rules-consolidate` | 24 micro rule files, partly generic and inapplicable, plus an unenforced justify-* token convention | -905 | refactor | W1.1 |
| `ops-docs:agents-readme-drift` | AGENTS/README duplicate the testing policy and link historical docs as contracts | -20 | refactor | W1.1 |
| `ops-docs:docs-historical-qualification-research` | 22 docs (4,686 lines) are self-declared historical records, qualification logs or research | -4,686 | deletion | W1.2 |
| `ops-docs:issues-stale-subject` | 9 issue records describe code that no longer exists | -301 | deletion | W1.2 |
| `ops-docs:issues-acceptance-debt` | 31 issue records (1,118 lines) are NOT_RUN acceptance checklists for long-shipped releases | -1,000 | cut | W1.2 |
| `android-terminal-notify:xterm-patch-toolchain-remediation` | 82% of the xterm patch is dev-toolchain lockfile/audit remediation that never ships | -4,550 | deletion | W1.3 |
| `android-terminal-notify:terminal-asset-lock-redundant` | terminal.lock + check-terminal-assets re-implement git content addressing and validate their own shape | -460 | deletion | W1.3 |
| `host-fleet:dead-surface` | Unreachable routes, fields, files and options across fleet/hostconfig/install | -90 | deletion | W1.4 |
| `gateway:healthz-unused` | /healthz endpoint has no consumer | -25 | cut | W1.4 |
| `gateway:dead-and-redundant-branches` | branches that cannot change any outcome | -45 | deletion | W1.4 |
| `gateway:composition-assertions` | constructor/listen assertions re-check what the single composition caller guarantees | -45 | deletion | W1.4 |
| `desktop-clients:dead-branches` | caller-proven dead code and impossible branches | -45 | deletion | W1.4 |
| `host-notifications:dead-surface` | Dead and vestigial surface: `status` subcommand, attention.Config, LocalRevision, duplicate ErrUnavailable,... | -45 | deletion | W1.4 |
| `host-notifications:receiver-tombstones-unreachable` | Removed-receiver tombstones and their Forbidden/'removed' branches are unreachable behind fleet bearer revo... | -40 | refactor | W1.4 |
| `host-notifications:redundant-locks-and-exhaustion` | Observer-file flock duplicates fleet's authority.lock; 13 int64-exhaustion checks guard counters that canno... | -60 | deletion | W1.4 |
| `sessions-tmux:preflight-create-vestige` | PreflightCreate is a vestige of removed codex conversation creation | -35 | refactor | W1.4 |
| `android-core:dead-reset-reasons` | three of four fleet reset reasons and one quarantine kind are never constructed | -30 | deletion | W1.5 |
| `android-core:dead-dashboard-state` | write-only state field, redundant predicates, aliases and an unreachable serializer path | -80 | deletion | W1.5 |
| `android-ui:debug-seal-gallery` | debug-only exported seal gallery for a frozen formula | -95 | cut | W1.5 |
| `android-ui:press-indication-boilerplate` | nine `remember { MutableInteractionSource() }` for an IndicationNodeFactory | -15 | refactor | W1.5 |
| `android-ui:misplaced-helpers` | shared helpers live in whichever screen first needed them | -20 | refactor | W1.5 |
| `android-core:main-activity-misplaced` | theme definitions and a single-caller host composable live in MainActivity | -25 | refactor | W1.5 |
| `android-core:duplicate-lifetime-predicate` | session-lifetime equality inlined 12 times beside an existing helper | -30 | refactor | W1.5 |
| `ops-docs:evidence-live-dead-ledger` | evidence/live is a dead p0 proof ledger hashing files that no longer exist | -337 | deletion | W1.6 |
| `ops-docs:catalogue-provenance-ledger` | check-catalogue enforces a second hand-maintained copy of a frozen catalogue | -205 | refactor | W1.6 |
| `ops-docs:check-gate-collapse` | two near-identical gates plus redundant and no-op static steps | -110 | refactor | W1.6 |
| `ops-docs:android-signing-check` | check-android-signing duplicates gradle's signing config parser and has a dead mode | -195 | refactor | W1.6 |
| `android-ui:gen-ornament-generator` | 525-line python generator and --check gate for ~116 lines of frozen constants | -545 | cut | W1.6 |
| `host-fleet:hub-transfer-protocol` | Five-phase hub-transfer 2PC protocol for a hub that never moves | -700 | cut | W2.1 |
| `host-fleet:signed-trust-model` | Ed25519-signed state, signed removal proofs, retired roster and relay for one owner's machines | -400 | cut | W2.1 |
| `host-fleet:crash-only-role` | Uncertainty fences, generation counters and 10 lock-held callbacks instead of crash-only restart | -300 | refactor | W2.1 |
| `host-fleet:fleet-files-as-api` | fleet's private multi-file state is read and re-verified by four other packages; staging anchor patches non... | -110 | refactor | W2.1 |
| `host-fleet:startup-composition` | Serve startup re-reads its own state, probes tmux -V for nothing, and aborts on login-shell lookup despite... | -85 | refactor | W2.1 |
| `host-fleet:enrollment-pending-recovery` | Persisted pending enrollment and pending-bearer probe exist only because re-join of an enrolled machine is... | -60 | cut | W2.1 |
| `host-fleet:self-signed-invitation` | Invitation proof is verified only against the key carried in the same invitation | -60 | refactor | W2.1 |
| `host-fleet:one-peer-http-primitive` | Peer HTTP request/error code copy-pasted across fleet, CLI, fleetclient, gateway, notifier | -90 | refactor | W2.1 |
| `cross:fleet-view-in-process` | The hub consumes its own in-memory fleet state through the desktop client library by re-reading and re-veri... | -40 | refactor | W2.1 |
| `desktop-clients:refresh-defensive` | Refresh re-validates and anti-rollback-checks files written by the host's own service; dead MachineByLabel;... | -35 | deletion | W2.1 |
| `host-fleet:installer-receipt` | Per-file ownership receipt (48,821 entries) re-hashed on every install/update/connect; releases never prune... | -380 | cut | W2.2 |
| `host-fleet:tailnet-route-ownership` | Receipt plus serve-status inspection to own one fixed tailscale serve route | -140 | cut | W2.2 |
| `host-fleet:release-validate-redundancy` | Installer re-proves signed-release invariants; ships unused 27 MB uv and 2.3 MB source per release | -55 | refactor | W2.2 |
| `host-fleet:paths-duplicate` | State-directory resolution duplicated; notifications app flags shadow env and ignore XDG_DATA_HOME | -30 | refactor | W2.2 |
| `host-fleet:machine-handle-file` | 125 LOC to create a 36-byte random id file whose value is also stored in identity.json | -140 | refactor | W2.1, file kept: it is the only identity root the cutover keeps |
| `host-fleet:hostconfig-triple-validation` | Config yaml parsed and validated three ways, plus a JSON-to-YAML re-parse on every receive | -140 | refactor | W2.3 |
| `host-fleet:machine-overrides` | Per-machine override schema (shell/terminal/default_machine/profile command+home) where the live fleet uses... | -90 | cut | W2.3 |
| `host-fleet:android-push-dual-struct` | Same four Firebase fields in two structs with two JSON casings, converters and a coherence check | -25 | refactor | W2.3 |
| `host-fleet:shell-validation-x4` | Configured shell validated in four layers; terminal-exec re-validates its own tmux-built arguments | -30 | refactor | W2.3 |
| `host-fleet:fcmpush-double-parse` | Service-account key parsed twice (hand-rolled PEM/PKCS8/RSA, then JWTConfigFromJSON) | -22 | refactor | W2.3 |
| `gateway:route-table-triplicated` | three hand-written route matchers plus per-handler path parsers instead of one ServeMux table | -130 | refactor | W2.4 |
| `gateway:json-response-writers` | five hand-rolled json writers with three different size-bounding schemes | -50 | refactor | W2.4 |
| `gateway:outbound-queue` | byte-bounded OutboundQueue and writer goroutine exist only to serialize writes the websocket library alread... | -190 | refactor | W2.4 |
| `gateway:terminal-reauth-poll` | terminal monitor re-authenticates every 2s with a synthetic request although revocation is pushed via Close... | -15 | refactor | W2.4 |
| `gateway:live-connection-lifecycle` | live-connection registry with done channels, lifecycle permit and deadline slicing is heavier than tmux sem... | -100 | refactor | W2.4 |
| `gateway:stringfield-null-presence` | stringField/null-rejection/presence machinery duplicates validation the services already perform | -60 | refactor | W2.4 |
| `gateway:terminal-protocol-self-validation` | terminal encoders validate their own output and decoders pin copy; callers convert impossible errors into I... | -45 | refactor | W2.4 |
| `gateway:input-validation-triplicated` | terminal text/keys/maxBytes bounds are validated byte-identically in fleetclient, gateway and agentcontrol | -30 | refactor | W2.4 |
| `gateway:logging-typed-event-algebra` | logging is a 460-line closed event algebra re-validated twice with panicking callers; replace with log/slog | -380 | refactor | W2.5 |
| `gateway:sessions-observed-phases` | per-phase inventory latency instrumentation threads six optional durations through gateway and logging | -90 | cut | W2.5 |
| `gateway:workdir-listing-race-hardening` | directory listing defends against concurrent local mutation although its spec calls listings advisory | -60 | refactor | W2.6 |
| `gateway:workdir-opaque-types` | workdir opaque-type zoo forces re-parse-and-panic in sessions and re-closing in gateway | -70 | refactor | W2.6 |
| `gateway:pressure-internals` | pressure keeps three sample representations, literal-validating panics and defensive clones | -60 | refactor | W2.6 |
| `gateway:pressure-wire-owned-by-pressure` | pressure wire shape is re-built and re-validated in gateway/dto.go instead of owned by internal/pressure | -200 | refactor | W2.6 |
| `sessions-tmux:one-session-read-path` | three parallel tmux read paths; hot paths use per-field execs | -280 | refactor | W2.7 |
| `sessions-tmux:redundant-lifetime-prechecks` | client-side lifetime pre-checks duplicate the atomic tmux if-shell guards | -90 | refactor | W2.7 |
| `sessions-tmux:reclassify-and-recapture-merge` | source-file batch, framing and metadata parsing implemented twice; length framing has a recorded race | -180 | refactor | W2.7 |
| `sessions-tmux:sample-path-smells` | duplicated foreground sampling and leftover scaffolding | -40 | refactor | W2.7 |
| `sessions-tmux:create-shell-guarded-branch` | shell creation re-encodes the whole new-session argv into an if-shell guard | -30 | refactor | W2.7 |
| `sessions-tmux:mutation-checkpoint-synchronous` | every mutation runs a full checkpoint capture plus two fsyncs inline under the lock | -20 | refactor | W2.7 |
| `sessions-tmux:character-normalization` | character normalization defends races with writers that do not exist | -70 | refactor | W2.7 |
| `sessions-tmux:naming-failure-reclassification` | rename and auto-name failure paths re-resolve and re-scan to guess causes | -35 | refactor | W2.7 |
| `sessions-tmux:recovery-error-taxonomy` | error-type reverse-engineering classifier, self-validating fact matrices and production panics | -150 | refactor | W2.8 |
| `sessions-tmux:checkpoint-reader-revalidation` | checkpoint reader re-walks its own JSON and re-validates data validated before write | -100 | refactor | W2.8 |
| `sessions-tmux:recovery-loop-capture-redundancy` | recovery loop and capture triple-check the same server lifetime | -50 | refactor | W2.8 |
| `sessions-tmux:validate-workspace-limits` | limit tables and structural re-validation of tmux's own coherent output | -50 | refactor | W2.8 |
| `sessions-tmux:duplicate-identity-validators` | same id patterns and server-identity readers duplicated across packages and layers | -70 | refactor | W2.8 |
| `sessions-tmux:sentinel-translation` | parallel error sentinel sets translated between tmux and sessions | -35 | refactor | W2.8 |
| `sessions-tmux:catalog-embed` | catalogue shipped as a runtime file and re-validated although CI validates it | -60 | refactor | W2.8 |
| `sessions-tmux:terminal-context-overobservation` | terminal context probe re-observes for TOCTOU, lives on Manager, and the registry has locks/caps | -60 | refactor | W2.8 |
| `cross:hub-owned-attention` | The attention device reducer is implemented twice (Go, Kotlin) with separate stores and epoch-reset rituals... | -1,250 | cut | W3.1 |
| `host-notifications:presentation-state-machine` | claimed/posted/closed presentation, unresolved-add map, silent repair and settle/authorize round-trips coll... | -250 | cut | W3.1 |
| `host-notifications:epoch-reset-and-handoff-transfer` | Explicit epoch reset plus hub-handoff transfer of observer state exist only to avoid an automatic device re... | -300 | cut | W3.1 |
| `host-notifications:darwin-socket-device-path` | Darwin routes device-memory reads and acks through the app socket while linux uses the file directly for th... | -300 | refactor | W3.1 |
| `host-notifications:mac-fetch-fencing` | hintFloor/hintFence/refetch/generation fetch fencing in the mac owner collapses to single-flight plus trail... | -35 | refactor | W3.1 |
| `host-notifications:minute-fcm-pulse` | Minute pulse sends an FCM 'sync' hint to every phone 1440 times a day, an ntfy-era cache repair | -5 | cut | W3.1 |
| `host-notifications:fcm-enrollment-challenge` | FCM enrollment challenge-proof protocol (desired/confirmed/active triples, expiry renewal, /confirm route)... | -170 | cut | W3.1 |
| `host-notifications:foreground-visit-machinery` | Visit RPC, launch nonces, surface receipts and focus AppleScript exist only to suppress banners for windows... | -650 | cut | W3.1 |
| `host-notifications:layered-json-validation` | Three to four JSON validation passes per document: strictjson + attention.Decode null tree + per-type requi... | -250 | refactor | W3.1 |
| `host-notifications:identity-type-duplication` | attention.Key and fleetclient.Reference are the same identity with duplicated validation and decoding | -40 | refactor | W3.1 |
| `host-notifications:misplaced-helpers-and-store` | attention owns unrelated helpers (text sanitizer, status copy, XDG resolution), while its own device store... | -40 | refactor | W3.1 |
| `host-notifications:owner-stringly-actor` | The mac owner is a stringly-typed actor (19 ops, `any` payloads, 20 justify-assertion casts) where a mutex... | -100 | refactor | W3.1 |
| `host-notifications:double-lifecycle` | Two stacked lifecycles: Observer cancel/done/drained/closed plus notificationHost worker/generation/owned/f... | -60 | refactor | W3.1 |
| `host-notifications:meaningful-diff` | meaningful() is a hand-rolled O(n²) field-by-field diff of machine snapshots | -25 | refactor | W3.1 |
| `host-notifications:duplicate-gateway-auth` | notifier.ServeHTTP re-authenticates, re-binds the machine and re-checks path/query that the gateway already... | -50 | refactor | W3.1 |
| `android-terminal-notify:notification-owner-overbuilt` | NotificationOwner encodes OS-visibility uncertainty and hint floors as persistent state | -600 | cut | W3.1 |
| `android-terminal-notify:epoch-reset-manual` | observer epoch change requires a manual 'reset notification memory' ritual | -80 | cut | W3.1 |
| `android-terminal-notify:notification-wire-invariants` | phone mirrors host invariants in init blocks; generic counter validator misplaced in NotificationStore | -120 | refactor | W3.1 |
| `android-terminal-notify:notification-fetch-cost` | each notification refresh does a fleet fetch, receiver fetch, state fetch and repeated Keystore decrypts; s... | -40 | refactor | W3.1 |
| `android-terminal-notify:push-plumbing-duplication` | duplicated push target derivation, catch ladders, and a typed hint round-tripped through bytes | -80 | refactor | W3.1 |
| `desktop-clients:cut-tui-ready-visit` | PRODUCT CUT (cross-slice): desktop stops participating in ready/acknowledge notification semantics | -350 | cut | W3.1 |
| `desktop-clients:notification-access-misplaced` | attention device store and hub notification client live in fleetclient; notification reading lives in termi... | -80 | refactor | W3.1 |
| `host-fleet:python-quota-runtime` | 146 MB managed Python runtime plus llm-calling pin shipped to make one JSON-RPC call | -110 | refactor | W3.2 |
| `agent-observation:quota-helper-defensive` | quota helper invocation re-checks settings, filters env redundantly, and pre-scans own-helper JSON for null | -45 | refactor | W3.2 |
| `agent-observation:quota-misplaced` | account quota is a separate concept inside agentcontrol; clients import the host classifier for wire types | -10 | refactor | W3.2 |
| `android-ui:a11y-narration-layer` | curated talkback narration runs parallel to every visible fact | -130 | cut | W3.3 |
| `android-ui:bidi-isolation` | bidi isolates on the owner's own labels force duplicate visual/spoken strings | -40 | cut | W3.3 |
| `android-terminal-notify:a11y-wheel-actions` | custom TalkBack wheel actions and xterm screen-reader DOM mirror for a single sighted user | -170 | cut | W3.3; the `screenReaderMode` flip moves to W3.5 |
| `agent-observation:claude-screen-reader` | claude screen-reader renderer support (~230 lines) for an opt-in accessibility mode | -230 | cut | W3.4 |
| `agent-observation:diagnostics-region-capture-elapsed` | explain diagnostics (rule regions, capture facts, stage timings) are threaded through every grammar rule fo... | -180 | cut | W3.4 |
| `desktop-clients:cut-info-explain` | PRODUCT CUT: remove `skid info --explain` detector diagnostics | -130 | cut | W3.4, refuted as separate work and folded into the diagnostics cut |
| `android-terminal-notify:xterm-fork-replace` | the xterm fork solves a three-route wheel problem that collapses to one route under tmux | -5,800 | refactor | W3.5 |
| `cross:tolerant-reader` | Closed-schema strict decoding on every same-system hop forces lockstep releases and turns any drift into a... | -280 | refactor | W4.1 |
| `cross:wire-error-envelope` | Error code/status/message lives in 5 server tables, a logging enum and 3 client pin sets; the pins already... | -120 | refactor | W4.1 |
| `android-core:strict-json-policy` | hand-written duplicate-key JSON scanner, null pre-scans and closed-schema decoding force lockstep phone rel... | -200 | cut | W4.1 |
| `android-core:wire-revalidation` | phone re-derives and re-validates gateway-owned invariants, and any drift crashes the app | -380 | refactor | W4.1 |
| `android-core:error-decoding` | nine per-route closed error decoders plus phone-side copies of the gateway's status and message tables | -250 | refactor | W4.1 |
| `android-core:unused-wire-fields` | decoded and validated fields that nothing on the phone reads | -60 | refactor | W4.1 |
| `android-terminal-notify:terminal-connection-decode` | phone re-validates gateway frames and even compares error message wording | -50 | refactor | W4.1; moves to W4.12 |
| `android-terminal-notify:page-protocol-validation` | both ends of a same-APK message port validate each other exhaustively | -250 | refactor | W4.1; moves to W4.13 |
| `gateway:error-envelope-pinned-four-ways` | error code->status->message table is duplicated and byte-pinned in gateway, fleetclient, android and the te... | -280 | refactor | W4.1 |
| `desktop-clients:mutation-failure-auth-misclassified` | decodeMutationFailure turns real auth rejections on start/shell/group into protocol_error with dispatch unk... | -80 | refactor | W4.1 |
| `cross:go-wire-package` | No shared wire package: the session row is modeled 4 times and clients import host implementation packages... | -60 | refactor | W4.2 |
| `gateway:session-dto-mirrors` | session inventory DTO is copied in gateway, mirrored in fleetclient, and its invariants validated by produc... | -200 | refactor | W4.2 |
| `desktop-clients:wire-dto-duplication` | fleetclient mirrors the gateway wire schema and re-validates every invariant the gateway already enforces o... | -270 | refactor | W4.2 |
| `sessions-tmux:wire-vocabulary-misplaced` | client/server wire vocabulary parked in the host session manager | -10 | refactor | W4.2; moves to W4.14 (status, usage, footer, name-mode, recovery and presentation types into `internal/api`) |
| `cross:host-rendered-copy` | Prose derived from closed host enums is duplicated byte-for-byte in Go and Kotlin; render it once on the pr... | -330 | refactor | W4.3 |
| `cross:host-canonicalizes-identifiers` | Clients re-implement host canonicalization (Unicode-15 NFC mirror, id regexes) because the gateway rejects... | -120 | cut | W4.3, kept by #152; the phone's icu mirror moves to W4.11 |
| `sessions-tmux:group-custom-nfc` | hand-rolled plain NFC to dodge stream-safe CGJ insertion | -45 | cut | W4.3, kept by #152 |
| `android-ui:availability-prose-tables` | five separate `when (machineAvailability)` prose tables; empty state repeats the strip above it | -65 | cut | W4.3 |
| `android-core:availability-copy` | five producers of per-machine availability prose with drifted wording | -40 | cut | W4.3 |
| `android-ui:profile-label-duplicate` | local and remote agent labels are two functions; execution-context prose is re-derived in six `when`s | -35 | refactor | W4.3 |
| `android-ui:pressure-color-roles` | pressure presentation returns enums named after colours, mapped 1:1 by three UI functions | -35 | refactor | W4.3 |
| `gateway:phone-rederives-pressure-policy` | android re-derives the host's pressure platform policy and evaluator to validate the response | -110 | refactor | W4.3 |
| `android-core:pressure-value-model` | nine single-Double PressureValue variants force nine-arm whens everywhere | -150 | refactor | W4.3 |
| `desktop-clients:operation-dispatch-scatter` | each operation's facts are spread over ~10 string switches; Request.Valid re-checks a field/operation matri... | -180 | refactor | W4.4 |
| `desktop-clients:typed-session-reference` | Session.Ref and Request.Ref carry pre-encoded base64 JSON; 36 DecodeReference sites, mostly ignoring errors... | -60 | refactor | W4.4 |
| `desktop-clients:sessionui-observation-lanes` | four hand-rolled copies of one read-lane pattern plus WaitGroup/stop-flag lifecycle machinery for a process... | -130 | refactor | W4.4 |
| `desktop-clients:cli-parse-validation-duplication` | cli parse validates the option matrix, then Request.Valid re-validates it; placeholder text hack; dead writ... | -90 | refactor | W4.4 |
| `desktop-clients:sessionui-update-god-function` | model.Update is cyclo 159 / 420 lines mixing routing, action adoption, search adoption, paste and per-page... | -60 | refactor | W4.4 |
| `desktop-clients:context-enrichment-overbuilt` | remote-context enrichment uses a sync.Map/sync.Once cache that cannot hit, a semaphore that cannot bind, an... | -45 | refactor | W4.4 |
| `desktop-clients:client-duplicates` | small duplicated primitives across the client packages | -45 | refactor | W4.4 |
| `desktop-clients:fleetclient-grab-bag` | fleetclient mixes routing/transport with cli+tui presentation copy and tui palette roles | -40 | refactor | W4.4; deferred by #155 to W4.15 |
| `android-core:hand-rolled-concurrency` | three executors, generation, credential and identity fences, and bespoke poll lanes instead of structured c... | -600 | refactor | W4.5 |
| `android-core:controller-god-file` | 2438-line controller: single-use test-era helpers, misplaced terminal predicates, repeated state assembly | -180 | refactor | W4.5 |
| `android-core:remote-context` | remote execution-context resolution uses flights, global admission, generations and CompletableFuture fan-out | -90 | refactor | W4.5 |
| `android-core:verify-before-attach` | client-side lifetime pre-verification before terminal attach duplicates the gateway's attach-time identity... | -130 | cut | W4.5 |
| `android-core:gateway-client-guards` | every GatewayClient method repeats the origin-null and handle-equality guards; request builders are single-use | -110 | refactor | W4.5 |
| `android-core:fleet-store` | store re-decrypts and re-verifies signatures on every access; ad hoc instances and HTTP clients | -80 | refactor | W4.5 |
| `android-core:product-model-god-file` | ProductModel.kt is a grab bag with Wire/domain mirror pairs and hand-written value classes | -150 | refactor | W4.5 |
| `android-terminal-notify:controller-terminal-misplaced` | terminal attach orchestration and its state vocabulary live in the 2438-line controller | -50 | refactor | W4.5 |
| `android-core:working-directory-picker` | picker state reinvents nullable, over-fences replies and re-checks codes with exhaustive whens | -150 | refactor | W4.5 |
| `android-core:text-size-store` | a DataStore with a custom serializer and a five-state model for one Int preference | -130 | cut | W4.5 |
| `android-terminal-notify:text-size-datastore` | one integer preference modeled as a DataStore with custom codec and a 6-state read/write machine | -170 | refactor | W4.5 |
| `android-terminal-notify:persistence-primitives` | three persistence mechanisms and a duplicated paranoid atomic-write for tiny phone state | -60 | refactor | W4.5 |
| `android-core:build-validation` | gradle re-validates signing and version inputs that the release scripts already validate | -45 | refactor | W4.5 |
| `android-ui:sheet-scaffold` | six bottom sheets hand-roll the same scaffold with drifting title, busy gating and pending UI | -90 | refactor | W4.6 |
| `android-ui:theme-owns-defaults` | M3 theme does not carry the design system, so call sites re-pass shape, container and font | -55 | refactor | W4.6 |
| `android-ui:picker-row-adts` | picker builds three private ADTs into a row list, then re-dispatches them with `when` | -170 | refactor | W4.6 |
| `android-ui:picker-viewport-machinery` | directory picker scroll restore round-trips through the domain via snapshotFlow capture and an identity-equ... | -120 | refactor | W4.6 |
| `android-ui:dashboard-plumbing` | dashboard callback plumbing, duplicate grid call, triple pull indicator and test-seam residue | -110 | refactor | W4.6 |
| `android-ui:session-edit-machines` | rename and group edits are two parallel state machines; rename's lives in a UI file | -170 | refactor | W4.6 |
| `android-ui:seal-module` | the seal concept is spread over five files with three line-segment types | -60 | refactor | W4.6 |
| `android-ui:viewstrip-bring-into-view` | custom BringIntoViewSpec reproduces the platform default; view-list derivation lives in the UI | -18 | refactor | W4.6 |
| `android-ui:comment-essays` | about 236 comment lines of design essays, history and stale test references | -150 | refactor | W4.6 |
| `android-terminal-notify:selection-handshake` | generation-numbered two-phase selection protocol split across native and page | -200 | refactor | W4.7 |
| `android-terminal-notify:fail-closed-touch` | benign multi-touch anomalies tear down the terminal session | -60 | refactor | W4.7 |
| `android-terminal-notify:webview-main-thread-lock` | main-thread-only WebView carries a lock and off-main branches | -40 | refactor | W4.7 |
| `android-terminal-notify:output-double-queue` | two byte-bounded output queues plus one-frame-in-flight base64 acks for one stream | -60 | refactor | W4.7 |
| `android-terminal-notify:resize-drain` | resize coalescing machinery that does not coalesce | -45 | refactor | W4.7 |
| `android-terminal-notify:ime-containment-thrash` | per-keystroke focus round trip and triple IME geometry containment force repeated layout | -15 | refactor | W4.7 |
| `agent-observation:process-argv-legacy` | Observation.Argv is a legacy field read only by equality, costing a 1 MiB PROCARGS2 sysctl per sample | -63 | refactor | W4.8 |
| `agent-observation:process-coherence-loop` | Observe's two-identical-samples loop duplicates the before/after brackets every caller already performs | -21 | refactor | W4.8 |
| `agent-observation:agentruntime-vestigial-types` | Foreground vs AgentRuntime split, Project copier and ValidateAgentRuntime are registration-era vestiges | -45 | refactor | W4.8 |
| `agent-observation:profiles-validated-thrice` | ValidateProfiles runs at resolve, at manager New/ApplySettings, and on every gateway response | -40 | refactor | W4.8; the hostconfig overlap stays ([issue](issues/member-settings-validation.md)) |
| `agent-observation:launch-json-envelope` | Launch wraps one command string in JSON+base64 with caps and re-encode checks, decoded twice | -35 | refactor | W4.8 |
| `agent-observation:mirrored-enums-and-closed-panics` | private turnNotice/run-state/composer enums mirror public ones, and closed-switch panics guard them | -70 | refactor | W4.8 |
| `agent-observation:statusline-readers-duplicated` | usage and footer readers scan the same statusline rows twice per provider | -55 | refactor | W4.8 |
| `agent-observation:terminal-ops-revalidate` | agentcontrol terminal ops repeat gateway and tmux input validation and nest no-op 10 s timeouts | -45 | refactor | W4.8 |
| `agent-observation:grammar-god-functions` | codexComposer (cyclo 110) and claude.go (1867 lines) are blobs mixing every dimension | -10 | refactor | W4.8 |
| `cross:one-private-file-primitive` | Atomic durable write reimplemented ~9 times and flock in 7 packages | -150 | refactor | W4.9; #151 kept the four flock lease copies |
| `ops-docs:release-pipeline-collapse` | release path is 11 scripts that each re-validate and re-verify the same facts | -787 | refactor | W4.10 |
| `followup:directory-entry-kind` | the phone's domain `DirectoryEntry.kind` lost its only reader in #135; the strict wire decode stays | -15 | deletion | W4.1 |
| `followup:pressure-detail-metric` | `PressureDetailRow.metric` has no reader | -5 | deletion | W4.6; resolved by #152 |
| `followup:sessionui-device-removed` | dead `DeviceRemoved` checks in sessionui (session.go, usage.go) and a stale 'verified roster' comment after #137 | -15 | deletion | W4.4 |
| `followup:startup-composition-rest` | `platform.Current`'s three build-tagged files and single-caller `runtimeenv.WithoutLaunchContext`, left by #137 | -40 | refactor | W2.6 |
| `followup:touch-scroll-protocol-doc` | terminal-touch-scroll.md says page protocol 3; the code sends 5 | 0 | docs | W5; resolved by #159 |
| `followup:gateway-dispatch-predicate` | #141 kept a coarse dispatch predicate beside the route table for `dispatch:not_sent` on unmatched and admission answers; derive it from routed mutations only | -20 | refactor | W4.2 |
| `followup:gateway-no-store-bound` | #141's writeJSON ties the 64 KiB response bound to `Cache-Control: no-store` | -5 | refactor | W4.2, kept by #157: decoupling adds surface and changes no bytes |
| `followup:fleetclient-input-bounds` | fleetclient still duplicates the text/keys/maxBytes bounds agentcontrol owns after #141 | -20 | deletion | W4.4; #155 moved the copy to agentcli ([issue](issues/cli-terminal-input-bounds.md)) |
| `followup:session-handlers-move` | move the session handlers from gateway.go to sessions.go as a pure move | 0 | refactor | W4.2 |
| `followup:id-grammar-copies` | %digits/$digits predicates remain in gateway digitsAfter and terminal validPaneID beside tmux.ValidID after #148 | -15 | refactor | W4.2 |
| `followup:tolerant-consumers` | desktop `ParseServerText` stays strict (#141) and fleetclient's usage-report checks are half a contract (#145); convert with the other same-release consumers | -30 | refactor | W4.1 |
| `followup:config-revalidation` | after #142: default_machine check could live in fleet; profiles are re-validated by agentruntime; sessions.New re-checks the tmux path | -25 | deletion | W4.4, partly by #155; the rest in [default machine](issues/default-machine-case.md) and [member settings](issues/member-settings-validation.md) |
| `ops-docs:render-release-notes-legacy` | release notes duplicate README prose, re-validate inputs and carry a legacy migration pointer | -85 | cut | W4.10 |
| `ops-docs:feature-spec-consolidation` | ~45 feature specs (17k lines) amend each other via preambles; merge into ~7 current-state docs | -14,700 | refactor | W5 |
| `ops-docs:design-deltas-fold` | design deltas D1-D8 are implementation diaries; geometry numbers duplicated between doc and generator | -2,458 | refactor | W5 |
| `ops-docs:grammar-docs-trim` | provider screen grammars restate qualification tables and requalification process beside the code they desc... | -600 | refactor | W5 |
| `ops-docs:core-docs-diary` | architecture.md and roadmap.md are doc-ownership indexes plus changelogs, with stale claims | -1,310 | refactor | W5 |
| `android-ui:ui-design-docs` | 6,124 lines of UI spec and delta docs for about 4k lines of UI code, many citing deleted tests | -5,800 | refactor | W5 |
| `sessions-tmux:docs-slop` | slice docs are 3k+ lines of spec, rationale and qualification logs | -2,900 | refactor | W5 |
| `host-notifications:docs-retired-transport` | Notification docs still specify the retired ntfy/unifiedpush/rfc8291 transport and the superseded client-si... | -900 | refactor | W5 |
| `android-terminal-notify:slice-docs-diaries` | ~5,000 lines of slice docs are evidence diaries, not specs | -4,500 | refactor | W5 |
| `agent-observation:docs-process-narrative` | current contracts carry ~520 lines of finished project-management narrative; grammar docs restate code | -620 | deletion | W5 |
| `host-fleet:docs-historical` | About 2900 lines of historical/superseded fleet and install docs; three issues cite code that no longer exists | -2,700 | deletion | W5 |
| `desktop-clients:docs-palimpsest` | seven overlapping desktop docs (2,442 lines) describe one tui through stacked supersession notices | -2,100 | refactor | W5 |
| `desktop-clients:cut-create-options-form` | PRODUCT CUT: drop the tui `N` full creation form (profile, name, directory search, group) and keep `n`, `T`... | -450 | cut | kept |
| `desktop-clients:cut-usage-selection` | PRODUCT CUT: collapse the usage selector to freshest native else freshest terminal else dash | -220 | cut | kept |
| `sessions-tmux:restore-verification` | post-restore full graph verification turns cosmetic divergence into permanent broken recovery | -175 | cut | kept |
| `sessions-tmux:server-epoch` | @skid_server_epoch is redundant with tmux pid+start_time | -90 | cut | kept |
| `agent-observation:coarse-classifier` | radical option: replace the frozen version-pinned grammars with coarse positive-marker classifiers | -2,700 | cut | kept |
| `android-terminal-notify:runtime-firebase-generalization` | runtime multi-project Firebase options machinery for a one-user app | -200 | cut | kept |
| `ops-docs:release-pin-published-audit` | release-pin.json and the published-release audit pin what immutable signed releases already guarantee | -214 | cut | question |
| `android-core:foreground-reboot` | every onStart resets to Booting and rebuilds the dashboard; three near-duplicate fleet-install paths | -120 | cut | question |
| `android-core:forge-recovery` | two-stage ForgeRecovery state machine for unknown-outcome creates | -110 | cut | question |
| `android-ui:dashboard-process-death-restore` | hand-rolled Bundle codec restores the dashboard viewport by semantic anchor | -200 | cut | question |
| `android-core:saved-state-fingerprints` | SHA-256 fingerprints and dual view hierarchies exist only to keep labels out of the saved-state Bundle | -150 | cut | question |
| `desktop-clients:cut-metadata-reconcile` | PRODUCT CUT: simplify rename/group editors' post-write 'checking' reconciliation | -90 | cut | deferred |
| `sessions-tmux:restore-native-groups-links` | reconstruction complexity driven by native session groups and multiply-linked windows | -120 | cut | deferred |
| `agent-observation:rare-setup-families` | one-time first-run/config dialogs have bespoke grammar rules | -120 | cut | deferred |
| `agent-observation:unknown-reason-taxonomy` | four successful-unknown reasons (partial/layout_unknown/evidence_clipped/evidence_conflict) need per-dimens... | -150 | cut | deferred |
| `agent-observation:claude-idle-truncation-proofs` | claude footer vocabulary with prefix-cut matching and P0-P2 proofs (~330 lines) exists to claim idle on tru... | -150 | cut | deferred |
| `cross:fleet-read-model` | Every client fans out to every host, and every poll re-runs full tmux inventory plus screen classification;... | -300 | cut | deferred |
| `ops-docs:ci-scope-classifier` | ci path classifier and repo-id guard are optimization ceremony for one developer | -45 | cut | deferred |
| `cross:digest-corrections` | Digest overlaps, double counts, dependency errors and misclassifications to fix before scheduling | 0 | refactor | deferred |

## trade-offs taken

- #121 rules: same-release consumers decode tolerantly, so a producer bug renders wrong data instead of crashing; drafts are canonicalized only by the host; `justify-*` grep markers go with the guards they decorate. a carve-out keeping signed-authority checks lasts until w2.1 removes signatures.
- #122 docs: 60 historical docs deleted (git keeps them); unrun acceptance obligations collapsed into [one record](issues/unrun-acceptance.md).
- #123 terminal: `terminal.lock` and its checker are gone; reproducibility is shown on demand by a rebuild and an empty `git diff`, not by a gate. the patch shrank only 565 lines because its lockfile pins are needed for byte-identical output; w3.5 deletes the rest.
- #124 host: `/healthz` removed (no consumer); `handoff/status` and `local/refresh` now 404; wrong-hub errors lose `routing`; composition assertions become first-use nil dereferences; the observer flock is replaced by single-writer composition; int64 exhaustion guards removed.
- #125 android: the debug seal gallery is deleted (restore from git for a seal redesign). press, back and dialog behaviour on device is `NOT_RUN`.
- #126 ops: the catalogue has no pre-release check (the gateway loader rejects bad keys at startup); the ornament generator is deleted, so regeneration is manual; `scripts/build` is folded into `scripts/check`; the release path has not run end to end since, so the next release is its first live run.
- #129 providers: the zsh tty probe is silenced only in interactive shells without a terminal.
- #131 install: launchd runs serve, tmux and agents as `Interactive`; the new installer fails an install whose serve runs below priority 31. `skid update` from v0.20.0 does not apply it; only the v0.20.1 installer does.
- #133 observation: claude screen-reader screens classify `unknown`; `info --explain` reports the classification and bare rule ids.
- #135 android: screen readers get compose's merged visible text only: no curated narration, custom actions, headings or live regions, and talkback can no longer scroll the terminal. rtl names may reorder neighbouring punctuation (paths keep `TextDirection.Ltr`). pixel identity and talkback behaviour on device are `NOT_RUN`.
- #136 release: one linear `scripts/release`; every integrity check runs once, in one `check-release` call before the draft; the key-to-pem check runs in preflight; release notes link the readme instead of repeating it. the next real release is its first live run.
- #137 fleet: a removed device gets 401 and the phone keeps its store; a removed computer drops its cached members. same-fleet phone repair sends the saved bearer to an origin read from an unverified qr, accepted under the tailnet-boundary threat model. join, recover and failed writes restart serve (3-10 s, websockets drop). the machine-handle file and persisted applied settings stay: deleting the first re-keys every host and breaks workspace recovery, and deriving the second in memory makes an invalid host refuse every restart. the qr pairing emulator journey is `NOT_RUN`.
- #138 install: hand-edited skid-owned files are overwritten; uninstall turns off `tailscale serve --https=8443` unconditionally; the linux systemd branch, real launchd and real tailscale are `NOT_RUN`.
- #139 sessions: an absent id with a valid token answers 404 `SessionNotFound` (was 500, or 409 `TerminalUnavailable` for terminal ops); an id on an epochless server answers 409 everywhere; a failed CreateShell source read answers 500 (was 422); a window- or pane-local `@skid_` option shadows the format value undetected. a steady list is 2+2K execs (was 6+2K); the M1-M14 mutation pass was not rerun on the final code.
- #141 gateway: null request members read as absent; a stalled reader closes 5 s after the stall instead of at a 1 MiB backlog, and a slow reader stays connected while tmux paces output; revocation is CloseDevice only; `.`/`..` session ids answer 400 instead of 404; desktop terminal frames still decode strictly until w4.1.
- #142 config: `machines:` is an unknown key (dev-server yaml edit in the cutover); yaml errors carry yaml.v3's wording; a vanished shell keeps serve up with `invalid_host` and creates answer 500 instead of crash-looping serve; a malformed push key now loads and fails at the first send.
- #143 terminal: stock xterm.js 6.0.0 is vendored byte for byte, with no build step. wheel and tap reports are always SGR; page-owned word bounds differ from xterm's at soft-wrap edges; a held word whose row leaves the buffer clears with no Copy, and a lower split pane's scroll region moves neither anchor nor highlight; an attached mouse wheel goes through stock xterm, a second router. `settleFonts` still reaches a private xterm measure, so an upgrade is a file swap plus a check of that reach. the full stock suite ran with `screenReaderMode` off and was not rerun with the shipped setting, on; the physical phone, Gboard dictation, real multi-touch and cjk ime are `NOT_RUN` ([acceptance](issues/stock-xterm-phone-acceptance.md)).
- #144 notifications: the hub holds acknowledgement, so dismissing, clicking or visiting on any device clears the notice everywhere, and a watched session banners. the cutover discards device memory (ready sessions stay quiet until they work and idle again; current requests alert once more per device) and needs a lockstep install ([attention cutover](issues/attention-cutover.md)). a notice removed without a callback re-alerts once at the next process start; clients no longer check the hub's enums, so a skewed hub's unknown delivery is inert on the mac and unavailable on android; with no minute pulse, a failed fetch while the stream stays up waits for the next hint; the observer file has no write cap, so about 2,500 records fail the next start's 1 MiB read. `skid notifications reset` is a usage error. every mac and android device journey is `NOT_RUN`, and the mutation run was not repeated after the finalizer.
- #145 quota: skid owns the codex app-server protocol, so a codex change to it breaks skid directly. releases drop about 146 MB of managed python per host (archives 56.9 to 6.9 MB on darwin, 66.8 to 4.6 MB on linux). a hung daemon holds the route about 5 s; a socket path over 104 bytes (darwin) or 108 (linux) fails; a pre-cutover `skid update` refuses this release, and the installer refuses any archive symlink, so a downgrade goes through the old release's bootstrap. a read against a real codex 0.163+ daemon is `NOT_RUN` (release-cutover step 6).
- #146 logging: every event line starts with `time` (the coordinator kept it so the mac file can place faults); Sessions.Observed is cut, so inventory latency is Request.Completed alone; two overlapping serve processes can push serve.log toward twice its bound; mac serve stderr is still discarded ([issue](issues/macos-serve-stderr-discarded.md)).
- #147 host resources: a listing under concurrent mutation may show an entry's earlier kind; close errors after a successful read are ignored; pressure owns its wire, so a producer bug ships wrong data instead of a 500; a fifo token still hangs a listing ([issue](issues/directory-listing-fifo-hang.md)).
- #148 recovery: failures are built at each call site, so an unlisted phase/cause pair now reaches the client instead of panicking the gateway; the reader admits decodable hand edits and drops the old count and length caps; the catalogue is compiled in (editing it needs a rebuild); terminal context records change format, so a rollback binary cannot read the new ones.
- #151 private files: every private-state write and remove goes through `internal/privatefile`. the recovery write log's `operation` is always `write`; a deleted state directory is recreated at the next save instead of raising a saving-delayed notice; terminal-context register is now durable, about 7 ms more per inbound ssh or mosh shell; crash debris is `.skid-*` everywhere. the four flock lease copies, install's public-file writer and the fcmpush credential read stay hand-rolled; the new-fleet remove path is untested.
- #152 copy: the host renders status, recovery and agent-label copy and sends it as `status`, `recovery.notice` and `agent.label`, so wording ships with host releases and hosts and phone install in lockstep. while a destination's catalogue is unreachable, a remote agent shows its configured label instead of `profile unknown`, and `info --json`'s remote `execution.agent.label` changes from key to label. the phone no longer re-validates host facts, so a host bug renders literally instead of marking the machine stale; the gateway renders while serving, so a sessions bug publishing an invalid status panics the inventory handler (#157 narrowed this to unknown values). the phone keeps its icu group-label mirror until W4.11. delayed recovery, the tui and the phone were not run live.
- #154 android core: decoding is tolerant: a producer bug renders or becomes a lost response, never a crash, and an unknown error code reads as InternalError with the gateway's message and dispatch, which fixes the crash on the hub's 401. verify-before-attach is cut (owner): a gone or replaced session shows the gateway's copy, and a reattach within one poll of a pane change adopts the old pane, so an interrupt answers 409 `TerminalTargetChanged`, not dispatched ([pane retarget](issues/terminal-pane-retarget.md)). reads cancel on background while mutations finish; a notification click's transport failure disables that machine's actions until the next poll; the phone drops its global cap on remote-context passes; the fleet store has no read-back after save. emulator journeys ran base and new, with the stale-pane 409 window their one difference.
- #155 desktop: one route table, typed refs, one observation lane type and per-message update handlers; cli and `--json` output are byte-identical. a gateway reply listing one lifetime twice now fails `handle_ambiguous` instead of resolving; invalid programmatic requests reach the gateway and fail `InvalidRequest`; neither client caps remote-context passes globally; agentcli keeps a verbatim copy of the terminal input bounds ([issue](issues/cli-terminal-input-bounds.md)). a start whose readiness deadline expires during an inspect now reports `readiness_timeout`, a base race the live run found. the fleetclient split is deferred to W4.15.
- #156 observation: a2 and a3 land. the coordinator accepted a steady list of 2 execs (was 2+2K), not 1, because the capture batch's bracket is the lifetime proof that replaced the post-enrichment inspect. a batch overrunning the shared 2 s times out every pending agent pane for that list; one over 8 MiB falls back for every pane with no reuse; a server replaced between inventory and captures answers 500. a3 keeps a cached detection after `send-keys -R` with no later output (list shows idle while inspect sees layout_unknown and send refuses 409), and after a resize away and back with no redraw; keying on the cursor would change a1's frozen row, so the coordinator kept that residual. a list racing `skid apply` answers the pre-apply profiles and labels; process observation is one sample with no retry. the macbook exec-rate measurement at N ≥ 18 is `NOT_RUN` ([exec overhead](issues/tmux-exec-overhead.md)).
- #157 wire: gateway and go clients share `internal/api`, decoded with plain `encoding/json`. the coordinator accepted two rules: a missing key is its zero value (a document without `observedAt` would print year 1; no same-release gateway sends one), and dispatch comes from the matched route, so error answers to unrouted requests lose `"dispatch":"not_sent"` (12 http cases, 1 websocket). the gateway no longer re-checks producer invariants, so a producer bug renders wrong data instead of a 500. a fleet 401 on start, shell or group reads `Unauthenticated` (`not_sent`), not `protocol_error`. peer `--json` echoes `savedAt` and `failure.at` in the host's offset; `api` imports sessions and quota until W4.14. android, the phone and macos notifications are `NOT_RUN`; the tui create form, group editor and attach were not run live.
- #158 android ui: one sheet scaffold, theme-owned defaults, a plain picker and one `SessionEdit`; the pixel-preserving drift is kept ([sheet uniformity](issues/sheet-uniformity.md)). by the coordinator's classification, a rename answered with another route's error code reads `checking` with the gateway's message instead of an error, which resolves rename-route-foreign-error. the picker restores its exact first row and offset; the grid no longer remounts when some machine turns Ready, so an open card menu and the pull distance persist, and a release after the last Ready machine left no longer refreshes; the theme fills M3's surfaceContainer slots, recolouring future components at a distance. phone Back semantics on android 16 are `NOT_RUN`.
- #159 terminal page: the page owns selection, and touch anomalies block the stream until every finger lifts instead of failing the page. one 1 MiB output budget counts bytes xterm has not written (coordinator): bursts xterm keeps up with survive where base always failed, and a main-thread stall backlogs frames without bound until ANR. the page requires `WEB_MESSAGE_ARRAY_BUFFER`, with no base64 fallback; page-sent input no longer refocuses the textarea, so DECSET 1004 apps lose that ESC[I; every Resize after open is sent uncoalesced; a delayed ClearSelection can clear a newer selection's highlight. the single rAF ime pass and everything on the physical S22+ are `NOT_RUN` ([acceptance](issues/stock-xterm-phone-acceptance.md)).

## cross-repo cutover

every obligation, with operator steps, lives in [release cutover](issues/release-cutover.md). nothing is deployed.
