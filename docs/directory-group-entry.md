# directory and group entry

accepted design, source implemented 2026-10-02. directory and group entry
share one interaction and a small reusable field within each client:
typing updates suggestions, explicit acceptance fills the draft, and a separate
create/save action submits it. mobile directory entry moves into the forge form
with live search and tap to select. this document owns the contract, implementation
plan and [qualification](#qualification). release and production installation
remain separate work.

[group entry](group-entry.md) owns group candidates, ranking, observation scope
and unresolved prefills. [terminal continuity](terminal-continuity.md#directory-search)
owns host zoxide search and its bounds. [the directory chooser](working-directory-chooser.md)
owns secondary home browsing, path grammar and creation validation. this design
supersedes their input presentation where stated below, including mobile's
places, search and exact-path pages and `z <words>` entrance. [desktop browser](desktop-browser.md)
owns the surrounding form, actions and return. [architecture](architecture.md)
owns scope; [roadmap](roadmap.md) tracks implementation and qualification.

## problem and scope

the predecessor's desktop directory field searched as the user typed; mobile
required a places page, a separate search page and explicit search submission.
searchable group entry existed on both clients, but directory and group owned
separate presentation and selection mechanics. the mobile picker in `v0.12.2`
had the same directory flow. that release did not repair it.

cover directory and group in interactive creation, plus group membership editing.
reuse interaction within go's terminal ui and within android compose; each
client keeps its native implementation. group filtering remains synchronous.
directory requests retain their existing host endpoint and ranking. shared code
owns focus, suggestion presentation and selection mechanics; each domain owns
which values are admissible. no gateway, cli argument, discovery, crawler,
registry, persistence, dependency or polling change is required.

## shared field contract

1. one authoritative raw draft remains with the existing form/controller.
   suggestions and highlights are separate from it. rendering, refresh and
   moving a highlight never rewrite text or restore an old accepted value.
2. the domain supplies ordered choices, stable value identities, status,
   validation and acceptance. the field neither ranks choices nor parses values,
   performs search requests, or dispatches create/save. identities distinguish
   actions from named values; list indices and display labels are not identities.
3. editing invalidates choices for the previous draft immediately. accepting a
   current choice writes its exact domain value into the visible draft. final
   submission validates that draft; it never consumes a hidden highlight or
   last accepted directory. directory queries must resolve before creation.
4. empty, loading, failed and ready suggestions are explicit states. absence of
   matches does not erase the draft. a valid literal remains usable according to
   its domain, even without suggestions. validation retains invalid text.
5. choices, drafts and interaction state stay transient. reuse existing owner
   lifetimes and request guards. no raw path, label or query enters logs, saved
   state or qualification evidence.

## value rules

| concern | group | directory |
| --- | --- | --- |
| source | observed labels from retained inventory, with existing client scope | selected host's zoxide results; mobile empty input also offers active local cwd values |
| update timing | filter immediately, with existing group ranking | issue a search after 150 ms without another edit; preserve host ranking |
| empty input | unassigned for a chosen draft; an unresolved prefill requires explicit choice | home, visibly labelled; normalize exact empty input to `~` at submission on both clients |
| literal input | valid labels can be accepted without an observed match | exact `~`, `~/…` or an absolute path; grammar checked locally and existence checked at creation |
| query input | existing group matching, canonicalization and equality | other nonempty input is whitespace-separated search words under the existing term bounds |
| extra choices | use typed label and explicit unassigned, distinct from a group named `unassigned` | home and active directories on mobile, plus secondary browse home |

literal directory detection precedes search parsing. a draft beginning `/` or
`~` never issues a zoxide request; invalid path syntax stays editable. leading
whitespace before a path is invalid, not trimmed into a path. whitespace-only
directory input is invalid; only the exact empty string defaults to home.
spaces and `z` inside literal paths remain literal. remove special treatment of
`z <words>` in mobile entry: it becomes an ordinary query, as on desktop.
quotes, variables and substitutions have no shell meaning. preserve the existing
path and query limits without truncating input or interpreting shell expressions.

mobile's empty-directory choices are home first, then distinct safe cwd values
from the selected fresh machine's local sessions, in the chooser's existing
order. remote session cwd is not a local launch suggestion. use retained
inventory without an extra read. accepting an active path performs no filesystem
request; creation revalidates it. disappearance from inventory does not clear an
accepted literal. desktop retains its current empty-field home default.

## terminal interaction

| input while a field is focused | effect |
| --- | --- |
| typing or paste | edit the draft; update choices immediately for group and schedule directory search after 150 ms |
| left/right | cycle current choices with wraparound; preserve the query |
| tab or enter | accept the highlighted choice, otherwise a valid literal/default, and advance; dispatch no mutation |
| tab or enter without an acceptable value | retain focus, text and the reason; unresolved directory queries cannot advance |
| shift-tab | move backward without acceptance |
| ctrl-u | clear the draft; directory becomes home and group explicitly becomes unassigned |
| escape | cancel through the containing form's existing return path |

show the highlighted choice and position beneath the field. query edits choose
the first current match; group retains its existing literal/unassigned initial
choices. within an unchanged query, a refreshed list preserves the highlight by
choice kind and exact domain value. if that choice disappears, clear the highlight
without rewriting the draft or silently selecting a replacement. an observed
group becoming a use-label action is a different choice. arrows can choose again.

keep the create form's machine, launch, name, directory, group, create focus
order, and the group editor's group, save order. enter dispatches only on the
focused action. the existing group-editor `ctrl-s` shortcut submits the visible
literal draft, not a highlight. final submission returns focus to any invalid
field or unresolved directory query, including values bypassed with shift-tab.
retain pending-operation suppression and exact target capture. share field
preview and cycling/acceptance mechanics between directory and group, including
group editing; keep form navigation and mutations with their current owners.

## phone interaction

directory and group are ordinary editable fields in the forge. focusing either
opens its live choices beneath it in the same sheet and closes the other's
suggestions. show the selected machine beside directory entry. entering a field
starts a focused input session; do not make both fields request focus on form
mount. group membership editing uses the same field component.

keep the active field, bounded scrollable choices and separate final action
reachable with the keyboard open. other form content may scroll. edits reset
the suggestion list to the top; replies and recomputation preserve text focus,
keyboard visibility and ime composition. do not replace the field when results
arrive. blank directory visibly means `home (~)`; no separate directory choice
is required before creating at home on a selected fresh machine.

tapping a current choice fills the draft, closes suggestions and ends text
focus, as group entry already does. the phone has no implicit candidate
highlight. keyboard next/enter accepts only a valid visible literal/default;
it closes suggestions and advances focus to the next form control or ends text
entry before the final action. it never chooses the first search result or
presses create/save. an unresolved directory query remains focused with
`choose a matching directory`; invalid input remains editable with validation.

back first hides the keyboard and ends text focus, retaining the draft. with
the keyboard hidden, back closes suggestions; the next back uses the containing
sheet's existing cancel behavior. reopening preserves the draft. retain group
entry's unresolved-prefill rule and explicit unassigned choice.

use single-line input, disabled autocorrect and automatic capitalization,
complete spoken values and the existing path isolation. choices have at least
48dp touch targets; 2× text must keep the field, choices and final action usable.
show the existing directory search states and omitted-result notice beneath
the field. keep browse home available when search is unavailable. there is no
search button, `z` hint, or separate search/exact-path page.

## directory requests and secondary browsing

the existing ui model/controller owns directory debounce, request state and
completion. on an edit, invalidate previous choices and queued work before
scheduling a new valid query. blank, literal and invalid inputs issue no search.
cancel superseded work where the existing transport permits it; correctness
depends on accepting only the current response, not on successful cancellation.

bind results to the exact form lifetime, machine, query revision and existing
foreground/credential admission. query change, machine change, form dismissal,
backgrounding or access loss invalidates pending work. a late reply cannot
restore choices, enable creation, overwrite text or reopen a field. backgrounding
settles loading to the retained editable draft with no usable old results;
resume does not retry automatically. focusing or editing a query starts its
normal debounce again only through a later user action. invalidation must not
strand a loading state.

changing machine clears directory and its search, closes browsing, and follows
the existing profile/terminal-choice rule. preserve group, name and objective.
stale or unavailable machine inventory still disables that host's creation and
directory actions; group suggestions remain typing assistance.

browse home is secondary within the existing forge sheet. open its home listing
directly, retaining current directory text. keep one-level browsing, parent,
filter, hidden folders, bounded history, truthful loading/failure snapshots and
explicit use. use fills the primary field and returns to the form. cancelling
or backing out of the root listing returns to that field without changing its
draft. browse recovery's enter-path action returns to the focused primary field.
loading-back and background invalidation restore a retained browse snapshot as
loaded; with no retained snapshot, return to the unchanged primary field instead
of the retired places page. explicit return actions focus that field;
background invalidation does not reopen the keyboard or restart a query. these
browse transitions are distinct from inline search's cleared-result state.
the chooser retains its host/path/symlink and request guards; retire the old
places, search and exact-path pages and their redundant state/actions.

## implementation plan

1. **shared interaction.** extract the existing group field's focus, disclosure,
   bounded choices and acceptance presentation into a small compose component.
   extract the terminal ui's common selection/preview mechanics. design only for
   these two real callers; domain code supplies choices and acceptance. keep
   drafts and effects with their existing owners. do not introduce a backend
   interface, query framework, configurable matching engine or cross-client ui
   package.
2. **terminal ui.** update `internal/sessionui/{create,group_entry,metadata,view,session}.go`
   so creation's directory/group fields and group editing use the shared
   mechanics. retain group candidate rules, zoxide ordering, debounce, cancellation,
   action focus and the existing request/completion path. update help in
   `internal/agentcli/run.go` only where it describes changed interaction.
3. **android.** under `android/app/src/main/java/dev/niels/skidbladnir/`, update
   `GroupSheet.kt`, `ForgeSheet.kt` and their shared field component; keep group
   candidates in `Groups.kt`. put inline directory request ownership in
   `SkidbladnirController.kt`, using its existing scheduling/network/foreground
   owners. update `ProductModel.kt` submission admission so exact empty cwd becomes
   `~`, a literal follows path validation, and queries cannot submit. update
   `WorkingDirectoryPicker.kt`, `WorkingDirectoryPickerScreen.kt` and narrow
   `DashboardScreen.kt` wiring for secondary browsing and retire obsolete pages,
   callbacks and exact-path/search state. reuse `GatewayClient.searchDirectories`
   and the existing strict response decoder; no gateway change is planned.
4. **verify and record.** exercise the acceptance below against the actual
   client owners and changed behavior. follow [testing policy](rules/testing.md),
   remove temporary checks before commit and run `scripts/check verify`. record
   source/transport, host and physical-phone results separately. review source
   and docs together for one primary interaction and no retired state. release
   and installation remain separate work.

## acceptance and delivery

| boundary | required evidence |
| --- | --- |
| shared client interaction | directory/group in creation and group editing use the same per-client mechanics; typing and cycling preserve drafts; acceptance dispatches no mutation; only the final action or existing explicit save shortcut submits |
| values and defaults | exact empty directory submits `~`; whitespace-only input and leading-whitespace paths stay invalid; group unassigned/new labels/unresolved prefills retain meaning; case variants and a named `unassigned` retain exact identity; literal paths with spaces or `z` issue no search; invalid values stay editable |
| directory request and submission | 150 ms debounce coalesces edits; literal/blank/invalid inputs cancel obsolete work; changing an accepted path into a query cannot create using the old path; empty/error/loading and delayed replies after edits, host switches, dismissal, background or access loss cannot provide a cwd |
| terminal forms | arrows cycle without editing; tab/enter accept then advance; shift-tab and final actions cannot bypass query/validation; refresh retains choice kind plus exact value or clears the highlight on removal; observed and use-label choices remain distinct; 80×24, long values and no color remain usable |
| actual phone sheets | forge and group editing keep one focused field with bounded live choices; tapping accepts; ime next never chooses a hidden result or submits; back/reopen, keyboard changes, 2× text and spoken values remain usable with the final action reachable |
| secondary browse | selected host, home/parent/filter/hidden/history and failed/loading snapshots retain their contracts; use edits only cwd; cancel preserves the query; enter-path returns to the primary field; invalidation restores a retained browse view or the unchanged field, without a places fallback or automatic keyboard/query restart |
| actual host search and creation | configured host zoxide produces ranked visited paths; removed/unavailable selections receive existing typed creation rejection without mutation; selection never launches a session |

use a small set of temporary tests at the existing client and transport owners.
the baseline failure is mobile's extra-page/manual-search interaction; the
old-path/query rule is a regression invariant for the new inline field, not a
claim of a current submission bug. a controlled response proves debounce and
reply admission; it does not qualify the real host zoxide boundary. phone
source/render checks do not prove keyboard or touch behavior on the device. tmux and phone operations
require explicit approval in their execution turn; a spec is not that approval.
unexecuted live boundaries remain `NOT_RUN`; engineering checks are not product
acceptance. the older
[directory qualification issue](issues/terminal-continuity-directory-qualification.md)
retains host-search cases beyond this focused change.

## qualification

2026-10-02, isolated `feat/directory-group-entry` worktree. the mobile extra-page
flow and absent automatic search failed the temporary baseline probes. the
behavioral results below apply to `8eeb634`. integration with main's profile-usage
changes retains the field implementations, validation and request admission;
adversarial source review and full engineering checks pass on the combined source.

| boundary | evidence |
| --- | --- |
| terminal client and transport | 12 temporary integration checks pass with `-race`. the real model and input-decoding program cover debounce, acceptance without mutation, final actions, exact drafts/defaults, choice kind/value retention, bypass rejection, access denial and stale replies. both creation and group editing render within 80×24 without color with long values. |
| android owner and transport | 12 temporary owner checks pass using the real controller and strict controlled https peers. they cover debounce, superseded replies, query revisions, host/access/lifetime invalidation, exact empty submission, literal/query rejection, home-first active local choices, no read on selection and retained browse recovery. successful child/parent navigation and Back restore listing/filter/hidden/row-offset snapshots without new reads; 34 transitions retain only the latest 32 snapshots; use edits only cwd. |
| physical phone sheets | all 12 production-composable journeys pass on the approved isolated app on an s22+, android api 36, with native gboard at normal and 2× text. actual touch, next, back/reopen, composition across replies, one expanded field, unresolved/named-unassigned groups, group editing, long scrollable choices, full spoken values, 48dp targets and reachable final actions pass. secondary browse return, stale retry/use admission and filter/hidden/new-reply viewport restoration pass. literal-to-query expansion reveals the full field, label and browse action; background-invalidated loading browse returns the unchanged draft with neither focus, keyboard nor a search intent. synthetic screens were inspected at 2×. |
| physical phone controller | the real controller starts from an encrypted synthetic stored fleet and controlled https peers. background/resume retains the editable draft, clears old results and does not retry; only later user focus starts another search. |
| actual darwin host | the real gateway, filesystem, configured zoxide executable/database and isolated tmux socket pass ranked visited-path search, read-only selection and home browsing. a deleted selection receives typed `WorkingDirectoryUnavailable` with `dispatch=not_sent` and no session; valid literal creation preserves the exact cwd and group. only probe-owned sessions were created and removed. |

adversarial review exposed defects at their owners: authoritative access denial
must fence preceding inventory; cancelled directory work must lose reply admission;
android's native whitespace parser must use portable character classification;
executor futures must preserve fatal defects; active-field reveal must follow
guidance and available-height changes; and browse restoration must finish before
scroll capture resumes. its scope is the exact listing response snapshot,
filter and hidden state, so a new equal-content reply restores while scroll-only
updates do not. focused red/green checks cover those repairs.

the native clients share mechanics within their own language/runtime. domains
retain ranking, value admission and submission. cancellation is best effort;
snapshot admission supplies correctness. directory search covers visited paths,
with literal entry and Home browse supplying other locations. long phone paths
reuse the existing horizontal path row; group choices and long host labels use
one-line ellipsis with complete spoken values. these are the implementation's
scope and presentation tradeoffs.

the terminal program probe uses pipe input with rendering disabled; separate
model renders establish layout. it does not qualify terminal paint or native
focus-report delivery. controlled https responses establish client admission,
not real-host ranking or successful transport cancellation. the phone sheets
use synthetic state, with stored-fleet controller lifetime checked separately;
they do not establish production pairing or installed-fleet deployment. actual
host execution here is darwin only. broader host, provider, linux and human
workflow acceptance retain their existing issue owners.

temporary tests, test-only dependencies, probe build outputs and isolated phone
packages are removed. the original font scale is restored. final
`scripts/check verify` passes in full; it proves engineering checks
and builds, separately from the behavior above. release and production
installation were not performed.
