# retrieve work without adding navigation clutter

status: improvement proposal from the [workspace review](workspace-coherence.md).

problem: groups and needs-input already unify the fleet, but locating a particular
session still depends on navigating the collection. adding more sidebar hierarchy
would not by itself improve retrieval or preserve a small intentional working set.

impact: many simultaneous sessions become a recognition and navigation burden.
activity-driven rearrangement can destroy the owner's spatial memory.

evidence: `Groups.kt` (`observedGroups`, `dashboardViews`, `dashboardItems`),
`DashboardEntryState.kt`, `internal/sessionui/navigation.go` and `view.go`.
the phone already uses stable item keys and an adaptive lazy grid; it does not need
a new card framework or a duplicate grouping projection.

## recommended direction

filter retained fleet inventory locally by session name, group, machine, observed
profile and working directory. search text selects visible candidates; an action
still captures an exact reference. keep stale results visibly stale and retain
per-host failure information. preserve selection and scroll when refreshing or
returning from a terminal. prefer a single search entry and clear empty result
over another set of intersecting navigation modes.

an optional small pinned working set is a separate retained preference, not an
observed fact. current group labels can disappear when no sessions expose them.
before adding pins, choose device-local versus fleet-shared ownership, empty-group
behavior and rename semantics. reuse [project identity](project-launch-presets.md)
if that feature is selected; do not persist copied inventory to make pins survive.

use the studio-workbench analogy: preserve the items where the owner put them and
make everything else easy to find. provide an explicit next-needs-me action through
[attention](attention-dispositions.md), not compulsory reordering on every update.

## trade-offs and evidence from competitors

explicit curation costs a little work but preserves spatial memory. local search
needs no service, embeddings, transcript index or provider history. searchable
history would be a new capability, not an extension of this filter.

t3 already offers project grouping and synchronized manual order; grouping disables
the working shelf and reordering, while the working shelf overrides manual active
order. the review's complaint concerns interacting organizational grammars, not
simply absent organization. do not copy all modes
([released sidebar guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/thread-sidebar.md)).

resolved when: a bounded search/working-set scope is chosen and the owner can find
work across hosts using the existing inventory, with keyboard/touch access,
stable selected identity and return position, explicit stale/empty/offline states,
and no mutation ever resolved through a display string. persistence for pins is
either owned and demonstrated or explicitly deferred.
