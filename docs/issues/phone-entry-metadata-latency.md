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

the [workspace review](workspace-coherence.md) recommends revisiting that decision;
recording the recommendation does not approve the change. start attachment from
the captured machine/session lifetime, keeping rich metadata optional until fresh
facts arrive. the gateway remains the lifetime admission owner. dependent pane and
provider controls stay unavailable until their facts are known; do not manufacture
a `TmuxSession` or reattribute old pane facts merely to remove the read.

ownership: split these responsibilities in the existing controller/attachment
model, with the same separation used by [foreground return](phone-terminal-continuity.md).
socket connectivity, attached lifetime and fresh control metadata are different
states. cost: that distinction becomes explicit in the presentation model. avoid a
parallel cache or observation transport. keep failure and return navigation useful,
and do not open the keyboard or replay input on reconnect. actual attachment can
resize the shared desktop terminal under `window-size latest`; it is not passive.

resolved when: an approved contract separates the transport lifetime from the
display/control model, this read goes without fabricating a session or
reattributing old pane facts, and cold notification clicks, pane or process
replacement, controls and return are verified on the real phone. phone work needs
current-turn approval. the [interaction redesign](phone-interaction-redesign.md)
is separate.
