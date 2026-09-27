# darwin terminal continuity qualification

problem: isolated mac zsh, native foreground observation, and ssh/mosh routes
passed. the explicit mac bash login-shell path remains unqualified.

impact: a mac bash startup path could omit the scoped integration or fail to
return to its shell after provider exit.

evidence: the installed `v0.10.2` darwin binary on a test-owned `tmux -L`
socket created a terminal, changed cwd, observed stock codex, returned to its
zsh shell on `/exit`, replaced the provider, and resolved ssh/mosh remote
contexts. the darwin observer read the marker from homebrew openssh; macos
withheld environment bytes from its platform `/usr/bin/ssh`. mac bash is
`NOT_RUN`.

resolved when: run the mac bash login-shell path on a test-owned isolated
socket and verify cwd, provider home, exit to shell, and replacement. remove
this record then.
