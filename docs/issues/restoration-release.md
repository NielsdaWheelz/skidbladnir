# restoration release remains gated

2026-09-25: the github name transfer is complete. original repository id
`1386409483` is `NielsdaWheelz/skidbladnir`; the other product, id `1342599607`,
is `NielsdaWheelz/herdr-mobile`. the canonical origin and numeric identity are
verified, and `scripts/check-release-repository immutable` passes. the former
github-name and disabled-immutable-release blockers are resolved.

the former `release-pin.json` selected `v0.6.0` at
`2d6184c63d62396f69342200e4229cc902ca140c`; those assets belong to the
other repository. no original-product release or tags exist. a new pin cannot
be written until original-product artifacts are published. the dev-server pin
and rollback generations must likewise exclude those other-product assets.

the phone reportedly runs a herdr-backed `0.8.0`/`8000` under
`dev.niels.skidbladnir`. the pinned public signing certificate is retained.
the installed version and signer need a fresh device observation before
selecting `v0.9.0`/`9000`; no device gate was run in this source-preparation turn.
the local signing configuration passes `scripts/check-android-signing
--config-only` against the retained public certificate; this does not observe
the phone. `scripts/check-release --android-version v0.9.0` yields `0.9.0 9000`.

dev-server's ten-file contract acknowledgment and admission correction
introduced in `94931a13c045fa9c9b294080149524501ff30b68` remain complete in
pushed source `ae70f2b78ad9b02a7dbccfd79d53be5bcfd11b50`;
[the handoff](../dev-server-handoff.md) records the bounded rollback evidence.
the generation receipt contract is unchanged. that source also implements and
acknowledges existing skid accounts, no codex hook installation, scoped claude
identity and skid-owned shell setup. both source discrepancy records are closed;
actual shared-account behavior remains live qualification.

the provider continuity and hook correction, including its shell contract,
passed [hosted verification](https://github.com/NielsdaWheelz/skidbladnir/actions/runs/36212232289)
at `0bb7e2a521aade3dadec5795fc5c4a726b566c5c`. subsequent source changes require
their own exact-source hosted check; use the preflight below for the final
reviewed commit rather than assuming an earlier pass applies.

resolution requires a successful `verify.yml` push check at the exact reviewed
release source sha and a fresh installed phone version/signer observation.
`scripts/check-release-source v0.9.0`
checks clean main against `origin/main`, tag availability, repository identity,
immutable releases, and the exact successful hosted run; its output is the
release source sha and run id. the operator then creates and reviews a signed
original-product release with an increasing android code. publication may
proceed before live host namespace handback.
publish it, replace the five-asset upstream pin and the dev-server host pin with
that release's exact source and digests, and run the published-release check.
the operator then installs and pairs herdr-mobile, clears only obsolete
skidbladnir phone data, installs the new skidbladnir apk, and pairs afresh.
live three-host and phone acceptance remains open under the current-turn
tmux/device approval rules.
