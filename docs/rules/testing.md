# testing status

2026-09-17: the user retired all behavioral tests and their harnesses. this
policy supersedes the former test tiers, mandatory red/green workflow,
and test-gate prescriptions throughout the repository.

the subsequent cleanup uses temporary tests at the affected integration/live
boundary. run them against the original and changed implementation; a bug fix
must demonstrate the intended failure first. review the implementation and the
test's sensitivity, then delete the temporary test before committing. this is
change-specific evidence, with no retained automatic regression protection.
do not rebuild the retired harness or add production seams for these tests.
unreachable-code and documentation deletions use caller/build/link checks when
a behavioral test would exercise no changed behavior.

product requirements remain. a successful build or static check does not prove
product behavior. historical test results apply only to their recorded source;
removed, unavailable, or unexecuted checks cannot be reported as passes.

current commands:

- `scripts/check static`: formatting, syntax, shell/go/android lint, dependency
  integrity, and catalogue/generated-asset checks.
- `scripts/build`: go and android debug builds.
- `scripts/check verify`: static checks and builds; also the hosted ci command.
- `scripts/release TAG`: build and verify signed artifacts once, then create a
  draft release. signing uses the private key; publication remains a separate action.
- `scripts/check-release [--source SOURCE_DIRECTORY] RELEASE_DIRECTORY TAG SOURCE_SHA`:
  verify existing artifacts with the public certificate and clean exact source.
- `scripts/check published-release TAG SOURCE_SHA`: read-only published source,
  assets, pin, and hosted-check verification; no approval flag or environment token.

there is no current behavioral test command. the former unit, integration,
provider-live, live, platform, product, second-phone, and full gates are removed.
tmux and phone operations still require explicit current-turn approval under
`AGENTS.md`. release integrity is independent of behavioral acceptance.

[the coverage gap](../issues/test-system-reset.md) remains explicit; temporary
tests do not close it.

retained checks have narrow owners:

- `scripts/gen-ornament --check` compares one regeneration without writing files;
  terminal assets and their patch must match `android/terminal.lock`.
  the lock owns dependency pins; validators
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
  `scripts/check-release --reproduce RELEASE_DIRECTORY TAG SOURCE_SHA`
  from the clean exact source checkout for an explicit byte-comparison audit with
  the original build environment. without it, declared identity and checksums do
  not independently prove source-to-binary equivalence. published-release uses
  the current checker with the immutable release's exact source, catalogue, and
  certificate asset. its source certificate must also match the current signer pin.
