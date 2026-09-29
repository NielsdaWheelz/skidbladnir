# native agent release qualification

problem: skid and helper source changes are merged, but the published skid release and
dev-server pin still select v0.10.3. the new helper is intentionally marked
unqualified for gateway application.

impact: ordinary installation cannot activate the native gateway/helper
contract. source and disposable mac checks do not establish linux apparmor,
installed-fleet behavior or physical-phone usability.

evidence: [source qualification](../native-agent-qualification.md) records the
provider, helper and client boundaries. [dev-server draft pr #139](https://github.com/NielsdaWheelz/dev-server/pull/139) stages the
pinned package but remains unmerged; its ubuntu noble x86_64 namespace check
is `NOT_RUN`.

resolved when: publish and pin one exact skid host/apk release, qualify that
release with the final provider/helper package on the intended linux boundary,
then complete approved fleet and phone acceptance. record content-free results
and remove this issue.
