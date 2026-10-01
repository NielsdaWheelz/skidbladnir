# claude required screen families not run on darwin

problem: these required rows of the
[claude grammar](../terminal-observation-claude.md#1-frozen-capability-table) are
`NOT_RUN` live on darwin with claude 2.1.286:

- row 4: the spinner row wrapped at a narrow width;
- row 5: the `Next:` task, expanded task list and compaction hint child blocks;
- rows 7 and 8: the overload child and the stalled, no-response, low-priority and
  auto-mode-check retry variants (repeated 529s ended in an `API Error` row);
- row 10: nested panel rows; row 12: a mixed agents-and-workflows waiting row;
- row 18: working under a banner top rule;
- row 34: Fetch, notebook, subagent- and plugin-sourced permissions;
- row 37: a question with its countdown or plugin notice row;
- row 38: the two-question preview hint;
- row 46: the transcript viewer over a waiting dialog;
- row 54: screen-reader bypass, mcp and login setup;
- row 69: quoted chrome, live (authored frames only);
- row 73: the vim mode prefix; row 74: compaction and hook spinners;
- row 88: the server-gated unified footer;
- spec §8 negatives: the title-generation spinner and the static claude title
  across provider → shell → same provider (the classifier never reads titles, so
  these hold by construction, unproven live);
- rows 41 and 58 in the classic renderer: plan approval (rule 15 reads column 3;
  classic draws the dialog at column 1) and `/model` (rule 4 reads its hint at
  column 3 under an edge; classic draws it at column 2 under a full rule) read
  unknown/unknown at every width. both pass live in fullscreen, which 2.1.286
  draws by default; classic is opt-in.
- spec §8 controls: the byte check of claude's dialog refusal. the refusal itself
  passed live (`TerminalInputBlocked`, not sent); no endpoint or pane byte count
  was taken, so codex alone carries the dialog no-bytes proof.

impact: each reads by bundle inspection and authored frames only. a rendering
difference would misclassify the family, usually as unknown; rows 7, 8, 18 and 74
are working cues whose loss would read unknown during work.

evidence: the grammar's capability table and the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification).
the request dialogs without a rule are recorded
[separately](claude-unruled-request-dialogs.md); linux coverage
[separately](terminal-observation-linux-coverage.md).

resolved when: each listed family runs live on darwin through the real capture and
classifier with its expected values, or is shown unreachable in the qualified
configuration and moved to class `U` with a stated reason, and the capability
table marks it (the qualification records the spec §8 items); then delete this
record.
