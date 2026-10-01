# terminal attention cannot describe pending human input adequately

problem: the former status contract collapsed every recognized dialog to
blocked/waiting and could not distinguish permission, question, setup, or a menu
the user opened; a pending question could not coexist with continuing work.

state: the [terminal observation](../terminal-observation.md) cutover replaces it
in source with independent activity, interaction and notice facts, per-kind
request labels, a needs-input filter and the arming/clearing/ready machine.
recorded evidence
([qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification),
[attention](../reply-notifications.md#disjoint-delivery-and-verification)) meets
these resolution criteria: the screen matrix distinguishes permissions,
structured questions, elicitation, setup and user-opened menus; concurrent work
with a question is representable (live codex); stale or failed observations
cannot fabricate resolution; desktop attention stays consistent through visits,
answers, provider replacement and reconnection; unknown, an outage and a
foreground change cannot bridge working to ready. the false idles
[spec §9](../terminal-observation.md#9-final-state-costs-and-completion) accepts,
codex's cue-less turn starts and claude's remote usage-limit copy, can mint
ready.

three criteria are not met:

- false idle and stale requests: codex's unaccepted residuals
  ([codex residual ambiguity](codex-residual-ambiguity.md)) include r6, a false
  idle on panes at most header + 4 columns wide whose millisecond window can
  follow an armed working sample and so mint ready, and r5, a request that stays
  readable after its provider is killed, and in the needs-input filter, until
  the relaunched provider draws.
- cancellation: no attention run, on either client, declines a request or
  interrupts a turn. those screens were classified, never followed through the
  notification store.
- phone attention under the same conditions.

impact: the phone's request labels, needs-input chip and ready attention are
unproven on a device, and so is the ready machine's handling of a cancelled
request or turn on both clients. on a narrow enough pane, r6 can raise ready as
compaction, a review or an mcp pending turn starts.

resolved when: [physical-phone acceptance](terminal-observation-phone-acceptance.md)
passes its attention and filter rows; an attention run on the desktop and the
phone records a declined request reading idle and raising no ready unless working
is sampled after it, and an interrupted turn reading as spec §9 states; and the
[codex residual ambiguity](codex-residual-ambiguity.md) is resolved. then delete
this record.
