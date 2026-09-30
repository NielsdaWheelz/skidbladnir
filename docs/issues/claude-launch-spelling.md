# claude recognition depends on command spelling

problem: the configured claude foreground signature compares only the exact
absolute `argv[0]`. the same native executable launched with bare `claude`, a
different symlink, or a relative path can fail recognition. this limits the
[terminal contract](../terminal-agent-control.md), which requires observation
and controls to follow the current agent without tracking ceremony.

impact: when recognition fails, claude terminal status is unknown and guarded
send refuses. terminal read, deliberate text/keys, interrupt and closure remain
available. skid's own launchers execute the configured absolute command, so
their success does not qualify ordinary shell invocation.

evidence, 2026-09-30:

- `deployment/providers/host-config.json` supplies only `argument0` for claude.
  `internal/agentruntime/runtime.go:matchesSignature` compares it literally;
  the observed executable does not participate in that signature.
- a temporary native executable launched through clean macos bash and zsh
  receives relative `argv[0]` through path lookup and absolute `argv[0]` through
  an absolute invocation.
- a temporary probe of the actual `ClassifyForeground` function recognizes an
  observation with the configured absolute argument, then rejects the same
  executable/pid/start observation with bare `argv[0]`.
- installed macbook configuration uses the same argument-only signature. its
  configured command resolves to an executable whose basename is not `claude`;
  changing the predicate to that basename alone is not an established repair.

the probes were removed. they prove shell and classifier behavior, not the
installed provider's eventual process arguments: claude could rewrite them.
actual unmanaged claude, linux, tmux and phone checks are `NOT_RUN` here.

reproduction: on an approved isolated tmux socket, compare the same installed
claude reached through the skid profile, marked-shell `claude-work`, ordinary
path lookup, absolute command and another symlink. compare provider presence,
inferred status and guarded input using content-free results. include normal
permission modes as well as bypass mode: the current detector requires specific
footer patterns, so recognition alone does not establish status coverage.

resolved when: supported invocations of the same installed provider classify
consistently on linux and darwin using verified executable evidence independent
of command spelling. unrelated programs remain unclassified; exit, replacement
and suspend/resume lose or restore the correct foreground identity. qualify
status and controls through the real provider boundary. any expanded signature
contract belongs to the architecture/configuration owners.
