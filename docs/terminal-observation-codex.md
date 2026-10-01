# codex screen grammar

status: implemented in `internal/agentcontrol/codex.go`, whose comments cite the
sections below. qualified live for codex-cli 0.159.2 on darwin and on linux as
§1 records; the [qualification](terminal-agent-control-qualification.md#terminal-observation-qualification)
holds the evidence and its limits, and [the spec](terminal-observation.md) owns
the status contract this grammar feeds.

upstream source: `openai/codex@ff6aec96948b70d94983af2641a6b67c94faeff5`, whose
workspace `Cargo.toml` says `0.159.2`. paths below are under `codex-rs/tui/src/`
unless they name another crate. line numbers refer to that commit.

installed versions qualified:

| platform | tmux | codex |
| --- | --- | --- |
| darwin 25.4.0 arm64 | 3.7c | TUI 0.159.2 (native darwin-arm64 binary), embedded or against a 0.159.2 app-server daemon; the question auto-resolve and idle rows also against a copy of the installed 0.159.3 daemon |
| linux: devbox (ubuntu, kernel 6.8, x86_64), arch (kernel 7.2.6, x86_64) | 3.4 (devbox), 3.7c (arch) | TUI 0.159.2 (native musl binary) against the installed 0.159.3 daemon, the production pairing |

notation:

- `K` is a key-label span: one non-empty run of bold cells, any fg. it is not
  dim, except inside the `Press` footer (2.3), where user verification dims the
  whole line.
- `D` (duration) is `\d+s`, `\d+m \d\ds` or `\d+h \d\dm \d\ds`.
- `E` is the last non-blank row of the screen; `F` is the composer footer run
  (2.1); `C` is the composer row.
- style tags: `{d}` dim, `{b}` bold, `{br}` bold+reverse, `{bd}` bold+dim, `{/}`
  reset.
- columns count cells from 0. a row is one physical row parsed on its own.
- `h` is the pane height and `W` the width.
- a row is *blank* when it holds no non-space glyph; sgr and background fills are
  ignored. tmux trims the trailing spaces after a row's last escape even when
  they carry a background, so a band padding row captures as a bare sgr prefix.
- a *col-0 row* has a non-space cell at col 0. an *indented row* has spaces at
  cols 0–1.
- a row is *dropped* when the capture did not hold it (a byte cap, or rows
  `192..h−65` of a pane taller than 256 rows) and *unparseable* when it holds an
  escape tmux's writer never emits ([spec §3](terminal-observation.md#3-composition-and-observation-boundary)).
  a dropped row is missing evidence; an unparseable row is present but
  unrecognized: it matches no rule and is never clipped evidence.
- values are written activity/interaction/notice/composer.

## 1. frozen capability table

classes:

- RP, required-positive: spec §4 names the family, or codex renders it as a
  permission or question (spec §8).
- AC, ambiguity control: the family must produce exactly the stated values, or
  no evidence, and never a claim stronger than they are. most state unknown or
  none; two mcp rows state the form's input in place of the approval's
  permission.
- PU, permitted unknown-or-none: when the row carries a rule id, the family is
  recognized if seen.
- UA, upstream-unavailable in 0.159.2: not implemented, not a pass.

`qualified` names the platforms where the family ran live against the installed
versions above. `capture` means authored rows painted through real tmux on that
platform, classified with the evidence dropped from the real capture (on darwin
also by a real byte cap): that proves capture and parser mechanics, never
provider behaviour, and it applies only to rows about dropped evidence.
`source-only` means upstream source and authored rows only. an RP or AC row is
`NOT_RUN` on every platform it does not name, and each such `NOT_RUN` is a
blocker under spec §8
([darwin](issues/codex-observation-darwin-coverage.md),
[linux](issues/terminal-observation-linux-coverage.md),
[server-driven](issues/codex-server-driven-families.md)). a PU `NOT_RUN` is
informative. cue-less timing windows are not families; §6 holds them.

| family / variant | class | values | rule id | qualified | basis |
| --- | --- | --- | --- | --- | --- |
| status row: glyph, `K to interrupt` hint | RP | working | `codex.activity.status_row` | darwin, linux | `status_indicator_widget.rs:230-290` |
| status row without glyph (reduced motion) | RP | working | same | darwin, linux | `motion.rs` |
| status row, remapped interrupt key | RP | working | same | darwin, linux | `key_hint.rs` |
| status row suffix (`· N background terminal(s) …`, hook text) | RP | working | same | darwin | `unified_exec_footer.rs` |
| status row with `└` detail rows | RP | working | same | darwin, linux | — |
| status row cut by `…` (anywhere in the paren group) | RP | working | same | darwin | `line_truncation.rs:76-99` |
| col-0 row starting with a dim `• ` cut inside a paren group (`• Viewed image …(1…`, hook row) | AC | no activity evidence | none | darwin (view-image row; the hook row source-only) | `history_cell/patches.rs:166-178`, `bottom_pane/hook_status.rs:19-26` |
| hintless status row (unbound interrupt key) | RP | working | same | darwin, linux | `status_indicator_widget.rs:252-259` |
| status row whose header swallows the paren group | PU | no activity evidence | none | darwin | §6 c14 |
| background-terminal waiting | RP | working | same (+`codex.run_state.working`) | darwin | `chatwidget/command_lifecycle.rs` |
| retry row (`Reconnecting... waiting for network`) | RP | working | same | darwin, linux | `chatwidget/streaming.rs:393-400` |
| run-state `Working` / `Waiting` | RP | working | `codex.run_state.working` | darwin (both), linux (`Working`) | `chatwidget/status_surfaces.rs:991-1012` |
| run-state `Thinking` | RP | working | `codex.run_state.working` | darwin | `chatwidget/streaming.rs:54,341,396` |
| run-state `Starting` | RP | starting | `codex.run_state.starting` | darwin, linux | `chatwidget/mcp_startup.rs` |
| run-state `Ready` + clean band with the main placeholder + settled transcript scan | RP | idle | `codex.run_state.ready` | darwin, linux | §3 |
| `Ready` + hinted status row (the stale-refresh paths of §3) | AC | unknown, `evidence_conflict` | rules of both | source-only | §3 |
| submit window: `Ready`, no row, the transcript scan stops at a prompt | AC | unknown, `evidence_conflict` | `codex.activity.prompt_pending` | darwin | `chatwidget/input_submission.rs:473-492` |
| goal continuation: `Ready` + `Pursuing goal` indicator between goal turns | AC | unknown, `evidence_conflict` | `codex.activity.goal_active` | darwin | `ext/goal/src/runtime.rs:425-497`, `extension.rs:180-191`, `core/src/tasks/mod.rs:811-883`, `bottom_pane/footer.rs:579-626`, `chatwidget/turn_runtime.rs:205-218` |
| goal indicator truncating the run-state word (narrow panes) | PU | no run-state cue | none | darwin | `chat_composer/status_surface.rs:22-55` |
| disconnect: hint row is the `K quit` override | AC | unknown, overrides status row and run-state | `codex.activity.disconnected` | darwin, linux | `chatwidget/reconnect.rs:31-35` |
| external editor: hint row is `Save and close external editor to continue.` | AC | composer blocked; `Ready` → unknown; working stands | `codex.composer.external_editor` | darwin | `app.rs:293`, `app/input.rs:300`, `tui.rs:333-336` |
| default footer (no run-state) | AC | activity unknown unless a status row shows | `codex.interaction.none` | darwin, linux | `config/src/types.rs` default items |
| layout without a status line | AC | activity from the status row only | `codex.interaction.none` | darwin | — |
| empty composer during work | AC | composer empty; activity from row or run-state | — | darwin, linux | — |
| composer surface with no request element | RP | interaction none | `codex.interaction.none` | darwin, linux | 2.1 |
| shortcut overlay (`?`) | RP | the composer surface's values; `evidence_clipped` when it pushes the transcript terminator off-screen | `codex.interaction.none` | darwin (100 columns idle; 60 and 40 columns `evidence_clipped`) | `bottom_pane/footer.rs:226-233` |
| `Ready` with a draft, history search or any unclean band | AC | activity unknown | — | darwin, linux (draft) | §3 step 4.1 |
| side view (placeholder `Ask a follow-up question`) | AC | `Ready` → unknown; composer blocked; working stands | `codex.scope.side` | darwin | `chatwidget/side.rs:17-28`, `chatwidget.rs:2042` |
| side view with a draft (placeholder gone, `Side` label hidden in some modes) | AC | activity unknown (draft) | — | darwin | `bottom_pane/chat_composer.rs:4833-4846` |
| parent-owned sub-agent view (placeholder `Viewing sub-agent — direct input is disabled`, input still enabled, bold glyph) | AC | composer unknown; `Ready` → unknown; working stands | — | source-only: needs a thread with `can_accept_direct_input=false`, which multi_agent_v2 spawns would give and the 0.159.2 router rejected | `bottom_pane/chat_composer.rs:1652-1659`, `:4933-4952`, `app_server_session.rs:388-393` |
| v1 sub-agent view (enabled composer, main placeholder) | PU | reads as the displayed thread (§6 c2) | same as main | darwin | — |
| background sub-agents behind a main `Ready` | PU | idle as displayed (§6 c2) | — | darwin | — |
| exec approval overlay | RP | unknown/permission/none/blocked | `codex.permission.overlay` | darwin, linux | `bottom_pane/approval_overlay.rs:639-663` |
| edits approval | RP | same | same | darwin | — |
| permissions-grant approval | RP | same | same | darwin | — |
| write-stdin approval (`… send input to terminal N?`) | RP | same | same | darwin | `approval_overlay.rs:256-262` |
| mcp elicitation fallback approval (unsupported field) | RP | same | same | darwin | `chatwidget/tool_requests.rs:362-372` |
| network approval (`Do you want to approve network access to …?`) | RP | same | same | source-only: needs a managed network-proxy policy (`core/src/tools/network_approval.rs`) | `approval_overlay.rs:264-270`, a title-only variant of the exec overlay |
| user verification (whole footer dim, keys bold+dim) | RP | same | same | source-only: the app-server advertises verification only when the device supports it, and this client rejected `openai/elicitation/create` | `bottom_pane/user_verification.rs:92-103`, `app-server/src/request_processors/initialize_processor.rs:107-128` |
| approval with open-thread footer | RP | permission | `codex.permission.overlay` | darwin | — |
| `! Approval needed in <thread>` preview | RP | permission | `codex.permission.other_thread` | darwin | `bottom_pane/pending_thread_approvals.rs`, `app/thread_routing.rs:1096-1125` |
| approval with the cancel key unbound (`Press K to confirm` only) | AC | layout_unknown | none | source-only | `popup_consts.rs:42-60` |
| approval details pager (`/ E X E C`, `/ P A T C H`, …) | RP | unknown/permission/none/blocked | `codex.permission.details_pager` | darwin | `app/event_dispatch.rs:2912-2985` |
| mcp tool approval / message-only elicitation | RP | unknown/permission/none/blocked | `codex.permission.mcp_approval` | darwin, including hidden options with the `option N/M` count clipping `esc to cancel` or the submit hint (22–31 columns) | `bottom_pane/mcp_server_elicitation.rs:226-310` |
| mcp approval whose option rows draw no label (≤ 21 columns) | AC | the form, not the approval | `codex.input.mcp_form` | darwin | §6 c15 |
| mcp form, single field | RP | unknown/input/none/blocked | `codex.input.mcp_form` | darwin | — |
| mcp form, multi-field (`to submit answer` / `to submit all`) | RP | unknown/input/none/blocked | `codex.input.mcp_form` | darwin, including `esc to cancel` or the submit hint cut by the count (20–38 columns) | `mcp_server_elicitation.rs:971-996` |
| mcp option block holding a row that was not parsed | AC | the form, not the approval | `codex.input.mcp_form` | capture: darwin, linux | 2.4 |
| url elicitation / app link view | PU | unknown | none | source-only | `bottom_pane/app_link_view.rs` |
| legacy request_user_input with options, single or multi | RP | unknown/question/none/blocked | `codex.question.legacy` | darwin (single, multi, wrapped footer, notes hint cut at 16–28 columns), linux (single) | `bottom_pane/request_user_input/mod.rs:572-626` |
| legacy question with hidden options (`option N/M` leading the footer; submit hint pushed out of the drawn rows; the count alone in a one-row footer) | RP | same | same | darwin | `request_user_input/render.rs:344-358`, `mod.rs:632-656`, `layout.rs:150-173` |
| legacy question, notes mode or all-answered header | RP | question | same | darwin | — |
| legacy option-less free-text question | UA | — | (`codex.question.legacy` if ever rendered) | — | core rejects it: `core/src/tools/handlers/request_user_input_spec.rs:105-114`; free text enters through the auto-added Other/notes path |
| legacy `Submit with unanswered questions?` | RP | question | `codex.question.legacy_confirm` | darwin | `request_user_input/render.rs:128` |
| async questions collapsed (`? N questions`) | RP | question | `codex.question.async_collapsed` | darwin | `bottom_pane/questions.rs` |
| async questions editor | RP | question, composer blocked | `codex.question.async_editor` | darwin | `bottom_pane/async_questions/` |
| async editor inline flash replacing its footer | PU | layout_unknown | none | source-only | `async_questions/render.rs:171-174` |
| plan implementation decision | RP | unknown/confirmation/none/blocked | `codex.confirmation.plan` | darwin | `chatwidget/plan_implementation.rs` |
| footered provider pickers (`Approaching rate limits`, `Resume paused goal?`, `Usage limit reached`, `You've reached your workspace credit limit`) | PU | confirmation | `codex.confirmation.provider_picker` | darwin (`Approaching rate limits`); the other titles need server state | `chatwidget/rate_limits.rs:530,578`, `goal_menu.rs:50`, `chatwidget/backend_banners.rs:420-426` |
| footerless pickers: precaution titles, reserve (`… to continue working`), `Usage limit resets` progress, safety buffering | PU | layout_unknown | none | source-only | `chatwidget/misalignment_policy.rs`, `backend_banners.rs:446-470`, `chatwidget/usage.rs:338-347`, `chatwidget/safety_buffering.rs:190-280` |
| `/ What we detected` pager | PU | confirmation | `codex.confirmation.details_pager` | source-only | `app/misalignment_policy.rs:23-46` |
| inline banner with actions (`Press a number to choose`) | PU | confirmation | `codex.confirmation.inline_banner` | source-only | `bottom_pane/actionable_banner.rs:167-205`, `bottom_pane/mod.rs:2181-2187` |
| inline banner, information only | PU | interaction unknown | none | source-only | same |
| trust / folder access | RP | unknown/setup/none/blocked | `codex.setup.trust` | darwin (including a title in the top region, `compound`), linux | `onboarding/trust_directory.rs` |
| update prompt | RP | setup | `codex.setup.update` | darwin | `update_prompt.rs` |
| login, first screen | RP | setup | `codex.setup.login` | darwin | `onboarding/auth.rs` |
| model migration prompt | PU | setup | `codex.setup.migration` | source-only | `model_migration.rs:280-305` |
| startup hooks review | PU | setup | `codex.setup.hooks_review` | source-only | `startup_hooks_review.rs:213-275` |
| daemon recovery (footerless, startup) | PU | setup | `codex.setup.daemon_recovery` | darwin | `daemon_recovery.rs:115-176`, `daemon_startup.rs:119-186` |
| picker whose title is unproven (a dropped row) | AC | interaction unknown, `evidence_clipped`; composer blocked when footered | none | capture: darwin, linux | 2.6 |
| a dropped row met while locating the composer surface (after every other rule fell through) | AC | unknown/unknown, `evidence_clipped` | none | capture: darwin, linux | 2.0 step 4, 2.1 |
| blank bottom region on a pane taller than 256 rows (rows `192..h−65` dropped) | AC | unknown/unknown, `evidence_clipped` | none | capture: darwin, linux | 2.0 step 1 |
| login follow-ups, cwd, unarchive, oss selection prompts | PU | layout_unknown | none | source-only | `onboarding/auth.rs`, `cwd_prompt.rs`, `unarchive_prompt.rs`, `oss_selection.rs` |
| user-opened picker, numbered | RP | unknown/menu/none/blocked | `codex.menu.picker` | darwin, linux | `bottom_pane/list_selection_view.rs:640-700` |
| user-opened searchable picker (unnumbered rows, search row, `↑`/`↓` indicators) | RP | same | same | darwin | `list_selection_view.rs:670-674,1466-1532`, `bottom_pane/picker_style.rs:54-67` |
| resume picker | RP | unknown/menu/none/blocked | `codex.menu.resume` | darwin | `resume_picker.rs:2405-2450` |
| slash popup over a draft | AC | interaction none, composer draft, activity unknown | `codex.interaction.none` | darwin, linux | — |
| transcript / diff pager | RP | menu | `codex.menu.pager` | darwin | `pager_overlay.rs:206-212` |
| interactive transcript footers (browse, find, selection, disclosure): composer dimmed | RP | menu, composer blocked; `Ready` still gives idle | `codex.menu.transcript_footer` | darwin (browse, find); selection and disclosure need a mouse drag or an activity focus that did not engage, and share the rule because the footer text is never read | `chat_composer.rs:5041-5043`, `transcript_view/footer.rs:30-66`, `transcript_view/disclosure.rs:357-403`, `app_backtrack/prompt_navigation.rs:30-70` |
| warnings view (f2) | RP | menu | `codex.menu.warnings` | darwin | `bottom_pane/warnings_view*.rs` |
| fullscreen detail toggle (ctrl+t) | AC | not an overlay | — | darwin | — |
| interruption and error cells (`■ …`) | PU | notice none; a transcript terminator (2.2) | none | darwin | `chatwidget/input_restore.rs:312-336`, `history_cell/notices.rs:327-332` |
| composer gap row (fullscreen): usage notice (bold at ≥ 90 % used), copy feedback, follow control `↓ …` | AC | never activity or request evidence | — | darwin | `bottom_pane/composer_gap.rs`, `app/owned_transcript.rs:237-254`, `transcript_view/composer_gap.rs:70-114`, `transcript_view/follow_control.rs:21-81`, `chatwidget/usage_notice.rs:31-87` |
| sticky prompt header (transcript row 0, rendered like a historical prompt) | AC | a prompt stop of the transcript scan | — | darwin | `transcript_view/prompt_header.rs:18-60`, `transcript_view.rs:189-200` |
| completion and working tips (`  └ Tip: …`) | AC | ordinary transcript rows; never a stop | — | darwin | `app/turn_tips.rs:146-230`, `transcript_view/turn_tip.rs`, `transcript_view/layout.rs:238-242` |
| image-only prompt (`  [Image #N]` rows, no `›` row) | AC | a prompt stop of the transcript scan | — | source-only: only remote images (another client's history) render labels outside the message | `history_cell/messages.rs:204-237` |
| inline layout (`alt=0`) | RP | same values | same ids | darwin, linux | — |
| known terminal background (OSC 10/11 answered) | RP | same values | same ids | darwin | `style.rs:60-75,143-153,209-214`, `style/contrast.rs:35-78` |
| tall fullscreen, 65 ≤ h ≤ 256, short (top-anchored) transcript | RP | idle through continuation | `codex.run_state.ready` (region compound) | darwin, linux | `transcript_view.rs:518-545` |
| tall fullscreen, h > 256, transcript tail above row `h−64` (rows `192..h−65` dropped) | AC | unknown, `evidence_clipped` (idle lost) | `codex.run_state.ready` | darwin, linux | spec §3 |
| stale chrome after the provider is killed (provider → shell → same provider, before the first draw) | AC | layout_unknown | none | darwin (`zsh -f`, `bash --norc`; 0.32–0.42 s until the first draw) | §6 c13 |
| remote image rows `[Image #N]` above the prompt | RP | composer draft | — | source-only: only rehydrated history from another client attaches remote images | `chat_composer.rs:4927-4932`, `composer_layout.rs:118-129` |
| Astra sparkle (band except the placeholder span, ≤ 15 s) | PU | layout_unknown, or composer and activity unknown | none | source-only (not observed with the model selected) | `chat_composer/sparkle.rs:344-417` |
| Max/Ultra ignition sparks (0.9–1.6 s, padding rows only) | PU | composer unknown, activity unknown | none | source-only (not observed) | `bottom_pane/effort_ignition_styles.rs:68-86` |
| Max/Ultra status-line transition (≈ 2.5 s) | PU | run-state unreadable | none | source-only | `effort_status_line.rs` |
| standalone hook row, previews, standalone background row | AC | no value of their own | — | darwin | `bottom_pane/mod.rs:2177-2263` |
| forking, external writer, agents overview, realtime voice | PU | layout_unknown | none | source-only | `chatwidget/rendering.rs` |
| explicit ready cue under the default footer | UA | idle impossible | none | — | default items have no run-state |
| windows sandbox prompts | UA (fleet is darwin/linux) | — | — | — | `chatwidget/windows_sandbox_prompts.rs` |
| title / progress / bell | UA (excluded by spec) | — | — | — | not read |

on linux the status line kept `Working` during a retry when `tui.terminal_title`
lacked run-state; §3's refresh rule allows that lag, and both words read working.

## 2. screen grammar

### 2.0 surface selection

1. region. rows are pane rows; the regions are contiguous unless a limit dropped
   rows ([spec §3](terminal-observation.md#3-composition-and-observation-boundary):
   the top region is rows `0..min(h−65, 191)`).
   - `E` is found upward from `h−1` over blank rows, from the bottom region into
     the top region; rules matched wholly in the top region carry region `top`.
   - a dropped row reached before any non-blank row: the screen may continue
     into rows that were not captured: unknown/unknown, `evidence_clipped` (a
     pre-session screen near the top of a pane taller than 256 rows leaves the
     bottom region blank and rows `192..h−65` dropped).
   - `E` unparseable: every surface anchors on `E`, which no rule recognizes:
     layout_unknown.
   - no non-blank row → layout_unknown.
2. continuation. every read above the bottom region follows one rule: the
   transcript scan (2.2), the option scan and the title lookup (below), and the
   pager's row 0 (2.7). a scan crosses from the bottom region into the top
   region by row. a dropped row stops it *unproven*; an unparseable row stops it
   *unreadable*: unknown, never clipped.
   - a rule whose decisive row lies in the top region carries region `compound`:
     the title rules and `codex.menu.pager` / the details pagers (row 0),
     `codex.run_state.ready` (the settled stop), `codex.activity.prompt_pending`
     (the prompt stop).
   - a byte-capped top region keeps rows from row 0 downward and drops its lower
     rows, so it keeps row 0 but never continues the bottom region.
3. `E` is the last non-blank row of the screen. never assume `E == h−1`.
4. rules, first match wins. footers wrap upward from `E` as 2.3 (word wrap), 2.4
   and 2.5 (whole hints) state. a rule that does not match falls through:
   1. `Press` footer → approval (2.3) or legacy confirm (2.4)
   2. ` | ` footer with a hint only the legacy view renders, or of the count
      alone → legacy question (2.4)
   3. otherwise a ` | ` footer with a submit hint, or led by the count and
      holding a hint only the mcp form renders → mcp form (2.4)
   4. triple-space footer with `K submit` and `K skip` → async editor (2.5)
   5. resume footer under a `─ n / m · p% ─` rule → resume (2.7)
   6. ` K close` over ` K to scroll · …` → pager (2.7)
   7. `K keep & next · K dismiss & close` → warnings (2.7)
   8. `Press K to continue` over `> N.` options → login (2.8)
   9. an option scan from `E` finds a selected row → picker (2.6)
   10. composer surface (2.1)

   no match: layout_unknown, composer unknown. two partial matches are final:
   rule 6, whose matched footer makes row 0's read decisive (2.7: a dropped row 0
   reads interaction unknown, `evidence_clipped`; an unparseable or non-`/ ` row 0
   reads interaction unknown), and rule 9, which found a selected row whose title
   is unproven (2.6). any other rule whose lookup meets a row that was not parsed
   fails, so it falls through. rule 10 comes last and so preempts nothing: it
   reads `evidence_clipped` when locating `F`, `B` or `C` meets a dropped row, and
   does not match when it meets an unparseable one (2.1).
5. every surface except the composer surface and the async editor hides the
   status row, composer and status line: activity unknown, nothing carried
   forward.
6. diagnostics, in this order: a matched overlay, viewer, setup or picker emits
   its one rule (the async editor adds `codex.activity.status_row`), and a final
   partial match of rule 6 or 9 emits none. the composer surface emits
   `codex.scope.side`, `codex.composer.external_editor`, the interaction rule,
   `codex.activity.status_row`, the activity cause (`codex.activity.disconnected`,
   `codex.activity.goal_active` or `codex.activity.prompt_pending`), then the
   run-state rule, each when it applies: at most six, under
   [spec §5](terminal-observation.md#5-controls-and-diagnostic-api)'s cap of
   eight. no rule names a clipped read; the reason carries it (claude names one,
   `claude.region.clipped`; the difference is accepted).

shared anatomy:

- **selected row**: col 0 `›` bold and not dim, col 2 bold, and the glyph and
  col 2 agree on reverse. selection_style is bold+reverse over the whole row when
  the background is unknown, and bold with a highlight fg+bg and no reverse when
  it is known (`style/contrast.rs:35-78`).
- **scroll indicator row**: its only non-space cell is `↑` or `↓` at col 0.
- **option scan** from a start row upward: skip blank rows, indented rows and
  scroll indicator rows. the first other row `R` must be a selected row; a row
  that was not parsed ends the scan with no picker. options, descriptions,
  disabled rows, notes, the search row and key-hint footers are all indented, so
  numbering and continuation shape are tolerated, never required. on the
  composer surface the scan meets `C` first, which is never a selected row (2.1).
- **key-hint footer**: indented; col 2 starts `K`; the next run is dim and starts
  with a space; every later run is `K` or dim (` · ` joins pairs). composer hint
  rows fail because their labels are plain or coloured, never dim.
- **title lookup** from `R−1` upward: skip blank and scroll indicator rows. a
  col-0 row ends the scan with no title. the first indented row whose col 2 is
  bold and not dim is the title. reaching pane row 0 ends the scan with no
  title; a row that was not parsed leaves the title unproven (2.6).

### 2.1 composer surface

anchoring:

- `F` (footer) = the maximal run of non-blank rows ending at `E`, at most 3 rows;
  a longer run is no match. three rows occur only with the transcript find
  footer (a query row and a status row).
- `B` = the row above `F`: blank (the composer's bottom padding).
- scan upward from `B−1` over blank or indented rows to the first col-0 row `C`.
  none → no match.
- a dropped row where `B` or `C` should be → unknown/unknown, `evidence_clipped`
  (2.0 step 4). an unparseable row there is neither blank nor a composer row: no
  match.
- `C` test, row-local:
  - the glyph at col 0 is not reverse and is one of:
    - enabled: `›` or `»` bold, not dim, any fg (max, ultra and reserve recolour
      it); or `!` bold (shell mode);
    - disabled: `›` dim, not bold;
    - dimmed: `›`, `»` or `!` bold+dim, with every non-space cell at col ≥ 1 of
      `C` and of its continuation rows dim.
  - no cell at col ≥ 1 of `C` is bold unless it is also reverse.
  - otherwise no match.
- the glyph's background is not checked. with a known background the composer
  band is filled (`chat_composer.rs:4925-4926`) and the glyph keeps the band fill
  (`:4935-4959`).
- the test separates `C` from the rows it must reject:
  - a picker's selected row fails: reverse glyph, or bold label cells.
  - a historical user row fails: bold+dim `› ` followed by non-dim text
    (`history_cell/messages.rs:229-237`).
  - history-search and vim-search highlights (bold+reverse,
    `chat_composer.rs:4975-4981`) pass.
  - drafts never carry bold: elements are cyan, plugin mentions magenta,
    selection reverse (`bottom_pane/textarea.rs:2208-2253`).

`SL` and the hint row:

- `F` with 2 or 3 rows: `SL = F[0]`, and the hint row is `F[−1]`.
- `F` with 1 row: that row is `SL` when its first item is a run-state word (§3);
  otherwise it is the hint row and there is no `SL`.
- the known layouts: fullscreen `SL` + hint; `SL` with a blank hint row; hint
  only without a status line; the inline one-row footer; the find footer.

band and *clean band*. the band is the top padding row `C−1`, `C` and its
continuation rows, and `B`; with remote image rows it also holds those rows
(`"  "{cyan}"[Image #N]"`) and one more padding row above them (`C−1` stays
blank). the band is *clean* when its only non-space cells are the glyph and one
dim run starting at col 2 of `C`: `C−1`, `B`, col 1 of `C`, every cell after that
run, continuation rows and image rows are all blank or absent. sparkle paints the
band outside the placeholder span (`chat_composer/sparkle.rs:401-417`) and
ignition paints padding rows (`bottom_pane/effort_ignition_styles.rs:68-86`), so
both leave it unclean. a glyph in `B` breaks the anchoring itself (no match).

composer gap row (fullscreen only). the row directly above the top padding
(`C−2`) is reserved for the composer gap (`bottom_pane/composer_gap.rs`,
`app/owned_transcript.rs:237-254`). it is blank, or holds one of
(`transcript_view/composer_gap.rs:70-114`):

- a right-aligned usage notice `⚠ <window> limit: … left`, warning colour, bold at
  ≥ 90 % used (`chatwidget/usage_notice.rs:66-69`);
- right-aligned copy feedback, accent or red;
- the centred follow control `↓ Back to bottom …` (accent on the user-message
  style, bold+reverse while hovered), shown only while the transcript tail is
  off-screen (`transcript_view/follow_control.rs:52-81`). submitting snaps the
  transcript to its tail in the same frame.

the gap row is never activity or request evidence. the transcript scan (2.2)
passes over it like any other non-stop row; nothing in it can imitate a stop
(none starts at col 0 or carries an all-dim col-2 label).

bottom-pane elements between the transcript tail and the band
(`bottom_pane/mod.rs:2177-2290`, top to bottom: inline banner, status row, hook
row, standalone background row, working tip, other-thread preview, queued/steer
preview, collapsed questions, gap row):

| element | anchor | effect |
| --- | --- | --- |
| status row | 2.2 | activity |
| other-thread approval | `"  "{b}{red}"!"{/}" Approval needed in "…`, next row `{d}"    "{bd}{cyan}"/subagents"{d}" to switch threads"` | permission (`codex.permission.other_thread`) |
| collapsed async questions | 2.5 | question |
| inline banner | option block + indented dim `Press a number to choose…` | confirmation (`codex.confirmation.inline_banner`) |
| information banner | indented dim `esc to dismiss · type to continue`, no `Press a number` | interaction unknown |
| previews, standalone background row, hook row, tips, gap row | none needed | none; never activity evidence |

these elements are searched between the transcript scan's stop and the band.

the dimmed composer is the interactive-transcript-footer signature. every footer
with `is_interactive` dims the whole composer rect (`chat_composer.rs:5041-5043`):
browse (`Browsing…`), find (`Find:` + status), selection and disclosure. the
result is interaction menu (`codex.menu.transcript_footer`) and composer blocked.
the footer text is tolerated and never read.

interaction precedence on this surface: permission (other-thread) > question
(collapsed) > confirmation (inline banner) > menu (dimmed composer) > unknown
(information banner) > none (`codex.interaction.none`: the complete surface was
located and carries no request or menu element).

scope: the placeholders are constants (`chatwidget.rs:2041-2042`):
`Ask Codex to do anything` (main) and `Ask a follow-up question` (side). the side
placeholder makes the composer blocked, because the ordinary composer is not
displayed, and adds `codex.scope.side`. idle requires the main placeholder (§3
step 4.1).

### 2.2 the transcript scan and the status row

one upward scan serves both jobs. it starts at the row above the band's top
padding (the gap row in fullscreen; above the image rows' padding when they
exist), moves upward with continuation (2.0 step 2), and its first *stop*
classifies the transcript's end:

- **prompt** → *open*:
  - a user row: col 0 `›` bold+dim and not reverse (red+bold for spoken
    prompts), col 1 a space (`history_cell/messages.rs:229-237`). the sticky
    prompt header at transcript row 0 renders the same way
    (`transcript_view/prompt_header.rs:47-60`) and counts as a prompt.
  - an image-label row: cols 0–1 spaces, text from col 2 one or more
    `[Image #N]` labels, every non-space cell coloured and not dim. an image-only
    prompt renders only these rows (`messages.rs:204-223`).
- **terminator** → *settled*:
  - a completion separator: cols 0–1 spaces, col 2 non-space, every non-space
    cell dim, text from col 2 starting `Worked for ` or a clock time (`H:MM`,
    optionally ` AM`/` PM`, or a `Mon D[, YYYY] at ` date prefix). every live
    completed turn ends with one (`chatwidget/completion.rs:7-52`,
    `chatwidget/protocol.rs:461-466`, `history_cell/separators.rs:59-114`). a
    completion tip occupies a spacer row above it
    (`transcript_view/layout.rs:238-242`), so the separator stays the first stop.
  - a notice cell: `■` at col 0 (interruption, goal budget, error;
    `chatwidget/input_restore.rs:319-336`).
  - the session header: cols 0–1 spaces, text from col 2 starting `>_ ` and bold
    `OpenAI Codex` at col 5.
- the scan reaching pane row 0 or a dropped row without a stop → *unproven*.
- the scan meeting an unparseable row → *unreadable*: the row could be the stop.

every other row is passed over: transcript content, previews, edit hints, hook
and background rows, tips, banners and the gap row. no row is skipped by its
text, so truncation at narrow widths cannot hide a stop. bottom-pane rows sit
below the transcript and none renders a stop shape, so the first stop is the
transcript's last prompt or terminator.

costs of requiring a visible stop: `Ready` reads unknown when the transcript end
has no terminator: replayed turns without saved timestamps (`completion.rs:7-37`
builds no separator then), a failed turn with no error message
(`protocol.rs:504-508`), a side-view interrupt (its notice is suppressed,
`app/side.rs:266`), a browse viewport parked with an older prompt nearest the
band, and a scrolled transcript whose terminator is off-screen.

the status row is the first row between the stop and the band (or the scan's
end) that matches:

```
row      := H " " P [T]                ; H: a non-space cell at col 0, any style (glyph or header), not a dim "• "
P        := full | hintless | cut
full     := {d}"(" D " • "{/} K {d}" to interrupt)"{/}
hintless := {d}"(" D ")"{/}
cut      := {d}"(" G "…", "…" the row's last used cell, G a proper prefix of D " • " K " to interrupt)" or
            of D ")" (part of D, or D then nothing, " ", ")" or " •" …); dim through " • ", then K's bold run, then dim
T        := {d}" · " … (background-terminal summary, hook text), possibly ending "…"
```

- `H` absorbs the optional glyph: `•` bold with a truecolor fg, plain `•` or dim
  `◦` without truecolor, or no glyph under reduced motion (`motion.rs:65-78`,
  `status_indicator_widget.rs:236-240`). a dim `• ` at col 0 is a history bullet,
  the hook row or the reduced-motion static bullet (`motion.rs:45-48`), never the
  status glyph.
- `K` is structural; the dim halves delimit it (darwin `delete`/`fwd del`,
  chords, `ctrl+]`).
- detail rows below it (`{d}"  └ "…`, `{d}"    "…`, hook overflow) are optional.
- `cut`: `truncate_line_with_ellipsis_if_overflow` (`line_truncation.rs:76-99`)
  keeps `W−1` cells and appends `…` in the style of the last span it kept, so the
  cut falls after any cell: `(…`, `(5…`, `(5s …`, `(1m 05s …`, `(5s •…`,
  `(5s • es…`, `(5s)…`. a cut inside `T` reads as `full` or `hintless` with its
  dim tail.
- `full`, `hintless` and `cut` all mean working (`codex.activity.status_row`).
  hintless is the unbound-interrupt layout and also the reconnect row. the
  disconnect decision is taken from the hint row (§3 step 1), which is
  width-independent, so a truncated or hintless row needs no disambiguation.
- negatives, none of which match:
  - an agent message quoting the row: `{d}"• "` + plain text, or plain indented
    rows.
  - uniformly dim command output: the key is not undimmed.
  - `Worked for …` separators; exec cells `• Ran …`; the empty-state braille
    logo.
  - spinner glyphs or the word `Working` in prose.
  - inline-mode residue above the current session header: the scan stops at the
    header.
  - a row starting with a dim `• `: agent messages, `• Viewed image <file>`
    truncated inside a parenthesised file name (`history_cell/patches.rs:166-178`),
    the hook row `{d}"• <status message>"`, which can hold and cut a paren group
    (`hook_status.rs:19-26`).
  - a header that pushes `(` off the row: no status row at all.
  - no other codex history cell renders a dim ` (` group on a col-0 row: mcp
    invocations open with a plain `(` (`history_cell/mcp.rs:837-856`), and exec
    cells wrap instead of cutting.

### 2.3 permission (`Press` footer)

- the footer word-wraps: its first row is the nearest row at or above `E` whose
  text from col 2 starts `Press `, every row from it to `E` indented and
  non-blank; the rows join with one space, bold only between two key cells (a
  chord key `prefix completion` is one bold span and can break at its own space).
  parse it as `Press K <label>[ or K <label>]*`. keys are bold runs; bold+dim is
  allowed because user verification dims the whole line
  (`user_verification.rs:92-103`). labels are dim, or plain in the legacy
  confirm.
- a closed map on the labels:

  | labels | requires | result |
  | --- | --- | --- |
  | last label `to cancel` or `to open thread` (`to confirm or … to cancel`, `to cancel` alone, `… or K to open thread`) | an option scan from the footer's upper neighbour finds a selected row | permission (`codex.permission.overlay`) |
  | exactly `to confirm`, `to go back`, with plain `Press` | — | question (`codex.question.legacy_confirm`) |
  | anything else: `to confirm` alone (cancel key unbound), `to continue working` (reserve), `to continue` (login, rule 8) | — | falls through |

- the open-thread suffix renders dim. a single footer family covers exec, edits,
  network, write-stdin, permissions grant, mcp elicitation fallback and user
  verification (`approval_overlay.rs:639-663`, `popup_consts.rs:42-60`). titles,
  header lines and option labels are tolerated and never checked.
- result: unknown/permission/none/blocked.

details pager: see 2.7. the five approval titles map to permission.

negatives: numbered prose `  1. Yes, proceed (y)` (no selected row); a quoted
`Press enter to confirm …` that is not at `E`, or has no bold key; a stale
overlay left by a killed process, whose footer is no longer at `E` once the shell
prints.

### 2.4 legacy request_user_input and mcp forms

both footers are ` | `-joined members (the separator is dim); every member starts
with `K` except the keyless dim `option N/M` count, which is skipped. both show the
count when the options do not fit:

- the legacy view wraps it as the first hint (`request_user_input/render.rs:344-353`)
  and cuts a hint wider than its footer (`W−4`) at a word boundary with `…`
  (`render.rs:361-368`, `truncate_line_word_boundary_with_ellipsis`);
- the mcp form prefixes it to its first footer row after wrapping
  (`mcp_server_elicitation.rs:1327-1350`) and clips rows at the edge without `…`
  (`:1351-1370`), so the edge cuts whatever the count pushed past it: the last
  members (36–47 columns with one field, 67–79 with two) or the submit hint itself
  (22–31 columns with one field, 20–38 with two), while the rows below stay whole.
  a short pane also leaves its last footer row undrawn (`take(area.height)`).
- the legacy view gives its footer `min(footer_pref, remaining)` rows
  (`layout.rs:150-173`); when one row is left and the count cannot share it with
  the next hint, that row is the count alone (at h 6: 24–32 columns with notes
  hidden, 26–40 with notes shown). the mcp form never draws a lone count row (the
  count prefixes its submit row), nor does the async editor (the count follows
  `submit` and `skip`).

hints wrap whole, one row per group (`footer_hint.rs` `wrap_hint_rows`; four rows
at 40 columns), so the footer is the run of hint rows ending at `E` (indented, col
2 starting with a key or the count), and its members are every row's members. any
other keyless member is no footer. the selected option and the notes row start at
col 2 with a bold `›`, which no key label is, and sit right above the footer when
options are hidden: they are not hint rows.

the members `esc to cancel`, `to navigate fields` and `change field` (mcp) and
the notes, question-navigation and interrupt hints (legacy) each exist in one view
only, but the submit hints exist in both, and either view can lose any hint to
the edge or the footer height. so the legacy view is told first, by a hint only
it renders or by the count alone; the mcp form then by a submit hint, or by the
count leading its first row with a hint only it renders.

legacy question (rule 2): a member ` to add notes`, ` or esc to clear notes`,
` to navigate questions`, ` change question` or ` to interrupt`
(`request_user_input/mod.rs:572-626`), or a notes hint cut after its
distinguishing words: ` to add…`, ` or esc…`, ` or esc to…`, ` or esc to clear…`;
or a footer whose only member is the count. only the legacy view adds `…`, and a
question with options always draws its notes hint, its first hint, unless the
count alone fills a one-row footer (the count, when added, comes before it).

- the notes hints exist only in this view. they qualify it alone because the view
  reserves its footer height without the count (`footer_required_height` →
  `footer_tip_lines`) and draws the counted rows through
  `take(footer_area.height)` (`render.rs:354-358`): when the count adds a row,
  the last one is not drawn, and with notes shown and `esc` as the interrupt key
  (or the interrupt key unbound) that row holds the submit hint (for example
  `option 4/5 | tab or esc to clear notes` alone at 60 columns).
- tolerated, never required:
  - the submit hints, which the mcp form also renders;
  - the header `"  "{d}"Question N/M"[" (K unanswered)"]{/}[{d}" · "{/}{red}"auto-resolves in …"{/}]`;
  - the title;
  - options `  › N. label  description` (glyph at col 2);
  - the notes row `  › Add notes`.
- result: unknown/question/none/blocked.
- auto-resolve. the TUI owns the timer, and only a non-blocking request runs
  it. a stock install never shows one:
  - the model's `request_user_input` is offered only in plan mode
    (`protocol/src/config_types.rs:700-702`) unless
    `features.default_mode_request_user_input`, under development and off by
    default (`features/src/lib.rs:1620-1625`), adds default mode
    (`core/src/tools/handlers/request_user_input_spec_tests.rs:158-174`).
  - blocking requests stay until answered: a plan-mode question
    (`core/src/tools/handlers/request_user_input.rs:83`), the mcp
    dependency-install prompt (`core/src/mcp_skill_dependencies.rs:299`) and the
    mcp tool-approval prompt used when `features.tool_call_mcp_elicitation`
    (stable, on by default) is off (`core/src/mcp_tool_call.rs:1716`). the two
    mcp prompts render in this view, so by source they read question
    (`needs answer`), the approval included; that reading is accepted (not
    run).
  - with the switch on, a default-mode question is non-blocking. the view
    resolves it after a 60 s hidden grace and a 60 s visible
    `auto-resolves in …` countdown, counted from when the request is shown
    (`request_user_input/mod.rs:72-73,263-310`); a key or paste in the view stops
    the timer (`:1183`, `:1457`). resolution submits empty answers
    (`submit_empty_auto_resolution`, `:892-909`) and the model continues the turn
    on them, so an unvisited question disappears and the pane reads the
    continuation: question → idle raises no ready (spec §6), and working sampled
    in the continuation arms ready as usual.
  - observed with the switch on, on darwin with the 0.159.2 TUI embedded and
    against daemons 0.159.2 and 0.159.3: each tool output arrived 120.0 s after
    its question, and the scripted endpoint ended each continuation at once, so
    no working sample fell between the question and idle.

mcp form (rule 3): otherwise a member `K to submit`, `K to submit all` or
`K to submit answer` (`mcp_server_elicitation.rs:971-996`), or a first row led by
the count with a member `K to cancel`, `K to navigate fields` or `K change field`
somewhere in the footer (the count clipped the submit hint). a `Press` footer's
wrapped `esc to cancel` row is no mcp form: the count never leads it.

- `codex.permission.mcp_approval` → permission, when the form is single-field (no
  `to submit all`, `to submit answer`, `to navigate fields` or `change field`
  member: a clip can leave `to submit` of `to submit answer`), option rows
  (`  [› ]N. label`) are present, and every label is one of `Allow`,
  `Allow for this session`, `Always allow`, `Deny`, `Cancel` (approval-action mode:
  tool approvals and message-only elicitations, always one synthesized field). a
  two-field form whose enum offers `Allow` and `Deny` is told apart by its
  navigation member. an option block that holds a row that was not parsed (dropped
  or unparseable) fails the test, since that row could hold another option, and
  so do option rows that draw no label (≤ 21 columns).
- `codex.input.mcp_form` → input, for any other form: text, boolean, enum,
  multi-field.
- result: unknown/<permission|input>/none/blocked.

legacy confirm (`Submit with unanswered questions?`): 2.3. `h−1` is blank in the
alternate screen.

negatives: `• Which … ? / • Red / • Blue` and `answer: …` history rows;
`› > question` echo rows.

### 2.5 async questions

collapsed (`codex.question.async_collapsed`), above the band:
`{d}"  ? "{/}{b}{acc}"N question"["s"]{/}` [`{d}" · <countdown>"`], with an
optional next row `"    "K{d}" to answer"{/}`. the widget exists only while a
turn runs. result: interaction question; activity from 2.2 and §3; composer from
§4.

expanded editor (`codex.question.async_editor`): it replaces the composer and the
status line; the status row stays above it.

- required in the run of hint rows ending at `E` (it wraps to three rows at 40
  columns): hints separated by three spaces, containing `K submit` and `K skip`.
  the keyless dim `option N/M` count follows `skip` when the options do not fit and
  can lead a wrapped row (`async_questions/render.rs:166-229`); it is the one
  keyless member, as in 2.4.
- the inline flash replaces the footer, so the editor reads layout_unknown.
- result: activity working when the transcript scan from the row above the
  footer meets a status row before its stop, else unknown (`evidence_clipped` when
  the scan is unproven); question; none; blocked.

### 2.6 pickers

picker (rule 9): an option scan from `E` finds the selected row `R`. a key-hint
footer at `E` is indented and skipped; a footerless picker's selected row may be
`E` itself. the title lookup from `R` decides:

- `E` is a key-hint footer:

  | title (prefix) | value | rule id |
  | --- | --- | --- |
  | `Implement this plan?` | confirmation | `codex.confirmation.plan` |
  | `Approaching rate limits`, `Resume paused goal?`, `Usage limit reached`, `You've reached your workspace credit limit` | confirmation | `codex.confirmation.provider_picker` |
  | `Folder access` | setup | `codex.setup.trust` |
  | `Update available` | setup | `codex.setup.update` |
  | `Codex just got an upgrade` | setup | `codex.setup.migration` |
  | `Hooks need review` | setup | `codex.setup.hooks_review` |
  | any other title, or no title | menu | `codex.menu.picker` |

- `E` is not a key-hint footer (footerless): the title
  `Background server has incompatible feature settings` or
  `Cannot use the background server` → setup (`codex.setup.daemon_recovery`;
  `daemon_recovery.rs:115-176`). any other title falls through; the composer rule
  then rejects the selected row, so precaution, safety buffering,
  `Usage limit resets` progress and reserve (whose `to continue working` footer
  fell through rule 1) read layout_unknown, never menu or a draft.
- an unproven title, footered or not: interaction unknown, no rule id; the
  missing row could have decided setup or confirmation. a dropped row makes the
  reason `evidence_clipped`; an unparseable one leaves it unrecognized. the
  composer is blocked when `E` is a key-hint footer (a picker is certainly
  displayed) and unknown when footerless.

results: unknown/<value>/none/blocked.

- the nearest bold title wins. an unlisted picker title therefore masks a listed
  phrase in the transcript above it.
- footer labels vary (`select · back`, `default · s session · back`,
  `continue · quit`, `continue · skip`, `K/K confirm · K quit`, `confirm · skip`).
  the structure anchors the picker, not the vocabulary.
- searchable pickers drop the numbers (`list_selection_view.rs:670-674`) and add a
  search row and `↑`/`↓` indicators at col 0.

completeness. these are every production path that opens a bottom-pane view or
overlay without user navigation:

- approval requests, including delayed and cross-thread ones
  (`bottom_pane/mod.rs:784,1864`, `app/thread_routing.rs:1434-1470`);
- request_user_input (`:1895`); mcp elicitation form, fallback approval and url
  app link (`chatwidget/tool_requests.rs:335-380`); user verification;
- inline banners;
- `show_selection_view` pickers: plan, rate limits, paused goal, usage and
  workspace banners, reserve, precaution, safety buffering;
- reconnect's restoration of a user-opened agents overview
  (`app/reconnect.rs:459`);
- pre-session screens: trust, update, login, migration, hooks review, daemon
  recovery, cwd, unarchive and oss prompts.

direct `ListSelectionView::new` constructions outside `show_selection_view` are
all covered by that list: `approval_overlay.rs:198,244`, `user_verification.rs:111`,
`actionable_banner.rs:189`, `daemon_recovery.rs:142`, `startup_hooks_review.rs:213`.
`keymap_setup.rs:744,1381` are test helpers. every other call site answers user
navigation, so menu is correct. titles supplied by the server exist only for
reserve (unknown) and inline banners (hint-anchored).

negative: plan text in the transcript (`• Proposed Plan`, `## …`) is not a
decision.

### 2.7 viewers

pager (rule 6):

- `E` = `{d}" "{/}K{d}" close"…`; `E−1` = `" "K{d}" to scroll · "…`; optional rows
  above: `{b}"Ctrl+Space"{/}{d}" select"` and a dim `─…─ p% ─` rule.
- row 0 of the pane is the dim header `"/ " <title>` over a `/ / /` fill
  (`pager_overlay.rs:206-212`). it is in the bottom region when h ≤ 64 (region
  bottom) and in the top region otherwise (region `compound`). the top region
  keeps row 0 even when clipped. a clipped bottom region at h ≤ 64 drops it:
  interaction unknown, `evidence_clipped`. an unparseable row 0 is no title:
  interaction unknown, not clipped.
- closed title map:
  - `E X E C`, `P A T C H`, `P E R M I S S I O N S`, `E L I C I T A T I O N`,
    `U S E R  V E R I F I C A T I O N` → permission
    (`codex.permission.details_pager`);
  - `What we detected` → confirmation (`codex.confirmation.details_pager`);
  - any other `/ ` title → menu (`codex.menu.pager`);
  - row 0 present but not a `/ ` header → interaction unknown.
- the pager enters the alternate screen even from inline mode.

resume (`codex.menu.resume`):

- `E−2` (or `E−1`) is a dim `─` rule with ` n / m · p% ` near its right end.
- `E−1..E` are footer rows of `K label` hints separated by three spaces; the first
  label is `resume`, `fork` or `restore`.

warnings (`codex.menu.warnings`): header `"  "{b}"Warnings · i of n · <source>"`,
`E` = `"  "K" keep & next · "K" dismiss & close · "…`.

result for resume, warnings and menu pagers: unknown/menu/none/blocked.

### 2.8 setup

- trust, update, migration, hooks review and daemon recovery are pickers (2.6).
- login (`codex.setup.login`, rule 8): `E` = `{d}"  Press "{/}K{d}" to continue"{/}`
  over options marked `{d}{cyan}"> 1. "`. `"  Welcome to "{b}"Codex"…` is
  tolerated.
- result: unknown/setup/none/blocked.
- tall panes: a pre-session screen is top-anchored. for 65 ≤ h ≤ 256 it lies in
  the top region (region `top`) when it fits rows `0..h−65`, and otherwise
  straddles both regions and resolves through continuation (region `compound`).
  h > 256 with a blank bottom region: unknown/unknown, `evidence_clipped`.
- negative: the history box `╭…╮ │ ✨ Update available! A -> B │ …` left after
  skipping the update is not setup.

### 2.9 notices

codex has no banner-style current-notice structure. interruption
(`■ Conversation interrupted - use /feedback if something went wrong`) and error
(red `■ …` from about 240 `add_error_message` sites, most of them UI validation)
are transcript cells that persist until the next turn. the grammar emits
**notice none** (spec §2: "if no current notice structure can be qualified, emit
none"). history cells never set a value; a `■` cell terminates the transcript
scan and so permits idle. §6 c5 holds the rejected alternative.

## 3. run-state status line

source `chatwidget/status_surfaces.rs:991-1012`:

| rendered | condition | activity |
| --- | --- | --- |
| `Starting` | mcp startup in progress (wins over everything) | starting (working when a status row is present) |
| `Ready` | no running turn, review or mcp startup, as of the last surface refresh | idle, when qualified below |
| `Working` | turn running, status kind working | working |
| `Thinking` | turn running, reasoning-summary header (`streaming.rs:54,341`) or stream-error retry (`:396`) | working |
| `Waiting` | turn running, polling a background terminal | working |

qualification: `SL` (2.1), cols 0–1 spaces, and the first ` · `-separated item
from col 2 equals one of the five words exactly. the item also ends at a double
space: with a single left item a right-aligned indicator follows after a gap
(plan mode's `Plan mode`). a truncated first item and the Max/Ultra transition are
no cue. colour is optional. only the first position counts; the `status` alias
renders identically.

goal indicator: a magenta (`38;5;5`) run on `SL` reading `Pursuing goal` or
`Pursuing goal (…)`, right-aligned; ` · IDE context` can follow it
(`bottom_pane/footer.rs:579-590,618-635`, drawn by
`chat_composer/status_surface.rs:31-55`). it renders whenever it fits the row,
and the left items are truncated first: `Worki…` at 30 columns and no run-state
word at 24, which is no cue. other goal states (`Goal achieved (…)`,
`Goal paused …`, …) are not the indicator. in plan mode the collaboration-mode
indicator replaces it (`footer.rs:619-620`; §6 c4).

refresh: the word is recomputed only by `refresh_status_surfaces` /
`refresh_status_line`. those run at turn start and end, mcp startup, settings
changes and some usage updates. status-kind changes refresh it only when the
terminal title uses run-state (`status_controls.rs:52-68`). so `Working`,
`Thinking` and `Waiting` lag one another and all mean working. `Ready` can be
stale while work starts:

1. `/compact`, review entry (two paths) and the mcp pending-turn set the running
   flag without a refresh. each also shows a hinted status row, so `Ready` plus a
   row is a conflict.
2. the submit window: `submit_user_message` renders the user prompt cell before
   the op (`input_submission.rs:473-480`, production AppEvent target
   `chatwidget/constructor.rs:7`). the running flag, status row and `Working`
   arrive only with `on_task_started` (`turn_runtime.rs:77-100`). measured on
   darwin: the prompt is first seen 11–34 ms after Enter and `Working` by
   21–84 ms; 4.4 ms sampling caught 2–3 samples per window. queued follow-ups
   submit one at a time through the same path (`input_flow.rs:234-262`); the
   remaining queue stays in the preview between the new prompt and the band.
3. goal continuation, server-initiated. after each goal turn completes, core
   emits thread-idle (`core/src/tasks/mod.rs:811-883`) and the goal extension
   starts the next turn (`ext/goal/src/extension.rs:180-191`,
   `runtime.rs:425-497`) with a model-only input and no prompt cell. measured on
   darwin: `Ready`, no status row and a settled scan for ≤ ~10 ms at 10–25 % of
   turn boundaries, with `Pursuing goal` on `SL` in every such sample. the TUI
   itself withholds its completion notification while the goal is active
   (`turn_runtime.rs:205-218`).
4. daemon-recovery continuation
   (`app-server/src/request_processors/daemon_continuation.rs`): a restarted
   daemon starts one continuation turn while restoring a thread, before any client
   attaches (`thread_processor.rs:3990-4005`). a reconnecting TUI normally resumes
   a thread whose turn already runs. a drained restart finished the turn, and a
   forced restart left the interruption notice and `Ready` with no continuation
   within 30 s, so the window was not induced (§6 c4).
5. `!cmd` and queued slash commands render no prompt cell: a ≤ ~45 ms window with
   no cue (§6 c4).

activity decision on the composer surface:

1. hint-row overrides, read first:
   - disconnect: the hint row is indented, `K` then ` quit` (plain or secondary
     colour, never dim), then nothing or the right-aligned context
     (`reconnect.rs:35`, cleared on reconnect; `render_context_right`). →
     unknown (`codex.activity.disconnected`); `evidence_conflict` when a status
     row or run-state word is present. only a key-chord continuation (§6 c10) could
     render `K quit`.
   - external editor: the hint row is one `K` run reading
     `Save and close external editor to continue.`, optionally followed by the
     right-aligned context (a constant, `app.rs:293`; set by `app/input.rs:300`;
     the screen is kept, `tui.rs:333-336`). → composer blocked
     (`codex.composer.external_editor`); `Ready` gives no idle (step 4.2); working
     evidence still stands. keys typed into the pane while the editor is open
     were seen discarded when it returned.
   - the remaining override, `Waiting for startup · esc cancel`
     (`startup_draft_input.rs:112`), and the empty overrides of question and
     elicitation views change nothing here.
2. a status row → with `SL` `Ready`: unknown (`evidence_conflict`); otherwise
   working.
3. no status row: `SL` `Working` / `Thinking` / `Waiting` → working; `Starting` →
   starting.
4. no status row, `SL` `Ready` → idle only when all of these hold, checked in this
   order:
   1. the band test: the glyph is enabled or dimmed, the band is clean, and its
      dim run is the main placeholder or a dim proper prefix of it (≥ 5 cells,
      ending within 3 cells of the right edge). otherwise → unknown; the side
      placeholder adds `codex.scope.side`. the composer value (§4) is separate: a
      dimmed composer reads blocked and still permits idle, because `SL` stays
      visible under the interactive footers.
   2. the hint row is not the external-editor override. otherwise → unknown.
   3. `SL` carries no goal indicator. otherwise → unknown, `evidence_conflict`,
      `codex.activity.goal_active`.
   4. the transcript scan settled. *open* → unknown, `evidence_conflict`,
      `codex.activity.prompt_pending`. *unproven* → unknown, `evidence_clipped`.
      *unreadable* → unknown, no clipping.
5. no run-state word → unknown; `evidence_clipped` when the scan is unproven.

the async editor (2.5) takes only the status row from this section.

what idle means: the provider claims that no turn, review or mcp startup runs in
the displayed thread and no goal is being pursued. the main placeholder shows, the
ordinary composer has focus, and the transcript ends in a terminator. idle does
not mean:

- background terminals are gone (§6 c9);
- other threads are idle (§6 c2);
- a default footer is idle (no run-state → never idle).

during final-answer streaming the status row is hidden while `SL` still says
`Working`.

## 4. composer semantics

evaluated in this order on a located composer surface; the first match wins:

1. **blocked**: a disabled glyph (dim `›`, not bold: `Input disabled.`,
   `Answer the questions to continue.`, …); a dimmed composer; the side
   placeholder on a clean band; the external-editor override. every non-composer
   surface that matched a rule is also blocked, as is a footered picker with an
   unproven title.
2. **draft**: remote image rows, or the shell glyph `!`.
3. **unknown**: a placeholder run with any other non-space cell in the band
   (sparkle, ignition).
4. **draft**: a non-dim non-space cell at col ≥ 2 on `C` or a continuation row
   (history-search matches included; slash popups sit over a draft).
5. **empty**: an enabled glyph and a clean band showing the main placeholder.
6. **unknown**: everything else: dim non-placeholder text under an enabled glyph
   (the parent-owned sub-agent placeholder
   `Viewing sub-agent — direct input is disabled` keeps input enabled and the
   glyph bold, `chat_composer.rs:1652-1659,4933-4952`); `C` with no text.

no rule matched → unknown.

a user draft that literally reads `Ask Codex to do anything` is not dim, so it is
a draft. under the dimmed composer it is dim and would pass as the placeholder;
the composer is blocked there anyway. queued messages during work leave the
composer empty. codex emits no rule id for the composer's value (empty, draft,
blocked); `codex.composer.external_editor` names the hint-row override. the
composer reaches only guarded send
([spec §5](terminal-observation.md#5-controls-and-diagnostic-api)).

## 5. configuration knobs

| knob | effect on the grammar |
| --- | --- |
| `tui.animations=false`, `tui.effects.progress=false` / `.shimmer=false` | no glyph / plain header; also the default once screen-reader detection fires |
| no 16m colour | glyph blinks plain `•` / dim `◦` |
| `tui.effects.starfield`, Max/Ultra effort | sparkle paints the band outside the placeholder; ignition paints padding rows (composer unknown or layout_unknown) |
| `tui.keymap.chat.interrupt_turn` | status-hint key label; `[]` → hintless row, still working |
| `tui.keymap.list.accept` / `.cancel` | `Press` footer keys; unbound cancel → approval layout_unknown; unbound accept → `Press K to cancel` still permission |
| `tui.keymap.approval.*` | option shortcut suffixes, `open_thread`, `open_fullscreen` (ctrl+a → details pager) |
| `tui.keymap.global.find_transcript` (F3), `.focus_activity` (F4) | open the interactive find / disclosure footers (dimmed composer) |
| `tui.status_line` | managed `["run-state","model-with-reasoning","current-dir","thread-name"]`; unset → default items, no run-state; `[]` → no `SL` |
| `tui.status_line_use_colors=false` | `SL` uncoloured (the goal indicator keeps magenta) |
| `tui.terminal_title` containing `run-state` | status-kind changes refresh `SL`; managed titles are untouched, so the word may lag |
| `tui.fullscreen_transcript=false`, `tui.alternate_screen=never`, `--no-alt-screen` | inline layout (`alt=0`, no gap row); pagers still switch to alt |
| `tui.vim_mode_default` | `Vim: …` on the hint row; composer rules unchanged |
| `tui.show_tooltips` | completion and working tips (ordinary transcript rows) |
| `features.goals` (stable, on by default; `features/src/lib.rs:1686-1691`) | `/goal` and automatic goal continuation; the `Pursuing goal` indicator |
| `features.default_mode_request_user_input` (under development, off by default; `features/src/lib.rs:1620-1625`) | offers `request_user_input` in default mode as well as plan mode; a default-mode question is non-blocking, so its header adds `auto-resolves in …` and it resolves itself (2.4) |
| `$VISUAL` / `$EDITOR`, ctrl+g | external editor override on the hint row |
| terminal background known (OSC 10/11) | band and panel fills, highlight-bg selected rows, coloured hint labels; the `C` test ignores backgrounds |
| `-c` keys outside the daemon allowlist on a local launch | force embedded mode and add `⚠ 1 warning` to the hint row (`daemon_startup.rs:56-104`); the managed `--remote` launch connects to the account daemon with the same `-c` and shows no warning |
| `-c features.<shared server feature>` differing from a running daemon | daemon recovery picker |
| `check_for_update_on_startup` + `$CODEX_HOME/version.json` | update prompt when an install action exists |

## 6. accepted costs

accepted costs ([spec §9](terminal-observation.md#9-final-state-costs-and-completion)),
each unknown or none, never a false claim unless the item says otherwise. c4 and
c11–c14 claim falsely and c2 admits guarded send into a sub-agent, each under
the conditions it names; spec §9 lists them:

- **c1** idle needs `Ready`, the main placeholder on a clean band, no external editor
  and a visible transcript terminator (§3 step 4, 2.2). no idle while a draft,
  image or history-search match sits in the composer; a turn that ends with a
  pre-typed draft produces no ready notice; `skid wait --state idle` does not
  match until the draft is sent or cleared or the editor closes. a scrolled or
  replayed transcript without a visible terminator reads unknown.
- **c2** displayed-thread scope. the classifier describes the thread the TUI displays,
  and every claim is about that thread. a v1 sub-agent view has the enabled
  composer and the main placeholder, so it reads like the main thread: its idle
  and empty composer are the sub-agent's, and guarded send types into the
  sub-agent. background sub-agents behind a main `Ready` read idle. the only
  discriminator is text: with more than one thread the passive status line
  appends an agent label, `Main [role]` or a nickname
  (`app/agent_navigation.rs:327-352`, `footer.rs:838-847`), truncated first at
  narrow widths; keying on it would be a width-dependent text rule.
- **c3** `Pursuing goal` withholds idle (unknown, `evidence_conflict`), even when no
  continuation will run: after a fork the continuation is deferred until the next
  turn (`state/src/runtime/goals.rs:68-120`), and a thread without goal tools
  never continues.
- **c4** cue-less turn starts read idle for their duration, a false idle, with no
  two-sample rule: `!cmd` and queued slash commands (≤ ~45 ms), the
  daemon-recovery continuation if a reconnecting TUI resumes before the restored
  turn starts (not induced; its window is unmeasured), and a goal pursued while
  plan mode's indicator replaces the goal indicator (not run; with the indicator
  the goal continuation's window measured ≤ ~10 ms, §3 refresh item 3). the
  acceptance covers the two unmeasured windows as it does the measured ones.
  a 5 s poll lands in a ≤ 45 ms window with probability under 1 %; `ready` can fire
  when a queued shell command follows a turn, and `skid wait --state idle` right
  after `!cmd` can return early. right after a guarded send the visible draft reads
  unknown until Enter is processed, so it cannot return early there.
- **c5** codex emits no notices (2.9). the rejected alternative read a `■` cell at the
  transcript tail as `interrupted` or `error`; trivial UI validation errors would
  trigger it, it would persist until the next turn, CLEARING would suppress ready,
  and guarded send would refuse a retry after an error.
- **c6** the managed launch replaces the launch's status-line layout; a local launch
  with the same `-c` runs embedded (§5).
- **c7** tall panes: above 256 rows, rows `192..h−65` are not captured, so `Ready` reads
  unknown, `evidence_clipped`, until the transcript tail passes row `h−64`, and a
  pre-session screen near the top reads unknown/unknown, `evidence_clipped`.
- **c8** npm's node launcher leads the foreground group, so a codex started through it
  is a generic terminal; managed launches and `codex` typed in a skid shell run
  the native executable ([deployment schema](dev-server-handoff.md#host-config-and-validator)).
- **c9** `Ready` with surviving background terminals reads idle. that is no false
  claim: spec §2 makes idle the provider's ready-state evidence and a surviving
  process or task count insufficient for working, and §3 says idle does not mean
  background terminals are gone. it is a scope limit, like c2's background
  sub-agents.
- **c10** the disconnect override `K quit` can be imitated only by a
  user-configured key chord whose pending continuation is labelled `quit`
  (`app/input.rs:171-181`). the result is unknown.
- **c11** a wrong picker kind, a false claim. the title lookup takes the nearest
  bold indented row (2.6). every production `SelectionViewParams` construction
  sets a title or header; a picker whose header has no bold row, drawn under a
  transcript row that starts with a listed title, would read that title's value.
- **c12** an unmanaged status line whose first item is a user value equal to a
  run-state word reads as that word's cue, a false claim, idle included. the
  managed profile cannot produce it.
- **c13** stale chrome, a false request. after the provider is killed, the pane
  keeps its last screen (and the alternate screen) until the relaunched provider
  draws. live samples read layout_unknown before the first draw (0.32–0.42 s),
  because the shell's job message, prompt and typed command land at the old
  cursor and break the bottom structure. a shell that prints nothing (not
  induced), with the old cursor above an intact request overlay, would leave the
  stale footer at `E` and read the old request for that window, so the row can
  enter the needs-input filter and `skid wait --state needs-input` could return
  early. the ready machine cannot fire across a foreground change.
- **c14** a status row whose header pushes `(` past the right edge is no status
  row (seen live). the stale-`Ready` paths of §3 refresh item 1 show such a row
  with `Ready` still on `SL`, so in that millisecond window (from source) a pane
  at most header + 4 columns wide (≤ 22 for `• Compacting context`) reads idle
  instead of `evidence_conflict`: a false idle, which can raise ready when
  `/compact` or a review starts within one poll of a turn whose working was
  sampled. the narrower repair, withholding idle when a col-0 row between the
  settled stop and the band ends in `…` and does not start with a dim `• `, would
  cost idle after every long `!cmd` cell on narrow panes until the next turn's
  separator.
- **c15** narrow and short question footers, measured live with options hidden.
  every case reads layout_unknown or a request of another subtype, never none or
  idle:
  - legacy view at 15 columns or fewer: the notes hint is cut to `tab to…` or
    `tab or…`, which names no word only that view renders: layout_unknown.
    accepting a bare ` to…` or ` or…` would let any cut hint row read a question.
  - legacy view in a pane too short for any footer row (h 5 at 30–100 columns;
    at h 6 and ≤ 22 columns the wrapped question takes the row): layout_unknown.
  - mcp approval at exactly 35 columns: the one-row footer ends
    `… | enter to submit |`, so no member is the submit hint: layout_unknown.
  - mcp approval at 18–21 columns draws no option labels and reads input; at 17
    columns and fewer the first row is a dangling `option 1/2 |` or the cut count:
    layout_unknown.
  - a two-field mcp form at 25 columns or fewer cuts `to navigate fields`, so an
    enum offering only `Allow` and `Deny` reads permission instead of input.
  - an mcp form narrow enough to draw the count alone on a one-row footer would
    read a legacy question (not observed: 14 and 15 columns drew a second row).

## 7. requalification

run this after a codex upgrade, before claiming the new version, with the
current-turn approval [spec §8](terminal-observation.md#8-red--green--refactor-acceptance)
requires for isolated tmux and live runs. it reuses no retained harness; every
probe is temporary and deleted before commit ([testing](rules/testing.md)), and
every record is content-free.

1. read the upstream diff of the files §1's basis column names, and of anything
   new under `bottom_pane/`, `chatwidget/` and `transcript_view/`. mark each row
   whose rendering or trigger changed.
2. drive the installed TUI against a scripted local Responses endpoint: a custom
   `model_provider` with `wire_api="responses"`, SSE events shaped like upstream's
   test responses, `connection: close`, retries 0, a one-model catalog, a trusted
   scratch git cwd and a temporary `CODEX_HOME`. script holds (a turn held until a
   release), `request_user_input` (a stock session offers it only in plan mode:
   switch to plan mode, or enable `features.default_mode_request_user_input` for
   the non-blocking default-mode view), an escalated `exec_command` under
   `-a on-request -s read-only`, goal turns (a continuation-marked input answered
   for N turns, then `update_goal {status: complete}`), and rate-limit headers for
   the usage notice and the rate-limit picker. a closed local port produces the
   retry row; a dummy stdio mcp server produces approvals, forms and `Starting`.
3. qualify the production pairing: the managed argv against a daemon running the
   installed app-server package, not only embedded. a fresh `CODEX_HOME` installs
   the cli itself as its daemon package, so copy the installed package in. keep
   socket paths under darwin's 104-byte limit or the daemon updater fails.
4. run every pane on an isolated tmux server (`tmux -L <own socket> -f /dev/null`,
   with `TMUX`/`TMUX_PANE` unset in the harness's environment so no command
   reaches the user's server, and `HOME` and `HISTFILE` pointed away from the user),
   on each platform and tmux version in use, at the default width and at the
   narrow widths §6 names. set `HOME` and `CODEX_HOME` inside the probe directory
   on every codex invocation, `--version` and `--help` included: codex writes
   `codex-arg0*` helper directories under `$CODEX_HOME`, which defaults to the
   user's `~/.codex`. classify through the real capture and `detect`;
   expectations come from the scripted step and the keys sent, never from skid's
   output.
5. for families no endpoint can produce, author frames from source and paint them
   through real tmux: that proves parser mechanics only, so the family stays
   `NOT_RUN` for provider support. pair each new rule with a negative and confirm
   a mutation of the rule flips it.
6. clean up: stop probe daemons and remove the socket and lock named by the
   sha256 of each probe's control-socket path from the shared daemon socket
   directory (`/tmp/codex-daemon-<uid>/`), touching no other entry. if a codex
   call slipped past the probe home, remove only the `codex-arg0*` directories
   that call created (its birth time and link targets attribute them).
7. update §1's qualified column and versions, record the run in the
   [qualification](terminal-agent-control-qualification.md#terminal-observation-qualification),
   and file each new `NOT_RUN` required family as an issue.
