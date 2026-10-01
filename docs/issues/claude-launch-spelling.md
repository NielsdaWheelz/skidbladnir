# installed claude launch spellings are unqualified on linux

problem: recognition compares the kernel's foreground executable with the
configured `executablePath`, resolved at each comparison, and never reads argv
([observation §4](../terminal-observation.md#4-provider-adapters-and-managed-displays)),
so command spelling should not matter. on linux that is proven only with a
native fake image: the installed claude 2.1.284 was recognized through its
configured path alone. its bare, relative, second-symlink and exec-wrapper
launches, and the controls through it, are `NOT_RUN` there.

impact: if the installed claude reaches a different image under some linux
spelling (a re-exec or a launcher of its own), that session reads as a generic
terminal: status unknown and guarded send refused. text, keys, stop and close
stay available. the failure is safe but would hide ordinary claude work.

evidence, from the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification):

- darwin meets the original criteria. bare, relative, absolute and symlinked
  launches of a native image and of claude 2.1.286 are recognized; copies,
  wrappers and unrelated programs are not; exit, exec replacement,
  suspend/resume, respawn and relink behave; controls pass through the real
  provider.
- linux, devbox and arch, against `/proc` with a native fake image: bare,
  relative, absolute, second-symlink and exec-wrapper launches recognized; a
  script wrapper, same-basename copies and an unrelated program unrecognized;
  exit, exec replacement in both directions, ctrl-z/`fg` and relink behave.
- linux, installed claude 2.1.284: recognized on both hosts through
  `executablePath` resolved per comparison to its versioned image, typed through
  the configured path only. linux controls are `NOT_RUN` for every provider
  ([linux coverage](terminal-observation-linux-coverage.md)).

resolved when: on linux, with current-turn approval, the installed claude
launched bare, relatively, through a second symlink and through an exec wrapper
classifies as it does through its configured path, and guarded send, text,
keys, stop and close pass on one such launch through a real gateway; or a waiver
of the fake-image substitution is recorded in the qualification. then delete
this record.
