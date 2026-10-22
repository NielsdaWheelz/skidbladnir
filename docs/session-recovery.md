# tmux workspace recovery

status: original mechanism implemented and isolated-runtime qualified, 2026-10-07.
[checkpoint reliability](workspace-checkpoint-reliability.md), implemented and
qualified 2026-10-15, supersedes the blanket failure latch, wire/content contract
and implementation plan. this document retains the file/graph/reconstruction
contract and original qualification. the correction's coordinated rollout and
actual linux/mac reboots remain separate.

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
- retain the confirmed checkpoint and disclose failure. the 2026-10-15 revision
  separates delayed saving from blocked recovery. no recovery controls, pages,
  commands, retry buttons, merge or repair service in skid.
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
| gateway/clients | required inventory recovery fact, tolerant decoding, passive machine notice; existing five-second client polling |
| dev-server | tmux configuration, sole recovery-owner cutover, existing linux service/mac launch agent and shell integration |

acquire a process-held exclusive lock beside the checkpoint before recovery
effects; a second gateway cannot become its writer. initial ownership/storage
admission failure reports broken recovery without disabling ordinary terminals.
listener admission precedes starting recovery. stop/join the loop before releasing
the lock. serialize
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
absent. the reader rejects unknown and duplicate fields; a missing or null field
reads as its zero value, except that `savedAt` must be present. the writer emits
canonical utc nanosecond timestamps.

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
restoration passes that original layout to tmux. verification remaps native pane
ids for comparison in json v2 or checksum-prefixed text layouts, retaining every
other field; tmux still owns geometry. the current correction is qualified on
tmux 3.8 json layouts, not a text-to-json checkpoint migration.

the reader and capture admit only what restoration needs to rebuild and verify:
globally unique native-id keys and session names, unique window indices within
each session, resolved links/selections, nonempty windows/sessions, matching
indexed links within native groups, every window linked, a nonempty layout and
character, and absolute cwds. window names may repeat. empty workspaces have
both arrays empty. native names use tmux's literal name rules (nonempty, no `:`
or `.`), not skid's narrower create-name grammar. both also admit each session's
catalogue character and skid group label and each pane's cwd grammar;
restoration preflight checks that every cwd exists and is canonical. the record and capture are bounded at 8 mib, with no count or
string limits; exceeding it rejects that complete capture, never omitting
entries or truncating values.

take the session ids from the normalized inventory, then capture required tmux
facts in one synchronous `source-file -` stdin queue bracketed by the source's
identity records. reuse inventory's quoted fields, e.g.
`#{q:session_name}|`, and its byte decoder. evaluate each field once: cwd may
change between native reads, so a separate length read cannot frame it safely.
decode escaped bytes before recognizing field and record boundaries; preserve
utf-8, newlines, pipes and backslashes. the existing 8 mib capture limit includes
quoting overhead. linked windows repeat pane rows: require the same
window/index/pane identity and retain the first cwd sample for that pane.
bound output before decoding; validate the full graph and unchanged source
lifetime before publishing.
query each id's session-local metadata with inventory's existence-guarded
`show-options -q` script in that same queue; format expansion would inherit
global metadata. captured sessions that differ from those ids are a topology
change. stdin avoids tmux's command-argument size bound.
reuse existing dwarf/name normalization. do not call enriched inventory to
capture cwd: its remote projection deliberately suppresses the local path.
capture no terminal bytes, provider state or user environment. process observation
retains only server identity; discard its incidental argv observation.

publication: write a same-directory temporary file, sync it, close it, rename
over the checkpoint, sync the parent directory. retain the existing recovery
writer and use `strictjson` for admission. a failed write never becomes a
successful save; retain the confirmed in-memory recipe and report according to
the [failure policy](workspace-checkpoint-reliability.md#state-and-ordering).
no rolling history, sqlite, journal or per-pane receipts.

## state and restoration algorithm

only `attempted` is extra durable recovery state. all other states are in memory.
[checkpoint reliability](workspace-checkpoint-reliability.md#state-and-ordering)
owns transitions: ordinary saving can be delayed and retried by the existing
owner; broken still latches for this gateway lifetime. ordinary terminals remain
usable. the last confirmed recipe remains eligible regardless of age.

missing socket is not proof of death. differing boot id proves the old process
gone; on the same boot use the saved pid/kernel start identity. absent or reused
pid proves it gone; observation failures do not. no pid-only test.

1. validate the entire checkpoint and every requested local cwd before creating
   panes. no partial admission or substitute directory. for a live empty target,
   also validate configured executable bash/zsh shell, empty `default-command`,
   `exit-empty off` and `exit-unattached off`; this check runs once, before the
   claim.
2. durably publish the **same confirmed** checkpoint with `attempted=true` before
   any recovery tmux effect. this can replace an unconfirmed candidate left by a
   failed ordinary save; successful durable claim resolves that uncertainty.
   claim publication uncertainty prohibits proceeding. if absent,
   start the server without panes, stamp its canonical server identity and
   validate the configuration above. an existing empty destination is only
   stamped, when it has no epoch. startup/configuration failure is broken.
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
capture latency is additional. `savedAt` is the capture time of the last
durably published recipe, not the instant its final sync completed.

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
session visible with the rest of the partial workspace. the 8 mib record bound
and the 30-second operation deadline, not count limits, can reject an unusually
large workspace.

## api and authored content

[checkpoint reliability](workspace-checkpoint-reliability.md#failure-boundary-and-strict-api)
owns the required `GET /v1/sessions` recovery union, strict host/go/android
cutover and exact passive copy. the old reason-only union is retired at that
cutover. ordinary live inventory remains authoritative even when recovery fails.

reuse phone machine notices and desktop `noticeLines`. show outside session
filters, including empty needs-input; preserve action/pressure notice coexistence.
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

manual repair is outside skid. delayed saving resumes automatically once its
cause clears. for broken recovery, stop the gateway and diagnose the named
phase/cause; repair storage/source access before restarting with the same valid
recipe. preserve/move the file aside only when deliberately abandoning an invalid
or claimed recipe or choosing the live workspace as a new baseline. this loses
that recipe's automatic recovery. inspect partial construction before that choice.
stop the gateway before intentional `kill-server`. rollback preserves live tmux
and provider data; it must not reactivate retired plugins or compatibility readers.

## implementation slices and review order

the original mechanism and correction are implemented. the narrower
[checkpoint reliability slices and workflow](workspace-checkpoint-reliability.md#exclusive-implementation-slices)
records ownership and qualification of the correction. it does not repeat
deployment-owner retirement or the original feature's whole implementation.

## original acceptance

this historical matrix belongs to the 2026-10-07 source, including its original
blanket failure policy. [new acceptance](workspace-checkpoint-reliability.md#red-green-refactor-acceptance)
supersedes the affected rows; [current evidence](workspace-checkpoint-reliability.md#qualification)
records their new boundaries. qualification below records only the original source.
temporary tests follow [testing policy](rules.md#testing). tmux/live/device use
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
| actual reboot | **NOT_RUN**: owner deferred both hosts to a maintenance window. test units from existing templates served before reboot; that proves installation/startup only. all probe units/sockets/files were removed. [remaining acceptance](issues/unrun-acceptance.md) owns the check |

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
