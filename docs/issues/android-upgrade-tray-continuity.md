# android apk replacement clears existing tray notices

problem: the accepted cutover promised posted-notice continuity as well as durable
acknowledgement continuity. an actual android package-replacement probe cleared
the posted tray notice before the temporary converter launched.

impact: same-package/same-signer upgrade can preserve acknowledgement records and
counters exactly, but cannot promise the existing os tray item survives. replaying
an old notice would risk duplicate alerts and alter admission semantics.

evidence, 2026-10-20: private api35 emulator, production-signer seed/converter/final
upgrade. one native notice before `adb install -r`, zero immediately afterward,
before converter launch. full converted schema4 records/counters equal their
source; repeated conversion and final upgrade preserve newer memory. the actual
production api36 phone later reproduced tray clearing during the authorized
converter/final update; all11 durable records and counters survived exactly.

resolution: explicit owner acceptance of os tray clearing while retaining exact
durable memory and no replay, or defer phone activation until the owner handles
the existing tray item. the owner accepted os tray clearing on2026-10-20; actual-phone conversion and durable-reread equality passed. fresh installed
notification qualification remains in the separate optional-push issue. this
accepted os behavior requires no product workaround.
