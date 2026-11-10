# separate attention, organization and execution effects

status: product proposal from the [workspace review](workspace-coherence.md).

problem: the current needs-input queue places ready results before actions. a
blocking question may deserve attention before another completed turn. future
decluttering could also conflate acknowledgement, hiding and stopping work.

impact: attention order may not match intervention value, and overly broad lifecycle
verbs can make a clean sidebar unexpectedly change running work.

evidence: [needs-input contract](../architecture.md#views-and-the-needs-input-queue),
`internal/fleetview/fleetview.go`, phone `ProductModel.kt`, `Groups.kt`,
`NotificationOwner.kt`, and the hub's `internal/notifier` / `internal/attention`.
current ready is an attributable hub verdict, not proof a task succeeded; requests,
notices and ready already have distinct typed causes. existing queue semantics are
intentional, so changing priority is a product decision rather than a bug fix.

## recommended direction

show distinct sections such as "needs a decision" and "ready to inspect", with
blocking intervention first. keep each section's stable order and preserve the
selected identity during updates; status may change without moving the control
being targeted. an explicit next-needs-me action can advance the owner without
forcing the whole collection to rearrange.

show the smallest actionable fact, its machine and evidence freshness. terminal
observation cannot provide semantic task understanding. unknown, stale and menu
states must not become guessed completion. a visit is not an answer to a question.

if shelving, pinning or snoozing is wanted, define it as retained user intent with
one owner. decide whether it is device-local or fleet-shared, what resurfaces it,
and how urgent requests remain discoverable. it must not clear a provider question,
stop a terminal, delete a checkout or rewrite observed status. keep the existing hub
attention authority; do not introduce a second acknowledgement store in each client.

## evidence to avoid copying blindly

t3 offers pin/snooze/settle and configurable automatic settlement. pinning does not
prevent settlement. settlement can dismiss unanswered async questions, close idle
terminals and run configured cleanup; one word therefore spans several effects
([released sidebar guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/thread-sidebar.md)).
bb's manager updates depend on thread nesting, according to its guide; visual
organization should not silently define supervision or communication relationships
([orchestration guide](https://getbb.app/guides/orchestrate-coding-agents)).

cost: a few explicit dispositions and some human curation instead of one magical
cleanup action. reject blanket auto-archive, compulsory activity ordering and any
use of silence as success. [search](fleet-search.md) supplies discovery;
[managed work](managed-collaboration.md) would own actual task relationships.

resolved when: the owner selects priorities and any retained disposition semantics;
decision/ready/unknown/stale states, changing foregrounds, hub outages and updates
under a selected row preserve truthful membership and target identity. hiding or
acknowledging work must have only its documented effects. qualify device attention
and notifications at their actual boundary, not from static rendering alone.
