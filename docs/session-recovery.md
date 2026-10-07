# tmux workspace recovery

status: source implemented and isolated runtime qualified, 2026-10-07. actual
linux/mac reboots are owner-deferred. release and production activation remain
separate; this document grants no execution or deployment authority.

## target and decisions

after host reboot or tmux server loss, the gateway automatically reconstructs
the last saved workspace as fresh shells. the user opens a terminal, selects the
same provider account and resumes the conversation through that provider.
running agents and conversations are not resumed by skid.

owner decisions, 2026-10-07:

- cover every session skid exposes, including desktop-created sessions and all
  their windows and panes.
- checkpoint every 30 seconds and immediately after successful skid structural
  mutations. native tmux changes can be lost between checkpoints.
- restore automatically after reboot **or** same-boot server loss, only into an
  absent or empty server. a populated replacement server is broken recovery.
- preserve saved session names as manual names; existing automatic-title reset
  remains available.
- report failure, preserve the checkpoint and stop recovery. no recovery
  controls, pages, commands, retry buttons, merge or repair service in skid.
- retain existing shell startup behavior; reject already-missing directories
  before recovery rather than changing every new terminal's cwd failure policy.

this is workspace reconstruction, with manual conversation continuation. a
gateway/client restart while the original tmux server survives only re-lists it.

| preserved | contract |
| --- | --- |
| sessions | exact names, dwarf character, skid group label, active window index |
| windows | names, indexed session links, shared-window identity, native tmux session-group membership |
| panes | order, local cwd, split layout, active pane, zoom state |
| geometry | saved dimensions for reconstruction; ordinary tmux client sizing applies after attachment |

native tmux group names are regenerated; skid group labels remain exact. save
each shared window once. copying it per session would duplicate processes and
change close semantics. ssh/mosh panes become local shells at their **local** cwd.
saved window names are also restored with automatic renaming disabled.

non-goals: process/memory preservation; agent launch or resume; provider ids,
accounts or history; input/command replay; scrollback; objectives; environment
or arbitrary tmux options; ssh reconnection; old attachments, focus, ready
notifications or terminal references; cross-host recovery; an offline inventory;
user-configurable recovery policy; a general backup, hook or lifecycle framework.

## ownership and composition

tmux remains the authority for the live workspace. one host-local checkpoint is
only a reconstruction recipe. it never supplies inventory or agent status.

| owner | responsibility and reused primitives |
| --- | --- |
| `sessions.Manager` | checkpoint policy, restore decision, metadata normalization, durable file; existing mutation lock, cwd/group/catalogue admission |
| `tmux.Client` | presence distinct from empty, coherent graph capture, guarded reconstruction; existing exact server identities, format escaping, `TerminalCommand(cwd, "")` |
| gateway lifetime | one immediate-start, 30-second checkpoint loop independent of connected clients; bounded cancellation and exclusive ownership |
| `process` | existing `BootIdentity`, `Observe`, `StartIdentity`, `ErrProcessAbsent` prove whether the saved server process still lives |
| gateway/clients | required inventory recovery fact, strict decoding, passive machine notice; existing five-second client polling |
| dev-server | tmux configuration, sole recovery-owner cutover, existing linux service/mac launch agent and shell integration |

acquire a process-held exclusive lock beside the checkpoint before recovery
effects; a second gateway cannot become its writer. lock/storage failure reports
broken recovery without disabling ordinary terminals. listener admission precedes
starting recovery. stop/join the loop before releasing the lock. serialize
capture, reconstruction and skid structural mutations under the existing manager
lock; that lock does not serialize native tmux clients.

one recovery operation has a 30-second gateway-owned deadline, independent of
any inventory request. a checkpoint shares the existing bounded host-operation
budget. no detached jobs, durable job ids, second poller or unbounded retry.
document the loop's `justify-polling` reason and lifecycle. reconstruction may
delay an ordinary inventory request; the next poll reports the resulting state.

## checkpoint schema and capture

path: `~/.local/state/skidbladnir/workspace.json`, outside release generations.
lock: adjacent `workspace.lock`. private directory `0700`, files `0600`.
one schema, no migration or compatibility reader. `?` below means omitted when
absent; json null, unknown fields and duplicate fields are rejected.
timestamps use the exact canonical utc nanosecond representation: `Z`, at most
nine fractional digits and no trailing fractional zeros. validate raw spelling
before converting it to a time value; parser normalization is not admission.

```text
checkpoint = {
  schema: 1,
  machine: installation-handle,
  savedAt: utc-rfc3339nano,
  source: {bootId, processStartIdentity, server: {epoch, pid, startTime}},
  attempted: boolean,
  sessions: [{key, name, character, group?, nativeGroup?, activeWindowIndex,
              links: [{index, windowKey}]}],
  windows: [{key, name, width, height, layout, activePaneKey, zoomed,
             panes: [{key, cwd}]}]
}
```

`key` values are snapshot-local tmux ids (`$…`, `@…`, `%…`), never live targets.
`nativeGroup` is the observed native name used only as a membership key;
`group` uses the existing skid group type. dimensions and indices are integers;
`zoomed`/`attempted` are booleans; other scalar values are strings. panes are in native pane-index order.
`layout` is the full, unzoomed native layout. server fields use existing identity
grammars; the kernel start identity plus boot id distinguishes pid reuse.

admit one complete graph: globally unique keys and session names, unique window
indices within each session, resolved links/selections, nonempty windows/sessions,
matching indexed links within native groups, valid character/group/path values
and bounded native layout strings. window names may repeat. empty
workspaces have both arrays empty. native names use tmux's literal
name rules, not skid's narrower create-name grammar. validate utf-8 and bound
strings; reuse cwd's 4096-byte contract. limit the record/capture to 8 mib,
256 sessions, 512 unique windows and 1024 panes. exceeding a limit breaks saving;
never silently omit entries or truncate values.

discover live session ids, then capture required tmux facts in one synchronous
`source-file -` stdin queue, with source
identity and completion markers. use native byte-length-prefixed raw fields, e.g.
`#{n:session_name}:#{session_name}`, not delimiter splitting. bound output before
decoding; validate the full graph and unchanged source lifetime before publishing.
query session-local metadata with `show-options -q` in that same queue; format
expansion would inherit global metadata. require discovered and captured ids to
match. stdin avoids tmux's command-argument size bound at the accepted 256 sessions.
reuse existing dwarf/name normalization. do not call enriched inventory to
capture cwd: its remote projection deliberately suppresses the local path.
capture no terminal bytes, provider state or user environment. process observation
retains only server identity; discard its incidental argv observation.

publication: write a same-directory temporary file, sync it, close it, rename
over the checkpoint, sync the parent directory. reuse the pattern in
`internal/fleetclient/notifications.go`; extract a helper only if both callers
actually share the complete operation. use `strictjson` for admission. a failed
write never becomes a successful save; retain any surviving checkpoint and
report broken. no rolling history, sqlite, journal or per-pane receipts.

## state and restoration algorithm

only `attempted` is extra durable recovery state. pending, restoring and broken
are derived or in-memory facts. broken latches for this gateway lifetime: stop
checkpointing/restoration, retain the file, keep ordinary live terminals usable.

| observation | action |
| --- | --- |
| no checkpoint | baseline a successfully observed live server; absent server stays absent |
| invalid, wrong-machine or unreadable checkpoint | broken; no replacement or recovery tmux effects |
| `attempted=true` | broken; no automatic retry or checkpoint replacement |
| same exact source server | checkpoint its current graph, including an observed empty graph |
| saved graph empty, source gone | nothing to restore; baseline a new live server when present |
| source inaccessible but still alive, or liveness uncertain | broken; never start a duplicate server |
| source gone, different populated server | broken; preserve both live work and checkpoint |
| source gone, target absent/empty | preflight, then attempt reconstruction once |

missing socket is not proof of death. differing boot id proves the old process
gone; on the same boot use the saved pid/kernel start identity. absent or reused
pid proves it gone; observation failures do not. no pid-only test.

1. validate the entire checkpoint and every requested local cwd before creating
   panes. no partial admission or substitute directory. for a live empty target,
   also validate configured executable bash/zsh shell, empty `default-command`,
   `exit-empty off` and `exit-unattached off`.
2. durably publish the **same** checkpoint with `attempted=true` before any
   recovery tmux effect. publication uncertainty prohibits proceeding. if absent,
   start the server without panes. for either new or existing empty destination,
   initialize/read its canonical server identity and validate the configuration
   above. startup/configuration failure is broken.
3. guard the first creation in the same synchronous tmux queue with expected
   destination epoch/pid/start, `#{==:#{server_sessions},0}` and the required
   configuration. use `-N if-shell -F`, no shell predicate. format booleans are
   `0`/`1`, unlike `show-options`' `off`/`on`. a native client winning the race
   causes refusal. pin all later operations to this exact destination lifetime.
4. create temporary sessions/native groups first, disabling session-local
   `renumber-windows` immediately on each. give group leaders fresh safe temporary
   names; add members targeting the leader's fresh `$id`. tmux derives the new
   native group name from that leader. singleton groups need one owned auxiliary
   member, then closure of that exact auxiliary. tmux briefly starts an ordinary
   shell for grouped members before synchronizing windows; empty `default-command`
   prevents command replay. never use saved group names as target selectors.
5. construct each unique window once in one owned, ungrouped staging session,
   then link from its exact fresh session/window index into every saved index.
   tmux refuses linking within the same native group; a separate source also
   handles repeated links to one window within a session. use one representative
   per native group; tmux synchronizes the other members. close only the fresh
   staging session after every final link exists. preserve ordered panes. every
   persistent pane uses `TerminalCommand(cwd, "")`, without a launch envelope.
   encode `-c` and other format-bearing arguments through `formatLiteral`; tmux
   command-token quoting is separate. never re-expand captured values or replay
   shell commands. target fresh exact ids; saved ids only index the in-memory map.
6. remove the renumber override after final indices are installed. restore layout
   by native pane order, then active selections and zoom; never rewrite old pane ids/checksums
   inside layouts. restore metadata, omit automatic-session-name markers, and
   set window `automatic-rename off` and `allow-rename off`. assign final names
   last; use current deployment's other options.
7. capture the result and verify the expected graph, metadata and observed cwd
   values through the in-memory old→new id map; compare native group membership,
   not its regenerated name. native `select-layout` owns layout
   validation; malformed layouts can therefore fail after panes exist. for
   equality only, normalize away the checksum and map leaf pane ids; compare
   everything else. a small format normalizer suffices, not another layout
   engine. command success alone is insufficient: tmux can prune unmatched cells.
   only full verification and durable publication of a fresh checkpoint
   with `attempted=false` complete recovery. partial effects, timeout, lost reply
   or final-save failure remain broken; never roll back, merge or replay.

snapshot immediately after confirmed skid create/create-shell, rename, group
and close, before releasing their mutation lock. preserve each mutation's actual
result if saving fails; a checkpoint error never invites repeating a confirmed
terminal effect. input, resize and agent stop are not structural checkpoints.
30 seconds is the healthy sampling cadence, not a hard real-time guarantee;
capture latency is additional and `savedAt` is the honest durability boundary.

## shell boundary and accepted limits

reuse `TerminalCommand` and existing
[shell startup](terminal-continuity.md#creation-and-shell-startup) unchanged.
preflight rejects a missing cwd before panes are created. if it disappears later,
existing startup reports the failure and may leave a usable shell elsewhere.
recovery introduces no alternate directory or launch path.

success proves a reconstructed tmux graph at observation time, not completion
of arbitrary shell startup or provider readiness. later startup effects are
ordinary workspace changes; the original checkpoint may already have been
replaced. no startup acknowledgement protocol. user startup files run normally.

other deliberate costs: no merge or automatic retry after possible effects;
broken recovery leaves new live work uncheckpointed; a crash after setting
`attempted` but before creation still needs manual repair; saved names lose
automatic naming; recent native closure can reappear after a crash; deliberate
`kill-server` triggers reconstruction while the gateway runs. preserving all
native groups entails the transient shell described above and regenerated native
group names: manual tmux commands using old group names need updating. staging
adds one ordinary transient shell; failed construction can leave that owned
session visible with the rest of the partial workspace. bounded
workspace size and operation time can reject an unusually large workspace.

## api and authored content

no new route, mutation, recovery command or client-side recovery store. add one
required member to the existing authenticated `GET /v1/sessions` envelope:

```text
recovery =
  {state: "tracking", savedAt?: timestamp}
  | {state: "recovered", savedAt: timestamp}
  | {state: "broken", savedAt?: timestamp, reason:
      "checkpoint_invalid" | "checkpoint_failed" | "server_unreachable"
      | "server_not_empty" | "restore_failed"}
```

`tracking` without `savedAt` means no checkpoint yet. `recovered` lasts until
the next ordinary successful checkpoint; it is not persisted. `savedAt` is
omitted when no valid checkpoint time is known. use the existing top-level machine
binding; do not send saved topology, stderr, paths, prompts or provider metadata.
all clients hard-cut to the required schema together; no optional-field reader.
update `Peer.MarshalJSON` as well as its model so fleet output carries the fact.
ordinary live inventory remains authoritative even when recovery is broken.

the content designer owns the following closed state/reason projection in each
existing client content owner. success means recognizable shells; failure means
the user can identify the host, stopped capability and lost checkpoint coverage.

| fact | authored content |
| --- | --- |
| tracking | no notice |
| recovered | `<host>: terminals restored as shells. resume conversations using the same provider account.` |
| broken | `<host>: workspace recovery stopped: <reason>. last saved: <time or unknown>. current workspace changes are not being saved.` |
| checkpoint_invalid | `saved workspace is invalid or unreadable` |
| checkpoint_failed | `workspace could not be saved` |
| server_unreachable | `the previous tmux server may still be running` |
| server_not_empty | `tmux already has sessions` |
| restore_failed | `workspace could not be restored` |

reuse phone machine notices and desktop `noticeLines`; recovered uses existing
degraded tone, broken uses failure. no new visual system, actions, shell banner,
provider picker, toast loop or dismissal state. show outside session filters,
including empty needs-input. preserve selected view and ordinary action outcomes;
coexist with machine/pressure notices rather than replacing their projection.
phone recovery notices precede cards in the existing scrollable collection;
gesture starts on notices do not initiate pull-to-refresh. transient item offsets
preserve semantic card anchors. the dashboard top bar retains its 64 dp minimum
and grows for enlarged text. desktop full notices reduce table capacity. both
choices spend collection space to keep complete failure content readable.
never revive stale refs, native bindings, attachments or ready notices by matching
names/dwarfs. show recovery claims only with fresh inventory. a late client can
miss the recovered notice; ordinary shell cards suffice.

## deployment, hard cut and manual repair

dev-server already installs/enables tmux-resurrect and continuum. retire that
owner before activating skid recovery: remove plugin provisioning, active links,
configuration `run` lines, continuum restore setting and plugin hash dependencies.
remove live resurrect bindings by their exact owned command, not assumed keys;
reload managed status-right and remove owned resurrect script options. config
reload alone does not remove old key bindings. no old snapshot import, dual
writer or fallback. old snapshot data may remain inert; do not delete user data.

managed tmux config must set server options `exit-empty off` and
`exit-unattached off`. keeping an empty server alive lets a normal last-session
exit become an observed empty checkpoint. apply to a surviving server without
killing it. old gateway shutdown must finish before the new recovery owner starts.
use existing linux user-service startup and mac login launch agent; no new boot
service. mac recovery starts after login, not before disk unlock/login.

manual repair is outside skid: stop the gateway, preserve/move `workspace.json`
aside, repair tmux directly, restart the gateway to checkpoint the current
workspace as a fresh baseline. stopping the gateway is also the maintenance
escape hatch before intentional `kill-server`. partial reconstruction is left
visible for manual repair. rollback preserves live tmux and provider data; it
must not reactivate the retired plugins. there is no backward recovery reader.

## implementation slices and review order

each builder owns temporary tests only in its slice. assign a designer to each
acceptance feature: author input/event, expected graph/state, exact notice or
silence, and one adverse case before its builder starts. review notices at
desktop 80×24 and enlarged phone type. a separate adversary reviews the contract,
red-test sensitivity, implementation and green/refactor result.
designers/reviewers write no production files. the final verifier writes neither
production files nor tests. no unassigned cross-slice edits.

| exclusive builder | files / responsibility |
| --- | --- |
| tmux mechanism | `internal/tmux/recovery.go`, `client.go`; capture/restore/presence and exact guards |
| host policy | `internal/sessions/recovery.go`, `recovery_store.go`, `manager.go`, `naming.go`, `group.go`, `control.go`, `types.go`; file/state/loop, metadata and mutation checkpointing; `internal/group/group.go` owns shared metadata decoding |
| composition/api | `cmd/skidbladnir/main.go`, `internal/gateway/{server,gateway,dto}.go`, `internal/logging/logger.go`; lifecycle and required recovery DTO, closed content-free logs |
| desktop/cli projection | `internal/fleetclient/{response,client,content}.go`, `internal/sessionui/{session,view}.go`, `internal/agentcli/run.go`; decode/store/passive notice, existing list output only |
| phone projection | `android/app/src/main/java/dev/niels/skidbladnir/{ProductModel,GatewayClient,SkidbladnirController,DashboardScreen,DashboardEntryState}.kt`; strict model, passive grid notices and transient semantic-anchor offsets |
| deployment | sibling `dev-server/assets/dotfiles/tmux.conf`, `dev-server/lib/{dotfiles,tmux}.sh`; plugin retirement, server options, startup qualification; read that repo's instructions first |
| root integrator | `docs/{session-recovery,architecture,roadmap,dev-server-handoff,codebase-map,dashboard-pull-to-refresh}.md`; temporary `scripts/tmp-workspace-recovery` e2e driver and final integration |

order: freeze contracts/content → write temporary red acceptance against current
source → implement tmux and host/deployment prerequisites → compose api and
clients → green at real boundaries → adversarial refactor review → run required
engineering checks → delete temporary tests/harnesses → build/link review.
independent slices may proceed in parallel only after their interfaces are fixed.
reuse existing primitives; delete superseded plugin wiring in its owning slice.
do not recreate behavioral gates or a general persistence utility.
update [architecture](architecture.md) §§1/2/5/8 and
affected deployment/api docs at implementation cutover; shell startup is unchanged.

## acceptance

qualification below records the actual boundaries; source reading is not a pass.
temporary tests follow [testing policy](rules/testing.md). tmux/live/device use
requires explicit current-turn approval; every destructive tmux test uses only
its own isolated `-L` socket and temporary checkpoint/lock paths injected through
ordinary constructor config. no test uses the real checkpoint. reboot tests use
an approved disposable host or explicitly approved host reboot; restarting a
gateway does not prove reboot.

| feature / designer's authored case | temporary end-to-end acceptance |
| --- | --- |
| checkpoint continuity | real gateway/tmux with no clients; native changes saved on next 30-second tick; skid create/shell/rename/group/close checkpoint immediately; gateway-only restart leaves exact processes/graph untouched |
| complete reconstruction | same-cwd sessions with distinct names/dwarfs/groups, native singleton/multi-member groups, linked windows at nontrivial indices, rotated/mixed splits, active/zoom state and distinct local cwd; kill owned server, automatic restore matches graph with fresh refs and only shells; no captured command, remote reconnection or provider launch |
| normal closure | skid close and native final-shell exit yield durable empty checkpoints; no resurrection; crash before the next native checkpoint demonstrates the accepted loss window; surviving empty server stays alive |
| strict failure | populated replacement, corrupt/wrong-machine/oversize file, missing cwd, nonempty default-command, unsupported shell and missing socket with live source produce broken without harming live work; no checkpoint overwrite, migration, preflight directory substitution or retry |
| interruption and concurrency | native creation racing empty guard; process crash after claim/between creations/after creation before final save; deadline, lost reply, unwritable storage and interruption around atomic replace leave honest broken state and frozen recipe; no duplicates, automatic rollback or false mutation failure; no production fault-injection seams |
| shell/content/identity | real bash and zsh; admitted cwd observed after restore, with existing late-startup limitation; new refs reject old attachment/control; phone/desktop fresh-shell cards and passive notices survive empty filters, preserve action outcomes and do not revive ready; stale recovery claims rejected |
| deployment and reboot | installed linux user-service and mac login startup after actual reboot recover automatically; no resurrect/continuum writer/binding remains; repeat deployment and allowed rollback preserve live sessions; physical phone plus real desktop tty render both notices without controls |

record source, actual boundary and content-free result in this document; no
terminal bytes, paths, prompts, credentials or account data in evidence. blocked
qualification becomes one concise `docs/issues/<name>.md` issue with resolution
criteria. required engineering check: `scripts/check verify`. engineering checks
alone never satisfy a behavioral row; repeat checks only for a concrete concern.

source constraints checked against primary tmux implementations:
[options](https://raw.githubusercontent.com/tmux/tmux/3.7b/options-table.c),
[formats](https://raw.githubusercontent.com/tmux/tmux/3.4/format.c),
[layout reconstruction](https://raw.githubusercontent.com/tmux/tmux/3.4/layout-custom.c),
[group creation](https://raw.githubusercontent.com/tmux/tmux/3.7b/cmd-new-session.c),
and [process spawning](https://raw.githubusercontent.com/tmux/tmux/3.7b/spawn.c).
these establish feasibility, not installed-fleet acceptance.

## qualification

2026-10-07 source: skid `feat/session-recovery` based on `9ee461e`; sibling
dev-server `feat/session-recovery` based on `5831907`. both are isolated
implementation worktrees. evidence covers their reviewed source, not the
published release or installed production fleet. approved destructive checks
used only test-owned sockets, files, units and a separate phone package.

| acceptance | result and actual boundary |
| --- | --- |
| checkpoint continuity | pass: real darwin/linux gateway and tmux; clientless 30-second native changes, immediate create/shell/rename/group/close, same-server gateway restart with unchanged processes |
| complete reconstruction | pass: native tmux 3.7b darwin and 3.4 linux, real bash/zsh; full shared-window/group graph, repeated links, sparse indices, mixed layouts, active/zoom state, local cwd, manual names and metadata. 256-session stdin capture and whole-capture rejection at limits |
| normal closure | pass: real gateway/tmux on both hosts; final skid close/native shell exit checkpoint empty, empty server survives; crash before sampling demonstrates accepted recent-closure resurrection |
| strict failure | pass: real gateway/tmux and private files; populated replacement, missing cwd, source socket loss with live process, invalid/wrong-machine/oversize/attempted recipe and unsupported configuration. surviving work and frozen recipes unchanged |
| interruption/concurrency | pass: real darwin/linux native race, claim/create/final-save crashes, lost reply and deadline. real filesystem permission failure preserves confirmed mutation; delayed close retains its original ten-second budget. actual killed file writers leave only complete old/new records; exclusive lock and listener ownership qualified |
| shell/content/identity | pass: native fresh shells/new authority reject old refs. required wire schema and canonical timestamps through actual http and private files. real darwin 80×24 tty over https→gateway→tmux renders recovered/broken copy under empty needs-input. isolated physical s22+ api 36 exercises strict client/controller/dashboard, five reasons, pressure/action coexistence, stale silence, 2× scrollable type, notice-start exclusion, normal pull and saved semantic anchors |
| deployment | pass: sibling installer primitives against native populated/empty/absent servers on darwin/linux; exact owned alias/generation custom-table bindings, script options, links and recurring trigger retired; foreign commands/snapshots/generations and live authority preserved. config-only identity, retry after failed activation, repeat apply and fresh managed zero-session startup qualified |
| allowed binary rollback | pass: actual current→immutable previous `9ee461e1e4ac505a716e0957512ea2c644327baa`→current gateway composition on both platforms with the managed private config. exact server/session/window/pane authority, pane processes and metadata survive; previous leaves the frozen recipe untouched. rollforward owns the lock, tracks the same server and saves the identical graph with a newer time. release signing/installer transaction is outside this unchanged boundary |
| actual reboot | **NOT_RUN**: owner deferred both hosts to a maintenance window. test units from existing templates served before reboot; that proves installation/startup only. all probe units/sockets/files were removed. [remaining acceptance](issues/session-recovery-reboot.md) owns the check |

baseline runtime tests rejected missing recovery/checkpoint behavior. corrective
red cases exposed deadline extension and permissive timestamp admission before
their fixes. independent adversarial source/content reviews passed after the
corrections. tmux 3.4 kill acknowledgement precedes process exit: the native test
now proves kernel death before reconstruction, matching the host precondition;
production adds no retry or fallback.

full `scripts/check verify` passed; the final go-only correction received affected
boundary checks and final host engineering checks. temporary behavioral tests,
probe applications, units and copied qualification sources are removed before
delivery. tests are not retained regression protection. no production host apply,
release, provider launch/resume or reboot is claimed.
