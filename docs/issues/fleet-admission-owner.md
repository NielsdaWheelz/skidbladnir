# duplicated fleet admission

problem: `MachineStore.readLocked` and `isExactFleet` duplicate fleet admission.
`uniqueCredentials` constructs filtered subsets, but no caller admits a partial
fleet.

evidence: the stored index permits exactly three handles. the reader quarantines
unless three readable credentials survive with exact fixed labels; write
admission requires those same ordered labels before checking uniqueness.

resolved when: sort readable credentials once and admit them through the existing
`isExactFleet` predicate. retain exact ordered labels plus distinct handles,
origins and bearers; remove `uniqueCredentials`. exact fixed labels already
imply case-insensitive label uniqueness. preserve field decoding, decryption
order, index versus pairing quarantine and whole-fleet rejection. no new
abstraction, storage format or crypto change.

characterize typed admission and invalid installation through actual preferences;
compare the complete read admission and result paths. these checks do not prove
successful encrypted readback or physical keystore behavior.
