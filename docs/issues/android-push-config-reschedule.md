# push reenable skips enrollment with unchanged credentials

problem: the paired phone adopts a push-only config change but watchFleet returns
when computer credentials are unchanged before notifying the push owner.

impact: omit android_push→apply disables cleanly; restoring the identical public
configuration→apply leaves all receiver generations null and displays setup
required. automatic reenable does not happen without another user action.
session management remains healthy. no setup tap/restart/reset is the accepted
fix for this missed event.

evidence, 2026-10-20: actual signed v0.19.3 config4 disabled; exact original yaml
restore config5, same phone/project/permission, generations remain null at two
reads38s apart. SkidbladnirController.watchFleet early credential-set equality
return precedes notifications.fleetChanged. no credential/pairing/ack reset.

resolve: notify the existing idempotent notification owner when accepted fleet
configuration changes, independently of inventory-credential optimization. prove
real controller transition red→green with unchanged credentials, no enrollment
churn on unchanged state, preserved removal/cancellation semantics; independent
review, temporary probe retirement, immutable release, then actual ordinary
phone restoration/enrollment without a setup tap and complete final fleet match.
