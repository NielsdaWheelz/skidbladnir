# workspace checkpoint reliability

status: released as v0.16.0 and installed on all three hosts and android,
2026-10-15.
this document owns the replacement failure policy, diagnostics, client contract
and [qualification](#qualification). [workspace recovery](session-recovery.md)
retains graph capture/reconstruction, file schema, ownership and deployment.
[installed qualification](#installed-qualification) records the coordinated
rollout; actual host reboots remain separate.

## goal and capability contract

an unsuccessful ordinary checkpoint must not permanently disable safe future
saves. retain the last confirmed complete recipe, disclose the gap, and let the
existing owner make another bounded attempt. restoration remains conservative:
never infer server death from an unavailable observation or repeat possible
reconstruction effects. live inventory and confirmed terminal mutations retain
their existing authority regardless of saving or diagnostic failure.

owner decisions: restore the last confirmed recipe regardless of age. after an
uncertain ordinary replacement and proven source death, durably publish that
exact confirmed recipe with `attempted=true` before reconstruction. successful
claim publication resolves earlier file uncertainty; claim failure stops recovery.

keep one private schema-1 file, one writer lock, existing manager synchronization,
the 30-second owner, immediate mutation checkpoints and existing operation
budgets. no new controls, route, timer, service, durable failure state, journal,
checkpoint history, migration, provider behavior, restoration retry or generic
persistence/retry abstraction. no capture-performance rewrite in this correction.

## state and ordering

`recipe` means the admitted startup checkpoint or the last successfully published
checkpoint in this owner. an unsuccessful ordinary write never replaces that
in-memory value or advances its `savedAt`, even when rename already succeeded.
`savedAt` remains the capture time of that confirmed recipe, not the failure
time or exact instant its final sync completed.

| observation or result | required transition/effect |
| --- | --- |
| initial lock/read/admission failure; loaded `attempted=true` | `broken`; preserve storage; no recovery effects |
| no recipe and no live server | create nothing; retain current tracking/delayed fact |
| no recipe and a live server | attempt a complete baseline; failed save gives `delayed` without `savedAt` |
| exact saved source survives | capture its complete current graph, including a genuinely empty graph |
| ordinary checkpoint deadline, anywhere in its observation/identity work | `delayed`; deadline is not evidence of changed identity |
| named capture race, native capture unavailability, supported-limit rejection or unsupported live value | `delayed`; publish nothing; preserve the confirmed recipe |
| ordinary publication failure, before or after rename | `delayed`; preserve confirmed recipe; repeat full source checks before another attempt |
| malformed native protocol, contradiction to a pinned live source, missing socket with known-live source, non-timeout boot/process/source observation failure | `broken`; retain existing conservative identity/admission policy |
| source proven dead; confirmed recipe empty | reconstruct nothing; baseline a new live server when present |
| source proven dead; confirmed recipe nonempty | regardless of age, use existing target/preflight checks and durably claim the exact confirmed recipe; never select an unconfirmed candidate from the pathname |
| populated replacement, failed restoration preflight or claim | `broken`; no automatic repair or retry |
| durable claim succeeds | set in-memory `attempted=true` before native effects; retain guarded construction and full verification |
| any subsequent restoration failure, including final capture/publication | `broken`; never replay construction |
| complete ordinary publication succeeds | `tracking`, new `savedAt`, no failure |
| verified restoration and final publication succeed | `recovered`, new `savedAt`, no failure; next ordinary success becomes `tracking` |
| gateway cancellation | stop/join before releasing writer lock; cancellation itself creates no failure fact/event |

only `tracking`, `delayed` and `recovered` continue the existing loop. delayed
mutation checkpoints may also attempt saving; broken ones may not. no inner
retry loop. the next tick reruns the full original-source decision; delayed
never authorizes saving a populated replacement over a nonempty old recipe.

every ordinary publication uses the same admission: no recipe permits baselining;
an unattempted recipe permits the same complete source lifetime; an empty recipe
permits a different source only after its original source is proven dead.
a mutation's expected server identifies its target, never permission to replace
the recipe's source. failed admission preserves the recipe and confirmed mutation
result. mutation checkpointing never initiates restoration. centralize this rule
at the existing ordinary-save boundary in `recovery.go`.

keep each mutation's original deadline. attempt its immediate checkpoint within
the remaining budget and preserve its confirmed result. a missed save waits for
the existing owner. context budgets bound context-aware work; synchronous native
file sync is not preemptible by a go context. add no detached filesystem worker.

without a confirmed first recipe, source loss provides nothing acknowledged to
restore. after gateway restart, admit whichever complete valid file survived
atomic replacement through the existing reader; `attempted=true` still blocks.
do not reconstruct the previous process's acknowledgement history.

## failure boundary and strict api

keep the existing authenticated `GET /v1/sessions` envelope and replace only its
required `recovery` member. `?` means omitted when absent; no json null.

```text
recovery =
  {state:"tracking", savedAt?:instant}
  | {state:"recovered", savedAt:instant}
  | {state:"delayed", savedAt?:instant, failure:failure}
  | {state:"broken", savedAt?:instant, failure:failure}

failure = {at:instant, phase:phase, cause:cause}
instant = existing canonical utc rfc3339nano
```

`failure.at` is the latest failed attempt's observation time. do not require
timestamp ordering: wall clocks can move. tracking/recovered forbid failure;
delayed/broken require it. both clients reject unknown/duplicate fields, null,
omitted required values and invalid variants/pairs. `Peer.MarshalJSON` preserves
the same fact in cli json. state alone determines whether saving will retry.

| phase | allowed causes |
| --- | --- |
| `ownership` | `busy`, `permission`, `no_space`, `io` |
| `checkpoint_read` | `permission`, `io`, `invalid`, `limit`, `already_attempted` |
| `source` | `timeout`, `changed`, `unavailable`, `invalid`, `permission`, `io` |
| `capture` | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `io` |
| `checkpoint_write` | `limit`, `permission`, `no_space`, `io` |
| `restore` | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `no_space`, `io`, `not_empty` |

client admission: delayed permits source/timeout, every listed capture pair
and every listed checkpoint_write pair. broken permits every listed pair.
private typed evidence selects the state: malformed protocol is broken;
unsupported live values are delayed. boot/process/identity checks belong to
source even inside capture. restoration preflight/execution/native verification
use restore; final recapture retains source/capture labels but always breaks
recovery on failure. claim/final storage failures use checkpoint_write/broken.

classification belongs where evidence exists:

- timeout: exhausted budget before context-aware work, or such work terminated
  by its deadline; inspect the context because a killed subprocess loses that
  cause. owner cancellation is separate. preserve positive identity/admission
  evidence and actual filesystem errors even if the deadline also expired.
- changed: positively detected workspace/server race; limit: named existing
  byte/count/field bound; invalid: rejected external data/configuration.
- permission: actual `EACCES`/`EPERM`; no_space: `ENOSPC`/`EDQUOT`; busy:
  nonblocking writer-lock contention; io: other actual filesystem/kernel errors,
  including `io.ErrShortWrite` without an errno.
- unavailable: failed native observation or unavailable required cwd/executable,
  without inventing absence. already_attempted/not_empty use existing refusals.
  `ErrProcessNotPermitted` means unavailable: it also covers an unlinked executable
  and does not preserve an errno.
- unexpected internal failures and impossible constructed facts remain defects
  under [errors](rules.md#errors-and-panics), never synthetic retryable observations.

preserve narrow private distinctions at their detectors: `topology_changed`,
`source_changed`, `unsupported`, `protocol_invalid`, `limit`. map topology churn
to capture/changed/delayed and lifetime contradiction to source/changed/broken;
map unsupported live values to capture/invalid/delayed, malformed output to
invalid/broken. preserve subprocess causes; never classify by error text.
handle native execution failure or proven session disappearance before transcript
admission: deletion can prevent completion. after successful execution, differing
complete membership is topology_changed; missing completion, duplicates and
malformed structure are protocol_invalid. remove the manager's cause-erasing wrapper.

in the checkpoint store, retain the failing operation and wrapped error. preserve
the primary error if cleanup also fails. keep the current temporary-file → sync
→ close → rename → parent-sync sequence and success boundary. impossible json
encoding of the admitted fixed schema is a defect; size overflow is limit.

## diagnostics and composition

`sessions` owns immutable `RecoveryChange`: `{at:instant, recovery:recovery,
operation?:closed-label, errno?:positive-native-os-integer}`. absent optional
values are omitted. supply `func(RecoveryChange)` through
`StartRecovery(ctx, observe) func() error`; the gateway writes it to the process's
event logger; no shared package, channel, event bus or constructor configuration.

invoke after updating the fact, under the existing mutation lock, to preserve
event order. callback may only format/write the event; no manager reentry or
product effects. a failed log write is dropped.
logging failure cannot change recovery, mutation success or retry policy.

emit `Workspace.RecoveryChanged` with existing `event.name`,
plus `skidbladnir.recovery.at`, `.state`, optional `.saved_at`, `.failure_at`,
`.phase`, `.cause`, `.operation`, `.errno`. serialize instants canonically.
failure fields exist only for delayed/broken. errno exists only when extracted
from an actual syscall error. operation is a source-authored constant naming
the failed boundary, never `err.Error()`, command arguments or observed data.
operation literals: `mkdir`, `chmod`, `open`, `lock`, `stat`, `read`, `close`,
`create`, `write`, `file_sync`, `rename`, `directory_open`, `directory_sync`,
`directory_close`, `cleanup`, `boot_identity`, `process_observe`, `server_inspect`,
`server_identity`, `normalize`, `capture`, `validate`, `restore_preflight`,
`restore_execute`. no invented errno or elapsed duration.

emit on state or diagnostic-signature change, including resumed saving and
successful reconstruction. signature is state/phase/cause/operation/errno;
exclude timestamps and savedAt. repeated identical failures update failure.at
without logging every tick; unchanged tracking→tracking saves are silent. events are
diagnostics, never receipts or a history store. no paths, names, terminal bytes,
prompts, provider/account data or raw error text in logs or wire.

## authored content

the content designer owns these exact templates and the acceptance fixtures
below. good content identifies the host, failed capability, safe cause, coverage
and next action without implying that editor files or conversations were lost.
use existing desktop notice wrapping/tones and android machine notices.

| state | text / tone |
| --- | --- |
| tracking | no notice, including after saving resumes |
| delayed | `<host>: workspace saving delayed. <failure sentence> <last-save>. retrying automatically; live terminals remain usable.` / degraded |
| broken | `<host>: workspace recovery stopped. <failure sentence> <last-save>. manual repair required; live terminals remain usable.` / failure |
| recovered | existing `<host>: terminals restored as shells. resume conversations using the same provider account.` / degraded |

failure sentence is `<phase label>: <cause phrase>.` the host supplies facts;
clients never infer retryability. exact vocabulary:

| phase | label |
| --- | --- |
| ownership | writer lock |
| checkpoint_read | checkpoint read |
| source | tmux server check |
| capture | workspace capture |
| checkpoint_write | checkpoint save |
| restore | workspace restore |

| cause | phrase |
| --- | --- |
| timeout | timed out |
| changed | workspace or server changed |
| unavailable | required resource unavailable |
| invalid | invalid data or configuration |
| limit | supported limit exceeded |
| busy | lock already held |
| permission | access denied |
| no_space | storage space exhausted |
| io | input/output error |
| already_attempted | prior restore may be incomplete |
| not_empty | replacement server has sessions |

last-save is `last confirmed save: <time>` or `no confirmed save`. human time is
client-local 24-hour date/time with seconds and numeric offset, for example
`2026-10-14 23:55:13 utc-07:00`, without fractions; json remains
exact utc. use small formatters in the existing content owners; strict timestamp
admission remains shared. no relative age, countdown, retry button, success toast,
new timer or dismissal persistence.

retain fresh-inventory-only claims, visibility outside filters including an empty
needs-input view, pressure/action notice coexistence, selection/scroll preservation
and notice-start exclusion from pull-to-refresh. no visual redesign.

## exclusive implementation slices

root freezes this contract before builders start. each feature has a content
designer who authors inputs, expected state/graph, exact notice or silence, and
one adverse case before its builder writes the temporary red test. a separate
adversary reviews every contract, red failure, green result and refactor. neither
designer nor adversary edits production; final verifier writes no tests or code.

| exclusive builder | owned production files and responsibility |
| --- | --- |
| native capture | `internal/tmux/recovery.go`, identity/character receipt methods only in `internal/tmux/client.go`, naming receipt only in `internal/tmux/naming.go`: typed capture/source failures and native-layout comparison; retain reconstruction mechanics |
| host policy/store | `internal/sessions/{recovery,recovery_store}.go`, identity/normalization admission only in `internal/sessions/manager.go`, malformed-receipt propagation only in `internal/sessions/naming.go`: state, classification, callback, file errors; no mutation-call-site rewrite |
| composition/wire/logging | `internal/gateway/{server,dto}.go`, `internal/logging/logger.go`: callback, serialization, event validation |
| desktop/cli content | `internal/fleetclient/{response,content}.go`: strict union, shared copy, human time |
| phone content | `android/app/src/main/java/dev/niels/skidbladnir/ProductModel.kt`: strict union, copy, human time |
| root integrator | this spec, `docs/{session-recovery,architecture,roadmap,codebase-map}.md`, incident record, temporary cross-boundary driver |

temporary tests belong only to their builder's package; root owns disposable
`scripts/tmp-workspace-checkpoint-reliability`. widening a slice requires root
reassignment before editing. native error identities precede host policy;
host event/wire contracts precede composition. client work may proceed in
parallel once the schema and content are frozen.

reuse `Recovery.Valid`, `strictjson`, canonical instants, native lifetime/graph
guards, current file writer, gateway logging and both notice owners. the similar
`attention.WriteFile` serves different ownership/error contracts; do not make
recovery depend on attention or extract a generic writer for this change.
remove old recovery reason enums/fields/rendering, cause-erasing wrappers and
the blanket ordinary-save permanent-stop branch. retain broken handling for genuine
blocked recovery. no compatibility decoder, old-message fallback or dual path.
stored schema 1 is unchanged, so there is no file migration.
the go decoder owns strict wire shape/canonical instants and keeps one private
`sessions.Recovery`; semantic admission delegates to its existing `Valid` contract.
android owns the equivalent platform projection. no second go phase/cause table.

## red green refactor acceptance

| feature / designer-owned content case | required boundary and adverse evidence |
| --- | --- |
| safe saving resumes | real gateway and isolated native tmux on darwin/linux: force both addition and deletion between discovery/capture; old code fails to resume saving; new code exposes delayed/capture/changed or proven unavailable, retains old file/time, then saves the stable graph on the existing tick without a client |
| mutation admission/deadline | confirmed structural mutation near its original deadline; delayed with timeout at the actual failed phase, unchanged confirmed outcome, no extended deadline; next tick saves. cover identity and capture expiry. a mutation on a replacement before the next tick cannot overwrite a nonempty recipe, or an empty recipe whose original source still lives |
| publication and confirmed recipe | real private filesystem: denied write and post-rename directory-sync failure never advance savedAt; later success clears notice. after post-rename uncertainty, prove source death and exactly one restoration of the older confirmed recipe after a successful durable claim; failed claim starts no reconstruction |
| blocked safety and first baseline | lock conflict, invalid/attempted file, live-source socket loss, populated replacement and partial/final-save restoration failure stay broken with no replay/overwrite. first save failure has no savedAt; no first confirmed recipe means no acknowledged reconstruction. owner cancellation during a save produces no cancellation failure; join work before releasing its writer lock |
| diagnosis and strict cutover | real gateway serialization plus both client decoders: valid variants/pairs pass; old reason field, unknown/null/duplicate/missing fields and invalid pairs fail. repeated same failures emit one event; changed diagnosis and resumed saving emit one each; log failure changes no product outcome; injected private content never reaches logs/notices |
| notice usability | real 80×24 desktop tty: all three host notices plus action outcome, quota, selected-session details and keys remain readable. physical phone at ordinary/enlarged type: scroll through all three notices; notice-start excludes pull-to-refresh. both: exact copy/silence, unknown save time, local offset, empty filters, stale suppression and preserved selection/actions/pressure |

designer's desktop fixture: all machines; selected local shell `scratch` on
macbook; macbook delayed/capture/timeout, devbox broken/checkpoint_read/already_attempted,
arch broken/restore/not_empty; each uses the example save time above. include the
existing uncertain-interruption/deletion outcome, all-profile quota summary,
target, shell/cwd detail and both key rows. static budget: 18 footer + 2 header
rows leaves 4 body rows. also check three broken notices, unknown times and an
additional notification-failure line. shorten copy before any layout change;
reassign `sessionui/view.go` only if actual rendering proves it necessary.

use dummy content and test-owned `-L` sockets/checkpoint paths only. inject
precise syscall failures only in disposable test instrumentation; exercise the
real filesystem/native path around the fault and record which boundary was
injected. no production fault hooks, fake end-to-end passes or flaky timing-only
claims. a nondeterministic race without evidence of the intended interleaving
does not qualify its row.

sequence: designer cases → adversarial contract review → temporary failing
acceptance against original source → review failure sensitivity → implement →
green at named boundaries → adversarial refactor to repository rules → rerun
affected checks → `scripts/check verify` → delete temporary tests/instrumentation
→ final build/link/diff review. root records source, actual boundary, result and
limitations here. no passing build substitutes for behavior. tmux/live/device
execution requires current-turn approval; missing boundary is `NOT_RUN`.

coordinated hard cut: host, go clients and android ship the required new union
together through the existing release process. mixed versions are unsupported;
no deployment/service/config redesign. implementation, qualification and fleet
deployment remain separate. update/delete the incident only after its required
behavior is qualified; [actual reboot qualification](issues/unrun-acceptance.md)
remains separate.

## explicit costs

- the 30-second sampling gap and failed-save intervals remain; old recovery can
  omit recent work or resurrect closed sessions as fresh shells. full-graph
  admission lets one unsupported value prevent saving the whole workspace.
- re-claiming the confirmed recipe can replace a newer unacknowledged candidate.
  startup may later admit a surviving candidate; no cross-crash acknowledgement
  history is promised.
- non-timeout source uncertainty and restoration preflight/claim failures retain
  conservative manual repair, even where another read or pre-effect attempt
  could be safe. expanding restoration recovery is outside this correction.
- the richer wire union requires coordinated rollout. absolute local times trade
  relative convenience for unambiguous instants without clock arithmetic/timers.
- terse diagnostic labels spend less collection space; the last confirmed save
  and stopped/delayed capability replace the redundant unsaved-changes sentence.
- transition-only synchronous logging assumes the existing service output is
  drained; it can delay work if its writer blocks. no logging isolation service.
- native json layout comparison adds a small format adapter because unmapped
  pane ids made real tmux 3.8 restoration fail verification. it maps only ids and
  retains geometry, ordering, flags and opaque fields; tmux keeps construction.
  no cross-version text-to-json checkpoint conversion is added or qualified.
- temporary tests are deleted as requested. the change gets qualification, not
  retained regression protection; [that gap](issues/test-system-reset.md) remains.

design review: independent persistence, composition/diagnostics and content
reviews corrected private error distinctions, deadline attribution, source
admission across save paths and crowded-notice copy. implementation reviews also
closed cause-erasing normalization receipts, malformed/overflowing source pids
and real tmux 3.8 layout verification. the native adapter follows
[tmux's format owner](https://raw.githubusercontent.com/tmux/tmux/3.8/layout-custom.c);
it preserves the original `select-layout` argument and adds no geometry engine,
file schema, native nesting limit or restoration retry.

## qualification

2026-10-15 source: `eb6cdac` on `codex/workspace-checkpoint-reliability`, based on `1101d8a`
(including the terminal-footer/compact-row and viewport fixes). all native effects
used test-owned sockets and private files. darwin: go 1.26.5, tmux 3.8; actual
linux host: go 1.27.1, tmux 3.8. no linux version was inferred from old evidence.
native/host/client qualification ran before the final desktop-only rebase, on
`6c70ee0` plus the byte-identical checkpoint implementation. the only upstream
production change was `sessionui/layout.go`; focused notice/selection/backfill and
real tty qualification was repeated after that merge. native/storage/client
boundaries were not rerun without a changed input.

| acceptance | actual boundary and result |
| --- | --- |
| safe saving resumes | pass on both hosts: real authenticated gateway/http and native tmux. deterministic addition/deletion between discovery/capture retains confirmed file/time and becomes delayed; stable full graphs save on the existing 30-second tick with no client. real denied writes and changed failures log once per signature, then resumption clears failure |
| mutation/source admission | pass on both hosts: real confirmed group mutation with identity/capture deadline exhaustion preserves its outcome and original budget; later owner saves. replacement mutations cannot overwrite nonempty or still-live empty recipes; proven death permits a new baseline for an empty recipe without reconstruction. source lifetime checks remain common to all ordinary publication |
| publication/claim | pass on both hosts: real rename plus disposable directory-sync `EIO` injection retains acknowledged memory/time; proven source death claims the exact 400-day-old confirmed recipe before native effects and restores real bash shells once. failed claim, partial native construction, final recapture/publication failure remain blocked without replay. restart admits the surviving complete candidate; first unconfirmed baseline is never reconstructed |
| blocked safety/cancellation | pass on both hosts: real writer contention, invalid/claimed/over-limit file, live-source socket loss, populated replacement, native malformed receipts and unrepresentable pids fail closed. unavailable saved cwd stops before claim/effects. cancellation joins in-flight work and releases ownership without a cancellation failure |
| strict diagnostics/clients | pass: actual gateway envelopes enter the go client and physical android decoder; both platform matrices cover all 264 state/phase/cause/save-presence combinations, strict negative shapes, canonical instants and exact copy in three offsets. real log-writer failure changes no confirmed mutation or saving policy. diagnostic assertions exclude private paths/content. combined footer/recovery fixtures pass after rebase |
| desktop notices | pass: real 80×24 pty with the production browser model/renderer and authored facts, decoded by the shipped terminal engine's xterm 6 major. all three complete notices coexist with action outcome, quota, target details and keys; empty needs-input keeps notices and selection. static cases cover unknown times, three broken notices, notification coexistence, exact copy and stale silence. after the viewport merge, real 80→135→80 resizing retains notices/selection; a static transition clearing notices near the tail backfills available rows |
| phone notices | pass: physical s22/android 36, isolated probe package using production decoder/dashboard. ordinary and 2× font, all three scrollable notices, unknown/recovered/tracking/stale cases, empty needs-input, preserved card anchor/actions and notice-start refresh exclusion. an identical collection-start gesture proves refresh works. original full gateway envelope decoded without stripping or ICU substitutes |
| engineering/review | pass: `scripts/check verify` with all platforms, ordinary android debug assembly, then `scripts/check verify host` after the desktop-only merge; independent native/host/client/composition/content reviews and final diff/link review. temporary behavioral tests, overlay, probes and remote copied sources are removed before commit |

red evidence against `16a6ed9`: ordinary deadlines, native churn and write
failures latch broken; saving never resumes; source admission/cancellation and
typed wire/copy expectations fail. new layout evidence reproduced the false
restore-verification refusal before the format correction. green runs exercise
real native/filesystem work around injected boundaries; subprocess interposers
and the directory-sync overlay are disposable, with no production fault seam.

limits of this isolated qualification: physical power loss, full-disk exhaustion,
reboot and release/deployment were not exercised by those probes. the desktop
driver uses authored admitted inventory; it is not a full authenticated fleet
journey. phone acceptance uses
the exact gateway envelope plus an isolated dashboard fixture, not phone↔gateway
authentication/transport. merged Android view/gesture code is byte-identical to
the physically qualified source; post-rebase decoder fixtures verify composition.
no qualification transfers to a different native version or cross-format file.

the source defect behind permanent saving cessation is resolved. the original
installed incident's exact triggering error was discarded and remains unknown;
current free space, permissions and a live saved source never proved its cause.

## installed qualification

2026-10-15: immutable [v0.16.0](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.16.0),
source `d8c3c5574569ddf6d4c43747f72961dc14be0eb1`, passes published-release
verification for all five assets, signatures and exact source.
[upstream pin pr 79](https://github.com/NielsdaWheelz/skidbladnir/pull/79) and
[dev-server pin pr 171](https://github.com/NielsdaWheelz/dev-server/pull/171) are merged.
managed gateway-only applies and final `scripts/fleet verify` pass on macbook,
devbox and arch; every installed gateway reports that exact version/source.

authenticated before/after observations preserve every pre-upgrade session
authority and native pane/process membership. each server, checkpoint-source,
bearer, machine-handle and private fleet-client hash is independently unchanged.
additive work on devbox is retained. two post-upgrade observations
show `tracking` without failure, unchanged schema 1, mode `0600`,
`attempted=false`, and advancing confirmed checkpoints on the existing owner:

| host | sessions before → after | confirmed savedAt, 2026-10-15 utc |
| --- | --- | --- |
| macbook | 11 → 11 | `18:38:01.122126Z` → `18:38:31.198088Z` |
| devbox | 3 → 4 | `18:37:53.482579576Z` → `18:38:53.568202312Z` |
| arch | 0 → 0 | `18:38:20.227217866Z` → `18:38:50.221587331Z` |

the macbook's checkpoint, formerly stuck at `2026-10-15T06:55:13.972475Z`, now
advances against its surviving source. its stale 17-session recipe is replaced
with the 11-session live graph preserved through deployment. these observations
prove healthy confirmed advancement, not the absence of every intervening failure.
deployment used no production tmux controls, fault injection or provider binary
upgrades. jarvis remains disabled, inactive and paused; its root cli matches the
new gateway.

android `0.16.0` / `16000` is installed in place on the selected physical phone.
the installed apk matches the exact published bytes and pinned signing
certificate; first-install time and both user-0 data-directory inodes are
unchanged. the stock installer refused multiple devices; installation used the
known serial and user 0 after signer admission. ordinary production paired-fleet
reads pass: dashboard → native machines sheet → dashboard shows all three paired
hosts with fresh inventory and no availability notices. hierarchy inspection
streamed through memory without persisted ui files. this journey did not exercise
terminal attachment/actions, pairing/reset, notifications or production failures.

temporary rollout probes, downloads and copied apks were removed.

actual reboot, physical power loss, full-disk exhaustion and cloud inference
were not performed. [actual reboot](issues/unrun-acceptance.md) and
[retained regression coverage](issues/test-system-reset.md) remain open;
this rollout does not close unrelated feature acceptance.
