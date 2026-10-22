# picker model spells nullables as sum types

problem: the directory picker model reinvents nullable and boolean values:
`ParentDirectory {Absent, Available}` and `DirectoryOmissions {None, Present}`
(`WorkingDirectoryEntry.kt`) exist only so `decodeDirectoryListing` can map the
wire's `parentDirectory?` and `omitted`, and `RetainedDirectoryView {None,
Present}` (`WorkingDirectoryPicker.kt`) is a nullable `DirectoryView`.

impact: a wire-to-domain mapping and a custom `map` that a nullable would not
need; no behaviour.

evidence: android-core cleanup design §11.6; the screen
`WorkingDirectoryPickerScreen.kt` (W4.6, android-ui-structure) matches on all
three, so the model kept them for that screen to compile.

resolved when: `DirectoryListing` decodes directly with a nullable parent and an
`omitted` flag, the retained view is nullable, and the picker screen and model
change together; the picker JVM cases still pass.
