# live claude quota acceptance

the [desktop quota contract](../architecture.md#desktop-usage) passed isolated
80×24 browser checks in colour and plain text. the deployed gateway and real
claude footer still need a live comparison.

| live case | result |
| --- | --- |
| compare the per-window minima in the caption and expanded band with the eligible panes | `NOT_RUN` |
| hide the last eligible footer and verify the stale label and band age | `NOT_RUN` |
| restore the footer and verify the next read replaces the stale display | `NOT_RUN` |

the check requires current owner approval for the fleet read and for hiding
and restoring an identified pane's footer. record case outcomes only.
