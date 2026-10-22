# unrun acceptance of shipped releases

problem: these checks were specified for shipped releases and never observed.
they are unobserved journeys, not known defects. each line names the release
the check was specified against and the former record, in parentheses, whose
procedure and evidence remain at
[docs/issues as of `12b1516`](https://github.com/NielsdaWheelz/skidbladnir/tree/12b15163d9faf529416d486e0156b8b83821fa4a/docs/issues).

impact: a regression on one of these paths ships unseen until daily use meets it.
source checks, isolated journeys and installation do not substitute for them.

terminal continuity and creation:

- v0.10.3: shell row on the installed fleet: initial-agent return to its shell, special-character cwd/argv, job control, input/resize, startup-child isolation, repeated sourcing (terminal-continuity-shell-qualification).
- v0.10.3: foreground observation on linux and darwin: every configured home, absent/empty/unlisted/inaccessible homes, read cap, background exclusion, stale-action rejection (terminal-continuity-observation-qualification).
- v0.10.3: directory search against real host zoxide: ranked results, literal paths, no match, missing/disabled tool, bounds, removed directory, host-switch races (terminal-continuity-directory-qualification).
- v0.10.3: remote lifecycle: simultaneous connections, nested hops, suspend/resume, reconnect, gateway restart, source loss, and the [remote adversarial cases](../terminal-continuity.md#6-delivery-and-adversarial-acceptance) (terminal-continuity-remote-qualification).
- v0.10.3: phone forge initial-agent launch and explicit terminal selection/Copy on the installed apk (terminal-continuity-phone-controls-qualification).
- v0.10.3: one of 12 unrelated sessions vanished during the physical-phone window; cause unattributed, so unrelated-session preservation is unproven (terminal-continuity-phone-qualification).
- v0.14.0: workspace recovery after an actual linux and mac reboot, with changed boot identity and no client request (session-recovery-reboot).
- v0.20.0: clean supported-host installation, real uninstall with child-session survival, ghostty and apple terminal adapters, installed owner visual parity (generalization-installed-cutover).
- v0.20.0: the owner confirms no blank apple terminal window from the 2026-10-20 adapter probes remains open; never close an unreceipted one automatically (generalization-terminal-probe-windows).

desktop:

- v0.11.0: live `s`/`x` provider and tmux closure, and [a6](../desktop-browser.md#7-acceptance-and-delivery) info editing (takeover/reset, naming conflicts, lost acknowledgements, replaced targets) through a real gateway (desktop-browser-runtime-acceptance).
- v0.18.0: installed production terminal entry/greeting on desktop and phone; needs an owner exception to the default-server guardrail (desktop-browser-runtime-acceptance).
- v0.17.3: owner readability review of single-line rows at 80×24 and 160×24 (desktop-single-line-rows).
- v0.11.0: live stale and checking cells: a failed host's rows muted `last observed` and never ready, re-read hosts `checking`, pending ready restored after recovery (terminal-observation-stale-live).

phone:

- v0.5.0: hands-on groups/shells workflow: membership set/change/clear, intersecting filters and return position, forge/session-sheet creation with cwd/group inheritance, duplicate taps, narrow and enlarged text (groups-shells-hands-on).
- v0.10.0: groups filter/edit/create/detach against a real host; darwin isolated-host membership continuity. its capsule-restore rows are superseded by [session views](../session-views.md) schema 4 (groups-cutover-acceptance).
- v0.10.7: naming composition: strict `nameMode` decoding, narrow large-text naming controls, ready persistence through rename/reset (reply-notifications-phone-composition).
- v0.13.0: terminal chrome on the s22+: one-row rail through connect/frozen/too-small and both sheets (terminal-chrome-hands-on).
- v0.15.0: on real sessions, open and cancel both closure confirmations, then a terminal visit/return keeping the dashboard anchor (session-card-hands-on).
- v0.11.0: concurrent work with a request, generic needs input, production-gateway delayed response (terminal-observation-phone-acceptance).

terminal observation and attention:

- v0.11.0: codex grammar RP/AC rows whose [§1 `qualified` column](../terminal-observation-codex.md#1-frozen-capability-table) omits a platform, including server/account-gated families: network approval, user verification, sub-agent view, remote images (codex-observation-darwin-coverage, codex-server-driven-families, terminal-observation-linux-coverage).
- v0.11.0: claude grammar `R` rows whose [§1 `qualified` column](../terminal-observation-claude.md#1-frozen-capability-table) omits a platform, plus a byte count of claude's dialog refusal (claude-observation-darwin-coverage, terminal-observation-linux-coverage).
- v0.11.0: linux product rows through a real gateway: cli/desktop, attention and filter, guarded send/text/keys/stop/close, `Terminal.ObservationFailed` logging, resolve/capture deadlines with a stalled tmux server, spec §8 negatives, tmux 3.4 width divergence (terminal-observation-linux-coverage).
- v0.11.0: live provider smoke on devbox and arch against the real services: idle, working and one request (terminal-observation-provider-smoke).
- v0.11.0: attention through cancellation on desktop and phone: declined request, interrupted turn, request → idle raising ready outside a visit, unanswered request surviving a visit (agent-needs-input).

notifications and push:

- v0.14.0: the spec's one required [physical delivery](https://github.com/NielsdaWheelz/skidbladnir/blob/623280a5bde00c1b732f49e33afe89716633e042/docs/native-notifications.md#implementation-and-acceptance): a locked phone alerts with skid closed and the mac asleep. ntfy is gone, so the run now uses optional push (native-notifications).
- v0.14.0: complete previous-release rollback without migrations. no host runs v0.14.0 any more, so this line awaits the owner's retirement (native-notifications).
- v0.14.0: delivery to a locked phone in proved deep idle; android rejected forced idle at `inactive` (native-notifications-doze).
- v0.14.0: audible default notification sound on the phone, which stayed muted (native-notifications-phone-sound).
- v0.14.0: mac popup, sound and exact-session click from the current signed app at its installed path (native-notifications, generalization-installed-cutover).
- v0.13.1: sustained idle/offline/setup energy of the mac notification helper; only short captures exist and the original heat report is unattributed (native-notifications-mac-energy).
- v0.19.0: optional push installed round trip: ordinary apk and user hub credential, pairing confirmation, background current-attention delivery, offline and process-death recovery, project change, rotation/removal, delayed hints (android-optional-push-qualification).
- v0.19.0: two physical phones: independent pairing, discovery, rotation and removal, and shared acknowledgement; owner-deferred for lack of a second phone (android-two-phone-push-acceptance).
- w3.1 release: hub-owned attention on devices: mac banner, sound, silent retitle, click and dismiss from the installed signed app; android heads-up, silent update and one re-alert after reboot; a phone swipe or click clearing the mac copy and the reverse; apple terminal `do script` coming to the front; normal-priority clears reaching a dozing phone (native-notifications).

resolved when: each line passes, recorded content-free where its feature spec
keeps qualification, or the owner retires it. delete its line then, and this
file when no line remains. device, reboot and default-server work each require
current-turn approval.
