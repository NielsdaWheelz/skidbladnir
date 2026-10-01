# terminal observation has linux qualification gaps

problem: linux qualification covers fewer families and acceptance rows than
darwin, and with other versions. the installed linux pairing is codex TUI
0.159.2 against daemon 0.159.3 and claude 2.1.284, on devbox (tmux 3.4) and arch
(tmux 3.7c). every required row of the
[codex](../terminal-observation-codex.md#1-frozen-capability-table) and
[claude](../terminal-observation-claude.md#1-frozen-capability-table) capability
tables that does not name linux is `NOT_RUN` there; darwin evidence does not cover
it, because claude 2.1.284 already draws a required family differently from
2.1.286 (its numbered theme picker).

impact: the linux fleet runs families whose rendering was never observed with its
installed versions. the largest gaps:

- codex: the status row's suffix and cut forms, background-terminal waiting and
  `Thinking`; the submit window, goal continuation, external editor, side view and
  the other ambiguity controls; every approval but exec; mcp forms; multi, hidden,
  notes and async questions; the plan decision; update and login setup; the
  searchable and resume pickers, pagers, transcript footers and warnings view; a
  known terminal background; every narrow width (all codex rows ran at 100
  columns).
- claude: tool, narrow, child-block and retry spinners; custom statuslines;
  Write/Edit/MCP/workflow permissions; free-text, multi, preview and review
  questions; plan, exit-plan and enter-plan dialogs; elicitation; login, project
  mcp and settings-error setup; the `/theme` and other menus; the screen-reader
  renderer entirely; classic beyond idle, draft, work, a single question, Bash
  permission and tall panes; permission modes other than default and bypass.
- recognition: the installed claude's launch spellings
  ([claude launch spelling](claude-launch-spelling.md)).
- product: gateway → cli/desktop, attention, filter, guarded send, text, keys,
  stop and close, and `Terminal.ObservationFailed` logging; resolve- and
  capture-stage deadlines and a stalled tmux server; the spec §8 negatives beyond
  composer during codex work, daemon disconnect and provider → shell; real
  byte-capped clip frames; tmux 3.4's width-divergence classes and OSC 8 URI
  length limits.

evidence: the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification)
records what ran on both hosts: capture, authored corpora, recognition,
admission, the managed create path, enrichment cost, and the codex and claude
families the capability tables mark linux.

resolved when: each required family runs live on linux with the installed
versions, or the darwin and linux versions are aligned and a version-difference
check shows the family renders identically, recorded in the capability tables;
[claude launch spelling](claude-launch-spelling.md) is resolved; the product,
attention, controls, deadline and negative rows run through a real linux gateway;
then delete this record.
