# two physical phones: push and independent acknowledgement

status: owner-deferred on 2026-10-20. this acceptance gap does not block the
currently approved single-phone qualification. it is not a pass.

problem: a second physical android phone is unavailable for simultaneous
standalone push/admission qualification. separate principals and isolated clients
on one device do not establish two real installations' background behavior.

impact: two-device delivery, independent acknowledgement under real push/doze,
rotation/removal and new-machine discovery remain unqualified on physical phones.
source still supports multiple independently enrolled phones; no singleton
credential or shared acknowledgement is permitted.

evidence: isolated physical test-package journeys cover durable pairing,
registration generations, native presentation, output-only acknowledgement and
workmanager recovery. sender/membership probes cover independent synthetic
principals. one project-agnostic probe apk now passes real user-owned direct-fcm
delivery across two projects and ordinary process death on one phone. that does
not qualify production integration or two-phone behavior. installed fleet
activation remains separate. no two-phone result is inferred from these probes.

resolved when: pair two physical phones independently; add a computer and observe
automatic discovery on both; generate current attention, acknowledge only a and
verify b remains unseen. qualify rotation, delayed/reordered hints, process
reclamation/doze, network loss/recovery and removal during sender outage followed
by hub transfer with destination-local sending credentials. removed identities
must not regain delivery or terminal access.
