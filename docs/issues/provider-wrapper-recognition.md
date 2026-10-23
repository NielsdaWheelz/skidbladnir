# provider hidden behind a foreground launcher

problem: skid observes only the terminal's foreground process-group leader
(`ObserveForeground`, `internal/process/process.go:49`) and recognizes an
executable named `codex` or `claude`, or one inside claude's versions directory
(`Classify`, `internal/agentruntime/runtime.go:21-40`). a launcher that stays the
leader while the native provider runs as its child, such as stock npm's codex
launcher or a waiting shell, hides the provider.

impact: the session reads as an ordinary terminal without provider status, footer,
usage or runtime identity. attachment and explicit terminal operations still work.

evidence: source reading; the
[npm codex launcher](https://github.com/openai/codex/blob/main/codex-cli/bin/codex.js)
spawns the native binary with inherited stdio and waits for it.

resolved when: under an accepted scope, with darwin and linux evidence, stock npm
and waiting-shell launches are recognized without changing how the user starts
them; identity stays coherent through profile observation, capture and controls;
background or ambiguous children never select an unrelated provider; raw launches
need no hooks; a gateway restart rediscovers work. no discovery mechanism is
authorized by this record.
