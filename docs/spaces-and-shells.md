# spaces, shells, and client composition

2026-09-15: user-approved direction. pr 1's [implementation spec](spaces.md) is
implemented in source,
including the approved restoration and post-create selection decisions.
[the roadmap](roadmap.md#spaces--source-implemented-runtime-acceptance-open) records
verification and open runtime acceptance. this document owns the delivery
boundary; pr 2 is [implemented in source](shells.md), with separately attributed
runtime evidence and remaining gaps in the roadmap.

2026-09-17: approved navigation direction, split into two further prs: pr 3
implements the [organized desktop browser](desktop-browser.md) with existing fullscreen attachment;
pr 4 investigates an embedded terminal and has no implementation or scaffolding.
pr 3 is independently useful and does not depend on pr 4 succeeding.

## goal and approach

make related agent work, ordinary shells, and inspection tools easy to reach.
herdr's example puts claude and a shell in adjacent panes of one tab. borrow
that useful proximity while preserving skid's existing execution owners.

tmux owns terminals and process lifetime. each gateway owns its local operations.
clients compose the fleet and its presentation. space membership organizes
sessions; it does not own them. git owns checkouts and worktrees.

## delivery split

| pr | goal and scope | acceptance and accepted cost |
| --- | --- | --- |
| 1. spaces | optional per-session work label, inventory exposure, assignment and clearing, grouped views in cli/tui/phone. see [spaces.md](spaces.md). | sessions remain addressable and usable through regrouping. labels last only for the tmux session lifetime; manual filing; no saved empty spaces. |
| 2. new shell here | standalone terminal choice; source-session create/attach. see [shells.md](shells.md). works with zero agent profiles or no space. | independent lifetime, generated shortcut name, sampled location, manual uncertainty recovery; browser-based return. |
| 3. organized desktop browser | spaces, global agents, session tabs and main browser content; immediate local selection; existing fullscreen attachment. [desktop-browser.md](desktop-browser.md) owns implementation and acceptance. | one current browser state survives detach; no per-space memory or special source return. navigation and fleet status are hidden while attached. |
| 4. embedded-terminal experiment | evaluate one borrowed terminal component inside the main area, using the existing authenticated attachment. persistent navigation during attachment is the proposed outcome. | establish terminal compatibility before committing to production integration. a negative feasibility result is valid; it does not block pr 3 or justify a homegrown emulator. |

each shipping pr completes its feature end to end, including docs and appropriate checks.
do not add scaffolding for later prs. pr 1 owns space filtering and collection
return continuity; pr 3 owns the new desktop navigation. phone navigation beyond
prs 1 and 2 remains separate; neither of the new desktop prs requires phone ui changes.

## desktop navigation boundary

[the pr 3 spec](desktop-browser.md) owns behavior, state, keys, geometry, file
ownership, hard cut and acceptance. latest decisions replace the earlier proposed
per-space memory and source-return exception: selection updates locally; detach
resumes the same browser model; a new shell stays selected. no history is added.
pr 3 introduces no host, phone, wire, terminal-transport or dependency change.

pr 4 starts by checking the actual terminal boundary: bounded rendering, cursor,
terminal replies, key modes, paste, unicode, resize, tmux copy mode, disconnect,
and ordinary agent/shell use. production embedding needs a reviewed contract for
input ownership, geometry, and attachment lifetime, with measured acceptance.
library availability is not compatibility evidence. keep the experiment separate
from pr 3; no permanent parallel renderer is implied by this delivery split.

## architectural trap

current discovery follows the active pane in a session's current window. selecting
a shell inside that same tmux session changes the observed agent and makes the
old agent target stale. see [agent-control.md](agent-control.md#identity-state-and-dispatch),
[manager.go](../internal/sessions/manager.go), and
[control.go](../internal/sessions/control.go).

two independent sessions can still be displayed side by side through two
`skid enter` attachments. expose multiple individually addressable panes inside
one tmux session only through a separately specified pane-level discovery change.
visual spaces are not tmux session groups; do not restore retired shadow grouping.

## boundaries

no new terminal supervisor, database, coordinator, ownership graph, worktree
manager, unread system, inferred task state, or automatic cleanup. names and cwd
are not control identities. detach, provider halt, and session closure retain
their distinct contracts. stale or uncertain writes are not silently retargeted
or replayed.

## handoff and document authority

pr 1 follows its closed contracts and observed owner red proofs. these documents
record the user's accepted scope changes. pr 1's affected scope and acceptance
are incorporated into [architecture.md](architecture.md), with delivery in
[roadmap.md](roadmap.md). pr 2's [shells contract](shells.md) is now incorporated
there with source implementation and separately attributed acceptance. the
2026-09-17 desktop spec is incorporated as the accepted pr 3 target; it supersedes
only the named desktop presentation/selection rules and retains the prohibition
on per-space history. direct attachment remains unchanged. pr 4 is a
feasibility investigation, not an accepted production terminal contract. never
relabel historical evidence as new proof.
broader capability changes require a new explicit scope decision.

read [../AGENTS.md](../AGENTS.md), [rules/index.md](rules/index.md),
[agent-control.md](agent-control.md), and
[agent-control-ux.md](agent-control-ux.md) before implementation. the accepted
agent-control deltas extend v0; the old activity-only language is not the whole
current contract. use [design-language.md](design-language.md) for presentation.

retain the [testing rules](rules/testing.md) and architecture's bounded red/green
proof shape. builders observe their acceptance test fail before implementing;
verifiers do not write tests or production code. inspect test composition before
running it: tmux and integration/live gates require explicit user approval in
the current turn; platform/adb requires separate current-turn approval. operate
only on exact test-owned sessions on isolated sockets. keep evidence content-free.
unavailable live boundaries are `NOT_RUN`, never passed.

each pr follows the existing coordinated host/cli/android release contract;
optional fields do not imply compatibility with strict old decoders. hard-cut
superseded paths with no compatibility readers, negotiation, alternative routes,
or later-pr scaffolding. pr 1 uses one new session metadata option, one exact
membership route, and the existing runtime/navigation owners. its explicit costs
and implementation file map live in [spaces.md](spaces.md). evidence belongs in
the roadmap; historical runtime results do not establish later changes.

background references, not additional scope: [herdr concepts](https://herdr.dev/docs/concepts/),
[wezterm's label-based workspaces](https://github.com/wezterm/wezterm/discussions/2243),
[conductor's shared versus independent work](https://www.conductor.build/docs/concepts/parallel-agents).
