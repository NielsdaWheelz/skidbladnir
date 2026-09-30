# terminal status heuristics miss supported provider screens

problem: ordinary status can report unknown, unavailable, or idle during work.
the current detector observes too little terminal structure, misses supported
provider variants, and treats some persistent composer chrome as idle evidence.

impact: cards and terminal waits become unreliable; false working-to-idle
transitions can produce misleading ready attention. guarded send can also refuse
usable screens. status evidence and composer-safety evidence need different
acceptance thresholds even when they share parsing.

scope: research on 2026-09-30, source `564d32f`; release pin is v0.10.7 at
`240141b`. this records defects and research, not installed-fleet reproduction.
the accepted [implementation plan](../terminal-observation.md) owns the future
contract; it is not implemented. the user clarified that the
investigation concerns non-native signals. native integration is not the proposed
repair here. no production code, provider settings, sessions or deployment changed.

evidence:

- `internal/agentcontrol/service.go:72` samples terminal status for ordinary
  inventory. native status remains separate. git `67af37f` removed the old
  detector; `92ee4a8` restored heuristics; `98fe674` and `240141b` hardened them.
  the old detector at `c164ded` was already a narrow eight-line parser, not a
  complete permission/question/outcome implementation. restoration added useful
  composer safeguards but did not establish complete status coverage.
- `internal/tmux/control.go:45` captures the whole styled visible screen into
  an 8192-byte retained tail. `internal/agentcontrol/terminal.go:30` rejects any
  truncated capture before `detect.go:47` selects eight joined lines. unrelated
  upper-screen volume can therefore suppress intact lower-screen status evidence.
  ignoring truncation blindly would also be wrong: retained rows or styling may
  be incomplete. capture must bound the intended evidence region explicitly.
- `internal/agentcontrol/detect.go:14` requires a leading activity glyph, literal
  escape interruption hint, and a closing parenthesis at the end. stock codex
  0.159.2 supports reduced motion, remapped hints, suffixes and wrapped detail
  rows. adjacency checks at lines 124-135 can reject legitimate detail rows.
  [versioned upstream renderer](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/status_indicator_widget.rs).
- `detect.go:157` still maps prompt plus recognized footer to idle when no
  working cue matches. codex retains that chrome during work. herdr documented
  the same ambiguity and removed the inference in
  [pr 4563](https://github.com/herdrdev/herdr/pull/4563), following
  [issue 4507](https://github.com/herdrdev/herdr/issues/4507).
  skid's own earlier `docs/agent-status-research.md:52` already records this limit.
- claude's [supported custom status line](https://code.claude.com/docs/en/statusline)
  suppresses the interruption hint while retaining footer badges. the detector
  conditions its standalone activity recognition on that hint at lines 73 and
  109-110. recognized remaining footer chrome may produce false idle; other
  layouts produce unknown. no actual user configuration was inspected.
- `detect.go:25,113` requires a selected numbered choice plus a narrow
  confirmation footer. claude question footers insert navigation instructions;
  codex questions use answer/all submission and navigation footers. these are
  distinct screen families, not minor spelling variants of one permission dialog.
  [claude report and versions](https://github.com/anthropics/claude-code/issues/92694),
  [codex question renderer](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/bottom_pane/questions.rs).
- status receives no title or progress input. `internal/sessions/naming.go:102`
  reads the pane title for naming only. herdr separately consumes title/progress
  and provider-specific screen regions. a title is not intrinsically current
  process evidence; static titles and missing progress never prove idle.
- `service.go:73` gives every concurrent session sample one shared two-second
  budget. each capture resolves and enriches the target before and afterward
  (`internal/sessions/control.go:82`). errors collapse to unavailable. excessive
  observation cost is a plausible explanation, not measured evidence. diagnose
  the failing stage and elapsed time before changing budgets.

herdr comparison: inspected release 0.9.3 (`7eaf574b`) and master `347f9c99`.
the [codex rules](https://github.com/herdrdev/herdr/blob/347f9c99bc95672e5ebcb26753648e85801d3c71/src/detect/manifests/codex.toml)
include action-required titles, title activity, question submission, trust/update
dialogs, transcript views and broader live activity shapes. the
[claude rules](https://github.com/herdrdev/herdr/blob/347f9c99bc95672e5ebcb26753648e85801d3c71/src/detect/manifests/claude.toml)
also cover navigation-rich forms, background agents/tasks, side questions,
workflow prompts, elicitation and approval variants. this is materially broader
than skid. herdr's optimistic idle fallback for non-codex agents and retained
state behind viewers are not correctness guarantees. master also has newer
codex lifecycle hooks; those are separate from heuristic coverage and outside
this investigation's proposed scope.

the useful borrowing is region-aware provider parsing and a content-free
explanation of which rule or failure produced the result. do not import a
manifest updater, hook runtime, transcript reader or execution supervisor to
obtain those properties. compare the terminal integration boundary: herdr owns
its terminal emulator; skid consumes tmux's projection. tmux 3.7 exposes
`pane_pb_state` and `pane_pb_progress`; 3.4 and 3.6a source do not. source support
does not establish installed-fleet support or provider emission. see the
[tmux 3.7 manual source](https://github.com/tmux/tmux/blob/3.7/tmux.1).

additional terminal-signal constraints:

- codex 0.159.2 has a configurable action-required title, activity titles and an
  optional explicit run-state item. its run-state waiting label means waiting
  for a background terminal, not a human. title-generation animation is separate
  from execution activity. inspect the
  [versioned status/title owner](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/chatwidget/status_surfaces.rs).
  disabled or custom title items remove these signals; absence proves nothing.
- static inspection of installed claude 2.1.285, build
  `afb212976052ab038df25d5e871f6c049e094d3b`, found that its default multiplexer
  title branch uses the same static marker for busy and idle. the branch is near
  executable byte offset 204271236. this is build-specific implementation
  evidence, not a public integration contract; no private session was inspected.
  copying herdr's static-title idle rule would be unsound for that branch.
- the same claude build's progress selector, near offset 202647200, includes
  pending tools/background agents/workflows as active. progress can corroborate
  activity, not classify an input request or prove task success. codex progress
  emission was not established; its osc 9 notification is distinct from osc 9;4
  progress. [upstream progress request](https://github.com/openai/codex/issues/37032).
- [claude accessibility modes](https://code.claude.com/docs/en/accessibility)
  change screen structure, animation and menu controls. their bell may announce
  a reply, a dialog or a tool finishing. codex also uses bells for several
  notification kinds. a bell is an attention hint, not a state or outcome;
  the bell terminator inside an osc sequence is not a standalone bell event.

screen families to qualify, rather than one growing footer expression:

| family | evidence sought | interpretation limit |
| --- | --- | --- |
| active turn/tools | current activity row, timer, provider structure | optional glyph/key hint and wrapped detail rows |
| retry/provider wait | live retry or wait controls | not idle; not a human request |
| background work | current agent/task/workflow summary | main composer can be empty |
| permission | request heading plus decision controls | ordinary selection menus are insufficient |
| structured questions | question form, answer/navigation controls | include free-text and multiple questions |
| collapsed codex question | question count plus answer affordance | work may continue |
| plan review | plan-specific confirmation/refinement controls | generic picker implementation is not generic meaning |
| mcp elicitation | input-request heading plus accept/decline controls | may lack enter-to-confirm wording |
| trust/login/update | recognized setup controls | pre-turn attention, not task failure |
| model/settings menus | recognized navigation surface | user interaction, not necessarily agent-requested |
| transcript/side-question views | viewer/overlay structure | main execution may be hidden |
| interruption/error notice | current structured provider notice | last visible event, not all-work termination |
| ready composer | complete provider-specific layout | codex composer alone is indistinguishable during work |
| provider absent | fresh foreground/process evidence | cause is not necessarily crash |

reproduction and qualification, requiring current-turn approval for tmux/live
work: use disposable provider sessions on an isolated socket; record only
expected/observed categories, detector rule/failure stage, dimensions and timing.
cover large styled screens with intact lower controls; narrow/wrapped screens;
default/custom footer; reduced motion and remapped keys; active commentary,
reasoning, tools and background waits; question and approval variants; menus and
transcript views; provider exit/replacement; stale title/progress; capture failure
and a representative number of simultaneous sessions. include quoted historical
chrome as negative cases. existing qualification does not cover this full matrix.

resolved when: the bounded capture observes the intended complete regions;
supported screen families have positive recognition; indistinguishable codex
composer states do not become idle; unrelated historical content cannot win;
status recognition cannot weaken composer/draft protection; and each unknown or
unavailable result has a content-free diagnostic cause. qualify the actual
supported provider/tmux versions at the live boundary. show the explicit cost
of conservative unknown and which idle waits remain unsupported.

blockers: implementation and live qualification are `NOT_RUN`. the accepted plan
authorizes the new screen/diagnostic contract; titles/progress are excluded.
the user reaffirmed temporary red/green/refactor probes deleted before commit,
with no retained corpus or testing-policy change.
