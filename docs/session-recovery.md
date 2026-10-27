# Workspace recovery

After a host reboot or the loss of its tmux server, the gateway reconstructs the
last confirmed workspace and requests selected native conversations through the
current configured profile commands. Unresolved or unusable targets become
explained shells. tmux owns terminals, providers own history and execution, and
skid owns this private reconstruction recipe. The
[architecture](architecture.md#8-workspace-recovery) states the capability boundary.

## Decisions

Owner decisions:

- Cover every session skid exposes, desktop-created ones included, with all their
  windows and panes.
- Checkpoint every 30 seconds and right after each confirmed skid structural
  mutation. Native tmux changes can be lost between checkpoints.
- Restore automatically after a reboot **or** a same-boot server loss, only into an
  absent or empty server. A populated replacement server is broken recovery.
- Restore the last confirmed recipe regardless of its age; saved session names
  return as manual names, and automatic-title reset stays available.
- An unsuccessful ordinary checkpoint never disables later saves: keep the
  confirmed recipe, disclose the gap, retry on the next tick. Restoration stays
  conservative: never infer server death from a failed observation, never repeat
  possible reconstruction effects.
- No recovery controls, pages, commands, retry buttons, merge or repair service in
  skid; repair is manual.
- Include every unique recognized local Codex/Claude pane, inactive and manually
  launched agents included. Linked appearances count once. Discover native IDs
  first, then infer from provider titles; equal best candidates stay unresolved.
- Distinct panes selecting the same provider/home/ID launch once, in the lowest
  numeric eligible saved pane key. Other panes explain the duplicate.
- Finish at encoded startup dispatch and durable graph publication; say resume
  requested. Provider login, trust, errors and readiness remain in the terminal.
- Shell startup is unchanged: missing directories are rejected before recovery
  rather than changing every terminal's cwd failure policy.

| Preserved | Contract |
| --- | --- |
| Sessions | Exact names, dwarf character, skid group label, active window index |
| Windows | Names, indexed session links, shared-window identity, native tmux session-group membership |
| Panes | Order, local cwd, split layout, active pane, zoom state |
| Geometry | Saved dimensions for reconstruction; ordinary client sizing applies after attachment |

Native tmux group names are regenerated; skid group labels stay exact. Each shared
window is saved once: copying it per session would duplicate processes and change
close semantics. SSH and mosh panes become local shells at their **local** cwd.
Window names are restored with automatic renaming disabled.

Non-goals: process or memory preservation; fresh agent launch; provider history
storage, authentication, conversation browsing or live controls; input replay,
continuing interrupted work, daemon management, hooks or footer scraping;
scrollback; objectives; environment
or arbitrary tmux options; SSH reconnection; old attachments, focus, ready notices
or terminal references; cross-host recovery; offline inventory; configurable
recovery policy; a general backup, hook or lifecycle framework.

## Ownership and composition

tmux remains the authority for the live workspace. The checkpoint is only a
reconstruction recipe; it never supplies inventory or agent status.

| Owner | Responsibility |
| --- | --- |
| `sessions` (`recovery.go`, `recovery_store.go`) | Checkpoint policy, loss decision, restore decision, the file, the state, failure classification; `internal/api` declares the fact and its notice copy (`api.Recovery.Notice`) |
| `tmux` (`recovery.go`) | Presence distinct from empty, coherent graph capture, opaque final-pane startups, guarded reconstruction, layout comparison |
| `agentruntime` (`conversation*.go`) | Bounded native metadata discovery, provider title grammar and ranking; no recovery policy |
| `hostconfig` / profile runtime | Unchanged user command text and current profile/home scope |
| Gateway | Starts recovery after the listener, owns its cancellation, logs its transitions |
| `process` | Boot identity and kernel start identity that prove whether the saved server process lives |
| Clients | The inventory's recovery fact and notice, shown outside every view, with fresh inventory only |
| dev-server | The managed tmux configuration (operations below) |

A process-held exclusive flock on `workspace.lock` precedes any recovery effect, so
a second gateway cannot become the writer; an initial lock or storage failure
reports broken recovery without disabling ordinary terminals. The loop starts after
the listener, observes at once and then every 30 s without clients, and stops and
joins before the lock is released. Graph capture, publication, reconstruction and
skid structural mutations use the manager gate; provider metadata discovery runs
outside it. The gate does not serialize native tmux clients. One restore operation has a 30 s deadline, an ordinary checkpoint
10 s, and a mutation's checkpoint runs within the rest of that mutation's own
deadline, at most 10 s. There are no detached jobs, durable job IDs, second poller
or unbounded retry; reconstruction may delay an ordinary inventory request, and
the next poll reports the result.

## Profile commands

Extend the existing profile; no new configuration subsystem:

```yaml
profiles:
  - name: work
    command: codex-work
    provider: codex
    home: ~/.codex-work
    resume_command: 'codex-work resume "$1"'
  - name: personal
    command: claude-personal
    provider: claude
    home: ~/.claude-personal
    resume_command: 'claude-personal --resume "$1"'
```

`resume_command` is optional ordinary shell text. Omitted/null/empty means
disabled; other values must be nonblank UTF-8 without NUL and require `provider`.
Disabling resume does not prevent discovery or ordinary creation.
Propagate it through `hostconfig.Profile` and `agentruntime.Profile` unchanged.

Pass the saved canonical UUID as the sole positional argument by composing
`set -- <shell-quoted-id>`, a newline, then the exact configured text. base64url
encode with the existing launch codec. Reuse `TerminalCommand → terminal-exec →
agent-exec → shell-init`; evaluate in its existing one-shot startup function.
`set --` changes that function's arguments, not the interactive shell's arguments.
Aliases, functions, startup, permissions and account selection remain user-owned.
No template parser, flag appending, stock-command substitute or command history.

Configuration owns using `"$1"` and the advertised provider home correctly. skid
cannot prove arbitrary shell code's semantics. Command edits affect later recovery;
profile removal or changed provider/home makes that saved target unusable.

## Checkpoint schema and capture

Path `${XDG_STATE_HOME:-~/.local/state}/skidbladnir/workspace.json`, beside its
`workspace.lock`; directory `0700`, files `0600`. One schema, no migration or
compatibility reader. `?` means omitted when absent. The reader rejects unknown and
duplicate fields; a missing or null field reads as its zero value, except that
`savedAt` must be present and `agents` must be a non-null map. Variant absence
is semantic: null `resume` or null/empty `reason` is absent; exactly one complete
variant must remain. The writer emits canonical UTC nanosecond timestamps.

```text
checkpoint = {
  schema: 2,
  machine: installation-handle,
  savedAt: utc-rfc3339nano,
  source: {bootId, processStartIdentity, server: {epoch, pid, startTime}},
  attempted: boolean,
  sessions: [{key, name, character, group?, nativeGroup?, activeWindowIndex,
              links: [{index, windowKey}]}],
  windows: [{key, name, width, height, layout, activePaneKey, zoomed,
             panes: [{key, cwd}]}],
  agents: {"%N": {provider:"Codex"|"Claude",
    resume?:{profile, accountScope, id, source}, reason?:reason}}
}
```

- `key` values are snapshot-local tmux IDs (`$…`, `@…`, `%…`), never live targets.
  `nativeGroup` is the observed native name, used only as a membership key; `group`
  is a skid label. Panes are in native pane-index order. `layout` is the full,
  unzoomed native layout; the kernel start identity plus boot ID distinguishes PID
  reuse.
- Admission (reader and capture alike) accepts only what restoration needs to
  rebuild and verify: globally unique native-ID keys and session names, unique
  window indices per session, resolved links and selections, nonempty windows and
  sessions, matching indexed links within native groups, every window linked, a
  nonempty layout and character, absolute cwds, the catalogue character and skid
  label grammars. Window names may repeat; native names follow tmux's literal rules
  (nonempty, no `:` or `.`), not skid's create-name grammar. An empty workspace has
  both arrays empty. The record and the capture are bounded at 8 MiB with no count
  or string limits; exceeding it rejects the whole capture, never omitting entries
  or truncating values.
- Every agent key names a saved unique pane, with one complete resume target or
  unresolved reason. IDs are canonical lowercase UUIDs; account scope is the
  existing provider/canonical-home hash, not a verified login. Valid sources and
  reasons are defined below. Persist no titles, commands, credentials or history.
  Ordinary captures include agents recognized at either end of the observation
  bracket. An agent that becomes a shell keeps `observation_changed` for that
  pass; the next stable-shell capture removes it. The first restored checkpoint
  carries the dispatched intent described below.
- Before capture, source admission precedes guarded epoch/character preparation,
  including on mutation-triggered saves. An uncertain metadata write ends that
  pass; the next attempt reads current state before deriving a new conditional
  correction. Checkpointing never renames sessions: inventory owns automatic
  naming, and the recipe preserves actual tmux names.
- Both graph captures take the prepared session IDs and read every
  required fact in one synchronous `source-file -` stdin queue bracketed by the
  source's identity records, reusing inventory's `#{q:…}|` fields and decoder, each
  field evaluated once (cwd can change between reads, so a separate length read
  could not frame it). Session-local metadata come from inventory's
  existence-guarded `show-options -q` script in the same queue, since format
  expansion would inherit globals. Linked windows repeat pane rows: the same
  window, index and pane identity is required and the first cwd/PID/title sample kept.
  Captured sessions that differ from those IDs are a topology change. The full graph
  and an unchanged source lifetime are validated before publication.
- Graph capture performs no metadata writes. After restoration, obtain the final
  session IDs by native inventory and use the same read-only capture path.
- Capture never uses enriched inventory: its remote projection hides the local cwd.
  PID and raw title are capture-only (`json:"-"`); graph admission never requires
  them. Native process/environment evidence is transient and never serialized.
- Publication goes through `privatefile.WriteFile` (temporary file, sync, close,
  rename, parent sync; success only after the parent sync). There is no rolling
  history, journal or per-pane receipt.

## Conversation discovery

`agentruntime` owns stateless metadata adapters, reusing `MatchProfile` and
`AccountScope`. `sessions` owns observation acceptance. `WorkspacePane` gains
captured-only PID/title fields (`json:"-"`); tmux captures these and local cwd for
**all** unique panes. Persisted graph admission does not require transient fields.
Never use the lossy `titleName` slug or session launch-profile label as identity.

One batched call per observed profile/home:

```text
DiscoverConversations(ctx, profile, queries[]) -> results[]
query  = {paneKey, foregroundPid, rawTitle, cwd}
result = {paneKey, id, source} | {paneKey, reason}
source = process_record | title_match
reason = profile_unavailable | metadata_unavailable | no_match
       | tied_matches | observation_changed
```

`id` is a canonical lowercase UUID: Codex's thread ID or Claude's session ID,
never Codex's root-session/lineage ID. Result variants are exclusive; no scores,
stored candidate lists or invented confidence percentages.

| Provider | Acquisition |
| --- | --- |
| Claude, primary | Read `<home>/sessions/<foregroundPid>.json`; require matching `pid` and valid `sessionId`; reread the record before accepting it. Missing/invalid/unreadable native metadata permits title inference. |
| Codex | No supported selected-thread process record was found. Go to title inference; do not add fd, argv or inherited-environment guesses. Writers can retain old threads, and a daemon owns several clients' writers. |
| Codex, titles | Bounded complete `session_index.jsonl`; `{id,thread_name,updated_at}`; last appended row per ID wins before matching. Enumerate rollout filenames for matching IDs; read only first `session_meta` record for ID/cwd. No SQLite or transcript scan. |
| Claude, titles | Enumerate main-session UUID jsonl files under `projects`; inspect bounded head/tail metadata: latest `custom-title` record's `customTitle`, otherwise latest `ai-title` record's `aiTitle`, plus cwd; exclude sidechains. Read top-level records, never keys embedded in messages. |

Title comparison preserves case, punctuation and Unicode. Compare provider-rendered
names with complete recognized title fields, stripping only qualified provider
adornments. No generic substring, slug or fuzzy match. For Codex:

- Trim the name; retain ≤48 extended grapheme clusters, otherwise first 45 plus
  `...`; then apply its pinned terminal-title sanitizer, not the reverse order.
  Pin Unicode 16 segmentation to match Codex's `unicode-segmentation` 1.12.0.
  [go-textseg/v16](https://github.com/apparentlymart/go-textseg/tree/v16.0.2)
  supplies those boundaries. The existing Unicode-15 and
  Unicode-17 segmenters disagree on Indic conjuncts; reusing either changes native
  shortening. This narrow dependency avoids hand-maintained Unicode tables or
  changing text handling elsewhere in the application.
- Fields are bounded by start/end, ` | `, or standalone activity tokens: U+25CF,
  a spinner, or U+25CF plus a spinner. Spinner codepoints are U+280B, U+2819,
  U+2839, U+2838, U+283C, U+2834, U+2826, U+2827, U+2807, U+280F.
- Recognize longest tokens first; allow one trailing standalone name-generation
  spinner. Strip only the leading `[ ! ] Action Required` or `[ . ] Action Required`
  notice field, optionally preceded by U+25CF and a space.
- Names containing structural delimiters/activity tokens get whole-title equality
  only. Decline title inference at 240 raw-title Unicode scalar values, where
  global clipping becomes ambiguous; GNU screen's remapping is unsupported.

Untyped fields can mistake a matching project/model field for a name; accept that
inference risk. Full match means no name shortening; sanitation still applies.
Unknown custom layouts yield no candidate.
Claude 2.1.310 matches the whole title exactly against U+2733, one ASCII space,
then the saved name. No Codex field splitting, trimming or 240-character limit.
Isolated idle/working probes covered automatic/custom, punctuation, long and
Unicode names. Normalization/clipping can happen before storage; use the stored
name, not the original rename input. Other adornments/layouts are unsupported.

Rank actual title matches lexicographically: full before known shortened match;
equal canonical cwd before unknown before different; newest metadata update time
last. Missing time ranks oldest. Equal best tuples stay unresolved; UUID order is
never a tiebreak. Recency means metadata update, not proof of last activity.
Codex uses index `updated_at`; Claude uses transcript modification time, consistently
for that provider. Cwd/recency without a title match never chooses an ID.

Initial per-home read bounds: registry 64 KiB; Codex index 8 MiB; first rollout
record 1 MiB; Claude head/tail 64 KiB each; 16,384 directory entries and 64 MiB total
reads. A truncated index/candidate enumeration, or unreadable/invalid enumerated
Claude candidate, makes the tier unavailable, not a newest-n search. Incomplete
Codex cwd enrichment makes cwd unknown for every candidate. Claude names outside
sampled windows can be missed; that is an explicit limit. Inspect regular files,
ignore partial head/tail boundary lines, and reject malformed complete records.
Stat index/transcript files before/after reads; changed identity, size or mtime
makes that tier unavailable for this pass. Check cancellation between reads;
retain no content and add no retry.

## Checkpoint observation and commit

Reuse the existing recovery loop and manager gate. Structural handlers release
the gate after the confirmed effect, then synchronously attempt checkpointing
within the existing deadline before replying. Checkpoint failure cannot change
that mutation's confirmed result. No second lock or asynchronous worker is needed.

Checkpoint phases:

1. Under the manager gate, pin immutable settings and the current immutable recipe
   pointer as a commit anchor. Admit the source before required metadata preparation,
   then capture graph/raw pane facts without mutations.
   Observe foreground identity/environment through existing `process` primitives;
   use the observed profile, never the launch label.
2. Release that gate for provider metadata reads. Share a 2 s discovery sub-budget
   inside the existing 10 s checkpoint deadline; exhaustion produces unresolved
   entries, leaving time for topology validation/publication. Close native registry
   observations in this phase, before gate reentry. No nested retry.
3. Reacquire it. Discard results, including stale failures, if the recipe pointer
   changed, recovery became broken or its owner was canceled. Every successful
   save and restore claim already replaces that pointer. Otherwise recheck source,
   graph and settings. Changed membership, links, layout, selection or settings
   discard the candidate checkpoint as delayed. Recheck foreground lifetime,
   executable, profile environment and
   cwd. Native results require the same ID at registry close; title results require
   the same candidate match against freshly parsed title fields, ignoring only
   qualified activity/progress adornments. Raw-title equality would incorrectly
   reject every spinner tick. Changed evidence becomes `observation_changed`, not
   an old retained ID. Publish the final recaptured graph, including fresh
   cwd/name/group facts. Retain source admission: never replace a nonempty recipe
   with another source; replace an empty source only after proving its death.
   Commit only after durable `privatefile.WriteFile` completion. Recapture never
   performs preparation or automatic naming.

Published recipes, including nested maps/slices, are immutable. Graph comparison
excludes captured-only PID/title fields; per-pane freshness is checked separately.
The anchor prevents an older scan overwriting a newer recipe. No provider scan
holds the manager gate; no generation registry or extra polling loop. The discovery
deadline is cooperative between filesystem reads, not cancellation of a blocked
syscall; byte/count limits assume ordinary local provider storage.
Concurrent mutation checkpoints may duplicate bounded reads; only an admitted
candidate commits. If deadline expiry prevents reacquiring the gate, abandon that
attempt without an unlocked status write or extending the request: `savedAt` stays
unchanged and the next existing tick retries. Accept that missed immediate warning.
Ordinary observations replace the complete agent map. Unresolved samples remove
automatic-resume eligibility rather than silently retaining stale IDs. Native
same-process switches/title lag can escape these brackets; inference remains
best effort. Native tmux changes between final observation and save remain the
existing snapshot limitation.

Shutdown keeps the existing barrier: cancel the recovery owner, join its loop,
acquire the manager gate, then close `workspace.lock`. Every request checkpoint
derives its context from that owner. Late scans cannot publish or write after
cancellation; already-dispatched native commands remain provider-owned.

## State and ordering

`recipe` is the admitted startup checkpoint or the last checkpoint this owner
published successfully. An unsuccessful write never replaces it in memory or
advances `savedAt`, even when the rename landed; `savedAt` is the capture time of
the confirmed recipe, not of a failure or of the final sync. Only `attempted` is
durable recovery state; every state below lives in memory.

| Observation or result | Transition |
| --- | --- |
| Initial lock, read or admission failure; loaded `attempted=true` | `broken`; storage preserved; no recovery effects |
| No recipe and no live server | Create nothing; keep the current fact |
| No recipe and a live server | Attempt a complete baseline; a failed save is `delayed` without `savedAt` |
| The saved source survives | Capture its complete current graph, a genuinely empty one included |
| Ordinary checkpoint deadline with gate held | `delayed`; a deadline is not evidence of changed identity; gate-reentry timeout abandons without a status write |
| Capture race, malformed/unavailable native observation, a supported-limit rejection or an unsupported live value | `delayed`; publish nothing; keep the recipe |
| Inconclusive boot/process/socket observation, or source disappearance/change during capture | `delayed`; next tick repeats source-loss proof and target checks |
| Uncertain ordinary preparation write | `delayed`; end the pass; fresh observation precedes any later correction, never replay the failed invocation |
| Ordinary publication failure, before or after rename | `delayed`; keep the recipe; repeat full source checks before another attempt |
| Saved source positively alive but another server observed; ordinary save would replace a nonempty recipe from another source | `broken`; no preparation, overwrite or reconstruction |
| Source proven dead; recipe empty | Reconstruct nothing; baseline a new live server when present |
| Source proven dead; recipe nonempty | Regardless of age, run the target and preflight checks and durably claim the exact recipe; never select an unconfirmed candidate from the pathname |
| Populated replacement, failed preflight or failed claim | `broken`; no automatic repair or retry |
| Durable claim succeeds | In-memory `attempted=true` before native effects; guarded construction and full verification follow |
| Any later restoration failure, final capture or publication included | `broken`; construction is never replayed by this owner; failed final publication retains the confirmed claim in memory, while restart admits the actual file under the source fences |
| Complete ordinary publication | `tracking`, new `savedAt`, no failure |
| Verified restoration and final publication | `recovered`, new `savedAt`; the next ordinary success becomes `tracking` |
| Gateway cancellation | Stop and join before releasing the lock; no failure fact or event |

Only `tracking`, `delayed` and `recovered` continue the loop. Delayed mutation
checkpoints may also save; broken ones may not. There is no inner retry loop: the
next tick reruns the full source decision, and delayed never authorizes saving a
populated replacement over a nonempty old recipe.

Every ordinary publication passes one admission at the ordinary-save boundary: no
recipe permits a baseline; an unattempted recipe permits the same complete source
lifetime; an empty recipe permits a different source only after its original source
is proven dead. A mutation's expected server identifies its target, never
permission to replace the recipe's source. Failed admission preserves the recipe
and the mutation's confirmed result; mutation checkpointing never starts
restoration.

After a gateway restart the reader admits whichever complete file survived the
atomic replace; `attempted=true` still blocks. Without a confirmed first recipe,
source loss leaves nothing acknowledged to restore.

## Restoration

A missing socket proves nothing. A different boot ID proves the old process gone;
on the same boot, a saved PID that is absent or reused (different kernel start
identity) proves it; an observation failure does not. There is no PID-only test.

1. Pin current settings and prepare final-pane startups. Require the saved profile
   key, provider, accessible home scope and current resume command; otherwise
   prepare a fixed shell explanation. Deduplicate usable provider/scope/ID tuples
   by lowest numeric eligible saved pane. Never substitute another profile,
   conversation or directory. Validate the whole checkpoint and every requested local cwd before creating
   panes: no partial admission or substitute directory. For a live empty target,
   also require a configured executable bash or zsh, an empty `default-command`,
   `exit-empty off` and `exit-unattached off`; this check runs once, before the
   claim.
2. Durably publish the **same confirmed** checkpoint with `attempted=true` before
   any recovery tmux effect. This may replace an unconfirmed candidate left by a
   failed ordinary save; a successful claim resolves that uncertainty, and an
   uncertain claim stops recovery. If the server is absent, start it without panes,
   stamp its server identity and check the configuration above; an existing empty
   destination is only stamped when it has no epoch.
3. Guard the first creation in the same synchronous queue on the expected
   destination epoch, PID and start time, `#{==:#{server_sessions},0}` and the
   required configuration, with `-N if-shell -F` and no shell predicate (format
   booleans are `0`/`1`, unlike `show-options`' `off`/`on`). A native client winning
   the race causes refusal. Every later operation is pinned to that destination
   lifetime.
4. Create temporary sessions and native groups first, disabling session-local
   `renumber-windows` on each at once. Group leaders get fresh safe temporary names;
   members target the leader's fresh `$id`, and tmux derives the native group name
   from that leader. A singleton group needs one owned auxiliary member, closed
   afterwards. tmux briefly starts an ordinary shell for grouped members before
   synchronizing windows; an empty `default-command` prevents command replay. Saved
   group names are never target selectors.
5. Construct each unique window once in one owned, ungrouped staging session, then
   link it from its exact fresh index into every saved index: tmux refuses linking
   within one native group, and a separate source also handles repeated links to
   one window within a session. One representative per native group suffices; tmux
   synchronizes the others. The staging session closes after every final link
   exists. Only final unique-pane creation receives its prepared encoded launch
   through `TerminalCommand`; placeholders, grouped-member and staging shells
   receive none. Format-bearing arguments use `formatLiteral`; captured values
   are never re-expanded. Saved IDs only index the in-memory map; every target is
   a fresh exact ID. No prior command or terminal input is replayed.
6. Remove the renumber override once final indices exist; restore layout by native
   pane order, then active selections and zoom; restore metadata without automatic
   name markers; set window `automatic-rename off` and `allow-rename off`; assign
   final names last.
7. Recapture and verify the expected graph and metadata through the old→new ID
   map, comparing native group membership, not its regenerated name. Shell-only
   panes must retain observed cwd equality. Resume requests may change cwd after
   startup; their starting cwd is enforced by preflight/construction. Opaque
   commands that destroy panes still fail full graph verification.
   `select-layout` receives the original layout and owns validation, so a malformed
   layout can fail after panes exist. For equality only, verification removes the
   checksum and maps leaf pane IDs in JSON or checksum-prefixed text layouts,
   comparing everything else: tmux prunes unmatched cells, so command success alone
   is insufficient. Return the existing pane map only after full verification.
8. Capture the final graph and remap only dispatched targets into its new agent
   map as recipe intent, without waiting for provider startup. Retain the original
   selection source; skipped panes have no target. Durable publication with
   `attempted=false` completes recovery. The next ordinary capture replaces these
   bindings from fresh evidence. Partial effects, timeout,
   lost reply or final-save failure stay broken, never rolled back, merged or
   replayed.

skid checkpoints right after confirmed create, shell, rename, group and kill,
after releasing their gate, within the existing request deadline. A failed save never changes the mutation's result and never
invites repeating a confirmed terminal effect. Input, resize and agent stop are not
structural. 30 s is the healthy cadence, not a real-time guarantee.

## Failure wire and diagnostics

The inventory's required `recovery` member:

```text
recovery =
  {state:"tracking", savedAt?:instant}
  | {state:"recovered", savedAt:instant,
     resume?:{requestedPanes,inferredPanes,skippedAgentPanes}}
  | {state:"delayed", savedAt?:instant, failure:failure}
  | {state:"broken", savedAt?:instant, failure:failure}
  plus notice?:{summary, action?}     // the host's rendering

failure = {at:instant, phase:phase, cause:cause}
instant = canonical utc rfc3339nano
```

`failure.at` is the latest failed attempt's observation time; no ordering between
instants is required, since wall clocks move. Tracking and recovered forbid a
failure; delayed and broken require one. Resume counts occur only in recovered,
are nonnegative unique-pane counts, and inferred never exceeds requested. Requested
means a final pane created with an encoded resume launch, not proof that shell
startup reached eval or a provider ran. Partial conversation coverage never breaks
healthy graph saving. State alone says whether saving will
retry. The Go client rejects only an unknown state, phase or cause.

| Phase | Label | Causes |
| --- | --- | --- |
| `ownership` | writer lock | `busy`, `permission`, `no_space`, `io` |
| `checkpoint_read` | checkpoint read | `permission`, `io`, `invalid`, `limit`, `already_attempted` |
| `source` | tmux server check | `timeout`, `changed`, `unavailable`, `invalid`, `permission`, `io` |
| `capture` | workspace capture | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `io` |
| `checkpoint_write` | checkpoint save | `limit`, `permission`, `no_space`, `io` |
| `restore` | workspace restore | `timeout`, `changed`, `unavailable`, `invalid`, `limit`, `permission`, `no_space`, `io`, `not_empty` |

Cause phrases: timeout `timed out`; changed `workspace or server changed`;
unavailable `required resource unavailable`; invalid `invalid data or
configuration`; limit `supported limit exceeded`; busy `lock already held`;
permission `access denied`; no_space `storage space exhausted`; io `input/output
error`; already_attempted `prior restore may be incomplete`; not_empty
`replacement server has sessions`.

Disposition belongs to the lifecycle callsite, independently of failure cause.
Ordinary source, preparation, capture and publication failures are delayed;
positive source conflicts and startup ownership/admission failures are broken.
Restoration preflight/claim and every post-claim failure are broken. Boot, process
and identity checks belong to `source` even inside capture; restoration preflight,
execution and verification use `restore`; final recapture keeps its source or
capture label but always breaks; claim and final storage failures are
checkpoint_write and broken.

Classification happens where the evidence is, never from error text:

- timeout: a budget exhausted before context-aware work, or such work ended by its
  deadline, read from the context because a killed subprocess loses the cause;
  owner cancellation is separate. Positive identity or admission evidence and real
  filesystem errors win even when the deadline also expired.
- changed: a positively detected workspace or server race (capture loss is
  source/changed/delayed; a positively live conflicting source is broken). limit:
  the 8 MiB bound. invalid: rejected external data or configuration (unsupported
  live values and malformed ordinary observations are delayed).
- permission `EACCES`/`EPERM`; no_space `ENOSPC`/`EDQUOT`; busy: writer-lock
  contention; io: any other filesystem or kernel error, `io.ErrShortWrite`
  included.
- unavailable: failed native observation, or an unavailable cwd or executable,
  without inventing absence (`ErrProcessNotPermitted` is unavailable).
- A failure no detector names is unavailable on tmux and kernel paths and io on
  storage paths; classification never panics.

Every state or signature change (state, phase, cause, operation, errno, protocol
record/field/reason and dispatch uncertainty) logs one
`Workspace.RecoveryChanged` with `skidbladnir.recovery.at`, `.state`, and when
present `.saved_at`, `.failure_at`, `.phase`, `.cause`, `.operation` and `.errno`.
Operation is a source-written literal (`mkdir`, `chmod`, `open`, `lock`, `read`,
`close`, `write`, `boot_identity`, `process_observe`, `server_inspect`,
`server_identity`, `inventory_read`, `character_assign`, `graph_capture`, `capture`,
`validate`, `restore_preflight`, `restore_execute`); errno appears only when
extracted from a real syscall error.
Native protocol diagnostics add optional `.protocol_record`, `.protocol_field`
(1-based; omitted when no particular field), `.protocol_reason` and
`.dispatch_unknown` (true only). Record labels identify batch begin/end, inventory
row, session metadata, workspace session/window/pane/graph, server preflight or
guard completion. Reasons are `framing`, `invalid_value`, `duplicate`,
`inconsistent` or `unexpected_marker`. These are closed source literals; no raw
values or response hashes. Diagnostics preserve joined protocol/source/dispatch
facts without entering the public recovery document. Absent dispatch evidence
never means not sent. Successful publication clears diagnostics.
Repeated identical failures update `failure.at` without logging; unchanged
tracking is silent. The callback runs under the gate, after the fact changes, and
only writes the event; a failed log write is dropped and changes nothing. Events
are diagnostics, never receipts, and carry no paths, names, terminal bytes or raw
error text.

## Notice

The host renders `summary` and `action` (`api.Recovery.Notice`); clients add
the host, the last save in the device's own zone and the tone, and never infer
retryability:

| State | Text / tone |
| --- | --- |
| tracking | No notice, also after saving resumes |
| delayed | `<host>: workspace saving delayed. <phase label>: <cause phrase>. <last-save>. retrying automatically; live terminals remain usable.` / degraded |
| broken | `<host>: workspace recovery stopped. <phase label>: <cause phrase>. <last-save>. manual repair required; live terminals remain usable.` / failure |
| recovered | `<host>: terminals restored.` plus nonzero requested/inferred/skipped clauses below / degraded |

Recovered clauses: `conversation resume requested in {n} panes.`;
`{i} requests used title inference.`; `{m} agent panes restored as shells.`
Use singular forms for one. When any were skipped, action is `open those terminals
for details.` (singular: `open that terminal for details.`). Never claim the
provider resumed. The notice clears on the next successful checkpoint.

Skipped startup text begins `skidbladnir: conversation resume skipped: ` and names
the reason: no id saved; profile removed; provider/home changed; home unavailable;
no resume command; conversation assigned to another restored pane. Except for
duplicates, show a saved ID when available and `resume manually in this shell using
the original provider profile.` Duplicates receive no instruction to start another
writer. This text has ordinary scrollback lifetime, not a durable outcome store.

`<last-save>` is `last confirmed save: <time>` or `no confirmed save`; the time is
the client's local 24-hour date and time with seconds and numeric offset, for
example `2026-10-14 23:55:13 utc-07:00`; JSON stays exact UTC. Clients show the
notice only with fresh inventory, outside every view (an empty needs-input view
included), and never revive refs, attachments or ready notices by matching names
or dwarves.

## Shell boundary and accepted limits

Panes start through `TerminalCommand` and the ordinary
[shell startup](architecture.md#creation). Preflight rejects a missing cwd before
panes exist; if it disappears later, startup reports it and may leave a usable
shell elsewhere. Success proves a reconstructed tmux graph at observation time,
not completed shell startup or provider readiness.

- Title inference can choose the wrong conversation; exact-ID dispatch cannot
  correct a wrong selection. Scope identifies a provider home, not its login.
- Native same-process switches/title lag can escape observation brackets; metadata
  readers are version-sensitive. Bounded Claude windows can miss stored names.
- Provider commands may begin before the graph is verified. Subsequent failure
  preserves the broken claim and partial terminals, without rollback or replay.
- Slow login/trust can outlive the first ordinary sample and lose seeded intent;
  another loss then restores a shell until discovery rearms that pane.
- There is no readiness supervisor, outcome ledger or automatic resume retry.
- The 30 s sampling gap and failed-save intervals remain: recovery can omit recent
  work or resurrect a recently closed session and its saved conversation request.
- Full-graph admission lets one unsupported value block saving the whole workspace.
- Persistent ordinary faults keep a visible delayed notice and bounded attempts
  on the existing tick. No checkpoint naming means title-derived names may lag
  until inventory runs.
- Re-claiming the confirmed recipe can replace a newer unacknowledged candidate.
- A crash after the claim but before creation needs manual repair; broken recovery
  leaves new live work uncheckpointed.
- Saved names lose automatic naming; native group names are regenerated, so manual
  tmux commands using old group names need updating; preserving groups costs a
  transient shell, and staging adds one ordinary transient shell that a failed
  construction can leave visible.
- A deliberate `kill-server` while the gateway runs triggers reconstruction.
- Mac recovery starts after login, not before disk unlock.
- The 8 MiB bound and the 30 s deadline, not count limits, can reject an unusually
  large workspace.
- Transition logging is synchronous and can delay work if its writer blocks.

## Operations

- The managed tmux configuration must set server options `exit-empty off` and
  `exit-unattached off`, so an empty server stays alive and a normal last-session
  exit becomes an observed empty checkpoint. Apply it to a surviving server without
  killing it. Startup uses the ordinary Linux user service and Mac login agent; no
  boot service exists.
- Delayed saving resumes by itself once its cause clears.
- Broken recovery: stop the gateway and diagnose the named phase and cause; repair
  storage or source access before restarting with the same valid recipe. Move the
  file aside only to abandon an invalid or claimed recipe deliberately, or to take
  the live workspace as a new baseline; that loses the recipe's automatic recovery,
  so inspect any partial construction first.
- Stop the gateway before an intentional `kill-server`.

## Cutover and qualification

Schema 2 only: no migration, dual reader or old restore signature. While live tmux
survives, stop the old gateway, retain its checkpoint separately, start the new
release with an empty checkpoint path and verify a durable schema-2 baseline.
If only the old recipe survives, recover with the old release first. Never discard
the sole recipe to finish an upgrade. This operational cutover needs current-turn
authority; scope approval is not deployment approval.

Qualification uses temporary public-host/native-tmux/native-provider checks with
independent registry or fixture-only visible-ID oracles. Prove original failure,
changed success, review/refactor, rerun affected checks, then delete tests. Keep no
production seam or new gate. macOS and Linux isolated runs precede separately
authorized real-account, deployed-client and reboot acceptance.

| Requirement | Observable acceptance |
| --- | --- |
| Lifecycle | Claude clear/resume/branch and Codex new/resume save current selection; failed/new samples retain no stale ID |
| Discovery | Full/clipped/Unicode titles, renames, cwd/recency ranking, ties and separate homes yield the specified result; incomplete/corrupt/over-limit candidate sets never silently select |
| Workspace | Active/inactive, linked windows, native groups, ordinary/remote panes retain graph semantics; only eligible final unique panes launch |
| Exact recovery | Destroy private tmux, restart gateway and separately Codex daemon; saved IDs reopen dummy history and continue those same transcripts |
| Launch | bash/zsh aliases/functions receive sole saved `$1`; current edits apply; missing/changed profiles, duplicate targets and invalid IDs cause no fresh or alternate launch |
| Concurrency/durability | Overlapping structural saves and bounded slow reads do not deadlock or overwrite newer recipes; source replacement, uncertain claim, partial graph and final-save failure never redispatch |
| Content | requested/inferred/skipped copy agrees with startup intent and native errors; healthy topology still saves; notice lifetime matches contract |
| Cutover | Schema 1 rejected, fresh schema 2 armed, obsolete paths/tests removed, build/static/link checks pass |
| Live | Deployed macOS/Linux reopen saved IDs after loss and reboot; phone/desktop show fresh notices; mid-turn loss preserves history without skid input replay, not resumed computation |

No host/device/live boundary means `NOT_RUN`, never pass. Outstanding qualification
and evidence live in [conversation recovery acceptance](issues/conversation-recovery-acceptance.md).

Provider contracts were probed on Codex 0.163.0 and Claude 2.1.310. Primary references:
[Codex index](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/rollout/src/session_index.rs),
[rendered title](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/chatwidget/status_surfaces.rs),
[title fields](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/bottom_pane/title_setup.rs),
[title sanitizer](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/terminal_title.rs),
[Claude registry](https://code.claude.com/docs/en/agent-view#list-sessions-as-json),
[Claude metadata](https://github.com/anthropics/claude-agent-sdk-python/blob/main/src/claude_agent_sdk/_internal/sessions.py).
