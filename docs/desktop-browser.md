# desktop browser

2026-10-02: [session views](session-views.md) owns the exclusive needs-input view,
queue membership/order and navigation keys. source implements this cutover;
that specification owns its qualification. earlier evidence below retains its
recorded source boundary.

2026-10-02: [searchable group entry](group-entry.md) specifies local group matching
and explicit create/save actions after group. source implements that contract;
[qualification](group-entry.md#qualification) records the focused evidence.
the broader browser delivery status below remains separate.

implemented. one table with needs-input, all and group views, under a row-1 view strip,
replaces pr 3's sidebar, agent list and session tabs; the reasoning is in §8.
[terminal observation](terminal-observation.md) owns inferred status, its copy
and the strict request predicate; [terminal control](terminal-agent-control.md) owns
ordinary read/send/wait/stop/close. [terminal attention](reply-notifications.md)
owns notices and visits. native history/control remains explicitly addressed.
[darwin native acceptance](issues/desktop-browser-runtime-acceptance.md) remains
skipped. [architecture](architecture.md) owns scope; [roadmap](roadmap.md) owns
delivery/evidence; [the design language](design-language.md#19-terminal-browser)
owns visual values. this spec replaces the desktop table/picker layout and affected
navigation rules in [groups](groups.md) and [agent-control ux](agent-control-ux.md).
this document governs desktop behavior. [terminal continuity](terminal-continuity.md) owns
creation (`n`, `N`, directory search) and the current execution context rows
display. [pr 4](groups-and-shells.md) separately investigates terminal embedding.

## 1. outcome and limits

one keyboard-only browser: one table, one cursor, one scope. a view strip on row
1 lists the views in stepping order, counting any that overflow, and marks the
current one; the table lists every session in it, and one rule names the session
the keys act on. forms, confirmations, details and explicitly requested output
take the table's place. entering a session still replaces the whole screen.
detach resumes the same browser state and refreshes.

the needs-input view collects qualified ready, response requests and current
error/interruption notices across groups, ready first. group views answer where
work lives. [session views](session-views.md) owns membership, ordering and
removal; both use the same table and admitted observations.

design for a half-screen terminal: 3–4 groups, 6–7 agents, about 3 sessions per
group; fully usable at 80 columns × 24 rows. no mouse interaction, embedding,
new dependency, host/android change, table search, saved empty groups, manual ordering,
collapsing, panes, attention counters or badges, or
navigation history. one current selection and scroll position; no disk state.

## 2. composition and data contract

```text
sessionui (one bubble tea model)
  -> fleetclient.Execute(Request) -> existing gateway/session/agent operations
  -> tea.Exec -> terminalclient.Run -> existing websocket -> direct tmux client
```

no new public api, route, dto, config, or persisted schema. reuse `Peer`, `Session`,
`Reference.SessionEqual`, `group.Filter`, `fleetclient.Groups` and `ObservedGroups`.
retain fleetclient's cli ordering; only needs input adds queue-tier ordering.
tmux still owns processes; groups are labels, rows present sessions, agent status
is a sampled property of a session. names and row positions are never control
identities.

the model owns scoped peer observations, the machine filter, one selected view
(needs input, all or a group), the selected session lifetime (or none),
the table's scroll position, and existing modal/pending-operation state. rows
derive from those observations. no independent highlighted-session,
preview-session or open-tab state.

keep five-second refresh, one inventory in flight, scoped-result admission,
post-write read fences, stale-row retention, and existing operation timeouts.
machine changes require a fresh scoped read before remote actions; view changes
are local. refresh cannot rewrite a draft, captured action reference, or read snapshot.

## 3. selection and navigation

initial state: all machines, all view; select the first row after inventory,
or none. the machine filter applies to every view.

| view | rows and order |
| --- | --- |
| needs input | fresh local agents with qualified ready first, then explicit requests/current error or interruption notices; ties keep configured peer then numeric tmux-id order |
| all | every session, including terminals, in existing `Groups` order, each group under a heading |
| a named group, unassigned | that group's sessions in `Groups` order |

the views are needs input, all, then the labels observed in scope plus the current
view's label in `group.Compare` order (ascii-case-folded; unassigned, the zero
label, last), so drawn, stepped and all-view heading order are one order and a
selected empty label stays. the current view is a value, never a position: a
label appearing or vanishing to its left moves its tab, never the view.

left/right (also h/l) step to the previous or next view in the strip, clamping
at the ends, never wrapping. a new view keeps the current session if it contains
it, otherwise selects its first row. up/down (also j/k) move the cursor, clamping.
`f` selects needs input through ordinary view selection; repeated `f` is a
no-op. movement never attaches or fetches output. changing machine uses the
same keep-if-matching/otherwise-first rule. `a` has no obsolete action.

the queue rederives after admitted inventory/attention and attachment
acknowledgement. its cursor follows the exact session lifetime through reorder.
quiet idle, plain working/unknown, menus alone, remote agents and stale rows
stay outside it. all/groups retain those rows for inspection. readiness is
device-local and inferred; no state proves successful task completion.

named tabs are double-quoted; headings/details and the queue's group column
keep bare labels. unassigned rows have a blank group column. current hidden
menu notices appear in queue status/detail so inclusion has a visible reason.

session actions require a selected row and target exactly the session named by
the rule. refresh retains the selected lifetime while it stays in the view;
otherwise it uses the previous index clamped to the surviving rows, or none.
never auto-attach. external membership changes never change the view.
rows of a host whose read failed retain their last-observed facts, their status
cell reading `last observed: <label>`, also while that host is re-read; rows of
any other host being re-read read `checking` (§5). remote actions stay disabled
on both. once a scoped host fails a read, its notice names the failure and shows
in every view, even with no retained rows; an unobserved host is not announced
as unavailable. inventory failures never replace an action's outcome notice, so
an unknown outcome stays visible. reuse existing honest empty copy.

## 4. actions and return

| context | keys/behavior |
| --- | --- |
| ordinary navigation | `f` needs input; left/right view; `n` terminal on the target machine; `N` options; `m` existing machine picker; `ctrl-r` refresh; `q` or `ctrl-c` quit; `escape` does nothing and shows no notice |
| selected row | spacebar opens info; `s` sends one interrupt on every fresh terminal; `x` sends one interrupt, then independently closes the entire session; `T` terminal-here retains its remote guard; info remains readable when unavailable |
| info | `r` edits name; `g` edits group; arrows/j/k and page keys scroll; escape or `q` returns to the table |
| name editor | enter saves; escape cancels or dismisses to the same info page; `ctrl-a` offers `use automatic title` in manual mode |
| group editor | [group entry](group-entry.md#desktop-interaction) owns search/select and the separate save focus stop; escape cancels or dismisses to the same info page |
| modal page | owns input while the row 1 strip (without chevrons) and the rule stay visible; forms keep field/paste/validation keys; details scroll; escape closes or cancels, `q` closes non-text pages; no global navigation mnemonics except `ctrl-c`, which quits, discarding drafts |
| operation in flight | every key, `ctrl-c` included, is refused with the in-flight notice; nothing quits |
| attached terminal | existing fullscreen tty ownership and key handling; `ctrl-] d` detaches; `ctrl-c` reaches the provider; no new prefix commands |

stop/close name and pin their target/effect before confirmation. inventory cannot
substitute a replacement pane or session lifetime. keep the existing single pending-operation lane,
duplicate suppression, completion guards, and unknown-outcome/no-replay behavior.
info owns a captured session lifetime, independently of table selection and
group filtering. its name, naming mode and group appear first, with `r`/`g`
beside the editable rows. both editors resolve that lifetime from underlying
scoped inventory, so regrouping or agent exit can remove its table row without
retargeting or disabling info. unavailable or replaced lifetimes disable editing.
the table has no metadata-edit shortcut.

confirmed saves return to the same info after a post-write inventory read;
cancellation also returns there. assignment never changes the table's view.
unknown writes are never replayed: preserve the draft and uncertainty while
checking, including after dismissal to info. matching fresh state resolves the
editor with an unknown-outcome notice; differing state retains the draft for
review, reopened with its `r`/`g` key. a definite rejection never becomes success
because another writer happens to match the draft. the machine picker and `N`
options are unavailable while checking, keeping the unresolved target in the
existing scoped refresh. other pages still close to the table.

the name editor follows [automatic/manual ownership](automatic-session-names.md#client-composition):
saving a name selects manual naming, including unchanged text in automatic
mode. unchanged manual names disable save. `use automatic title` bypasses draft
validation; automatic reconciliation compares ownership, never a predicted title.
title updates do not rewrite the draft; changed manual naming or ownership
retains it with a conflict notice and a fresh expectation for deliberate resubmission.

`n` creates a terminal at home on the target machine (the machine filter,
otherwise the configured default; never the first reachable peer) in the selected
named group. `N` opens machine/launch/name/cwd/group followed by an explicit
`create` action, under [group entry](group-entry.md#desktop-interaction). its
directory field starts empty, visibly defaulting to home: typing ordinary words
searches that machine's visited directories after a 150 ms pause, without a `z`
prefix, enter, or a separate page. `/…` and `~…` stay literal path drafts;
creation still admits only absolute paths and exact `~`/`~/…` expansion.
the first ranked match appears beneath the unchanged query, with its full path
and position. left/right cycles the matches with wraparound. tab or enter accepts
the selected path and advances; shift-tab goes back without accepting. ctrl-u
clears the field to home. empty results, pending search and failures cannot be
accepted or submitted as a cwd. the create action returns to directory if a
query has not been accepted, including one bypassed with shift-tab.
machine changes restore the empty home default;
editing, cancellation and machine changes invalidate and cancel pending search.
late results cannot alter another draft. selection only edits the draft;
creation uses the separate create action and revalidates the chosen path.
`n`, `N` and `T` share one completion path:
it retains an all/group view that admits the returned session; otherwise it
selects the returned group, leaving needs input on confirmed creation. it reveals
and selects that exact session, then attaches it. the pending request
and page adopt the completion: `n` and `T` from the table, `N` from its form.
detach or attachment failure leaves the new terminal selected; retry attachment,
never creation. failure/unknown creation changes no filters/selection. reuse
[shell completion](shells.md#4-client-ownership-and-completion) and
[membership/creation rules](groups.md#7-collection-behavior-and-creation).

ordinary detach resumes the table with the current view, session and scroll, then
refreshes. it does not reset the view, restore a source, or recall earlier groups.
changing views retains no old scroll history; keep the new selection visible.
restarting skid begins fresh. unchanged attachment still owns tty restoration,
input-reader cancellation/joining, geometry, and session preservation.

## 5. presentation

row 1 is the reversed ` skid ` wordmark, the view strip and, only when a machine
filter narrows the scope, `machine: <label>` (at most 24 cells) at the right
edge; its absence means all machines. one blank row follows. the strip reads
`needs input`, `all`, then each named group in double quotes or the unquoted
`unassigned` view. quotes distinguish group labels from skid's selector words;
truncate long labels inside the quotes.
the current tab is bold,
between gold `‹ ›` at the table while no operation is in flight; nothing in the
strip is faint. skid's words never truncate; `needs input`, `all` and the current tab
never hide. other labels share one cap, 24 cells down to 7, that does not change
while the current view moves between groups; the current label is whole up to 24
cells unless a long machine filter leaves no room. past the floor, the groups
farthest from the current tab hide behind `←n` and `n→`, which count tabs, never
work; once groups hide, a step between groups never moves a marker against it.
from 80 columns row 1 fits in width − 1. with nothing hidden, a step between
groups changes the text of only the two tabs it crosses; entering the groups
from `all` may reflow the strip once.

the table's two-cell gutter carries the cursor mark. columns follow: name,
status, agent (the configured profile label, else `<provider> · profile
unknown`), group (needs-input view only), machine (the terminal's owner, only when all
machines are in scope), and the current directory in the remaining width,
truncated from the left and omitted below 8 cells; remote work reads `host:path`
and an unresolved connection `remote context unknown`. columns other than
status shrink widest-first to fit. in the all view a faint heading (the bare
label or `unassigned`) precedes each group. the status cell is the shared projection's
label in [observation §6](terminal-observation.md#6-content-attention-and-filtering)
copy, coloured by its tone. only in needs input, a menu with a qualifying notice
adds that notice to the status cell and selected detail, as specified by
[session views](session-views.md#membership-and-ordering).
[terminal attention](reply-notifications.md) owns
exclusive green `ready`, durable non-idle-to-idle memory and visit boundaries.
creation responses enter the same scoped owner before auto-entry. only actual
output presentation acknowledges ready; armed work survives entry and departure.
recorded native identity never supplies status. shell/remote rows use
`terminal`/existing unknown context. a row of a host whose read failed is stale:
its cell reads muted `last observed: <label>` and is never ready, and it stays
stale while that host is re-read. a row of any other host being re-read (a
pending scoped read, or the re-read after a metadata change) is never ready, and
its cell makes no status claim: it reads faint `checking`. a failed host keeps
its notice. the stale cell is up to 33 cells (`last observed: status
unavailable`); the longest ordinary fresh label is 18 cells, while queue-only
menu/notice detail can be wider. other columns give way to status (in an 80×24
render with a failed host, names
fell from 24 to 12 cells). the outage form is accepted with that cost, an
exception to [observation §6](terminal-observation.md#6-content-attention-and-filtering)'s
fit rule; the 8-cell `checking` widens nothing.

below the table, top to bottom: scoped notices; the rule, with the target set into
it and, only when the table scrolls, the faint cursor position (`i of n`) at its
end; the selected session's facts on one line:
`unavailable; ` or `checking; ` for a row of an unavailable or checking host, the
status label (`last observed: <label>` on those rows) with `work continues` and
`inferred from terminal` when they apply, `: <command>` for a non-agent program,
then `· N attached`; its directory on a separate line; the keys. for an available
local session, the directory line ends with `shift+t new shell here`, with the
key bold and the label plain. reserve two spaces before the action and truncate a
long directory from the left so its final components remain visible.
the action is absent for ssh/mosh sources, unavailable sessions, modal pages and
pending operations. it creates and enters an independent shell on the named
target's machine, in its current directory and group; the original session keeps
running. while an action is in flight the rule names that action's captured
target instead. observed text is sanitized for display: controls, format
characters such as bidi overrides, and line separators become spaces.

hints list actions, not navigation: the selected session's remaining verbs, then
`f needs input  m machine  n terminal on <host>  N options  q quit`. each
set of hints stays on one line when it fits and otherwise wraps by whole hints; the
key is bold, the label plain. the global keys fit one 80-column line for host
labels up to 9 cells; a longer label wraps them and costs one table row. the
strip's `‹ ›` chevrons, shown exactly when stepping is possible, and `--help` teach
←→. the directory owns the shell-here hint; do not repeat it in this list.
navigation and session keys are in `--help`.

pages keep row 1 and the rule. the page title is bold, and labels right-align on
one axis. the rule names the captured target (pending action or details
snapshot), never the live selection. focused choice fields show `‹ value ›`,
focused text fields a caret. confirmation names its effect: `enter close terminal
only`, `enter interrupt terminal` or `enter interrupt and close terminal`.
info pins the captured session lifetime; refresh may update its observed name
and facts without retargeting. its `state` fact is the status label with `work
continues` and `inferred from terminal` when they apply, followed by a `status
reason` fact that ends with `; open the terminal to inspect` when the label is
`status unknown` or `status unavailable`, stale included. when the session leaves
inventory or its host fails, the page keeps its last observed facts and their
`observed` time, with `state: last observed: …` and `availability: unavailable`.
page scrolling uses the visible body height; forms keep the focused field visible.

cursor, current view, unavailability and failure never rely on color: the cursor
is a glyph plus bold, the current view is bold, and every status and unavailable
row carries a word. nothing that must be read is faint. below 80×24, show
`80 × 24 minimum; ctrl-c quits` above the current outcome notice, wrapped, retain
state, and accept only resize, escape to close or cancel a page, `q` to close
details or the machine picker, `q` or `n` to cancel a confirmation, `q` to quit
from the table, and `ctrl-c`. pending operations still complete normally.
do not change fullscreen terminal geometry/admission.

## 6. implementation boundary

`internal/sessionui` owns state, projection, layout and existing actions as one
cohesive bubble tea model; `internal/agentcli` owns the browser's help text.
styles are `x/ansi` values: bright slots for navigation and ember, explicit
frost/moss/muted RGB; the projected tone, never the printed word, selects one.
bubble tea downsamples per detected profile; NO_COLOR strips styles without
losing labels. layout works on plain sanitized text and styles only finished
fragments, each closing its own style. styled text never passes back through
sanitization, which would turn escapes into spaces. no public component api,
presentation framework or new dependency: lipgloss 2.0.6 would pull an
ultraviolet revision that bubble tea 2.0.9 was not released with.

## 7. acceptance and delivery

desktop directory search acceptance: open `N`, reach directory and type familiar
words without clearing home; see the first full path without enter; cycle left
and right, then tab directly to group. create must use that selected path. blank
and ctrl-u mean home; literal paths containing spaces or `z` remain literal.
typing/paste must discard old matches immediately. delayed replies after another
query, machine change or cancellation cannot supply a cwd. no matches and search
failure keep the query editable; no pending or unresolved query can create a
session. the [directory qualification issue](issues/terminal-continuity-directory-qualification.md)
records the boundary of current evidence.

| criterion | proof |
| --- | --- |
| a1: arrows, `f`, modal keys, the view strip and the displayed target agree: the strip lists the views in stepping order and marks the current one by value; no arrow attaches; repeated `f` is a no-op and `a` has no action | model interaction cases; verify resulting views and captured exact requests |
| a2: needs-input ordering, identity under reorder, view stepping, unavailable peers, and captured targets under refresh preserve the rules above | small deterministic observation/transition cases; queue qualification belongs to [session views](session-views.md#acceptance) |
| a3: 80×24 fits the stated working set; long labels, strip overflow (shared cap, counted markers, the current view whole and never hidden, a long machine filter), empty/offline states, forms and resize notice stay usable in color and under NO_COLOR | rendered fixture inspection |
| a4: create → select, shell-here → exact fullscreen attach → detach on new shell; ordinary enter → detach preserves context and the first subsequent navigation key; escape after `ctrl-] d` stays in skid; `ctrl-c` quits; source survives; lost reply never repeats creation | the real browser → pty → gateway → isolated tmux journey on linux and darwin |
| a5: old keys gone; escape never quits, `q` quits from the table, `ctrl-c` quits from any browser frame unless an operation is in flight; help and browser agree | diff review, `scripts/check verify`, and the real binary under a pty: a bare escape keeps it running, `q` and 0x03 exit 0 |
| a6: info alone owns name/group editing; cancellation and reconciled saves return to its exact lifetime, including after leaving the table's view; automatic/manual ownership, conflict drafts, post-write fences and unknown/no-replay behavior are preserved | temporary model and loopback HTTP checks; actual browser/gateway/isolated-tmux editing journey remains with the runtime acceptance issue |

2026-09-27: a1–a3 were shown with temporary out-of-tree fixture renders and
model checks, removed afterwards; they provide no retained regression protection.
a5 passed `scripts/check verify`. attachment and return code is unchanged, and a4
remains with [the runtime acceptance issue](issues/desktop-browser-runtime-acceptance.md).

2026-09-29, view strip and keys: temporary model checks (the strip's order,
unassigned and collisions, the current view under refresh, overflow at 80 and
100 columns, chevrons, escape, `ctrl-c`, the status cell) failed on `dd06814` and
passed on the change; guards for page keys, busy refusal, the facts line, the
cursor, keep-if-matching, `a`, creation reveal and the machine picker passed on
both. a property check of the strip covered 89,776 steps between groups, and 25
of 26 deliberately wrong implementations failed at least one check. under a pty
with isolated state, `dd06814` exited on a bare escape and ignored 0x03; the
change stayed up on escape and exited 0 on `q` and 0x03. the checks were deleted
before commit; `scripts/check verify` passed. a4 remains `NOT_RUN` by the
user's choice.

2026-09-30, info-owned metadata editing (`codex/info-editors`, baseline `b4a4711`):
two temporary cases first failed on the baseline: info lost a regrouped lifetime,
and the table still owned group editing. the change passed ten model cases and
three loopback HTTP checks, including manual takeover/reset, title ticks,
conflicts and draft rebasing, lost acknowledgements without replay, post-write
read fences, regrouping/return, replacement rejection, dismissed-draft recovery,
machine-scope retention, and long fields at 80×24. independent review found and
resolved dismissal and cross-host-creation traps. temporary tests were removed;
`scripts/check verify` passed. actual browser/gateway/tmux editing remains
`NOT_RUN` under the [runtime acceptance issue](issues/desktop-browser-runtime-acceptance.md).

accepted costs:
- attached: chrome/status disappear; navigation requires detach.
- views: a group view hides sessions elsewhere; needs input is one key away for
  qualified attention, and all retains the full roster. group views do not sort
  by urgency. unassigned sorts last, and its tab
  comes and goes with observation.
- `ctrl-c` quits from any browser frame, discarding drafts. in flight it is
  refused for at most the 15 s timeout; a held or repeated `ctrl-c` then quits
  as soon as the outcome arrives, so an unknown outcome can go unread, and a
  restart keeps no record of it. after `n`, `N` or `T` it is refused and the new
  terminal still attaches: detach, then quit. a reflexive `ctrl-c` after
  `ctrl-] d` quits skid.
- long labels: at 80 columns they share one cap, so prefix twins (`infra-…`)
  can read alike until one is current; excess tabs hide behind counted markers.
  quotes distinguish label boundaries and skid's words, but consume two cells
  per named tab; the longer needs-input label also leaves less room. every group view
  reserves room for the widest label whole, so with a machine filter at 80
  columns one label of 24 cells can leave a short current label alone between
  markers. labels differing only in invisible characters can still read alike
  ([issue](issues/group-label-invisible-characters.md)).
- inference: unknown/unavailable layouts do not establish idle; sampled non-idle-to-idle
  attention may miss work between polls or treat cancellation/navigation as ready.
  recorded native identity is secondary and cannot repair that epistemic limit.
- appearance follows the operator's terminal theme. bright yellow (the cursor bar
  and the view chevrons) is weak on light themes; bold carries both marks. faint
  rendering varies and disappears under mosh.
- refresh, including `ctrl-r`, shows no progress indicator; hints are written per
  page and can drift from dispatch.
- `▌`, `← →` and `…` have east-asian-ambiguous width. the strip can fill row 1,
  so an ambiguous-wide terminal can wrap it.

## 8. why one table

pr 3 put the few short things, 3–4 group labels, on the long vertical axis and the
many long things, sessions, on one horizontal strip. at 80 columns the default view
showed 3 of 11 sessions. its agent list was a sorted projection of sessions rather
than a concept, so one session had two cursors. arrows meant different things
per region, selecting an agent silently rewrote the group filter, and the sort
needed a freeze-while-focused rule. the needs-input view keeps that list's purpose
as a view of the same table. its status color gives a single-feature target that
the eye finds in parallel, so group views need no urgency sort and keep their rows
still.

the view strip is the referent left/right always lacked: they walked a hidden,
clamped sequence of views, and row 1 draws it. it names views, never sessions,
so pr 3's horizontal session strip does not return: views are few and short,
and row 1 was already spent on the header. escape settles on the table and
never leaves skid, because codex and claude train a reflexive escape, often
pressed right after `ctrl-] d`. rows show the status word alone: the
association is a fact of the selected row, not a column every row repeats.

rejected: a persistent agent sidebar, which spends width on a second rendering
of the table's rows (pr 3's two cursors); badges or marks on tabs, since status
is sampled and incomplete (a finished codex turn reads `idle`, `unknown` may be
a dialog), so a missing badge would claim nothing needs you; an attention row or
visit memory, which would stand indefinitely over agents idle for long periods;
digits, tab/shift-tab or `[ ]`, since tabs come from observed labels and a digit
would name different views on different days; escape → agents, which would jump
views on the reflexive escape after `ctrl-] d`; a seam glyph between the special
views and groups, which position and the collision prefix make redundant;
attached awareness (a tmux status line or title); printing the last outcome on
exit, a new surface where the `ctrl-c` cost is stated instead.
