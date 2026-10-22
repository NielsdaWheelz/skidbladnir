# terminal context init prints a tty error in zsh without a terminal

problem: `deployment/providers/terminal-context-init:3` guards with
`: 2>/dev/null </dev/tty`. bash suppresses the failed redirection, but zsh
reports it on the shell's own stderr. so an interactive zsh with no
controlling terminal prints
`…/providers/terminal-context-init:3: device not configured: /dev/tty`. the
guard still returns correctly.

impact: tools that start `zsh -i` without a tty get the noise in captured
stderr, for example agent shell probes and scripts. the behavior is correct;
only the output is polluted.

evidence: 2026-10-21, zsh 5.9 on macos, stdin from `/dev/null` and no
controlling tty:
- `zsh -f -c '[[ 1 == 1 ]] && : 2>/dev/null </dev/tty || echo returned'`
  prints `zsh:1: device not configured: /dev/tty`, then `returned`.
- the grouped form `{ : </dev/tty; } 2>/dev/null` prints only `returned`.
- bash `--norc` prints only `returned` for both forms.
- the error was first seen from `zsh -i -c exit` on the macbook.

resolved when line 3 uses the grouped redirection, and a zsh and a bash
interactive shell started without a tty print nothing from this file, while
tty-scoped behavior is unchanged.
