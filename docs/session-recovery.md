# workspace recovery

after a host reboot or the loss of its tmux server, the gateway reconstructs the
last confirmed workspace and requests selected native conversations through the
current configured profile commands. unresolved or unusable targets become
explained shells. tmux owns terminals, providers own history and execution, and
skid owns this private reconstruction recipe. the
[architecture](architecture.md#8-workspace-recovery) states the capability boundary.

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
- include every unique recognized local codex/claude pane, inactive and manually
  launched agents included. linked appearances count once. discover native ids
  first, then infer from provider titles; equal best candidates stay unresolved.
- distinct panes selecting the same provider/home/id launch once, in the lowest
  numeric eligible saved pane key. other panes explain the duplicate.
- finish at encoded startup dispatch and durable graph publication; say resume
  requested. provider login, trust, errors and readiness remain in the terminal.
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

non-goals: process or memory preservation; fresh agent launch; provider history
storage, authentication, conversation browsing or live controls; input replay,
continuing interrupted work, daemon management, hooks or footer scraping;
scrollback; objectives; environment
or arbitrary tmux options; ssh reconnection; old attachments, focus, ready notices
or terminal references; cross-host recovery; offline inventory; configurable
recovery policy; a general backup, hook or lifecycle framework.

## ownership and composition

tmux remains the authority for the live workspace. the checkpoint is only a
reconstruction recipe; it never supplies inventory or agent status.

| owner | responsibility |
| --- | --- |
| `sessions` (`recovery.go`, `recovery_store.go`) | checkpoint policy, loss decision, restore decision, the file, the state, failure classification; `internal/api` declares the fact and its notice copy (`api.Recovery.Notice`) |
| `tmux` (`recovery.go`) | presence distinct from empty, coherent graph capture, opaque final-pane startups, guarded reconstruction, layout comparison |
| `agentruntime` (`conversation*.go`) | bounded native metadata discovery, provider title grammar and ranking; no recovery policy |
| `hostconfig` / profile runtime | unchanged user command text and current profile/home scope |
| gateway | starts recovery after the listener, owns its cancellation, logs its transitions |
| `process` | boot identity and kernel start identity that prove whether the saved server process lives |
| clients | the inventory's recovery fact and notice, shown outside every view, with fresh inventory only |
| dev-server | the managed tmux configuration (operations below) |

a process-held exclusive flock on `workspace.lock` precedes any recovery effect, so
a second gateway cannot become the writer; an initial lock or storage failure
reports broken recovery without disabling ordinary terminals. the loop starts after
the listener, observes at once and then every 30 s without clients, and stops and
joins before the lock is released. graph capture, publication, reconstruction and
skid structural mutations use the manager gate; provider metadata discovery runs
outside it. the gate does not serialize native tmux clients. one restore operation has a 30 s deadline, an ordinary checkpoint
10 s, and a mutation's checkpoint runs within the rest of that mutation's own
deadline, at most 10 s. there are no detached jobs, durable job ids, second poller
or unbounded retry; reconstruction may delay an ordinary inventory request, and
the next poll reports the result.

## profile commands

extend the existing profile; no new configuration subsystem:

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

`resume_command` is optional ordinary shell text. omitted/null/empty means
disabled; other values must be nonblank utf-8 without nul and require `provider`.
disabling resume does not prevent discovery or ordinary creation.
propagate it through `hostconfig.Profile` and `agentruntime.Profile` unchanged.

pass the saved canonical uuid as the sole positional argument by composing
`set -- <shell-quoted-id>`, a newline, then the exact configured text. base64url
encode with the existing launch codec. reuse `TerminalCommand → terminal-exec →
agent-exec → shell-init`; evaluate in its existing one-shot startup function.
`set --` changes that function's arguments, not the interactive shell's arguments.
aliases, functions, startup, permissions and account selection remain user-owned.
no template parser, flag appending, stock-command substitute or command history.

configuration owns using `"$1"` and the advertised provider home correctly. skid
cannot prove arbitrary shell code's semantics. command edits affect later recovery;
profile removal or changed provider/home makes that saved target unusable.

## checkpoint schema and capture

path `${XDG_STATE_HOME:-~/.local/state}/skidbladnir/workspace.json`, beside its
`workspace.lock`; directory `0700`, files `0600`. one schema, no migration or
compatibility reader. `?` means omitted when absent. the reader rejects unknown and
duplicate fields; a missing or null field reads as its zero value, except that
`savedAt` must be present and `agents` must be a non-null map. variant absence
is semantic: null `resume` or null/empty `reason` is absent; exactly one complete
variant must remain. the writer emits canonical utc nanosecond timestamps.

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
- every agent key names a saved unique pane, with one complete resume target or
  unresolved reason. ids are canonical lowercase uuids; account scope is the
  existing provider/canonical-home hash, not a verified login. valid sources and
  reasons are defined below. persist no titles, commands, credentials or history.
  ordinary captures include agents recognized at either end of the observation
  bracket. an agent that becomes a shell keeps `observation_changed` for that
  pass; the next stable-shell capture removes it. the first restored checkpoint
  carries the dispatched intent described below.
- capture takes the session ids from the normalized inventory, then reads every
  required fact in one synchronous `source-file -` stdin queue bracketed by the
  source's identity records, reusing inventory's `#{q:…}|` fields and decoder, each
  field evaluated once (cwd can change between reads, so a separate length read
  could not frame it). session-local metadata come from inventory's
  existence-guarded `show-options -q` script in the same queue, since format
  expansion would inherit globals. linked windows repeat pane rows: the same
  window, index and pane identity is required and the first cwd/pid/title sample kept.
  captured sessions that differ from those ids are a topology change. the full graph
  and an unchanged source lifetime are validated before publication.
- capture never uses enriched inventory: its remote projection hides the local cwd.
  pid and raw title are capture-only (`json:"-"`); graph admission never requires
  them. native process/environment evidence is transient and never serialized.
- publication goes through `privatefile.WriteFile` (temporary file, sync, close,
  rename, parent sync; success only after the parent sync). there is no rolling
  history, journal or per-pane receipt.

## conversation discovery

`agentruntime` owns stateless metadata adapters, reusing `MatchProfile` and
`AccountScope`. `sessions` owns observation acceptance. `WorkspacePane` gains
captured-only pid/title fields (`json:"-"`); tmux captures these and local cwd for
**all** unique panes. persisted graph admission does not require transient fields.
never use the lossy `titleName` slug or session launch-profile label as identity.

one batched call per observed profile/home:

```text
DiscoverConversations(ctx, profile, queries[]) -> results[]
query  = {paneKey, foregroundPid, rawTitle, cwd}
result = {paneKey, id, source} | {paneKey, reason}
source = process_record | title_match
reason = profile_unavailable | metadata_unavailable | no_match
       | tied_matches | observation_changed
```

`id` is a canonical lowercase uuid: codex's thread id or claude's session id,
never codex's root-session/lineage id. result variants are exclusive; no scores,
stored candidate lists or invented confidence percentages.

| provider | acquisition |
| --- | --- |
| claude, primary | read `<home>/sessions/<foregroundPid>.json`; require matching `pid` and valid `sessionId`; reread the record before accepting it. missing/invalid/unreadable native metadata permits title inference. |
| codex | no supported selected-thread process record was found. go to title inference; do not add fd, argv or inherited-environment guesses. writers can retain old threads, and a daemon owns several clients' writers. |
| codex, titles | bounded complete `session_index.jsonl`; `{id,thread_name,updated_at}`; last appended row per id wins before matching. enumerate rollout filenames for matching ids; read only first `session_meta` record for id/cwd. no sqlite or transcript scan. |
| claude, titles | enumerate main-session uuid jsonl files under `projects`; inspect bounded head/tail metadata: latest `custom-title` record's `customTitle`, otherwise latest `ai-title` record's `aiTitle`, plus cwd; exclude sidechains. read top-level records, never keys embedded in messages. |

title comparison preserves case, punctuation and unicode. compare provider-rendered
names with complete recognized title fields, stripping only qualified provider
adornments. no generic substring, slug or fuzzy match. for codex:

- trim the name; retain ≤48 extended grapheme clusters, otherwise first 45 plus
  `...`; then apply its pinned terminal-title sanitizer, not the reverse order.
  pin unicode 16 segmentation to match codex's `unicode-segmentation` 1.12.0.
  [go-textseg/v16](https://github.com/apparentlymart/go-textseg/tree/v16.0.2)
  supplies those boundaries. the existing unicode-15 and
  unicode-17 segmenters disagree on indic conjuncts; reusing either changes native
  shortening. this narrow dependency avoids hand-maintained unicode tables or
  changing text handling elsewhere in the application.
- fields are bounded by start/end, ` | `, or standalone activity tokens: U+25CF,
  a spinner, or U+25CF plus a spinner. spinner codepoints are U+280B, U+2819,
  U+2839, U+2838, U+283C, U+2834, U+2826, U+2827, U+2807, U+280F.
- recognize longest tokens first; allow one trailing standalone name-generation
  spinner. strip only the leading `[ ! ] Action Required` or `[ . ] Action Required`
  notice field, optionally preceded by U+25CF and a space.
- names containing structural delimiters/activity tokens get whole-title equality
  only. decline title inference at 240 raw-title unicode scalar values, where
  global clipping becomes ambiguous; gnu screen's remapping is unsupported.

untyped fields can mistake a matching project/model field for a name; accept that
inference risk. full match means no name shortening; sanitation still applies.
unknown custom layouts yield no candidate.
claude 2.1.310 matches the whole title exactly against U+2733, one ascii space,
then the saved name. no codex field splitting, trimming or 240-character limit.
isolated idle/working probes covered automatic/custom, punctuation, long and
unicode names. normalization/clipping can happen before storage; use the stored
name, not the original rename input. other adornments/layouts are unsupported.

rank actual title matches lexicographically: full before known shortened match;
equal canonical cwd before unknown before different; newest metadata update time
last. missing time ranks oldest. equal best tuples stay unresolved; uuid order is
never a tiebreak. recency means metadata update, not proof of last activity.
codex uses index `updated_at`; claude uses transcript modification time, consistently
for that provider. cwd/recency without a title match never chooses an id.

initial per-home read bounds: registry 64 kib; codex index 8 mib; first rollout
record 1 mib; claude head/tail 64 kib each; 16,384 directory entries and 64 mib total
reads. a truncated index/candidate enumeration, or unreadable/invalid enumerated
claude candidate, makes the tier unavailable, not a newest-n search. incomplete
codex cwd enrichment makes cwd unknown for every candidate. claude names outside
sampled windows can be missed; that is an explicit limit. inspect regular files,
ignore partial head/tail boundary lines, and reject malformed complete records.
stat index/transcript files before/after reads; changed identity, size or mtime
makes that tier unavailable for this pass. check cancellation between reads;
retain no content and add no retry.

## checkpoint observation and commit

reuse the existing recovery loop and manager gate. structural handlers release
the gate after the confirmed effect, then synchronously attempt checkpointing
within the existing deadline before replying. checkpoint failure cannot change
that mutation's confirmed result. no second lock or asynchronous worker is needed.

checkpoint phases:

1. under the manager gate, admit the source, capture graph/raw pane facts and pin
   immutable settings and the current immutable recipe pointer as a commit anchor.
   observe foreground identity/environment through existing `process` primitives;
   use the observed profile, never the launch label.
2. release that gate for provider metadata reads. share a 2 s discovery sub-budget
   inside the existing 10 s checkpoint deadline; exhaustion produces unresolved
   entries, leaving time for topology validation/publication. close native registry
   observations in this phase, before gate reentry. no nested retry.
3. reacquire it. discard results, including stale failures, if the recipe pointer
   changed, recovery became broken or its owner was canceled. every successful
   save and restore claim already replaces that pointer. otherwise recheck source,
   graph and settings. changed membership, links, layout, selection or settings
   discard the candidate checkpoint as delayed. recheck foreground lifetime,
   executable, profile environment and
   cwd. native results require the same id at registry close; title results require
   the same candidate match against freshly parsed title fields, ignoring only
   qualified activity/progress adornments. raw-title equality would incorrectly
   reject every spinner tick. changed evidence becomes `observation_changed`, not
   an old retained id. publish the final recaptured graph, including fresh
   cwd/name/group facts. retain source admission: never replace a nonempty recipe
   with another source; replace an empty source only after proving its death.
   commit only after durable `privatefile.WriteFile` completion.

published recipes, including nested maps/slices, are immutable. graph comparison
excludes captured-only pid/title fields; per-pane freshness is checked separately.
the anchor prevents an older scan overwriting a newer recipe. no provider scan
holds the manager gate; no generation registry or extra polling loop. the discovery
deadline is cooperative between filesystem reads, not cancellation of a blocked
syscall; byte/count limits assume ordinary local provider storage.
concurrent mutation checkpoints may duplicate bounded reads; only an admitted
candidate commits. if deadline expiry prevents reacquiring the gate, abandon that
attempt without an unlocked status write or extending the request: `savedAt` stays
unchanged and the next existing tick retries. accept that missed immediate warning.
ordinary observations replace the complete agent map. unresolved samples remove
automatic-resume eligibility rather than silently retaining stale ids. native
same-process switches/title lag can escape these brackets; inference remains
best effort. native tmux changes between final observation and save remain the
existing snapshot limitation.

shutdown keeps the existing barrier: cancel the recovery owner, join its loop,
acquire the manager gate, then close `workspace.lock`. every request checkpoint
derives its context from that owner. late scans cannot publish or write after
cancellation; already-dispatched native commands remain provider-owned.

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
| ordinary checkpoint deadline with gate held | `delayed`; a deadline is not evidence of changed identity; gate-reentry timeout abandons without a status write |
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

1. pin current settings and prepare final-pane startups. require the saved profile
   key, provider, accessible home scope and current resume command; otherwise
   prepare a fixed shell explanation. deduplicate usable provider/scope/id tuples
   by lowest numeric eligible saved pane. never substitute another profile,
   conversation or directory. validate the whole checkpoint and every requested local cwd before creating
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
   exists. only final unique-pane creation receives its prepared encoded launch
   through `TerminalCommand`; placeholders, grouped-member and staging shells
   receive none. format-bearing arguments use `formatLiteral`; captured values
   are never re-expanded. saved ids only index the in-memory map; every target is
   a fresh exact id. no prior command or terminal input is replayed.
6. remove the renumber override once final indices exist; restore layout by native
   pane order, then active selections and zoom; restore metadata without automatic
   name markers; set window `automatic-rename off` and `allow-rename off`; assign
   final names last.
7. recapture and verify the expected graph and metadata through the old→new id
   map, comparing native group membership, not its regenerated name. shell-only
   panes must retain observed cwd equality. resume requests may change cwd after
   startup; their starting cwd is enforced by preflight/construction. opaque
   commands that destroy panes still fail full graph verification.
   `select-layout` receives the original layout and owns validation, so a malformed
   layout can fail after panes exist. for equality only, verification removes the
   checksum and maps leaf pane ids in json or checksum-prefixed text layouts,
   comparing everything else: tmux prunes unmatched cells, so command success alone
   is insufficient. return the existing pane map only after full verification.
8. capture the final graph and remap only dispatched targets into its new agent
   map as recipe intent, without waiting for provider startup. retain the original
   selection source; skipped panes have no target. durable publication with
   `attempted=false` completes recovery. the next ordinary capture replaces these
   bindings from fresh evidence. partial effects, timeout,
   lost reply or final-save failure stay broken, never rolled back, merged or
   replayed.

skid checkpoints right after confirmed create, shell, rename, group and kill,
after releasing their gate, within the existing request deadline. a failed save never changes the mutation's result and never
invites repeating a confirmed terminal effect. input, resize and agent stop are not
structural. 30 s is the healthy cadence, not a real-time guarantee.

## failure wire and diagnostics

the inventory's required `recovery` member:

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
instants is required, since wall clocks move. tracking and recovered forbid a
failure; delayed and broken require one. resume counts occur only in recovered,
are nonnegative unique-pane counts, and inferred never exceeds requested. requested
means a final pane created with an encoded resume launch, not proof that shell
startup reached eval or a provider ran. partial conversation coverage never breaks
healthy graph saving. state alone says whether saving will
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

the host renders `summary` and `action` (`api.Recovery.Notice`); clients add
the host, the last save in the device's own zone and the tone, and never infer
retryability:

| state | text / tone |
| --- | --- |
| tracking | no notice, also after saving resumes |
| delayed | `<host>: workspace saving delayed. <phase label>: <cause phrase>. <last-save>. retrying automatically; live terminals remain usable.` / degraded |
| broken | `<host>: workspace recovery stopped. <phase label>: <cause phrase>. <last-save>. manual repair required; live terminals remain usable.` / failure |
| recovered | `<host>: terminals restored.` plus nonzero requested/inferred/skipped clauses below / degraded |

recovered clauses: `conversation resume requested in {n} panes.`;
`{i} requests used title inference.`; `{m} agent panes restored as shells.`
use singular forms for one. when any were skipped, action is `open those terminals
for details.` (singular: `open that terminal for details.`). never claim the
provider resumed. the notice clears on the next successful checkpoint.

skipped startup text begins `skidbladnir: conversation resume skipped: ` and names
the reason: no id saved; profile removed; provider/home changed; home unavailable;
no resume command; conversation assigned to another restored pane. except for
duplicates, show a saved id when available and `resume manually in this shell using
the original provider profile.` duplicates receive no instruction to start another
writer. this text has ordinary scrollback lifetime, not a durable outcome store.

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

- title inference can choose the wrong conversation; exact-id dispatch cannot
  correct a wrong selection. scope identifies a provider home, not its login.
- native same-process switches/title lag can escape observation brackets; metadata
  readers are version-sensitive. bounded claude windows can miss stored names.
- provider commands may begin before the graph is verified. subsequent failure
  preserves the broken claim and partial terminals, without rollback or replay.
- slow login/trust can outlive the first ordinary sample and lose seeded intent;
  another loss then restores a shell until discovery rearms that pane.
- there is no readiness supervisor, outcome ledger or automatic resume retry.
- the 30 s sampling gap and failed-save intervals remain: recovery can omit recent
  work or resurrect a recently closed session and its saved conversation request.
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

## cutover and qualification

schema 2 only: no migration, dual reader or old restore signature. while live tmux
survives, stop the old gateway, retain its checkpoint separately, start the new
release with an empty checkpoint path and verify a durable schema-2 baseline.
if only the old recipe survives, recover with the old release first. never discard
the sole recipe to finish an upgrade. this operational cutover needs current-turn
authority; scope approval is not deployment approval.

qualification uses temporary public-host/native-tmux/native-provider checks with
independent registry or fixture-only visible-id oracles. prove original failure,
changed success, review/refactor, rerun affected checks, then delete tests. keep no
production seam or new gate. macos and linux isolated runs precede separately
authorized real-account, deployed-client and reboot acceptance.

| requirement | observable acceptance |
| --- | --- |
| lifecycle | claude clear/resume/branch and codex new/resume save current selection; failed/new samples retain no stale id |
| discovery | full/clipped/unicode titles, renames, cwd/recency ranking, ties and separate homes yield the specified result; incomplete/corrupt/over-limit candidate sets never silently select |
| workspace | active/inactive, linked windows, native groups, ordinary/remote panes retain graph semantics; only eligible final unique panes launch |
| exact recovery | destroy private tmux, restart gateway and separately codex daemon; saved ids reopen dummy history and continue those same transcripts |
| launch | bash/zsh aliases/functions receive sole saved `$1`; current edits apply; missing/changed profiles, duplicate targets and invalid ids cause no fresh or alternate launch |
| concurrency/durability | overlapping structural saves and bounded slow reads do not deadlock or overwrite newer recipes; source replacement, uncertain claim, partial graph and final-save failure never redispatch |
| content | requested/inferred/skipped copy agrees with startup intent and native errors; healthy topology still saves; notice lifetime matches contract |
| cutover | schema 1 rejected, fresh schema 2 armed, obsolete paths/tests removed, build/static/link checks pass |
| live | deployed macos/linux reopen saved ids after loss and reboot; phone/desktop show fresh notices; mid-turn loss preserves history without skid input replay, not resumed computation |

no host/device/live boundary means `NOT_RUN`, never pass. outstanding qualification
and evidence live in [conversation recovery acceptance](issues/conversation-recovery-acceptance.md).

provider contracts were probed on codex 0.163.0 and claude 2.1.310. primary references:
[codex index](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/rollout/src/session_index.rs),
[rendered title](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/chatwidget/status_surfaces.rs),
[title fields](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/bottom_pane/title_setup.rs),
[title sanitizer](https://github.com/openai/codex/blob/rust-v0.163.0/codex-rs/tui/src/terminal_title.rs),
[claude registry](https://code.claude.com/docs/en/agent-view#list-sessions-as-json),
[claude metadata](https://github.com/anthropics/claude-agent-sdk-python/blob/main/src/claude_agent_sdk/_internal/sessions.py).
