# groups contract in terminal continuity integration

problem: the concurrent terminal-continuity worktree changes creation and
client code that this cutover also changes. its draft specification still uses
the former membership field in terminal-create bodies, inherited labels and
client examples.

impact: merging that work unchanged could restore an obsolete wire field or
client name after the groups cutover. the branches have no shared runtime
dependency, but their final contract must agree.

evidence: read-only inspection of that worktree's uncommitted
`docs/terminal-continuity.md` found the former field in both `POST /v1/sessions`
examples and source-terminal inheritance text. this groups branch does not
contain that concurrent work.

resolved when: at integration, update the continuity document and affected
creation/client code to the `group` contract, reject the old field, then rerun
its change-specific checks and `scripts/check verify`. preserve continuity's
other accepted behavior and evidence.
