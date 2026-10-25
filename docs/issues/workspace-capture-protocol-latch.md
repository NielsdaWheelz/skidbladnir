# workspace checkpoint recovery repair

## problem and scope

target implementation plan; no implementation or installed repair is authorized
by this document. failed ordinary checkpoints must preserve the confirmed recipe
and permit later saves. restoration must retain its no-replay barrier.

macbook v0.22.0, `fd704c14b40b9ead42dd171bd7845b3e95a4898b`: the last save was
`2026-10-25T20:34:53.986181Z` with 22 sessions. at
`2026-10-25T13:35:24.317185-07:00`, recovery logged
`broken/capture/invalid/normalize`; subsequent saves stopped, including after a
23rd session appeared. later native inventory probes passed on the same server.
the initiating response remains unknown: normalization includes reads and writes.

repair two features: checkpoint progress and attributable protocol diagnostics.
owner decisions: restore preflight failures remain manual; checkpoints stop
automatic naming; verification is temporary red/green/refactor, then deletion.

## capability contract

- tmux owns live sessions; providers own execution/history; sessions owns the
  reconstruction recipe and failure policy; tmux adapters own native evidence.
- retain one manager gate, recovery owner, 30 s loop, writer lock and recipe.
  ordinary attempts retain the 10 s budget, discovery its 2 s sub-budget, and
  restoration its 30 s deadline. no adapter retries or additional scheduler.
- publish only an admitted complete graph from one admitted source. preserve
  source, graph, process, settings and recipe-anchor checks around discovery.
- failure never advances confirmed `savedAt` or promotes a candidate in memory,
  including a failed write whose rename may have landed. preserve atomic durable
  publication through `privatefile.WriteFile` and the 8 MiB limits.
- a failed observation never proves server death. reconstruction requires
  positive boot/process loss evidence, an absent/empty destination, and a durable
  `attempted=true` claim before effects. never merge, replay or roll back.
- a confirmed user mutation retains its successful outcome when checkpointing
  fails. cancellation publishes no failure; gate-reentry expiry abandons work
  without an unlocked status write. existing clients continue using live inventory.

## policy and composition

choose disposition at lifecycle callsites. `tmuxFailure` translates evidence;
it must not decide whether future work is allowed. replace `failRecovery`'s
`latch bool` with an explicit `api.RecoveryState` argument, limited by its callers
to `delayed` or `broken`. remove `recoveryFailure.malformed` and its policy branch.

| evidence and lifecycle | target disposition |
| --- | --- |
| ordinary malformed/unavailable observation, unsupported live value, timeout, inconclusive boot/process/socket read, or publication failure | `delayed`; retain recipe; fresh attempt on the existing tick |
| ordinary preparation metadata write may have executed | `delayed`; end this pass; read current state before deriving any later conditional correction; never resend the old invocation |
| source disappears/changes during ordinary capture | `delayed`; next tick repeats source-loss proof and destination checks |
| saved source is positively alive but another server is observed; ordinary saving would replace a nonempty recipe from another source | `broken`; preserve recipe; no preparation, overwrite or reconstruction |
| old empty source is positively dead | allow a current admitted live server to establish a baseline |
| startup lock/read/admission failure, loaded attempted recipe, unsafe destination, or failed restore preflight/claim | `broken`; manual repair |
| any failure after durable restore claim, including final observation/publication | `broken`; no further reconstruction by this owner; retain the confirmed claim in memory if final publication fails; restart admits the actual file under existing source fences |
| complete durable ordinary publication | `tracking`; new `savedAt`; clear failure/diagnostics/notice |
| verified reconstruction and durable final publication | existing `recovered` behavior; counts mean startup dispatch only |

ordinary flow, under the existing gate except discovery:

1. pin recipe/settings; admit the source against the confirmed recipe BEFORE
   any preparation effect. repeat the same checks for mutation-triggered saves.
2. `prepareCheckpoint(ctx, server)` returns admitted server identity and session
   ids using existing inventory, guarded epoch admission and `normalizeCharacters`.
   this is the only checkpoint stage permitted to repair required metadata.
3. `captureCheckpoint(ctx, server, ids)` reads graph/process/boot evidence and
   validates it without mutations. no epoch writes, character writes or naming.
4. observe agents; release the gate for existing provider discovery; reenter,
   reject changed owner/anchor/settings, capture afresh without preparation, and
   retain existing graph/process/profile brackets before publication.

inventory still owns `normalizeNames`; checkpoints preserve actual native names.
native sessions without clients still receive required character preparation.
restoration already admits/stamps its destination and restores characters; obtain
its final ids by native observation and use the same read-only capture contract.
do not change provider discovery, startup composition or resume selection.

## interfaces and diagnostic schema

no new endpoint, configuration, checkpoint field, wire enum or client branch.
retain schema-2 `workspace.json`, `attempted`, `api.Recovery`, and
`RecoveryNotice {summary, action?}`. retain existing error classifications for
wire copy; internal evidence becomes more precise.

add one `tmux.ProtocolError {Record, Field, Reason}` with content-free `Error()`
and `Unwrap()` to `WorkspaceProtocolInvalid`:

| field | closed contract |
| --- | --- |
| `Record` | `batch_begin`, `batch_end`, `inventory_row`, `session_metadata`, `workspace_session`, `workspace_window`, `workspace_pane`, `workspace_graph`, `server_preflight`, `guard_completion` |
| `Field` | 1-based ordinal in that record; 0 means no particular field |
| `Reason` | `framing`, `invalid_value`, `duplicate`, `inconsistent`, `unexpected_marker` |

instrument inventory/batch/metadata/graph parsers and guarded completion; adapt
the shared quoted-field decoder's eight-field preflight caller as `server_preflight`
without changing validation. attribute cross-record protocol rejection from
`ValidateWorkspace` to `workspace_graph`, field 0, reason `inconsistent`; preserve
its unsupported-value classification. leave unrelated restore/layout validation
unchanged. preserve joined source/protocol/dispatch facts; inspect
independent sentinels with `errors.Is`, never let first-match `errors.As` erase
another fact. positive source evidence retains precedence over provisional
malformed output; source observation failure supplies no positive loss evidence.

carry `Protocol *tmux.ProtocolError` and `DispatchUnknown bool` through internal
`recoveryFailure` and exported `sessions.RecoveryChange`, never `api.Recovery`.
reuse `Workspace.RecoveryChanged`; replace aggregate `normalize` operations with
`inventory_read`, `character_assign`, or `graph_capture` at their
actual callsites. retain existing operation literals for other phases.

add optional event attributes `skidbladnir.recovery.protocol_record`,
`.protocol_field` (only when >0), `.protocol_reason`, and `.dispatch_unknown`
(only when true). include the triple and dispatch flag in transition
deduplication. update `failure.at` on repeated identical failures without logging;
omit absent evidence and clear it on later outcomes. absent dispatch evidence
never means not sent. retain only real syscall errno. no raw errors, responses,
hashes, names, titles, paths, commands, credentials or account data.

## content design

the content designer owns both feature contracts; no visual asset is needed.

| feature | good content and final decision |
| --- | --- |
| checkpoint progress | existing host-rendered delayed copy promises automatic retry only when it will occur; broken copy requires manual repair only when work is fenced. preserve exact strings, phase/cause labels, client machine/local-time/tone composition, and resume-request wording |
| protocol diagnostics | closed record/field/reason identifies the failed boundary without private values; independent dispatch evidence preserves uncertainty. detailed evidence stays in logs, outside public notice copy |

copy acceptance for `capture/invalid`:

- summary: `workspace saving delayed. workspace capture: invalid data or configuration`
- action: `retrying automatically; live terminals remain usable.`

successful inventory alone never clears degradation; successful checkpoint
publication does. leave `internal/api/recovery.go` and client renderers unchanged.

## work boundaries and files

each owner edits only its slice; dependencies are contracts above, not shared
files. independent adversarial reviewers write no production or test files.

| owner | exclusive paths and responsibility |
| --- | --- |
| adapter | `internal/tmux/{protocol,inventory,guard,recovery}.go`; one protocol type, native evidence and sentinel preservation; no recovery policy |
| sessions | `internal/sessions/recovery.go`; preparation/read separation, explicit disposition, diagnostic propagation/signature; reuse helpers in `manager.go`, `naming.go`, `recovery_store.go` without editing them |
| acceptance author | temporary `cmd/skidbladnir/recovery_acceptance_tmp_test.go` and owned scratch helpers/fixtures; public/native boundary scenarios below; delete at completion |
| root integrator | `internal/gateway/server.go`, `docs/session-recovery.md`, architecture §8, this issue; logger composition, designer acceptance and final integration |

adapter contract precedes sessions integration; root then connects logging.
acceptance author can establish red against original code independently. root
alone reallocates a file when a concrete compile/dependency issue requires it.
no edits to catalogue, roadmap, scripts/check, clients, storage primitives or
provider adapters. consolidate only touched evidence plumbing; remove the old
malformed flag, boolean-latch path and checkpoint naming calls outright. no
parallel legacy implementation, feature flag, compatibility reader or fallback.

## acceptance and execution

use production gateway routes and real private-socket tmux with short scratch
`HOME`/`XDG_*`/`TMUX_TMPDIR`, `TMUX` unset, and explicit non-7341 loopback port.
use `Config.TmuxPath`/`SocketName` to delegate to real tmux and corrupt exactly
one selected reply AFTER command execution. wrappers record synthetic effect
counts only. no real providers/accounts, installed services, devices or hosts.

temporary acceptance runner lives in the cli package: its `TestMain` routes
`terminal-exec`/`agent-exec` to existing `run`, so `os.Executable()` launches real
startup rather than dead test panes. source existing
`deployment/providers/shell-init` from scratch bash/zsh login rc; copy its companion
to `$XDG_DATA_HOME/skidbladnir/current/providers/terminal-context-init`.
configure private tmux with
bash/zsh `default-shell`, empty `default-command`, `exit-empty off` and
`exit-unattached off`. compose existing gateway dependencies and
`gateway.ListenAndServe`; never invoke `serveStandalone` or add production seams.

| case | observable acceptance |
| --- | --- |
| rejected ordinary capture | after a baseline, corrupt the mutation-triggered inventory response; authenticated group PUT returns 204 and native group changes; checkpoint bytes/time stay fixed; GET shows delayed; next normal tick plus its budget saves the mutation and returns tracking/no notice |
| lost preparation acknowledgement | create an untagged native session; execute its character assignment then suppress acknowledgement; one assignment occurred, no candidate published; a fresh pass observes the character and saves without another assignment |
| naming ownership | change a synthetic title on an automatic session; checkpoint preserves actual name; later explicit inventory may rename it |
| source fences | lose the private server during capture: delay, then prove loss before restoration; a populated replacement or positively live conflicting source permits no restore/overwrite/preparation |
| restoration barrier | inject failure after a real claimed reconstruction effect and before final file replacement; another tick and runner restart retain attempted state and never repeat construction/startup; missing-cwd preflight stays manual even after cwd repair |
| diagnostics and copy | framing/value and lost-marker injections produce correct closed diagnostics, distinct transition signatures and dispatch uncertainty; public notice is truthful, unchanged in shape and free of diagnostic/private values |

1. **spec review:** independently attack the policy table, effect/read separation,
   diagnostic/copy schema and file ownership; resolve counterexamples first.
2. **red:** run temporary scenarios on original code. review that each changed
   behavior fails for its intended reason, while safety controls pass. reject
   failures caused by test setup, dead panes or wrong injection stage.
3. **green:** implement only the slices above; run the same scenarios. independently
   challenge effect counts, native graph, durable file and public response—not
   private helper structure or merely a cleared banner.
4. **refactor:** remove touched duplication/dead paths; rerun the same scenarios.
   adversarially check source admission before effects, no command replay, no
   speculative absence, and no weakened whole-graph/storage/restore rules.
5. **finish:** delete temporary tests/wrappers/fixtures; update architecture and
   recovery spec in place, resolving their malformed-output contradiction.
   run `scripts/check verify host` and whitespace/diff review. keep content-free
   scenario/platform results in the change review; unavailable boundaries are
   `NOT_RUN`. static checks do not prove behavior. delete this issue when its
   reproducible containment/attribution defect is resolved; do not claim the
   unrecorded historical response was explained or its parser repaired.

## limits and tradeoffs

retain 30 s sampling gaps, complete-graph rejection, inferred conversation ids
and manual preflight repair. persistent native faults retain a warning and incur
bounded work each tick; removing checkpoint naming permits stale title-derived
names until inventory. privacy-safe diagnostics cannot reconstruct raw failures.
temporary test deletion intentionally forfeits retained regression protection.

no recovery control ui/command, watchdog/restart loop, new daemon/database/journal,
partial checkpoint, schema migration, protocol rewrite, or fleet/device redesign.
installation and operational repair are separate work requiring current-turn
owner approval.
