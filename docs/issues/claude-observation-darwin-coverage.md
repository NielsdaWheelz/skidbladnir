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
  these hold by construction, unproven live).

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
table marks it; then delete this record.
