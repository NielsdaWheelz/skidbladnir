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

source qualification, 2026-09-28: the isolated patched helper with actual native
claude 2.1.284 and sdk 0.2.130 passed on arch. two owned background workers reached
real native tool work; a wrong kernel lifetime was refused before dispatch, exact
stop confirmed `stopped` with pid absent, the control retained its exact lifetime,
and saved history survived both stops. this qualifies the source path, not the
installed three-host fleet; the deployment criterion above remains open.

stock source qualification, 2026-09-29: the current pinned unpatched helper with
Claude 2.1.284 and sdk 0.2.130 passed native background identities, exact stop and
saved-history/results survival in a disposable home. wrong kernel lifetime was
refused and the control worker retained its pid. [current qualification](../native-agent-qualification.md)
owns the final package evidence; installed fleet remains NOT_RUN.
