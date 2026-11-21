# The install and phone procedures are unwalked

Problem: [install](../install.md) and [phone](../phone.md) were written from the
code at v0.23.0. Nobody has followed them on a fresh macOS or Linux computer, or
paired a phone from them alone; the campaign's fresh-machine walks were dropped.

Impact: a stranger can meet a failure the pages do not mention: a missing
prerequisite, a different message, or a step in the wrong order.

Evidence: the publication campaign's tracker, deleted at its close
(`git show b0dd12a:docs/publication.md`): its W2.0 and W3 rows (dropped,
2026-10-26), and its fresh Linux install, `NOT_RUN` from the start.

Constraint: test machines join the owner's tailnet as ephemeral nodes, each use
approved by the owner and the nodes removed after it; there is no throwaway
tailnet, so a stranger's tailnet defaults are never seen live (owner,
2026-10-25).

Resolved when: a fresh macOS 26 machine and a fresh Ubuntu 24.04 or Arch machine
each reach a typing session, and a phone pairs, by following only the README and
these pages, with the pages corrected for whatever differed.
