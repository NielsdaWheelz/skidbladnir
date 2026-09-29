# native agent release qualification

problem: the signed skid v0.10.4 release is published, but no coordinated
native-agent installation or physical-phone acceptance has been recorded.
dev-server main still pins v0.10.3. its draft update pins v0.10.4 but keeps
gateway application unqualified pending installed-host acceptance.

impact: the native gateway/helper contract is not qualified on the installed
fleet. source and disposable mac checks do not establish installed linux apparmor,
installed-fleet behavior or physical-phone usability.

evidence: [source qualification](../native-agent-qualification.md) records the
provider, helper and client boundaries. [dev-server draft pr #139](https://github.com/NielsdaWheelz/dev-server/pull/139)
stages the pinned package but remains unmerged. its exact mac installer release
package passed a controlled finalized reply through the published gateway and frozen
helper. a disposable ubuntu noble x86_64 provider package passed native
view/control and bubblewrap checks under a temporary policy. persistent
installation and policy activation are `NOT_RUN`.

resolved when: qualify the exact v0.10.4 release with the final provider/helper
package on the intended hosts, then complete approved fleet and phone
acceptance. record content-free results and remove this issue.
