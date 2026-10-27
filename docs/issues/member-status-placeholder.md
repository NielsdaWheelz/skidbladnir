# A member's status states what it does not know

Problem: on a member computer, `skid status` lists every other computer as
`pending_offline` with no version or origin. Only the hub holds reports, so the
member fills the row with a placeholder (`internal/fleet/service.go`, `status`)
that reads as a fact: those computers may be online and applied.

Impact: the same command answers differently on the hub and on a member, and the
member's answer is wrong in the common case. A stranger following a quickstart on
a member computer sees a broken fleet.

Evidence: 2026-10-26, v0.23.0. On the macbook (a member), `skid status` showed the
hub and arch as `pending_offline` without versions or origins; on the hub at the
same moment, all three computers were v0.23.0, `applied`, with origins. The
behavior is documented in [architecture](../architecture.md) (`skid status` row)
and in `skid --help`.

Resolved when: `skid status` on a member shows the hub's view, as a member's apply
and network report already forward to the hub, or marks rows it cannot know as
unknown rather than `pending_offline`; the architecture and help text say which.
