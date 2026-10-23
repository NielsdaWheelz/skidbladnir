# phone notification entry waits for rich session metadata

problem: a notification click reads the machine's full inventory before entry
(`openNotificationReference`, `SkidbladnirController.kt:756-780`) to obtain the
`TmuxSession` that supplies pane-scoped controls, agent, status and display
metadata (`TerminalControl.kt`). a captured lifetime cannot honestly replace those
facts.

impact: notification entry is the only entry path that waits for an inventory
read. card entry is direct, reconnect adopts the machine's latest observed
session, a created terminal attaches on the create response's session, and the
gateway checks the lifetime at attach.

decision: the owner deferred splitting the transport lifetime from the
display/control model (2026-10-20).

resolved when: an approved contract separates the transport lifetime from the
display/control model, this read goes without fabricating a session or
reattributing old pane facts, and cold notification clicks, pane or process
replacement, controls and return are verified on the real phone. device work needs
current-turn approval. the [interaction redesign](phone-interaction-redesign.md)
is separate.
