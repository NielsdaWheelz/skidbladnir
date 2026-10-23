# inventory snapshot is a one-field wrapper

problem: `InventorySnapshot(val inventory: SessionsResponse)` (`Machines.kt:25`)
only wraps the response, so every reader spells `.snapshot.inventory` or
`lastSnapshot()?.inventory`.

impact: one needless hop in about a dozen readers; no behaviour.

evidence: readers in `Machines.kt`, `Groups.kt`, `ForgeSheet.kt`,
`ProductModel.kt`, `SkidbladnirController.kt` and `WorkingDirectoryEntry.kt`.

resolved when: `InventoryState` holds `SessionsResponse` directly, every reader
drops the hop, and the build passes.
