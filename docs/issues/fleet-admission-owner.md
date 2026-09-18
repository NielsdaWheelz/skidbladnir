# duplicated fleet admission

problem: `MachineStore.readLocked` and `isExactFleet` duplicate fleet admission.
`uniqueCredentials` constructs filtered subsets, but no caller admits a partial
fleet.

the reader's `quarantined` counter is also redundant. the admitted index has
exactly three entries; each successful read appends once and each failed read
appends nothing. exact fixed labels require three readable credentials, which
already proves that no entry failed. the counter has no diagnostic consumer.

evidence: the stored index permits exactly three handles. the reader quarantines
unless three readable credentials survive with exact fixed labels; write
admission requires those same ordered labels before checking uniqueness.

resolved when: sort readable credentials once and admit them through the existing
`isExactFleet` predicate. retain exact ordered labels plus distinct handles,
origins and bearers; remove `uniqueCredentials`. exact fixed labels already
imply case-insensitive label uniqueness. preserve field decoding, decryption
order, index versus pairing quarantine and whole-fleet rejection. no new
abstraction, storage format or crypto change.

remove that counter and its rejection clause while preserving both catch
comments, iteration order and continued reading after an individual failure.

characterize typed admission and invalid installation through actual preferences;
compare the complete read admission and result paths. these checks do not prove
successful encrypted readback or physical keystore behavior.
