# notification baselinePending decides nothing

problem: both clients still persist, write and validate the notification
record's `baselinePending` flag, but no outcome depends on it. the closing
baseline is quiet because Presented and EndVisit advance the record's revision,
which expires every predecessor armed before them; arming, clearing and ready
samples (and positive exit) then clear the flag. the record invariant
`pending ⇒ !baselinePending` holds by construction, so the desktop `Ready`
conjunct `!BaselinePending` is implied. the field stays only because
[terminal observation §6](../terminal-observation.md#6-content-attention-and-filtering)
freezes the store schema: no notification-store migration.

impact: dead persisted state that reads as a gate. the clients already write it
differently without consequence: desktop Presented resets it to false, android's
preserves it. a future change could start relying on a flag whose value no
behavior has kept meaningful.

evidence: `internal/fleetclient/notifications.go` (observe's baseline case
changes only the flag relative to its clearing/ready cases; `consume` advances
the revision; `Ready`; `validNotificationSnapshot`) and android
`NotificationStore.kt` (`observe`, `presentsReady`, `consume`,
`NotificationRecord.init`) at `d78cbda`, unchanged through `6859010`.

resolved when: a coordinated store schema change removes the field from both
clients' records, strict decoders and
[the attention schema](../reply-notifications.md#owner-schemas-and-interfaces),
or a specified behavior reads it; then remove this record.
