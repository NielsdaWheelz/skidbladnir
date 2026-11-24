# Status sign fleet cutover

Hosts send a required `sign` and `workContinues`. The previous APK still
requires `tone` and `queue`. Either APK loses the other release's status
responses. Go clients render status from raw facts, so the desktop, CLI,
Jarvis and Mac notification app read both releases. No state migration is
required.
[Wire contract](../terminal-observation.md#2-capability-and-wire-contract),
[lockstep cutovers](../architecture.md#lockstep-cutovers).

The signed v0.30.0 release carries this cutover. Deployment is pending.
At publication, arch was offline and the enrolled phone was unavailable
through adb, so the existing fleet was kept together.

Have the APK ready before updating hosts. With current-turn owner approval,
publish, pin and verify the release, update every enrolled host, then
install the APK at once. Reopen the desktop browser for its updated binary.
Follow the [release](../architecture.md#release) and
[update](../install.md#update) procedures.

During the mismatch the phone reports that it could not reach the affected
machine over the Tailnet. Retained cards read STALE with actions disabled, and
an app restart leaves the machine UNAVAILABLE with no cards. A create or shell
request that races the update can return an unknown outcome. Hub notifications
keep arriving. Do not tap them during the cutover: tapping marks the episode
handled across devices before the incompatible host read fails.

Resolve when every enrolled host serves the release and the installed APK
shows its cards fresh, with signs. Roll back the fleet and phone together.
Android blocks an in-place downgrade, so the previous APK requires an uninstall
and re-pair. The [owner look](unrun-acceptance.md) remains a separate check.
