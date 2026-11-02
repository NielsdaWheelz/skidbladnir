# recurring hub disk exhaustion

problem: dev-server repeatedly exhausts writable storage. the responsible workload
and retention policy have not been established.

impact: workspace checkpoints are delayed; the installed notification observer
failed persistence and stayed unavailable. automatic observer recovery does not
prevent further disk exhaustion.

evidence: authorized read-only inspection on 2026-10-30 found workspace writes
failing with errno 28 (`no_space`) on october 29 at 23:57:49 utc and october 30
at 17:25:49 and 17:38:49 utc. rootless docker also logged no-space failures near
17:25. around 18:36 the filesystem had 9.84 gb free and ample free inodes.
these observations establish exhaustion, not which workload consumed the space.

follow-up: identify the consumer and temporary growth pattern, then define a
bounded retention/capacity policy without deleting active work. the owner scoped
this outside [notification recovery](../architecture.md#12-notifications).

resolved when: the cause is established and the agreed storage policy prevents
recurrence through the responsible workload. any live inspection or cleanup needs
the applicable current-turn authorization.
