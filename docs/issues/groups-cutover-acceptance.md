# groups cutover runtime acceptance

problem: the groups source cutover has no observed android device restore or
interaction result, and no darwin isolated-host result. a linux isolated tmux
journey and client wire fixtures cover different boundaries.

impact: the renamed phone controls, schema-3 task-capsule restoration (including
`needsInputOnly`), the one-time reset of an older capsule, and the coordinated
phone-to-gateway mutation remain unverified in their runtime.
an android build and source fixture cannot establish that behavior. darwin
membership continuity remains `NOT_RUN` separately from linux.

evidence: the go client temporary tls journey covered list filtering,
assignment/clear and old-name rejection. the corrected linux isolated tmux
journey covered a pre-rename `@skid_space_b64` value, assignment, inheritance,
clear and source identity. `scripts/check verify` passed engineering checks.
the android source fixture retained schema-2 literals but could not execute
`Bundle` restoration without a device or android runtime.
the user declined adb/platform execution for this turn; phone acceptance is
`NOT_RUN` by explicit direction.
the configured `macbook` ssh name did not resolve from this linux worktree, so
no darwin tmux command was attempted.

resolved when: on an explicitly approved device, round-trip a schema-3 capsule
and confirm its named selection, needs-input filter and heading anchor; confirm a
schema-2 capsule produced by the pre-rename code resets once to all/all/top with
needs input off; then exercise filtering, editing, creation and detach/back
against an approved isolated host. on darwin, run the same bounded host metadata
journey on an isolated `-L` tmux socket. attribute each result to the exact
source and remove temporary tests afterward.
