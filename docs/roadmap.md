# Roadmap

What is next and what is deferred. The [architecture](architecture.md) is the
contract; each item's evidence and resolution criteria live in its
[issue](issues).

## Next

1. Carry the [publication campaign](publication.md) to its close.
2. Measure the macbook's tmux exec cost
   ([exec overhead](issues/tmux-exec-overhead.md)) and run the phone pass for
   stock xterm ([phone acceptance](issues/stock-xterm-phone-acceptance.md)).
3. Observe the shipped but unobserved journeys
   ([unrun acceptance](issues/unrun-acceptance.md)), including an actual reboot
   recovery, and explain Linux Claude's guarded start
   ([Linux Claude turn](issues/terminal-only-live-qualification.md)).
4. Finish qualifying conversation recovery on deployed hosts and clients:
   Linux loss, notices and actual reboots
   ([conversation recovery acceptance](issues/conversation-recovery-acceptance.md)).

## Open defects

- Phone terminal: [keyboard resize churn](issues/terminal-resize-churn.md),
  [landscape with the keyboard](issues/terminal-landscape-keyboard.md),
  [IME reshow clears the selection](issues/terminal-ime-reshow-selection.md).
- Phone: [sheet uniformity](issues/sheet-uniformity.md),
  [Tailscale DNS outage](issues/phone-tailscale-dns.md).
- Fleet and install: [hub self-report after an update](issues/hub-self-report-after-update.md),
  [member status placeholder](issues/member-status-placeholder.md),
  [connect operator check](issues/connect-operator-uid-match.md),
  [systemd unit warning](issues/systemd-unit-changed-warning.md).
- Desktop: [SIGTERM hang](issues/tui-sigterm-hang.md),
  [canceled input race](issues/bubbletea-canceled-input-race.md),
  [list JSON output limit](issues/list-json-output-limit.md),
  [default machine case](issues/default-machine-case.md).
- Host: [directory listing FIFO hang](issues/directory-listing-fifo-hang.md),
  [NFC validation](issues/xtext-nfc-validation.md),
  [invisible label characters](issues/group-label-invisible-characters.md),
  [pane retarget](issues/terminal-pane-retarget.md),
  [agent creation 500](issues/agent-creation-internal-error.md),
  [member settings validation](issues/member-settings-validation.md).
- Observation: [Claude panel rows](issues/claude-panel-status-unknown.md),
  [unruled Claude dialogs](issues/claude-unruled-request-dialogs.md).

## Queued simplifications

Each removes a second owner or a needless hop:
[phone ready eligibility](issues/phone-ready-eligibility.md),
[notification member auth](issues/notification-member-auth.md),
[hello identity](issues/terminal-hello-identity.md),
[terminal error copy](issues/terminal-error-copy.md),
[attention helpers](issues/attention-shared-helpers.md),
[inventory snapshot](issues/inventory-snapshot-wrapper.md),
[picker nullables](issues/picker-model-nullables.md),
[CLI input bounds](issues/cli-terminal-input-bounds.md).

## Deferred by the owner

- [Phone notification entry](issues/phone-entry-metadata-latency.md): splitting
  the transport lifetime from the display model.
- [Phone interaction redesign](issues/phone-interaction-redesign.md) and
  [visual redesign](issues/visual-redesign.md): unscoped.
- [Launcher recognition](issues/provider-wrapper-recognition.md) and
  [agent exit attention](issues/agent-exit-attention.md): need scope first.
- [Mac serve stderr](issues/macos-serve-stderr-discarded.md) and
  [behavioral suite](issues/test-system-reset.md): owner decisions.
- [Cloud resources](issues/cloud-resource-cleanup.md): cleanup needs explicit
  authorization.
- [Terminal runtime](issues/terminal-runtime-decision.md): control mode, an owned
  PTY or tmux as is; undecided.
