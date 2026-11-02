# notification recovery qualification

problem: the installed dev-server hub's observer stopped after a persistence
failure. automatic recovery is implemented in this branch; installation and
native mac/phone delivery remain unqualified.

impact: the installed observer can remain unavailable after storage returns,
suppressing fleet readiness and notifications while terminal classification works.

evidence: authorized read-only checks on 2026-10-30 found direct inventories
returning 200 with recognized status, while hub notification state returned
503 `NotificationsUnavailable`. the valid observer file stopped advancing at
2026-10-29 23:56:54 utc, revision 262434. workspace writes reported errno 28 just
before `Notifications.Unavailable`; workspace persistence subsequently recovered,
but notifications did not. the gateway remained running with zero restarts.
retained logs and cgroup counters provided no oom evidence. the notification
logger lacked operation/errno, so disk exhaustion is strongly supported rather
than proven for that exact write.

cause: a failed observer save permanently latched unavailability and stopped its
owner loop. standalone kept the gateway running, so its supervisor could not
recover the observer. constructor failures also discarded it.

repair: [architecture §12](../architecture.md#12-notifications) owns automatic
five-second readmission, paused attention, durable acknowledgements, current
configuration, fresh evidence and client hints. corrupt state stays unavailable
until owner repair. [disk exhaustion](hub-disk-exhaustion.md) remains separate.

isolated acceptance: temporary exported-owner integration checks demonstrated
original failures and candidate recovery across all six store writers, startup,
corruption, stream closure, fresh readiness, configuration changes and receiver
revocation. a real private-tmux websocket survived the outage. linux
path-filtered directory-fsync injection demonstrated a landed acknowledgement
plus 503, then recovery preserving both acknowledgements. sender replacement,
in-flight registration revocation and diagnostic behavior were also checked.
these checks do not prove installed startup or physical notification delivery.

remaining: with current-turn owner approval, install the reviewed candidate on
the hub and qualify its startup; observe a fresh busy-to-idle transition yielding
matching hub readiness, client ready presentation and native mac/phone delivery.
use only an explicitly approved test session and scratch provider frames; no real
provider account or existing session input is needed. record the source and
content-free acceptance evidence in the change review, then delete this issue.
