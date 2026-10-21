# mac-asleep and naturally dozing phone follow-up

problem: the combined mac-asleep/naturally dozing-phone boundary remains unrun.
the owner previously deferred it; source proof does not replace it.

impact: actual forced-deep-idle delivery is now proved for the standalone release,
but that does not establish natural doze admission or operation with the mac asleep.

actual evidence, 2026-10-20: signed v0.19.3, physical api36 phone, current user-owned
fcm project. ordinary `am kill` left skid absent; screen locked/off, wakefulness
Dozing, controller IDLE/forced and network-policy idle restrictions simultaneously
proved before trigger and around native notice. fresh owned episode571964 arrived;
fcm restarted skid while global idle remained. no force-stop or permanent app
whitelist. temporary `min_time_to_alarm=0` bypassed only the upcoming-alarm entry
guard; the full original constants/global-row absence/device-config/power policy
were restored. no alarm cancellation or battery simulation. the earlier first
attempt entered maintenance and failed strict entry; no trigger occurred and its
restoration passed. retain it as failed, not delivery evidence.

resolve: a separately authorized real mac-asleep/locked-phone smoke check with a
fresh current episode, actual natural idle restrictions and content-free native
identity proof, followed by normal-power restoration. keep the exact owned-session
and phone authorization boundaries; do not recreate retired gates or reuse the
removed older qa package/build instructions.
