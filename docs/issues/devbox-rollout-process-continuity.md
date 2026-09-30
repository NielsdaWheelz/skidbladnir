# devbox rollout process qualification gap

problem: one codex executable sampled before the v0.10.7 gateway rollout was
absent afterward. its role and exit cause remain unknown; the sample is not
a worker-preservation oracle or evidence of a worker regression.

impact: fleet activation is verified, but complete provider-worker continuity
cannot be claimed from this rollout's broad executable sample.

evidence, 2026-09-30: devbox pid `2762947`, ppid `2762535`, start tick
`120047046`; neither pid remained. no tty, cgroup or session association was
captured. bounded rollout journal queries found no records for either pid or
a coredump. the gateway restarted at 15:37:43 utc with `KillMode=process` and
result success; no causal link to the departure is established.

all captured tmux server pid/start identities survived. sampled codex identities
survived on macbook 43/43, arch 2/2 and devbox 7/8; macbook gained three.
machine-handle, bearer and client-config fingerprints, ownership and modes
remained unchanged; jarvis's client config stayed unchanged and jarvis remained
inactive/paused. all three gateways match the published source/artifacts;
authenticated tls pressure and installed three-host fleet/client verification
passed. these observations do not establish terminal/session ownership.

resolved when: establish this process's role and permitted exit from concrete
evidence, or qualify worker continuity on a later approved controlled rollout
using exact tmux session lifetimes and their associated provider pid/start
identities. keep this historical departure explicitly unattributed if it remains
unknown; do not substitute counts of every executable for worker identities.
