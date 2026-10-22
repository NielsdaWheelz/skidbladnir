# picker model spells nullables as sum types

problem: the directory picker model reinvents nullable and boolean values:
`ParentDirectory {Absent, Available}` and `DirectoryOmissions {None, Present}`
(`WorkingDirectoryEntry.kt`) exist only so `decodeDirectoryListing` can map the
wire's `parentDirectory?` and `omitted`. the retained view is already a nullable
`DirectoryView` (android-ui-structure).

impact: a wire-to-domain mapping that a nullable and a boolean would not need; no
behaviour.

evidence: android-core cleanup design §11.6; the screen
`WorkingDirectoryPickerScreen.kt` matches on both.

resolved when: `DirectoryListing` decodes directly with a nullable parent and an
`omitted` flag, and the picker screen and model change together; the picker JVM
cases still pass.
