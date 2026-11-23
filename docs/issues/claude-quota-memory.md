# Live Claude cache acceptance

Check the installed [usage band](../architecture.md#desktop-usage) and
one-line footer against Claude's web Settings → Usage on the same account.
The private file may predate the web view. Opening `/usage` in Claude Code
can itself refresh the cache being checked.

On 2026-11-20 UTC, the v0.27.0 `claude-usage` route and installed formatter
passed on all three hosts. The fleet returned general weekly and Fable amounts
and a Fable reset. The cases below need their own live observations.

| Live case | Result |
| --- | --- |
| Compare cached 5h, 7d and Fable amounts and supplied reset times with the web view, accounting for the fetch age | `NOT_RUN` |
| Compare footer used with band left. Verify the general weekly caption, the gold stroke's half scale and the single weekly countdown | `NOT_RUN` |
| Observe a second host supply a newer reading with no open pane. Restart the displays while the source is offline. Verify the amounts persist, the countdown crosses its deadline and the next reading updates only the supplied fields | `NOT_RUN` |

The check requires current owner approval for the fleet and provider reads.
Record case outcomes only.
