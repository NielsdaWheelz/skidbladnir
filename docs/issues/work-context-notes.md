# retain human intent without inferring it from terminal output

status: exploratory product proposal from the
[workspace review](workspace-coherence.md).

problem: terminal inventory preserves process/session facts but does not tell the
returning owner what they intended to do next. agent silence or an idle screen
cannot reconstruct that intention reliably.

impact: returning after an interruption or a day away requires re-reading terminals
and remembering the next decision. this is different from missing provider history.

evidence: current groups are tmux labels and views are derived from observed
inventory (`internal/group`, phone `Groups.kt`); no persistent work-context note
owner exists in the reviewed product. proposed [launch presets](project-launch-presets.md)
may supply a suitable identity, but are themselves unapproved.

## candidate direction

allow a short owner-authored "working on / next decision" note on a persistent
work context. keep it visibly human intent, separate from terminal status,
observed readiness, provider execution and agent-reported results. editing it
does not start or steer an agent.

choose its identity and storage owner before implementation. a group that disappears
from inventory is not a durable anchor. reuse a selected project/work identity
rather than adding a competing task database; if none is warranted, defer the note.
decide device-local versus shared storage, rename/deletion behavior, conflict handling
and explicit completion/removal. retain only the text the user chose to save.

## trade-offs

manual authorship costs a few seconds and can become stale; it gives dependable
ownership of meaning. show that it is a note, not fresh machine evidence. reject
automatic transcript summaries, inferred task completion, an autonomous memory
pipeline and another project-management application for this feature.

the studio-workbench analogy is useful: leave a small intelligible note beside
the work so resumption does not require reconstructing the whole session. this is
an experiment worth trying after [retrieval](fleet-search.md), not a prerequisite
for ordinary terminals or [managed collaboration](managed-collaboration.md).

resolved when: the owner chooses a bounded note scope and one persistence owner,
then next-day resumption, edits, missing/offline sessions and renamed/deleted work
contexts preserve the note's identity and human-authored meaning; or the idea is
retired because existing external notes already satisfy the need.
