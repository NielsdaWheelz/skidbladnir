# picker model spells nullables as sum types

problem: the directory picker model reinvents nullable and boolean values:
`ParentDirectory {Absent, Available}` and `DirectoryOmissions {None, Present}`
(`WorkingDirectoryEntry.kt:33-43`) exist only to map the wire's
`parentDirectory?` and `omitted`; `WorkingDirectoryPickerScreen.kt` matches on
both.

impact: a wire-to-domain mapping that a nullable and a boolean would not need; no
behaviour.

resolved when: the listing decodes directly with a nullable parent and an
`omitted` flag, the picker screen and model change together, and the build passes.
