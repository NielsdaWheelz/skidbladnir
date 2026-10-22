# the phone re-checks host group labels

problem: `Groups.kt` `GroupLabel` mirrors the host's label grammar with platform
icu (unicode-15 nfc, [groups](../groups.md#canonical-label)) and has only
`parse`/`fromDraft`. the phone validates drafts with it, and the session decode
re-validates every host label: `GroupLabelSerializer` in `Sessions.kt` throws
when `parse` rejects one.

impact: two owners of one invariant. a label the host accepts and the phone's
grammar rejects (a unicode-table difference) makes that machine's whole
inventory a lost response, so its cards go stale.

evidence: android-core cleanup, design §2.3 and §11.2. host-rendered-copy kept
the mirror because removing it was then unapproved. on 2026-10-22 the owner
decided to drop the phone's icu mirror so the host's 422 `GroupInvalid` is the
only check. the cut is a follow-up pr after wave 4, and it gives up the phone's
instant inline validity.

resolved when: `GroupLabel` is plain host text decoded without validation, the
phone submits the typed draft and shows the host's 422, `GroupLabelSerializer`
and the icu loop are gone, and groups.md names the host as the only label owner.
