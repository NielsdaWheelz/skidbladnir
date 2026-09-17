# single-use fleet transaction callbacks

problem: `replacePreferencesWithVerifiedRollback` is an internal free function
with one caller, `MachineStore.commitFleet`. its target-verification and key
quarantine callbacks always invoke the same store-owned operations.

resolved when: make the transaction a private `MachineStore` method taking the
target and expected credentials, with direct readback and key-quarantine calls.
keep the method separate: its early failures must still reach `commitFleet`'s
recovery checks. preserve sealing, commit acknowledgement, target comparison,
credential readback, rollback verification and key deletion before quarantine,
including the deletion-failure `finally` path. no format or crypto change.

numeric preference-copy/write branches have no admitted producer: installation
requires an empty map; reconnection requires readable string/string-set fields;
this store is the sole preference-file owner. remove `Int`/`Long`/`Float` cases.
retain boolean handling for the quarantine marker, string-set copies and
unsupported-value rejection.

verify complete caller/admission and source equivalence, plus focused temporary
actual-preferences/readback integration where executable. distinguish those
results from unexecuted disk-failure and Android keystore behavior; do not build
a crypto framework or add production seams for this cleanup.
