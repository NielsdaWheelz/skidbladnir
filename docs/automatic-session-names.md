# automatic session names and public handles

status: implemented and verified in the isolated worktree. source cutover is
complete; temporary tests/harnesses are removed. deployment is outside this slice.
this is the contract for one canonical tmux/skid name, automatic naming and short
live selectors. canonical documents use this contract. 2026-09-29 accepted
amendment: skid never names provider conversations; codex starts a normal new
remote conversation and remains unassociated. manual association is removed;
existing bindings and direct native commands remain available. physical
phone acceptance passed on the separately approved physical device.
[testing policy](rules/testing.md) governs temporary tests and their deletion.

## outcome and scope

- `session_name` is the only session name. cards, terminal headers, desktop
  tables and tmux display that actual name; there is no display alias.
- names flow from the running program's terminal title into skid. skid changes
  only the tmux session name; it never sets pane titles, process/tui names or
  provider conversation names.
- a supplied creation name is manual. an omitted name, including new terminal
  here and desktop quick creation, selects automatic naming. existing/unmarked
  sessions remain manual, even when their names resemble generated defaults.
- automatic naming follows the active pane's `pane_title` in the session's
  current window. a pane switch changes the source, never the session identity.
- saving a name selects manual ownership. `use automatic title` restores
  automatic ownership. an observed external tmux rename relinquishes automation.
- terminal and native conversation identities remain separate. terminal titles
  never establish agent identity, conversation binding, work state or terminal attention.
- orchestration uses short typed handles or existing exact references. session
  names cease to be selectors. existing conversation controls and explicit
  bindings retain their semantics; manual association is unavailable and no
  `tracking: <name>` is added.

non-goals: direct provider-name apis, transcript parsing, model-generated skid names,
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
or announcement on every title tick. retain recorded native conversation-id
disclosure; it describes a separate control target, not another session name.
the terminal header separates context/navigation from actions into two rows;
close text cannot consume the weighted name control's space. existing viewport
measurement absorbs the height; do not force a terminal size or shrink text.

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
bare escaping is insufficient. use escaped `#{l:...}` expressions. request tmux utf-8 output explicitly with
`-u`; otherwise tmux sanitizes control/non-ascii bytes under a non-utf-8 locale
before skid can validate or compare them. no shell, raw
format interpolation, truncated-title predicate or regex screen parser.
qualification below records the evidence.

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

skid never names either provider, including when the user supplies a manual skid
name. claude never receives a skid-generated `--name`. keep the existing ban on
configured claude name flags. codex preparation starts only the existing owning
daemon; its remote terminal interface starts the new conversation itself, with
no reserved id or `resume` argument. preserve cwd, account, configured permission
arguments and owner; stock tui owns its ordinary no-override startup policy.
remove skid's native conversation creation, partial-created-id error fields and
recovery disclosure;
there is no helper create request or helper-pin change in this slice.

new codex sessions remain unassociated; manual tracking actions are removed.
existing tracked conversations and
saved exact references retain their contracts. claude's existing process-bound
identity registration remains its observation path. titles never supply either
binding. no new hook or native-id capture machinery is added.

normal session creation still preserves uncertain terminal outcomes without
replay. later tmux changes never rename a provider or alter its configuration.
some providers emit generic titles; useful titles are not guaranteed.

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
| terminal handle | `info`, `enter`, `read`, `send`, `wait`, `text`, `keys`, `stop`, `shell`, `group`, `close` |
| conversation handle | explicit native `read`, `send`, `wait`, `stop` |

`info t-...` returns both handles; `info c-...` is invalid.
`inspect --ref` observes a captured native conversation. retain full-reference/
direct-native admission. terminal handles select terminal operations without a
mode flag; close attempts interruption then independently closes the session.
`--terminal-only` skips interruption. human `replies` and the viewer are removed.

resolve once against fresh complete inventory of `--machine HOST`, or all
configured peers if omitted; ignore group filters. derive conversations only
from current `Session.conversation`, deduplicate their full identity tuples,
then match handles. a conversation match constructs a conversation-only existing
reference, never an arbitrarily selected terminal. commands and waits retain
that full target after resolution; later rename/rebinding never retargets them.

reuse the existing resolver. a conversation match captures its metadata identity,
then explicit native inspect obtains runtime/turn; it never reinterprets failure
as terminal input. no native status/history calls during ordinary inventory.

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

the desktop browser edits names with `r` inside its spacebar info page;
`ctrl-a` in the manual name editor restores automatic ownership. save/cancel
returns to that captured info. [desktop browser](desktop-browser.md#4-actions-and-return)
owns the contextual keys, regrouping behavior and post-write return contract.

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
| separate terminal context and action rows | at least 48dp of additional header height; large action labels may wrap, and existing viewport sizing absorbs the space |
| short live selectors plus existing exact refs | short selectors need complete scoped inventory and have the stated collision limit |
| provider-owned conversation names | new codex sessions have no native card binding; useful terminal titles remain provider-dependent |
| ordinary remote codex startup | without configured overrides, stock tui policy is read-only rather than the former private pre-creation default; configured permission arguments remain intact |

## implementation slices and adversarial review

root owns this spec and canonical documentation. builders own disjoint paths;
interface amendments return to root; accepted amendments are stated here. each builder reviews its contract before
coding, observes a behavioral red, implements green, then obtains independent
adversarial review before refactoring. the reviewer writes no production/test
file. the content designer owns the table above and checks rendered meaning,
truncation, focus and action copy for both naming and handle features.

| order / owner | exclusive implementation paths | acceptance focus |
| --- | --- | --- |
| 1 native launch | `internal/agentruntime/{launch,profile}.go`, `internal/agentcontrol/{create,native}.go` | no provider naming, new remote codex launch, permissions, native-control preservation |
| 2 host naming and identity | `internal/sessions/`, `internal/tmux/`, `internal/gateway/{dto,gateway}.go`, `internal/logging/logger.go` | ownership races, literal boundaries, names, collision handling, name-independent attach/close |
| 3 desktop and handles | `internal/fleetclient/`, `internal/agentcli/run.go`, `internal/sessionui/` | typed selectors, exact refs, complete scope, capture once, stable ordering/content |
| 4 android and content | `android/app/src/main/java/dev/niels/skidbladnir/{ProductModel,GatewayClient,SkidbladnirController,SessionRename,ForgeSheet,DashboardScreen,SessionCard,TerminalScreen,WorkingDirectoryPicker}.kt` | strict schema/exhaustive error handling, rename/reset, reconciliation, focus/attachment/order; remove obsolete partial-creation disclosure |
| 5 root integrator | affected `docs/`; helper pin unchanged; no check-composition change | coordinated cutover, stale contract removal, reviewed evidence |

orders 3 and 4 may proceed independently after the host schema is fixed. native
and host owners agree the prepared-creation seam before editing. temporary tests
belong to their builder's slice (or its isolated temporary directory), are never
shared between builders, and are removed before commit. the existing helper protocol and pin remain unchanged.

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
requires the applicable current-turn authorization in `AGENTS.md`. every unexecuted boundary remains `NOT_RUN`.

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
   across rebinding, including explicit native `read c-...`. review the collision branch and
   use one temporary resolver fixture for forced ambiguity; do not brute-force hashes or add production
   test hooks. native saved targets remain usable after terminal closure.
5. qualify actual managed codex/claude launch: neither omitted nor supplied skid
   names reach a provider naming operation. codex starts its own zero-turn remote
   conversation, remains unassociated, and accepts the first terminal input. direct
   native controls retain exact-id targeting independently of the card. record
   whether each provider emits a useful title; lack of emission is a documented
   capability limit, not a naming failure. preserve cwd/account/permission policy. use only disposable test
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

hard-cut gateway and clients together; retain the existing helper pin/protocol. reject old rename/delete schemas
and old name selectors; keep no version branch, migration, dual reader or fallback.
at implementation completion reconcile `session-renaming.md`, architecture,
roadmap, agent-control docs, native observation, desktop/browser and design-language
contracts; remove superseded naming/selector acceptance text. whole-release
rollback remains external to this feature and requires no compatibility code.

## implementation evidence — 2026-09-29

branch: `codex/automatic-session-names`; baseline `edf85f8`. helper pin remains
`992e7915caf1111ffad3a82d6593a1c8673dcf1f`; helper source/protocol is unchanged.
no deployment/publication or existing-session mutation is part of this work.
the recorded association check below predates manual-linking removal; it is
historical evidence, not a supported current action.

| boundary | observed outcome |
| --- | --- |
| real authenticated gateway, isolated tmux/pty, darwin + linux | original implementation red; candidate automatic/manual/reset, panes, collision retention, restart, external rename, stale lifetime, residual ownership and lost-response/no-replay journeys green |
| exact tmux literal/guard boundary, tmux 3.7c on both platforms | format/style-looking values, punctuation, interior newline/unicode, positive/negative equality, stale guards and destination races green; explicit `LC_ALL=C` caught output sanitization and passes with `-u` |
| desktop cli/fleetclient over real loopback HTTP/TLS | handles/capture/rebinding, complete scope, closed mode schema, numeric order, prior claude reply recovery and definite creation errors green; isolated forced-hash fixture proves ambiguity/dedup without a production hook |
| android production-source JVM harness | same-name takeover, invalid-draft reset, fresh-state conflict/rebase, pending dismissal, strict creation errors and attachment/order transitions green (39 assertions); a definite rejection keeps the editor even when another writer matches its draft |
| actual managed codex 0.159.1 | real gateway remote-new startup creates one unnamed zero-turn conversation; first terminal input, explicit association, bound native read, identity-only close and saved native read afterward green; configured `--yolo` survives |
| actual pinned claude 2.1.284 | explicit/omitted skid names never reach provider argv; real interactive input/response green |
| actual provider title emission | both providers replace a controlled pane-title sentinel; sleep respawn preserves it. settled titles stayed unchanged after the test turn; codex native name remained absent and claude's native observation contract exposes no name |
| engineering | `scripts/check verify` passed after all temporary tests/harnesses were removed; go build and focused vet passed on the qualified host source |
| physical phone | both real companion-package tests green: naming/focus/order, lost acknowledgement/no replay, WSS/input/reconnect and identity-only close; 1.3× text, long-title rename/reset, positive visual bounds, 48dp touch bounds and button roles pass; packages/forwarding removed and exact font scale restored |
| cleanup | temporary tests, source copies, companion artifacts/config/keys and isolated fixtures removed; owned sessions and native daemon processes verified absent; existing user sessions/provider homes/app data untouched |

host reds ran against the baseline before implementation. desktop loopback HTTP
qualification also reran archived original source after implementation to confirm
sensitivity; creation-error repair observed a fresh red before its fix. literal
qualification caught a real linux failure, reproduced it under the C locale on
darwin, then verified the responsible shared-output repair on both platforms.
recovery evidence constructs the post-rename/pre-marker state; it does not claim
mid-queue crash injection. tmux evidence qualifies 3.7c, not every tmux release. the additional actual
gateway/cli composition uses only process-local fixture certificate roots through
a temporary launcher; it does not qualify unchanged-executable macos certificate
trust. no-override codex startup follows stock tui policy (observed read-only),
rather than the former private thread/start default (workspace-write); configured
production `--yolo` still gives `never`/`dangerFullAccess`.
physical qualification exposed and repaired zero-width name controls caused by
the single-row header's unbounded close label. separate context/actions rows
passed both physical journeys afterward. touch-size assertions use compose's
actual touch bounds; material button layout bounds can be smaller.
phone companion isolation does not qualify installed pairing migration or full
three-peer reachability; large-font geometry/semantics is not human readability
or full screen-reader navigation. no retained behavioral protection is promised.
