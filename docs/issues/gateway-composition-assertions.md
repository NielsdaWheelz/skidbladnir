# repeated gateway composition assertions

problem: `gateway.New` rechecks workdir, machine, platform and pairing guarantees
already established by its only production supplier in `serveGateway`.

evidence: startup checks `workdir.New` and `machine.Load` errors; the machine
handle representation is private. platform constructors return their respective
linux/darwin constants. startup passes the unconditional `pairing.NewSlot`
result directly. these guards neither admit new external input nor reobserve
changing runtime state.

`agentcontrol.New` also repeats the manager/nonempty absolute-path guarantees
and returns an error no current caller can receive. its sole caller has checked
`sessions.New` and uses the unchanged path admitted by `hostconfig.Load`; the
constructor acquires no resource or live fact. return the service directly and
make startup's assignment linear. retain the constructor's private-field owner.

resolved when: remove these repeated composition checks, retaining all
actual file, home, platform and session admission. verify the complete supplier
and constructor paths and builds. no replacement guards or wrapper types.
the cost is losing immediate, specific diagnostics for a future composition
bug; current valid construction and request behavior are unchanged.
