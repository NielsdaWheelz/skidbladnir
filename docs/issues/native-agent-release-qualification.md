# installed native conversation qualification

problem: phone-native output presentation and unread acknowledgement are not
qualified by the retirement's normal attachment checks.

impact: native-output unread behavior on the physical phone remains unclaimed.

evidence: [current qualification](../native-agent-qualification.md) separates stock
final installed Darwin/Linux and Claude evidence from superseded tests.
[installer coordination](https://github.com/NielsdaWheelz/dev-server/pull/139)
pins the same helper generation. v0.10.6 is now installed on all three hosts and
the phone; installed native lifecycle, cli tls, jarvis uid reads and owner
phone attachment pass. those checks do not exercise phone-native reply display
and captured-only acknowledgement.

resolved when: physical-phone native output presentation and captured-only
unread acknowledgement pass. NOT_RUN is never pass. this separate feature
acceptance is not a herdr-retirement gate.
