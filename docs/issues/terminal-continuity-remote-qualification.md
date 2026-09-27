# remote terminal continuity qualification

problem: source and installer changes are prepared, but the actual paired
hosts have not installed this generation or qualified forwarding of
`SKIDBLADNIR_CONNECTION` through stock ssh/mosh. the required `SendEnv` and
`AcceptEnv` or tailscale ssh `acceptEnv` settings are external deployment
prerequisites.

impact: a remote terminal may show `remote context unknown`; nested hops,
reconnect, and remote provider swaps are unclaimed on the fleet.

evidence: isolated linux tmux registration and shell proofs and fake-gateway
client integration tests passed on the feature source. `ssh -G` for the three
configured target labels currently reports only `LANG` and `LC_*` as `SendEnv`;
it does not forward the marker. no live fleet ssh/mosh boundary was run.
`docs/terminal-continuity.md` names the full acceptance set.

resolved when: install the matching host generation and forwarding settings on
all three paired hosts; prove distinct simultaneous connections, nested hops,
cwd and provider changes, suspension/resume, reconnect, gateway restart,
source loss, and source-only controls with credential-free evidence. remove this
record then.
