# fleet apply progress

problem: `scripts/fleet accept-host` runs two installer applies while the devbox
installer buffers ansible output. several minutes can pass without a progress
message, so normal work looks stuck.

evidence: during v0.6.0 rollout the user interrupted quiet attempts after roughly
54 seconds and 84 seconds. a subsequent process-only inspection showed ansible
advancing through fresh workers. the same pasted command also suffered a line
break inside a long flag; future handoffs should put each flag on its own line.

resolved when: the existing fleet wrapper announces which host and apply pass
is starting before it blocks. keep output content-free and preserve the existing
quiescence check. this needs ordinary phase text, not polling or a progress ui.
