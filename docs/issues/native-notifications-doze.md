# locked-phone doze verification

problem: actual delivery during phone doze remains unverified. android rejects
forced deep idle at `inactive`; no product defect is established.

impact: the spec's locked/dozing delivery boundary remains `NOT_RUN`.
the user approved treating forced-deep-idle verification as a nonblocking
follow-up on 2026-10-09. one real mac-asleep/locked-phone smoke check remains
required for completion.

evidence: source `a43208eb`, observation `f9d1646a` / `8018ef92`, records
completed return code 255, `deep-stopped/inactive`, 82 ms execution and 83.42 s
remaining; no timeout, first idle read or fresh question occurs. normal power
is positively restored. cleanup `a125a029` passes all 27 protections with no
pending cleanup. earlier `920d376b` and `48d20a7c` retain unknown causes.

reproduce only with explicit current-turn tmux/adb and global-doze approval:
run the reviewed temporary driver with a fresh run id, the admitted `c4b4d14a`
build and `364b3eb0` live receipt, and the manually locked phone on its original
tailnet connection. use only the separate qa app and owned isolated tmux socket.

repair `a43208eb` passes inert checks and independent review. it records actual
command budget, completion, timeout and return code, plus only static outcomes
from the [android 16 handler](https://android.googlesource.com/platform/frameworks/base/+/android-16.0.0_r1/apex/jobscheduler/service/java/com/android/server/DeviceIdleController.java).
command, caps, strict idle/lock/history/native proof and all cleanup protections
are unchanged. the upcoming-wake-alarm guard can produce this stopped state in
the tagged handler. after cleanup, read-only device data gives a 3,600,000 ms threshold and a
wake due in 1,271,687 ms: the predicate is currently true, consistent with the
rejection. this later snapshot cannot prove the command-time value. preserve
alarms and settings; recheck eligibility after the wake event before another
cycle. source admission is not delivery acceptance; no raw command output
belongs in evidence.

the agreed later read-only recheck still finds the predicate true: the threshold
remains 3,600,000 ms and a wake is due in 685,697 ms. no power mutation occurred.
stop polling and forced-doze attempts under the lean finish; retain this
unverified follow-up.

resolved when: a fresh current question reaches the exact native notice while
the phone is locked, noninteractive and in proved deep idle, with retained idle
history, untouched-phone confirmation, normal-power restoration and all 27
cleanup protections passing. do not substitute a source test for device delivery.
