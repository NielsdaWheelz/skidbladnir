# roadmap

what is next and what is deferred. the [architecture](architecture.md) is the
contract; each item's evidence and resolution criteria live in its
[issue](issues).

## next

1. measure the macbook's tmux exec cost
   ([exec overhead](issues/tmux-exec-overhead.md)) and run the phone pass for
   stock xterm ([phone acceptance](issues/stock-xterm-phone-acceptance.md)).
2. observe the shipped but unobserved journeys
   ([unrun acceptance](issues/unrun-acceptance.md)), including an actual reboot
   recovery, and explain linux claude's guarded start
   ([linux claude turn](issues/terminal-only-live-qualification.md)).
3. finish qualifying conversation recovery on deployed hosts and clients:
   linux loss, notices and actual reboots
   ([conversation recovery acceptance](issues/conversation-recovery-acceptance.md)).

## open defects

- phone terminal: [keyboard resize churn](issues/terminal-resize-churn.md),
  [landscape with the keyboard](issues/terminal-landscape-keyboard.md),
  [ime reshow clears the selection](issues/terminal-ime-reshow-selection.md).
- phone: [sheet uniformity](issues/sheet-uniformity.md),
  [tailscale dns outage](issues/phone-tailscale-dns.md).
- desktop: [sigterm hang](issues/tui-sigterm-hang.md),
  [canceled input race](issues/bubbletea-canceled-input-race.md),
  [list json output limit](issues/list-json-output-limit.md),
  [default machine case](issues/default-machine-case.md).
- host: [directory listing fifo hang](issues/directory-listing-fifo-hang.md),
  [nfc validation](issues/xtext-nfc-validation.md),
  [invisible label characters](issues/group-label-invisible-characters.md),
  [pane retarget](issues/terminal-pane-retarget.md),
  [agent creation 500](issues/agent-creation-internal-error.md),
  [member settings validation](issues/member-settings-validation.md).
- observation: [claude panel rows](issues/claude-panel-status-unknown.md),
  [unruled claude dialogs](issues/claude-unruled-request-dialogs.md).

## queued simplifications

each removes a second owner or a needless hop:
[phone ready eligibility](issues/phone-ready-eligibility.md),
[notification member auth](issues/notification-member-auth.md),
[hello identity](issues/terminal-hello-identity.md),
[terminal error copy](issues/terminal-error-copy.md),
[attention helpers](issues/attention-shared-helpers.md),
[inventory snapshot](issues/inventory-snapshot-wrapper.md),
[picker nullables](issues/picker-model-nullables.md),
[cli input bounds](issues/cli-terminal-input-bounds.md).

## deferred by the owner

- [phone notification entry](issues/phone-entry-metadata-latency.md): splitting
  the transport lifetime from the display model.
- [phone interaction redesign](issues/phone-interaction-redesign.md) and
  [visual redesign](issues/visual-redesign.md): unscoped.
- [launcher recognition](issues/provider-wrapper-recognition.md) and
  [agent exit attention](issues/agent-exit-attention.md): need scope first.
- [mac serve stderr](issues/macos-serve-stderr-discarded.md) and
  [behavioral suite](issues/test-system-reset.md): owner decisions.
- [cloud resources](issues/cloud-resource-cleanup.md): cleanup needs explicit
  authorization.
- [terminal runtime](issues/terminal-runtime-decision.md): control mode, an owned
  pty or tmux as is; undecided.
