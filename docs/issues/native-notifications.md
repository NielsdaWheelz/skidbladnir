# remaining native notification qualification

problem: core standalone notification delivery is qualified; separate physical
variants and historical attribution gaps remain unpassed.

impact: successful forced-doze and current-device runs do not establish natural
idle/mac-asleep operation, every audible policy or an old energy/preference cause.

evidence: [standalone qualification](../generalization-qualification.md) records
actual conversion, independent mac/phone acknowledgement, both mac terminal
adapters, mac banner/sound, phone native click/output, reclaimed-process forced
deep-idle delivery, same-process optional-push disable/restore and actual hub
round trip with fresh fcm delivery. earlier version-specific evidence remains in
[the original qualification](../native-notifications-qualification.md).

remaining records:

- [mac-asleep/natural idle](native-notifications-doze.md).
- [phone audible sound](native-notifications-phone-sound.md).
- [historical mac energy](native-notifications-mac-energy.md) and
  [historical sdk preference drift](native-notifications-sdk-memory.md).
- [additional installed push variants](android-optional-push-qualification.md)
  and [two physical phones](android-two-phone-push-acceptance.md).

retired ntfy setup/distributor conditions no longer apply. hard cutover also
supersedes the old previous-wire binary-rollback requirement: recovery requires
deliberate matching private state, never an incompatible binary symlink change.
no legacy reader, transport or retired behavioral gate is authorized.

resolve only with evidence at each named boundary or an explicit owner scope
change. preserve historical uncertainty; new successful runs do not explain it.
