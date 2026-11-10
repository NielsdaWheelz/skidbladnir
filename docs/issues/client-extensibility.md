# choose client and customization scope before adding platforms

status: architectural/product recommendation from the
[workspace review](workspace-coherence.md); no new client or extension runtime is
approved. the current [scope](../architecture.md#1-scope) excludes a chat ui and
provider history integration.

problem: competitors offer graphical threads, mobile apps and moddability. copying
those surfaces together would introduce new ownership, interaction and compatibility
obligations without first establishing which user journey the existing clients fail.

impact: a terminal fleet tool can accidentally become a second conversation platform,
plugin host and desktop application before its everyday interactions are coherent.

evidence: skid's desktop browser is `internal/sessionui`'s bubble tea model; its
other main client is native android with xterm. `internal/fleetclient`,
`internal/fleetview`, `internal/api` and existing phone action components already
own shared contracts. there is no ordinary graphical web client in the reviewed
surface. code ownership is mapped in [codebase map](../codebase-map.md).

## competitor evidence and its limits

bb's server owns threads, settings and history; execution machines and clients
connect to it. independent installations create separate workspaces, and a sleeping
server makes the product unreachable. its core guide says work must start in bb
rather than attach existing terminal sessions. the marketplace's session-adoption
claim is a separate feature, not proof of live attachment
([remote model](https://getbb.app/guides/work-from-anywhere),
[handoff listing](https://getbb.app/marketplace/handoff)).

bb's customization is substantive: replacement sidebar/navigation/composer surfaces,
commands, skills and services. its changelog also records experimental api churn,
removed/restored sidebar support and safe mode. reported reconnect, retry and
database repairs establish vendor-reported changes, not comparative reliability
([changelog](https://getbb.app/changelog)). no bb runtime or source checkout was
qualified during this review.

## recommendation

improve the terminal browser and phone first. use configurable key bindings,
density, explicit launch choices and scripts over the existing cli for bounded
customization. consistent human/agent actions matter more than a plugin marketplace
([action discovery](agent-action-discovery.md)). do not create a plugin runtime,
service registry, extension api or arbitrary ui injection for one user without a
specific unmet requirement.

if graphical desktop review becomes decisive, scope a narrow client against the
existing gateways. a new terminal client and a structured chat/history client are
different commitments. the latter requires provider semantics and persistence,
not just a renderer. neither should quietly replace tmux ownership. the
[managed-collaboration record](managed-collaboration.md) explains why adopting t3's
backend would be a major product/ownership choice rather than a small integration.

share contracts and user-action semantics across go/kotlin/future clients; do not
add a cross-language framework solely to share code. connection lifetime,
cancellation and mutation uncertainty need one owner within each client, not retry
logic scattered through views.

## explicit costs

a third client adds independent keyboard/focus/accessibility, reconnect, packaging
and release work. a plugin api adds compatibility, failure containment and upgrades.
a chat renderer adds provider event/history/approval integration. choosing the
existing clients accepts their surface limitations in exchange for concentrating
effort on a few complete journeys. this is a recommendation, not a permanent ban.

resolved when: the owner identifies an unmet journey, selects the smallest surface
and customization scope that satisfies it, states these costs and qualifies the
result; or explicitly defers the competing surfaces. do not treat feature count,
moddability claims or screenshots as evidence that the current product needs a
platform rewrite.
