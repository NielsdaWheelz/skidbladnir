# terminal attention cannot describe pending human input adequately

problem: the former status contract collapsed every recognized dialog to
blocked/waiting and could not distinguish permission, question, setup, or a menu
the user opened; a pending question could not coexist with continuing work.

state: the [terminal observation](../terminal-observation.md) cutover replaces it
in source with independent activity, interaction and notice facts, per-kind
request labels and a needs-input filter; [terminal attention](../reply-notifications.md)
owns the durable quiet/armed/ready machine.
recorded evidence
([qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification),
[historical attention](../reply-notifications.md#historical-qualification)) meets
these earlier resolution criteria: the screen matrix distinguishes permissions,
structured questions, elicitation, setup and user-opened menus; concurrent work
with a question is representable (live codex); stale or failed observations
cannot fabricate resolution; desktop attention stays consistent through visits,
answers, provider replacement and reconnection; unknown, an outage and a
foreground change cannot bridge working to ready. the accepted
[attention revision](../reply-notifications.md#behavior) supersedes the
unknown/outage and request-resolution rules; its implemented client memory has
[separate qualification](../reply-notifications.md#qualification). the false claims
[spec §9](../terminal-observation.md#9-final-state-costs-and-completion) accepts
are the exceptions: its false idles can mint ready; codex's stale request after
a relaunch and wrong picker kind (codex grammar §6 c13, c11) can enter the
needs-input filter; and narrow mcp requests (c15), the mcp tool-approval
fallback in the legacy question view (c16) and an mcp approval whose option
block holds an unparsed row (c17) read as another request kind.

two criteria are not met:

- cancellation: no attention run, on either client, declines a request or
  interrupts a turn. those screens were classified, never followed through the
  notification store.
- phone attention under the same conditions.

impact: the prior phone run passed the other recorded rows; concurrent work with
a request, generic needs-input, production-gateway acknowledgement protection and
spoken talkback remain [unqualified](terminal-observation-phone-acceptance.md).
the new isolated phone run proves delayed-response acknowledgement protection
through the real controller/datastore/WebView and a scripted TLS/WSS peer.
actual declined-request and interrupted-turn attention journeys remain unrun on
both clients. the new attention qualification proves typed request → idle and
notice precedence through real client owners, including isolated physical-phone
TLS/WSS peers; it does not prove those production-provider journeys.

resolved when: [physical-phone acceptance](terminal-observation-phone-acceptance.md)
passes its attention and filter rows; an attention run on the desktop and the
phone records declined and interrupted journeys against the accepted
[attention cases](../reply-notifications.md#acceptance), including request → idle
raising ready outside a visit and an unanswered request surviving a visit.
ready makes no completion or answer claim. then delete this record.
