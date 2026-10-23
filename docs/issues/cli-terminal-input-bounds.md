# the cli copies the gateway's terminal input bounds

problem: `skid send`, `text`, `keys` and `read` check their input at parse
against private copies of the terminal input contract. `validText` is 1-32768
bytes of utf-8 without nul (`internal/agentcli/run.go:367-370`); stdin is read to
the same bound (`run.go:411-418`); `--max-bytes` is 1-32768 (`run.go:192-195`);
keys are 1-16 of 11 names (`run.go:348-357`). the owners are the gateway's
`terminalRequest.shaped`, the only bounds check since #156
(`internal/gateway/terminal_control.go:35-58`), and tmux's key encoding
(`internal/tmux/control.go:105` `Keys`). nothing ties the copies to their owners,
and no test pins either side.

impact: none today; the values agree. a change on either side drifts silently.
a larger gateway bound or a new key name is refused by the cli as usage (exit 2,
`invalid_input`); a smaller bound is sent and answered 400 `InvalidRequest`
(exit 1). desktop-core (#155) kept the copy because dropping parse-time
rejection turns exit 2 into exit 1.

evidence: code reading at `defbf3f`. #155 moved the copy out of fleetclient into
agentcli; #156 then moved the owner from agentcontrol to the gateway.

fix: declare the text predicate, the read bound and the key names once in
`internal/api`, which the gateway and go clients already share
([rules](../rules.md#trust-boundaries)). `shaped` and the cli parse call them,
and tmux's switch encodes exactly that list.

resolved when: each bound and the key list has one declaration that the cli and
the gateway both call, and the cli's exit codes are unchanged.
