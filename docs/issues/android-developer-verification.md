# Android developer verification is undecided

Problem: Google is rolling out developer verification for apps installed on
certified Android devices, sideloaded ones included. Skíðblaðnir's APK is signed
by a key no verified developer account has registered.

Impact: where verification is enforced, a certified phone may refuse to install
or update the APK, which is skid's only distribution ([phone](../phone.md)).

Evidence: an open question in the publication campaign since 2026-10-25,
deferred by the owner on 2026-10-26. The current signer is pinned in
`android/app-signing-cert.sha256`; its key was created on 2026-10-26.

Resolved when: the owner registers the signing key with a verified developer
account, or decides not to and [phone](../phone.md) says what that means for
installing.
