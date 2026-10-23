# inventory snapshot is a one-field wrapper

problem: `Machines.kt` `InventorySnapshot(val inventory: SessionsResponse)` lost
its receive timestamp in the android-core cleanup and now only wraps the
response. every reader spells `.snapshot.inventory` or
`lastSnapshot()?.inventory`.

impact: one needless hop in about a dozen readers; no behaviour.

evidence: at `defbf3f` the readers are in `Machines.kt`, `SkidbladnirController.kt`,
`ProductModel.kt`, `Groups.kt`, `ForgeSheet.kt` and `WorkingDirectoryEntry.kt`.

resolved when: `InventoryState` holds `SessionsResponse` directly and those
readers drop the hop, in one change with their owner; the build passes.
