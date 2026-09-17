# organized desktop browser — pr 3

2026-09-17: source implemented; [verification](roadmap.md#desktop-browser--source-implemented-runtime-acceptance-open)
tracks the remaining native acceptance. [architecture](architecture.md)
owns scope; [roadmap](roadmap.md) owns delivery/evidence. this spec replaces the
desktop table/picker layout and affected navigation rules in [spaces](spaces.md)
and [agent-control ux](agent-control-ux.md). phone behavior is unchanged.
[pr 4](spaces-and-shells.md) separately investigates terminal embedding.

## 1. outcome and limits

one keyboard-only browser: spaces above agents in a left sidebar, session tabs
above a main area containing summary, forms, details, or explicitly requested
output. entering a session still replaces the whole screen. detach resumes the
same browser state and refreshes.

design for a half-screen terminal: 3–4 spaces, 6–7 agents, about 3 sessions per
space; fully usable at 80 columns × 24 rows. no mouse interaction, embedding,
new dependency, host/android change, search, saved empty spaces, manual ordering,
collapsing, resizable panes, counters, new status semantics, or navigation history.
one current selection and current scroll positions; no disk state.

## 2. composition and data contract

```text
sessionui (one bubble tea model)
  -> fleetclient.Execute(Request) -> existing gateway/session/agent operations
  -> tea.Exec -> terminalclient.Run -> existing websocket -> direct tmux client
```

no new public api, route, dto, config, or persisted schema. reuse `Peer`, `Session`,
`Reference.SessionEqual`, `space.Filter`, `fleetclient.Groups` and `ObservedSpaces`.
retain fleetclient's cli ordering; only the tui agent view adds status ordering.
tmux still owns processes; spaces are labels, tabs present sessions, agents are
current observations. names and row positions are never control identities.

the model owns scoped peer observations, machine/space filters, selected session
lifetime (or none), focused region (`spaces | agents | tabs`), current viewport
positions, and existing modal/pending-operation state. region rows derive from
those observations. retain only the current agent order while its region is focused.
no independent highlighted-session/preview-session/open-tab state.

keep five-second refresh, one inventory in flight, scoped-result admission,
post-write read fences, stale-row retention, and existing operation timeouts.
machine changes require a fresh scoped read before remote actions; space changes
are local. refresh cannot rewrite a draft, captured action reference, or read snapshot.

## 3. selection and navigation

initial state: all machines, all spaces, tabs focused; select the first tab after
inventory, or none. filters intersect; the agent list ignores only the space filter.

| region | contents/order | arrows | enter |
| --- | --- | --- | --- |
| spaces | all spaces, unassigned, sorted named observations; preserve the selected empty named label | select/filter immediately; keep current session if it matches, otherwise first tab | focus tabs |
| agents | one row per observed agent in machine scope, including retained unavailable rows | select that session, set its exact named/unassigned space, update tabs/summary | attach selected agent's session |
| tabs | all matching sessions, including shells; existing `Groups` order | select session and update summary | attach selected session |

named labels use `space: <label>` to distinguish names from special selectors.
arrows clamp at ends; up/down (also j/k) for lists, left/right (also h/l) for tabs.
movement never attaches or fetches output. changing machine uses the same
keep-if-matching/otherwise-first selection rule as changing space.

`g/a/t` or tab/shift-tab move focus without changing selection. focus aligns with
the active space/session. if the selected session has no agent row, agents focus
has no active row; the first down/up selects the first/last agent. show that hint.
session actions require an active agent/tab row and target exactly the session
identified by the main summary. spaces focus admits no session actions.

status order: blocked, failed, done, working, idle, stopped, unknown; unavailable
hosts last. ties retain configured peer then host-published name/id order. while
agents has focus, retain survivors' relative order, update facts, remove confirmed
departures, and append arrivals; sort again on leaving. unknown is not offline;
done retains its existing native-completion meaning, never unread state.

refresh retains selected lifetime while it matches the filters; otherwise use
its previous tab index clamped to the surviving tabs, or none. never auto-attach.
an agent disappearing while its session remains does not change session selection;
agents may then have no active row. external membership changes never change the
space filter. unavailable rows retain last-observed facts, labelled unavailable;
remote actions stay disabled. show scoped unavailable-host notices independently
of space filtering, even with no retained rows. reuse existing honest empty copy.

## 4. actions and return

| context | keys/behavior |
| --- | --- |
| ordinary navigation | `n` create; `m` existing machine picker; `ctrl-r` refresh; `q/escape` quit |
| selected agent/tab | spacebar full metadata; `r` bounded read; `i` interrupt; `s` stop; `x` kill; `e` membership; `T` (shift+t) terminal-here; existing remote capability/availability guards; local metadata remains readable when unavailable |
| modal page | owns input while chrome remains visible/inactive; forms keep field/paste/validation keys; details/read scroll; existing confirm/cancel keys; no global navigation mnemonics |
| attached terminal | existing fullscreen tty ownership and key handling; `ctrl-] d` detaches; no new prefix commands |

stop/kill name and pin their target/effect before confirmation. inventory cannot
substitute a replacement process. keep the existing single pending-operation lane,
duplicate suppression, completion guards, and unknown-outcome/no-replay behavior.
modal close returns to its initiating region; refresh reconciliation still applies.

`n` retains the five-field machine/launch/name/cwd/space form, including terminal
with zero profiles and existing defaults. confirmed creation reveals/selects the
returned session and focuses tabs; it does not attach. `T` uses the same
completion path, then attaches that exact new shell. detach or attachment failure
leaves the shell selected; retry attachment, never creation. failure/unknown
creation changes no filters/selection. reuse [shell completion](shells.md#4-client-ownership-and-completion)
and [membership/creation rules](spaces.md#7-collection-behavior-and-creation).

ordinary detach resumes summary with current filters/session/focus/scroll, then
refreshes. it does not reset to all/all, restore a source, or recall earlier spaces.
changing filters retains no old viewport history; keep the new selection visible.
restarting skid begins fresh. unchanged attachment still owns tty restoration,
input-reader cancellation/joining, geometry, and session preservation.

## 5. presentation

use a 26-column sidebar and a single divider. spaces use only needed rows, capped
at half the sidebar; agents use the remainder, one row per agent. reserve notices
and contextual key hints before laying out content. four named spaces plus the
two special choices and seven agents fit at 80×24 without sidebar scrolling.
overflow lists scroll independently; tabs remain one horizontally scrolling row,
with selected tab visible and overflow indicated. no wrapping tab bar.

summary: session name, machine, space, cwd, provider/profile or shell,
state/source/reason, attached clients, availability, relevant actions. narrow rows
and summary values truncate by display cells; spacebar opens complete scrollable
metadata. no summary scrolling mode or fourth navigation region.
forms, suggestions, metadata and notices wrap/scroll within the main area; the
focused field and confirm/cancel controls stay visible. bounded output remains
an explicit snapshot with its captured machine/session header and
source/scope/truncation; refresh cannot relabel it as the newly selected tab's output.
long captured names/machines truncate independently; coverage, confirmation effects,
and a scrollable output body keep their space. page scrolling uses the visible body height.

distinguish focus, selected state and unavailable state without relying on color.
reuse existing ansi width/sanitization and restrained design-language emphasis;
no fonts, icons, ornament or motion required. below 80×24, show a resize notice,
retain state, and accept only resize and existing cancel/quit input. pending
operations still complete normally. do not change fullscreen terminal geometry/admission.

## 6. ownership, reuse and hard cut

| owner | exclusive implementation files/responsibility |
| --- | --- |
| browser builder | `internal/sessionui/**`: state, projections, layout, existing actions, colocated behavioral reds/greens; keep this cohesive, not parallel model/view builders |
| help builder | `internal/agentcli/run.go`, `internal/agentcli/run_test.go`: browser usage and matching help assertions only |
| journey builder | `tests/integration/shell_clients_test.go`: adapt existing real browser/pty/gateway/isolated-tmux journey and its file-local helpers |
| root integrator | this spec and affected docs; shared integration fixtures only if required; final composition/review; no new gate/dependency/catalog changes |
| verifier | read-only review and authorized checks; writes no production/test files |

builders own their reds and observe failure before implementation. help/journey
work can proceed alongside the browser against this contract; final green needs
the composed tree. shared edits go through root. use small cohesive files inside
sessionui when useful, with no new public component api or presentation framework.

reuse the machine picker, form/editor validation, scoped refresh, exact-reference
dispatch, common create/shell completion, summary/detail facts, and `tea.Exec`.
replace the grouped table and space-picker page outright; delete their exclusive
branches, `collectionItem`/heading keys, heading-aware viewport, obsolete hints,
and superseded assertions. retain substantive lifetime/freshness/uncertainty tests.
one browser, one keymap: no old-mode toggle, alias for old `t`, compatibility path,
compact alternative renderer, or pr 4 scaffolding. unrelated cleanup is excluded.

## 7. acceptance and delivery

| criterion | proof |
| --- | --- |
| a1: local arrows/focus/empty lists/modal keys and displayed action target agree; no arrow attaches | focused model interaction cases; verify resulting views and captured exact requests |
| a2: filters, rename/process change, regrouping, removal, frozen ordering, arrivals and unavailable peers preserve the rules above | small deterministic observation/transition cases; pending target/draft never rebound |
| a3: 80×24 ordinary layout fits the stated working set; long labels, overflow, empty/offline states, forms and resize notice remain usable | row/column bounds and meaningful controls asserted; manually inspect existing sessionui fixture renders; no new fixture/golden framework |
| a4: create → select, shell-here → exact fullscreen attach → detach on new shell; ordinary enter → detach preserves current context and first subsequent navigation key; source survives; lost reply never repeats creation | extend `TestShellDesktopRealTTYCreateAttachDetachAndLostReply` with both attachment paths through the real production boundary on linux and darwin |
| a5: old paths/keys gone; help and browser agree; unchanged contracts remain green | diff review and `./scripts/test verify` |

adversarial review at contract, owner red/green, and composed-diff stages; fix
contradictions at their owner. existing simulated-gateway pty tests are local
regressions, not a4 evidence. no new phone or provider-live matrix for this desktop
change. tmux/integration/live requires explicit current-turn approval and exact
test-owned sessions on isolated sockets; missing boundaries are `NOT_RUN`, never
pass. evidence is content-free. unresolved defects go in `docs/issues/`.

accepted costs: chrome/status disappear while attached; navigation requires
detach; no past-space/source-return convenience; temporary unsorted agent rows
while focused; narrow labels truncate; smaller browser windows require resizing;
terminal-here changes key. no endpoint or deployment capability changes. publish
only through the existing release process; prior evidence does not prove this pr.
