# initial agent and instant prompt

problem: the reported devbox codex launch inherited non-terminal stdin while
powerlevel10k instant prompt still owned the descriptors during the first
`precmd`. the provider rejected launch; skid's shell remained.

impact: an initial interactive provider cannot use its terminal. quiet mode only
hides the theme warning.

repair in this worktree: dev-server guards its cached preamble for the exact
pending skid startup shell, installs that guard into recognized existing
preambles, and supports its explicit inverse. it leaves theme configuration
alone. skid checks terminal stdin/stdout before launch and reports actual
nonzero command status. the deployment copy matches the product source.

evidence, 2026-09-29: temporary local controlling-pty tests used zsh 5.9,
dev-server's managed startup files, its pinned powerlevel10k
`3308262dfbd743b6e1d3956a2b5572f7a049d692`, and the real `agent-exec`
with a content-free stub provider. warming an ordinary shell reproduced the
original descriptor capture and warning; installation corrected the same
existing cache. foreground input, resize, suspend/resume, same-shell command
execution, literal cwd/argv, profile isolation, one-shot sourcing, nonzero exit,
ordinary/descendant policy and installer reversal were exercised.
the separate launch-boundary matrix covered bash 3.2/5.3 and zsh 5.9 with
`errexit` enabled at callback invocation, including redirected stderr.
after rebasing, the startup guard, installer, terminal admission and command
status were checked again against current main.
temporary fixtures are removed under the [testing policy](../rules/testing.md).

tradeoffs: new skid startup actions lose the early preview. the theme can discard
its shared cache after a skipped preamble; later ordinary shells may need to
warm it again. custom instant-prompt snippets must use the recognized managed
preamble before installation; the installer rejects unfamiliar or duplicated
snippets. no extra pty, retries, private theme calls or provider lifecycle state.

remaining: installed devbox generation and actual startup stack were not
inspected or changed. gateway/tmux/provider/fleet/phone acceptance is `NOT_RUN`.
both source pins now name v0.10.6. the reported installed-fleet qualification
of that release does not qualify this fix. deployment startup edits are outside
runtime rollback;
use the matching dev-server runbook's `restore-instant-prompt` operation.

resolved when: with current-turn approval, qualify the corrected installed
startup stack on an owned isolated tmux socket with the host's existing warm
cache. verify input/job control, same-shell survival and one-shot launch, then
remove this issue. broader [shell qualification](terminal-continuity-shell-qualification.md)
and native interaction qualification remain separate.
