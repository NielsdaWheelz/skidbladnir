# list bounds its inventory before enrichment, not its final envelope

problem: `skid list` applies the 1 MiB bound to the raw fleet inventory
(`fleetclient` `inventory`), before the group filter and remote-context
enrichment. [agent control](../agent-control-ux.md) says the bound applies to
the final projected envelope and that `--json` emits exactly one envelope. when
only the enriched envelope exceeds 1 MiB, `list --json` prints nothing and
exits 1 (`agentcli` `render`), and the human `list` prints the whole table and
exits 0.

impact: an automation caller gets an empty stdout instead of an `output_limit`
envelope. it needs hundreds of remote rows with multi-KiB working directories,
so no real fleet has hit it.

evidence: 2026-10-22, the desktop-core characterization against 3ef8c2e (T2
cases `list --json enriched over 1MiB` and `list enriched over 1MiB human`).
the desktop-core cleanup kept this behaviour to stay behaviour-preserving.

fix: drop the check in `inventory` and bound the final envelope in `render`
for list too, in both modes, reporting `output_limit`/`not_sent`. it changes
human list output and stops failing handle resolution on a large combined
inventory, so it is an owner-visible decision.

resolved when: an enriched `list --json` over 1 MiB prints the `output_limit`
envelope and exits 1, and the human list's behaviour at that size is chosen
and documented.
