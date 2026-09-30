# desktop browser

implemented. 2026-09-27: one table with an agents view and group views replaces
pr 3's sidebar, agent list and session tabs; the reasoning is in §8.
[terminal control](terminal-agent-control.md) owns inferred status and ordinary
read/send/wait/stop/close for both providers and generic terminals. native
interaction remains explicitly addressed; notifications have a separate release dependency.
[darwin native acceptance](issues/desktop-browser-runtime-acceptance.md) remains
skipped. [architecture](architecture.md) owns scope; [roadmap](roadmap.md) owns
delivery/evidence; [the design language](design-language.md#19-terminal-browser)
owns visual values. this spec replaces the desktop table/picker layout and affected
navigation rules in [groups](groups.md) and [agent-control ux](agent-control-ux.md).
phone behavior is unchanged. [terminal continuity](terminal-continuity.md) owns
creation (`n`, `N`, directory search) and the current execution context rows
display. [pr 4](groups-and-shells.md) separately investigates terminal embedding.

## 1. outcome and limits

one keyboard-only browser: one table, one cursor, one scope. a header states the
scope, the table lists every session in it, and one rule names the session the
keys act on. forms, confirmations, details and explicitly requested output take
the table's place. entering a session still replaces the whole screen. detach
resumes the same browser state and refreshes.

the agents view answers the most frequent question, which agents may be waiting
on the operator, across every group. group views answer where work lives. both
are the same table over the same observations.

design for a half-screen terminal: 3–4 groups, 6–7 agents, about 3 sessions per
group; fully usable at 80 columns × 24 rows. no mouse interaction, embedding,
new dependency, host/android change, search, saved empty groups, manual ordering,
collapsing, panes, attention counters, or
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
| agents | one row per current local/remote agent across all groups, never recorded native identity; blocked, idle, unknown, working; unavailable hosts last; ties keep configured peer then host-published name/id order |
| all groups | every session, including terminals, in existing `Groups` order, each group under a heading |
| unassigned, `group: <label>` | that group's sessions in `Groups` order; named views are the sorted observations, preserving a selected empty label |

left/right (also h/l) step through agents, all groups, unassigned, then named
groups, clamping at the ends. a new view keeps the current session if it contains
it, otherwise selects its first row. up/down (also j/k) move the cursor, clamping.
`a` opens the agents view on its first row from anywhere, so after handling one
agent the next is one key away. movement never attaches or fetches output.
changing machine uses the same keep-if-matching/otherwise-first rule.

the agents view puts what may be waiting on the operator first. codex reports a
prompt/footer as inferred idle, and unknown can be an unrecognized dialog. no state
establishes task completion. the order updates on every refresh. the cursor follows
its session's lifetime, never a row position, so a reorder moves rows but never
retargets a key. an exited agent remains in terminal/group views; unknown is not
offline. the contract publishes no transition age, so nothing is
ordered by time.

named labels read `group: <label>` wherever a label could be mistaken for a special
selector: the header, group headings, and details. the agents view's group column
shows bare labels and stays blank for unassigned sessions; a label cannot be empty.

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
| ordinary navigation | `a` agents; left/right view; `n` terminal on the target machine; `N` options; `m` existing machine picker; `ctrl-r` refresh; `q/escape` quit |
| selected row | spacebar full metadata; `r` remains with the notification owner; `s` sends one interrupt on every fresh terminal, `c` then independently closes its entire session, `x` closes without input; `e` change group; `T` terminal-here retains its remote guard; metadata remains readable when unavailable |
| modal page | owns input while the header and rule stay visible; forms keep field/paste/validation keys; details/read scroll; existing confirm/cancel keys; no global navigation mnemonics |
| attached terminal | existing fullscreen tty ownership and key handling; `ctrl-] d` detaches; no new prefix commands |

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

the header is the reversed `skid` wordmark, then the scope (`agents`, `all groups`,
`unassigned` or `group: <label>`) and the machine scope (`all machines` or
`machine: <label>`), then one blank row.

the table's two-cell gutter carries the cursor mark. columns follow: name,
status, agent (the configured profile label, else `<provider> · profile
unknown`), group (agents view only), machine (the terminal's owner, only when all
machines are in scope), and the current directory in the remaining width,
truncated from the left and omitted below 8 cells; remote work reads `host:path`
and an unresolved connection `remote context unknown`. columns shrink
widest-first to fit. in all groups a faint heading precedes each group. status is
the literal state of the current context (a resolved remote agent is always
`unknown`), `terminal` without a recognized agent, `unavailable` once its host has
failed a read, or `checking` while a scoped read is still outstanding.

below the table, top to bottom: scoped notices; the rule, with the target set into
it and, only when the table scrolls, the cursor position at its end; the selected
session's state and device-local unread, attached clients and full directory; the keys.
while an action is in flight the rule names that action's captured target instead.
observed text is sanitized for display: controls, format characters such as bidi
overrides, and line separators become spaces.

hints list actions, not navigation: the selected session's verbs, then `a`,
left/right, `m`, `n` (naming its target machine), `N`, `q`. each set of hints
stays on one line when it fits and otherwise wraps by whole hints; the key is
bold, the label plain. arrows, `ctrl-r` and the vim aliases are documented in
`--help`.

pages keep the header and rule. the page title is bold, and labels right-align on
one axis. the rule names the captured target (pending action, details or output
snapshot), never the live selection. focused choice fields show `‹ value ›`,
focused text fields a caret. confirmation names its effect: `enter close terminal`
or `enter stop work and close terminal`.
bounded output remains an explicit snapshot; its title and source/scope/truncation
stay pinned while the body scrolls, and refresh cannot relabel it. page scrolling
uses the visible body height; forms keep the focused field visible.

cursor, unavailability and failure never rely on color: the cursor is a glyph
plus bold, and every status and unavailable row carries a word. nothing that must be
read is faint. below 80×24, show a resize notice, retain state, and accept only
resize and existing cancel/quit input. pending operations still complete normally.
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
| a1: arrows, `a`, modal keys and the displayed target agree; no arrow attaches | model interaction cases; verify resulting views and captured exact requests |
| a2: agents order, identity under reorder, view stepping, unavailable peers, and captured targets under refresh preserve the rules above | small deterministic observation/transition cases |
| a3: 80×24 fits the stated working set; long labels, overflow, empty/offline states, forms and resize notice stay usable in color and under NO_COLOR | rendered fixture inspection |
| a4: create → select, shell-here → exact fullscreen attach → detach on new shell; ordinary enter → detach preserves context and the first subsequent navigation key; source survives; lost reply never repeats creation | the real browser → pty → gateway → isolated tmux journey on linux and darwin |
| a5: old keys gone; help and browser agree | diff review and `scripts/check verify` |

2026-09-27: a1–a3 were shown with temporary out-of-tree fixture renders and
model checks, removed afterwards; they provide no retained regression protection.
a5 passed `scripts/check verify`. attachment and return code is unchanged, and a4
remains with [the runtime acceptance issue](issues/desktop-browser-runtime-acceptance.md).

accepted costs: chrome/status disappear while attached; navigation requires
detach; a group view hides agents elsewhere, though the agents view is one key
away; group views do not sort by urgency; appearance follows the operator's
terminal theme, and a bright-yellow cursor is weak on light themes; faint rendering
varies and disappears under mosh; refresh, including `ctrl-r`, shows no progress
indicator; hints are written per page and can drift from dispatch; `▌` and `‹ ›`
have east-asian-ambiguous width.

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
