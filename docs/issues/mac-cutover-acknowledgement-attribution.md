# mac post-startup acknowledgement attribution

problem: one existing mac notification record advanced acknowledgement after the
new app started; the historical native dismissal/output cause was not captured.
no product defect is established.

impact: exact durable cutover memory preservation is proved before app startup;
this later live acknowledgement cannot be attributed to an output visit or native
notice disappearance retrospectively. do not call it a proved semantic ack or an
accepted android tray-clear consequence.

evidence, 2026-10-20: frozen schema4 bytes matched immediately before startup;
all11 keys/resetId/epoch survived and counters never decreased. one prior posted
record changed presentation to closed; a DIFFERENT record advanced ready
acknowledgement0→569314. these are distinct keys and must not be attributed to
one event. no manual memory writes
or reset occurred. later focus query showed ghostty foreground but no current
tmux client for that exact existing target; that cannot prove historical focus.

resolve: reproduce and qualify notice/click/output/restart semantics on the sole
owned test target, preserving existing records; retain this historical attribution
gap explicitly unless contemporaneous content-free evidence establishes its cause.
do not add a lifecycle database or claim an unproved cause.
