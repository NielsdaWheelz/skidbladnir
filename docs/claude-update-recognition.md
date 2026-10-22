# claude recognition across native updates

2026-10-15: implemented, qualified and deployed in `v0.17.0`.
the confirmed macbook failure had a running image `2.1.296` while the launch
alias pointed to `2.1.300`. both images existed; native process observation and
registration still agreed on the old process lifetime. the installed `v0.16.0`
matcher rejected that image before projecting registration or sampling its screen.
this specification replaces only the mutable executable-path recognition rule.

## target and invariants

a supported native claude process remains recognized when an update moves the
launch alias. old and new images can be recognized simultaneously, including
after a cold gateway restart. an affected surviving process becomes recognizable
on the next inventory after deployment, without provider restart or input.

launch configuration selects the next executable; recognition configuration
identifies the installation containing a running executable. neither substitutes
for the other. existing foreground process-group leader selection, codex basename
recognition, registration, exact lifetime checks and controls remain unchanged.

non-goals: group enumeration, descendants, descriptor inspection, wrapper or
proxy recognition, deleted-image recovery, installation discovery, conversation
repair, status/usage/model grammar changes, client schemas or presentation.
there is no universal-launcher or version-independent-screen-parsing promise.
[wrapper discovery](issues/provider-wrapper-recognition.md) is separate work.

## structure and host schema

```text
deployment -> strict host config -> existing native foreground observation
  -> agentruntime image match -> unchanged registration and session projection
  -> unchanged terminal capture/classifiers -> unchanged clients
```

replace `ForegroundSignature.ExecutablePath` with `ExecutableDirectory` and json
`executablePath` with `executableDirectory`. retain the existing signature
collection, optional basename predicate, conjunctive field semantics, alternative
signatures and cross-provider ambiguity rules. no new abstraction or endpoint.

```json
{"foregroundSignatures":[{"executableBase":"codex"}]}
{"foregroundSignatures":[{"executableDirectory":"/canonical/claude/versions"}]}
```

- `hostconfig.Load` admits the directory: absolute, clean, nonempty, utf-8,
  nul-free, resolving to an existing directory. canonicalize once at admission
  and store that canonical path. keep `ValidateProfiles` pure; it validates shape,
  not filesystem state. preserve existing strict null/duplicate/unknown rejection.
- `matchesSignature` retains codex's basename comparison. the directory predicate
  compares `filepath.Dir(observation.Executable)` with the admitted directory.
  only immediate children match; no prefix, recursive or version-name matching.
  inside this directory predicate, reject the exact linux ` (deleted)` suffix
  BEFORE comparing directories. do not strip it, recover an image or alter the
  basename predicate/native process observation.
- no launcher resolution, installation scan or image-file stat during matching.
  runtime config admission does not depend on the launch alias's current target.
- deployment independently renders `@CLAUDE_VERSIONS_DIRECTORY@` from
  `<home>/.local/share/claude/versions`; `@CLAUDE@` remains the launch command.
  never derive recognition from `dirname(command)`
  or today's resolved launcher. before activating a generation, separately verify
  that its current claude launch target belongs directly to the admitted directory.
- directory relocation, including retargeting a directory alias, requires config
  repair/redeployment. it is outside the version-update continuity guarantee.

hard cut: remove the old field, decoder, validation and live `EvalSymlinks`
matcher; old configuration fails admission. update both deployment templates
and the producer together. no compatibility reader, migration or fallback.
public gateway/client/reference schemas and stored registration remain unchanged.

## content and preserved behavior

one content designer reviews existing output; no new content feature is needed.
good content preserves the distinction between provider presence, proven profile,
registered conversation and current screen evidence. reuse the existing labels,
unknown/unavailable states, explanations, freshness and attention policy.

recognition makes status/footer/usage capture eligible again; it does not invent
values or guarantee a supported display. desktop quota attribution still requires
a proven profile. missing registration remains missing. no reconnect button,
success toast, new badge, cached identity or changed native target.
[terminal observation](terminal-observation.md) and [terminal usage](terminal-usage.md)
retain those contracts and own the existing presentation.

## disjoint work and review

| owner | exclusive implementation paths |
| --- | --- |
| recognition | `internal/agentruntime/{profile.go,runtime.go}`: rename field, pure validation, directory predicate; remove old matching code |
| config/deployment | `internal/hostconfig/config.go`, `deployment/providers/host-config.json`; external `dev-server/assets/skidbladnir/host-config.json`, `dev-server/lib/skidbladnir.sh`: admission, canonical directory, matching templates/producer |
| temporary probe author | disposable `internal/recognitionprobe/` only: public production entrypoints and actual integration/live boundaries; no production seam |
| root integrator | this spec, `docs/{architecture,roadmap,terminal-observation,dev-server-handoff}.md`, issue/qualification evidence and coordinated repository/installer release pins |

freeze the field/admission contract before parallel implementation. independent
adversarial verifier and content designer write neither production nor tests.
review requirements first, baseline failure next, each implementation boundary,
composed behavior, then refactor and final evidence. root resolves disagreements;
reviewers cannot expand scope. no `process`, session, hook, control or client
production edits are assigned.

## acceptance and delivery

| boundary | required evidence |
| --- | --- |
| config | valid canonical/aliased directory admitted and stored canonically; missing/non-directory/malformed values and old `executablePath` rejected; unchanged codex config admitted |
| matching | direct image child matches; sibling-prefix and nested paths do not; old/new versions both match; preserved basename/conjunction/ambiguity semantics; `/versions/image (deleted)` never matches a `/versions` directory signature |
| actual darwin/linux pipeline | old native claude remains the foreground leader while launch alias moves; new leader image also recognized; first observation after update and gateway restart rediscover both without a warm cache |
| existing consumers | original pid/start and matching registration survive; ordinary inventory restores eligible status/footer/usage; missing metadata or unsupported screens stay honest; returning to a shell clears detection |
| preserved controls | existing captured native target does not change; existing lifetime revalidation/refusal remains effective; no stronger terminal-delivery guarantee is claimed |
| deployment | candidate binary validates candidate config; new launch target outside the configured directory rejects activation; existing provider/tmux processes survive gateway generation change |

1. with required current-turn runtime approval, write temporary probes.
   reproduce the original bug through real tmux/gateway/native process boundaries
   before editing production. baseline-compatible probes must fail behaviorally,
   not merely fail to compile or reject a new schema. reviewer checks sensitivity.
2. implement, make the same probes green, then refactor under [codebase rules](rules.md).
   use direct temporary probes for pure schema/matching cases and real stock
   claude on darwin/linux for runtime claims. no process mocks as live evidence.
   verify quota/footer via ordinary inventory, not status-only `info --explain`.
3. review code, output and evidence; delete temporary probes under
   [testing policy](rules.md#testing), then run `scripts/check verify host`.
   record source/platform/provider identities and case outcomes, without terminal
   bytes, prompts, conversation ids or account data. `NOT_RUN` is never a pass.
4. publish/activate the matching binary/config generation through the existing
   installer; validate with the candidate binary before the atomic generation
   switch. root owns this repository's `release-pin.json` and the coordinated
   external `dev-server/assets/skidbladnir/release-pin.json` update. rollback uses
   the prior complete generation, not a compatibility parser.
5. qualify installed continuity using a provider process kept alive across the
   cutover: the original affected process if still alive, otherwise a deliberately
   reproduced old-image process. record which; never claim recovery of an exited
   process. close the issue after source and installed-continuity evidence;
   no phone claim without device evidence.

accepted costs: the configured directory is trusted as claude's installation
namespace, not cryptographic publisher identity; layout relocation needs explicit
maintenance; old schemas hard-fail; wrappers/deleted images remain unsupported;
screen grammar drift remains independent; deleting probes leaves no retained
automatic regression protection. qualification below records executed boundaries.

## qualification

2026-10-15, change-specific probes through public production interfaces;
independent adversarial review covered requirements, test sensitivity, source,
composition, content and refactoring. no further refactor was warranted.

- baseline `b253c05f8374d4db625a83c8494e9f7317022d23`: the same recognition
  assertions failed on darwin arm64 and arch linux amd64 after relinking and
  after a fresh gateway process. the old image lost provider/capture eligibility
  and registration projection; the new image remained recognized.
- candidate: those assertions passed on both platforms. actual isolated tmux,
  native process observation and gateway http inventory were used. darwin used
  stock claude `2.1.296` and `2.1.300`; linux used two distinct image paths copied
  from stock `2.1.288`, qualifying path rotation, not a vendor-version upgrade.
  original pid/start and projected registration survived; a raw launch gained
  no unproven metadata; returning to the shell cleared provider metadata.
- all 27 admission/matching cases passed on both platforms: canonical admission,
  directory aliases and explicit reload, malformed/obsolete schema rejection,
  direct-child boundaries, deleted suffix, codex basename, conjunction,
  alternatives, ambiguity and filesystem-independent profile validation.
- capture eligibility passed. footer/usage value preservation was `NOT_RUN`:
  ordinary inventory exposed neither value before rotation. absence remained
  honest. fixture registration was seeded through the public codec; this proves
  projection/preservation, not hook publication.
- native target/control paths are unchanged by dependency review; projected
  conversation and process lifetime stayed stable. stale observed-foreground
  reads were refused after the owned provider exited. native dispatch and
  provider-specific terminal writes were `NOT_RUN` in this change.
- the actual installer renderer at baseline
  `c77b278865ad9bf520ca7ab40292541a8e5ceb8f` accepted an unrelated launch target;
  the candidate rejected it. valid rendered config passed candidate-binary
  admission. cloned assets and temporary output isolated this check from live
  activation; installed generation continuity is recorded below.

probes sent no prompts or inference requests and recorded no terminal content,
credentials or conversation identifiers. owned isolated sessions were cleaned
up. temporary probe sources were deleted after review; `scripts/check verify host`
passed, including after incorporating current main. installer shell syntax/lint
and template parity passed. no phone or independent quota-value accuracy
qualification is claimed.

### installed continuity

immutable [v0.17.0](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.17.0)
ships source `d22d46f40968d187863ac93ad0480bf08bfa461f`; its exact-source hosted
verification, signed-asset checks and `scripts/check published-release` passed.
the latter used android studio's bundled jdk after the first invocation lacked
a java runtime. matching pins were derived from the published checksum manifest.
installer source `fc8c7cac6290c5184267dba3ad50471ce2147a06` activated the candidate
binary/config through gateway-only apply on macbook, arch and devbox; existing
candidate-config admission and atomic generation switching were unchanged.

- the ORIGINAL affected macbook process survived: same pid, kernel start and
  old `2.1.296` image; launch alias still selected `2.1.300`. first post-activation
  inventory restored registered profile/conversation projection and footer/usage
  fields. the subsequent explicit provider check confirmed `Claude`. no provider
  input or restart was used; values and identifiers were not logged.
- all 12 macbook pane-root and foreground lifetimes present immediately before
  activation survived unchanged, including registration hashes. one unrelated
  pane had exited before that cutover snapshot and is not counted as preserved.
- both devbox pane-root and native codex foreground lifetimes, images and
  registration hashes survived unchanged. arch had zero existing sessions before
  and after activation; linux runtime recognition is qualified by the isolated
  native-provider checks above, not an invented installed-session witness.
- `scripts/fleet verify` passed all three installed gateway/desktop-client checks
  against the committed matching installer pin. jarvis remained paused and
  cleanly stopped through its managed cli replacement. no phone operation ran.

the confirmed update-recognition issue is resolved. wrapper discovery, deleted
images, screen grammar drift and the retired regression suite remain outside
this acceptance.
