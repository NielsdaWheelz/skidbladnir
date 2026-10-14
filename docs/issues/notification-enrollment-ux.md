# notification enrollment guidance

problem: mobile setup does not adequately distinguish android permission from
fleet notification configuration, private ntfy setup and receiver enrollment.
the 2026-10-13 [production investigation](native-notifications.md) confirms
permission granted with missing configuration and no registered phone receiver.

impact: the owner can reasonably expect notifications after granting permission
while the production transport remains unconfigured. a wrong first ntfy server
also creates a retry trap; that condition was not observed on this phone.

evidence at source `8551cd3` (relevant code unchanged from v0.14.0):

- `MainActivity.kt:119-151` gives a reconnect action for missing configuration,
  but private ntfy guidance does not identify the server or explain reader
  authentication. setup selects ntfy, schedules enrollment and requests permission;
  it does not configure ntfy.
- `DashboardScreen.kt:175-183` offers system notification settings for every
  unhealthy state, including failures that those settings cannot repair.
- `NotificationOwner.kt:788-792` folds multiple missing prerequisites into setup
  required; missing desired registration precedes the distributor-missing check.
- `NotificationOwner.kt:120-121` silently rejects wrong-origin callbacks.
  [ntfy 1.25.2](https://github.com/binwiederhier/ntfy-android/blob/v1.25.2/app/src/main/java/io/heckel/ntfy/up/BroadcastReceiver.kt)
  returns an existing token's endpoint before consulting its current default
  server. changing that default and retrying registration cannot move the token.

resolved when: setup names the first unmet prerequisite and its specific action,
guides private-server/reader setup before registration, and distinguishes
permission from confirmed enrollment. rejected origins need an explicit reason
and a narrowly scoped recovery path; preserve strict origin validation, existing
pairings and acknowledgement memory. verify both fresh setup and wrong-server
recovery at the actual distributor boundary, followed by background delivery.

the current generic disclosure is specified in `native-notifications.md`; revise
that contract explicitly before changing the flow. trade-off: a few explicit
setup stages expose the operational cost of private push; a general settings
dashboard or notification framework is unnecessary. no code change made.
