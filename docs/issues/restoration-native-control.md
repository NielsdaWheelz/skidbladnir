# claude background-job stop qualification

problem: matched claude background-job stop remains `NOT_RUN` on the restored
fleet. this is a native-control coverage gap, not a namespace-separation blocker.

impact: interactive native status, bounded history and terminal closure are
qualified, but native halt confirmation for a background job is unproved. terminal closure
must never be treated as proof that an attached background job stopped.

evidence: the root operator selected dev-server `7c4500d`, with the corrected
plugin from original source `ca9bcf6`, on macbook, devbox and arch. live claude
SessionStart/profile binding, native idle status and bounded saved-history reads
passed on all three hosts. interactive stop returned terminal closed and agent
halt unconfirmed; the exact provider processes were absent at the later check.
that response is contract-valid: the gateway checks foreground exit once after
closure, and later absence does not establish earlier confirmation. the pinned
helper `ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f` uses native stop only for a
matched background job. see [the handoff](../dev-server-handoff.md#qualification-and-remaining-work)
and [stop contract](../agent-control.md).

resolved when: an approved disposable background claude job on each host is
matched and halted through the installed helper; native confirmation and
terminal closure remain separate, and unrelated jobs survive. record
content-free results and remove this file. no provider-home relocation is needed.
