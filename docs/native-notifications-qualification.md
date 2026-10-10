# native notification qualification

2026-10-09, `feat/native-notifications`; source work is uncommitted. native
boundaries are qualified only within the artifacts below. complete acceptance
and normal-fleet deployment remain pending; this record changes no requirement in the [spec](native-notifications.md).

useful qualification states the current result, tested source/artifact, concrete
observation and remaining boundary. immutable private receipts and complete input
manifests remain under `/tmp/skid-notification-qualification/` and
`/tmp/skid-notification-deployment-tests/`; hash prefixes below identify those
records. evidence contains no credentials, account data or terminal/provider text.

## artifact scope

| evidence | exact recorded scope |
| --- | --- |
| mac physical results | signed `0.13.11`, archive `c73c8e0d`, build receipt `332d7704`, sealed 128-input manifest. this is older source, before later counter, native-probe and visit-field cleanup; it does not qualify a current-tree mac artifact |
| current mac packaging/startup | signed `0.13.12`, archive `1f267375`, build `a5170cf6`; 124 selected repository inputs match before, after and current source. signature, persistent leaf, entitlements, bundle modes and both binary versions pass. private startup `5d079ccc` admits exact app/socket/public read and normal stop; short healthy and 150-second healthy/offline resource captures pass their bounded guards. presentation and clean release remain unqualified |
| most recent android focus, reset, dnd, expiry and independence results | qa main `5122165c`, restored test `0dc1c646`; earlier metadata/enrollment probes retain their separately recorded builds |
| later android ready-generation repair | main `ac2229c2`, controller source `2c97d7a6`, identical probe apk `8fb6b0ee`/source `755e0f60` and driver `fce44ab0`; exact 93-input green build `c4b4d14a` |
| ordinary phone tailnet recovery | main `ac2229c2`, restored test `0dc1c646`, controller `2c97d7a6`, driver `112e31a7`; build `c4b4d14a` joined to the admitted ready-generation green |
| transport | stock ntfy server `2.28.0`, android ntfy `1.25.2`, official connector `3.3.5`; authored live fixture `ad2066fa` plus separate installed-provider smoke |
| deployment | three private qa tailnet addresses, distinct android qa uid and six owned services/five qa routes. userspace nodes share host loopback; this is endpoint/application isolation, not host-network isolation |

source/component checks below belong to their recorded source, independently of
physical artifact results. dirty-source builds do not establish clean release
provenance. production `/v1` remains on port 7341; its original routes, pairings
and application are protected. the earlier production-route diversion was
restored; later tests use the separate qa endpoints. inspected quarantine state
and the disabled old mac login job remain protected. six qa services still need
final disposition; permanent installation is unqualified.

## observed boundaries

| boundary | result and concrete limit |
| --- | --- |
| observer and contracts | pass: authored codex/claude frames through isolated real tmux, stock gateways, five-second observer lanes, authenticated snapshots and captured local acknowledgement; schema, gap/restart, ordering, cas and canonical http error boundaries have integration evidence |
| encrypted private delivery | pass: normative rfc8291 vector, independent http-ece decoder and authenticated stock ntfy binary forwarding; private publisher/reader ACL and controlled ingress checks pass |
| installed providers | `cded83a4` passes real codex idle and claude setup, exact foreground/reference and matching observer cause, then retires both sessions/observer lifetimes. this is startup smoke, separate from os presentation |
| android native content/clicks | recorded builds pass closed reason phrases, composites, menu+notice, ready, current session title, unicode/control handling and silent rename. codex/claude native clicks attach only the captured lifetime; working removes the notice. no audible-phone claim follows |
| android permission/enrollment | native red/green repairs submission settlement and durable registration ordering. denied permission leaves unseen attention; ordinary recovery posts the same current episode. ordinary cold process-loss/retained-endpoint recovery uses actual unfinished worker/callback receipts while the observer is down and skid stays closed |
| android foreground | exact presented-session suppression/departure and denied-foreground recovery pass. dashboard `e6700c75`, another exact session `96ca4c85`, and same resumed session with its main window unfocused `6c9d2e5e` each alert for a fresh question; all 24 cleanup checks and seven preference files pass on `5122/0dc` |
| android reset | `bdbe968d` passes outage reset with one public retry and original-episode readmission. joined `5582a072` includes genuine public epoch-mismatch reset/new enrollment and `88193df9` original-epoch restoration with working/native zero and protected cleanup |
| android dnd | `90d9ade9` proves system interception of one fresh normal intent, with skid neither resumed nor focused before delivery. the subsequent read-only API inspection proves relevant raw flags clear and unchanged native tuple/post time; all 26 cleanup checks pass. visual suppression, audible suppression and uninterrupted process closure remain `NOT_RUN` |
| expired-cache catchup | `d49a67ab` passes missed-resolved silence, one current alert, unchanged-hint silence and empty missed-clear removal after actual stock pruning of both populated caches under their same process/file. original 12-hour cache/config, route/auth and phone/production values are restored; no database edit or synthetic hint drives recovery |
| independent devices | `516bdef4` posts the same fresh ready on both devices. actual mac output acknowledges its captured generation and clears two running mac dashboards; the phone's whole owner record/native post stays unchanged. all 33 cleanup checks pass; scope is signed mac `0.13.11` and android `5122/0dc` |
| installation/rollback | native mac symlink admission is red, real signed-bundle registration green; two-build consent continuity and managed login/private upgrades pass. phone rollback restores prior apk bytes across recorded same-version and 13000-to-12002 upgrades, with identical sealed pairings and three real keystore-decrypted credentials. full coordinated rollback is unqualified |
| ordinary tailnet recovery | `c2a95ec4` passes actual phone vpn off/on: a resolved missed request stays unclaimed and absent, while unseen current attention posts with the exact owner/native tuple and copy. five samples retain the same post time for at least eight seconds; known resolution removes it. no skid launch or instrumentation; all 27 cleanup checks and seven preference comparisons pass. scope: awake, unlocked, background delivery |

ntfy `1.25.2` keeps an existing token's endpoint fixed. the accepted enrollment
scope is retained-endpoint/lost-response recovery and stale-submission fencing;
autonomous endpoint rotation and distributor data loss are outside this pin's
qualified lifecycle. successful setup is not blanket delivery acceptance.

the preceding `706be951` remains `NOT_RUN`: its single-apk inspector rejects
tailscale's base plus four splits before any UI/vpn/frame mutation. the repaired
test fingerprints every installed apk before and after; no product change or
weaker cleanup guard was needed.

the expiry run uses a real 20-second cache and five-second manager interval. it
qualifies the expired-cache/missing-cursor mechanism, not a twelve-hour physical
outage. attribution to the canonical minute hint is source inference. overnight
lock/doze and actual sound remain unverified.

forced-doze baseline attempts `82c073d1` and `e640e38a` remain `NOT_RUN`, before
power overrides or checkpoints. reader repairs distinguish cached keyguard
fields from the live monitor and use android's all-display focused-activity
summary. 46 keyguard and 31 activity checks pass independent review; strict
lock/noninteractive proof and all 27 protections remain unchanged.

attempt `920d376b`, receipt `56046840`, stops at force-entry verification before
publishing a question. the exact failed guard is unknown. normal power is
positively restored; after manual unlock, cleanup `b6738b22` passes all 27
protections with no pending cleanup. delivery remains `NOT_RUN`. bounded
transition repair `ea4e80a7` passes 31 pure checks and independent review: only
valid parsed transitions wait, final proof remains strict, and restoration keeps
its reserved deadline. retry `48d20a7c` / `740951d3` stops at the force-idle
command before any idle read or fresh question. the command-phase duration is
192 ms; its exit result and remaining budget are unavailable, so rejection and
deadline exhaustion cannot yet be distinguished. restoration passes after three
samples; cleanup `5a2edca9` passes all 27 protections with no pending cleanup.
command-diagnostic repair `a43208eb` passes independent source review. retry
`f9d1646a` / `8018ef92` establishes actual rejection: return code 255,
`deep-stopped/inactive`, 82 ms execution, 83.42 s remaining and no timeout.
there is no idle sample or fresh question. restoration and cleanup `a125a029`
pass all 27 protections. a later read-only snapshot shows a wake in 1,271,687 ms
inside the 3,600,000 ms idle guard, consistent with the primary-source explanation
but not proof of the earlier value. the [idle eligibility recheck](issues/native-notifications-doze.md)
later still shows a wake in 685,697 ms inside the same guard. no alarm or
power-setting change was made; forced-doze work stops as the accepted follow-up.
source admission is not delivery acceptance; no cleanup is pending.

## mac native results

current signed `0.13.12` private startup `5d079ccc` passes stock installation,
exact singleton/kernel app identity, finished launch, zero on-screen windows,
its mode-600 socket and available public read with zero eligible attention.
normal stop takes 2.227 seconds; its job is absent/disabled and original
production/quarantine and stock sources match. no watchdog fires or cleanup
remains. this does not establish permission, notices or clicks.

source comparison `332d7704` -> `a5170cf6` finds nine changed inputs: diagnostic
and redundant-field removal, equivalent validation reuse, symbol/comment cleanup
and the separately checked ready-token ceiling. post, click, cancellation,
sound and surface-association behaviour remain unchanged. the earlier physical
witnesses remain applicable; a full human sweep adds no identified coverage.
one current-artifact notice/click still needs actual os permission, delivery and
response-routing evidence for this installation.

current short resource receipt `bf03a109` records eight samples over 35.006
seconds after five seconds of settling: 0.0076925 seconds cpu, mean 0.022% of
one core, peak interval 0.142%, sampled rss maximum 47.125 mib and physical
footprint maximum 16.14 mib. it reports two package idle wakeups, 262 interrupt
wakeups and 4096 disk bytes each read/written. the exact helper is inactive
with no on-screen windows at both endpoints; availability is bounded by those
reads. normal lifetime is 40.254 seconds, below 45 seconds; all 12 cleanup
checks pass without watchdog, forced kill or pending cleanup. these process
counters exclude system watts and establish neither sustained behaviour nor
historical battery attribution.

approved 150-second captures `0d939ae4` healthy and `5daf57da` offline each
record 31 samples and all 12 cleanup checks passing. mean cpu is 0.013% and
0.016% of one core; maximum sampled rss is 47.72 and 47.55 mib. lifetimes are
155.219 and 155.261 seconds, below the 165-second caps, with no watchdog or
forced kill. joined series `fa074378` proves restoration of the internal qa
observer and equality of six service pids, routes, session identities, epoch
and registration; no cleanup is pending. setup `42d6f480` remains `NOT_RUN`:
its final app/window admission failed after 31 samples, and no final identity
was recorded. the user reports switching away or closing the window, violating
the foreground condition. the series remains `NOT_RUN`; the user deferred a
setup-only retry until evening. this is optional energy diagnostics, not a
spec acceptance requirement. finite process counters do not prove watts,
overnight cost or historical attribution.

all following physical results use the older signed `0.13.11` scope above.
`39580a3c` proves one actual claude notification response, helper-created focused
window/surface, exact cli/nonce/socket association and same-cli reconnection
after helper restart. a fresh focused question is handled closed with completed
native absence; ordinary viewport sizing passes. the user heard the default
sound and saw the expected session without setup.

`b87b7f01` proves focused presented-visit suppression, then fresh alerts for a
background window, tab, split and another exact session. all seven lifetimes
prove normal native clear/stop, remain within the 45-second cap and use no stop
fallback; all 14 final cleanup checks pass. stable tab/surface identities account
for public ghostty regrouping. exact title/reason metadata is checked; native
category inspection, os focus-policy and physical sleep are unqualified.

manual-window `78772c11` acknowledges captured ready output, then alerts for a
fresh question despite being foreground because it has no helper association;
all 20 cleanup checks pass. passive controller `600a556e` retains one unchanged
permission notice for 8.785 seconds and nine completed advancing inspections,
then proves working removal. this finite observation does not establish
indefinite retention or explain the historical disappearance.

public-reset acceptance `274d950c` joins immutable live effects `9ea1afce` with
the user's one-click confirmation: outage reset removes the native notice and
clears memory, recovery admits the original episode, and all 14 cleanup checks
pass. the original receipt remains `NOT_RUN` for absent attestation at write
time. os/disk atomicity and private setup-window singleton identity are unclaimed.

## later android ready generation

actual output acknowledges g1. the same connected visit remains captured while
its owned session sheet takes main-window focus; a fresh working-to-idle g2
remains unacknowledged and alerts. the sole owner says ready, but the old
controller's visit-key mask hides it from the queue.

red `643a9bf0`, receipt
`b31dc6ec9afb74168d88bd45ee9ce18ebc121a9c9e7389f03a8585c8c19e80dc`,
reproduces owner-ready/controller-not-ready on main `5122`, controller `4102`.
removing only the redundant visit-key conjunct produces controller
`2c97d7a6266c6707d02170e84a56a105b520ef19dd396140a0ab4da3976c7129`
and main `ac2229c2`; all other controller bytes/callbacks remain unchanged.

green `7b63c901`, receipt
`364b3eb0418ead83db2f24ddc435f3fc5c3d5f2f8aeb22edd9339d4cb51c5082`,
passes the identical `8fb6b0ee` test apk and all 17 captured prerequisites with
owner/controller ready true, normal runner completion and all 25 cleanup checks.
both red and green preserve exact target/foreground, captured visit, fresh
owner/direct inventory, native tuple and protected values. wider coherent
before/after diagnostics also find all seven preference files unchanged in raw
bytes, canonical entries and key sets; that interval exceeds the internal guard.
current sdk sqlite identity is not measured.

`scripts/check verify all` passes shared checks, go module verification/vet/build
and android debug assembly/lint after retirement. 34 temporary go tests, seven
android probes, three support files and their runner/test dependencies are removed.
the authored detector replay/corpus is unchanged. temporary source copies and
compiled qa artifacts are now retired; recorded historical results retain their
original source limits and do not qualify a current release.

## defect sensitivity and historical limits

meaningful temporary red/green also establishes:

- actual http/private-file partial-ready and delayed-read regressions: gaps hide
  qualification without losing tokens; an older read cannot escape a newer
  admission. rebuilt stock gateway/observer passes the same live partial frames.
- exhausted-counter rejection before persistence/publication across go/kotlin;
  actual socket registration/presentation also rejects exhaustion. this later
  source fix was not rebuilt into the physical mac artifact; valid-counter
  evidence above keeps its original scope.
- real tty/tls/websocket terminal admission remains usable during a slow owner
  read, and detachment retires its visit before a held secondary refresh.
- native add completion before copy visibility reproduces premature cancellation
  with a controlled external os service; full matching metadata now settles
  posted state. captured reset cancellation intent and obsolete callback fences
  pass affected owner/native composition checks. these do not identify an old os
  schedule or replace physical observations.
- real owner/socket/tls hint-then-EOF tests reproduce repeated one-second retries;
  the stability repair backs off one then two seconds, with 30-second reset and
  cancellation controls. this is a retry defect, not historical energy attribution.

`791b42cd` and `48ed70ad` retain `NOT_RUN` cleanup because raw preference
hash-list equality failed; their before lists were lost in ram. later passes do not identify either
writer. [sdk memory qualification](issues/native-notifications-sdk-memory.md)
remains open. do not restore unknown bytes or treat legacy preference captures
as coherent current sdk database identity.

historical mac heat/battery attribution remains open in
[energy qualification](issues/native-notifications-mac-energy.md), a separate
forensic follow-up. finite native profiles do not establish overnight cost;
setup-foreground measurement is unavailable, and the missing historical
executable blocks exact old-image reproduction. earlier unavailable
click/reset/foreground receipts stay `NOT_RUN`; later passes do not explain the
neutral-window launch warning or historical notice disappearance. the earlier
mac watchdog-only shutdown stays unavailable; separate restoration `0220b40a`
proves native zero/normal stop/protected cleanup without promoting that run.

## installed release baseline

read-only stock metadata identifies macbook and devbox current generations as
`v0.13.0`, source `b2ea62aea57a87668835eefbcaebcffcf5761559`. measured binaries are
`4080b6fc` and `92c0a590`; current runtime receipts `62787422` and `bfe9f21c`
match their recorded deployment pairs. these are installed facts, separate from
repository pins and dirty qa artifacts.

both older `previous` links select other `v0.13.0` runtime generations without
matching previous-pair receipts; they are not admitted rollback targets.
capture the current admitted baseline before candidate cutover. arch's installed
baseline and actual coordinated rollback remain unverified. metadata inspection
is preparation, not a rollback pass.

## remaining acceptance

[implementation and qualification](issues/native-notifications.md) remains open.
the user approved the [lean completion amendment](native-notifications.md#implementation-and-acceptance)
on 2026-10-09. retain qualified unchanged behaviour; do not repeat broad sweeps.

2026-10-09 source integration preserves main's workspace recovery, profile usage
and directory/group entry. independent go/android/deployment review finds no
blocker; full host/android engineering verification and deployment bash,
warning-level shellcheck and ansible syntax checks pass. integration is source
qualification, not a new physical witness.

cleanup removes six qa services, seven remote stage directories, their isolated
tmux servers, both phone qa packages and three private mac app homes/jobs.
production routes/pairings, the original phone packages, audio/dnd/power and
signing search list are unchanged. temporary harnesses and compiled artifacts
are retired; the surviving stopped historical bundle is preserved separately.
phone distributor cleanup removed five orphaned test subscriptions and the exact
qa saved user; the other saved users and original skid apk are unchanged.
authenticated admin search confirms all three retired qa node names are already
absent; no other devices changed. private phone cleanup context is deleted;
content-free receipts remain. no new delivery test is required.

- one actual mac-asleep/locked-phone delivery smoke check remains required;
  the user previously deferred its physical sleep step.
- forced-deep-idle delivery remains `NOT_RUN`, an accepted
  [follow-up](issues/native-notifications-doze.md), not a completion gate. the
  agreed alarm recheck is read-only; alarms and settings remain unchanged.
- audible phone sound remains `NOT_RUN`, an accepted
  [follow-up](issues/native-notifications-phone-sound.md); audio stays muted.
  channel high, default sound/vibration and no bypass are observed settings,
  not audible proof. existing policy/recovery witnesses retain their limits.
- exact current coordinated signed release and complete
  previous-release rollback are `NOT_RUN`; fold the current mac bundle's
  permission/delivery/click into this final verification.

engineering success qualifies neither unperformed devices nor release behaviour.
release/deployment require separate authorization; no complete acceptance claim
follows from this record.

permanent fleet deployment remains pending separate authorization; it is distinct
from the retained release/rollback verification.
