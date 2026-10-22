# provider hidden behind a foreground launcher

problem: stock npm or waiting-shell launchers can remain the terminal's
foreground process-group leader while the native provider runs as a child.
skid observes only the leader, so a healthy provider can remain unrecognized.
this is independent of the resolved [claude update defect](../claude-update-recognition.md)
and excluded from that implementation.

impact: the session can read as an ordinary terminal, omitting provider-derived
status/footer/usage and optional runtime identity. terminal attachment and
explicit terminal operations remain available. existing separately captured
native conversations are not thereby retargeted.

evidence:

- `internal/process/process.go`, `ObserveForeground`, reads the foreground group
  id and observes the process with that pid; it does not discover children.
- `internal/agentruntime/runtime.go`, `ClassifyForeground`, classifies that one
  observation. config recognizes native images, not a node or waiting shell.
- the [stock npm codex launcher](https://github.com/openai/codex/blob/main/codex-cli/bin/codex.js)
  spawns the native binary with inherited stdio and waits for it instead of
  replacing the launcher. this is static source evidence, not new live qualification.
- existing [qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/terminal-agent-control-qualification.md) explicitly
  recorded npm-led codex as a generic terminal under the previous acceptance.

resolved when separately accepted scope and actual darwin/linux evidence show
the supported stock npm and waiting-shell launches recognized without changing
how the user starts them; identity remains coherent through profile observation,
capture and controls; background/ambiguous cases cannot select an unrelated
provider; raw launches do not depend on hooks; gateway restart rediscovers work.
record exact supported boundaries and remaining limitations. no discovery
mechanism, schema or control/hook change is designed or authorized by this issue.
