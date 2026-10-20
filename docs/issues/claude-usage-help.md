# claude usage help disagrees with presentation

problem: [browser help](../../internal/agentcli/run.go) says `~ marks claude's
estimate`, but [the renderer](../../internal/sessionui/usage.go) and
[presentation contract](../desktop-usage.md) use plain remaining percentages.

impact: help promises a visual distinction that the user cannot find.

reproduction: compare `skid --help` with `claudeUsage`'s remaining-value projection.
source inspection and the earlier help read confirm the mismatch; no provider
or terminal sampling is needed.

resolution: align help with the accepted plain-percentage presentation and check
the literal help output. adding approximation markers requires a separate product
decision. no change to help or numeric rendering belongs in the
[selection correction](../claude-usage-selection.md).
