# workspace recovery

after a host reboot or the loss of its tmux server, the gateway reconstructs the
last confirmed workspace as fresh shells. the user opens a terminal, selects the
same provider account and resumes the conversation through that provider; skid
resumes no agent or conversation. the [architecture](architecture.md#8-workspace-recovery)
states the contract; this spec owns the recipe, the failure policy and the
restore algorithm.

## decisions

owner decisions:

- cover every session skid exposes, desktop-created ones included, with all their
  windows and panes.
- checkpoint every 30 seconds and right after each confirmed skid structural
  mutation. native tmux changes can be lost between checkpoints.
- restore automatically after a reboot **or** a same-boot server loss, only into an
  absent or empty server. a populated replacement server is broken recovery.
- restore the last confirmed recipe regardless of its age; saved session names
  return as manual names, and automatic-title reset stays available.
- an unsuccessful ordinary checkpoint never disables later saves: keep the
  confirmed recipe, disclose the gap, retry on the next tick. restoration stays
  conservative: never infer server death from a failed observation, never repeat
  possible reconstruction effects.
- no recovery controls, pages, commands, retry buttons, merge or repair service in
  skid; repair is manual.
- shell startup is unchanged: missing directories are rejected before recovery
  rather than changing every terminal's cwd failure policy.

| preserved | contract |
| --- | --- |
| sessions | exact names, dwarf character, skid group label, active window index |
| windows | names, indexed session links, shared-window identity, native tmux session-group membership |
| panes | order, local cwd, split layout, active pane, zoom state |
| geometry | saved dimensions for reconstruction; ordinary client sizing applies after attachment |

native tmux group names are regenerated; skid group labels stay exact. each shared
window is saved once: copying it per session would duplicate processes and change
close semantics. ssh and mosh panes become local shells at their **local** cwd.
window names are restored with automatic renaming disabled.

non-goals: process or memory preservation; agent launch or resume; provider ids,
accounts or history; input or command replay; scrollback; objectives; environment
or arbitrary tmux options; ssh reconnection; old attachments, focus, ready notices
or terminal references; cross-host recovery; offline inventory; configurable
recovery policy; a general backup, hook or lifecycle framework.

## ownership and composition

tmux remains the authority for the live workspace. the checkpoint is only a
reconstruction recipe; it never supplies inventory or agent status.

| owner | responsibility |
| --- | --- |
| `sessions` (`recovery.go`, `recovery_store.go`) | checkpoint policy, loss decision, restore decision, the file, the state and its notice copy (`sessions.Recovery.Notice`), failure classification |
| `tmux` (`recovery.go`) | presence distinct from empty, coherent graph capture, guarded reconstruction, layout comparison |
| gateway | starts recovery after the listener, owns its cancellation, logs its transitions |
| `process` | boot identity and kernel start identity that prove whether the saved server process lives |
| clients | the inventory's recovery fact and notice, shown outside every view, with fresh inventory only |
| dev-server | the managed tmux configuration (operations below) |

a process-held exclusive flock on `workspace.lock` precedes any recovery effect, so
a second gateway cannot become the writer; an initial lock or storage failure
reports broken recovery without disabling ordinary terminals. the loop starts after
the listener, observes at once and then every 30 s without clients, and stops and
joins before the lock is released. capture, reconstruction and skid structural
mutations are serialized under the manager gate, which does not serialize native
tmux clients. one restore operation has a 30 s deadline, an ordinary checkpoint
10 s, and a mutation's checkpoint runs within the rest of that mutation's own
deadline, at most 10 s. there are no detached jobs, durable job ids, second poller
or unbounded retry; reconstruction may delay an ordinary inventory request, and
the next poll reports the result.

## checkpoint schema and capture

path `${XDG_STATE_HOME:-~/.local/state}/skidbladnir/workspace.json`, beside its
`workspace.lock`; directory `0700`, files `0600`. one schema, no migration or
compatibility reader. `?` means omitted when absent. the reader rejects unknown and
duplicate fields; a missing or null field reads as its zero value, except that
`savedAt` must be present. the writer emits canonical utc nanosecond timestamps.

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

- `key` values are snapshot-local tmux ids (`$…`, `@…`, `%…`), never live targets.
  `nativeGroup` is the observed native name, used only as a membership key; `group`
  is a skid label. panes are in native pane-index order. `layout` is the full,
  unzoomed native layout; the kernel start identity plus boot id distinguishes pid
  reuse.
- admission (reader and capture alike) accepts only what restoration needs to
  rebuild and verify: globally unique native-id keys and session names, unique
  window indices per session, resolved links and selections, nonempty windows and
  sessions, matching indexed links within native groups, every window linked, a
  nonempty layout and character, absolute cwds, the catalogue character and skid
  label grammars. window names may repeat; native names follow tmux's literal rules
  (nonempty, no `:` or `.`), not skid's create-name grammar. an empty workspace has
  both arrays empty. the record and the capture are bounded at 8 MiB with no count
  or string limits; exceeding it rejects the whole capture, never omitting entries
  or truncating values.
- capture takes the session ids from the normalized inventory, then reads every
  required fact in one synchronous `source-file -` stdin queue bracketed by the
  source's identity records, reusing inventory's `#{q:…}|` fields and decoder, each
  field evaluated once (cwd can change between reads, so a separate length read
  could not frame it). session-local metadata come from inventory's
  existence-guarded `show-options -q` script in the same queue, since format
  expansion would inherit globals. linked windows repeat pane rows: the same
  window, index and pane identity is required and the first cwd sample kept.
  captured sessions that differ from those ids are a topology change. the full graph
  and an unchanged source lifetime are validated before publication.
- capture never uses enriched inventory: its remote projection hides the local cwd.
  it captures no terminal bytes, provider state or environment.
- publication goes through `privatefile.WriteFile` (temporary file, sync, close,
  rename, parent sync; success only after the parent sync). there is no rolling
  history, journal or per-pane receipt.

## state and ordering

`recipe` is the admitted startup checkpoint or the last checkpoint this owner
published successfully. an unsuccessful write never replaces it in memory or
advances `savedAt`, even when the rename landed; `savedAt` is the capture time of
the confirmed recipe, not of a failure or of the final sync. only `attempted` is
durable recovery state; every state below lives in memory.

| observation or result | transition |
| --- | --- |
| initial lock, read or admission failure; loaded `attempted=true` | `broken`; storage preserved; no recovery effects |
| no recipe and no live server | create nothing; keep the current fact |
| no recipe and a live server | attempt a complete baseline; a failed save is `delayed` without `savedAt` |
| the saved source survives | capture its complete current graph, a genuinely empty one included |
| ordinary checkpoint deadline, anywhere in its observation or identity work | `delayed`; a deadline is not evidence of changed identity |
| capture race, native capture unavailability, a supported-limit rejection or an unsupported live value | `delayed`; publish nothing; keep the recipe |
| ordinary publication failure, before or after rename | `delayed`; keep the recipe; repeat full source checks before another attempt |
| malformed native protocol, contradiction of a pinned live source, missing socket with a known-live source, non-timeout boot, process or source observation failure | `broken` |
| source proven dead; recipe empty | reconstruct nothing; baseline a new live server when present |
| source proven dead; recipe nonempty | regardless of age, run the target and preflight checks and durably claim the exact recipe; never select an unconfirmed candidate from the pathname |
| populated replacement, failed preflight or failed claim | `broken`; no automatic repair or retry |
| durable claim succeeds | in-memory `attempted=true` before native effects; guarded construction and full verification follow |
| any later restoration failure, final capture or publication included | `broken`; construction is never replayed |
| complete ordinary publication | `tracking`, new `savedAt`, no failure |
| verified restoration and final publication | `recovered`, new `savedAt`; the next ordinary success becomes `tracking` |
| gateway cancellation | stop and join before releasing the lock; no failure fact or event |

only `tracking`, `delayed` and `recovered` continue the loop. delayed mutation
checkpoints may also save; broken ones may not. there is no inner retry loop: the
next tick reruns the full source decision, and delayed never authorizes saving a
populated replacement over a nonempty old recipe.

every ordinary publication passes one admission at the ordinary-save boundary: no
recipe permits a baseline; an unattempted recipe permits the same complete source
lifetime; an empty recipe permits a different source only after its original source
is proven dead. a mutation's expected server identifies its target, never
permission to replace the recipe's source. failed admission preserves the recipe
and the mutation's confirmed result; mutation checkpointing never starts
restoration.

after a gateway restart the reader admits whichever complete file survived the
atomic replace; `attempted=true` still blocks. without a confirmed first recipe,
source loss leaves nothing acknowledged to restore.

## restoration

a missing socket proves nothing. a different boot id proves the old process gone;
on the same boot, a saved pid that is absent or reused (different kernel start
identity) proves it; an observation failure does not. there is no pid-only test.

1. validate the whole checkpoint and every requested local cwd before creating
   panes: no partial admission or substitute directory. for a live empty target,
   also require a configured executable bash or zsh, an empty `default-command`,
   `exit-empty off` and `exit-unattached off`; this check runs once, before the
   claim.
2. durably publish the **same confirmed** checkpoint with `attempted=true` before
   any recovery tmux effect. this may replace an unconfirmed candidate left by a
   failed ordinary save; a successful claim resolves that uncertainty, and an
   uncertain claim stops recovery. if the server is absent, start it without panes,
   stamp its server identity and check the configuration above; an existing empty
   destination is only stamped when it has no epoch.
3. guard the first creation in the same synchronous queue on the expected
   destination epoch, pid and start time, `#{==:#{server_sessions},0}` and the
   required configuration, with `-N if-shell -F` and no shell predicate (format
   booleans are `0`/`1`, unlike `show-options`' `off`/`on`). a native client winning
   the race causes refusal. every later operation is pinned to that destination
   lifetime.
4. create temporary sessions and native groups first, disabling session-local
   `renumber-windows` on each at once. group leaders get fresh safe temporary names;
   members target the leader's fresh `$id`, and tmux derives the native group name
   from that leader. a singleton group needs one owned auxiliary member, closed
   afterwards. tmux briefly starts an ordinary shell for grouped members before
   synchronizing windows; an empty `default-command` prevents command replay. saved
   group names are never target selectors.
5. construct each unique window once in one owned, ungrouped staging session, then
   link it from its exact fresh index into every saved index: tmux refuses linking
   within one native group, and a separate source also handles repeated links to
   one window within a session. one representative per native group suffices; tmux
   synchronizes the others. the staging session closes after every final link
   exists. every pane runs `TerminalCommand(cwd, "")` without a launch;
   format-bearing arguments go through `formatLiteral`; captured values are never
   re-expanded and shell commands never replayed. saved ids only index the
   in-memory map; every target is a fresh exact id.
6. remove the renumber override once final indices exist; restore layout by native
   pane order, then active selections and zoom; restore metadata without automatic
   name markers; set window `automatic-rename off` and `allow-rename off`; assign
   final names last.
7. recapture and verify the expected graph, metadata and observed cwds through the
   old→new id map, comparing native group membership, not its regenerated name.
   `select-layout` receives the original layout and owns validation, so a malformed
   layout can fail after panes exist. for equality only, verification removes the
   checksum and maps leaf pane ids in json or checksum-prefixed text layouts,
   comparing everything else: tmux prunes unmatched cells, so command success alone
   is insufficient. only full verification and durable publication of a fresh
   checkpoint with `attempted=false` complete recovery; partial effects, timeout,
   lost reply or final-save failure stay broken, never rolled back, merged or
   replayed.

skid checkpoints right after confirmed create, shell, rename, group and kill,
under their gate. a failed save never changes the mutation's result and never
invites repeating a confirmed terminal effect. input, resize and agent stop are not
structural. 30 s is the healthy cadence, not a real-time guarantee.

## failure wire and diagnostics

the inventory's required `recovery` member:

```text
recovery =
  {state:"tracking", savedAt?:instant}
  | {state:"recovered", savedAt:instant}
  | {state:"delayed", savedAt?:instant, failure:failure}
  | {state:"broken", savedAt?:instant, failure:failure}
  plus notice?:{summary, action?}     // the host's rendering

failure = {at:instant, phase:phase, cause:cause}
instant = canonical utc rfc3339nano
```

`failure.at` is the latest failed attempt's observation time; no ordering between
instants is required, since wall clocks move. tracking and recovered forbid a
failure; delayed and broken require one. state alone says whether saving will
retry. the go client rejects only an unknown state, phase or cause.

| phase | label | causes |
| --- | --- | --- |
| `ownership` | writer lock | `busy`, `permission`, `no_space`, `io` |
| `checkpoint_read` | checkpoint read | `permission`, `io`, `invalid`, `limit`, `already_attempted` |
| `source` | tmux server check | `timeout`, `changed`, `unavailable`, `invalid`, `permission`, `io` |
| `capture` | workspace capture | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `io` |
| `checkpoint_write` | checkpoint save | `limit`, `permission`, `no_space`, `io` |
| `restore` | workspace restore | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `no_space`, `io`, `not_empty` |

cause phrases: timeout `timed out`; changed `workspace or server changed`;
unavailable `required resource unavailable`; invalid `invalid data or
configuration`; limit `supported limit exceeded`; busy `lock already held`;
permission `access denied`; no_space `storage space exhausted`; io `input/output
error`; already_attempted `prior restore may be incomplete`; not_empty
`replacement server has sessions`.

delayed is published only for source/timeout, every listed capture pair and every
listed checkpoint_write pair; broken may carry any listed pair. boot, process and
identity checks belong to `source` even inside capture; restoration preflight,
execution and verification use `restore`; final recapture keeps its source or
capture label but always breaks; claim and final storage failures are
checkpoint_write and broken.

classification happens where the evidence is, never from error text:

- timeout: a budget exhausted before context-aware work, or such work ended by its
  deadline, read from the context because a killed subprocess loses the cause;
  owner cancellation is separate. positive identity or admission evidence and real
  filesystem errors win even when the deadline also expired.
- changed: a positively detected workspace or server race (topology churn is
  capture/changed/delayed; a lifetime contradiction source/changed/broken). limit:
  the 8 MiB bound. invalid: rejected external data or configuration (unsupported
  live values capture/invalid/delayed; malformed output invalid/broken).
- permission `EACCES`/`EPERM`; no_space `ENOSPC`/`EDQUOT`; busy: writer-lock
  contention; io: any other filesystem or kernel error, `io.ErrShortWrite`
  included.
- unavailable: failed native observation, or an unavailable cwd or executable,
  without inventing absence (`ErrProcessNotPermitted` is unavailable).
- a failure no detector names is unavailable on tmux and kernel paths and io on
  storage paths; classification never panics.

every state or signature change (state, phase, cause, operation, errno) logs one
`Workspace.RecoveryChanged` with `skidbladnir.recovery.at`, `.state`, and when
present `.saved_at`, `.failure_at`, `.phase`, `.cause`, `.operation` and `.errno`.
operation is a source-written literal (`mkdir`, `chmod`, `open`, `lock`, `read`,
`close`, `write`, `boot_identity`, `process_observe`, `server_inspect`,
`server_identity`, `normalize`, `capture`, `validate`, `restore_preflight`,
`restore_execute`); errno appears only when extracted from a real syscall error.
repeated identical failures update `failure.at` without logging; unchanged
tracking is silent. the callback runs under the gate, after the fact changes, and
only writes the event; a failed log write is dropped and changes nothing. events
are diagnostics, never receipts, and carry no paths, names, terminal bytes or raw
error text.

## notice

the host renders `summary` and `action` (`sessions.Recovery.Notice`); clients add
the host, the last save in the device's own zone and the tone, and never infer
retryability:

| state | text / tone |
| --- | --- |
| tracking | no notice, also after saving resumes |
| delayed | `<host>: workspace saving delayed. <phase label>: <cause phrase>. <last-save>. retrying automatically; live terminals remain usable.` / degraded |
| broken | `<host>: workspace recovery stopped. <phase label>: <cause phrase>. <last-save>. manual repair required; live terminals remain usable.` / failure |
| recovered | `<host>: terminals restored as shells. resume conversations using the same provider account.` / degraded |

`<last-save>` is `last confirmed save: <time>` or `no confirmed save`; the time is
the client's local 24-hour date and time with seconds and numeric offset, for
example `2026-10-14 23:55:13 utc-07:00`; json stays exact utc. clients show the
notice only with fresh inventory, outside every view (an empty needs-input view
included), and never revive refs, attachments or ready notices by matching names
or dwarves.

## shell boundary and accepted limits

panes start through `TerminalCommand` and the ordinary
[shell startup](architecture.md#creation). preflight rejects a missing cwd before
panes exist; if it disappears later, startup reports it and may leave a usable
shell elsewhere. success proves a reconstructed tmux graph at observation time,
not completed shell startup or provider readiness.

- the 30 s sampling gap and failed-save intervals remain: recovery can omit recent
  work or resurrect a recently closed session as a fresh shell.
- full-graph admission lets one unsupported value block saving the whole workspace.
- re-claiming the confirmed recipe can replace a newer unacknowledged candidate.
- a crash after the claim but before creation needs manual repair; broken recovery
  leaves new live work uncheckpointed.
- saved names lose automatic naming; native group names are regenerated, so manual
  tmux commands using old group names need updating; preserving groups costs a
  transient shell, and staging adds one ordinary transient shell that a failed
  construction can leave visible.
- a deliberate `kill-server` while the gateway runs triggers reconstruction.
- mac recovery starts after login, not before disk unlock.
- the 8 MiB bound and the 30 s deadline, not count limits, can reject an unusually
  large workspace.
- transition logging is synchronous and can delay work if its writer blocks.

## operations

- the managed tmux configuration must set server options `exit-empty off` and
  `exit-unattached off`, so an empty server stays alive and a normal last-session
  exit becomes an observed empty checkpoint. apply it to a surviving server without
  killing it. startup uses the ordinary linux user service and mac login agent; no
  boot service exists.
- delayed saving resumes by itself once its cause clears.
- broken recovery: stop the gateway and diagnose the named phase and cause; repair
  storage or source access before restarting with the same valid recipe. move the
  file aside only to abandon an invalid or claimed recipe deliberately, or to take
  the live workspace as a new baseline; that loses the recipe's automatic recovery,
  so inspect any partial construction first.
- stop the gateway before an intentional `kill-server`.
