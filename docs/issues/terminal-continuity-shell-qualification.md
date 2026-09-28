# terminal shell qualification

problem: the shipped `v0.10.3` shell path has only partial live acceptance.
the initial-agent path, special-character cwd and argv, job control, input and
resize, startup-child isolation, and repeated sourcing have no recorded complete
journey on the installed macbook, arch, and devbox generation.

impact: a new provider could fail to return to its original shell, or startup
integration could launch it more than once. the installed release is not a pass
for those cases.

evidence: [pr 3](https://github.com/NielsdaWheelz/skidbladnir/pull/3)
proved an isolated linux gateway, tmux, and bash shell surviving an exiting test
agent. the `v0.10.3` delivery record in
[pr 10](https://github.com/NielsdaWheelz/skidbladnir/pull/10) reports isolated
mac zsh/bash and the named phone shell journey. neither records the complete
[shell acceptance row](../terminal-continuity.md#6-delivery-and-adversarial-acceptance)
on the final three-host installation. the remaining cases are `NOT_RUN`.

resolved when: with current-turn approval, exercise both creation kinds and the
listed shell behaviours on test-owned isolated tmux sockets using the installed
generation and actual host startup files. retain only content-free outcomes and
remove this record when the row passes.
