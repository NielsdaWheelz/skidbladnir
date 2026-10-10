# mac notification QA energy report

problem: user reports severe heat/battery drain and activity monitor's 12-hour
power value 9485.15 for the owned notification QA application on 2026-10-03.
historical cause is unresolved; notification release acceptance is incomplete.

containment: all-process executable inspection found no notification helper
running; the owned QA launch agent was already unloaded and is now disabled.
the exact reported signed QA bundle was moved out of Applications into its
private forensic directory. no permanent notification deployment exists.

evidence: bounded offline run of that exact signed version 0.13.1 sampled
0.0–0.1% CPU, constant cumulative CPU 0.06 seconds and about 49 MB RSS over
twenty seconds. native notice inspection ran twice; no tight retry loop was
observed. the child stopped afterward. empty/error streams use 1–30-second
failed-stream backoff; source has no reply/effect feedback loop. unchanged state reads
still inspect notices/settings; this is avoidable work, not a proved cause.
separate nexus `feature/latest-models-cutover` next-server was using 122–126% CPU.
at the user's explicit request, its verified five-process make/bash/bun/node/server
job was stopped with SIGTERM. all five pids and their process group disappeared;
port 3000 has no listener. no forced kill was needed. this does not establish the
notification application's historical energy attribution.

activity monitor's 12-hour power is historical average relative energy impact,
not current CPU or watts; its value alone does not identify the active heater.
[apple's column definition](https://support.apple.com/guide/activity-monitor/view-energy-consumption-actmntr43697/mac).
cleanup preserves the surviving bundle and quarantine descriptor under
`~/.local/state/skidbladnir/notification-forensics/`; temporary profiling
harnesses/artifacts are retired. no credential/content logging.

additional temporary actual owner/socket/authenticated-http measurement: quiet
five-second interval used 199 microseconds CPU, with one stream, no autonomous
requests/effects and no further device revision. immediate EOF and 503 failures
waited at least one then two seconds between attempts and created no repeated
native effects. injected retry/effect faults fail the assertions. race/vet pass;
these tests omit native presentation and do not resolve historical attribution.

additional defect: every valid initial SSE hint reset reconnect delay. a real
owner/socket/TLS test reproduced hint-then-EOF retry gaps of 1.016 and 1.006 seconds;
each reconnect fetched state and woke native reconciliation. the source repair
resets delay only after an attempt lasts at least the existing 30-second cap.
the same test now backs off 1 then 2 seconds; a held-open 30-second attempt resets
to 1 second, and cancellation joins. hints still fetch immediately and unchanged
snapshots remain write-free. short useful streams may wait up to 30 seconds
after disconnecting. targeted owner/socket/visit/planner/idle race tests, vet and
diff checks pass; removing the stability reset fails its specific assertion.
the temporary tests are retired. this is a proved retry defect, not a proved
historical cause.

2026-10-04: two bounded native profiles of the repaired signed 0.13.3 app used
the actual private observer, fresh device state and the real operator home.
after five seconds of warmup, thirty seconds averaged 0.048% cpu in background
and 0.085% with setup visible; peak samples were 0.248% and 0.323%.
rss was 48 and 69 mb. both children exited cleanly within thirty-six seconds.
healthy ingress, unchanged native inspection and the setup window produced no
observed cpu loop. permissions were unqualified at that temporary bundle path;
these short process counters exclude system services, graphics and watts.
they neither explain the historical report nor establish installed-path consent.

bounded native 0.13.6 profiles averaged 0.039% cpu over twenty measured seconds
in background and 0.448% over thirty with setup visible, after five seconds of
warmup; peaks were 0.145% and 1.732%. both children exited within the 45-second
cap. actual temporary-path bundle lookup and authorization failed; these
measurements neither qualify installed-path behaviour nor resolve the historical
energy report.

2026-10-08: the old quarantine descriptor, executable and icon are missing.
original paths are recovered from this task's historical tool output; the
remaining plist and resource seal match their original hashes. exact icon
copies survive, but the scoped binaries and archives contain no matching
executable. no rebuild or re-signing substitutes for that historical image.
exact old-image reproduction is blocked. current-helper qualification captures
the actual surviving quarantine state and preserves containment separately.

current signed installed-path `0.13.12` short healthy capture `bf03a109` records
eight samples over 35.006 seconds after five seconds of settling: 0.0076925
seconds cpu, mean 0.022% of one core, peak interval 0.142%, maximum sampled rss
47.125 mib and physical footprint 16.14 mib. package idle wakeups total two;
interrupt wakeups total 262. the helper stops normally at 40.254 seconds, and
all 12 cleanup checks pass. these are process counters with endpoint-bound
availability/background checks; sustained healthy/offline/setup cost and the
historical attribution remain unresolved.

2026-10-09: approved 150-second installed-path healthy/offline captures
`0d939ae4`/`5daf57da` average 0.013%/0.016% of one core; maximum sampled rss is
47.72/47.55 mib. each has 31 samples and all 12 cleanup checks passing, with
normal lifetime below 165 seconds. the internal observer is restored without
service restart or changed routes, epoch, subscription or session identities.
setup `42d6f480` remains `NOT_RUN` after final app/window admission failed;
the user switched away or closed setup. its exact final guard failure is
unproved. the user deferred a setup-only retry until evening. this optional
diagnostic is not a spec acceptance requirement; historical attribution is a
separate forensic follow-up. these finite process counters neither identify
the missing historical image's cause nor prove overnight cost.

resolved when: identify and reproduce the relevant high-energy path using
bounded actual signed/native profiling, repair its responsible owner if a
defect exists, and verify sustained idle/offline/foreground resource behavior.
do not relaunch the test login app or infer resolution from the short offline
sample. all required native acceptance remains separate.
