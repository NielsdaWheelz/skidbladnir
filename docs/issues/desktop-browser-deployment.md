# desktop browser deployment

2026-09-17: publication and fleet deployment are authorized, but blocked before
release creation. the installed release and both repository pins remain v0.5.0;
the organized desktop browser is not installed.

`scripts/build release-assets` requires the native darwin/arm64 signing host;
`scripts/fleet` also requires the macbook operator. the macbook is reachable,
but a batch-mode ssh probe to
`nnandal@niels-eriks-macbook-pro.tail6340bd.ts.net` rejects this workspace's
available identity (`id_ed25519_github`). no usable alternative is configured.
skipping the darwin browser test does not waive the release signing boundary.

resume through an authorized macbook access route: release verified clean main
with `scripts/release`, validate and publish its draft, commit the real artifact
digests to both release pins, and deploy through dev-server's existing apply
path. preserve the hosts-first update order and required boundary approvals.
do not replace installed binaries manually or fabricate pins before artifacts
exist. delete this issue once the published release and required installed
versions are verified; record delivery evidence in the roadmap. the separate
[darwin runtime issue](desktop-browser-runtime-acceptance.md) remains independent.
