# spaces, shells, and client composition

2026-09-15: user-approved direction. deliver three separate, independently
shippable prs. pr 1's [implementation spec](spaces.md) is implemented in source,
including the approved restoration and post-create selection decisions.
[the roadmap](roadmap.md#spaces--source-implemented-runtime-acceptance-open) records
verification and open runtime acceptance. this document owns the three-pr
boundary; prs 2 and 3 have no implementation or scaffolding.

## goal and approach

make related agent work, ordinary shells, and inspection tools easy to reach.
herdr's example puts claude and a shell in adjacent panes of one tab. borrow
that useful proximity while preserving skid's existing execution owners.

tmux owns terminals and process lifetime. each gateway owns its local operations.
clients compose the fleet and its presentation. space membership organizes
sessions; it does not own them. git owns checkouts and worktrees.

## the three prs

| pr | goal and scope | acceptance and accepted cost |
| --- | --- | --- |
| 1. spaces | optional per-session work label, inventory exposure, assignment and clearing, grouped views in cli/tui/phone. see [spaces.md](spaces.md). | sessions remain addressable and usable through regrouping. labels last only for the tmux session lifetime; manual filing; no saved empty spaces. |
| 2. new shell here | create an independent ordinary tmux session on an exact target's host, using its validated cwd sampled at launch. expose through the existing clients. shell creation works without space membership. | creating/selecting/closing the shell preserves the agent session. separate cleanup; no cloned agent environment, automatic commands, or mandatory companion relationship. specify the shell launch kind and host shell policy before implementation. |
| 3. client navigation/composition | quick agent/shell switching and preserved return context. desktop can compose separate attachments in an existing terminal split; phone shows one readable terminal at a time. | inspecting the shell and returning preserves the intended targets and context. desktop layout belongs to its client; phone sacrifices simultaneous viewing. implement only missing navigation behavior; no new split renderer is required. |

each pr completes its feature end to end, including docs and appropriate checks.
do not add scaffolding for later prs. pr 1 owns space filtering and collection
return continuity; pr 3 owns additional navigation between agent/shell terminals.

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
[roadmap.md](roadmap.md). later prs require their own incorporation, without
relabeling historical evidence as new proof.
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
