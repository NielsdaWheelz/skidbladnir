# list bounds its inventory before enrichment, not its final envelope

problem: `skid list` applies the 1 MiB bound to the raw fleet inventory
(`internal/fleetclient/client.go:176`), before the group filter and remote-context
enrichment, and `render` skips its own envelope bound for list
(`internal/agentcli/run.go:471`). when only the enriched envelope exceeds 1 MiB,
`list --json` prints nothing and exits 1 (`:481-484`), and the human list prints
the whole table and exits 0. the cli contract is one envelope per `--json` call.

impact: an automation caller gets an empty stdout instead of an `output_limit`
envelope. it needs hundreds of remote rows with multi-KiB working directories, so
no real fleet has hit it.

evidence: 2026-10-22, characterization against 3ef8c2e (`list --json enriched over
1MiB`, `list enriched over 1MiB human`).

fix: drop the bound in the inventory read and bound the final envelope in
`render` for list too, in both modes, reporting `output_limit`/`not_sent`. it
changes human list output and stops failing handle resolution on a large combined
inventory, so it needs the owner.

resolved when: an enriched `list --json` over 1 MiB prints the `output_limit`
envelope and exits 1, and the human list's behaviour at that size is chosen and
stated in the cli contract.
