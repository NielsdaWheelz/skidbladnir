# shared path grammar owned by entry state

problem: `WorkingDirectoryPath` is declared in `WorkingDirectoryEntry.kt`, beside
forge-entry helpers (`classifyWorkingDirectory`, `DirectorySearchState`,
`workingDirectoryChoices`). session admission in `ProductModel.kt` uses its
grammar for terminal-context, directory-search and inventory responses. its
shared byte bound `MAXIMUM_WORKING_DIRECTORY_BYTES` and the related
`HomeDirectory` contract live in `ProductModel.kt`.

resolved when: move the unchanged declaration beside those shared path
contracts. no new file, API, visibility change or parser duplication is needed.
verify exact declaration equivalence and a temporary compiled decoder-to-picker
journey: admitted inventory cwd, rejected unsafe cwd and exact-path selection
preserving unrelated forge fields. this is not phone acceptance.

tradeoff: the common model stays larger; shared contracts remain together and
entry state owns only its classification and choices. line count alone does not
justify fragmenting the remaining shared session/directory admission.
