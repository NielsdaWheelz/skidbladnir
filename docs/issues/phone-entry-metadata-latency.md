# phone entry still waits for rich session metadata

problem: phone notification-click and reconnect paths obtain full inventory to
populate the existing `SessionTarget.session`. that model supplies pane-scoped
controls, agent/status/footer and display metadata. a captured lifetime and the
new greeting's pane/foreground cannot honestly replace all those facts.

impact: these two paths retain a host inventory dependency after the
[latency cutover](../session-latency.md). ordinary card entry is already direct;
created-terminal post-write verification also retains its existing contract.

evidence: `SkidbladnirController.kt` notification-click/reconnect paths and
`TerminalControl.kt` derive controls from the rich target. 2026-10-20 source
review identified the dependency; the owner explicitly deferred the model split
and retained both lookups. no phone timing or model-change acceptance is claimed.

resolved when: approve a focused transport-lifetime versus display/control-model
contract, remove these whole-host reads without fabricating a session or
reattributing old pane facts, and verify cold notification clicks, reconnect,
pane/process replacement, controls and return through the real phone. keep exact
targets, unknown metadata and post-write admission explicit. broader
[interaction redesign](phone-interaction-redesign.md) remains separate. device
work needs current-turn approval.
