# live claude cache acceptance

check the installed [usage band](../architecture.md#desktop-usage) and
one-line footer against claude's web settings → usage on the same account.
the private file may predate the web view. opening `/usage` in claude code
can itself refresh the cache being checked.

on 2026-11-20 utc, the v0.27.0 claude-usage route and installed formatter
passed on all three hosts. the fleet returned general weekly and fable amounts
and a fable reset. the cases below need their own live observations.

| live case | result |
| --- | --- |
| compare cached 5h, 7d and fable amounts and supplied reset times with the web view, accounting for the fetch age | `NOT_RUN` |
| compare footer used with band left. verify the general weekly caption, the gold overlay's half scale and the independent fable countdown | `NOT_RUN` |
| observe a second host supply a newer reading with no open pane. restart the displays while the source is offline. verify the amounts persist, the countdown crosses its deadline and the next reading updates only the supplied fields | `NOT_RUN` |

the check requires current owner approval for the fleet and provider reads.
record case outcomes only.
