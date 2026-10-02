# searchable group entry implementation

problem: the approved [group entry specification](../group-entry.md) is not
implemented. desktop cycling and the phone menu ignore typed text; desktop
enter from group still creates or saves immediately.

impact: finding an observed label requires scanning or cycling the whole set.
desktop cycling from a partial label can clear the draft to unassigned, and
selection lacks the requested separate create/save focus stop.

evidence: 2026-10-02 source survey of `internal/sessionui/{session,create,metadata,view}.go`
and android `GroupSheet.kt`; both create and edit share the existing input paths.
the accepted spec and implementation plan are documentation only. runtime and
device acceptance are `NOT_RUN`.

resolved when: both clients implement the spec and its bounded acceptance is
recorded for the changed source, with candidate selection dispatching nothing
and the final action dispatching once. retain separate results for client checks,
desktop rendering and the approved phone journey. implementation has not been
requested; tmux/device execution requires explicit current-turn approval.
