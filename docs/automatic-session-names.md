# automatic session names and public handles

status: implementation plan; production unchanged. this is the target
contract for one canonical tmux/skid name, automatic naming, and short live
selectors. it supersedes conflicting naming/selector details in
[rename](session-renaming.md), [architecture](architecture.md), and
[native interaction](native-agent-observation.md) at coordinated cutover.
[testing policy](rules/testing.md) governs temporary tests and their deletion.

## outcome and scope

- `session_name` is the only session name. cards, terminal headers, desktop
  tables and tmux display that actual name; there is no display alias.
- a supplied creation name is manual. an omitted name, including new terminal
  here and desktop quick creation, selects automatic naming. existing/unmarked
  sessions remain manual, even when their names resemble generated defaults.
- automatic naming follows the active pane's `pane_title` in the session's
  current window. a pane switch changes the source, never the session identity.
- saving a name selects manual ownership. `use automatic title` restores
  automatic ownership. an observed external tmux rename relinquishes automation.
- terminal and native conversation identities remain separate. terminal titles
  never establish agent identity, conversation binding, work state or unread.
- orchestration uses short typed handles or existing exact references. session
  names cease to be selectors. existing conversation controls and creation
  association retain their semantics; manual linking is removed and no
  `tracking: <name>` is added.

non-goals: provider title reads, transcript parsing, model-generated skid names,
name synchronization into providers, title/status parsing, progress, notifications,
new hooks, polling services, registries, copied history, new terminal transport,
new dashboard actions, provider discovery, deployment or publication.

## names and content

retain the current manual grammar: `[A-Za-z0-9][A-Za-z0-9_-]{0,63}`.
validate manual input; never silently rewrite it. the server owns automatic
conversion: accept a complete valid utf-8 title of at most 4,096 bytes without
`unicode.IsControl`, `unicode.Bidi_Control`, U+2028 or U+2029 characters;
lowercase ascii letters; replace each maximal run
outside `[a-z0-9]` with one hyphen; trim hyphens; truncate to 64 ascii characters
and trim the resulting trailing hyphen. no provider-specific cleaning.

empty, invalid, unavailable or non-ascii-only titles cause **no update**. keep
the current actual name and automatic mode. initial automatic sessions use the
existing smallest-free generated name until a usable title is observed.

tmux owns uniqueness. preserve the current name when it exactly equals the base
or a reconstructed permitted suffix candidate for this title and session.
otherwise choose the base if
free, then `<base>-s<session-number>`, then append `-2`, `-3`, etc., choosing the
smallest free candidate from the current inventory. shorten the base to keep
the complete candidate within 64 characters, then trim its trailing hyphen.
candidate numbers are canonical decimal; the session number comes from `$id`.
retaining an already assigned suffix prevents renaming when a competitor closes.
an external destination race defers an automatic update to the next inventory;
it never steals a name or retries a dispatched explicit mutation.

the content designer owns these literals and their phone/desktop placement:

| surface | content |
| --- | --- |
| rename sheet / field / primary | `rename session` / `session name` / `save name` |
| rename helper | `saving a name stops automatic naming.` |
| manual mode secondary action | `use automatic title` |
| automatic mode description | `follows the active pane's terminal title.` |
| forge helper | `leave blank to follow the terminal title.` |
| invalid manual name | `use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number.` |
| occupied name | `another session on this machine uses that name.` |
| naming conflict | `the session name changed. review and save again.` |
| uncertain write | `name change outcome unknown. checking tmux.` |

good content states the effect and names the actual target. retain the existing
two-line card name, ellipsis, full accessible label, literal machine context,
48dp controls and inline errors. no second session name, success toast, new icon
or announcement on every title tick. retain existing native conversation-id/status
disclosure; it describes a separate control target, not another session name.

## ownership and state

```text
provider or shell -> tmux pane_title
client inventory -> gateway -> sessions.Manager -> guarded tmux naming write
                            -> authoritative name/mode -> existing clients
cli handle -> fleetclient fresh inventory -> captured full reference -> gateway
```

one session-local option, `@skid_auto_name_b64`, contains canonical unpadded
base64url of the exact last name accepted by automatic naming. this is ownership
state, not a second public name or a provenance/history system. reserve this
private key exclusively for skid session options; definitions at server, global,
window or pane scope are unsupported. derive ownership from local reads without
`-A`, never inherited format lookup; add no scope scanner. absent/malformed
metadata means manual. a valid marker matching
the current name means automatic; a mismatch means manual and is conditionally
unset. explicit reset replaces absent/malformed metadata with the current name.

`sessions.Manager` owns conversion, candidate selection and transitions.
`internal/tmux` owns exact literal encoding, predicates and command queues.
`fleetclient` owns public handles. clients own draft/focus and reconciliation.
reuse the existing mutation lock and `Manager.List` normalization boundary;
provider enrichment remains outside that lock. no second observer or clock.

each automatic inventory pass:

1. capture lifetime, current name, local marker, current active pane and its
   complete title. read title separately; never delimiter-split arbitrary text.
2. derive effective mode and at most one desired update per session.
3. guard the write in tmux by lifetime/id, current name and exact marker;
   automatic renames additionally guard active pane id and full sampled title.
4. rename first, then update the marker, in the same synchronous guarded queue.
   manual takeover renames first, then removes the marker. reset records the
   current name and may leave renaming to the following inventory pass. encode
   that actual name to base64url in go; insert only the encoded marker token.
5. reread actual name/mode before projection. never synthesize a renamed card.

the queue serializes commands; it is not a rollback transaction. interruption
after rename but before marker update leaves a mismatch, conservatively manual
on observation. preserve unknown outcomes; never replay to finish the write.
an automatic predicate loss causes a fresh observation, not an unguarded retry.
optional title-read failure leaves the session visible; failure to obtain required
canonical name/lifetime facts retains existing machine-inventory failure behavior.

reuse and qualify `formatLiteral` at its existing owner. arbitrary titles need
literal format expressions, including tmux's `#[...]` special case; the current
bare escaping is insufficient. use escaped `#{l:...}` expressions and prove
exact comparison on supported tmux versions before adopting it. no shell, raw
format interpolation, truncated-title predicate or regex screen parser.
[qualification issue](issues/tmux-name-literal-comparison.md) records the evidence.

## capability and api contract

inventory/create retain `tmuxName` and add required
`nameMode: "automatic" | "manual"`. expose no raw title or marker.
replace the rename body at the existing route:

```text
PATCH /v1/sessions/{tmuxId}
{
  identityToken: string,
  expectedNaming: {mode:"automatic"} | {mode:"manual", name:string},
  naming:        {mode:"automatic"} | {mode:"manual", name:string}
}
```

these are closed union shapes: automatic forbids `name`; manual requires it.
reject unknown/duplicate/null keys and old bodies through the existing strict
decoder. expected manual name is the exact observed native name, including
external names outside skid's input grammar; only the new manual name uses
ascii64 validation. retain authentication, pinned machine and body limits.

- expected automatic accepts newer automatic names on the same lifetime;
  the user must not race a title animation. it rejects external/manual takeover.
- expected manual requires matching current manual ownership and exact name.
- fresh read plus lifetime/name/marker comparison and mutation share the
  existing manager lock and guarded tmux boundary. a preflight read is not a lock.
- success/no-op returns `204`, then the existing awaited inventory read. saving
  the same name in automatic mode is a real transition to manual.
- reuse name-invalid, name-conflict, not-found and identity-mismatch errors.
  add `SessionNameChanged` (`409`) for changed naming ownership/manual text.
  never classify a changed lifetime as a mere name conflict.
- transport/internal failure may have committed: report unknown, reread, never
  resend. definite naming conflict preserves the draft and obtains fresh state.

remove mutable names from attachment and closure authority. hard-cut terminal
`DELETE` to `{identityToken}`; remove `SourceName` from tmux attachment startup
and use the existing server/session-lifetime predicate. process-sensitive actions
retain their exact pane/pid/start guards. destructive confirmation captures the
exact lifetime and optional conversation/turn; a later rename changes neither.

## launch integration

preserve original `OptionalTmuxName` through preflight; keep the selected initial
tmux name in a separate internal prepared field. do not infer manual intent
from a generated string. create the automatic marker in the existing creation
queue before returning the session. supplied names have no automatic marker.

only an explicitly supplied name may seed the native provider name once.
claude omits `--name` otherwise. codex's private helper create input becomes
`{cwd, bypassPermissions, name?}`: omitted name skips `thread/name/set`, but
**still performs exact `thread/resume` preparation**. preserve permissions,
captured conversation ids and partial-failure semantics. reject empty/null name.
update the pinned `llm-calling` helper and skid together; no old-helper fallback.
later tmux changes never rename a provider or alter provider configuration.
some providers may still emit generic titles; useful titles are not guaranteed.

## short handles and orchestration

short handles are live-inventory abbreviations. existing full `--ref` values
remain exact captured targets for saved automation; existing direct native
`--conversation ID --profile PROFILE --machine HOST` remains for detached history.
these have distinct purposes; none falls back to name lookup.

`fleetclient` derives `t-` or `c-` plus 16 lowercase hex characters: the first
eight bytes of sha256 over the following ordered compact json string array,
with html escaping disabled and no trailing newline:

```text
t: ["skid-terminal-handle", machine, tmuxId, identityToken]
c: ["skid-conversation-handle", machine, provider, profileKey, historyScope, conversationId]
```

hash no name, cwd, pane, process, runtime status or turn. add `terminalHandle`
and optional `conversationHandle` to cli/tui session projection and successful
list/info/start output; retain `ref`. derive them locally from validated identities;
no gateway/phone handle fields, persistence, index or new reference codec.

replace positional `NAME` selection with exact handle grammar. retain `Name`
only for optional `start [NAME]`; add `Handle` for selection.

| target | public commands |
| --- | --- |
| terminal handle | `info`, `enter`, `text`, `keys`, `shell`, `group`, `close` |
| conversation handle | `read`, `replies`, `send`, `wait`, `stop` |
| terminal handle, explicit terminal mode | `read --terminal`, `stop --terminal` |

`info t-...` returns both handles; `info c-...` is invalid. inspection remains
internal; no new public command. retain full-reference/direct-native admission.
`close t-...` retains existing combined stop/close behavior by capturing the
current binding once; `--terminal-only` captures only the terminal.

resolve once against fresh complete inventory of `--machine HOST`, or all
configured peers if omitted; ignore group filters. derive conversations only
from current `Session.conversation`, deduplicate their full identity tuples,
then match handles. a conversation match constructs a conversation-only existing
reference, never an arbitrarily selected terminal. commands and waits retain
that full target after resolution; later rename/rebinding never retargets them.

expose the existing resolver as
`Client.Capture(ctx, Request) (Reference, *Failure)`: validate a target-bearing
request, apply the existing timeout and delegate to `resolve`; reject list/start.
explicit native `read` captures its request once, then reads using only the
returned `Ref`, scope and byte limit. the reference owns machine selection,
even without `--machine`; never reconstruct it from a label or route `c-...`
through `info`. retain exact-ref/direct-native admission. native reads have no
notification effect; [terminal attention](reply-notifications.md) owns that policy
and removes `replies` and human read receipts.

malformed/wrong-kind/old-name selectors return `invalid_input`; absent match
`handle_not_found`; distinct full identities sharing a handle `handle_ambiguous`;
incomplete selected inventory `inventory_incomplete`; unknown machine
`machine_unknown`. all are `not_sent`. preserve exact-target failure and uncertain
dispatch semantics after capture. no automatic re-resolution/replay.

handles disappear when their target leaves inventory; a conversation may remain
accessible by exact ref/direct native id. 64-bit historical collisions cannot be
detected without retained history: do not promise permanent identity for short
handles. saved automation requiring replacement rejection uses full references.

## client composition

retain `editing -> sending -> reconciling`, per-machine mutation fences and
bodyless-response handling. capture the draft and `expectedNaming` once.
automatic-to-automatic title updates preserve that expectation. an observed
manual-name or ownership change shows the conflict message and retains the draft;
after fresh inventory, rebase the expectation and allow deliberate resubmission.
inventory updates preserve cursor, focus, editor state and attachment attempt/webview.
request keyboard focus on opening the editor, never on a target-title update.
same-name `save name` is enabled in automatic mode; unchanged manual input stays
disabled. `use automatic title` is available in manual mode, bypasses draft-name
validation, and never runs on sheet opening/cancellation. closing the sheet while
reconciling retains the unresolved operation, including automatic reset.

manual success reconciles by lifetime plus manual mode and desired name;
automatic success by lifetime plus automatic mode, not by a predicted title.
a later writer wins; show fresh truth, retain the draft for review, never replay.
update same-lifetime terminal headers from inventory; remove name equality from
phone attachment admission/reconnect. keep existing stale-inventory restrictions.

replace name sorting with numeric tmux session-id order inside existing
machine/group order. retain explicit desktop attention ordering and lifetime
keys/selection/restoration. name changes alone never move a card or cursor.
show handles in cli lists/start/info and desktop details, not another phone row.
update help/return-address examples to use handles or exact refs.

## decisions and costs

| choice | accepted cost |
| --- | --- |
| one actual tmux name | terminal title changes can rename a working session |
| ascii64 conversion and stable suffixes | titles lose punctuation/unicode and may be shortened |
| existing inventory owns updates | names stop following while no client inventories the host; no background freshness promise |
| marker records current ownership only | external rename-away-and-back between samples is undetectable |
| preserve old/unmarked sessions as manual | existing generated names require explicit automatic selection |
| stable session-id order | alphabetical session-name ordering is removed |
| short live selectors plus existing exact refs | short selectors need complete scoped inventory and have the stated collision limit |
| optional native launch name | coordinated helper change and empty-thread adoption qualification are required |

## implementation slices and adversarial review

root owns this spec and canonical documentation. builders own disjoint paths;
interface amendments return to root. each builder reviews its contract before
coding, observes a behavioral red, implements green, then obtains independent
adversarial review before refactoring. the reviewer writes no production/test
file. the content designer owns the table above and checks rendered meaning,
truncation, focus and action copy for both naming and handle features.

| order / owner | exclusive implementation paths | acceptance focus |
| --- | --- | --- |
| 1 native launch | `internal/agentruntime/{launch,profile}.go`, `internal/agentcontrol/create.go`; separate pinned `llm-calling` checkout: `src/provider_runtime/agent_runtime/{codex_control,native_control_cli}.py` | omitted name, explicit name, empty-thread adoption, permissions and partial outcomes |
| 2 host naming and identity | `internal/sessions/`, `internal/tmux/`, `internal/gateway/{dto,gateway}.go`, `internal/logging/logger.go` | ownership races, literal boundaries, names, collision handling, name-independent attach/close |
| 3 desktop and handles | `internal/fleetclient/`, `internal/agentcli/run.go`, `internal/sessionui/` | typed selectors, exact refs, complete scope, capture once, stable ordering/content |
| 4 android and content | `android/app/src/main/java/dev/niels/skidbladnir/{ProductModel,GatewayClient,SkidbladnirController,SessionRename,ForgeSheet,SessionCard,TerminalScreen,WorkingDirectoryPicker}.kt` | strict schema/exhaustive error handling, rename/reset, reconciliation, focus/attachment/order |
| 5 root integrator | `deployment/native-control/pin.json`, affected `docs/`; no check-composition change | coordinated cutover, stale contract removal, reviewed evidence |

orders 3 and 4 may proceed independently after the host schema is fixed. native
and host owners agree the prepared-creation seam before editing. temporary tests
belong to their builder's slice (or its isolated temporary directory), are never
shared between builders, and are removed before commit. read the separate helper
repository's instructions before implementing its slice.

reuse lifetime predicates, local-option reads, format encoding, rename queue,
mutation lock, strict dto/error mapping, group metadata encoding patterns,
creation queue, exact references and ui mutation lanes. add focused naming and
handle modules only where they absorb the two concrete responsibilities.
remove old name lookup/errors/help, unconditional same-name conflict, obsolete
name-as-authority checks, duplicate rename paths and synthetic provider-name
seeding. do not copy the old rename implementation beside a new path.

names can now contain transformed terminal content: remove session-name fields
from create/kill logging and their event constructors. no name/title/marker,
prompt, terminal bytes, provider/account data or credentials enter evidence/logs.

## acceptance and red green refactor

use temporary tests against real boundaries; never recreate retired gates or
claim builds as behavioral acceptance. executing tmux/live/provider/phone work
requires the applicable current-turn authorization in `AGENTS.md`; this planning
turn runs none. every unexecuted boundary remains `NOT_RUN`.

1. on both supported host platforms, an authenticated gateway plus isolated
   `tmux -L` and disposable title emitter proves automatic creation, normalized
   title adoption, pane switching, missing/invalid title, persistent mode across
   gateway restart, and preservation of old/unmarked names.
2. the same journey proves manual takeover, same-name freeze, reset without a
   current title, external rename, manual/automatic races, collision/suffix
   stability, stale lifetime/id reuse and strict rejection of old request shapes.
   separately exercise the shared literal helper against a temporary tmux option
   containing `#[red]`, `#{session_name}`, `#(...)`, commas, braces, quotes and
   newlines. controls cannot qualify through the title path; print no payloads.
3. retain a real attachment while names change. attach/reconnect/close address
   the exact lifetime; controls never affect another session. construct the
   post-rename/pre-marker residual state on the isolated socket, then prove
   conservative ownership recovery. this proves recovery from that state, not
   mid-queue crash injection. separately lose a response and reconcile without resend.
4. a cli/gateway journey proves stable typed handles across rename, replacement
   rejection with exact refs, complete-scope admission, duplicate conversation
   binding deduplication, wrong-kind/name rejection and captured-target behavior
   across rebinding, including `replies c-...`. review the collision branch and
   use one temporary resolver fixture for forced ambiguity; do not brute-force hashes or add production
   test hooks. native saved targets remain usable after terminal closure.
5. qualify actual managed codex/claude launch: omitted name remains unseeded,
   explicit name is preserved, codex empty-thread terminal adoption works without
   a fabricated turn. record whether each provider emits a useful title; lack of
   emission is a documented capability limit, not a naming failure. preserve
   permission policy and partial-created-id reporting. use only disposable test
   sessions; existing user sessions/accounts are not cleanup targets.
6. one separately approved physical-phone journey proves unchanged-name freeze,
   automatic reset, unknown-outcome reconciliation, strict schema, same-lifetime
   reconnect, uninterrupted terminal input, stable card position and editor focus.
   the designer checks small-width/large-text copy and accessible controls.

before requesting device approval, root supplies a concrete execution sheet:
isolated gateway/socket and reachable endpoint, candidate apk/config, response-loss
injection outside production code, exact install and restoration commands. use
the changed gateway schema; no unchanged-host fixture may count as phone acceptance.

observe relevant reds on the original implementation, then greens on changed
source. refactor only against concrete duplication/rule violations; rerun affected
proofs and `scripts/check verify`. delete temporary tests/harness files before
commit, run engineering checks again, and retain only content-free outcomes tied
to the tested source. no retained behavioral-regression protection is claimed.

hard-cut gateway, clients and helper together. reject old rename/delete schemas
and old name selectors; keep no version branch, migration, dual reader or fallback.
at implementation completion reconcile `session-renaming.md`, architecture,
roadmap, agent-control docs, native observation, desktop/browser and design-language
contracts; remove superseded naming/selector acceptance text. whole-release
rollback remains external to this feature and requires no compatibility code.
