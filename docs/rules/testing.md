# testing status

2026-09-17: the user retired all behavioral tests and their harnesses. this
interim policy supersedes the former test tiers, mandatory red/green workflow,
and test-gate prescriptions throughout the repository. the next pr defines the
replacement system; this pr adds no behavioral tests or replacement framework.

product requirements remain. a successful build or static check does not prove
product behavior. historical test results apply only to their recorded source;
removed, unavailable, or unexecuted checks cannot be reported as passes.

current commands:

- `scripts/check static`: formatting, syntax, shell/go/android lint, dependency
  integrity, and catalogue/generated-asset checks.
- `scripts/build`: go and android debug builds.
- `scripts/check verify`: static checks and builds; also the hosted ci command.
- `scripts/check release --allow-release-verification TAG`: signed release asset
  verification under the existing release approval and environment requirements.
- `scripts/check published-release --allow-public-release-read TAG SOURCE_SHA`:
  published source, assets, pin, and hosted-check verification under its existing
  approval and environment requirements.

there is no current behavioral test command. the former unit, integration,
provider-live, live, platform, product, second-phone, and full gates are removed.
tmux and phone operations still require explicit current-turn approval under
`AGENTS.md`. release integrity is independent of behavioral acceptance.

[the coverage gap](../issues/test-system-reset.md) tracks the follow-up.

retained checks have narrow owners:

- ornament outputs must match one regeneration; terminal assets and their patch
  must match `android/terminal.lock`. the lock owns dependency pins; validators
  check agreement with actual inputs rather than duplicate those pins in code.
- catalogue checks validate data and source inventory. icon behavior belongs to
  the renderer; no second icon implementation remains in the checker.
- release checks validate source/tag, signatures, package/platform/version,
  archive contents, checksums, published pins, and successful hosted verification.
  both host platforms are read from binary metadata; source/version are checked
  in both archive manifests and by executing the native binary's version command.
  the foreign binary's source/version are not independently observed.
  release-note prose and json whitespace are not artifact identity.
- ordinary release checks do not reproduce host binaries. run
  `scripts/check-release --public-assets --reproduce RELEASE_DIRECTORY TAG SOURCE_SHA`
  from the clean exact source checkout for an explicit byte-comparison audit with
  the original build environment. without it, declared identity and checksums do
  not independently prove source-to-binary equivalence. published-release uses
  the immutable release's checker, so older releases retain their original checks.
