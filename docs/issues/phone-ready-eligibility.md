# Phone ready eligibility

The host's `status.sign == "idle"` means a local agent's terminal sample
is idle with no request, menu or notice. The phone repeats part of this
test when it projects the hub's ready.

A host change to eligibility can also require an APK change.

`NotificationOwner.kt` checks source terminal, activity idle and interaction
none in `presentsReady`. `ProductModel.kt` overlays ready when the host
sign is `SessionSign.Idle` and `notification.ready` holds. The host's
`PresentStatus` in `internal/api/status.go` also requires notice none.

Resolve when `presentsReady` keeps only its hub and identity checks (ready
generation, foreground lifetime, no connection) and the phone takes
status eligibility from `status.sign` alone.
