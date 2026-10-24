# conversation recovery acceptance

problem: conversation recovery is implemented on `codex/conversation-recovery`,
but deployed acceptance and schema-2 cutover have not run. the
[contract](../session-recovery.md#cutover-and-qualification) remains unfulfilled
at those boundaries. isolated evidence is not fleet acceptance.

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
  command. the already-open [release cutover](release-cutover.md) and
  [attention cutover](attention-cutover.md) precede deploying this branch.
- real provider accounts, fleet installation, phone/desktop notice acceptance,
  actual macos/linux reboot and mid-turn loss are `NOT_RUN`.

to resolve, after explicit current-turn authority:

1. complete the existing fleet cutover with a matching host/client release.
   keep live tmux and the old recipes until each
   new gateway has durably saved schema 2. do not discard a sole old recipe.
2. add owner-approved `resume_command` entries using each existing configured
   launcher and quoted `"$1"`; validate ordinary account/home behavior. no stock
   command substitution or injected permission policy.
3. create disposable real-provider conversations on macos/linux; record private
   native-id oracles, checkpoint, exercise proven loss and reopen those ids.
   verify history and a subsequent response in the same transcripts. report
   requests separately from native success and any skipped panes.
4. verify fresh recovered notices on desktop and phone, then actual reboot on
   both platforms. mid-turn checks prove history/no skid input replay only.
5. delete temporary qualification tests; retain only credential/content-free
   evidence. remove this issue after these boundaries pass, not after builds.

the fleet upgrade, real accounts, phone and disruptive loss/reboots still need
owner approval. read-only preparation, task-owned linux containers and isolated
arch checks were authorized; none authorizes these effects.

proposed entries from the currently applied profile commands; add these fields
to the existing profiles, preserving every other setting:

| profile | `resume_command` |
| --- | --- |
| `personal` | `codex resume "$1"` |
| `work` | `codex-work resume "$1"` |
| `work2` | `codex-work2 resume "$1"` |
| `claude-work` | `claude-work --resume "$1"` |

do not apply these to v0.20.0: that release rejects the new field. install/accept
configuration in the coordinated cutover. use the `dev-server` ssh alias for
devbox and `arch` for arch; read-only preparation confirmed both routes.
