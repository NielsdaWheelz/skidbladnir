# stale redemption replaces the current persistence marker

problem: `acceptFleetScan` publishes `pendingFleetPersistence` from its network
completion before checking the captured generation. an abandoned redemption can
overwrite the marker for a newer credential write.

impact: after background/resume, a valid newer fleet can be compared against the
old credentials and incorrectly require app-data reset (installation) or another
invite (reconnection). no stale write is established; the credential worker does
check generation before persistence.

evidence: `SkidbladnirController.kt` assigns the marker before
`executeCredentialOperation` checks generation. backgrounding does not cancel
redemption futures. start captures the marker before enqueueing its durable read.

reproduction to verify: suspend redemption A, background/resume to a fresh scan,
complete redemption B and begin its credential operation, background, then
complete stale A before resuming. with distinct returned bearer sets, durable B
must be reconciled with B, never A.

resolved when: admit completion on the controller's main-thread generation/state
boundary before publishing its marker or queueing persistence. preserve recovery
for a write already started before backgrounding. demonstrate the race through
actual controller/redemption scheduling and resume classification; distinguish
controlled transport/credential scheduling from real Keystore acceptance.
