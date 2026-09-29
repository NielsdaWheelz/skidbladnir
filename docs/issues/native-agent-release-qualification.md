# stock native conversation release qualification

problem: the stock conversation-target source passed isolated acceptance, but
its coordinated release and installed-fleet acceptance remain open. published
v0.10.4 uses the former selected-view contract; dev-server draft pr139 awaits
the new release pin before enabling installation.

impact: installed-fleet behavior and native-output unread acknowledgement cannot
be claimed from former fork evidence or isolated source checks.

evidence: [current qualification](../native-agent-qualification.md) separates stock
final installed Darwin/Linux and Claude evidence from superseded tests.
[installer draft](https://github.com/NielsdaWheelz/dev-server/pull/139)
remains unmerged; no phone/fleet deployment occurred. original working copies
remain untouched.

resolved when: new signed coordinated release/pins agree, and approved installed-host/phone checks
pass. NOT_RUN is never pass. record content-free evidence and remove this issue.
