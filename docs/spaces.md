# spaces

2026-09-15: accepted implementation target for pr 1 of
[spaces-and-shells.md](spaces-and-shells.md). the product decisions and the
restoration/creation edge cases are approved. source implementation and owner
behavioral reds are complete on `spaces-pr1`. current verification and the
remaining runtime boundaries are recorded in [roadmap.md](roadmap.md#spaces--source-implemented-runtime-acceptance-open).
v0.5.0 is deployed to the three hosts and s22+. corrected darwin runtime and the
real phone membership/filter/restoration journey pass. the complete released
platform gate remains failed; the narrow-card fix passes a separate development
component proof and remains unreleased. hands-on acceptance is `NOT_RUN`.
[current delivery evidence](roadmap.md#v050-deployment-and-runtime-acceptance)
and the [remaining deployed card defect](issues/spaces-card-large-text.md) retain
those boundaries.

[architecture.md](architecture.md) incorporates this scope and acceptance;
[roadmap.md](roadmap.md) owns delivery. this document owns the detailed spaces
contract. [agent-control.md](agent-control.md) and
[agent-control-ux.md](agent-control-ux.md) still own provider controls, references,
transport, and attachment. their current status and machine/name ordering rules
supersede older activity-only descriptions. follow [../AGENTS.md](../AGENTS.md),
[rules/index.md](rules/index.md), and [rules/testing.md](rules/testing.md).

## 1. outcome, scope, and final state

help the operator find related sessions across the fleet without changing their
execution or control identity. **space** is the product term. its durable model
is one optional validated label on each tmux session. clients collect equal
labels into groups; there is no space resource to create, rename, or delete.

pr 1 delivers inventory projection, initial assignment during creation,
set/change/clear on existing sessions, grouped cli/tui/phone collections,
intersecting machine/space filters, and collection return continuity. ordinary
shell-only sessions already in inventory participate in session operations.
creating a new ordinary shell belongs to pr 2. additional agent/shell terminal
switching and desktop composition belong to pr 3. add no scaffolding for either.

```text
tmux session option                         authoritative membership
  -> sessions                               local facts and exact mutation
  -> gateway                                strict authenticated host api
       -> fleetclient -> cli / tui          fleet observations and presentation
       -> android model / controller        phone observations and operations
            -> dashboard entry              filters and semantic viewport
            -> dashboard / forge            grouping and explicit editing
```

there is no new runtime service, database, poller, provider helper, configuration,
permission, launch profile, or external account dependency. membership never
supplies a cwd, checkout, profile, machine, prompt, or agent context.

## 2. invariants and capability contract

1. a session belongs to zero or one space. equal canonical labels group across
   machines, with exact case-sensitive equality. the machine remains part of
   every session/control identity.
2. membership lasts for the tmux session lifetime. session rename, cwd change,
   agent replacement, detach, and client/gateway restart preserve it. session
   destruction leaves no space record. grouped tmux sessions have independent
   membership; it never follows their shared windows.
3. assignment addresses `(machine, tmuxId, identityToken)`. it neither requires
   nor refreshes an agent reference. a retained reference remains usable after
   its agent changes; a replacement session rejects that reference.
4. classification never selects a pane/window, attaches, changes geometry,
   moves a process, creates a tmux group, sends input, interrupts, or stops work.
   names, dwarf identities, provider facts, and existing controls retain their
   contracts. labels never appear inside opaque references.
5. initial membership is part of the existing create queue. invalid input rejects
   before creation. retain existing partial-create recovery: later queue failure
   may leave a visible session; never compensate with an unproven kill or a
   client follow-up assignment.
6. writes are absolute assignments, with last applied write semantics at tmux.
   no expected old label, compare-and-swap, version, or fleet transaction exists.
   setting the same label or clearing an unassigned session succeeds after
   lifetime validation. acknowledgement confirms that write at its command
   boundary, not that another writer cannot immediately change it.
7. each mutation is attempted once. uncertain transport/command completion stays
   uncertain. inventory can reveal current membership; it cannot prove which
   uncertain writer caused it. no automatic resend, rollback, or retarget.
8. groups, suggestions, and emptiness derive from observed inventory. unavailable
   hosts and stale rows stay explicit. a zero-row intersection never proves
   fleet-wide absence. neither a filter nor a suggestion is action authority.
9. editing retains filters. selection and pending actions follow session
   lifetimes, never row positions. terminal entry and detach/back retain both
   filters and semantic viewport. creation has the narrow exception in section 7.

## 3. labels, equality, and ordering

### canonical label

a named space is 1–64 unicode scalar values in unicode-15 nfc, at most 256 utf-8 bytes.
emoji and non-ascii letters are allowed. reject:

- invalid utf-8 or unpaired utf-16/escaped surrogates;
- c0/c1 controls: u+0000–001f and u+007f–009f;
- u+061c, u+200e–200f, u+2028–202e, and u+2066–2069, matching the phone/path
  display-safety predicate;
- leading or trailing u+0020;
- other unicode whitespace anywhere: u+00a0, u+1680, u+2000–200a, u+202f,
  u+205f, and u+3000. whitespace controls/separators are already rejected above.

interior ordinary spaces are preserved, including repeated spaces. equality has
no case folding, trimming, whitespace collapsing, fuzzy matching, transliteration,
or aliasing. `alpha`, `Alpha`, and `alpha  beta` remain distinct. `all spaces`,
`unassigned`, and `previously selected space` are valid labels, not special states.

human cli arguments and tui/phone drafts normalize to nfc at submission, before
validation. invalid drafts stay editable and are not truncated. api requests,
same-system responses, and externally written metadata accept only canonical
nfc; their readers never normalize. typing convenience stays at human ingress,
as [boundaries.md](rules/boundaries.md) requires.

nfc is pinned to unicode 15.0 in both clients; a runtime/toolchain upgrade must
not silently change equality. code points unassigned in that version remain
unchanged normalization boundaries, not rejected characters. go's table version
is checked by the label contract test. android uses the public platform icu
`UnicodeSet.span` over a frozen `[:age=15.0:]` set: normalize included spans with
`Normalizer2`, append excluded spans unchanged. keep this small loop private to
the label owner; the platform's internal `FilteredNormalizer2` is not a public
android sdk api. plain jvm-17 and android-36
normalizers use different unicode tables and cannot own this wire contract.

nfc means canonical normalization without stream-safe separator insertion. a
valid long combining-mark sequence remains exact text; an explicitly authored
u+034f remains present. go's whole-string `x/text/unicode/norm` normalizer inserts
that character after 30 non-starters, so the small label owner instead reuses its
per-scalar decomposition, combining classes, and pair composition. keep the
unicode tables in the dependency; do not copy tables or add a general text layer.

### one order

sort named labels by unsigned utf-8 bytes with ascii `A`–`Z` folded to `a`–`z`
for the primary comparison; break ties by original unsigned utf-8 bytes. shorter
equal prefixes sort first. this is the existing directory chooser's
`compareCaseInsensitiveUtf8` policy. do not use device locale, full unicode case
folding, or kotlin's default utf-16 order. equality still uses exact text.
place unassigned after every named group; omit empty headings.

within each group, retain the client's present order: cli/tui configured peer
order then host-published name/id order; android case-folded/exact machine label,
machine handle, case-folded/exact tmux name, then tmux id. no urgency sorting,
manual ordering, collapsing, nested groups, or completeness-implying counters.

### owned values

add one small pure go package `internal/space` for label parsing, human nfc draft
normalization, label comparison, and the closed all/unassigned/named filter.
`Label` has private text; its zero value means unassigned. parsing empty accepts
that absence, never a named label. the boundary enforces its required/optional
field rule before constructing the value. named filters require a nonempty label.
no service, registry, generic optional-value package, or persistence lives here.

go session facts and create/set inputs carry that owned value. gateway and
fleetclient decode their respective wire boundaries once; cli/tui share the
parser and comparison. android owns the equivalent nonempty `SpaceLabel` and
uses its existing nullable optional-session-field convention for unassigned.
one phone parser serves draft validation and wire acceptance; composables consume
accepted values. raw human drafts remain text until submission.

`fleetclient.Request` gains separate typed membership input and space-filter
fields: start uses membership or unassigned, space replaces membership including
clear, and list uses the filter. reject fields on unrelated operations. this is
the existing internal request, not a new wire protocol. raw dto string conversion
and omitted-field encoding happen only in boundary adapters; do not serialize a
private go label struct directly.

## 4. tmux storage and the exact write

option: **`@skid_space_b64`**, session-local only. assigned value: canonical
unpadded base64url of the label's nfc utf-8 bytes, maximum 342 ascii characters.
unassigned: the local option is absent. clear unsets it. no empty-option writer,
global default, pane/window option, alternate key, or versioned value is added.

reuse `sessionOption` / `show-options -qv -t <id>` without `-A` or `-g` so a
global option is not inherited. tmux's
[local-option lookup](https://github.com/tmux/tmux/blob/master/cmd-show-options.c)
uses the local value unless `-A` requests its parent. malformed, empty,
noncanonical base64url, oversized, invalid-text, or unreadable optional metadata
projects as unassigned. require decode/re-encode equality; a decoder that skips
newlines is insufficient. never repair metadata during reads. required session
or host observation failures retain their existing failure semantics.

read membership independently of the active pane and foreground process; do not
put it behind `enrichSession`'s missing-pane early return. retain server-identity
checks around inventory. no new snapshot atomicity or hostile same-user mutation
containment is claimed.

create validates membership with the other inputs before invoking tmux, then
sets the encoded option in the existing `new-session` queue alongside
profile/character/objective options, before its final identity observation.
unassigned creation emits no space option. successful creation reports the
actually observed session, not a fabricated echo over a missing observation.

existing-session assignment:

1. gateway authenticates, binds the machine, decodes the exact body, and parses
   the label. no label enters a url, raw shell string, or tmux format.
2. `sessions.Manager.SetSpace` takes the existing mutation lock and resolves the
   supplied id/token using session-lifetime validation. missing/replaced lifetime
   rejects. agent identity is irrelevant.
3. one `if-shell -F -t <id>` queue checks server epoch, server pid/start time,
   and session id, then sets/unsets only `@skid_space_b64` on that id. its
   predicate has no name, old-label, pane, or process condition. rename during
   assignment must not cause a stale-name/agent error.
4. a content-free success marker after set/unset establishes success. the existing
   mismatch marker establishes rejection without assignment. command error,
   unexpected output, or lost completion after possible dispatch means unknown.
   never retry the queue or infer dispatch from a later label.

factor the common lifetime terms from the tmux identity predicates in
`internal/tmux/client.go`. keep name-sensitive rename/kill/attachment predicates
as explicit wrappers over them, preserving behavior. reuse the lifetime-only
validation currently named `terminalIdentity` in `internal/sessions/attachment.go`,
giving it a session-oriented name and retaining existing callers. add one narrow
`SetSessionSpaceIfIdentity` operation, not an arbitrary-option setter. use existing
argv execution and encoded tokens for the nested tmux command; no shell or hook.

## 5. host api and projection

existing bearer, pinned `Skidbladnir-Machine`, path, content-type, strict-json,
deadline, size, and no-replay rules remain. optional response fields are omitted,
never `null`. there is no `/spaces` inventory or membership-query endpoint.

| boundary | exact addition / result |
| --- | --- |
| host session dto | optional `space: string`; present means canonical nonempty label |
| `GET /v1/sessions` | unchanged envelope/order; project `space?` per session |
| `POST /v1/sessions` | existing required `cwd, profile` and optional `optionalTmuxName, objective`, plus optional `space`; present empty is `SpaceInvalid`; null/wrong type is `InvalidRequest` |
| create success | existing `201 {observedAt,session}`, using the same session dto |
| `PUT /v1/sessions/{tmuxId}/space` | exactly `{identityToken: string, space: string}`; nonempty assigns, empty clears; both keys required |
| membership success | bodyless `204`; no new observation time, receipt, agent, or refreshed reference |
| fleetclient row | existing row plus optional `space`; opaque `ref` unchanged |
| cli `info` / `start` | existing observed-session envelope with `session.space?` |

the mutation's empty string is the explicit whole-property clear value. omission
does not mean clear; `null` is never accepted. reuse strict `stringField` without
a second nullable request grammar. reject extra, duplicate, mis-cased, absent,
wrongly typed fields and noncanonical paths before mutation. supplied `tmuxName`,
agent fields, or expected old space are extra fields, not alternate contracts.
route `/space` before generic session-path handling; reuse `parseSessionPath`
after removing the exact suffix.

response decoders must distinguish absent `space` from present empty/null/invalid
text; only absence means unassigned on inventory. a pointer decoder that silently
maps both null and omission to the same value is insufficient. reject malformed
same-system fields as protocol failure, rather than applying the tmux-invalid-
metadata omission rule after the transport boundary.

new domain error: `422 SpaceInvalid`, literal message
`use 1–64 nfc characters; only interior ordinary spaces, without display controls.`
add it to host/client/phone closed error mappings. never echo the invalid label.

| membership failure | status / dispatch |
| --- | --- |
| malformed body/path, empty token | `400 InvalidRequest`, `not_sent` |
| over existing body limit | `413 RequestTooLarge`, `not_sent` |
| invalid nonempty label | `422 SpaceInvalid`, `not_sent` |
| bearer / machine rejection | existing `401 Unauthenticated` / `409 MachineIdentityMismatch`, `not_sent` |
| original session absent before dispatch | `404 SessionNotFound`, `not_sent` |
| invalid/replaced lifetime or conditional rejection | `409 SessionIdentityMismatch`, `not_sent` |
| failed required observation before queue | `500 InternalError`, `not_sent` |
| uncertain command completion | `500 InternalError`, `unknown` |
| lost, malformed, oversized reply after possible dispatch | existing client failure classification, `unknown` |

reuse the api error's existing `dispatch` member for this route. the session
owner distinguishes pre-dispatch error from uncertain completion with one
content-free error classification; no receipt or mutation state store. a late
disappearance cannot relabel a possibly applied write `not_sent`. other routes
retain their existing error contracts.

`fleetclient.call` adds `PUT` and strict bodyless membership success through its
existing no-`GetBody`, no-redirect, bounded-read transport. android uses
`authorizedRequest` and `executeBodyless`, with its existing disabled retry policy.
http method idempotence does not authorize retry: repeating an assignment after
another writer acts would overwrite that newer intent.
add the normalized `/v1/sessions/{tmuxId}/space` log route, `PUT` method, and
`SpaceInvalid` code. no new label-bearing event or raw request/response logging.

retain bounds: host request and android response 64 kib; go control response
64 kib; per-peer/final go inventory 1 mib; existing 15-second client deadline.
filters do not evade an oversized source inventory: decode the bounded host
result first. do not increase bounds for this feature.

## 6. cli and shared fleet presentation

```text
skid list [--machine host] [--space label | --unassigned] [--json]
skid start name --machine host --profile profile [--cwd '~'] [--space label] [--json]
skid space name [--machine host] (--set label | --clear) [--json]
skid space --ref reference (--set label | --clear) [--json]
```

`--space` and `--set` require nonempty valid labels. exactly one of `--set` or
`--clear` is required for `space`. filter flags are invalid on info, enter,
agent operations, kill, or assignment. `--unassigned` is list-only. omission
means all spaces for list and unassigned for start. preserve current flag
ordering, `--flag=value`, `--` literal operands, duplicate rejection, and usage
exit 2. bare `skid` remains the tui; no initial-filter flags are added in this pr.

`space` reuses exact-name/machine/ref selection, timeout, and result envelopes.
unqualified names still require complete fleet uniqueness, regardless of space.
`--ref` routes directly by machine and retains supplied session identity; never
fetch a replacement agent or require `ref.agent`. assignment uses the new host
route, not `agent/{operation}` or the existing kill-name preparation.

after host acknowledgement, json is exactly
`{"ok":true,"result":{"space":"label"}}`, or the same shape with `"space":""`
for clear. this acknowledges the requested write, not a fresh inventory. no ref
is returned. normal text reports `space assigned` / `space cleared`. exit 0 means
acknowledged; 1 means operational failure/unknown; the failure envelope retains
`dispatch`. inspect with `info --ref` after unknown, without automatic resend.

`list --json` remains `{partial,peers}` inside the existing success envelope.
filter each successful peer's sessions locally after decoding; preserve every
peer in machine scope, profiles, observation time, availability, and required
empty successful arrays. errors and `partial` describe source inventory, not
filtered row count. keep host row order in json; no duplicate grouped rows,
groups array, or second envelope.

normal `list` prints named headings and unassigned over existing table columns.
print unavailable peers once outside headings even with zero matching rows.
reuse one fleetclient-owned pure row/group projection for cli and tui, retaining
input peer/name order within groups. renderers do not own equality/filter/sort.
headings are presentation only. `info` includes membership with its other facts.

## 7. collection behavior and creation

machine and space are independent selectors. each has an all state; space also
has unassigned and named. display named picker entries as `space: <label>` to
distinguish labels from selector states. selected empty labels remain in the
selected control; other empty labels need not remain suggestions. no collapsed
group state exists.

named group headings also use `space: <label>`; the unassigned heading is
`unassigned`. the unresolved selected control says `previously selected space`
without the named prefix. these distinctions apply in text and accessibility.

suggestions are sorted distinct labels from currently retained observations,
independent of the space filter. they may include stale evidence; they promise
neither existence nor completeness and confer no action readiness. label the
list `observed spaces`. no extra discovery request or durable suggestion cache.
a machine scope can expose fewer observed suggestions than all machines; free
text always permits another valid label.

retain unfiltered source inventories for refresh and stale-row retention.
`list --machine` reads that host; phone manual verification snapshots machine
scope; tui refresh reads its selected machine scope. a space filter never chooses
hosts. phone's independent automatic pollers, pressure scope, and pull/read
completion ordering remain unchanged.

zero-row copy: `no sessions in this view` after complete fresh scoped reads;
`no matching sessions in available inventory` with existing unavailable/stale
notices when scoped hosts cannot establish current inventory. initial reads
retain checking. never say a space was deleted. out-of-scope outages do not
prevent declaring the selected machine's intersection empty.

### selection and edits

- retain the selected lifetime when it remains visible after regrouping.
- otherwise select its former visible session index clamped to the new last
  session index; empty means no selection. headings are not selectable and do
  not count as sessions. different filters use this rule; active-filter selection
  is a no-op.
- edits, confirmations, and reads retain their original captured target. changing
  row selection never changes an open action. replacement lifetime disables or
  closes its editor with an unavailable notice; rename/agent change alone does not.
- assignment never changes filters. moving the last row away leaves its named
  filter selected and its honest empty state. the phone gains no card-selection
  cursor; its viewport and existing exact dialog targets carry continuity.

unchanged-save disabling compares the canonical draft with the latest accepted
membership of that pinned lifetime, not an obsolete value from when the editor
opened. a poll may update the displayed current membership, but never the draft.

### creation exception

interactive creation from a resolved named space visibly prefills its label;
all/unassigned defaults to unassigned. changing machine preserves the space draft
alongside name/objective; existing cwd/profile rules remain. editing or cancelling
the draft never changes collection filters.

after confirmed creation, use the returned session's observed membership. retain
the current space filter if it admits that session; otherwise select the returned
named space or unassigned, cancel saved restoration, and reset viewport to top.
this transition belongs only to deliberate successful creation. tui selects and
reveals the returned exact session. phone keeps its existing post-create terminal
admission and returns to the resulting filter. existing post-create machine
behavior is unchanged; space never silently chooses another host.

unknown/failed creation does not switch filters or assign separately. preserve
its space draft in the existing in-memory create recovery, never android saved
state. no recovery path automatically resubmits.

if a restored named filter has no recovered label, opening create displays
`choose a space for this new session`. submission requires a deliberate named or
unassigned choice. use a small `unresolved | unassigned | named draft` field state
for this distinction: initial blank is unresolved; explicit unassigned, choosing
a suggestion, or entering nonempty text resolves the choice. invalid text still
fails validation. ordinary resolved editors may use blank to mean unassigned.
after an explicit choice, deleting the field contents also chooses unassigned;
only the untouched unresolved state blocks submission. later inventory resolution
must not overwrite a form the operator has already opened or edited.

## 8. tui interaction and return

retain one bubble tea model, one collection, five-second refresh, and at most
one inventory request in flight. add `g` for the space picker, `m` for the machine
picker, and `e` for the selected session's space editor. keep spacebar for info
and current control keys. picker arrows/j/k move; enter selects; escape cancels.
include the additions in visible hints and `skid --help`.

the machine picker lists configured peers, including unavailable ones, plus all
machines. expose only labels/handles from fleetclient configuration to this
consumer, never origins/bearers. initial state is all/all. changing machine
requests fresh inventory for that scope before enabling its actions. carry
requested scope in the existing inventory message: an old-scope result cannot
satisfy the new-scope read; coalesce one follow-up refresh if necessary. space
changes need no request. no second poll loop or request-generation framework.

retain complete received session rows before space filtering. a failed scoped
peer retains its previous rows as unavailable; an unqueried peer is not reported
as freshly checked. no per-filter row histories. returning to all performs an
all-machine read before enabling its collection.

the space editor has the pinned machine/name, current membership, one text field,
observed suggestions, explicit unassigned, and save/cancel. selecting a suggestion
fills the draft, never submits. blank means unassigned in an ordinary editor.
disable unchanged save and invalid input; direct api no-ops remain valid. after
a possible write, request inventory and preserve uncertainty without replay.
use existing refresh-after-action sequencing to require a post-write read;
an already-running pre-write result cannot enable another membership submission.
retain the affected peer's rows as non-actionable until that read succeeds.
definite missing/replaced-session, access, or internal failures also require a
fresh read; `not_sent` proves no assignment, not continued source freshness.
validation rejection can leave the draft editable. invalid drafts display escaped
characters in the tui while retaining their original editable bytes; valid
unicode labels retain their ordinary presentation.

append one visible space field to the current create form. share its label input
and suggestions with the editor. an explicit machine filter prefills the create
machine visibly and editably, preserving the current machine/profile form rules.
if successful creation is outside that machine filter, select the returned
machine so the newly selected row is visible; this is the tui's new-filter
counterpart of its existing select-created-session behavior. all-machines remains
all. phone's existing post-create machine behavior is unchanged. cancelled or
uncertain creation changes neither selector.

group headings consume terminal lines but cannot receive the cursor. retain a
semantic top item (session lifetime or heading label) alongside selected lifetime
in the model, with no disk state. count headings when fitting content to height;
keep selection visible, then preserve the top item when it still fits. attachment
returns to this same model/filters; refresh re-resolves keys. resize may clamp
viewport. no alternative renderer, per-space history, or terminal-composition state.

## 9. phone presentation and operation ownership

keep title, machine strip, pressure/notices, and card facts. add one compact space
selector immediately below the machine selector, outside the pull owner. its
sheet lists all spaces, unassigned, and observed named spaces. current unresolved
or empty selection stays visible. add one full-span heading per nonempty group
in the current lazy grid, including unassigned.

each card gains an explicit `space` text action beside the existing stop/kill
action. preserve readable machine/profile footer text above actions if width
requires; never shrink text/touch targets. card-body tap still opens terminal.
the new action neither attaches nor invokes the card tap. its spoken name names
session, host, and membership. editing is dashboard-only; terminal chrome gains
no control.

use controller-owned drafts and ordinary transient compose state, never
`rememberSaveable` for labels, space editors, or forge membership fields.

reuse rename/forge's cut-corner modal, field, buttons, validation copy, and angular
indication. one field, unassigned, observed suggestions, save/cancel; no wizard,
gesture-only editing, per-space colour/icon, counters, or decorative hierarchy.
disable autocorrect/automatic capitalisation; permit unicode. one space-field
composable serves forge and the editor. suggestions only fill; clear saves
unassigned and never deletes sessions.
the observed suggestions open in one disclosure menu so a large observed set
does not lengthen the form. the card footer sits above the spaced action row,
accepting extra card height to retain readable context and 48dp targets. when
width or enlarged text prevents both actions fitting, the action container wraps
with the same 8dp separation; labels and touch targets remain complete.

follow [design-language.md](design-language.md): literal labels, existing body/data
faces and surfaces, quiet headings, gold selection, 48dp actions, existing spacing,
full spoken labels despite ellipsis, heading semantics, and no new animation.
terminal sizing, input, back, detach, and provider presentation stay unchanged.

controller owns one optional dashboard editor, mutually exclusive with forge and
destructive dialogs. it contains original `SessionTarget`, draft, and
`editing | sending | checking` phase. equality excludes name/agent; do not reuse
rename's `sameSessionAuthority` unchanged because it includes the expected name.
polls do not overwrite the draft or substitute another lifetime.

submit through the machine's existing `inventoryOperation.submitMutation`:

- reserve its fence and supersede inventory before dispatch, retaining visible
  non-actionable source rows;
- send one `GatewayClient.setSessionSpace` using pinned session identity;
- definite validation rejection preserves draft, clears that exact fence, and
  displays the error; access errors use the current access-failure owner;
- definite missing/replaced-session or internal failure retains the fence until
  an ordered read; keep its definite rejection copy, never relabel it unknown;
- `204` and uncertain completion require a successful inventory read ordered
  after the mutation. a pre-mutation read cannot release the fence;
- prevent submit/dismiss while sending. after http completion, checking can be
  dismissed; the content-free fence survives dismissal;
- a later accepted read releases the fence regardless of observed label. after
  acknowledgement, close the editor and render current membership, including a
  concurrent writer's different value. after unknown, retain uncertainty/draft
  for explicit review if the editor stays open; matching values do not prove
  which write happened;
- a vanished target closes its editor with an unavailable notice. failed reads
  preserve the existing stale/admission behavior and pending requirement until
  a successful read or the existing access/reset lifecycle retires it. checking
  remains dismissible, rather than trapping the user in an offline modal.

centralize rename-specific metadata fences into `pendingMetadataFences` and
semantic require/clear helpers used by rename and space. retain
`MachineInventoryOperations`, `AwaitedInventoryReads`, generations, and credentials
as current owners. no copied lane logic, second fence map, or generic mutation
framework. rename retains its name-conflict reconciliation; space uses last-write
semantics. recreation discards editor/network state and re-lists without sending.

## 10. android navigation and content-free restoration

extend `DashboardEntryState`; no second owner. retain
`DashboardScope.All | Machine(handle)` as the machine dimension and add separate
space selection `all | unassigned | named`. named selection has a comparison
fingerprint and optional resolved label. missing label explicitly means unresolved
restoration. known selected labels stay only in process memory, even after their
last membership disappears; this is navigation intent, not a space registry.

name the live value `DashboardSpaceSelection`. the saved representation carries
only its all/unassigned/named key and optional named fingerprint, never its
resolved label. keep that distinction in the snapshot type, not merely an
instruction to omit a field during serialization. named constructors require
that any resolved label hashes to their fingerprint; resolving the display name
preserves the comparison key and is not a filter-change event.

fingerprint: lowercase hex sha-256 over utf-8 `skidbladnir.space-label.v1`, then
a four-byte unsigned big-endian byte length and the canonical label's utf-8
bytes. no machine enters the hash. it is phone-local comparison data, never a
host field, reference component, space id, control address, log, or credential.
reuse platform sha-256 and the existing card fingerprint framing pattern.
this avoids raw content persistence, not dictionary guessing of likely labels.

resolve a selected name by fingerprint from observed inventory, independently
of machine filter. resolution does not change selection or cancel restoration.
no match retains the filter as `previously selected space`; never switch to all
or unassigned. later polls may resolve it. initial unresolved host reads may
show checking; a modeled unavailable outcome must allow restoration to settle.
missing pairing retains the reset-to-all/top rule and resets both filters.

### rendered keys and task schema

headings are full-span lazy items. remove the old assumption that session index
and lazy-item index are equal. one ordered heading/session projection serves
rendering, capture, and restoration:

- session: existing unchanged `DashboardCardKey` fingerprint string;
- named heading: `space:<space-label-fingerprint>`;
- unassigned heading: `space:unassigned`.

use a small typed `DashboardItemKey` union; headings never become targets. raw
labels never enter compose keys. capture the actual first visible keyed item and
scroll offset, so a top heading restores without signed card offsets or saved
neighbours.

hard-cut to task schema **2**, under the same registry key
`dev.niels.skidbladnir.dashboard-entry`. exact primitive-only bundle:

| key | required / value |
| --- | --- |
| `version` | integer `2` |
| `scopeKind` | `all` or `machine` |
| `scopeMachine` | iff machine; existing valid handle |
| `spaceKind` | `all`, `unassigned`, or `named` |
| `spaceLabelSha256` | iff named; 64 lowercase hex |
| `anchorKind` | `none`, `session`, `space`, or `unassigned` |
| `anchorSha256` | iff anchor is session or named space; 64 lowercase hex |
| `fallbackIndex` | nonnegative integer, now a rendered-item index |
| `offsetPx` | nonnegative integer, existing scroll-offset meaning |

no anchor requires index/offset zero. extra keys, wrong primitive types,
malformed current-version variants, and inconsistent fields are trusted-state
defects. no capsule or unsupported version starts fresh all/all/top. delete
schema-1 reader/writer and `anchorLifetimeSha256`; no migration or dual reader.
upgrade may lose navigation position once; pairing is untouched.

keep the existing restoration sequence: accept fleet, restore filters before
verification, wait for machine-scope inventory outcomes, resolve saved item key
or clamp its former rendered index, then request one immediate non-animated scroll
before enabling cards. empty projection consumes restoration into the existing
empty surface. a vanished session may clamp viewport to a heading, never an action.

retain one live grid object across terminal round trips. no per-filter viewport
history, saved inventory/order/label map, terminal/editor state, or attachment.
a different filter cancels pending restoration; the same filter is a no-op.
geometry clamps normally. reveal the selected machine chip; the compact space
control always shows selection and needs no horizontal-offset persistence.

terminal access loss still selects its affected machine and resets viewport to
top; retain the space filter. machine notices are outside space filtering, so
the reason stays visible. detach/back and supported task recreation preserve both
filters; recreation lands on dashboard, never resumes attachment or mutation.

## 11. reuse, removals, and files

these paths are the implementation assignment, not permission to alter unrelated
features in them. root owns canonical docs and any gate composition changes.
this implementation uses the requested host, desktop, and android builders;
cross-owner adversarial reviews make no test or production edits.

| paths | responsibility / reuse |
| --- | --- |
| `internal/space/space.go`, matching tests | pure shared go label/filter/order owner |
| `internal/sessions/{types,manager,attachment,validation}.go`, new `space.go` if useful, matching tests | typed property/input, pane-independent metadata, create/set, lifetime validation reuse |
| `internal/tmux/client.go`, matching tests | common lifetime terms, conditional encoded set/unset; preserve name-sensitive wrappers |
| `internal/gateway/{dto,gateway}.go`, matching tests | field, put route/body, error/dispatch; reuse strict decoding and session projection |
| `internal/logging/logger.go`, matching tests | normalized route, put method, error code; no labels |
| `internal/fleetclient/{request,response,client,config}.go`, new `spaces.go`, matching tests | request/projection/dispatch, bodyless result, shared grouping, safe machine-picker data |
| `internal/agentcli/run.go`, matching tests | grammar, grouped text, info and help; same json envelope |
| `internal/sessionui/session.go`, matching tests | full source rows, filters/editor, five-field creation, heading-aware cursor/viewport |
| android `ProductModel.kt`, new `Spaces.kt` | session/draft/wire types, parser, grouping; reuse `hasDisplayUnsafeCodePoint` and `compareCaseInsensitiveUtf8` |
| `android/app/build.gradle.kts`; android `src/test/java/android/icu/text/Normalizers.kt` | test-only icu4j 76.1 and narrow sdk namespace forwarding for jvm tests; production uses platform icu, with no apk dependency or alternate algorithm |
| android `GatewayClient.kt` | bodyless authenticated put and route errors |
| android `WorkingDirectoryPicker.kt`, `TerminalConnection.kt` | only exhaustive error-enum consumers made necessary by `SpaceInvalid`; no route or behavior expansion |
| android `SkidbladnirController.kt`, `SessionRename.kt` | space operation and shared metadata-fence bookkeeping; distinct rename semantics |
| android `DashboardEntryState.kt`, `DashboardScreen.kt` | two filters, item projection, schema-2 capsule, selector and restoration |
| android `SessionCard.kt`, `ForgeSheet.kt`, new `SpaceSheet.kt` | card action and shared space field using existing chrome |
| android `MainActivity.kt` | thread events only as required; retain single saved-state owner |
| go colocated tests; android `src/test/.../SpacesTest.kt` and current contract/entry tests | label/transport/group/filter/fence/restore behavior |
| `tests/integration/spaces_test.go`, current isolated fixtures | authenticated host/tmux membership and lifetime boundaries |
| android `src/androidTest/.../SpacesInstrumentedTest.kt`, current fixtures | real compose/registry editor, selector, heading anchor and return |
| this plan, canonical docs, directly superseded client/navigation specs | current contracts and historical evidence attribution |

android production paths are relative to
`android/app/src/main/java/dev/niels/skidbladnir/`; tests use the same package.
new filenames name owners and may remain colocated when clearer; no one-use
helper needs a file just to mirror the table.

required cleanup:

- replace flat human collection rendering with the grouped projection; no old
  renderer flag. peer-oriented json is the intended data contract;
- remove source retention based on filtered rows and cursor/index calculations
  that confuse headings with sessions;
- replace the four-field tui count/help, not a second create form;
- delete schema-1 capsule paths and card-only lazy-index assumptions; update every
  consumer to the same rendered-item projection;
- rewire both metadata operations to shared fence bookkeeping and delete its
  superseded rename-only helpers;
- no copied name-staleness contract, duplicate fleet selector, or reconstructed
  agent target;
- objective/name/path grammars differ. reuse safety/comparison primitives without
  generalizing all text validation. the short go path-sort helper does not justify
  a cross-feature refactor; android's existing common comparator is directly reusable;
- delete newly dead symbols after checking production and test callers. no
  unrelated cleanup, schema generator, framework, or future-proofing slice.

## 12. acceptance and bounded proof plan

builders own an acceptance proof and observe its intended failure before
implementing that owner. compile failure alone is not the intended behavioral
red. test where behavior is owned. do not mock internal services/controllers to
claim a live journey. reuse fixtures. pure tests never invoke tmux. synthetic
labels may be inputs, but failures/evidence report case names and outcomes,
not label values, terminal bytes, prompts, credentials, or provider output.

| criterion | observable proof |
| --- | --- |
| a1 · label/wire | canonical/decomposed distinction; 64/65 scalars including supplementary unicode; whitespace/controls; exact equality/order; optional omission and explicit clear; strict fields/types/nulls; both languages agree |
| a2 · host membership | create assigned/unassigned; set/change/clear/no-op; list/create/info projection; invalid input mutates nothing; invalid/local-absent/global-only metadata projects unassigned without repair |
| a3 · lifetime | old ref survives rename and pane/foreground replacement; stale session/server ref rejects; process identities, pane/window, cwd, name, character and attachment survive assignment; grouped sessions have independent labels |
| a4 · ordering/uncertainty | concurrent absolute assignments yield whole last-applied values; possible dispatch never permits replay; one client write; pre-mutation reads cannot clear phone fence; a later differing label is authoritative without a fabricated failed-write claim |
| a5 · fleet | equal labels group across hosts; case-distinct labels stay distinct; unassigned last; within-group order preserved; intersecting filters; selector-looking labels distinguishable; peer json and partial status honest |
| a6 · unavailable | stale actions disabled; unavailable peers visible with zero matches; filter changes reveal retained rows; refresh discovers new membership on any host in machine scope; old tui scope result cannot admit new scope |
| a7 · edit/create | all clients set/change/clear; suggestions fill without sending; invalid drafts survive; cancel has no effect; target stays pinned; visible prefill and unresolved explicit choice; only confirmed out-of-filter creation changes filters |
| a8 · navigation | selection follows lifetime or specified clamped session index; heading/card viewport survives detach/back, insertion/reorder and recreation; missing anchor clamps rendered index; absent label stays selected; unavailable restore settles; capsule contains only exact schema-2 primitives |
| a9 · regression/scope | unassigned terminals preserve controls/defaults/host rules; no shell launcher, tmux grouping, provider meaning, launch context, persistent space resource, or compatibility path |

proof shape:

1. compact pure fixture matrices per language for labels/schema and grouping,
   filters, and restoration. centralize within each test language, with no new
   fixture-ingestion framework. existing cli/tui and android jvm contract tests
   cover their distinct boundaries; they do not substitute for real interaction.
2. extend the approved isolated gateway/tmux journey on linux and darwin for
   a2–a4. use only exact test-owned sessions. include plain shells, grouped-session
   independence, rename, pane/foreground change, gateway reconstruction, and old
   server references. compare content-free lifetime/geometry facts. do not launch
   paid providers merely to test metadata.
3. one approved real-compose/real-registry journey covers editing, both filters,
   heading/card anchors, post-create transitions, detach/back and task restore.
   a phone-to-approved-isolated-host sample proves the actual membership request;
   a UI fixture alone does not prove host mutation. use synthetic sessions and
   preserve existing pairing/release-recovery requirements.
4. run applicable routine checks. `scripts/test verify` currently composes static,
   build, and unit; static compiles integration/live tests without executing them.
   inspect composition again before running. no new gate is needed, and compiling
   a runtime test does not pass its boundary.
5. publication/deployment follows coordinated release admission. historical release,
   provider, and device evidence proves no new spaces claim.

tmux invocation and integration/live execution require explicit current-turn
approval. platform/adb and phone mutation require their own current-turn approval.
opt-in variables and composite commands are not approval. missing host/device/live
boundaries are `NOT_RUN`, never skipped into a pass. without approval, complete
available work and report open acceptance; do not claim the feature accepted.

## 13. hard cutover, non-goals, and explicit costs

one pr changes host, fleetclient/cli/tui, and phone together. one coordinated
release carries them through existing distribution owners. no negotiation,
feature detection, schema aliases, dual metadata readers, fallback routes, legacy
renderers, migrations, or mixed-version support. missing label means unassigned,
not an old-server compatibility branch. this pr changes no version, publication,
pin, or deployed host/device. those follow their existing separately approved
release and runtime workflows.

| choice | accepted cost / rejected alternative |
| --- | --- |
| one exact label, no lifecycle | manual filing; no overlaps, nesting, bulk rename, or saved empty groups; no registry |
| host session metadata | destruction loses membership; no durable project catalogue or automatic reassignment |
| bounded nfc display-safe text | human drafts normalize; noncanonical api/metadata rejects or omits; no non-ascii whitespace or fuzzy equality |
| plain nfc in go | a small local ordering/composition loop is required because the existing whole-string library inserts separators; reuse its unicode data, with no new dependency |
| fixed unicode-15 normalization | later code points remain inert normalization boundaries until an explicit coordinated contract change; platform icu supplies phone normalization; jvm proof adds a roughly 14.6 mb test-only icu4j artifact and narrow sdk forwarding, with no apk dependency |
| escaped invalid tui drafts | invalid text temporarily shows escapes for safe repair; original editable bytes remain intact, and valid unicode remains readable |
| ascii-folded byte order | deterministic across clients, but non-ascii collation is not localised |
| base64url option | raw tmux inspection is less readable; one canonical safe encoding replaces interpolation/alternate formats |
| one required-string put body | clear uses empty string while optional inventory omits absence; exact field-presence checks are required, with no nullable grammar or separate clear route |
| session-only assignment | agent replacement deliberately does not block filing; agent controls retain process validation |
| last applied write, no replay | edits can overwrite and acknowledgements can be lost; no versions, receipts, conflict subsystem, or repair daemon |
| observed suggestions | incomplete/stale names are typing assistance, not a catalogue or readiness claim |
| filtering source inventory locally | no source payload reduction or space-derived host scope; preserve discovery and honest partial results |
| confirmed creation can change filters | deliberate creation may change return context; edits and uncertain creation cannot; tui reveals the created machine when needed |
| phone editing on dashboard | detach to refile; no terminal-chrome cost or added terminal navigation |
| phone suggestions and action row | one extra tap opens suggestions; cards gain height to preserve readable context and 48dp actions |
| saved label fingerprint | disappeared name becomes generic after recreation; creation requires explicit choice; hashes do not conceal guessable labels |
| rendered-item anchors | a vanished anchor may clamp to a heading; no saved historical neighbour list |
| task schema 2 | upgrade may reset navigation once; pairing preserved; no old decoder |
| coordinated release | mixed versions unsupported; optional fields can break strict old decoders; rollback restores the coordinated release |
| bounded proof plan | no new infrastructure or broad provider qualification; unavailable boundaries remain explicit acceptance gaps |

other non-goals: git/worktree management, environment cloning, shell launch kinds,
terminal splits/renderers, layout persistence, multiple-addressable-pane discovery,
unread/task state, hooks, automatic cleanup, gateway-to-gateway knowledge, failover,
analytics, schema generation, and abstractions for later prs. do not invent new
product behavior merely to simplify a difficult implementation edge case.
