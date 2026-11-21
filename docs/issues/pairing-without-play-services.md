# A phone without Google Play services cannot pair

Problem: the app pairs only by scanning a `skid connect` QR code with Google's
code scanner, which Google Play services provides. It has no way to paste the
invitation line that `skid connect` also prints, and no link handler for
`skid://connect/`. A phone without Play services cannot join a fleet, and neither
can an emulator image without them. An emulator with a Google Play image might
scan a code shown in its camera's virtual scene; that is untried.

Impact: an Android phone without Play services cannot use skid at all, although
only pairing and optional push need them. A phone walk, a demo recording or an
acceptance check that needs a paired app runs on the owner's phone, because an
emulator a run starts has never reached a paired state
([unwalked install](fresh-install-walk.md)).

Evidence: `FleetScanner.kt` calls `GmsBarcodeScanning`;
`android/app/build.gradle.kts` depends on `play-services-code-scanner`;
`AndroidManifest.xml` declares no `skid://` intent filter. [Phone](../phone.md#pair)
says "there is no way to paste an invitation", and the README lists Google Play
services as a requirement. The publication campaign left this open before its
stranger walk, which was then dropped (`git show b0dd12a:docs/publication.md`).

Deferred decision: a paste or link fallback is a new capability, so
[architecture §1](../architecture.md#1-scope) needs an explicit scope and
acceptance change, which is the owner's to make.

Resolved when: the owner scopes a fallback and it ships with the architecture,
[phone](../phone.md) and README updated, or keeps the scanner as the only path,
which the docs already state, and deletes this file.
