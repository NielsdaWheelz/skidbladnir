# restored desktop entry point is missing

problem: `skid` is absent on macbook, devbox and arch. devbox and arch also lack
the desktop peer config. the installed `skidbladnir` binary already contains the
browser; a new release is unnecessary.

cause: the prior retirement removed the `skid` symlink, but restoration omitted
it and incorrectly made it optional in the handoff. phone/gateway checks missed
the existing desktop entry contract and Linux client provisioning.

resolution underway: dev-server restores the exact owned public link throughout
its installation lifecycle; the root operator provisions the existing three-peer
client config. fleet verification now checks both links and invokes the real
installed cli for complete read-only inventory. no archive or receipt change.

resolved when: reviewed sources are pushed, all three login shells resolve
`skid`, real client inventory succeeds on each host, and bare `skid` opens and
quits its browser without touching existing sessions. remove this issue after
recording content-free evidence in the handoff.
