# agent exit has no attention notification

problem: a recognized agent returning to its shell clears its foreground
attention identity. an exit without a still-observable provider error or
interruption notice can therefore leave no row in needs input.

impact: the attention view cannot promise to report every unexpected stop.
process disappearance alone does not distinguish crash, intentional quit,
normal exit or replacement.

evidence: [attention identity rules](../reply-notifications.md#identity-and-lifetime),
`internal/fleetclient/notifications.go` foreground reconciliation and android's
matching notification owner. [session views](../session-views.md) deliberately
retains that contract. this is a source-derived limitation, not a reproduced
crash or an accepted lifecycle feature.

resolved when: a separate scope decision defines which observed exits require
attention, their identity and acknowledgement, and the implementation is
qualified for intended exit, unexpected exit, replacement, outages and visits
without claiming an unavailable cause. explicit acceptance of the limitation
as permanent may instead close this follow-up. no supervisor, incident ledger
or provider-history inference is authorized by this record.
