# Unrun acceptance

These acceptance journeys still need observation. Versioned entries name the
shipped release and the former record, in parentheses. Its procedure and
evidence remain in git at
[docs/issues as of `12b1516`](https://github.com/NielsdaWheelz/skidbladnir/tree/12b15163d9faf529416d486e0156b8b83821fa4a/docs/issues),
or at [`ba60841`](https://github.com/NielsdaWheelz/skidbladnir/tree/ba60841d3eeefde94eda0aea623f81f4133e88e2/docs/issues)
where marked.

Impact: a regression on one of these paths ships unseen until daily use meets it.
Source checks, isolated journeys and installation do not substitute for them.

Terminal continuity and creation:

- v0.10.3: shell row on the installed fleet: initial-agent return to its shell, special-character cwd/argv, job control, input/resize, startup-child isolation, repeated sourcing (terminal-continuity-shell-qualification).
- v0.10.3: foreground observation on Linux and Darwin: every configured home, absent/empty/unlisted/inaccessible homes, read cap, background exclusion, stale-action rejection (terminal-continuity-observation-qualification).
- v0.10.3: directory search against real host zoxide: ranked results, literal paths, no match, missing/disabled tool, bounds, removed directory, host-switch races (terminal-continuity-directory-qualification).
- v0.10.3: remote lifecycle: simultaneous connections, nested hops, suspend/resume, reconnect, gateway restart, source loss, and the [remote adversarial cases](https://github.com/NielsdaWheelz/skidbladnir/blob/ba60841d3eeefde94eda0aea623f81f4133e88e2/docs/terminal-continuity.md#6-delivery-and-adversarial-acceptance) (terminal-continuity-remote-qualification).
- v0.10.3: phone forge initial-agent launch on the installed APK (terminal-continuity-phone-controls-qualification).
- v0.10.3: one of 12 unrelated sessions vanished during the physical-phone window; cause unattributed, so unrelated-session preservation is unproven (terminal-continuity-phone-qualification).
- v0.21.0: initial agent under Powerlevel10k instant prompt on an installed Zsh host with a warm cache: terminal stdin/stdout at launch, input and job control, same-shell survival, one-shot launch (initial-agent-instant-prompt, at `ba60841`).
- v0.14.0: workspace recovery after an actual Linux and Mac reboot, with changed boot identity and no client request (session-recovery-reboot).
- v0.20.0: clean supported-host installation, Ghostty and Apple Terminal adapters, installed owner visual parity (generalization-installed-cutover). Uninstall now requires a drain; its native qualification is tracked in [terminal service](terminal-service-qualification.md).
- v0.20.0: the owner confirms no blank Apple Terminal window from the 2026-10-20 adapter probes remains open; never close an unreceipted one automatically (generalization-terminal-probe-windows).

Desktop:

- v0.11.0: live `s`/`x` provider and tmux closure, and [a6](https://github.com/NielsdaWheelz/skidbladnir/blob/ba60841d3eeefde94eda0aea623f81f4133e88e2/docs/desktop-browser.md#7-acceptance-and-delivery) info editing (takeover/reset, naming conflicts, lost acknowledgements, replaced targets) through a real gateway (desktop-browser-runtime-acceptance).
- v0.18.0: installed production terminal entry/greeting on desktop and phone; needs an owner exception to the default-server guardrail (desktop-browser-runtime-acceptance).
- v0.14.0: `start --stdin` on the installed fleet: cold startup to the first prompt, and each supported startup-dialog layout refused without blind input (jarvis-launch-readiness, jarvis-launch-receipts, at `ba60841`).
- Usage band, owner look in Ghostty (`NOT_RUN`). Check the Fable underline's
  weight and its gap below the general line, whether the dotted part reads as
  dots, the faded underline, the header's underlined sample and faded rows.
  Watch `◕` and `●` cross above it and check how they sit and whether they
  pulse. Check the reveal at 7.5 cells a second across 24 cells, the stem's
  legibility and a light theme.
- Usage band, Codex sources (`NOT_RUN`, checked only by reading `internal/sessionui/usage.go`): a terminal estimate fades an hour after its receipt, and a profile whose native and terminal readings have both faded shows the native one. The release's temporary harness could not wait an hour.
- Browser fold owner pass on the installed fleet (`NOT_RUN`, waived by the owner
  for this release). The pass covers copy and
  appearance in dark and light Ghostty. Attach, detach, next and attach without
  waiting or entering an unchosen row. Judge whether the screen answers what
  the session is doing more often than not. Decide whether the port's third ink
  is wanted. Check legibility over Mosh and under `NO_COLOR`.
- Status signs, owner look in Ghostty (`NOT_RUN`). Check each sign in dark and
  light themes, including bold `?` and `!` beside `●`, and `▸`, `·` and `—`
  told apart. Check the double Frost line under `?`, `!`, `■` and `≡`, directly
  and through tmux and Mosh. Check the held and stale rows and their captions,
  the needs-input tab, the keys legend, and every sign under `NO_COLOR`.
- Status signs, installed entry points (`NOT_RUN`). Check the browser binary
  and `skid serve` against the installed loopback endpoint. Show the double
  work line on an error or interruption produced by the real gateway. The
  isolated observation corpus has no such frame.
- v0.11.0: pending ready restored after a failed host recovers (terminal-observation-stale-live).

Phone:

- v0.5.0: hands-on groups/shells workflow: membership set/change/clear, intersecting filters and return position, forge/session-sheet creation with cwd/group inheritance, duplicate taps, narrow and enlarged text (groups-shells-hands-on).
- v0.10.0: groups filter/edit/create/detach against a real host; Darwin isolated-host membership continuity (groups-cutover-acceptance).
- v0.10.7: naming composition: narrow large-text naming controls, ready persistence through rename/reset (reply-notifications-phone-composition).
- v0.2.30: readable terminal sizing: one real gateway, wss and isolated-tmux run on Linux and Darwin of a small first resize → phone-sized shared window → desktop input → desktop-sized window → phone input and resize, keeping pane, PID, draft and unrelated sessions; then a hands-on S22+ journey through real Codex and Claude text, menus, approvals, diffs, selection, Gboard and dictation, and desktop handback, comparing 14, 16 and 18 sp. The 16 sp default counts as accepted only after it ([readable sizing acceptance](https://github.com/NielsdaWheelz/skidbladnir/blob/ba60841d3eeefde94eda0aea623f81f4133e88e2/docs/terminal-readable-sizing.md#red--green--refactor-and-acceptance)).
- v0.13.0: terminal chrome on the S22+: one-row rail through connect/frozen/too-small and both sheets (terminal-chrome-hands-on).
- v0.15.0: on real sessions, open and cancel both closure confirmations, then a terminal visit/return keeping the dashboard anchor (session-card-hands-on).
- v0.11.0: concurrent work with a request, generic needs input, production-gateway delayed response (terminal-observation-phone-acceptance).
- Status signs on the S22+ (`NOT_RUN`). Check each sign in the Data face at
  default and largest text, both Frost strokes beneath a working claim, and
  the needs-input words and order. Check the strip's sign, ink and reserved
  cell and the session sheet's sign and words, including `· stale`. Check
  that STALE cards read `—`. With a pending ready from the hub, start a
  metadata write from the phone and verify that its controller keeps the
  card's ready sign and queue place during REFRESHING while withholding
  actions.

Terminal observation and attention:

- v0.11.0: Codex grammar RP/AC rows whose [§1 `qualified` column](../terminal-observation-codex.md#1-frozen-capability-table) omits a platform, including server/account-gated families: network approval, user verification, sub-agent view, remote images (codex-observation-darwin-coverage, codex-server-driven-families, terminal-observation-linux-coverage).
- v0.11.0: Claude grammar `R` rows whose [§1 `qualified` column](../terminal-observation-claude.md#1-frozen-capability-table) omits a platform, plus a byte count of Claude's dialog refusal (claude-observation-darwin-coverage, terminal-observation-linux-coverage).
- v0.11.0: Linux product rows through a real gateway: CLI/desktop, attention and filter, guarded send/text/keys/stop/close, `Terminal.ObservationFailed` logging, resolve/capture deadlines with a stalled tmux server, the observation negatives (quoted old chrome, menu versus request, historical errors under current work), tmux 3.4 width divergence (terminal-observation-linux-coverage).
- v0.11.0: live provider smoke on devbox and arch against the real services: idle, working and one request (terminal-observation-provider-smoke).
- v0.11.0: attention through cancellation on desktop and phone: declined request, interrupted turn, request → idle raising ready outside a visit, unanswered request surviving a visit (agent-needs-input).

Notifications and push:

- v0.14.0: the spec's one required [physical delivery](https://github.com/NielsdaWheelz/skidbladnir/blob/623280a5bde00c1b732f49e33afe89716633e042/docs/native-notifications.md#implementation-and-acceptance): a locked phone alerts with skid closed and the Mac asleep. ntfy is gone, so the run now uses optional push (native-notifications).
- v0.14.0: complete previous-release rollback without migrations. No host runs v0.14.0 any more, so this line awaits the owner's retirement (native-notifications).
- v0.14.0: delivery to a locked phone in proved deep idle; Android rejected forced idle at `inactive` (native-notifications-doze).
- v0.14.0: audible default notification sound on the phone, which stayed muted (native-notifications-phone-sound).
- v0.14.0: Mac popup, sound and exact-session click from the current signed app at its installed path (native-notifications, generalization-installed-cutover).
- v0.22.0: the desktop usage band shows a number for each Codex profile whose daemon runs; the Go quota client was proven only against a fake app-server (attention-cutover, at `b9c9806`).
- v0.13.1: sustained idle/offline/setup energy of the Mac notification helper; only short captures exist and the original heat report is unattributed (native-notifications-mac-energy).
- v0.19.0: optional push installed round trip: ordinary APK and user hub credential, pairing confirmation, background current-attention delivery, offline and process-death recovery, project change, rotation/removal, delayed hints (android-optional-push-qualification).
- v0.19.0: two physical phones: independent pairing, discovery, rotation and removal, and shared acknowledgement; owner-deferred for lack of a second phone (android-two-phone-push-acceptance).
- w3.1 release: hub-owned attention on devices: Mac banner, sound, silent retitle, click and dismiss from the installed signed app; Android heads-up, silent update and one re-alert after reboot; a phone swipe or click clearing the Mac copy and the reverse; Apple Terminal `do script` coming to the front; normal-priority clears reaching a dozing phone (native-notifications).

Resolved when: each line passes, recorded content-free where its feature spec
keeps qualification, or the owner retires it. Delete its line then, and this
file when no line remains. Phone, reboot and default-server work each require
current-turn approval.
