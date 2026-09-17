# shared path grammar owned by picker state

problem: `WorkingDirectoryPath` lives in `WorkingDirectoryPicker.kt`, but session
admission uses its grammar for inventory and created-session responses. its
shared byte bound and the related `HomeDirectory` contract live in
`ProductModel.kt`.

resolved when: move the unchanged declaration beside those shared path
contracts. no new file, API, visibility change or parser duplication is needed.
verify exact declaration equivalence and a temporary compiled decoder-to-picker
journey: admitted inventory cwd, rejected unsafe cwd and exact-path selection
preserving unrelated forge fields. this is not phone acceptance.

tradeoff: the common model stays larger; shared contracts remain together and
the picker owns only its state and transitions. line count alone does not
justify fragmenting the remaining shared session/directory admission.
