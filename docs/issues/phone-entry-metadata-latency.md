# phone notification entry still waits for rich session metadata

problem: the phone's notification-click path reads the machine's full inventory
to populate `SessionTarget.session`. that model supplies pane-scoped controls,
agent/status/footer and display metadata. a captured lifetime and the new
greeting's pane/foreground cannot honestly replace all those facts.

impact: notification entry keeps a host inventory read after the
[latency cutover](../session-latency.md). it is the only entry path that does.
card entry is direct, reconnect adopts the machine's latest observed session, a
created terminal attaches on the create response's session, and the gateway
checks the lifetime at attach.

evidence: `SkidbladnirController.kt` `openNotificationReference` reads the
machine's inventory through its lane before entry, and `TerminalControl.kt`
derives controls from the rich target. the 2026-10-20 source review found the
dependency and the owner deferred the model split. on 2026-10-22 the owner
accepted the android-core cut of the reconnect read and created-terminal
verification. no phone timing or model-change acceptance is claimed.

resolved when: approve a focused transport-lifetime versus display/control-model
contract, remove this read without fabricating a session or reattributing old
pane facts, and verify cold notification clicks, pane/process replacement,
controls and return through the real phone. keep exact targets and unknown
metadata explicit. broader [interaction redesign](phone-interaction-redesign.md)
remains separate. device work needs current-turn approval.
