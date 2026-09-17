# single-use stored-machine read wrapper

problem: `StoredMachineRead`, `ReconciledStoredControllerState` and
`beginStoredMachineRead` package one callback's captured locals and a boolean
freshness check. `readingState` is always `Booting`.

evidence: each has one construction/call chain, entirely in controller `start`.
reconciliation itself already belongs to `reconcileStoredMachines` and
`forgeAuthoritySurvives`.

resolved when: capture the same immutable credential/machine snapshots and forge
carry directly in `start`, gate its callback before reconciliation, and delete
the wrappers. preserve repeated-background/start carry, credential equality,
stale completion rejection and exact publication/operation ordering. temporary
controller integration evidence should cover those boundaries without adding
production injection seams.
