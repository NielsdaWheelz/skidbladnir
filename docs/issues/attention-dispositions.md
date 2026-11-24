# Separate attention, organization and execution effects

Product proposal from the [workspace review](workspace-coherence.md).

Needs input is a flat list ordered by sign: request, error, ready, interrupted.
The owner chose this order. Sections, shelving, pinning and snoozing remain
open decisions. The list mixes decisions and results, and broader lifecycle
verbs could make organizing the list change running work.

Ready is an attributable hub verdict. Requests, notices and ready each have a
typed cause. The sources are `internal/fleetview/fleetview.go`, phone
`ProductModel.kt`, `Groups.kt` and `NotificationOwner.kt`, and the hub's
`internal/notifier` and `internal/attention`. See the
[needs-input contract](../architecture.md#views-and-the-needs-input-queue).

## Proposed direction

Sections such as "needs a decision" and "ready to inspect" remain open. The
sign order already puts blocking intervention first. Changes to a section
would need to preserve its stable order and the selected identity. An explicit
next-needs-me action can advance the owner without rearranging the collection.

Show the smallest actionable fact, its machine and evidence freshness. Terminal
observation cannot provide semantic task understanding. Unknown, stale and menu
states must not become guessed completion. A visit leaves the question unanswered.

If shelving, pinning or snoozing is wanted, define it as retained user intent with
one owner. Decide whether it is device-local or fleet-shared, what resurfaces it,
and how urgent requests remain discoverable. It must not clear a provider question,
stop a terminal, delete a checkout or rewrite observed status. Keep
acknowledgements at the hub.

## Evidence from other clients

t3 offers pin/snooze/settle and configurable automatic settlement. Pinned rows can
still settle. Settlement can dismiss unanswered async questions, close idle
terminals and run configured cleanup. One word therefore spans several effects
([released sidebar guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/thread-sidebar.md)).
bb's manager updates depend on thread nesting, according to its guide. Visual
organization should not silently define supervision or communication relationships
([orchestration guide](https://getbb.app/guides/orchestrate-coding-agents)).

This approach costs explicit dispositions and some human curation. Reject
blanket auto-archive, compulsory activity ordering and using silence as
success. See [search](fleet-search.md) for discovery and
[managed work](managed-collaboration.md) for task relationships.

Resolve when the owner rules on sections and retained dispositions, and
changes in status, foreground and hub availability keep membership truthful
and preserve target identity. Hiding and acknowledging work must have only
their documented effects. Qualify those effects through the devices and hub.
