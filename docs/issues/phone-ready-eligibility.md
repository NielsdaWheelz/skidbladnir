# phone ready eligibility

problem: the host's `status.queue == "ready"` already means a local agent's
terminal sample that is idle with no request, menu or notice. the phone still re-derives
part of it: `NotificationView.presentsReady` (`NotificationOwner.kt`) checks
`terminalStatus` source terminal, activity idle and interaction none itself, and
`sessionStatusContent` ANDs that with the host's queue.

impact: two owners for one predicate. they agree today only because the host's
rule is stricter (it also requires no notice); a host change to eligibility
needs an apk.

evidence: `NotificationOwner.kt` `presentsReady`; `ProductModel.kt`
`sessionStatusContent` (`status.queue == SessionQueueCategory.Ready &&
notification.ready`); `internal/api/status.go` `PresentStatus` (the idle
row alone sets `QueueReady`).

resolved when: `presentsReady` keeps only the hub facts (ready generation,
foreground lifetime, no connection) and the phone reads eligibility from
`status.queue` alone.
