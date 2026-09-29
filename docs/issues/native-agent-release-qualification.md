# native agent release qualification

problem: the signed skid v0.10.4 release is published, but the dev-server pin
still selects v0.10.3. the new helper is intentionally marked
unqualified for gateway application.

impact: ordinary installation cannot activate the native gateway/helper
contract. source and disposable mac checks do not establish installed linux apparmor,
installed-fleet behavior or physical-phone usability.

evidence: [source qualification](../native-agent-qualification.md) records the
provider, helper and client boundaries. [dev-server draft pr #139](https://github.com/NielsdaWheelz/dev-server/pull/139) stages the
pinned package but remains unmerged. its ubuntu noble x86_64 namespace check
passed under a temporary policy; persistent installation and full provider
control are `NOT_RUN`.

resolved when: pin the exact v0.10.4 host/apk release in dev-server, qualify it
with the final provider/helper package on the intended linux boundary,
then complete approved fleet and phone acceptance. record content-free results
and remove this issue.
