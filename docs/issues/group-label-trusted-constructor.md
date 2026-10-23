# the phone re-checks host group labels

problem: `GroupLabel` (`Groups.kt:10-40`) mirrors the host's label grammar with
platform icu nfc and has only `parse`/`fromDraft`. the phone validates drafts with
it, and `GroupLabelSerializer` (`Sessions.kt:139-142`) throws on every host label
`parse` rejects.

impact: two owners of one invariant. a label the host accepts and the phone's
grammar rejects (a unicode-table difference) makes that machine's whole inventory a
lost response, so its cards go stale.

decision: on 2026-10-22 the owner chose to drop the phone's icu mirror, so the
host's 422 `GroupInvalid` is the only check, giving up the phone's instant inline
validity.

resolved when: `GroupLabel` is plain host text decoded without validation, the
phone submits the typed draft and shows the host's 422, `GroupLabelSerializer` and
the icu normalizer are gone, and the architecture names the host as the only label
owner.
