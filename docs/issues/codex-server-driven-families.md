# codex families that need server or account state

problem: required families of the
[codex grammar](../terminal-observation-codex.md#1-frozen-capability-table) whose
trigger a local endpoint cannot produce are `NOT_RUN` everywhere:

- the network approval, which needs a managed network-proxy policy;
- user verification, which the app-server advertises only when the device
  supports it (this client rejected the request);
- the parent-owned sub-agent view, which needs a thread that refuses direct input
  (multi-agent v2 spawns, which the 0.159.2 router rejected);
- remote image rows and image-only prompts, which appear only in history
  rehydrated from another client.

permitted families of the same kind stay informative: the provider pickers other
than `Approaching rate limits`, footerless precaution, reserve and safety pickers,
the `What we detected` pager, inline banners, url elicitation, model migration,
hooks review, the daemon-recovery continuation and the Astra and Max/Ultra
effects.

impact: the required ones read by source and authored frames only. the network
approval and user verification share the exec overlay's footer, so a drift there
would most likely read unknown; the sub-agent view is an ambiguity control whose
`unknown` composer is unproven live.

evidence: upstream source at the grammar's pinned commit and the attempts recorded
in the capability table's qualified column.

resolved when: each required family is produced live (with the server or account
state it needs) and classifies as the table states, or is shown unreachable for
the deployed configuration and reclassified with a reason; then delete this
record.
