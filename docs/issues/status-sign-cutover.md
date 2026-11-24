# Status sign fleet cutover

Hosts send a required `sign` and `workContinues`. The previous APK still
requires `tone` and `queue`. Either APK loses the other release's status
responses. Go clients render status from raw facts, so the desktop, CLI,
Jarvis and Mac notification app read both releases. No state migration is
required.
[Wire contract](../terminal-observation.md#2-capability-and-wire-contract),
[lockstep cutovers](../architecture.md#lockstep-cutovers).

The signed v0.30.0 release is active on macbook, dev-server and the enrolled
S22+. Both gateway updates preserved their tmux server lifetimes. The APK
upgraded in place, and its dashboard showed enabled cards and status signs
from both upgraded hosts. The owner deferred arch for this rollout. The
phone cannot read arch's sessions until arch serves the new status fields.

When arch is available for an approved update, follow the
[update procedure](../install.md#update). Verify its gateway version and a
fresh card on the enrolled phone. Reopen any desktop browser still running
the previous binary.

During the mismatch the phone reports that it could not reach the affected
machine over the Tailnet. Retained cards read STALE with actions disabled, and
an app restart leaves the machine UNAVAILABLE with no cards. A create or shell
request that races the update can return an unknown outcome. Hub notifications
keep arriving. Do not tap them during the cutover: tapping marks the episode
handled across devices before the incompatible host read fails.

Resolve when arch serves the new status fields and the installed APK shows
its cards fresh, with signs. Roll back the fleet and phone together.
Android blocks an in-place downgrade, so the previous APK requires an uninstall
and re-pair. The [owner look](unrun-acceptance.md) remains a separate check.
