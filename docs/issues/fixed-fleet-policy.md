# duplicated fixed-fleet policy

problem: invite parsing, stored-fleet parsing, and reconnect comparison repeat
the same arch/devbox/macbook label policy. a policy change requires several
independent edits.

evidence: `FleetInvite.kt` owns `FLEET_LABELS`; `MachineStore.kt` repeats the
labels twice; `FleetPersistence.kt` reconstructs their order to compare machines.
that file already compares reconnect credentials by machine-set equality.

resolved when: share the existing fixed label list and compare admitted fleets
directly. characterize invite admission, persisted admission, and reconnect
identity, preserving uniqueness, canonical ordering, and credential handling.
