# initial agent starts before instant prompt releases the terminal

problem: skid invokes its initial foreground agent from zsh's first `precmd`.
powerlevel10k instant prompt can still own the shell's standard descriptors at
that point. reading all startup files does not establish terminal readiness.

impact: the initial provider can reject non-terminal stdin, leaving the shell
and any previously created native conversation. suppressing the powerlevel10k
warning does not restore stdin. this affects the shared initial-agent boundary;
the reported failure is codex on devbox.

evidence, 2026-09-29:

- the operator reported a powerlevel10k startup warning, a non-terminal-stdin
  error, and skid's fixed initial-agent failure diagnostic.
- [shell-init](../../deployment/providers/shell-init) registers `_skid_startup`
  through `add-zsh-hook precmd` and invokes `agent-exec` before returning.
  [agent-exec](../../cmd/skidbladnir/agent_exec.go) preserves inherited descriptors
  when replacing itself with the configured provider.
- the sibling dev-server checkout's `assets/dotfiles/zshrc` sources the cached
  instant prompt near its beginning and skid integration at its end.
- dev-server pins powerlevel10k `3308262dfbd743b6e1d3956a2b5572f7a049d692`.
  the locally readable theme checkout matches that pin. its
  [`internal/p10k.zsh`](https://github.com/romkatv/powerlevel10k/blob/3308262dfbd743b6e1d3956a2b5572f7a049d692/internal/p10k.zsh)
  captures descriptors at lines 6491–6493 and restores them through prompt
  expansion at lines 6022 and 6935–6937, after the precmd callbacks.
  `p10k finalize` only adjusts prompt options at lines 9506–9508.
- [upstream documentation](https://github.com/romkatv/powerlevel10k#how-do-i-configure-instant-prompt)
  describes stdin redirected to `/dev/null` and stdout/stderr captured in a file.
  quiet mode only suppresses its warning.

this is source-supported diagnosis, not an installed-host reproduction. the
affected host's active generation, actual startup files and descriptor state
were not inspected. no tmux, provider, remote-host or phone operation was run.
the existing [shell qualification](terminal-continuity-shell-qualification.md)
and [installed native qualification](native-agent-release-qualification.md)
remain open.

proposed repair, not an accepted scope change: make the installer-owned shell
composition prevent instant-prompt descriptor capture for the exact shell with
a pending skid startup action, before sourcing its cached preamble. preserve the
normal foreground child and same-shell return. the managed p10k configuration
clears prior `POWERLEVEL9K_*` values and sets verbose mode, so an early assignment
alone is not a complete configuration design. use supported configuration;
do not reorder private theme hooks or call private descriptor helpers.
qualify a small explicit terminal precondition and content-free exit diagnostic
at skid's launch boundary. no retry, extra pty or provider lifecycle registry.

deployment constraints: the inspected dev-server checkout pins v0.10.3, while
this checkout pins v0.10.5; neither pin establishes the running host generation.
dev-server's `lib/skidbladnir.sh` renders its own `assets/skid-provider/shell-init`,
so changing the product template alone will not deliver the fix. gateway-only
apply stages no managed dotfiles, and its existing shell installer appends the
late integration block. delivering an early policy requires an explicit scoped
startup update or the appropriate managed-dotfile deployment. startup files sit
outside runtime generations; rolling back a gateway does not reverse those
edits. qualify the policy's explicit reversal separately.

reproduction and resolution: with current-turn approval, use an owned isolated
tmux socket and the installed bash/zsh and managed startup stack. demonstrate
the original failure with an active warm instant-prompt cache; a cold cache is
not an equivalent boundary. verify the corrected path has terminal descriptors
and proper foreground job control, launches once, preserves cwd/argv and account
selection, and returns to the same shell after normal exit and failure. include
input, resize, suspend/resume, repeated sourcing and startup-child isolation.
verify ordinary and herdr shells retain their policy, and installer repeat apply
and rollback preserve the intended startup composition. use temporary tests
under the [testing policy](../rules/testing.md); retain only content-free results.
installed behavior remains `NOT_RUN` until that boundary is exercised.
