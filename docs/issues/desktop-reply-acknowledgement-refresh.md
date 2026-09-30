# desktop reply acknowledgement leaves the displayed snapshot stale

problem: a successful reply-view acknowledgement writes the local unread store
but discards its returned snapshot. returning to the browser does not update the
displayed unread state or request a local-store read.

impact: an acknowledged `new reply` marker can remain visible until a later
inventory/result operation reloads the store. local feedback depends on unrelated
network polling. this does not explain persistence after ordinary terminal entry,
which deliberately performs no acknowledgement.

evidence: `internal/sessionui/unread.go:145` discards the snapshot returned by
`Acknowledge`; `internal/sessionui/session.go:378` handles `outputPresentedMsg`
without updating `unreadSnapshot`. inventory handling reloads it through `Sync`
at line 199. the normal timer is five seconds, plus request completion; this is
source reasoning, not measured runtime latency.

resolved when: successful local acknowledgement updates the browser's local
projection through its existing model owner, independently of inventory. verify
that captured ids disappear on return, concurrent later ids remain visible, and
storage failures retain honest markers/errors. use a temporary focused check
without introducing a new state framework or polling loop.

runtime reproduction remains `NOT_RUN`; no code change is authorized here.

2026-09-29 product direction: the user wants the human reply viewer removed in
favor of notification clearing on terminal entry and renewed work. resolve this
viewer-specific defect by removing that path with the new lifecycle, rather than
polishing a retired interaction. immediate local display reconciliation remains
a requirement for the replacement; see the [notification plan](../reply-notifications.md).
