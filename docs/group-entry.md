# searchable group entry

accepted and source implemented 2026-10-02. this specification covers
group entry during session creation and membership editing on desktop and
android. typing filters observed labels; selecting a suggestion fills the draft;
a separate create or save action submits it. the desktop create form gains an
explicit `create` action below group, and the group editor gains `save`.

[groups](groups.md) owns label identity, observation scope, membership mutations,
creation defaults and return behavior. [desktop browser](desktop-browser.md)
owns the surrounding form and navigation. this document replaces their group
input presentation and desktop submission keys only. the [roadmap](roadmap.md)
tracks delivery; [qualification](#qualification) records the exercised boundaries.

## problem and scope

desktop create and edit previously shared `nextGroupDraft`, which cycled all
observed labels without filtering. right from an unmatched draft replaced it
with unassigned. android's shared `GroupField` used a separate observed-groups
menu that ignored the text being entered. both clients already had the labels.

reuse the directory interaction: editable text, matching choices and explicit
acceptance. group matching runs synchronously over retained inventory. directory
search keeps its host requests, zoxide ranking and path semantics. this change
adds no endpoint, discovery, registry, persistence, dependency or poller.
dashboard collection views, cli arguments, session naming and immediate desktop
`n`/`T` creation retain their existing behavior.

## choices and matching

one raw draft remains the editable form value. candidates derive from that draft
and observed labels; highlighting never rewrites it. create and edit use one
group candidate owner per client.

- desktop uses `fleetclient.ObservedGroups(m.scopedPeers())`: the dashboard's
  machine scope, independent of the form's target machine and selected view.
  android uses `observedGroups(machines)` across all retained machine snapshots.
  neither client performs an extra read when the field opens or changes.
- empty text exposes all observed labels in the existing group order. an
  explicit `unassigned` action means no membership and is the initial choice.
  distinguish that action from a named label whose literal text is `unassigned`.
- nonempty text ranks observed matches by exact canonical equality, then
  case-insensitive equality, prefix, substring and ordered subsequence. use the
  existing label order for ties. subsequence means the query's unicode scalars
  occur in order, with gaps allowed; it is not edit-distance matching.
- case-insensitive matching folds ascii `A`–`Z` only, as group ordering already
  does. other unicode scalars compare exactly. use the canonical value from the
  existing draft parser when valid; otherwise match the original draft text.
  matching never rewrites the draft, trims or collapses whitespace,
  transliterates or changes stored equality. `alpha` and `Alpha` remain distinct.
- a valid nonempty draft without an exact canonical observed match also offers
  `use label: <text>`, even when other suggestions match. that action accepts the
  typed label; it does not create a group resource. omit a duplicate action when
  the exact label already appears. invalid text has the existing validation
  message and no usable literal-label action; it stays editable and can be cleared.
- show `observed groups` above results and `no observed matches` when appropriate.
  observations may be stale or incomplete. no result is an existence or readiness
  promise. selecting a stale label is valid typing assistance; the target's
  existing mutation admission still decides whether create/save is available.

on desktop, matching observed labels precede the literal-label action; unassigned
remains reachable as an explicit choice and through `ctrl-u`. when text changes,
highlight the first observed match, otherwise the literal-label action, otherwise
no choice. empty text highlights unassigned. on phone, suggestions are accepted
by tapping them; rendering results alone changes nothing.

retain a desktop highlighted choice by exact value through inventory refresh,
never by list index. if it disappears, clear the highlight; the visible draft
remains available as literal input. refresh never rewrites the draft or
captured session target. an accepted label remains valid when its last observed
member disappears.

## desktop interaction

the create form's focus order is machine, launch, name, directory, group,
**create**. create is an action, not a sixth text value in the request. the group
editor's focus order is group, **save**. both actions are visibly below the group
field and use the existing form focus treatment.

| focus and input | effect |
| --- | --- |
| group, typing or paste | edit the draft and recompute choices immediately |
| group, left/right | cycle choices with wraparound, leaving the draft intact |
| group, enter or tab | accept the highlighted choice, or the valid literal draft when none is highlighted, and focus create/save; dispatch nothing |
| group, enter or tab without a valid choice or literal draft | remain in the field; retain text and validation |
| group, shift-tab | move backward without accepting a candidate |
| group, ctrl-u | clear the draft to unassigned; dispatch nothing |
| create/save, enter | validate the visible form values and invoke the existing mutation once |
| create/save, shift-tab | return to group |
| create/save, tab | wrap to the form's first field |
| create/save, text, paste, backspace, ctrl-u or left/right | do not edit a field or submit |
| group editor, ctrl-s | retain the explicit save shortcut; submit the visible literal draft through the same validation as save |
| either form, escape | cancel through the existing return path |

the group editor has two focus stops, so backward traversal from group reaches
save without accepting a candidate. final submission always parses the visible
draft as a literal label; it never consumes a highlighted suggestion. this also
covers deliberately reaching create through backward traversal. invalid text
returns focus to group. an unresolved directory query returns focus to directory
under its existing rules. keep the target-availability and unchanged-save checks.

while group is focused, show one candidate preview below it, with its complete
label and position, using the directory preview's layout. action focus shows the
literal draft and action; it does not suggest that submission accepts a candidate.
distinguish `use label` and `unassigned` actions from observed labels. remove the
unfiltered paragraph of all groups.
keep the active field or action and its relevant preview visible at 80×24, with
wrapped labels and the existing form viewport. helper copy teaches `left/right
chooses; enter uses and continues` at group and the action's effect when focused.

enter submits only on the focused action. the existing group editor's explicit
`ctrl-s` save shortcut never accepts a highlighted suggestion; the name editor's
keys and automatic-title controls are outside this change. repeated input after
dispatch uses the existing in-flight suppression. returning to group prefills
the accepted label and chooses its exact match or literal-label action.

## phone interaction

retain one `GroupField` for forge and membership editing. focusing it or opening
`observed groups` reveals a filtered list beneath the field in the same sheet.
typing retains keyboard focus and updates the list immediately. constrain the
list to available sheet space and scroll its contents; a large inventory must
not lengthen the whole form without bound. reveal the group field and first
matching result when suggestions open or the keyboard changes the available
space. typing resets the list to its first result. the group field and bounded
results stay above the separate create/save action; only preceding form content
scrolls. the final action remains reachable with the keyboard open. keeping
group visible gives preceding fields a smaller viewport while suggestions are
open.

tapping an observed result or `use label` fills the draft and closes suggestions.
the explicit unassigned action is inside the choice list: first for empty text,
after matches and the literal action for nonempty text. it clears membership;
each action only edits the draft. keyboard next/enter accepts the valid literal
draft, closes suggestions and hides the keyboard to expose create/save; it never
activates that button.
invalid input remains editable with validation. there is no separate phone
candidate highlight. tapping create/save submits the visible draft literally
through existing validation, without silently choosing a suggestion.

back hides the keyboard and ends text focus first when visible, retaining the
draft and choices; the next tap starts a fresh input session. then back closes
open suggestions, then uses the containing sheet's existing cancel behavior.
dismissing suggestions retains the draft. reopening suggestions retains the
current draft. prefilled labels and the unresolved restored-group state keep
their existing meanings:
an untouched unresolved field cannot silently become unassigned; it requires an
explicit choice, including an explicit unassigned action.

keep autocorrect and automatic capitalization disabled, full spoken labels,
48dp actions, readable large text and existing validation. suggestions use
ordinary transient compose state; controller-owned drafts and mutation phases
remain authoritative. no raw label or query enters saved state or logs.

## implementation plan

these are the implementation ownership boundaries, not a requirement for
separate files or agents.

1. **group candidates.** add a small pure candidate/ranking function beside each
   client's group input owner. reuse the existing label parser, ordering and
   observed-label source. keep ui matching out of the gateway and membership
   storage. the directory's ranking provides the pattern; its unicode lowercase
   policy and host search machinery are not part of this group contract.
2. **desktop forms.** update `internal/sessionui/{session,create,metadata,view}.go`
   to use the shared group choices, preserve query text while cycling, and add
   action focus below group. replace `nextGroupDraft` and the unfiltered
   suggestions renderer. update the editor/form hints in `internal/agentcli/run.go`.
   keep action focus out of form-array indexing, including the model's paste
   handler and the form's backspace path. reuse existing execution/completion.
3. **phone field.** update `GroupSheet.kt` and `Groups.kt`, with narrow wiring in
   `ForgeSheet.kt` if needed, under `android/app/src/main/java/dev/niels/skidbladnir/`.
   replace the unfiltered dropdown with the bounded searchable disclosure.
   preserve the public callbacks and controller-owned drafts where possible.
4. **verify and close.** exercise the acceptance below, remove temporary checks
   under [testing policy](rules/testing.md), run `scripts/check verify`, and update
   delivery status with actual evidence. runtime checks remain separate from
   engineering checks. temporary device test installation is part of approved
   qualification; release and production deployment are outside this scope.

## acceptance

| boundary | required evidence |
| --- | --- |
| shared choice rules in each client | empty/all, exact/prefix/substring/subsequence ranking, stable ties, ascii case variants, unicode and decomposed drafts, invalid text retention, a new label despite existing matches, and named `unassigned` versus no membership |
| actual desktop form transitions | both create and edit: typing filters; arrows preserve the query; enter from group only fills and focuses the action; enter on the action dispatches once; action-focused editing keys are inert; backward traversal and explicit save shortcuts submit only the visible literal value; invalid values, cancellation and repeated input behave as specified |
| current observations and exact targets | candidate insertion/reordering preserves highlight identity; removal clears it; accepted labels survive disappearance; scope stays unchanged; refresh never retargets editing or admits a stale target |
| desktop rendering and help | create/save focus and preview remain usable at 80×24 with long labels and no color; instructions describe actual keys; directory acceptance and the name editor retain their behavior |
| actual phone interaction | both forge and edit: typing while suggestions update, touch choice/new label/unassigned, back and cancel, unresolved prefill, keyboard next without submission, large lists, full spoken labels and 2× text remain usable |

use a small set of temporary candidate and actual client model/render checks,
including a baseline failure for unfiltered selection and immediate desktop
submission. intercept the existing mutation boundary to verify zero requests
on selection and one on the final action; do not build a second form implementation
or recreate retired harnesses. phone interaction requires an approved device
journey; source checks cannot establish it. tmux and device operations retain
their explicit current-turn approval requirements. any unexecuted boundary is
`NOT_RUN` and stays recorded in its own issue.

the pre-existing [invisible-label ambiguity](issues/group-label-invisible-characters.md)
remains outside this change; search preserves label identity and does not resolve
that separate display contract.

## qualification

2026-10-02: the specified client boundaries passed temporary acceptance checks
after adversarial source and probe review. the production source remained frozen
after its final behavioral checks.

- **baseline RED:** three desktop checks exposed immediate group submission and
  unfiltered cycling; two phone checks failed at the actual shared field before
  filtered suggestions were implemented. a later native 2× check failed in
  both sheets when reopening an unchanged, scrolled query; the existing scroll
  reset was corrected and the same checks passed.
- **desktop PASS:** 14 temporary tests passed with the race detector. they used
  the actual model, rendering and input decoder, plus a running `tea.Program`
  and the existing client against an isolated https mutation endpoint. selection
  sent zero requests; the final action sent one. checks covered choice rules,
  refresh identity and scope, captured targets, literal prefill and traversal,
  action-focused input, validation, cancellation, in-flight suppression, 80×24
  wrapped rendering without color, help, directory behavior and the name editor.
- **phone PASS:** 13 ordinary checks and two native 2× journeys passed on the
  approved connected phone. the probe mounted production forge/edit sheets
  under the production theme and surface, with synthetic inventory and the
  existing draft/admission types. it intercepted their parent callbacks:
  selection and keyboard acceptance submitted nothing; the final button
  submitted once. checks covered matching, literal/unassigned choices,
  unresolved drafts, native enter and three-stage back, stale-target admission,
  scrolling through 100 labels plus a 64-character label, unchanged-query
  reopening, full spoken labels, visible 48dp choices and create/save with the
  keyboard open, including hide/reopen and subsequent typing. native 2× text
  was verified from the field's actual
  text layout. synthetic screenshots were visually reviewed. this qualifies
  the changed sheets and their callback boundary; the unchanged phone
  controller/gateway transport was outside the probe.
- **engineering PASS:** `scripts/check verify` passed for all build scopes after
  temporary checks and probe configuration were removed: formatting, shell and
  python checks, catalogue/assets/ornament validation, go module verification,
  vet/build and android lint/debug assembly. these checks are separate from the
  behavioral evidence above.

temporary tests, their android host and dependencies, and installed probe
packages were removed. the phone's original font scale was restored. release
and production deployment are outside this change. the existing
[testing coverage gap](issues/test-system-reset.md) remains explicit.
