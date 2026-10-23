# roadmap

what is next and what is deferred. the [architecture](architecture.md) is the
contract; each item's evidence and resolution criteria live in its
[issue](issues), and the [cleanup campaign](cleanup.md) tracks the code-reduction
prs.

## next

1. finish the cleanup campaign, one reviewed pr at a time
   ([prs](cleanup.md#prs), [open questions](cleanup.md#open-questions)).
2. install v0.21.0 as a fresh fleet on devbox, macbook, arch and the phone
   ([release cutover](issues/release-cutover.md)), then run the macbook's
   maintenance window ([launchd priority](issues/launchd-standard-priority.md)).
3. cut the next release, which carries hub-owned attention, host-rendered copy,
   the go quota client, stock xterm and exec budget a2/a3, and install it in
   lockstep ([attention cutover](issues/attention-cutover.md)).
4. after that install, measure the macbook's tmux exec cost
   ([exec overhead](issues/tmux-exec-overhead.md)) and run the phone pass for
   stock xterm ([phone acceptance](issues/stock-xterm-phone-acceptance.md)).
5. observe the shipped but unobserved journeys
   ([unrun acceptance](issues/unrun-acceptance.md)), including an actual reboot
   recovery, and explain linux claude's guarded start
   ([linux claude turn](issues/terminal-only-live-qualification.md)).

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
[group label constructor](issues/group-label-trusted-constructor.md) (owner
decided), [phone ready eligibility](issues/phone-ready-eligibility.md),
[notification member auth](issues/notification-member-auth.md),
[hello identity](issues/terminal-hello-identity.md),
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
- [mac serve stderr](issues/macos-serve-stderr-discarded.md),
  [behavioral suite](issues/test-system-reset.md) and
  [project license](issues/project-license.md): owner decisions.
- [cloud resources](issues/cloud-resource-cleanup.md): cleanup needs explicit
  authorization.
- [terminal runtime](issues/terminal-runtime-decision.md): control mode, an owned
  pty or tmux as is; undecided.
