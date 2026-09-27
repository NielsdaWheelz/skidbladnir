# darwin terminal continuity qualification

problem: the changed native foreground environment, cwd, boot identity, and
login shell paths have not run on a mac. the linux host cannot compile the
darwin cgo process observer without an appropriate cross compiler.

impact: mac terminal and remote context behavior remains `NOT_RUN`.

evidence: linux go engineering checks and isolated tmux proofs passed; no
darwin executable or isolated mac tmux proof ran.

resolved when: on the mac, build the changed source and run isolated `tmux -L`
live proofs for bash/zsh agent exit, cwd/home labels, remote tty registration,
and provider replacement. remove this record then.
