# claude usage selection

2026-10-20: implemented; [isolated qualification](#qualification) passes;
[v0.18.0 installed qualification](v0.18.0-installed-qualification.md) records rollout.
architecture, quota content and adversarial acceptance reviews approved.
this document owns claude disagreement handling, amending [terminal usage](terminal-usage.md)
and [desktop usage](desktop-usage.md). their acquisition, presentation and historical
qualification retain their owners. [architecture](architecture.md), [rules](rules.md)
and [testing policy](rules.md#testing) govern implementation.

## goal and scope

show one usable approximate quota observation despite differing claude displays.
usage is read-only evidence; it never authorizes execution, controls or admission.
the owner guarantees that a configured profile denotes the same account across hosts.

the desktop selector owns this correction. remove its disagreement veto and usage
`conflict` state completely. no compatibility branch, fallback, flag or migration.
unrelated terminal `evidence_conflict`, duplicate-footer rejection and failed-capture
admission remain intact: ambiguous screen evidence is different from differing samples.

the [persistent installed symptom](issues/claude-usage-persistent-conflict.md) and
[stale help text](issues/claude-usage-help.md) are separate issues. this work does not
diagnose installed state, change help/columns/keys/layout or acquisition, add timestamps,
retain claude samples after failure, add services/pollers/caches/dependencies, discover
accounts, change codex, add phone quotas, or release/deploy anything.

## selection and lifetime contract

1. use current admitted host inventories, independent of table/group filtering.
   require the host's matching configured claude profile, successful inventory,
   current registered `agent.provider == "Claude"` and matching `agent.profile`,
   no remote connection, and a present validated `terminalUsage`.
2. within each host/profile, select one actual session-row snapshot: most present
   windows first, then ascending numeric tmux session id, then numeric active pane id.
   reuse `usageTerminalLess`; disagreement never affects eligibility or ranking.
   zero is a present value. never combine windows from different observations.
3. across hosts, select the candidate with greatest original client-local
   `Peer.ReceivedAt`; equal receipts choose the lexically smaller machine handle.
   reuse `usageLater`. never compare remote clocks or retimestamp retained data.
4. replace the whole selected observation, including omissions. a newer host's
   partial observation replaces an older complete one. source disappearance may
   reveal another eligible host; that is ordinary selection, not a legacy path.
5. selection clears `usageRow.state`, suppressing other-host failures. without a
   candidate, retain existing precedence: `unavailable`, successful absence (blank
   state and dashes), then `checking`. preserve profile-declaration diagnostics.
6. recompute from inventory; add no retained selector state. pending reads retain
   the previous admitted sample and receipt. successful omission, failure, rejected
   access/identity, closure or process/profile replacement withdraw eligibility
   under existing admission rules. a successful refresh with one remaining eligible
   observation displays that observation without a special recovery action.

preserve capture-age staleness at `max(0, now-receivedAt) >= 1h` and existing native
usage clocks. a new successful capture renews capture freshness even if percentages
are unchanged; it does not establish provider freshness.

## architecture and interfaces

```text
existing gateway capture/parser -> strict session transport -> admitted inventory
  -> claudeUsage -> usageRows -> windowPresentation -> footer and usage page
existing codex native lane ----------------------------------> usageRows
```

reuse these owners; add no selector service, generic candidate abstraction or
parallel rendering path. keep `claudeUsage(profile, now) usageRow` and the current
private records. the allowed private states become:

```text
usageRow.state = "" | "checking" | "unavailable"
terminalUsage = {fiveHourUsedPercent?: integer 0..100,
                sevenDayUsedPercent?: integer 0..100}  // at least one present
```

`terminalUsage` is the EXISTING schema, not a new declaration or decoder. pointers
preserve absence and zero. `windowText` and all public api/config/wire/persistence
shapes stay unchanged. `GET /v1/sessions` carries optional claude evidence;
`GET /v1/profile-usage` remains codex-only. preserve current refresh owners,
generation/cancellation fences, receipt assignment, schema validation and lane isolation.

## content contract

one quota content designer authors expected output independently of implementation
and reviews both surfaces. good content states remaining capacity, preserves missing
values and exceptional states, and never implies newest-provider data. retain plain
`100-used` percentages, existing columns/keys and reset dashes; add no marker,
warning, source text, disagreement explanation or new layout.

authored case: same host/profile, session/pane `$9/%7` reports used `{5h:20,7d:43}`;
`$10/%8` reports `{5h:23,7d:46}`. both have two windows; numeric session 9 wins.
footer is exactly `claude-work 7d 57%`. the page has weekly `57%`, a blank-profile
`5h` continuation of `80%`, blank status and reset/countdown `—` for both. reversing
input order changes nothing. if session 9 becomes weekly-only, session 10 wins:
weekly `54%`, five-hour `77%`. neither surface displays usage `conflict`.

a selected five-hour-only observation leaves weekly `—`; do not borrow a weekly
value. render real `0%`/`100%`. preserve `checking`, `unavailable`, and `stale`
according to their existing contracts; missing windows never inherit `stale`.
colour and `NO_COLOR` carry identical facts at 80×24.

## exclusive work boundaries

| owner | exclusive files and responsibility |
| --- | --- |
| selector implementer | `internal/sessionui/usage.go`: remove disagreement accumulators, entire-host veto and usage conflict branch; retain ranking/projection helpers |
| acceptance author | temporary `internal/sessionui/claude_usage_selection_integration_test.go`, `claude_usage_selection_live_test.go`, and one owned scratch directory; probes and their deletion |
| root integrator | this file, `docs/{terminal-usage,desktop-usage,roadmap}.md`, and the two linked issue records; contracts, integration, qualification and cleanup |
| quota content designer | authors literal fixtures/oracles in review messages; reviews content; no repository writes |
| independent adversarial verifier | reviews contract, baseline sensitivity, implementation, results and final diff; writes no test or production file |

freeze this contract before parallel probe/implementation work. use an isolated
task worktree; preserve unrelated changes. no other production file is in scope.

## red green refactor and acceptance

1. review the contract and designer's oracles adversarially before coding.
   freeze a fresh baseline source identity; incorporate concurrent changes before
   probing rather than assuming earlier inventory-message shapes.
2. author disposable integration probes through real fleetclient decoding,
   current inventory admission and shared footer/page rendering. control actual
   receipt order with authored peer replies; deterministic tie cases may set existing non-wire
   `Peer.ReceivedAt` on decoded observations before normal `acceptInventory`
   admission. add no clock seam; do not replace the selector or inject rows
   directly into the renderer. baseline red requires two admitted valid differing
   contributors and missing expected numbers because of `conflict`; setup,
   compilation, parsing or transport failures are not behavioral red.
3. after explicit current-turn tmux approval, run one native desktop journey:
   production `sessionui.Run`, `(*gateway.Gateway).ServeHTTP`, `fleetclient.Open`,
   browser/pty and isolated `tmux -L` socket, with disposable locally
   recognized/registered provider fixture processes displaying authored
   quota footers. exercise capture -> transport -> selection -> both surfaces;
   baseline must fail the target behavior. use a compiled fixture image matching
   its scratch canonical `executableDirectory`; consume real foreground identity
   via `process.ObserveForeground` and encode its exact process-lifetime facts
   with `agentruntime.EncodeRegistration`.
   install registration only on owned panes through the explicit isolated socket.
   do not invoke the hook cli: its tmux path alone does not select that socket.
   all homes, checkpoint/config paths and credentials are scratch-local; configure
   a harmless local helper returning unavailable so codex reads cannot reach account
   daemons. assert actual rendered grid cells, not raw ansi substrings. prove both
   contributors reached admission before the baseline assertion.
   add no production test seam, fixture mode, real account access, inference turn
   or provider getter.
4. implement only the selector deletion; rerun the same integration and native
   probes green, including known selected numbers on both surfaces, repaint without
   a keypress and ordinary automatic refresh, one surviving contributor, then
   omission of the last contributor. assert the browser sends reads only.
   refactor only to remove now-dead disagreement logic and related state. keep still-used selectors
   and shared projection. independent review challenges each red/green/refactor
   boundary; fix findings and rerun affected probes before proceeding.
5. record source identity, baseline failure, passing cases, exercised boundaries,
   review findings and limitations here; retain content-free receipts only.
   delete temporary probes, fixtures, binaries and owned runtime/scratch resources.
   track exact socket/session/pane/process ownership; close only those sessions,
   reap children and never use the default socket or `kill-server`.
   review cleanup/final diff; run `scripts/check verify host` after test deletion.
   engineering checks cannot substitute for behavioral acceptance.

| acceptance group | required observable result |
| --- | --- |
| disagreement/ranking | authored example on both surfaces; input-order independence; completeness before numeric session/pane ties; other-host disagreement/failure cannot veto selected usage |
| host selection | ordered/equal receipts select the specified whole sample; skewed remote clocks cannot alter selection; newer partial replaces older complete |
| missingness/content | complementary panes never splice; five-hour-only leaves weekly unknown; zero/100 preserved; footer/page agree; no conflict copy; unchanged geometry and keys |
| lifetime/failure | one-survivor success updates; all-source omission/failure removes quota values while retaining declared profile rows; profile/process replacement and access rejection cannot retain attribution; pending samples keep original age; inclusive one-hour stale threshold |
| composition | existing codex report and independent native/inventory failure handling retain their behavior; no extra read, control request or new refresh owner |

live acceptance proves the selector with real local plumbing, registration
consumption and authored displays, not hook publication, stock-provider updates
or actual claude data freshness. one native desktop platform suffices for this
platform-independent selector change. actual providers, installed fleet, phone and
deployment remain `NOT_RUN`. missing tmux approval leaves live acceptance `NOT_RUN`,
never a pass; this plan is not approval to operate tmux or the phone.

## accepted tradeoffs and final state

useful availability wins over equality of independently refreshed readings.
deterministic choice avoids sampling-order flicker but can select older pixels
indefinitely; completeness can prefer an older two-window observation. receipt
order measures local response arrival, not provider freshness. whole observations
can discard useful older windows. capture timestamps would order old pixels;
per-window maxima could preserve pre-reset readings. neither is added.

final source has one selector path, no usage-conflict machinery, unchanged public
schemas and acquisition, and no retained temporary tests or harness. probe deletion
leaves no automatic regression protection. the two separate issue records remain
until their own resolution criteria are met; do not claim the installed symptom
fixed from this change or isolated acceptance.

## qualification

baseline: `7b8aa2be16557a15e3b0e1b22251f372a5977080`. qualified production change:
only `internal/sessionui/usage.go`, git blob `f5db34429595141cc2b6376705a5707301da6077`.
the reviewed deletion preserves eligibility, ranking, clocks, projections and schemas.
no further refactor was justified.

- integration red: two exact valid differing observations admitted through real
  tls/fleetclient decoding and current per-host inventory admission; expected
  numbers were absent because of usage conflict. the five other groups passed.
  the same frozen probe passed all six groups after the deletion (0.06s).
  coverage: receipt order/ties/skew and whole partial replacement; missing/zero/100;
  withdrawal/failure; pending/stale threshold; native clocks/lane isolation.
  exact read counts and allowed methods/routes also passed.
- native red: two registered, recognized fixture processes supplied differing
  parsed windows through the production gateway and client; the actual browser
  grid showed conflict instead of the selected numbers (15.74s).
  the same frozen journey passed on darwin/arm64 (17.56s): colour/plain 80×24,
  footer/page values, `u`/back, repaint without keys, survivor then final absence.
  all 16 http requests used the two allowed read routes.
- architecture/content, probe sensitivity, baseline red, production green,
  refactor and retirement reviews passed. review corrected probe admission,
  exact assertions and cleanup; no production test seam or added dependency remains.
  raw input permutation is normalized by transport; the unchanged numeric pane
  comparator was source-reviewed, without inventing duplicate-session rows.
- both temporary test files and owned scratch fixtures/configs/credentials were
  deleted. exact owned sessions/processes exited; default socket and `kill-server`
  were unused. `scripts/check verify host` passed after retirement: formatting,
  syntax/lint, asset/dependency integrity, go vet and go build.

this qualifies authored local displays through real native plumbing and current
desktop admission. at this source qualification, installed fleet, phone and
deployment were `NOT_RUN`; [subsequent rollout](v0.18.0-installed-qualification.md)
owns installed evidence. actual claude updates/account freshness and the historical
installed symptom remain unverified. the two separate issues remain open;
there is no retained automatic regression coverage.
