# Every release asks the phone to update

Problem: computers and the phone ship from one release, and the user docs keep
them on the same one: [install](../install.md#update) updates each computer,
then the phone; [phone](../phone.md#update) installs the APK of the release the
computers run; [contributing](../../CONTRIBUTING.md) says they should match; the
[security policy](../../SECURITY.md) supports only the latest release. Releases
come often: nine from v0.23.0 (2026-10-26) to v0.27.0 (2026-11-19). Under the
architecture's [lockstep rule](../architecture.md#lockstep-cutovers), consumers
decode tolerantly, so only a lockstep cutover forces the phone to update with the
computers, but nothing tells a reader which releases those are.

Impact: a stranger sideloads a new APK every few days, by hand or through
Obtainium, or lets the phone drift without knowing whether the drift matters.

Evidence: the release tags and their dates (`git tag --sort=creatordate`); the
release notes `scripts/release` writes list merged pull requests and signers,
and say nothing about the phone. The publication campaign left this open before
its close (`git show b0dd12a:docs/publication.md`).

Deferred decision: the owner chooses a policy, for example release notes that
say whether the phone must update, a stated compatibility range, fewer releases
that touch the phone, or accepting the cost.

Resolved when: install, phone and the release notes state the chosen policy, or
the owner accepts the cost and deletes this file.
