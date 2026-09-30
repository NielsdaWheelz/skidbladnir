# desktop browser

implemented. one table with agents and group views, under a row-1 view strip,
replaces pr 3's sidebar, agent list and session tabs; the reasoning is in §8.
[native interaction](native-agent-observation.md) adds native status, separate
unread and stop/close semantics; its content contract owns those fields.
[darwin native acceptance](issues/desktop-browser-runtime-acceptance.md) remains
skipped. [architecture](architecture.md) owns scope; [roadmap](roadmap.md) owns
delivery/evidence; [the design language](design-language.md#19-terminal-browser)
owns visual values. this spec replaces the desktop table/picker layout and affected
navigation rules in [groups](groups.md) and [agent-control ux](agent-control-ux.md).
phone behavior is unchanged. [terminal continuity](terminal-continuity.md) owns
creation (`n`, `N`, directory search) and the current execution context rows
display. [pr 4](groups-and-shells.md) separately investigates terminal embedding.

## 1. outcome and limits

one keyboard-only browser: one table, one cursor, one scope. a view strip on row
1 lists the views in stepping order, counting any that overflow, and marks the
current one; the table lists every session in it, and one rule names the session
the keys act on. forms, confirmations, details and explicitly requested output
take the table's place. entering a session still replaces the whole screen.
detach resumes the same browser state and refreshes.

the agents view answers the most frequent question, which agents may be waiting
on the operator, across every group. group views answer where work lives. both
are the same table over the same observations.

design for a half-screen terminal: 3–4 groups, 6–7 agents, about 3 sessions per
group; fully usable at 80 columns × 24 rows. no mouse interaction, embedding,
new dependency, host/android change, search, saved empty groups, manual ordering,
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
retain fleetclient's cli ordering; only the agents view adds status ordering.
tmux still owns processes; groups are labels, rows present sessions, agent status
is a sampled property of a session. names and row positions are never control
identities.

the model owns scoped peer observations, the machine filter, the view (agents, or
a group filter), the selected session lifetime (or none), the table's scroll
position, and existing modal/pending-operation state. rows derive from those
observations. no independent highlighted-session, preview-session or open-tab state.

keep five-second refresh, one inventory in flight, scoped-result admission,
post-write read fences, stale-row retention, and existing operation timeouts.
machine changes require a fresh scoped read before remote actions; view changes
are local. refresh cannot rewrite a draft, captured action reference, or read snapshot.

## 3. selection and navigation

initial state: all machines, agents view; select the first row after inventory,
or none. the machine filter applies to every view.

| view | rows and order |
| --- | --- |
| agents | one row per agent in its current execution context across all groups, including resolved remote agents (ordered as unknown) and retained unavailable rows: blocked, failed, done, idle, unknown, working, stopped; unavailable hosts last; ties keep configured peer then numeric tmux-id order |
| all | every session, including terminals, in existing `Groups` order, each group under a heading |
| a named group, unassigned | that group's sessions in `Groups` order |

the views are agents, all, then the labels observed in scope plus the current
view's label in `group.Compare` order (ascii-case-folded; unassigned, the zero
label, last), so drawn, stepped and all-view heading order are one order and a
selected empty label stays. the current view is a value, never a position: a
label appearing or vanishing to its left moves its tab, never the view.

left/right (also h/l) step to the previous or next view in the strip, clamping
at the ends, never wrapping. a new view keeps the current session if it contains
it, otherwise selects its first row. up/down (also j/k) move the cursor, clamping.
`a` opens the agents view on its first row from anywhere, so after handling one
agent the next is one key away. movement never attaches or fetches output.
changing machine uses the same keep-if-matching/otherwise-first rule.

the agents view puts what may be waiting on the operator first. codex reports a
finished turn as idle, and unknown can be an unrecognized dialog; only working
affirmatively needs nothing. the order updates on every refresh. the cursor follows
its session's lifetime, never a row position, so a reorder moves rows but never
retargets a key. done keeps its native-completion meaning, never unread state;
unknown is not offline. the contract publishes no transition age, so nothing is
ordered by time.

named labels read `group: <label>` in group headings and details. the agents
view's group column shows bare labels and stays blank for unassigned sessions; a
label cannot be empty.

session actions require a selected row and target exactly the session named by
the rule. refresh retains the selected lifetime while it stays in the view;
otherwise it uses the previous index clamped to the surviving rows, or none.
never auto-attach. external membership changes never change the view.
unavailable rows retain last-observed facts, labelled unavailable; remote
actions stay disabled. once a scoped host fails a read, its notice names the
failure and shows in every view, even with no retained rows; an unobserved host
is not announced as unavailable. inventory failures never replace an action's
outcome notice, so an unknown outcome stays visible. reuse existing honest empty
copy.

## 4. actions and return

| context | keys/behavior |
| --- | --- |
| ordinary navigation | `a` agents; left/right view; `n` terminal on the target machine; `N` options; `m` existing machine picker; `ctrl-r` refresh; `q` or `ctrl-c` quit; `escape` does nothing and shows no notice |
| selected row | spacebar full metadata; `r` bounded read, `s` stop current work and `c` stop work and close terminal for local agents only; `x` close terminal; `e` change group; `T` (shift+t) terminal-here, refused for remote connections; existing remote capability/availability guards; local metadata remains readable when unavailable |
| modal page | owns input while the row 1 strip (without chevrons) and the rule stay visible; forms keep field/paste/validation keys; details scroll; escape closes or cancels, `q` closes non-text pages; no global navigation mnemonics except `ctrl-c`, which quits, discarding drafts |
| operation in flight | every key, `ctrl-c` included, is refused with the in-flight notice; nothing quits |
| attached terminal | existing fullscreen tty ownership and key handling; `ctrl-] d` detaches; `ctrl-c` reaches the provider; no new prefix commands |
| presented `r` output | enter returns; `ctrl-c` is ignored |

stop/close name and pin their target/effect before confirmation. inventory cannot
substitute a replacement process. keep the existing single pending-operation lane,
duplicate suppression, completion guards, and unknown-outcome/no-replay behavior.
modal close returns to the table; refresh reconciliation still applies.

`n` creates a terminal at home on the target machine (the machine filter,
otherwise the configured default; never the first reachable peer) in the selected
named group. `N` opens the five-field machine/launch/name/cwd/group form; `z
<words>` in its directory field searches visited directories on that machine, and
a chosen path only edits the draft. `n`, `N` and `T` share one completion path:
it reveals and selects the returned session in its group view, leaving the agents
view, then attaches it. the pending request and page adopt the completion: `n`
and `T` from the table, `N` from its form. detach or attachment failure leaves
the new terminal selected; retry attachment, never creation.
failure/unknown creation changes no filters/selection. reuse
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
`agents`, `all`, then each group as its bare label or `unassigned`; a label that
prints as `agents`, `all` or `unassigned`, or begins `group: `, reads
`group: <label>`, so no label reads as one of skid's words except through
invisible characters (§7's costs). the current tab is bold,
between gold `‹ ›` at the table while no operation is in flight; nothing in the
strip is faint. skid's words never truncate; `agents`, `all` and the current tab
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
unknown`), group (agents view only), machine (the terminal's owner, only when all
machines are in scope), and the current directory in the remaining width,
truncated from the left and omitted below 8 cells; remote work reads `host:path`
and an unresolved connection `remote context unknown`. columns shrink
widest-first to fit. in the all view a faint heading (`group: <label>` or
`unassigned`) precedes each group. status is the status text alone:
`working`, `waiting`, `idle`, `done`, `failed`, `stopped` or `status unavailable`
for a tracked conversation; `conversation not tracked` for untracked local codex;
`status unavailable` for any other agent, every resolved remote agent included,
and any unresolved remote connection;
`terminal` without a recognized agent; `unavailable` once its host has failed a
read; or `checking` while a scoped read is still outstanding. unavailable and
checking rows are faint, name included.

below the table, top to bottom: scoped notices; the rule, with the target set into
it and, only when the table scrolls, the cursor position at its end; the selected
session's association, state and device-local unread, attached clients and full
directory; the keys.
while an action is in flight the rule names that action's captured target instead.
observed text is sanitized for display: controls, format characters such as bidi
overrides, and line separators become spaces.

hints list actions, not navigation: the selected session's verbs, then
`a agents  ←→ view  m machine  n terminal on <host>  N options  q quit`. each set
of hints stays on one line when it fits and otherwise wraps by whole hints; the
key is bold, the label plain. navigation and session keys are in `--help`.

pages keep row 1 and the rule. the page title is bold, and labels right-align on
one axis. the rule names the captured target (pending action, details or output
snapshot), never the live selection. focused choice fields show `‹ value ›`,
focused text fields a caret. confirmation names its effect: `enter close terminal
only`, `enter stop tracked conversation` or `enter stop tracked conversation and
close terminal`.
bounded output remains an explicit snapshot; its title and source/scope/truncation
stay pinned while the body scrolls, and refresh cannot relabel it. page scrolling
uses the visible body height; forms keep the focused field visible.

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
styles are `x/ansi` values over the sixteen basic colors. bubble tea downsamples
per detected profile and honours NO_COLOR. layout works on plain sanitized text
and styles only finished fragments, each closing its own style. styled text never
passes back through sanitization, which would turn escapes into spaces. no public
component api, presentation framework or new dependency: lipgloss 2.0.6 would pull
an ultraviolet revision that bubble tea 2.0.9 was not released with.

## 7. acceptance and delivery

| criterion | proof |
| --- | --- |
| a1: arrows, `a`, modal keys, the view strip and the displayed target agree: the strip lists the views in stepping order and marks the current one by value; no arrow attaches | model interaction cases; verify resulting views and captured exact requests |
| a2: agents order, identity under reorder, view stepping, unavailable peers, and captured targets under refresh preserve the rules above | small deterministic observation/transition cases |
| a3: 80×24 fits the stated working set; long labels, strip overflow (shared cap, counted markers, the current view whole and never hidden, a long machine filter), empty/offline states, forms and resize notice stay usable in color and under NO_COLOR | rendered fixture inspection |
| a4: create → select, shell-here → exact fullscreen attach → detach on new shell; ordinary enter → detach preserves context and the first subsequent navigation key; escape after `ctrl-] d` stays in skid; `ctrl-c` quits; source survives; lost reply never repeats creation | the real browser → pty → gateway → isolated tmux journey on linux and darwin |
| a5: old keys gone; escape never quits, `q` quits from the table, `ctrl-c` quits from any browser frame unless an operation is in flight; help and browser agree | diff review, `scripts/check verify`, and the real binary under a pty: a bare escape keeps it running, `q` and 0x03 exit 0 |

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

accepted costs:
- attached: chrome/status disappear; navigation requires detach.
- views: a group view hides agents elsewhere, though the agents view is one key
  away; group views do not sort by urgency. unassigned sorts last, and its tab
  comes and goes with observation.
- `ctrl-c` quits from any browser frame, discarding drafts. in flight it is
  refused for at most the 15 s timeout; a held or repeated `ctrl-c` then quits
  as soon as the outcome arrives, so an unknown outcome can go unread, and a
  restart keeps no record of it. after `n`, `N` or `T` it is refused and the new
  terminal still attaches: detach, then quit. a reflexive `ctrl-c` after
  `ctrl-] d` quits skid.
- long labels: at 80 columns they share one cap, so prefix twins (`infra-…`)
  read alike until one is current; beyond about four long groups in a group
  view, or six from agents or all, tabs hide behind counted markers; a label
  containing spaces can read as several tabs until it is current; truncated
  labels, and labels shaped like `←n` or `n→`, can read alike. every group view
  reserves room for the widest label whole, so with a machine filter at 80
  columns one label of 24 cells can leave a short current label alone between
  markers. labels differing only in invisible characters read alike, and one can
  read as `all` or `agents`
  ([issue](issues/group-label-invisible-characters.md)).
- association: `status unavailable` reads the same for a tracked conversation
  whose status is unavailable, an untracked non-codex agent, a resolved remote
  agent (codex included) and an unresolved remote connection. tracking shows
  only on the selected row's facts line and in details.
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
needed a freeze-while-focused rule. the agents view keeps that list's purpose
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
