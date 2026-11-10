# measure interaction latency and remove unnecessary dependencies

status: performance investigation proposal from the
[workspace review](workspace-coherence.md); no new latency measurements exist.

problem: the reviews praise t3's responsiveness, but static comparison cannot
identify skid's actual latency budget. a slow visible journey can arise from work
waiting on unnecessary metadata, repeated connection setup, observation cost,
layout or provider output. replacing the entire transport before measuring would
confuse those owners.

impact: perceived sluggishness and standing resource cost can distract from work;
an indiscriminate optimization may weaken target or delivery guarantees instead.

## known source facts and existing issues

- the desktop dashboard already reads/publishes each peer independently in
  `internal/sessionui/inventory.go`; one unavailable host is not a mandatory
  fleet-wide rendering barrier. cli complete-list collection is a different
  operation (`internal/fleetclient/client.go`).
- notification entry alone waits for rich inventory before attaching; ordinary
  card entry does not. repair the
  [identity/metadata boundary](phone-entry-metadata-latency.md), not a fake
  placeholder session or a second cache claiming current controls.
- hub observation inventories peers every five seconds; clients and recovery add
  work. batch/settled-detection improvements already exist. the
  [energy issue](tmux-exec-overhead.md) retains observed improvement and unresolved
  controlled measurement. do not cite its old runaway samples as current behavior.
- [keyboard resizing](terminal-resize-churn.md) can cause repeated shared-window
  repaint. it is an input-geometry publication issue, not proof of slow networking.
- the go fleet client disables keep-alives; android disables automatic connection
  retry. any transport tuning must preserve the explicit no-replay contract and
  distinguish observations from effects. the review measured no benefit from
  changing either setting (`internal/fleetclient/config.go`, `GatewayClient.kt`).

## useful t3 mechanisms

compact navigation state, bounded thread windows, incremental reconnect, output
subscriptions only when needed, and active-candidate recovery queries reduce work
before animation is considered. its performance documentation distinguishes encoded
payload limits from actual memory/network measurement
([performance contracts](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/docs/internals/performance-regressions.md)).
route prechecks share a 500 ms head start so a failed lan route need not consume a
full serial timeout before another route is tried
([connection driver](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/packages/client-runtime/src/connection/driver.ts#L48)).
these are source mechanisms, not measured comparative speed; skid does not need
t3's history paging or route scheduler without the corresponding requirement.

## proposed investigation and trade-off

measure time to first useful session list, card/notification entry, reconnect and
input response separately, with an offline peer and ordinary session count. divide
client rendering, required target admission, transport and host work; keep content
out of timing evidence. choose concrete budgets from that baseline and user need.

remove a measured unnecessary dependency at its owner before adding caches, push
streams, persistent observation connections or another poller. keep loading
geometry/focus stable and preserve last useful stale data without presenting it as
fresh. do not optimistically report successful writes.

cost: bounded measurement before visible optimization. alternative machinery may
be justified later, but creates freshness, cancellation and recovery obligations.
this issue preserves the ownership cutover's measurement-first approach; its
remaining native proof belongs to [terminal-service qualification](terminal-service-qualification.md).

resolved when: selected journeys have measured budgets and their observed bottleneck
is repaired or explicitly accepted; slow/offline-host cases retain exact targets,
no input replay and truthful freshness. device comfort and gateway overhead require
their real boundaries; neither is proved by source inspection or payload sizes.
