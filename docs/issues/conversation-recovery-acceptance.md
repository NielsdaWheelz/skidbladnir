# conversation recovery acceptance

problem: conversation recovery ships in v0.22.0 and its schema-2 cutover ran on
every host, but deployed acceptance is partial. the
[contract](../session-recovery.md#cutover-and-qualification) remains unfulfilled
at the boundaries below. isolated evidence is not fleet acceptance.

impact: exact native conversation reopening, startup/profile behavior, fresh
client notices and actual reboot recovery remain unverified on the installed
fleet. schema 1 is deliberately rejected; upgrading without retaining its recipe
and arming a fresh baseline disables recovery.

evidence, 2026-10-23:

- isolated macos: original target journey failed; changed public gateway, private
  tmux and real codex 0.163.0 / claude 2.1.310 against local model mocks passed.
  checked same-process native selection changes, inactive panes, daemon restart,
  exact-id history reopening and continuation in the same native transcript.
- isolated macos graph/policy: linked windows, native groups, absent/empty target,
  cwd rules, duplicate selection, profile changes, bash/zsh startup, strict schema,
  overlapping saves, partial construction and final-save failure passed. failed
  reconstruction retained its claim and never replayed. synthetic metadata cases
  are separate from native-provider evidence.
- isolated native linux-amd64 on arch: codex and claude selection changes,
  exact-id history/continuation, actual codex daemon restart, graph/policy,
  overlapping saves and failed reconstruction/publication passed. the final
  native daemon journey used the final unicode-corrected binary. arm64 docker
  evidence is supplemental; amd64 emulation did not qualify native recognition.
- actual claim-write fault: scratch syscall injection failed the parent-directory
  sync after rename. the claim was visible, not confirmed durable; recovery
  reported broken with zero launches, and restart refused the visible claim.
  a populated replacement was preserved and rejected without changing the recipe.
- unicode regression: native codex `/rename` reproduced a unicode-15 shortening
  mismatch. the pinned unicode-16 reader restored and continued that conversation
  and preserved a second case that unicode 17 would render differently.
- host build/static checks and module consistency passed. temporary tests are
  removed before committing; no permanent harness or new gate is introduced.
- read-only preparation found installed v0.20.0 gateways on macbook, devbox and
  arch; local checkpoint schema 1, unclaimed. configured profiles have no resume
  command.
- 2026-10-24, v0.22.0: on devbox, macbook and arch the gateway was stopped with
  tmux live, the schema-1 `workspace.json` moved to the state directory's
  `cutover-20261024/`, and the new release saved a schema-2 baseline (devbox 4
  sessions, macbook 11, arch none). dev-server #202 added the four proposed
  `resume_command` entries to the hub's yaml. the phone joined the same day.
- 2026-10-24 13:04 local, macbook same-boot loss with real conversations: serve
  stopped, `tmux kill-server`, serve started. recovery reported `recovered` and
  rebuilt all 11 saved sessions; the 10 recognized agents were requested through
  their profiles' `resume_command` and came back (owner confirmed). two codex
  `personal` sessions then showed `needs setup`. the one claude pane skid had not
  recognized came back as a shell. two codex agents were mid-turn at the loss.
  the owner judged the recovery complete and chose to leave the unrecognized
  claude pane unless it recurs.
- real provider accounts, phone/desktop notice acceptance, actual macos/linux
  reboot and mid-turn loss are `NOT_RUN`.

to resolve:

1. linux: exercise a proven loss on devbox or arch with real conversations,
   reopen them, and verify history and a subsequent response in the same
   transcripts.
2. verify fresh recovered notices on desktop and phone, then an actual reboot on
   both platforms. mid-turn checks prove history and no skid input replay only.
3. retain only credential- and content-free evidence; remove this issue once
   these pass.

disruptive loss, reboots, real accounts and the phone need owner approval in the
current turn.
